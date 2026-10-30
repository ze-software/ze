# RFC 2385 - Protection of BGP Sessions via the TCP MD5 Signature Option

Supported on Linux; FreeBSD needs a `setkey(8)` SAD entry. Every requirement this repository extracted from RFC 2385, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 6 of 8 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 25.0% | 2 of 8 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 8 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 8 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 8 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 16 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |
| Audit verdicts | 8 | of 8 gated MUSTs judged | 0 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 8 | of 9 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 8 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 8 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 8 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 8 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 8 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

The 9 shares marked as a part above are the whole of the 8 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | ok | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported on Linux; FreeBSD needs a `setkey(8)` SAD entry |
| Enrolment | Enrolled |
| Requirements | 9 |
| Gated MUST-level | 8 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 16 |
| Tagged units | 16 |
| Recorded audit verdicts | 8 |
| Discrimination records | 16 |
| Summary | `rfc/short/rfc2385.md` |
| Requirement shard | `rfc/requirements/rfc2385.md` |
| RFC text | `rfc/full/rfc2385.txt` |

## Enrolment

Enrolled: Protection of BGP Sessions via the TCP MD5 Signature Option. The RFC writes its obligations in lowercase; the extraction sign-off derives register 'prose'. Ze installs the application key with TCP_MD5SIG and Linux signs and validates the segments. Configuration and loopback tests cover key plumbing and acceptance. The separate-namespace veth tests in internal/core/network/rfc2385_md5_egress_integration_linux_test.go observe actual signatures, independently recompute digests, inject an unsigned SYNACK before a signed retransmission, and contrast malformed signatures with independently signed controls. Enrolled 2026-09-01.

## What the public ledger says

**Status:** Supported on Linux; FreeBSD needs a `setkey(8)` SAD entry

**What the ledger says is covered**

- Per-peer TCP MD5 through `connection { md5 { password; ip; } }`. `parsePeer` reads the key, `NewSession` configures the dialer, `md5PeersForListener` configures the listener, and `setTCPMD5Sig` installs it before connect or bind. Linux performs signing and verification. Loopback tests cover matching, mismatched and missing keys
- configured veth tests capture handshake, data and FIN packets in both directions, check exact Kind 19 Length 18 bytes and independently recomputed digests, and inject malformed signatures and an unsigned SYNACK. Application opt-out is observed as unsigned SYN and SYNACK packets. The FRR nightly scenario `test/interop/scenarios/bgp-md5-auth-frr` checks FRR's view of the authenticated session. Linux caps keys at 80 octets, the floor Section 4.5 recommends.


**What the ledger says remains**

No conformance gap is tracked.

- **One platform limit:** on FreeBSD `setTCPMD5Sig` ([`internal/core/network/md5_freebsd.go`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_freebsd.go)) enables the socket flag only and takes the key from the Security Association Database, so a configured password installs nothing there until `setkey(8)` carries it ([`plan/journal/silent-fall-through.md`](https://github.com/ze-software/ze/blob/main/plan/journal/silent-fall-through.md), 2026-09-01). macOS and every other platform refuse the socket option, so the connection fails rather than falling back to an unsigned session.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated (including scoped evidence) | 2 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **8** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC2385-2.0-1`](#rfc2385-2.0-1), [`RFC2385-2.0-2`](#rfc2385-2.0-2), [`RFC2385-2.0-3`](#rfc2385-2.0-3), [`RFC2385-2.0-5`](#rfc2385-2.0-5), [`RFC2385-2.0-6`](#rfc2385-2.0-6), [`RFC2385-3.0-1`](#rfc2385-3.0-1)

**Annotated (including scoped evidence) (2):** [`RFC2385-2.0-4`](#rfc2385-2.0-4), [`RFC2385-4.3-2`](#rfc2385-4.3-2)

## Attainable ceiling

Mandatory ceiling: 8 of 8 (100.0%). Optional ceiling: 1 of 1 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc2385 8 of 8 (100.0%). Lowest optional ceiling: rfc2385 1 of 1 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2385-2.0-1` | The nature of the key is deliberately left unspecified, but it must be known by both ends of the connection. (§2.0) | MUST | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L76). **negative:** `unit/verify` [`TestRFC2385KeyOnOneEndOnlyCarriesNoSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L178) |
| `RFC2385-2.0-2` | Upon receiving a signed segment, the receiver must validate it by calculating its own digest from the same data (using its own key) and comparing the two digest. (§2.0) | MUST | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L79). **negative:** `unit/verify` [`TestRFC2385MismatchedKeyIsDroppedWithNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L148) |
| `RFC2385-2.0-3` | A failing comparison must result in the segment being dropped and must not produce any response back to the sender. (§2.0) | MUST | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385MalformedWireSignaturesAreDropped`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L341). **negative:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L82) |
| `RFC2385-2.0-4` | Unlike other TCP extensions (e.g., the Window Scale option [RFC1323]), the absence of the option in the SYN,ACK segment must not cause the sender to disable its sending of signatures. (§2.0) | MUST NOT | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385UnsignedSYNACKDoesNotDisableSignatures`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L145). **negative:** no negative test. **{single-polarity}:** ze holds no code path that stops signing: the key is installed from the peer's settings on every dial attempt and is never cleared by anything the remote host does or fails to do, so there is no rejecting branch a negative test could reach |
| `RFC2385-2.0-5` | More importantly, the sending of signatures must be under the complete control of the application, not at the mercy of the remote host not understanding the option. (§2.0) | MUST | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385ConfiguredKeyReachesBothSockets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L40). **positive:** `unit/verify` [`TestRFC2385UnsignedSYNACKDoesNotDisableSignatures`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L146). **negative:** `unit/verify` [`TestRFC2385NoKeyWithoutConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L93). **negative:** `unit/verify` [`TestRFC2385UnconfiguredSocketSendsNoSignature`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L303) |
| `RFC2385-2.0-6` | Every segment sent on a TCP connection to be protected against spoofing will contain the 16-byte MD5 digest produced by applying the MD5 algorithm to these items in the following order: 1. the TCP pseudo-header (in the order: source IP address, destination IP address, zero-padded protocol number, and segment length) 2. the TCP header, excluding options, and assuming a checksum of zero 3. the TCP segment data (if any) 4. an independently-specified key or password, known to both TCPs and presumably connection-specific (§2.0) | MUST | 2.0 - Proposal | **positive:** `unit/verify` [`TestRFC2385CapturedSegmentsHaveIndependentDigests`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L36). **negative:** `unit/verify` [`TestRFC2385MalformedWireSignaturesAreDropped`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L342) |
| `RFC2385-3.0-1` | The proposed option has the following format: +---------+---------+-------------------+ \| Kind=19 \|Length=18\| MD5 digest... \| +---------+---------+-------------------+ \| \| +---------------------------------------+ \| \| +---------------------------------------+ \| \| +-------------------+-------------------+ \| \| +-------------------+ The MD5 digest is always 16 bytes in length, and the option would appear in every segment of a connection. (§3.0) | MUST | 3.0 - Syntax | **positive:** `unit/verify` [`TestRFC2385CapturedSegmentsHaveIndependentDigests`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L37). **negative:** `unit/verify` [`TestRFC2385MalformedWireSignaturesAreDropped`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L343) |
| `RFC2385-4.3-2` | This means that the total size of the header plus option must be less than or equal to 60 bytes -- this leaves 40 bytes for options. (§4.3) | MUST | 4.3 - TCP Header Size | **positive:** `unit/verify` [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L84). **negative:** no negative test. **{single-polarity}:** the option list is assembled by the kernel and ze contributes no TCP option of its own, so there is no rejecting branch a negative test could reach |
| `RFC2385-4.5-1` | It is strongly recommended that an implementation be able to support at minimum a key composed of a string of printable ASCII of 80 bytes or less, as this is current practice. (§4.5) | SHOULD | 4.5 - Key configuration | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 2385 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2385-2.0-1`](#rfc2385-2.0-1)

The nature of the key is deliberately left unspecified, but it must be known by both ends of the connection. (§2.0)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Collateral source re-judgment 2026-10-02 after removing unrelated 2.0-6/3.0-1 and retired4.3-1 tags from the enclosing legacy test comment; executable assertions remain unchanged. RFC2385 Section2.0 says the key must be known by both ends. TestRFC2385MatchingKeysCarryASignedSession supplies the same key to RealListenerFactory through rfc2385Listener and to RealDialer, requires a completed connection, then requires exactly256KiB delivered. Negative TestRFC2385KeyOnOneEndOnlyCarriesNoSession keys only the listening socket, leaves the dialer unkeyed and requires a net.Error timeout; rfc2385DialDropped fails on a completed dial. A no-op key installation would make that negative connect and fail. Actual producer network.go controls call md5_linux.go::setTCPMD5Sig before bind/connect and propagate failures. This is Linux whole-stack key-sharing evidence, not an independent digest-composition claim. The legacy helper skips unsupported-kernel ENOPROTOOPT/EOPNOTSUPP, so skipped execution must never be reported as proof; the separately observed configured runtime now demonstrably supports keyed sockets. Source-only rejudgment; no new execution or discrimination completion claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385KeyOnOneEndOnlyCarriesNoSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L178) | unit/verify | revert, verified |
| positive | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L76) | unit/verify | revert, verified |

### [`RFC2385-2.0-2`](#rfc2385-2.0-2)

Upon receiving a signed segment, the receiver must validate it by calculating its own digest from the same data (using its own key) and comparing the two digest. (§2.0)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Collateral source re-judgment 2026-10-02 after unrelated tag removals in MatchingKeys and MismatchedKey doc comments; executable assertions remain unchanged. RFC2385 Section2.0 requires receiving TCP to validate a signed segment by calculating with its own key and comparing. Positive TestRFC2385MatchingKeysCarryASignedSession installs matching keys on real TCP sockets and requires256KiB received. Negative TestRFC2385MismatchedKeyIsDroppedWithNoResponse installs different listener/dialer keys and requires timeout rather than establishment; rfc2385DialDropped explicitly fails on successful dial. The actual configured TCP_MD5SIG socket path is network.go::RealDialer.DialContext and RealListenerFactory.Listen through md5_linux.go::setTCPMD5Sig, with Linux doing verification. Matching acceptance plus mismatching refusal discriminates key-dependent receiver validation at that boundary; exact independent digest composition is separately covered by the new2.0-6 packet carrier and is not attributed to this old loopback unit. Unsupported-kernel skips are not positive evidence. Source-only rejudgment, no new test run or native record claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385MismatchedKeyIsDroppedWithNoResponse`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L148) | unit/verify | revert, verified |
| positive | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L79) | unit/verify | revert, verified |

### [`RFC2385-2.0-3`](#rfc2385-2.0-3)

A failing comparison must result in the segment being dropped and must not produce any response back to the sender. (§2.0)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor after the userns conversion. The unit's only change since the last verdict is userns.Enter(t) as its first statement (the child re-runs the whole body in its own user, network and mount namespace; a child failure or skip propagates to the parent, and a child that ran nothing fails it) plus removal of EPERM/EACCES skips, so a refused privilege now fails instead of skipping. Assertions are byte-identical. Section 2.0: 'A failing comparison must result in the segment being dropped and must not produce any response back to the sender.' TestRFC2385MalformedWireSignaturesAreDropped raw-sends to a keyed RealListenerFactory listener a SYN whose Kind 19 digest alone is corrupted with valid IP/TCP checksums, follows it with non-TCP noise, and rfc2385ExpectReply fails on any TCP reply to that port in a bounded 300ms window; an independently signed control on the next port must then get a SYNACK whose digest the independent oracle verifies, so silence is not a dead path. Bounded packet evidence, not unlimited silence. TestRFC2385MatchingKeysCarryASignedSession (unchanged) is the keyed-session positive. Producer: network.go Listen installing the key via setTCPMD5Sig; Linux TCP compares.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestRFC2385MalformedWireSignaturesAreDropped`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L341) | unit/verify | revert, verified |

### [`RFC2385-2.0-4`](#rfc2385-2.0-4)

Unlike other TCP extensions (e.g., the Window Scale option [RFC1323]), the absence of the option in the SYN,ACK segment must not cause the sender to disable its sending of signatures. (§2.0)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor after the userns conversion. The unit's only change since the last verdict is userns.Enter(t) as its first statement (the child re-runs the whole body in its own user, network and mount namespace; a child failure or skip propagates to the parent, and a child that ran nothing fails it) plus removal of EPERM/EACCES skips, so a refused privilege now fails instead of skipping. Assertions are byte-identical. Section 2.0: 'Unlike other TCP extensions (e.g., the Window Scale option [RFC1323]), the absence of the option in the SYN,ACK segment must not cause the sender to disable its sending of signatures.' Single positive per the row's annotation. TestRFC2385UnsignedSYNACKDoesNotDisableSignatures dials with a key, the observer verifies the initial SYN's digest independently, injects an unsigned SYNACK acknowledging it with repaired checksums, then requires a retransmitted SYN on the same ports and sequence with an independently valid digest; the caller requires a net.Error timeout, so an unsigned establishment fails it. Observer errors return over a channel to the caller's Fatal.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2385UnsignedSYNACKDoesNotDisableSignatures`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L145) | unit/verify | revert, verified |

### [`RFC2385-2.0-5`](#rfc2385-2.0-5)

More importantly, the sending of signatures must be under the complete control of the application, not at the mercy of the remote host not understanding the option. (§2.0)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor after the userns conversion. The unit's only change since the last verdict is userns.Enter(t) as its first statement (the child re-runs the whole body in its own user, network and mount namespace; a child failure or skip propagates to the parent, and a child that ran nothing fails it) plus removal of EPERM/EACCES skips, so a refused privilege now fails instead of skipping. Assertions are byte-identical. Section 2.0: 'More importantly, the sending of signatures must be under the complete control of the application, not at the mercy of the remote host not understanding the option.' Positive TestRFC2385UnsignedSYNACKDoesNotDisableSignatures: the key the application installed survives an unsigned SYNACK (signed retransmission, dial timeout). Negative TestRFC2385UnconfiguredSocketSendsNoSignature: with no key, RealDialer and the listener complete an ordinary handshake, any Kind 19 on a captured segment fails it, and two SYN-bearing packets are required. Config units TestRFC2385ConfiguredKeyReachesBothSockets and TestRFC2385NoKeyWithoutConfiguration (unchanged) bind the key to configuration.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385NoKeyWithoutConfiguration`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L93) | unit/verify | revert, verified |
| negative | [`TestRFC2385UnconfiguredSocketSendsNoSignature`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L303) | unit/verify | revert, verified |
| positive | [`TestRFC2385ConfiguredKeyReachesBothSockets`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc2385_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestRFC2385UnsignedSYNACKDoesNotDisableSignatures`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L146) | unit/verify | revert, verified |

### [`RFC2385-2.0-6`](#rfc2385-2.0-6)

Every segment sent on a TCP connection to be protected against spoofing will contain the 16-byte MD5 digest produced by applying the MD5 algorithm to these items in the following order: 1. the TCP pseudo-header (in the order: source IP address, destination IP address, zero-padded protocol number, and segment length) 2. the TCP header, excluding options, and assuming a checksum of zero 3. the TCP segment data (if any) 4. an independently-specified key or password, known to both TCPs and presumably connection-specific (§2.0)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor after the userns conversion. The unit's only change since the last verdict is userns.Enter(t) as its first statement (the child re-runs the whole body in its own user, network and mount namespace; a child failure or skip propagates to the parent, and a child that ran nothing fails it) plus removal of EPERM/EACCES skips, so a refused privilege now fails instead of skipping. Assertions are byte-identical. Section 2.0, the four-item digest composition (pseudo-header, TCP header without options with checksum zero, segment data, key). TestRFC2385CapturedSegmentsHaveIndependentDigests runs keyed RealDialer/RealListenerFactory across namespaces and checks every captured packet with an oracle that hashes the captured addresses, protocol and TCP length, the 20-byte header with checksum zeroed, the payload and rfc2385Key without the kernel's verifier; it requires SYN, pure ACK, data, FIN and FIN-ACK in both directions and exact data totals. Negative TestRFC2385MalformedWireSignaturesAreDropped: a corrupted digest gets no reply while an independently composed signed SYN gets a verified SYNACK.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385MalformedWireSignaturesAreDropped`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L342) | unit/verify | revert, verified |
| positive | [`TestRFC2385CapturedSegmentsHaveIndependentDigests`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L36) | unit/verify | revert, verified |

### [`RFC2385-3.0-1`](#rfc2385-3.0-1)

The proposed option has the following format: +---------+---------+-------------------+ | Kind=19 |Length=18| MD5 digest... | +---------+---------+-------------------+ | | +---------------------------------------+ | | +---------------------------------------+ | | +-------------------+-------------------+ | | +-------------------+ The MD5 digest is always 16 bytes in length, and the option would appear in every segment of a connection. (§3.0)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 by an independent auditor after the userns conversion. The unit's only change since the last verdict is userns.Enter(t) as its first statement (the child re-runs the whole body in its own user, network and mount namespace; a child failure or skip propagates to the parent, and a child that ran nothing fails it) plus removal of EPERM/EACCES skips, so a refused privilege now fails instead of skipping. Assertions are byte-identical. Section 3.0: Kind=19, Length=18, and 'The MD5 digest is always 16 bytes in length, and the option would appear in every segment of a connection.' Positive: TestRFC2385CapturedSegmentsHaveIndependentDigests applies assertRFC2385Digest to every captured segment of the full lifecycle in both directions, which requires exactly one Kind 19 option of 18 octets whose 16 digest octets match the oracle. Negative: TestRFC2385MalformedWireSignaturesAreDropped sends SYNs with the option missing, Kind 30, and Length 17, each with valid ordinary checksums, and requires no reply in 300ms followed by a SYNACK to a correctly encoded signed control.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2385MalformedWireSignaturesAreDropped`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L343) | unit/verify | revert, verified |
| positive | [`TestRFC2385CapturedSegmentsHaveIndependentDigests`](https://github.com/ze-software/ze/blob/main/internal/core/network/rfc2385_md5_egress_integration_linux_test.go#L37) | unit/verify | revert, verified |

### [`RFC2385-4.3-2`](#rfc2385-4.3-2)

This means that the total size of the header plus option must be less than or equal to 60 bytes -- this leaves 40 bytes for options. (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Collateral source re-judgment 2026-10-02 after unrelated tag removals from TestRFC2385MatchingKeysCarryASignedSession; the executable body and this retained tag are unchanged. RFC2385 Section4.3 requires total TCP header plus options at most60bytes, leaving40bytes for options. Ze contributes no manual TCP header/options assembly: actual RealDialer and RealListenerFactory configure TCP_MD5SIG, and Linux owns the option list and data-offset field. The retained positive unit establishes keyed TCP and carries256KiB through the listener, so options fit the interoperating TCP header rather than preventing the protected connection; this is boundary evidence under the whole-stack rule. The single-positive annotation remains appropriate because Ze has no independent oversized TCP-option input/reject branch. Independent new captured-lifecycle evidence additionally observes52-byte handshake TCP headers and40-byte data/FIN headers, but that untagged observation is not substituted for this retained unit's identity. This is the header ceiling, not retired4.3-1 advertised-MSS advice; RFC6691 corrects the latter only. No native run or record refresh claimed in this source-only collateral judgment.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2385MatchingKeysCarryASignedSession`](https://github.com/ze-software/ze/blob/main/internal/core/network/md5_rfc2385_linux_test.go#L84) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | Original 2026-09-01 walk retained; MD5LinuxAuthor re-read RFC2385 Section 4.3 and RFC6691 Section 3.2 for the owner-authorized MSS retirement on 2026-10-02 |
| Signed off | 2026-10-02 |
| Register | prose |
| Source | rfc/full/rfc2385.txt |
| Source fingerprint | d4667cc1ab9fd8ab |
| Record | rfc/extraction/rfc2385.json |
| Mapped sentences | 6 |
| Declined as scope | 2 |
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
| `4.3` | TCP Header Size | 2 | walked | TCP Header Size. Two derived sites: 4.3:1 is excluded because RFC6691 Section 3.2 explicitly corrects the advertised-MSS advice (retirement authorized 2026-10-02); 4.3:2 remains mapped to the 60-byte TCP header ceiling. The 4.4BSD worked example states no further obligation. |
| `4.4` | MD5 as a Hashing Algorithm | 0 | walked | MD5 as a Hashing Algorithm. Why the memo keeps MD5 despite the collision-search result: the option is already deployed and carries no algorithm-type field. It states what a FUTURE document could do and binds nobody here. |
| `4.5` | Key configuration | 0 | walked | Key configuration. One obligation, written as 'It is strongly recommended that an implementation be able to support at minimum a key composed of a string of printable ASCII of 80 bytes or less, as this is current practice'. 'strongly recommended' is SHOULD-level and carries no modal the scan counts, so it is captured as RFC2385-4.5-1. |
| `5.0` | Security Considerations | 0 | walked | Security Considerations. States that this is a weak but currently practiced mechanism and that stronger ones are expected later. No obligation. |
| `6.0` | not stated | 1 | skipped (references) | References, the author's address and the Full Copyright Statement, which the section parser attached to the last numbered heading. Its one derived site is the copyright licence sentence. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | No current MSS requirement: RFC6691 Section 3.2 quotes this exact sentence and states 'This is incorrect. The value for the MSS option is only adjusted by the fixed IP and TCP headers.' RFC9293 Section 3.7.1 retains the correction. Owner ruling MSS retirement and EVPN polarity (2026-10-02) retires RFC2385-4.3-1; this exclusion records that explicit standards correction, not an absent implementation. | As with other options that are added to every segment, the size of the MD5 option must be factored into the MSS offered to the other side during connection negotiation. |
| `6.0:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the Full Copyright Statement's licence condition on modifying and republishing THIS DOCUMENT. It binds a party redistributing the memo, not a TCP implementation, and states nothing about a segment, a digest or a key. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2385, so its obligations are stated where they were written.
