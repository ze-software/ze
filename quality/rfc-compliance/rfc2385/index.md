# RFC 2385 - Protection of BGP Sessions via the TCP MD5 Signature Option

Supported on Linux; FreeBSD needs a `setkey(8)` SAD entry. Every requirement this repository extracted from RFC 2385, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 66.7% | 6 of 9 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 33.3% | 3 of 9 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 9 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 15 of 15 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 10 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 9 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported on Linux; FreeBSD needs a `setkey(8)` SAD entry |
| Enrolment | Enrolled |
| Requirements | 10 |
| Gated MUST-level | 9 |
| Obligations that bind Ze | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tagged units | 15 |
| Recorded audit verdicts | 0 |
| Discrimination records | 15 |
| Summary | `rfc/short/rfc2385.md` |
| Requirement shard | `rfc/requirements/rfc2385.md` |
| RFC text | `rfc/full/rfc2385.txt` |

## Enrolment

Enrolled: Protection of BGP Sessions via the TCP MD5 Signature Option: nine gated requirements, every one written in lowercase by a document that predates RFC 2119, so the extraction sign-off derives register 'prose' (rfc/extraction/rfc2385.json, 8 sites, 1 excluded). Ze computes no digest: it installs the operator's key with TCP_MD5SIG and the kernel signs and validates every segment, which is the whole-stack ruling in ai/rules/rfc-compliance.md. The proof runs at the boundary ze owns and reads what the stack answers on loopback (internal/core/network/md5_rfc2385_linux_test.go): one key installed on both sockets carries a 256 KiB payload (RFC2385-2.0-1, -2.0-2, -2.0-6, -3.0-1, -4.3-1 and -4.3-2 positive, RFC2385-2.0-3 negative); a mismatched key is dropped with no answer at all, so the dial times out instead of being refused or reset (RFC2385-2.0-2, -2.0-6 and -3.0-1 negative, RFC2385-2.0-3 positive); and a key held by only one end carries no session (RFC2385-2.0-1 negative). The application-control rows are proven over the config path (internal/component/bgp/reactor/rfc2385_test.go): a peer carrying md5 { password } gets that key on the dialer and in the listener's key set (RFC2385-2.0-5 positive), a peer without one gets none on either (RFC2385-2.0-5 negative), and a failed connection attempt leaves the key in place (RFC2385-2.0-4 positive). Three rows are single-polarity positive because ze holds no rejecting branch for them: RFC2385-2.0-4, and the two section 4.3 rows, whose MSS and option list are assembled by the kernel. RFC2385-4.5-1 is SHOULD-level and met exactly at its floor: setTCPMD5Sig refuses a key above 80 octets (internal/core/network/md5_linux.go), which is Linux's TCP_MD5SIG_MAXKEYLEN. Enrolled 2026-09-01.

## What the public ledger says

**Status:** Supported on Linux; FreeBSD needs a `setkey(8)` SAD entry

**What the ledger says is covered**

- Per-peer TCP MD5 through `connection { md5 { password; ip; } }`. `parsePeer` ([`internal/component/bgp/reactor/config.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config.go)) reads the key, `NewSession` puts it on the dialing socket and `md5PeersForListener` puts it in the listening socket's key set ([`internal/component/bgp/reactor/session.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session.go), reactor.go), and `setTCPMD5Sig` installs it with `TCP_MD5SIG` before connect and before bind ([`internal/core/network/md5_linux.go`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_linux.go)), so the kernel signs and validates every segment of the peering. Ze computes no digest of its own
- conformance is judged on what the whole stack produces, and every requirement row is proven at the boundary ze owns. The nine gated rows of [`rfc/short/rfc2385.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2385.md) each carry both polarities or a single-polarity annotation: a loopback session under one shared key carries 256 KiB, a mismatched key is dropped with no answer at all, and a key held by one end only carries no session ([`internal/core/network/md5_rfc2385_linux_test.go`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go)). An FRR peer configured with the same password establishes with ze in the nightly interop lab, where the scenario's assertion is FRR's own view of the session (test/interop/scenarios/bgp-md5-auth-frr, `scenarioOperations` in [`internal/le/interoplab/bgp/checkers.go`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/checkers.go)). The key is capped at 80 octets, which is the floor Section 4.5 recommends and Linux's own `TCP_MD5SIG_MAXKEYLEN`.


**What the ledger says remains**

No conformance gap is tracked.

- **One platform limit:** on FreeBSD `setTCPMD5Sig` ([`internal/core/network/md5_freebsd.go`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_freebsd.go)) enables the socket flag only and takes the key from the Security Association Database, so a configured password installs nothing there until `setkey(8)` carries it ([`plan/journal/silent-fall-through.md`](https://github.com/ze-software/ze/blob/main/plan/journal/silent-fall-through.md), 2026-09-01). macOS and every other platform refuse the socket option, so the connection fails rather than falling back to an unsigned session.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC2385-2.0-1`](#rfc2385-2.0-1), [`RFC2385-2.0-2`](#rfc2385-2.0-2), [`RFC2385-2.0-3`](#rfc2385-2.0-3), [`RFC2385-2.0-5`](#rfc2385-2.0-5), [`RFC2385-2.0-6`](#rfc2385-2.0-6), [`RFC2385-3.0-1`](#rfc2385-3.0-1)

**Annotated instead of tested (3):** [`RFC2385-2.0-4`](#rfc2385-2.0-4), [`RFC2385-4.3-1`](#rfc2385-4.3-1), [`RFC2385-4.3-2`](#rfc2385-4.3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2385-2.0-1` | The key must be known by both ends of the connection, so the same configured key is installed for the peer on the outbound socket and on the listening socket (§2.0) | MUST | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L76). **negative:** `unit/verify` [`TestRFC2385KeyOnOneEndOnlyCarriesNoSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L192) |
| `RFC2385-2.0-2` | Upon receiving a signed segment, the receiver must validate it by calculating its own digest from the same data using its own key and comparing the two digests (§2.0) | MUST | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L79). **negative:** `unit/verify` [`TestRFC2385MismatchedKeyIsDroppedWithNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L153) |
| `RFC2385-2.0-3` | A failing comparison must result in the segment being dropped, and must not produce any response back to the sender (§2.0) | MUST | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385MismatchedKeyIsDroppedWithNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L156). **negative:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L82) |
| `RFC2385-2.0-4` | The absence of the option in the SYN,ACK segment must not cause the sender to disable its sending of signatures (§2.0) | MUST NOT | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385FailedConnectDoesNotDisableSigning`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L134). **negative:** no negative test. **{single-polarity}:** ze holds no code path that stops signing: the key is installed from the peer's settings on every dial attempt and is never cleared by anything the remote host does or fails to do, so there is no rejecting branch a negative test could reach |
| `RFC2385-2.0-5` | The sending of signatures must be under the complete control of the application, not at the mercy of the remote host not understanding the option (§2.0) | MUST | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385ConfiguredKeyReachesBothSockets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L40). **negative:** `unit/verify` [`TestRFC2385NoKeyWithoutConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L90) |
| `RFC2385-2.0-6` | Every segment sent on a protected connection carries the 16-byte MD5 digest of the TCP pseudo-header, the TCP header with a zero checksum, the segment data and the key, in that order (§2.0) | MUST | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L84). **negative:** `unit/verify` [`TestRFC2385MismatchedKeyIsDroppedWithNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L159) |
| `RFC2385-3.0-1` | The option is Kind 19, Length 18, carrying a 16-byte digest, and it appears in every segment of the connection (§3.0) | MUST | 3.0 - Syntax | **positive:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L87). **negative:** `unit/verify` [`TestRFC2385MismatchedKeyIsDroppedWithNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L162) |
| `RFC2385-4.3-1` | The size of the MD5 option must be factored into the MSS offered to the other side during connection negotiation (§4.3) | MUST | 4.3 - TCP Header Size | **positive:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L90). **negative:** no negative test. **{single-polarity}:** the MSS is chosen by the kernel that signs the segments, and ze holds no code that lowers, rejects or recomputes an MSS, so there is no rejecting branch a negative test could reach |
| `RFC2385-4.3-2` | The total size of the TCP header plus its options must be less than or equal to 60 bytes, leaving 40 bytes for options (§4.3) | MUST | 4.3 - TCP Header Size | **positive:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L93). **negative:** no negative test. **{single-polarity}:** the option list is assembled by the kernel and ze contributes no TCP option of its own, so there is no rejecting branch a negative test could reach |
| `RFC2385-4.5-1` | It is strongly recommended that an implementation support at minimum a key composed of a string of printable ASCII of 80 bytes or less (§4.5) | SHOULD | 4.5 - Key configuration | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 2385 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2385-2.0-1`](#rfc2385-2.0-1)

The key must be known by both ends of the connection, so the same configured key is installed for the peer on the outbound socket and on the listening socket (§2.0)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385KeyOnOneEndOnlyCarriesNoSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L192) | unit/verify | revert, verified |
| positive | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L76) | unit/verify | revert, verified |

### [`RFC2385-2.0-2`](#rfc2385-2.0-2)

Upon receiving a signed segment, the receiver must validate it by calculating its own digest from the same data using its own key and comparing the two digests (§2.0)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385MismatchedKeyIsDroppedWithNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L153) | unit/verify | revert, verified |
| positive | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L79) | unit/verify | revert, verified |

### [`RFC2385-2.0-3`](#rfc2385-2.0-3)

A failing comparison must result in the segment being dropped, and must not produce any response back to the sender (§2.0)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestRFC2385MismatchedKeyIsDroppedWithNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L156) | unit/verify | revert, verified |

### [`RFC2385-2.0-4`](#rfc2385-2.0-4)

The absence of the option in the SYN,ACK segment must not cause the sender to disable its sending of signatures (§2.0)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2385FailedConnectDoesNotDisableSigning`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L134) | unit/verify | revert, verified |

### [`RFC2385-2.0-5`](#rfc2385-2.0-5)

The sending of signatures must be under the complete control of the application, not at the mercy of the remote host not understanding the option (§2.0)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385NoKeyWithoutConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L90) | unit/verify | revert, verified |
| positive | [`TestRFC2385ConfiguredKeyReachesBothSockets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L40) | unit/verify | revert, verified |

### [`RFC2385-2.0-6`](#rfc2385-2.0-6)

Every segment sent on a protected connection carries the 16-byte MD5 digest of the TCP pseudo-header, the TCP header with a zero checksum, the segment data and the key, in that order (§2.0)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385MismatchedKeyIsDroppedWithNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L159) | unit/verify | revert, verified |
| positive | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L84) | unit/verify | revert, verified |

### [`RFC2385-3.0-1`](#rfc2385-3.0-1)

The option is Kind 19, Length 18, carrying a 16-byte digest, and it appears in every segment of the connection (§3.0)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385MismatchedKeyIsDroppedWithNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L162) | unit/verify | revert, verified |
| positive | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L87) | unit/verify | revert, verified |

### [`RFC2385-4.3-1`](#rfc2385-4.3-1)

The size of the MD5 option must be factored into the MSS offered to the other side during connection negotiation (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L90) | unit/verify | revert, verified |

### [`RFC2385-4.3-2`](#rfc2385-4.3-2)

The total size of the TCP header plus its options must be less than or equal to 60 bytes, leaving 40 bytes for options (§4.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L93) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-implement walk agent, spec-rfcgate-6-supported-extraction-signoff phase 7 (Class B, rfc2385) |
| Signed off | 2026-09-01 |
| Register | prose |
| Source | rfc/full/rfc2385.txt |
| Source fingerprint | d4667cc1ab9fd8ab |
| Record | rfc/extraction/rfc2385.json |
| Mapped sentences | 7 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, Copyright Notice, IESG Note and Abstract. The IESG Note records that the mechanism is weak against a concerted attack and the Abstract restates section 1; neither states an obligation. |
| `1.0` | Introduction | 0 | walked | Introduction. Indicative throughout: what an attacker would have to guess, that the password never appears in the connection stream, that its form is up to the application, and that there is no negotiation for the option because its use is site policy. The last of these is the ground the section 2 sentence at site 2.0:5 states normatively, so nothing is left unmapped here. |
| `2.0` | Proposal | 5 | walked | Proposal. The core of the document: what is signed, in what order, what a receiver does with the digest, and who controls signing. Five derived sites, all mapped. Its first sentence, 'Every segment sent on a TCP connection to be protected against spoofing will contain the 16-byte MD5 digest produced by applying the MD5 algorithm to these items in the following order', carries the whole computation and no modal, so the site scan cannot see it; it is captured as RFC2385-2.0-6. |
| `3.0` | Syntax | 0 | walked | Syntax. The option diagram and the sentence 'The MD5 digest is always 16 bytes in length, and the option would appear in every segment of a connection'. Both are written in the indicative, so the scan derives no site, and the encoding they fix is captured as RFC2385-3.0-1. |
| `4.0` | not stated | 0 | walked | 'Some Implications' is a heading with no body of its own; its four subsections follow. |
| `4.1` | Connectionless Resets | 0 | walked | Connectionless Resets. A consequence, not an obligation: a reset from a party without the key is ignored, so a connect to a port with no listener times out instead of being refused and a stale-connection reset no longer clears the session quickly. |
| `4.2` | Performance | 0 | walked | Performance. Two measured digest timings on a 100 MHz R4600 and the note that the cost is paid on both paths. No obligation. |
| `4.3` | TCP Header Size | 2 | walked | TCP Header Size. Two derived sites, both mapped: the option's 18 octets have to be allowed for in the MSS offered at setup, and the whole header including options has to fit the 60 bytes the data-offset field can express. The 4.4BSD worked example that follows is arithmetic over those two, and states nothing further. |
| `4.4` | MD5 as a Hashing Algorithm | 0 | walked | MD5 as a Hashing Algorithm. Why the memo keeps MD5 despite the collision-search result: the option is already deployed and carries no algorithm-type field. It states what a FUTURE document could do and binds nobody here. |
| `4.5` | Key configuration | 0 | walked | Key configuration. One obligation, written as 'It is strongly recommended that an implementation be able to support at minimum a key composed of a string of printable ASCII of 80 bytes or less, as this is current practice'. 'strongly recommended' is SHOULD-level and carries no modal the scan counts, so it is captured as RFC2385-4.5-1. |
| `5.0` | Security Considerations | 0 | walked | Security Considerations. States that this is a weak but currently practiced mechanism and that stronger ones are expected later. No obligation. |
| `6.0` | not stated | 1 | skipped (references) | References, the author's address and the Full Copyright Statement, which the section parser attached to the last numbered heading. Its one derived site is the copyright licence sentence. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `6.0:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the Full Copyright Statement's licence condition on modifying and republishing THIS DOCUMENT. It binds a party redistributing the memo, not a TCP implementation, and states nothing about a segment, a digest or a key. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2385, so its obligations are stated where they were written.
