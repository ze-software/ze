# RFC 8106 - IPv6 Router Advertisement Options for DNS Configuration

RA DNS options (RDNSS, DNSSL). Every requirement this repository extracted from RFC 8106, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 11 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0 | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 24 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 11 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 11 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 11 | of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 11 gated MUSTs: they add up to 11. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | RA DNS options (RDNSS, DNSSL) |
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

**Area:** RA DNS options (RDNSS, DNSSL)

**Proof:** 0 of 11 MUST proven

**Evidence:** -

**Unmet:** RFC8106-1.2-1, RFC8106-1.2-2, RFC8106-5.1-1, RFC8106-5.1-2, RFC8106-5.2-1, RFC8106-5.2-2, RFC8106-5.2-3, RFC8106-5.2-4, RFC8106-5.3.1-1, RFC8106-5.3.1-2, RFC8106-5.3.1-3


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

## Figures

- **MUST bar**: Ze correctness gap 11
- **Overview bar**: SHOULD not decided 13, Ze correctness gap 11

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 11 |
| MUST rows kept (the MUST bar) | 11 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 11 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 13 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 13 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc8106.json](../../../data/rfc/rfc8106.json).

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
