# RFC 7858 - Specification for DNS over Transport Layer Security (TLS)

DNS over TLS (DoT). Every requirement listed for RFC 7858, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 19 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3 | of 19 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 19 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 19 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 8 | 8 of 15 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 12 | of 19 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 19 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 19 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 19 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 19 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 19 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 19 | of 38 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 19 mandatory requirements: they add up to 19. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Public row | DNS over TLS (DoT) |
| Automatic checks | Included |
| Requirements | 38 |
| Mandatory requirements (MUST level) | 19 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 12 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tests linked to requirements | 15 |
| Recorded audit verdicts | 6 |
| Fault test records | 8 |
| Summary | `rfc/short/rfc7858.md` |
| Requirement table | `rfc/requirements/rfc7858.md` |
| RFC text | `rfc/full/rfc7858.txt` |

## Automatic checks

Included in automatic checks: DNS over TLS / DoT (RFC 7858): ze is a DoT server (the shared DNS harness) and a DNS client that speaks no DoT (the dnsresolver forwarder, UDP and TCP upstream, and the resolve/dns stub, UDP). 4 MET (TLS-first handshake, DoT port refuses cleartext (2 facets), TLS 1.2/BCP 195 floor) + 3 single-polarity positive (listen/accept on 853, RFC 7766 two-octet length framing, robust to idle-connection close) + 12 gaps: the DoT listen-port 53 is not rejected; nine DoT client MUSTs bind ze's DNS clients, which implement no DoT, and one of them, cleartext to a DoT port, an operator can break by giving a forward server port 853; the two TCP Fast Open MUSTs hold by construction, with no test

## Published support status

**Area:** DNS over TLS (DoT)

**Proof:** 3 of 19 MUST proven

**Evidence:** unit 3

**Unmet:** RFC7858-3.1-2, RFC7858-3.1-3, RFC7858-3.1-7, RFC7858-3.3-4, RFC7858-3.3-5, RFC7858-3.4-7, RFC7858-3.4-8, RFC7858-3.4-9, RFC7858-4.2-4, RFC7858-4.2-5, RFC7858-4.2-6, RFC7858-4.2-7


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 15 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **19** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (4):** [`RFC7858-3.1-5`](#rfc7858-3.1-5), [`RFC7858-3.1-6`](#rfc7858-3.1-6), [`RFC7858-3.1-8`](#rfc7858-3.1-8), [`RFC7858-8-1`](#rfc7858-8-1)

**Annotated (including scoped evidence) (15):** [`RFC7858-3.1-1`](#rfc7858-3.1-1), [`RFC7858-3.1-2`](#rfc7858-3.1-2), [`RFC7858-3.1-3`](#rfc7858-3.1-3), [`RFC7858-3.1-7`](#rfc7858-3.1-7), [`RFC7858-3.3-1`](#rfc7858-3.3-1), [`RFC7858-3.3-4`](#rfc7858-3.3-4), [`RFC7858-3.3-5`](#rfc7858-3.3-5), [`RFC7858-3.4-5`](#rfc7858-3.4-5), [`RFC7858-3.4-7`](#rfc7858-3.4-7), [`RFC7858-3.4-8`](#rfc7858-3.4-8), [`RFC7858-3.4-9`](#rfc7858-3.4-9), [`RFC7858-4.2-4`](#rfc7858-4.2-4), [`RFC7858-4.2-5`](#rfc7858-4.2-5), [`RFC7858-4.2-6`](#rfc7858-4.2-6), [`RFC7858-4.2-7`](#rfc7858-4.2-7)

## Figures

- **MUST bar**: Ze proven 3, Proof pending 4, Not built 12
- **Overview bar**: Ze MUST proven 3, Proof pending 4, Not assessed 19, Not built 12

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 19 |
| MUST rows kept (the MUST bar) | 19 |
| Ze proven | 3 |
| OS verified | 0 |
| Proof pending | 4 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 12 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 19 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 19 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7858.json](../../../data/rfc/rfc7858.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7858-3.1-1` | By default, a DNS server that supports DNS over TLS MUST listen for and accept TCP connections on port 853, unless it has mutual agreement with its clients to use a port other than 853 for DNS over TLS. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestDefaultSecureConfig`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L379). **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L52). **negative:** no negative test. **{single-polarity}:** affirmative listen mandate. Positive proof: a DoT listener accepts a TCP+TLS connection and answers (internal/core/dnsserver/secure_test.go TestDoTListener) with the default DoT port pinned to 853 (internal/core/dnsserver/secure.go:41, TestDefaultSecureConfig). A "must listen and accept" obligation has no rejecting counter-behavior to assert as a negative. |
| `RFC7858-3.1-2` | By default, a DNS client desiring privacy from DNS over TLS from a particular server MUST establish a TCP connection to port 853 on the server, unless it has mutual agreement with its server to use a port other than port 853 for DNS over TLS. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client establishes a DoT connection to port 853 |
| `RFC7858-3.1-3` | Such another port MUST NOT be port 53 (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not enforce this. ParseSecureLeaves validates the DoT listen-port only as 1..65535 (internal/core/dnsserver/secure.go:276-279) and does not reject 53, so an operator can configure the DoT listener on port 53. |
| `RFC7858-3.1-5` | The first data exchange on this TCP connection MUST be the client and server initiating a TLS handshake using the procedure described in [RFC5246]. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L53). **negative:** `unit/verify` [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L642) |
| `RFC7858-3.1-6` | DNS clients and servers MUST NOT use port 853 to transport cleartext DNS messages (§3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L54). **negative:** `unit/verify` [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L643) |
| `RFC7858-3.1-7` | DNS clients MUST NOT send and DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS (including, for example, after a failed TLS handshake). (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the client clause binds ze's DNS clients and nothing holds it: the forward server parser in internal/plugins/dnsresolver/config.go accepts any upstream port from 1 to 65535, 853 included, and the forwarder sends cleartext to it (exchange in internal/plugins/dnsresolver/forward.go), so an operator can make ze send cleartext DNS to a port used for DoT; the server clause is met under RFC7858-3.1-8 |
| `RFC7858-3.1-8` | DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS (including, for example, after a failed TLS handshake). (§3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L55). **negative:** `unit/verify` [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L644) |
| `RFC7858-3.3-1` | All messages (requests and responses) in the established TLS session MUST use the two-octet length field described in Section 4.2.2 of [RFC1035]. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L56). **negative:** no negative test. **{single-polarity}:** the RFC 1035 4.2.2 two-octet length-prefixed framing is provided by the miekg/dns Server ze hands the TLS listener to (internal/core/dnsserver/secure.go:514-515). A successful DoT round trip (internal/core/dnsserver/secure_test.go TestDoTListener) proves conformant framing; ze has no code path that emits non-length-prefixed framing to exercise as a negative. |
| `RFC7858-3.3-4` | Since pipelined responses can arrive out of order, clients MUST match responses to outstanding queries on the same TLS connection using the Message ID. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client matches pipelined DoT responses by Message ID |
| `RFC7858-3.3-5` | If the response contains a Question Section, the client MUST match the QNAME, QCLASS, and QTYPE fields. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client matches the Question Section of a DoT response |
| `RFC7858-3.4-5` | Clients and servers that keep idle connections open MUST be robust to termination of idle connection by either party. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestDoTRobustToIdleConnectionClose`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L718). **positive:** `unit/verify` [`TestRFC7858ServerIdleCloseLeavesTheServerServing`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc7858_idle_close_test.go#L29). **negative:** no negative test. **{single-polarity}:** server-side liveness property. Positive proof, one per party: after a DoT client abruptly closes an idle connection the server keeps serving and answers a fresh connection (internal/core/dnsserver/secure_test.go TestDoTRobustToIdleConnectionClose), and after the server itself closes an idle connection it answers a fresh one (internal/core/dnsserver/rfc7858_idle_close_test.go TestRFC7858ServerIdleCloseLeavesTheServerServing). Ze runs no DoT client. "Remains robust" has no failure polarity to assert as a negative. |
| `RFC7858-3.4-7` | As with current DNS over TCP, clients MUST handle abrupt closes and be prepared to reestablish connections and/or retry queries. (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client handles an abrupt DoT close or retries over DoT |
| `RFC7858-3.4-8` | when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's DoT listener does not use TCP Fast Open: bindDoT opens an ordinary TCP socket through lc.Listen and sets no TCP_FASTOPEN option (internal/core/dnsserver/secure.go), and ze's DNS clients speak no DoT; the obligation holds by construction for the socket ze opens, but no test asserts the listener carries no TCP Fast Open |
| `RFC7858-3.4-9` | when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (cleartext DNS MUST NOT be exchanged). (§3.4) | MUST NOT | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's DoT listener does not use TCP Fast Open: bindDoT opens an ordinary TCP socket through lc.Listen and sets no TCP_FASTOPEN option (internal/core/dnsserver/secure.go), and ze's DNS clients speak no DoT; the prohibition holds by construction for the socket ze opens, but no test asserts the listener carries no TCP Fast Open |
| `RFC7858-4.2-4` | The user MUST be alerted whenever possible that the DNS is not private during such bootstrap. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client bootstraps a DoT profile or alerts a user about one |
| `RFC7858-4.2-5` | Otherwise, the client MUST treat the SPKI validation failure as a non-recoverable error. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client validates a pinned SPKI |
| `RFC7858-4.2-6` | Implementations of this privacy profile MUST support the calculation of a fingerprint as the SHA-256 [RFC6234] hash of the DER-encoded ASN.1 representation of the SPKI of an X.509 certificate. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client computes an SPKI fingerprint |
| `RFC7858-4.2-7` | Implementations MUST support the representation of a SHA-256 fingerprint as a base64-encoded character string [RFC4648]. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client stores a base64 SPKI pin |
| `RFC7858-8-1` | Clients and servers MUST adhere to the TLS implementation recommendations and security considerations of [BCP195]. (§8) | MUST | 8 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L57). **positive:** `unit/verify` [`TestRFC7858DoTNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc7858_bcp195_test.go#L24). **negative:** `unit/verify` [`TestDoTRejectsBelowTLS12`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L694). **negative:** `unit/verify` [`TestRFC7858DoTNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc7858_bcp195_test.go#L25) |
| `RFC7858-3.1-9` | DNS clients SHOULD remember server IP addresses that don't support DNS over TLS, including timeouts, connection refusals, and TLS handshake failures, and not request DNS over TLS from them for a reasonable period (such as one hour per server). (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.3-2` | For reasons of efficiency, DNS clients and servers SHOULD pass the two-octet length field, and the message described by that length field, to the TCP layer at the same time (e.g., in a single "write" system call) to make it more likely that all the data will be transmitted in a single TCP segment ([RFC7766], Section 8). (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.3-3` | In order to minimize latency, clients SHOULD pipeline multiple queries over a TLS session. (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-1` | To avoid excess TCP connections, each with a single query, clients SHOULD reuse a single TCP connection to the recursive resolver. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-2` | In order to amortize TCP and TLS connection setup costs, clients and servers SHOULD NOT immediately close a connection after each response. (§3.4) | SHOULD NOT | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-3` | Instead, clients and servers SHOULD reuse existing connections for subsequent queries as long as they have sufficient resources. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-4` | An implementor of DNS over TLS SHOULD follow best practices for DNS over TCP, as described in [RFC7766]. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-10` | DNS servers SHOULD enable fast TLS session resumption (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-11` | DNS servers SHOULD enable fast TLS session resumption [RFC5077], and this SHOULD be used when reestablishing connections. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-12` | When closing a connection, DNS servers SHOULD use the TLS close- notify request to shift TCP TIME-WAIT state to the clients. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-4.2-1` | With this out-of-band key-pinned privacy profile, client administrators SHOULD deploy a backup pin along with the primary pin, for the reasons explained in [RFC7469]. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-4.2-2` | After a change of keys on the server, an updated pin set SHOULD be distributed to all clients in some secure way (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-5-1` | To minimize state on DNS servers and connection startup time, clients SHOULD minimize the creation of new TCP connections. (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.1-4` | Such another port MUST NOT be port 53 but MAY be from the "first-come, first-served" port range. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.1-10` | DNS clients following an out-of-band key-pinned privacy profile (Section 4.2) MAY be more aggressive about retrying DNS-over-TLS connection failures. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-6` | As with current DNS over TCP, DNS servers MAY close the connection at any time (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-4.2-3` | Techniques such as those used by DNSSEC-trigger [DNSSEC-TRIGGER] MAY be used during network configuration, with the intent to transition to the designated DNS provider after authentication. (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-4.2-8` | Additional fingerprint types MAY also be supported (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-8-2` | For this reason, clients MAY discard cached information about server capabilities advertised in cleartext. (§8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7858-3.1-2`](#rfc7858-3.1-2) By default, a DNS client desiring privacy from DNS over TLS from a particular server MUST establish a TCP connection to port 853 on the server, unless it has mutual agreement with its server to use a port other than port 853 for DNS over TLS. (§3.1) | {gap}, no test | ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client establishes a DoT connection to port 853 |
| [`RFC7858-3.1-3`](#rfc7858-3.1-3) Such another port MUST NOT be port 53 (§3.1) | {gap}, no test | ze does not enforce this. ParseSecureLeaves validates the DoT listen-port only as 1..65535 (internal/core/dnsserver/secure.go:276-279) and does not reject 53, so an operator can configure the DoT listener on port 53. |
| [`RFC7858-3.1-7`](#rfc7858-3.1-7) DNS clients MUST NOT send and DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS (including, for example, after a failed TLS handshake). (§3.1) | {gap}, no test | the client clause binds ze's DNS clients and nothing holds it: the forward server parser in internal/plugins/dnsresolver/config.go accepts any upstream port from 1 to 65535, 853 included, and the forwarder sends cleartext to it (exchange in internal/plugins/dnsresolver/forward.go), so an operator can make ze send cleartext DNS to a port used for DoT; the server clause is met under RFC7858-3.1-8 |
| [`RFC7858-3.3-4`](#rfc7858-3.3-4) Since pipelined responses can arrive out of order, clients MUST match responses to outstanding queries on the same TLS connection using the Message ID. (§3.3) | {gap}, no test | ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client matches pipelined DoT responses by Message ID |
| [`RFC7858-3.3-5`](#rfc7858-3.3-5) If the response contains a Question Section, the client MUST match the QNAME, QCLASS, and QTYPE fields. (§3.3) | {gap}, no test | ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client matches the Question Section of a DoT response |
| [`RFC7858-3.4-7`](#rfc7858-3.4-7) As with current DNS over TCP, clients MUST handle abrupt closes and be prepared to reestablish connections and/or retry queries. (§3.4) | {gap}, no test | ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client handles an abrupt DoT close or retries over DoT |
| [`RFC7858-3.4-8`](#rfc7858-3.4-8) when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (§3.4) | {gap}, no test | ze's DoT listener does not use TCP Fast Open: bindDoT opens an ordinary TCP socket through lc.Listen and sets no TCP_FASTOPEN option (internal/core/dnsserver/secure.go), and ze's DNS clients speak no DoT; the obligation holds by construction for the socket ze opens, but no test asserts the listener carries no TCP Fast Open |
| [`RFC7858-3.4-9`](#rfc7858-3.4-9) when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (cleartext DNS MUST NOT be exchanged). (§3.4) | {gap}, no test | ze's DoT listener does not use TCP Fast Open: bindDoT opens an ordinary TCP socket through lc.Listen and sets no TCP_FASTOPEN option (internal/core/dnsserver/secure.go), and ze's DNS clients speak no DoT; the prohibition holds by construction for the socket ze opens, but no test asserts the listener carries no TCP Fast Open |
| [`RFC7858-4.2-4`](#rfc7858-4.2-4) The user MUST be alerted whenever possible that the DNS is not private during such bootstrap. (§4.2) | {gap}, no test | ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client bootstraps a DoT profile or alerts a user about one |
| [`RFC7858-4.2-5`](#rfc7858-4.2-5) Otherwise, the client MUST treat the SPKI validation failure as a non-recoverable error. (§4.2) | {gap}, no test | ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client validates a pinned SPKI |
| [`RFC7858-4.2-6`](#rfc7858-4.2-6) Implementations of this privacy profile MUST support the calculation of a fingerprint as the SHA-256 [RFC6234] hash of the DER-encoded ASN.1 representation of the SPKI of an X.509 certificate. (§4.2) | {gap}, no test | ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client computes an SPKI fingerprint |
| [`RFC7858-4.2-7`](#rfc7858-4.2-7) Implementations MUST support the representation of a SHA-256 fingerprint as a base64-encoded character string [RFC4648]. (§4.2) | {gap}, no test | ze is a DNS client that implements no DoT: the dnsresolver forwarder sends its upstream queries over plain UDP and TCP only (exchange in internal/plugins/dnsresolver/forward.go) and the stub resolver queries over UDP (internal/component/resolve/dns/resolver.go), and ze's only DoT code is the server listener bindDoT (internal/core/dnsserver/secure.go); no owner decision declines a DoT client, so no ze client stores a base64 SPKI pin |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC7858-3.1-1`](#rfc7858-3.1-1)

By default, a DNS server that supports DNS over TLS MUST listen for and accept TCP connections on port 853, unless it has mutual agreement with its clients to use a port other than 853 for DNS over TLS. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-15 (subagent, after 14d1c8f19c and 224b0ef514). The tagged unit changed only in its setup call: mgr.apply/applyListeners became reconcile(..., BindLenient), the policy the deleted wrappers passed; the unit binds one free loopback port, where lenient and strict bind alike, and every assertion line is byte-identical. Where this unit carries a revert record (TestDoTListener, TestDoHListener, TestRFC7858DoTNegotiatesOnlyBCP195Suites, the two RFC 2181 4.1-1 units), it was re-observed red today. Prior judgement re-read against the current body and stands: Re-judged 2026-09-30. Clause 2 now discriminates: TestDefaultSecureConfig compares the default DoT port with the literal 853 (no longer DefaultDoTPort against itself) and a config enabling tls without listen-port parses to 853; clause 1 stays TestDoTListener (listen, accept, answer). Record: DefaultSecureConfig revert observed red. {single-polarity} holds: a listen-and-accept mandate has no refusal path. Edit to a HEAD test adds assertions only (approval D-15).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDefaultSecureConfig`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L379) | unit/verify | revert, verified |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L52) | unit/verify | revert, verified |

### [`RFC7858-3.1-2`](#rfc7858-3.1-2)

By default, a DNS client desiring privacy from DNS over TLS from a particular server MUST establish a TCP connection to port 853 on the server, unless it has mutual agreement with its server to use a port other than port 853 for DNS over TLS. (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-3.1-2, so there is no linked test.

### [`RFC7858-3.1-3`](#rfc7858-3.1-3)

Such another port MUST NOT be port 53 (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-3.1-3, so there is no linked test.

### [`RFC7858-3.1-5`](#rfc7858-3.1-5)

The first data exchange on this TCP connection MUST be the client and server initiating a TLS handshake using the procedure described in [RFC5246]. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-15 (subagent, after 14d1c8f19c and 224b0ef514). The tagged unit changed only in its setup call: mgr.apply/applyListeners became reconcile(..., BindLenient), the policy the deleted wrappers passed; the unit binds one free loopback port, where lenient and strict bind alike, and every assertion line is byte-identical. Where this unit carries a revert record (TestDoTListener, TestDoHListener, TestRFC7858DoTNegotiatesOnlyBCP195Suites, the two RFC 2181 4.1-1 units), it was re-observed red today. Prior judgement re-read against the current body and stands: Forbidden: a server that accepts cleartext DNS as the first data exchange instead of a TLS handshake. TestDoTRefusesCleartext sends a length-prefixed cleartext query over raw TCP and fails at t.Fatalf("server answered a cleartext DNS query on the DoT port") when a DNS reply with the query's Id comes back. Positive: TestDoTListener completes the handshake with a verifying tcp-tls client and asserts the answer. Server role only; the client half is the querying resolver's.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L642) | unit/verify | unproven |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L53) | unit/verify | unproven |

### [`RFC7858-3.1-6`](#rfc7858-3.1-6)

DNS clients and servers MUST NOT use port 853 to transport cleartext DNS messages (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L643) | unit/verify | unproven |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L54) | unit/verify | unproven |

### [`RFC7858-3.1-7`](#rfc7858-3.1-7)

DNS clients MUST NOT send and DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS (including, for example, after a failed TLS handshake). (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-3.1-7, so there is no linked test.

### [`RFC7858-3.1-8`](#rfc7858-3.1-8)

DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS (including, for example, after a failed TLS handshake). (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-15 (subagent, after 14d1c8f19c and 224b0ef514). The tagged unit changed only in its setup call: mgr.apply/applyListeners became reconcile(..., BindLenient), the policy the deleted wrappers passed; the unit binds one free loopback port, where lenient and strict bind alike, and every assertion line is byte-identical. Where this unit carries a revert record (TestDoTListener, TestDoHListener, TestRFC7858DoTNegotiatesOnlyBCP195Suites, the two RFC 2181 4.1-1 units), it was re-observed red today. Prior judgement re-read against the current body and stands: Forbidden: a DNS server responding to a cleartext DNS message on its DoT port, including after the TLS handshake fails. TestDoTRefusesCleartext sends a cleartext query whose bytes fail the TLS handshake and fails at t.Fatalf("server answered a cleartext DNS query on the DoT port") on any unpackable reply with the matching Id. Positive: TestDoTListener asserts the TLS-wrapped query on the same kind of port is answered with the expected A record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L644) | unit/verify | unproven |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L55) | unit/verify | unproven |

### [`RFC7858-3.3-1`](#rfc7858-3.3-1)

All messages (requests and responses) in the established TLS session MUST use the two-octet length field described in Section 4.2.2 of [RFC1035]. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-15 (subagent, after 14d1c8f19c and 224b0ef514). The tagged unit changed only in its setup call: mgr.apply/applyListeners became reconcile(..., BindLenient), the policy the deleted wrappers passed; the unit binds one free loopback port, where lenient and strict bind alike, and every assertion line is byte-identical. Where this unit carries a revert record (TestDoTListener, TestDoHListener, TestRFC7858DoTNegotiatesOnlyBCP195Suites, the two RFC 2181 4.1-1 units), it was re-observed red today. Prior judgement re-read against the current body and stands: Single-polarity positive (row marker). Forbidden: a server response on the TLS session that does not carry the RFC 1035 4.2.2 two-octet length prefix. TestDoTListener uses the miekg tcp-tls client, which reads a two-octet length and then the message; an unprefixed response would be mis-framed and fail at t.Fatalf("DoT exchange") or the answer-count / A-record assertions. The server's reading of prefixed requests is exercised by the same exchange.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L56) | unit/verify | unproven |

### [`RFC7858-3.3-4`](#rfc7858-3.3-4)

Since pipelined responses can arrive out of order, clients MUST match responses to outstanding queries on the same TLS connection using the Message ID. (§3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-3.3-4, so there is no linked test.

### [`RFC7858-3.3-5`](#rfc7858-3.3-5)

If the response contains a Question Section, the client MUST match the QNAME, QCLASS, and QTYPE fields. (§3.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-3.3-5, so there is no linked test.

### [`RFC7858-3.4-5`](#rfc7858-3.4-5)

Clients and servers that keep idle connections open MUST be robust to termination of idle connection by either party. (§3.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30. Both parties now: client-side close (HEAD TestDoTRobustToIdleConnectionClose) and server-side close (TestRFC7858ServerIdleCloseLeavesTheServerServing: EOF within 16 s of the 8 s idle timeout, then a fresh connection answered). Ze runs no DoT client. Record: bindDoT revert observed red. {single-polarity} holds: robustness has no failure input to refuse.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC7858ServerIdleCloseLeavesTheServerServing`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc7858_idle_close_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestDoTRobustToIdleConnectionClose`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L718) | unit/verify | revert, verified |

### [`RFC7858-3.4-7`](#rfc7858-3.4-7)

As with current DNS over TCP, clients MUST handle abrupt closes and be prepared to reestablish connections and/or retry queries. (§3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-3.4-7, so there is no linked test.

### [`RFC7858-3.4-8`](#rfc7858-3.4-8)

when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (§3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-3.4-8, so there is no linked test.

### [`RFC7858-3.4-9`](#rfc7858-3.4-9)

when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (cleartext DNS MUST NOT be exchanged). (§3.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-3.4-9, so there is no linked test.

### [`RFC7858-4.2-4`](#rfc7858-4.2-4)

The user MUST be alerted whenever possible that the DNS is not private during such bootstrap. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-4.2-4, so there is no linked test.

### [`RFC7858-4.2-5`](#rfc7858-4.2-5)

Otherwise, the client MUST treat the SPKI validation failure as a non-recoverable error. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-4.2-5, so there is no linked test.

### [`RFC7858-4.2-6`](#rfc7858-4.2-6)

Implementations of this privacy profile MUST support the calculation of a fingerprint as the SHA-256 [RFC6234] hash of the DER-encoded ASN.1 representation of the SPKI of an X.509 certificate. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-4.2-6, so there is no linked test.

### [`RFC7858-4.2-7`](#rfc7858-4.2-7)

Implementations MUST support the representation of a SHA-256 fingerprint as a base64-encoded character string [RFC4648]. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC7858-4.2-7, so there is no linked test.

### [`RFC7858-8-1`](#rfc7858-8-1)

Clients and servers MUST adhere to the TLS implementation recommendations and security considerations of [BCP195]. (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-15 (subagent, after 14d1c8f19c and 224b0ef514). The tagged unit changed only in its setup call: mgr.apply/applyListeners became reconcile(..., BindLenient), the policy the deleted wrappers passed; the unit binds one free loopback port, where lenient and strict bind alike, and every assertion line is byte-identical. Where this unit carries a revert record (TestDoTListener, TestDoHListener, TestRFC7858DoTNegotiatesOnlyBCP195Suites, the two RFC 2181 4.1-1 units), it was re-observed red today. Prior judgement re-read against the current body and stands: Re-judged 2026-09-30. Beyond the HEAD TLS 1.2 floor: TestRFC7858DoTNegotiatesOnlyBCP195Suites drives the DoT listener built from selfcert.NewTLSConfig (the production constructor) with a hand-built TLS 1.2 ClientHello (internal/test/tlsprobe), ECDSA and RSA certs: ECDHE AES-128-GCM selected with null compression and renegotiation_info, also after every forbidden suite; each NULL/RC4/export/DES/3DES/static-RSA/static-ECDH/DHE suite alone never selected; DEFLATE alone refused. The good suite in the same hello shape is the control, so refusals are about the suite. Records: NewTLSConfig revert both polarities; an overlay adding RC4/3DES/static RSA turned the negative red for 7 suites. Not asserted: RFC 9325 SHOULD-level items outside negotiation (session tickets, EMS).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7858DoTNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc7858_bcp195_test.go#L25) | unit/verify | revert, verified |
| negative | [`TestDoTRejectsBelowTLS12`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L694) | unit/verify | revert, verified |
| positive | [`TestRFC7858DoTNegotiatesOnlyBCP195Suites`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/rfc7858_bcp195_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L57) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc7858.txt |
| Source fingerprint | 2a80bd8127e3e113 |
| Record | rfc/extraction/rfc7858.json |
| Mapped sentences | 17 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 6 | reviewed | not stated |
| `3.2` | not stated | 1 | reviewed | not stated |
| `3.3` | not stated | 3 | reviewed | not stated |
| `3.4` | not stated | 3 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `4.2` | not stated | 4 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 1 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust Legal Provisions boilerplate in the front matter, not a protocol obligation: the lowercase 'must' governs republication of code components under the Simplified BSD License. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `3.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive narration of the handshake sequence with no RFC 2119 keyword; the sentence after it says 'This document does not propose new ideas for authentication', and the authentication policy is set by the privacy profile in Section 4. | The client will then authenticate the server, if required. |

## Superseded

No document obsoletes RFC 7858, so its obligations are stated where they were written.
