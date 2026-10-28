# RFC 8522 - Looking Glass Command Set

Unsupported. Every requirement this repository extracted from RFC 8522, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 18 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 18 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 18 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 18 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 18 | of 23 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 18 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 18 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 18 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 18 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 18 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 18 of 18 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 18 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Unsupported |
| Enrolment | Enrolled |
| Requirements | 23 |
| Gated MUST-level | 18 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 18 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc8522.md` |
| Requirement shard | `rfc/requirements/rfc8522.md` |
| RFC text | `rfc/full/rfc8522.txt` |

## Enrolment

Enrolled: The document cites neither RFC 2119 nor BCP 14 and uses no capitalised keyword, but it specifies an HTTP API with obligations in lower-case "must" and "mandatory" sentences, so it is walked at register prose and its server obligations are gated at MUST level. 18 gated rows: 16 sourced from a "must" site and two unsourced ("Implementation of the ping command is mandatory." in Section 3.1.1 and the same for "show route" in Section 3.2.1, which the modal inventory cannot see). All 18 are `{gap}`: Ze's looking glass serves its own UI and a birdwatcher-compatible API under /api/looking-glass/ (internal/component/lg/server.go::registerRoutes) and implements no /.well-known/looking-glass/v1 endpoint, no JSend response envelope and none of the RFC 8522 functions. Site 2:1 binds the API client and is excluded as binds-another-role. The walk is recorded in rfc/extraction/rfc8522.json at register prose. Enrolled 2026-10-26.

## What the public ledger says

**Status:** Unsupported

**What the ledger says is covered:**

Ze ships a looking glass with a web UI and a birdwatcher-compatible JSON API under `/api/looking-glass/` ([`internal/component/lg/server.go`](https://github.com/ze-software/ze/blob/main/internal/component/lg/server.go)::registerRoutes). It does not implement the RFC 8522 API.

**What the ledger says remains**

Every RFC 8522 obligation is a gap: the `/.well-known/looking-glass/v1` URI space, the query parameters, the JSend `success`/`fail`/`error` envelope with `performed_at` and `runtime`, the mandatory `ping` and `show route` functions, the mandatory organizational commands (`routers`, router details, `cmd`), and HTTPS for the API.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 18 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **18** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (18):** [`RFC8522-2-1`](#rfc8522-2-1), [`RFC8522-2-2`](#rfc8522-2-2), [`RFC8522-2-3`](#rfc8522-2-3), [`RFC8522-2.2-1`](#rfc8522-2.2-1), [`RFC8522-2.3-1`](#rfc8522-2.3-1), [`RFC8522-2.3-2`](#rfc8522-2.3-2), [`RFC8522-2.3.1-1`](#rfc8522-2.3.1-1), [`RFC8522-2.3.1-2`](#rfc8522-2.3.1-2), [`RFC8522-2.3.1-3`](#rfc8522-2.3.1-3), [`RFC8522-2.3.1-4`](#rfc8522-2.3.1-4), [`RFC8522-2.3.1-5`](#rfc8522-2.3.1-5), [`RFC8522-2.3.2-1`](#rfc8522-2.3.2-1), [`RFC8522-2.3.3-1`](#rfc8522-2.3.3-1), [`RFC8522-3.1.1-1`](#rfc8522-3.1.1-1), [`RFC8522-3.2.1-1`](#rfc8522-3.2.1-1), [`RFC8522-3.3-1`](#rfc8522-3.3-1), [`RFC8522-3.3.2-1`](#rfc8522-3.3.2-1), [`RFC8522-6-1`](#rfc8522-6-1)

## Attainable ceiling

Mandatory ceiling: 18 of 18 (100.0%). Optional ceiling: 5 of 5 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc8522 18 of 18 (100.0%). Lowest optional ceiling: rfc8522 5 of 5 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC8522-2-1` | The path component of the resource URI must use the prefix of ".well- known/looking-glass" (see Section 5.1) to stay namespace neutral. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze's looking glass registers its API under /api/looking-glass/ and nothing under /.well-known/looking-glass -- internal/component/lg/server.go::registerRoutes |
| `RFC8522-2-2` | The client must not be able to execute random commands on the targeting router. (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze implements no RFC 8522 API, so no RFC 8522 call exists to restrict; Ze's own looking glass serves fixed handlers only -- internal/component/lg/server.go::registerRoutes |
| `RFC8522-2-3` | There must not be any direct communication between the client and the router. (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze implements no RFC 8522 API -- internal/component/lg/server.go::registerRoutes |
| `RFC8522-2.2-1` | The server must ignore its value. (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no RFC 8522 query parameter is parsed, `random` included -- internal/component/lg/server.go::registerRoutes |
| `RFC8522-2.3-1` | The response text must comply with the JSON syntax specification JSend, which is briefly explained in Appendix A. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze's looking-glass API answers in the birdwatcher shape, not JSend -- internal/component/lg/handler_api.go |
| `RFC8522-2.3-2` | Consequently, every response must contain a "status" field of either "success", "fail", or "error" as explained in the following sections. (§2.3) | MUST | 2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no JSend status field is produced -- internal/component/lg/handler_api.go |
| `RFC8522-2.3.1-1` | A successful response must set the "status" field to "success". (§2.3.1) | MUST | 2.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no JSend envelope is produced -- internal/component/lg/handler_api.go |
| `RFC8522-2.3.1-2` | It must also contain a "data" object including the following information: (§2.3.1) | MUST | 2.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no JSend envelope is produced -- internal/component/lg/handler_api.go |
| `RFC8522-2.3.1-3` | Combined date and time in UTC ISO 8601 [iso8601] indicating when the operation finished. This information must be present. (§2.3.1) | MUST | 2.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no performed_at field is produced -- internal/component/lg/handler_api.go |
| `RFC8522-2.3.1-4` | Amount of seconds (wallclock) used to run the command. This information must be present. (§2.3.1) | MUST | 2.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no runtime field is produced -- internal/component/lg/handler_api.go |
| `RFC8522-2.3.1-5` | Adding more information to the response is permitted and must be placed inside the "data" object. (§2.3.1) | MUST | 2.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no JSend envelope is produced -- internal/component/lg/handler_api.go |
| `RFC8522-2.3.2-1` | The response message must set the "status" field to "fail" and must contain the "data" object with command-specific content listed in Section 2.3.1. (§2.3.2) | MUST | 2.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no JSend fail response is produced -- internal/component/lg/handler_api.go |
| `RFC8522-2.3.3-1` | The response message must set the "status" field to "error" and must contain the "message" key, which keeps a meaningful message, explaining what went wrong. (§2.3.3) | MUST | 2.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no JSend error response is produced -- internal/component/lg/handler_api.go |
| `RFC8522-3.1.1-1` | Implementation of the ping command is mandatory. (§3.1.1) | REQUIRED | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no /.well-known/looking-glass/v1/ping endpoint exists -- internal/component/lg/server.go::registerRoutes |
| `RFC8522-3.2.1-1` | Implementation of the "show route" command is mandatory. (§3.2.1) | REQUIRED | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no /.well-known/looking-glass/v1/show/route endpoint exists -- internal/component/lg/server.go::registerRoutes |
| `RFC8522-3.3-1` | The following organizational commands must be included in the implementation. (§3.3) | MUST | 3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** none of the routers, router details or cmd endpoints exists -- internal/component/lg/server.go::registerRoutes |
| `RFC8522-3.3.2-1` | The response must contain the router's hostname and router index. (§3.3.2) | MUST | 3.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no router details endpoint exists -- internal/component/lg/server.go::registerRoutes |
| `RFC8522-6-1` | The use of HTTPS is required to ensure a high level of security, privacy, and confidentiality during transit. (§6) | REQUIRED | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze's looking glass can serve TLS, but no RFC 8522 API exists for it to protect -- internal/component/lg/server.go::registerRoutes |
| `RFC8522-2.3.1-6` | The HTTP status code should be 200. (§2.3.1) | SHOULD | 2.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8522-2.3.3-3` | The HTTP status code should be 400 in case of a client-side error, 500 in case of a server-side error, or 502 for errors occurring on the target router. (§2.3.3) | SHOULD | 2.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8522-2.3.3-4` | Code 504 should be used when a command timed out. (§2.3.3) | SHOULD | 2.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC8522-6.1-1` | It is recommended that implementers of the Looking Glass API take steps to mitigate the above described abuse. (§6.1) | RECOMMENDED | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC8522-2.3.3-2` | The response may contain the "data" key with required values listed in Section 2.3.1. (§2.3.3) | MAY | 2.3.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC8522-2-1`](#rfc8522-2-1) The path component of the resource URI must use the prefix of ".well- known/looking-glass" (see Section 5.1) to stay namespace neutral. (§2) | {gap}, no test | Ze's looking glass registers its API under /api/looking-glass/ and nothing under /.well-known/looking-glass -- internal/component/lg/server.go::registerRoutes |
| [`RFC8522-2-2`](#rfc8522-2-2) The client must not be able to execute random commands on the targeting router. (§2) | {gap}, no test | Ze implements no RFC 8522 API, so no RFC 8522 call exists to restrict; Ze's own looking glass serves fixed handlers only -- internal/component/lg/server.go::registerRoutes |
| [`RFC8522-2-3`](#rfc8522-2-3) There must not be any direct communication between the client and the router. (§2) | {gap}, no test | Ze implements no RFC 8522 API -- internal/component/lg/server.go::registerRoutes |
| [`RFC8522-2.2-1`](#rfc8522-2.2-1) The server must ignore its value. (§2.2) | {gap}, no test | no RFC 8522 query parameter is parsed, `random` included -- internal/component/lg/server.go::registerRoutes |
| [`RFC8522-2.3-1`](#rfc8522-2.3-1) The response text must comply with the JSON syntax specification JSend, which is briefly explained in Appendix A. (§2.3) | {gap}, no test | Ze's looking-glass API answers in the birdwatcher shape, not JSend -- internal/component/lg/handler_api.go |
| [`RFC8522-2.3-2`](#rfc8522-2.3-2) Consequently, every response must contain a "status" field of either "success", "fail", or "error" as explained in the following sections. (§2.3) | {gap}, no test | no JSend status field is produced -- internal/component/lg/handler_api.go |
| [`RFC8522-2.3.1-1`](#rfc8522-2.3.1-1) A successful response must set the "status" field to "success". (§2.3.1) | {gap}, no test | no JSend envelope is produced -- internal/component/lg/handler_api.go |
| [`RFC8522-2.3.1-2`](#rfc8522-2.3.1-2) It must also contain a "data" object including the following information: (§2.3.1) | {gap}, no test | no JSend envelope is produced -- internal/component/lg/handler_api.go |
| [`RFC8522-2.3.1-3`](#rfc8522-2.3.1-3) Combined date and time in UTC ISO 8601 [iso8601] indicating when the operation finished. This information must be present. (§2.3.1) | {gap}, no test | no performed_at field is produced -- internal/component/lg/handler_api.go |
| [`RFC8522-2.3.1-4`](#rfc8522-2.3.1-4) Amount of seconds (wallclock) used to run the command. This information must be present. (§2.3.1) | {gap}, no test | no runtime field is produced -- internal/component/lg/handler_api.go |
| [`RFC8522-2.3.1-5`](#rfc8522-2.3.1-5) Adding more information to the response is permitted and must be placed inside the "data" object. (§2.3.1) | {gap}, no test | no JSend envelope is produced -- internal/component/lg/handler_api.go |
| [`RFC8522-2.3.2-1`](#rfc8522-2.3.2-1) The response message must set the "status" field to "fail" and must contain the "data" object with command-specific content listed in Section 2.3.1. (§2.3.2) | {gap}, no test | no JSend fail response is produced -- internal/component/lg/handler_api.go |
| [`RFC8522-2.3.3-1`](#rfc8522-2.3.3-1) The response message must set the "status" field to "error" and must contain the "message" key, which keeps a meaningful message, explaining what went wrong. (§2.3.3) | {gap}, no test | no JSend error response is produced -- internal/component/lg/handler_api.go |
| [`RFC8522-3.1.1-1`](#rfc8522-3.1.1-1) Implementation of the ping command is mandatory. (§3.1.1) | {gap}, no test | no /.well-known/looking-glass/v1/ping endpoint exists -- internal/component/lg/server.go::registerRoutes |
| [`RFC8522-3.2.1-1`](#rfc8522-3.2.1-1) Implementation of the "show route" command is mandatory. (§3.2.1) | {gap}, no test | no /.well-known/looking-glass/v1/show/route endpoint exists -- internal/component/lg/server.go::registerRoutes |
| [`RFC8522-3.3-1`](#rfc8522-3.3-1) The following organizational commands must be included in the implementation. (§3.3) | {gap}, no test | none of the routers, router details or cmd endpoints exists -- internal/component/lg/server.go::registerRoutes |
| [`RFC8522-3.3.2-1`](#rfc8522-3.3.2-1) The response must contain the router's hostname and router index. (§3.3.2) | {gap}, no test | no router details endpoint exists -- internal/component/lg/server.go::registerRoutes |
| [`RFC8522-6-1`](#rfc8522-6-1) The use of HTTPS is required to ensure a high level of security, privacy, and confidentiality during transit. (§6) | {gap}, no test | Ze's looking glass can serve TLS, but no RFC 8522 API exists for it to protect -- internal/component/lg/server.go::registerRoutes |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC8522-2-1`](#rfc8522-2-1)

The path component of the resource URI must use the prefix of ".well- known/looking-glass" (see Section 5.1) to stay namespace neutral. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2-1, so no unit is bound to it.

### [`RFC8522-2-2`](#rfc8522-2-2)

The client must not be able to execute random commands on the targeting router. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2-2, so no unit is bound to it.

### [`RFC8522-2-3`](#rfc8522-2-3)

There must not be any direct communication between the client and the router. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2-3, so no unit is bound to it.

### [`RFC8522-2.2-1`](#rfc8522-2.2-1)

The server must ignore its value. (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2.2-1, so no unit is bound to it.

### [`RFC8522-2.3-1`](#rfc8522-2.3-1)

The response text must comply with the JSON syntax specification JSend, which is briefly explained in Appendix A. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2.3-1, so no unit is bound to it.

### [`RFC8522-2.3-2`](#rfc8522-2.3-2)

Consequently, every response must contain a "status" field of either "success", "fail", or "error" as explained in the following sections. (§2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2.3-2, so no unit is bound to it.

### [`RFC8522-2.3.1-1`](#rfc8522-2.3.1-1)

A successful response must set the "status" field to "success". (§2.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2.3.1-1, so no unit is bound to it.

### [`RFC8522-2.3.1-2`](#rfc8522-2.3.1-2)

It must also contain a "data" object including the following information: (§2.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2.3.1-2, so no unit is bound to it.

### [`RFC8522-2.3.1-3`](#rfc8522-2.3.1-3)

Combined date and time in UTC ISO 8601 [iso8601] indicating when the operation finished. This information must be present. (§2.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2.3.1-3, so no unit is bound to it.

### [`RFC8522-2.3.1-4`](#rfc8522-2.3.1-4)

Amount of seconds (wallclock) used to run the command. This information must be present. (§2.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2.3.1-4, so no unit is bound to it.

### [`RFC8522-2.3.1-5`](#rfc8522-2.3.1-5)

Adding more information to the response is permitted and must be placed inside the "data" object. (§2.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2.3.1-5, so no unit is bound to it.

### [`RFC8522-2.3.2-1`](#rfc8522-2.3.2-1)

The response message must set the "status" field to "fail" and must contain the "data" object with command-specific content listed in Section 2.3.1. (§2.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2.3.2-1, so no unit is bound to it.

### [`RFC8522-2.3.3-1`](#rfc8522-2.3.3-1)

The response message must set the "status" field to "error" and must contain the "message" key, which keeps a meaningful message, explaining what went wrong. (§2.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-2.3.3-1, so no unit is bound to it.

### [`RFC8522-3.1.1-1`](#rfc8522-3.1.1-1)

Implementation of the ping command is mandatory. (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-3.1.1-1, so no unit is bound to it.

### [`RFC8522-3.2.1-1`](#rfc8522-3.2.1-1)

Implementation of the "show route" command is mandatory. (§3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-3.2.1-1, so no unit is bound to it.

### [`RFC8522-3.3-1`](#rfc8522-3.3-1)

The following organizational commands must be included in the implementation. (§3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-3.3-1, so no unit is bound to it.

### [`RFC8522-3.3.2-1`](#rfc8522-3.3.2-1)

The response must contain the router's hostname and router index. (§3.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-3.3.2-1, so no unit is bound to it.

### [`RFC8522-6-1`](#rfc8522-6-1)

The use of HTTPS is required to ensure a high level of security, privacy, and confidentiality during transit. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC8522-6-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude (spec-rfc-profile-declared-membership phase 1) |
| Signed off | 2026-10-26 |
| Register | prose |
| Source | rfc/full/rfc8522.txt |
| Source fingerprint | 4dc3cf63cfe82351 |
| Record | rfc/extraction/rfc8522.json |
| Mapped sentences | 17 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | Title page, abstract, status, copyright and contents | 0 | skipped (front-matter) | Title page, abstract, status, copyright and contents |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 1 | walked | not stated |
| `2` | not stated | 4 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 1 | walked | not stated |
| `2.3` | not stated | 2 | walked | not stated |
| `2.3.1` | not stated | 5 | walked | not stated |
| `2.3.2` | not stated | 1 | walked | not stated |
| `2.3.3` | not stated | 2 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.1.1` | not stated | 0 | walked | not stated |
| `3.1.2` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.2.1` | not stated | 0 | walked | not stated |
| `3.2.2` | not stated | 0 | walked | not stated |
| `3.2.3` | not stated | 0 | walked | not stated |
| `3.2.4` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 1 | walked | not stated |
| `3.3.1` | not stated | 0 | walked | not stated |
| `3.3.2` | not stated | 1 | walked | not stated |
| `3.3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | Well-Known URI registration | 0 | skipped (iana) | Well-Known URI registration |
| `5.1` | Well-Known URI registration | 0 | skipped (iana) | Well-Known URI registration |
| `6` | not stated | 1 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 0 | walked | not stated |
| `7` | References | 0 | skipped (references) | References |
| `7.1` | Normative references | 0 | skipped (references) | Normative references |
| `7.2` | Informative references | 0 | skipped (references) | Informative references |
| `A` | not stated | 3 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: says the examples in the document omit some required parameters; it states nothing an implementation does | The examples skip some required parameters for reasons of simplicity. |
| `2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the looking-glass API client, the HTTP caller that sends the request ('The client must present all necessary parameters'). Ze is the server; the client is outside Ze, and Ze holds no looking-glass API client code | The client must present all necessary parameters for the server to execute the command on the selected router. |
| `A:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the status-key obligation of Section 2.3 for JSend in general; the 'should contain' part is advisory and keyed to the status value, which Sections 2.3.1 to 2.3.3 state per status | A basic JSend-compliant response must contain a "status" key and should contain "data", "message", and "code" keys dependent on the status value. |
| `A:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive: introduces Table 1; 'required' names a column of the table | The following table lists the required and optional keys. |
| `A:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Table 1 of JSend keys; its Required column restates the per-status obligations of Sections 2.3.1, 2.3.2 and 2.3.3, which are mapped there | +---------+-----------------+---------------+ \| Type \| Required keys \| Optional keys \| +---------+-----------------+---------------+ \| success \| status, data \| \| \| fail \| status, data \| \| \| error \| status, message \| code, data \| +---------+-----------------+---------------+ |

## Superseded

No document obsoletes RFC 8522, so its obligations are stated where they were written.
