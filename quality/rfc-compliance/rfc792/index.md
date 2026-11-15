# RFC 792 - Internet Control Message Protocol

No entry in the public support list. Every requirement listed for RFC 792, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7 | of 10 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 3 | of 10 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 10 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 10 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 10 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 14 | 14 of 22 tests linked to requirements, 7 without a fault test and 0 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 10 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 10 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 10 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 10 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 10 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 10 | of 14 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 10 mandatory requirements: they add up to 10. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | No entry in the public support list |
| Automatic checks | Included |
| Requirements | 14 |
| Mandatory requirements (MUST level) | 10 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 22 |
| Tests linked to requirements | 22 |
| Recorded audit verdicts | 8 |
| Fault test records | 21 |
| Summary | `rfc/short/rfc792.md` |
| Requirement table | `rfc/requirements/rfc792.md` |
| RFC text | `rfc/full/rfc792.txt` |

## Automatic checks

Included in automatic checks: ICMP is part of Ze's whole Linux stack: Go constructs diagnostic Echo Requests, while Linux responds to local Echo Requests, emits ICMP errors, and enforces IPv4 gateway discard requirements. The gateway and unused-field requirements are exercised at Ze's netlink/packet boundary in internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go.

## Published support status

No entry in the public support list, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 792.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 3 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **10** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (7):** [`RFC792-Echo-1`](#rfc792-echo-1), [`RFC792-Echo-3`](#rfc792-echo-3), [`RFC792-Echo-5`](#rfc792-echo-5), [`RFC792-Echo-6`](#rfc792-echo-6), [`RFC792-Unreachable-1`](#rfc792-unreachable-1), [`RFC792-TimeExceeded-1`](#rfc792-timeexceeded-1), [`RFC792-ParamProblem-1`](#rfc792-paramproblem-1)

**Annotated (including scoped evidence) (3):** [`RFC792-Echo-2`](#rfc792-echo-2), [`RFC792-Echo-4`](#rfc792-echo-4), [`RFC792-Format-1`](#rfc792-format-1)

## Figures

- **MUST bar**: Ze proven 6, Proof pending 4
- **Overview bar**: Ze MUST proven 6, Proof pending 4, Not assessed 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 10 |
| MUST rows kept (the MUST bar) | 10 |
| Ze proven | 6 |
| OS verified | 0 |
| Proof pending | 4 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 4 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc792.json](../../../data/rfc/rfc792.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC792-Echo-1` | Type 8 for echo message; 0 for echo reply message. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L402). **positive:** `unit/verify` [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L198). **negative:** `unit/verify` [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L199) |
| `RFC792-Echo-2` | Type 8 for echo message; 0 for echo reply message. Code 0 (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L403). **positive:** `unit/verify` [`TestRFC792EchoRequestCode`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L226). **negative:** no negative test. **{single-polarity}:** ze emits Code 0 and never varies it, so there is no non-zero-code echo it produces to assert against |
| `RFC792-Echo-3` | The checksum is the 16-bit ones's complement of the one's complement sum of the ICMP message starting with the ICMP Type. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L404). **positive:** `unit/verify` [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L233). **negative:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L405) |
| `RFC792-Echo-4` | If the total length is odd, the received data is padded with one octet of zeros for computing the checksum. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L252). **negative:** no negative test. **{single-polarity}:** the zero pad is an internal step of a correct computation and ze rejects nothing on this basis, so only the positive direction is assertable |
| `RFC792-Echo-5` | The data received in the echo message must be returned in the echo reply message. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L406). **negative:** `unit/verify` [`TestGatewayICMPEchoReplyToTheQueriedAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L483) |
| `RFC792-Echo-6` | To form an echo reply message, the source and destination addresses are simply reversed, the type code changed to 0, and the checksum recomputed. (§Echo) | MUST | Echo | **positive:** `unit/verify` [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L407). **negative:** `unit/verify` [`TestGatewayICMPEchoReplyToTheQueriedAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L484) |
| `RFC792-Format-1` | Any field labeled "unused" is reserved for later extensions and must be zero when sent (§Format) | MUST | Format | **positive:** `unit/verify` [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L321). **positive:** `unit/verify` [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L375). **positive:** `unit/verify` [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L348). **negative:** no negative test. **{single-polarity}:** Linux builds every ICMP error the gateway emits and ze writes no octet of it, so no input to ze yields an error with a nonzero unused field; the positive tags already contrast the zero unused octets with the nonzero assigned fields (pointer, next-hop MTU, RFC 4884 length) |
| `RFC792-Unreachable-1` | Another case is when a datagram must be fragmented to be forwarded by a gateway yet the Don't Fragment flag is on. In this case the gateway must discard the datagram and may return a destination unreachable message. (§Unreachable) | MUST | Unreachable | **positive:** `unit/verify` [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L319). **negative:** `unit/verify` [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L320) |
| `RFC792-TimeExceeded-1` | If the gateway processing a datagram finds the time to live field is zero it must discard the datagram. (§TimeExceeded) | MUST | TimeExceeded | **positive:** `unit/verify` [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L346). **negative:** `unit/verify` [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L347) |
| `RFC792-ParamProblem-1` | If the gateway or host processing a datagram finds a problem with the header parameters such that it cannot complete processing the datagram it must discard the datagram (§ParamProblem) | MUST | ParamProblem | **positive:** `unit/verify` [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L373). **negative:** `unit/verify` [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L374) |
| `RFC792-Echo-7` | For computing the checksum , the checksum field should be zero. (§Echo) | SHOULD | Echo | **positive:** no positive test. **negative:** no negative test |
| `RFC792-Echo-8` | If code = 0, an identifier to aid in matching echos and replies, may be zero. (§Echo) | MAY | Echo | **positive:** no positive test. **negative:** no negative test |
| `RFC792-Echo-9` | If code = 0, a sequence number to aid in matching echos and replies, may be zero. (§Echo) | MAY | Echo | **positive:** no positive test. **negative:** no negative test |
| `RFC792-Echo-10` | Code 0 may be received from a gateway or a host. (§Echo) | MAY | Echo | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 792 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC792-Echo-1`](#rfc792-echo-1)

Type 8 for echo message; 0 for echo reply message. (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPEchoReply unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Echo: "0 for echo reply message." Fresh judgement: the stack reply to Ze's installed gateway address must carry Type 0 (answer[0] != 0 fails); TestRFC792EchoRequestType (unchanged) pins Type 8/0 for Ze's own probes and its negative refuses a Type 8 as reply. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestRFC792EchoRequestType`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L198) | unit/verify | revert, verified |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L402) | unit/verify | revert, verified |

### [`RFC792-Echo-2`](#rfc792-echo-2)

Type 8 for echo message; 0 for echo reply message. Code 0 (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPEchoReply unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Echo: Code "0". Fresh judgement: the reply's Code is asserted 0 and TestRFC792EchoRequestCode pins the request byte; single-polarity per the row. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792EchoRequestCode`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L226) | unit/verify | revert, verified |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L403) | unit/verify | revert, verified |

### [`RFC792-Echo-3`](#rfc792-echo-3)

The checksum is the 16-bit ones's complement of the one's complement sum of the ICMP message starting with the ICMP Type. (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPEchoReply unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Echo: "The checksum is the 16-bit ones's complement of the one's complement sum of the ICMP message starting with the ICMP Type." Fresh judgement: the reply's checksum must fold to zero, and a request with a deliberately wrong checksum (sequence 2) must never be reflected, beside the valid control; the rejecting branch is Linux's on the address Ze installs (whole-stack rule). Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L405) | unit/verify | revert, verified |
| positive | [`TestRFC792ChecksumValid`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L233) | unit/verify | revert, verified |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L404) | unit/verify | revert, verified |

### [`RFC792-Echo-4`](#rfc792-echo-4)

If the total length is odd, the received data is padded with one octet of zeros for computing the checksum. (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: computing the checksum of an odd-length message without one trailing zero pad octet. Red on it: TestRFC792ChecksumOddLength builds an 11-octet echo and asserts checksumOnesFold == 0xffff, which fails if icmpChecksum dropped the last octet or padded it into the low byte. The receive side is Linux; ze verifies no received ICMP checksum. Single-polarity marker on the row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC792ChecksumOddLength`](https://github.com/ze-software/ze/blob/main/internal/core/probe/icmp_test.go#L252) | unit/verify | unproven |

### [`RFC792-Echo-5`](#rfc792-echo-5)

The data received in the echo message must be returned in the echo reply message. (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPEchoReply unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Echo: "The data received in the echo message must be returned in the echo reply message." Fresh judgement: odd-length data must come back byte-equal, and TestGatewayICMPEchoReplyToTheQueriedAddress (same guard-only change) requires 1, 333 and 1400 distinct-octet payloads each returned exactly to its own sequence and every request answered. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPEchoReplyToTheQueriedAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L483) | unit/verify | revert, verified |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L406) | unit/verify | revert, verified |

### [`RFC792-Echo-6`](#rfc792-echo-6)

To form an echo reply message, the source and destination addresses are simply reversed, the type code changed to 0, and the checksum recomputed. (§Echo)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPEchoReply unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Echo: "To form an echo reply message, the source and destination addresses are simply reversed, the type code changed to 0, and the checksum recomputed." Fresh judgement: reversed addresses, Type 0 and a verifying checksum on the reply, and the secondary-address request must be answered from .253 to the requester, never from the primary .254. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPEchoReplyToTheQueriedAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L484) | unit/verify | revert, verified |
| positive | [`TestGatewayICMPEchoReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L407) | unit/verify | revert, verified |

### [`RFC792-Format-1`](#rfc792-format-1)

Any field labeled "unused" is reserved for later extensions and must be zero when sent (§Format)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L321) | unit/verify | no-break without a fault test (kernel-producer), which does not count as proof, verified |
| positive | [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L375) | unit/verify | no-break without a fault test (kernel-producer), which does not count as proof, verified |
| positive | [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L348) | unit/verify | no-break without a fault test (kernel-producer), which does not count as proof, verified |

### [`RFC792-Unreachable-1`](#rfc792-unreachable-1)

Another case is when a datagram must be fragmented to be forwarded by a gateway yet the Don't Fragment flag is on. In this case the gateway must discard the datagram and may return a destination unreachable message. (§Unreachable)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPDFDiscard unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Destination Unreachable: "In this case the gateway must discard the datagram and may return a destination unreachable message." Fresh judgement: a 900-octet DF datagram must not appear on the 576-MTU exit, now configured through Ze's netlinkBackend.SetMTU in routedExit (b8c4909334, helper only), with Fragmentation Needed returned; a DF datagram exactly at MTU and a DF-clear oversize datagram must forward, bounding the rule. newGatewayWire no longer skips on EPERM, which can only make the unit fail more often. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L320) | unit/verify | mutant, verified |
| positive | [`TestGatewayICMPDFDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L319) | unit/verify | mutant, verified |

### [`RFC792-TimeExceeded-1`](#rfc792-timeexceeded-1)

If the gateway processing a datagram finds the time to live field is zero it must discard the datagram. (§TimeExceeded)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-30 (independent auditor) after the user-namespace conversion (d8427e4380): the TestGatewayICMPTTLDiscard unit gained only `if !userns.Enter(t) { return }` as its first statement and its build tag moved from integration&&linux to linux. userns.Enter (internal/test/userns/userns_linux.go) re-runs the one named test in a fresh user+net+mount namespace and returns false to the parent only when the child reported PASS or SKIP; a failing child, a non-zero exit, or a child that ran nothing calls t.Fatalf, so every assertion below the guard still fails the test when the behaviour breaks. Body otherwise byte-identical. RFC 792 Time Exceeded: "If the gateway processing a datagram finds the time to live field is zero it must discard the datagram." Fresh judgement: TTL zero and TTL one must be absent on the routed exit and answered with Time Exceeded, while TTL two must forward on the same route, so a forwarding gateway fails. The discrimination record is a kernel-producer escape (ip_forward decides it from the packet alone): the test enforces the stack behaviour, but the row is published as escaped, not as proven by a break of Ze's code. Enforced.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L347) | unit/verify | no-break without a fault test (kernel-producer), which does not count as proof, verified |
| positive | [`TestGatewayICMPTTLDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L346) | unit/verify | no-break without a fault test (kernel-producer), which does not count as proof, verified |

### [`RFC792-ParamProblem-1`](#rfc792-paramproblem-1)

If the gateway or host processing a datagram finds a problem with the header parameters such that it cannot complete processing the datagram it must discard the datagram (§ParamProblem)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L374) | unit/verify | no-break without a fault test (kernel-producer), which does not count as proof, verified |
| positive | [`TestGatewayICMPHeaderDiscard`](https://github.com/ze-software/ze/blob/main/internal/plugins/vrrp/rfc792_gateway_icmp_integration_linux_test.go#L373) | unit/verify | no-break without a fault test (kernel-producer), which does not count as proof, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | prose |
| Source | rfc/full/rfc792.txt |
| Source fingerprint | 1cc33872d3b2a6af |
| Record | rfc/extraction/rfc792.json |
| Mapped sentences | 5 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 8 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Introduction's placement sentence, which says where ICMP lives rather than adding an obligation of its own: 'ICMP, uses the basic support of IP as if it were a higher level protocol, however, ICMP is actually an integral part of IP, and must be implemented by every IP module.' It is a conformance umbrella over the message obligations this document then specifies, and each of those is decided on its own site. The IP module in ze's stack is the Linux IP stack, which implements ICMP. | ICMP, uses the basic support of IP as if it were a higher level protocol, however, ICMP is actually an integral part of IP, and must be implemented by every IP module. |
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Introduction's statement that ICMP is not a reliability mechanism, addressed to protocols above IP rather than to an ICMP implementation. The two sentences before it read 'These ICMP messages ... report problems in the communication environment, not to make IP reliable. There are still no guarantees that a datagram will be delivered'. No ICMP behavior answers this sentence; the reliability it names is TCP's. | The higher level protocols that use IP must implement their own reliability procedures if reliable communication is required. |
| `front:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Descriptive, not normative: 'Another case is when a datagram must be fragmented to be forwarded by a gateway yet the Don't Fragment flag is on.' The 'must' says what the forwarding case requires, it obliges no role. The obligation of that case is the next sentence, site front:5. | Another case is when a datagram must be fragmented to be forwarded by a gateway yet the Don't Fragment flag is on. |

## Superseded

No document obsoletes RFC 792, so its obligations are stated where they were written.
