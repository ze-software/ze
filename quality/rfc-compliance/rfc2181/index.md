# RFC 2181 - Clarifications to the DNS Specification

Partial. Every requirement this repository extracted from RFC 2181, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 13.0% | 3 of 23 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 39.1% | 9 of 23 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 23 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 23 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 65.0% | 13 of 20 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 23 | of 70 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 10 | of 23 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 43.5% | 10 of 23 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 23 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 23 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 4.3% | 1 of 23 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 23 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 70 |
| Gated MUST-level | 23 |
| Not applicable, so out of scope | 10 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 20 |
| Tagged units | 20 |
| Recorded audit verdicts | 8 |
| Discrimination records | 13 |
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
| Positive and negative tests | 3 | one part of the gated population |
| Annotated (including scoped evidence) | 20 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **23** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC2181-5.4-2`](#rfc2181-5.4-2), [`RFC2181-8-1`](#rfc2181-8-1), [`RFC2181-11-1`](#rfc2181-11-1)

**Annotated (including scoped evidence) (20):** [`RFC2181-4.1-1`](#rfc2181-4.1-1), [`RFC2181-4.2-1`](#rfc2181-4.2-1), [`RFC2181-4.2-2`](#rfc2181-4.2-2), [`RFC2181-5.1-1`](#rfc2181-5.1-1), [`RFC2181-5.2-1`](#rfc2181-5.2-1), [`RFC2181-5.3.1-2`](#rfc2181-5.3.1-2), [`RFC2181-5.3.1-3`](#rfc2181-5.3.1-3), [`RFC2181-5.3.1-4`](#rfc2181-5.3.1-4), [`RFC2181-5.4-1`](#rfc2181-5.4-1), [`RFC2181-5.4.1-3`](#rfc2181-5.4.1-3), [`RFC2181-5.4.1-8`](#rfc2181-5.4.1-8), [`RFC2181-5.4.1-9`](#rfc2181-5.4.1-9), [`RFC2181-5.5-4`](#rfc2181-5.5-4), [`RFC2181-10.1-4`](#rfc2181-10.1-4), [`RFC2181-10.1.1-1`](#rfc2181-10.1.1-1), [`RFC2181-10.2-1`](#rfc2181-10.2-1), [`RFC2181-10.3-1`](#rfc2181-10.3-1), [`RFC2181-10.3-2`](#rfc2181-10.3-2), [`RFC2181-11-2`](#rfc2181-11-2), [`RFC2181-11-3`](#rfc2181-11-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2181-4.1-1` | To avoid these problems, servers when responding to queries using UDP must cause the reply to be sent with the source address field in the IP header set to the address that was in the destination address field of the IP header of the packet containing the query causing the response. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC2181ReplySourcedFromTheQueriedAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_clarifications_test.go#L56). **positive:** `unit/verify` [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L345). **negative:** no negative test. **{single-polarity}:** each UDP listener binds one specific IP at internal/core/dnsserver/manager.go:160 so the kernel sources every reply from the query destination address, and ze has no wildcard-bind or explicit-source path that could send from another address |
| `RFC2181-4.2-1` | Replies to all queries must be directed to the port from which they were sent. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L350). **negative:** no negative test. **{single-polarity}:** the reply is written on the same socket the query arrived on via internal/core/dnsserver/handler.go:62 so miekg/dns directs it to the query source port, a property ze cannot violate |
| `RFC2181-4.2-2` | For queries received by UDP the server must take note of the source port and use that as the destination port in the response. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L355). **negative:** no negative test. **{single-polarity}:** miekg/dns ServeUDP records the datagram source port and uses it as the reply destination for the write at internal/core/dnsserver/handler.go:62, so ze always answers to the query source port |
| `RFC2181-5.1-1` | The response must be marked as "truncated" if the entire RRSet will not fit in the response. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns and as112 never set the TC bit or call miekg Truncate, and miekg WriteMsg at vendor/github.com/miekg/dns/server.go:747 packs and sends without auto-truncating, so an oversized RRSet would be sent unmarked |
| `RFC2181-5.2-1` | Consequently the use of differing TTLs in an RRSet is hereby deprecated, the TTLs of all RRs in an RRSet must be the same. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC2181_RRSetEqualTTL`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L397). **negative:** no negative test. **{single-polarity}:** geodns assigns one TTL per host record set at internal/plugins/geodns/config.go:271 and as112 uses fixed per-zone TTL constants, so an emitted RRSet never carries unequal TTLs and no code path can produce one |
| `RFC2181-5.3.1-2` | However, where SIG records are being returned in the answer section, in response to a query for SIG records, or a query for all records associated with a name (type=ANY) the entire SIG RRSet must be included, as for any other RR type. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no SIG or DNSSEC records -- geodns emits only A/AAAA/SRV at internal/plugins/geodns/record.go:10 and as112 only SOA/NS/TXT, so there is no SIG RRSet to include |
| `RFC2181-5.3.1-3` | Servers that receive responses containing SIG records in the authority section, or (probably incorrectly) as additional data, must understand that the entire RRSet has almost certainly not been included. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's stub resolver extracts only answer-section A/AAAA/TXT/PTR/CNAME/MX/NS/SRV at internal/component/resolve/dns/resolver.go:299 and processes no SIG records, so there is no partial SIG RRSet to reason about |
| `RFC2181-5.3.1-4` | Thus, they must not cache that SIG record in a way that would permit it to be returned should a query for SIG records be received at that server. (§5.3.1) | MUST NOT | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the resolver caches only records it extracted from the answer section at internal/component/resolve/dns/resolver.go:299 and never handles SIG, so an authority-section SIG can never be cached or returned |
| `RFC2181-5.4-1` | Servers must never merge RRs from a response with RRs in their cache to form an RRSet. (§5.4) | MUST NOT | 5.4 | **positive:** `unit/verify` [`TestRFC2181_CacheReplacesRRSetNoMerge`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc2181_cache_test.go#L226). **negative:** no negative test. **{single-polarity}:** the resolver cache replaces the whole RRSet for a name+type at internal/component/resolve/dns/cache.go:145 by removing the existing entry before storing the new records, so response RRs are never merged with cached ones |
| `RFC2181-5.4-2` | If a response contains data that would form an RRSet with data in a server's cache the server must either ignore the RRs in the response, or discard the entire RRSet currently in the cache, as appropriate. (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC2181FormingAnRRSetDiscardsTheWholeCachedSet`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc2181_cache_rrset_test.go#L21). **positive:** `unit/verify` [`TestRFC2181_CacheReplacesRRSetNoMerge`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc2181_cache_test.go#L234). **negative:** `unit/verify` [`TestRFC2181FormingAnRRSetDiscardsTheWholeCachedSet`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc2181_cache_rrset_test.go#L31) |
| `RFC2181-5.4.1-3` | Trustworthiness shall be, in order from most to least: + Data from a primary zone file, other than glue data, + Data from a zone transfer, other than glue, + The authoritative data included in the answer section of an authoritative reply. + Data from the authority section of an authoritative answer, + Glue from a primary zone, or glue from a zone transfer, + Data from the answer section of a non-authoritative answer, and non-authoritative data from the answer section of authoritative answers, + Additional information from an authoritative answer, Data from the authority section of a non-authoritative answer, Additional information from non-authoritative answers. (§5.4.1) | SHALL | 5.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's resolver is a stub forwarder to one configured upstream at internal/component/resolve/dns/resolver.go:263 with a single-source cache keyed by name+type, so it never ranks data from competing trustworthiness sources |
| `RFC2181-5.4.1-8` | When DNS security [RFC2065] is in use, and an authenticated reply has been received and verified, the data thus authenticated shall be considered more trustworthy than unauthenticated data of the same type. (§5.4.1) | SHALL | 5.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the stub resolver performs no per-record trust ranking -- it relies on a validating upstream returning SERVFAIL at internal/component/resolve/dns/resolver.go:99 rather than comparing authenticated against unauthenticated data |
| `RFC2181-5.4.1-9` | However DNSSEC aware servers must still correctly set the AA bit in responses to enable correct operation with servers that are not security aware (almost all currently). (§5.4.1) | MUST | 5.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's authoritative servers implement no DNSSEC signing so the DNSSEC-aware precondition does not hold; the AA bit is nonetheless always set at internal/core/dnsserver/handler.go:73 |
| `RFC2181-5.5-4` | Where duplicates are required this way, the TTL transmitted in each case must be the same. (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no AXFR -- the geodns YANG notes this at internal/plugins/geodns/yang/ze-geodns-conf.yang:95 and no plugin emits an SOA twice in one message, so no duplicate RRSet arises |
| `RFC2181-8-1` | It is hereby specified that a TTL value is an unsigned number, with a minimum value of 0, and a maximum value of 2147483647. That is, a maximum of 2^31 - 1. When transmitted, this value shall be encoded in the less significant 31 bits of the 32 bit TTL field, with the most significant, or sign, bit set to zero. (§8) | SHALL | 8 | **positive:** `unit/verify` [`TestRFC2181TTLTransmittedWithSignBitClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_clarifications_test.go#L171). **positive:** `unit/verify` [`TestRFC2181_TTLSignBitBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_config_test.go#L226). **negative:** `unit/verify` [`TestRFC2181_TTLSignBitBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_config_test.go#L239) |
| `RFC2181-10.1-4` | An alias name (label of a CNAME record) may, if DNSSEC is in use, have SIG, NXT, and KEY RRs, but may have no other data. (§10.1) | MUST NOT | 10.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze serves no CNAME records -- geodns emits only A/AAAA/SRV at internal/plugins/geodns/record.go:10 and as112 only SOA/NS/TXT, so no CNAME can coexist with other data |
| `RFC2181-10.1.1-1` | Care must therefore be taken to be very clear whether the label, or the value (the canonical name) of a CNAME resource record is intended. (§10.1.1) | MUST | 10.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze authors no CNAME records at internal/plugins/geodns/record.go:10, so there is no label-versus-canonical-name ambiguity for an implementation to resolve |
| `RFC2181-10.2-1` | Note that while the value of a PTR record must not be an alias, there is no requirement that the process of resolving a PTR record not encounter any aliases. (§10.2) | MUST NOT | 10.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze authors no PTR records -- geodns serves A/AAAA/SRV and as112 reverse zones return NODATA with the SOA at internal/plugins/as112/zones.go:273 rather than any PTR, so no PTR value can be an alias |
| `RFC2181-10.3-1` | The domain name used as the value of a NS resource record, or part of the value of a MX resource record must not be an alias. (§10.3) | MUST NOT | 10.3 | **positive:** `unit/verify` [`TestRFC2181NSTargetIsNotAnAlias`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_clarifications_test.go#L88). **positive:** `unit/verify` [`TestRFC2181_NSCanonicalWithGlue`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L437). **negative:** no negative test. **{single-polarity}:** geodns synthesizes NS targets as canonical ns<n>.<zone> names at internal/plugins/geodns/server.go:154 and as112 uses fixed canonical names, so ze never emits a CNAME as an NS or MX value and serves no MX at all |
| `RFC2181-10.3-2` | This domain name must have as its value one or more address records. (§10.3) | MUST | 10.3 | **positive:** `unit/verify` [`TestRFC2181_NSCanonicalWithGlue`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L454). **negative:** no negative test. **{single-polarity}:** geodns emits A glue for every synthesized NS target at internal/plugins/geodns/server.go:161, and as112 NS targets are canonical names whose address records are authoritative elsewhere, so the target name always has address records |
| `RFC2181-11-1` | The length of any one label is limited to between 1 and 63 octets. A full domain name is limited to 255 octets (including the separators). (§11) | MUST | 11 | **positive:** `unit/verify` [`TestRFC2181NameLimitsBothSidesOfEachBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_clarifications_test.go#L131). **positive:** `unit/verify` [`TestRFC2181_WireNameLimits`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L481). **negative:** `unit/verify` [`TestRFC2181NameLimitsBothSidesOfEachBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_clarifications_test.go#L145) |
| `RFC2181-11-2` | Implementations of the DNS protocols must not place any restrictions on the labels that can be used. (§11) | MUST NOT | 11 | **positive:** `unit/verify` [`TestRFC2181_LabelsUnrestricted`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_config_test.go#L272). **negative:** no negative test. **{single-polarity}:** geodns applies no label-content restriction -- parseHost at internal/plugins/geodns/config.go:270 accepts any label characters and only requires a configured-zone suffix, so underscore and other non-hostname labels are served |
| `RFC2181-11-3` | DNS servers must not refuse to serve a zone because it contains labels that might not be acceptable to some DNS client programs. (§11) | MUST NOT | 11 | **positive:** `unit/verify` [`TestRFC2181_LabelsUnrestricted`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_config_test.go#L261). **negative:** no negative test. **{single-polarity}:** geodns never refuses a zone for questionable labels -- config parsing at internal/plugins/geodns/config.go:246 rejects only a missing zone suffix or an invalid IP, never label characters |
| `RFC2181-4.1-3` | That address should be chosen to maximise the possibility that the client will be able to use it for further queries. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-4.2-3` | Replies should always be sent from the port to which they were directed. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5-1` | It is meaningless for two records to ever have label, class, type and data all equal - servers should suppress such duplicates if encountered. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.2-2` | Should a client receive a response containing RRs from an RRSet with differing TTLs, it should treat this as an error. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.2-3` | If the RRSet concerned is from a non-authoritative source for this data, the client should simply ignore the RRSet, and if the values were required, seek to acquire them from an authoritative source. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.2-4` | Clients that are configured to send all queries to one, or more, particular servers should treat those servers as authoritative for this purpose. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.2-5` | Should an authoritative source send such a malformed RRSet, the client should treat the RRs for all purposes as if all TTLs in the RRSet had been set to the value of the lowest TTL in the RRSet. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4-4` | It should do this if the received answer would be considered more authoritative (as discussed in the next section) than the previously cached answer. (§5.4) | SHOULD | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-1` | When considering whether to accept an RRSet in a reply, or retain an RRSet already in its cache instead, a server should consider the relative likely trustworthiness of the various data. (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-2` | An authoritative answer from a reply should replace cached data that had been obtained from additional information in an earlier reply. (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-4` | However when the name sought is an alias (see section 10.1.1) only the record describing that alias is necessarily authoritative. Clients should assume that other records may have come from the server's cache. (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-5` | Where authoritative answers are required, the client should query again, using the canonical name associated with the alias. (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-6` | Unauthenticated RRs received and cached from the least trustworthy of those groupings, that is data from the additional data section, and data from the authority section of a non-authoritative answer, should not be cached in such a way that they would ever be returned as answers to a received query. (§5.4.1) | SHOULD NOT | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-10` | Where glue for the same name exists in multiple zones, and differs in value, the nameserver should select data from a primary zone file in preference to secondary (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-12` | Where a server can detect from two zone files that one or more are incorrectly configured, so as to create conflicts, it should refuse to load the zones determined to be erroneous, and issue suitable diagnostics. (§5.4.1) | SHOULD | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.5-1` | A Resource Record Set should only be included once in any DNS reply. (§5.5) | SHOULD | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.5-3` | However it should not be repeated in the same, or any other, section, except where explicitly required by a specification. (§5.5) | SHOULD NOT | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-6.1-1` | A server for a zone should not return authoritative answers for queries related to names in another zone, which includes the NS, and perhaps A, records at a zone cut, unless it also happens to be a server for the other zone. (§6.1) | SHOULD NOT | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-6.1-2` | Other than the DNSSEC cases mentioned immediately below, servers should ignore data other than NS records, and necessary A records to locate the servers listed in the NS records, that may happen to be configured in a zone at a zone cut. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-6.2-2` | Where a subzone is secure, the KEY and SIG records will be present, and authoritative, in that zone, but should also always be present in the parent zone (if secure). (§6.2) | SHOULD | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-6.2-3` | Note that in none of these cases should a server for the parent zone, not also being a server for the subzone, set the AA bit in any response for a label at a zone cut. (§6.2) | SHOULD NOT | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-7.2-1` | Implementations should not assume that SOA records will have a TTL of zero (§7.2) | SHOULD NOT | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-7.3-1` | the MNAME field of the SOA record should contain the name of the primary (master) server for the zone identified by the SOA. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-7.3-2` | It should not contain the name of the zone itself. (§7.3) | SHOULD NOT | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-8-2` | Implementations should treat TTL values received with the most significant bit set as if the entire value received was zero. (§8) | SHOULD | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-9-1` | The TC bit should be set in responses only when an RRSet is required as a part of the response, but could not be included in its entirety. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-9-2` | The TC bit should not be set merely because some extra information could have been included, but there was insufficient room. This includes the results of additional section processing. (§9) | SHOULD NOT | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-9-3` | In such cases the entire RRSet that will not fit in the response should be omitted, and the reply sent as is, with the TC bit clear. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-9-5` | When a DNS client receives a reply with TC set, it should ignore that response, and query again, using a mechanism, such as a TCP connection, that will permit larger replies. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-10.1-2` | That name should generally be a name that exists elsewhere in the DNS, though there are some rare applications for aliases with the accompanying canonical name undefined in the DNS. (§10.1) | SHOULD | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-10.2-2` | There is no implication in that section that only one PTR record is permitted for a name. No such restriction should be inferred. (§10.2) | SHOULD NOT | 10.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-11-5` | A DNS server may be configurable to issue warnings when loading, or even to refuse to load, a primary zone containing labels that might be considered questionable, however this should not happen by default. (§11) | SHOULD NOT | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-4.1-2` | If this would cause the response to be sent from an IP address that is not permitted for this purpose, then the response may be sent from any legal IP address allocated to the server. (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.3.1-1` | Thus, it is specifically permitted for the authority section to contain only those SIG RRs with the "type covered" field equal to the type field of an answer being returned. (§5.3.1) | MAY | 5.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.3.2-1` | However, servers are not required to treat this as a special case when receiving NXT records in a response. They may elect to notice the existence of two different NXT RRSets, and treat that as they would two different RRSets of any other type. That is, cache one, and ignore the other. (§5.3.2) | MAY | 5.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4-3` | Note that if a server receives an answer containing an RRSet that is identical to that in its cache, with the possible exception of the TTL value, it may, optionally, update the TTL in its cache with the TTL of the received answer. (§5.4) | MAY | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-7` | They may be returned as additional information where appropriate. (§5.4.1) | MAY | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.4.1-11` | but otherwise may choose any single set of such data. (§5.4.1) | MAY | 5.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-5.5-2` | It may occur in any of the Answer, Authority, or Additional Information sections, as required. (§5.5) | MAY | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-6.2-1` | Since NXT records are intended to be automatically generated, rather than configured by DNS operators, servers may, but are not required to, retain all differing NXT records they receive regardless of the rules in section 5.4. (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-7.1-1` | RFC1034, in section 3.7, indicates that the authority section of an authoritative answer may contain the SOA record for the zone from which the answer was obtained. When discussing negative caching, RFC1034 section 4.3.4 refers to this technique but mentions the additional section of the response. The former is correct, as is implied by the example shown in section 6.2.5 of RFC1034. SOA records, if added, are to be placed in the authority section. (§7.1) | MAY | 7.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-7.2-2` | nor are they required to send SOA records with a TTL of zero. (§7.2) | MAY | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-8-3` | Implementations are always free to place an upper bound on any TTL received, and treat any larger values as if they were that upper bound. (§8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-9-4` | Where TC is set, the partial RRSet that would not completely fit may be left in the response. (§9) | MAY | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-10.1-1` | There may be only one such canonical name for any one alias. (§10.1) | MAY | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-10.1-3` | An alias name (label of a CNAME record) may, if DNSSEC is in use, have SIG, NXT, and KEY RRs (§10.1) | MAY | 10.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2181-11-4` | A DNS server may be configurable to issue warnings when loading, or even to refuse to load, a primary zone containing labels that might be considered questionable (§11) | MAY | 11 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2181-5.1-1`](#rfc2181-5.1-1) The response must be marked as "truncated" if the entire RRSet will not fit in the response. (§5.1) | {gap}, no test | geodns and as112 never set the TC bit or call miekg Truncate, and miekg WriteMsg at vendor/github.com/miekg/dns/server.go:747 packs and sends without auto-truncating, so an oversized RRSet would be sent unmarked |
| [`RFC2181-5.3.1-2`](#rfc2181-5.3.1-2) However, where SIG records are being returned in the answer section, in response to a query for SIG records, or a query for all records associated with a name (type=ANY) the entire SIG RRSet must be included, as for any other RR type. (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no SIG or DNSSEC records -- geodns emits only A/AAAA/SRV at internal/plugins/geodns/record.go:10 and as112 only SOA/NS/TXT, so there is no SIG RRSet to include |
| [`RFC2181-5.3.1-3`](#rfc2181-5.3.1-3) Servers that receive responses containing SIG records in the authority section, or (probably incorrectly) as additional data, must understand that the entire RRSet has almost certainly not been included. (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's stub resolver extracts only answer-section A/AAAA/TXT/PTR/CNAME/MX/NS/SRV at internal/component/resolve/dns/resolver.go:299 and processes no SIG records, so there is no partial SIG RRSet to reason about |
| [`RFC2181-5.3.1-4`](#rfc2181-5.3.1-4) Thus, they must not cache that SIG record in a way that would permit it to be returned should a query for SIG records be received at that server. (§5.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: the resolver caches only records it extracted from the answer section at internal/component/resolve/dns/resolver.go:299 and never handles SIG, so an authority-section SIG can never be cached or returned |
| [`RFC2181-5.4.1-3`](#rfc2181-5.4.1-3) Trustworthiness shall be, in order from most to least: + Data from a primary zone file, other than glue data, + Data from a zone transfer, other than glue, + The authoritative data included in the answer section of an authoritative reply. + Data from the authority section of an authoritative answer, + Glue from a primary zone, or glue from a zone transfer, + Data from the answer section of a non-authoritative answer, and non-authoritative data from the answer section of authoritative answers, + Additional information from an authoritative answer, Data from the authority section of a non-authoritative answer, Additional information from non-authoritative answers. (§5.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's resolver is a stub forwarder to one configured upstream at internal/component/resolve/dns/resolver.go:263 with a single-source cache keyed by name+type, so it never ranks data from competing trustworthiness sources |
| [`RFC2181-5.4.1-8`](#rfc2181-5.4.1-8) When DNS security [RFC2065] is in use, and an authenticated reply has been received and verified, the data thus authenticated shall be considered more trustworthy than unauthenticated data of the same type. (§5.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: the stub resolver performs no per-record trust ranking -- it relies on a validating upstream returning SERVFAIL at internal/component/resolve/dns/resolver.go:99 rather than comparing authenticated against unauthenticated data |
| [`RFC2181-5.4.1-9`](#rfc2181-5.4.1-9) However DNSSEC aware servers must still correctly set the AA bit in responses to enable correct operation with servers that are not security aware (almost all currently). (§5.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze's authoritative servers implement no DNSSEC signing so the DNSSEC-aware precondition does not hold; the AA bit is nonetheless always set at internal/core/dnsserver/handler.go:73 |
| [`RFC2181-5.5-4`](#rfc2181-5.5-4) Where duplicates are required this way, the TTL transmitted in each case must be the same. (§5.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no AXFR -- the geodns YANG notes this at internal/plugins/geodns/yang/ze-geodns-conf.yang:95 and no plugin emits an SOA twice in one message, so no duplicate RRSet arises |
| [`RFC2181-10.1-4`](#rfc2181-10.1-4) An alias name (label of a CNAME record) may, if DNSSEC is in use, have SIG, NXT, and KEY RRs, but may have no other data. (§10.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze serves no CNAME records -- geodns emits only A/AAAA/SRV at internal/plugins/geodns/record.go:10 and as112 only SOA/NS/TXT, so no CNAME can coexist with other data |
| [`RFC2181-10.1.1-1`](#rfc2181-10.1.1-1) Care must therefore be taken to be very clear whether the label, or the value (the canonical name) of a CNAME resource record is intended. (§10.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze authors no CNAME records at internal/plugins/geodns/record.go:10, so there is no label-versus-canonical-name ambiguity for an implementation to resolve |
| [`RFC2181-10.2-1`](#rfc2181-10.2-1) Note that while the value of a PTR record must not be an alias, there is no requirement that the process of resolving a PTR record not encounter any aliases. (§10.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze authors no PTR records -- geodns serves A/AAAA/SRV and as112 reverse zones return NODATA with the SOA at internal/plugins/as112/zones.go:273 rather than any PTR, so no PTR value can be an alias |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2181-4.1-1`](#rfc2181-4.1-1)

To avoid these problems, servers when responding to queries using UDP must cause the reply to be sent with the source address field in the IP header set to the address that was in the destination address field of the IP header of the packet containing the query causing the response. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. TestRFC2181ReplySourcedFromTheQueriedAddress binds the listener to 127.0.0.2 and queries from 127.0.0.1, asserting reply source 127.0.0.2; a wildcard bind would source from 127.0.0.1, so the assertion discriminates (the earlier weak finding). Record: dnsserver Manager.bind revert observed red. {single-polarity} holds: every UDP listener binds one address and there is no explicit-source path to drive negatively.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181ReplySourcedFromTheQueriedAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_clarifications_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L345) | unit/verify | revert, verified |

### [`RFC2181-4.2-1`](#rfc2181-4.2-1)

Replies to all queries must be directed to the port from which they were sent. (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L350) | unit/verify | unproven |

### [`RFC2181-4.2-2`](#rfc2181-4.2-2)

For queries received by UDP the server must take note of the source port and use that as the destination port in the response. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2181_UDPReplySourceAndPort reads the reply on the client's own ephemeral socket and matches the query id; a reply sent to any other destination port never arrives and the read fails.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_UDPReplySourceAndPort`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L355) | unit/verify | unproven |

### [`RFC2181-5.1-1`](#rfc2181-5.1-1)

The response must be marked as "truncated" if the entire RRSet will not fit in the response. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.1-1, so no unit is bound to it.

### [`RFC2181-5.2-1`](#rfc2181-5.2-1)

Consequently the use of differing TTLs in an RRSet is hereby deprecated, the TTLs of all RRs in an RRSet must be the same. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2181_RRSetEqualTTL drives answerQuestions for a three-address host and asserts every A record in the emitted RRSet carries the one configured TTL (120).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_RRSetEqualTTL`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L397) | unit/verify | unproven |

### [`RFC2181-5.3.1-2`](#rfc2181-5.3.1-2)

However, where SIG records are being returned in the answer section, in response to a query for SIG records, or a query for all records associated with a name (type=ANY) the entire SIG RRSet must be included, as for any other RR type. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.3.1-2, so no unit is bound to it.

### [`RFC2181-5.3.1-3`](#rfc2181-5.3.1-3)

Servers that receive responses containing SIG records in the authority section, or (probably incorrectly) as additional data, must understand that the entire RRSet has almost certainly not been included. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.3.1-3, so no unit is bound to it.

### [`RFC2181-5.3.1-4`](#rfc2181-5.3.1-4)

Thus, they must not cache that SIG record in a way that would permit it to be returned should a query for SIG records be received at that server. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.3.1-4, so no unit is bound to it.

### [`RFC2181-5.4-1`](#rfc2181-5.4-1)

Servers must never merge RRs from a response with RRs in their cache to form an RRSet. (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_CacheReplacesRRSetNoMerge`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc2181_cache_test.go#L226) | unit/verify | unproven |

### [`RFC2181-5.4-2`](#rfc2181-5.4-2)

If a response contains data that would form an RRSet with data in a server's cache the server must either ignore the RRs in the response, or discard the entire RRSet currently in the cache, as appropriate. (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. Positive: an overlapping response {B} replaces the cached A RRSet {A,B,C} with exactly {B}, so neither a union nor a remainder passes. Negative: the condition's false arm, data that forms no RRSet (AAAA of the same name, A of another name), leaves those sets whole. Records: cache.put reverts, both polarities observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2181FormingAnRRSetDiscardsTheWholeCachedSet`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc2181_cache_rrset_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestRFC2181FormingAnRRSetDiscardsTheWholeCachedSet`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc2181_cache_rrset_test.go#L21) | unit/verify | revert, verified |
| positive | [`TestRFC2181_CacheReplacesRRSetNoMerge`](https://github.com/ze-software/ze/blob/main/internal/component/resolve/dns/rfc2181_cache_test.go#L234) | unit/verify | revert, verified |

### [`RFC2181-5.4.1-3`](#rfc2181-5.4.1-3)

Trustworthiness shall be, in order from most to least: + Data from a primary zone file, other than glue data, + Data from a zone transfer, other than glue, + The authoritative data included in the answer section of an authoritative reply. + Data from the authority section of an authoritative answer, + Glue from a primary zone, or glue from a zone transfer, + Data from the answer section of a non-authoritative answer, and non-authoritative data from the answer section of authoritative answers, + Additional information from an authoritative answer, Data from the authority section of a non-authoritative answer, Additional information from non-authoritative answers. (§5.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.4.1-3, so no unit is bound to it.

### [`RFC2181-5.4.1-8`](#rfc2181-5.4.1-8)

When DNS security [RFC2065] is in use, and an authenticated reply has been received and verified, the data thus authenticated shall be considered more trustworthy than unauthenticated data of the same type. (§5.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.4.1-8, so no unit is bound to it.

### [`RFC2181-5.4.1-9`](#rfc2181-5.4.1-9)

However DNSSEC aware servers must still correctly set the AA bit in responses to enable correct operation with servers that are not security aware (almost all currently). (§5.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.4.1-9, so no unit is bound to it.

### [`RFC2181-5.5-4`](#rfc2181-5.5-4)

Where duplicates are required this way, the TTL transmitted in each case must be the same. (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-5.5-4, so no unit is bound to it.

### [`RFC2181-8-1`](#rfc2181-8-1)

It is hereby specified that a TTL value is an unsigned number, with a minimum value of 0, and a maximum value of 2147483647. That is, a maximum of 2^31 - 1. When transmitted, this value shall be encoded in the less significant 31 bits of the 32 bit TTL field, with the most significant, or sign, bit set to zero. (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. Positive: parseTTL('0') is 0, and with default-ttl 2147483647 every A, NS and SOA record in the packed and re-read answer has the sign bit clear and value <= 2^31-1, closing the earlier 'when transmitted' gap. Negative: TestRFC2181_TTLSignBitBound refuses 2147483648 at the config boundary. Records: parseTTL revert, positive (author) and negative (judge) observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2181_TTLSignBitBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_config_test.go#L239) | unit/verify | revert, verified |
| positive | [`TestRFC2181TTLTransmittedWithSignBitClear`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_clarifications_test.go#L171) | unit/verify | revert, verified |
| positive | [`TestRFC2181_TTLSignBitBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_config_test.go#L226) | unit/verify | revert, verified |

### [`RFC2181-10.1-4`](#rfc2181-10.1-4)

An alias name (label of a CNAME record) may, if DNSSEC is in use, have SIG, NXT, and KEY RRs, but may have no other data. (§10.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-10.1-4, so no unit is bound to it.

### [`RFC2181-10.1.1-1`](#rfc2181-10.1.1-1)

Care must therefore be taken to be very clear whether the label, or the value (the canonical name) of a CNAME resource record is intended. (§10.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-10.1.1-1, so no unit is bound to it.

### [`RFC2181-10.2-1`](#rfc2181-10.2-1)

Note that while the value of a PTR record must not be an alias, there is no requirement that the process of resolving a PTR record not encounter any aliases. (§10.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2181-10.2-1, so no unit is bound to it.

### [`RFC2181-10.3-1`](#rfc2181-10.3-1)

The domain name used as the value of a NS resource record, or part of the value of a MX resource record must not be an alias. (§10.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. TestRFC2181NSTargetIsNotAnAlias queries each NS target itself: a CNAME query returns no CNAME, an A query returns only A records owned by the target holding the configured addresses (an alias would answer a CNAME chain). Record: appendNS revert observed red. {single-polarity} holds: Ze serves no CNAME and no MX, so there is no alias to refuse.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181NSTargetIsNotAnAlias`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_clarifications_test.go#L88) | unit/verify | revert, verified |
| positive | [`TestRFC2181_NSCanonicalWithGlue`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L437) | unit/verify | revert, verified |

### [`RFC2181-10.3-2`](#rfc2181-10.3-2)

This domain name must have as its value one or more address records. (§10.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2181_NSCanonicalWithGlue asserts every synthesized NS target has an A record in the additional section of the same answer.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_NSCanonicalWithGlue`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L454) | unit/verify | unproven |

### [`RFC2181-11-1`](#rfc2181-11-1)

The length of any one label is limited to between 1 and 63 octets. A full domain name is limited to 255 octets (including the separators). (§11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. Both sides of each bound: a 63-octet label accepted by parseConfig, a 255-octet name accepted by checkName and packed by the codec; a 64-octet label, an EMPTY label (the D-8 defect, fixed in checkName with the section 11 quote) and a 256-octet name refused, and the codec refuses 256. Records: checkName revert, both polarities observed red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2181NameLimitsBothSidesOfEachBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_clarifications_test.go#L145) | unit/verify | revert, verified |
| positive | [`TestRFC2181NameLimitsBothSidesOfEachBound`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_clarifications_test.go#L131) | unit/verify | revert, verified |
| positive | [`TestRFC2181_WireNameLimits`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_server_test.go#L481) | unit/verify | revert, verified |

### [`RFC2181-11-2`](#rfc2181-11-2)

Implementations of the DNS protocols must not place any restrictions on the labels that can be used. (§11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_LabelsUnrestricted`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_config_test.go#L272) | unit/verify | unproven |

### [`RFC2181-11-3`](#rfc2181-11-3)

DNS servers must not refuse to serve a zone because it contains labels that might not be acceptable to some DNS client programs. (§11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2181_LabelsUnrestricted`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2181_config_test.go#L261) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc2181.txt |
| Source fingerprint | b86942be6a672cdb |
| Record | rfc/extraction/rfc2181.json |
| Mapped sentences | 29 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 2 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `5.2` | not stated | 2 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.3.1` | not stated | 4 | walked | not stated |
| `5.3.2` | not stated | 1 | walked | not stated |
| `5.4` | not stated | 2 | walked | not stated |
| `5.4.1` | not stated | 4 | walked | not stated |
| `5.5` | not stated | 3 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 2 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 1 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `9` | not stated | 1 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.1.1` | not stated | 1 | walked | not stated |
| `10.2` | not stated | 2 | walked | not stated |
| `10.3` | not stated | 2 | walked | not stated |
| `11` | not stated | 2 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `15` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 3 states the memo's own drafting convention ('This memo does not use the oft used expressions MUST, SHOULD, MAY'); it names no behaviour a DNS implementation performs. | This memo does not use the oft used expressions MUST, SHOULD, MAY, or their negative forms. |
| `3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The reading instruction for the whole document: it tells a reader to treat the lowercase words as fundamental. It carries no obligation of its own, and the rfc/short/rfc2181.md checklist preamble quotes it as the rule under which every row's level was assigned. | Anywhere that this memo suggests that some action should be carried out, or must be carried out, or that some behaviour is acceptable, or not, that is to be considered as a fundamental aspect of this specification, regardless of the specific words used. |
| `5.3.1:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive prose closing the SIG-forwarding discussion: it says what a server that already follows RFC2181-5.3.1-4 can then determine about its own cache. 'the query must be forwarded' names the outcome of that determination, not a new obligation. | Then the server can determine when it is safe to reply from the cache, and when the answer is not available and the query must be forwarded. |
| `6.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | A statement of fact about the NXT RR type: it 'must necessarily relate to the zone in which it exists' describes what the record's contents mean, not an action an implementation takes. | In particular the NXT ("next") RR type contains information about which names exist in a zone, and hence which do not, and thus must necessarily relate to the zone in which it exists. |
| `10.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The closing sentence of Section 10.2 restates the obligation site 10.2:1 carries: 'This final result, the value of the PTR RR, is the label which must not be an alias.' | This final result, the value of the PTR RR, is the label which must not be an alias. |

## Superseded

No document obsoletes RFC 2181, so its obligations are stated where they were written.
