# RFC 2308 - Negative Caching of DNS Queries (DNS NCACHE)

DNS negative caching. Every requirement listed for RFC 2308, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 12 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 4 | of 12 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 12 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 12 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 14 | 14 of 14 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 4 | of 12 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

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
| Mandatory requirements (MUST level) | 12 | of 18 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 12 mandatory requirements: they add up to 12. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | DNS negative caching |
| Automatic checks | Included |
| Requirements | 18 |
| Mandatory requirements (MUST level) | 12 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 6 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 14 |
| Tests linked to requirements | 14 |
| Recorded audit verdicts | 0 |
| Fault test records | 14 |
| Summary | `rfc/short/rfc2308.md` |
| Requirement table | `rfc/requirements/rfc2308.md` |
| RFC text | `rfc/full/rfc2308.txt` |

## Automatic checks

Included in automatic checks: Negative Caching of DNS Queries, brought into scope by the owner for the fttx-edge use case: the home-connection resolver (internal/plugins/dnsresolver) caches negative answers, and the answering plugins (internal/plugins/geodns, internal/plugins/as112) are the authoritative servers Section 3 binds. Every MUST is classified. Met by a producer and owed its tags: the zone SOA in the authority section of an authoritative NXDOMAIN or NODATA (3-1), an expired negative entry never used again (5-1), the cached SOA returned with its TTL decremented (6-1), the authority-section SOA cached (8-1). Met because the resolver caches no server failure and no dead-server indication, so only the negative polarity is observable (7.1-1, 7.1-2, 7.2-1, 7.2-2). Gaps: the answering plugins serve unsigned zones only (3-2, 5-3), and the resolver caches the SOA alone, dropping any NXT and SIG record of a received negative answer (5-2, 6-2).

## Published support status

**Area:** DNS negative caching

**Test verification:** 8 of 12 MUST proven

**Evidence:** unit 8

**Unmet:** RFC2308-3-2, RFC2308-5-2, RFC2308-5-3, RFC2308-6-2, RFC2308-6-3, RFC2308-6-4


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

**Positive and negative tests (4):** [`RFC2308-3-1`](#rfc2308-3-1), [`RFC2308-5-1`](#rfc2308-5-1), [`RFC2308-6-1`](#rfc2308-6-1), [`RFC2308-8-1`](#rfc2308-8-1)

**Annotated (including scoped evidence) (8):** [`RFC2308-3-2`](#rfc2308-3-2), [`RFC2308-5-2`](#rfc2308-5-2), [`RFC2308-5-3`](#rfc2308-5-3), [`RFC2308-6-2`](#rfc2308-6-2), [`RFC2308-7.1-1`](#rfc2308-7.1-1), [`RFC2308-7.1-2`](#rfc2308-7.1-2), [`RFC2308-7.2-1`](#rfc2308-7.2-1), [`RFC2308-7.2-2`](#rfc2308-7.2-2)

## Figures

- **MUST bar**: Ze verified 8, Not built 4
- **Overview bar**: Ze MUST verified 8, Not assessed 4, Not built 6

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 12 |
| MUST rows kept (the MUST bar) | 12 |
| Ze verified | 8 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 4 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 2 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc2308.json](../../../data/rfc/rfc2308.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2308-3-1` | Name servers authoritative for a zone MUST include the SOA record of the zone in the authority section of the response when reporting an NXDOMAIN or indicating that no data of the requested type exists. (§3) | MUST | 3 - Negative Answers from Authoritative Servers | **positive:** `unit/verify` [`as112/TestRFC2308_NegativeAnswerCarriesZoneSOA`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc2308_negative_soa_test.go#L49). **positive:** `unit/verify` [`geodns/TestRFC2308_NegativeAnswerCarriesZoneSOA`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2308_negative_soa_test.go#L50). **negative:** `unit/verify` [`as112/TestRFC2308_NegativeAnswerCarriesZoneSOA`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc2308_negative_soa_test.go#L63). **negative:** `unit/verify` [`geodns/TestRFC2308_NegativeAnswerCarriesZoneSOA`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2308_negative_soa_test.go#L64) |
| `RFC2308-3-2` | If the containing zone is signed [RFC2065] the SOA and appropriate NXT and SIG records MUST be added. (§3) | MUST | 3 - Negative Answers from Authoritative Servers | **positive:** no positive test. **negative:** no negative test. **{gap}:** neither answering plugin signs a zone, so a signed zone's negative answer with its NXT (now NSEC) and SIG (now RRSIG) records is never produced; internal/plugins/geodns/server.go::answerQuestions and internal/plugins/as112/zones.go::answerQuestions add the SOA alone |
| `RFC2308-5-1` | This TTL decrements in a similar manner to a normal cached answer and upon reaching zero (0) indicates the cached negative answer MUST NOT be used again. (§5) | MUST NOT | 5 - Caching Negative Answers | **positive:** `unit/verify` [`TestRFC2308NegativeEntryExpires`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L62). **negative:** `unit/verify` [`TestRFC2308NegativeEntryExpires`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L46) |
| `RFC2308-5-2` | The NXT record, if it exists in the authority section of a negative answer received, MUST be stored such that it can be be located and returned with SOA record in the authority section (§5) | MUST | 5 - Caching Negative Answers | **positive:** no positive test. **negative:** no negative test. **{gap}:** storeNegative caches the SOA alone, so an NXT (or NSEC) record in a received negative answer is dropped and never returned; internal/plugins/dnsresolver/resolve.go::storeNegative |
| `RFC2308-5-3` | The NXT record MUST have the same owner name as the query name for NODATA responses. (§5) | MUST | 5 - Caching Negative Answers | **positive:** no positive test. **negative:** no negative test. **{gap}:** no part of Ze produces a NODATA response carrying an NXT (or NSEC) record, because the answering plugins serve unsigned zones and the resolver caches none; internal/plugins/geodns/server.go::answerQuestions, internal/plugins/dnsresolver/resolve.go::storeNegative |
| `RFC2308-6-1` | When a server, in answering a query, encounters a cached negative response it MUST add the cached SOA record to the authority section of the response with the TTL decremented by the amount of time it was stored in the cache. (§6) | MUST | 6 - Negative answers from the cache | **positive:** `unit/verify` [`TestRFC2308CachedSOATTLDecremented`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L109). **negative:** `unit/verify` [`TestRFC2308CachedSOATTLDecremented`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L86) |
| `RFC2308-6-2` | If a NXT record was cached along with SOA record it MUST be added to the authority section. (§6) | MUST | 6 - Negative answers from the cache | **positive:** no positive test. **negative:** no negative test. **{gap}:** the negative cache holds the SOA alone, so no NXT (or NSEC) record is cached with it or added from it; internal/plugins/dnsresolver/resolve.go::storeNegative, internal/plugins/dnsresolver/cache.go::lookup |
| `RFC2308-7.1-1` | If it does so it MUST NOT cache it for longer than five (5) minutes (§7.1) | MUST NOT | 7.1 - Server Failure, headed OPTIONAL | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC2308ServerFailureNotCached`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L204). **{single-polarity}:** the resolver caches no server failure at all (internal/plugins/dnsresolver/resolve.go::servfail is never stored), so the only observable is that a later identical query is forwarded again rather than answered from a cache |
| `RFC2308-7.1-2` | it MUST be cached against the specific query tuple <query name, type, class, server IP address>. (§7.1) | MUST | 7.1 - Server Failure, headed OPTIONAL | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC2308ServerFailureHeldAgainstNoTuple`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L231). **{single-polarity}:** the resolver caches no server failure, so no cached failure can reach a query outside its tuple, and the observable is a later query for the same or another tuple being forwarded again |
| `RFC2308-7.2-1` | If it does so it MUST NOT be deemed dead for longer than five (5) minutes. (§7.2) | MUST NOT | 7.2 - Dead / Unreachable Server, headed OPTIONAL | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC2308SilentUpstreamAskedAgain`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L271). **{single-polarity}:** the resolver keeps no dead-server indication (internal/plugins/dnsresolver/resolve.go::forward asks every configured upstream again on each query), so the observable is a silent upstream being asked again on the next query |
| `RFC2308-7.2-2` | The indication MUST be stored against query tuple <query name, type, class, server IP address> unless there was a transport layer indication that the server does not exist, in which case it applies to all queries to that specific IP address. (§7.2) | MUST | 7.2 - Dead / Unreachable Server, headed OPTIONAL | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC2308SilentUpstreamNotDeemedDeadBeyondTheTuple`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L298). **{single-polarity}:** the resolver stores no dead-server indication, so none can be applied beyond its tuple, and the observable is an upstream that failed one query being asked for the next |
| `RFC2308-8-1` | The SOA record from the authority section MUST be cached. (§8) | MUST | 8 - Changes from RFC 1034 | **positive:** `unit/verify` [`TestRFC2308AuthoritySOACached`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L142). **negative:** `unit/verify` [`TestRFC2308AuthoritySOACached`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L165) |
| `RFC2308-5-4` | Negative responses without SOA records SHOULD NOT be cached as there is no way to prevent the negative responses looping forever between a pair of servers even with a short TTL. (§5) | SHOULD NOT | 5 - Caching Negative Answers | **positive:** no positive test. **negative:** no negative test |
| `RFC2308-6-3` | If a SIG record was cached along with a NXT record it SHOULD be added to the authority section. (§6) | SHOULD | 6 - Negative answers from the cache | **positive:** no positive test. **negative:** no negative test. **{gap}:** the negative cache holds the SOA alone, so no SIG (or RRSIG) record is cached with an NXT or added from it; internal/plugins/dnsresolver/resolve.go::storeNegative |
| `RFC2308-6-4` | As with all answers coming from the cache, negative answers SHOULD have an implicit referral built into the answer. (§6) | SHOULD | 6 - Negative answers from the cache | **positive:** no positive test. **negative:** no negative test. **{gap}:** a negative answer from the cache carries the SOA alone in the authority section and no NS records referring the client to an authoritative source; internal/plugins/dnsresolver/cache.go::lookup |
| `RFC2308-7.1-3` | In either case a resolver MAY cache a server failure response. (§7.1) | MAY | 7.1 - Server Failure, headed OPTIONAL | **positive:** no positive test. **negative:** no negative test |
| `RFC2308-7.2-3` | A server MAY cache a dead server indication. (§7.2) | MAY | 7.2 - Dead / Unreachable Server, headed OPTIONAL | **positive:** no positive test. **negative:** no negative test |
| `RFC2308-8-2` | Non-authoritative negative answers MAY be cached. (§8) | MAY | 8 - Changes from RFC 1034 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2308-3-2`](#rfc2308-3-2) If the containing zone is signed [RFC2065] the SOA and appropriate NXT and SIG records MUST be added. (§3) | {gap}, no test | neither answering plugin signs a zone, so a signed zone's negative answer with its NXT (now NSEC) and SIG (now RRSIG) records is never produced; internal/plugins/geodns/server.go::answerQuestions and internal/plugins/as112/zones.go::answerQuestions add the SOA alone |
| [`RFC2308-5-2`](#rfc2308-5-2) The NXT record, if it exists in the authority section of a negative answer received, MUST be stored such that it can be be located and returned with SOA record in the authority section (§5) | {gap}, no test | storeNegative caches the SOA alone, so an NXT (or NSEC) record in a received negative answer is dropped and never returned; internal/plugins/dnsresolver/resolve.go::storeNegative |
| [`RFC2308-5-3`](#rfc2308-5-3) The NXT record MUST have the same owner name as the query name for NODATA responses. (§5) | {gap}, no test | no part of Ze produces a NODATA response carrying an NXT (or NSEC) record, because the answering plugins serve unsigned zones and the resolver caches none; internal/plugins/geodns/server.go::answerQuestions, internal/plugins/dnsresolver/resolve.go::storeNegative |
| [`RFC2308-6-2`](#rfc2308-6-2) If a NXT record was cached along with SOA record it MUST be added to the authority section. (§6) | {gap}, no test | the negative cache holds the SOA alone, so no NXT (or NSEC) record is cached with it or added from it; internal/plugins/dnsresolver/resolve.go::storeNegative, internal/plugins/dnsresolver/cache.go::lookup |
| [`RFC2308-6-3`](#rfc2308-6-3) If a SIG record was cached along with a NXT record it SHOULD be added to the authority section. (§6) | {gap} | the negative cache holds the SOA alone, so no SIG (or RRSIG) record is cached with an NXT or added from it; internal/plugins/dnsresolver/resolve.go::storeNegative |
| [`RFC2308-6-4`](#rfc2308-6-4) As with all answers coming from the cache, negative answers SHOULD have an implicit referral built into the answer. (§6) | {gap} | a negative answer from the cache carries the SOA alone in the authority section and no NS records referring the client to an authoritative source; internal/plugins/dnsresolver/cache.go::lookup |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC2308-3-1`](#rfc2308-3-1)

Name servers authoritative for a zone MUST include the SOA record of the zone in the authority section of the response when reporting an NXDOMAIN or indicating that no data of the requested type exists. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`as112/TestRFC2308_NegativeAnswerCarriesZoneSOA`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc2308_negative_soa_test.go#L63) | unit/verify | mutant, verified |
| negative | [`geodns/TestRFC2308_NegativeAnswerCarriesZoneSOA`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2308_negative_soa_test.go#L64) | unit/verify | mutant, verified |
| positive | [`as112/TestRFC2308_NegativeAnswerCarriesZoneSOA`](https://github.com/ze-software/ze/blob/main/internal/plugins/as112/rfc2308_negative_soa_test.go#L49) | unit/verify | mutant, verified |
| positive | [`geodns/TestRFC2308_NegativeAnswerCarriesZoneSOA`](https://github.com/ze-software/ze/blob/main/internal/plugins/geodns/rfc2308_negative_soa_test.go#L50) | unit/verify | mutant, verified |

### [`RFC2308-3-2`](#rfc2308-3-2)

If the containing zone is signed [RFC2065] the SOA and appropriate NXT and SIG records MUST be added. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2308-3-2, so there is no linked test.

### [`RFC2308-5-1`](#rfc2308-5-1)

This TTL decrements in a similar manner to a normal cached answer and upon reaching zero (0) indicates the cached negative answer MUST NOT be used again. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2308NegativeEntryExpires`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L46) | unit/verify | mutant, verified |
| positive | [`TestRFC2308NegativeEntryExpires`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L62) | unit/verify | mutant, verified |

### [`RFC2308-5-2`](#rfc2308-5-2)

The NXT record, if it exists in the authority section of a negative answer received, MUST be stored such that it can be be located and returned with SOA record in the authority section (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2308-5-2, so there is no linked test.

### [`RFC2308-5-3`](#rfc2308-5-3)

The NXT record MUST have the same owner name as the query name for NODATA responses. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2308-5-3, so there is no linked test.

### [`RFC2308-6-1`](#rfc2308-6-1)

When a server, in answering a query, encounters a cached negative response it MUST add the cached SOA record to the authority section of the response with the TTL decremented by the amount of time it was stored in the cache. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2308CachedSOATTLDecremented`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L86) | unit/verify | mutant, verified |
| positive | [`TestRFC2308CachedSOATTLDecremented`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L109) | unit/verify | mutant, verified |

### [`RFC2308-6-2`](#rfc2308-6-2)

If a NXT record was cached along with SOA record it MUST be added to the authority section. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC2308-6-2, so there is no linked test.

### [`RFC2308-7.1-1`](#rfc2308-7.1-1)

If it does so it MUST NOT cache it for longer than five (5) minutes (§7.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2308ServerFailureNotCached`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L204) | unit/verify | mutant, verified |

### [`RFC2308-7.1-2`](#rfc2308-7.1-2)

it MUST be cached against the specific query tuple <query name, type, class, server IP address>. (§7.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2308ServerFailureHeldAgainstNoTuple`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L231) | unit/verify | mutant, verified |

### [`RFC2308-7.2-1`](#rfc2308-7.2-1)

If it does so it MUST NOT be deemed dead for longer than five (5) minutes. (§7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2308SilentUpstreamAskedAgain`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L271) | unit/verify | mutant, verified |

### [`RFC2308-7.2-2`](#rfc2308-7.2-2)

The indication MUST be stored against query tuple <query name, type, class, server IP address> unless there was a transport layer indication that the server does not exist, in which case it applies to all queries to that specific IP address. (§7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2308SilentUpstreamNotDeemedDeadBeyondTheTuple`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L298) | unit/verify | mutant, verified |

### [`RFC2308-8-1`](#rfc2308-8-1)

The SOA record from the authority section MUST be cached. (§8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC2308AuthoritySOACached`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L165) | unit/verify | mutant, verified |
| positive | [`TestRFC2308AuthoritySOACached`](https://github.com/ze-software/ze/blob/main/internal/plugins/dnsresolver/rfc2308_resolver_test.go#L142) | unit/verify | mutant, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-11-19 |
| Register | rfc2119 |
| Source | rfc/full/rfc2308.txt |
| Source fingerprint | 0e2087c85f5f71a9 |
| Record | rfc/extraction/rfc2308.json |
| Mapped sentences | 11 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | Header, status, copyright and abstract | 0 | reviewed | Header, status, copyright and abstract. The abstract states the document replaces RFC 1034 Section 4.3.4 and that negative caching should no longer be seen as optional; the lowercase should is not a BCP 14 keyword in this post-RFC 2119 document, and Section 8 restates it. |
| `1` | Terminology | 0 | reviewed | Terminology. The RFC 2119 key-words paragraph and the definitions of negative caching, QNAME, NXDOMAIN, NODATA and FORWARDER. The lowercase should about ordering CNAME records states no capitalised obligation. |
| `2` | not stated | 0 | reviewed | Negative Responses: introduction to the two kinds, with no normative sentence. |
| `2.1` | not stated | 0 | reviewed | Name Error: defines NXDOMAIN and its four types by authority-section content, against a referral. Descriptive, no keyword. |
| `2.1.1` | not stated | 0 | reviewed | Special Handling of Name Error: the lowercase recommendation that authoritative servers send only TYPE 2 NXDOMAIN, and the note on forcing the authority flag for forwarders. No capitalised keyword. |
| `2.2` | not stated | 0 | reviewed | No Data: defines NODATA and its three types, and how it is told from a referral. Descriptive, no keyword. |
| `2.2.1` | not stated | 0 | reviewed | Special Handling of No Data: the lowercase recommendation to send only TYPE 2 NODATA and the lowercase must to query again using the QNAME when a server fails to set NXDOMAIN beside CNAMEs. No capitalised keyword. |
| `3` | Negative Answers from Authoritative Servers | 2 | reviewed | Negative Answers from Authoritative Servers. Two capitalised MUSTs, mapped to RFC2308-3-1 and RFC2308-3-2. The SOA TTL rule (minimum of MINIMUM and the SOA's TTL) and the lowercase should on trimming the SIG TTL carry no keyword. |
| `4` | SOA Minimum Field | 0 | reviewed | SOA Minimum Field. Deprecates MINIMUM as the minimum TTL of every RR in a zone (the RFC 1035 Section 3.3.13 meaning), adds the $TTL master-file directive, and makes the negative-response TTL MINIMUM's one meaning. No capitalised keyword; the lowercase should on providing a default-TTL mechanism binds a primary server reading master files. |
| `5` | Caching Negative Answers | 3 | reviewed | Caching Negative Answers. Three capitalised MUST-level sites, mapped to RFC2308-5-1, RFC2308-5-2 and RFC2308-5-3. The advisory SHOULD NOT on caching a negative response without an SOA carries no site and is RFC2308-5-4. The lowercase shoulds on the NXDOMAIN and NODATA cache keys, on SIG records, and on a negative cache limit carry no keyword. |
| `6` | Negative answers from the cache | 2 | reviewed | Negative answers from the cache. Two capitalised MUSTs, mapped to RFC2308-6-1 and RFC2308-6-2. The two advisory SHOULDs carry no site: the SIG record cached with an NXT (RFC2308-6-3) and the implicit referral (RFC2308-6-4). |
| `7` | not stated | 0 | reviewed | Other Negative Responses: introduction, no normative sentence. |
| `7.1` | Server Failure, headed OPTIONAL | 1 | reviewed | Server Failure, headed OPTIONAL. The MAY to cache a server failure response carries no site (RFC2308-7.1-3). Site 7.1:1 holds two obligations conditional on that MAY: the MUST NOT on the five-minute limit, mapped to RFC2308-7.1-1, and the MUST on the query tuple, which is its second clause and is listed here as RFC2308-7.1-2. |
| `7.2` | Dead / Unreachable Server, headed OPTIONAL | 2 | reviewed | Dead / Unreachable Server, headed OPTIONAL. The MAY to cache a dead server indication carries no site (RFC2308-7.2-3). Two capitalised MUST-level sites conditional on it, mapped to RFC2308-7.2-1 and RFC2308-7.2-2. The lowercase may on deeming a server dead after 120 seconds carries no keyword. |
| `8` | Changes from RFC 1034 | 1 | reviewed | Changes from RFC 1034. One capitalised MUST, mapped to RFC2308-8-1, and the MAY to cache non-authoritative negative answers (RFC2308-8-2). The lowercase musts restate Sections 5 and 6 and carry no keyword. |
| `9` | not stated | 0 | reviewed | History of Negative Caching, which states it forms no part of the technical specification. |
| `9.2` | BIND history, non-normative under Section 9 | 0 | reviewed | BIND history, non-normative under Section 9. |
| `10` | not stated | 0 | reviewed | Example of a signed zone's initial and cached NXDOMAIN response. Illustrative, no keyword. |
| `11` | not stated | 0 | reviewed | Security Considerations: the high-TTL NXDOMAIN denial of service and TTL sanity checking. No capitalised keyword. |

### Excluded sentences

The review of RFC 2308 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 2308, so its obligations are stated where they were written.
