# RFC 3031 - Multiprotocol Label Switching Architecture

No row in the public ledger. Every requirement this repository extracted from RFC 3031, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 7 | of 11 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
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
| Requirements | 11 |
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
| Summary | `rfc/short/rfc3031.md` |
| Requirement shard | `rfc/requirements/rfc3031.md` |
| RFC text | `rfc/full/rfc3031.txt` |

## Enrolment

Enrolled: Multiprotocol Label Switching Architecture: seven MUST-level requirements, all {not-applicable}. ze is an MPLS control plane that programs label operations as kernel AF_MPLS routes (internal/plugins/fib/kernel/mplsentry_linux.go addMPLSSwap) and VPP entries (internal/plugins/fib/vpp/mpls.go); the gated MUSTs (NHLFE lookup, empty-label-stack network-layer forwarding, label-stack TTL decrement and TTL-zero discard, top-label-only forwarding, unknown-label discard) are packet forwarding-plane behaviors executed by the kernel/VPP dataplane, and ze has no in-process MPLS packet-forwarding path. Label merging (Section 3.14) is an ATM/Frame-Relay VC-merge concern ze does not implement as a packet LSR.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 3031.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **7** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (7):** [`RFC3031-3.8-1`](#rfc3031-3.8-1), [`RFC3031-3.14-1`](#rfc3031-3.14-1), [`RFC3031-3.10-1`](#rfc3031-3.10-1), [`RFC3031-3.24-1`](#rfc3031-3.24-1), [`RFC3031-3.24-2`](#rfc3031-3.24-2), [`RFC3031-3.10-2`](#rfc3031-3.10-2), [`RFC3031-x-1`](#rfc3031-x-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3031-3.8-1` | Label forwarding MUST use the Next Hop Label Forwarding Entry (NHLFE) for lookup (S3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is an MPLS control plane that programs the incoming-label-to-NHLFE mapping as kernel AF_MPLS routes (internal/plugins/fib/kernel/mplsentry_linux.go:21 addMPLSSwap) and VPP entries (internal/plugins/fib/vpp/mpls.go); the NHLFE lookup on a forwarded packet is executed by the kernel/VPP dataplane, and ze has no in-process MPLS packet-forwarding path |
| `RFC3031-3.14-1` | Merged upstream labels MUST all map into the same egress point (label merging) (S3.14) | MUST | 3.14 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** label merging is an ATM/Frame-Relay VC-merge concern; ze is a packet LSR with no VC-merge code path, and the RSVP-TE merge-point handling in internal/plugins/rsvpte is Fast Reroute, unrelated to RFC 3031 label merging |
| `RFC3031-3.10-1` | If the packet's label stack is empty, forward based on network layer header (S3.10) | MUST | 3.10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze programs the egress pop disposition so the kernel IP-routes the exposed inner packet (internal/plugins/fib/kernel/mplsentry_linux.go:44-56); the empty-label-stack network-layer forwarding decision is executed by the kernel dataplane, not by ze |
| `RFC3031-3.24-1` | TTL field MUST be decremented at each LSR (S3.24) | MUST | 3.24 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** MPLS label-stack TTL decrement on a forwarded packet is performed by the kernel/VPP dataplane; ze sets only the initial push TTL (internal/plugins/fib/vpp/mpls.go) and has no transit packet-forwarding TTL code path |
| `RFC3031-3.24-2` | If TTL reaches 0, packet MUST be discarded (S3.24) | MUST | 3.24 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** discarding a packet whose MPLS TTL reached zero is executed by the kernel/VPP dataplane; ze has no MPLS packet-forwarding path that could observe or act on the label-stack TTL |
| `RFC3031-3.10-2` | Top label only determines forwarding; lower labels are opaque to transit LSRs (S3.10) | MUST | 3.10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keys each programmed swap entry on the top/incoming label (internal/plugins/fib/kernel/mplsentry_linux.go:24 MPLSDst), but the top-label-only forwarding decision on a packet is executed by the kernel dataplane; ze has no in-process forwarding path |
| `RFC3031-x-1` | Unknown label in lookup: discard packet (Validation) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** discarding a packet whose top label has no ILM entry is executed by the kernel/VPP dataplane on a lookup miss; ze programs the entries and has no MPLS packet-forwarding path that could observe an unknown-label miss |
| `RFC3031-3.15-1` | An LSR SHOULD be able to support both liberal and conservative label retention modes (S3.15) | SHOULD | 3.15 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-3.16-1` | LSR SHOULD support penultimate hop popping (PHP) (S3.16) | SHOULD | 3.16 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-3.14-2` | LSR MAY support label merging to reduce label consumption (S3.14) | MAY | 3.14 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-2.1-1` | Explicitly routed LSPs MAY be used for traffic engineering (S2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3031-3.8-1`](#rfc3031-3.8-1) Label forwarding MUST use the Next Hop Label Forwarding Entry (NHLFE) for lookup (S3.8) | no test | no test carries this requirement id; annotated {not-applicable}: ze is an MPLS control plane that programs the incoming-label-to-NHLFE mapping as kernel AF_MPLS routes (internal/plugins/fib/kernel/mplsentry_linux.go:21 addMPLSSwap) and VPP entries (internal/plugins/fib/vpp/mpls.go); the NHLFE lookup on a forwarded packet is executed by the kernel/VPP dataplane, and ze has no in-process MPLS packet-forwarding path |
| [`RFC3031-3.14-1`](#rfc3031-3.14-1) Merged upstream labels MUST all map into the same egress point (label merging) (S3.14) | no test | no test carries this requirement id; annotated {not-applicable}: label merging is an ATM/Frame-Relay VC-merge concern; ze is a packet LSR with no VC-merge code path, and the RSVP-TE merge-point handling in internal/plugins/rsvpte is Fast Reroute, unrelated to RFC 3031 label merging |
| [`RFC3031-3.10-1`](#rfc3031-3.10-1) If the packet's label stack is empty, forward based on network layer header (S3.10) | no test | no test carries this requirement id; annotated {not-applicable}: ze programs the egress pop disposition so the kernel IP-routes the exposed inner packet (internal/plugins/fib/kernel/mplsentry_linux.go:44-56); the empty-label-stack network-layer forwarding decision is executed by the kernel dataplane, not by ze |
| [`RFC3031-3.24-1`](#rfc3031-3.24-1) TTL field MUST be decremented at each LSR (S3.24) | no test | no test carries this requirement id; annotated {not-applicable}: MPLS label-stack TTL decrement on a forwarded packet is performed by the kernel/VPP dataplane; ze sets only the initial push TTL (internal/plugins/fib/vpp/mpls.go) and has no transit packet-forwarding TTL code path |
| [`RFC3031-3.24-2`](#rfc3031-3.24-2) If TTL reaches 0, packet MUST be discarded (S3.24) | no test | no test carries this requirement id; annotated {not-applicable}: discarding a packet whose MPLS TTL reached zero is executed by the kernel/VPP dataplane; ze has no MPLS packet-forwarding path that could observe or act on the label-stack TTL |
| [`RFC3031-3.10-2`](#rfc3031-3.10-2) Top label only determines forwarding; lower labels are opaque to transit LSRs (S3.10) | no test | no test carries this requirement id; annotated {not-applicable}: ze keys each programmed swap entry on the top/incoming label (internal/plugins/fib/kernel/mplsentry_linux.go:24 MPLSDst), but the top-label-only forwarding decision on a packet is executed by the kernel dataplane; ze has no in-process forwarding path |
| [`RFC3031-x-1`](#rfc3031-x-1) Unknown label in lookup: discard packet (Validation) | no test | no test carries this requirement id; annotated {not-applicable}: discarding a packet whose top label has no ILM entry is executed by the kernel/VPP dataplane on a lookup miss; ze programs the entries and has no MPLS packet-forwarding path that could observe an unknown-label miss |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3031-3.8-1`](#rfc3031-3.8-1)

Label forwarding MUST use the Next Hop Label Forwarding Entry (NHLFE) for lookup (S3.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.8-1, so no unit is bound to it.

### [`RFC3031-3.14-1`](#rfc3031-3.14-1)

Merged upstream labels MUST all map into the same egress point (label merging) (S3.14)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.14-1, so no unit is bound to it.

### [`RFC3031-3.10-1`](#rfc3031-3.10-1)

If the packet's label stack is empty, forward based on network layer header (S3.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.10-1, so no unit is bound to it.

### [`RFC3031-3.24-1`](#rfc3031-3.24-1)

TTL field MUST be decremented at each LSR (S3.24)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.24-1, so no unit is bound to it.

### [`RFC3031-3.24-2`](#rfc3031-3.24-2)

If TTL reaches 0, packet MUST be discarded (S3.24)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.24-2, so no unit is bound to it.

### [`RFC3031-3.10-2`](#rfc3031-3.10-2)

Top label only determines forwarding; lower labels are opaque to transit LSRs (S3.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.10-2, so no unit is bound to it.

### [`RFC3031-x-1`](#rfc3031-x-1)

Unknown label in lookup: discard packet (Validation)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-x-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 3031, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 3031, so its obligations are stated where they were written.
