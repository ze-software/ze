# RFC 2181 - Clarifications to the DNS Specification

Partial. Every requirement this repository extracted from RFC 2181, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7.7% | 1 of 13 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 84.6% | 11 of 13 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 13 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 13 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 23 | of 70 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 10 | of 23 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 7.7% | 1 of 13 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 13 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 70 |
| Gated MUST-level | 23 |
| Obligations that bind Ze | 13 |
| Not applicable, so out of scope | 10 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 13 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2181.md` |
| Requirement shard | `rfc/requirements/rfc2181.md` |
| RFC text | `rfc/full/rfc2181.txt` |

## Enrolment

Enrolled: Clarifications to the DNS Specification (RFC 2181): ze is authoritative (internal/plugins/geodns, internal/plugins/as112) plus a stub resolver+cache (internal/component/resolve/dns). 1 MET (section 8 TTL 0..2147483647 bound, both polarities) + 11 single-polarity positive (UDP reply source-IP/port fidelity 4.1-1/4.2-1/4.2-2, RRSet equal TTLs 5.2-1, cache replaces RRSets without merging 5.4-1/5.4-2, canonical NS targets with A glue 10.3-1/10.3-2, wire label/name limits and unrestricted labels 11-1/11-2/11-3) + 1 gap (5.1-1 no TC/Truncate on oversized RRSet) + 10 not-applicable (SIG/DNSSEC 5.3.1-2/5.3.1-3/5.3.1-4/5.4.1-8/5.4.1-9, recursive-resolver ranking 5.4.1-3, AXFR 5.5-4, CNAME/PTR authoring 10.1-4/10.1.1-1/10.2-1)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Authoritative GeoDNS/AS112 answers with RRSet-consistent per-record TTLs, the section 8 0..2147483647 TTL bound ([`internal/plugins/geodns/config.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/config.go)), canonical NS targets with A glue, UDP reply source-address/port fidelity, wire label/name limits, and a stub-resolver cache that replaces whole RRSets without merging
- tests bound per requirement in [`rfc/short/rfc2181.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2181.md).


**What the ledger says remains**

One MUST gap ([`RFC2181-5.1-1`](#rfc2181-5.1-1)): GeoDNS and AS112 never set the TC bit or call miekg Truncate, so an oversized RRSet would be sent unmarked rather than truncated. The recursive-resolver data-ranking, DNSSEC/SIG, AXFR, and CNAME/PTR-authoring MUSTs are not-applicable (ze is authoritative plus a stub resolver only).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 22 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **23** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC2181-8-1`](#rfc2181-8-1)

**Annotated instead of tested (22):** [`RFC2181-4.1-1`](#rfc2181-4.1-1), [`RFC2181-4.2-1`](#rfc2181-4.2-1), [`RFC2181-4.2-2`](#rfc2181-4.2-2), [`RFC2181-5.1-1`](#rfc2181-5.1-1), [`RFC2181-5.2-1`](#rfc2181-5.2-1), [`RFC2181-5.3.1-2`](#rfc2181-5.3.1-2), [`RFC2181-5.3.1-3`](#rfc2181-5.3.1-3), [`RFC2181-5.3.1-4`](#rfc2181-5.3.1-4), [`RFC2181-5.4-1`](#rfc2181-5.4-1), [`RFC2181-5.4-2`](#rfc2181-5.4-2), [`RFC2181-5.4.1-3`](#rfc2181-5.4.1-3), [`RFC2181-5.4.1-8`](#rfc2181-5.4.1-8), [`RFC2181-5.4.1-9`](#rfc2181-5.4.1-9), [`RFC2181-5.5-4`](#rfc2181-5.5-4), [`RFC2181-10.1-4`](#rfc2181-10.1-4), [`RFC2181-10.1.1-1`](#rfc2181-10.1.1-1), [`RFC2181-10.2-1`](#rfc2181-10.2-1), [`RFC2181-10.3-1`](#rfc2181-10.3-1), [`RFC2181-10.3-2`](#rfc2181-10.3-2), [`RFC2181-11-1`](#rfc2181-11-1), [`RFC2181-11-2`](#rfc2181-11-2), [`RFC2181-11-3`](#rfc2181-11-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2181-4.1-1` | When responding to a query over UDP, a server must send the reply with the IP source address set to the address that was in the destination address field of the query packet. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L315). **negative:** no negative test. **{single-polarity}:** each UDP listener binds one specific IP at internal/core/dnsserver/manager.go:160 so the kernel sources every reply from the query destination address, and ze has no wildcard-bind or explicit-source path that could send from another address |
| `RFC2181-4.2-1` | Replies to all queries must be directed to the port from which they were sent. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L320). **negative:** no negative test. **{single-polarity}:** the reply is written on the same socket the query arrived on via internal/core/dnsserver/handler.go:62 so miekg/dns directs it to the query source port, a property ze cannot violate |
| `RFC2181-4.2-2` | For queries received by UDP, the server must take note of the source port and use it as the destination port in the response. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L325). **negative:** no negative test. **{single-polarity}:** miekg/dns ServeUDP records the datagram source port and uses it as the reply destination for the write at internal/core/dnsserver/handler.go:62, so ze always answers to the query source port |
| `RFC2181-5.1-1` | The response must be marked "truncated" if the entire RRSet will not fit in the response. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns and as112 never set the TC bit or call miekg Truncate, and miekg WriteMsg at vendor/github.com/miekg/dns/server.go:747 packs and sends without auto-truncating, so an oversized RRSet would be sent unmarked |
| `RFC2181-5.2-1` | The TTLs of all RRs in an RRSet must be the same; in no case may a server send an RRSet with TTLs not all equal. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC2181_RRSetEqualTTL`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L367). **negative:** no negative test. **{single-polarity}:** geodns assigns one TTL per host record set at internal/plugins/geodns/config.go:271 and as112 uses fixed per-zone TTL constants, so an emitted RRSet never carries unequal TTLs and no code path can produce one |
| `RFC2181-5.3.1-2` | Where SIG records are returned in the answer section (a query for SIG records, or type=ANY), the entire SIG RRSet must be included, as for any other RR type. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no SIG or DNSSEC records -- geodns emits only A/AAAA/SRV at internal/plugins/geodns/record.go:10 and as112 only SOA/NS/TXT, so there is no SIG RRSet to include |
| `RFC2181-5.3.1-3` | A server receiving SIG records in the authority section (or, probably incorrectly, as additional data) must understand that the entire RRSet has almost certainly not been included. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's stub resolver extracts only answer-section A/AAAA/TXT/PTR/CNAME/MX/NS/SRV at internal/component/resolve/dns/resolver.go:299 and processes no SIG records, so there is no partial SIG RRSet to reason about |
| `RFC2181-5.3.1-4` | Such a server must not cache that SIG record in a way that would permit it to be returned in response to a query for SIG records. (§5.3.1) | MUST NOT | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the resolver caches only records it extracted from the answer section at internal/component/resolve/dns/resolver.go:299 and never handles SIG, so an authority-section SIG can never be cached or returned |
| `RFC2181-5.4-1` | Servers must never merge RRs from a response with RRs in their cache to form an RRSet. (§5.4) | MUST NOT | 5.4 | **positive:** `unit/verify` [`TestRFC2181_CacheReplacesRRSetNoMerge`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/cache_test.go#L226). **negative:** no negative test. **{single-polarity}:** the resolver cache replaces the whole RRSet for a name+type at internal/component/resolve/dns/cache.go:145 by removing the existing entry before storing the new records, so response RRs are never merged with cached ones |
| `RFC2181-5.4-2` | When a response would form an RRSet with cached data, the server must either ignore the response RRs or discard the entire cached RRSet, as appropriate. (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC2181_CacheReplacesRRSetNoMerge`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/cache_test.go#L234). **negative:** no negative test. **{single-polarity}:** put discards the entire cached RRSet before storing the new answer at internal/component/resolve/dns/cache.go:145, taking the discard-cached branch of the rule rather than merging |
| `RFC2181-5.4.1-3` | Data trustworthiness shall rank, most to least: primary zone file (non-glue), zone transfer (non-glue), authoritative answer-section data, authority-section data of an authoritative answer, glue, non-authoritative answer data, then additional information and non-authoritative authority-section data. (§5.4.1) | SHALL | 5.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's resolver is a stub forwarder to one configured upstream at internal/component/resolve/dns/resolver.go:263 with a single-source cache keyed by name+type, so it never ranks data from competing trustworthiness sources |
| `RFC2181-5.4.1-8` | When DNS security is in use and an authenticated reply has been received and verified, the authenticated data shall be considered more trustworthy than unauthenticated data of the same type. (§5.4.1) | SHALL | 5.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the stub resolver performs no per-record trust ranking -- it relies on a validating upstream returning SERVFAIL at internal/component/resolve/dns/resolver.go:99 rather than comparing authenticated against unauthenticated data |
| `RFC2181-5.4.1-9` | DNSSEC-aware servers must still correctly set the AA bit in responses, to enable correct operation with servers that are not security aware. (§5.4.1) | MUST | 5.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers implement no DNSSEC signing so the DNSSEC-aware precondition does not hold; the AA bit is nonetheless always set at internal/core/dnsserver/handler.go:73 |
| `RFC2181-5.5-4` | Where a duplicate RRSet is required (e.g. the SOA at the first and last record of an AXFR), the TTL transmitted in each case must be the same. (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no AXFR -- the geodns YANG notes this at internal/plugins/geodns/yang/ze-geodns-conf.yang:95 and no plugin emits an SOA twice in one message, so no duplicate RRSet arises |
| `RFC2181-8-1` | A TTL is an unsigned number in the range 0..2147483647 (2^31 - 1); when transmitted it shall be encoded in the less significant 31 bits of the 32-bit TTL field, with the most significant (sign) bit set to zero. (§8) | SHALL | 8 | **positive:** `unit/verify` [`TestRFC2181_TTLSignBitBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/config_test.go#L225). **negative:** `unit/verify` [`TestRFC2181_TTLSignBitBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/config_test.go#L238) |
| `RFC2181-10.1-4` | An alias (the label of a CNAME record) may have no data other than SIG, NXT, and KEY RRs; a CNAME must not coexist with any other data. (§10.1) | MUST NOT | 10.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no CNAME records -- geodns emits only A/AAAA/SRV at internal/plugins/geodns/record.go:10 and as112 only SOA/NS/TXT, so no CNAME can coexist with other data |
| `RFC2181-10.1.1-1` | Care must be taken to be very clear whether the label or the value (the canonical name) of a CNAME resource record is intended. (§10.1.1) | MUST | 10.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze authors no CNAME records at internal/plugins/geodns/record.go:10, so there is no label-versus-canonical-name ambiguity for an implementation to resolve |
| `RFC2181-10.2-1` | The value of a PTR record must not be an alias; it should be a canonical name. (§10.2) | MUST NOT | 10.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze authors no PTR records -- geodns serves A/AAAA/SRV and as112 reverse zones return NODATA with the SOA at internal/plugins/as112/zones.go:273 rather than any PTR, so no PTR value can be an alias |
| `RFC2181-10.3-1` | The domain name used as the value of an NS record, or as part of the value of an MX record, must not be an alias, and must never have a CNAME RR. (§10.3) | MUST NOT | 10.3 | **positive:** `unit/verify` [`TestRFC2181_NSCanonicalWithGlue`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L407). **negative:** no negative test. **{single-polarity}:** geodns synthesizes NS targets as canonical ns<n>.<zone> names at internal/plugins/geodns/server.go:154 and as112 uses fixed canonical names, so ze never emits a CNAME as an NS or MX value and serves no MX at all |
| `RFC2181-10.3-2` | That domain name must have as its value one or more address records. (§10.3) | MUST | 10.3 | **positive:** `unit/verify` [`TestRFC2181_NSCanonicalWithGlue`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L424). **negative:** no negative test. **{single-polarity}:** geodns emits A glue for every synthesized NS target at internal/plugins/geodns/server.go:161, and as112 NS targets are canonical names whose address records are authoritative elsewhere, so the target name always has address records |
| `RFC2181-11-1` | Any one label is limited to between 1 and 63 octets; a full domain name is limited to 255 octets, including the separators. (§11) | MUST | 11 | **positive:** `unit/verify` [`TestRFC2181_WireNameLimits`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L451). **negative:** no negative test. **{single-polarity}:** the DNS wire codec ze packs and unpacks through rejects a label of 64+ octets at vendor/github.com/miekg/dns/msg.go:281 and caps a name at 255, and geodns/as112 emit only short synthetic names, so no over-limit name is produced |
| `RFC2181-11-2` | Implementations of the DNS protocols must not place any restrictions on the labels that can be used. (§11) | MUST NOT | 11 | **positive:** `unit/verify` [`TestRFC2181_LabelsUnrestricted`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/config_test.go#L271). **negative:** no negative test. **{single-polarity}:** geodns applies no label-content restriction -- parseHost at internal/plugins/geodns/config.go:270 accepts any label characters and only requires a configured-zone suffix, so underscore and other non-hostname labels are served |
| `RFC2181-11-3` | DNS servers must not refuse to serve a zone because it contains labels that might not be acceptable to some DNS client programs. (§11) | MUST NOT | 11 | **positive:** `unit/verify` [`TestRFC2181_LabelsUnrestricted`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/config_test.go#L260). **negative:** no negative test. **{single-polarity}:** geodns never refuses a zone for questionable labels -- config parsing at internal/plugins/geodns/config.go:246 rejects only a missing zone suffix or an invalid IP, never label characters |
| `RFC2181-4.1-3` | If the required source address is not permitted for this purpose, the legal source address chosen should be one that maximises the possibility that the client can use it for further queries. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-4.2-3` | Replies should always be sent from the port to which they were directed. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5-1` | Servers should suppress duplicate RRs (equal label, class, type, and data) if encountered. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.2-2` | A client that receives a response containing an RRSet whose RRs have differing TTLs should treat this as an error. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.2-3` | If such an RRSet is from a non-authoritative source, the client should ignore the RRSet and, if the values are required, seek to acquire them from an authoritative source. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.2-4` | Clients configured to send all queries to one or more particular servers should treat those servers as authoritative for this purpose. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.2-5` | Should an authoritative source send such a malformed RRSet, the client should treat all its RRs as if every TTL had been set to the value of the lowest TTL in the RRSet. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4-4` | A server should update a cached RRSet's TTL from an identical received answer only if that answer would be considered more authoritative than the previously cached answer. (§5.4) | SHOULD | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-1` | When deciding whether to accept a reply's RRSet or retain one already cached, a server should consider the relative likely trustworthiness of the various data. (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-2` | An authoritative answer from a reply should replace cached data that had been obtained from additional information in an earlier reply. (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-4` | Clients should assume that records other than the alias record in an authoritative answer may have come from the server's cache. (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-5` | Where authoritative answers are required, the client should query again using the canonical name associated with the alias. (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-6` | Unauthenticated RRs cached from the least trustworthy groupings (additional data, and the authority section of a non-authoritative answer) should not be cached in such a way that they would ever be returned as answers to a received query. (§5.4.1) | SHOULD NOT | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-10` | Where glue for the same name exists in multiple zones and differs in value, the nameserver should select data from a primary zone file in preference to secondary. (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-12` | Where a server can detect from two zone files that one or more are incorrectly configured so as to create conflicts, it should refuse to load the zones determined to be erroneous and issue suitable diagnostics. (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.5-1` | A Resource Record Set should only be included once in any DNS reply. (§5.5) | SHOULD | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.5-3` | An RRSet should not be repeated in the same or any other section, except where explicitly required by a specification. (§5.5) | SHOULD NOT | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-6.1-1` | A server for a zone should not return authoritative answers for queries related to names in another zone (including the NS, and perhaps A, records at a zone cut) unless it also happens to be a server for the other zone. (§6.1) | SHOULD NOT | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-6.1-2` | Servers should ignore data other than NS records, and the A records necessary to locate the servers listed in those NS records, that may happen to be configured in a zone at a zone cut. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-6.2-2` | Where a subzone is secure, its KEY and SIG records should also always be present in the parent zone (if secure). (§6.2) | SHOULD | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-6.2-3` | In none of these zone-cut cases should a server for the parent zone, not also being a server for the subzone, set the AA bit in any response for a label at a zone cut. (§6.2) | SHOULD NOT | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-7.2-1` | Implementations should not assume that SOA records will have a TTL of zero. (§7.2) | SHOULD NOT | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-7.3-1` | The MNAME field of the SOA record should contain the name of the primary (master) server for the zone identified by the SOA. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-7.3-2` | The SOA MNAME field should not contain the name of the zone itself. (§7.3) | SHOULD NOT | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-8-2` | Implementations should treat TTL values received with the most significant bit set as if the entire value received was zero. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-9-1` | The TC bit should be set in responses only when an RRSet is required as part of the response but could not be included in its entirety. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-9-2` | The TC bit should not be set merely because some extra information (including additional-section processing) could have been included but there was insufficient room. (§9) | SHOULD NOT | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-9-3` | In such cases the entire RRSet that will not fit should be omitted and the reply sent as is, with the TC bit clear. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-9-5` | When a client receives a reply with TC set, it should ignore that response and query again using a mechanism, such as a TCP connection, that will permit larger replies. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-10.1-2` | The canonical name given by a CNAME record should generally be a name that exists elsewhere in the DNS. (§10.1) | SHOULD | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-10.2-2` | No restriction that only one PTR record is permitted for a name should be inferred. (§10.2) | SHOULD NOT | 10.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-11-5` | Warning about, or refusing to load, a primary zone because of questionable labels should not happen by default. (§11) | SHOULD NOT | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-4.1-2` | If setting the required source address is not permitted for this purpose, the response may be sent from any legal IP address allocated to the server. (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.3.1-1` | The authority section may contain only those SIG RRs whose "type covered" field equals the type field of an answer being returned. (§5.3.1) | MAY | 5.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.3.2-1` | Servers are not required to treat two differing NXT RRSets as a special case; they may elect to notice the two NXT RRSets and treat them as they would any two different RRSets (cache one, ignore the other). (§5.3.2) | MAY | 5.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4-3` | When a received answer contains an RRSet identical to the cached one except for the TTL value, the server may optionally update the TTL in its cache with the TTL of the received answer. (§5.4) | MAY | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-7` | Such untrustworthy RRs may be returned as additional information where appropriate. (§5.4.1) | MAY | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-11` | Where conflicting glue exists, the nameserver may otherwise choose any single set of such data. (§5.4.1) | MAY | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.5-2` | An RRSet may occur in any of the Answer, Authority, or Additional Information sections, as required. (§5.5) | MAY | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-6.2-1` | Servers may, but are not required to, retain all differing NXT records they receive, regardless of the rules in section 5.4. (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-7.1-1` | The authority section of an authoritative answer may contain the SOA record for the zone; SOA records, if added, are to be placed in the authority section. (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-7.2-2` | Implementations are not required to send SOA records with a TTL of zero (they may send a non-zero SOA TTL). (§7.2) | MAY | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-8-3` | Implementations are always free to place an upper bound on any received TTL and treat any larger values as if they were that upper bound. (§8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-9-4` | Where TC is set, the partial RRSet that would not completely fit may be left in the response. (§9) | MAY | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-10.1-1` | There may be only one canonical name for any one alias. (§10.1) | MAY | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-10.1-3` | An alias (the label of a CNAME record) may, if DNSSEC is in use, have SIG, NXT, and KEY RRs. (§10.1) | MAY | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-11-4` | A DNS server may be configurable to issue warnings when loading, or even to refuse to load, a primary zone containing labels that might be considered questionable. (§11) | MAY | 11 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2181-5.1-1`](#rfc2181-5.1-1) The response must be marked "truncated" if the entire RRSet will not fit in the response. (§5.1) | {gap}, no test | geodns and as112 never set the TC bit or call miekg Truncate, and miekg WriteMsg at vendor/github.com/miekg/dns/server.go:747 packs and sends without auto-truncating, so an oversized RRSet would be sent unmarked |
| [`RFC2181-5.3.1-2`](#rfc2181-5.3.1-2) Where SIG records are returned in the answer section (a query for SIG records, or type=ANY), the entire SIG RRSet must be included, as for any other RR type. (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no SIG or DNSSEC records -- geodns emits only A/AAAA/SRV at internal/plugins/geodns/record.go:10 and as112 only SOA/NS/TXT, so there is no SIG RRSet to include |
| [`RFC2181-5.3.1-3`](#rfc2181-5.3.1-3) A server receiving SIG records in the authority section (or, probably incorrectly, as additional data) must understand that the entire RRSet has almost certainly not been included. (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's stub resolver extracts only answer-section A/AAAA/TXT/PTR/CNAME/MX/NS/SRV at internal/component/resolve/dns/resolver.go:299 and processes no SIG records, so there is no partial SIG RRSet to reason about |
| [`RFC2181-5.3.1-4`](#rfc2181-5.3.1-4) Such a server must not cache that SIG record in a way that would permit it to be returned in response to a query for SIG records. (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: the resolver caches only records it extracted from the answer section at internal/component/resolve/dns/resolver.go:299 and never handles SIG, so an authority-section SIG can never be cached or returned |
| [`RFC2181-5.4.1-3`](#rfc2181-5.4.1-3) Data trustworthiness shall rank, most to least: primary zone file (non-glue), zone transfer (non-glue), authoritative answer-section data, authority-section data of an authoritative answer, glue, non-authoritative answer data, then additional information and non-authoritative authority-section data. (§5.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's resolver is a stub forwarder to one configured upstream at internal/component/resolve/dns/resolver.go:263 with a single-source cache keyed by name+type, so it never ranks data from competing trustworthiness sources |
| [`RFC2181-5.4.1-8`](#rfc2181-5.4.1-8) When DNS security is in use and an authenticated reply has been received and verified, the authenticated data shall be considered more trustworthy than unauthenticated data of the same type. (§5.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: the stub resolver performs no per-record trust ranking -- it relies on a validating upstream returning SERVFAIL at internal/component/resolve/dns/resolver.go:99 rather than comparing authenticated against unauthenticated data |
| [`RFC2181-5.4.1-9`](#rfc2181-5.4.1-9) DNSSEC-aware servers must still correctly set the AA bit in responses, to enable correct operation with servers that are not security aware. (§5.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers implement no DNSSEC signing so the DNSSEC-aware precondition does not hold; the AA bit is nonetheless always set at internal/core/dnsserver/handler.go:73 |
| [`RFC2181-5.5-4`](#rfc2181-5.5-4) Where a duplicate RRSet is required (e.g. the SOA at the first and last record of an AXFR), the TTL transmitted in each case must be the same. (§5.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no AXFR -- the geodns YANG notes this at internal/plugins/geodns/yang/ze-geodns-conf.yang:95 and no plugin emits an SOA twice in one message, so no duplicate RRSet arises |
| [`RFC2181-10.1-4`](#rfc2181-10.1-4) An alias (the label of a CNAME record) may have no data other than SIG, NXT, and KEY RRs; a CNAME must not coexist with any other data. (§10.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no CNAME records -- geodns emits only A/AAAA/SRV at internal/plugins/geodns/record.go:10 and as112 only SOA/NS/TXT, so no CNAME can coexist with other data |
| [`RFC2181-10.1.1-1`](#rfc2181-10.1.1-1) Care must be taken to be very clear whether the label or the value (the canonical name) of a CNAME resource record is intended. (§10.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze authors no CNAME records at internal/plugins/geodns/record.go:10, so there is no label-versus-canonical-name ambiguity for an implementation to resolve |
| [`RFC2181-10.2-1`](#rfc2181-10.2-1) The value of a PTR record must not be an alias; it should be a canonical name. (§10.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze authors no PTR records -- geodns serves A/AAAA/SRV and as112 reverse zones return NODATA with the SOA at internal/plugins/as112/zones.go:273 rather than any PTR, so no PTR value can be an alias |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2181-4.1-1`](#rfc2181-4.1-1)

When responding to a query over UDP, a server must send the reply with the IP source address set to the address that was in the destination address field of the query packet. (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L315) | unit/verify | unproven |

### [`RFC2181-4.2-1`](#rfc2181-4.2-1)

Replies to all queries must be directed to the port from which they were sent. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L320) | unit/verify | unproven |

### [`RFC2181-4.2-2`](#rfc2181-4.2-2)

For queries received by UDP, the server must take note of the source port and use it as the destination port in the response. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L325) | unit/verify | unproven |

### [`RFC2181-5.1-1`](#rfc2181-5.1-1)

The response must be marked "truncated" if the entire RRSet will not fit in the response. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.1-1, so no unit is bound to it.

### [`RFC2181-5.2-1`](#rfc2181-5.2-1)

The TTLs of all RRs in an RRSet must be the same; in no case may a server send an RRSet with TTLs not all equal. (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_RRSetEqualTTL`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L367) | unit/verify | unproven |

### [`RFC2181-5.3.1-2`](#rfc2181-5.3.1-2)

Where SIG records are returned in the answer section (a query for SIG records, or type=ANY), the entire SIG RRSet must be included, as for any other RR type. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.3.1-2, so no unit is bound to it.

### [`RFC2181-5.3.1-3`](#rfc2181-5.3.1-3)

A server receiving SIG records in the authority section (or, probably incorrectly, as additional data) must understand that the entire RRSet has almost certainly not been included. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.3.1-3, so no unit is bound to it.

### [`RFC2181-5.3.1-4`](#rfc2181-5.3.1-4)

Such a server must not cache that SIG record in a way that would permit it to be returned in response to a query for SIG records. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.3.1-4, so no unit is bound to it.

### [`RFC2181-5.4-1`](#rfc2181-5.4-1)

Servers must never merge RRs from a response with RRs in their cache to form an RRSet. (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_CacheReplacesRRSetNoMerge`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/cache_test.go#L226) | unit/verify | unproven |

### [`RFC2181-5.4-2`](#rfc2181-5.4-2)

When a response would form an RRSet with cached data, the server must either ignore the response RRs or discard the entire cached RRSet, as appropriate. (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_CacheReplacesRRSetNoMerge`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/cache_test.go#L234) | unit/verify | unproven |

### [`RFC2181-5.4.1-3`](#rfc2181-5.4.1-3)

Data trustworthiness shall rank, most to least: primary zone file (non-glue), zone transfer (non-glue), authoritative answer-section data, authority-section data of an authoritative answer, glue, non-authoritative answer data, then additional information and non-authoritative authority-section data. (§5.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.4.1-3, so no unit is bound to it.

### [`RFC2181-5.4.1-8`](#rfc2181-5.4.1-8)

When DNS security is in use and an authenticated reply has been received and verified, the authenticated data shall be considered more trustworthy than unauthenticated data of the same type. (§5.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.4.1-8, so no unit is bound to it.

### [`RFC2181-5.4.1-9`](#rfc2181-5.4.1-9)

DNSSEC-aware servers must still correctly set the AA bit in responses, to enable correct operation with servers that are not security aware. (§5.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.4.1-9, so no unit is bound to it.

### [`RFC2181-5.5-4`](#rfc2181-5.5-4)

Where a duplicate RRSet is required (e.g. the SOA at the first and last record of an AXFR), the TTL transmitted in each case must be the same. (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.5-4, so no unit is bound to it.

### [`RFC2181-8-1`](#rfc2181-8-1)

A TTL is an unsigned number in the range 0..2147483647 (2^31 - 1); when transmitted it shall be encoded in the less significant 31 bits of the 32-bit TTL field, with the most significant (sign) bit set to zero. (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2181_TTLSignBitBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/config_test.go#L238) | unit/verify | unproven |
| positive | [`TestRFC2181_TTLSignBitBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/config_test.go#L225) | unit/verify | unproven |

### [`RFC2181-10.1-4`](#rfc2181-10.1-4)

An alias (the label of a CNAME record) may have no data other than SIG, NXT, and KEY RRs; a CNAME must not coexist with any other data. (§10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-10.1-4, so no unit is bound to it.

### [`RFC2181-10.1.1-1`](#rfc2181-10.1.1-1)

Care must be taken to be very clear whether the label or the value (the canonical name) of a CNAME resource record is intended. (§10.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-10.1.1-1, so no unit is bound to it.

### [`RFC2181-10.2-1`](#rfc2181-10.2-1)

The value of a PTR record must not be an alias; it should be a canonical name. (§10.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-10.2-1, so no unit is bound to it.

### [`RFC2181-10.3-1`](#rfc2181-10.3-1)

The domain name used as the value of an NS record, or as part of the value of an MX record, must not be an alias, and must never have a CNAME RR. (§10.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_NSCanonicalWithGlue`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L407) | unit/verify | unproven |

### [`RFC2181-10.3-2`](#rfc2181-10.3-2)

That domain name must have as its value one or more address records. (§10.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_NSCanonicalWithGlue`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L424) | unit/verify | unproven |

### [`RFC2181-11-1`](#rfc2181-11-1)

Any one label is limited to between 1 and 63 octets; a full domain name is limited to 255 octets, including the separators. (§11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_WireNameLimits`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/server_test.go#L451) | unit/verify | unproven |

### [`RFC2181-11-2`](#rfc2181-11-2)

Implementations of the DNS protocols must not place any restrictions on the labels that can be used. (§11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_LabelsUnrestricted`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/config_test.go#L271) | unit/verify | unproven |

### [`RFC2181-11-3`](#rfc2181-11-3)

DNS servers must not refuse to serve a zone because it contains labels that might not be acceptable to some DNS client programs. (§11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_LabelsUnrestricted`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/config_test.go#L260) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 2181, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2181, so its obligations are stated where they were written.
