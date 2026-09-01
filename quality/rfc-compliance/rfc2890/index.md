# RFC 2890 - Key and Sequence Number Extensions to GRE

No row in the public ledger. Every requirement this repository extracted from RFC 2890, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 7 | of 13 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 7 | of 7 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Requirements | 13 |
| Gated MUST-level | 7 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 7 |
| Declared gaps | 0 |
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

Enrolled: Key and Sequence Number Extensions to GRE: seven MUST-level requirements, all {not-applicable}. ze builds and parses no GRE header: it configures kernel GRE tunnels via netlink (internal/plugins/iface/netlink/tunnel_linux.go buildGretun sets only IKey/OKey) and VPP tunnels via gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go), delegating all C/K/S flag construction, Key/Sequence field encoding, receiver ordering (OUTOFORDER_TIMER), and IPsec protection to the kernel/VPP dataplane. ze has no GRE header-construction, sequence, decapsulation, or receiver code path.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 2890.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **7** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (7):** [`RFC2890-2.1-1`](#rfc2890-2.1-1), [`RFC2890-2.1-2`](#rfc2890-2.1-2), [`RFC2890-2.2-1`](#rfc2890-2.2-1), [`RFC2890-2.2-2`](#rfc2890-2.2-2), [`RFC2890-2.2-3`](#rfc2890-2.2-3), [`RFC2890-2.2-4`](#rfc2890-2.2-4), [`RFC2890-3-1`](#rfc2890-3-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2890-2.1-1` | When K=1, the Key field MUST be present (4 octets) (S2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze builds no GRE header; the netlink backend sets only the tunnel IKey/OKey config fields (internal/plugins/iface/netlink/tunnel_linux.go:129) and the VPP backend calls gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:70), so the kernel/VPP dataplane constructs the K flag and Key octets, not ze |
| `RFC2890-2.1-2` | When K=0, the Key field MUST NOT be present (S2.1) | MUST NOT | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze emits no GRE header bytes; the K-bit-and-Key-absence invariant is enforced by the kernel, which sets the flag only when IKey/OKey are non-zero (internal/plugins/iface/netlink/tunnel_linux.go:128-139); ze has no header-flag code path |
| `RFC2890-2.2-1` | When S=1, the Sequence Number field MUST be present (S2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze exposes no GRE sequence-number configuration and constructs no Sequence Number field; grep for a sequence producer across the iface tunnel paths finds none |
| `RFC2890-2.2-2` | When S=0, the Sequence Number field MUST NOT be present (S2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no S-bit or Sequence Number code path; it configures kernel/VPP tunnels and builds no GRE header |
| `RFC2890-2.2-3` | Sequence Number MUST be used by the receiver to establish packet order (S2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** GRE receive-side sequence ordering is performed by the kernel/VPP datapath; ze has no GRE decapsulation or packet-parse code path |
| `RFC2890-2.2-4` | If a packet has waited longer than OUTOFORDER_TIMER milliseconds in the buffer, the receiver MUST immediately traverse the buffer in sorted order, decapsulating packets (S2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no GRE receiver buffer or OUTOFORDER_TIMER; it programs kernel/VPP tunnels and does not process GRE payloads |
| `RFC2890-3-1` | IP security protocols (ESP or AH) MUST be used to protect the GRE header and tunneled payload when using Sequence Number (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze applies no ESP or AH to GRE; IPsec is not wired to the GRE tunnel builders (internal/plugins/iface/netlink/tunnel_linux.go, internal/plugins/iface/vpp/tunnel.go) |
| `RFC2890-1.1-1` | When silently discarding, the implementation SHOULD provide the capability of logging the error (S1.1) | SHOULD | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2890-1.1-2` | When silently discarding, the implementation SHOULD record the event in a statistics counter (S1.1) | SHOULD | 1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2890-2.2-5` | An out-of-sequence packet SHOULD be silently discarded (S2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2890-2.2-6` | The first packet's sequence number MAY be any value (S2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2890-2.2-7` | A receiver MAY discard out-of-order packets (S2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2890-2.2-8` | Reordering of out-of-sequence packets MAY be performed by the decapsulator (S2.2) | MAY | 2.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2890-2.1-1`](#rfc2890-2.1-1) When K=1, the Key field MUST be present (4 octets) (S2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze builds no GRE header; the netlink backend sets only the tunnel IKey/OKey config fields (internal/plugins/iface/netlink/tunnel_linux.go:129) and the VPP backend calls gre_tunnel_add_del (internal/plugins/iface/vpp/tunnel.go:70), so the kernel/VPP dataplane constructs the K flag and Key octets, not ze |
| [`RFC2890-2.1-2`](#rfc2890-2.1-2) When K=0, the Key field MUST NOT be present (S2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze emits no GRE header bytes; the K-bit-and-Key-absence invariant is enforced by the kernel, which sets the flag only when IKey/OKey are non-zero (internal/plugins/iface/netlink/tunnel_linux.go:128-139); ze has no header-flag code path |
| [`RFC2890-2.2-1`](#rfc2890-2.2-1) When S=1, the Sequence Number field MUST be present (S2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze exposes no GRE sequence-number configuration and constructs no Sequence Number field; grep for a sequence producer across the iface tunnel paths finds none |
| [`RFC2890-2.2-2`](#rfc2890-2.2-2) When S=0, the Sequence Number field MUST NOT be present (S2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no S-bit or Sequence Number code path; it configures kernel/VPP tunnels and builds no GRE header |
| [`RFC2890-2.2-3`](#rfc2890-2.2-3) Sequence Number MUST be used by the receiver to establish packet order (S2.2) | no test | no test carries this requirement id; annotated {not-applicable}: GRE receive-side sequence ordering is performed by the kernel/VPP datapath; ze has no GRE decapsulation or packet-parse code path |
| [`RFC2890-2.2-4`](#rfc2890-2.2-4) If a packet has waited longer than OUTOFORDER_TIMER milliseconds in the buffer, the receiver MUST immediately traverse the buffer in sorted order, decapsulating packets (S2.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no GRE receiver buffer or OUTOFORDER_TIMER; it programs kernel/VPP tunnels and does not process GRE payloads |
| [`RFC2890-3-1`](#rfc2890-3-1) IP security protocols (ESP or AH) MUST be used to protect the GRE header and tunneled payload when using Sequence Number (S3) | no test | no test carries this requirement id; annotated {not-applicable}: ze applies no ESP or AH to GRE; IPsec is not wired to the GRE tunnel builders (internal/plugins/iface/netlink/tunnel_linux.go, internal/plugins/iface/vpp/tunnel.go) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2890-2.1-1`](#rfc2890-2.1-1)

When K=1, the Key field MUST be present (4 octets) (S2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2890-2.1-1, so no unit is bound to it.

### [`RFC2890-2.1-2`](#rfc2890-2.1-2)

When K=0, the Key field MUST NOT be present (S2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2890-2.1-2, so no unit is bound to it.

### [`RFC2890-2.2-1`](#rfc2890-2.2-1)

When S=1, the Sequence Number field MUST be present (S2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2890-2.2-1, so no unit is bound to it.

### [`RFC2890-2.2-2`](#rfc2890-2.2-2)

When S=0, the Sequence Number field MUST NOT be present (S2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2890-2.2-2, so no unit is bound to it.

### [`RFC2890-2.2-3`](#rfc2890-2.2-3)

Sequence Number MUST be used by the receiver to establish packet order (S2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2890-2.2-3, so no unit is bound to it.

### [`RFC2890-2.2-4`](#rfc2890-2.2-4)

If a packet has waited longer than OUTOFORDER_TIMER milliseconds in the buffer, the receiver MUST immediately traverse the buffer in sorted order, decapsulating packets (S2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2890-2.2-4, so no unit is bound to it.

### [`RFC2890-3-1`](#rfc2890-3-1)

IP security protocols (ESP or AH) MUST be used to protect the GRE header and tunneled payload when using Sequence Number (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2890-3-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 2890, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2890, so its obligations are stated where they were written.
