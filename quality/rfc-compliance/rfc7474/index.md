# RFC 7474 - Security Extensions for OSPFv2 when Using Manual Key Management

Experimental. Every requirement this repository extracted from RFC 7474, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 9 of 12 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 16.7% | 2 of 12 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 12 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 12 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 64.9% | 24 of 37 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 12 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 12 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 12 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 12 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 12 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 8.3% | 1 of 12 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 12 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 12 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 38 |
| Tagged units | 37 |
| Recorded audit verdicts | 11 |
| Discrimination records | 24 |
| Summary | `rfc/short/rfc7474.md` |
| Requirement shard | `rfc/requirements/rfc7474.md` |
| RFC text | `rfc/full/rfc7474.txt` |

## Enrolment

Enrolled: OSPFv2 manual-key security extension (AuType 3), including durable boot counts, packet-type replay protection, source-bound Apad, protocol-ID key derivation and zero-padding Ko to the hash block size. Section 8 also requires replacement of authentication keys after non-volatile storage loss or router replacement.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

AuType-3 cryptographic authentication with extended sequence numbers, per-packet-type replay protection, source-bound Apad and protocol-ID key derivation.

**What the ledger says remains**

Experimental. One MUST row carries {gap}. Section 8 requires authentication-key replacement after non-volatile storage loss or router replacement. An absent durable counter starts at one and cannot distinguish first use from total storage loss; startup does not automatically rotate the configured shared secrets.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the gated population |
| Annotated (including scoped evidence) | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **12** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (9):** [`RFC7474-2-1`](#rfc7474-2-1), [`RFC7474-2-4`](#rfc7474-2-4), [`RFC7474-2-5`](#rfc7474-2-5), [`RFC7474-2-6`](#rfc7474-2-6), [`RFC7474-3-1`](#rfc7474-3-1), [`RFC7474-5-1`](#rfc7474-5-1), [`RFC7474-5-2`](#rfc7474-5-2), [`RFC7474-6-1`](#rfc7474-6-1), [`RFC7474-6-2`](#rfc7474-6-2)

**Annotated (including scoped evidence) (3):** [`RFC7474-2-2`](#rfc7474-2-2), [`RFC7474-2-3`](#rfc7474-2-3), [`RFC7474-8-1`](#rfc7474-8-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7474-2-1` | Since there is no room in the OSPFv2 packet for a 64-bit sequence number, it will occupy the 8 octets following the OSPFv2 packet and MUST be included when calculating the OSPFv2 packet digest. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L177). **positive:** `unit/verify` [`TestRFC7474SequenceIncludedInDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L102). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L228). **negative:** `unit/verify` [`TestRFC7474SequenceChangeBreaksDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L142) |
| `RFC7474-2-2` | the OSPFv2 sequence number is expanded to 64 bits with the least significant 32-bit value containing a strictly increasing sequence number and the most significant 32-bit value containing the boot count (§2) | MUST | 2 | **positive:** `unit/verify` [`TestSetBootCountSeedsSequence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L430). **negative:** no negative test. **{single-polarity}:** the 64-bit composition (boot count in the high word, per-packet counter in the low word) is a numeric construction property with no violating input to feed a negative test |
| `RFC7474-2-3` | The lower-order 32-bit sequence number MUST be incremented for every OSPF packet sent by the OSPF router. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestOSPFAuthESNCounterWrapAdvancesBootCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L458). **positive:** `unit/verify` [`TestRFC7474EverySentPacketIncrementsSequence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_replay_test.go#L146). **negative:** no negative test. **{single-polarity}:** a monotonic send-side counter increment has no observable violating case, since every sent crypto packet advances it (and bumps the boot word on wrap) |
| `RFC7474-2-4` | OSPF routers implementing this specification MUST use available mechanisms to preserve the sequence number's strictly increasing property for the deployed life of the OSPFv2 router (including cold restarts). (§2) | MUST | 2 | **positive:** `unit/verify` [`TestBootCountMonotonicAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L394). **positive:** `unit/verify` [`TestOSPFESNWrapPersistsBeforeRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_state_store_gate_test.go#L186). **negative:** `unit/verify` [`TestOSPFESNWrapFailureRefusesPackets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_state_store_gate_test.go#L259) |
| `RFC7474-2-5` | Upon reception, the sequence number MUST be greater than the sequence number in the last OSPF packet of that type accepted from the sending OSPF neighbor. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L133). **positive:** `unit/verify` [`TestRFC7474GreaterExtendedSequenceAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_replay_test.go#L69). **negative:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L138). **negative:** `unit/verify` [`TestOSPFAuthReplayEqualSequenceByAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L186). **negative:** `unit/verify` [`TestRFC7474NotGreaterExtendedSequenceDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_replay_test.go#L82) |
| `RFC7474-2-6` | Upon reception, the sequence number MUST be greater than the sequence number in the last OSPF packet of that type accepted from the sending OSPF neighbor. Otherwise, the OSPF packet is considered a replayed packet and dropped. OSPF packets of different types may arrive out of order if they are prioritized as recommended in [RFC4222]. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestOSPFAuthReplayPerType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L212). **positive:** `unit/verify` [`TestRFC7474ReplayMarkPerNeighborAndType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_replay_test.go#L96). **negative:** `unit/verify` [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L139). **negative:** `unit/verify` [`TestRFC7474ReplayedPacketDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_replay_test.go#L110) |
| `RFC7474-3-1` | The sequence number is removed and the Key ID is extended to 32 bits and moved to the former position of the sequence number. Additionally, the 64-bit sequence number is moved to the first 64 bits following the OSPFv2 packet and is protected by the authentication digest. These additional 64 bits or 8 octets are included in the IP header length but not the OSPF header packet length. Finally, the 0 field at the start of the OSPFv2 header authentication is extended from 16 bits to 24 bits. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L167). **positive:** `unit/verify` [`TestRFC7474SequenceIncludedInDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L106). **negative:** `unit/verify` [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L257). **negative:** `unit/verify` [`TestRFC7474SequenceChangeBreaksDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L145) |
| `RFC7474-5-1` | the 64-bit sequence number will be included in the First-Hash along with the Authentication Trailer and OSPF packet (§5) | MUST | 5 | **positive:** `unit/verify` [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L178). **negative:** `unit/verify` [`TestOSPFAuthType3SequenceTamperRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L289) |
| `RFC7474-5-2` | OSPF routers sending OSPF packets must initialize the first 4 octets of Apad to the value of the IP source address that would be used when sending the OSPFv2 packet. The remainder of Apad will contain the value 0x878FE1F3 repeated (L - 4)/4 times, where L is the length of the hash, measured in octets. The basic idea is to incorporate the IP source address from the IP header in the cryptographic authentication computation so that any change of IP source address in a replayed packet can be detected. When an OSPF packet is received, implementations MUST initialize the first 4 octets of Apad to the IP source address from the IP header of the incoming OSPFv2 packet. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestOSPFAuthType3SourceBinding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L202). **positive:** `unit/verify` [`TestRFC7474ReceiveApadFromIPHeaderSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_apad_source_test.go#L266). **positive:** `unit/verify` [`TestRFC7474SentApadIsInterfaceSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_apad_source_test.go#L216). **negative:** `unit/verify` [`TestOSPFAuthType3SourceBinding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L198). **negative:** `unit/verify` [`TestRFC7474ReceiveApadOtherThanIPHeaderSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_apad_source_test.go#L275). **negative:** `unit/verify` [`TestRFC7474SentApadNotRouterIDOrZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_apad_source_test.go#L226) |
| `RFC7474-6-1` | In order to prevent cross-protocol replay attacks for protocols sharing common keys, the two-octet OSPFv2 Cryptographic Protocol ID is appended to the authentication key prior to use. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L179). **negative:** `unit/verify` [`TestOSPFAuthType3RequiresProtocolIDSuffix`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L325) |
| `RFC7474-6-2` | When XORing Ko and Ipad of Opad, Ko MUST be padded with zeros to the length of Ipad or Opad (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC7474KoZeroPaddedToBlockSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7474_auth_test.go#L52). **negative:** `unit/verify` [`TestRFC7474KoNonZeroPadRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7474_auth_test.go#L80) |
| `RFC7474-8-1` | If the non-volatile storage is ever repaired or upgraded such that the contents are lost or the OSPFv2 router is replaced, the authentication keys MUST be changed to prevent replay attacks (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** startup does not rotate or refuse the configured shared secrets when the durable boot count is lost; internal/plugins/ospf/auth_keystore.go::loadOSPFBootCount restarts an absent counter at one, which cannot be told apart from first use. Disclosed in this summary's Support remaining row |
| `RFC7474-4-1` | For packet reception, the key validity interval as defined by AcceptLifetimeStart and AcceptLifetimeEnd must include the current time. (§4) | SHOULD | 4 | **positive:** `unit/verify` [`TestVerifyRejectsOutsideAcceptLifetime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L507). **negative:** `unit/verify` [`TestVerifyRejectsOutsideAcceptLifetime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L501) |
| `RFC7474-4.1-1` | If there are still multiple keys that match, the key with the most recent SendLifetimeStart will be selected. This will facilitate graceful key rollover. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC7474-2-7` | This is achieved by maintaining a boot count in non- volatile storage and incrementing it each time the OSPF router loses its prior sequence number state. The SNMPv3 snmpEngineBoots variable [RFC3414] MAY be used for this purpose. However, maintaining a separate boot count solely for OSPF sequence numbers has the advantage of decoupling SNMP reinitialization and OSPF reinitialization. Also, in the rare event that the lower-order 32-bit sequence number wraps, the boot count can be incremented to preserve the strictly increasing property of the aggregate sequence number. Hence, a separate OSPF boot count is RECOMMENDED. (§2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7474-2-8` | This is achieved by maintaining a boot count in non- volatile storage and incrementing it each time the OSPF router loses its prior sequence number state. The SNMPv3 snmpEngineBoots variable [RFC3414] MAY be used for this purpose. However, maintaining a separate boot count solely for OSPF sequence numbers has the advantage of decoupling SNMP reinitialization and OSPF reinitialization. Also, in the rare event that the lower-order 32-bit sequence number wraps, the boot count can be incremented to preserve the strictly increasing property of the aggregate sequence number. (§2) | RECOMMENDED | 2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7474-8-1`](#rfc7474-8-1) If the non-volatile storage is ever repaired or upgraded such that the contents are lost or the OSPFv2 router is replaced, the authentication keys MUST be changed to prevent replay attacks (§8) | {gap}, no test | startup does not rotate or refuse the configured shared secrets when the durable boot count is lost; internal/plugins/ospf/auth_keystore.go::loadOSPFBootCount restarts an absent counter at one, which cannot be told apart from first use. Disclosed in this summary's Support remaining row |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7474-2-1`](#rfc7474-2-1)

Since there is no room in the OSPFv2 packet for a 64-bit sequence number, it will occupy the 8 octets following the OSPFv2 packet and MUST be included when calculating the OSPFv2 packet digest. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c23 re-judge (independent judge). Clause 'occupy the 8 octets following the OSPFv2 packet': TestRFC7474SequenceIncludedInDigest asserts signed[plen:plen+8] == literal 00 00 00 05 00 00 00 09 and the total length plen+8+32. Clause 'MUST be included when calculating the digest': the same unit asserts the digest equals an independent HMAC-SHA-256 (Ko = key || 00 01 zero-padded, hand-built HMAC) over packet || sequence || Apad(src, 0x878FE1F3 fill); the negative TestRFC7474SequenceChangeBreaksDigest flips one sequence octet after signing and Verify refuses, unchanged control verifies. Overlay leaving the sequence out of the hash on both Sign and Verify reds both new units while TestOSPFAuthType3SequenceTrailer stays green (scratch/c23/a3), which was the earlier finding. Observed-red records on Sign and Verify. HEAD units supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsExtraTrailerBytes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L228) | unit/verify | unproven |
| negative | [`TestRFC7474SequenceChangeBreaksDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L142) | unit/verify | revert, verified |
| positive | [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L177) | unit/verify | unproven |
| positive | [`TestRFC7474SequenceIncludedInDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L102) | unit/verify | revert, verified |

### [`RFC7474-2-2`](#rfc7474-2-2)

the OSPFv2 sequence number is expanded to 64 bits with the least significant 32-bit value containing a strictly increasing sequence number and the most significant 32-bit value containing the boot count (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: boot count and per-packet counter swapped or merged. TestSetBootCountSeedsSequence asserts signKey's seq == 0x1234<<32|1 after setBootCount(0x1234), red on any other composition. Single-polarity marker: a construction property with no violating input.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestSetBootCountSeedsSequence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L430) | unit/verify | unproven |

### [`RFC7474-2-3`](#rfc7474-2-3)

The lower-order 32-bit sequence number MUST be incremented for every OSPF packet sent by the OSPF router. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment after host-independent fixture repair; runtime renewal is not claimed here. RFC 7474 section 2: 'The lower-order 32-bit sequence number MUST be incremented for every OSPF packet sent by the OSPF router.' Read the full RFC, current checklist, derived row and both current positive carriers. internal/plugins/ospf/rfc7474_replay_test.go::TestRFC7474EverySentPacketIncrementsSequence installs installOSPFAddressBackend before rfc7474Store configures its key chain, calls engine.signPacket three times, and asserts the verified wire sequences are exactly 7<<32|1, 7<<32|2 and 7<<32|3. rfc7474SentSequence verifies against the fixture's explicit 192.0.2.1, not cached signer state, and separately rejects zero and 192.0.2.2, eliminating the prior host-source confound without weakening the counter assertion. internal/plugins/ospf/auth_keystore_test.go::TestOSPFAuthESNCounterWrapAdvancesBootCount independently seeds low word 0xffffffff and durable boot 0x1234, then asserts 0x1235<<32 followed by 0x1235<<32|1 and strict increase. Producer internal/plugins/ospf/auth_keystore.go::signKey computes c = s.sendSeq[iface] + 1, durably advances the boot count on wrap, composes the two words and stores c; auth_wiring.go::signPacket signs that sequence. installAuthHooks installs the same signer, and transport.go::SendPacket and SendPacketRouted both invoke it without packet-type branching. Four questions: the oracle is the RFC's exact increment, not an inequality accepting skipped increments; a frozen or repeated low word fails the exact-value assertions; the fixture isolates address acquisition before configuration and uses a valid explicit source; ordinary and wrap increments cover this row's complete obligation at the shared signer boundary. Scope remains the historical AuType-3 signing/counter claim: the tagged send test invokes the signer with three Hellos, not a five-packet-type transport or external-peer integration test. Two positive tagged units, no negative; the existing single-polarity annotation is justified as a send-side construction with no violating input to submit, not as an assertion that a broken counter is unobservable. The existing native record disables the whole signKey body and proves reachability only; renew its exact tagged unit after a clean baseline and separately observe a targeted counter-increment break before treating that break as runtime proof. No source, protocol, tag or requirement change is proposed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOSPFAuthESNCounterWrapAdvancesBootCount`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L458) | unit/verify | unproven |
| positive | [`TestRFC7474EverySentPacketIncrementsSequence`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_replay_test.go#L146) | unit/verify | revert, verified |

### [`RFC7474-2-4`](#rfc7474-2-4)

OSPF routers implementing this specification MUST use available mechanisms to preserve the sequence number's strictly increasing property for the deployed life of the OSPFv2 router (including cold restarts). (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a sequence reused after a cold restart or a low-word wrap. TestBootCountMonotonicAcrossRestart asserts the persisted boot count strictly increases per load; TestOSPFESNWrapPersistsBeforeRestart asserts the wrapped boot count is persisted (value 2) and a second engine's first packet (3<<32|1) sorts above the first engine's final packet; TestOSPFESNWrapFailureRefusesPackets asserts signPacket emits nothing when the durable increment is unavailable, corrupt, failed, exhausted, missing or non-increasing. Lost storage is the separate RFC7474-8-1 gap.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFESNWrapFailureRefusesPackets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_state_store_gate_test.go#L259) | unit/verify | unproven |
| positive | [`TestBootCountMonotonicAcrossRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L394) | unit/verify | unproven |
| positive | [`TestOSPFESNWrapPersistsBeforeRestart`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_state_store_gate_test.go#L186) | unit/verify | unproven |

### [`RFC7474-2-5`](#rfc7474-2-5)

Upon reception, the sequence number MUST be greater than the sequence number in the last OSPF packet of that type accepted from the sending OSPF neighbor. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. AuType 3 through authStore.verify. Positive: boot 1 low 5, boot 1 low 6, then boot 2 low 1 (smaller low word, greater boot) all accepted, so the comparison is 64-bit with the boot word first. Negative: after boot 2 low 1, boot 1 low 0xFFFFFFF0, boot 2 low 0 and an equal boot 2 low 1 are each dropped with reason 'replay'. Revert records on replayedSequence. The older AuType 2 tags stay.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L138) | unit/verify | revert, verified |
| negative | [`TestOSPFAuthReplayEqualSequenceByAuType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L186) | unit/verify | revert, verified |
| negative | [`TestRFC7474NotGreaterExtendedSequenceDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_replay_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L133) | unit/verify | revert, verified |
| positive | [`TestRFC7474GreaterExtendedSequenceAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_replay_test.go#L69) | unit/verify | revert, verified |

### [`RFC7474-2-6`](#rfc7474-2-6)

Upon reception, the sequence number MUST be greater than the sequence number in the last OSPF packet of that type accepted from the sending OSPF neighbor. Otherwise, the OSPF packet is considered a replayed packet and dropped. OSPF packets of different types may arrive out of order if they are prioritized as recommended in [RFC4222]. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29. Row span checked: it is the verbatim RFC 7474 section 2 text from 'Upon reception' to '[RFC4222].'; the last sentence is explanatory, not an obligation. Positive TestRFC7474ReplayMarkPerNeighborAndType: after 2.2.2.2 Hello 100, 3.3.3.3 Hello 5 and 2.2.2.2 LS Ack 5 are accepted (mark per neighbor and per type); TestOSPFAuthReplayPerType's prose now claims per type only. Negative TestRFC7474ReplayedPacketDropped: per-neighbor replays (lower and equal) dropped as 'replay', while 3.3.3.3 then accepts 6. Revert records on verify and replayedSequence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthReplay`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L139) | unit/verify | revert, verified |
| negative | [`TestRFC7474ReplayedPacketDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_replay_test.go#L110) | unit/verify | revert, verified |
| positive | [`TestOSPFAuthReplayPerType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L212) | unit/verify | revert, verified |
| positive | [`TestRFC7474ReplayMarkPerNeighborAndType`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_replay_test.go#L96) | unit/verify | revert, verified |

### [`RFC7474-3-1`](#rfc7474-3-1)

The sequence number is removed and the Key ID is extended to 32 bits and moved to the former position of the sequence number. Additionally, the 64-bit sequence number is moved to the first 64 bits following the OSPFv2 packet and is protected by the authentication digest. These additional 64 bits or 8 octets are included in the IP header length but not the OSPF header packet length. Finally, the 0 field at the start of the OSPFv2 header authentication is extended from 16 bits to 24 bits. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c23 re-judge (independent judge). Four sentences of sec 3, each asserted with literal octets in TestRFC7474SequenceIncludedInDigest: authentication field 00 00 00 28 01 02 03 04 (24-bit zero field, 32-bit Key ID 0x01020304 in the former sequence position); OSPF Packet Length == plen so the 8 sequence octets sit outside it; the sequence occupies the first 64 bits after the packet; 'protected by the authentication digest' by the independent-digest comparison, with the negative TestRFC7474SequenceChangeBreaksDigest (changed sequence refused) and the a3 overlay reding both. TestOSPFAuthCryptoRejectsKeyIDMismatch stays as the Key ID negative. Caveat outside this row's quote: the unit also pins Auth Data Len = 0x28 (8 + L), which is Ze's choice (Sign af[3] = 8 + l); RFC 7474 never states the value and sec 5 leaves the RFC 5709 process unchanged, whose sec 3.1 says the field is the hash length L. The verdict does not rest on that octet; the reading is an open question for the main thread.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthCryptoRejectsKeyIDMismatch`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L257) | unit/verify | unproven |
| negative | [`TestRFC7474SequenceChangeBreaksDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L145) | unit/verify | revert, verified |
| positive | [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L167) | unit/verify | unproven |
| positive | [`TestRFC7474SequenceIncludedInDigest`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc5709_rfc7474_digest_test.go#L106) | unit/verify | revert, verified |

### [`RFC7474-5-1`](#rfc7474-5-1)

the 64-bit sequence number will be included in the First-Hash along with the Authentication Trailer and OSPF packet (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a First-Hash that omits the 64-bit sequence. TestOSPFAuthType3SequenceTamperRejected flips one octet of signed[plen:plen+8] and asserts Verify rejects it, red if the sequence is outside the hash; TestOSPFAuthType3SequenceTrailer asserts the untampered round trip verifies and returns the sequence.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthType3SequenceTamperRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L289) | unit/verify | unproven |
| positive | [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L178) | unit/verify | unproven |

### [`RFC7474-5-2`](#rfc7474-5-2)

OSPF routers sending OSPF packets must initialize the first 4 octets of Apad to the value of the IP source address that would be used when sending the OSPFv2 packet. The remainder of Apad will contain the value 0x878FE1F3 repeated (L - 4)/4 times, where L is the length of the hash, measured in octets. The basic idea is to incorporate the IP source address from the IP header in the cryptographic authentication computation so that any change of IP source address in a replayed packet can be detected. When an OSPF packet is received, implementations MUST initialize the first 4 octets of Apad to the IP source address from the IP header of the incoming OSPFv2 packet. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent semantic rejudgment of the extracted fixture dependency, not a claim that unchanged tagged bodies became stale-unit. RFC 7474 section 5: 'OSPF routers sending OSPF packets must initialize the first 4 octets of Apad to the value of the IP source address that would be used when sending the OSPFv2 packet. The remainder of Apad will contain the value 0x878FE1F3 repeated (L - 4)/4 times, where L is the length of the hash, measured in octets. The basic idea is to incorporate the IP source address from the IP header in the cryptographic authentication computation so that any change of IP source address in a replayed packet can be detected. When an OSPF packet is received, implementations MUST initialize the first 4 octets of Apad to the IP source address from the IP header of the incoming OSPFv2 packet.' The immediately following RFC sentence also requires the same repeated remainder on reception. Read every current carrier: internal/plugins/ospf/rfc7474_apad_source_test.go::TestRFC7474SentApadIsInterfaceSource, TestRFC7474SentApadNotRouterIDOrZero, TestRFC7474ReceiveApadFromIPHeaderSource and TestRFC7474ReceiveApadOtherThanIPHeaderSourceDropped; internal/plugins/ospf/packet/auth_verify_test.go::TestOSPFAuthType3SourceBinding supplies both additional polarities with distinct matching-source and spoofed-source assertions. The shared installOSPFAddressBackend now registers/loads eth0 192.0.2.1/24, asserts address and mask through production lookup, and restores the prior backend after engine shutdown; rfc7474ApadEngine calls it before setConfig/openInterfaces. This helper is semantically relevant even though the four tagged bodies are unchanged and their file movement is machine-level SHIFTED under the audit's enclosing-unit model. Send positive compares the captured production-transport Hello digest with independent HMAC-SHA-256 over wire packet, eight-byte sequence, and literal source plus seven remainder words. Send negative independently rejects router ID 10.0.0.1, zero source and zero remainder. Receive positive sends a valid /24 backbone Hello bound to IP-header source 192.0.2.2 and asserts zero dispatcher drops plus one neighbor; receive negative changes only digest-bound Apad source to 192.0.2.3, local 192.0.2.1 or zero, retaining actual source 192.0.2.2, and asserts one drop plus no neighbor using fresh engines/replay stores. Producer auth_keystore.go::configure obtains the source via interfaceIPv4Address; signPacket passes it to packet.Sign; verifyPacket obtains rp.Src and authStore.verify passes it to packet.Verify. packet/auth_verify.go::apadSrc initializes the common remainder and overwrites its first four bytes; both Sign and Verify use it. Four questions: literal RFC source/remainder oracles rather than mere self-round-trip; wrong-source or wrong-remainder output fails independent assertions; mask, area, key, packet framing and actual receive source remain conforming so neighboring guards do not explain rejection; send source, common remainder and receive source obligations are covered collectively at these shared producers. Three positive and three negative tags, five distinct test functions; the packet-level pair is two different assertions, not one assertion counted twice. Historical capability scope remains HMAC-SHA-256 engine fixtures plus the shared Apad implementation; no new all-algorithm, routed-source or dynamic-address integration claim. No semantic regression from the extraction was found. Existing revert records prove only their named producer disables, not source-substitution sensitivity or current runtime success; helper-dependent clean runs and any chosen renewed red observations remain the parent's work.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthType3SourceBinding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L198) | unit/verify | revert, verified |
| negative | [`TestRFC7474ReceiveApadOtherThanIPHeaderSourceDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_apad_source_test.go#L275) | unit/verify | revert, verified |
| negative | [`TestRFC7474SentApadNotRouterIDOrZero`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_apad_source_test.go#L226) | unit/verify | revert, verified |
| positive | [`TestOSPFAuthType3SourceBinding`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L202) | unit/verify | revert, verified |
| positive | [`TestRFC7474ReceiveApadFromIPHeaderSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_apad_source_test.go#L266) | unit/verify | revert, verified |
| positive | [`TestRFC7474SentApadIsInterfaceSource`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc7474_apad_source_test.go#L216) | unit/verify | revert, verified |

### [`RFC7474-6-1`](#rfc7474-6-1)

In order to prevent cross-protocol replay attacks for protocols sharing common keys, the two-octet OSPFv2 Cryptographic Protocol ID is appended to the authentication key prior to use. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: hashing with the bare key. TestOSPFAuthType3RequiresProtocolIDSuffix builds a digest over the bare key and asserts Verify rejects it, with the real signer over the same key, seq and source as the verifying control; TestOSPFAuthType3SequenceTrailer round-trips the suffixed key.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOSPFAuthType3RequiresProtocolIDSuffix`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L325) | unit/verify | unproven |
| positive | [`TestOSPFAuthType3SequenceTrailer`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/auth_verify_test.go#L179) | unit/verify | unproven |

### [`RFC7474-6-2`](#rfc7474-6-2)

When XORing Ko and Ipad of Opad, Ko MUST be padded with zeros to the length of Ipad or Opad (§6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC7474KoNonZeroPadRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7474_auth_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC7474KoZeroPaddedToBlockSize`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc7474_auth_test.go#L52) | unit/verify | revert, verified |

### [`RFC7474-8-1`](#rfc7474-8-1)

If the non-volatile storage is ever repaired or upgraded such that the contents are lost or the OSPFv2 router is replaced, the authentication keys MUST be changed to prevent replay attacks (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7474-8-1, so no unit is bound to it.

### [`RFC7474-4-1`](#rfc7474-4-1)

For packet reception, the key validity interval as defined by AcceptLifetimeStart and AcceptLifetimeEnd must include the current time. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-30 (continuation 8, judge): TestVerifyRejectsOutsideAcceptLifetime verifies identical bytes at three clocks: refused accept-lifetime before the window opens (negative), accepted inside (positive), refused after it closes while successor Key ID 2 is live (negative). The successor was added because RFC 5709 s3.2 (row RFC5709-3.2-5) now keeps an expired LAST key; the closed-window refusal of a non-last key is exactly this row. Revert records on acceptsAt re-observed red for both polarities.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestVerifyRejectsOutsideAcceptLifetime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L501) | unit/verify | revert, verified |
| positive | [`TestVerifyRejectsOutsideAcceptLifetime`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/auth_keystore_test.go#L507) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc7474.txt |
| Source fingerprint | c360bb0c18ef9cb4 |
| Record | rfc/extraction/rfc7474.json |
| Mapped sentences | 8 |
| Declined as scope | 9 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | walked | not stated |
| `1` | not stated | 2 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 5 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 2 | walked | not stated |
| `4.1` | not stated | 2 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `5` | not stated | 3 | walked | not stated |
| `6` | not stated | 1 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 1 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust Legal Provisions boilerplate in the front matter. It binds the extractor of code components to a licence text and carries no protocol obligation. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence quotes RFC 2328, which it cites: the keyed MD5 obligation belongs to RFC 2328 Appendix D. RFC 7474 reports it as background for the replay problem it goes on to fix. | [RFC2328] states that implementations MUST offer keyed MD5 authentication. |
| `1:2` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The algorithm requirement belongs to RFC 6094, which the sentence cites; RFC 7474 only predicts that MD5 will be deprecated in favour of the RFC 5709 algorithms. | It is likely that this will be deprecated in favor of the stronger algorithms described in [RFC5709] and required in [RFC6094]. |
| `2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates in lowercase the boot-count mechanism of the MUST at site 2:5. The same section says of that MUST: 'This is achieved by maintaining a boot count in non-volatile storage and incrementing it each time the OSPF router loses its prior sequence number state.' | OSPFv2 implementations are required to retain the boot count in non-volatile storage for the deployment life of the OSPF router. |
| `4:1` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The send-lifetime bullet sits inside the advisory list opened by 'Generally, a key used for OSPFv2 packet authentication should satisfy the following requirements:'. The enclosing construction is a SHOULD, and the receive half of the same list is recorded as RFC7474-4-1. | o For packet transmission, the key validity interval as defined by SendLifetimeStart and SendLifetimeEnd must include the current time. |
| `4.1:1` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | A lead-in to the virtual-link key-selection list, which the same paragraph opens as 'Hence, the key should satisfy the following requirements:'. The obligation is carried by that SHOULD list, whose Peers-field bullet names the transit area ID and the virtual endpoint's router ID. | When R1 and R2 are connected to a virtual link, the Peers field must identify the virtual endpoint rather than the virtual link. |
| `4.1:2` | `advisory-in-context` (never bound Ze): the sentence advises on applying a rule stated elsewhere and adds no obligation of its own | The second half of the same lead-in, inside the same construction: 'Hence, the key should satisfy the following requirements:'. The transit area ID requirement is stated by the Peers-field bullet of that SHOULD list. | Since there may be virtual links to the same router, the transit area ID must be part of the identifier. |
| `5:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The computation obligation belongs to RFC 5709 Section 3.3, which the sentence cites. RFC 7474 changes only what this section then lists: the 64-bit sequence number in the First-Hash and the Apad value. | RFC 5709, Section 3.3 describes how the cryptographic authentication must be computed. |
| `5:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The receive-side half of the Apad initialization the send-side sentence at site 5:2 already states. RFC7474-5-2 declares one obligation for both directions: initialize the first 4 octets of Apad to the packet's IP source address on send and on receive. | When an OSPF packet is received, implementations MUST initialize the first 4 octets of Apad to the IP source address from the IP header of the incoming OSPFv2 packet. |

## Superseded

No document obsoletes RFC 7474, so its obligations are stated where they were written.
