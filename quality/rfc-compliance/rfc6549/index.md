# RFC 6549 - OSPFv2 Multi-Instance Extensions

No row in the public ledger. Every requirement this repository extracted from RFC 6549, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 1 of 1 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 1 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 1 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 1 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 2 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 1 | of 4 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 4 |
| Gated MUST-level | 1 |
| Obligations that bind Ze | 1 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 2 |
| Tagged units | 2 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc6549.md` |
| Requirement shard | `rfc/requirements/rfc6549.md` |
| RFC text | `rfc/full/rfc6549.txt` |

## Enrolment

Enrolled: OSPFv2 Multi-Instance Extensions: the sole MUST (RFC6549-2-1) requires a received OSPFv2 packet whose Instance ID does not match one configured on the receiving interface to be discarded before any handler runs. The per-instance demux lives in the shared dispatcher (internal/plugins/ospf/dispatcher.go, `h.InstanceID != instanceID`), one engine per Instance ID (internal/plugins/ospf/multi_instance.go). The positive/negative pair drives dispatch() directly: mismatched Instance IDs {0,1,4,6,255} are dropped before the handler, while the engine's own Instance ID is delivered, so the demux is not a blanket drop. The 5-1/6-1 SHOULDs and 3-1 MAY are backward-compat guidance and config surface, not gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 6549.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **1** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC6549-2-1`](#rfc6549-2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6549-2-1` | Received packets with an Instance ID not equal to one of the Instance IDs corresponding to one of the configured OSPFv2 Instances for the receiving interface MUST be discarded (§2, §3.1) -- `internal/plugins/ospf/dispatcher.go` (`h.InstanceID != instanceID` discard, before any handler), one engine per Instance ID (`internal/plugins/ospf/multi_instance.go`); spec-ospf-ext-12 | MUST | 2 | **positive:** `unit/verify` [`TestDispatchDropsMismatchedInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/instance_test.go#L457). **negative:** `unit/verify` [`TestDispatchDropsMismatchedInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/instance_test.go#L470) |
| `RFC6549-6-1` | ("recommended") Implementations of this specification and the OSPF MIB also implement SNMP Notification filtering as specified in Section 6 of RFC 3413 (§6) -- N/A: Ze has no OSPF SNMP MIB surface, so there is nothing to filter (recorded as a Known Limitation) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC6549-5-1` | OSPFv2 routers not supporting this specification should only support the default instance (§5) -- Ze at Instance ID 0 is bit-for-bit compatible with base OSPFv2 (`internal/plugins/ospf/packet/header.go`; `TestHeaderInstanceZeroUnchanged`) | SHOULD | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC6549-3-1` | Setting the OSPFv2 Interface Instance ID to a non-zero value may be accomplished through configuration (§3) -- the per-interface `instance-id` leaf-list (`internal/plugins/ospf/yang/ze-ospf-conf.yang`); spec-ospf-ext-12 | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 6549 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6549-2-1`](#rfc6549-2-1)

Received packets with an Instance ID not equal to one of the Instance IDs corresponding to one of the configured OSPFv2 Instances for the receiving interface MUST be discarded (§2, §3.1) -- `internal/plugins/ospf/dispatcher.go` (`h.InstanceID != instanceID` discard, before any handler), one engine per Instance ID (`internal/plugins/ospf/multi_instance.go`); spec-ospf-ext-12

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDispatchDropsMismatchedInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/instance_test.go#L470) | unit/verify | unproven |
| positive | [`TestDispatchDropsMismatchedInstance`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/instance_test.go#L457) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 6549, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 6549, so its obligations are stated where they were written.
