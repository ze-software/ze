# RFC 4486 - Subcodes for BGP Cease NOTIFICATION Message

Supported. Every requirement this repository extracted from RFC 4486, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 1 of 1 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 1 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 1 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 1 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 1 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 1 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 1 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 11 |
| Gated MUST-level | 1 |
| Obligations that bind Ze | 1 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4486.md` |
| Requirement shard | `rfc/requirements/rfc4486.md` |
| RFC text | `rfc/full/rfc4486.txt` |

## Enrolment

Enrolled: Subcodes for BGP Cease NOTIFICATION Message: one MUST-level requirement, RFC4486-4-1 -- terminating a peering because the prefix count exceeded a locally configured upper bound MUST send a NOTIFICATION with Error Code Cease and Error Subcode "Maximum Number of Prefixes Reached". Both polarities are proven over the reactor's prefix-limit path (internal/component/bgp/reactor/session_prefix.go:399 decides, :448 builds the NOTIFICATION with message.NotifyCease and message.NotifyCeaseMaxPrefixes): positive in TestPrefixExceedTeardown, and negative in TestPrefixWarningThreshold, where a peer at the warning threshold but below the maximum receives no NOTIFICATION at all -- so the Cease is bound to the termination and not merely to a counter moving. test/plugin/prefix-maximum-enforce.ci pins the same bytes on the wire (error code 06, subcode 01, plus the AFI/SAFI/count Data field of RFC4486-4-10). The other ten rows in section 4 are SHOULD, RECOMMENDED or MAY and are not gated.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

The subcode catalog for error code 6, subcodes 1 to 8 ([`internal/component/bgp/message/notification.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/notification.go)). Also max-prefix teardown, retry backoff, and the operator reset paths. Enrolled 2026-07-30. Its sole MUST-level requirement is proven in both polarities. That requirement binds subcode 1 to the teardown a prefix-maximum breach causes. [`test/plugin/prefix-maximum-enforce.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefix-maximum-enforce.ci) asserts the bytes on the wire, including the optional AFI/SAFI/upper-bound Data field of Figure 1. The ten advisory statements of section 4 are bound per line in [`rfc/short/rfc4486.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc4486.md).

**What the ledger says remains:**

No tracked gap in current source anchors.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **1** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC4486-4-1`](#rfc4486-4-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4486-4-1` | If a BGP speaker decides to terminate its peering with a neighbor because the number of address prefixes received from the neighbor exceeds a locally configured upper bound (as described in [BGP-4]), then the speaker MUST send to the neighbor a NOTIFICATION message with the Error Code Cease and the Error Subcode "Maximum Number of Prefixes Reached" (§4) | MUST | 4 - Subcode Usage | **positive:** `unit/verify` [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L114). **negative:** `unit/verify` [`TestPrefixWarningThreshold`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L89). **positive:** `functional/verify` [`prefix-maximum-enforce.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefix-maximum-enforce.ci#L6) |
| `RFC4486-4-2` | If a BGP speaker decides to administratively shut down its peering with a neighbor, then the speaker SHOULD send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Administrative Shutdown" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-3` | If a BGP speaker decides to de-configure a peer, then the speaker SHOULD send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Peer De-configured" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-4` | If a BGP speaker decides to administratively reset the peering with a neighbor, then the speaker SHOULD send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Administrative Reset" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-5` | If a BGP speaker decides to disallow a BGP connection (e.g., the peer is not configured locally) after the speaker accepts a transport protocol connection, then the BGP speaker SHOULD send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Connection Rejected" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-6` | If a BGP speaker decides to administratively reset the peering with a neighbor due to a configuration change other than the ones described above, then the speaker SHOULD send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Other Configuration Change" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-7` | If a BGP speaker decides to send a NOTIFICATION message with the Error Code Cease as a result of the collision resolution procedure (as described in [BGP-4]), then the subcode SHOULD be set to "Connection Collision Resolution" (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-8` | An implementation SHOULD impose an upper bound on the number of consecutive automatic retries (§4) | SHOULD | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-9` | It is RECOMMENDED that a BGP speaker behave as though the DampPeerOscillations attribute [BGP-4] were true for this peer when re-trying a BGP connection after the speaker receives a Cease NOTIFICATION message with a subcode of "Administrative Shutdown", "Peer De-configured", "Connection Rejected", or "Out of Resources" (§4) | RECOMMENDED | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |
| `RFC4486-4-10` | The message MAY optionally include the Address Family information [BGP-MP] and the upper bound in the "Data" field, as shown in Figure 1 (§4) | MAY | 4 - Subcode Usage | **positive:** `unit/verify` [`TestPrefixNotificationDataCarriesTheConfiguredUpperBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L240). **negative:** no negative test |
| `RFC4486-4-11` | If a BGP speaker runs out of resources (e.g., memory) and decides to reset a session, then the speaker MAY send a NOTIFICATION message with the Error Code Cease and the Error Subcode "Out of Resources" (§4) | MAY | 4 - Subcode Usage | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 4486 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4486-4-1`](#rfc4486-4-1)

If a BGP speaker decides to terminate its peering with a neighbor because the number of address prefixes received from the neighbor exceeds a locally configured upper bound (as described in [BGP-4]), then the speaker MUST send to the neighbor a NOTIFICATION message with the Error Code Cease and the Error Subcode "Maximum Number of Prefixes Reached" (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPrefixWarningThreshold`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L89) | unit/verify | unproven |
| positive | [`TestPrefixExceedTeardown`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L114) | unit/verify | unproven |
| positive | [`prefix-maximum-enforce.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/prefix-maximum-enforce.ci#L6) | functional/verify | unproven |

### [`RFC4486-4-10`](#rfc4486-4-10)

The message MAY optionally include the Address Family information [BGP-MP] and the upper bound in the "Data" field, as shown in Figure 1 (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestPrefixNotificationDataCarriesTheConfiguredUpperBound`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_prefix_test.go#L240) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, spec-rfcgate-4-ledger phase 6 |
| Signed off | 2026-07-30 |
| Register | rfc2119 |
| Source | rfc/full/rfc4486.txt |
| Source fingerprint | ee862bf55c82959b |
| Record | rfc/extraction/rfc4486.json |
| Mapped sentences | 1 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of this Memo, copyright notice and Abstract. The Abstract restates section 1 word for word and states no obligation. |
| `1` | Introduction | 0 | walked | Introduction. Says what the document defines and that it 'also recommends' a backoff mechanism. The recommendation itself is stated normatively in section 4 and is captured there as RFC4486-4-9. |
| `2` | not stated | 0 | walked | Specification of Requirements: the RFC 2119 key-words paragraph. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Subcode Definition | 0 | walked | Subcode Definition. The registry table assigning subcodes 1 to 8 to their symbolic names. A value assignment, not a directive: the obligation to USE each value is in section 4. |
| `4` | Subcode Usage | 1 | walked | Subcode Usage. The only normative section. Its one capitalised MUST is the site below; the remaining ten sentences are SHOULD, RECOMMENDED or MAY and are captured as RFC4486-4-2 through RFC4486-4-11. |
| `5` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Defines subcodes 1 to 8 in the registry and names the process for future assignments. Binds IANA, not a speaker. |
| `6` | Security Considerations | 0 | walked | Security Considerations. One sentence: the extension does not change BGP's underlying security issues. No countermeasure is directed at a speaker. |
| `7` | Acknowledgements | 0 | skipped (acknowledgements) | Acknowledgements. |
| `8` | References heading | 0 | skipped (references) | References heading. |
| `8.1` | Normative References: RFC 2119, RFC 4271, RFC 4760 | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 4760. |
| `8.2` | Informative References: RFC 2434, RFC 4020 | 0 | skipped (references) | Informative References: RFC 2434, RFC 4020. |

### Excluded sentences

The walk over RFC 4486 declined no sentence: every site it found is mapped to a requirement.

## Superseded

No document obsoletes RFC 4486, so its obligations are stated where they were written.
