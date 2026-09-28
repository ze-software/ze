# RFC 7858 - Specification for DNS over Transport Layer Security (TLS)

Partial. Every requirement this repository extracted from RFC 7858, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 21.1% | 4 of 19 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 15.8% | 3 of 19 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 19 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 12 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 19 | of 38 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 11 | of 19 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 57.9% | 11 of 19 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 19 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 19 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 5.3% | 1 of 19 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 6 | of 19 gated MUSTs judged | 3 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 19 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 38 |
| Gated MUST-level | 19 |
| Not applicable, so out of scope | 11 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tagged units | 12 |
| Recorded audit verdicts | 6 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7858.md` |
| Requirement shard | `rfc/requirements/rfc7858.md` |
| RFC text | `rfc/full/rfc7858.txt` |

## Enrolment

Enrolled: DNS over TLS / DoT (RFC 7858): server role. 4 MET (TLS-first handshake, DoT port refuses cleartext (2 facets), TLS 1.2/BCP 195 floor) + 3 single-polarity positive (listen/accept on 853, RFC 7766 two-octet length framing, robust to idle-connection close) + 1 gap (DoT listen-port 53 not rejected) + 11 not-applicable (9 DoT client-role MUSTs + 2 TCP-Fast-Open MUSTs)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- DoT server on the shared DNS harness: a TLS-wrapped TCP listener (miekg/dns RFC 7766 two-octet length-prefixed framing) on the DoT port, default 853, sharing the same dns.Handler as cleartext and DoH
- TLS 1.2 minimum (BCP 195)
- cleartext queries to the DoT port are refused. Server role only -- ze is not a DoT client.


**What the ledger says remains**

One MUST NOT gap ([`RFC7858-3.1-3`](#rfc7858-3.1-3)): ze does not reject a DoT listen-port of 53 ([`internal/core/dnsserver/secure.go`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure.go) validates only 1..65535). The DoT client-role MUSTs (connect, response-matching, key-pinning, bootstrap alerting) and the TCP-Fast-Open MUSTs are not-applicable -- ze plays no DoT client and its DoT listener uses no TCP Fast Open.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 15 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **19** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC7858-3.1-5`](#rfc7858-3.1-5), [`RFC7858-3.1-6`](#rfc7858-3.1-6), [`RFC7858-3.1-8`](#rfc7858-3.1-8), [`RFC7858-8-1`](#rfc7858-8-1)

**Annotated instead of tested (15):** [`RFC7858-3.1-1`](#rfc7858-3.1-1), [`RFC7858-3.1-2`](#rfc7858-3.1-2), [`RFC7858-3.1-3`](#rfc7858-3.1-3), [`RFC7858-3.1-7`](#rfc7858-3.1-7), [`RFC7858-3.3-1`](#rfc7858-3.3-1), [`RFC7858-3.3-4`](#rfc7858-3.3-4), [`RFC7858-3.3-5`](#rfc7858-3.3-5), [`RFC7858-3.4-5`](#rfc7858-3.4-5), [`RFC7858-3.4-7`](#rfc7858-3.4-7), [`RFC7858-3.4-8`](#rfc7858-3.4-8), [`RFC7858-3.4-9`](#rfc7858-3.4-9), [`RFC7858-4.2-4`](#rfc7858-4.2-4), [`RFC7858-4.2-5`](#rfc7858-4.2-5), [`RFC7858-4.2-6`](#rfc7858-4.2-6), [`RFC7858-4.2-7`](#rfc7858-4.2-7)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7858-3.1-1` | By default, a DNS server that supports DNS over TLS MUST listen for and accept TCP connections on port 853, unless it has mutual agreement with its clients to use a port other than 853 for DNS over TLS. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestDefaultSecureConfig`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L377). **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L52). **negative:** no negative test. **{single-polarity}:** affirmative listen mandate. Positive proof: a DoT listener accepts a TCP+TLS connection and answers (internal/core/dnsserver/secure_test.go TestDoTListener) with the default DoT port pinned to 853 (internal/core/dnsserver/secure.go:39, TestDefaultSecureConfig). A "must listen and accept" obligation has no rejecting counter-behavior to assert as a negative. |
| `RFC7858-3.1-2` | By default, a DNS client desiring privacy from DNS over TLS from a particular server MUST establish a TCP connection to port 853 on the server, unless it has mutual agreement with its server to use a port other than port 853 for DNS over TLS. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a DoT server only, not a DoT client. The querying resolver initiates the connection; ze's only DoT code path is the server listener bindDoT (internal/core/dnsserver/secure.go:307), and dnsserver/client.go is EDNS0 client-subnet resolution (client.go:21), not a DoT client. |
| `RFC7858-3.1-3` | Such another port MUST NOT be port 53 (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not enforce this. ParseSecureLeaves validates the DoT listen-port only as 1..65535 (internal/core/dnsserver/secure.go:166-172) and does not reject 53, so an operator can configure the DoT listener on port 53. |
| `RFC7858-3.1-5` | The first data exchange on this TCP connection MUST be the client and server initiating a TLS handshake using the procedure described in [RFC5246]. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L53). **negative:** `unit/verify` [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L632) |
| `RFC7858-3.1-6` | DNS clients and servers MUST NOT use port 853 to transport cleartext DNS messages (§3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L54). **negative:** `unit/verify` [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L633) |
| `RFC7858-3.1-7` | DNS clients MUST NOT send and DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS (including, for example, after a failed TLS handshake). (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** constrains a DoT client's send behavior; ze is a DoT server only (internal/core/dnsserver/secure.go:307 bindDoT) and issues no DoT queries. The server-side counterpart (do not respond to cleartext on the DoT port) is RFC7858-3.1-8. |
| `RFC7858-3.1-8` | DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS (including, for example, after a failed TLS handshake). (§3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L55). **negative:** `unit/verify` [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L634) |
| `RFC7858-3.3-1` | All messages (requests and responses) in the established TLS session MUST use the two-octet length field described in Section 4.2.2 of [RFC1035]. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L56). **negative:** no negative test. **{single-polarity}:** the RFC 1035 4.2.2 two-octet length-prefixed framing is provided by the miekg/dns Server ze hands the TLS listener to (internal/core/dnsserver/secure.go:314-315). A successful DoT round trip (internal/core/dnsserver/secure_test.go TestDoTListener) proves conformant framing; ze has no code path that emits non-length-prefixed framing to exercise as a negative. |
| `RFC7858-3.3-4` | Since pipelined responses can arrive out of order, clients MUST match responses to outstanding queries on the same TLS connection using the Message ID. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client obligation to match pipelined responses to outstanding queries; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and issues no DoT queries. dnsserver/client.go is EDNS0 client-subnet resolution (client.go:21), not a DoT client. |
| `RFC7858-3.3-5` | If the response contains a Question Section, the client MUST match the QNAME, QCLASS, and QTYPE fields. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client response-matching obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and issues no DoT queries to match responses against. |
| `RFC7858-3.4-5` | Clients and servers that keep idle connections open MUST be robust to termination of idle connection by either party. (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestDoTRobustToIdleConnectionClose`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L708). **negative:** no negative test. **{single-polarity}:** server-side liveness property. Positive proof: after a DoT client abruptly closes an idle connection the server keeps serving and answers a fresh connection (internal/core/dnsserver/secure_test.go TestDoTRobustToIdleConnectionClose). "Remains robust" has no failure polarity to assert as a negative. |
| `RFC7858-3.4-7` | As with current DNS over TCP, clients MUST handle abrupt closes and be prepared to reestablish connections and/or retry queries. (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client reconnect/retry obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and opens no client connections to reestablish. The server-side robustness counterpart is RFC7858-3.4-5. |
| `RFC7858-3.4-8` | when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DoT listener does not use TCP Fast Open. bindDoT opens an ordinary TCP socket via lc.Listen (internal/core/dnsserver/secure.go:308-309) with no TCP_FASTOPEN, so the TFO-specific handshake obligation has no bearing. |
| `RFC7858-3.4-9` | when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (cleartext DNS MUST NOT be exchanged). (§3.4) | MUST NOT | 3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DoT listener does not use TCP Fast Open (internal/core/dnsserver/secure.go:308-309 opens a plain TCP socket, no TCP_FASTOPEN), so the TFO-specific cleartext prohibition has no bearing. |
| `RFC7858-4.2-4` | The user MUST be alerted whenever possible that the DNS is not private during such bootstrap. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client bootstrap-configuration obligation (opportunistic-privacy user alerting); ze is a DoT server only (internal/core/dnsserver/secure.go:307) and performs no client bootstrap. |
| `RFC7858-4.2-5` | Otherwise, the client MUST treat the SPKI validation failure as a non-recoverable error. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client out-of-band key-pinning obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and pins no server SPKI. Client authentication policy is the querying resolver's concern. |
| `RFC7858-4.2-6` | Implementations of this privacy profile MUST support the calculation of a fingerprint as the SHA-256 [RFC6234] hash of the DER-encoded ASN.1 representation of the SPKI of an X.509 certificate. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client key-pinned-profile obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and implements no client-side SPKI pinning profile. |
| `RFC7858-4.2-7` | Implementations MUST support the representation of a SHA-256 fingerprint as a base64-encoded character string [RFC4648]. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client key-pinning representation obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and implements no client-side pin storage. |
| `RFC7858-8-1` | Clients and servers MUST adhere to the TLS implementation recommendations and security considerations of [BCP195]. (§8) | MUST | 8 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L57). **negative:** `unit/verify` [`TestDoTRejectsBelowTLS12`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L684) |
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
| [`RFC7858-3.1-2`](#rfc7858-3.1-2) By default, a DNS client desiring privacy from DNS over TLS from a particular server MUST establish a TCP connection to port 853 on the server, unless it has mutual agreement with its server to use a port other than port 853 for DNS over TLS. (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a DoT server only, not a DoT client. The querying resolver initiates the connection; ze's only DoT code path is the server listener bindDoT (internal/core/dnsserver/secure.go:307), and dnsserver/client.go is EDNS0 client-subnet resolution (client.go:21), not a DoT client. |
| [`RFC7858-3.1-3`](#rfc7858-3.1-3) Such another port MUST NOT be port 53 (§3.1) | {gap}, no test | ze does not enforce this. ParseSecureLeaves validates the DoT listen-port only as 1..65535 (internal/core/dnsserver/secure.go:166-172) and does not reject 53, so an operator can configure the DoT listener on port 53. |
| [`RFC7858-3.1-7`](#rfc7858-3.1-7) DNS clients MUST NOT send and DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS (including, for example, after a failed TLS handshake). (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: constrains a DoT client's send behavior; ze is a DoT server only (internal/core/dnsserver/secure.go:307 bindDoT) and issues no DoT queries. The server-side counterpart (do not respond to cleartext on the DoT port) is RFC7858-3.1-8. |
| [`RFC7858-3.3-4`](#rfc7858-3.3-4) Since pipelined responses can arrive out of order, clients MUST match responses to outstanding queries on the same TLS connection using the Message ID. (§3.3) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client obligation to match pipelined responses to outstanding queries; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and issues no DoT queries. dnsserver/client.go is EDNS0 client-subnet resolution (client.go:21), not a DoT client. |
| [`RFC7858-3.3-5`](#rfc7858-3.3-5) If the response contains a Question Section, the client MUST match the QNAME, QCLASS, and QTYPE fields. (§3.3) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client response-matching obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and issues no DoT queries to match responses against. |
| [`RFC7858-3.4-7`](#rfc7858-3.4-7) As with current DNS over TCP, clients MUST handle abrupt closes and be prepared to reestablish connections and/or retry queries. (§3.4) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client reconnect/retry obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and opens no client connections to reestablish. The server-side robustness counterpart is RFC7858-3.4-5. |
| [`RFC7858-3.4-8`](#rfc7858-3.4-8) when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (§3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DoT listener does not use TCP Fast Open. bindDoT opens an ordinary TCP socket via lc.Listen (internal/core/dnsserver/secure.go:308-309) with no TCP_FASTOPEN, so the TFO-specific handshake obligation has no bearing. |
| [`RFC7858-3.4-9`](#rfc7858-3.4-9) when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (cleartext DNS MUST NOT be exchanged). (§3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DoT listener does not use TCP Fast Open (internal/core/dnsserver/secure.go:308-309 opens a plain TCP socket, no TCP_FASTOPEN), so the TFO-specific cleartext prohibition has no bearing. |
| [`RFC7858-4.2-4`](#rfc7858-4.2-4) The user MUST be alerted whenever possible that the DNS is not private during such bootstrap. (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client bootstrap-configuration obligation (opportunistic-privacy user alerting); ze is a DoT server only (internal/core/dnsserver/secure.go:307) and performs no client bootstrap. |
| [`RFC7858-4.2-5`](#rfc7858-4.2-5) Otherwise, the client MUST treat the SPKI validation failure as a non-recoverable error. (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client out-of-band key-pinning obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and pins no server SPKI. Client authentication policy is the querying resolver's concern. |
| [`RFC7858-4.2-6`](#rfc7858-4.2-6) Implementations of this privacy profile MUST support the calculation of a fingerprint as the SHA-256 [RFC6234] hash of the DER-encoded ASN.1 representation of the SPKI of an X.509 certificate. (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client key-pinned-profile obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and implements no client-side SPKI pinning profile. |
| [`RFC7858-4.2-7`](#rfc7858-4.2-7) Implementations MUST support the representation of a SHA-256 fingerprint as a base64-encoded character string [RFC4648]. (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client key-pinning representation obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and implements no client-side pin storage. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7858-3.1-1`](#rfc7858-3.1-1)

By default, a DNS server that supports DNS over TLS MUST listen for and accept TCP connections on port 853, unless it has mutual agreement with its clients to use a port other than 853 for DNS over TLS. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: (1) listen for and accept TCP connections, (2) on port 853 by default. Clause 1: a server that refused the TLS connection makes secure_test.go TestDoTListener fail at t.Fatalf("DoT exchange"). Clause 2: TestDefaultSecureConfig asserts sc.DoTPort != DefaultDoTPort, comparing the default to its own constant, and TestDoTListener binds a freePort; changing DefaultDoTPort (secure.go:39) to any other value keeps every tagged unit green. No assertion pins the default to 853.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDefaultSecureConfig`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L377) | unit/verify | unproven |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L52) | unit/verify | unproven |

### [`RFC7858-3.1-2`](#rfc7858-3.1-2)

By default, a DNS client desiring privacy from DNS over TLS from a particular server MUST establish a TCP connection to port 853 on the server, unless it has mutual agreement with its server to use a port other than port 853 for DNS over TLS. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.1-2, so no unit is bound to it.

### [`RFC7858-3.1-3`](#rfc7858-3.1-3)

Such another port MUST NOT be port 53 (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.1-3, so no unit is bound to it.

### [`RFC7858-3.1-5`](#rfc7858-3.1-5)

The first data exchange on this TCP connection MUST be the client and server initiating a TLS handshake using the procedure described in [RFC5246]. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a server that accepts cleartext DNS as the first data exchange instead of a TLS handshake. TestDoTRefusesCleartext sends a length-prefixed cleartext query over raw TCP and fails at t.Fatalf("server answered a cleartext DNS query on the DoT port") when a DNS reply with the query's Id comes back. Positive: TestDoTListener completes the handshake with a verifying tcp-tls client and asserts the answer. Server role only; the client half is the querying resolver's.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L632) | unit/verify | unproven |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L53) | unit/verify | unproven |

### [`RFC7858-3.1-6`](#rfc7858-3.1-6)

DNS clients and servers MUST NOT use port 853 to transport cleartext DNS messages (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L633) | unit/verify | unproven |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L54) | unit/verify | unproven |

### [`RFC7858-3.1-7`](#rfc7858-3.1-7)

DNS clients MUST NOT send and DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS (including, for example, after a failed TLS handshake). (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.1-7, so no unit is bound to it.

### [`RFC7858-3.1-8`](#rfc7858-3.1-8)

DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS (including, for example, after a failed TLS handshake). (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a DNS server responding to a cleartext DNS message on its DoT port, including after the TLS handshake fails. TestDoTRefusesCleartext sends a cleartext query whose bytes fail the TLS handshake and fails at t.Fatalf("server answered a cleartext DNS query on the DoT port") on any unpackable reply with the matching Id. Positive: TestDoTListener asserts the TLS-wrapped query on the same kind of port is answered with the expected A record.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L634) | unit/verify | unproven |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L55) | unit/verify | unproven |

### [`RFC7858-3.3-1`](#rfc7858-3.3-1)

All messages (requests and responses) in the established TLS session MUST use the two-octet length field described in Section 4.2.2 of [RFC1035]. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive (row marker). Forbidden: a server response on the TLS session that does not carry the RFC 1035 4.2.2 two-octet length prefix. TestDoTListener uses the miekg tcp-tls client, which reads a two-octet length and then the message; an unprefixed response would be mis-framed and fail at t.Fatalf("DoT exchange") or the answer-count / A-record assertions. The server's reading of prefixed requests is exercised by the same exchange.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L56) | unit/verify | unproven |

### [`RFC7858-3.3-4`](#rfc7858-3.3-4)

Since pipelined responses can arrive out of order, clients MUST match responses to outstanding queries on the same TLS connection using the Message ID. (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.3-4, so no unit is bound to it.

### [`RFC7858-3.3-5`](#rfc7858-3.3-5)

If the response contains a Question Section, the client MUST match the QNAME, QCLASS, and QTYPE fields. (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.3-5, so no unit is bound to it.

### [`RFC7858-3.4-5`](#rfc7858-3.4-5)

Clients and servers that keep idle connections open MUST be robust to termination of idle connection by either party. (§3.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Clauses: robust to termination of an idle connection by (a) the client, (b) the server. (a) TestDoTRobustToIdleConnectionClose closes an idle client connection and fails at t.Fatalf("DoT exchange after idle-connection close") if the server stops answering. (b) No tagged unit makes the server terminate an idle connection (idle timeout or server-side close) and then asserts the server still serves; that half of "either party" has no assertion.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDoTRobustToIdleConnectionClose`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L708) | unit/verify | unproven |

### [`RFC7858-3.4-7`](#rfc7858-3.4-7)

As with current DNS over TCP, clients MUST handle abrupt closes and be prepared to reestablish connections and/or retry queries. (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.4-7, so no unit is bound to it.

### [`RFC7858-3.4-8`](#rfc7858-3.4-8)

when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.4-8, so no unit is bound to it.

### [`RFC7858-3.4-9`](#rfc7858-3.4-9)

when using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (cleartext DNS MUST NOT be exchanged). (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.4-9, so no unit is bound to it.

### [`RFC7858-4.2-4`](#rfc7858-4.2-4)

The user MUST be alerted whenever possible that the DNS is not private during such bootstrap. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-4.2-4, so no unit is bound to it.

### [`RFC7858-4.2-5`](#rfc7858-4.2-5)

Otherwise, the client MUST treat the SPKI validation failure as a non-recoverable error. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-4.2-5, so no unit is bound to it.

### [`RFC7858-4.2-6`](#rfc7858-4.2-6)

Implementations of this privacy profile MUST support the calculation of a fingerprint as the SHA-256 [RFC6234] hash of the DER-encoded ASN.1 representation of the SPKI of an X.509 certificate. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-4.2-6, so no unit is bound to it.

### [`RFC7858-4.2-7`](#rfc7858-4.2-7)

Implementations MUST support the representation of a SHA-256 fingerprint as a base64-encoded character string [RFC4648]. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-4.2-7, so no unit is bound to it.

### [`RFC7858-8-1`](#rfc7858-8-1)

Clients and servers MUST adhere to the TLS implementation recommendations and security considerations of [BCP195]. (§8)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. BCP 195 (RFC 7525) recommendations include the TLS 1.2 floor, no SSL, no NULL / export / RC4 cipher suites, no TLS compression, and secure renegotiation. Only the version floor is asserted: TestDoTRejectsBelowTLS12 fails at t.Fatal("DoT exchange with a TLS 1.1-capped client succeeded") and TestDoTListener proves TLS 1.2 succeeds. No tagged unit offers a forbidden cipher suite (e.g. RC4, NULL, export) or compression and asserts refusal, so a server that negotiated a BCP 195-prohibited suite at TLS 1.2 keeps both units green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoTRejectsBelowTLS12`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L684) | unit/verify | unproven |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L57) | unit/verify | unproven |

## Extraction sign-off

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
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 6 | walked | not stated |
| `3.2` | not stated | 1 | walked | not stated |
| `3.3` | not stated | 3 | walked | not stated |
| `3.4` | not stated | 3 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 4 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust Legal Provisions boilerplate in the front matter, not a protocol obligation: the lowercase 'must' governs republication of code components under the Simplified BSD License. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `3.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive narration of the handshake sequence with no RFC 2119 keyword; the sentence after it says 'This document does not propose new ideas for authentication', and the authentication policy is set by the privacy profile in Section 4. | The client will then authenticate the server, if required. |

## Superseded

No document obsoletes RFC 7858, so its obligations are stated where they were written.
