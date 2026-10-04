# RFC 2890 - Key and Sequence Number Extensions to GRE

No row in the public ledger. Every requirement this repository extracted from RFC 2890, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 3 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 3 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 3 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 3 | of 7 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (third-party), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 3 | of 3 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 100.0% | 3 of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 3 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 3 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 8 shares marked as a part above are the whole of the 3 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (third-party) |
| Requirements | 7 |
| Gated MUST-level | 3 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2890.md` |
| Requirement shard | `rfc/requirements/rfc2890.md` |
| RFC text | `rfc/full/rfc2890.txt` |

## Enrolment

Not enrolled (third-party, a layer under or beside Ze performs the document and Ze holds no Go code for it, so the reason beside this kind names the component that does): The Linux ip_gre module encodes and checks the Key and Sequence fields. Ze only sets IKey and OKey on the netlink descriptor, internal/plugins/iface/netlink/tunnel_linux.go::buildGretun.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2890.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 3 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **3** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (3):** [`RFC2890-2.2-3`](#rfc2890-2.2-3), [`RFC2890-2.2-4`](#rfc2890-2.2-4), [`RFC2890-3-1`](#rfc2890-3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2890-2.2-3` | The Sequence Number MUST be used by the receiver to establish the order in which packets have been transmitted from the encapsulator to the receiver. (S2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** GRE receive-side sequence ordering is performed by the kernel/VPP datapath; ze has no GRE decapsulation or packet-parse code path |
| `RFC2890-2.2-4` | If a packet has been waiting that long, the receiver MUST immediately traverse the buffer in sorted order, decapsulating packets (and ignoring any sequence number gaps) until there are no more packets in the buffer that have been waiting longer than OUTOFORDER_TIMER milliseconds. (S2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no GRE receiver buffer or OUTOFORDER_TIMER; it programs kernel/VPP tunnels and does not process GRE payloads |
| `RFC2890-3-1` | In order to protect against such attacks, IP security protocols [4] MUST be used to protect the GRE header and the tunneled payload. (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze applies no ESP or AH to GRE; IPsec is not wired to the GRE tunnel builders (internal/plugins/iface/netlink/tunnel_linux.go, internal/plugins/iface/vpp/tunnel.go) |
| `RFC2890-1.1-1` | The implementation SHOULD provide the capability of logging the error, including the contents of the discarded datagram (S1.1) | SHOULD | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2890-1.1-2` | The implementation SHOULD provide the capability of logging the error, including the contents of the discarded datagram, and SHOULD record the event in a statistics counter. (S1.1) | SHOULD | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2890-2.2-5` | When the decapsulator receives an out-of sequence packet it SHOULD be silently discarded. (S2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2890-2.2-8` | Reordering of out-of sequence packets MAY be performed by the decapsulator for improved performance and tolerance to reordering in the network. (S2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2890-2.2-3`](#rfc2890-2.2-3) The Sequence Number MUST be used by the receiver to establish the order in which packets have been transmitted from the encapsulator to the receiver. (S2.2) | no test | no test carries this requirement id; annotated {not-applicable}: GRE receive-side sequence ordering is performed by the kernel/VPP datapath; ze has no GRE decapsulation or packet-parse code path |
| [`RFC2890-2.2-4`](#rfc2890-2.2-4) If a packet has been waiting that long, the receiver MUST immediately traverse the buffer in sorted order, decapsulating packets (and ignoring any sequence number gaps) until there are no more packets in the buffer that have been waiting longer than OUTOFORDER_TIMER milliseconds. (S2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no GRE receiver buffer or OUTOFORDER_TIMER; it programs kernel/VPP tunnels and does not process GRE payloads |
| [`RFC2890-3-1`](#rfc2890-3-1) In order to protect against such attacks, IP security protocols [4] MUST be used to protect the GRE header and the tunneled payload. (S3) | no test | no test carries this requirement id; annotated {not-applicable}: ze applies no ESP or AH to GRE; IPsec is not wired to the GRE tunnel builders (internal/plugins/iface/netlink/tunnel_linux.go, internal/plugins/iface/vpp/tunnel.go) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2890-2.2-3`](#rfc2890-2.2-3)

The Sequence Number MUST be used by the receiver to establish the order in which packets have been transmitted from the encapsulator to the receiver. (S2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2890-2.2-3, so no unit is bound to it.

### [`RFC2890-2.2-4`](#rfc2890-2.2-4)

If a packet has been waiting that long, the receiver MUST immediately traverse the buffer in sorted order, decapsulating packets (and ignoring any sequence number gaps) until there are no more packets in the buffer that have been waiting longer than OUTOFORDER_TIMER milliseconds. (S2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2890-2.2-4, so no unit is bound to it.

### [`RFC2890-3-1`](#rfc2890-3-1)

In order to protect against such attacks, IP security protocols [4] MUST be used to protect the GRE header and the tunneled payload. (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2890-3-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc2890.txt |
| Source fingerprint | a51de7afc69fbb42 |
| Record | rfc/extraction/rfc2890.json |
| Mapped sentences | 3 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 2 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the obligation site 3:1 already maps. Row RFC2890-3-1 already names the two protocols: 'IP security protocols (ESP or AH) MUST be used to protect the GRE header and tunneled payload'. | Either ESP (Encapsulating Security Payload) [5] or AH (Authentication Header)[6] MUST be used to protect the GRE header. |

## Superseded

No document obsoletes RFC 2890, so its obligations are stated where they were written.
