# RFC 8106 - IPv6 Router Advertisement Options for DNS Configuration

RA DNS options (RDNSS, DNSSL). Every requirement listed for RFC 8106, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5 | of 11 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 11 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 11 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 11 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 15 | 15 of 15 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 6 | of 11 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 11 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 11 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 11 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 11 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 11 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 11 | of 24 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 11 mandatory requirements: they add up to 11. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | RA DNS options (RDNSS, DNSSL) |
| Automatic checks | Included |
| Requirements | 24 |
| Mandatory requirements (MUST level) | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 6 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tests linked to requirements | 15 |
| Recorded audit verdicts | 0 |
| Fault test records | 15 |
| Summary | `rfc/short/rfc8106.md` |
| Requirement table | `rfc/requirements/rfc8106.md` |
| RFC text | `rfc/full/rfc8106.txt` |

## Automatic checks

Included in automatic checks: IPv6 Router Advertisement Options for DNS Configuration: the RDNSS and DNSSL Neighbor Discovery options. Every MUST-level row is classified; each is a {gap}, because ze sends RDNSS as a router but sends no DNSSL and, as a host, processes no received RDNSS or DNSSL option.

## Published support status

**Area:** RA DNS options (RDNSS, DNSSL)

**Proof:** 5 of 11 MUST proven

**Evidence:** unit 5

**Unmet:** RFC8106-5.2-1, RFC8106-5.2-2, RFC8106-5.2-3, RFC8106-5.2-4, RFC8106-5.3.1-2, RFC8106-5.3.1-3


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 6 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **11** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (5):** [`RFC8106-1.2-1`](#rfc8106-1.2-1), [`RFC8106-1.2-2`](#rfc8106-1.2-2), [`RFC8106-5.1-1`](#rfc8106-5.1-1), [`RFC8106-5.1-2`](#rfc8106-5.1-2), [`RFC8106-5.3.1-1`](#rfc8106-5.3.1-1)

**Annotated (including scoped evidence) (6):** [`RFC8106-5.2-1`](#rfc8106-5.2-1), [`RFC8106-5.2-2`](#rfc8106-5.2-2), [`RFC8106-5.2-3`](#rfc8106-5.2-3), [`RFC8106-5.2-4`](#rfc8106-5.2-4), [`RFC8106-5.3.1-2`](#rfc8106-5.3.1-2), [`RFC8106-5.3.1-3`](#rfc8106-5.3.1-3)

## Figures

- **MUST bar**: Ze proven 5, Not built 6
- **Overview bar**: Ze MUST proven 5, SHOULD proven 2, Not assessed 11, Not built 6

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 11 |
| MUST rows kept (the MUST bar) | 11 |
| Ze proven | 5 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 6 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 13 |
| SHOULD proven | 2 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 11 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc8106.json](../../../data/rfc/rfc8106.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8106-1.2-1` | Hosts conforming to this specification MUST extract DNS information from RA messages, unless static DNS configuration has been specified by the user. (§1.2) | MUST | 1.2 | **positive:** `unit/verify` [`TestRARDNSSRecordFromUserOpt`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L80). **negative:** `unit/verify` [`TestRARDNSSStaticNameServerWins`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L112) |
| `RFC8106-1.2-2` | If there is DNS information available from multiple RAs and/or from DHCP, the host MUST maintain an ordered list of this information as specified in Section 5.3.1. (§1.2) | MUST | 1.2 | **positive:** `unit/verify` [`TestRARDNSSListedAfterDHCP`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L149). **negative:** `unit/verify` [`TestRARDNSSLaterRAKeepsDHCPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L339) |
| `RFC8106-5.1-1` | A value of zero means that the RDNSS addresses MUST no longer be used. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRARDNSSLifetimeZeroWithdraws`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L196). **positive:** `unit/verify` [`TestRDNSSWithdrawnAdvertisedLifetimeZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc8106_advertiser_rdnss_test.go#L34). **negative:** `unit/verify` [`TestRARDNSSRefreshKeeps`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L131) |
| `RFC8106-5.1-2` | When a resolver sends a DNS query message to an RDNSS identified by a link-local address, it MUST use the corresponding link. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRARDNSSLinkLocalCarriesZone`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L169). **positive:** `unit/verify` [`TestRDNSSRefusesLinkLocalOfAnotherLink`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_dns_source_test.go#L168). **negative:** `unit/verify` [`TestRARDNSSLinkLocalScopedToItsOwnLink`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L313) |
| `RFC8106-5.2-1` | A value of zero means that the DNSSL domain names MUST no longer be used. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze as a host processes no DNSSL option; host-side processing is absent |
| `RFC8106-5.2-2` | One or more domain names of the DNSSL that MUST be encoded as described in Section 3.1 of [RFC1035]. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA writes Source Link-Layer Address, Prefix Information and RDNSS only |
| `RFC8106-5.2-3` | Note that for the simple decoding, the domain names MUST NOT be encoded in the compressed form described in Section 4.1.4 of [RFC1035]. (§5.2) | MUST NOT | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA |
| `RFC8106-5.2-4` | Because the size of this field MUST be a multiple of 8 octets, for the minimum multiple including the domain name representations, the remaining octets other than the encoding parts of the domain name representations MUST be padded with zeros. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA |
| `RFC8106-5.3.1-1` | Otherwise, the host MUST discard the options. (§5.3.1) | MUST | 5.3.1 | **positive:** `unit/verify` [`TestParseRDNSSOptionRefusesBadLength`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc8106_rdnss_test.go#L62). **positive:** `unit/verify` [`TestRARDNSSMalformedDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L219). **negative:** `unit/verify` [`TestParseRDNSSOption`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc8106_rdnss_test.go#L30) |
| `RFC8106-5.3.1-2` | On the other hand, for DNS options announced by RAs, if some RAs use the Secure Neighbor Discovery (SEND) protocol [RFC3971] for RA security, they MUST be preferred over those that do not use SEND. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze consumes RDNSS from the RAs the Linux kernel accepts (internal/component/iface/ra_rdnss.go), and neither Ze nor the kernel implements SEND (RFC 3971), so no RA is known to use SEND and none is preferred |
| `RFC8106-5.3.1-3` | Also, DNS options announced by RAs via SEND MUST be preferred over those announced by unauthenticated DHCP [RFC3118]. (§5.3.1) | MUST | 5.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze consumes RDNSS from the RAs the Linux kernel accepts, and neither Ze nor the kernel implements SEND (RFC 3971), so no RA DNS option is preferred over DHCP for being authenticated |
| `RFC8106-5.1-3` | The value of Lifetime SHOULD by default be at least 3 * MaxRtrAdvInterval, where MaxRtrAdvInterval is the maximum RA interval as defined in [RFC4861]. (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.1-4` | Such link-local addresses SHOULD be registered in the Resolver Repository along with the corresponding link zone indices of the links that receive the RDNSS option(s) for them. (§5.1) | SHOULD | 5.1 | **positive:** `unit/verify` [`TestRARDNSSLinkLocalCarriesZone`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L170). **negative:** no negative test |
| `RFC8106-5.2-5` | That is, Lifetime SHOULD by default be at least 3 * MaxRtrAdvInterval. (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.3.1-4` | If the DNS options are valid, the host SHOULD copy the values of the options into the DNS Repository and the Resolver Repository in order. (§5.3.1) | SHOULD | 5.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.3.1-5` | In the case where the DNS information of RDNSS and DNSSL can be obtained from multiple sources, such as RAs and DHCP, the IPv6 host SHOULD keep some DNS options from all sources. (§5.3.1) | SHOULD | 5.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.3.1-6` | However, in the case of multiple sources, the ability to store a total of at least three RDNSS addresses (or DNSSL domain names) from the multiple sources is RECOMMENDED. (§5.3.1) | RECOMMENDED | 5.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.3.1-7` | The DNS options from RAs and DHCP SHOULD be stored in the DNS Repository and Resolver Repository so that information from DHCP appears there first and therefore takes precedence. (§5.3.1) | SHOULD | 5.3.1 | **positive:** `unit/verify` [`TestRARDNSSListedAfterDHCP`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L150). **negative:** no negative test |
| `RFC8106-6.1-1` | For DNS repository management, the following two data structures SHOULD be synchronized with the Resolver Repository: (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-7.2-1` | The corresponding technique for network devices is RECOMMENDED to block rogue RA messages that include the RDNSS and DNSSL options from unauthorized nodes [RFC6104] [RFC6105]. (§7.2) | RECOMMENDED | 7.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.1-5` | The maximum time in seconds (relative to the time the packet is received) over which these RDNSS addresses MAY be used for name resolution. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.1-6` | Note: The addresses for RDNSSes in the RDNSS option MAY be link-local addresses. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.1-7` | The link-local addresses MAY be represented in the Resolver Repository with their link zone indices in the textual format for scoped addresses as described in [RFC4007]. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8106-5.2-6` | The maximum time in seconds (relative to the time the packet is received) over which these DNSSL domain names MAY be used for name resolution. (§5.2) | MAY | 5.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8106-5.2-1`](#rfc8106-5.2-1) A value of zero means that the DNSSL domain names MUST no longer be used. (§5.2) | {gap}, no test | ze as a host processes no DNSSL option; host-side processing is absent |
| [`RFC8106-5.2-2`](#rfc8106-5.2-2) One or more domain names of the DNSSL that MUST be encoded as described in Section 3.1 of [RFC1035]. (§5.2) | {gap}, no test | ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA writes Source Link-Layer Address, Prefix Information and RDNSS only |
| [`RFC8106-5.2-3`](#rfc8106-5.2-3) Note that for the simple decoding, the domain names MUST NOT be encoded in the compressed form described in Section 4.1.4 of [RFC1035]. (§5.2) | {gap}, no test | ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA |
| [`RFC8106-5.2-4`](#rfc8106-5.2-4) Because the size of this field MUST be a multiple of 8 octets, for the minimum multiple including the domain name representations, the remaining octets other than the encoding parts of the domain name representations MUST be padded with zeros. (§5.2) | {gap}, no test | ze sends no DNSSL option; internal/core/ndp/ra.go BuildRA |
| [`RFC8106-5.3.1-2`](#rfc8106-5.3.1-2) On the other hand, for DNS options announced by RAs, if some RAs use the Secure Neighbor Discovery (SEND) protocol [RFC3971] for RA security, they MUST be preferred over those that do not use SEND. (§5.3.1) | {gap}, no test | ze consumes RDNSS from the RAs the Linux kernel accepts (internal/component/iface/ra_rdnss.go), and neither Ze nor the kernel implements SEND (RFC 3971), so no RA is known to use SEND and none is preferred |
| [`RFC8106-5.3.1-3`](#rfc8106-5.3.1-3) Also, DNS options announced by RAs via SEND MUST be preferred over those announced by unauthenticated DHCP [RFC3118]. (§5.3.1) | {gap}, no test | ze consumes RDNSS from the RAs the Linux kernel accepts, and neither Ze nor the kernel implements SEND (RFC 3971), so no RA DNS option is preferred over DHCP for being authenticated |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC8106-1.2-1`](#rfc8106-1.2-1)

Hosts conforming to this specification MUST extract DNS information from RA messages, unless static DNS configuration has been specified by the user. (§1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRARDNSSStaticNameServerWins`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L112) | unit/verify | mutant, verified |
| positive | [`TestRARDNSSRecordFromUserOpt`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L80) | unit/verify | mutant, verified |

### [`RFC8106-1.2-2`](#rfc8106-1.2-2)

If there is DNS information available from multiple RAs and/or from DHCP, the host MUST maintain an ordered list of this information as specified in Section 5.3.1. (§1.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRARDNSSLaterRAKeepsDHCPFirst`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L339) | unit/verify | mutant, verified |
| positive | [`TestRARDNSSListedAfterDHCP`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L149) | unit/verify | mutant, verified |

### [`RFC8106-5.1-1`](#rfc8106-5.1-1)

A value of zero means that the RDNSS addresses MUST no longer be used. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRARDNSSRefreshKeeps`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L131) | unit/verify | mutant, verified |
| positive | [`TestRARDNSSLifetimeZeroWithdraws`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L196) | unit/verify | mutant, verified |
| positive | [`TestRDNSSWithdrawnAdvertisedLifetimeZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/ra/rfc8106_advertiser_rdnss_test.go#L34) | unit/verify | mutant, verified |

### [`RFC8106-5.1-2`](#rfc8106-5.1-2)

When a resolver sends a DNS query message to an RDNSS identified by a link-local address, it MUST use the corresponding link. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRARDNSSLinkLocalScopedToItsOwnLink`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L313) | unit/verify | mutant, verified |
| positive | [`TestRDNSSRefusesLinkLocalOfAnotherLink`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_dns_source_test.go#L168) | unit/verify | mutant, verified |
| positive | [`TestRARDNSSLinkLocalCarriesZone`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L169) | unit/verify | mutant, verified |

### [`RFC8106-5.2-1`](#rfc8106-5.2-1)

A value of zero means that the DNSSL domain names MUST no longer be used. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8106-5.2-1, so there is no linked test.

### [`RFC8106-5.2-2`](#rfc8106-5.2-2)

One or more domain names of the DNSSL that MUST be encoded as described in Section 3.1 of [RFC1035]. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8106-5.2-2, so there is no linked test.

### [`RFC8106-5.2-3`](#rfc8106-5.2-3)

Note that for the simple decoding, the domain names MUST NOT be encoded in the compressed form described in Section 4.1.4 of [RFC1035]. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8106-5.2-3, so there is no linked test.

### [`RFC8106-5.2-4`](#rfc8106-5.2-4)

Because the size of this field MUST be a multiple of 8 octets, for the minimum multiple including the domain name representations, the remaining octets other than the encoding parts of the domain name representations MUST be padded with zeros. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8106-5.2-4, so there is no linked test.

### [`RFC8106-5.3.1-1`](#rfc8106-5.3.1-1)

Otherwise, the host MUST discard the options. (§5.3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseRDNSSOption`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc8106_rdnss_test.go#L30) | unit/verify | mutant, verified |
| positive | [`TestRARDNSSMalformedDiscarded`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L219) | unit/verify | mutant, verified |
| positive | [`TestParseRDNSSOptionRefusesBadLength`](https://github.com/ze-software/ze/blob/main/internal/core/ndp/rfc8106_rdnss_test.go#L62) | unit/verify | mutant, verified |

### [`RFC8106-5.3.1-2`](#rfc8106-5.3.1-2)

On the other hand, for DNS options announced by RAs, if some RAs use the Secure Neighbor Discovery (SEND) protocol [RFC3971] for RA security, they MUST be preferred over those that do not use SEND. (§5.3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8106-5.3.1-2, so there is no linked test.

### [`RFC8106-5.3.1-3`](#rfc8106-5.3.1-3)

Also, DNS options announced by RAs via SEND MUST be preferred over those announced by unauthenticated DHCP [RFC3118]. (§5.3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC8106-5.3.1-3, so there is no linked test.

### [`RFC8106-5.1-4`](#rfc8106-5.1-4)

Such link-local addresses SHOULD be registered in the Resolver Repository along with the corresponding link zone indices of the links that receive the RDNSS option(s) for them. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRARDNSSLinkLocalCarriesZone`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L170) | unit/verify | mutant, verified |

### [`RFC8106-5.3.1-7`](#rfc8106-5.3.1-7)

The DNS options from RAs and DHCP SHOULD be stored in the DNS Repository and Resolver Repository so that information from DHCP appears there first and therefore takes precedence. (§5.3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRARDNSSListedAfterDHCP`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc8106_ra_rdnss_test.go#L150) | unit/verify | mutant, verified |

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 2 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 2 | reviewed | not stated |
| `5.2` | not stated | 4 | reviewed | not stated |
| `5.3` | not stated | 0 | reviewed | not stated |
| `5.3.1` | not stated | 3 | reviewed | not stated |
| `5.3.2` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 0 | reviewed | not stated |
| `6.2` | not stated | 0 | reviewed | not stated |
| `6.3` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `7.1` | not stated | 0 | reviewed | not stated |
| `7.2` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 8106 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 8106, so its obligations are stated where they were written.
