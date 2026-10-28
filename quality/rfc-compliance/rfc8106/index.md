# RFC 8106 - IPv6 Router Advertisement Options for DNS Configuration

Partial. Every requirement this repository extracted from RFC 8106, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 11 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 24 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 11 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 11 of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 11 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 24 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 11 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8106.md` |
| Requirement shard | `rfc/requirements/rfc8106.md` |
| RFC text | `rfc/full/rfc8106.txt` |

## Enrolment

Enrolled: IPv6 Router Advertisement Options for DNS Configuration: the RDNSS and DNSSL Neighbor Discovery options. Every MUST-level row is classified; each is a {gap}, because ze sends RDNSS as a router but sends no DNSSL and, as a host, processes no received RDNSS or DNSSL option.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

Router side, RDNSS only: ze's Router Advertisements carry one RDNSS option (type 25, Length 1+2n, up to 127 addresses, one shared Lifetime) built by `BuildRA` in [`internal/core/ndp/ra.go`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/ra.go).

**What the ledger says remains**

No DNSSL option is sent ([`RFC8106-5.2-2`](#rfc8106-5.2-2), [`RFC8106-5.2-3`](#rfc8106-5.2-3), [`RFC8106-5.2-4`](#rfc8106-5.2-4)). Ze as a host processes no RDNSS or DNSSL option from a received RA: no extraction, no ordered list, no Lifetime expiry, no length validation, no SEND preference ([`RFC8106-1.2-1`](#rfc8106-1.2-1), [`RFC8106-1.2-2`](#rfc8106-1.2-2), [`RFC8106-5.1-1`](#rfc8106-5.1-1), [`RFC8106-5.1-2`](#rfc8106-5.1-2), [`RFC8106-5.2-1`](#rfc8106-5.2-1), [`RFC8106-5.3.1-1`](#rfc8106-5.3.1-1), [`RFC8106-5.3.1-2`](#rfc8106-5.3.1-2), [`RFC8106-5.3.1-3`](#rfc8106-5.3.1-3)).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (11):** [`RFC8106-1.2-1`](#rfc8106-1.2-1), [`RFC8106-1.2-2`](#rfc8106-1.2-2), [`RFC8106-5.1-1`](#rfc8106-5.1-1), [`RFC8106-5.1-2`](#rfc8106-5.1-2), [`RFC8106-5.2-1`](#rfc8106-5.2-1), [`RFC8106-5.2-2`](#rfc8106-5.2-2), [`RFC8106-5.2-3`](#rfc8106-5.2-3), [`RFC8106-5.2-4`](#rfc8106-5.2-4), [`RFC8106-5.3.1-1`](#rfc8106-5.3.1-1), [`RFC8106-5.3.1-2`](#rfc8106-5.3.1-2), [`RFC8106-5.3.1-3`](#rfc8106-5.3.1-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8106-1.2-1` | Hosts conforming to this specification MUST extract DNS information from RA messages, unless static DNS configuration has been specified by the user. (§1.2) | MUST | 1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze reads no RDNSS or DNSSL option from a received RA; its resolver takes upstream servers from configuration only, and internal/core/ndp/ra.go holds a builder and no parser |
| `RFC8106-1.2-2` | If there is DNS information available from multiple RAs and/or from DHCP, the host MUST maintain an ordered list of this information as specified in Section 5.3.1. (§1.2) | MUST | 1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze keeps no DNS Server List learned from RAs; host-side processing is absent |
| `RFC8106-5.1-1` | A value of zero means that the RDNSS addresses MUST no longer be used. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze as a host learns no RDNSS address, so no Lifetime expiry exists; host-side processing is absent |
| `RFC8106-5.1-2` | When a resolver sends a DNS query message to an RDNSS identified by a link-local address, it MUST use the corresponding link. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze as a host learns no RDNSS address from an RA; host-side processing is absent |
| `RFC8106-5.2-1` | A value of zero means that the DNSSL domain names MUST no longer be used. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze as a host processes no DNSSL option; host-side processing is absent |
| `RFC8106-5.2-2` | One or more domain names of the DNSSL that MUST be encoded as described in Section 3.1 of [RFC1035]. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA writes Source Link-Layer Address, Prefix Information and RDNSS only |
| `RFC8106-5.2-3` | Note that for the simple decoding, the domain names MUST NOT be encoded in the compressed form described in Section 4.1.4 of [RFC1035]. (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA |
| `RFC8106-5.2-4` | Because the size of this field MUST be a multiple of 8 octets, for the minimum multiple including the domain name representations, the remaining octets other than the encoding parts of the domain name representations MUST be padded with zeros. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA |
| `RFC8106-5.3.1-1` | Otherwise, the host MUST discard the options. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze as a host neither validates nor stores a received RDNSS or DNSSL option; host-side processing is absent |
| `RFC8106-5.3.1-2` | On the other hand, for DNS options announced by RAs, if some RAs use the Secure Neighbor Discovery (SEND) protocol [RFC3971] for RA security, they MUST be preferred over those that do not use SEND. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze as a host consumes no DNS option from RAs, SEND or otherwise; host-side processing is absent |
| `RFC8106-5.3.1-3` | Also, DNS options announced by RAs via SEND MUST be preferred over those announced by unauthenticated DHCP [RFC3118]. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze as a host consumes no DNS option from RAs; host-side processing is absent |
| `RFC8106-5.1-3` | The value of Lifetime SHOULD by default be at least 3 * MaxRtrAdvInterval, where MaxRtrAdvInterval is the maximum RA interval as defined in [RFC4861]. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.1-4` | Such link-local addresses SHOULD be registered in the Resolver Repository along with the corresponding link zone indices of the links that receive the RDNSS option(s) for them. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.2-5` | That is, Lifetime SHOULD by default be at least 3 * MaxRtrAdvInterval. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.3.1-4` | If the DNS options are valid, the host SHOULD copy the values of the options into the DNS Repository and the Resolver Repository in order. (§5.3.1) | SHOULD | 5.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.3.1-5` | In the case where the DNS information of RDNSS and DNSSL can be obtained from multiple sources, such as RAs and DHCP, the IPv6 host SHOULD keep some DNS options from all sources. (§5.3.1) | SHOULD | 5.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.3.1-6` | However, in the case of multiple sources, the ability to store a total of at least three RDNSS addresses (or DNSSL domain names) from the multiple sources is RECOMMENDED. (§5.3.1) | RECOMMENDED | 5.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.3.1-7` | The DNS options from RAs and DHCP SHOULD be stored in the DNS Repository and Resolver Repository so that information from DHCP appears there first and therefore takes precedence. (§5.3.1) | SHOULD | 5.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-6.1-1` | For DNS repository management, the following two data structures SHOULD be synchronized with the Resolver Repository: (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-7.2-1` | The corresponding technique for network devices is RECOMMENDED to block rogue RA messages that include the RDNSS and DNSSL options from unauthorized nodes [RFC6104] [RFC6105]. (§7.2) | RECOMMENDED | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.1-5` | The maximum time in seconds (relative to the time the packet is received) over which these RDNSS addresses MAY be used for name resolution. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.1-6` | Note: The addresses for RDNSSes in the RDNSS option MAY be link-local addresses. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.1-7` | The link-local addresses MAY be represented in the Resolver Repository with their link zone indices in the textual format for scoped addresses as described in [RFC4007]. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.2-6` | The maximum time in seconds (relative to the time the packet is received) over which these DNSSL domain names MAY be used for name resolution. (§5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8106-1.2-1`](#rfc8106-1.2-1) Hosts conforming to this specification MUST extract DNS information from RA messages, unless static DNS configuration has been specified by the user. (§1.2) | {gap}, no test | ze reads no RDNSS or DNSSL option from a received RA; its resolver takes upstream servers from configuration only, and internal/core/ndp/ra.go holds a builder and no parser |
| [`RFC8106-1.2-2`](#rfc8106-1.2-2) If there is DNS information available from multiple RAs and/or from DHCP, the host MUST maintain an ordered list of this information as specified in Section 5.3.1. (§1.2) | {gap}, no test | ze keeps no DNS Server List learned from RAs; host-side processing is absent |
| [`RFC8106-5.1-1`](#rfc8106-5.1-1) A value of zero means that the RDNSS addresses MUST no longer be used. (§5.1) | {gap}, no test | ze as a host learns no RDNSS address, so no Lifetime expiry exists; host-side processing is absent |
| [`RFC8106-5.1-2`](#rfc8106-5.1-2) When a resolver sends a DNS query message to an RDNSS identified by a link-local address, it MUST use the corresponding link. (§5.1) | {gap}, no test | ze as a host learns no RDNSS address from an RA; host-side processing is absent |
| [`RFC8106-5.2-1`](#rfc8106-5.2-1) A value of zero means that the DNSSL domain names MUST no longer be used. (§5.2) | {gap}, no test | ze as a host processes no DNSSL option; host-side processing is absent |
| [`RFC8106-5.2-2`](#rfc8106-5.2-2) One or more domain names of the DNSSL that MUST be encoded as described in Section 3.1 of [RFC1035]. (§5.2) | {gap}, no test | ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA writes Source Link-Layer Address, Prefix Information and RDNSS only |
| [`RFC8106-5.2-3`](#rfc8106-5.2-3) Note that for the simple decoding, the domain names MUST NOT be encoded in the compressed form described in Section 4.1.4 of [RFC1035]. (§5.2) | {gap}, no test | ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA |
| [`RFC8106-5.2-4`](#rfc8106-5.2-4) Because the size of this field MUST be a multiple of 8 octets, for the minimum multiple including the domain name representations, the remaining octets other than the encoding parts of the domain name representations MUST be padded with zeros. (§5.2) | {gap}, no test | ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA |
| [`RFC8106-5.3.1-1`](#rfc8106-5.3.1-1) Otherwise, the host MUST discard the options. (§5.3.1) | {gap}, no test | ze as a host neither validates nor stores a received RDNSS or DNSSL option; host-side processing is absent |
| [`RFC8106-5.3.1-2`](#rfc8106-5.3.1-2) On the other hand, for DNS options announced by RAs, if some RAs use the Secure Neighbor Discovery (SEND) protocol [RFC3971] for RA security, they MUST be preferred over those that do not use SEND. (§5.3.1) | {gap}, no test | ze as a host consumes no DNS option from RAs, SEND or otherwise; host-side processing is absent |
| [`RFC8106-5.3.1-3`](#rfc8106-5.3.1-3) Also, DNS options announced by RAs via SEND MUST be preferred over those announced by unauthenticated DHCP [RFC3118]. (§5.3.1) | {gap}, no test | ze as a host consumes no DNS option from RAs; host-side processing is absent |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8106-1.2-1`](#rfc8106-1.2-1)

Hosts conforming to this specification MUST extract DNS information from RA messages, unless static DNS configuration has been specified by the user. (§1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8106-1.2-1, so no unit is bound to it.

### [`RFC8106-1.2-2`](#rfc8106-1.2-2)

If there is DNS information available from multiple RAs and/or from DHCP, the host MUST maintain an ordered list of this information as specified in Section 5.3.1. (§1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8106-1.2-2, so no unit is bound to it.

### [`RFC8106-5.1-1`](#rfc8106-5.1-1)

A value of zero means that the RDNSS addresses MUST no longer be used. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8106-5.1-1, so no unit is bound to it.

### [`RFC8106-5.1-2`](#rfc8106-5.1-2)

When a resolver sends a DNS query message to an RDNSS identified by a link-local address, it MUST use the corresponding link. (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8106-5.1-2, so no unit is bound to it.

### [`RFC8106-5.2-1`](#rfc8106-5.2-1)

A value of zero means that the DNSSL domain names MUST no longer be used. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8106-5.2-1, so no unit is bound to it.

### [`RFC8106-5.2-2`](#rfc8106-5.2-2)

One or more domain names of the DNSSL that MUST be encoded as described in Section 3.1 of [RFC1035]. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8106-5.2-2, so no unit is bound to it.

### [`RFC8106-5.2-3`](#rfc8106-5.2-3)

Note that for the simple decoding, the domain names MUST NOT be encoded in the compressed form described in Section 4.1.4 of [RFC1035]. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8106-5.2-3, so no unit is bound to it.

### [`RFC8106-5.2-4`](#rfc8106-5.2-4)

Because the size of this field MUST be a multiple of 8 octets, for the minimum multiple including the domain name representations, the remaining octets other than the encoding parts of the domain name representations MUST be padded with zeros. (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8106-5.2-4, so no unit is bound to it.

### [`RFC8106-5.3.1-1`](#rfc8106-5.3.1-1)

Otherwise, the host MUST discard the options. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8106-5.3.1-1, so no unit is bound to it.

### [`RFC8106-5.3.1-2`](#rfc8106-5.3.1-2)

On the other hand, for DNS options announced by RAs, if some RAs use the Secure Neighbor Discovery (SEND) protocol [RFC3971] for RA security, they MUST be preferred over those that do not use SEND. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8106-5.3.1-2, so no unit is bound to it.

### [`RFC8106-5.3.1-3`](#rfc8106-5.3.1-3)

Also, DNS options announced by RAs via SEND MUST be preferred over those announced by unauthenticated DHCP [RFC3118]. (§5.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8106-5.3.1-3, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc8106.txt |
| Source fingerprint | d0fb19f96b955de6 |
| Record | rfc/extraction/rfc8106.json |
| Mapped sentences | 11 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 2 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 2 | walked | not stated |
| `5.2` | not stated | 4 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.3.1` | not stated | 3 | walked | not stated |
| `5.3.2` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

The walk over RFC 8106 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 8106, so its obligations are stated where they were written.
