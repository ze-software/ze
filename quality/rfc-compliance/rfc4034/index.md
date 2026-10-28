# RFC 4034 - Resource Records for the DNS Security Extensions

No row in the public ledger. Every requirement this repository extracted from RFC 4034, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 59 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 59 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 59 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 59 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 59 | of 59 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (out-of-scope), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 59 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 59 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 59 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 59 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 59 of 59 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 59 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (out-of-scope) |
| Requirements | 59 |
| Gated MUST-level | 59 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 59 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4034.md` |
| Requirement shard | `rfc/requirements/rfc4034.md` |
| RFC text | `rfc/full/rfc4034.txt` |

## Enrolment

Not enrolled (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): Decision taken 2026-10-26: the owner ruled that DNSSEC is not urgent and lists RFC 4033 and RFC 4034 as optional only, never required, for the fttx-edge use case (spec-rfc-profile-declared-membership), and moved DNSSEC validation to post-release the same day (commit 5ff4834b67). The extraction is done: every MUST-level sentence of the text is a row below and is mapped in rfc/extraction/rfc4034.json. The declined feature is DNSSEC record handling itself: ze signs no zone, holds and serves no DNSKEY, RRSIG, NSEC or DS record of its own, and validates no signature. Its resolver is a security-aware non-validating stub whose obligations are gated in rfc/short/rfc4035.md.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 4034.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 59 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **59** | every gated MUST falls in exactly one bucket above |

**No test and no annotation (59):** [`RFC4034-2-1`](#rfc4034-2-1), [`RFC4034-2.1.1-1`](#rfc4034-2.1.1-1), [`RFC4034-2.1.1-2`](#rfc4034-2.1.1-2), [`RFC4034-2.1.1-3`](#rfc4034-2.1.1-3), [`RFC4034-2.1.1-4`](#rfc4034-2.1.1-4), [`RFC4034-2.1.1-5`](#rfc4034-2.1.1-5), [`RFC4034-2.1.2-1`](#rfc4034-2.1.2-1), [`RFC4034-2.2-1`](#rfc4034-2.2-1), [`RFC4034-2.2-2`](#rfc4034-2.2-2), [`RFC4034-2.2-3`](#rfc4034-2.2-3), [`RFC4034-2.2-4`](#rfc4034-2.2-4), [`RFC4034-3-1`](#rfc4034-3-1), [`RFC4034-3-2`](#rfc4034-3-2), [`RFC4034-3-3`](#rfc4034-3-3), [`RFC4034-3-4`](#rfc4034-3-4), [`RFC4034-3.1.3-1`](#rfc4034-3.1.3-1), [`RFC4034-3.1.3-2`](#rfc4034-3.1.3-2), [`RFC4034-3.1.5-1`](#rfc4034-3.1.5-1), [`RFC4034-3.1.5-2`](#rfc4034-3.1.5-2), [`RFC4034-3.1.7-1`](#rfc4034-3.1.7-1), [`RFC4034-3.1.7-2`](#rfc4034-3.1.7-2), [`RFC4034-3.1.8.1-1`](#rfc4034-3.1.8.1-1), [`RFC4034-3.1.8.1-2`](#rfc4034-3.1.8.1-2), [`RFC4034-3.1.8.1-3`](#rfc4034-3.1.8.1-3), [`RFC4034-3.1.8.1-4`](#rfc4034-3.1.8.1-4), [`RFC4034-3.1.8.1-5`](#rfc4034-3.1.8.1-5), [`RFC4034-3.1.8.1-6`](#rfc4034-3.1.8.1-6), [`RFC4034-3.2-1`](#rfc4034-3.2-1), [`RFC4034-3.2-2`](#rfc4034-3.2-2), [`RFC4034-3.2-3`](#rfc4034-3.2-3), [`RFC4034-3.2-4`](#rfc4034-3.2-4), [`RFC4034-3.2-5`](#rfc4034-3.2-5), [`RFC4034-3.2-6`](#rfc4034-3.2-6), [`RFC4034-3.2-7`](#rfc4034-3.2-7), [`RFC4034-4-1`](#rfc4034-4-1), [`RFC4034-4.1.1-1`](#rfc4034-4.1.1-1), [`RFC4034-4.1.1-2`](#rfc4034-4.1.1-2), [`RFC4034-4.1.2-1`](#rfc4034-4.1.2-1), [`RFC4034-4.1.2-2`](#rfc4034-4.1.2-2), [`RFC4034-4.1.2-3`](#rfc4034-4.1.2-3), [`RFC4034-4.1.2-4`](#rfc4034-4.1.2-4), [`RFC4034-4.1.2-5`](#rfc4034-4.1.2-5), [`RFC4034-4.1.2-6`](#rfc4034-4.1.2-6), [`RFC4034-4.1.2-7`](#rfc4034-4.1.2-7), [`RFC4034-4.2-1`](#rfc4034-4.2-1), [`RFC4034-5.2-1`](#rfc4034-5.2-1), [`RFC4034-5.2-2`](#rfc4034-5.2-2), [`RFC4034-5.2-3`](#rfc4034-5.2-3), [`RFC4034-5.3-1`](#rfc4034-5.3-1), [`RFC4034-5.3-2`](#rfc4034-5.3-2), [`RFC4034-5.3-3`](#rfc4034-5.3-3), [`RFC4034-5.3-4`](#rfc4034-5.3-4), [`RFC4034-6.3-1`](#rfc4034-6.3-1), [`RFC4034-6.3-2`](#rfc4034-6.3-2), [`RFC4034-A-1`](#rfc4034-a-1), [`RFC4034-A.1.1-1`](#rfc4034-a.1.1-1), [`RFC4034-B-1`](#rfc4034-b-1), [`RFC4034-B-2`](#rfc4034-b-2), [`RFC4034-B-3`](#rfc4034-b-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4034-2-1` | The DNSKEY RR is not intended as a record for storing arbitrary public keys and MUST NOT be used to store certificates or public keys that do not directly relate to the DNS infrastructure. (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-2.1.1-1` | If bit 7 has value 1, then the DNSKEY record holds a DNS zone key, and the DNSKEY RR's owner name MUST be the name of a zone. (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-2.1.1-2` | If bit 7 has value 0, then the DNSKEY record holds some other type of DNS public key and MUST NOT be used to verify RRSIGs that cover RRsets. (§2.1.1) | MUST NOT | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-2.1.1-3` | This flag is only intended to be a hint to zone signing or debugging software as to the intended use of this DNSKEY record; validators MUST NOT alter their behavior during the signature validation process in any way based on the setting of this bit. (§2.1.1) | MUST NOT | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-2.1.1-4` | A DNSKEY RR with the SEP set and the Zone Key flag not set MUST NOT be used to verify RRSIGs that cover RRsets. (§2.1.1) | MUST NOT | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-2.1.1-5` | Bits 0-6 and 8-14 are reserved: these bits MUST have value 0 upon creation of the DNSKEY RR and MUST be ignored upon receipt. (§2.1.1) | MUST | 2.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-2.1.2-1` | The Protocol Field MUST have value 3, and the DNSKEY RR MUST be treated as invalid during signature verification if it is found to be some value other than 3. (§2.1.2) | MUST | 2.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-2.2-1` | The Flag field MUST be represented as an unsigned decimal integer. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-2.2-2` | The Protocol Field MUST be represented as an unsigned decimal integer with a value of 3. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-2.2-3` | The Algorithm field MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic as specified in Appendix A.1. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-2.2-4` | The Public Key field MUST be represented as a Base64 encoding of the Public Key. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3-1` | The RRSIG RR MUST only be used to carry verification material (digital signatures) used to secure DNS operations. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3-2` | A RRSIG and NSEC (see Section 4) MUST exist for the same name as a CNAME resource record in a signed zone. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3-3` | An RRSIG RR MUST have the same class as the RRset it covers. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3-4` | The TTL value of an RRSIG RR MUST match the TTL value of the RRset it covers. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.3-1` | The value of the Labels field MUST NOT count either the null (root) label that terminates the owner name or the wildcard label (if present). (§3.1.3) | MUST NOT | 3.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.3-2` | The value of the Labels field MUST be less than or equal to the number of labels in the RRSIG owner name. (§3.1.3) | MUST | 3.1.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.5-1` | The RRSIG record MUST NOT be used for authentication prior to the inception date and MUST NOT be used for authentication after the expiration date. (§3.1.5) | MUST NOT | 3.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.5-2` | Because of this, all comparisons involving these fields MUST use "Serial number arithmetic", as defined in [RFC1982]. (§3.1.5) | MUST | 3.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.7-1` | The Signer's Name field MUST contain the name of the zone of the covered RRset. (§3.1.7) | MUST | 3.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.7-2` | A sender MUST NOT use DNS name compression on the Signer's Name field when transmitting a RRSIG RR. (§3.1.7) | MUST NOT | 3.1.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.8.1-1` | Each RR MUST have the same owner name as the RRSIG RR; (§3.1.8.1) | MUST | 3.1.8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.8.1-2` | Each RR MUST have the same class as the RRSIG RR; (§3.1.8.1) | MUST | 3.1.8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.8.1-3` | Each RR in the RRset MUST have the RR type listed in the RRSIG RR's Type Covered field; (§3.1.8.1) | MUST | 3.1.8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.8.1-4` | Each RR in the RRset MUST have the TTL listed in the RRSIG Original TTL Field; (§3.1.8.1) | MUST | 3.1.8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.8.1-5` | Any DNS names in the RDATA field of each RR MUST be in canonical form; and (§3.1.8.1) | MUST | 3.1.8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.1.8.1-6` | The RRset MUST be sorted in canonical order. (§3.1.8.1) | MUST | 3.1.8.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.2-1` | When the mnemonic is not known, the TYPE representation as described in [RFC3597], Section 5, MUST be used. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.2-2` | The Algorithm field value MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic, as specified in Appendix A.1. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.2-3` | The Labels field value MUST be represented as an unsigned decimal integer. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.2-4` | The Original TTL field value MUST be represented as an unsigned decimal integer. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.2-5` | The Signature Expiration Time and Inception Time field values MUST be represented either as an unsigned decimal integer indicating seconds since 1 January 1970 00:00:00 UTC, or in the form YYYYMMDDHHmmSS in UTC, where: (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.2-6` | The Key Tag field MUST be represented as an unsigned decimal integer. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-3.2-7` | The Signer's Name field value MUST be represented as a domain name. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-4-1` | An RRSIG (see Section 3) and NSEC MUST exist for the same name as does a CNAME resource record in a signed zone. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-4.1.1-1` | A sender MUST NOT use DNS name compression on the Next Domain Name field when transmitting an NSEC RR. (§4.1.1) | MUST NOT | 4.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-4.1.1-2` | Owner names of RRsets for which the given zone is not authoritative (such as glue records) MUST NOT be listed in the Next Domain Name unless at least one authoritative RRset exists at the same owner name. (§4.1.1) | MUST NOT | 4.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-4.1.2-1` | Bits representing pseudo-types MUST be clear, as they do not appear in zone data. (§4.1.2) | MUST | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-4.1.2-2` | If encountered, they MUST be ignored upon being read. (§4.1.2) | MUST | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-4.1.2-3` | Blocks with no types present MUST NOT be included. (§4.1.2) | MUST NOT | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-4.1.2-4` | Trailing zero octets in the bitmap MUST be omitted. (§4.1.2) | MUST | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-4.1.2-5` | Trailing zero octets not specified MUST be interpreted as zero octets. (§4.1.2) | MUST | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-4.1.2-6` | Bits corresponding to the delegation NS RRset and the RR types for which the parent zone has authoritative data MUST be set; bits corresponding to any non-NS RRset for which the parent is not authoritative MUST be clear. (§4.1.2) | MUST | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-4.1.2-7` | A zone MUST NOT include an NSEC RR for any domain name that only holds glue records. (§4.1.2) | MUST NOT | 4.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-4.2-1` | When the mnemonic is not known, the TYPE representation described in [RFC3597], Section 5, MUST be used. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-5.2-1` | The DNSKEY RR referred to in the DS RR MUST be a DNSSEC zone key. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-5.2-2` | The DNSKEY RR Flags MUST have Flags bit 7 set. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-5.2-3` | If the DNSKEY flags do not indicate a DNSSEC zone key, the DS RR (and the DNSKEY RR it references) MUST NOT be used in the validation process. (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-5.3-1` | The Key Tag field MUST be represented as an unsigned decimal integer. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-5.3-2` | The Algorithm field MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic specified in Appendix A.1. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-5.3-3` | The Digest Type field MUST be represented as an unsigned decimal integer. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-5.3-4` | The Digest MUST be represented as a sequence of case-insensitive hexadecimal digits. (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-6.3-1` | Therefore, if an implementation detects duplicate RRs when putting the RRset in canonical form, it MUST treat this as a protocol error. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-6.3-2` | If the implementation chooses to handle this protocol error in the spirit of the robustness principle (being liberal in what it accepts), it MUST remove all but one of the duplicate RR(s) for the purposes of calculating the canonical form of the RRset. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-A-1` | A DNSSEC aware resolver or name server MUST implement all MANDATORY algorithms. (§A) | MUST | A | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-A.1.1-1` | The public key area in the DNSKEY RR and the signature area in the RRSIG RR begin with a wire encoded domain name, which MUST NOT be compressed. (§A.1.1) | MUST NOT | A.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-B-1` | Implementations MUST NOT assume that the key tag uniquely identifies a DNSKEY RR. (§B) | MUST NOT | B | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-B-2` | It is not necessary to use the following reference code verbatim, but the numerical value of the Key Tag MUST be identical to what the reference implementation would generate for the same input. (§B) | MUST | B | **positive:** no positive test. **negative:** no negative test |
| `RFC4034-B-3` | Key Tags MUST be calculated using the algorithm described here rather than the ones complement checksum. (§B) | MUST | B | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4034-2-1`](#rfc4034-2-1) The DNSKEY RR is not intended as a record for storing arbitrary public keys and MUST NOT be used to store certificates or public keys that do not directly relate to the DNS infrastructure. (§2) | no test | no test carries this requirement id |
| [`RFC4034-2.1.1-1`](#rfc4034-2.1.1-1) If bit 7 has value 1, then the DNSKEY record holds a DNS zone key, and the DNSKEY RR's owner name MUST be the name of a zone. (§2.1.1) | no test | no test carries this requirement id |
| [`RFC4034-2.1.1-2`](#rfc4034-2.1.1-2) If bit 7 has value 0, then the DNSKEY record holds some other type of DNS public key and MUST NOT be used to verify RRSIGs that cover RRsets. (§2.1.1) | no test | no test carries this requirement id |
| [`RFC4034-2.1.1-3`](#rfc4034-2.1.1-3) This flag is only intended to be a hint to zone signing or debugging software as to the intended use of this DNSKEY record; validators MUST NOT alter their behavior during the signature validation process in any way based on the setting of this bit. (§2.1.1) | no test | no test carries this requirement id |
| [`RFC4034-2.1.1-4`](#rfc4034-2.1.1-4) A DNSKEY RR with the SEP set and the Zone Key flag not set MUST NOT be used to verify RRSIGs that cover RRsets. (§2.1.1) | no test | no test carries this requirement id |
| [`RFC4034-2.1.1-5`](#rfc4034-2.1.1-5) Bits 0-6 and 8-14 are reserved: these bits MUST have value 0 upon creation of the DNSKEY RR and MUST be ignored upon receipt. (§2.1.1) | no test | no test carries this requirement id |
| [`RFC4034-2.1.2-1`](#rfc4034-2.1.2-1) The Protocol Field MUST have value 3, and the DNSKEY RR MUST be treated as invalid during signature verification if it is found to be some value other than 3. (§2.1.2) | no test | no test carries this requirement id |
| [`RFC4034-2.2-1`](#rfc4034-2.2-1) The Flag field MUST be represented as an unsigned decimal integer. (§2.2) | no test | no test carries this requirement id |
| [`RFC4034-2.2-2`](#rfc4034-2.2-2) The Protocol Field MUST be represented as an unsigned decimal integer with a value of 3. (§2.2) | no test | no test carries this requirement id |
| [`RFC4034-2.2-3`](#rfc4034-2.2-3) The Algorithm field MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic as specified in Appendix A.1. (§2.2) | no test | no test carries this requirement id |
| [`RFC4034-2.2-4`](#rfc4034-2.2-4) The Public Key field MUST be represented as a Base64 encoding of the Public Key. (§2.2) | no test | no test carries this requirement id |
| [`RFC4034-3-1`](#rfc4034-3-1) The RRSIG RR MUST only be used to carry verification material (digital signatures) used to secure DNS operations. (§3) | no test | no test carries this requirement id |
| [`RFC4034-3-2`](#rfc4034-3-2) A RRSIG and NSEC (see Section 4) MUST exist for the same name as a CNAME resource record in a signed zone. (§3) | no test | no test carries this requirement id |
| [`RFC4034-3-3`](#rfc4034-3-3) An RRSIG RR MUST have the same class as the RRset it covers. (§3) | no test | no test carries this requirement id |
| [`RFC4034-3-4`](#rfc4034-3-4) The TTL value of an RRSIG RR MUST match the TTL value of the RRset it covers. (§3) | no test | no test carries this requirement id |
| [`RFC4034-3.1.3-1`](#rfc4034-3.1.3-1) The value of the Labels field MUST NOT count either the null (root) label that terminates the owner name or the wildcard label (if present). (§3.1.3) | no test | no test carries this requirement id |
| [`RFC4034-3.1.3-2`](#rfc4034-3.1.3-2) The value of the Labels field MUST be less than or equal to the number of labels in the RRSIG owner name. (§3.1.3) | no test | no test carries this requirement id |
| [`RFC4034-3.1.5-1`](#rfc4034-3.1.5-1) The RRSIG record MUST NOT be used for authentication prior to the inception date and MUST NOT be used for authentication after the expiration date. (§3.1.5) | no test | no test carries this requirement id |
| [`RFC4034-3.1.5-2`](#rfc4034-3.1.5-2) Because of this, all comparisons involving these fields MUST use "Serial number arithmetic", as defined in [RFC1982]. (§3.1.5) | no test | no test carries this requirement id |
| [`RFC4034-3.1.7-1`](#rfc4034-3.1.7-1) The Signer's Name field MUST contain the name of the zone of the covered RRset. (§3.1.7) | no test | no test carries this requirement id |
| [`RFC4034-3.1.7-2`](#rfc4034-3.1.7-2) A sender MUST NOT use DNS name compression on the Signer's Name field when transmitting a RRSIG RR. (§3.1.7) | no test | no test carries this requirement id |
| [`RFC4034-3.1.8.1-1`](#rfc4034-3.1.8.1-1) Each RR MUST have the same owner name as the RRSIG RR; (§3.1.8.1) | no test | no test carries this requirement id |
| [`RFC4034-3.1.8.1-2`](#rfc4034-3.1.8.1-2) Each RR MUST have the same class as the RRSIG RR; (§3.1.8.1) | no test | no test carries this requirement id |
| [`RFC4034-3.1.8.1-3`](#rfc4034-3.1.8.1-3) Each RR in the RRset MUST have the RR type listed in the RRSIG RR's Type Covered field; (§3.1.8.1) | no test | no test carries this requirement id |
| [`RFC4034-3.1.8.1-4`](#rfc4034-3.1.8.1-4) Each RR in the RRset MUST have the TTL listed in the RRSIG Original TTL Field; (§3.1.8.1) | no test | no test carries this requirement id |
| [`RFC4034-3.1.8.1-5`](#rfc4034-3.1.8.1-5) Any DNS names in the RDATA field of each RR MUST be in canonical form; and (§3.1.8.1) | no test | no test carries this requirement id |
| [`RFC4034-3.1.8.1-6`](#rfc4034-3.1.8.1-6) The RRset MUST be sorted in canonical order. (§3.1.8.1) | no test | no test carries this requirement id |
| [`RFC4034-3.2-1`](#rfc4034-3.2-1) When the mnemonic is not known, the TYPE representation as described in [RFC3597], Section 5, MUST be used. (§3.2) | no test | no test carries this requirement id |
| [`RFC4034-3.2-2`](#rfc4034-3.2-2) The Algorithm field value MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic, as specified in Appendix A.1. (§3.2) | no test | no test carries this requirement id |
| [`RFC4034-3.2-3`](#rfc4034-3.2-3) The Labels field value MUST be represented as an unsigned decimal integer. (§3.2) | no test | no test carries this requirement id |
| [`RFC4034-3.2-4`](#rfc4034-3.2-4) The Original TTL field value MUST be represented as an unsigned decimal integer. (§3.2) | no test | no test carries this requirement id |
| [`RFC4034-3.2-5`](#rfc4034-3.2-5) The Signature Expiration Time and Inception Time field values MUST be represented either as an unsigned decimal integer indicating seconds since 1 January 1970 00:00:00 UTC, or in the form YYYYMMDDHHmmSS in UTC, where: (§3.2) | no test | no test carries this requirement id |
| [`RFC4034-3.2-6`](#rfc4034-3.2-6) The Key Tag field MUST be represented as an unsigned decimal integer. (§3.2) | no test | no test carries this requirement id |
| [`RFC4034-3.2-7`](#rfc4034-3.2-7) The Signer's Name field value MUST be represented as a domain name. (§3.2) | no test | no test carries this requirement id |
| [`RFC4034-4-1`](#rfc4034-4-1) An RRSIG (see Section 3) and NSEC MUST exist for the same name as does a CNAME resource record in a signed zone. (§4) | no test | no test carries this requirement id |
| [`RFC4034-4.1.1-1`](#rfc4034-4.1.1-1) A sender MUST NOT use DNS name compression on the Next Domain Name field when transmitting an NSEC RR. (§4.1.1) | no test | no test carries this requirement id |
| [`RFC4034-4.1.1-2`](#rfc4034-4.1.1-2) Owner names of RRsets for which the given zone is not authoritative (such as glue records) MUST NOT be listed in the Next Domain Name unless at least one authoritative RRset exists at the same owner name. (§4.1.1) | no test | no test carries this requirement id |
| [`RFC4034-4.1.2-1`](#rfc4034-4.1.2-1) Bits representing pseudo-types MUST be clear, as they do not appear in zone data. (§4.1.2) | no test | no test carries this requirement id |
| [`RFC4034-4.1.2-2`](#rfc4034-4.1.2-2) If encountered, they MUST be ignored upon being read. (§4.1.2) | no test | no test carries this requirement id |
| [`RFC4034-4.1.2-3`](#rfc4034-4.1.2-3) Blocks with no types present MUST NOT be included. (§4.1.2) | no test | no test carries this requirement id |
| [`RFC4034-4.1.2-4`](#rfc4034-4.1.2-4) Trailing zero octets in the bitmap MUST be omitted. (§4.1.2) | no test | no test carries this requirement id |
| [`RFC4034-4.1.2-5`](#rfc4034-4.1.2-5) Trailing zero octets not specified MUST be interpreted as zero octets. (§4.1.2) | no test | no test carries this requirement id |
| [`RFC4034-4.1.2-6`](#rfc4034-4.1.2-6) Bits corresponding to the delegation NS RRset and the RR types for which the parent zone has authoritative data MUST be set; bits corresponding to any non-NS RRset for which the parent is not authoritative MUST be clear. (§4.1.2) | no test | no test carries this requirement id |
| [`RFC4034-4.1.2-7`](#rfc4034-4.1.2-7) A zone MUST NOT include an NSEC RR for any domain name that only holds glue records. (§4.1.2) | no test | no test carries this requirement id |
| [`RFC4034-4.2-1`](#rfc4034-4.2-1) When the mnemonic is not known, the TYPE representation described in [RFC3597], Section 5, MUST be used. (§4.2) | no test | no test carries this requirement id |
| [`RFC4034-5.2-1`](#rfc4034-5.2-1) The DNSKEY RR referred to in the DS RR MUST be a DNSSEC zone key. (§5.2) | no test | no test carries this requirement id |
| [`RFC4034-5.2-2`](#rfc4034-5.2-2) The DNSKEY RR Flags MUST have Flags bit 7 set. (§5.2) | no test | no test carries this requirement id |
| [`RFC4034-5.2-3`](#rfc4034-5.2-3) If the DNSKEY flags do not indicate a DNSSEC zone key, the DS RR (and the DNSKEY RR it references) MUST NOT be used in the validation process. (§5.2) | no test | no test carries this requirement id |
| [`RFC4034-5.3-1`](#rfc4034-5.3-1) The Key Tag field MUST be represented as an unsigned decimal integer. (§5.3) | no test | no test carries this requirement id |
| [`RFC4034-5.3-2`](#rfc4034-5.3-2) The Algorithm field MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic specified in Appendix A.1. (§5.3) | no test | no test carries this requirement id |
| [`RFC4034-5.3-3`](#rfc4034-5.3-3) The Digest Type field MUST be represented as an unsigned decimal integer. (§5.3) | no test | no test carries this requirement id |
| [`RFC4034-5.3-4`](#rfc4034-5.3-4) The Digest MUST be represented as a sequence of case-insensitive hexadecimal digits. (§5.3) | no test | no test carries this requirement id |
| [`RFC4034-6.3-1`](#rfc4034-6.3-1) Therefore, if an implementation detects duplicate RRs when putting the RRset in canonical form, it MUST treat this as a protocol error. (§6.3) | no test | no test carries this requirement id |
| [`RFC4034-6.3-2`](#rfc4034-6.3-2) If the implementation chooses to handle this protocol error in the spirit of the robustness principle (being liberal in what it accepts), it MUST remove all but one of the duplicate RR(s) for the purposes of calculating the canonical form of the RRset. (§6.3) | no test | no test carries this requirement id |
| [`RFC4034-A-1`](#rfc4034-a-1) A DNSSEC aware resolver or name server MUST implement all MANDATORY algorithms. (§A) | no test | no test carries this requirement id |
| [`RFC4034-A.1.1-1`](#rfc4034-a.1.1-1) The public key area in the DNSKEY RR and the signature area in the RRSIG RR begin with a wire encoded domain name, which MUST NOT be compressed. (§A.1.1) | no test | no test carries this requirement id |
| [`RFC4034-B-1`](#rfc4034-b-1) Implementations MUST NOT assume that the key tag uniquely identifies a DNSKEY RR. (§B) | no test | no test carries this requirement id |
| [`RFC4034-B-2`](#rfc4034-b-2) It is not necessary to use the following reference code verbatim, but the numerical value of the Key Tag MUST be identical to what the reference implementation would generate for the same input. (§B) | no test | no test carries this requirement id |
| [`RFC4034-B-3`](#rfc4034-b-3) Key Tags MUST be calculated using the algorithm described here rather than the ones complement checksum. (§B) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4034-2-1`](#rfc4034-2-1)

The DNSKEY RR is not intended as a record for storing arbitrary public keys and MUST NOT be used to store certificates or public keys that do not directly relate to the DNS infrastructure. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-2-1, so no unit is bound to it.

### [`RFC4034-2.1.1-1`](#rfc4034-2.1.1-1)

If bit 7 has value 1, then the DNSKEY record holds a DNS zone key, and the DNSKEY RR's owner name MUST be the name of a zone. (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-2.1.1-1, so no unit is bound to it.

### [`RFC4034-2.1.1-2`](#rfc4034-2.1.1-2)

If bit 7 has value 0, then the DNSKEY record holds some other type of DNS public key and MUST NOT be used to verify RRSIGs that cover RRsets. (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-2.1.1-2, so no unit is bound to it.

### [`RFC4034-2.1.1-3`](#rfc4034-2.1.1-3)

This flag is only intended to be a hint to zone signing or debugging software as to the intended use of this DNSKEY record; validators MUST NOT alter their behavior during the signature validation process in any way based on the setting of this bit. (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-2.1.1-3, so no unit is bound to it.

### [`RFC4034-2.1.1-4`](#rfc4034-2.1.1-4)

A DNSKEY RR with the SEP set and the Zone Key flag not set MUST NOT be used to verify RRSIGs that cover RRsets. (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-2.1.1-4, so no unit is bound to it.

### [`RFC4034-2.1.1-5`](#rfc4034-2.1.1-5)

Bits 0-6 and 8-14 are reserved: these bits MUST have value 0 upon creation of the DNSKEY RR and MUST be ignored upon receipt. (§2.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-2.1.1-5, so no unit is bound to it.

### [`RFC4034-2.1.2-1`](#rfc4034-2.1.2-1)

The Protocol Field MUST have value 3, and the DNSKEY RR MUST be treated as invalid during signature verification if it is found to be some value other than 3. (§2.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-2.1.2-1, so no unit is bound to it.

### [`RFC4034-2.2-1`](#rfc4034-2.2-1)

The Flag field MUST be represented as an unsigned decimal integer. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-2.2-1, so no unit is bound to it.

### [`RFC4034-2.2-2`](#rfc4034-2.2-2)

The Protocol Field MUST be represented as an unsigned decimal integer with a value of 3. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-2.2-2, so no unit is bound to it.

### [`RFC4034-2.2-3`](#rfc4034-2.2-3)

The Algorithm field MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic as specified in Appendix A.1. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-2.2-3, so no unit is bound to it.

### [`RFC4034-2.2-4`](#rfc4034-2.2-4)

The Public Key field MUST be represented as a Base64 encoding of the Public Key. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-2.2-4, so no unit is bound to it.

### [`RFC4034-3-1`](#rfc4034-3-1)

The RRSIG RR MUST only be used to carry verification material (digital signatures) used to secure DNS operations. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3-1, so no unit is bound to it.

### [`RFC4034-3-2`](#rfc4034-3-2)

A RRSIG and NSEC (see Section 4) MUST exist for the same name as a CNAME resource record in a signed zone. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3-2, so no unit is bound to it.

### [`RFC4034-3-3`](#rfc4034-3-3)

An RRSIG RR MUST have the same class as the RRset it covers. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3-3, so no unit is bound to it.

### [`RFC4034-3-4`](#rfc4034-3-4)

The TTL value of an RRSIG RR MUST match the TTL value of the RRset it covers. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3-4, so no unit is bound to it.

### [`RFC4034-3.1.3-1`](#rfc4034-3.1.3-1)

The value of the Labels field MUST NOT count either the null (root) label that terminates the owner name or the wildcard label (if present). (§3.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.3-1, so no unit is bound to it.

### [`RFC4034-3.1.3-2`](#rfc4034-3.1.3-2)

The value of the Labels field MUST be less than or equal to the number of labels in the RRSIG owner name. (§3.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.3-2, so no unit is bound to it.

### [`RFC4034-3.1.5-1`](#rfc4034-3.1.5-1)

The RRSIG record MUST NOT be used for authentication prior to the inception date and MUST NOT be used for authentication after the expiration date. (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.5-1, so no unit is bound to it.

### [`RFC4034-3.1.5-2`](#rfc4034-3.1.5-2)

Because of this, all comparisons involving these fields MUST use "Serial number arithmetic", as defined in [RFC1982]. (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.5-2, so no unit is bound to it.

### [`RFC4034-3.1.7-1`](#rfc4034-3.1.7-1)

The Signer's Name field MUST contain the name of the zone of the covered RRset. (§3.1.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.7-1, so no unit is bound to it.

### [`RFC4034-3.1.7-2`](#rfc4034-3.1.7-2)

A sender MUST NOT use DNS name compression on the Signer's Name field when transmitting a RRSIG RR. (§3.1.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.7-2, so no unit is bound to it.

### [`RFC4034-3.1.8.1-1`](#rfc4034-3.1.8.1-1)

Each RR MUST have the same owner name as the RRSIG RR; (§3.1.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.8.1-1, so no unit is bound to it.

### [`RFC4034-3.1.8.1-2`](#rfc4034-3.1.8.1-2)

Each RR MUST have the same class as the RRSIG RR; (§3.1.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.8.1-2, so no unit is bound to it.

### [`RFC4034-3.1.8.1-3`](#rfc4034-3.1.8.1-3)

Each RR in the RRset MUST have the RR type listed in the RRSIG RR's Type Covered field; (§3.1.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.8.1-3, so no unit is bound to it.

### [`RFC4034-3.1.8.1-4`](#rfc4034-3.1.8.1-4)

Each RR in the RRset MUST have the TTL listed in the RRSIG Original TTL Field; (§3.1.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.8.1-4, so no unit is bound to it.

### [`RFC4034-3.1.8.1-5`](#rfc4034-3.1.8.1-5)

Any DNS names in the RDATA field of each RR MUST be in canonical form; and (§3.1.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.8.1-5, so no unit is bound to it.

### [`RFC4034-3.1.8.1-6`](#rfc4034-3.1.8.1-6)

The RRset MUST be sorted in canonical order. (§3.1.8.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.1.8.1-6, so no unit is bound to it.

### [`RFC4034-3.2-1`](#rfc4034-3.2-1)

When the mnemonic is not known, the TYPE representation as described in [RFC3597], Section 5, MUST be used. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.2-1, so no unit is bound to it.

### [`RFC4034-3.2-2`](#rfc4034-3.2-2)

The Algorithm field value MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic, as specified in Appendix A.1. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.2-2, so no unit is bound to it.

### [`RFC4034-3.2-3`](#rfc4034-3.2-3)

The Labels field value MUST be represented as an unsigned decimal integer. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.2-3, so no unit is bound to it.

### [`RFC4034-3.2-4`](#rfc4034-3.2-4)

The Original TTL field value MUST be represented as an unsigned decimal integer. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.2-4, so no unit is bound to it.

### [`RFC4034-3.2-5`](#rfc4034-3.2-5)

The Signature Expiration Time and Inception Time field values MUST be represented either as an unsigned decimal integer indicating seconds since 1 January 1970 00:00:00 UTC, or in the form YYYYMMDDHHmmSS in UTC, where: (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.2-5, so no unit is bound to it.

### [`RFC4034-3.2-6`](#rfc4034-3.2-6)

The Key Tag field MUST be represented as an unsigned decimal integer. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.2-6, so no unit is bound to it.

### [`RFC4034-3.2-7`](#rfc4034-3.2-7)

The Signer's Name field value MUST be represented as a domain name. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-3.2-7, so no unit is bound to it.

### [`RFC4034-4-1`](#rfc4034-4-1)

An RRSIG (see Section 3) and NSEC MUST exist for the same name as does a CNAME resource record in a signed zone. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-4-1, so no unit is bound to it.

### [`RFC4034-4.1.1-1`](#rfc4034-4.1.1-1)

A sender MUST NOT use DNS name compression on the Next Domain Name field when transmitting an NSEC RR. (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-4.1.1-1, so no unit is bound to it.

### [`RFC4034-4.1.1-2`](#rfc4034-4.1.1-2)

Owner names of RRsets for which the given zone is not authoritative (such as glue records) MUST NOT be listed in the Next Domain Name unless at least one authoritative RRset exists at the same owner name. (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-4.1.1-2, so no unit is bound to it.

### [`RFC4034-4.1.2-1`](#rfc4034-4.1.2-1)

Bits representing pseudo-types MUST be clear, as they do not appear in zone data. (§4.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-4.1.2-1, so no unit is bound to it.

### [`RFC4034-4.1.2-2`](#rfc4034-4.1.2-2)

If encountered, they MUST be ignored upon being read. (§4.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-4.1.2-2, so no unit is bound to it.

### [`RFC4034-4.1.2-3`](#rfc4034-4.1.2-3)

Blocks with no types present MUST NOT be included. (§4.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-4.1.2-3, so no unit is bound to it.

### [`RFC4034-4.1.2-4`](#rfc4034-4.1.2-4)

Trailing zero octets in the bitmap MUST be omitted. (§4.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-4.1.2-4, so no unit is bound to it.

### [`RFC4034-4.1.2-5`](#rfc4034-4.1.2-5)

Trailing zero octets not specified MUST be interpreted as zero octets. (§4.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-4.1.2-5, so no unit is bound to it.

### [`RFC4034-4.1.2-6`](#rfc4034-4.1.2-6)

Bits corresponding to the delegation NS RRset and the RR types for which the parent zone has authoritative data MUST be set; bits corresponding to any non-NS RRset for which the parent is not authoritative MUST be clear. (§4.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-4.1.2-6, so no unit is bound to it.

### [`RFC4034-4.1.2-7`](#rfc4034-4.1.2-7)

A zone MUST NOT include an NSEC RR for any domain name that only holds glue records. (§4.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-4.1.2-7, so no unit is bound to it.

### [`RFC4034-4.2-1`](#rfc4034-4.2-1)

When the mnemonic is not known, the TYPE representation described in [RFC3597], Section 5, MUST be used. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-4.2-1, so no unit is bound to it.

### [`RFC4034-5.2-1`](#rfc4034-5.2-1)

The DNSKEY RR referred to in the DS RR MUST be a DNSSEC zone key. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-5.2-1, so no unit is bound to it.

### [`RFC4034-5.2-2`](#rfc4034-5.2-2)

The DNSKEY RR Flags MUST have Flags bit 7 set. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-5.2-2, so no unit is bound to it.

### [`RFC4034-5.2-3`](#rfc4034-5.2-3)

If the DNSKEY flags do not indicate a DNSSEC zone key, the DS RR (and the DNSKEY RR it references) MUST NOT be used in the validation process. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-5.2-3, so no unit is bound to it.

### [`RFC4034-5.3-1`](#rfc4034-5.3-1)

The Key Tag field MUST be represented as an unsigned decimal integer. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-5.3-1, so no unit is bound to it.

### [`RFC4034-5.3-2`](#rfc4034-5.3-2)

The Algorithm field MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic specified in Appendix A.1. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-5.3-2, so no unit is bound to it.

### [`RFC4034-5.3-3`](#rfc4034-5.3-3)

The Digest Type field MUST be represented as an unsigned decimal integer. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-5.3-3, so no unit is bound to it.

### [`RFC4034-5.3-4`](#rfc4034-5.3-4)

The Digest MUST be represented as a sequence of case-insensitive hexadecimal digits. (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-5.3-4, so no unit is bound to it.

### [`RFC4034-6.3-1`](#rfc4034-6.3-1)

Therefore, if an implementation detects duplicate RRs when putting the RRset in canonical form, it MUST treat this as a protocol error. (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-6.3-1, so no unit is bound to it.

### [`RFC4034-6.3-2`](#rfc4034-6.3-2)

If the implementation chooses to handle this protocol error in the spirit of the robustness principle (being liberal in what it accepts), it MUST remove all but one of the duplicate RR(s) for the purposes of calculating the canonical form of the RRset. (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-6.3-2, so no unit is bound to it.

### [`RFC4034-A-1`](#rfc4034-a-1)

A DNSSEC aware resolver or name server MUST implement all MANDATORY algorithms. (§A)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-A-1, so no unit is bound to it.

### [`RFC4034-A.1.1-1`](#rfc4034-a.1.1-1)

The public key area in the DNSKEY RR and the signature area in the RRSIG RR begin with a wire encoded domain name, which MUST NOT be compressed. (§A.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-A.1.1-1, so no unit is bound to it.

### [`RFC4034-B-1`](#rfc4034-b-1)

Implementations MUST NOT assume that the key tag uniquely identifies a DNSKEY RR. (§B)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-B-1, so no unit is bound to it.

### [`RFC4034-B-2`](#rfc4034-b-2)

It is not necessary to use the following reference code verbatim, but the numerical value of the Key Tag MUST be identical to what the reference implementation would generate for the same input. (§B)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-B-2, so no unit is bound to it.

### [`RFC4034-B-3`](#rfc4034-b-3)

Key Tags MUST be calculated using the algorithm described here rather than the ones complement checksum. (§B)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4034-B-3, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc4034.txt |
| Source fingerprint | a2e74ca3f29567ab |
| Record | rfc/extraction/rfc4034.json |
| Mapped sentences | 59 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.1.1` | not stated | 5 | walked | not stated |
| `2.1.2` | not stated | 1 | walked | not stated |
| `2.1.3` | not stated | 0 | walked | not stated |
| `2.1.4` | not stated | 0 | walked | not stated |
| `2.1.5` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 4 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `3` | not stated | 4 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.1.1` | not stated | 0 | walked | not stated |
| `3.1.2` | not stated | 0 | walked | not stated |
| `3.1.3` | not stated | 2 | walked | not stated |
| `3.1.4` | not stated | 0 | walked | not stated |
| `3.1.5` | not stated | 2 | walked | not stated |
| `3.1.6` | not stated | 0 | walked | not stated |
| `3.1.7` | not stated | 2 | walked | not stated |
| `3.1.8` | not stated | 0 | walked | not stated |
| `3.1.8.1` | not stated | 6 | walked | not stated |
| `3.2` | not stated | 7 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.1.1` | not stated | 2 | walked | not stated |
| `4.1.2` | not stated | 7 | walked | not stated |
| `4.1.3` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 1 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.1.1` | not stated | 0 | walked | not stated |
| `5.1.2` | not stated | 0 | walked | not stated |
| `5.1.3` | not stated | 0 | walked | not stated |
| `5.1.4` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 3 | walked | not stated |
| `5.3` | not stated | 4 | walked | not stated |
| `5.4` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 2 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 1 | walked | not stated |
| `A.1` | not stated | 0 | walked | not stated |
| `A.1.1` | not stated | 1 | walked | not stated |
| `A.2` | not stated | 0 | walked | not stated |
| `B` | not stated | 3 | walked | not stated |
| `B.1` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 4034 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 4034, so its obligations are stated where they were written.
