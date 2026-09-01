# RFC 4035 - Protocol Modifications for the DNS Security Extensions

Partial. Every requirement this repository extracted from RFC 4035, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 29.4% | 5 of 17 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 52.9% | 9 of 17 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 17 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 19 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 108 | of 158 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 91 | of 108 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 17.6% | 3 of 17 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 17 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 158 |
| Gated MUST-level | 108 |
| Obligations that bind Ze | 17 |
| Not applicable, so out of scope | 91 |
| Declared gaps | 3 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 19 |
| Tagged units | 19 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Annotated instead of tested | 103 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **108** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC4035-3-8`](#rfc4035-3-8), [`RFC4035-3-9`](#rfc4035-3-9), [`RFC4035-3.1.4.1-1`](#rfc4035-3.1.4.1-1), [`RFC4035-4.6-3`](#rfc4035-4.6-3), [`RFC4035-4.9-1`](#rfc4035-4.9-1)

**Annotated instead of tested (103):** [`RFC4035-2.1-2`](#rfc4035-2.1-2), [`RFC4035-2.1-4`](#rfc4035-2.1-4), [`RFC4035-2.1-5`](#rfc4035-2.1-5), [`RFC4035-2.2-1`](#rfc4035-2.2-1), [`RFC4035-2.2-3`](#rfc4035-2.2-3), [`RFC4035-2.2-4`](#rfc4035-2.2-4), [`RFC4035-2.2-5`](#rfc4035-2.2-5), [`RFC4035-2.2-6`](#rfc4035-2.2-6), [`RFC4035-2.2-7`](#rfc4035-2.2-7), [`RFC4035-2.2-8`](#rfc4035-2.2-8), [`RFC4035-2.3-1`](#rfc4035-2.3-1), [`RFC4035-2.3-3`](#rfc4035-2.3-3), [`RFC4035-2.3-4`](#rfc4035-2.3-4), [`RFC4035-2.3-5`](#rfc4035-2.3-5), [`RFC4035-2.3-6`](#rfc4035-2.3-6), [`RFC4035-2.3-7`](#rfc4035-2.3-7), [`RFC4035-2.4-3`](#rfc4035-2.4-3), [`RFC4035-2.4-4`](#rfc4035-2.4-4), [`RFC4035-2.5-1`](#rfc4035-2.5-1), [`RFC4035-2.5-2`](#rfc4035-2.5-2), [`RFC4035-2.6-1`](#rfc4035-2.6-1), [`RFC4035-3-1`](#rfc4035-3-1), [`RFC4035-3-2`](#rfc4035-3-2), [`RFC4035-3-5`](#rfc4035-3-5), [`RFC4035-3-6`](#rfc4035-3-6), [`RFC4035-3.1-1`](#rfc4035-3.1-1), [`RFC4035-3.1-2`](#rfc4035-3.1-2), [`RFC4035-3.1-3`](#rfc4035-3.1-3), [`RFC4035-3.1.1-3`](#rfc4035-3.1.1-3), [`RFC4035-3.1.1-4`](#rfc4035-3.1.1-4), [`RFC4035-3.1.1-5`](#rfc4035-3.1.1-5), [`RFC4035-3.1.1-6`](#rfc4035-3.1.1-6), [`RFC4035-3.1.1-8`](#rfc4035-3.1.1-8), [`RFC4035-3.1.2-3`](#rfc4035-3.1.2-3), [`RFC4035-3.1.2-4`](#rfc4035-3.1.2-4), [`RFC4035-3.1.3-1`](#rfc4035-3.1.3-1), [`RFC4035-3.1.3.1-1`](#rfc4035-3.1.3.1-1), [`RFC4035-3.1.3.2-1`](#rfc4035-3.1.3.2-1), [`RFC4035-3.1.3.3-1`](#rfc4035-3.1.3.3-1), [`RFC4035-3.1.3.3-2`](#rfc4035-3.1.3.3-2), [`RFC4035-3.1.3.4-1`](#rfc4035-3.1.3.4-1), [`RFC4035-3.1.4-1`](#rfc4035-3.1.4-1), [`RFC4035-3.1.4-2`](#rfc4035-3.1.4-2), [`RFC4035-3.1.4-3`](#rfc4035-3.1.4-3), [`RFC4035-3.1.5-2`](#rfc4035-3.1.5-2), [`RFC4035-3.1.5-3`](#rfc4035-3.1.5-3), [`RFC4035-3.1.5-4`](#rfc4035-3.1.5-4), [`RFC4035-3.1.5-5`](#rfc4035-3.1.5-5), [`RFC4035-3.1.5-6`](#rfc4035-3.1.5-6), [`RFC4035-3.1.5-7`](#rfc4035-3.1.5-7), [`RFC4035-3.1.6-2`](#rfc4035-3.1.6-2), [`RFC4035-3.1.6-4`](#rfc4035-3.1.6-4), [`RFC4035-3.1.6-5`](#rfc4035-3.1.6-5), [`RFC4035-3.1.6-6`](#rfc4035-3.1.6-6), [`RFC4035-3.2.1-1`](#rfc4035-3.2.1-1), [`RFC4035-3.2.1-2`](#rfc4035-3.2.1-2), [`RFC4035-3.2.1-3`](#rfc4035-3.2.1-3), [`RFC4035-3.2.2-1`](#rfc4035-3.2.2-1), [`RFC4035-3.2.2-4`](#rfc4035-3.2.2-4), [`RFC4035-3.2.3-2`](#rfc4035-3.2.3-2), [`RFC4035-4.1-1`](#rfc4035-4.1-1), [`RFC4035-4.1-2`](#rfc4035-4.1-2), [`RFC4035-4.1-4`](#rfc4035-4.1-4), [`RFC4035-4.1-5`](#rfc4035-4.1-5), [`RFC4035-4.2-1`](#rfc4035-4.2-1), [`RFC4035-4.2-3`](#rfc4035-4.2-3), [`RFC4035-4.2-5`](#rfc4035-4.2-5), [`RFC4035-4.2-6`](#rfc4035-4.2-6), [`RFC4035-4.3-1`](#rfc4035-4.3-1), [`RFC4035-4.4-1`](#rfc4035-4.4-1), [`RFC4035-4.6-2`](#rfc4035-4.6-2), [`RFC4035-4.7-2`](#rfc4035-4.7-2), [`RFC4035-4.7-3`](#rfc4035-4.7-3), [`RFC4035-4.7-7`](#rfc4035-4.7-7), [`RFC4035-4.8-1`](#rfc4035-4.8-1), [`RFC4035-4.9.1-2`](#rfc4035-4.9.1-2), [`RFC4035-4.9.3-2`](#rfc4035-4.9.3-2), [`RFC4035-5-1`](#rfc4035-5-1), [`RFC4035-5-2`](#rfc4035-5-2), [`RFC4035-5-3`](#rfc4035-5-3), [`RFC4035-5.2-1`](#rfc4035-5.2-1), [`RFC4035-5.2-3`](#rfc4035-5.2-3), [`RFC4035-5.3.1-1`](#rfc4035-5.3.1-1), [`RFC4035-5.3.1-2`](#rfc4035-5.3.1-2), [`RFC4035-5.3.1-3`](#rfc4035-5.3.1-3), [`RFC4035-5.3.1-4`](#rfc4035-5.3.1-4), [`RFC4035-5.3.1-5`](#rfc4035-5.3.1-5), [`RFC4035-5.3.1-6`](#rfc4035-5.3.1-6), [`RFC4035-5.3.1-7`](#rfc4035-5.3.1-7), [`RFC4035-5.3.1-8`](#rfc4035-5.3.1-8), [`RFC4035-5.3.1-9`](#rfc4035-5.3.1-9), [`RFC4035-5.3.1-10`](#rfc4035-5.3.1-10), [`RFC4035-5.3.2-1`](#rfc4035-5.3.2-1), [`RFC4035-5.3.2-2`](#rfc4035-5.3.2-2), [`RFC4035-5.3.2-3`](#rfc4035-5.3.2-3), [`RFC4035-5.3.3-1`](#rfc4035-5.3.3-1), [`RFC4035-5.3.3-2`](#rfc4035-5.3.3-2), [`RFC4035-5.4-1`](#rfc4035-5.4-1), [`RFC4035-5.4-2`](#rfc4035-5.4-2), [`RFC4035-5.4-3`](#rfc4035-5.4-3), [`RFC4035-5.4-4`](#rfc4035-5.4-4), [`RFC4035-5.5-2`](#rfc4035-5.5-2), [`RFC4035-5.5-3`](#rfc4035-5.5-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4035-2.1-2` | A zone key DNSKEY RR MUST have the Zone Key bit of the flags RDATA field set (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.1-4` | Public keys stored in DNSKEY RRs that are not marked as zone keys MUST NOT be used to verify RRSIGs (§2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.1-5` | For a signed zone usable other than as an island of security, the zone apex MUST contain at least one DNSKEY RR to act as a secure entry point (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-1` | For each authoritative RRset in a signed zone there MUST be at least one RRSIG record whose owner, class, Type Covered, Original TTL, TTL, Labels, and Signer's Name match the RRset and identify an apex zone key (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-3` | An RRSIG RR itself MUST NOT be signed (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-4` | The NS RRset that appears at the zone apex name MUST be signed (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-5` | The NS RRsets that appear at delegation points MUST NOT be signed (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-6` | Glue address RRsets associated with delegations MUST NOT be signed (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-7` | There MUST be an RRSIG for each RRset using at least one DNSKEY of each algorithm in the zone apex DNSKEY RRset (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.2-8` | The apex DNSKEY RRset MUST be signed by each algorithm appearing in the DS RRset at the delegating parent, if any (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-1` | Each owner name in the zone that has authoritative data or a delegation point NS RRset MUST have an NSEC resource record (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-3` | An NSEC record and its associated RRSIG RRset MUST NOT be the only RRset at any particular owner name (§2.3) | MUST NOT | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-4` | The signing process MUST NOT create NSEC or RRSIG RRs for owner name nodes that were not the owner name of any RRset before the zone was signed (§2.3) | MUST NOT | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-5` | The type bitmap of every NSEC RR MUST indicate the presence of both the NSEC record itself and its corresponding RRSIG record (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-6` | In the NSEC bitmap at a delegation point, bits for the delegation NS RRset and any RRsets for which the parent zone has authoritative data MUST be set (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.3-7` | In the NSEC bitmap at a delegation point, bits for any non-NS RRset for which the parent is not authoritative MUST be clear (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.4-3` | All DS RRsets in a zone MUST be signed (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.4-4` | DS RRsets MUST NOT appear at a zone's apex (§2.4) | MUST NOT | 2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.5-1` | If a CNAME RRset is present at a name in a signed zone, appropriate RRSIG and NSEC RRsets are REQUIRED at that name (§2.5) | REQUIRED | 2.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.5-2` | Types other than CNAME, its RRSIG and NSEC, and a KEY RRset for secure dynamic update MUST NOT be present at a CNAME name (§2.5) | MUST NOT | 2.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-2.6-1` | At the parental side of a zone cut, NSEC RRs are REQUIRED at the owner name (§2.6) | REQUIRED | 2.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| `RFC4035-3-1` | A security-aware name server MUST support the EDNS0 message size extension (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's authoritative servers give the EDNS0 message size extension no support -- the only SetEdns0 producer in ze is the stub resolver (internal/component/resolve/dns/resolver.go:261); the server path reads an OPT only for the client-subnet address (internal/core/dnsserver/client.go:23) and writes a reply built by msg.SetReply with no OPT pseudo-RR and no requestor-payload-size handling (internal/core/dnsserver/handler.go:55) |
| `RFC4035-3-2` | A security-aware name server MUST support a message size of at least 1220 octets (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the UDP listener accepts at most 512 octets -- dns.Server is constructed with UDPSize left zero (internal/core/dnsserver/manager.go:165), which miekg defaults to MinMsgSize 512 (vendor/github.com/miekg/dns/server.go:287), so a query above 512 octets is never read whole and no reply advertises a larger size |
| `RFC4035-3-5` | A name server receiving a query without the EDNS OPT pseudo-RR or with the DO bit clear MUST treat the RRSIG, DNSKEY, and NSEC RRs as it would any other RRset (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3-6` | Such a name server MUST NOT perform any of the DNSSEC additional processing (§3) | MUST NOT | 3 | **positive:** `unit/verify` [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L164). **negative:** no negative test. **{single-polarity}:** ze performs no DNSSEC additional processing for any query -- answerQuestions builds A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) and never consults the DO bit, so no processing path exists to drive negatively |
| `RFC4035-3-8` | A security-aware name server MUST copy the CD bit from a query into the corresponding response (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L51). **negative:** `unit/verify` [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L56) |
| `RFC4035-3-9` | A security-aware name server MUST ignore the setting of the AD bit in queries (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L62). **negative:** `unit/verify` [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L69) |
| `RFC4035-3.1-1` | Upon a DO-set query to a signed zone, RRSIG RRs that can be used to authenticate the response MUST be included (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1-2` | NSEC RRs that provide authenticated denial of existence MUST be included in the response automatically (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1-3` | Either a DS RRset or an NSEC RR proving that no DS RRs exist MUST be included in referrals automatically (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.1-3` | When placing a signed RRset in the Answer section, the name server MUST also place its RRSIG RRs in the Answer section (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.1-4` | If space does not permit inclusion of the RRSIG RRs that must accompany a signed RRset, or of a mandatory NSEC or DS RRset and its RRSIGs, the name server MUST set the TC bit (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.1-5` | When placing a signed RRset in the Authority section, the name server MUST also place its RRSIG RRs in the Authority section (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.1-6` | When placing a signed RRset in the Additional section, the name server MUST also place its RRSIG RRs in the Additional section (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.1-8` | The name server MUST NOT set the TC bit solely because RRSIG RRs did not fit in the Additional section (§3.1.1) | MUST NOT | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.2-3` | If there is not enough space for the apex DNSKEY RRset and its RRSIGs, the name server MUST omit them (§3.1.2) | MUST | 3.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.2-4` | The name server MUST NOT set the TC bit solely because the apex DNSKEY and RRSIG RRs did not fit (§3.1.2) | MUST NOT | 3.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3-1` | When responding to a DO-set query, the name server MUST include NSEC RRs in the No Data, Name Error, Wildcard Answer, and Wildcard No Data cases (§3.1.3) | MUST | 3.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3.1-1` | For a No Data response, the name server MUST include the NSEC RR for the queried name and its RRSIGs in the Authority section (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3.2-1` | For a Name Error response, the name server MUST include, with their RRSIGs, an NSEC RR proving no exact match and an NSEC RR proving no wildcard match (§3.1.3.2) | MUST | 3.1.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3.3-1` | For a Wildcard Answer response, the name server MUST include the wildcard-expanded answer and its wildcard-expanded RRSIG RRs in the Answer section (§3.1.3.3) | MUST | 3.1.3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3.3-2` | For a Wildcard Answer response, the name server MUST include in the Authority section an NSEC RR and its RRSIGs proving that no closer match exists (§3.1.3.3) | MUST | 3.1.3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.3.4-1` | For a Wildcard No Data response, the name server MUST include, with their RRSIGs, an NSEC RR proving no matching type at the wildcard owner name and an NSEC RR proving no closer match (§3.1.3.4) | MUST | 3.1.3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.4-1` | If a DS RRset is present at the delegation point, the name server MUST return the DS RRset and its RRSIGs in the Authority section with the NS RRset (§3.1.4) | MUST | 3.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.4-2` | If no DS RRset is present, the name server MUST return the NSEC RR proving the DS RRset is absent and its RRSIGs with the NS RRset (§3.1.4) | MUST | 3.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.4-3` | The name server MUST place the NS RRset before the NSEC RRset and its RRSIGs (§3.1.4) | MUST | 3.1.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| `RFC4035-3.1.4.1-1` | When authoritative for the child zone but not the parent and not offering recursion, on a DS query at the zone cut the name server MUST return an authoritative no-data response (§3.1.4.1) | MUST | 3.1.4.1 | **positive:** `unit/verify` [`TestRFC4035_DSQueryIsAuthoritativeNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L85). **negative:** `unit/verify` [`TestRFC4035_DSQueryIsAuthoritativeNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L112) |
| `RFC4035-3.1.5-2` | A name server performing its own zone validation MUST NOT selectively reject some RRs and accept others (§3.1.5) | MUST NOT | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.5-3` | The DS RRset MUST be included in zone transfers of the parent zone in which it is authoritative data (§3.1.5) | MUST | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.5-4` | NSEC RRs MUST be included in zone transfers of the zone in which they are authoritative data (§3.1.5) | MUST | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.5-5` | The parental NSEC RR at a zone cut MUST be included in zone transfers of the parent zone (§3.1.5) | MUST | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.5-6` | The NSEC at the zone apex of the child zone MUST be included in zone transfers of the child zone (§3.1.5) | MUST | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.5-7` | RRSIG RRs MUST be included in zone transfers of the zone in which they are authoritative data (§3.1.5) | MUST | 3.1.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| `RFC4035-3.1.6-2` | A security-aware name server MUST NOT set the AD bit in a response unless it considers all RRsets in the Answer and Authority sections to be authentic (§3.1.6) | MUST NOT | 3.1.6 | **positive:** `unit/verify` [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L177). **negative:** no negative test. **{single-polarity}:** no ze code sets AuthenticatedData on a reply -- the single non-test AuthenticatedData reference reads the upstream's bit in the stub resolver (internal/component/resolve/dns/resolver.go:278) -- so an AD-asserting response cannot be produced to reject |
| `RFC4035-3.1.6-4` | The name server MUST NOT treat authoritative-zone data as authentic unless it obtained the zone via secure means (§3.1.6) | MUST NOT | 3.1.6 | **positive:** `unit/verify` [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L179). **negative:** no negative test. **{single-polarity}:** ze takes its zone data from the local running configuration and still serves it with AD clear, because no reply-building path sets AuthenticatedData (internal/core/dnsserver/handler.go:55, internal/plugins/geodns/server.go:168); there is no AD-setting path to drive negatively |
| `RFC4035-3.1.6-5` | The name server MUST NOT treat authoritative-zone data as authentic unless this behavior has been configured explicitly (§3.1.6) | MUST NOT | 3.1.6 | **positive:** `unit/verify` [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L182). **negative:** no negative test. **{single-polarity}:** ze has no configuration leaf that marks local zone data authentic, so the AD bit stays clear whatever the operator configures (the geodns YANG carries no such leaf and no server path writes AuthenticatedData, internal/plugins/geodns/server.go:168) |
| `RFC4035-3.1.6-6` | A security-aware name server that supports recursion MUST follow the recursive-server CD and AD bit rules for data obtained via recursion (§3.1.6) | MUST | 3.1.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.1-1` | The resolver side of a security-aware recursive name server MUST set the DO bit when sending requests (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.1-2` | If the DO bit in an initiating query is not set, the name server side MUST strip any authenticating DNSSEC RRs from the response (§3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.1-3` | The name server side MUST NOT strip any DNSSEC RR types that the initiating query explicitly requested (§3.2.1) | MUST NOT | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.2-1` | The name server side MUST pass the state of the CD bit to the resolver side along with the initiating query (§3.2.2) | MUST | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.2-4` | If the CD bit is not set and the query matches a BAD cache entry, the name server side MUST return RCODE 2 (server failure) (§3.2.2) | MUST | 3.2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-3.2.3-2` | The resolver side MUST determine whether the RRs are authentic by following the RFC's authentication procedure (§3.2.3) | MUST | 3.2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-4.1-1` | A security-aware resolver MUST include an EDNS OPT pseudo-RR with the DO bit set when sending queries (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L60). **negative:** no negative test. **{single-polarity}:** the DO bit is emitted rather than parsed -- SetEdns0(4096, validating) sets it whenever dnssec-validation is permissive or strict (internal/component/resolve/dns/resolver.go:261) -- so there is no non-conformant input to reject; off mode is a plain non-security-aware stub, which this requirement does not govern |
| `RFC4035-4.1-2` | A security-aware resolver MUST support a message size of at least 1220 octets (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4035_LargeUDPResponseAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L116). **negative:** no negative test. **{single-polarity}:** the resolver advertises 4096 octets and miekg sizes the receive buffer from that advertisement (vendor/github.com/miekg/dns/client.go:201), so a response above 1220 octets arrives whole; a message-size floor has no non-conformant input to reject |
| `RFC4035-4.1-4` | A security-aware resolver MUST use the sender's UDP payload size field in the EDNS OPT pseudo-RR to advertise the message size it will accept (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L70). **negative:** no negative test. **{single-polarity}:** the sender's UDP payload size field is emitted, not parsed -- SetEdns0 writes 4096 on every query (internal/component/resolve/dns/resolver.go:261) -- so no malformed input exists to reject |
| `RFC4035-4.1-5` | A security-aware resolver's IP layer MUST handle fragmented UDP packets correctly whether received via IPv4 or IPv6 (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** UDP fragment reassembly belongs to the host IP stack, which ze neither implements nor bypasses -- the resolver exchanges datagrams through the ordinary miekg client socket (internal/component/resolve/dns/resolver.go:263) and grep -rniE 'fragment\|reassembl' --include=*.go internal/component/resolve internal/core/dnsserver finds no producer |
| `RFC4035-4.2-1` | A security-aware resolver MUST support the signature verification mechanisms the RFC describes (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.2-3` | A resolver's signature verification support MUST include verification of wildcard owner names (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.2-5` | When retrieving missing NSEC RRs on the parental side of a zone cut, an iterative-mode resolver MUST query the parent zone name servers, not the child (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.2-6` | When retrieving a missing DS, an iterative-mode resolver MUST query the parent zone name servers, not the child (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.3-1` | A security-aware resolver MUST be able to determine whether it should expect a particular RRset to be signed, distinguishing Secure, Insecure, Bogus, and Indeterminate (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.4-1` | A security-aware resolver MUST be capable of being configured with at least one trusted public key or DS RR (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.6-2` | A security-aware resolver MUST clear the AD bit when composing query messages (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L75). **negative:** no negative test. **{single-polarity}:** the AD bit of an outgoing query is emitted, not parsed -- the query is composed by new(mdns.Msg) plus SetQuestion (internal/component/resolve/dns/resolver.go:255) and no ze code sets AuthenticatedData on a query -- so no negative input exists |
| `RFC4035-4.6-3` | A resolver MUST disregard the meaning of the CD and AD bits in a response unless it was obtained over a secure channel or the resolver was configured to trust them (§4.6) | MUST | 4.6 | **positive:** `unit/verify` [`TestRFC4035_ResponseADBitDisregarded`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L148). **negative:** `unit/verify` [`TestRFC4035_ResponseADBitDisregarded`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L159) |
| `RFC4035-4.7-2` | A resolver that implements a BAD cache MUST take steps to prevent the cache being used as a denial-of-service amplifier (§4.7) | MUST | 4.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| `RFC4035-4.7-3` | Since RRsets that fail to validate lack trustworthy TTLs, the implementation MUST assign a TTL (§4.7) | MUST | 4.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| `RFC4035-4.7-7` | A resolver MUST NOT return RRsets from the BAD cache unless it is not required to validate their signatures (§4.7) | MUST NOT | 4.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| `RFC4035-4.8-1` | A validating resolver MUST treat the signature of a valid signed DNAME RR as also covering unsigned CNAME RRs synthesizable from it, at least by not rejecting the message solely for containing them (§4.8) | MUST | 4.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-4.9-1` | A security-aware stub resolver MUST support the DNSSEC RR types, at least by not mishandling responses that contain them (§4.9) | MUST | 4.9 | **positive:** `unit/verify` [`TestRFC4035_StubHandlesDNSSECRRTypes`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L208). **negative:** `unit/verify` [`TestRFC4035_StubHandlesDNSSECRRTypes`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L219) |
| `RFC4035-4.9.1-2` | A validating security-aware stub resolver MUST set the DO bit (§4.9.1) | MUST | 4.9.1 | **positive:** `unit/verify` [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L62). **negative:** no negative test. **{single-polarity}:** ze's stub sets the DO bit under permissive and strict (internal/component/resolve/dns/resolver.go:261) and leaves the signature check to the upstream; the bit is emitted, not parsed, so there is no non-conformant input to reject |
| `RFC4035-4.9.3-2` | A security-aware stub resolver MUST NOT place any reliance on signature validation performed on its behalf except when it obtained the data from a trusted recursive name server over a secure channel (§4.9.3) | MUST NOT | 4.9.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's stub rests a security decision on validation performed for it over an unauthenticated channel -- strict mode rejects an answer solely because the configured upstream returned SERVFAIL (internal/component/resolve/dns/resolver.go:103-106) while the client speaks plain UDP with no TLS, TSIG, or other authentication of that server (internal/component/resolve/dns/resolver.go:81) |
| `RFC4035-5-1` | To authenticate an apex DNSKEY RRset with an initial key, the resolver MUST verify that the initial DNSKEY RR appears in the apex DNSKEY RRset and has the Zone Key Flag set (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5-2` | To authenticate an apex DNSKEY RRset with an initial key, the resolver MUST verify that some RRSIG RR covers the apex DNSKEY RRset and that it together with the initial DNSKEY authenticates the RRset (§5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5-3` | The absence of DNSSEC data in a response MUST NOT by itself be taken as an indication that no authentication information exists (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.2-1` | A security-aware resolver MUST query the parent zone name servers for the DS RRset if a referral includes neither a DS RRset nor an NSEC RRset proving the DS RRset does not exist (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.2-3` | A security-aware resolver MUST use the parent NSEC RR when attempting to prove that a DS RRset does not exist (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-1` | The RRSIG RR and the RRset MUST have the same owner name and the same class (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-2` | The RRSIG RR's Signer's Name field MUST be the name of the zone that contains the RRset (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-3` | The RRSIG RR's Type Covered field MUST equal the RRset's type (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-4` | The number of labels in the RRset owner name MUST be greater than or equal to the value in the RRSIG RR's Labels field (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-5` | The validator's notion of the current time MUST be less than or equal to the RRSIG RR's Expiration field (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-6` | The validator's notion of the current time MUST be greater than or equal to the RRSIG RR's Inception field (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-7` | The RRSIG RR's Signer's Name, Algorithm, and Key Tag fields MUST match the owner name, algorithm, and key tag of some DNSKEY RR in the zone's apex DNSKEY RRset (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-8` | The matching DNSKEY RR MUST be present in the zone's apex DNSKEY RRset (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-9` | The matching DNSKEY RR MUST have the Zone Flag bit set (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.1-10` | If more than one DNSKEY RR matches, the validator MUST try each until the signature validates or the matching keys are exhausted (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.2-1` | If the RRSIG Labels field is greater than the RRset's label count, the RRSIG did not pass validation and MUST NOT be used to authenticate the RRset (§5.3.2) | MUST NOT | 5.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.2-2` | When reconstructing the parent-zone NSEC RRset at a delegation, its NSEC RRs MUST NOT be combined with NSEC RRs from the child zone (§5.3.2) | MUST NOT | 5.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.2-3` | When reconstructing the child-apex NSEC RRset, its NSEC RRs MUST NOT be combined with NSEC RRs from the parent zone (§5.3.2) | MUST NOT | 5.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.3-1` | When the RRSIG Labels field does not equal the owner name's label count, the resolver MUST verify that wildcard expansion was applied properly before considering the RRset authentic (§5.3.3) | MUST | 5.3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.3.3-2` | On accepting an RRset as authentic, the validator MUST set the RRSIG RR and each RR's TTL to no greater than the minimum of the RRset TTL, the RRSIG TTL, the RRSIG Original TTL, and the time until the RRSIG expires (§5.3.3) | MUST | 5.3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.4-1` | A security-aware resolver MUST authenticate the NSEC RRsets that comprise a denial-of-existence proof (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.4-2` | If the complete set of necessary NSEC RRsets is not present in a response, the resolver MUST resend the query to obtain them (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.4-3` | The resolver MUST bound the work it puts into answering any particular query (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.4-4` | A validator MUST ignore the settings of the NSEC and RRSIG bits in an NSEC RR (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| `RFC4035-5.5-2` | When validation was done to service a recursive query, the name server MUST return RCODE 2 to the originating client (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-5.5-3` | The name server MUST return the full response if and only if the original query had the CD bit set (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| `RFC4035-2.1-1` | For each private key used to create RRSIGs in a zone, the zone SHOULD include a zone DNSKEY RR containing the corresponding public key (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.3-2` | The TTL value for any NSEC RR SHOULD be the same as the minimum TTL value field in the zone SOA RR (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.4-1` | A DS RRset SHOULD be present at a delegation point when the child zone is signed (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.4-5` | A DS RR SHOULD point to a DNSKEY RR that is present in the child's apex DNSKEY RRset (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.4-6` | The child's apex DNSKEY RRset SHOULD be signed by the private key corresponding to the DS-referenced DNSKEY (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.4-7` | The TTL of a DS RRset SHOULD match the TTL of the delegating NS RRset (§2.4) | SHOULD | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3-3` | A security-aware name server SHOULD support a message size of 4000 octets (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3-4` | A security-aware name server SHOULD ensure UDP datagrams it transmits over IPv6 are fragmented at the minimum IPv6 MTU when necessary, unless the path MTU is known (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3-10` | A name server that synthesizes CNAME RRs from DNAME RRs SHOULD NOT generate signatures for the synthesized CNAME RRs (§3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.1-1` | When responding to a DO-set query, the name server SHOULD attempt to send RRSIG RRs a resolver can use to authenticate the response (§3.1.1) | SHOULD | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.1-2` | The name server SHOULD make every attempt to keep an RRset and its associated RRSIGs together in a response (§3.1.1) | SHOULD | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.2-2` | The name server SHOULD NOT include the apex DNSKEY RRset unless there is enough space for both it and its associated RRSIGs (§3.1.2) | SHOULD NOT | 3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.3.2-2` | If a single NSEC RR proves both required points, the name server SHOULD include that NSEC RR and its RRSIGs only once in the Authority section (§3.1.3.2) | SHOULD | 3.1.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.3.4-2` | If a single NSEC RR proves both required points, the name server SHOULD include that NSEC RR and its RRSIGs only once in the Authority section (§3.1.3.4) | SHOULD | 3.1.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.6-1` | A security-aware name server SHOULD clear the CD bit when composing an authoritative response (§3.1.6) | SHOULD | 3.1.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.2.2-2` | When the CD bit is set, the recursive name server SHOULD, if possible, return the requested data even if its local policy would reject the records (§3.2.2) | SHOULD | 3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.2.2-3` | If the CD bit is set and the query matches a BAD cache entry, the name server side SHOULD return the data from the BAD cache (§3.2.2) | SHOULD | 3.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.2.3-1` | The name server side SHOULD set the AD bit if and only if the resolver side considers all Answer RRsets and any relevant negative Authority RRs authentic (§3.2.3) | SHOULD | 3.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.1-3` | A security-aware resolver SHOULD support a message size of 4000 octets (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.2-2` | A security-aware resolver SHOULD apply the signature verification mechanisms to every received response, except the enumerated cases (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.4-2` | A security-aware resolver SHOULD be capable of being configured with multiple trusted public keys or DS RRs (§4.4) | SHOULD | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.4-3` | A security-aware resolver SHOULD have a reasonably robust mechanism for obtaining trust anchor keys when it boots (§4.4) | SHOULD | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.5-1` | A security-aware resolver SHOULD cache each response as a single atomic entry containing the entire answer and its associated DNSSEC RRs (§4.5) | SHOULD | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.5-2` | The resolver SHOULD discard the entire atomic cache entry when any RR contained in it expires (§4.5) | SHOULD | 4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.7-4` | The TTL assigned to a failed-validation RRset SHOULD be small, to limit the effect of caching an attack's results (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.7-5` | Resolvers SHOULD track queries that result in validation failures (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.7-6` | Resolvers SHOULD answer from the BAD cache only after the failure count for the query exceeds a threshold value (§4.7) | SHOULD | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.9.2-1` | A non-validating security-aware stub resolver SHOULD NOT set the CD bit unless requested by the application layer (§4.9.2) | SHOULD NOT | 4.9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.9.2-2` | A validating security-aware stub resolver SHOULD set the CD bit (§4.9.2) | SHOULD | 4.9.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.9.3-3` | A validating security-aware stub resolver SHOULD NOT examine the setting of the AD bit (§4.9.3) | SHOULD NOT | 4.9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5-4` | A resolver SHOULD expect authentication information from signed zones (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5-5` | A resolver SHOULD believe a zone is signed if it is configured with the zone's public key, or the parent is signed and the delegation contains a DS RRset (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5.1-1` | If a validator cannot obtain an initial authenticated key for an island of security, it SHOULD operate as if the island's zones are unsigned (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5.2-4` | If the resolver supports none of the algorithms in an authenticated DS RRset, it SHOULD treat the child zone as if it were unsigned (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5.5-1` | If none of the RRSIGs can be validated, the response SHOULD be considered BAD (§5.5) | SHOULD | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.1-3` | Public keys associated with other DNS operations MAY be stored in DNSKEY RRs that are not marked as zone keys (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.2-2` | An RRset MAY have multiple RRSIG RRs associated with it (§2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-2.4-2` | A DS RRset MAY contain multiple records, each referencing a public key in the child zone (§2.4) | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3-7` | For explicit security-RR-type queries matching more than one served zone, as long as responses stay self-consistent, the name server MAY return one of the enumerated response forms (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.1-7` | When both a signed RRset and its RRSIGs will not fit in the Additional section, the name server MAY retain the RRset and drop the RRSIG RRs (§3.1.1) | MAY | 3.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.2-1` | When a DO-set query requests the SOA or NS RRs at a signed zone's apex, the name server MAY return the apex DNSKEY RRset in the Additional section (§3.1.2) | MAY | 3.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.1.5-1` | An authoritative name server MAY reject an entire zone transfer if the zone fails to meet the signing requirements (§3.1.5) | MAY | 3.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-3.2.3-3` | For backward compatibility, a recursive name server MAY set the AD bit when a response includes unsigned CNAME RRs demonstrably synthesizable from an authentic DNAME RR also in the response (§3.2.3) | MAY | 3.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.2-4` | Security-aware resolvers MAY query for missing security RRs in an attempt to perform validation (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.6-1` | A security-aware resolver MAY set a query's CD bit to indicate that it takes responsibility for authentication (§4.6) | MAY | 4.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.7-1` | Security-aware resolvers MAY cache data with invalid signatures, subject to restrictions (§4.7) | MAY | 4.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.8-2` | A resolver MAY retain synthesized CNAME RRs in its cache or in the answers it hands back (§4.8) | MAY | 4.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.9.1-1` | A non-validating security-aware stub resolver MAY include the DNSSEC RRs returned by a recursive name server in the data it hands back to the application (§4.9.1) | MAY | 4.9.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-4.9.3-1` | A non-validating security-aware stub resolver MAY examine the AD bit to see whether the recursive name server claims to have verified the data (§4.9.3) | MAY | 4.9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4035-5.2-2` | If an authenticated NSEC proves that no DS exists, an initial DNSKEY or DS RR for the child zone or a delegation below it MAY be used to re-establish an authentication path (§5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4035-2.1-2`](#rfc4035-2.1-2) A zone key DNSKEY RR MUST have the Zone Key bit of the flags RDATA field set (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.1-4`](#rfc4035-2.1-4) Public keys stored in DNSKEY RRs that are not marked as zone keys MUST NOT be used to verify RRSIGs (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.1-5`](#rfc4035-2.1-5) For a signed zone usable other than as an island of security, the zone apex MUST contain at least one DNSKEY RR to act as a secure entry point (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-1`](#rfc4035-2.2-1) For each authoritative RRset in a signed zone there MUST be at least one RRSIG record whose owner, class, Type Covered, Original TTL, TTL, Labels, and Signer's Name match the RRset and identify an apex zone key (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-3`](#rfc4035-2.2-3) An RRSIG RR itself MUST NOT be signed (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-4`](#rfc4035-2.2-4) The NS RRset that appears at the zone apex name MUST be signed (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-5`](#rfc4035-2.2-5) The NS RRsets that appear at delegation points MUST NOT be signed (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-6`](#rfc4035-2.2-6) Glue address RRsets associated with delegations MUST NOT be signed (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-7`](#rfc4035-2.2-7) There MUST be an RRSIG for each RRset using at least one DNSKEY of each algorithm in the zone apex DNSKEY RRset (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.2-8`](#rfc4035-2.2-8) The apex DNSKEY RRset MUST be signed by each algorithm appearing in the DS RRset at the delegating parent, if any (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-1`](#rfc4035-2.3-1) Each owner name in the zone that has authoritative data or a delegation point NS RRset MUST have an NSEC resource record (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-3`](#rfc4035-2.3-3) An NSEC record and its associated RRSIG RRset MUST NOT be the only RRset at any particular owner name (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-4`](#rfc4035-2.3-4) The signing process MUST NOT create NSEC or RRSIG RRs for owner name nodes that were not the owner name of any RRset before the zone was signed (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-5`](#rfc4035-2.3-5) The type bitmap of every NSEC RR MUST indicate the presence of both the NSEC record itself and its corresponding RRSIG record (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-6`](#rfc4035-2.3-6) In the NSEC bitmap at a delegation point, bits for the delegation NS RRset and any RRsets for which the parent zone has authoritative data MUST be set (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.3-7`](#rfc4035-2.3-7) In the NSEC bitmap at a delegation point, bits for any non-NS RRset for which the parent is not authoritative MUST be clear (§2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.4-3`](#rfc4035-2.4-3) All DS RRsets in a zone MUST be signed (§2.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.4-4`](#rfc4035-2.4-4) DS RRsets MUST NOT appear at a zone's apex (§2.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.5-1`](#rfc4035-2.5-1) If a CNAME RRset is present at a name in a signed zone, appropriate RRSIG and NSEC RRsets are REQUIRED at that name (§2.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.5-2`](#rfc4035-2.5-2) Types other than CNAME, its RRSIG and NSEC, and a KEY RRset for secure dynamic update MUST NOT be present at a CNAME name (§2.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-2.6-1`](#rfc4035-2.6-1) At the parental side of a zone cut, NSEC RRs are REQUIRED at the owner name (§2.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze creates no DNSKEY, RRSIG, NSEC, or DS record for this rule to constrain -- ze signs no zone: grep -rnE 'TypeRRSIG\|TypeDNSKEY\|TypeNSEC\|TypeDS' --include=*.go internal/ pkg/ cmd/ matches only the RFC 4035 conformance tests, and answerQuestions synthesizes A/AAAA/SRV/SOA/NS records only (internal/plugins/geodns/server.go:168) |
| [`RFC4035-3-1`](#rfc4035-3-1) A security-aware name server MUST support the EDNS0 message size extension (§3) | {gap}, no test | ze's authoritative servers give the EDNS0 message size extension no support -- the only SetEdns0 producer in ze is the stub resolver (internal/component/resolve/dns/resolver.go:261); the server path reads an OPT only for the client-subnet address (internal/core/dnsserver/client.go:23) and writes a reply built by msg.SetReply with no OPT pseudo-RR and no requestor-payload-size handling (internal/core/dnsserver/handler.go:55) |
| [`RFC4035-3-2`](#rfc4035-3-2) A security-aware name server MUST support a message size of at least 1220 octets (§3) | {gap}, no test | the UDP listener accepts at most 512 octets -- dns.Server is constructed with UDPSize left zero (internal/core/dnsserver/manager.go:165), which miekg defaults to MinMsgSize 512 (vendor/github.com/miekg/dns/server.go:287), so a query above 512 octets is never read whole and no reply advertises a larger size |
| [`RFC4035-3-5`](#rfc4035-3-5) A name server receiving a query without the EDNS OPT pseudo-RR or with the DO bit clear MUST treat the RRSIG, DNSKEY, and NSEC RRs as it would any other RRset (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1-1`](#rfc4035-3.1-1) Upon a DO-set query to a signed zone, RRSIG RRs that can be used to authenticate the response MUST be included (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1-2`](#rfc4035-3.1-2) NSEC RRs that provide authenticated denial of existence MUST be included in the response automatically (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1-3`](#rfc4035-3.1-3) Either a DS RRset or an NSEC RR proving that no DS RRs exist MUST be included in referrals automatically (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.1-3`](#rfc4035-3.1.1-3) When placing a signed RRset in the Answer section, the name server MUST also place its RRSIG RRs in the Answer section (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.1-4`](#rfc4035-3.1.1-4) If space does not permit inclusion of the RRSIG RRs that must accompany a signed RRset, or of a mandatory NSEC or DS RRset and its RRSIGs, the name server MUST set the TC bit (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.1-5`](#rfc4035-3.1.1-5) When placing a signed RRset in the Authority section, the name server MUST also place its RRSIG RRs in the Authority section (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.1-6`](#rfc4035-3.1.1-6) When placing a signed RRset in the Additional section, the name server MUST also place its RRSIG RRs in the Additional section (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.1-8`](#rfc4035-3.1.1-8) The name server MUST NOT set the TC bit solely because RRSIG RRs did not fit in the Additional section (§3.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.2-3`](#rfc4035-3.1.2-3) If there is not enough space for the apex DNSKEY RRset and its RRSIGs, the name server MUST omit them (§3.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.2-4`](#rfc4035-3.1.2-4) The name server MUST NOT set the TC bit solely because the apex DNSKEY and RRSIG RRs did not fit (§3.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3-1`](#rfc4035-3.1.3-1) When responding to a DO-set query, the name server MUST include NSEC RRs in the No Data, Name Error, Wildcard Answer, and Wildcard No Data cases (§3.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3.1-1`](#rfc4035-3.1.3.1-1) For a No Data response, the name server MUST include the NSEC RR for the queried name and its RRSIGs in the Authority section (§3.1.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3.2-1`](#rfc4035-3.1.3.2-1) For a Name Error response, the name server MUST include, with their RRSIGs, an NSEC RR proving no exact match and an NSEC RR proving no wildcard match (§3.1.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3.3-1`](#rfc4035-3.1.3.3-1) For a Wildcard Answer response, the name server MUST include the wildcard-expanded answer and its wildcard-expanded RRSIG RRs in the Answer section (§3.1.3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3.3-2`](#rfc4035-3.1.3.3-2) For a Wildcard Answer response, the name server MUST include in the Authority section an NSEC RR and its RRSIGs proving that no closer match exists (§3.1.3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.3.4-1`](#rfc4035-3.1.3.4-1) For a Wildcard No Data response, the name server MUST include, with their RRSIGs, an NSEC RR proving no matching type at the wildcard owner name and an NSEC RR proving no closer match (§3.1.3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.4-1`](#rfc4035-3.1.4-1) If a DS RRset is present at the delegation point, the name server MUST return the DS RRset and its RRSIGs in the Authority section with the NS RRset (§3.1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.4-2`](#rfc4035-3.1.4-2) If no DS RRset is present, the name server MUST return the NSEC RR proving the DS RRset is absent and its RRSIGs with the NS RRset (§3.1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.4-3`](#rfc4035-3.1.4-3) The name server MUST place the NS RRset before the NSEC RRset and its RRSIGs (§3.1.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers hold no RRSIG, NSEC, DS, or DNSKEY record to place in any section: answerQuestions builds A/AAAA/SRV/SOA/NS answers from local host-sets (internal/plugins/geodns/server.go:168) and as112 answers only negatively (internal/plugins/as112/server.go:86) |
| [`RFC4035-3.1.5-2`](#rfc4035-3.1.5-2) A name server performing its own zone validation MUST NOT selectively reject some RRs and accept others (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.5-3`](#rfc4035-3.1.5-3) The DS RRset MUST be included in zone transfers of the parent zone in which it is authoritative data (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.5-4`](#rfc4035-3.1.5-4) NSEC RRs MUST be included in zone transfers of the zone in which they are authoritative data (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.5-5`](#rfc4035-3.1.5-5) The parental NSEC RR at a zone cut MUST be included in zone transfers of the parent zone (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.5-6`](#rfc4035-3.1.5-6) The NSEC at the zone apex of the child zone MUST be included in zone transfers of the child zone (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.5-7`](#rfc4035-3.1.5-7) RRSIG RRs MUST be included in zone transfers of the zone in which they are authoritative data (§3.1.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no zone transfer: grep -rniE 'axfr\|ixfr' --include=*.go internal/ finds no producer, and no ze DNS server holds DNSSEC records to transfer |
| [`RFC4035-3.1.6-6`](#rfc4035-3.1.6-6) A security-aware name server that supports recursion MUST follow the recursive-server CD and AD bit rules for data obtained via recursion (§3.1.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.1-1`](#rfc4035-3.2.1-1) The resolver side of a security-aware recursive name server MUST set the DO bit when sending requests (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.1-2`](#rfc4035-3.2.1-2) If the DO bit in an initiating query is not set, the name server side MUST strip any authenticating DNSSEC RRs from the response (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.1-3`](#rfc4035-3.2.1-3) The name server side MUST NOT strip any DNSSEC RR types that the initiating query explicitly requested (§3.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.2-1`](#rfc4035-3.2.2-1) The name server side MUST pass the state of the CD bit to the resolver side along with the initiating query (§3.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.2-4`](#rfc4035-3.2.2-4) If the CD bit is not set and the query matches a BAD cache entry, the name server side MUST return RCODE 2 (server failure) (§3.2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-3.2.3-2`](#rfc4035-3.2.3-2) The resolver side MUST determine whether the RRs are authentic by following the RFC's authentication procedure (§3.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-4.1-5`](#rfc4035-4.1-5) A security-aware resolver's IP layer MUST handle fragmented UDP packets correctly whether received via IPv4 or IPv6 (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: UDP fragment reassembly belongs to the host IP stack, which ze neither implements nor bypasses -- the resolver exchanges datagrams through the ordinary miekg client socket (internal/component/resolve/dns/resolver.go:263) and grep -rniE 'fragment\|reassembl' --include=*.go internal/component/resolve internal/core/dnsserver finds no producer |
| [`RFC4035-4.2-1`](#rfc4035-4.2-1) A security-aware resolver MUST support the signature verification mechanisms the RFC describes (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.2-3`](#rfc4035-4.2-3) A resolver's signature verification support MUST include verification of wildcard owner names (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.2-5`](#rfc4035-4.2-5) When retrieving missing NSEC RRs on the parental side of a zone cut, an iterative-mode resolver MUST query the parent zone name servers, not the child (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.2-6`](#rfc4035-4.2-6) When retrieving a missing DS, an iterative-mode resolver MUST query the parent zone name servers, not the child (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.3-1`](#rfc4035-4.3-1) A security-aware resolver MUST be able to determine whether it should expect a particular RRset to be signed, distinguishing Secure, Insecure, Bogus, and Indeterminate (§4.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.4-1`](#rfc4035-4.4-1) A security-aware resolver MUST be capable of being configured with at least one trusted public key or DS RR (§4.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.7-2`](#rfc4035-4.7-2) A resolver that implements a BAD cache MUST take steps to prevent the cache being used as a denial-of-service amplifier (§4.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| [`RFC4035-4.7-3`](#rfc4035-4.7-3) Since RRsets that fail to validate lack trustworthy TTLs, the implementation MUST assign a TTL (§4.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| [`RFC4035-4.7-7`](#rfc4035-4.7-7) A resolver MUST NOT return RRsets from the BAD cache unless it is not required to validate their signatures (§4.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no BAD cache: Resolve caches only a non-empty successful answer (internal/component/resolve/dns/resolver.go:192) and a SERVFAIL yields an uncached empty result (internal/component/resolve/dns/resolver.go:285) |
| [`RFC4035-4.8-1`](#rfc4035-4.8-1) A validating resolver MUST treat the signature of a valid signed DNAME RR as also covering unsigned CNAME RRs synthesizable from it, at least by not rejecting the message solely for containing them (§4.8) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-4.9.3-2`](#rfc4035-4.9.3-2) A security-aware stub resolver MUST NOT place any reliance on signature validation performed on its behalf except when it obtained the data from a trusted recursive name server over a secure channel (§4.9.3) | {gap}, no test | ze's stub rests a security decision on validation performed for it over an unauthenticated channel -- strict mode rejects an answer solely because the configured upstream returned SERVFAIL (internal/component/resolve/dns/resolver.go:103-106) while the client speaks plain UDP with no TLS, TSIG, or other authentication of that server (internal/component/resolve/dns/resolver.go:81) |
| [`RFC4035-5-1`](#rfc4035-5-1) To authenticate an apex DNSKEY RRset with an initial key, the resolver MUST verify that the initial DNSKEY RR appears in the apex DNSKEY RRset and has the Zone Key Flag set (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5-2`](#rfc4035-5-2) To authenticate an apex DNSKEY RRset with an initial key, the resolver MUST verify that some RRSIG RR covers the apex DNSKEY RRset and that it together with the initial DNSKEY authenticates the RRset (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5-3`](#rfc4035-5-3) The absence of DNSSEC data in a response MUST NOT by itself be taken as an indication that no authentication information exists (§5) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.2-1`](#rfc4035-5.2-1) A security-aware resolver MUST query the parent zone name servers for the DS RRset if a referral includes neither a DS RRset nor an NSEC RRset proving the DS RRset does not exist (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.2-3`](#rfc4035-5.2-3) A security-aware resolver MUST use the parent NSEC RR when attempting to prove that a DS RRset does not exist (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-1`](#rfc4035-5.3.1-1) The RRSIG RR and the RRset MUST have the same owner name and the same class (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-2`](#rfc4035-5.3.1-2) The RRSIG RR's Signer's Name field MUST be the name of the zone that contains the RRset (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-3`](#rfc4035-5.3.1-3) The RRSIG RR's Type Covered field MUST equal the RRset's type (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-4`](#rfc4035-5.3.1-4) The number of labels in the RRset owner name MUST be greater than or equal to the value in the RRSIG RR's Labels field (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-5`](#rfc4035-5.3.1-5) The validator's notion of the current time MUST be less than or equal to the RRSIG RR's Expiration field (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-6`](#rfc4035-5.3.1-6) The validator's notion of the current time MUST be greater than or equal to the RRSIG RR's Inception field (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-7`](#rfc4035-5.3.1-7) The RRSIG RR's Signer's Name, Algorithm, and Key Tag fields MUST match the owner name, algorithm, and key tag of some DNSKEY RR in the zone's apex DNSKEY RRset (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-8`](#rfc4035-5.3.1-8) The matching DNSKEY RR MUST be present in the zone's apex DNSKEY RRset (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-9`](#rfc4035-5.3.1-9) The matching DNSKEY RR MUST have the Zone Flag bit set (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.1-10`](#rfc4035-5.3.1-10) If more than one DNSKEY RR matches, the validator MUST try each until the signature validates or the matching keys are exhausted (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.2-1`](#rfc4035-5.3.2-1) If the RRSIG Labels field is greater than the RRset's label count, the RRSIG did not pass validation and MUST NOT be used to authenticate the RRset (§5.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.2-2`](#rfc4035-5.3.2-2) When reconstructing the parent-zone NSEC RRset at a delegation, its NSEC RRs MUST NOT be combined with NSEC RRs from the child zone (§5.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.2-3`](#rfc4035-5.3.2-3) When reconstructing the child-apex NSEC RRset, its NSEC RRs MUST NOT be combined with NSEC RRs from the parent zone (§5.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.3-1`](#rfc4035-5.3.3-1) When the RRSIG Labels field does not equal the owner name's label count, the resolver MUST verify that wildcard expansion was applied properly before considering the RRset authentic (§5.3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.3.3-2`](#rfc4035-5.3.3-2) On accepting an RRset as authentic, the validator MUST set the RRSIG RR and each RR's TTL to no greater than the minimum of the RRset TTL, the RRSIG TTL, the RRSIG Original TTL, and the time until the RRSIG expires (§5.3.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.4-1`](#rfc4035-5.4-1) A security-aware resolver MUST authenticate the NSEC RRsets that comprise a denial-of-existence proof (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.4-2`](#rfc4035-5.4-2) If the complete set of necessary NSEC RRsets is not present in a response, the resolver MUST resend the query to obtain them (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.4-3`](#rfc4035-5.4-3) The resolver MUST bound the work it puts into answering any particular query (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.4-4`](#rfc4035-5.4-4) A validator MUST ignore the settings of the NSEC and RRSIG bits in an NSEC RR (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no local validator: the resolver's whole DNSSEC surface is dnssecDecision (internal/component/resolve/dns/resolver.go:99), which inspects the rcode alone -- no signature verification, trust-anchor store, DS chain, or NSEC proof exists in ze |
| [`RFC4035-5.5-2`](#rfc4035-5.5-2) When validation was done to service a recursive query, the name server MUST return RCODE 2 to the originating client (§5.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |
| [`RFC4035-5.5-3`](#rfc4035-5.5-3) The name server MUST return the full response if and only if the original query had the CD bit set (§5.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DNS servers never recurse and have no resolver side: shapeAuthoritative clears RecursionAvailable on every reply (internal/core/dnsserver/handler.go:74) and each AnswerFunc answers from local state alone, forwarding nothing upstream (internal/plugins/geodns/server.go:221, internal/plugins/as112/server.go:86) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4035-2.1-2`](#rfc4035-2.1-2)

A zone key DNSKEY RR MUST have the Zone Key bit of the flags RDATA field set (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.1-2, so no unit is bound to it.

### [`RFC4035-2.1-4`](#rfc4035-2.1-4)

Public keys stored in DNSKEY RRs that are not marked as zone keys MUST NOT be used to verify RRSIGs (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.1-4, so no unit is bound to it.

### [`RFC4035-2.1-5`](#rfc4035-2.1-5)

For a signed zone usable other than as an island of security, the zone apex MUST contain at least one DNSKEY RR to act as a secure entry point (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.1-5, so no unit is bound to it.

### [`RFC4035-2.2-1`](#rfc4035-2.2-1)

For each authoritative RRset in a signed zone there MUST be at least one RRSIG record whose owner, class, Type Covered, Original TTL, TTL, Labels, and Signer's Name match the RRset and identify an apex zone key (§2.2)

Audit verdict: not audited: no reader has judged these tests

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

The NS RRsets that appear at delegation points MUST NOT be signed (§2.2)

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

The apex DNSKEY RRset MUST be signed by each algorithm appearing in the DS RRset at the delegating parent, if any (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.2-8, so no unit is bound to it.

### [`RFC4035-2.3-1`](#rfc4035-2.3-1)

Each owner name in the zone that has authoritative data or a delegation point NS RRset MUST have an NSEC resource record (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.3-1, so no unit is bound to it.

### [`RFC4035-2.3-3`](#rfc4035-2.3-3)

An NSEC record and its associated RRSIG RRset MUST NOT be the only RRset at any particular owner name (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.3-3, so no unit is bound to it.

### [`RFC4035-2.3-4`](#rfc4035-2.3-4)

The signing process MUST NOT create NSEC or RRSIG RRs for owner name nodes that were not the owner name of any RRset before the zone was signed (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.3-4, so no unit is bound to it.

### [`RFC4035-2.3-5`](#rfc4035-2.3-5)

The type bitmap of every NSEC RR MUST indicate the presence of both the NSEC record itself and its corresponding RRSIG record (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.3-5, so no unit is bound to it.

### [`RFC4035-2.3-6`](#rfc4035-2.3-6)

In the NSEC bitmap at a delegation point, bits for the delegation NS RRset and any RRsets for which the parent zone has authoritative data MUST be set (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.3-6, so no unit is bound to it.

### [`RFC4035-2.3-7`](#rfc4035-2.3-7)

In the NSEC bitmap at a delegation point, bits for any non-NS RRset for which the parent is not authoritative MUST be clear (§2.3)

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

Types other than CNAME, its RRSIG and NSEC, and a KEY RRset for secure dynamic update MUST NOT be present at a CNAME name (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.5-2, so no unit is bound to it.

### [`RFC4035-2.6-1`](#rfc4035-2.6-1)

At the parental side of a zone cut, NSEC RRs are REQUIRED at the owner name (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-2.6-1, so no unit is bound to it.

### [`RFC4035-3-1`](#rfc4035-3-1)

A security-aware name server MUST support the EDNS0 message size extension (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3-1, so no unit is bound to it.

### [`RFC4035-3-2`](#rfc4035-3-2)

A security-aware name server MUST support a message size of at least 1220 octets (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3-2, so no unit is bound to it.

### [`RFC4035-3-5`](#rfc4035-3-5)

A name server receiving a query without the EDNS OPT pseudo-RR or with the DO bit clear MUST treat the RRSIG, DNSKEY, and NSEC RRs as it would any other RRset (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3-5, so no unit is bound to it.

### [`RFC4035-3-6`](#rfc4035-3-6)

Such a name server MUST NOT perform any of the DNSSEC additional processing (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L164) | unit/verify | unproven |

### [`RFC4035-3-8`](#rfc4035-3-8)

A security-aware name server MUST copy the CD bit from a query into the corresponding response (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L56) | unit/verify | unproven |
| positive | [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L51) | unit/verify | unproven |

### [`RFC4035-3-9`](#rfc4035-3-9)

A security-aware name server MUST ignore the setting of the AD bit in queries (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L69) | unit/verify | unproven |
| positive | [`TestRFC4035_CDCopiedADIgnored`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc4035_test.go#L62) | unit/verify | unproven |

### [`RFC4035-3.1-1`](#rfc4035-3.1-1)

Upon a DO-set query to a signed zone, RRSIG RRs that can be used to authenticate the response MUST be included (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1-1, so no unit is bound to it.

### [`RFC4035-3.1-2`](#rfc4035-3.1-2)

NSEC RRs that provide authenticated denial of existence MUST be included in the response automatically (§3.1)

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

If space does not permit inclusion of the RRSIG RRs that must accompany a signed RRset, or of a mandatory NSEC or DS RRset and its RRSIGs, the name server MUST set the TC bit (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

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

The name server MUST NOT set the TC bit solely because RRSIG RRs did not fit in the Additional section (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.1-8, so no unit is bound to it.

### [`RFC4035-3.1.2-3`](#rfc4035-3.1.2-3)

If there is not enough space for the apex DNSKEY RRset and its RRSIGs, the name server MUST omit them (§3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.2-3, so no unit is bound to it.

### [`RFC4035-3.1.2-4`](#rfc4035-3.1.2-4)

The name server MUST NOT set the TC bit solely because the apex DNSKEY and RRSIG RRs did not fit (§3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.2-4, so no unit is bound to it.

### [`RFC4035-3.1.3-1`](#rfc4035-3.1.3-1)

When responding to a DO-set query, the name server MUST include NSEC RRs in the No Data, Name Error, Wildcard Answer, and Wildcard No Data cases (§3.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3-1, so no unit is bound to it.

### [`RFC4035-3.1.3.1-1`](#rfc4035-3.1.3.1-1)

For a No Data response, the name server MUST include the NSEC RR for the queried name and its RRSIGs in the Authority section (§3.1.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3.1-1, so no unit is bound to it.

### [`RFC4035-3.1.3.2-1`](#rfc4035-3.1.3.2-1)

For a Name Error response, the name server MUST include, with their RRSIGs, an NSEC RR proving no exact match and an NSEC RR proving no wildcard match (§3.1.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3.2-1, so no unit is bound to it.

### [`RFC4035-3.1.3.3-1`](#rfc4035-3.1.3.3-1)

For a Wildcard Answer response, the name server MUST include the wildcard-expanded answer and its wildcard-expanded RRSIG RRs in the Answer section (§3.1.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3.3-1, so no unit is bound to it.

### [`RFC4035-3.1.3.3-2`](#rfc4035-3.1.3.3-2)

For a Wildcard Answer response, the name server MUST include in the Authority section an NSEC RR and its RRSIGs proving that no closer match exists (§3.1.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3.3-2, so no unit is bound to it.

### [`RFC4035-3.1.3.4-1`](#rfc4035-3.1.3.4-1)

For a Wildcard No Data response, the name server MUST include, with their RRSIGs, an NSEC RR proving no matching type at the wildcard owner name and an NSEC RR proving no closer match (§3.1.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.3.4-1, so no unit is bound to it.

### [`RFC4035-3.1.4-1`](#rfc4035-3.1.4-1)

If a DS RRset is present at the delegation point, the name server MUST return the DS RRset and its RRSIGs in the Authority section with the NS RRset (§3.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.4-1, so no unit is bound to it.

### [`RFC4035-3.1.4-2`](#rfc4035-3.1.4-2)

If no DS RRset is present, the name server MUST return the NSEC RR proving the DS RRset is absent and its RRSIGs with the NS RRset (§3.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.4-2, so no unit is bound to it.

### [`RFC4035-3.1.4-3`](#rfc4035-3.1.4-3)

The name server MUST place the NS RRset before the NSEC RRset and its RRSIGs (§3.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.4-3, so no unit is bound to it.

### [`RFC4035-3.1.4.1-1`](#rfc4035-3.1.4.1-1)

When authoritative for the child zone but not the parent and not offering recursion, on a DS query at the zone cut the name server MUST return an authoritative no-data response (§3.1.4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4035_DSQueryIsAuthoritativeNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L112) | unit/verify | unproven |
| positive | [`TestRFC4035_DSQueryIsAuthoritativeNoData`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L85) | unit/verify | unproven |

### [`RFC4035-3.1.5-2`](#rfc4035-3.1.5-2)

A name server performing its own zone validation MUST NOT selectively reject some RRs and accept others (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.5-2, so no unit is bound to it.

### [`RFC4035-3.1.5-3`](#rfc4035-3.1.5-3)

The DS RRset MUST be included in zone transfers of the parent zone in which it is authoritative data (§3.1.5)

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

The NSEC at the zone apex of the child zone MUST be included in zone transfers of the child zone (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.5-6, so no unit is bound to it.

### [`RFC4035-3.1.5-7`](#rfc4035-3.1.5-7)

RRSIG RRs MUST be included in zone transfers of the zone in which they are authoritative data (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.1.5-7, so no unit is bound to it.

### [`RFC4035-3.1.6-2`](#rfc4035-3.1.6-2)

A security-aware name server MUST NOT set the AD bit in a response unless it considers all RRsets in the Answer and Authority sections to be authentic (§3.1.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L177) | unit/verify | unproven |

### [`RFC4035-3.1.6-4`](#rfc4035-3.1.6-4)

The name server MUST NOT treat authoritative-zone data as authentic unless it obtained the zone via secure means (§3.1.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L179) | unit/verify | unproven |

### [`RFC4035-3.1.6-5`](#rfc4035-3.1.6-5)

The name server MUST NOT treat authoritative-zone data as authentic unless this behavior has been configured explicitly (§3.1.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_NoDNSSECAdditionalProcessing`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc4035_server_test.go#L182) | unit/verify | unproven |

### [`RFC4035-3.1.6-6`](#rfc4035-3.1.6-6)

A security-aware name server that supports recursion MUST follow the recursive-server CD and AD bit rules for data obtained via recursion (§3.1.6)

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

The name server side MUST NOT strip any DNSSEC RR types that the initiating query explicitly requested (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.2.1-3, so no unit is bound to it.

### [`RFC4035-3.2.2-1`](#rfc4035-3.2.2-1)

The name server side MUST pass the state of the CD bit to the resolver side along with the initiating query (§3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.2.2-1, so no unit is bound to it.

### [`RFC4035-3.2.2-4`](#rfc4035-3.2.2-4)

If the CD bit is not set and the query matches a BAD cache entry, the name server side MUST return RCODE 2 (server failure) (§3.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.2.2-4, so no unit is bound to it.

### [`RFC4035-3.2.3-2`](#rfc4035-3.2.3-2)

The resolver side MUST determine whether the RRs are authentic by following the RFC's authentication procedure (§3.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-3.2.3-2, so no unit is bound to it.

### [`RFC4035-4.1-1`](#rfc4035-4.1-1)

A security-aware resolver MUST include an EDNS OPT pseudo-RR with the DO bit set when sending queries (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L60) | unit/verify | unproven |

### [`RFC4035-4.1-2`](#rfc4035-4.1-2)

A security-aware resolver MUST support a message size of at least 1220 octets (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_LargeUDPResponseAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L116) | unit/verify | unproven |

### [`RFC4035-4.1-4`](#rfc4035-4.1-4)

A security-aware resolver MUST use the sender's UDP payload size field in the EDNS OPT pseudo-RR to advertise the message size it will accept (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC4035_QueryCarriesEDNS0DOAndClearAD`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L70) | unit/verify | unproven |

### [`RFC4035-4.1-5`](#rfc4035-4.1-5)

A security-aware resolver's IP layer MUST handle fragmented UDP packets correctly whether received via IPv4 or IPv6 (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.1-5, so no unit is bound to it.

### [`RFC4035-4.2-1`](#rfc4035-4.2-1)

A security-aware resolver MUST support the signature verification mechanisms the RFC describes (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.2-1, so no unit is bound to it.

### [`RFC4035-4.2-3`](#rfc4035-4.2-3)

A resolver's signature verification support MUST include verification of wildcard owner names (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.2-3, so no unit is bound to it.

### [`RFC4035-4.2-5`](#rfc4035-4.2-5)

When retrieving missing NSEC RRs on the parental side of a zone cut, an iterative-mode resolver MUST query the parent zone name servers, not the child (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.2-5, so no unit is bound to it.

### [`RFC4035-4.2-6`](#rfc4035-4.2-6)

When retrieving a missing DS, an iterative-mode resolver MUST query the parent zone name servers, not the child (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.2-6, so no unit is bound to it.

### [`RFC4035-4.3-1`](#rfc4035-4.3-1)

A security-aware resolver MUST be able to determine whether it should expect a particular RRset to be signed, distinguishing Secure, Insecure, Bogus, and Indeterminate (§4.3)

Audit verdict: not audited: no reader has judged these tests

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

A resolver MUST disregard the meaning of the CD and AD bits in a response unless it was obtained over a secure channel or the resolver was configured to trust them (§4.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC4035_ResponseADBitDisregarded`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L159) | unit/verify | unproven |
| positive | [`TestRFC4035_ResponseADBitDisregarded`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc4035_test.go#L148) | unit/verify | unproven |

### [`RFC4035-4.7-2`](#rfc4035-4.7-2)

A resolver that implements a BAD cache MUST take steps to prevent the cache being used as a denial-of-service amplifier (§4.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.7-2, so no unit is bound to it.

### [`RFC4035-4.7-3`](#rfc4035-4.7-3)

Since RRsets that fail to validate lack trustworthy TTLs, the implementation MUST assign a TTL (§4.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.7-3, so no unit is bound to it.

### [`RFC4035-4.7-7`](#rfc4035-4.7-7)

A resolver MUST NOT return RRsets from the BAD cache unless it is not required to validate their signatures (§4.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.7-7, so no unit is bound to it.

### [`RFC4035-4.8-1`](#rfc4035-4.8-1)

A validating resolver MUST treat the signature of a valid signed DNAME RR as also covering unsigned CNAME RRs synthesizable from it, at least by not rejecting the message solely for containing them (§4.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.8-1, so no unit is bound to it.

### [`RFC4035-4.9-1`](#rfc4035-4.9-1)

A security-aware stub resolver MUST support the DNSSEC RR types, at least by not mishandling responses that contain them (§4.9)

Audit verdict: not audited: no reader has judged these tests

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

A security-aware stub resolver MUST NOT place any reliance on signature validation performed on its behalf except when it obtained the data from a trusted recursive name server over a secure channel (§4.9.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-4.9.3-2, so no unit is bound to it.

### [`RFC4035-5-1`](#rfc4035-5-1)

To authenticate an apex DNSKEY RRset with an initial key, the resolver MUST verify that the initial DNSKEY RR appears in the apex DNSKEY RRset and has the Zone Key Flag set (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5-1, so no unit is bound to it.

### [`RFC4035-5-2`](#rfc4035-5-2)

To authenticate an apex DNSKEY RRset with an initial key, the resolver MUST verify that some RRSIG RR covers the apex DNSKEY RRset and that it together with the initial DNSKEY authenticates the RRset (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5-2, so no unit is bound to it.

### [`RFC4035-5-3`](#rfc4035-5-3)

The absence of DNSSEC data in a response MUST NOT by itself be taken as an indication that no authentication information exists (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5-3, so no unit is bound to it.

### [`RFC4035-5.2-1`](#rfc4035-5.2-1)

A security-aware resolver MUST query the parent zone name servers for the DS RRset if a referral includes neither a DS RRset nor an NSEC RRset proving the DS RRset does not exist (§5.2)

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

The validator's notion of the current time MUST be less than or equal to the RRSIG RR's Expiration field (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-5, so no unit is bound to it.

### [`RFC4035-5.3.1-6`](#rfc4035-5.3.1-6)

The validator's notion of the current time MUST be greater than or equal to the RRSIG RR's Inception field (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-6, so no unit is bound to it.

### [`RFC4035-5.3.1-7`](#rfc4035-5.3.1-7)

The RRSIG RR's Signer's Name, Algorithm, and Key Tag fields MUST match the owner name, algorithm, and key tag of some DNSKEY RR in the zone's apex DNSKEY RRset (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-7, so no unit is bound to it.

### [`RFC4035-5.3.1-8`](#rfc4035-5.3.1-8)

The matching DNSKEY RR MUST be present in the zone's apex DNSKEY RRset (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-8, so no unit is bound to it.

### [`RFC4035-5.3.1-9`](#rfc4035-5.3.1-9)

The matching DNSKEY RR MUST have the Zone Flag bit set (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-9, so no unit is bound to it.

### [`RFC4035-5.3.1-10`](#rfc4035-5.3.1-10)

If more than one DNSKEY RR matches, the validator MUST try each until the signature validates or the matching keys are exhausted (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.1-10, so no unit is bound to it.

### [`RFC4035-5.3.2-1`](#rfc4035-5.3.2-1)

If the RRSIG Labels field is greater than the RRset's label count, the RRSIG did not pass validation and MUST NOT be used to authenticate the RRset (§5.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.2-1, so no unit is bound to it.

### [`RFC4035-5.3.2-2`](#rfc4035-5.3.2-2)

When reconstructing the parent-zone NSEC RRset at a delegation, its NSEC RRs MUST NOT be combined with NSEC RRs from the child zone (§5.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.2-2, so no unit is bound to it.

### [`RFC4035-5.3.2-3`](#rfc4035-5.3.2-3)

When reconstructing the child-apex NSEC RRset, its NSEC RRs MUST NOT be combined with NSEC RRs from the parent zone (§5.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.2-3, so no unit is bound to it.

### [`RFC4035-5.3.3-1`](#rfc4035-5.3.3-1)

When the RRSIG Labels field does not equal the owner name's label count, the resolver MUST verify that wildcard expansion was applied properly before considering the RRset authentic (§5.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.3-1, so no unit is bound to it.

### [`RFC4035-5.3.3-2`](#rfc4035-5.3.3-2)

On accepting an RRset as authentic, the validator MUST set the RRSIG RR and each RR's TTL to no greater than the minimum of the RRset TTL, the RRSIG TTL, the RRSIG Original TTL, and the time until the RRSIG expires (§5.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.3.3-2, so no unit is bound to it.

### [`RFC4035-5.4-1`](#rfc4035-5.4-1)

A security-aware resolver MUST authenticate the NSEC RRsets that comprise a denial-of-existence proof (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.4-1, so no unit is bound to it.

### [`RFC4035-5.4-2`](#rfc4035-5.4-2)

If the complete set of necessary NSEC RRsets is not present in a response, the resolver MUST resend the query to obtain them (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.4-2, so no unit is bound to it.

### [`RFC4035-5.4-3`](#rfc4035-5.4-3)

The resolver MUST bound the work it puts into answering any particular query (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.4-3, so no unit is bound to it.

### [`RFC4035-5.4-4`](#rfc4035-5.4-4)

A validator MUST ignore the settings of the NSEC and RRSIG bits in an NSEC RR (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.4-4, so no unit is bound to it.

### [`RFC4035-5.5-2`](#rfc4035-5.5-2)

When validation was done to service a recursive query, the name server MUST return RCODE 2 to the originating client (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.5-2, so no unit is bound to it.

### [`RFC4035-5.5-3`](#rfc4035-5.5-3)

The name server MUST return the full response if and only if the original query had the CD bit set (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4035-5.5-3, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 4035, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4035, so its obligations are stated where they were written.
