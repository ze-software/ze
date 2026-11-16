# RFC 1035 - Domain Names - Implementation and Specification

DNS message behavior. Every requirement listed for RFC 1035, the tests linked to it, and their recorded reviews. This summary is excluded from automatic compliance checks.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 25 | of 27 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 27 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 27 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 27 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 26 | 26 of 85 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 2 | of 27 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 27 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 27 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 27 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 27 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 27 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level), not checked | 27 | of 33 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements listed in this summary. This RFC is excluded from automatic checks (backlog), so these requirements are not checked. The counts above show what the summary records. |

The 9 counts marked as a part above are the whole of the 27 mandatory requirements: they add up to 27. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | DNS message behavior |
| Automatic checks | Excluded (backlog) |
| Requirements | 33 |
| Mandatory requirements (MUST level), not checked | 27 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 2 |
| Nightly-only evidence | 0 |
| Test tags | 85 |
| Tests linked to requirements | 85 |
| Recorded audit verdicts | 0 |
| Fault test records | 26 |
| Summary | `rfc/short/rfc1035.md` |
| Requirement table | `rfc/requirements/rfc1035.md` |
| RFC text | `rfc/full/rfc1035.txt` |

## Automatic checks

Excluded from automatic checks (backlog, the requirements have not been extracted from the document yet; this is work owed rather than a decision): Domain Names: Implementation and Specification. Re-authored 2026-07-30 and it now declares 27 MUST-level obligations read from the indicative prose of a 1987 document (0 capitalised keywords, 23 lowercase must), so this is no longer an empty checklist. It is not enrolled because the obligations are not all proven and the unproven ones need an owner ruling, not an implementer's annotation. The obligation with no code path in Ze is zone transfer: Ze performs none, and the owner ruled RFC 1035 out of scope on 2026-08-18, so that work is not to be started. The 512-octet UDP bound and the TC bit ARE enforced -- send calls Msg.Truncate(udpReplyLimit(r)) for a datagram reply in internal/core/dnsserver/handler.go, and udpReplyLimit holds the Section 2.3.4 floor while letting an RFC 6891 Section 6.2.3 OPT record raise it. An unsupported inverse query DOES draw Not Implemented: Authoritative branches on the opcode before any zone lookup, in the same file. It was the one obligation the 73-section walk found OUTSIDE the summary's declared scope and added to it. The response TTL is deliberately not raised to the SOA MINIMUM -- RFC 2308 Section 4 withdrew that rule, hdr in internal/plugins/geodns/server.go applies no floor, and TestRFC2308_NoZoneWideTTLFloor holds the decision. About 6 requirements admit only a positive polarity because miekg/dns owns the wire codec and no Ze-side change can break them. Escalated for scoping per OR-1b.

## Published support status

**Area:** DNS message behavior

**Test verification:** 12 of 27 MUST proven

**Evidence:** functional 1, unit 11

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 25 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 2 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level), not checked** | **27** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (25):** [`RFC1035-2.3.3-1`](#rfc1035-2.3.3-1), [`RFC1035-2.3.3-2`](#rfc1035-2.3.3-2), [`RFC1035-2.3.4-1`](#rfc1035-2.3.4-1), [`RFC1035-2.3.4-2`](#rfc1035-2.3.4-2), [`RFC1035-3.1-1`](#rfc1035-3.1-1), [`RFC1035-3.1-2`](#rfc1035-3.1-2), [`RFC1035-3.1-3`](#rfc1035-3.1-3), [`RFC1035-3.1-4`](#rfc1035-3.1-4), [`RFC1035-3.1-5`](#rfc1035-3.1-5), [`RFC1035-3.1-6`](#rfc1035-3.1-6), [`RFC1035-4.1.1-1`](#rfc1035-4.1.1-1), [`RFC1035-4.1.1-2`](#rfc1035-4.1.1-2), [`RFC1035-4.1.1-3`](#rfc1035-4.1.1-3), [`RFC1035-4.1.3-1`](#rfc1035-4.1.3-1), [`RFC1035-4.1.3-2`](#rfc1035-4.1.3-2), [`RFC1035-4.1.4-1`](#rfc1035-4.1.4-1), [`RFC1035-4.1.4-2`](#rfc1035-4.1.4-2), [`RFC1035-4.1.4-3`](#rfc1035-4.1.4-3), [`RFC1035-4.1.4-4`](#rfc1035-4.1.4-4), [`RFC1035-4.1.4-5`](#rfc1035-4.1.4-5), [`RFC1035-4.2.1-1`](#rfc1035-4.2.1-1), [`RFC1035-4.2.1-2`](#rfc1035-4.2.1-2), [`RFC1035-4.2.1-3`](#rfc1035-4.2.1-3), [`RFC1035-4.2.2-1`](#rfc1035-4.2.2-1), [`RFC1035-6.4-1`](#rfc1035-6.4-1)

**No test and no annotation (2):** [`RFC1035-3.3.13-1`](#rfc1035-3.3.13-1), [`RFC1035-4.2-1`](#rfc1035-4.2-1)

## Figures

- **MUST bar**: Ze verified 12, Tests awaiting verification 13, Not assessed 2
- **Overview bar**: Ze MUST verified 12, Tests awaiting verification 13, Not assessed 8

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 27 |
| MUST rows kept (the MUST bar) | 27 |
| Ze verified | 12 |
| OS verified | 0 |
| Tests awaiting verification | 13 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 2 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 6 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc1035.json](../../../data/rfc/rfc1035.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1035-2.3.3-1` | For all parts of the DNS that are part of the official protocol, all comparisons between character strings (e.g., labels, domain names, etc.) are done in a case-insensitive manner (§2.3.3) | MUST | 2.3.3 - Character Case | **positive:** `unit/verify` [`TestRFC1035ResolverNameCase`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L45). **positive:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L54). **negative:** `unit/verify` [`TestRFC1035ResolverNameCase`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L58). **negative:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L97) |
| `RFC1035-2.3.3-2` | Loss of case sensitive data must be minimized (§2.3.3) | MUST | 2.3.3 - Character Case | **positive:** `unit/verify` [`TestRFC1035_QueryNameCasePreservedInTheReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L133). **negative:** `unit/verify` [`TestRFC1035_QueryNameCasePreservedInTheReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L148) |
| `RFC1035-2.3.4-1` | TTL positive values of a signed 32 bit number. (§2.3.4) | MUST | 2.3.4 - Size limits | **positive:** `unit/verify` [`TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L144). **negative:** `unit/verify` [`TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L159) |
| `RFC1035-2.3.4-2` | UDP messages 512 octets or less (§2.3.4) | MUST | 2.3.4 - Size limits | **positive:** `unit/verify` [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L102). **negative:** `unit/verify` [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L132) |
| `RFC1035-3.1-1` | Each label is represented as a one octet length field followed by that number of octets (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L179). **negative:** `unit/verify` [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L204) |
| `RFC1035-3.1-2` | Since every domain name ends with the null label of the root, a domain name is terminated by a length byte of zero (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L186). **negative:** `unit/verify` [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L192) |
| `RFC1035-3.1-3` | The high order two bits of every length octet must be zero, and the remaining six bits of the length field limit the label to 63 octets or less (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035_ConfiguredLabelBoundedTo63Octets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L57). **negative:** `unit/verify` [`TestRFC1035_ConfiguredLabelBoundedTo63Octets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L73). **positive:** `functional/verify` [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L24). **negative:** `functional/verify` [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L67) |
| `RFC1035-3.1-4` | To simplify implementations, the total length of a domain name (i.e., label octets and label length octets) is restricted to 255 octets or less. (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035_ConfiguredNameBoundedTo255WireOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L134). **negative:** `unit/verify` [`TestRFC1035_ConfiguredNameBoundedTo255WireOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L115). **positive:** `functional/verify` [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L43). **negative:** `functional/verify` [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L65) |
| `RFC1035-3.1-5` | Name servers and resolvers must compare labels in a case-insensitive manner (i.e., A=a), assuming ASCII with zero parity (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035ResolverNameCase`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L35). **positive:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L59). **negative:** `unit/verify` [`TestRFC1035ResolverNameCase`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L70). **negative:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L105) |
| `RFC1035-3.1-6` | Non-alphabetic codes must match exactly (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L87). **negative:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L110) |
| `RFC1035-3.3.13-1` | Whenever a RR is sent in a response to a query, the TTL field is set to the maximum of the TTL field from the RR and the MINIMUM field in the appropriate SOA (§3.3.13) | MUST | 3.3.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-4.1.1-1` | Must be zero in all queries and responses. (§4.1.1) | MUST | 4.1.1 - Header section format | **positive:** `unit/verify` [`TestRFC1035ResolverHeaderZ`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L94). **positive:** `unit/verify` [`TestRFC1035_ReservedZFieldIsZero`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L69). **negative:** `unit/verify` [`TestRFC1035ResolverHeaderZ`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L105). **negative:** `unit/verify` [`TestRFC1035_ReservedZFieldIsZero`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L102) |
| `RFC1035-4.1.1-2` | AA Authoritative Answer - this bit is valid in responses, and specifies that the responding name server is an authority for the domain name in question section. (§4.1.1) | MUST | 4.1.1 - Header section format | **positive:** `unit/verify` [`TestRFC1035ResolverClearsAA`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L130). **positive:** `unit/verify` [`TestRFC1035_AuthoritativeAnswerBitOnEveryReply`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L137). **negative:** `unit/verify` [`TestRFC1035ResolverClearsAA`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L137). **negative:** `unit/verify` [`TestRFC1035_AuthoritativeAnswerBitOnEveryReply`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L162). **negative:** `unit/verify` [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L117). **negative:** `unit/verify` [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L253) |
| `RFC1035-4.1.1-3` | 3 Name Error - Meaningful only for responses from an authoritative name server, this code signifies that the domain name referenced in the query does not exist. (§4.1.1) | MUST | 4.1.1 - Header section format | **positive:** `unit/verify` [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L102). **positive:** `unit/verify` [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L237). **negative:** `unit/verify` [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L107). **negative:** `unit/verify` [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L242) |
| `RFC1035-4.1.3-1` | TTL a 32 bit unsigned integer that specifies the time interval (in seconds) that the resource record may be cached before it should be discarded. (§4.1.3) | MUST | 4.1.3 - Resource record format | **positive:** `unit/verify` [`TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L109). **negative:** `unit/verify` [`TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L127) |
| `RFC1035-4.1.3-2` | RDLENGTH an unsigned 16 bit integer that specifies the length in octets of the RDATA field. (§4.1.3) | MUST | 4.1.3 - Resource record format | **positive:** `unit/verify` [`TestRFC1035_RDLengthCountsTheRDataOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L181). **negative:** `unit/verify` [`TestRFC1035_RDLengthCountsTheRDataOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L201) |
| `RFC1035-4.1.4-1` | This allows a pointer to be distinguished from a label, since the label must begin with two zero bits because labels are restricted to 63 octets or less. (§4.1.4) | MUST | 4.1.4 - Message compression | **positive:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L157). **positive:** `unit/verify` [`TestRFC1035_LabelLengthOctetBeginsWithTwoZeroBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L503). **negative:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L232). **negative:** `unit/verify` [`TestRFC1035_LabelLengthOctetBeginsWithTwoZeroBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L517) |
| `RFC1035-4.1.4-2` | The OFFSET field specifies an offset from the start of the message (i.e., the first octet of the ID field in the domain header). (§4.1.4) | MUST | 4.1.4 - Message compression | **positive:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L176). **negative:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L236) |
| `RFC1035-4.1.4-3` | Pointers can only be used for occurances of a domain name where the format is not class specific (§4.1.4) | MUST | 4.1.4 - Message compression | **positive:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L215). **negative:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L239) |
| `RFC1035-4.1.4-4` | If a domain name is contained in a part of the message subject to a length field (such as the RDATA section of an RR), and compression is used, the length of the compressed name is used in the length calculation, rather than the length of the expanded name. (§4.1.4) | MUST | 4.1.4 - Message compression | **positive:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L191). **negative:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L242) |
| `RFC1035-4.1.4-5` | However all programs are required to understand arriving messages that contain pointers. (§4.1.4) | REQUIRED | 4.1.4 - Message compression | **positive:** `unit/verify` [`TestRFC1035ResolverUnderstandsPointers`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L158). **positive:** `unit/verify` [`TestRFC1035_InboundCompressionPointerUnderstood`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L352). **negative:** `unit/verify` [`TestRFC1035ResolverDiscardsPointerLoop`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L213). **negative:** `unit/verify` [`TestRFC1035_InboundCompressionPointerUnderstood`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L372) |
| `RFC1035-4.2-1` | Zone refresh activities must use virtual circuits because of the need for reliable transfer (§4.2) | MUST | 4.2 - Transport preamble | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-4.2.1-1` | Messages carried by UDP are restricted to 512 bytes (not counting the IP or UDP headers). (§4.2.1) | MUST | 4.2.1 - UDP usage | **positive:** `unit/verify` [`TestRFC1035ResolverDatagramBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L264). **positive:** `unit/verify` [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L97). **positive:** `unit/verify` [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L83). **negative:** `unit/verify` [`TestRFC1035ResolverDatagramBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L297). **negative:** `unit/verify` [`TestRFC1035_UDPBoundFollowsAdvertisedEDNSSize`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L164) |
| `RFC1035-4.2.1-2` | Longer messages are truncated and the TC bit is set in the header (§4.2.1) | MUST | 4.2.1 - UDP usage | **positive:** `unit/verify` [`TestRFC1035ResolverDatagramBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L271). **positive:** `unit/verify` [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L107). **positive:** `unit/verify` [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L92). **negative:** `unit/verify` [`TestRFC1035ResolverDatagramBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L282). **negative:** `unit/verify` [`TestRFC1035_StreamTransportNotTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L195). **negative:** `unit/verify` [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L127). **negative:** `unit/verify` [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L106) |
| `RFC1035-4.2.1-3` | Messages sent using UDP user server port 53 (decimal) (§4.2.1) | MUST | 4.2.1 - UDP usage | **positive:** `unit/verify` [`TestRFC1035ResolverForwardsToServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L348). **positive:** `unit/verify` [`TestRFC1035ResolverListensOnServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L327). **positive:** `unit/verify` [`TestRFC1035_DNSTransportsUseServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc1035_port_test.go#L26). **negative:** `unit/verify` [`TestRFC1035ResolverForwardsToServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L359). **negative:** `unit/verify` [`TestRFC1035_DNSTransportsUseServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc1035_port_test.go#L49) |
| `RFC1035-4.2.2-1` | Messages sent over TCP connections use server port 53 (decimal). The message is prefixed with a two byte length field which gives the message length, excluding the two byte length field. (§4.2.2) | MUST | 4.2.2 - TCP usage | **positive:** `unit/verify` [`TestRFC1035ResolverStreamFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L404). **positive:** `unit/verify` [`TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L428). **negative:** `unit/verify` [`TestRFC1035ResolverStreamFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L390). **negative:** `unit/verify` [`TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L452) |
| `RFC1035-6.4-1` | While inverse query support is optional, all name servers must be at least able to return the error response (§6.4) | MUST | 6.4 - Inverse queries (Optional) | **positive:** `unit/verify` [`TestRFC1035ResolverInverseQueryNotImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L441). **positive:** `unit/verify` [`TestRFC1035_UnsupportedOpcodeReturnsNotImplemented`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L241). **negative:** `unit/verify` [`TestRFC1035ResolverInverseQueryNotImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L428). **negative:** `unit/verify` [`TestRFC1035_QueryOpcodeAnsweredNormally`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L290) |
| `RFC1035-2.3.1-1` | The labels must follow the rules for ARPANET host names. They must start with a letter, end with a letter or digit, and have as interior characters only letters, digits, and hyphen. (§2.3.1) | SHOULD | 2.3.1 - Preferred name syntax | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-2.3.3-3` | When data enters the domain system, its original case should be preserved whenever possible (§2.3.3) | SHOULD | 2.3.3 - Character Case | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-2.3.3-4` | However, future additions beyond current usage may need to use the full binary octet capabilities in names, so attempts to store domain names in 7-bit ASCII or use of special bytes to terminate labels, etc., should be avoided. (§2.3.3) | SHOULD NOT | 2.3.3 - Character Case | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-3.1-7` | Although labels can contain any 8 bit values in octets that make up a label, it is strongly recommended that labels follow the preferred syntax described elsewhere in this memo (§3.1) | RECOMMENDED | 3.1 - Name space definitions: the wire form of a domain name | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-3.3.13-2` | Note that this use of MINIMUM should occur when the RRs are copied into the response and not when the zone is loaded from a master file or via a zone transfer. (§3.3.13) | SHOULD | 3.3.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-4.1.3-3` | Zero values are interpreted to mean that the RR can only be used for the transaction in progress, and should not be cached. (§4.1.3) | SHOULD NOT | 4.1.3 - Resource record format | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1035-3.3.13-1`](#rfc1035-3.3.13-1) Whenever a RR is sent in a response to a query, the TTL field is set to the maximum of the TTL field from the RR and the MINIMUM field in the appropriate SOA (§3.3.13) | no test | no test carries this requirement id |
| [`RFC1035-4.2-1`](#rfc1035-4.2-1) Zone refresh activities must use virtual circuits because of the need for reliable transfer (§4.2) | no test | no test carries this requirement id |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC1035-2.3.3-1`](#rfc1035-2.3.3-1)

For all parts of the DNS that are part of the official protocol, all comparisons between character strings (e.g., labels, domain names, etc.) are done in a case-insensitive manner (§2.3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035ResolverNameCase`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L58) | unit/verify | mutant, verified |
| negative | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L97) | unit/verify | not verified |
| positive | [`TestRFC1035ResolverNameCase`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L45) | unit/verify | mutant, verified |
| positive | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L54) | unit/verify | not verified |

### [`RFC1035-2.3.3-2`](#rfc1035-2.3.3-2)

Loss of case sensitive data must be minimized (§2.3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_QueryNameCasePreservedInTheReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L148) | unit/verify | not verified |
| positive | [`TestRFC1035_QueryNameCasePreservedInTheReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L133) | unit/verify | not verified |

### [`RFC1035-2.3.4-1`](#rfc1035-2.3.4-1)

TTL positive values of a signed 32 bit number. (§2.3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L159) | unit/verify | not verified |
| positive | [`TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L144) | unit/verify | not verified |

### [`RFC1035-2.3.4-2`](#rfc1035-2.3.4-2)

UDP messages 512 octets or less (§2.3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L132) | unit/verify | not verified |
| positive | [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L102) | unit/verify | not verified |

### [`RFC1035-3.1-1`](#rfc1035-3.1-1)

Each label is represented as a one octet length field followed by that number of octets (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L204) | unit/verify | not verified |
| positive | [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L179) | unit/verify | not verified |

### [`RFC1035-3.1-2`](#rfc1035-3.1-2)

Since every domain name ends with the null label of the root, a domain name is terminated by a length byte of zero (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L192) | unit/verify | not verified |
| positive | [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L186) | unit/verify | not verified |

### [`RFC1035-3.1-3`](#rfc1035-3.1-3)

The high order two bits of every length octet must be zero, and the remaining six bits of the length field limit the label to 63 octets or less (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_ConfiguredLabelBoundedTo63Octets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L73) | unit/verify | not verified |
| negative | [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L67) | functional/verify | not verified |
| positive | [`TestRFC1035_ConfiguredLabelBoundedTo63Octets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L57) | unit/verify | not verified |
| positive | [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L24) | functional/verify | revert, verified |

### [`RFC1035-3.1-4`](#rfc1035-3.1-4)

To simplify implementations, the total length of a domain name (i.e., label octets and label length octets) is restricted to 255 octets or less. (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_ConfiguredNameBoundedTo255WireOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L115) | unit/verify | not verified |
| negative | [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L65) | functional/verify | not verified |
| positive | [`TestRFC1035_ConfiguredNameBoundedTo255WireOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L134) | unit/verify | not verified |
| positive | [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L43) | functional/verify | not verified |

### [`RFC1035-3.1-5`](#rfc1035-3.1-5)

Name servers and resolvers must compare labels in a case-insensitive manner (i.e., A=a), assuming ASCII with zero parity (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035ResolverNameCase`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L70) | unit/verify | mutant, verified |
| negative | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L105) | unit/verify | not verified |
| positive | [`TestRFC1035ResolverNameCase`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L35) | unit/verify | mutant, verified |
| positive | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L59) | unit/verify | not verified |

### [`RFC1035-3.1-6`](#rfc1035-3.1-6)

Non-alphabetic codes must match exactly (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L110) | unit/verify | not verified |
| positive | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L87) | unit/verify | not verified |

### [`RFC1035-3.3.13-1`](#rfc1035-3.3.13-1)

Whenever a RR is sent in a response to a query, the TTL field is set to the maximum of the TTL field from the RR and the MINIMUM field in the appropriate SOA (§3.3.13)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1035-3.3.13-1, so there is no linked test.

### [`RFC1035-4.1.1-1`](#rfc1035-4.1.1-1)

Must be zero in all queries and responses. (§4.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_ReservedZFieldIsZero`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L102) | unit/verify | not verified |
| negative | [`TestRFC1035ResolverHeaderZ`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L105) | unit/verify | mutant, verified |
| positive | [`TestRFC1035_ReservedZFieldIsZero`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L69) | unit/verify | not verified |
| positive | [`TestRFC1035ResolverHeaderZ`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L94) | unit/verify | mutant, verified |

### [`RFC1035-4.1.1-2`](#rfc1035-4.1.1-2)

AA Authoritative Answer - this bit is valid in responses, and specifies that the responding name server is an authority for the domain name in question section. (§4.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_AuthoritativeAnswerBitOnEveryReply`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L162) | unit/verify | not verified |
| negative | [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L253) | unit/verify | not verified |
| negative | [`TestRFC1035ResolverClearsAA`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L137) | unit/verify | mutant, verified |
| negative | [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L117) | unit/verify | not verified |
| positive | [`TestRFC1035_AuthoritativeAnswerBitOnEveryReply`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L137) | unit/verify | not verified |
| positive | [`TestRFC1035ResolverClearsAA`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L130) | unit/verify | mutant, verified |

### [`RFC1035-4.1.1-3`](#rfc1035-4.1.1-3)

3 Name Error - Meaningful only for responses from an authoritative name server, this code signifies that the domain name referenced in the query does not exist. (§4.1.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L242) | unit/verify | not verified |
| negative | [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L107) | unit/verify | not verified |
| positive | [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L237) | unit/verify | not verified |
| positive | [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L102) | unit/verify | not verified |

### [`RFC1035-4.1.3-1`](#rfc1035-4.1.3-1)

TTL a 32 bit unsigned integer that specifies the time interval (in seconds) that the resource record may be cached before it should be discarded. (§4.1.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L127) | unit/verify | not verified |
| positive | [`TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L109) | unit/verify | not verified |

### [`RFC1035-4.1.3-2`](#rfc1035-4.1.3-2)

RDLENGTH an unsigned 16 bit integer that specifies the length in octets of the RDATA field. (§4.1.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_RDLengthCountsTheRDataOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L201) | unit/verify | not verified |
| positive | [`TestRFC1035_RDLengthCountsTheRDataOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L181) | unit/verify | not verified |

### [`RFC1035-4.1.4-1`](#rfc1035-4.1.4-1)

This allows a pointer to be distinguished from a label, since the label must begin with two zero bits because labels are restricted to 63 octets or less. (§4.1.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L232) | unit/verify | revert, verified |
| negative | [`TestRFC1035_LabelLengthOctetBeginsWithTwoZeroBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L517) | unit/verify | revert, verified |
| positive | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L157) | unit/verify | revert, verified |
| positive | [`TestRFC1035_LabelLengthOctetBeginsWithTwoZeroBits`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L503) | unit/verify | revert, verified |

### [`RFC1035-4.1.4-2`](#rfc1035-4.1.4-2)

The OFFSET field specifies an offset from the start of the message (i.e., the first octet of the ID field in the domain header). (§4.1.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L236) | unit/verify | not verified |
| positive | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L176) | unit/verify | not verified |

### [`RFC1035-4.1.4-3`](#rfc1035-4.1.4-3)

Pointers can only be used for occurances of a domain name where the format is not class specific (§4.1.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L239) | unit/verify | not verified |
| positive | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L215) | unit/verify | not verified |

### [`RFC1035-4.1.4-4`](#rfc1035-4.1.4-4)

If a domain name is contained in a part of the message subject to a length field (such as the RDATA section of an RR), and compression is used, the length of the compressed name is used in the length calculation, rather than the length of the expanded name. (§4.1.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L242) | unit/verify | not verified |
| positive | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L191) | unit/verify | not verified |

### [`RFC1035-4.1.4-5`](#rfc1035-4.1.4-5)

However all programs are required to understand arriving messages that contain pointers. (§4.1.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035ResolverDiscardsPointerLoop`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L213) | unit/verify | mutant, verified |
| negative | [`TestRFC1035_InboundCompressionPointerUnderstood`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L372) | unit/verify | not verified |
| positive | [`TestRFC1035ResolverUnderstandsPointers`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L158) | unit/verify | mutant, verified |
| positive | [`TestRFC1035_InboundCompressionPointerUnderstood`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L352) | unit/verify | not verified |

### [`RFC1035-4.2-1`](#rfc1035-4.2-1)

Zone refresh activities must use virtual circuits because of the need for reliable transfer (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC1035-4.2-1, so there is no linked test.

### [`RFC1035-4.2.1-1`](#rfc1035-4.2.1-1)

Messages carried by UDP are restricted to 512 bytes (not counting the IP or UDP headers). (§4.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_UDPBoundFollowsAdvertisedEDNSSize`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L164) | unit/verify | not verified |
| negative | [`TestRFC1035ResolverDatagramBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L297) | unit/verify | mutant, verified |
| positive | [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L97) | unit/verify | not verified |
| positive | [`TestRFC1035ResolverDatagramBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L264) | unit/verify | mutant, verified |
| positive | [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L83) | unit/verify | not verified |

### [`RFC1035-4.2.1-2`](#rfc1035-4.2.1-2)

Longer messages are truncated and the TC bit is set in the header (§4.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_StreamTransportNotTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L195) | unit/verify | not verified |
| negative | [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L127) | unit/verify | not verified |
| negative | [`TestRFC1035ResolverDatagramBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L282) | unit/verify | mutant, verified |
| negative | [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L106) | unit/verify | not verified |
| positive | [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L107) | unit/verify | not verified |
| positive | [`TestRFC1035ResolverDatagramBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L271) | unit/verify | mutant, verified |
| positive | [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L92) | unit/verify | not verified |

### [`RFC1035-4.2.1-3`](#rfc1035-4.2.1-3)

Messages sent using UDP user server port 53 (decimal) (§4.2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_DNSTransportsUseServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc1035_port_test.go#L49) | unit/verify | not verified |
| negative | [`TestRFC1035ResolverForwardsToServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L359) | unit/verify | mutant, verified |
| positive | [`TestRFC1035_DNSTransportsUseServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc1035_port_test.go#L26) | unit/verify | not verified |
| positive | [`TestRFC1035ResolverForwardsToServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L348) | unit/verify | mutant, verified |
| positive | [`TestRFC1035ResolverListensOnServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L327) | unit/verify | mutant, verified |

### [`RFC1035-4.2.2-1`](#rfc1035-4.2.2-1)

Messages sent over TCP connections use server port 53 (decimal). The message is prefixed with a two byte length field which gives the message length, excluding the two byte length field. (§4.2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035ResolverStreamFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L390) | unit/verify | mutant, verified |
| negative | [`TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L452) | unit/verify | not verified |
| positive | [`TestRFC1035ResolverStreamFraming`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L404) | unit/verify | mutant, verified |
| positive | [`TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L428) | unit/verify | not verified |

### [`RFC1035-6.4-1`](#rfc1035-6.4-1)

While inverse query support is optional, all name servers must be at least able to return the error response (§6.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC1035_QueryOpcodeAnsweredNormally`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L290) | unit/verify | not verified |
| negative | [`TestRFC1035ResolverInverseQueryNotImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L428) | unit/verify | mutant, verified |
| positive | [`TestRFC1035_UnsupportedOpcodeReturnsNotImplemented`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L241) | unit/verify | not verified |
| positive | [`TestRFC1035ResolverInverseQueryNotImplemented`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc1035_resolver_test.go#L441) | unit/verify | mutant, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-rfcgate-4-ledger phase 6 |
| Signed off | 2026-07-30 |
| Register | prose |
| Source | rfc/full/rfc1035.txt |
| Source fingerprint | b8efa09ba52db966 |
| Record | rfc/extraction/rfc1035.json |
| Mapped sentences | 11 |
| Declined as scope | 20 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Network Working Group header, obsoletes/updates block and the table of contents. |
| `1` | STATUS OF THIS MEMO | 0 | skipped (front-matter) | STATUS OF THIS MEMO. Distribution statement and the note that this RFC obsoletes RFC 882 and RFC 883. |
| `2` | INTRODUCTION heading, with no body of its own | 0 | reviewed | INTRODUCTION heading, with no body of its own. |
| `2.1` | not stated | 0 | reviewed | Overview of the three-part domain system (name space, name servers, resolvers). Descriptive. |
| `2.2` | not stated | 1 | reviewed | Common configurations: the deployment shapes a resolver and a name server can take. Its one site is a characterisation, excluded below. |
| `2.3` | Conventions | 1 | reviewed | Conventions. The section preamble is its one site, an applicability statement excluded below; the conventions themselves are in 2.3.1 to 2.3.4. |
| `2.3.1` | Preferred name syntax | 3 | reviewed | Preferred name syntax. Two sites map to the single [SHOULD] row that carries both halves; the third restates section 3.1's 63-octet limit. |
| `2.3.2` | not stated | 0 | reviewed | Data Transmission Order: the big-endian octet and bit order of every multi-octet field. Stated as a description of the diagrams, with no modal verb, so the prose scan finds no site. Ze inherits the encoding from miekg/dns and adds nothing. |
| `2.3.3` | Character Case | 1 | reviewed | Character Case. Its one site is the case-preservation obligation; the case-insensitive comparison rule and the two 'should' clauses are read from the surrounding prose, which carries no scanned keyword. |
| `2.3.4` | Size limits | 0 | reviewed | Size limits. Stated as a bare list ('labels 63 octets or less', 'names 255 octets or less', 'TTL positive values of a signed 32 bit number', 'UDP messages 512 octets or less') with no modal verb, so the scan finds no site here even though two of the four are declared as requirements. |
| `3` | DOMAIN NAME SPACE AND RR DEFINITIONS heading | 0 | reviewed | DOMAIN NAME SPACE AND RR DEFINITIONS heading. |
| `3.1` | Name space definitions: the wire form of a domain name | 3 | reviewed | Name space definitions: the wire form of a domain name. Three sites map to the length-octet, case-folding and exact-match rules; the label/terminator/total-length statements and the 'strongly recommended' clause are indicative prose. |
| `3.2` | RR definitions heading | 0 | reviewed | RR definitions heading. |
| `3.2.1` | not stated | 0 | reviewed | Format: the NAME/TYPE/CLASS/TTL/RDLENGTH/RDATA layout, given as a diagram plus field descriptions with no modal verb. |
| `3.2.2` | TYPE values: the registry table | 0 | reviewed | TYPE values: the registry table. |
| `3.2.3` | QTYPE values: the registry table | 0 | reviewed | QTYPE values: the registry table. |
| `3.2.4` | CLASS values: the registry table | 0 | reviewed | CLASS values: the registry table. |
| `3.2.5` | QCLASS values: the registry table | 0 | reviewed | QCLASS values: the registry table. |
| `3.3` | not stated | 0 | reviewed | Standard RRs preamble, defining the <domain-name> and <character-string> sub-formats. |
| `3.3.1` | CNAME RDATA format | 0 | reviewed | CNAME RDATA format. Ze emits no CNAME. |
| `3.3.2` | HINFO RDATA format | 0 | reviewed | HINFO RDATA format. Ze emits no HINFO. |
| `3.3.3` | MB RDATA format (EXPERIMENTAL) | 0 | reviewed | MB RDATA format (EXPERIMENTAL). Ze emits no MB. |
| `3.3.4` | MD RDATA format (Obsolete) | 0 | reviewed | MD RDATA format (Obsolete). Ze emits no MD. |
| `3.3.5` | MF RDATA format (Obsolete) | 0 | reviewed | MF RDATA format (Obsolete). Ze emits no MF. |
| `3.3.6` | MG RDATA format (EXPERIMENTAL) | 0 | reviewed | MG RDATA format (EXPERIMENTAL). Ze emits no MG. |
| `3.3.7` | MINFO RDATA format (EXPERIMENTAL) | 0 | reviewed | MINFO RDATA format (EXPERIMENTAL). Ze emits no MINFO. |
| `3.3.8` | MR RDATA format (EXPERIMENTAL) | 0 | reviewed | MR RDATA format (EXPERIMENTAL). Ze emits no MR. |
| `3.3.9` | MX RDATA format | 0 | reviewed | MX RDATA format. Ze emits no MX. |
| `3.3.10` | NULL RDATA format (EXPERIMENTAL) | 0 | reviewed | NULL RDATA format (EXPERIMENTAL). Ze emits no NULL. |
| `3.3.11` | NS RDATA format | 0 | reviewed | NS RDATA format. Ze emits NS for the zone apex and for synthesised nameserver glue; the section states the RDATA layout with no modal verb. |
| `3.3.12` | PTR RDATA format | 0 | reviewed | PTR RDATA format. Ze's as112 zones answer PTR queries with NODATA and emit no PTR record. |
| `3.3.13` | not stated | 0 | reviewed | SOA RDATA format, and the source of the MINIMUM-versus-TTL rule. Both its statements are indicative ('the TTL field is set to the maximum of', 'this use of MINIMUM should occur'), so the scan finds no site. |
| `3.3.14` | TXT RDATA format | 0 | reviewed | TXT RDATA format. Ze's as112 hostname zone emits TXT. |
| `3.4` | Internet specific RRs heading | 0 | reviewed | Internet specific RRs heading. |
| `3.4.1` | A RDATA format: a single 32-bit address | 0 | reviewed | A RDATA format: a single 32-bit address. Ze emits A records. |
| `3.4.2` | WKS RDATA format | 1 | reviewed | WKS RDATA format. Its one site is the bit-map alignment rule, excluded below because Ze emits no WKS record. |
| `3.5` | IN-ADDR.ARPA domain | 1 | reviewed | IN-ADDR.ARPA domain. Its one site binds a host bootstrapping routing from DNS. |
| `3.6` | Defining new types, classes, and special namespaces | 1 | reviewed | Defining new types, classes, and special namespaces. Its one site binds whoever defines a new TYPE or CLASS. |
| `4` | MESSAGES heading | 0 | reviewed | MESSAGES heading. |
| `4.1` | Format: the five-section message layout | 0 | reviewed | Format: the five-section message layout. |
| `4.1.1` | Header section format | 1 | reviewed | Header section format. Its one site is the Z field; AA and RCODE 3 are described field by field with no modal verb. |
| `4.1.2` | Question section format: QNAME, QTYPE, QCLASS | 0 | reviewed | Question section format: QNAME, QTYPE, QCLASS. |
| `4.1.3` | Resource record format | 0 | reviewed | Resource record format. TTL and RDLENGTH are given as field descriptions, and the zero-TTL clause is a 'should not', so the scan finds no site. |
| `4.1.4` | Message compression | 3 | reviewed | Message compression. Three sites; the pointer form and the receive-side obligation map, the third is rationale. The OFFSET base, the class-specific restriction and the enclosing-length rule are indicative prose. |
| `4.2` | Transport preamble | 1 | reviewed | Transport preamble. Its one site is the zone-refresh rule. |
| `4.2.1` | UDP usage | 1 | reviewed | UDP usage. Its one site is a querier obligation; the 512-octet limit, the truncation-plus-TC rule and the port are indicative prose. |
| `4.2.2` | TCP usage | 0 | reviewed | TCP usage. The port and the two-octet length prefix are stated indicatively. |
| `5` | MASTER FILES heading | 0 | reviewed | MASTER FILES heading. Ze parses no master file: a repo-wide search for NewZoneParser, ZoneParser, ReadRR and dns.NewRR finds no production call site. |
| `5.1` | Format | 1 | reviewed | Format. Its one site is master-file quoting syntax. |
| `5.2` | Use of master files to define zones | 1 | reviewed | Use of master files to define zones. Its one site concerns glue. |
| `5.3` | Master file example | 0 | reviewed | Master file example. Illustrative. |
| `6` | NAME SERVER IMPLEMENTATION heading | 0 | reviewed | NAME SERVER IMPLEMENTATION heading. The sections under it describe one suggested internal architecture, and Ze implements none of the recursion, caching or zone refresh they assume. |
| `6.1` | not stated | 0 | reviewed | Architecture, whose preamble states that the optimal structure depends on the host operating system and that the section 'discusses implementation considerations'. |
| `6.1.1` | Control | 1 | reviewed | Control. Its one site constrains internal task structure. |
| `6.1.2` | Database | 1 | reviewed | Database. Its one site is conditioned on choosing the suggested hash structure. |
| `6.1.3` | not stated | 0 | reviewed | Time: the suggestion to keep absolute rather than relative times internally. |
| `6.2` | Standard query processing | 1 | reviewed | Standard query processing. Its one site is the 'should' governing truncation order. |
| `6.3` | Zone refresh and reload processing | 0 | reviewed | Zone refresh and reload processing. Ze performs no zone transfer of any kind: a repo-wide search for AXFR, IXFR, TypeAXFR, TypeIXFR, dns.Transfer, SetAxfr, SetNotify and OpcodeNotify finds nothing. |
| `6.4` | Inverse queries (Optional) | 2 | reviewed | Inverse queries (Optional). Two sites: the permission not to implement them, and the obligation that survives declining -- the one obligation this walk added to the summary. |
| `6.4.1` | The contents of inverse queries and responses | 0 | reviewed | The contents of inverse queries and responses. Descriptive. |
| `6.4.2` | Inverse query and response example | 0 | reviewed | Inverse query and response example. Illustrative. |
| `6.4.3` | Inverse query processing | 0 | reviewed | Inverse query processing. Describes the optional algorithm. |
| `6.5` | not stated | 0 | reviewed | Completion queries and responses: obsoleted by this RFC and retained for historical reference only. |
| `7` | RESOLVER IMPLEMENTATION heading | 0 | reviewed | RESOLVER IMPLEMENTATION heading. Every obligation under it binds a resolver, a role the authoritative-server surface this summary scopes does not play. |
| `7.1` | Transforming a user request into a query | 4 | reviewed | Transforming a user request into a query. Four sites, all resolver obligations or descriptions of the suggested request structure. |
| `7.2` | Sending the queries | 1 | reviewed | Sending the queries. Its one site is resolver round-trip-time estimation. |
| `7.3` | Processing responses | 1 | reviewed | Processing responses. Its one site is resolver request matching. |
| `7.4` | Using the cache | 0 | reviewed | Using the cache. Ze holds no resolver cache. |
| `8` | MAIL SUPPORT heading | 0 | reviewed | MAIL SUPPORT heading. Ze emits no MX, MB, MG, MR or MINFO record and implements no mail binding. |
| `8.1` | Mail exchange binding | 0 | reviewed | Mail exchange binding. Describes the MX transition from MD/MF. |
| `8.2` | Mailbox binding (Experimental) | 0 | reviewed | Mailbox binding (Experimental). Experimental and unimplemented everywhere. |
| `9` | not stated | 0 | skipped (references) | REFERENCES and BIBLIOGRAPHY, followed by the index and the author's address. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 2.2 Common configurations, describing a stub-resolver host that forwards to a recursive server. 'the amount of new network code which is required' characterises one deployment choice; it directs nobody. | This can be appropriate for PCs or hosts which want to minimize the amount of new network code which is required. |
| `2.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The applicability statement that opens section 2.3 Conventions: 'While the implementor is free to violate these conventions WITHIN HIS OWN SYSTEM, he must observe these conventions in ALL behavior observed from other hosts.' It scopes the subsections that follow rather than stating an obligation of its own, and each of those conventions is captured separately. | While the implementor is free to violate these conventions WITHIN HIS OWN SYSTEM, he must observe these conventions in ALL behavior observed from other hosts. |
| `2.3.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | 'Labels must be 63 characters or less' restates in the preferred-syntax section the limit section 3.1 states normatively as the six-bit length field. Captured once, at the site that gives it its wire form. | Labels must be 63 characters or less. |
| `3.4.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The WKS RDATA bit-map alignment rule. It binds a server that serves WKS records; Ze emits A, AAAA, SRV, SOA and NS only (internal/plugins/geodns/server.go builds those five and no other type), so no Ze code path can produce a WKS bit map at all. | The bit map must be a multiple of 8 bits long. |
| `3.5:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 3.5 IN-ADDR.ARPA, on a host that bootstraps its ROUTING TABLE from the domain database and must therefore already know a gateway. Ze's routing tables come from BGP, IS-IS, OSPF and the kernel; nothing in Ze reads DNS to build one. The producer that would act as it if ze did is ze's only DNS code, the authoritative `answerQuery` (`internal/plugins/geodns/server.go`, `internal/plugins/as112/server.go`), which answers a query out of the configured zone data: it sends no query, holds no resolver cache and reads no master file. | - Systems that use the domain database to initialize their routing tables must start with enough gateway information to guarantee that they can access the appropriate name server. |
| `3.6:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 3.6 Defining new types, classes, and special namespaces. It binds whoever DEFINES a new TYPE or CLASS, constraining the registry relationship between TYPE and QTYPE. Ze defines none. The role is a registry act rather than an implementation, so no producer could perform it. Ze's DNS code is the authoritative `answerQuery` (`internal/plugins/geodns/server.go`, `internal/plugins/as112/server.go`), which reads whatever TYPE and CLASS values it is given. | TYPE and CLASS values must be a proper subset of QTYPEs and QCLASSes respectively. |
| `4.1.4:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The rationale for the preceding restriction ('If this were not the case, a name server or resolver would be required to know the format of all RRs it handles'). The restriction itself is captured as RFC1035-4.1.4-3; this sentence explains why it exists. | If this were not the case, a name server or resolver would be required to know the format of all RRs it handled. |
| `4.2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The retransmission strategy for queries SENT over UDP. It binds the querier; this summary's declared scope is the authoritative-server surface (see 'Extraction scope' in rfc/short/rfc1035.md), which answers queries and originates none. The producer that would act as it if ze did is ze's only DNS code, the authoritative `answerQuery` (`internal/plugins/geodns/server.go`, `internal/plugins/as112/server.go`), which answers a query out of the configured zone data: it sends no query, holds no resolver cache and reads no master file. | Queries sent using UDP may be lost, and hence a retransmission strategy is required. |
| `5.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Master-file quoting syntax. It binds a master-file PARSER. Ze parses none: geodns and as112 are configured from the YANG tree, and a repo-wide search for NewZoneParser, ZoneParser, ReadRR and dns.NewRR finds no production call site. | Inside a " delimited string any character can occur, except for a " itself, which must be quoted using \\ (back slash). |
| `5.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Glue in a master file that defines a zone. Same role as the site above: Ze has no master-file path. Its own level is 'should' in any case. The producer that would act as it if ze did is ze's only DNS code, the authoritative `answerQuery` (`internal/plugins/geodns/server.go`, `internal/plugins/as112/server.go`), which answers a query out of the configured zone data: it sends no query, holds no resolver cache and reads no master file. | If delegations are present and glue information is required, it should be present. 4. |
| `6.1.1:1` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | Section 6.1 Architecture opens 'The optimal structure for the name server will depend on the host operating system ... This section discusses implementation considerations'. The sentence constrains a server's internal task structure, has no wire-visible form, and nothing about it can be observed by a peer. | A name server must employ multiple concurrent activities, whether they are implemented as separate tasks in the host's OS or multiplexing inside a single name server program. |
| `6.1.2:1` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | Section 6.1.2 Database opens 'While name server implementations are free to use any internal data structures they choose, the suggested structure consists of'. The sentence is conditioned on having chosen the suggested hash structure, and it restates the case-preservation rule already captured as RFC1035-2.3.3-2. | In any case, hash structures used to store tree sections must insure that hash functions and procedures preserve the casing conventions of the DNS. |
| `6.2:1` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The ORDER in which a response should be truncated, at level 'should'. It never gates. The obligation to truncate at all and to set TC is captured separately as RFC1035-4.2.1-2. | When a response is so long that truncation is required, the truncation should start at the end of the response and work forward in the datagram. |
| `6.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | 'Name servers are not required to support any form of inverse queries' is a permission, and the sentence that qualifies it is the real obligation -- captured from the next site as RFC1035-6.4-1. | Name servers are not required to support any form of inverse queries. |
| `7.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Section 7 RESOLVER IMPLEMENTATION: a resolver multiplexing concurrent client requests. The authoritative server this summary scopes serves one query per response and originates none. The producer that would act as it if ze did is ze's only DNS code, the authoritative `answerQuery` (`internal/plugins/geodns/server.go`, `internal/plugins/as112/server.go`), which answers a query out of the configured zone data: it sends no query, holds no resolver cache and reads no master file. | Since a resolver must be able to multiplex multiple requests if it is to perform its function efficiently, each pending request is usually represented in some block of state information. |
| `7.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Resolver cache timeliness for a relative-time TTL. Ze's authoritative server holds no resolver cache. The producer that would act as it if ze did is ze's only DNS code, the authoritative `answerQuery` (`internal/plugins/geodns/server.go`, `internal/plugins/as112/server.go`), which answers a query out of the configured zone data: it sends no query, holds no resolver cache and reads no master file. | Note that when an RRs TTL indicates a relative time, the RR must be timely, since it is part of a zone. |
| `7.1:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | A bound on the work a RESOLVER does per client request, to guard against errors in the database. Same role as above. The producer that would act as it if ze did is ze's only DNS code, the authoritative `answerQuery` (`internal/plugins/geodns/server.go`, `internal/plugins/as112/server.go`), which answers a query out of the configured zone data: it sends no query, holds no resolver cache and reads no master file. | The amount of work which a resolver will do in response to a client request must be limited to guard against errors in the database, such as circular CNAME references, and operational problems, such as network partition which prevents the resolver from accessing the name servers it needs. |
| `7.1:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A descriptive sentence about the resolver's per-request state structure ('This structure keeps track of the state of a request if it must wait for answers'). It describes the suggested structure rather than obliging anyone. | This structure keeps track of the state of a request if it must wait for answers from foreign name servers. |
| `7.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Round-trip-time estimation when a resolver has no history for an address. A resolver obligation with no authoritative-server counterpart. The producer that would act as it if ze did is ze's only DNS code, the authoritative `answerQuery` (`internal/plugins/geodns/server.go`, `internal/plugins/as112/server.go`), which answers a query out of the configured zone data: it sends no query, holds no resolver cache and reads no master file. | Part of this step must deal with addresses which have no such history; in this case an expected round trip time of 5-10 seconds should be the worst case, with lower estimates for the same local network, etc. |
| `7.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Matching a response to its outstanding request in order to sample round-trip time. Again a resolver obligation: the authoritative server sends responses, never receives them. The producer that would act as it if ze did is ze's only DNS code, the authoritative `answerQuery` (`internal/plugins/geodns/server.go`, `internal/plugins/as112/server.go`), which answers a query out of the configured zone data: it sends no query, holds no resolver cache and reads no master file. | However, if it is using the response to sample the round trip time to access the name server, it must be able to determine which transmission matches the response (and keep transmission times for each outgoing message), or only calculate round trip times based on initial transmissions. |

## Superseded

No document obsoletes RFC 1035, so its obligations are stated where they were written.
