# RFC 1035 - Domain Names - Implementation and Specification

Partial. Every requirement this repository extracted from RFC 1035, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 92.6% | 25 of 27 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 27 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 27 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 1.6% | 1 of 62 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 27 | of 33 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (backlog), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 0 | of 27 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 7.4% | 2 of 27 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 27 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Enrolment | Not enrolled (backlog) |
| Requirements | 33 |
| Gated MUST-level | 27 |
| Obligations that bind Ze | 27 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 2 |
| Nightly-only evidence | 0 |
| Test tags | 62 |
| Tagged units | 62 |
| Recorded audit verdicts | 0 |
| Discrimination records | 1 |
| Summary | `rfc/short/rfc1035.md` |
| Requirement shard | `rfc/requirements/rfc1035.md` |
| RFC text | `rfc/full/rfc1035.txt` |

## Enrolment

Not enrolled (backlog, the requirements have not been extracted from the document yet; this is work owed rather than a decision): Domain Names: Implementation and Specification. Re-authored 2026-07-30 and it now declares 27 MUST-level obligations read from the indicative prose of a 1987 document (0 capitalised keywords, 23 lowercase must), so this is no longer an empty checklist. It is not enrolled because the obligations are not all proven and the unproven ones need an owner ruling, not an implementer's annotation. The obligation with no code path in Ze is zone transfer: Ze performs none, and the owner ruled RFC 1035 out of scope on 2026-08-18, so that work is not to be started. The 512-octet UDP bound and the TC bit ARE enforced -- send calls Msg.Truncate(udpReplyLimit(r)) for a datagram reply in internal/core/dnsserver/handler.go, and udpReplyLimit holds the Section 2.3.4 floor while letting an RFC 6891 Section 6.2.3 OPT record raise it. An unsupported inverse query DOES draw Not Implemented: Authoritative branches on the opcode before any zone lookup, in the same file. It was the one obligation the 73-section walk found OUTSIDE the summary's declared scope and added to it. The response TTL is deliberately not raised to the SOA MINIMUM -- RFC 2308 Section 4 withdrew that rule, hdr in internal/plugins/geodns/server.go applies no floor, and TestRFC2308_NoZoneWideTTLFloor holds the decision. About 6 requirements admit only a positive polarity because miekg/dns owns the wire codec and no Ze-side change can break them. Escalated for scoping per OR-1b.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

DNS cache TTL handling, plus authoritative GeoDNS and AS112 response shaping. The reply carries the AA bit and the compression-off, recursion-unavailable shape, re-asserted after the answer function on every query (`Authoritative` in [`internal/core/dnsserver/handler.go`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/handler.go)). A reply on a datagram transport is bounded and carries TC when it is shortened (`send` and `udpReplyLimit`, same file): the Section 2.3.4 floor of 512 octets, raised to the reassembly buffer an RFC 6891 Section 6.2.3 OPT record advertises. A stream reply is written whole. An opcode Ze does not serve draws Not Implemented before any zone lookup, which is the Section 6.4 reply an unsupported inverse query gets (`Authoritative`, same file). A name outside every served zone draws NXDOMAIN, and a name inside one with no matching record draws NODATA plus the zone SOA. Zone and label matching folds case. TTLs are bounded to 0..2147483647. The AS112 UDP and TCP listeners bind port 53. Obligations extracted 2026-07-30 and bound per line in [`rfc/short/rfc1035.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc1035.md). The walk of all 73 sections is recorded in [`rfc/extraction/rfc1035.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc1035.json).

**What the ledger says remains**

Not enrolled. RFC 1035 predates RFC 2119 and states every obligation in lowercase indicative prose. The extraction read that prose as normative, and the surviving obligation with no code path is zone transfer: Ze performs none, and the owner ruled RFC 1035 out of scope on 2026-08-18, so that work is not to be started. A response TTL is deliberately not raised to the zone SOA MINIMUM. RFC 2308 Section 4 withdrew that rule ("the minimum TTL value of all RRs in a zone, has never in practice been used and is hereby deprecated"), `hdr` in [`internal/plugins/geodns/server.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server.go) applies no floor, and `TestRFC2308_NoZoneWideTTLFloor` holds the decision. The DNS wire codec is `github.com/miekg/dns`, so several encoding obligations admit no negative test through Ze at all. Declared `backlog` while an owner ruling is outstanding.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 25 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 2 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **27** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (25):** [`RFC1035-2.3.3-1`](#rfc1035-2.3.3-1), [`RFC1035-2.3.3-2`](#rfc1035-2.3.3-2), [`RFC1035-2.3.4-1`](#rfc1035-2.3.4-1), [`RFC1035-2.3.4-2`](#rfc1035-2.3.4-2), [`RFC1035-3.1-1`](#rfc1035-3.1-1), [`RFC1035-3.1-2`](#rfc1035-3.1-2), [`RFC1035-3.1-3`](#rfc1035-3.1-3), [`RFC1035-3.1-4`](#rfc1035-3.1-4), [`RFC1035-3.1-5`](#rfc1035-3.1-5), [`RFC1035-3.1-6`](#rfc1035-3.1-6), [`RFC1035-4.1.1-1`](#rfc1035-4.1.1-1), [`RFC1035-4.1.1-2`](#rfc1035-4.1.1-2), [`RFC1035-4.1.1-3`](#rfc1035-4.1.1-3), [`RFC1035-4.1.3-1`](#rfc1035-4.1.3-1), [`RFC1035-4.1.3-2`](#rfc1035-4.1.3-2), [`RFC1035-4.1.4-1`](#rfc1035-4.1.4-1), [`RFC1035-4.1.4-2`](#rfc1035-4.1.4-2), [`RFC1035-4.1.4-3`](#rfc1035-4.1.4-3), [`RFC1035-4.1.4-4`](#rfc1035-4.1.4-4), [`RFC1035-4.1.4-5`](#rfc1035-4.1.4-5), [`RFC1035-4.2.1-1`](#rfc1035-4.2.1-1), [`RFC1035-4.2.1-2`](#rfc1035-4.2.1-2), [`RFC1035-4.2.1-3`](#rfc1035-4.2.1-3), [`RFC1035-4.2.2-1`](#rfc1035-4.2.2-1), [`RFC1035-6.4-1`](#rfc1035-6.4-1)

**No test and no annotation (2):** [`RFC1035-3.3.13-1`](#rfc1035-3.3.13-1), [`RFC1035-4.2-1`](#rfc1035-4.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC1035-2.3.3-1` | For all parts of the DNS that are part of the official protocol, all comparisons between character strings (e.g., labels, domain names, etc.) are done in a case-insensitive manner (§2.3.3) | MUST | 2.3.3 - Character Case | **positive:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L54). **negative:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L97) |
| `RFC1035-2.3.3-2` | Loss of case sensitive data must be minimized (§2.3.3) | MUST | 2.3.3 - Character Case | **positive:** `unit/verify` [`TestRFC1035_QueryNameCasePreservedInTheReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L133). **negative:** `unit/verify` [`TestRFC1035_QueryNameCasePreservedInTheReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L148) |
| `RFC1035-2.3.4-1` | Size limit -- TTL: positive values of a signed 32 bit number (§2.3.4) | MUST | 2.3.4 - Size limits | **positive:** `unit/verify` [`TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L144). **negative:** `unit/verify` [`TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L159) |
| `RFC1035-2.3.4-2` | Size limit -- UDP messages: 512 octets or less (§2.3.4) | MUST | 2.3.4 - Size limits | **positive:** `unit/verify` [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L102). **negative:** `unit/verify` [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L132) |
| `RFC1035-3.1-1` | Each label is represented as a one octet length field followed by that number of octets (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L179). **negative:** `unit/verify` [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L204) |
| `RFC1035-3.1-2` | Since every domain name ends with the null label of the root, a domain name is terminated by a length byte of zero (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L186). **negative:** `unit/verify` [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L192) |
| `RFC1035-3.1-3` | The high order two bits of every length octet must be zero, and the remaining six bits of the length field limit the label to 63 octets or less (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035_ConfiguredLabelBoundedTo63Octets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L57). **negative:** `unit/verify` [`TestRFC1035_ConfiguredLabelBoundedTo63Octets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L73). **positive:** `functional/verify` [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L24). **negative:** `functional/verify` [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L67) |
| `RFC1035-3.1-4` | The total length of a domain name (i.e., label octets and label length octets) is restricted to 255 octets or less (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035_ConfiguredNameBoundedTo255WireOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L134). **negative:** `unit/verify` [`TestRFC1035_ConfiguredNameBoundedTo255WireOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L115). **positive:** `functional/verify` [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L43). **negative:** `functional/verify` [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L65) |
| `RFC1035-3.1-5` | Name servers and resolvers must compare labels in a case-insensitive manner (i.e., A=a), assuming ASCII with zero parity (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L59). **negative:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L105) |
| `RFC1035-3.1-6` | Non-alphabetic codes must match exactly (§3.1) | MUST | 3.1 - Name space definitions: the wire form of a domain name | **positive:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L87). **negative:** `unit/verify` [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L110) |
| `RFC1035-3.3.13-1` | Whenever a RR is sent in a response to a query, the TTL field is set to the maximum of the TTL field from the RR and the MINIMUM field in the appropriate SOA (§3.3.13) | MUST | 3.3.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-4.1.1-1` | Z is reserved for future use and must be zero in all queries and responses (§4.1.1) | MUST | 4.1.1 - Header section format | **positive:** `unit/verify` [`TestRFC1035_ReservedZFieldIsZero`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L69). **negative:** `unit/verify` [`TestRFC1035_ReservedZFieldIsZero`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L102) |
| `RFC1035-4.1.1-2` | AA (Authoritative Answer) is valid in responses and specifies that the responding name server is an authority for the domain name in question section (§4.1.1) | MUST | 4.1.1 - Header section format | **positive:** `unit/verify` [`TestRFC1035_AuthoritativeAnswerBitOnEveryReply`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L137). **negative:** `unit/verify` [`TestRFC1035_AuthoritativeAnswerBitOnEveryReply`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L162). **negative:** `unit/verify` [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L117). **negative:** `unit/verify` [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L253) |
| `RFC1035-4.1.1-3` | RCODE 3 (Name Error), meaningful only for responses from an authoritative name server, signifies that the domain name referenced in the query does not exist (§4.1.1) | MUST | 4.1.1 - Header section format | **positive:** `unit/verify` [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L102). **positive:** `unit/verify` [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L237). **negative:** `unit/verify` [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L107). **negative:** `unit/verify` [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L242) |
| `RFC1035-4.1.3-1` | TTL is a 32 bit unsigned integer that specifies the time interval in seconds that the resource record may be cached (§4.1.3) | MUST | 4.1.3 - Resource record format | **positive:** `unit/verify` [`TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L109). **negative:** `unit/verify` [`TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L127) |
| `RFC1035-4.1.3-2` | RDLENGTH is an unsigned 16 bit integer that specifies the length in octets of the RDATA field (§4.1.3) | MUST | 4.1.3 - Resource record format | **positive:** `unit/verify` [`TestRFC1035_RDLengthCountsTheRDataOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L181). **negative:** `unit/verify` [`TestRFC1035_RDLengthCountsTheRDataOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L201) |
| `RFC1035-4.1.4-1` | A compression pointer takes the form of a two octet sequence whose first two bits are ones, distinguishing it from a label, which must begin with two zero bits (§4.1.4) | MUST | 4.1.4 - Message compression | **positive:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L150). **negative:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L225) |
| `RFC1035-4.1.4-2` | The OFFSET field specifies an offset from the start of the message, i.e. the first octet of the ID field in the domain header (§4.1.4) | MUST | 4.1.4 - Message compression | **positive:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L169). **negative:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L229) |
| `RFC1035-4.1.4-3` | Pointers can only be used for occurances of a domain name where the format is not class specific (§4.1.4) | MUST | 4.1.4 - Message compression | **positive:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L208). **negative:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L232) |
| `RFC1035-4.1.4-4` | If a domain name is contained in a part of the message subject to a length field, such as the RDATA section of an RR, and compression is used, the length of the compressed name is used in the length calculation, rather than the length of the expanded name (§4.1.4) | MUST | 4.1.4 - Message compression | **positive:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L184). **negative:** `unit/verify` [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L235) |
| `RFC1035-4.1.4-5` | All programs are required to understand arriving messages that contain pointers (§4.1.4) | REQUIRED | 4.1.4 - Message compression | **positive:** `unit/verify` [`TestRFC1035_InboundCompressionPointerUnderstood`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L345). **negative:** `unit/verify` [`TestRFC1035_InboundCompressionPointerUnderstood`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L365) |
| `RFC1035-4.2-1` | Zone refresh activities must use virtual circuits because of the need for reliable transfer (§4.2) | MUST | 4.2 - Transport preamble | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-4.2.1-1` | Messages carried by UDP are restricted to 512 bytes, not counting the IP or UDP headers (§4.2.1) | MUST | 4.2.1 - UDP usage | **positive:** `unit/verify` [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L97). **positive:** `unit/verify` [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L81). **negative:** `unit/verify` [`TestRFC1035_UDPBoundFollowsAdvertisedEDNSSize`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L164) |
| `RFC1035-4.2.1-2` | Longer messages are truncated and the TC bit is set in the header (§4.2.1) | MUST | 4.2.1 - UDP usage | **positive:** `unit/verify` [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L107). **positive:** `unit/verify` [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L90). **negative:** `unit/verify` [`TestRFC1035_StreamTransportNotTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L195). **negative:** `unit/verify` [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L127). **negative:** `unit/verify` [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L104) |
| `RFC1035-4.2.1-3` | Messages sent using UDP user server port 53 (decimal) (§4.2.1) <!-- "user" is verbatim: RFC 1035 rfc/full/rfc1035.txt:1754 has a typo for "use", and the id contract pins the quoted text, so it is reproduced rather than silently corrected. Compare :1783, which reads "use server port 53" for TCP. --> | MUST | 4.2.1 - UDP usage | **positive:** `unit/verify` [`TestRFC1035_DNSTransportsUseServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc1035_port_test.go#L26). **negative:** `unit/verify` [`TestRFC1035_DNSTransportsUseServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc1035_port_test.go#L49) |
| `RFC1035-4.2.2-1` | Messages sent over TCP connections use server port 53 decimal, and the message is prefixed with a two byte length field which gives the message length (§4.2.2) | MUST | 4.2.2 - TCP usage | **positive:** `unit/verify` [`TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L421). **negative:** `unit/verify` [`TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L445) |
| `RFC1035-6.4-1` | While inverse query support is optional, all name servers must be at least able to return the error response (§6.4) | MUST | 6.4 - Inverse queries (Optional) | **positive:** `unit/verify` [`TestRFC1035_UnsupportedOpcodeReturnsNotImplemented`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L241). **negative:** `unit/verify` [`TestRFC1035_QueryOpcodeAnsweredNormally`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L290) |
| `RFC1035-2.3.1-1` | Preferred name syntax: labels must start with a letter, end with a letter or digit, and have as interior characters only letters, digits, and hyphen (§2.3.1) | SHOULD | 2.3.1 - Preferred name syntax | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-2.3.3-3` | When data enters the domain system, its original case should be preserved whenever possible (§2.3.3) | SHOULD | 2.3.3 - Character Case | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-2.3.3-4` | Attempts to store domain names in 7-bit ASCII or use of special bytes to terminate labels, etc., should be avoided (§2.3.3) | SHOULD NOT | 2.3.3 - Character Case | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-3.1-7` | Although labels can contain any 8 bit values in octets that make up a label, it is strongly recommended that labels follow the preferred syntax described elsewhere in this memo (§3.1) | RECOMMENDED | 3.1 - Name space definitions: the wire form of a domain name | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-3.3.13-2` | This use of MINIMUM should occur when the RRs are copied into the response and not when the zone is loaded from a master file or via a zone transfer (§3.3.13) | SHOULD | 3.3.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC1035-4.1.3-3` | Zero TTL values are interpreted to mean that the RR can only be used for the transaction in progress, and should not be cached (§4.1.3) | SHOULD NOT | 4.1.3 - Resource record format | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC1035-3.3.13-1`](#rfc1035-3.3.13-1) Whenever a RR is sent in a response to a query, the TTL field is set to the maximum of the TTL field from the RR and the MINIMUM field in the appropriate SOA (§3.3.13) | no test | no test carries this requirement id |
| [`RFC1035-4.2-1`](#rfc1035-4.2-1) Zone refresh activities must use virtual circuits because of the need for reliable transfer (§4.2) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC1035-2.3.3-1`](#rfc1035-2.3.3-1)

For all parts of the DNS that are part of the official protocol, all comparisons between character strings (e.g., labels, domain names, etc.) are done in a case-insensitive manner (§2.3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L97) | unit/verify | unproven |
| positive | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L54) | unit/verify | unproven |

### [`RFC1035-2.3.3-2`](#rfc1035-2.3.3-2)

Loss of case sensitive data must be minimized (§2.3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_QueryNameCasePreservedInTheReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L148) | unit/verify | unproven |
| positive | [`TestRFC1035_QueryNameCasePreservedInTheReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L133) | unit/verify | unproven |

### [`RFC1035-2.3.4-1`](#rfc1035-2.3.4-1)

Size limit -- TTL: positive values of a signed 32 bit number (§2.3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L159) | unit/verify | unproven |
| positive | [`TestRFC1035_ConfiguredTTLBoundedToASigned32BitPositive`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L144) | unit/verify | unproven |

### [`RFC1035-2.3.4-2`](#rfc1035-2.3.4-2)

Size limit -- UDP messages: 512 octets or less (§2.3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L132) | unit/verify | unproven |
| positive | [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L102) | unit/verify | unproven |

### [`RFC1035-3.1-1`](#rfc1035-3.1-1)

Each label is represented as a one octet length field followed by that number of octets (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L204) | unit/verify | unproven |
| positive | [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L179) | unit/verify | unproven |

### [`RFC1035-3.1-2`](#rfc1035-3.1-2)

Since every domain name ends with the null label of the root, a domain name is terminated by a length byte of zero (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L192) | unit/verify | unproven |
| positive | [`TestRFC1035_NameEncodedAsLengthPrefixedLabelsEndingAtRoot`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L186) | unit/verify | unproven |

### [`RFC1035-3.1-3`](#rfc1035-3.1-3)

The high order two bits of every length octet must be zero, and the remaining six bits of the length field limit the label to 63 octets or less (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_ConfiguredLabelBoundedTo63Octets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L73) | unit/verify | unproven |
| negative | [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L67) | functional/verify | unproven |
| positive | [`TestRFC1035_ConfiguredLabelBoundedTo63Octets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L57) | unit/verify | unproven |
| positive | [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L24) | functional/verify | revert, verified |

### [`RFC1035-3.1-4`](#rfc1035-3.1-4)

The total length of a domain name (i.e., label octets and label length octets) is restricted to 255 octets or less (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_ConfiguredNameBoundedTo255WireOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L115) | unit/verify | unproven |
| negative | [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L65) | functional/verify | unproven |
| positive | [`TestRFC1035_ConfiguredNameBoundedTo255WireOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_limits_test.go#L134) | unit/verify | unproven |
| positive | [`dns-name-too-long.ci`](https://github.com/ze-software/ze/blob/main/test/parse/dns-name-too-long.ci#L43) | functional/verify | unproven |

### [`RFC1035-3.1-5`](#rfc1035-3.1-5)

Name servers and resolvers must compare labels in a case-insensitive manner (i.e., A=a), assuming ASCII with zero parity (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L105) | unit/verify | unproven |
| positive | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L59) | unit/verify | unproven |

### [`RFC1035-3.1-6`](#rfc1035-3.1-6)

Non-alphabetic codes must match exactly (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L110) | unit/verify | unproven |
| positive | [`TestRFC1035_NameComparisonIsCaseInsensitiveForLettersOnly`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_name_case_test.go#L87) | unit/verify | unproven |

### [`RFC1035-3.3.13-1`](#rfc1035-3.3.13-1)

Whenever a RR is sent in a response to a query, the TTL field is set to the maximum of the TTL field from the RR and the MINIMUM field in the appropriate SOA (§3.3.13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1035-3.3.13-1, so no unit is bound to it.

### [`RFC1035-4.1.1-1`](#rfc1035-4.1.1-1)

Z is reserved for future use and must be zero in all queries and responses (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_ReservedZFieldIsZero`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L102) | unit/verify | unproven |
| positive | [`TestRFC1035_ReservedZFieldIsZero`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L69) | unit/verify | unproven |

### [`RFC1035-4.1.1-2`](#rfc1035-4.1.1-2)

AA (Authoritative Answer) is valid in responses and specifies that the responding name server is an authority for the domain name in question section (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_AuthoritativeAnswerBitOnEveryReply`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L162) | unit/verify | unproven |
| negative | [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L253) | unit/verify | unproven |
| negative | [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L117) | unit/verify | unproven |
| positive | [`TestRFC1035_AuthoritativeAnswerBitOnEveryReply`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_header_test.go#L137) | unit/verify | unproven |

### [`RFC1035-4.1.1-3`](#rfc1035-4.1.1-3)

RCODE 3 (Name Error), meaningful only for responses from an authoritative name server, signifies that the domain name referenced in the query does not exist (§4.1.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L242) | unit/verify | unproven |
| negative | [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L107) | unit/verify | unproven |
| positive | [`TestZoneAnswer_ResponseCodeByNamePosition`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/zones_test.go#L237) | unit/verify | unproven |
| positive | [`TestRFC1035_ResponseCodeByNameAndClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_negative_test.go#L102) | unit/verify | unproven |

### [`RFC1035-4.1.3-1`](#rfc1035-4.1.3-1)

TTL is a 32 bit unsigned integer that specifies the time interval in seconds that the resource record may be cached (§4.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L127) | unit/verify | unproven |
| positive | [`TestRFC1035_RecordTTLIsA32BitUnsignedSecondCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L109) | unit/verify | unproven |

### [`RFC1035-4.1.3-2`](#rfc1035-4.1.3-2)

RDLENGTH is an unsigned 16 bit integer that specifies the length in octets of the RDATA field (§4.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_RDLengthCountsTheRDataOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L201) | unit/verify | unproven |
| positive | [`TestRFC1035_RDLengthCountsTheRDataOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_rr_test.go#L181) | unit/verify | unproven |

### [`RFC1035-4.1.4-1`](#rfc1035-4.1.4-1)

A compression pointer takes the form of a two octet sequence whose first two bits are ones, distinguishing it from a label, which must begin with two zero bits (§4.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L225) | unit/verify | unproven |
| positive | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L150) | unit/verify | unproven |

### [`RFC1035-4.1.4-2`](#rfc1035-4.1.4-2)

The OFFSET field specifies an offset from the start of the message, i.e. the first octet of the ID field in the domain header (§4.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L229) | unit/verify | unproven |
| positive | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L169) | unit/verify | unproven |

### [`RFC1035-4.1.4-3`](#rfc1035-4.1.4-3)

Pointers can only be used for occurances of a domain name where the format is not class specific (§4.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L232) | unit/verify | unproven |
| positive | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L208) | unit/verify | unproven |

### [`RFC1035-4.1.4-4`](#rfc1035-4.1.4-4)

If a domain name is contained in a part of the message subject to a length field, such as the RDATA section of an RR, and compression is used, the length of the compressed name is used in the length calculation, rather than the length of the expanded name (§4.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L235) | unit/verify | unproven |
| positive | [`TestRFC1035_CompressionPointersInATruncatedDatagram`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L184) | unit/verify | unproven |

### [`RFC1035-4.1.4-5`](#rfc1035-4.1.4-5)

All programs are required to understand arriving messages that contain pointers (§4.1.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_InboundCompressionPointerUnderstood`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L365) | unit/verify | unproven |
| positive | [`TestRFC1035_InboundCompressionPointerUnderstood`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L345) | unit/verify | unproven |

### [`RFC1035-4.2-1`](#rfc1035-4.2-1)

Zone refresh activities must use virtual circuits because of the need for reliable transfer (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC1035-4.2-1, so no unit is bound to it.

### [`RFC1035-4.2.1-1`](#rfc1035-4.2.1-1)

Messages carried by UDP are restricted to 512 bytes, not counting the IP or UDP headers (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_UDPBoundFollowsAdvertisedEDNSSize`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L164) | unit/verify | unproven |
| positive | [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L97) | unit/verify | unproven |
| positive | [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L81) | unit/verify | unproven |

### [`RFC1035-4.2.1-2`](#rfc1035-4.2.1-2)

Longer messages are truncated and the TC bit is set in the header (§4.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_StreamTransportNotTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L195) | unit/verify | unproven |
| negative | [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L127) | unit/verify | unproven |
| negative | [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L104) | unit/verify | unproven |
| positive | [`TestRFC1035_UDPReplyBoundedAndTruncated`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L107) | unit/verify | unproven |
| positive | [`TestRFC1035_UDPTruncatedTCPWholeOverRealSockets`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_server_transport_test.go#L90) | unit/verify | unproven |

### [`RFC1035-4.2.1-3`](#rfc1035-4.2.1-3)

Messages sent using UDP user server port 53 (decimal) (§4.2.1) <!-- "user" is verbatim: RFC 1035 rfc/full/rfc1035.txt:1754 has a typo for "use", and the id contract pins the quoted text, so it is reproduced rather than silently corrected. Compare :1783, which reads "use server port 53" for TCP. -->

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_DNSTransportsUseServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc1035_port_test.go#L49) | unit/verify | unproven |
| positive | [`TestRFC1035_DNSTransportsUseServerPort53`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc1035_port_test.go#L26) | unit/verify | unproven |

### [`RFC1035-4.2.2-1`](#rfc1035-4.2.2-1)

Messages sent over TCP connections use server port 53 decimal, and the message is prefixed with a two byte length field which gives the message length (§4.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L445) | unit/verify | unproven |
| positive | [`TestRFC1035_TCPRepliesCarryATwoOctetLengthPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc1035_compression_test.go#L421) | unit/verify | unproven |

### [`RFC1035-6.4-1`](#rfc1035-6.4-1)

While inverse query support is optional, all name servers must be at least able to return the error response (§6.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC1035_QueryOpcodeAnsweredNormally`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L290) | unit/verify | unproven |
| positive | [`TestRFC1035_UnsupportedOpcodeReturnsNotImplemented`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc1035_handler_test.go#L241) | unit/verify | unproven |

## Extraction sign-off

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
| `2` | INTRODUCTION heading, with no body of its own | 0 | walked | INTRODUCTION heading, with no body of its own. |
| `2.1` | not stated | 0 | walked | Overview of the three-part domain system (name space, name servers, resolvers). Descriptive. |
| `2.2` | not stated | 1 | walked | Common configurations: the deployment shapes a resolver and a name server can take. Its one site is a characterisation, excluded below. |
| `2.3` | Conventions | 1 | walked | Conventions. The section preamble is its one site, an applicability statement excluded below; the conventions themselves are in 2.3.1 to 2.3.4. |
| `2.3.1` | Preferred name syntax | 3 | walked | Preferred name syntax. Two sites map to the single [SHOULD] row that carries both halves; the third restates section 3.1's 63-octet limit. |
| `2.3.2` | not stated | 0 | walked | Data Transmission Order: the big-endian octet and bit order of every multi-octet field. Stated as a description of the diagrams, with no modal verb, so the prose scan finds no site. Ze inherits the encoding from miekg/dns and adds nothing. |
| `2.3.3` | Character Case | 1 | walked | Character Case. Its one site is the case-preservation obligation; the case-insensitive comparison rule and the two 'should' clauses are read from the surrounding prose, which carries no scanned keyword. |
| `2.3.4` | Size limits | 0 | walked | Size limits. Stated as a bare list ('labels 63 octets or less', 'names 255 octets or less', 'TTL positive values of a signed 32 bit number', 'UDP messages 512 octets or less') with no modal verb, so the scan finds no site here even though two of the four are declared as requirements. |
| `3` | DOMAIN NAME SPACE AND RR DEFINITIONS heading | 0 | walked | DOMAIN NAME SPACE AND RR DEFINITIONS heading. |
| `3.1` | Name space definitions: the wire form of a domain name | 3 | walked | Name space definitions: the wire form of a domain name. Three sites map to the length-octet, case-folding and exact-match rules; the label/terminator/total-length statements and the 'strongly recommended' clause are indicative prose. |
| `3.2` | RR definitions heading | 0 | walked | RR definitions heading. |
| `3.2.1` | not stated | 0 | walked | Format: the NAME/TYPE/CLASS/TTL/RDLENGTH/RDATA layout, given as a diagram plus field descriptions with no modal verb. |
| `3.2.2` | TYPE values: the registry table | 0 | walked | TYPE values: the registry table. |
| `3.2.3` | QTYPE values: the registry table | 0 | walked | QTYPE values: the registry table. |
| `3.2.4` | CLASS values: the registry table | 0 | walked | CLASS values: the registry table. |
| `3.2.5` | QCLASS values: the registry table | 0 | walked | QCLASS values: the registry table. |
| `3.3` | not stated | 0 | walked | Standard RRs preamble, defining the <domain-name> and <character-string> sub-formats. |
| `3.3.1` | CNAME RDATA format | 0 | walked | CNAME RDATA format. Ze emits no CNAME. |
| `3.3.2` | HINFO RDATA format | 0 | walked | HINFO RDATA format. Ze emits no HINFO. |
| `3.3.3` | MB RDATA format (EXPERIMENTAL) | 0 | walked | MB RDATA format (EXPERIMENTAL). Ze emits no MB. |
| `3.3.4` | MD RDATA format (Obsolete) | 0 | walked | MD RDATA format (Obsolete). Ze emits no MD. |
| `3.3.5` | MF RDATA format (Obsolete) | 0 | walked | MF RDATA format (Obsolete). Ze emits no MF. |
| `3.3.6` | MG RDATA format (EXPERIMENTAL) | 0 | walked | MG RDATA format (EXPERIMENTAL). Ze emits no MG. |
| `3.3.7` | MINFO RDATA format (EXPERIMENTAL) | 0 | walked | MINFO RDATA format (EXPERIMENTAL). Ze emits no MINFO. |
| `3.3.8` | MR RDATA format (EXPERIMENTAL) | 0 | walked | MR RDATA format (EXPERIMENTAL). Ze emits no MR. |
| `3.3.9` | MX RDATA format | 0 | walked | MX RDATA format. Ze emits no MX. |
| `3.3.10` | NULL RDATA format (EXPERIMENTAL) | 0 | walked | NULL RDATA format (EXPERIMENTAL). Ze emits no NULL. |
| `3.3.11` | NS RDATA format | 0 | walked | NS RDATA format. Ze emits NS for the zone apex and for synthesised nameserver glue; the section states the RDATA layout with no modal verb. |
| `3.3.12` | PTR RDATA format | 0 | walked | PTR RDATA format. Ze's as112 zones answer PTR queries with NODATA and emit no PTR record. |
| `3.3.13` | not stated | 0 | walked | SOA RDATA format, and the source of the MINIMUM-versus-TTL rule. Both its statements are indicative ('the TTL field is set to the maximum of', 'this use of MINIMUM should occur'), so the scan finds no site. |
| `3.3.14` | TXT RDATA format | 0 | walked | TXT RDATA format. Ze's as112 hostname zone emits TXT. |
| `3.4` | Internet specific RRs heading | 0 | walked | Internet specific RRs heading. |
| `3.4.1` | A RDATA format: a single 32-bit address | 0 | walked | A RDATA format: a single 32-bit address. Ze emits A records. |
| `3.4.2` | WKS RDATA format | 1 | walked | WKS RDATA format. Its one site is the bit-map alignment rule, excluded below because Ze emits no WKS record. |
| `25` | NOT A SECTION | 0 | walked | NOT A SECTION. The derivation's heading pattern over-matches a column-0 line inside section 3.4.2's WKS bit-map explanation ('25 (SMTP). If this bit is set, a SMTP server should be listening on TCP port 25'), which _SECTION_HEADING_RE documents as unavoidable by shape alone. It carries no site and no obligation; it is classified so the artifact stays complete against the derivation. |
| `3.5` | IN-ADDR.ARPA domain | 1 | walked | IN-ADDR.ARPA domain. Its one site binds a host bootstrapping routing from DNS. |
| `3.6` | Defining new types, classes, and special namespaces | 1 | walked | Defining new types, classes, and special namespaces. Its one site binds whoever defines a new TYPE or CLASS. |
| `4` | MESSAGES heading | 0 | walked | MESSAGES heading. |
| `4.1` | Format: the five-section message layout | 0 | walked | Format: the five-section message layout. |
| `4.1.1` | Header section format | 1 | walked | Header section format. Its one site is the Z field; AA and RCODE 3 are described field by field with no modal verb. |
| `4.1.2` | Question section format: QNAME, QTYPE, QCLASS | 0 | walked | Question section format: QNAME, QTYPE, QCLASS. |
| `4.1.3` | Resource record format | 0 | walked | Resource record format. TTL and RDLENGTH are given as field descriptions, and the zero-TTL clause is a 'should not', so the scan finds no site. |
| `4.1.4` | Message compression | 3 | walked | Message compression. Three sites; the pointer form and the receive-side obligation map, the third is rationale. The OFFSET base, the class-specific restriction and the enclosing-length rule are indicative prose. |
| `4.2` | Transport preamble | 1 | walked | Transport preamble. Its one site is the zone-refresh rule. |
| `4.2.1` | UDP usage | 1 | walked | UDP usage. Its one site is a querier obligation; the 512-octet limit, the truncation-plus-TC rule and the port are indicative prose. |
| `4.2.2` | TCP usage | 0 | walked | TCP usage. The port and the two-octet length prefix are stated indicatively. |
| `5` | MASTER FILES heading | 0 | walked | MASTER FILES heading. Ze parses no master file: a repo-wide search for NewZoneParser, ZoneParser, ReadRR and dns.NewRR finds no production call site. |
| `5.1` | Format | 1 | walked | Format. Its one site is master-file quoting syntax. |
| `5.2` | Use of master files to define zones | 1 | walked | Use of master files to define zones. Its one site concerns glue. |
| `5.3` | Master file example | 0 | walked | Master file example. Illustrative. |
| `6` | NAME SERVER IMPLEMENTATION heading | 0 | walked | NAME SERVER IMPLEMENTATION heading. The sections under it describe one suggested internal architecture, and Ze implements none of the recursion, caching or zone refresh they assume. |
| `6.1` | not stated | 0 | walked | Architecture, whose preamble states that the optimal structure depends on the host operating system and that the section 'discusses implementation considerations'. |
| `6.1.1` | Control | 1 | walked | Control. Its one site constrains internal task structure. |
| `6.1.2` | Database | 1 | walked | Database. Its one site is conditioned on choosing the suggested hash structure. |
| `6.1.3` | not stated | 0 | walked | Time: the suggestion to keep absolute rather than relative times internally. |
| `6.2` | Standard query processing | 1 | walked | Standard query processing. Its one site is the 'should' governing truncation order. |
| `6.3` | Zone refresh and reload processing | 0 | walked | Zone refresh and reload processing. Ze performs no zone transfer of any kind: a repo-wide search for AXFR, IXFR, TypeAXFR, TypeIXFR, dns.Transfer, SetAxfr, SetNotify and OpcodeNotify finds nothing. |
| `6.4` | Inverse queries (Optional) | 2 | walked | Inverse queries (Optional). Two sites: the permission not to implement them, and the obligation that survives declining -- the one obligation this walk added to the summary. |
| `6.4.1` | The contents of inverse queries and responses | 0 | walked | The contents of inverse queries and responses. Descriptive. |
| `6.4.2` | Inverse query and response example | 0 | walked | Inverse query and response example. Illustrative. |
| `6.4.3` | Inverse query processing | 0 | walked | Inverse query processing. Describes the optional algorithm. |
| `6.5` | not stated | 0 | walked | Completion queries and responses: obsoleted by this RFC and retained for historical reference only. |
| `7` | RESOLVER IMPLEMENTATION heading | 0 | walked | RESOLVER IMPLEMENTATION heading. Every obligation under it binds a resolver, a role the authoritative-server surface this summary scopes does not play. |
| `7.1` | Transforming a user request into a query | 4 | walked | Transforming a user request into a query. Four sites, all resolver obligations or descriptions of the suggested request structure. |
| `7.2` | Sending the queries | 1 | walked | Sending the queries. Its one site is resolver round-trip-time estimation. |
| `7.3` | Processing responses | 1 | walked | Processing responses. Its one site is resolver request matching. |
| `7.4` | Using the cache | 0 | walked | Using the cache. Ze holds no resolver cache. |
| `8` | MAIL SUPPORT heading | 0 | walked | MAIL SUPPORT heading. Ze emits no MX, MB, MG, MR or MINFO record and implements no mail binding. |
| `8.1` | Mail exchange binding | 0 | walked | Mail exchange binding. Describes the MX transition from MD/MF. |
| `8.2` | Mailbox binding (Experimental) | 0 | walked | Mailbox binding (Experimental). Experimental and unimplemented everywhere. |
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
