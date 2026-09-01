# RFC 7871 - Client Subnet in DNS Queries

Partial. Every requirement this repository extracted from RFC 7871, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 9 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 33.3% | 3 of 9 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 3 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 38 | of 90 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 29 | of 38 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 66.7% | 6 of 9 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 9 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 90 |
| Gated MUST-level | 38 |
| Obligations that bind Ze | 9 |
| Not applicable, so out of scope | 29 |
| Declared gaps | 6 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 3 |
| Tagged units | 3 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7871.md` |
| Requirement shard | `rfc/requirements/rfc7871.md` |
| RFC text | `rfc/full/rfc7871.txt` |

## Enrolment

Enrolled: Client Subnet in DNS Queries / EDNS0 ECS (RFC 7871): ECS consumer role only. geodns reads the incoming EDNS0 client-subnet ADDRESS for geo source-selection (a MAY) and constructs no ECS option. 3 single-polarity positive (emits no ECS option in any response 7.2.1-7/7.2.2-1, never refuses a 0-address-bit query 7.5-6) + 6 gap (tailors answers from ECS but does not echo the option/indicate support/set SCOPE 7.2.1-5/7.2.2-2/12.1-4/12.1-5, nor FORMERR-reject a malformed option 7.2.1-3/7.2.1-4) + 29 not-applicable (originates/forwards/validates no ECS query, caches nothing by network)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- GeoDNS consumes the EDNS0 client-subnet ADDRESS (a MAY, sections 7.2.1/11.1) to select a tailored answer by longest-prefix host-set match, or the packet source, per the client-ip-source mode ([`internal/core/dnsserver/client.go`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/client.go))
- it emits no ECS option in any query or response and caches nothing by network
- tests bound per requirement in [`rfc/short/rfc7871.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7871.md).


**What the ledger says remains**

Six MUST gaps, each annotated in [`rfc/short/rfc7871.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc7871.md): GeoDNS uses ECS to tailor answers but includes no ECS option in its responses, so it neither echoes the option, indicates support, nor sets SCOPE ([`RFC7871-7.2.1-5`](#rfc7871-7.2.1-5), 7.2.2-2, 12.1-4, 12.1-5), and it validates no consumed option's FAMILY nor returns FORMERR for a malformed one ([`RFC7871-7.2.1-3`](#rfc7871-7.2.1-3), 7.2.1-4). The resolver/forwarder origination, ECS-response validation, network-scope caching, and DNSSEC-tailoring MUSTs are not-applicable: ze originates, forwards, and caches no ECS and is a plain stub resolver.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 38 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **38** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (38):** [`RFC7871-6-1`](#rfc7871-6-1), [`RFC7871-6-2`](#rfc7871-6-2), [`RFC7871-7.1.1-2`](#rfc7871-7.1.1-2), [`RFC7871-7.1.1-3`](#rfc7871-7.1.1-3), [`RFC7871-7.1.1-4`](#rfc7871-7.1.1-4), [`RFC7871-7.1.1-7`](#rfc7871-7.1.1-7), [`RFC7871-7.1.2-2`](#rfc7871-7.1.2-2), [`RFC7871-7.1.2-3`](#rfc7871-7.1.2-3), [`RFC7871-7.1.2-5`](#rfc7871-7.1.2-5), [`RFC7871-7.1.3-1`](#rfc7871-7.1.3-1), [`RFC7871-7.1.3-2`](#rfc7871-7.1.3-2), [`RFC7871-7.1.3-3`](#rfc7871-7.1.3-3), [`RFC7871-7.2.1-2`](#rfc7871-7.2.1-2), [`RFC7871-7.2.1-3`](#rfc7871-7.2.1-3), [`RFC7871-7.2.1-4`](#rfc7871-7.2.1-4), [`RFC7871-7.2.1-5`](#rfc7871-7.2.1-5), [`RFC7871-7.2.1-7`](#rfc7871-7.2.1-7), [`RFC7871-7.2.1-8`](#rfc7871-7.2.1-8), [`RFC7871-7.2.1-12`](#rfc7871-7.2.1-12), [`RFC7871-7.2.2-1`](#rfc7871-7.2.2-1), [`RFC7871-7.2.2-2`](#rfc7871-7.2.2-2), [`RFC7871-7.3-3`](#rfc7871-7.3-3), [`RFC7871-7.3-4`](#rfc7871-7.3-4), [`RFC7871-7.3-6`](#rfc7871-7.3-6), [`RFC7871-7.3.1-1`](#rfc7871-7.3.1-1), [`RFC7871-7.3.1-3`](#rfc7871-7.3.1-3), [`RFC7871-7.3.1-4`](#rfc7871-7.3.1-4), [`RFC7871-7.3.2-1`](#rfc7871-7.3.2-1), [`RFC7871-7.3.2-4`](#rfc7871-7.3.2-4), [`RFC7871-7.5-1`](#rfc7871-7.5-1), [`RFC7871-7.5-6`](#rfc7871-7.5-6), [`RFC7871-9-1`](#rfc7871-9-1), [`RFC7871-11.1-2`](#rfc7871-11.1-2), [`RFC7871-11.2-1`](#rfc7871-11.2-1), [`RFC7871-11.3-4`](#rfc7871-11.3-4), [`RFC7871-12.1-3`](#rfc7871-12.1-3), [`RFC7871-12.1-4`](#rfc7871-12.1-4), [`RFC7871-12.1-5`](#rfc7871-12.1-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7871-6-1` | In a query, SCOPE PREFIX-LENGTH MUST be set to 0. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no ECS-bearing query; the stub resolver sets only the EDNS0 DO bit and adds no client-subnet option (internal/component/resolve/dns/resolver.go:261), so it never writes a query SCOPE PREFIX-LENGTH |
| `RFC7871-6-2` | ADDRESS MUST be truncated to the number of bits given by SOURCE PREFIX-LENGTH, padding with 0 bits to the end of the last octet needed. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no ECS option in any query or response; the stub resolver adds none (internal/component/resolve/dns/resolver.go:261) and geodns answers with only A/AAAA/SRV/SOA/NS records (internal/plugins/geodns/server.go:168), so it truncates no ADDRESS it built |
| `RFC7871-7.1.1-2` | If the triggering client query included an ECS option, it MUST be examined for its SOURCE PREFIX-LENGTH. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is no Recursive Resolver forming ECS-bearing outgoing queries; it reads an incoming ECS ADDRESS only for source selection (internal/core/dnsserver/client.go:21) and originates no ECS query to size (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.1-3` | The Recursive Resolver's outgoing query MUST set SOURCE PREFIX-LENGTH to the shorter of the incoming query's SOURCE PREFIX-LENGTH or the server's maximum cacheable prefix length. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no ECS-bearing outgoing query, so it sets no outgoing SOURCE PREFIX-LENGTH (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.1-4` | The number of ADDRESS octets used MUST cover only SOURCE PREFIX-LENGTH bits, not the full width normally used by FAMILY. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no ECS option, so it sizes no ADDRESS octet count (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.1-7` | Subsequent queries to refresh the data MUST, if unrestricted by an incoming SOURCE PREFIX-LENGTH, specify the longest SOURCE PREFIX-LENGTH the Recursive Resolver is willing to cache, even if a previous response indicated a shorter prefix sufficed. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze issues no ECS-bearing refresh query and caches no ECS-scoped data to refresh (stub cache keyed by name+qtype at internal/component/resolve/dns/cache.go:16, no ECS query at internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.2-2` | An Intermediate Nameserver receiving a query that limits SOURCE PREFIX-LENGTH MUST NOT make queries that include more bits of client address than the originating query. (§7.1.2) | MUST NOT | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is no Intermediate Nameserver forwarding ECS-bearing queries; it originates none (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.2-3` | A SOURCE PREFIX-LENGTH of 0 means the Recursive Resolver MUST NOT add the client's address information to its queries; §7.5 restates this obligation for any Intermediate Nameserver. (§7.1.2) | MUST NOT | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze adds no client address to any outgoing query; the stub resolver emits no ECS option (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.2-5` | A Stub Resolver MUST set SCOPE PREFIX-LENGTH to 0. (§7.1.2) | MUST | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's stub resolver emits no ECS option, so it writes no SCOPE PREFIX-LENGTH (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.3-1` | A Forwarding Resolver using this option MUST prepare it as described for Recursive Resolvers in §7.1.1. (§7.1.3) | MUST | 7.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no Forwarding Resolver that prepares ECS options; geodns answers authoritatively (internal/plugins/geodns/server.go:221) and the stub resolver forwards no ECS (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.3-2` | A Forwarding Resolver that implements this protocol MUST honor the SOURCE PREFIX-LENGTH restrictions indicated in the incoming query from its client. (§7.1.3) | MUST | 7.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no ECS-bearing client query, so it honors no incoming SOURCE PREFIX-LENGTH on a forward path (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.3-3` | If a Forwarding Resolver receives a REFUSED response to a query that includes a non-zero ADDRESS, it MUST retry with no ADDRESS. (§7.1.3) | MUST | 7.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze sends no ECS-bearing query that could draw a REFUSED needing an ADDRESS-stripped retry (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.2.1-2` | A server that has not implemented or enabled ECS MUST NOT include an ECS option in replies to indicate lack of support. (§7.2.1) | MUST NOT | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** geodns emits no ECS option in any reply (internal/plugins/geodns/server.go:168), so it never sends one to signal lack of support; this prohibition targets servers with no ECS support |
| `RFC7871-7.2.1-3` | A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected. (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns reads the incoming ECS ADDRESS without validating its FAMILY and rejects no unknown-FAMILY option; internal/core/dnsserver/client.go:26 takes ecs.Address unconditionally, with no FORMERR path |
| `RFC7871-7.2.1-4` | A FORMERR response MUST be returned to the sender for a wrongly formatted option. (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns returns no FORMERR for a wrongly formatted ECS option it consumes; internal/core/dnsserver/client.go:21 has no rejection path and internal/plugins/geodns/server.go:221 never sets FORMERR |
| `RFC7871-7.2.1-5` | An Authoritative Nameserver implementing ECS that receives an ECS option MUST include an ECS option in its response, regardless of whether the client information was needed. (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns uses the ECS ADDRESS to tailor answers (internal/core/dnsserver/client.go:21) but includes no ECS option in its response (internal/plugins/geodns/server.go:168), so it does not echo the option |
| `RFC7871-7.2.1-7` | If an ECS option was not included in the query, one MUST NOT be included in the response, even when the server provides a Tailored Response. (§7.2.1) | MUST NOT | 7.2.1 | **positive:** `unit/verify` [`TestRFC7871_NoECSQueryNoECSResponse`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L72). **negative:** no negative test. **{single-polarity}:** geodns emits no ECS option in any response (internal/plugins/geodns/server.go:168), so the complementary state (an ECS option present when the query carried none) cannot be constructed; the positive case, an ECS-less query that still yields a tailored answer producing an ECS-less response, is pinned in internal/plugins/geodns/rfc7871_server_test.go |
| `RFC7871-7.2.1-8` | FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the response MUST match those in the query; the same echo requirement is restated as an anti-spoofing measure in §11.2. (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** geodns includes no ECS option in its response (internal/plugins/geodns/server.go:168), so there are no echoed FAMILY/SOURCE/ADDRESS fields that could fail to match the query; the missing echo itself is disclosed as the RFC7871-7.2.1-5 gap |
| `RFC7871-7.2.1-12` | An Authoritative Nameserver MUST NOT overlap prefixes among its Tailored Responses. (§7.2.1) | MUST NOT | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** geodns publishes no SCOPE-scoped Tailored Response whose prefixes a downstream cache could order-dependently overlap; its config source prefixes resolve deterministically by longest-prefix at lookup (internal/core/dnsserver/matcher.go:28) |
| `RFC7871-7.2.2-1` | If the client query did not include an ECS option, the server MUST NOT provide one in its response. (§7.2.2) | MUST NOT | 7.2.2 | **positive:** `unit/verify` [`TestRFC7871_NoECSQueryNoECSResponse`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L77). **negative:** no negative test. **{single-polarity}:** geodns includes no ECS option in any response (internal/plugins/geodns/server.go:168), so no negative case exists; internal/plugins/geodns/rfc7871_server_test.go asserts an ECS-less query draws an ECS-less response |
| `RFC7871-7.2.2-2` | If the client query did include the option, the server MUST include one in its response. (§7.2.2) | MUST | 7.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns consumes the query's ECS option to select an answer yet includes no ECS option in its response (internal/plugins/geodns/server.go:168) |
| `RFC7871-7.3-3` | If FAMILY, SOURCE PREFIX-LENGTH, and the SOURCE PREFIX-LENGTH bits of ADDRESS in the response do not match the non-zero fields in the corresponding query, the full response MUST be dropped. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze issues no ECS-bearing query and so validates no ECS response for a FAMILY/SOURCE/ADDRESS mismatch to drop (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.3-4` | In a response to a query that specified only SOURCE PREFIX-LENGTH for privacy masking, the FAMILY and ADDRESS fields MUST contain the appropriate non-zero information the Authoritative Nameserver used to generate the answer. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** geodns emits no ECS option in its response (internal/plugins/geodns/server.go:168), so it carries no FAMILY/ADDRESS response fields; the absent echo is disclosed as the RFC7871-7.2.1-5 gap |
| `RFC7871-7.3-6` | If a REFUSED response is received from an Authoritative Nameserver, an ECS-aware resolver MUST retry the query without ECS. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze sends no ECS-bearing query, so it has no ECS query to retry without on a REFUSED (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.3.1-1` | In the cache, all resource records in the Answer section MUST be tied to the network specified in the response. (§7.3.1) | MUST | 7.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze ties no cached record to a network; the DNS cache keys entries by name+qtype only (internal/component/resolve/dns/cache.go:16) and geodns answers each query fresh from config (internal/plugins/geodns/server.go:168) |
| `RFC7871-7.3.1-3` | Records from the Additional and Authority sections MUST NOT be tied to a network. (§7.3.1) | MUST NOT | 7.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no network-scoped answer cache, so no Additional/Authority record is ever tied to a network (internal/component/resolve/dns/cache.go:16) |
| `RFC7871-7.3.1-4` | Records cached as /0 because of a query's SOURCE PREFIX-LENGTH of 0 MUST be distinguished from those cached as /0 because of a response's SCOPE PREFIX-LENGTH of 0. (§7.3.1) | MUST | 7.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze caches no ECS-scoped answers, so it draws no distinction between a /0 from SOURCE 0 and a /0 from SCOPE 0 (internal/component/resolve/dns/cache.go:16) |
| `RFC7871-7.3.2-1` | The appropriate RRset MUST be chosen based on longest-prefix matching. (§7.3.2) | MUST | 7.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze selects no cached RRset by ECS longest-prefix; geodns's longest-prefix match runs over operator-configured source prefixes (internal/plugins/geodns/server.go:77, internal/core/dnsserver/matcher.go:40), an authoritative selection the RFC does not govern, not an ECS-response cache |
| `RFC7871-7.3.2-4` | If no matching network is found, the Intermediate Nameserver MUST perform resolution as usual. (§7.3.2) | MUST | 7.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no ECS-response cache to miss; geodns answers authoritatively and, on no host-set match, returns a normal NOERROR negative rather than a cache fallthrough (internal/plugins/geodns/server.go:182) |
| `RFC7871-7.5-1` | Any Intermediate Nameserver that forwards ECS options received from its clients MUST fully implement the caching behavior of §7.3. (§7.5) | MUST | 7.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no client ECS option and keeps no section 7.3 network-scoped cache; it consumes the ADDRESS locally (internal/core/dnsserver/client.go:21) and the cache is keyed by name+qtype (internal/component/resolve/dns/cache.go:16) |
| `RFC7871-7.5-6` | A query MUST NOT be refused solely because it provides 0 address bits. (§7.5) | MUST NOT | 7.5 | **positive:** `unit/verify` [`TestRFC7871_ZeroAddressBitsNotRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L113). **negative:** no negative test. **{single-polarity}:** geodns refuses only when disabled and never on ECS content (internal/plugins/geodns/server.go:236), so a query carrying 0 address bits is answered, not refused; the refuse-for-0-bits case is unreachable and internal/plugins/geodns/rfc7871_server_test.go pins the positive |
| `RFC7871-9-1` | RRSIG records MUST be tied to the RRset they sign in a Tailored Response. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** geodns emits no RRSIG or DNSSEC records and no SCOPE-scoped Tailored Response; it synthesizes only unsigned A/AAAA/SRV/SOA/NS answers (internal/plugins/geodns/server.go:106) |
| `RFC7871-11.1-2` | Recursive Resolvers forwarding the ECS option MUST NOT modify it to include the network address of the client. (§11.1) | MUST NOT | 11.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no ECS option, so it modifies none in flight; it reads the incoming ADDRESS only for local selection (internal/core/dnsserver/client.go:21) |
| `RFC7871-11.2-1` | Intermediate Nameservers processing a response MUST verify that FAMILY, ADDRESS, and SOURCE PREFIX-LENGTH match those of the corresponding query. (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is no Intermediate Nameserver validating an ECS response; it issues no ECS query whose response fields it would verify (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-11.3-4` | Recursive Resolvers MUST NOT send an ECS option whose SOURCE PREFIX-LENGTH provides more bits of ADDRESS than they are willing to cache responses for. (§11.3) | MUST NOT | 11.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze sends no ECS-bearing query, so it caps no outgoing SOURCE PREFIX-LENGTH by cache willingness (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-12.1-3` | Probing, if implemented, MUST be repeated periodically (e.g., daily). (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no ECS support-probing of authoritative servers, so no periodic-probe obligation arises; it consumes ECS only on the serving side (internal/core/dnsserver/client.go:21) |
| `RFC7871-12.1-4` | An Authoritative Nameserver that uses ECS information for one of its zones MUST indicate support for the option in all of its responses to ECS queries. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns uses ECS information for its zones (internal/core/dnsserver/client.go:21) but indicates support in none of its responses, emitting no ECS option (internal/plugins/geodns/server.go:168) |
| `RFC7871-12.1-5` | If the option is supported but not actually used to generate a response, its SCOPE PREFIX-LENGTH MUST be set to 0. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns emits no ECS option in its response (internal/plugins/geodns/server.go:168), so it sets no SCOPE PREFIX-LENGTH to 0 to mark the option supported but unused |
| `RFC7871-6-3` | A server receiving an ECS option that uses too few or too many ADDRESS octets, or has non-zero ADDRESS bits set beyond SOURCE PREFIX-LENGTH, SHOULD return FORMERR to reject the packet. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.1-1` | When initializing the option, the SOURCE PREFIX-LENGTH the Recursive Resolver sets SHOULD be shorter than the full address, for privacy. (§7.1.1) | SHOULD | 7.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.1-6` | If the Recursive Resolver will not forward FAMILY and ADDRESS data from the incoming ECS option, it SHOULD return a REFUSED response. (§7.1.1) | SHOULD | 7.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.2-7` | If no ADDRESS is set (SOURCE PREFIX-LENGTH is 0), FAMILY SHOULD be set to the transport over which the query is sent. (§7.1.2) | SHOULD | 7.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.1-6` | A response that carries an ECS option SHOULD be cached accordingly. (§7.2.1) | SHOULD | 7.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.1-9` | Future queries for the name within the specified network SHOULD use the longer SCOPE PREFIX-LENGTH returned by the server. (§7.2.1) | SHOULD | 7.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.1-10` | When multiple RRsets are returned, the response SHOULD include the longest relevant PREFIX-LENGTH of any RRset in the answer. (§7.2.1) | SHOULD | 7.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.1-11` | For a CNAME chain, the Authoritative Nameserver SHOULD place only the initial CNAME record in the Answer section. (§7.2.1) | SHOULD | 7.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.2-3` | If an Intermediate Nameserver receives a response with a longer SCOPE PREFIX-LENGTH than the SOURCE PREFIX-LENGTH it provided, it SHOULD still provide the result to the triggering client even if the client is in a different address range. (§7.2.2) | SHOULD | 7.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3-1` | When an Intermediate Nameserver receives a response with an ECS option and the TC bit clear, it SHOULD cache the result based on the option data. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3-2` | If the TC bit was set, the Intermediate Resolver SHOULD retry the query over TCP to get the complete Answer section for caching. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3-5` | If no ECS option is contained in the response, the Intermediate Nameserver SHOULD treat this as equivalent to a SCOPE PREFIX-LENGTH of 0. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3-7` | A client of a Recursive Resolver SHOULD retry after receiving a REFUSED response. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3.1-2` | All DNSSEC records other than RRSIG are RECOMMENDED to be scoped at /0; §9 restates this as a SHOULD. (§7.3.1) | RECOMMENDED | 7.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3.1-5` | Implementing full network-specific caching support as described in this section is strongly RECOMMENDED. (§7.3.1) | RECOMMENDED | 7.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3.2-3` | If local policy does not allow using the ADDRESS from a received ECS option, a REFUSED response SHOULD be sent. (§7.3.2) | SHOULD | 7.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.4-1` | It is RECOMMENDED that no specific behavior regarding negative answers be relied upon. (§7.4) | RECOMMENDED | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.4-2` | Authoritative Nameservers SHOULD set SCOPE PREFIX-LENGTH expecting that Intermediate Nameservers will treat all negative answers as /0. (§7.4) | SHOULD | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.4-3` | Addresses in the Additional section SHOULD ignore ECS data. (§7.4) | SHOULD | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.4-4` | The Authoritative Nameserver SHOULD return a zero SCOPE PREFIX-LENGTH on delegations. (§7.4) | SHOULD | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.4-5` | A Recursive Resolver SHOULD treat a non-zero SCOPE PREFIX-LENGTH in a delegation as though it were zero. (§7.4) | SHOULD | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.5-5` | If the Intermediate Nameserver does not want to use the information in a received ECS option, it SHOULD drop the query and return a REFUSED response. (§7.5) | SHOULD | 7.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-1` | The client's network address SHOULD NOT be added by NAT devices. (§10) | SHOULD NOT | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-2` | Existing ECS options, if present, SHOULD NOT be modified by NAT devices. (§10) | SHOULD NOT | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-4` | A Recursive Resolver that knows it is behind a NAT device SHOULD NOT originate ECS options with its external IP address. (§10) | SHOULD NOT | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-6` | An Authoritative Nameserver on the public Internet receiving a query whose ADDRESS is in RFC 1918 or RFC 4193 private space SHOULD ignore ADDRESS and look up its answer based on the Recursive Resolver's address. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-7` | In that response, it SHOULD set SCOPE PREFIX-LENGTH to cover all of the relevant private space. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.1-1` | For a /20 known to be homogeneous, the ISP's Recursive Resolver SHOULD truncate IP addresses in it to 20 bits rather than 24. (§11.1) | SHOULD | 11.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.2-2` | Intermediate Nameservers SHOULD discard the entire response if FAMILY, ADDRESS, and SOURCE PREFIX-LENGTH do not match the query; this is deliberately SHOULD, not MUST, in tension with the §7.3 drop rule. (§11.2) | SHOULD | 11.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-1` | Because of the high cache pressure introduced by ECS, the feature SHOULD be disabled in all default configurations. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-2` | Recursive Resolvers SHOULD limit the number of networks and answers they keep in the cache for any given query. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-3` | Recursive Resolvers SHOULD limit the total number of different networks they keep in cache. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-6` | Resolvers SHOULD at least treat unroutable addresses as equivalent to the Recursive Resolver's own identity. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-7` | Resolvers SHOULD ignore, and never forward, ECS options specifying other routable addresses known not to be served by the query source. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-12.1-1` | It is RECOMMENDED that resolvers remember which Authoritative Nameservers did not return the option and omit client address information from subsequent queries to them. (§12.1) | RECOMMENDED | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-12.1-2` | Recursive Resolvers SHOULD be configured never to send the option when querying root, top-level, and effective top-level (public suffix) domain servers. (§12.1) | SHOULD | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.1-5` | FAMILY and ADDRESS information MAY be reused from the ECS option in the incoming query. (§7.1.1) | MAY | 7.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.2-1` | A Stub Resolver MAY generate DNS queries with an ECS option that sets SOURCE PREFIX-LENGTH to limit how much network information is revealed. (§7.1.2) | MAY | 7.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.2-4` | When SOURCE PREFIX-LENGTH is 0, the subsequent Recursive Resolver query MAY optionally include its own address information. (§7.1.2) | MAY | 7.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.2-6` | A Stub Resolver MAY include FAMILY and ADDRESS data. (§7.1.2) | MAY | 7.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.1-1` | An Authoritative Nameserver supporting ECS MAY use the address information in the option to generate a tailored response. (§7.2.1) | MAY | 7.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.2-4` | The Intermediate Nameserver MAY instead retry with a longer SOURCE PREFIX-LENGTH before responding, as long as it does not exceed the SOURCE PREFIX-LENGTH specified in the triggering query. (§7.2.2) | MAY | 7.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3.2-2` | If there was an ECS option with an ADDRESS, that ADDRESS MAY be used if local policy allows. (§7.3.2) | MAY | 7.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.5-2` | An Intermediate Nameserver MAY forward ECS options with address information. (§7.5) | MAY | 7.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.5-3` | The forwarded address information MAY match the source IP address of the incoming query. (§7.5) | MAY | 7.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.5-4` | The forwarded address information MAY have more or fewer address bits than the nameserver would normally include in a locally originated ECS option. (§7.5) | MAY | 7.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-3` | An Intermediate Nameserver with detailed network-layout knowledge MAY use that information when originating ECS options. (§10) | MAY | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-5` | A Recursive Resolver behind a NAT device MAY include the option with its internal address to signal its own SOURCE PREFIX-LENGTH limit. (§10) | MAY | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-8` | The Intermediate Nameserver MAY elect to cache the answer for a private/special-purpose ADDRESS under a single entry. (§10) | MAY | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.1-3` | Recursive Resolvers or Authoritative Nameservers MAY use the source IP address of queries to return a cached entry or generate a Tailored Response. (§11.1) | MAY | 11.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-5` | Recursive Resolvers MAY, for example, decide to discard more-specific cache entries first. (§11.3) | MAY | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-12.2-1` | Implementations MAY allow additional configuring of the whitelist based on other criteria, such as zone or query type. (§12.2) | MAY | 12.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7871-6-1`](#rfc7871-6-1) In a query, SCOPE PREFIX-LENGTH MUST be set to 0. (§6) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no ECS-bearing query; the stub resolver sets only the EDNS0 DO bit and adds no client-subnet option (internal/component/resolve/dns/resolver.go:261), so it never writes a query SCOPE PREFIX-LENGTH |
| [`RFC7871-6-2`](#rfc7871-6-2) ADDRESS MUST be truncated to the number of bits given by SOURCE PREFIX-LENGTH, padding with 0 bits to the end of the last octet needed. (§6) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no ECS option in any query or response; the stub resolver adds none (internal/component/resolve/dns/resolver.go:261) and geodns answers with only A/AAAA/SRV/SOA/NS records (internal/plugins/geodns/server.go:168), so it truncates no ADDRESS it built |
| [`RFC7871-7.1.1-2`](#rfc7871-7.1.1-2) If the triggering client query included an ECS option, it MUST be examined for its SOURCE PREFIX-LENGTH. (§7.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze is no Recursive Resolver forming ECS-bearing outgoing queries; it reads an incoming ECS ADDRESS only for source selection (internal/core/dnsserver/client.go:21) and originates no ECS query to size (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.1-3`](#rfc7871-7.1.1-3) The Recursive Resolver's outgoing query MUST set SOURCE PREFIX-LENGTH to the shorter of the incoming query's SOURCE PREFIX-LENGTH or the server's maximum cacheable prefix length. (§7.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no ECS-bearing outgoing query, so it sets no outgoing SOURCE PREFIX-LENGTH (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.1-4`](#rfc7871-7.1.1-4) The number of ADDRESS octets used MUST cover only SOURCE PREFIX-LENGTH bits, not the full width normally used by FAMILY. (§7.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no ECS option, so it sizes no ADDRESS octet count (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.1-7`](#rfc7871-7.1.1-7) Subsequent queries to refresh the data MUST, if unrestricted by an incoming SOURCE PREFIX-LENGTH, specify the longest SOURCE PREFIX-LENGTH the Recursive Resolver is willing to cache, even if a previous response indicated a shorter prefix sufficed. (§7.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze issues no ECS-bearing refresh query and caches no ECS-scoped data to refresh (stub cache keyed by name+qtype at internal/component/resolve/dns/cache.go:16, no ECS query at internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.2-2`](#rfc7871-7.1.2-2) An Intermediate Nameserver receiving a query that limits SOURCE PREFIX-LENGTH MUST NOT make queries that include more bits of client address than the originating query. (§7.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze is no Intermediate Nameserver forwarding ECS-bearing queries; it originates none (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.2-3`](#rfc7871-7.1.2-3) A SOURCE PREFIX-LENGTH of 0 means the Recursive Resolver MUST NOT add the client's address information to its queries; §7.5 restates this obligation for any Intermediate Nameserver. (§7.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze adds no client address to any outgoing query; the stub resolver emits no ECS option (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.2-5`](#rfc7871-7.1.2-5) A Stub Resolver MUST set SCOPE PREFIX-LENGTH to 0. (§7.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's stub resolver emits no ECS option, so it writes no SCOPE PREFIX-LENGTH (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.3-1`](#rfc7871-7.1.3-1) A Forwarding Resolver using this option MUST prepare it as described for Recursive Resolvers in §7.1.1. (§7.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no Forwarding Resolver that prepares ECS options; geodns answers authoritatively (internal/plugins/geodns/server.go:221) and the stub resolver forwards no ECS (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.3-2`](#rfc7871-7.1.3-2) A Forwarding Resolver that implements this protocol MUST honor the SOURCE PREFIX-LENGTH restrictions indicated in the incoming query from its client. (§7.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no ECS-bearing client query, so it honors no incoming SOURCE PREFIX-LENGTH on a forward path (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.3-3`](#rfc7871-7.1.3-3) If a Forwarding Resolver receives a REFUSED response to a query that includes a non-zero ADDRESS, it MUST retry with no ADDRESS. (§7.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze sends no ECS-bearing query that could draw a REFUSED needing an ADDRESS-stripped retry (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.2.1-2`](#rfc7871-7.2.1-2) A server that has not implemented or enabled ECS MUST NOT include an ECS option in replies to indicate lack of support. (§7.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: geodns emits no ECS option in any reply (internal/plugins/geodns/server.go:168), so it never sends one to signal lack of support; this prohibition targets servers with no ECS support |
| [`RFC7871-7.2.1-3`](#rfc7871-7.2.1-3) A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected. (§7.2.1) | {gap}, no test | geodns reads the incoming ECS ADDRESS without validating its FAMILY and rejects no unknown-FAMILY option; internal/core/dnsserver/client.go:26 takes ecs.Address unconditionally, with no FORMERR path |
| [`RFC7871-7.2.1-4`](#rfc7871-7.2.1-4) A FORMERR response MUST be returned to the sender for a wrongly formatted option. (§7.2.1) | {gap}, no test | geodns returns no FORMERR for a wrongly formatted ECS option it consumes; internal/core/dnsserver/client.go:21 has no rejection path and internal/plugins/geodns/server.go:221 never sets FORMERR |
| [`RFC7871-7.2.1-5`](#rfc7871-7.2.1-5) An Authoritative Nameserver implementing ECS that receives an ECS option MUST include an ECS option in its response, regardless of whether the client information was needed. (§7.2.1) | {gap}, no test | geodns uses the ECS ADDRESS to tailor answers (internal/core/dnsserver/client.go:21) but includes no ECS option in its response (internal/plugins/geodns/server.go:168), so it does not echo the option |
| [`RFC7871-7.2.1-8`](#rfc7871-7.2.1-8) FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the response MUST match those in the query; the same echo requirement is restated as an anti-spoofing measure in §11.2. (§7.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: geodns includes no ECS option in its response (internal/plugins/geodns/server.go:168), so there are no echoed FAMILY/SOURCE/ADDRESS fields that could fail to match the query; the missing echo itself is disclosed as the RFC7871-7.2.1-5 gap |
| [`RFC7871-7.2.1-12`](#rfc7871-7.2.1-12) An Authoritative Nameserver MUST NOT overlap prefixes among its Tailored Responses. (§7.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: geodns publishes no SCOPE-scoped Tailored Response whose prefixes a downstream cache could order-dependently overlap; its config source prefixes resolve deterministically by longest-prefix at lookup (internal/core/dnsserver/matcher.go:28) |
| [`RFC7871-7.2.2-2`](#rfc7871-7.2.2-2) If the client query did include the option, the server MUST include one in its response. (§7.2.2) | {gap}, no test | geodns consumes the query's ECS option to select an answer yet includes no ECS option in its response (internal/plugins/geodns/server.go:168) |
| [`RFC7871-7.3-3`](#rfc7871-7.3-3) If FAMILY, SOURCE PREFIX-LENGTH, and the SOURCE PREFIX-LENGTH bits of ADDRESS in the response do not match the non-zero fields in the corresponding query, the full response MUST be dropped. (§7.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze issues no ECS-bearing query and so validates no ECS response for a FAMILY/SOURCE/ADDRESS mismatch to drop (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.3-4`](#rfc7871-7.3-4) In a response to a query that specified only SOURCE PREFIX-LENGTH for privacy masking, the FAMILY and ADDRESS fields MUST contain the appropriate non-zero information the Authoritative Nameserver used to generate the answer. (§7.3) | no test | no test carries this requirement id; annotated {not-applicable}: geodns emits no ECS option in its response (internal/plugins/geodns/server.go:168), so it carries no FAMILY/ADDRESS response fields; the absent echo is disclosed as the RFC7871-7.2.1-5 gap |
| [`RFC7871-7.3-6`](#rfc7871-7.3-6) If a REFUSED response is received from an Authoritative Nameserver, an ECS-aware resolver MUST retry the query without ECS. (§7.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze sends no ECS-bearing query, so it has no ECS query to retry without on a REFUSED (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.3.1-1`](#rfc7871-7.3.1-1) In the cache, all resource records in the Answer section MUST be tied to the network specified in the response. (§7.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze ties no cached record to a network; the DNS cache keys entries by name+qtype only (internal/component/resolve/dns/cache.go:16) and geodns answers each query fresh from config (internal/plugins/geodns/server.go:168) |
| [`RFC7871-7.3.1-3`](#rfc7871-7.3.1-3) Records from the Additional and Authority sections MUST NOT be tied to a network. (§7.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no network-scoped answer cache, so no Additional/Authority record is ever tied to a network (internal/component/resolve/dns/cache.go:16) |
| [`RFC7871-7.3.1-4`](#rfc7871-7.3.1-4) Records cached as /0 because of a query's SOURCE PREFIX-LENGTH of 0 MUST be distinguished from those cached as /0 because of a response's SCOPE PREFIX-LENGTH of 0. (§7.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze caches no ECS-scoped answers, so it draws no distinction between a /0 from SOURCE 0 and a /0 from SCOPE 0 (internal/component/resolve/dns/cache.go:16) |
| [`RFC7871-7.3.2-1`](#rfc7871-7.3.2-1) The appropriate RRset MUST be chosen based on longest-prefix matching. (§7.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze selects no cached RRset by ECS longest-prefix; geodns's longest-prefix match runs over operator-configured source prefixes (internal/plugins/geodns/server.go:77, internal/core/dnsserver/matcher.go:40), an authoritative selection the RFC does not govern, not an ECS-response cache |
| [`RFC7871-7.3.2-4`](#rfc7871-7.3.2-4) If no matching network is found, the Intermediate Nameserver MUST perform resolution as usual. (§7.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no ECS-response cache to miss; geodns answers authoritatively and, on no host-set match, returns a normal NOERROR negative rather than a cache fallthrough (internal/plugins/geodns/server.go:182) |
| [`RFC7871-7.5-1`](#rfc7871-7.5-1) Any Intermediate Nameserver that forwards ECS options received from its clients MUST fully implement the caching behavior of §7.3. (§7.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no client ECS option and keeps no section 7.3 network-scoped cache; it consumes the ADDRESS locally (internal/core/dnsserver/client.go:21) and the cache is keyed by name+qtype (internal/component/resolve/dns/cache.go:16) |
| [`RFC7871-9-1`](#rfc7871-9-1) RRSIG records MUST be tied to the RRset they sign in a Tailored Response. (§9) | no test | no test carries this requirement id; annotated {not-applicable}: geodns emits no RRSIG or DNSSEC records and no SCOPE-scoped Tailored Response; it synthesizes only unsigned A/AAAA/SRV/SOA/NS answers (internal/plugins/geodns/server.go:106) |
| [`RFC7871-11.1-2`](#rfc7871-11.1-2) Recursive Resolvers forwarding the ECS option MUST NOT modify it to include the network address of the client. (§11.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no ECS option, so it modifies none in flight; it reads the incoming ADDRESS only for local selection (internal/core/dnsserver/client.go:21) |
| [`RFC7871-11.2-1`](#rfc7871-11.2-1) Intermediate Nameservers processing a response MUST verify that FAMILY, ADDRESS, and SOURCE PREFIX-LENGTH match those of the corresponding query. (§11.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze is no Intermediate Nameserver validating an ECS response; it issues no ECS query whose response fields it would verify (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-11.3-4`](#rfc7871-11.3-4) Recursive Resolvers MUST NOT send an ECS option whose SOURCE PREFIX-LENGTH provides more bits of ADDRESS than they are willing to cache responses for. (§11.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze sends no ECS-bearing query, so it caps no outgoing SOURCE PREFIX-LENGTH by cache willingness (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-12.1-3`](#rfc7871-12.1-3) Probing, if implemented, MUST be repeated periodically (e.g., daily). (§12.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no ECS support-probing of authoritative servers, so no periodic-probe obligation arises; it consumes ECS only on the serving side (internal/core/dnsserver/client.go:21) |
| [`RFC7871-12.1-4`](#rfc7871-12.1-4) An Authoritative Nameserver that uses ECS information for one of its zones MUST indicate support for the option in all of its responses to ECS queries. (§12.1) | {gap}, no test | geodns uses ECS information for its zones (internal/core/dnsserver/client.go:21) but indicates support in none of its responses, emitting no ECS option (internal/plugins/geodns/server.go:168) |
| [`RFC7871-12.1-5`](#rfc7871-12.1-5) If the option is supported but not actually used to generate a response, its SCOPE PREFIX-LENGTH MUST be set to 0. (§12.1) | {gap}, no test | geodns emits no ECS option in its response (internal/plugins/geodns/server.go:168), so it sets no SCOPE PREFIX-LENGTH to 0 to mark the option supported but unused |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7871-6-1`](#rfc7871-6-1)

In a query, SCOPE PREFIX-LENGTH MUST be set to 0. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-6-1, so no unit is bound to it.

### [`RFC7871-6-2`](#rfc7871-6-2)

ADDRESS MUST be truncated to the number of bits given by SOURCE PREFIX-LENGTH, padding with 0 bits to the end of the last octet needed. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-6-2, so no unit is bound to it.

### [`RFC7871-7.1.1-2`](#rfc7871-7.1.1-2)

If the triggering client query included an ECS option, it MUST be examined for its SOURCE PREFIX-LENGTH. (§7.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.1-2, so no unit is bound to it.

### [`RFC7871-7.1.1-3`](#rfc7871-7.1.1-3)

The Recursive Resolver's outgoing query MUST set SOURCE PREFIX-LENGTH to the shorter of the incoming query's SOURCE PREFIX-LENGTH or the server's maximum cacheable prefix length. (§7.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.1-3, so no unit is bound to it.

### [`RFC7871-7.1.1-4`](#rfc7871-7.1.1-4)

The number of ADDRESS octets used MUST cover only SOURCE PREFIX-LENGTH bits, not the full width normally used by FAMILY. (§7.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.1-4, so no unit is bound to it.

### [`RFC7871-7.1.1-7`](#rfc7871-7.1.1-7)

Subsequent queries to refresh the data MUST, if unrestricted by an incoming SOURCE PREFIX-LENGTH, specify the longest SOURCE PREFIX-LENGTH the Recursive Resolver is willing to cache, even if a previous response indicated a shorter prefix sufficed. (§7.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.1-7, so no unit is bound to it.

### [`RFC7871-7.1.2-2`](#rfc7871-7.1.2-2)

An Intermediate Nameserver receiving a query that limits SOURCE PREFIX-LENGTH MUST NOT make queries that include more bits of client address than the originating query. (§7.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.2-2, so no unit is bound to it.

### [`RFC7871-7.1.2-3`](#rfc7871-7.1.2-3)

A SOURCE PREFIX-LENGTH of 0 means the Recursive Resolver MUST NOT add the client's address information to its queries; §7.5 restates this obligation for any Intermediate Nameserver. (§7.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.2-3, so no unit is bound to it.

### [`RFC7871-7.1.2-5`](#rfc7871-7.1.2-5)

A Stub Resolver MUST set SCOPE PREFIX-LENGTH to 0. (§7.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.2-5, so no unit is bound to it.

### [`RFC7871-7.1.3-1`](#rfc7871-7.1.3-1)

A Forwarding Resolver using this option MUST prepare it as described for Recursive Resolvers in §7.1.1. (§7.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.3-1, so no unit is bound to it.

### [`RFC7871-7.1.3-2`](#rfc7871-7.1.3-2)

A Forwarding Resolver that implements this protocol MUST honor the SOURCE PREFIX-LENGTH restrictions indicated in the incoming query from its client. (§7.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.3-2, so no unit is bound to it.

### [`RFC7871-7.1.3-3`](#rfc7871-7.1.3-3)

If a Forwarding Resolver receives a REFUSED response to a query that includes a non-zero ADDRESS, it MUST retry with no ADDRESS. (§7.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.3-3, so no unit is bound to it.

### [`RFC7871-7.2.1-2`](#rfc7871-7.2.1-2)

A server that has not implemented or enabled ECS MUST NOT include an ECS option in replies to indicate lack of support. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-2, so no unit is bound to it.

### [`RFC7871-7.2.1-3`](#rfc7871-7.2.1-3)

A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-3, so no unit is bound to it.

### [`RFC7871-7.2.1-4`](#rfc7871-7.2.1-4)

A FORMERR response MUST be returned to the sender for a wrongly formatted option. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-4, so no unit is bound to it.

### [`RFC7871-7.2.1-5`](#rfc7871-7.2.1-5)

An Authoritative Nameserver implementing ECS that receives an ECS option MUST include an ECS option in its response, regardless of whether the client information was needed. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-5, so no unit is bound to it.

### [`RFC7871-7.2.1-7`](#rfc7871-7.2.1-7)

If an ECS option was not included in the query, one MUST NOT be included in the response, even when the server provides a Tailored Response. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7871_NoECSQueryNoECSResponse`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L72) | unit/verify | unproven |

### [`RFC7871-7.2.1-8`](#rfc7871-7.2.1-8)

FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the response MUST match those in the query; the same echo requirement is restated as an anti-spoofing measure in §11.2. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-8, so no unit is bound to it.

### [`RFC7871-7.2.1-12`](#rfc7871-7.2.1-12)

An Authoritative Nameserver MUST NOT overlap prefixes among its Tailored Responses. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-12, so no unit is bound to it.

### [`RFC7871-7.2.2-1`](#rfc7871-7.2.2-1)

If the client query did not include an ECS option, the server MUST NOT provide one in its response. (§7.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7871_NoECSQueryNoECSResponse`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L77) | unit/verify | unproven |

### [`RFC7871-7.2.2-2`](#rfc7871-7.2.2-2)

If the client query did include the option, the server MUST include one in its response. (§7.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.2-2, so no unit is bound to it.

### [`RFC7871-7.3-3`](#rfc7871-7.3-3)

If FAMILY, SOURCE PREFIX-LENGTH, and the SOURCE PREFIX-LENGTH bits of ADDRESS in the response do not match the non-zero fields in the corresponding query, the full response MUST be dropped. (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3-3, so no unit is bound to it.

### [`RFC7871-7.3-4`](#rfc7871-7.3-4)

In a response to a query that specified only SOURCE PREFIX-LENGTH for privacy masking, the FAMILY and ADDRESS fields MUST contain the appropriate non-zero information the Authoritative Nameserver used to generate the answer. (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3-4, so no unit is bound to it.

### [`RFC7871-7.3-6`](#rfc7871-7.3-6)

If a REFUSED response is received from an Authoritative Nameserver, an ECS-aware resolver MUST retry the query without ECS. (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3-6, so no unit is bound to it.

### [`RFC7871-7.3.1-1`](#rfc7871-7.3.1-1)

In the cache, all resource records in the Answer section MUST be tied to the network specified in the response. (§7.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3.1-1, so no unit is bound to it.

### [`RFC7871-7.3.1-3`](#rfc7871-7.3.1-3)

Records from the Additional and Authority sections MUST NOT be tied to a network. (§7.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3.1-3, so no unit is bound to it.

### [`RFC7871-7.3.1-4`](#rfc7871-7.3.1-4)

Records cached as /0 because of a query's SOURCE PREFIX-LENGTH of 0 MUST be distinguished from those cached as /0 because of a response's SCOPE PREFIX-LENGTH of 0. (§7.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3.1-4, so no unit is bound to it.

### [`RFC7871-7.3.2-1`](#rfc7871-7.3.2-1)

The appropriate RRset MUST be chosen based on longest-prefix matching. (§7.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3.2-1, so no unit is bound to it.

### [`RFC7871-7.3.2-4`](#rfc7871-7.3.2-4)

If no matching network is found, the Intermediate Nameserver MUST perform resolution as usual. (§7.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3.2-4, so no unit is bound to it.

### [`RFC7871-7.5-1`](#rfc7871-7.5-1)

Any Intermediate Nameserver that forwards ECS options received from its clients MUST fully implement the caching behavior of §7.3. (§7.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.5-1, so no unit is bound to it.

### [`RFC7871-7.5-6`](#rfc7871-7.5-6)

A query MUST NOT be refused solely because it provides 0 address bits. (§7.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7871_ZeroAddressBitsNotRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L113) | unit/verify | unproven |

### [`RFC7871-9-1`](#rfc7871-9-1)

RRSIG records MUST be tied to the RRset they sign in a Tailored Response. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-9-1, so no unit is bound to it.

### [`RFC7871-11.1-2`](#rfc7871-11.1-2)

Recursive Resolvers forwarding the ECS option MUST NOT modify it to include the network address of the client. (§11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-11.1-2, so no unit is bound to it.

### [`RFC7871-11.2-1`](#rfc7871-11.2-1)

Intermediate Nameservers processing a response MUST verify that FAMILY, ADDRESS, and SOURCE PREFIX-LENGTH match those of the corresponding query. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-11.2-1, so no unit is bound to it.

### [`RFC7871-11.3-4`](#rfc7871-11.3-4)

Recursive Resolvers MUST NOT send an ECS option whose SOURCE PREFIX-LENGTH provides more bits of ADDRESS than they are willing to cache responses for. (§11.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-11.3-4, so no unit is bound to it.

### [`RFC7871-12.1-3`](#rfc7871-12.1-3)

Probing, if implemented, MUST be repeated periodically (e.g., daily). (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-12.1-3, so no unit is bound to it.

### [`RFC7871-12.1-4`](#rfc7871-12.1-4)

An Authoritative Nameserver that uses ECS information for one of its zones MUST indicate support for the option in all of its responses to ECS queries. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-12.1-4, so no unit is bound to it.

### [`RFC7871-12.1-5`](#rfc7871-12.1-5)

If the option is supported but not actually used to generate a response, its SCOPE PREFIX-LENGTH MUST be set to 0. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-12.1-5, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 7871, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7871, so its obligations are stated where they were written.
