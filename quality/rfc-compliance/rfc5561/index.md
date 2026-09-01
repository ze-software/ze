# RFC 5561 - LDP Capabilities

No row in the public ledger. Every requirement this repository extracted from RFC 5561, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 0 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 0 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 0 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 0 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 5 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 5 | of 5 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

No card above is a share of a population, so there is nothing to add up.

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
| Requirements | 7 |
| Gated MUST-level | 5 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 5 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5561.md` |
| Requirement shard | `rfc/requirements/rfc5561.md` |
| RFC text | `rfc/full/rfc5561.txt` |

## Enrolment

Enrolled: ze implements base LDP (RFC 5036) only and has no RFC 5561 capability mechanism, so all 5 gated MUST-level requirements are {not-applicable} against the absent code path (no tests, per the annotation rule): RFC5561-3-1 (U/F bit set to 1), RFC5561-4-1 (include Dynamic Capability Announcement in Init), RFC5561-3-2 (silently ignore unknown capability TLV if U=1), RFC5561-x-1 (Reserved bits zero) and RFC5561-4-2 (MUST NOT send a Capability message) all cite the absence of any capability-TLV / Capability-message code path in internal/plugins/ldp/wire.go (EncodeInit:293, EncodeTLV:166, DecodeTLV:174, DecodeInit:324) and internal/plugins/ldp/session.go:419 (no 0x0202 encoder; no 0x0506 capability constant exists). No SHOULD/MAY requirements are gated.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 5561.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 5 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **5** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (5):** [`RFC5561-3-1`](#rfc5561-3-1), [`RFC5561-4-1`](#rfc5561-4-1), [`RFC5561-3-2`](#rfc5561-3-2), [`RFC5561-x-1`](#rfc5561-x-1), [`RFC5561-4-2`](#rfc5561-4-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5561-3-1` | Capability TLVs MUST have the U-bit and F-bit set to 1 (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no LDP capability-TLV code path; the sole Init encoder EncodeInit emits only the Common Session Parameters TLV (internal/plugins/ldp/wire.go:293,313) and the generic EncodeTLV (internal/plugins/ldp/wire.go:166) writes a full-uint16 Type with no U/F bit fields |
| `RFC5561-4-1` | An LSR MUST include the Dynamic Capability Announcement capability in its Initialization if it supports dynamic capability changes (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no dynamic-capability code path; the sole Init encoder EncodeInit (internal/plugins/ldp/wire.go:293) advertises no capability TLV and no 0x0506 constant exists, so the "if it supports dynamic capability changes" precondition is false for ze |
| `RFC5561-3-2` | An LSR MUST silently ignore a capability TLV with an unknown type if U=1 (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no LDP capability-TLV code path and never parses the U-bit; DecodeTLV reads Type as a full uint16 (internal/plugins/ldp/wire.go:174) and DecodeInit recognises only the Common Session Parameters TLV (internal/plugins/ldp/wire.go:324) |
| `RFC5561-x-1` | Reserved bits in the capability TLV must be zero (Wire Format) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no LDP capability-TLV code path; no S-bit or Reserved field is encoded anywhere, EncodeTLV writes only Type, Length and Value (internal/plugins/ldp/wire.go:166) |
| `RFC5561-4-2` | An LSR MUST NOT send a Capability message unless the peer advertised support for Dynamic Capability Announcement (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no Capability-message code path; there is no encoder for message type 0x0202 and the session read dispatch has no such case, treating it as an unknown message type (internal/plugins/ldp/session.go:419), so ze never sends one |
| `RFC5561-4-3` | An LSR SHOULD support the Dynamic Capability Announcement capability for operational flexibility (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC5561-3-3` | An LSR MAY tear down the session if a required capability is not supported by the peer (§3) | MAY | 3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5561-3-1`](#rfc5561-3-1) Capability TLVs MUST have the U-bit and F-bit set to 1 (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no LDP capability-TLV code path; the sole Init encoder EncodeInit emits only the Common Session Parameters TLV (internal/plugins/ldp/wire.go:293,313) and the generic EncodeTLV (internal/plugins/ldp/wire.go:166) writes a full-uint16 Type with no U/F bit fields |
| [`RFC5561-4-1`](#rfc5561-4-1) An LSR MUST include the Dynamic Capability Announcement capability in its Initialization if it supports dynamic capability changes (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no dynamic-capability code path; the sole Init encoder EncodeInit (internal/plugins/ldp/wire.go:293) advertises no capability TLV and no 0x0506 constant exists, so the "if it supports dynamic capability changes" precondition is false for ze |
| [`RFC5561-3-2`](#rfc5561-3-2) An LSR MUST silently ignore a capability TLV with an unknown type if U=1 (§3) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no LDP capability-TLV code path and never parses the U-bit; DecodeTLV reads Type as a full uint16 (internal/plugins/ldp/wire.go:174) and DecodeInit recognises only the Common Session Parameters TLV (internal/plugins/ldp/wire.go:324) |
| [`RFC5561-x-1`](#rfc5561-x-1) Reserved bits in the capability TLV must be zero (Wire Format) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no LDP capability-TLV code path; no S-bit or Reserved field is encoded anywhere, EncodeTLV writes only Type, Length and Value (internal/plugins/ldp/wire.go:166) |
| [`RFC5561-4-2`](#rfc5561-4-2) An LSR MUST NOT send a Capability message unless the peer advertised support for Dynamic Capability Announcement (§4) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no Capability-message code path; there is no encoder for message type 0x0202 and the session read dispatch has no such case, treating it as an unknown message type (internal/plugins/ldp/session.go:419), so ze never sends one |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5561-3-1`](#rfc5561-3-1)

Capability TLVs MUST have the U-bit and F-bit set to 1 (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-3-1, so no unit is bound to it.

### [`RFC5561-4-1`](#rfc5561-4-1)

An LSR MUST include the Dynamic Capability Announcement capability in its Initialization if it supports dynamic capability changes (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-4-1, so no unit is bound to it.

### [`RFC5561-3-2`](#rfc5561-3-2)

An LSR MUST silently ignore a capability TLV with an unknown type if U=1 (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-3-2, so no unit is bound to it.

### [`RFC5561-x-1`](#rfc5561-x-1)

Reserved bits in the capability TLV must be zero (Wire Format)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-x-1, so no unit is bound to it.

### [`RFC5561-4-2`](#rfc5561-4-2)

An LSR MUST NOT send a Capability message unless the peer advertised support for Dynamic Capability Announcement (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5561-4-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 5561, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 5561, so its obligations are stated where they were written.
