# RFC 5561 - LDP Capabilities

Partial. Every requirement this repository extracted from RFC 5561, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 5.9% | 1 of 17 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 17 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 17 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 100.0% | 2 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 17 | of 18 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 17 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 17 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 17 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 17 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 94.1% | 16 of 17 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 17 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 18 |
| Gated MUST-level | 17 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 16 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 1 |
| Discrimination records | 2 |
| Summary | `rfc/short/rfc5561.md` |
| Requirement shard | `rfc/requirements/rfc5561.md` |
| RFC text | `rfc/full/rfc5561.txt` |

## Enrolment

Enrolled: ze implements base LDP (RFC 5036) only and has no RFC 5561 capability mechanism. 17 MUST-level rows stand after the 2026-09-21 extraction walk. On 2026-09-21 the owner ruled that every MUST is a requirement until he declines it, so all 17 carry {gap} naming one of two skeleton specs, plan/spec-ldp-capability-advertisement.md and plan/spec-ldp-capability-status-codes.md. Four of them read {not-applicable} against the absent code path until that ruling: RFC5561-3-1 (the F-bit of a Capability Parameter TLV MUST be 0), RFC5561-3-2 (MUST NOT include two instances of one Capability Parameter), RFC5561-4-1 (Backward Compatibility TLVs MUST NOT be included in Capability messages) and RFC5561-4-2 (MUST NOT send a Capability message to a peer that did not advertise Dynamic Capability Announcement). Each cites the absence of any capability-TLV or Capability-message code path in internal/plugins/ldp/wire.go (EncodeInit:293, EncodeTLV:166, DecodeTLV:174, DecodeInit:324) and internal/plugins/ldp/session.go:419 (no 0x0202 encoder; no 0x0506 capability constant exists). The other 13 rows the walk added from §3, §6, §8, §9 and §10, and none carries a test. Three rows the walk deleted because RFC 5561 states no such rule: RFC5561-x-1 (reserved bits zero; the document describes no Reserved field), RFC5561-4-3 (SHOULD support the capability) and RFC5561-3-3 (MAY tear the session down). The two earlier claims this reason made are corrected by the document's own text: §3 sets the F-bit to 0, not 1, and leaves the U-bit to each capability document, and the Dynamic Capability Announcement Parameter is a MAY in §9, now RFC5561-9-4.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

None of the document's behavior exists yet. Ze speaks base LDP (RFC 5036) only: `EncodeInit` in [`internal/plugins/ldp/wire.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/wire.go) writes no Capability Parameter TLV, `DecodeTLV` recognizes no capability code point, and no Capability message (0x0202) encoder or decoder exists in [`internal/plugins/ldp/session.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/session.go). The row is here so the 17 gated MUSTs are disclosed rather than hidden behind an absent row.

**What the ledger says remains**

Every one of the 17 MUST-level rows is a `{gap}` scheduled by one of two specs. Capability advertisement ([`plan/spec-ldp-capability-advertisement.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ldp-capability-advertisement.md)) owes the send side: the Capability Parameter TLV with its U-bit, F-bit, S-bit and length rules ([`RFC5561-3-1`](#rfc5561-3-1), 3-2, 6-1, 6-3, 9-1, 9-2), the Capability message and the Dynamic Capability Announcement gate on it ([`RFC5561-4-1`](#rfc5561-4-1), 4-2, 9-3), and the IPv4 label distribution default ([`RFC5561-10-1`](#rfc5561-10-1)). Capability status codes ([`plan/spec-ldp-capability-status-codes.md`](https://github.com/ze-software/ze/blob/main/plan/spec-ldp-capability-status-codes.md)) owes the receive side: the Malformed TLV Value and Unsupported Capability status codes, the E-bit of 0 on that Notification, and the Returned TLVs TLV that carries the offending Capability Parameter or the unknown TLV ([`RFC5561-3-5`](#rfc5561-3-5), 6-2, 8-1, 8-2, 8-3, 8-4).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 16 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **17** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC5561-6-4`](#rfc5561-6-4)

**Annotated instead of tested (16):** [`RFC5561-3-1`](#rfc5561-3-1), [`RFC5561-4-2`](#rfc5561-4-2), [`RFC5561-3-2`](#rfc5561-3-2), [`RFC5561-3-5`](#rfc5561-3-5), [`RFC5561-4-1`](#rfc5561-4-1), [`RFC5561-6-1`](#rfc5561-6-1), [`RFC5561-6-2`](#rfc5561-6-2), [`RFC5561-6-3`](#rfc5561-6-3), [`RFC5561-8-1`](#rfc5561-8-1), [`RFC5561-8-2`](#rfc5561-8-2), [`RFC5561-8-3`](#rfc5561-8-3), [`RFC5561-8-4`](#rfc5561-8-4), [`RFC5561-9-1`](#rfc5561-9-1), [`RFC5561-9-2`](#rfc5561-9-2), [`RFC5561-9-3`](#rfc5561-9-3), [`RFC5561-10-1`](#rfc5561-10-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5561-3-1` | F-bit: Forward unknown TLV bit, as described in [RFC5036]. The value of this bit MUST be 0 since a Capability Parameter TLV is sent only in Initialization and Capability messages, which are not forwarded. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| `RFC5561-9-4` | The Dynamic Capability Announcement Parameter MAY be included by an LDP speaker in an Initialization message to signal its peer that the speaker is capable of processing Capability messages. (§9) | MAY | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5561-6-4` | If the U-bit is 1, then the speaker MUST silently ignore the Capability Parameter and allow the session to be established. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC5561UnknownCapabilityWithUBitSetIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/capability_rfc5561_test.go#L74). **negative:** `unit/verify` [`TestRFC5561UnknownCapabilityWithUBitSetDrawsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/capability_rfc5561_test.go#L112) |
| `RFC5561-4-2` | An LDP speaker MUST NOT send a Capability message to a peer unless its peer advertised the Dynamic Capability Announcement capability in its session Initialization message. (§7) | MUST NOT | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| `RFC5561-3-2` | An LDP speaker MUST NOT include more than one instance of a Capability Parameter (as identified by the same TLV code point) in an Initialization or Capability message. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| `RFC5561-3-5` | If an LDP speaker receives more than one instance of the same Capability Parameter type in a message, it SHOULD send a Notification message to the peer before terminating the session with the peer. The Status Code in the Status TLV of the Notification message MUST be Malformed TLV value, and the message SHOULD contain the second Capability Parameter TLV of the same type (code point) that is received in the message. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| `RFC5561-4-1` | Note that Backward Compatibility TLVs (see Section 3.1) MUST NOT be included in Capability messages. (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| `RFC5561-6-1` | The S-bit of a Capability Parameter in an Initialization message MUST be 1 and SHOULD be ignored on receipt. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| `RFC5561-6-2` | The Status Code in the Status TLV of the Notification message MUST be Unsupported Capability, and the message SHOULD contain the unsupported capability (see Section 8 for more details). (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| `RFC5561-6-3` | An LDP speaker that supports capability advertisement and includes a Capability Parameter in its Initialization message MUST set the TLV U-bit to 0 or 1, as specified by Capability document. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| `RFC5561-8-1` | The E-bit of the Status TLV carried in a Notification message that includes this status code MUST be set to 0. (§8, Unsupported Capability) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| `RFC5561-8-2` | When the Notification message specifies the unsupported capabilities, it MUST include a Returned TLVs TLV. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| `RFC5561-8-3` | The Returned TLVs TLV MUST include only the Capability Parameters for unsupported capabilities, and the Capability Parameter for each such capability SHOULD be encoded as received from the peer. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| `RFC5561-8-4` | When the Notification message specifies the TLV that was unknown, it MUST include the unknown TLV in a Returned TLVs TLV. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| `RFC5561-9-1` | (0x0506) \| Length (1) \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \|1\| Reserved \| +-+-+-+-+-+-+-+-+ The value of the U-bit for the Dynamic Capability Announcement Parameter TLV MUST be set to 1 so that a receiver MUST silently ignore this TLV if unknown to it, and continue processing the rest of the message. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| `RFC5561-9-2` | There is no "Capability Data" associated with this TLV and hence the TLV length MUST be set to 1. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| `RFC5561-9-3` | An LDP speaker MUST NOT include the Dynamic Capability Announcement Parameter in Capability messages sent to its peers. (§9) | MUST NOT | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| `RFC5561-10-1` | To ensure compatibility with an [RFC5036]-compliant peer, LDP implementations that support capability advertisement have label distribution for IPv4 enabled until it is explicitly disabled and MUST assume that their peers do as well. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5561-3-1`](#rfc5561-3-1) F-bit: Forward unknown TLV bit, as described in [RFC5036]. The value of this bit MUST be 0 since a Capability Parameter TLV is sent only in Initialization and Capability messages, which are not forwarded. (§3) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| [`RFC5561-4-2`](#rfc5561-4-2) An LDP speaker MUST NOT send a Capability message to a peer unless its peer advertised the Dynamic Capability Announcement capability in its session Initialization message. (§7) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| [`RFC5561-3-2`](#rfc5561-3-2) An LDP speaker MUST NOT include more than one instance of a Capability Parameter (as identified by the same TLV code point) in an Initialization or Capability message. (§3) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| [`RFC5561-3-5`](#rfc5561-3-5) If an LDP speaker receives more than one instance of the same Capability Parameter type in a message, it SHOULD send a Notification message to the peer before terminating the session with the peer. The Status Code in the Status TLV of the Notification message MUST be Malformed TLV value, and the message SHOULD contain the second Capability Parameter TLV of the same type (code point) that is received in the message. (§3) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| [`RFC5561-4-1`](#rfc5561-4-1) Note that Backward Compatibility TLVs (see Section 3.1) MUST NOT be included in Capability messages. (§4) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| [`RFC5561-6-1`](#rfc5561-6-1) The S-bit of a Capability Parameter in an Initialization message MUST be 1 and SHOULD be ignored on receipt. (§6) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| [`RFC5561-6-2`](#rfc5561-6-2) The Status Code in the Status TLV of the Notification message MUST be Unsupported Capability, and the message SHOULD contain the unsupported capability (see Section 8 for more details). (§6) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| [`RFC5561-6-3`](#rfc5561-6-3) An LDP speaker that supports capability advertisement and includes a Capability Parameter in its Initialization message MUST set the TLV U-bit to 0 or 1, as specified by Capability document. (§6) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| [`RFC5561-8-1`](#rfc5561-8-1) The E-bit of the Status TLV carried in a Notification message that includes this status code MUST be set to 0. (§8, Unsupported Capability) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| [`RFC5561-8-2`](#rfc5561-8-2) When the Notification message specifies the unsupported capabilities, it MUST include a Returned TLVs TLV. (§8) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| [`RFC5561-8-3`](#rfc5561-8-3) The Returned TLVs TLV MUST include only the Capability Parameters for unsupported capabilities, and the Capability Parameter for each such capability SHOULD be encoded as received from the peer. (§8) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| [`RFC5561-8-4`](#rfc5561-8-4) When the Notification message specifies the TLV that was unknown, it MUST include the unknown TLV in a Returned TLVs TLV. (§8) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/spec-ldp-capability-status-codes.md |
| [`RFC5561-9-1`](#rfc5561-9-1) (0x0506) \| Length (1) \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \|1\| Reserved \| +-+-+-+-+-+-+-+-+ The value of the U-bit for the Dynamic Capability Announcement Parameter TLV MUST be set to 1 so that a receiver MUST silently ignore this TLV if unknown to it, and continue processing the rest of the message. (§9) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| [`RFC5561-9-2`](#rfc5561-9-2) There is no "Capability Data" associated with this TLV and hence the TLV length MUST be set to 1. (§9) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| [`RFC5561-9-3`](#rfc5561-9-3) An LDP speaker MUST NOT include the Dynamic Capability Announcement Parameter in Capability messages sent to its peers. (§9) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |
| [`RFC5561-10-1`](#rfc5561-10-1) To ensure compatibility with an [RFC5036]-compliant peer, LDP implementations that support capability advertisement have label distribution for IPv4 enabled until it is explicitly disabled and MUST assume that their peers do as well. (§10) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/spec-ldp-capability-advertisement.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5561-3-1`](#rfc5561-3-1)

F-bit: Forward unknown TLV bit, as described in [RFC5036]. The value of this bit MUST be 0 since a Capability Parameter TLV is sent only in Initialization and Capability messages, which are not forwarded. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-3-1, so no unit is bound to it.

### [`RFC5561-6-4`](#rfc5561-6-4)

If the U-bit is 1, then the speaker MUST silently ignore the Capability Parameter and allow the session to be established. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviours: (1) reacting to an unsupported U=1 Capability Parameter (e.g. sending a Notification) instead of silently ignoring it, (2) refusing the session because of it. (1): TestRFC5561UnknownCapabilityWithUBitSetDrawsNoNotification fails on processMessages returning an error and on expectNoPDU seeing any PDU on the pipe. (2): TestRFC5561UnknownCapabilityWithUBitSetIsIgnored requires rx.State() == StateOperational and the peer's 30s keepalive negotiated, and that DecodeInit still reads the Common Session Parameters beside the parameter. Every capabilityCases code point runs both.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC5561UnknownCapabilityWithUBitSetDrawsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/capability_rfc5561_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestRFC5561UnknownCapabilityWithUBitSetIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/capability_rfc5561_test.go#L74) | unit/verify | revert, verified |

### [`RFC5561-4-2`](#rfc5561-4-2)

An LDP speaker MUST NOT send a Capability message to a peer unless its peer advertised the Dynamic Capability Announcement capability in its session Initialization message. (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-4-2, so no unit is bound to it.

### [`RFC5561-3-2`](#rfc5561-3-2)

An LDP speaker MUST NOT include more than one instance of a Capability Parameter (as identified by the same TLV code point) in an Initialization or Capability message. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-3-2, so no unit is bound to it.

### [`RFC5561-3-5`](#rfc5561-3-5)

If an LDP speaker receives more than one instance of the same Capability Parameter type in a message, it SHOULD send a Notification message to the peer before terminating the session with the peer. The Status Code in the Status TLV of the Notification message MUST be Malformed TLV value, and the message SHOULD contain the second Capability Parameter TLV of the same type (code point) that is received in the message. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-3-5, so no unit is bound to it.

### [`RFC5561-4-1`](#rfc5561-4-1)

Note that Backward Compatibility TLVs (see Section 3.1) MUST NOT be included in Capability messages. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-4-1, so no unit is bound to it.

### [`RFC5561-6-1`](#rfc5561-6-1)

The S-bit of a Capability Parameter in an Initialization message MUST be 1 and SHOULD be ignored on receipt. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-6-1, so no unit is bound to it.

### [`RFC5561-6-2`](#rfc5561-6-2)

The Status Code in the Status TLV of the Notification message MUST be Unsupported Capability, and the message SHOULD contain the unsupported capability (see Section 8 for more details). (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-6-2, so no unit is bound to it.

### [`RFC5561-6-3`](#rfc5561-6-3)

An LDP speaker that supports capability advertisement and includes a Capability Parameter in its Initialization message MUST set the TLV U-bit to 0 or 1, as specified by Capability document. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-6-3, so no unit is bound to it.

### [`RFC5561-8-1`](#rfc5561-8-1)

The E-bit of the Status TLV carried in a Notification message that includes this status code MUST be set to 0. (§8, Unsupported Capability)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-8-1, so no unit is bound to it.

### [`RFC5561-8-2`](#rfc5561-8-2)

When the Notification message specifies the unsupported capabilities, it MUST include a Returned TLVs TLV. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-8-2, so no unit is bound to it.

### [`RFC5561-8-3`](#rfc5561-8-3)

The Returned TLVs TLV MUST include only the Capability Parameters for unsupported capabilities, and the Capability Parameter for each such capability SHOULD be encoded as received from the peer. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-8-3, so no unit is bound to it.

### [`RFC5561-8-4`](#rfc5561-8-4)

When the Notification message specifies the TLV that was unknown, it MUST include the unknown TLV in a Returned TLVs TLV. (§8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-8-4, so no unit is bound to it.

### [`RFC5561-9-1`](#rfc5561-9-1)

(0x0506) | Length (1) | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ |1| Reserved | +-+-+-+-+-+-+-+-+ The value of the U-bit for the Dynamic Capability Announcement Parameter TLV MUST be set to 1 so that a receiver MUST silently ignore this TLV if unknown to it, and continue processing the rest of the message. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-9-1, so no unit is bound to it.

### [`RFC5561-9-2`](#rfc5561-9-2)

There is no "Capability Data" associated with this TLV and hence the TLV length MUST be set to 1. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-9-2, so no unit is bound to it.

### [`RFC5561-9-3`](#rfc5561-9-3)

An LDP speaker MUST NOT include the Dynamic Capability Announcement Parameter in Capability messages sent to its peers. (§9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-9-3, so no unit is bound to it.

### [`RFC5561-10-1`](#rfc5561-10-1)

To ensure compatibility with an [RFC5036]-compliant peer, LDP implementations that support capability advertisement have label distribution for IPv4 enabled until it is explicitly disabled and MUST assume that their peers do as well. (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-10-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc5561.txt |
| Source fingerprint | f521ece72256d999 |
| Record | rfc/extraction/rfc5561.json |
| Mapped sentences | 17 |
| Declined as scope | 4 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 1 | walked | not stated |
| `3` | not stated | 4 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `4` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 6 | walked | not stated |
| `7` | not stated | 1 | walked | not stated |
| `8` | not stated | 4 | walked | not stated |
| `9` | not stated | 3 | walked | not stated |
| `10` | not stated | 1 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `14.1` | not stated | 0 | walked | not stated |
| `14.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an LDP capability document, a document role and not a wire role: Section 2.1 addresses 'a document that' specifies an LDP enhancement, and the sentence obliges that document to describe how its capability data is interpreted and processed. Ze publishes no capability document. The producer that acts as the role in this tree is rfc/full/rfc5561.txt, whose Section 9 is itself such a document for the Dynamic Capability Announcement TLV. Ze would CONSUME the result: internal/plugins/ldp/wire.go decodes what such a document defines. | The capability document MUST also describe the interpretation and processing of associated capability data, if present. |
| `3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of the document specifying a capability, a document role and not a wire role: the sentence obliges that document to describe how the Capability Data is interpreted and processed. Ze publishes no capability document. The producer that acts as the role in this tree is rfc/full/rfc5561.txt, whose Section 9 defines the Dynamic Capability Announcement TLV and states it carries no Capability Data. Ze would CONSUME the result: internal/plugins/ldp/wire.go decodes what such a document defines. | The method for interpreting and processing this data is specific to the TLV code point and MUST be described in the document specifying the capability. |
| `6:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of the document specifying procedures for a capability, a document role and not a wire role: Section 6 first says 'It is the responsibility of the capability designer to specify the behavior', then obliges that document to describe the behavior when a peer did not advertise the capability. Ze publishes no capability document. The producer that acts as the role in this tree is rfc/full/rfc5561.txt, whose Section 9 specifies the procedures for the Dynamic Capability Announcement capability. Ze would CONSUME the result in internal/plugins/ldp/session.go. | The document specifying procedures for the capability MUST describe the behavior in this situation. |
| `6:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 states the same obligation twice: site 6:3 carries it for the speaker whose peer never advertised the capability, and this site repeats it for the speaker that meets an unsupported capability with the U-bit 0. Both sentences read 'The Status Code in the Status TLV of the Notification message MUST be Unsupported Capability, and the message SHOULD contain the unsupported capability'. Site 6:3 maps RFC5561-6-2. | The Status Code in the Status TLV of the Notification message MUST be Unsupported Capability, and the message SHOULD contain the unsupported capability (see Section 8 for more details). |

## Superseded

No document obsoletes RFC 5561, so its obligations are stated where they were written.
