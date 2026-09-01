# RFC 7012 - Information Model for IP Flow Information Export (IPFIX)

No row in the public ledger. Every requirement this repository extracted from RFC 7012, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 3 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 100.0% | 3 of 3 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 3 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 3 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 12 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 8 | of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 3 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 12 |
| Gated MUST-level | 11 |
| Obligations that bind Ze | 3 |
| Not applicable, so out of scope | 8 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 3 |
| Tagged units | 3 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc7012.md` |
| Requirement shard | `rfc/requirements/rfc7012.md` |
| RFC text | `rfc/full/rfc7012.txt` |

## Enrolment

Enrolled: IPFIX Information Model: exporter role; 8 not-applicable (no enterprise IEs / reduced-size / variable-length IEs / IANA-registry authorship) + 3 single-polarity positive

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 7012.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (11):** [`RFC7012-2.1-1`](#rfc7012-2.1-1), [`RFC7012-2.1-2`](#rfc7012-2.1-2), [`RFC7012-4-1`](#rfc7012-4-1), [`RFC7012-4-2`](#rfc7012-4-2), [`RFC7012-2.1-3`](#rfc7012-2.1-3), [`RFC7012-x-1`](#rfc7012-x-1), [`RFC7012-x-2`](#rfc7012-x-2), [`RFC7012-x-3`](#rfc7012-x-3), [`RFC7012-x-4`](#rfc7012-x-4), [`RFC7012-x-5`](#rfc7012-x-5), [`RFC7012-x-6`](#rfc7012-x-6)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7012-2.1-1` | Every IE MUST have: name, elementId, description, dataType, and status (current or deprecated) (Section 2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this binds the author of an IE definition in the IANA registry; ze references registered IEs by numeric elementId alone and emits no IE metadata in-band (internal/plugins/flowexport/ipfix/ie.go) |
| `RFC7012-2.1-2` | Enterprise-specific IEs MUST also carry enterpriseId (Section 2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no enterprise-specific IEs; every field specifier it writes is an IANA IE with the E bit clear (internal/plugins/flowexport/ipfix/flow_template.go:114-120) |
| `RFC7012-4-1` | IE identifier 0 is reserved and MUST NOT be used (Section 4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestIPFIXFlowTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/flow_template_test.go#L9). **negative:** no negative test. **{single-polarity}:** ze's static templates reference only non-zero IANA IE IDs and it has no code path that constructs IE identifier 0 to drive a negative test |
| `RFC7012-4-2` | Enterprise-specific IEs MUST use IDs in the range 1-32767 (Section 4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no enterprise-specific IEs, so no enterprise IE ID range applies to its output (internal/plugins/flowexport/ipfix/flow_template.go:114-120) |
| `RFC7012-2.1-3` | Enterprise-specific IEs MUST set E=1 in the field specifier and include the 4-octet Enterprise Number (Section 2.1, Section 4) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no enterprise-specific IEs and never sets the E bit, so there is no enterprise-number write path (internal/plugins/flowexport/ipfix/flow_template.go:114-120) |
| `RFC7012-x-1` | When E=0, the Enterprise Number MUST NOT be present in the Field Specifier (RFC 7011 Section 3.2) | MUST NOT | x | **positive:** `unit/verify` [`TestIPFIXTemplateSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/template_test.go#L9). **negative:** no negative test. **{single-polarity}:** every specifier ze emits is E=0 and exactly 4 octets with no enterprise number, and ze has no E=1 path to exercise the complementary case |
| `RFC7012-x-2` | When E=1, the Enterprise Number MUST be present in the Field Specifier (RFC 7011 Section 3.2) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never emits an E=1 field specifier, so this requirement's antecedent never holds for its output (internal/plugins/flowexport/ipfix/flow_template.go:114-120) |
| `RFC7012-x-3` | Reduced-size encoding MUST preserve the signed/unsigned property (RFC 7011 Section 6.2) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze applies no reduced-size encoding; every field length equals the IE's abstract-type default (internal/plugins/flowexport/ipfix/flow_template.go:20-48) |
| `RFC7012-x-4` | Reduced-size encoding MUST NOT be applied to fixed-length types other than integers and float64 (RFC 7011 Section 6.2) | MUST NOT | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze applies no reduced-size encoding at all, so no fixed-length type is ever narrowed (internal/plugins/flowexport/ipfix/flow_template.go:20-48) |
| `RFC7012-x-5` | All data records using a template MUST match the declared Field Lengths exactly, except for variable-length IEs (Exporter Implementation Notes) | MUST | x | **positive:** `unit/verify` [`TestIPFIXFlowData`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/flow_data_test.go#L10). **negative:** no negative test. **{single-polarity}:** ze's record writers emit exactly the widths declared in the template by construction, and it has no decoder/reject path to drive a negative test |
| `RFC7012-x-6` | Variable-length IE template Field Length MUST be 0xFFFF, with inline length prefix in each data record (Exporter Implementation Notes) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no variable-length IEs; interfaceName (IE 82) is defined but placed in no template, so no 0xFFFF field length is ever written (internal/plugins/flowexport/ipfix/ie.go) |
| `RFC7012-2.1-4` | An exporter using enterprise IEs SHOULD publish an information model document so collectors can interpret the fields (Section 2.1, Section 4) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC7012-2.1-1`](#rfc7012-2.1-1) Every IE MUST have: name, elementId, description, dataType, and status (current or deprecated) (Section 2.1) | no test | no test carries this requirement id; annotated {not-applicable}: this binds the author of an IE definition in the IANA registry; ze references registered IEs by numeric elementId alone and emits no IE metadata in-band (internal/plugins/flowexport/ipfix/ie.go) |
| [`RFC7012-2.1-2`](#rfc7012-2.1-2) Enterprise-specific IEs MUST also carry enterpriseId (Section 2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no enterprise-specific IEs; every field specifier it writes is an IANA IE with the E bit clear (internal/plugins/flowexport/ipfix/flow_template.go:114-120) |
| [`RFC7012-4-2`](#rfc7012-4-2) Enterprise-specific IEs MUST use IDs in the range 1-32767 (Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no enterprise-specific IEs, so no enterprise IE ID range applies to its output (internal/plugins/flowexport/ipfix/flow_template.go:114-120) |
| [`RFC7012-2.1-3`](#rfc7012-2.1-3) Enterprise-specific IEs MUST set E=1 in the field specifier and include the 4-octet Enterprise Number (Section 2.1, Section 4) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no enterprise-specific IEs and never sets the E bit, so there is no enterprise-number write path (internal/plugins/flowexport/ipfix/flow_template.go:114-120) |
| [`RFC7012-x-2`](#rfc7012-x-2) When E=1, the Enterprise Number MUST be present in the Field Specifier (RFC 7011 Section 3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never emits an E=1 field specifier, so this requirement's antecedent never holds for its output (internal/plugins/flowexport/ipfix/flow_template.go:114-120) |
| [`RFC7012-x-3`](#rfc7012-x-3) Reduced-size encoding MUST preserve the signed/unsigned property (RFC 7011 Section 6.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze applies no reduced-size encoding; every field length equals the IE's abstract-type default (internal/plugins/flowexport/ipfix/flow_template.go:20-48) |
| [`RFC7012-x-4`](#rfc7012-x-4) Reduced-size encoding MUST NOT be applied to fixed-length types other than integers and float64 (RFC 7011 Section 6.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze applies no reduced-size encoding at all, so no fixed-length type is ever narrowed (internal/plugins/flowexport/ipfix/flow_template.go:20-48) |
| [`RFC7012-x-6`](#rfc7012-x-6) Variable-length IE template Field Length MUST be 0xFFFF, with inline length prefix in each data record (Exporter Implementation Notes) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no variable-length IEs; interfaceName (IE 82) is defined but placed in no template, so no 0xFFFF field length is ever written (internal/plugins/flowexport/ipfix/ie.go) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7012-2.1-1`](#rfc7012-2.1-1)

Every IE MUST have: name, elementId, description, dataType, and status (current or deprecated) (Section 2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7012-2.1-1, so no unit is bound to it.

### [`RFC7012-2.1-2`](#rfc7012-2.1-2)

Enterprise-specific IEs MUST also carry enterpriseId (Section 2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7012-2.1-2, so no unit is bound to it.

### [`RFC7012-4-1`](#rfc7012-4-1)

IE identifier 0 is reserved and MUST NOT be used (Section 4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPFIXFlowTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/flow_template_test.go#L9) | unit/verify | unproven |

### [`RFC7012-4-2`](#rfc7012-4-2)

Enterprise-specific IEs MUST use IDs in the range 1-32767 (Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7012-4-2, so no unit is bound to it.

### [`RFC7012-2.1-3`](#rfc7012-2.1-3)

Enterprise-specific IEs MUST set E=1 in the field specifier and include the 4-octet Enterprise Number (Section 2.1, Section 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7012-2.1-3, so no unit is bound to it.

### [`RFC7012-x-1`](#rfc7012-x-1)

When E=0, the Enterprise Number MUST NOT be present in the Field Specifier (RFC 7011 Section 3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPFIXTemplateSet`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/template_test.go#L9) | unit/verify | unproven |

### [`RFC7012-x-2`](#rfc7012-x-2)

When E=1, the Enterprise Number MUST be present in the Field Specifier (RFC 7011 Section 3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7012-x-2, so no unit is bound to it.

### [`RFC7012-x-3`](#rfc7012-x-3)

Reduced-size encoding MUST preserve the signed/unsigned property (RFC 7011 Section 6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7012-x-3, so no unit is bound to it.

### [`RFC7012-x-4`](#rfc7012-x-4)

Reduced-size encoding MUST NOT be applied to fixed-length types other than integers and float64 (RFC 7011 Section 6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7012-x-4, so no unit is bound to it.

### [`RFC7012-x-5`](#rfc7012-x-5)

All data records using a template MUST match the declared Field Lengths exactly, except for variable-length IEs (Exporter Implementation Notes)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIPFIXFlowData`](https://github.com/ze-software/ze/blob/main/internal/plugins/flowexport/ipfix/flow_data_test.go#L10) | unit/verify | unproven |

### [`RFC7012-x-6`](#rfc7012-x-6)

Variable-length IE template Field Length MUST be 0xFFFF, with inline length prefix in each data record (Exporter Implementation Notes)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC7012-x-6, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 7012, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 7012, so its obligations are stated where they were written.
