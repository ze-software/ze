# RFC 7474 - Security Extensions for OSPFv2 when Using Manual Key Management

Experimental. Every requirement this repository extracted from RFC 7474, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 70.0% | 7 of 10 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 30.0% | 3 of 10 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 10 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 10 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 17 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 10 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 10 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 10 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 13 |
| Gated MUST-level | 10 |
| Obligations that bind Ze | 10 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 17 |
| Tagged units | 17 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7474.md` |
| Requirement shard | `rfc/requirements/rfc7474.md` |
| RFC text | `rfc/full/rfc7474.txt` |

## Enrolment

Enrolled: OSPFv2 manual-key security extension (AuType 3 extended 64-bit crypto sequence): 10 MUSTs, 7 met + 3 single-polarity, wired end-to-end

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

Cryptographic authentication with per-packet-type replay protection (RFC 7474 defines no OSPFv2 authentication trailer; that construct is OSPFv3/RFC 7166).

**What the ledger says remains:**

Same OSPF experimental status.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the gated population |
| Annotated instead of tested | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **10** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (7):** [`RFC7474-2-1`](#rfc7474-2-1), [`RFC7474-2-5`](#rfc7474-2-5), [`RFC7474-2-6`](#rfc7474-2-6), [`RFC7474-3-1`](#rfc7474-3-1), [`RFC7474-5-1`](#rfc7474-5-1), [`RFC7474-5-2`](#rfc7474-5-2), [`RFC7474-6-1`](#rfc7474-6-1)

**Annotated instead of tested (3):** [`RFC7474-2-2`](#rfc7474-2-2), [`RFC7474-2-3`](#rfc7474-2-3), [`RFC7474-2-4`](#rfc7474-2-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7474-2-1` | Carry the 64-bit sequence number in the 8 octets following the OSPFv2 packet and include it when computing the digest (§2) | MUST | 2 | **positive:** `unit/verify` [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L172). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L223) |
| `RFC7474-2-2` | Compose the 64-bit value as high-order boot count + low-order strictly increasing counter (§2) | MUST | 2 | **positive:** `unit/verify` [`TestSetBootCountSeedsSequence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L380). **negative:** no negative test. **{single-polarity}:** the 64-bit composition (boot count in the high word, per-packet counter in the low word) is a numeric construction property with no violating input to feed a negative test |
| `RFC7474-2-3` | Increment the lower-order 32-bit sequence number for every OSPF packet sent (§2) | MUST | 2 | **positive:** `unit/verify` [`TestOSPFAuthESNCounterWrapAdvancesBootCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L403). **negative:** no negative test. **{single-polarity}:** a monotonic send-side counter increment has no observable violating case, since every sent crypto packet advances it (and bumps the boot word on wrap) |
| `RFC7474-2-4` | Preserve the strictly increasing property of the aggregate sequence number for the deployed life of the router, including cold restarts (§2) | MUST | 2 | **positive:** `unit/verify` [`TestBootCountMonotonicAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L342). **negative:** no negative test. **{single-polarity}:** strict-increase preservation across restarts is a durability property with no violating input to feed; it is asserted positively across a simulated restart via the persisted boot count |
| `RFC7474-2-5` | On receive, accept only when the sequence number is greater than the last accepted packet **of that type** from that neighbor; otherwise drop as a replay (§2) | MUST | 2 | **positive:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L128). **negative:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L133) |
| `RFC7474-2-6` | Track the accepted-sequence high-water mark per neighbor and per OSPF packet type (allows out-of-order arrival across types, §2) | MUST | 2 | **positive:** `unit/verify` [`TestOSPFAuthReplayPerType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L164). **negative:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L134) |
| `RFC7474-3-1` | Set the OSPF Authentication field per AuType 3: 24-bit reserved `0`, 8-bit Auth Data Len, 32-bit Key ID in the former sequence-number position (§3) | MUST | 3 | **positive:** `unit/verify` [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L162). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L252) |
| `RFC7474-5-1` | Include the 64-bit sequence number in the First-Hash along with the Authentication Trailer and OSPF packet (§5) | MUST | 5 | **positive:** `unit/verify` [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L173). **negative:** `unit/verify` [`TestOSPFAuthType3SequenceTamperRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L284) |
| `RFC7474-5-2` | Initialize the first 4 octets of `Apad` to the packet's IP source address (send and receive), remainder 0x878FE1F3 (§5) | MUST | 5 | **positive:** `unit/verify` [`TestOSPFAuthType3SourceBinding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L197). **negative:** `unit/verify` [`TestOSPFAuthType3SourceBinding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L193) |
| `RFC7474-6-1` | Append the two-octet OSPFv2 Cryptographic Protocol ID to the authentication key prior to use, to block cross-protocol replay (§6) | MUST | 6 | **positive:** `unit/verify` [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L174). **negative:** `unit/verify` [`TestOSPFAuthType3RequiresProtocolIDSuffix`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L320) |
| `RFC7474-4.1-1` | On send, when multiple keys match, select the key with the most recent SendLifetimeStart to enable graceful rollover (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7474-2-7` | Maintain a separate OSPF boot count in non-volatile storage (decouples SNMP and OSPF reinitialization) (§2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7474-2-8` | Increment the boot count on cold restart and, if the low-order word wraps, to keep the aggregate sequence number strictly increasing (§2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7474 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7474-2-1`](#rfc7474-2-1)

Carry the 64-bit sequence number in the 8 octets following the OSPFv2 packet and include it when computing the digest (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L223) | unit/verify | unproven |
| positive | [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L172) | unit/verify | unproven |

### [`RFC7474-2-2`](#rfc7474-2-2)

Compose the 64-bit value as high-order boot count + low-order strictly increasing counter (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSetBootCountSeedsSequence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L380) | unit/verify | unproven |

### [`RFC7474-2-3`](#rfc7474-2-3)

Increment the lower-order 32-bit sequence number for every OSPF packet sent (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOSPFAuthESNCounterWrapAdvancesBootCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L403) | unit/verify | unproven |

### [`RFC7474-2-4`](#rfc7474-2-4)

Preserve the strictly increasing property of the aggregate sequence number for the deployed life of the router, including cold restarts (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBootCountMonotonicAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L342) | unit/verify | unproven |

### [`RFC7474-2-5`](#rfc7474-2-5)

On receive, accept only when the sequence number is greater than the last accepted packet **of that type** from that neighbor; otherwise drop as a replay (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L133) | unit/verify | unproven |
| positive | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L128) | unit/verify | unproven |

### [`RFC7474-2-6`](#rfc7474-2-6)

Track the accepted-sequence high-water mark per neighbor and per OSPF packet type (allows out-of-order arrival across types, §2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L134) | unit/verify | unproven |
| positive | [`TestOSPFAuthReplayPerType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L164) | unit/verify | unproven |

### [`RFC7474-3-1`](#rfc7474-3-1)

Set the OSPF Authentication field per AuType 3: 24-bit reserved `0`, 8-bit Auth Data Len, 32-bit Key ID in the former sequence-number position (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L252) | unit/verify | unproven |
| positive | [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L162) | unit/verify | unproven |

### [`RFC7474-5-1`](#rfc7474-5-1)

Include the 64-bit sequence number in the First-Hash along with the Authentication Trailer and OSPF packet (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthType3SequenceTamperRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L284) | unit/verify | unproven |
| positive | [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L173) | unit/verify | unproven |

### [`RFC7474-5-2`](#rfc7474-5-2)

Initialize the first 4 octets of `Apad` to the packet's IP source address (send and receive), remainder 0x878FE1F3 (§5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthType3SourceBinding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L193) | unit/verify | unproven |
| positive | [`TestOSPFAuthType3SourceBinding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L197) | unit/verify | unproven |

### [`RFC7474-6-1`](#rfc7474-6-1)

Append the two-octet OSPFv2 Cryptographic Protocol ID to the authentication key prior to use, to block cross-protocol replay (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthType3RequiresProtocolIDSuffix`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L320) | unit/verify | unproven |
| positive | [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L174) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 7474, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7474, so its obligations are stated where they were written.
