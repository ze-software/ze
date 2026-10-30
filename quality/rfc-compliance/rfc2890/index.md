# RFC 2890 - Key and Sequence Number Extensions to GRE

Partial. Every requirement this repository extracted from RFC 2890, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 3 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 3 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 3 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 3 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 3 | of 7 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 3 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 3 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 3 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 3 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 3 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 100.0% | 3 of 3 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 3 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 7 |
| Gated MUST-level | 3 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
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

Enrolled: Key and Sequence Number Extensions to GRE: three MUST-level requirements, all gaps. ze is the GRE encapsulator and decapsulator through the kernel and VPP: it configures kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go buildGretun sets only IKey/OKey) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go), and neither path enables sequence numbering, a receive buffer or the OUTOFORDER_TIMER (2.2-3, 2.2-4), nor requires IPsec over the tunnel (3-1).

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

- Ze configures GRE tunnels on the kernel and VPP backends, with an optional key on the kernel backend only (the VPP backend refuses a key)
- Linux ip_gre encodes and checks the Key field.


**What the ledger says remains**

Three MUST gaps in the requirement checklist: no backend enables GRE sequence numbering, so a received Sequence Number orders nothing ([`RFC2890-2.2-3`](#rfc2890-2.2-3)), no receive buffer or OUTOFORDER_TIMER exists ([`RFC2890-2.2-4`](#rfc2890-2.2-4)), and no GRE tunnel is required to run under IPsec ([`RFC2890-3-1`](#rfc2890-3-1)).

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

## Attainable ceiling

Mandatory ceiling: 3 of 3 (100.0%). Optional ceiling: 4 of 4 (100.0%). Every RFC shown with measured rows can reach 100% under current classifications. Lowest mandatory ceiling: rfc2890 3 of 3 (100.0%). Lowest optional ceiling: rfc2890 4 of 4 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2890-2.2-3` | The Sequence Number MUST be used by the receiver to establish the order in which packets have been transmitted from the encapsulator to the receiver. (S2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze configures no GRE sequence numbering: internal/plugins/iface/netlink/tunnel_linux.go::buildGretun sets only IKey and OKey, never the input sequence flag, and the VPP backend's createGRETunnel (internal/plugins/iface/vpp/tunnel.go) has no sequence field, so a received Sequence Number orders nothing on either backend |
| `RFC2890-2.2-4` | If a packet has been waiting that long, the receiver MUST immediately traverse the buffer in sorted order, decapsulating packets (and ignoring any sequence number gaps) until there are no more packets in the buffer that have been waiting longer than OUTOFORDER_TIMER milliseconds. (S2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no receive buffer or OUTOFORDER_TIMER exists on either backend: buildGretun (internal/plugins/iface/netlink/tunnel_linux.go) enables no sequence checking, Linux ip_gre only drops an out-of-order packet when it is enabled and never buffers, and the VPP backend's createGRETunnel (internal/plugins/iface/vpp/tunnel.go) carries no sequence option |
| `RFC2890-3-1` | In order to protect against such attacks, IP security protocols [4] MUST be used to protect the GRE header and the tunneled payload. (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a GRE tunnel is configured with no IPsec protection required: the GRE builders (internal/plugins/iface/netlink/tunnel_linux.go::buildGretun, internal/plugins/iface/vpp/tunnel.go::createGRETunnel) bind no SA, and nothing refuses a GRE tunnel that no ze IKE policy covers |
| `RFC2890-1.1-1` | The implementation SHOULD provide the capability of logging the error, including the contents of the discarded datagram (S1.1) | SHOULD | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2890-1.1-2` | The implementation SHOULD provide the capability of logging the error, including the contents of the discarded datagram, and SHOULD record the event in a statistics counter. (S1.1) | SHOULD | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2890-2.2-5` | When the decapsulator receives an out-of sequence packet it SHOULD be silently discarded. (S2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2890-2.2-8` | Reordering of out-of sequence packets MAY be performed by the decapsulator for improved performance and tolerance to reordering in the network. (S2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2890-2.2-3`](#rfc2890-2.2-3) The Sequence Number MUST be used by the receiver to establish the order in which packets have been transmitted from the encapsulator to the receiver. (S2.2) | {gap}, no test | ze configures no GRE sequence numbering: internal/plugins/iface/netlink/tunnel_linux.go::buildGretun sets only IKey and OKey, never the input sequence flag, and the VPP backend's createGRETunnel (internal/plugins/iface/vpp/tunnel.go) has no sequence field, so a received Sequence Number orders nothing on either backend |
| [`RFC2890-2.2-4`](#rfc2890-2.2-4) If a packet has been waiting that long, the receiver MUST immediately traverse the buffer in sorted order, decapsulating packets (and ignoring any sequence number gaps) until there are no more packets in the buffer that have been waiting longer than OUTOFORDER_TIMER milliseconds. (S2.2) | {gap}, no test | no receive buffer or OUTOFORDER_TIMER exists on either backend: buildGretun (internal/plugins/iface/netlink/tunnel_linux.go) enables no sequence checking, Linux ip_gre only drops an out-of-order packet when it is enabled and never buffers, and the VPP backend's createGRETunnel (internal/plugins/iface/vpp/tunnel.go) carries no sequence option |
| [`RFC2890-3-1`](#rfc2890-3-1) In order to protect against such attacks, IP security protocols [4] MUST be used to protect the GRE header and the tunneled payload. (S3) | {gap}, no test | a GRE tunnel is configured with no IPsec protection required: the GRE builders (internal/plugins/iface/netlink/tunnel_linux.go::buildGretun, internal/plugins/iface/vpp/tunnel.go::createGRETunnel) bind no SA, and nothing refuses a GRE tunnel that no ze IKE policy covers |

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
