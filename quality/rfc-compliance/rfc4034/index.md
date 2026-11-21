# RFC 4034 - Resource Records for the DNS Security Extensions

No entry in the public support list. Every requirement listed for RFC 4034, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 59 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 59 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 59 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 59 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 59 | of 59 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 59 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 59 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 59 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 59 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 59 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 59 | of 59 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (out-of-scope), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 59 mandatory requirements: they add up to 59. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Mandatory requirements (MUST level), not checked | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | No entry in the public support list |
| Automatic checks | Excluded (out-of-scope) |
| Requirements | 59 |
| Mandatory requirements (MUST level), not checked | 59 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 59 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc4034.md` |
| Requirement table | `rfc/requirements/rfc4034.md` |
| RFC text | `rfc/full/rfc4034.txt` |

## Automatic checks

Excluded from automatic checks (out-of-scope, the requirements ARE extracted and the owner decided not to offer the feature for now, so the absence is a scope decision rather than a conformance gap): Decision taken 2026-10-26: the owner ruled that DNSSEC is not urgent and lists RFC 4033 and RFC 4034 as optional only, never required, for the fttx-edge use case (spec-rfc-profile-declared-membership), and moved DNSSEC validation to post-release the same day (commit 5ff4834b67). The extraction is done: every MUST-level sentence of the text is a row below and is mapped in rfc/extraction/rfc4034.json. DNSSEC record handling is not declined: the owner made it optional and post-release, and DNSSEC validation is wanted later (plan/post-release/features/spec-dns-resolver-dnssec-validation.md), so these rows are owed after the release. Today ze signs no zone, holds and serves no DNSKEY, RRSIG, NSEC or DS record of its own, validates no signature, and reads no master file: ze resolve dns prints only A, AAAA, TXT and PTR answers (internal/component/resolve/dns/resolver.go::extractRecords renders no DNSSEC record in presentation format). Its resolver is a security-aware non-validating stub whose obligations are gated in rfc/short/rfc4035.md.

## Published support status

No entry in the public support list, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 4034.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 59 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **59** | every mandatory requirement appears in exactly one group above |

**No test and no annotation (59):** [`RFC4034-2-1`](#rfc4034-2-1), [`RFC4034-2.1.1-1`](#rfc4034-2.1.1-1), [`RFC4034-2.1.1-2`](#rfc4034-2.1.1-2), [`RFC4034-2.1.1-3`](#rfc4034-2.1.1-3), [`RFC4034-2.1.1-4`](#rfc4034-2.1.1-4), [`RFC4034-2.1.1-5`](#rfc4034-2.1.1-5), [`RFC4034-2.1.2-1`](#rfc4034-2.1.2-1), [`RFC4034-2.2-1`](#rfc4034-2.2-1), [`RFC4034-2.2-2`](#rfc4034-2.2-2), [`RFC4034-2.2-3`](#rfc4034-2.2-3), [`RFC4034-2.2-4`](#rfc4034-2.2-4), [`RFC4034-3-1`](#rfc4034-3-1), [`RFC4034-3-2`](#rfc4034-3-2), [`RFC4034-3-3`](#rfc4034-3-3), [`RFC4034-3-4`](#rfc4034-3-4), [`RFC4034-3.1.3-1`](#rfc4034-3.1.3-1), [`RFC4034-3.1.3-2`](#rfc4034-3.1.3-2), [`RFC4034-3.1.5-1`](#rfc4034-3.1.5-1), [`RFC4034-3.1.5-2`](#rfc4034-3.1.5-2), [`RFC4034-3.1.7-1`](#rfc4034-3.1.7-1), [`RFC4034-3.1.7-2`](#rfc4034-3.1.7-2), [`RFC4034-3.1.8.1-1`](#rfc4034-3.1.8.1-1), [`RFC4034-3.1.8.1-2`](#rfc4034-3.1.8.1-2), [`RFC4034-3.1.8.1-3`](#rfc4034-3.1.8.1-3), [`RFC4034-3.1.8.1-4`](#rfc4034-3.1.8.1-4), [`RFC4034-3.1.8.1-5`](#rfc4034-3.1.8.1-5), [`RFC4034-3.1.8.1-6`](#rfc4034-3.1.8.1-6), [`RFC4034-3.2-1`](#rfc4034-3.2-1), [`RFC4034-3.2-2`](#rfc4034-3.2-2), [`RFC4034-3.2-3`](#rfc4034-3.2-3), [`RFC4034-3.2-4`](#rfc4034-3.2-4), [`RFC4034-3.2-5`](#rfc4034-3.2-5), [`RFC4034-3.2-6`](#rfc4034-3.2-6), [`RFC4034-3.2-7`](#rfc4034-3.2-7), [`RFC4034-4-1`](#rfc4034-4-1), [`RFC4034-4.1.1-1`](#rfc4034-4.1.1-1), [`RFC4034-4.1.1-2`](#rfc4034-4.1.1-2), [`RFC4034-4.1.2-1`](#rfc4034-4.1.2-1), [`RFC4034-4.1.2-2`](#rfc4034-4.1.2-2), [`RFC4034-4.1.2-3`](#rfc4034-4.1.2-3), [`RFC4034-4.1.2-4`](#rfc4034-4.1.2-4), [`RFC4034-4.1.2-5`](#rfc4034-4.1.2-5), [`RFC4034-4.1.2-6`](#rfc4034-4.1.2-6), [`RFC4034-4.1.2-7`](#rfc4034-4.1.2-7), [`RFC4034-4.2-1`](#rfc4034-4.2-1), [`RFC4034-5.2-1`](#rfc4034-5.2-1), [`RFC4034-5.2-2`](#rfc4034-5.2-2), [`RFC4034-5.2-3`](#rfc4034-5.2-3), [`RFC4034-5.3-1`](#rfc4034-5.3-1), [`RFC4034-5.3-2`](#rfc4034-5.3-2), [`RFC4034-5.3-3`](#rfc4034-5.3-3), [`RFC4034-5.3-4`](#rfc4034-5.3-4), [`RFC4034-6.3-1`](#rfc4034-6.3-1), [`RFC4034-6.3-2`](#rfc4034-6.3-2), [`RFC4034-A-1`](#rfc4034-a-1), [`RFC4034-A.1.1-1`](#rfc4034-a.1.1-1), [`RFC4034-B-1`](#rfc4034-b-1), [`RFC4034-B-2`](#rfc4034-b-2), [`RFC4034-B-3`](#rfc4034-b-3)

## Figures

- **MUST bar**: Not assessed 59
- **Overview bar**: Not assessed 59

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 59 |
| MUST rows kept (the MUST bar) | 59 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 59 |
| Not built | 0 |
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

Source: [data/rfc/rfc4034.json](../../../data/rfc/rfc4034.json).

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

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC4034-2-1`](#rfc4034-2-1)

The DNSKEY RR is not intended as a record for storing arbitrary public keys and MUST NOT be used to store certificates or public keys that do not directly relate to the DNS infrastructure. (§2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-2-1, so there is no linked test.

### [`RFC4034-2.1.1-1`](#rfc4034-2.1.1-1)

If bit 7 has value 1, then the DNSKEY record holds a DNS zone key, and the DNSKEY RR's owner name MUST be the name of a zone. (§2.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-2.1.1-1, so there is no linked test.

### [`RFC4034-2.1.1-2`](#rfc4034-2.1.1-2)

If bit 7 has value 0, then the DNSKEY record holds some other type of DNS public key and MUST NOT be used to verify RRSIGs that cover RRsets. (§2.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-2.1.1-2, so there is no linked test.

### [`RFC4034-2.1.1-3`](#rfc4034-2.1.1-3)

This flag is only intended to be a hint to zone signing or debugging software as to the intended use of this DNSKEY record; validators MUST NOT alter their behavior during the signature validation process in any way based on the setting of this bit. (§2.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-2.1.1-3, so there is no linked test.

### [`RFC4034-2.1.1-4`](#rfc4034-2.1.1-4)

A DNSKEY RR with the SEP set and the Zone Key flag not set MUST NOT be used to verify RRSIGs that cover RRsets. (§2.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-2.1.1-4, so there is no linked test.

### [`RFC4034-2.1.1-5`](#rfc4034-2.1.1-5)

Bits 0-6 and 8-14 are reserved: these bits MUST have value 0 upon creation of the DNSKEY RR and MUST be ignored upon receipt. (§2.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-2.1.1-5, so there is no linked test.

### [`RFC4034-2.1.2-1`](#rfc4034-2.1.2-1)

The Protocol Field MUST have value 3, and the DNSKEY RR MUST be treated as invalid during signature verification if it is found to be some value other than 3. (§2.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-2.1.2-1, so there is no linked test.

### [`RFC4034-2.2-1`](#rfc4034-2.2-1)

The Flag field MUST be represented as an unsigned decimal integer. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-2.2-1, so there is no linked test.

### [`RFC4034-2.2-2`](#rfc4034-2.2-2)

The Protocol Field MUST be represented as an unsigned decimal integer with a value of 3. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-2.2-2, so there is no linked test.

### [`RFC4034-2.2-3`](#rfc4034-2.2-3)

The Algorithm field MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic as specified in Appendix A.1. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-2.2-3, so there is no linked test.

### [`RFC4034-2.2-4`](#rfc4034-2.2-4)

The Public Key field MUST be represented as a Base64 encoding of the Public Key. (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-2.2-4, so there is no linked test.

### [`RFC4034-3-1`](#rfc4034-3-1)

The RRSIG RR MUST only be used to carry verification material (digital signatures) used to secure DNS operations. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3-1, so there is no linked test.

### [`RFC4034-3-2`](#rfc4034-3-2)

A RRSIG and NSEC (see Section 4) MUST exist for the same name as a CNAME resource record in a signed zone. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3-2, so there is no linked test.

### [`RFC4034-3-3`](#rfc4034-3-3)

An RRSIG RR MUST have the same class as the RRset it covers. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3-3, so there is no linked test.

### [`RFC4034-3-4`](#rfc4034-3-4)

The TTL value of an RRSIG RR MUST match the TTL value of the RRset it covers. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3-4, so there is no linked test.

### [`RFC4034-3.1.3-1`](#rfc4034-3.1.3-1)

The value of the Labels field MUST NOT count either the null (root) label that terminates the owner name or the wildcard label (if present). (§3.1.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.3-1, so there is no linked test.

### [`RFC4034-3.1.3-2`](#rfc4034-3.1.3-2)

The value of the Labels field MUST be less than or equal to the number of labels in the RRSIG owner name. (§3.1.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.3-2, so there is no linked test.

### [`RFC4034-3.1.5-1`](#rfc4034-3.1.5-1)

The RRSIG record MUST NOT be used for authentication prior to the inception date and MUST NOT be used for authentication after the expiration date. (§3.1.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.5-1, so there is no linked test.

### [`RFC4034-3.1.5-2`](#rfc4034-3.1.5-2)

Because of this, all comparisons involving these fields MUST use "Serial number arithmetic", as defined in [RFC1982]. (§3.1.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.5-2, so there is no linked test.

### [`RFC4034-3.1.7-1`](#rfc4034-3.1.7-1)

The Signer's Name field MUST contain the name of the zone of the covered RRset. (§3.1.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.7-1, so there is no linked test.

### [`RFC4034-3.1.7-2`](#rfc4034-3.1.7-2)

A sender MUST NOT use DNS name compression on the Signer's Name field when transmitting a RRSIG RR. (§3.1.7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.7-2, so there is no linked test.

### [`RFC4034-3.1.8.1-1`](#rfc4034-3.1.8.1-1)

Each RR MUST have the same owner name as the RRSIG RR; (§3.1.8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.8.1-1, so there is no linked test.

### [`RFC4034-3.1.8.1-2`](#rfc4034-3.1.8.1-2)

Each RR MUST have the same class as the RRSIG RR; (§3.1.8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.8.1-2, so there is no linked test.

### [`RFC4034-3.1.8.1-3`](#rfc4034-3.1.8.1-3)

Each RR in the RRset MUST have the RR type listed in the RRSIG RR's Type Covered field; (§3.1.8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.8.1-3, so there is no linked test.

### [`RFC4034-3.1.8.1-4`](#rfc4034-3.1.8.1-4)

Each RR in the RRset MUST have the TTL listed in the RRSIG Original TTL Field; (§3.1.8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.8.1-4, so there is no linked test.

### [`RFC4034-3.1.8.1-5`](#rfc4034-3.1.8.1-5)

Any DNS names in the RDATA field of each RR MUST be in canonical form; and (§3.1.8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.8.1-5, so there is no linked test.

### [`RFC4034-3.1.8.1-6`](#rfc4034-3.1.8.1-6)

The RRset MUST be sorted in canonical order. (§3.1.8.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.1.8.1-6, so there is no linked test.

### [`RFC4034-3.2-1`](#rfc4034-3.2-1)

When the mnemonic is not known, the TYPE representation as described in [RFC3597], Section 5, MUST be used. (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.2-1, so there is no linked test.

### [`RFC4034-3.2-2`](#rfc4034-3.2-2)

The Algorithm field value MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic, as specified in Appendix A.1. (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.2-2, so there is no linked test.

### [`RFC4034-3.2-3`](#rfc4034-3.2-3)

The Labels field value MUST be represented as an unsigned decimal integer. (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.2-3, so there is no linked test.

### [`RFC4034-3.2-4`](#rfc4034-3.2-4)

The Original TTL field value MUST be represented as an unsigned decimal integer. (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.2-4, so there is no linked test.

### [`RFC4034-3.2-5`](#rfc4034-3.2-5)

The Signature Expiration Time and Inception Time field values MUST be represented either as an unsigned decimal integer indicating seconds since 1 January 1970 00:00:00 UTC, or in the form YYYYMMDDHHmmSS in UTC, where: (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.2-5, so there is no linked test.

### [`RFC4034-3.2-6`](#rfc4034-3.2-6)

The Key Tag field MUST be represented as an unsigned decimal integer. (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.2-6, so there is no linked test.

### [`RFC4034-3.2-7`](#rfc4034-3.2-7)

The Signer's Name field value MUST be represented as a domain name. (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-3.2-7, so there is no linked test.

### [`RFC4034-4-1`](#rfc4034-4-1)

An RRSIG (see Section 3) and NSEC MUST exist for the same name as does a CNAME resource record in a signed zone. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-4-1, so there is no linked test.

### [`RFC4034-4.1.1-1`](#rfc4034-4.1.1-1)

A sender MUST NOT use DNS name compression on the Next Domain Name field when transmitting an NSEC RR. (§4.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-4.1.1-1, so there is no linked test.

### [`RFC4034-4.1.1-2`](#rfc4034-4.1.1-2)

Owner names of RRsets for which the given zone is not authoritative (such as glue records) MUST NOT be listed in the Next Domain Name unless at least one authoritative RRset exists at the same owner name. (§4.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-4.1.1-2, so there is no linked test.

### [`RFC4034-4.1.2-1`](#rfc4034-4.1.2-1)

Bits representing pseudo-types MUST be clear, as they do not appear in zone data. (§4.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-4.1.2-1, so there is no linked test.

### [`RFC4034-4.1.2-2`](#rfc4034-4.1.2-2)

If encountered, they MUST be ignored upon being read. (§4.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-4.1.2-2, so there is no linked test.

### [`RFC4034-4.1.2-3`](#rfc4034-4.1.2-3)

Blocks with no types present MUST NOT be included. (§4.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-4.1.2-3, so there is no linked test.

### [`RFC4034-4.1.2-4`](#rfc4034-4.1.2-4)

Trailing zero octets in the bitmap MUST be omitted. (§4.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-4.1.2-4, so there is no linked test.

### [`RFC4034-4.1.2-5`](#rfc4034-4.1.2-5)

Trailing zero octets not specified MUST be interpreted as zero octets. (§4.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-4.1.2-5, so there is no linked test.

### [`RFC4034-4.1.2-6`](#rfc4034-4.1.2-6)

Bits corresponding to the delegation NS RRset and the RR types for which the parent zone has authoritative data MUST be set; bits corresponding to any non-NS RRset for which the parent is not authoritative MUST be clear. (§4.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-4.1.2-6, so there is no linked test.

### [`RFC4034-4.1.2-7`](#rfc4034-4.1.2-7)

A zone MUST NOT include an NSEC RR for any domain name that only holds glue records. (§4.1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-4.1.2-7, so there is no linked test.

### [`RFC4034-4.2-1`](#rfc4034-4.2-1)

When the mnemonic is not known, the TYPE representation described in [RFC3597], Section 5, MUST be used. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-4.2-1, so there is no linked test.

### [`RFC4034-5.2-1`](#rfc4034-5.2-1)

The DNSKEY RR referred to in the DS RR MUST be a DNSSEC zone key. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-5.2-1, so there is no linked test.

### [`RFC4034-5.2-2`](#rfc4034-5.2-2)

The DNSKEY RR Flags MUST have Flags bit 7 set. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-5.2-2, so there is no linked test.

### [`RFC4034-5.2-3`](#rfc4034-5.2-3)

If the DNSKEY flags do not indicate a DNSSEC zone key, the DS RR (and the DNSKEY RR it references) MUST NOT be used in the validation process. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-5.2-3, so there is no linked test.

### [`RFC4034-5.3-1`](#rfc4034-5.3-1)

The Key Tag field MUST be represented as an unsigned decimal integer. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-5.3-1, so there is no linked test.

### [`RFC4034-5.3-2`](#rfc4034-5.3-2)

The Algorithm field MUST be represented either as an unsigned decimal integer or as an algorithm mnemonic specified in Appendix A.1. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-5.3-2, so there is no linked test.

### [`RFC4034-5.3-3`](#rfc4034-5.3-3)

The Digest Type field MUST be represented as an unsigned decimal integer. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-5.3-3, so there is no linked test.

### [`RFC4034-5.3-4`](#rfc4034-5.3-4)

The Digest MUST be represented as a sequence of case-insensitive hexadecimal digits. (§5.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-5.3-4, so there is no linked test.

### [`RFC4034-6.3-1`](#rfc4034-6.3-1)

Therefore, if an implementation detects duplicate RRs when putting the RRset in canonical form, it MUST treat this as a protocol error. (§6.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-6.3-1, so there is no linked test.

### [`RFC4034-6.3-2`](#rfc4034-6.3-2)

If the implementation chooses to handle this protocol error in the spirit of the robustness principle (being liberal in what it accepts), it MUST remove all but one of the duplicate RR(s) for the purposes of calculating the canonical form of the RRset. (§6.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-6.3-2, so there is no linked test.

### [`RFC4034-A-1`](#rfc4034-a-1)

A DNSSEC aware resolver or name server MUST implement all MANDATORY algorithms. (§A)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-A-1, so there is no linked test.

### [`RFC4034-A.1.1-1`](#rfc4034-a.1.1-1)

The public key area in the DNSKEY RR and the signature area in the RRSIG RR begin with a wire encoded domain name, which MUST NOT be compressed. (§A.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-A.1.1-1, so there is no linked test.

### [`RFC4034-B-1`](#rfc4034-b-1)

Implementations MUST NOT assume that the key tag uniquely identifies a DNSKEY RR. (§B)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-B-1, so there is no linked test.

### [`RFC4034-B-2`](#rfc4034-b-2)

It is not necessary to use the following reference code verbatim, but the numerical value of the Key Tag MUST be identical to what the reference implementation would generate for the same input. (§B)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-B-2, so there is no linked test.

### [`RFC4034-B-3`](#rfc4034-b-3)

Key Tags MUST be calculated using the algorithm described here rather than the ones complement checksum. (§B)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC4034-B-3, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 1 | reviewed | not stated |
| `2.1` | not stated | 0 | reviewed | not stated |
| `2.1.1` | not stated | 5 | reviewed | not stated |
| `2.1.2` | not stated | 1 | reviewed | not stated |
| `2.1.3` | not stated | 0 | reviewed | not stated |
| `2.1.4` | not stated | 0 | reviewed | not stated |
| `2.1.5` | not stated | 0 | reviewed | not stated |
| `2.2` | not stated | 4 | reviewed | not stated |
| `2.3` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 4 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.1.1` | not stated | 0 | reviewed | not stated |
| `3.1.2` | not stated | 0 | reviewed | not stated |
| `3.1.3` | not stated | 2 | reviewed | not stated |
| `3.1.4` | not stated | 0 | reviewed | not stated |
| `3.1.5` | not stated | 2 | reviewed | not stated |
| `3.1.6` | not stated | 0 | reviewed | not stated |
| `3.1.7` | not stated | 2 | reviewed | not stated |
| `3.1.8` | not stated | 0 | reviewed | not stated |
| `3.1.8.1` | not stated | 6 | reviewed | not stated |
| `3.2` | not stated | 7 | reviewed | not stated |
| `3.3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `4.1.1` | not stated | 2 | reviewed | not stated |
| `4.1.2` | not stated | 7 | reviewed | not stated |
| `4.1.3` | not stated | 0 | reviewed | not stated |
| `4.2` | not stated | 1 | reviewed | not stated |
| `4.3` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.1.1` | not stated | 0 | reviewed | not stated |
| `5.1.2` | not stated | 0 | reviewed | not stated |
| `5.1.3` | not stated | 0 | reviewed | not stated |
| `5.1.4` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 3 | reviewed | not stated |
| `5.3` | not stated | 4 | reviewed | not stated |
| `5.4` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |
| `6.3` | not stated | 2 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `10.1` | not stated | 0 | reviewed | not stated |
| `10.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 1 | reviewed | not stated |
| `A.1` | not stated | 0 | reviewed | not stated |
| `A.1.1` | not stated | 1 | reviewed | not stated |
| `A.2` | not stated | 0 | reviewed | not stated |
| `B` | not stated | 3 | reviewed | not stated |
| `B.1` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 4034 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 4034, so its obligations are stated where they were written.
