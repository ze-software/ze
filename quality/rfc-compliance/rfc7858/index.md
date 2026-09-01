# RFC 7858 - Specification for DNS over Transport Layer Security (TLS)

Partial. Every requirement this repository extracted from RFC 7858, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50.0% | 4 of 8 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 37.5% | 3 of 8 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 12 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 19 | of 38 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 11 | of 19 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 12.5% | 1 of 8 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 8 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 38 |
| Gated MUST-level | 19 |
| Obligations that bind Ze | 8 |
| Not applicable, so out of scope | 11 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 12 |
| Tagged units | 12 |
| Recorded audit verdicts | 0 |
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
| **Gated MUST-level requirements** | **19** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC7858-3.1-5`](#rfc7858-3.1-5), [`RFC7858-3.1-6`](#rfc7858-3.1-6), [`RFC7858-3.1-8`](#rfc7858-3.1-8), [`RFC7858-8-1`](#rfc7858-8-1)

**Annotated instead of tested (15):** [`RFC7858-3.1-1`](#rfc7858-3.1-1), [`RFC7858-3.1-2`](#rfc7858-3.1-2), [`RFC7858-3.1-3`](#rfc7858-3.1-3), [`RFC7858-3.1-7`](#rfc7858-3.1-7), [`RFC7858-3.3-1`](#rfc7858-3.3-1), [`RFC7858-3.3-4`](#rfc7858-3.3-4), [`RFC7858-3.3-5`](#rfc7858-3.3-5), [`RFC7858-3.4-5`](#rfc7858-3.4-5), [`RFC7858-3.4-7`](#rfc7858-3.4-7), [`RFC7858-3.4-8`](#rfc7858-3.4-8), [`RFC7858-3.4-9`](#rfc7858-3.4-9), [`RFC7858-4.2-4`](#rfc7858-4.2-4), [`RFC7858-4.2-5`](#rfc7858-4.2-5), [`RFC7858-4.2-6`](#rfc7858-4.2-6), [`RFC7858-4.2-7`](#rfc7858-4.2-7)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7858-3.1-1` | By default, a DNS server that supports DNS over TLS MUST listen for and accept TCP connections on port 853, unless it has mutual agreement with its clients to use another port (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestDefaultSecureConfig`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L377). **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L52). **negative:** no negative test. **{single-polarity}:** affirmative listen mandate. Positive proof: a DoT listener accepts a TCP+TLS connection and answers (internal/core/dnsserver/secure_test.go TestDoTListener) with the default DoT port pinned to 853 (internal/core/dnsserver/secure.go:39, TestDefaultSecureConfig). A "must listen and accept" obligation has no rejecting counter-behavior to assert as a negative. |
| `RFC7858-3.1-2` | By default, a DNS client desiring privacy MUST establish a TCP connection to port 853 on the server, unless it has mutual agreement to use another port (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is a DoT server only, not a DoT client. The querying resolver initiates the connection; ze's only DoT code path is the server listener bindDoT (internal/core/dnsserver/secure.go:307), and dnsserver/client.go is EDNS0 client-subnet resolution (client.go:21), not a DoT client. |
| `RFC7858-3.1-3` | A mutually agreed alternative port MUST NOT be port 53 (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not enforce this. ParseSecureLeaves validates the DoT listen-port only as 1..65535 (internal/core/dnsserver/secure.go:166-172) and does not reject 53, so an operator can configure the DoT listener on port 53. |
| `RFC7858-3.1-5` | The first data exchange on the TCP connection MUST be the client and server initiating a TLS handshake per RFC 5246 (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L53). **negative:** `unit/verify` [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L632) |
| `RFC7858-3.1-6` | DNS clients and servers MUST NOT use port 853 to transport cleartext DNS messages (§3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L54). **negative:** `unit/verify` [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L633) |
| `RFC7858-3.1-7` | DNS clients MUST NOT send cleartext DNS messages on any port used for DNS over TLS, including after a failed TLS handshake (§3.1) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** constrains a DoT client's send behavior; ze is a DoT server only (internal/core/dnsserver/secure.go:307 bindDoT) and issues no DoT queries. The server-side counterpart (do not respond to cleartext on the DoT port) is RFC7858-3.1-8. |
| `RFC7858-3.1-8` | DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS, including after a failed TLS handshake (§3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L55). **negative:** `unit/verify` [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L634) |
| `RFC7858-3.3-1` | All messages (requests and responses) in the established TLS session MUST use the two-octet length field described in Section 4.2.2 of RFC 1035 (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L56). **negative:** no negative test. **{single-polarity}:** the RFC 1035 4.2.2 two-octet length-prefixed framing is provided by the miekg/dns Server ze hands the TLS listener to (internal/core/dnsserver/secure.go:314-315). A successful DoT round trip (internal/core/dnsserver/secure_test.go TestDoTListener) proves conformant framing; ze has no code path that emits non-length-prefixed framing to exercise as a negative. |
| `RFC7858-3.3-4` | Clients MUST match pipelined responses to outstanding queries on the same TLS connection using the Message ID (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client obligation to match pipelined responses to outstanding queries; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and issues no DoT queries. dnsserver/client.go is EDNS0 client-subnet resolution (client.go:21), not a DoT client. |
| `RFC7858-3.3-5` | If a response contains a Question Section, the client MUST match the QNAME, QCLASS, and QTYPE fields (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client response-matching obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and issues no DoT queries to match responses against. |
| `RFC7858-3.4-5` | Clients and servers that keep idle connections open MUST be robust to termination of an idle connection by either party (§3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestDoTRobustToIdleConnectionClose`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L708). **negative:** no negative test. **{single-polarity}:** server-side liveness property. Positive proof: after a DoT client abruptly closes an idle connection the server keeps serving and answers a fresh connection (internal/core/dnsserver/secure_test.go TestDoTRobustToIdleConnectionClose). "Remains robust" has no failure polarity to assert as a negative. |
| `RFC7858-3.4-7` | Clients MUST handle abrupt closes and be prepared to reestablish connections and/or retry queries (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client reconnect/retry obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and opens no client connections to reestablish. The server-side robustness counterpart is RFC7858-3.4-5. |
| `RFC7858-3.4-8` | When using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DoT listener does not use TCP Fast Open. bindDoT opens an ordinary TCP socket via lc.Listen (internal/core/dnsserver/secure.go:308-309) with no TCP_FASTOPEN, so the TFO-specific handshake obligation has no bearing. |
| `RFC7858-3.4-9` | When using TCP Fast Open, cleartext DNS MUST NOT be exchanged (§3.4) | MUST NOT | 3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DoT listener does not use TCP Fast Open (internal/core/dnsserver/secure.go:308-309 opens a plain TCP socket, no TCP_FASTOPEN), so the TFO-specific cleartext prohibition has no bearing. |
| `RFC7858-4.2-4` | The user MUST be alerted whenever possible that DNS is not private during such bootstrap network configuration (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client bootstrap-configuration obligation (opportunistic-privacy user alerting); ze is a DoT server only (internal/core/dnsserver/secure.go:307) and performs no client bootstrap. |
| `RFC7858-4.2-5` | If no computed fingerprint matches a configured pin, the client MUST treat the SPKI validation failure as a non-recoverable error (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client out-of-band key-pinning obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and pins no server SPKI. Client authentication policy is the querying resolver's concern. |
| `RFC7858-4.2-6` | Implementations of the key-pinned profile MUST support computing a fingerprint as the SHA-256 hash of the DER-encoded ASN.1 SubjectPublicKeyInfo of an X.509 certificate (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client key-pinned-profile obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and implements no client-side SPKI pinning profile. |
| `RFC7858-4.2-7` | Implementations MUST support representing a SHA-256 fingerprint as a base64-encoded character string (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** a DoT client key-pinning representation obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and implements no client-side pin storage. |
| `RFC7858-8-1` | Clients and servers MUST adhere to the TLS implementation recommendations and security considerations of BCP 195 (§8) | MUST | 8 | **positive:** `unit/verify` [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L57). **negative:** `unit/verify` [`TestDoTRejectsBelowTLS12`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L684) |
| `RFC7858-3.1-9` | DNS clients SHOULD remember server IP addresses that do not support DNS over TLS (timeouts, connection refusals, TLS handshake failures) and not request DNS over TLS from them for a reasonable period (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.3-2` | DNS clients and servers SHOULD pass the two-octet length field and the message it describes to the TCP layer at the same time, e.g. in a single write system call (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.3-3` | Clients SHOULD pipeline multiple queries over a TLS session (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-1` | Clients SHOULD reuse a single TCP connection to the recursive resolver (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-2` | Clients and servers SHOULD NOT immediately close a connection after each response (§3.4) | SHOULD NOT | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-3` | Clients and servers SHOULD reuse existing connections for subsequent queries as long as they have sufficient resources (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-4` | An implementor of DNS over TLS SHOULD follow best practices for DNS over TCP per RFC 7766 (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-10` | DNS servers SHOULD enable fast TLS session resumption (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-11` | Fast TLS session resumption SHOULD be used when reestablishing connections (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-12` | When closing a connection, DNS servers SHOULD use the TLS close-notify to shift the TCP TIME-WAIT state to the client (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-4.2-1` | Client administrators SHOULD deploy a backup pin along with the primary pin (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-4.2-2` | After a change of keys on the server, an updated pin set SHOULD be distributed to all clients in some secure way (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-5-1` | To minimize server state and connection startup time, clients SHOULD minimize the creation of new TCP connections (§5) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.1-4` | A mutually agreed alternative port MAY be from the "first-come, first-served" port range (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.1-10` | DNS clients following an out-of-band key-pinned privacy profile MAY be more aggressive about retrying DNS-over-TLS connection failures (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-3.4-6` | As with current DNS over TCP, DNS servers MAY close the connection at any time (§3.4) | MAY | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-4.2-3` | Techniques such as those used by DNSSEC-trigger MAY be used during network configuration, transitioning to the designated DNS provider after authentication (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-4.2-8` | Additional fingerprint types MAY also be supported (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7858-8-2` | Clients MAY discard cached information about server capabilities advertised in cleartext (§8) | MAY | 8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7858-3.1-2`](#rfc7858-3.1-2) By default, a DNS client desiring privacy MUST establish a TCP connection to port 853 on the server, unless it has mutual agreement to use another port (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze is a DoT server only, not a DoT client. The querying resolver initiates the connection; ze's only DoT code path is the server listener bindDoT (internal/core/dnsserver/secure.go:307), and dnsserver/client.go is EDNS0 client-subnet resolution (client.go:21), not a DoT client. |
| [`RFC7858-3.1-3`](#rfc7858-3.1-3) A mutually agreed alternative port MUST NOT be port 53 (§3.1) | {gap}, no test | ze does not enforce this. ParseSecureLeaves validates the DoT listen-port only as 1..65535 (internal/core/dnsserver/secure.go:166-172) and does not reject 53, so an operator can configure the DoT listener on port 53. |
| [`RFC7858-3.1-7`](#rfc7858-3.1-7) DNS clients MUST NOT send cleartext DNS messages on any port used for DNS over TLS, including after a failed TLS handshake (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: constrains a DoT client's send behavior; ze is a DoT server only (internal/core/dnsserver/secure.go:307 bindDoT) and issues no DoT queries. The server-side counterpart (do not respond to cleartext on the DoT port) is RFC7858-3.1-8. |
| [`RFC7858-3.3-4`](#rfc7858-3.3-4) Clients MUST match pipelined responses to outstanding queries on the same TLS connection using the Message ID (§3.3) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client obligation to match pipelined responses to outstanding queries; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and issues no DoT queries. dnsserver/client.go is EDNS0 client-subnet resolution (client.go:21), not a DoT client. |
| [`RFC7858-3.3-5`](#rfc7858-3.3-5) If a response contains a Question Section, the client MUST match the QNAME, QCLASS, and QTYPE fields (§3.3) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client response-matching obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and issues no DoT queries to match responses against. |
| [`RFC7858-3.4-7`](#rfc7858-3.4-7) Clients MUST handle abrupt closes and be prepared to reestablish connections and/or retry queries (§3.4) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client reconnect/retry obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and opens no client connections to reestablish. The server-side robustness counterpart is RFC7858-3.4-5. |
| [`RFC7858-3.4-8`](#rfc7858-3.4-8) When using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (§3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DoT listener does not use TCP Fast Open. bindDoT opens an ordinary TCP socket via lc.Listen (internal/core/dnsserver/secure.go:308-309) with no TCP_FASTOPEN, so the TFO-specific handshake obligation has no bearing. |
| [`RFC7858-3.4-9`](#rfc7858-3.4-9) When using TCP Fast Open, cleartext DNS MUST NOT be exchanged (§3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DoT listener does not use TCP Fast Open (internal/core/dnsserver/secure.go:308-309 opens a plain TCP socket, no TCP_FASTOPEN), so the TFO-specific cleartext prohibition has no bearing. |
| [`RFC7858-4.2-4`](#rfc7858-4.2-4) The user MUST be alerted whenever possible that DNS is not private during such bootstrap network configuration (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client bootstrap-configuration obligation (opportunistic-privacy user alerting); ze is a DoT server only (internal/core/dnsserver/secure.go:307) and performs no client bootstrap. |
| [`RFC7858-4.2-5`](#rfc7858-4.2-5) If no computed fingerprint matches a configured pin, the client MUST treat the SPKI validation failure as a non-recoverable error (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client out-of-band key-pinning obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and pins no server SPKI. Client authentication policy is the querying resolver's concern. |
| [`RFC7858-4.2-6`](#rfc7858-4.2-6) Implementations of the key-pinned profile MUST support computing a fingerprint as the SHA-256 hash of the DER-encoded ASN.1 SubjectPublicKeyInfo of an X.509 certificate (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client key-pinned-profile obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and implements no client-side SPKI pinning profile. |
| [`RFC7858-4.2-7`](#rfc7858-4.2-7) Implementations MUST support representing a SHA-256 fingerprint as a base64-encoded character string (§4.2) | no test | no test carries this requirement id; annotated {not-applicable}: a DoT client key-pinning representation obligation; ze is a DoT server only (internal/core/dnsserver/secure.go:307) and implements no client-side pin storage. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7858-3.1-1`](#rfc7858-3.1-1)

By default, a DNS server that supports DNS over TLS MUST listen for and accept TCP connections on port 853, unless it has mutual agreement with its clients to use another port (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDefaultSecureConfig`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L377) | unit/verify | unproven |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L52) | unit/verify | unproven |

### [`RFC7858-3.1-2`](#rfc7858-3.1-2)

By default, a DNS client desiring privacy MUST establish a TCP connection to port 853 on the server, unless it has mutual agreement to use another port (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.1-2, so no unit is bound to it.

### [`RFC7858-3.1-3`](#rfc7858-3.1-3)

A mutually agreed alternative port MUST NOT be port 53 (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.1-3, so no unit is bound to it.

### [`RFC7858-3.1-5`](#rfc7858-3.1-5)

The first data exchange on the TCP connection MUST be the client and server initiating a TLS handshake per RFC 5246 (§3.1)

Audit verdict: not audited: no reader has judged these tests

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

DNS clients MUST NOT send cleartext DNS messages on any port used for DNS over TLS, including after a failed TLS handshake (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.1-7, so no unit is bound to it.

### [`RFC7858-3.1-8`](#rfc7858-3.1-8)

DNS servers MUST NOT respond to cleartext DNS messages on any port used for DNS over TLS, including after a failed TLS handshake (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoTRefusesCleartext`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L634) | unit/verify | unproven |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L55) | unit/verify | unproven |

### [`RFC7858-3.3-1`](#rfc7858-3.3-1)

All messages (requests and responses) in the established TLS session MUST use the two-octet length field described in Section 4.2.2 of RFC 1035 (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L56) | unit/verify | unproven |

### [`RFC7858-3.3-4`](#rfc7858-3.3-4)

Clients MUST match pipelined responses to outstanding queries on the same TLS connection using the Message ID (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.3-4, so no unit is bound to it.

### [`RFC7858-3.3-5`](#rfc7858-3.3-5)

If a response contains a Question Section, the client MUST match the QNAME, QCLASS, and QTYPE fields (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.3-5, so no unit is bound to it.

### [`RFC7858-3.4-5`](#rfc7858-3.4-5)

Clients and servers that keep idle connections open MUST be robust to termination of an idle connection by either party (§3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDoTRobustToIdleConnectionClose`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L708) | unit/verify | unproven |

### [`RFC7858-3.4-7`](#rfc7858-3.4-7)

Clients MUST handle abrupt closes and be prepared to reestablish connections and/or retry queries (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.4-7, so no unit is bound to it.

### [`RFC7858-3.4-8`](#rfc7858-3.4-8)

When using TCP Fast Open, the client and server MUST immediately initiate or resume a TLS handshake (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.4-8, so no unit is bound to it.

### [`RFC7858-3.4-9`](#rfc7858-3.4-9)

When using TCP Fast Open, cleartext DNS MUST NOT be exchanged (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-3.4-9, so no unit is bound to it.

### [`RFC7858-4.2-4`](#rfc7858-4.2-4)

The user MUST be alerted whenever possible that DNS is not private during such bootstrap network configuration (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-4.2-4, so no unit is bound to it.

### [`RFC7858-4.2-5`](#rfc7858-4.2-5)

If no computed fingerprint matches a configured pin, the client MUST treat the SPKI validation failure as a non-recoverable error (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-4.2-5, so no unit is bound to it.

### [`RFC7858-4.2-6`](#rfc7858-4.2-6)

Implementations of the key-pinned profile MUST support computing a fingerprint as the SHA-256 hash of the DER-encoded ASN.1 SubjectPublicKeyInfo of an X.509 certificate (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-4.2-6, so no unit is bound to it.

### [`RFC7858-4.2-7`](#rfc7858-4.2-7)

Implementations MUST support representing a SHA-256 fingerprint as a base64-encoded character string (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7858-4.2-7, so no unit is bound to it.

### [`RFC7858-8-1`](#rfc7858-8-1)

Clients and servers MUST adhere to the TLS implementation recommendations and security considerations of BCP 195 (§8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoTRejectsBelowTLS12`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L684) | unit/verify | unproven |
| positive | [`TestDoTListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L57) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 7858, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7858, so its obligations are stated where they were written.
