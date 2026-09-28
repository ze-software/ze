# RFC 7871 - Client Subnet in DNS Queries

Partial. Every requirement this repository extracted from RFC 7871, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 38 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 7.9% | 3 of 38 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 38 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 3 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 38 | of 90 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 29 | of 38 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 76.3% | 29 of 38 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 38 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 38 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 15.8% | 6 of 38 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 3 | of 38 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 38 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 90 |
| Gated MUST-level | 38 |
| Not applicable, so out of scope | 29 |
| Declared gaps | 6 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 3 |
| Tagged units | 3 |
| Recorded audit verdicts | 3 |
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
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **38** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (38):** [`RFC7871-6-1`](#rfc7871-6-1), [`RFC7871-6-2`](#rfc7871-6-2), [`RFC7871-7.1.1-2`](#rfc7871-7.1.1-2), [`RFC7871-7.1.1-3`](#rfc7871-7.1.1-3), [`RFC7871-7.1.1-4`](#rfc7871-7.1.1-4), [`RFC7871-7.1.1-7`](#rfc7871-7.1.1-7), [`RFC7871-7.1.2-2`](#rfc7871-7.1.2-2), [`RFC7871-7.1.2-3`](#rfc7871-7.1.2-3), [`RFC7871-7.1.2-5`](#rfc7871-7.1.2-5), [`RFC7871-7.1.3-1`](#rfc7871-7.1.3-1), [`RFC7871-7.1.3-2`](#rfc7871-7.1.3-2), [`RFC7871-7.1.3-3`](#rfc7871-7.1.3-3), [`RFC7871-7.2.1-2`](#rfc7871-7.2.1-2), [`RFC7871-7.2.1-3`](#rfc7871-7.2.1-3), [`RFC7871-7.2.1-4`](#rfc7871-7.2.1-4), [`RFC7871-7.2.1-5`](#rfc7871-7.2.1-5), [`RFC7871-7.2.1-7`](#rfc7871-7.2.1-7), [`RFC7871-7.2.1-8`](#rfc7871-7.2.1-8), [`RFC7871-7.2.1-12`](#rfc7871-7.2.1-12), [`RFC7871-7.2.2-1`](#rfc7871-7.2.2-1), [`RFC7871-7.2.2-2`](#rfc7871-7.2.2-2), [`RFC7871-7.3-3`](#rfc7871-7.3-3), [`RFC7871-7.3-4`](#rfc7871-7.3-4), [`RFC7871-7.3-6`](#rfc7871-7.3-6), [`RFC7871-7.3.1-1`](#rfc7871-7.3.1-1), [`RFC7871-7.3.1-3`](#rfc7871-7.3.1-3), [`RFC7871-7.3.1-4`](#rfc7871-7.3.1-4), [`RFC7871-7.3.2-1`](#rfc7871-7.3.2-1), [`RFC7871-7.3.2-4`](#rfc7871-7.3.2-4), [`RFC7871-7.5-1`](#rfc7871-7.5-1), [`RFC7871-7.5-6`](#rfc7871-7.5-6), [`RFC7871-9-1`](#rfc7871-9-1), [`RFC7871-11.1-2`](#rfc7871-11.1-2), [`RFC7871-11.2-1`](#rfc7871-11.2-1), [`RFC7871-11.3-4`](#rfc7871-11.3-4), [`RFC7871-12.1-3`](#rfc7871-12.1-3), [`RFC7871-12.1-4`](#rfc7871-12.1-4), [`RFC7871-12.1-5`](#rfc7871-12.1-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7871-6-1` | SCOPE PREFIX-LENGTH, an unsigned octet representing the leftmost number of significant bits of ADDRESS that the response covers. In queries, it MUST be set to 0. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no ECS-bearing query; the stub resolver sets only the EDNS0 DO bit and adds no client-subnet option (internal/component/resolve/dns/resolver.go:261), so it never writes a query SCOPE PREFIX-LENGTH |
| `RFC7871-6-2` | o ADDRESS, variable number of octets, contains either an IPv4 or IPv6 address, depending on FAMILY, which MUST be truncated to the number of bits indicated by the SOURCE PREFIX-LENGTH field, padding with 0 bits to pad to the end of the last octet needed. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze constructs no ECS option in any query or response; the stub resolver adds none (internal/component/resolve/dns/resolver.go:261) and geodns answers with only A/AAAA/SRV/SOA/NS records (internal/plugins/geodns/server.go:168), so it truncates no ADDRESS it built |
| `RFC7871-7.1.1-2` | If the triggering query included an ECS option itself, it MUST be examined for its SOURCE PREFIX-LENGTH. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is no Recursive Resolver forming ECS-bearing outgoing queries; it reads an incoming ECS ADDRESS only for source selection (internal/core/dnsserver/client.go:21) and originates no ECS query to size (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.1-3` | The Recursive Resolver's outgoing query MUST then set SOURCE PREFIX-LENGTH to the shorter of the incoming query's SOURCE PREFIX-LENGTH or the server's maximum cacheable prefix length. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates no ECS-bearing outgoing query, so it sets no outgoing SOURCE PREFIX-LENGTH (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.1-4` | The total number of octets used MUST only be enough to cover SOURCE PREFIX- LENGTH bits, rather than the full width that would normally be used by addresses in FAMILY. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no ECS option, so it sizes no ADDRESS octet count (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.1-7` | Subsequent queries to refresh the data MUST, if unrestricted by an incoming SOURCE PREFIX-LENGTH, specify the longest SOURCE PREFIX- LENGTH that the Recursive Resolver is willing to cache, even if a previous response indicated that a shorter prefix length was sufficient. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze issues no ECS-bearing refresh query and caches no ECS-scoped data to refresh (stub cache keyed by name+qtype at internal/component/resolve/dns/cache.go:16, no ECS query at internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.2-2` | An Intermediate Nameserver that receives such a query MUST NOT make queries that include more bits of client address than in the originating query. (§7.1.2) | MUST NOT | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is no Intermediate Nameserver forwarding ECS-bearing queries; it originates none (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.2-3` | A SOURCE PREFIX-LENGTH value of 0 means that the Recursive Resolver MUST NOT add the client's address information to its queries. (§7.1.2) | MUST NOT | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze adds no client address to any outgoing query; the stub resolver emits no ECS option (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.2-5` | A Stub Resolver MUST set SCOPE PREFIX-LENGTH to 0. (§7.1.2) | MUST | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's stub resolver emits no ECS option, so it writes no SCOPE PREFIX-LENGTH (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.3-1` | A Forwarding Resolver using this option MUST prepare it as described in Section 7.1.1, "Recursive Resolvers". (§7.1.3) | MUST | 7.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no Forwarding Resolver that prepares ECS options; geodns answers authoritatively (internal/plugins/geodns/server.go:221) and the stub resolver forwards no ECS (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.3-2` | In particular, a Forwarding Resolver that implements this protocol MUST honor SOURCE PREFIX- LENGTH restrictions indicated in the incoming query from its client. (§7.1.3) | MUST | 7.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no ECS-bearing client query, so it honors no incoming SOURCE PREFIX-LENGTH on a forward path (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.1.3-3` | If the Forwarding Resolver receives a REFUSED response when it sends a query that includes a non-zero ADDRESS, it MUST retry with no ADDRESS. (§7.1.3) | MUST | 7.1.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze sends no ECS-bearing query that could draw a REFUSED needing an ADDRESS-stripped retry (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.2.1-2` | Such a server MUST NOT include an ECS option within replies to indicate lack of support for it. (§7.2.1) | MUST NOT | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** geodns emits no ECS option in any reply (internal/plugins/geodns/server.go:168), so it never sends one to signal lack of support; this prohibition targets servers with no ECS support |
| `RFC7871-7.2.1-3` | A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns reads the incoming ECS ADDRESS without validating its FAMILY and rejects no unknown-FAMILY option; internal/core/dnsserver/client.go:26 takes ecs.Address unconditionally, with no FORMERR path |
| `RFC7871-7.2.1-4` | A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected and a FORMERR response MUST be returned to the sender (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns returns no FORMERR for a wrongly formatted ECS option it consumes; internal/core/dnsserver/client.go:21 has no rejection path and internal/plugins/geodns/server.go:221 never sets FORMERR |
| `RFC7871-7.2.1-5` | An Authoritative Nameserver that implements this protocol and receives an ECS option MUST include an ECS option in its response to indicate that it SHOULD be cached accordingly, regardless of whether the client information was needed to formulate an answer. (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns uses the ECS ADDRESS to tailor answers (internal/core/dnsserver/client.go:21) but includes no ECS option in its response (internal/plugins/geodns/server.go:168), so it does not echo the option |
| `RFC7871-7.2.1-7` | (Note that the requirement in [RFC6891] to reserve space for the OPT record could mean that the Answer section of the response will be truncated and fall back to TCP indicated accordingly.) If an ECS option was not included in a query, one MUST NOT be included in the response even if the server is providing a Tailored Response -- presumably based on the address from which it received the query. (§7.2.1) | MUST NOT | 7.2.1 | **positive:** `unit/verify` [`TestRFC7871_NoECSQueryNoECSResponse`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L72). **negative:** no negative test. **{single-polarity}:** geodns emits no ECS option in any response (internal/plugins/geodns/server.go:168), so the complementary state (an ECS option present when the query carried none) cannot be constructed; the positive case, an ECS-less query that still yields a tailored answer producing an ECS-less response, is pinned in internal/plugins/geodns/rfc7871_server_test.go |
| `RFC7871-7.2.1-8` | FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the response MUST match those in the query. (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** geodns includes no ECS option in its response (internal/plugins/geodns/server.go:168), so there are no echoed FAMILY/SOURCE/ADDRESS fields that could fail to match the query; the missing echo itself is disclosed as the RFC7871-7.2.1-5 gap |
| `RFC7871-7.2.1-12` | Because it can't be guaranteed that queries for all longer prefix lengths would arrive before one that would be answered by the shorter prefix length, an Authoritative Nameserver MUST NOT overlap prefixes. (§7.2.1) | MUST NOT | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** geodns publishes no SCOPE-scoped Tailored Response whose prefixes a downstream cache could order-dependently overlap; its config source prefixes resolve deterministically by longest-prefix at lookup (internal/core/dnsserver/matcher.go:28) |
| `RFC7871-7.2.2-1` | Because a client that did not use an ECS option might not be able to understand it, the server MUST NOT provide one in its response. (§7.2.2) | MUST NOT | 7.2.2 | **positive:** `unit/verify` [`TestRFC7871_NoECSQueryNoECSResponse`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L77). **negative:** no negative test. **{single-polarity}:** geodns includes no ECS option in any response (internal/plugins/geodns/server.go:168), so no negative case exists; internal/plugins/geodns/rfc7871_server_test.go asserts an ECS-less query draws an ECS-less response |
| `RFC7871-7.2.2-2` | If the client query did include the option, the server MUST include one in its response, especially as it could be talking to a Forwarding Resolver, which would need the information for its own caching. (§7.2.2) | MUST | 7.2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns consumes the query's ECS option to select an answer yet includes no ECS option in its response (internal/plugins/geodns/server.go:168) |
| `RFC7871-7.3-3` | If FAMILY, SOURCE PREFIX-LENGTH, and SOURCE PREFIX-LENGTH bits of ADDRESS in the response don't match the non-zero fields in the corresponding query, the full response MUST be dropped, as described in Section 11. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze issues no ECS-bearing query and so validates no ECS response for a FAMILY/SOURCE/ADDRESS mismatch to drop (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.3-4` | In a response to a query that specified only SOURCE PREFIX-LENGTH for privacy masking, the FAMILY and ADDRESS fields MUST contain the appropriate non-zero information that the Authoritative Nameserver used to generate the answer, so that it can be cached accordingly. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** geodns emits no ECS option in its response (internal/plugins/geodns/server.go:168), so it carries no FAMILY/ADDRESS response fields; the absent echo is disclosed as the RFC7871-7.2.1-5 gap |
| `RFC7871-7.3-6` | If a REFUSED response is received from an Authoritative Nameserver, an ECS-aware resolver MUST retry the query without ECS to distinguish the response from one where the Authoritative Nameserver is not responsible for the name, which is a common convention for the REFUSED status. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze sends no ECS-bearing query, so it has no ECS query to retry without on a REFUSED (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-7.3.1-1` | In the cache, all resource records in the Answer section MUST be tied to the network specified in the response. (§7.3.1) | MUST | 7.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze ties no cached record to a network; the DNS cache keys entries by name+qtype only (internal/component/resolve/dns/cache.go:16) and geodns answers each query fresh from config (internal/plugins/geodns/server.go:168) |
| `RFC7871-7.3.1-3` | Any records from these sections MUST NOT be tied to a network. (§7.3.1) | MUST NOT | 7.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no network-scoped answer cache, so no Additional/Authority record is ever tied to a network (internal/component/resolve/dns/cache.go:16) |
| `RFC7871-7.3.1-4` | Records that are cached as /0 because of a query's SOURCE PREFIX- LENGTH of 0 MUST be distinguished from those that are cached as /0 because of a response's SCOPE PREFIX-LENGTH of 0. (§7.3.1) | MUST | 7.3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze caches no ECS-scoped answers, so it draws no distinction between a /0 from SOURCE 0 and a /0 from SCOPE 0 (internal/component/resolve/dns/cache.go:16) |
| `RFC7871-7.3.2-1` | Then, the appropriate RRset MUST be chosen based on the longest prefix matching. (§7.3.2) | MUST | 7.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze selects no cached RRset by ECS longest-prefix; geodns's longest-prefix match runs over operator-configured source prefixes (internal/plugins/geodns/server.go:77, internal/core/dnsserver/matcher.go:40), an authoritative selection the RFC does not govern, not an ECS-response cache |
| `RFC7871-7.3.2-4` | If no matching network is found, the Intermediate Nameserver MUST perform resolution as usual. (§7.3.2) | MUST | 7.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no ECS-response cache to miss; geodns answers authoritatively and, on no host-set match, returns a normal NOERROR negative rather than a cache fallthrough (internal/plugins/geodns/server.go:182) |
| `RFC7871-7.5-1` | Any Intermediate Nameserver that forwards ECS options received from its clients MUST fully implement the caching behavior described in Section 7.3. (§7.5) | MUST | 7.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no client ECS option and keeps no section 7.3 network-scoped cache; it consumes the ADDRESS locally (internal/core/dnsserver/client.go:21) and the cache is keyed by name+qtype (internal/component/resolve/dns/cache.go:16) |
| `RFC7871-7.5-6` | Note again that a query MUST NOT be refused solely because it provides 0 address bits. (§7.5) | MUST NOT | 7.5 | **positive:** `unit/verify` [`TestRFC7871_ZeroAddressBitsNotRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L113). **negative:** no negative test. **{single-polarity}:** geodns refuses only when disabled and never on ECS content (internal/plugins/geodns/server.go:236), so a query carrying 0 address bits is answered, not refused; the refuse-for-0-bits case is unreachable and internal/plugins/geodns/rfc7871_server_test.go pins the positive |
| `RFC7871-9-1` | Most DNSSEC records SHOULD be scoped at /0, except for the RRSIG records, which MUST be tied to the RRset that they sign in a Tailored Response. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** geodns emits no RRSIG or DNSSEC records and no SCOPE-scoped Tailored Response; it synthesizes only unsigned A/AAAA/SRV/SOA/NS answers (internal/plugins/geodns/server.go:106) |
| `RFC7871-11.1-2` | As described in previous sections, this option will be forwarded across all the Recursive Resolvers supporting ECS, which MUST NOT modify it to include the network address of the client. (§11.1) | MUST NOT | 11.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze forwards no ECS option, so it modifies none in flight; it reads the incoming ADDRESS only for local selection (internal/core/dnsserver/client.go:21) |
| `RFC7871-11.2-1` | To counter this, the ECS option in a response packet MUST contain the full FAMILY, ADDRESS, and SOURCE PREFIX-LENGTH fields from the corresponding query. Intermediate Nameservers processing a response MUST verify that these match (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is no Intermediate Nameserver validating an ECS response; it issues no ECS query whose response fields it would verify (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-11.3-4` | o Recursive Resolvers MUST NOT send an ECS option with SOURCE PREFIX-LENGTH providing more bits in ADDRESS than they are willing to cache responses for. (§11.3) | MUST NOT | 11.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze sends no ECS-bearing query, so it caps no outgoing SOURCE PREFIX-LENGTH by cache willingness (internal/component/resolve/dns/resolver.go:261) |
| `RFC7871-12.1-3` | Probing, if implemented, MUST be repeated periodically, e.g., daily. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no ECS support-probing of authoritative servers, so no periodic-probe obligation arises; it consumes ECS only on the serving side (internal/core/dnsserver/client.go:21) |
| `RFC7871-12.1-4` | Likewise, an Authoritative Nameserver that uses ECS information for one of its zones MUST indicate support for the option in all of its responses to ECS queries. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns uses ECS information for its zones (internal/core/dnsserver/client.go:21) but indicates support in none of its responses, emitting no ECS option (internal/plugins/geodns/server.go:168) |
| `RFC7871-12.1-5` | If the option is supported but not actually used for generating a response, its SCOPE PREFIX- LENGTH MUST be set to 0. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns emits no ECS option in its response (internal/plugins/geodns/server.go:168), so it sets no SCOPE PREFIX-LENGTH to 0 to mark the option supported but unused |
| `RFC7871-6-3` | A server receiving an ECS option that uses either too few or too many ADDRESS octets, or that has non-zero ADDRESS bits set beyond SOURCE PREFIX-LENGTH, SHOULD return FORMERR to reject the packet, as a signal to the software developer making the request to fix their implementation. (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.1-1` | For privacy reasons, and because the whole IP address is rarely required to determine a tailored response, this length SHOULD be shorter than the full address, as described in Section 11. (§7.1.1) | SHOULD | 7.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.1-6` | If the Recursive Resolver will not forward FAMILY and ADDRESS data from the incoming ECS option, it SHOULD return a REFUSED response. (§7.1.1) | SHOULD | 7.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.2-7` | If there is no ADDRESS set, i.e., SOURCE PREFIX-LENGTH is set to 0, then FAMILY SHOULD be set to the transport over which the query is sent. (§7.1.2) | SHOULD | 7.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.1-6` | An Authoritative Nameserver that implements this protocol and receives an ECS option MUST include an ECS option in its response to indicate that it SHOULD be cached accordingly (§7.2.1) | SHOULD | 7.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.1-9` | Future queries for the name within the specified network SHOULD use the longer SCOPE PREFIX-LENGTH. (§7.2.1) | SHOULD | 7.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.1-10` | The response SHOULD therefore, include the longest relevant PREFIX-LENGTH of any RRset in the answer, which could have the unfortunate side effect of redundantly caching some data that could be cached more broadly. (§7.2.1) | SHOULD | 7.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.1-11` | For the specific case of a Canonical Name (CNAME) chain, the Authoritative Nameserver SHOULD only place the initial CNAME record in the Answer section, to have it cached unambiguously and appropriately. (§7.2.1) | SHOULD | 7.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.2-3` | If an Intermediate Nameserver receives a response that has a longer SCOPE PREFIX-LENGTH than SOURCE PREFIX-LENGTH that it provided in its query, it SHOULD still provide the result as the answer to the triggering client request even if the client is in a different address range. (§7.2.2) | SHOULD | 7.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3-1` | When an Intermediate Nameserver receives a response containing an ECS option and without the TC bit set, it SHOULD cache the result based on the data in the option. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3-2` | If the TC bit was set, the Intermediate Resolver SHOULD retry the query over TCP to get the complete Answer section for caching. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3-5` | If no ECS option is contained in the response, the Intermediate Nameserver SHOULD treat this as being equivalent to having received a SCOPE PREFIX-LENGTH of 0, which is an answer suitable for all client addresses. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3-7` | Similarly, a client of a Recursive Resolver SHOULD retry after receiving a REFUSED response because it is not sufficiently clear whether the REFUSED response was because of the ECS option or some other reason. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3.1-2` | A Resource Record Signature (RRSIG) must obviously be tied to the RRset that it signs, but it is RECOMMENDED that all other DNSSEC records be scoped at /0. (§7.3.1) | RECOMMENDED | 7.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3.1-5` | Therefore, implementing full caching support as described in this section is strongly RECOMMENDED. (§7.3.1) | RECOMMENDED | 7.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3.2-3` | If the policy does not allow it, a REFUSED response SHOULD be sent. (§7.3.2) | SHOULD | 7.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.4-1` | It is RECOMMENDED that no specific behavior regarding negative answers be relied upon (§7.4) | RECOMMENDED | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.4-2` | but that Authoritative Nameservers should conservatively expect that Intermediate Nameservers will treat all negative answers as /0; therefore, they SHOULD set SCOPE PREFIX- LENGTH accordingly. (§7.4) | SHOULD | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.4-3` | Addresses in the Additional section SHOULD therefore ignore ECS data (§7.4) | SHOULD | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.4-4` | the Authoritative Nameserver SHOULD return a zero SCOPE PREFIX-LENGTH on delegations. (§7.4) | SHOULD | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.4-5` | A Recursive Resolver SHOULD treat a non-zero SCOPE PREFIX LENGTH in a delegation as though it were zero. (§7.4) | SHOULD | 7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.5-5` | If, for any reason, the Intermediate Nameserver does not want to use the information in an ECS option it receives (too little address information, network address from a range not authorized to use the server, private/unroutable address space, etc.), it SHOULD drop the query and return a REFUSED response. (§7.5) | SHOULD | 7.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-1` | The client's network address SHOULD NOT be added, and existing ECS options, if present, SHOULD NOT be modified by NAT devices. (§10) | SHOULD NOT | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-2` | The client's network address SHOULD NOT be added, and existing ECS options, if present, SHOULD NOT be modified by NAT devices. (§10) | SHOULD NOT | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-4` | In other cases, if a Recursive Resolver knows that it is situated behind a NAT device, it SHOULD NOT originate ECS options with their external IP address and instead rely on downstream Intermediate Nameservers to do so. (§10) | SHOULD NOT | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-6` | As a general guideline, if an Authoritative Nameserver on the publicly routed Internet receives a query that specifies an ADDRESS in [RFC1918] or [RFC4193] private address space, it SHOULD ignore ADDRESS and look up its answer based on the address of the Recursive Resolver. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-7` | In the response, it SHOULD set SCOPE PREFIX-LENGTH to cover all of the relevant private space. (§10) | SHOULD | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.1-1` | In those cases, for optimal cache utilization and improved privacy, the ISP's Recursive Resolver SHOULD truncate IP addresses in this /20 to just 20 bits, instead of 24 as recommended above. (§11.1) | SHOULD | 11.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.2-2` | Intermediate Nameservers processing a response MUST verify that these match, and they SHOULD discard the entire response if they do not. (§11.2) | SHOULD | 11.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-1` | Due to the high cache pressure introduced by ECS, the feature SHOULD be disabled in all default configurations. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-2` | Recursive Resolvers SHOULD limit the number of networks and answers they keep in the cache for any given query. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-3` | Recursive Resolvers SHOULD limit the total number of different networks that they keep in cache. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-6` | They SHOULD at least treat unroutable addresses, such as some of the address blocks defined in [RFC6890], as equivalent to the Recursive Resolver's own identity. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-7` | They SHOULD ignore and never forward ECS options specifying other routable addresses that are known not to be served by the query source. (§11.3) | SHOULD | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-12.1-1` | However, it is RECOMMENDED that resolvers remember which Authoritative Nameservers did not return the option with their response and omit client address information from subsequent queries to those nameservers. (§12.1) | RECOMMENDED | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-12.1-2` | Additionally, Recursive Resolvers SHOULD be configured never to send the option when querying root, top-level, and effective top-level (i.e., "public suffix" [Public_Suffix_List]) domain servers. (§12.1) | SHOULD | 12.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.1-5` | FAMILY and ADDRESS information MAY be used from the ECS option in the incoming query. (§7.1.1) | MAY | 7.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.2-1` | A Stub Resolver MAY generate DNS queries with an ECS option that sets SOURCE PREFIX-LENGTH to limit how network information should be revealed. (§7.1.2) | MAY | 7.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.2-4` | The subsequent Recursive Resolver query to the Authoritative Nameserver will then either not include an ECS option or MAY optionally include its own address information, which is what the Authoritative Nameserver will almost certainly use to generate any Tailored Response in lieu of an option. (§7.1.2) | MAY | 7.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.1.2-6` | It MAY include FAMILY and ADDRESS data, but should be prepared to handle a REFUSED response if the Intermediate Nameserver that it queries has a policy that denies forwarding of ADDRESS. (§7.1.2) | MAY | 7.1.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.1-1` | When a query containing an ECS option is received, an Authoritative Nameserver supporting ECS MAY use the address information specified in the option to generate a tailored response. (§7.2.1) | MAY | 7.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.2.2-4` | The Intermediate Nameserver MAY instead opt to retry with a longer SOURCE PREFIX-LENGTH to get a better reply before responding to its client, as long as it does not exceed a SOURCE PREFIX-LENGTH specified in the query that triggered resolution, but this obviously has implications for the latency of the overall lookup. (§7.2.2) | MAY | 7.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.3.2-2` | If there was an ECS option with an ADDRESS, the ADDRESS from it MAY be used if the local policy allows. (§7.3.2) | MAY | 7.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.5-2` | An Intermediate Nameserver MAY forward ECS options with address information. (§7.5) | MAY | 7.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.5-3` | This information MAY match the source IP address of the incoming query (§7.5) | MAY | 7.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-7.5-4` | MAY have more or fewer address bits than the nameserver would normally include in a locally originated ECS option. (§7.5) | MAY | 7.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-3` | an internal Intermediate Nameserver might have detailed network layout information, and may know which external subnets are used for egress traffic by each internal network. In such cases, the Intermediate Nameserver MAY use that information when originating ECS options. (§10) | MAY | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-5` | It MAY, however, choose to include the option with their internal address for the purposes of signaling its own limit for SOURCE PREFIX-LENGTH. (§10) | MAY | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-10-8` | The Intermediate Nameserver MAY elect to cache the answer under one entry for special-purpose addresses [RFC6890]; see Section 11.3 of this document. (§10) | MAY | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.1-3` | Recursive Resolvers or Authoritative Nameservers MAY use the source IP address of queries to return a cached entry or to generate a Tailored Response that best matches the query. (§11.1) | MAY | 11.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-11.3-5` | Recursive Resolvers MAY, for example, decide to discard more- specific cache entries first. (§11.3) | MAY | 11.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7871-12.2-1` | Implementations MAY also allow additional configuring of this based on other criteria, such as zone or query type. (§12.2) | MAY | 12.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7871-6-1`](#rfc7871-6-1) SCOPE PREFIX-LENGTH, an unsigned octet representing the leftmost number of significant bits of ADDRESS that the response covers. In queries, it MUST be set to 0. (§6) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no ECS-bearing query; the stub resolver sets only the EDNS0 DO bit and adds no client-subnet option (internal/component/resolve/dns/resolver.go:261), so it never writes a query SCOPE PREFIX-LENGTH |
| [`RFC7871-6-2`](#rfc7871-6-2) o ADDRESS, variable number of octets, contains either an IPv4 or IPv6 address, depending on FAMILY, which MUST be truncated to the number of bits indicated by the SOURCE PREFIX-LENGTH field, padding with 0 bits to pad to the end of the last octet needed. (§6) | no test | no test carries this requirement id; annotated {not-applicable}: ze constructs no ECS option in any query or response; the stub resolver adds none (internal/component/resolve/dns/resolver.go:261) and geodns answers with only A/AAAA/SRV/SOA/NS records (internal/plugins/geodns/server.go:168), so it truncates no ADDRESS it built |
| [`RFC7871-7.1.1-2`](#rfc7871-7.1.1-2) If the triggering query included an ECS option itself, it MUST be examined for its SOURCE PREFIX-LENGTH. (§7.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze is no Recursive Resolver forming ECS-bearing outgoing queries; it reads an incoming ECS ADDRESS only for source selection (internal/core/dnsserver/client.go:21) and originates no ECS query to size (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.1-3`](#rfc7871-7.1.1-3) The Recursive Resolver's outgoing query MUST then set SOURCE PREFIX-LENGTH to the shorter of the incoming query's SOURCE PREFIX-LENGTH or the server's maximum cacheable prefix length. (§7.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates no ECS-bearing outgoing query, so it sets no outgoing SOURCE PREFIX-LENGTH (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.1-4`](#rfc7871-7.1.1-4) The total number of octets used MUST only be enough to cover SOURCE PREFIX- LENGTH bits, rather than the full width that would normally be used by addresses in FAMILY. (§7.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no ECS option, so it sizes no ADDRESS octet count (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.1-7`](#rfc7871-7.1.1-7) Subsequent queries to refresh the data MUST, if unrestricted by an incoming SOURCE PREFIX-LENGTH, specify the longest SOURCE PREFIX- LENGTH that the Recursive Resolver is willing to cache, even if a previous response indicated that a shorter prefix length was sufficient. (§7.1.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze issues no ECS-bearing refresh query and caches no ECS-scoped data to refresh (stub cache keyed by name+qtype at internal/component/resolve/dns/cache.go:16, no ECS query at internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.2-2`](#rfc7871-7.1.2-2) An Intermediate Nameserver that receives such a query MUST NOT make queries that include more bits of client address than in the originating query. (§7.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze is no Intermediate Nameserver forwarding ECS-bearing queries; it originates none (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.2-3`](#rfc7871-7.1.2-3) A SOURCE PREFIX-LENGTH value of 0 means that the Recursive Resolver MUST NOT add the client's address information to its queries. (§7.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze adds no client address to any outgoing query; the stub resolver emits no ECS option (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.2-5`](#rfc7871-7.1.2-5) A Stub Resolver MUST set SCOPE PREFIX-LENGTH to 0. (§7.1.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's stub resolver emits no ECS option, so it writes no SCOPE PREFIX-LENGTH (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.3-1`](#rfc7871-7.1.3-1) A Forwarding Resolver using this option MUST prepare it as described in Section 7.1.1, "Recursive Resolvers". (§7.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no Forwarding Resolver that prepares ECS options; geodns answers authoritatively (internal/plugins/geodns/server.go:221) and the stub resolver forwards no ECS (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.3-2`](#rfc7871-7.1.3-2) In particular, a Forwarding Resolver that implements this protocol MUST honor SOURCE PREFIX- LENGTH restrictions indicated in the incoming query from its client. (§7.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no ECS-bearing client query, so it honors no incoming SOURCE PREFIX-LENGTH on a forward path (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.1.3-3`](#rfc7871-7.1.3-3) If the Forwarding Resolver receives a REFUSED response when it sends a query that includes a non-zero ADDRESS, it MUST retry with no ADDRESS. (§7.1.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze sends no ECS-bearing query that could draw a REFUSED needing an ADDRESS-stripped retry (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.2.1-2`](#rfc7871-7.2.1-2) Such a server MUST NOT include an ECS option within replies to indicate lack of support for it. (§7.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: geodns emits no ECS option in any reply (internal/plugins/geodns/server.go:168), so it never sends one to signal lack of support; this prohibition targets servers with no ECS support |
| [`RFC7871-7.2.1-3`](#rfc7871-7.2.1-3) A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected (§7.2.1) | {gap}, no test | geodns reads the incoming ECS ADDRESS without validating its FAMILY and rejects no unknown-FAMILY option; internal/core/dnsserver/client.go:26 takes ecs.Address unconditionally, with no FORMERR path |
| [`RFC7871-7.2.1-4`](#rfc7871-7.2.1-4) A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected and a FORMERR response MUST be returned to the sender (§7.2.1) | {gap}, no test | geodns returns no FORMERR for a wrongly formatted ECS option it consumes; internal/core/dnsserver/client.go:21 has no rejection path and internal/plugins/geodns/server.go:221 never sets FORMERR |
| [`RFC7871-7.2.1-5`](#rfc7871-7.2.1-5) An Authoritative Nameserver that implements this protocol and receives an ECS option MUST include an ECS option in its response to indicate that it SHOULD be cached accordingly, regardless of whether the client information was needed to formulate an answer. (§7.2.1) | {gap}, no test | geodns uses the ECS ADDRESS to tailor answers (internal/core/dnsserver/client.go:21) but includes no ECS option in its response (internal/plugins/geodns/server.go:168), so it does not echo the option |
| [`RFC7871-7.2.1-8`](#rfc7871-7.2.1-8) FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the response MUST match those in the query. (§7.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: geodns includes no ECS option in its response (internal/plugins/geodns/server.go:168), so there are no echoed FAMILY/SOURCE/ADDRESS fields that could fail to match the query; the missing echo itself is disclosed as the RFC7871-7.2.1-5 gap |
| [`RFC7871-7.2.1-12`](#rfc7871-7.2.1-12) Because it can't be guaranteed that queries for all longer prefix lengths would arrive before one that would be answered by the shorter prefix length, an Authoritative Nameserver MUST NOT overlap prefixes. (§7.2.1) | no test | no test carries this requirement id; annotated {not-applicable}: geodns publishes no SCOPE-scoped Tailored Response whose prefixes a downstream cache could order-dependently overlap; its config source prefixes resolve deterministically by longest-prefix at lookup (internal/core/dnsserver/matcher.go:28) |
| [`RFC7871-7.2.2-2`](#rfc7871-7.2.2-2) If the client query did include the option, the server MUST include one in its response, especially as it could be talking to a Forwarding Resolver, which would need the information for its own caching. (§7.2.2) | {gap}, no test | geodns consumes the query's ECS option to select an answer yet includes no ECS option in its response (internal/plugins/geodns/server.go:168) |
| [`RFC7871-7.3-3`](#rfc7871-7.3-3) If FAMILY, SOURCE PREFIX-LENGTH, and SOURCE PREFIX-LENGTH bits of ADDRESS in the response don't match the non-zero fields in the corresponding query, the full response MUST be dropped, as described in Section 11. (§7.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze issues no ECS-bearing query and so validates no ECS response for a FAMILY/SOURCE/ADDRESS mismatch to drop (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.3-4`](#rfc7871-7.3-4) In a response to a query that specified only SOURCE PREFIX-LENGTH for privacy masking, the FAMILY and ADDRESS fields MUST contain the appropriate non-zero information that the Authoritative Nameserver used to generate the answer, so that it can be cached accordingly. (§7.3) | no test | no test carries this requirement id; annotated {not-applicable}: geodns emits no ECS option in its response (internal/plugins/geodns/server.go:168), so it carries no FAMILY/ADDRESS response fields; the absent echo is disclosed as the RFC7871-7.2.1-5 gap |
| [`RFC7871-7.3-6`](#rfc7871-7.3-6) If a REFUSED response is received from an Authoritative Nameserver, an ECS-aware resolver MUST retry the query without ECS to distinguish the response from one where the Authoritative Nameserver is not responsible for the name, which is a common convention for the REFUSED status. (§7.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze sends no ECS-bearing query, so it has no ECS query to retry without on a REFUSED (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-7.3.1-1`](#rfc7871-7.3.1-1) In the cache, all resource records in the Answer section MUST be tied to the network specified in the response. (§7.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze ties no cached record to a network; the DNS cache keys entries by name+qtype only (internal/component/resolve/dns/cache.go:16) and geodns answers each query fresh from config (internal/plugins/geodns/server.go:168) |
| [`RFC7871-7.3.1-3`](#rfc7871-7.3.1-3) Any records from these sections MUST NOT be tied to a network. (§7.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no network-scoped answer cache, so no Additional/Authority record is ever tied to a network (internal/component/resolve/dns/cache.go:16) |
| [`RFC7871-7.3.1-4`](#rfc7871-7.3.1-4) Records that are cached as /0 because of a query's SOURCE PREFIX- LENGTH of 0 MUST be distinguished from those that are cached as /0 because of a response's SCOPE PREFIX-LENGTH of 0. (§7.3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze caches no ECS-scoped answers, so it draws no distinction between a /0 from SOURCE 0 and a /0 from SCOPE 0 (internal/component/resolve/dns/cache.go:16) |
| [`RFC7871-7.3.2-1`](#rfc7871-7.3.2-1) Then, the appropriate RRset MUST be chosen based on the longest prefix matching. (§7.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze selects no cached RRset by ECS longest-prefix; geodns's longest-prefix match runs over operator-configured source prefixes (internal/plugins/geodns/server.go:77, internal/core/dnsserver/matcher.go:40), an authoritative selection the RFC does not govern, not an ECS-response cache |
| [`RFC7871-7.3.2-4`](#rfc7871-7.3.2-4) If no matching network is found, the Intermediate Nameserver MUST perform resolution as usual. (§7.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no ECS-response cache to miss; geodns answers authoritatively and, on no host-set match, returns a normal NOERROR negative rather than a cache fallthrough (internal/plugins/geodns/server.go:182) |
| [`RFC7871-7.5-1`](#rfc7871-7.5-1) Any Intermediate Nameserver that forwards ECS options received from its clients MUST fully implement the caching behavior described in Section 7.3. (§7.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no client ECS option and keeps no section 7.3 network-scoped cache; it consumes the ADDRESS locally (internal/core/dnsserver/client.go:21) and the cache is keyed by name+qtype (internal/component/resolve/dns/cache.go:16) |
| [`RFC7871-9-1`](#rfc7871-9-1) Most DNSSEC records SHOULD be scoped at /0, except for the RRSIG records, which MUST be tied to the RRset that they sign in a Tailored Response. (§9) | no test | no test carries this requirement id; annotated {not-applicable}: geodns emits no RRSIG or DNSSEC records and no SCOPE-scoped Tailored Response; it synthesizes only unsigned A/AAAA/SRV/SOA/NS answers (internal/plugins/geodns/server.go:106) |
| [`RFC7871-11.1-2`](#rfc7871-11.1-2) As described in previous sections, this option will be forwarded across all the Recursive Resolvers supporting ECS, which MUST NOT modify it to include the network address of the client. (§11.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze forwards no ECS option, so it modifies none in flight; it reads the incoming ADDRESS only for local selection (internal/core/dnsserver/client.go:21) |
| [`RFC7871-11.2-1`](#rfc7871-11.2-1) To counter this, the ECS option in a response packet MUST contain the full FAMILY, ADDRESS, and SOURCE PREFIX-LENGTH fields from the corresponding query. Intermediate Nameservers processing a response MUST verify that these match (§11.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze is no Intermediate Nameserver validating an ECS response; it issues no ECS query whose response fields it would verify (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-11.3-4`](#rfc7871-11.3-4) o Recursive Resolvers MUST NOT send an ECS option with SOURCE PREFIX-LENGTH providing more bits in ADDRESS than they are willing to cache responses for. (§11.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze sends no ECS-bearing query, so it caps no outgoing SOURCE PREFIX-LENGTH by cache willingness (internal/component/resolve/dns/resolver.go:261) |
| [`RFC7871-12.1-3`](#rfc7871-12.1-3) Probing, if implemented, MUST be repeated periodically, e.g., daily. (§12.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no ECS support-probing of authoritative servers, so no periodic-probe obligation arises; it consumes ECS only on the serving side (internal/core/dnsserver/client.go:21) |
| [`RFC7871-12.1-4`](#rfc7871-12.1-4) Likewise, an Authoritative Nameserver that uses ECS information for one of its zones MUST indicate support for the option in all of its responses to ECS queries. (§12.1) | {gap}, no test | geodns uses ECS information for its zones (internal/core/dnsserver/client.go:21) but indicates support in none of its responses, emitting no ECS option (internal/plugins/geodns/server.go:168) |
| [`RFC7871-12.1-5`](#rfc7871-12.1-5) If the option is supported but not actually used for generating a response, its SCOPE PREFIX- LENGTH MUST be set to 0. (§12.1) | {gap}, no test | geodns emits no ECS option in its response (internal/plugins/geodns/server.go:168), so it sets no SCOPE PREFIX-LENGTH to 0 to mark the option supported but unused |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7871-6-1`](#rfc7871-6-1)

SCOPE PREFIX-LENGTH, an unsigned octet representing the leftmost number of significant bits of ADDRESS that the response covers. In queries, it MUST be set to 0. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-6-1, so no unit is bound to it.

### [`RFC7871-6-2`](#rfc7871-6-2)

o ADDRESS, variable number of octets, contains either an IPv4 or IPv6 address, depending on FAMILY, which MUST be truncated to the number of bits indicated by the SOURCE PREFIX-LENGTH field, padding with 0 bits to pad to the end of the last octet needed. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-6-2, so no unit is bound to it.

### [`RFC7871-7.1.1-2`](#rfc7871-7.1.1-2)

If the triggering query included an ECS option itself, it MUST be examined for its SOURCE PREFIX-LENGTH. (§7.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.1-2, so no unit is bound to it.

### [`RFC7871-7.1.1-3`](#rfc7871-7.1.1-3)

The Recursive Resolver's outgoing query MUST then set SOURCE PREFIX-LENGTH to the shorter of the incoming query's SOURCE PREFIX-LENGTH or the server's maximum cacheable prefix length. (§7.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.1-3, so no unit is bound to it.

### [`RFC7871-7.1.1-4`](#rfc7871-7.1.1-4)

The total number of octets used MUST only be enough to cover SOURCE PREFIX- LENGTH bits, rather than the full width that would normally be used by addresses in FAMILY. (§7.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.1-4, so no unit is bound to it.

### [`RFC7871-7.1.1-7`](#rfc7871-7.1.1-7)

Subsequent queries to refresh the data MUST, if unrestricted by an incoming SOURCE PREFIX-LENGTH, specify the longest SOURCE PREFIX- LENGTH that the Recursive Resolver is willing to cache, even if a previous response indicated that a shorter prefix length was sufficient. (§7.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.1-7, so no unit is bound to it.

### [`RFC7871-7.1.2-2`](#rfc7871-7.1.2-2)

An Intermediate Nameserver that receives such a query MUST NOT make queries that include more bits of client address than in the originating query. (§7.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.2-2, so no unit is bound to it.

### [`RFC7871-7.1.2-3`](#rfc7871-7.1.2-3)

A SOURCE PREFIX-LENGTH value of 0 means that the Recursive Resolver MUST NOT add the client's address information to its queries. (§7.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.2-3, so no unit is bound to it.

### [`RFC7871-7.1.2-5`](#rfc7871-7.1.2-5)

A Stub Resolver MUST set SCOPE PREFIX-LENGTH to 0. (§7.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.2-5, so no unit is bound to it.

### [`RFC7871-7.1.3-1`](#rfc7871-7.1.3-1)

A Forwarding Resolver using this option MUST prepare it as described in Section 7.1.1, "Recursive Resolvers". (§7.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.3-1, so no unit is bound to it.

### [`RFC7871-7.1.3-2`](#rfc7871-7.1.3-2)

In particular, a Forwarding Resolver that implements this protocol MUST honor SOURCE PREFIX- LENGTH restrictions indicated in the incoming query from its client. (§7.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.3-2, so no unit is bound to it.

### [`RFC7871-7.1.3-3`](#rfc7871-7.1.3-3)

If the Forwarding Resolver receives a REFUSED response when it sends a query that includes a non-zero ADDRESS, it MUST retry with no ADDRESS. (§7.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.1.3-3, so no unit is bound to it.

### [`RFC7871-7.2.1-2`](#rfc7871-7.2.1-2)

Such a server MUST NOT include an ECS option within replies to indicate lack of support for it. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-2, so no unit is bound to it.

### [`RFC7871-7.2.1-3`](#rfc7871-7.2.1-3)

A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-3, so no unit is bound to it.

### [`RFC7871-7.2.1-4`](#rfc7871-7.2.1-4)

A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected and a FORMERR response MUST be returned to the sender (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-4, so no unit is bound to it.

### [`RFC7871-7.2.1-5`](#rfc7871-7.2.1-5)

An Authoritative Nameserver that implements this protocol and receives an ECS option MUST include an ECS option in its response to indicate that it SHOULD be cached accordingly, regardless of whether the client information was needed to formulate an answer. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-5, so no unit is bound to it.

### [`RFC7871-7.2.1-7`](#rfc7871-7.2.1-7)

(Note that the requirement in [RFC6891] to reserve space for the OPT record could mean that the Answer section of the response will be truncated and fall back to TCP indicated accordingly.) If an ECS option was not included in a query, one MUST NOT be included in the response even if the server is providing a Tailored Response -- presumably based on the address from which it received the query. (§7.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7871_NoECSQueryNoECSResponse sends an ECS-less query whose packet source selects a tailored A (asserted 10.0.0.5), then asserts the response carries no EDNS0_SUBNET option; adding an ECS option to any geodns response turns it red. Negative polarity is unconstructible (geodns emits no ECS), as the single-polarity marker states.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7871_NoECSQueryNoECSResponse`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L72) | unit/verify | unproven |

### [`RFC7871-7.2.1-8`](#rfc7871-7.2.1-8)

FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the response MUST match those in the query. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-8, so no unit is bound to it.

### [`RFC7871-7.2.1-12`](#rfc7871-7.2.1-12)

Because it can't be guaranteed that queries for all longer prefix lengths would arrive before one that would be answered by the shorter prefix length, an Authoritative Nameserver MUST NOT overlap prefixes. (§7.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.1-12, so no unit is bound to it.

### [`RFC7871-7.2.2-1`](#rfc7871-7.2.2-1)

Because a client that did not use an ECS option might not be able to understand it, the server MUST NOT provide one in its response. (§7.2.2)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. the tag sits on a repeat of the 7.2.1-7 assertion; section 7.2.2 binds an Intermediate Nameserver, which geodns is not, so the tagged unit proves nothing for this row. Section 7.2.2 binds an Intermediate Nameserver ('When an Intermediate Nameserver uses ECS ... the server MUST NOT provide one in its response'); geodns is an Authoritative Nameserver and forwards nothing, so the role is not Ze's. The tagged assertion in TestRFC7871_NoECSQueryNoECSResponse is a byte-identical repeat of the 7.2.1-7 check, which is where the authoritative obligation is proved.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7871_NoECSQueryNoECSResponse`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L77) | unit/verify | unproven |

### [`RFC7871-7.2.2-2`](#rfc7871-7.2.2-2)

If the client query did include the option, the server MUST include one in its response, especially as it could be talking to a Forwarding Resolver, which would need the information for its own caching. (§7.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.2.2-2, so no unit is bound to it.

### [`RFC7871-7.3-3`](#rfc7871-7.3-3)

If FAMILY, SOURCE PREFIX-LENGTH, and SOURCE PREFIX-LENGTH bits of ADDRESS in the response don't match the non-zero fields in the corresponding query, the full response MUST be dropped, as described in Section 11. (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3-3, so no unit is bound to it.

### [`RFC7871-7.3-4`](#rfc7871-7.3-4)

In a response to a query that specified only SOURCE PREFIX-LENGTH for privacy masking, the FAMILY and ADDRESS fields MUST contain the appropriate non-zero information that the Authoritative Nameserver used to generate the answer, so that it can be cached accordingly. (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3-4, so no unit is bound to it.

### [`RFC7871-7.3-6`](#rfc7871-7.3-6)

If a REFUSED response is received from an Authoritative Nameserver, an ECS-aware resolver MUST retry the query without ECS to distinguish the response from one where the Authoritative Nameserver is not responsible for the name, which is a common convention for the REFUSED status. (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3-6, so no unit is bound to it.

### [`RFC7871-7.3.1-1`](#rfc7871-7.3.1-1)

In the cache, all resource records in the Answer section MUST be tied to the network specified in the response. (§7.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3.1-1, so no unit is bound to it.

### [`RFC7871-7.3.1-3`](#rfc7871-7.3.1-3)

Any records from these sections MUST NOT be tied to a network. (§7.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3.1-3, so no unit is bound to it.

### [`RFC7871-7.3.1-4`](#rfc7871-7.3.1-4)

Records that are cached as /0 because of a query's SOURCE PREFIX- LENGTH of 0 MUST be distinguished from those that are cached as /0 because of a response's SCOPE PREFIX-LENGTH of 0. (§7.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3.1-4, so no unit is bound to it.

### [`RFC7871-7.3.2-1`](#rfc7871-7.3.2-1)

Then, the appropriate RRset MUST be chosen based on the longest prefix matching. (§7.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3.2-1, so no unit is bound to it.

### [`RFC7871-7.3.2-4`](#rfc7871-7.3.2-4)

If no matching network is found, the Intermediate Nameserver MUST perform resolution as usual. (§7.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.3.2-4, so no unit is bound to it.

### [`RFC7871-7.5-1`](#rfc7871-7.5-1)

Any Intermediate Nameserver that forwards ECS options received from its clients MUST fully implement the caching behavior described in Section 7.3. (§7.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-7.5-1, so no unit is bound to it.

### [`RFC7871-7.5-6`](#rfc7871-7.5-6)

Note again that a query MUST NOT be refused solely because it provides 0 address bits. (§7.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC7871_ZeroAddressBitsNotRefused sends an ECS option with SOURCE PREFIX-LENGTH 0 and empty ADDRESS, and asserts Rcode is not REFUSED and the packet-source answer 10.0.0.5 is returned; a refusal on 0 address bits turns it red. Refuse-for-0-bits negative is unconstructible (geodns refuses only when disabled).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7871_ZeroAddressBitsNotRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L113) | unit/verify | unproven |

### [`RFC7871-9-1`](#rfc7871-9-1)

Most DNSSEC records SHOULD be scoped at /0, except for the RRSIG records, which MUST be tied to the RRset that they sign in a Tailored Response. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-9-1, so no unit is bound to it.

### [`RFC7871-11.1-2`](#rfc7871-11.1-2)

As described in previous sections, this option will be forwarded across all the Recursive Resolvers supporting ECS, which MUST NOT modify it to include the network address of the client. (§11.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-11.1-2, so no unit is bound to it.

### [`RFC7871-11.2-1`](#rfc7871-11.2-1)

To counter this, the ECS option in a response packet MUST contain the full FAMILY, ADDRESS, and SOURCE PREFIX-LENGTH fields from the corresponding query. Intermediate Nameservers processing a response MUST verify that these match (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-11.2-1, so no unit is bound to it.

### [`RFC7871-11.3-4`](#rfc7871-11.3-4)

o Recursive Resolvers MUST NOT send an ECS option with SOURCE PREFIX-LENGTH providing more bits in ADDRESS than they are willing to cache responses for. (§11.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-11.3-4, so no unit is bound to it.

### [`RFC7871-12.1-3`](#rfc7871-12.1-3)

Probing, if implemented, MUST be repeated periodically, e.g., daily. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-12.1-3, so no unit is bound to it.

### [`RFC7871-12.1-4`](#rfc7871-12.1-4)

Likewise, an Authoritative Nameserver that uses ECS information for one of its zones MUST indicate support for the option in all of its responses to ECS queries. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-12.1-4, so no unit is bound to it.

### [`RFC7871-12.1-5`](#rfc7871-12.1-5)

If the option is supported but not actually used for generating a response, its SCOPE PREFIX- LENGTH MUST be set to 0. (§12.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7871-12.1-5, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc7871.txt |
| Source fingerprint | de63f477f0265ea1 |
| Record | rfc/extraction/rfc7871.json |
| Mapped sentences | 37 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 2 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.1.1` | not stated | 4 | walked | not stated |
| `7.1.2` | not stated | 3 | walked | not stated |
| `7.1.3` | not stated | 3 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.2.1` | not stated | 6 | walked | not stated |
| `7.2.2` | not stated | 2 | walked | not stated |
| `7.3` | not stated | 3 | walked | not stated |
| `7.3.1` | not stated | 3 | walked | not stated |
| `7.3.2` | not stated | 2 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.5` | not stated | 3 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 1 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `11.1` | not stated | 1 | walked | not stated |
| `11.2` | not stated | 3 | walked | not stated |
| `11.3` | not stated | 1 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `12.1` | not stated | 3 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `14.1` | not stated | 0 | walked | not stated |
| `14.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `7.5:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates for any Intermediate Nameserver the obligation section 7.1.2 states for the Recursive Resolver, which this walk maps at 7.1.2:2: 'A SOURCE PREFIX-LENGTH value of 0 means that the Recursive Resolver MUST NOT add the client's address information to its queries.' The sentence cites that section itself ('see Section 7.1.2'), and RFC7871-7.1.2-3 already carries both readings. | If an Intermediate Nameserver receives a query with SOURCE PREFIX- LENGTH set to 0, it MUST NOT include client address information in queries made to resolve that client's request (see Section 7.1.2). |
| `11.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates as an anti-spoofing measure the echo requirement section 7.2.1 states and this walk maps at 7.2.1:5: 'FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the response MUST match those in the query.' RFC7871-7.2.1-8 already records that section 11.2 restates it. | To counter this, the ECS option in a response packet MUST contain the full FAMILY, ADDRESS, and SOURCE PREFIX-LENGTH fields from the corresponding query. |
| `11.2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Explains the LEVEL the sentence before it uses rather than stating an obligation: 'The requirement to discard is categorized as "SHOULD" instead of "MUST" because it stands in opposition to the instruction in Section 7.3'. The capitalised keywords are quoted words, the subject of the sentence. The obligations themselves are RFC7871-11.2-1 (MUST verify) and RFC7871-11.2-2 (SHOULD discard), both carried by the checklist. | The requirement to discard is categorized as "SHOULD" instead of "MUST" because it stands in opposition to the instruction in Section 7.3, which states that a response lacking an ECS option should be treated as though it had one of SCOPE PREFIX-LENGTH of 0. |

## Superseded

No document obsoletes RFC 7871, so its obligations are stated where they were written.
