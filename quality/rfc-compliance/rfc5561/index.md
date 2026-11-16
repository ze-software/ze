# RFC 5561 - LDP Capabilities

LDP. Every requirement listed for RFC 5561, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 17 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 17 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 17 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 17 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 2 | 2 of 2 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 16 | of 17 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 17 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 17 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 17 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 17 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 17 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 17 | of 18 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 17 mandatory requirements: they add up to 17. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | LDP |
| Automatic checks | Included |
| Requirements | 18 |
| Mandatory requirements (MUST level) | 17 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 16 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tests linked to requirements | 2 |
| Recorded audit verdicts | 1 |
| Fault test records | 2 |
| Summary | `rfc/short/rfc5561.md` |
| Requirement table | `rfc/requirements/rfc5561.md` |
| RFC text | `rfc/full/rfc5561.txt` |

## Automatic checks

Included in automatic checks: ze implements base LDP (RFC 5036) only and has no RFC 5561 capability mechanism. 17 MUST-level rows stand after the 2026-09-21 extraction walk. On 2026-09-21 the owner ruled that every MUST is a requirement until he declines it, so all 17 carry {gap} naming one of two skeleton specs, plan/post-release/features/spec-ldp-capability-advertisement.md and plan/release/fixes/spec-ldp-capability-status-codes.md. Four of them read {not-applicable} against the absent code path until that ruling: RFC5561-3-1 (the F-bit of a Capability Parameter TLV MUST be 0), RFC5561-3-2 (MUST NOT include two instances of one Capability Parameter), RFC5561-4-1 (Backward Compatibility TLVs MUST NOT be included in Capability messages) and RFC5561-4-2 (MUST NOT send a Capability message to a peer that did not advertise Dynamic Capability Announcement). Each cites the absence of any capability-TLV or Capability-message code path in internal/plugins/ldp/wire.go (EncodeInit:293, EncodeTLV:166, DecodeTLV:174, DecodeInit:324) and internal/plugins/ldp/session.go:419 (no 0x0202 encoder; no 0x0506 capability constant exists). The other 13 rows the walk added from §3, §6, §8, §9 and §10, and none carries a test. Three rows the walk deleted because RFC 5561 states no such rule: RFC5561-x-1 (reserved bits zero; the document describes no Reserved field), RFC5561-4-3 (SHOULD support the capability) and RFC5561-3-3 (MAY tear the session down). The two earlier claims this reason made are corrected by the document's own text: §3 sets the F-bit to 0, not 1, and leaves the U-bit to each capability document, and the Dynamic Capability Announcement Parameter is a MAY in §9, now RFC5561-9-4.

## Published support status

**Area:** LDP

**Test verification:** 1 of 17 MUST proven

**Evidence:** unit 1

**Unmet:** RFC5561-10-1, RFC5561-3-1, RFC5561-3-2, RFC5561-3-5, RFC5561-4-1, RFC5561-4-2, RFC5561-6-1, RFC5561-6-2, RFC5561-6-3, RFC5561-8-1, RFC5561-8-2, RFC5561-8-3, RFC5561-8-4, RFC5561-9-1, RFC5561-9-2, RFC5561-9-3


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 16 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **17** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC5561-6-4`](#rfc5561-6-4)

**Annotated (including scoped evidence) (16):** [`RFC5561-3-1`](#rfc5561-3-1), [`RFC5561-4-2`](#rfc5561-4-2), [`RFC5561-3-2`](#rfc5561-3-2), [`RFC5561-3-5`](#rfc5561-3-5), [`RFC5561-4-1`](#rfc5561-4-1), [`RFC5561-6-1`](#rfc5561-6-1), [`RFC5561-6-2`](#rfc5561-6-2), [`RFC5561-6-3`](#rfc5561-6-3), [`RFC5561-8-1`](#rfc5561-8-1), [`RFC5561-8-2`](#rfc5561-8-2), [`RFC5561-8-3`](#rfc5561-8-3), [`RFC5561-8-4`](#rfc5561-8-4), [`RFC5561-9-1`](#rfc5561-9-1), [`RFC5561-9-2`](#rfc5561-9-2), [`RFC5561-9-3`](#rfc5561-9-3), [`RFC5561-10-1`](#rfc5561-10-1)

## Figures

- **MUST bar**: Ze verified 1, Not built 16
- **Overview bar**: Ze MUST verified 1, Not assessed 1, Not built 16

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 17 |
| MUST rows kept (the MUST bar) | 17 |
| Ze verified | 1 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 16 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 1 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 1 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5561.json](../../../data/rfc/rfc5561.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5561-3-1` | F-bit: Forward unknown TLV bit, as described in [RFC5036]. The value of this bit MUST be 0 since a Capability Parameter TLV is sent only in Initialization and Capability messages, which are not forwarded. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| `RFC5561-9-4` | The Dynamic Capability Announcement Parameter MAY be included by an LDP speaker in an Initialization message to signal its peer that the speaker is capable of processing Capability messages. (§9) | MAY | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5561-6-4` | If the U-bit is 1, then the speaker MUST silently ignore the Capability Parameter and allow the session to be established. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestRFC5561UnknownCapabilityWithUBitSetIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5561_capability_test.go#L74). **negative:** `unit/verify` [`TestRFC5561UnknownCapabilityWithUBitSetDrawsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5561_capability_test.go#L112) |
| `RFC5561-4-2` | An LDP speaker MUST NOT send a Capability message to a peer unless its peer advertised the Dynamic Capability Announcement capability in its session Initialization message. (§7) | MUST NOT | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| `RFC5561-3-2` | An LDP speaker MUST NOT include more than one instance of a Capability Parameter (as identified by the same TLV code point) in an Initialization or Capability message. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| `RFC5561-3-5` | If an LDP speaker receives more than one instance of the same Capability Parameter type in a message, it SHOULD send a Notification message to the peer before terminating the session with the peer. The Status Code in the Status TLV of the Notification message MUST be Malformed TLV value, and the message SHOULD contain the second Capability Parameter TLV of the same type (code point) that is received in the message. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| `RFC5561-4-1` | Note that Backward Compatibility TLVs (see Section 3.1) MUST NOT be included in Capability messages. (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| `RFC5561-6-1` | The S-bit of a Capability Parameter in an Initialization message MUST be 1 and SHOULD be ignored on receipt. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| `RFC5561-6-2` | The Status Code in the Status TLV of the Notification message MUST be Unsupported Capability, and the message SHOULD contain the unsupported capability (see Section 8 for more details). (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| `RFC5561-6-3` | An LDP speaker that supports capability advertisement and includes a Capability Parameter in its Initialization message MUST set the TLV U-bit to 0 or 1, as specified by Capability document. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| `RFC5561-8-1` | The E-bit of the Status TLV carried in a Notification message that includes this status code MUST be set to 0. (§8, Unsupported Capability) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| `RFC5561-8-2` | When the Notification message specifies the unsupported capabilities, it MUST include a Returned TLVs TLV. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| `RFC5561-8-3` | The Returned TLVs TLV MUST include only the Capability Parameters for unsupported capabilities, and the Capability Parameter for each such capability SHOULD be encoded as received from the peer. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| `RFC5561-8-4` | When the Notification message specifies the TLV that was unknown, it MUST include the unknown TLV in a Returned TLVs TLV. (§8) | MUST | 8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| `RFC5561-9-1` | (0x0506) \| Length (1) \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \|1\| Reserved \| +-+-+-+-+-+-+-+-+ The value of the U-bit for the Dynamic Capability Announcement Parameter TLV MUST be set to 1 so that a receiver MUST silently ignore this TLV if unknown to it, and continue processing the rest of the message. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| `RFC5561-9-2` | There is no "Capability Data" associated with this TLV and hence the TLV length MUST be set to 1. (§9) | MUST | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| `RFC5561-9-3` | An LDP speaker MUST NOT include the Dynamic Capability Announcement Parameter in Capability messages sent to its peers. (§9) | MUST NOT | 9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| `RFC5561-10-1` | To ensure compatibility with an [RFC5036]-compliant peer, LDP implementations that support capability advertisement have label distribution for IPv4 enabled until it is explicitly disabled and MUST assume that their peers do as well. (§10) | MUST | 10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5561-3-1`](#rfc5561-3-1) F-bit: Forward unknown TLV bit, as described in [RFC5036]. The value of this bit MUST be 0 since a Capability Parameter TLV is sent only in Initialization and Capability messages, which are not forwarded. (§3) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| [`RFC5561-4-2`](#rfc5561-4-2) An LDP speaker MUST NOT send a Capability message to a peer unless its peer advertised the Dynamic Capability Announcement capability in its session Initialization message. (§7) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| [`RFC5561-3-2`](#rfc5561-3-2) An LDP speaker MUST NOT include more than one instance of a Capability Parameter (as identified by the same TLV code point) in an Initialization or Capability message. (§3) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| [`RFC5561-3-5`](#rfc5561-3-5) If an LDP speaker receives more than one instance of the same Capability Parameter type in a message, it SHOULD send a Notification message to the peer before terminating the session with the peer. The Status Code in the Status TLV of the Notification message MUST be Malformed TLV value, and the message SHOULD contain the second Capability Parameter TLV of the same type (code point) that is received in the message. (§3) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| [`RFC5561-4-1`](#rfc5561-4-1) Note that Backward Compatibility TLVs (see Section 3.1) MUST NOT be included in Capability messages. (§4) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| [`RFC5561-6-1`](#rfc5561-6-1) The S-bit of a Capability Parameter in an Initialization message MUST be 1 and SHOULD be ignored on receipt. (§6) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| [`RFC5561-6-2`](#rfc5561-6-2) The Status Code in the Status TLV of the Notification message MUST be Unsupported Capability, and the message SHOULD contain the unsupported capability (see Section 8 for more details). (§6) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| [`RFC5561-6-3`](#rfc5561-6-3) An LDP speaker that supports capability advertisement and includes a Capability Parameter in its Initialization message MUST set the TLV U-bit to 0 or 1, as specified by Capability document. (§6) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| [`RFC5561-8-1`](#rfc5561-8-1) The E-bit of the Status TLV carried in a Notification message that includes this status code MUST be set to 0. (§8, Unsupported Capability) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| [`RFC5561-8-2`](#rfc5561-8-2) When the Notification message specifies the unsupported capabilities, it MUST include a Returned TLVs TLV. (§8) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| [`RFC5561-8-3`](#rfc5561-8-3) The Returned TLVs TLV MUST include only the Capability Parameters for unsupported capabilities, and the Capability Parameter for each such capability SHOULD be encoded as received from the peer. (§8) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| [`RFC5561-8-4`](#rfc5561-8-4) When the Notification message specifies the TLV that was unknown, it MUST include the unknown TLV in a Returned TLVs TLV. (§8) | {gap}, no test | ze parses no Capability Parameter, so the capability error status codes and the Returned TLVs TLV are absent; plan/release/fixes/spec-ldp-capability-status-codes.md |
| [`RFC5561-9-1`](#rfc5561-9-1) (0x0506) \| Length (1) \| +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ \|1\| Reserved \| +-+-+-+-+-+-+-+-+ The value of the U-bit for the Dynamic Capability Announcement Parameter TLV MUST be set to 1 so that a receiver MUST silently ignore this TLV if unknown to it, and continue processing the rest of the message. (§9) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| [`RFC5561-9-2`](#rfc5561-9-2) There is no "Capability Data" associated with this TLV and hence the TLV length MUST be set to 1. (§9) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| [`RFC5561-9-3`](#rfc5561-9-3) An LDP speaker MUST NOT include the Dynamic Capability Announcement Parameter in Capability messages sent to its peers. (§9) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |
| [`RFC5561-10-1`](#rfc5561-10-1) To ensure compatibility with an [RFC5036]-compliant peer, LDP implementations that support capability advertisement have label distribution for IPv4 enabled until it is explicitly disabled and MUST assume that their peers do as well. (§10) | {gap}, no test | ze advertises no capability, so the send-side capability path is absent; plan/post-release/features/spec-ldp-capability-advertisement.md |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5561-3-1`](#rfc5561-3-1)

F-bit: Forward unknown TLV bit, as described in [RFC5036]. The value of this bit MUST be 0 since a Capability Parameter TLV is sent only in Initialization and Capability messages, which are not forwarded. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-3-1, so there is no linked test.

### [`RFC5561-6-4`](#rfc5561-6-4)

If the U-bit is 1, then the speaker MUST silently ignore the Capability Parameter and allow the session to be established. (§6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC5561 section 6 requires silently ignoring unsupported U=1 Capability Parameters and allowing establishment. Both dynamic-capability-announcement and unassigned code point fixtures encode U=1, F=0, S=1 beside valid Common Session Parameters. Positive checks decoded parameters, processes Initialization plus KeepAlive to Operational and pins 30s; negative requires nil processMessages error and strict deadline-timeout silence, rejecting EOF as well as Notification. Reviewed DecodeInit/skipTLV and the new common initialization message-type guard: an unknown TLV inside an expected Initialization remains accepted, unlike an unknown message during initialization. Native producer-halt evidence is supplemental, not the semantic verdict.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5561UnknownCapabilityWithUBitSetDrawsNoNotification`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5561_capability_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestRFC5561UnknownCapabilityWithUBitSetIsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/ldp/rfc5561_capability_test.go#L74) | unit/verify | revert, verified |

### [`RFC5561-4-2`](#rfc5561-4-2)

An LDP speaker MUST NOT send a Capability message to a peer unless its peer advertised the Dynamic Capability Announcement capability in its session Initialization message. (§7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-4-2, so there is no linked test.

### [`RFC5561-3-2`](#rfc5561-3-2)

An LDP speaker MUST NOT include more than one instance of a Capability Parameter (as identified by the same TLV code point) in an Initialization or Capability message. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-3-2, so there is no linked test.

### [`RFC5561-3-5`](#rfc5561-3-5)

If an LDP speaker receives more than one instance of the same Capability Parameter type in a message, it SHOULD send a Notification message to the peer before terminating the session with the peer. The Status Code in the Status TLV of the Notification message MUST be Malformed TLV value, and the message SHOULD contain the second Capability Parameter TLV of the same type (code point) that is received in the message. (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-3-5, so there is no linked test.

### [`RFC5561-4-1`](#rfc5561-4-1)

Note that Backward Compatibility TLVs (see Section 3.1) MUST NOT be included in Capability messages. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-4-1, so there is no linked test.

### [`RFC5561-6-1`](#rfc5561-6-1)

The S-bit of a Capability Parameter in an Initialization message MUST be 1 and SHOULD be ignored on receipt. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-6-1, so there is no linked test.

### [`RFC5561-6-2`](#rfc5561-6-2)

The Status Code in the Status TLV of the Notification message MUST be Unsupported Capability, and the message SHOULD contain the unsupported capability (see Section 8 for more details). (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-6-2, so there is no linked test.

### [`RFC5561-6-3`](#rfc5561-6-3)

An LDP speaker that supports capability advertisement and includes a Capability Parameter in its Initialization message MUST set the TLV U-bit to 0 or 1, as specified by Capability document. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-6-3, so there is no linked test.

### [`RFC5561-8-1`](#rfc5561-8-1)

The E-bit of the Status TLV carried in a Notification message that includes this status code MUST be set to 0. (§8, Unsupported Capability)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-8-1, so there is no linked test.

### [`RFC5561-8-2`](#rfc5561-8-2)

When the Notification message specifies the unsupported capabilities, it MUST include a Returned TLVs TLV. (§8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-8-2, so there is no linked test.

### [`RFC5561-8-3`](#rfc5561-8-3)

The Returned TLVs TLV MUST include only the Capability Parameters for unsupported capabilities, and the Capability Parameter for each such capability SHOULD be encoded as received from the peer. (§8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-8-3, so there is no linked test.

### [`RFC5561-8-4`](#rfc5561-8-4)

When the Notification message specifies the TLV that was unknown, it MUST include the unknown TLV in a Returned TLVs TLV. (§8)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-8-4, so there is no linked test.

### [`RFC5561-9-1`](#rfc5561-9-1)

(0x0506) | Length (1) | +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ |1| Reserved | +-+-+-+-+-+-+-+-+ The value of the U-bit for the Dynamic Capability Announcement Parameter TLV MUST be set to 1 so that a receiver MUST silently ignore this TLV if unknown to it, and continue processing the rest of the message. (§9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-9-1, so there is no linked test.

### [`RFC5561-9-2`](#rfc5561-9-2)

There is no "Capability Data" associated with this TLV and hence the TLV length MUST be set to 1. (§9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-9-2, so there is no linked test.

### [`RFC5561-9-3`](#rfc5561-9-3)

An LDP speaker MUST NOT include the Dynamic Capability Announcement Parameter in Capability messages sent to its peers. (§9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-9-3, so there is no linked test.

### [`RFC5561-10-1`](#rfc5561-10-1)

To ensure compatibility with an [RFC5036]-compliant peer, LDP implementations that support capability advertisement have label distribution for IPv4 enabled until it is explicitly disabled and MUST assume that their peers do as well. (§10)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5561-10-1, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 1 | reviewed | not stated |
| `3` | not stated | 4 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 6 | reviewed | not stated |
| `7` | not stated | 1 | reviewed | not stated |
| `8` | not stated | 4 | reviewed | not stated |
| `9` | not stated | 3 | reviewed | not stated |
| `10` | not stated | 1 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `12` | not stated | 0 | reviewed | not stated |
| `13` | not stated | 0 | reviewed | not stated |
| `14` | not stated | 0 | reviewed | not stated |
| `14.1` | not stated | 0 | reviewed | not stated |
| `14.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of an LDP capability document, a document role and not a wire role: Section 2.1 addresses 'a document that' specifies an LDP enhancement, and the sentence obliges that document to describe how its capability data is interpreted and processed. Ze publishes no capability document. The producer that acts as the role in this tree is rfc/full/rfc5561.txt, whose Section 9 is itself such a document for the Dynamic Capability Announcement TLV. Ze would CONSUME the result: internal/plugins/ldp/wire.go decodes what such a document defines. | The capability document MUST also describe the interpretation and processing of associated capability data, if present. |
| `3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of the document specifying a capability, a document role and not a wire role: the sentence obliges that document to describe how the Capability Data is interpreted and processed. Ze publishes no capability document. The producer that acts as the role in this tree is rfc/full/rfc5561.txt, whose Section 9 defines the Dynamic Capability Announcement TLV and states it carries no Capability Data. Ze would CONSUME the result: internal/plugins/ldp/wire.go decodes what such a document defines. | The method for interpreting and processing this data is specific to the TLV code point and MUST be described in the document specifying the capability. |
| `6:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | The role is the AUTHOR of the document specifying procedures for a capability, a document role and not a wire role: Section 6 first says 'It is the responsibility of the capability designer to specify the behavior', then obliges that document to describe the behavior when a peer did not advertise the capability. Ze publishes no capability document. The producer that acts as the role in this tree is rfc/full/rfc5561.txt, whose Section 9 specifies the procedures for the Dynamic Capability Announcement capability. Ze would CONSUME the result in internal/plugins/ldp/session.go. | The document specifying procedures for the capability MUST describe the behavior in this situation. |
| `6:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 states the same obligation twice: site 6:3 carries it for the speaker whose peer never advertised the capability, and this site repeats it for the speaker that meets an unsupported capability with the U-bit 0. Both sentences read 'The Status Code in the Status TLV of the Notification message MUST be Unsupported Capability, and the message SHOULD contain the unsupported capability'. Site 6:3 maps RFC5561-6-2. | The Status Code in the Status TLV of the Notification message MUST be Unsupported Capability, and the message SHOULD contain the unsupported capability (see Section 8 for more details). |

## Superseded

No document obsoletes RFC 5561, so its obligations are stated where they were written.
