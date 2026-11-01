# RFC 7871 - Client Subnet in DNS Queries

EDNS0 Client Subnet. Every requirement this repository extracted from RFC 7871, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 36 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 36 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 36 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 36 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0 | 0 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 36 | of 88 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 36 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 36 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 36 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 36 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 36 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 34 | of 36 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 3 | of 36 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 9 counts marked as a part above are the whole of the 36 gated MUSTs: they add up to 36. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | EDNS0 Client Subnet |
| Enrolment | Enrolled |
| Requirements | 88 |
| Gated MUST-level | 36 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 34 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 3 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7871.md` |
| Requirement shard | `rfc/requirements/rfc7871.md` |
| RFC text | `rfc/full/rfc7871.txt` |

## Enrolment

Enrolled: Client Subnet in DNS Queries / EDNS0 ECS (RFC 7871): ze fills four of its roles. geodns is an Authoritative Nameserver that reads the incoming EDNS0 client-subnet ADDRESS for geo source-selection (a MAY) and constructs no ECS option; the dnsresolver plugin is a caching Forwarding Resolver, and so an Intermediate Nameserver, that neither sends nor relays ECS; the resolve/dns client is a Stub Resolver that sends no ECS. 2 single-polarity positive (emits no ECS option in a response to an ECS-less query 7.2.1-7, never refuses a 0-address-bit query 7.5-6) + 34 gaps: geodns tailors answers from ECS but does not echo the option, indicate support or set SCOPE (7.2.1-5, 12.1-4, 12.1-5) nor FORMERR-reject a malformed option (7.2.1-3, 7.2.1-4), and the resolver-side, response-validation, network-scoped caching and DNSSEC-tailoring rows are owed because ze's resolvers implement no ECS (5 of them prohibitions that hold by construction, untested)

## What the public ledger says

**Area:** EDNS0 Client Subnet

**Proof:** 0 of 36 MUST proven

**Evidence:** -

**Unmet:** RFC7871-11.1-2, RFC7871-11.2-1, RFC7871-11.3-4, RFC7871-12.1-3, RFC7871-12.1-4, RFC7871-12.1-5, RFC7871-6-1, RFC7871-6-2, RFC7871-7.1.1-2, RFC7871-7.1.1-3, RFC7871-7.1.1-4, RFC7871-7.1.1-7, RFC7871-7.1.2-2, RFC7871-7.1.2-3, RFC7871-7.1.2-5, RFC7871-7.1.3-1, RFC7871-7.1.3-2, RFC7871-7.1.3-3, RFC7871-7.2.1-12, RFC7871-7.2.1-2, RFC7871-7.2.1-3, RFC7871-7.2.1-4, RFC7871-7.2.1-5, RFC7871-7.2.1-8, RFC7871-7.3-3, RFC7871-7.3-4, RFC7871-7.3-6, RFC7871-7.3.1-1, RFC7871-7.3.1-3, RFC7871-7.3.1-4, RFC7871-7.3.2-1, RFC7871-7.3.2-4, RFC7871-7.5-1, RFC7871-9-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 36 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **36** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (36):** [`RFC7871-6-1`](#rfc7871-6-1), [`RFC7871-6-2`](#rfc7871-6-2), [`RFC7871-7.1.1-2`](#rfc7871-7.1.1-2), [`RFC7871-7.1.1-3`](#rfc7871-7.1.1-3), [`RFC7871-7.1.1-4`](#rfc7871-7.1.1-4), [`RFC7871-7.1.1-7`](#rfc7871-7.1.1-7), [`RFC7871-7.1.2-2`](#rfc7871-7.1.2-2), [`RFC7871-7.1.2-3`](#rfc7871-7.1.2-3), [`RFC7871-7.1.2-5`](#rfc7871-7.1.2-5), [`RFC7871-7.1.3-1`](#rfc7871-7.1.3-1), [`RFC7871-7.1.3-2`](#rfc7871-7.1.3-2), [`RFC7871-7.1.3-3`](#rfc7871-7.1.3-3), [`RFC7871-7.2.1-2`](#rfc7871-7.2.1-2), [`RFC7871-7.2.1-3`](#rfc7871-7.2.1-3), [`RFC7871-7.2.1-4`](#rfc7871-7.2.1-4), [`RFC7871-7.2.1-5`](#rfc7871-7.2.1-5), [`RFC7871-7.2.1-7`](#rfc7871-7.2.1-7), [`RFC7871-7.2.1-8`](#rfc7871-7.2.1-8), [`RFC7871-7.2.1-12`](#rfc7871-7.2.1-12), [`RFC7871-7.3-3`](#rfc7871-7.3-3), [`RFC7871-7.3-4`](#rfc7871-7.3-4), [`RFC7871-7.3-6`](#rfc7871-7.3-6), [`RFC7871-7.3.1-1`](#rfc7871-7.3.1-1), [`RFC7871-7.3.1-3`](#rfc7871-7.3.1-3), [`RFC7871-7.3.1-4`](#rfc7871-7.3.1-4), [`RFC7871-7.3.2-1`](#rfc7871-7.3.2-1), [`RFC7871-7.3.2-4`](#rfc7871-7.3.2-4), [`RFC7871-7.5-1`](#rfc7871-7.5-1), [`RFC7871-7.5-6`](#rfc7871-7.5-6), [`RFC7871-9-1`](#rfc7871-9-1), [`RFC7871-11.1-2`](#rfc7871-11.1-2), [`RFC7871-11.2-1`](#rfc7871-11.2-1), [`RFC7871-11.3-4`](#rfc7871-11.3-4), [`RFC7871-12.1-3`](#rfc7871-12.1-3), [`RFC7871-12.1-4`](#rfc7871-12.1-4), [`RFC7871-12.1-5`](#rfc7871-12.1-5)

## Figures

- **MUST bar**: Ze correctness gap 36
- **Overview bar**: SHOULD not decided 52, Ze correctness gap 36

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 36 |
| MUST rows kept (the MUST bar) | 36 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 36 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 52 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 52 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc7871.json](../../../data/rfc/rfc7871.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7871-6-1` | SCOPE PREFIX-LENGTH, an unsigned octet representing the leftmost number of significant bits of ADDRESS that the response covers. In queries, it MUST be set to 0. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no ECS option in any query: ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), and the stub resolver sets only the EDNS0 DO bit and adds no option (internal/component/resolve/dns/resolver.go); ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-6-2` | o ADDRESS, variable number of octets, contains either an IPv4 or IPv6 address, depending on FAMILY, which MUST be truncated to the number of bits indicated by the SOURCE PREFIX-LENGTH field, padding with 0 bits to pad to the end of the last octet needed. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no ECS option in any query: ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), and the stub resolver sets only the EDNS0 DO bit and adds no option (internal/component/resolve/dns/resolver.go); ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.1.1-2` | If the triggering query included an ECS option itself, it MUST be examined for its SOURCE PREFIX-LENGTH. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the Forwarding Resolver, which ze's dnsresolver plugin is, through Section 7.1.3; ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it neither examines nor relays a client's ECS option; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.1.1-3` | The Recursive Resolver's outgoing query MUST then set SOURCE PREFIX-LENGTH to the shorter of the incoming query's SOURCE PREFIX-LENGTH or the server's maximum cacheable prefix length. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the Forwarding Resolver, which ze's dnsresolver plugin is, through Section 7.1.3; ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it neither examines nor relays a client's ECS option; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.1.1-4` | The total number of octets used MUST only be enough to cover SOURCE PREFIX- LENGTH bits, rather than the full width that would normally be used by addresses in FAMILY. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the Forwarding Resolver, which ze's dnsresolver plugin is, through Section 7.1.3; ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it neither examines nor relays a client's ECS option; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.1.1-7` | Subsequent queries to refresh the data MUST, if unrestricted by an incoming SOURCE PREFIX-LENGTH, specify the longest SOURCE PREFIX- LENGTH that the Recursive Resolver is willing to cache, even if a previous response indicated that a shorter prefix length was sufficient. (§7.1.1) | MUST | 7.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the Forwarding Resolver, which ze's dnsresolver plugin is, through Section 7.1.3; ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it neither examines nor relays a client's ECS option; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.1.2-2` | An Intermediate Nameserver that receives such a query MUST NOT make queries that include more bits of client address than in the originating query. (§7.1.2) | MUST NOT | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's forwarder is an Intermediate Nameserver, and ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it sends no client address bits at all; so the prohibition holds by construction, but no test carries this id |
| `RFC7871-7.1.2-3` | A SOURCE PREFIX-LENGTH value of 0 means that the Recursive Resolver MUST NOT add the client's address information to its queries. (§7.1.2) | MUST NOT | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds ze's forwarder, which acts as a resolver here; ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it adds no client address to any query; so the prohibition holds by construction, but no test carries this id |
| `RFC7871-7.1.2-5` | A Stub Resolver MUST set SCOPE PREFIX-LENGTH to 0. (§7.1.2) | MUST | 7.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze holds a Stub Resolver, but the stub resolver sets only the EDNS0 DO bit and adds no option (internal/component/resolve/dns/resolver.go), so it writes no SCOPE PREFIX-LENGTH; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.1.3-1` | A Forwarding Resolver using this option MUST prepare it as described in Section 7.1.1, "Recursive Resolvers". (§7.1.3) | MUST | 7.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's dnsresolver plugin is a Forwarding Resolver, but it does not use this option: ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go); ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.1.3-2` | In particular, a Forwarding Resolver that implements this protocol MUST honor SOURCE PREFIX- LENGTH restrictions indicated in the incoming query from its client. (§7.1.3) | MUST | 7.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's dnsresolver plugin is a Forwarding Resolver, but ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it honors no incoming SOURCE PREFIX-LENGTH; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.1.3-3` | If the Forwarding Resolver receives a REFUSED response when it sends a query that includes a non-zero ADDRESS, it MUST retry with no ADDRESS. (§7.1.3) | MUST | 7.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's dnsresolver plugin is a Forwarding Resolver, but ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so no query of it carries a non-zero ADDRESS that could draw this REFUSED; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.2.1-2` | Such a server MUST NOT include an ECS option within replies to indicate lack of support for it. (§7.2.1) | MUST NOT | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** of ze's servers, geodns consumes ECS and the forwarder does not; the forwarder answers ECS queries without the option (the forwarder's reply carries a fresh OPT with no option (setReplyOPT in internal/core/dnsserver/recursive.go)), and geodns answers with no OPT record and so with no ECS option (internal/plugins/geodns/server.go), so the prohibition holds by construction, but no test carries this id |
| `RFC7871-7.2.1-3` | A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns reads the incoming ECS ADDRESS without validating its FAMILY and rejects no unknown-FAMILY option; internal/core/dnsserver/client.go:26 takes ecs.Address unconditionally, with no FORMERR path |
| `RFC7871-7.2.1-4` | A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected and a FORMERR response MUST be returned to the sender (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns returns no FORMERR for a wrongly formatted ECS option it consumes; internal/core/dnsserver/client.go:21 has no rejection path and internal/plugins/geodns/server.go:221 never sets FORMERR |
| `RFC7871-7.2.1-5` | An Authoritative Nameserver that implements this protocol and receives an ECS option MUST include an ECS option in its response to indicate that it SHOULD be cached accordingly, regardless of whether the client information was needed to formulate an answer. (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns uses the ECS ADDRESS to tailor answers (internal/core/dnsserver/client.go:21) but includes no ECS option in its response (internal/plugins/geodns/server.go:168), so it does not echo the option |
| `RFC7871-7.2.1-7` | (Note that the requirement in [RFC6891] to reserve space for the OPT record could mean that the Answer section of the response will be truncated and fall back to TCP indicated accordingly.) If an ECS option was not included in a query, one MUST NOT be included in the response even if the server is providing a Tailored Response -- presumably based on the address from which it received the query. (§7.2.1) | MUST NOT | 7.2.1 | **positive:** `unit/verify` [`TestRFC7871_NoECSQueryNoECSResponse`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L72). **negative:** no negative test. **{single-polarity}:** geodns emits no ECS option in any response (internal/plugins/geodns/server.go:168), so the complementary state (an ECS option present when the query carried none) cannot be constructed; the positive case, an ECS-less query that still yields a tailored answer producing an ECS-less response, is pinned in internal/plugins/geodns/rfc7871_server_test.go |
| `RFC7871-7.2.1-8` | FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the response MUST match those in the query. (§7.2.1) | MUST | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns uses ECS but includes no ECS option in its response (geodns answers with no OPT record and so with no ECS option (internal/plugins/geodns/server.go)), so there are no echoed fields to match; the missing echo is the gap at RFC7871-7.2.1-5, and this rule is owed with it |
| `RFC7871-7.2.1-12` | Because it can't be guaranteed that queries for all longer prefix lengths would arrive before one that would be answered by the shorter prefix length, an Authoritative Nameserver MUST NOT overlap prefixes. (§7.2.1) | MUST NOT | 7.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns is the Authoritative Nameserver that uses ECS, but it returns no SCOPE PREFIX-LENGTH (geodns answers with no OPT record and so with no ECS option (internal/plugins/geodns/server.go)), so it publishes no prefix that could overlap; the obligation is owed with RFC7871-7.2.1-5 and no test carries this id |
| `RFC7871-7.3-3` | If FAMILY, SOURCE PREFIX-LENGTH, and SOURCE PREFIX-LENGTH bits of ADDRESS in the response don't match the non-zero fields in the corresponding query, the full response MUST be dropped, as described in Section 11. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds a resolver that sent ECS, and ze's resolvers send none (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go); the stub resolver sets only the EDNS0 DO bit and adds no option (internal/component/resolve/dns/resolver.go)), so they validate no ECS response; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.3-4` | In a response to a query that specified only SOURCE PREFIX-LENGTH for privacy masking, the FAMILY and ADDRESS fields MUST contain the appropriate non-zero information that the Authoritative Nameserver used to generate the answer, so that it can be cached accordingly. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds the Authoritative Nameserver, which geodns is, but geodns answers with no OPT record and so with no ECS option (internal/plugins/geodns/server.go); it is owed with RFC7871-7.2.1-5 and no test carries this id |
| `RFC7871-7.3-6` | If a REFUSED response is received from an Authoritative Nameserver, an ECS-aware resolver MUST retry the query without ECS to distinguish the response from one where the Authoritative Nameserver is not responsible for the name, which is a common convention for the REFUSED status. (§7.3) | MUST | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's forwarder and stub resolver are resolvers but not ECS-aware (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go); the stub resolver sets only the EDNS0 DO bit and adds no option (internal/component/resolve/dns/resolver.go)); ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.3.1-1` | In the cache, all resource records in the Answer section MUST be tied to the network specified in the response. (§7.3.1) | MUST | 7.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's forwarder caches answers but sends no ECS option upstream (upstreamQuery in internal/plugins/dnsresolver/forward.go), and the forwarder's cache keys entries by name, type and class only (cacheKey in internal/plugins/dnsresolver/cache.go), so it ties nothing to a network; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.3.1-3` | Any records from these sections MUST NOT be tied to a network. (§7.3.1) | MUST NOT | 7.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's forwarder caches answers but sends no ECS option upstream (upstreamQuery in internal/plugins/dnsresolver/forward.go), and the forwarder's cache keys entries by name, type and class only (cacheKey in internal/plugins/dnsresolver/cache.go), so it ties nothing to a network; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.3.1-4` | Records that are cached as /0 because of a query's SOURCE PREFIX- LENGTH of 0 MUST be distinguished from those that are cached as /0 because of a response's SCOPE PREFIX-LENGTH of 0. (§7.3.1) | MUST | 7.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's forwarder caches answers but sends no ECS option upstream (upstreamQuery in internal/plugins/dnsresolver/forward.go), and the forwarder's cache keys entries by name, type and class only (cacheKey in internal/plugins/dnsresolver/cache.go), so it ties nothing to a network; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.3.2-1` | Then, the appropriate RRset MUST be chosen based on the longest prefix matching. (§7.3.2) | MUST | 7.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's forwarder is an Intermediate Nameserver with a cache, but the forwarder's cache keys entries by name, type and class only (cacheKey in internal/plugins/dnsresolver/cache.go) and it chooses no RRset by network; geodns's longest-prefix match runs over operator-configured source prefixes, not a cache; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.3.2-4` | If no matching network is found, the Intermediate Nameserver MUST perform resolution as usual. (§7.3.2) | MUST | 7.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's forwarder is an Intermediate Nameserver with a cache, but the forwarder's cache keys entries by name, type and class only (cacheKey in internal/plugins/dnsresolver/cache.go), so it has no network lookup to miss; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.5-1` | Any Intermediate Nameserver that forwards ECS options received from its clients MUST fully implement the caching behavior described in Section 7.3. (§7.5) | MUST | 7.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's forwarder is an Intermediate Nameserver, but it forwards no client ECS option (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go)), so the condition never arises; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-7.5-6` | Note again that a query MUST NOT be refused solely because it provides 0 address bits. (§7.5) | MUST NOT | 7.5 | **positive:** `unit/verify` [`TestRFC7871_ZeroAddressBitsNotRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L108). **negative:** no negative test. **{single-polarity}:** geodns refuses only when disabled and never on ECS content (internal/plugins/geodns/server.go:236), so a query carrying 0 address bits is answered, not refused; the refuse-for-0-bits case is unreachable and internal/plugins/geodns/rfc7871_server_test.go pins the positive |
| `RFC7871-9-1` | Most DNSSEC records SHOULD be scoped at /0, except for the RRSIG records, which MUST be tied to the RRset that they sign in a Tailored Response. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** geodns uses ECS but signs nothing and returns no SCOPE-scoped Tailored Response (internal/plugins/geodns/server.go holds no RRSIG producer), and the forwarder clears DO upstream and tailors nothing, so no RRSIG is tied to a scope; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-11.1-2` | As described in previous sections, this option will be forwarded across all the Recursive Resolvers supporting ECS, which MUST NOT modify it to include the network address of the client. (§11.1) | MUST NOT | 11.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's forwarder forwards no ECS option (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go)), so it modifies none; so the prohibition holds by construction, but no test carries this id |
| `RFC7871-11.2-1` | To counter this, the ECS option in a response packet MUST contain the full FAMILY, ADDRESS, and SOURCE PREFIX-LENGTH fields from the corresponding query. Intermediate Nameservers processing a response MUST verify that these match (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the response half binds geodns, which includes no ECS option in its response (geodns answers with no OPT record and so with no ECS option (internal/plugins/geodns/server.go), the gap at RFC7871-7.2.1-5), and the verification half binds ze's forwarder as an Intermediate Nameserver, which sends no ECS query (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go)); ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| `RFC7871-11.3-4` | o Recursive Resolvers MUST NOT send an ECS option with SOURCE PREFIX-LENGTH providing more bits in ADDRESS than they are willing to cache responses for. (§11.3) | MUST NOT | 11.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the obligation binds a resolver sending ECS, and ze's forwarder, which prepares queries as Section 7.1.3 describes, sends no ECS option (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go)); so the prohibition holds by construction, but no test carries this id |
| `RFC7871-12.1-3` | Probing, if implemented, MUST be repeated periodically, e.g., daily. (§12.1) | MUST | 12.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's resolvers implement no ECS support-probing of authoritative servers (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go)), and the owner declined no ECS feature, so the obligation has no producer and no test |
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
| [`RFC7871-6-1`](#rfc7871-6-1) SCOPE PREFIX-LENGTH, an unsigned octet representing the leftmost number of significant bits of ADDRESS that the response covers. In queries, it MUST be set to 0. (§6) | {gap}, no test | ze sends no ECS option in any query: ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), and the stub resolver sets only the EDNS0 DO bit and adds no option (internal/component/resolve/dns/resolver.go); ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-6-2`](#rfc7871-6-2) o ADDRESS, variable number of octets, contains either an IPv4 or IPv6 address, depending on FAMILY, which MUST be truncated to the number of bits indicated by the SOURCE PREFIX-LENGTH field, padding with 0 bits to pad to the end of the last octet needed. (§6) | {gap}, no test | ze sends no ECS option in any query: ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), and the stub resolver sets only the EDNS0 DO bit and adds no option (internal/component/resolve/dns/resolver.go); ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.1.1-2`](#rfc7871-7.1.1-2) If the triggering query included an ECS option itself, it MUST be examined for its SOURCE PREFIX-LENGTH. (§7.1.1) | {gap}, no test | the obligation binds the Forwarding Resolver, which ze's dnsresolver plugin is, through Section 7.1.3; ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it neither examines nor relays a client's ECS option; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.1.1-3`](#rfc7871-7.1.1-3) The Recursive Resolver's outgoing query MUST then set SOURCE PREFIX-LENGTH to the shorter of the incoming query's SOURCE PREFIX-LENGTH or the server's maximum cacheable prefix length. (§7.1.1) | {gap}, no test | the obligation binds the Forwarding Resolver, which ze's dnsresolver plugin is, through Section 7.1.3; ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it neither examines nor relays a client's ECS option; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.1.1-4`](#rfc7871-7.1.1-4) The total number of octets used MUST only be enough to cover SOURCE PREFIX- LENGTH bits, rather than the full width that would normally be used by addresses in FAMILY. (§7.1.1) | {gap}, no test | the obligation binds the Forwarding Resolver, which ze's dnsresolver plugin is, through Section 7.1.3; ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it neither examines nor relays a client's ECS option; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.1.1-7`](#rfc7871-7.1.1-7) Subsequent queries to refresh the data MUST, if unrestricted by an incoming SOURCE PREFIX-LENGTH, specify the longest SOURCE PREFIX- LENGTH that the Recursive Resolver is willing to cache, even if a previous response indicated that a shorter prefix length was sufficient. (§7.1.1) | {gap}, no test | the obligation binds the Forwarding Resolver, which ze's dnsresolver plugin is, through Section 7.1.3; ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it neither examines nor relays a client's ECS option; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.1.2-2`](#rfc7871-7.1.2-2) An Intermediate Nameserver that receives such a query MUST NOT make queries that include more bits of client address than in the originating query. (§7.1.2) | {gap}, no test | ze's forwarder is an Intermediate Nameserver, and ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it sends no client address bits at all; so the prohibition holds by construction, but no test carries this id |
| [`RFC7871-7.1.2-3`](#rfc7871-7.1.2-3) A SOURCE PREFIX-LENGTH value of 0 means that the Recursive Resolver MUST NOT add the client's address information to its queries. (§7.1.2) | {gap}, no test | the obligation binds ze's forwarder, which acts as a resolver here; ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it adds no client address to any query; so the prohibition holds by construction, but no test carries this id |
| [`RFC7871-7.1.2-5`](#rfc7871-7.1.2-5) A Stub Resolver MUST set SCOPE PREFIX-LENGTH to 0. (§7.1.2) | {gap}, no test | ze holds a Stub Resolver, but the stub resolver sets only the EDNS0 DO bit and adds no option (internal/component/resolve/dns/resolver.go), so it writes no SCOPE PREFIX-LENGTH; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.1.3-1`](#rfc7871-7.1.3-1) A Forwarding Resolver using this option MUST prepare it as described in Section 7.1.1, "Recursive Resolvers". (§7.1.3) | {gap}, no test | ze's dnsresolver plugin is a Forwarding Resolver, but it does not use this option: ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go); ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.1.3-2`](#rfc7871-7.1.3-2) In particular, a Forwarding Resolver that implements this protocol MUST honor SOURCE PREFIX- LENGTH restrictions indicated in the incoming query from its client. (§7.1.3) | {gap}, no test | ze's dnsresolver plugin is a Forwarding Resolver, but ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so it honors no incoming SOURCE PREFIX-LENGTH; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.1.3-3`](#rfc7871-7.1.3-3) If the Forwarding Resolver receives a REFUSED response when it sends a query that includes a non-zero ADDRESS, it MUST retry with no ADDRESS. (§7.1.3) | {gap}, no test | ze's dnsresolver plugin is a Forwarding Resolver, but ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go), so no query of it carries a non-zero ADDRESS that could draw this REFUSED; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.2.1-2`](#rfc7871-7.2.1-2) Such a server MUST NOT include an ECS option within replies to indicate lack of support for it. (§7.2.1) | {gap}, no test | of ze's servers, geodns consumes ECS and the forwarder does not; the forwarder answers ECS queries without the option (the forwarder's reply carries a fresh OPT with no option (setReplyOPT in internal/core/dnsserver/recursive.go)), and geodns answers with no OPT record and so with no ECS option (internal/plugins/geodns/server.go), so the prohibition holds by construction, but no test carries this id |
| [`RFC7871-7.2.1-3`](#rfc7871-7.2.1-3) A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected (§7.2.1) | {gap}, no test | geodns reads the incoming ECS ADDRESS without validating its FAMILY and rejects no unknown-FAMILY option; internal/core/dnsserver/client.go:26 takes ecs.Address unconditionally, with no FORMERR path |
| [`RFC7871-7.2.1-4`](#rfc7871-7.2.1-4) A query with a wrongly formatted option (e.g., an unknown FAMILY) MUST be rejected and a FORMERR response MUST be returned to the sender (§7.2.1) | {gap}, no test | geodns returns no FORMERR for a wrongly formatted ECS option it consumes; internal/core/dnsserver/client.go:21 has no rejection path and internal/plugins/geodns/server.go:221 never sets FORMERR |
| [`RFC7871-7.2.1-5`](#rfc7871-7.2.1-5) An Authoritative Nameserver that implements this protocol and receives an ECS option MUST include an ECS option in its response to indicate that it SHOULD be cached accordingly, regardless of whether the client information was needed to formulate an answer. (§7.2.1) | {gap}, no test | geodns uses the ECS ADDRESS to tailor answers (internal/core/dnsserver/client.go:21) but includes no ECS option in its response (internal/plugins/geodns/server.go:168), so it does not echo the option |
| [`RFC7871-7.2.1-8`](#rfc7871-7.2.1-8) FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the response MUST match those in the query. (§7.2.1) | {gap}, no test | geodns uses ECS but includes no ECS option in its response (geodns answers with no OPT record and so with no ECS option (internal/plugins/geodns/server.go)), so there are no echoed fields to match; the missing echo is the gap at RFC7871-7.2.1-5, and this rule is owed with it |
| [`RFC7871-7.2.1-12`](#rfc7871-7.2.1-12) Because it can't be guaranteed that queries for all longer prefix lengths would arrive before one that would be answered by the shorter prefix length, an Authoritative Nameserver MUST NOT overlap prefixes. (§7.2.1) | {gap}, no test | geodns is the Authoritative Nameserver that uses ECS, but it returns no SCOPE PREFIX-LENGTH (geodns answers with no OPT record and so with no ECS option (internal/plugins/geodns/server.go)), so it publishes no prefix that could overlap; the obligation is owed with RFC7871-7.2.1-5 and no test carries this id |
| [`RFC7871-7.3-3`](#rfc7871-7.3-3) If FAMILY, SOURCE PREFIX-LENGTH, and SOURCE PREFIX-LENGTH bits of ADDRESS in the response don't match the non-zero fields in the corresponding query, the full response MUST be dropped, as described in Section 11. (§7.3) | {gap}, no test | the obligation binds a resolver that sent ECS, and ze's resolvers send none (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go); the stub resolver sets only the EDNS0 DO bit and adds no option (internal/component/resolve/dns/resolver.go)), so they validate no ECS response; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.3-4`](#rfc7871-7.3-4) In a response to a query that specified only SOURCE PREFIX-LENGTH for privacy masking, the FAMILY and ADDRESS fields MUST contain the appropriate non-zero information that the Authoritative Nameserver used to generate the answer, so that it can be cached accordingly. (§7.3) | {gap}, no test | the obligation binds the Authoritative Nameserver, which geodns is, but geodns answers with no OPT record and so with no ECS option (internal/plugins/geodns/server.go); it is owed with RFC7871-7.2.1-5 and no test carries this id |
| [`RFC7871-7.3-6`](#rfc7871-7.3-6) If a REFUSED response is received from an Authoritative Nameserver, an ECS-aware resolver MUST retry the query without ECS to distinguish the response from one where the Authoritative Nameserver is not responsible for the name, which is a common convention for the REFUSED status. (§7.3) | {gap}, no test | ze's forwarder and stub resolver are resolvers but not ECS-aware (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go); the stub resolver sets only the EDNS0 DO bit and adds no option (internal/component/resolve/dns/resolver.go)); ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.3.1-1`](#rfc7871-7.3.1-1) In the cache, all resource records in the Answer section MUST be tied to the network specified in the response. (§7.3.1) | {gap}, no test | ze's forwarder caches answers but sends no ECS option upstream (upstreamQuery in internal/plugins/dnsresolver/forward.go), and the forwarder's cache keys entries by name, type and class only (cacheKey in internal/plugins/dnsresolver/cache.go), so it ties nothing to a network; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.3.1-3`](#rfc7871-7.3.1-3) Any records from these sections MUST NOT be tied to a network. (§7.3.1) | {gap}, no test | ze's forwarder caches answers but sends no ECS option upstream (upstreamQuery in internal/plugins/dnsresolver/forward.go), and the forwarder's cache keys entries by name, type and class only (cacheKey in internal/plugins/dnsresolver/cache.go), so it ties nothing to a network; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.3.1-4`](#rfc7871-7.3.1-4) Records that are cached as /0 because of a query's SOURCE PREFIX- LENGTH of 0 MUST be distinguished from those that are cached as /0 because of a response's SCOPE PREFIX-LENGTH of 0. (§7.3.1) | {gap}, no test | ze's forwarder caches answers but sends no ECS option upstream (upstreamQuery in internal/plugins/dnsresolver/forward.go), and the forwarder's cache keys entries by name, type and class only (cacheKey in internal/plugins/dnsresolver/cache.go), so it ties nothing to a network; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.3.2-1`](#rfc7871-7.3.2-1) Then, the appropriate RRset MUST be chosen based on the longest prefix matching. (§7.3.2) | {gap}, no test | ze's forwarder is an Intermediate Nameserver with a cache, but the forwarder's cache keys entries by name, type and class only (cacheKey in internal/plugins/dnsresolver/cache.go) and it chooses no RRset by network; geodns's longest-prefix match runs over operator-configured source prefixes, not a cache; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.3.2-4`](#rfc7871-7.3.2-4) If no matching network is found, the Intermediate Nameserver MUST perform resolution as usual. (§7.3.2) | {gap}, no test | ze's forwarder is an Intermediate Nameserver with a cache, but the forwarder's cache keys entries by name, type and class only (cacheKey in internal/plugins/dnsresolver/cache.go), so it has no network lookup to miss; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-7.5-1`](#rfc7871-7.5-1) Any Intermediate Nameserver that forwards ECS options received from its clients MUST fully implement the caching behavior described in Section 7.3. (§7.5) | {gap}, no test | ze's forwarder is an Intermediate Nameserver, but it forwards no client ECS option (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go)), so the condition never arises; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-9-1`](#rfc7871-9-1) Most DNSSEC records SHOULD be scoped at /0, except for the RRSIG records, which MUST be tied to the RRset that they sign in a Tailored Response. (§9) | {gap}, no test | geodns uses ECS but signs nothing and returns no SCOPE-scoped Tailored Response (internal/plugins/geodns/server.go holds no RRSIG producer), and the forwarder clears DO upstream and tailors nothing, so no RRSIG is tied to a scope; ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-11.1-2`](#rfc7871-11.1-2) As described in previous sections, this option will be forwarded across all the Recursive Resolvers supporting ECS, which MUST NOT modify it to include the network address of the client. (§11.1) | {gap}, no test | ze's forwarder forwards no ECS option (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go)), so it modifies none; so the prohibition holds by construction, but no test carries this id |
| [`RFC7871-11.2-1`](#rfc7871-11.2-1) To counter this, the ECS option in a response packet MUST contain the full FAMILY, ADDRESS, and SOURCE PREFIX-LENGTH fields from the corresponding query. Intermediate Nameservers processing a response MUST verify that these match (§11.2) | {gap}, no test | the response half binds geodns, which includes no ECS option in its response (geodns answers with no OPT record and so with no ECS option (internal/plugins/geodns/server.go), the gap at RFC7871-7.2.1-5), and the verification half binds ze's forwarder as an Intermediate Nameserver, which sends no ECS query (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go)); ze's resolvers implement no ECS and the owner declined no ECS feature, so the obligation has no producer and no test |
| [`RFC7871-11.3-4`](#rfc7871-11.3-4) o Recursive Resolvers MUST NOT send an ECS option with SOURCE PREFIX-LENGTH providing more bits in ADDRESS than they are willing to cache responses for. (§11.3) | {gap}, no test | the obligation binds a resolver sending ECS, and ze's forwarder, which prepares queries as Section 7.1.3 describes, sends no ECS option (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go)); so the prohibition holds by construction, but no test carries this id |
| [`RFC7871-12.1-3`](#rfc7871-12.1-3) Probing, if implemented, MUST be repeated periodically, e.g., daily. (§12.1) | {gap}, no test | ze's resolvers implement no ECS support-probing of authoritative servers (ze's caching forwarder builds each upstream query from the question alone, with an OPT that carries no option (upstreamQuery in internal/plugins/dnsresolver/forward.go)), and the owner declined no ECS feature, so the obligation has no producer and no test |
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

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-30 (independent judge) after the row's annotation moved from {not-applicable} to {gap}. RFC 7871 Section 7.1.2 reads 'A SOURCE PREFIX-LENGTH value of 0 means that the Recursive Resolver MUST NOT add the client's address information to its queries.' and Section 7.1.3 reads 'A Forwarding Resolver using this option MUST prepare it as described in Section 7.1.1, "Recursive Resolvers".' The earlier not-applicable verdict said Ze sends no query on behalf of a client; that is false: the dnsresolver plugin is a caching Forwarding Resolver that sends upstream queries for its clients. The prohibition holds by construction, because internal/plugins/dnsresolver/forward.go::upstreamQuery builds each upstream query from the question alone, with an OPT that carries no option, so no client address reaches an outgoing query whatever SOURCE PREFIX-LENGTH the client sent. No test carries this requirement and Ze implements no ECS on the resolver side, so the row is a gap and the verdict is unimplemented until a test proves the prohibition on the forwarder's outgoing query.

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30: the unit changed only by dropping the retired RFC7871-7.2.2-1 tag and its duplicate assertion; the 7.2.1-7 assertion is unchanged. TestRFC7871_NoECSQueryNoECSResponse sends an ECS-less query whose packet source selects a tailored A (asserted 10.0.0.5), then asserts the response carries no EDNS0_SUBNET option; adding an ECS option to any geodns response turns it red. Negative polarity is unconstructible (geodns emits no ECS), as the single-polarity marker states.

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
| positive | [`TestRFC7871_ZeroAddressBitsNotRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc7871_server_test.go#L108) | unit/verify | unproven |

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
| Reviewer | independent judge, spec-rfc-verdict-fix-services (session 869df689) |
| Signed off | 2026-09-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc7871.txt |
| Source fingerprint | de63f477f0265ea1 |
| Record | rfc/extraction/rfc7871.json |
| Mapped sentences | 35 |
| Declined as scope | 5 |
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
| `7.2.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | section 7.2.2 binds an Intermediate Nameserver ("When an Intermediate Nameserver uses ECS"), which Ze does not implement: geodns is an Authoritative Nameserver and forwards no query. The authoritative obligation is row RFC7871-7.2.1-7. Row RFC7871-7.2.2-1 is retired (rfc/corrections/rfc7871.md) | Because a client that did not use an ECS option might not be able to understand it, the server MUST NOT provide one in its response. |
| `7.2.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | section 7.2.2 binds an Intermediate Nameserver ("When an Intermediate Nameserver uses ECS"; section 4: "Any nameserver in between the Stub Resolver and the Authoritative Nameserver"), which Ze does not implement: geodns is an Authoritative Nameserver and forwards no query. The authoritative echo obligation is the {gap} row RFC7871-7.2.1-5. Row RFC7871-7.2.2-2 is retired (rfc/corrections/rfc7871.md) | If the client query did include the option, the server MUST include one in its response, especially as it could be talking to a Forwarding Resolver, which would need the information for its own caching. |
| `7.5:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates for any Intermediate Nameserver the obligation section 7.1.2 states for the Recursive Resolver, which this walk maps at 7.1.2:2: 'A SOURCE PREFIX-LENGTH value of 0 means that the Recursive Resolver MUST NOT add the client's address information to its queries.' The sentence cites that section itself ('see Section 7.1.2'), and RFC7871-7.1.2-3 already carries both readings. | If an Intermediate Nameserver receives a query with SOURCE PREFIX- LENGTH set to 0, it MUST NOT include client address information in queries made to resolve that client's request (see Section 7.1.2). |
| `11.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates as an anti-spoofing measure the echo requirement section 7.2.1 states and this walk maps at 7.2.1:5: 'FAMILY, SOURCE PREFIX-LENGTH, and ADDRESS in the response MUST match those in the query.' RFC7871-7.2.1-8 already records that section 11.2 restates it. | To counter this, the ECS option in a response packet MUST contain the full FAMILY, ADDRESS, and SOURCE PREFIX-LENGTH fields from the corresponding query. |
| `11.2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Explains the LEVEL the sentence before it uses rather than stating an obligation: 'The requirement to discard is categorized as "SHOULD" instead of "MUST" because it stands in opposition to the instruction in Section 7.3'. The capitalised keywords are quoted words, the subject of the sentence. The obligations themselves are RFC7871-11.2-1 (MUST verify) and RFC7871-11.2-2 (SHOULD discard), both carried by the checklist. | The requirement to discard is categorized as "SHOULD" instead of "MUST" because it stands in opposition to the instruction in Section 7.3, which states that a response lacking an ECS option should be treated as though it had one of SCOPE PREFIX-LENGTH of 0. |

## Superseded

No document obsoletes RFC 7871, so its obligations are stated where they were written.
