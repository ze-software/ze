# RFC 8484 - DNS Queries over HTTPS (DoH)

Partial. Every requirement this repository extracted from RFC 8484, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 50.0% | 5 of 10 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 40.0% | 4 of 10 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 10 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 14 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 16 | of 27 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 6 | of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 10.0% | 1 of 10 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 10 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 27 |
| Gated MUST-level | 16 |
| Obligations that bind Ze | 10 |
| Not applicable, so out of scope | 6 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 14 |
| Tagged units | 14 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8484.md` |
| Requirement shard | `rfc/requirements/rfc8484.md` |
| RFC text | `rfc/full/rfc8484.txt` |

## Enrolment

Enrolled: DNS Queries over HTTPS / DoH (RFC 8484): server role. 5 MET (POST+GET, process application/dns-message, media type, base64url dns var, no padding) + 4 single-polarity positive (https-only, freshness<=smallest-TTL, ignore EDNS UDP size, POST body used directly) + 1 gap (NODATA freshness not capped at SOA MINIMUM) + 6 not-applicable (DoH client + media-type-definer roles)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

DoH server on the shared DNS harness: application/dns-message over TLS, POST and GET (base64url-unpadded ?dns=), the /dns-query path, Cache-Control max-age capped at the smallest Answer-section TTL, and 405/415/400 on bad method/media/body. Server role only -- ze is not a DoH client.

**What the ledger says remains**

One MUST gap ([`RFC8484-5.1-2`](#rfc8484-5.1-2)): for a NODATA response minAnswerTTL reads only the Answer section, so the HTTP freshness lifetime is not capped at the Authority SOA MINIMUM field. Six client-role and media-type-definer MUSTs are not-applicable (ze plays no DoH client and defines no media type beyond application/dns-message).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 5 | one part of the gated population |
| Annotated instead of tested | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (5):** [`RFC8484-4.1-2`](#rfc8484-4.1-2), [`RFC8484-4.2-1`](#rfc8484-4.2-1), [`RFC8484-5.4-1`](#rfc8484-5.4-1), [`RFC8484-6-2`](#rfc8484-6-2), [`RFC8484-6-3`](#rfc8484-6-3)

**Annotated instead of tested (11):** [`RFC8484-3-1`](#rfc8484-3-1), [`RFC8484-4.1-1`](#rfc8484-4.1-1), [`RFC8484-4.1-3`](#rfc8484-4.1-3), [`RFC8484-5-1`](#rfc8484-5-1), [`RFC8484-5.1-1`](#rfc8484-5.1-1), [`RFC8484-5.1-2`](#rfc8484-5.1-2), [`RFC8484-5.1-3`](#rfc8484-5.1-3), [`RFC8484-5.3-1`](#rfc8484-5.3-1), [`RFC8484-5.4-2`](#rfc8484-5.4-2), [`RFC8484-6-1`](#rfc8484-6-1), [`RFC8484-6-4`](#rfc8484-6-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8484-3-1` | A DoH client MUST NOT use a different URI simply because it was discovered outside the client's configuration, such as through HTTP/2 server push or a server offering an unsolicited response that appears to be a valid answer to a DNS query (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the DoH client's URI-selection logic, and ze implements no DoH client -- it is an authoritative DoH server only (internal/component/resolve performs no upstream HTTPS resolution) |
| `RFC8484-4.1-1` | `Future specifications` for new media types for DoH MUST define the variables used for URI Template processing with this protocol (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds authors of new DoH media-type specifications, and ze defines no media type beyond application/dns-message (internal/core/dnsserver/secure.go:47) |
| `RFC8484-4.1-2` | DoH servers MUST implement both the POST and GET methods (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L123). **negative:** `unit/verify` [`TestDoHMethodNotAllowed`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L230) |
| `RFC8484-4.1-3` | Irrespective of the value of the Accept request header field, the client MUST be prepared to process "application/dns-message" responses (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds a DoH client preparing to process responses, and ze implements no DoH client (server role only) |
| `RFC8484-4.2-1` | A DoH server MUST be able to process "application/dns-message" request messages (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L140). **negative:** `unit/verify` [`TestDoHRejectsWrongContentType`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L477) |
| `RFC8484-5-1` | This protocol MUST be used with the https URI scheme (§5) | MUST | 5 | **positive:** `unit/verify` [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L124). **negative:** no negative test. **{single-polarity}:** DoH is bound only on a TLS listener and a code guard refuses to start a secure listener without certificate material, so ze serves DoH exclusively over TLS with no cleartext-request rejection to test as a negative (internal/core/dnsserver/secure.go:341, :92-97) |
| `RFC8484-5.1-1` | The assigned freshness lifetime of a DoH HTTP response MUST be less than or equal to the smallest TTL in the Answer section of the DNS response (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestDoHCacheControlMatchesSmallestTTL`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L539). **negative:** no negative test. **{single-polarity}:** the handler sets Cache-Control max-age to exactly the smallest Answer-section TTL, so the assigned freshness equals (hence never exceeds) that minimum (internal/core/dnsserver/secure.go:416-419, :478) |
| `RFC8484-5.1-2` | If the DNS response has no records in the Answer section but has an SOA record in the Authority section, the response freshness lifetime MUST NOT be greater than the MINIMUM field from that SOA record (§5.1) | MUST NOT | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** minAnswerTTL iterates only the Answer section and the handler sets no Cache-Control when that minimum is 0, so a NODATA response's freshness is never capped at the Authority SOA MINIMUM field (internal/core/dnsserver/secure.go:478-488, :416) |
| `RFC8484-5.1-3` | DoH clients MUST account for the Age response header field's value when calculating the DNS TTL of a response (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds a DoH client's TTL computation using the Age response header, and ze implements no DoH client |
| `RFC8484-5.3-1` | Before using DoH response data for DNS resolution, the client MUST establish that the HTTP request URI can be used for the DoH query (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds a DoH client's URI validation before trusting a response, and ze implements no DoH client |
| `RFC8484-5.4-1` | DoH clients and DoH servers MUST support the "application/dns-message" media type (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`assertDoHAnswer`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L176). **negative:** `unit/verify` [`TestDoHRejectsWrongContentType`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L478) |
| `RFC8484-5.4-2` | Other media types MUST be flexible enough to express every DNS query that would normally be sent in DNS over UDP, including queries and responses that use DNS extensions but not those that require multiple responses (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds definers of media types other than application/dns-message, and ze offers only application/dns-message (internal/core/dnsserver/secure.go:47) |
| `RFC8484-6-1` | DoH servers using this media type MUST ignore the value given for the EDNS UDP payload size in DNS requests (§6) | MUST | 6 | **positive:** `unit/verify` [`TestDoHIgnoresEDNSUDPSize`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L564). **negative:** no negative test. **{single-polarity}:** the handler packs the full reply and writes it over HTTP framing, never consulting the request's EDNS UDP payload size and never UDP-truncating, so the size is ignored by construction (internal/core/dnsserver/secure.go:410-422) |
| `RFC8484-6-2` | When using the GET method, the data payload for this media type MUST be encoded with base64url and provided as a variable named "dns" to the URI Template expansion (§6) | MUST | 6 | **positive:** `unit/verify` [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L156). **negative:** `unit/verify` [`TestDoHGetRejectsBadDNSParam`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L491) |
| `RFC8484-6-3` | Padding characters for base64url MUST NOT be included (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L157). **negative:** `unit/verify` [`TestDoHGetRejectsPaddedDNSParam`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L518) |
| `RFC8484-6-4` | When using the POST method, the data payload for this media type MUST NOT be encoded and is used directly as the HTTP message body (§6) | MUST NOT | 6 | **positive:** `unit/verify` [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L141). **negative:** no negative test. **{single-polarity}:** the POST branch reads the body raw and the handler unpacks it directly as the wire DNS message with no decoding step, so the only meaningful assertion is that the raw body is used directly (internal/core/dnsserver/secure.go:451, :396) |
| `RFC8484-4.1-4` | The DoH client SHOULD include an HTTP Accept request header field to indicate what type of content can be understood in response (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8484-4.1-5` | DoH clients using media formats that include the ID field from the DNS message header, such as "application/dns-message", SHOULD use a DNS ID of 0 in every DNS request (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8484-5.1-4` | DoH servers SHOULD assign an explicit HTTP freshness lifetime so that the DoH client is more likely to use fresh DNS data (§5.1) | SHOULD | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8484-5.1-5` | A freshness lifetime equal to the smallest TTL in the Answer section is RECOMMENDED (§5.1) | RECOMMENDED | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8484-5.2-1` | HTTP/2 is the minimum RECOMMENDED version of HTTP for use with DoH (§5.2) | RECOMMENDED | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8484-8.2-1` | HTTP cookies SHOULD NOT be accepted by DoH clients unless they are explicitly required by a use case (§8.2) | SHOULD NOT | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC8484-10-1` | The authentication given DoH servers SHOULD NOT rely on DNS-based references to external resources in the TLS handshake (§10) | SHOULD NOT | 10 | **positive:** no positive test. **negative:** no negative test |
| `RFC8484-3-2` | DoH servers MAY support more than one URI Template (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8484-4.1-6` | The client MAY also process other DNS-related media types it receives (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8484-5.4-3` | Other media types MAY be used as defined by HTTP Content Negotiation (§5.4) | MAY | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC8484-6-5` | DoH clients using this media type MAY include one or more Extension Mechanisms for DNS EDNS options in the request (§6) | MAY | 6 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8484-3-1`](#rfc8484-3-1) A DoH client MUST NOT use a different URI simply because it was discovered outside the client's configuration, such as through HTTP/2 server push or a server offering an unsolicited response that appears to be a valid answer to a DNS query (§3) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the DoH client's URI-selection logic, and ze implements no DoH client -- it is an authoritative DoH server only (internal/component/resolve performs no upstream HTTPS resolution) |
| [`RFC8484-4.1-1`](#rfc8484-4.1-1) `Future specifications` for new media types for DoH MUST define the variables used for URI Template processing with this protocol (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: this binds authors of new DoH media-type specifications, and ze defines no media type beyond application/dns-message (internal/core/dnsserver/secure.go:47) |
| [`RFC8484-4.1-3`](#rfc8484-4.1-3) Irrespective of the value of the Accept request header field, the client MUST be prepared to process "application/dns-message" responses (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: this binds a DoH client preparing to process responses, and ze implements no DoH client (server role only) |
| [`RFC8484-5.1-2`](#rfc8484-5.1-2) If the DNS response has no records in the Answer section but has an SOA record in the Authority section, the response freshness lifetime MUST NOT be greater than the MINIMUM field from that SOA record (§5.1) | {gap}, no test | minAnswerTTL iterates only the Answer section and the handler sets no Cache-Control when that minimum is 0, so a NODATA response's freshness is never capped at the Authority SOA MINIMUM field (internal/core/dnsserver/secure.go:478-488, :416) |
| [`RFC8484-5.1-3`](#rfc8484-5.1-3) DoH clients MUST account for the Age response header field's value when calculating the DNS TTL of a response (§5.1) | no test | no test carries this requirement id; annotated {not-applicable}: this binds a DoH client's TTL computation using the Age response header, and ze implements no DoH client |
| [`RFC8484-5.3-1`](#rfc8484-5.3-1) Before using DoH response data for DNS resolution, the client MUST establish that the HTTP request URI can be used for the DoH query (§5.3) | no test | no test carries this requirement id; annotated {not-applicable}: this binds a DoH client's URI validation before trusting a response, and ze implements no DoH client |
| [`RFC8484-5.4-2`](#rfc8484-5.4-2) Other media types MUST be flexible enough to express every DNS query that would normally be sent in DNS over UDP, including queries and responses that use DNS extensions but not those that require multiple responses (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: this binds definers of media types other than application/dns-message, and ze offers only application/dns-message (internal/core/dnsserver/secure.go:47) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8484-3-1`](#rfc8484-3-1)

A DoH client MUST NOT use a different URI simply because it was discovered outside the client's configuration, such as through HTTP/2 server push or a server offering an unsolicited response that appears to be a valid answer to a DNS query (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8484-3-1, so no unit is bound to it.

### [`RFC8484-4.1-1`](#rfc8484-4.1-1)

`Future specifications` for new media types for DoH MUST define the variables used for URI Template processing with this protocol (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8484-4.1-1, so no unit is bound to it.

### [`RFC8484-4.1-2`](#rfc8484-4.1-2)

DoH servers MUST implement both the POST and GET methods (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoHMethodNotAllowed`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L230) | unit/verify | unproven |
| positive | [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L123) | unit/verify | unproven |

### [`RFC8484-4.1-3`](#rfc8484-4.1-3)

Irrespective of the value of the Accept request header field, the client MUST be prepared to process "application/dns-message" responses (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8484-4.1-3, so no unit is bound to it.

### [`RFC8484-4.2-1`](#rfc8484-4.2-1)

A DoH server MUST be able to process "application/dns-message" request messages (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoHRejectsWrongContentType`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L477) | unit/verify | unproven |
| positive | [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L140) | unit/verify | unproven |

### [`RFC8484-5-1`](#rfc8484-5-1)

This protocol MUST be used with the https URI scheme (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L124) | unit/verify | unproven |

### [`RFC8484-5.1-1`](#rfc8484-5.1-1)

The assigned freshness lifetime of a DoH HTTP response MUST be less than or equal to the smallest TTL in the Answer section of the DNS response (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDoHCacheControlMatchesSmallestTTL`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L539) | unit/verify | unproven |

### [`RFC8484-5.1-2`](#rfc8484-5.1-2)

If the DNS response has no records in the Answer section but has an SOA record in the Authority section, the response freshness lifetime MUST NOT be greater than the MINIMUM field from that SOA record (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8484-5.1-2, so no unit is bound to it.

### [`RFC8484-5.1-3`](#rfc8484-5.1-3)

DoH clients MUST account for the Age response header field's value when calculating the DNS TTL of a response (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8484-5.1-3, so no unit is bound to it.

### [`RFC8484-5.3-1`](#rfc8484-5.3-1)

Before using DoH response data for DNS resolution, the client MUST establish that the HTTP request URI can be used for the DoH query (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8484-5.3-1, so no unit is bound to it.

### [`RFC8484-5.4-1`](#rfc8484-5.4-1)

DoH clients and DoH servers MUST support the "application/dns-message" media type (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoHRejectsWrongContentType`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L478) | unit/verify | unproven |
| positive | [`assertDoHAnswer`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L176) | unit/verify | unproven |

### [`RFC8484-5.4-2`](#rfc8484-5.4-2)

Other media types MUST be flexible enough to express every DNS query that would normally be sent in DNS over UDP, including queries and responses that use DNS extensions but not those that require multiple responses (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8484-5.4-2, so no unit is bound to it.

### [`RFC8484-6-1`](#rfc8484-6-1)

DoH servers using this media type MUST ignore the value given for the EDNS UDP payload size in DNS requests (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDoHIgnoresEDNSUDPSize`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L564) | unit/verify | unproven |

### [`RFC8484-6-2`](#rfc8484-6-2)

When using the GET method, the data payload for this media type MUST be encoded with base64url and provided as a variable named "dns" to the URI Template expansion (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoHGetRejectsBadDNSParam`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L491) | unit/verify | unproven |
| positive | [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L156) | unit/verify | unproven |

### [`RFC8484-6-3`](#rfc8484-6-3)

Padding characters for base64url MUST NOT be included (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDoHGetRejectsPaddedDNSParam`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L518) | unit/verify | unproven |
| positive | [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L157) | unit/verify | unproven |

### [`RFC8484-6-4`](#rfc8484-6-4)

When using the POST method, the data payload for this media type MUST NOT be encoded and is used directly as the HTTP message body (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDoHListener`](https://github.com/ze-software/ze/blob/main/internal/core/dnsserver/secure_test.go#L141) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 8484, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 8484, so its obligations are stated where they were written.
