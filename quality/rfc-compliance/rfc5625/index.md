# RFC 5625 - DNS Proxy Implementation Guidelines

DNS proxy guidelines. Every requirement listed for RFC 5625, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 12 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 8 | of 12 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 12 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 12 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 12 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 16 | 16 of 16 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 12 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 12 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 12 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 12 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 12 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 12 | of 27 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 12 mandatory requirements: they add up to 12. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | DNS proxy guidelines |
| Automatic checks | Included |
| Requirements | 27 |
| Mandatory requirements (MUST level) | 12 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tests linked to requirements | 16 |
| Recorded audit verdicts | 0 |
| Fault test records | 16 |
| Summary | `rfc/short/rfc5625.md` |
| Requirement table | `rfc/requirements/rfc5625.md` |
| RFC text | `rfc/full/rfc5625.txt` |

## Automatic checks

Included in automatic checks: Summary written 2026-11-08 for spec-home-cpe-23 (owner decision D-2: RFC 5625 is followed and listed for fttx-edge). Walked forward and reverse in rfc/extraction/rfc5625.json on 2026-11-08: twelve MUST-level sites map the twelve gated rows, two are excluded (a description of RFC 1035, a quotation of RFC 3597)

## Published support status

**Area:** DNS proxy guidelines

**Test verification:** 12 of 12 MUST proven

**Evidence:** unit 12

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 8 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **12** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (4):** [`RFC5625-4.4-1`](#rfc5625-4.4-1), [`RFC5625-4.4-2`](#rfc5625-4.4-2), [`RFC5625-4.4-3`](#rfc5625-4.4-3), [`RFC5625-5.2-1`](#rfc5625-5.2-1)

**Annotated (including scoped evidence) (8):** [`RFC5625-4.1-1`](#rfc5625-4.1-1), [`RFC5625-4.2-1`](#rfc5625-4.2-1), [`RFC5625-4.3-1`](#rfc5625-4.3-1), [`RFC5625-4.3-2`](#rfc5625-4.3-2), [`RFC5625-4.4.1-1`](#rfc5625-4.4.1-1), [`RFC5625-4.4.2-1`](#rfc5625-4.4.2-1), [`RFC5625-4.5-1`](#rfc5625-4.5-1), [`RFC5625-4.5-2`](#rfc5625-4.5-2)

## Figures

- **MUST bar**: Ze verified 12
- **Overview bar**: Ze MUST verified 12, Not assessed 15

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 12 |
| MUST rows kept (the MUST bar) | 12 |
| Ze verified | 12 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 15 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 15 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5625.json](../../../data/rfc/rfc5625.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5625-4.1-1` | Therefore, proxies MUST ignore any unknown DNS flags and proxy those packets as usual. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestRFC5625ResolverUnknownFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L30). **negative:** no negative test. **{single-polarity}:** the obligation is to proxy a packet whatever its flags, so no flag value exists that a conformant proxy refuses; a refusal is the violation the positive test catches |
| `RFC5625-4.2-1` | Proxies MUST forward packets regardless of the presence or absence of compressed labels therein. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC5625ResolverCompressedReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L92). **negative:** no negative test. **{single-polarity}:** compressed and uncompressed packets are both forwarded, so no input exists that a conformant proxy refuses |
| `RFC5625-4.3-1` | All requests and responses MUST be proxied regardless of the values of the QTYPE and QCLASS fields. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC5625ResolverUnknownTypeAndClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L54). **negative:** no negative test. **{single-polarity}:** every QTYPE and QCLASS is proxied, so no question exists that a conformant proxy refuses |
| `RFC5625-4.3-2` | Similarly, all responses MUST be proxied regardless of the values of the TYPE and CLASS fields of any Resource Record therein. (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestRFC5625ResolverUnknownTypeAndClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L60). **negative:** no negative test. **{single-polarity}:** every TYPE and CLASS in a response is proxied, so no record exists that a conformant proxy withholds |
| `RFC5625-4.4-1` | Where the required portions of a response would not fit inside that limit, the DNS server MUST set the "TrunCation" (TC) bit in the DNS response header to indicate that truncation has occurred. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC5625ResolverTruncationSetsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L155). **negative:** `unit/verify` [`TestRFC5625ResolverTruncationSetsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L167) |
| `RFC5625-4.4-2` | If a proxy must unilaterally truncate a response, then the proxy MUST set the TC bit. (§4.4) | MUST | 4.4 | **positive:** `unit/verify` [`TestRFC5625ResolverTruncationSetsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L159). **negative:** `unit/verify` [`TestRFC5625ResolverTruncationSetsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L169) |
| `RFC5625-4.4-3` | Similarly, proxies MUST NOT remove the TC bit from responses. (§4.4) | MUST NOT | 4.4 | **positive:** `unit/verify` [`TestRFC5625ResolverNeverDropsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L198). **negative:** `unit/verify` [`TestRFC5625ResolverNeverDropsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L208) |
| `RFC5625-4.4.1-1` | DNS proxies MUST therefore be prepared to receive and forward queries over TCP. (§4.4.1) | MUST | 4.4.1 | **positive:** `unit/verify` [`TestRFC5625ProxyTCPAndOPT`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L112). **negative:** no negative test. **{single-polarity}:** every TCP query is received and forwarded, so no TCP query exists that a conformant proxy refuses |
| `RFC5625-4.4.2-1` | As per Section 4.1, proxies MUST NOT refuse to proxy such packets. (§4.4.2) | MUST NOT | 4.4.2 | **positive:** `unit/verify` [`TestRFC5625ProxyTCPAndOPT`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L122). **negative:** no negative test. **{single-polarity}:** the obligation is never to refuse a packet carrying an OPT RR, so its only test is that one is answered |
| `RFC5625-4.5-1` | DNS proxies MUST implement Section 4.7 of [RFC2845] and either forward packets unchanged (as recommended above) or fully implement TSIG. (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestRFC5625SignedQueryForwardedUnchanged`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_passthrough_test.go#L145). **negative:** no negative test. **{single-polarity}:** the obligation is to forward a signed query unchanged, so any change to it is the violation and no signed query exists that a conformant proxy may alter |
| `RFC5625-4.5-2` | As per Section 4.3, DNS proxies MUST be capable of proxying packets containing TKEY [RFC2930] Resource Records. (§4.5) | MUST | 4.5 | **positive:** `unit/verify` [`TestRFC5625TKEYQueryProxied`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_passthrough_test.go#L215). **negative:** no negative test. **{single-polarity}:** every packet carrying a TKEY RR is proxied, so no such packet exists that a conformant proxy refuses |
| `RFC5625-5.2-1` | this option MUST NOT be sent to clients when no value is configured. (§5.2) | MUST NOT | 5.2 | **positive:** `unit/verify` [`TestOption15AbsentWhenUnconfigured`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L206). **negative:** `unit/verify` [`TestOption15AbsentWhenUnconfigured`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L217) |
| `RFC5625-3-1` | It is RECOMMENDED that proxies should be as transparent as possible, such that any "hop-by-hop" mechanisms or newly introduced protocol extensions operate as if the proxy were not there. (§3) | RECOMMENDED | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-3-2` | Except when required to enforce an active security or network policy (such as maintaining a pre-authentication "walled garden"), end-users SHOULD be able to send their DNS queries to specified upstream resolvers, thereby bypassing the proxy altogether. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-3-3` | In this case, the gateway SHOULD NOT modify the DNS request or response packets in any way. (§3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-4.4-4` | Since UDP packets larger than 512 octets are now expected in normal operation, proxies SHOULD NOT truncate UDP packets that exceed that size. (§4.4) | SHOULD NOT | 4.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-4.4.1-2` | Therefore, whenever a proxy receives a request over TCP, the proxy SHOULD forward the query over TCP (§4.4.1) | SHOULD | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-4.4.1-3` | and SHOULD NOT attempt the same query over UDP first. (§4.4.1) | SHOULD NOT | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-4.4.3-1` | Therefore (irrespective of which of the above methods is in use), proxies SHOULD be capable of forwarding UDP packets up to a payload size of at least 4096 octets. (§4.4.3) | SHOULD | 4.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-5.1-1` | As per Section 3, end-users SHOULD be able to send their DNS queries directly to specified upstream resolvers, ideally without hard-coding those settings in their stub resolver. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-5.1-2` | It is therefore RECOMMENDED that gateways SHOULD support device- administrator configuration of values for the "Domain Name Server" DHCP option. (§5.1) | RECOMMENDED | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-5.2-2` | Since no standard exists for a "local" scoped domain name suffix, it is RECOMMENDED that the default value for this option SHOULD be empty (§5.2) | RECOMMENDED | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-6.1-1` | It is strongly RECOMMENDED that DNS proxies follow the relevant recommendations in [RFC5452], particularly those in Section 9.2 relating to randomisation of Query IDs and source ports. (§6.1) | RECOMMENDED | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-6.1-2` | If a DNS proxy is running on a broadband gateway with NAT that is compliant with [RFC4787], then it SHOULD also follow the recommendations in Section 10 of [RFC5452] concerning how long DNS state is kept. (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-6.2-1` | The DNS proxy in a gateway SHOULD NOT, by default, be accessible from the WAN interfaces of the device. (§6.2) | SHOULD NOT | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-6.3-2` | In these circumstances, proxies SHOULD synthesise a suitable DNS error response to the client (i.e., SERVFAIL) instead of dropping the packet completely. (§6.3) | SHOULD | 6.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5625-6.3-1` | Examples of malformed packets that MAY be dropped include: (§6.3) | MAY | 6.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 5625 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5625-4.1-1`](#rfc5625-4.1-1)

Therefore, proxies MUST ignore any unknown DNS flags and proxy those packets as usual. (§4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC5625ResolverUnknownFlags`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L30) | unit/verify | mutant, verified |

### [`RFC5625-4.2-1`](#rfc5625-4.2-1)

Proxies MUST forward packets regardless of the presence or absence of compressed labels therein. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC5625ResolverCompressedReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L92) | unit/verify | mutant, verified |

### [`RFC5625-4.3-1`](#rfc5625-4.3-1)

All requests and responses MUST be proxied regardless of the values of the QTYPE and QCLASS fields. (§4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC5625ResolverUnknownTypeAndClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L54) | unit/verify | mutant, verified |

### [`RFC5625-4.3-2`](#rfc5625-4.3-2)

Similarly, all responses MUST be proxied regardless of the values of the TYPE and CLASS fields of any Resource Record therein. (§4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC5625ResolverUnknownTypeAndClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L60) | unit/verify | mutant, verified |

### [`RFC5625-4.4-1`](#rfc5625-4.4-1)

Where the required portions of a response would not fit inside that limit, the DNS server MUST set the "TrunCation" (TC) bit in the DNS response header to indicate that truncation has occurred. (§4.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5625ResolverTruncationSetsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L167) | unit/verify | mutant, verified |
| positive | [`TestRFC5625ResolverTruncationSetsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L155) | unit/verify | mutant, verified |

### [`RFC5625-4.4-2`](#rfc5625-4.4-2)

If a proxy must unilaterally truncate a response, then the proxy MUST set the TC bit. (§4.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5625ResolverTruncationSetsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L169) | unit/verify | mutant, verified |
| positive | [`TestRFC5625ResolverTruncationSetsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L159) | unit/verify | mutant, verified |

### [`RFC5625-4.4-3`](#rfc5625-4.4-3)

Similarly, proxies MUST NOT remove the TC bit from responses. (§4.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5625ResolverNeverDropsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L208) | unit/verify | mutant, verified |
| positive | [`TestRFC5625ResolverNeverDropsTC`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L198) | unit/verify | mutant, verified |

### [`RFC5625-4.4.1-1`](#rfc5625-4.4.1-1)

DNS proxies MUST therefore be prepared to receive and forward queries over TCP. (§4.4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC5625ProxyTCPAndOPT`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L112) | unit/verify | mutant, verified |

### [`RFC5625-4.4.2-1`](#rfc5625-4.4.2-1)

As per Section 4.1, proxies MUST NOT refuse to proxy such packets. (§4.4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC5625ProxyTCPAndOPT`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_resolver_test.go#L122) | unit/verify | mutant, verified |

### [`RFC5625-4.5-1`](#rfc5625-4.5-1)

DNS proxies MUST implement Section 4.7 of [RFC2845] and either forward packets unchanged (as recommended above) or fully implement TSIG. (§4.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC5625SignedQueryForwardedUnchanged`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_passthrough_test.go#L145) | unit/verify | mutant, verified |

### [`RFC5625-4.5-2`](#rfc5625-4.5-2)

As per Section 4.3, DNS proxies MUST be capable of proxying packets containing TKEY [RFC2930] Resource Records. (§4.5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC5625TKEYQueryProxied`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc5625_passthrough_test.go#L215) | unit/verify | mutant, verified |

### [`RFC5625-5.2-1`](#rfc5625-5.2-1)

this option MUST NOT be sent to clients when no value is configured. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestOption15AbsentWhenUnconfigured`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L217) | unit/verify | mutant, verified |
| positive | [`TestOption15AbsentWhenUnconfigured`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L206) | unit/verify | mutant, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-home-cpe-23 phase 3, 2026-11-08 |
| Signed off | 2026-11-08 |
| Register | rfc2119 |
| Source | rfc/full/rfc5625.txt |
| Source fingerprint | d76369d28c5ab28d |
| Record | rfc/extraction/rfc5625.json |
| Mapped sentences | 12 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Header, status of this memo, copyright notice, abstract and table of contents. |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 2 | reviewed | not stated |
| `4.2` | not stated | 1 | reviewed | not stated |
| `4.3` | not stated | 3 | reviewed | not stated |
| `4.4` | not stated | 3 | reviewed | not stated |
| `4.4.1` | not stated | 1 | reviewed | not stated |
| `4.4.2` | not stated | 1 | reviewed | not stated |
| `4.4.3` | not stated | 0 | reviewed | not stated |
| `4.5` | not stated | 2 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 0 | reviewed | not stated |
| `5.2` | not stated | 1 | reviewed | not stated |
| `5.3` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |
| `6.3` | not stated | 0 | reviewed | not stated |
| `7` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `8` | References heading | 0 | skipped (references) | References heading. |
| `8.1` | Normative references | 0 | skipped (references) | Normative references. |
| `8.2` | Informative references | 0 | skipped (references) | Informative references. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 4.1 explains why proxies drop packets: it describes what RFC 1035 originally specified for the Z bits ("This may be because [RFC1035] originally specified"), and directs no proxy. The obligation it motivates is the next sentence, RFC5625-4.1-1. | This may be because [RFC1035] originally specified that these unused "Z" flag bits "MUST" be zero. |
| `4.3:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | Section 4.3 quotes RFC 3597 ("[RFC3597] requires that resolvers MUST handle Resource Records (RRs) of unknown type transparently."); the obligation belongs to RFC 3597. The proxy obligation this document states is the next two sentences, RFC5625-4.3-1 and RFC5625-4.3-2. | [RFC3597] requires that resolvers MUST handle Resource Records (RRs) of unknown type transparently. |

## Superseded

No document obsoletes RFC 5625, so its obligations are stated where they were written.
