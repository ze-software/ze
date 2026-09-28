# RFC 3031 - Multiprotocol Label Switching Architecture

No row in the public ledger. Every requirement this repository extracted from RFC 3031, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 12 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 12 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 12 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 12 | of 15 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (third-party), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 6 | of 12 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 50.0% | 6 of 12 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 12 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 12 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 50.0% | 6 of 12 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 12 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (third-party) |
| Requirements | 15 |
| Gated MUST-level | 12 |
| Not applicable, so out of scope | 6 |
| Declared gaps | 0 |
| Gated with no test | 6 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3031.md` |
| Requirement shard | `rfc/requirements/rfc3031.md` |
| RFC text | `rfc/full/rfc3031.txt` |

## Enrolment

Not enrolled (third-party, a layer under or beside Ze performs the document and Ze holds no Go code for it, so the reason beside this kind names the component that does): The Linux AF_MPLS forwarding path performs label lookup, swap and TTL handling. Ze only installs the swap route, internal/plugins/fib/kernel/mplsentry_linux.go::addMPLSSwap.

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 3031.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 6 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **12** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (6):** [`RFC3031-3.8-1`](#rfc3031-3.8-1), [`RFC3031-3.10-1`](#rfc3031-3.10-1), [`RFC3031-3.24-1`](#rfc3031-3.24-1), [`RFC3031-3.24-2`](#rfc3031-3.24-2), [`RFC3031-3.10-2`](#rfc3031-3.10-2), [`RFC3031-x-1`](#rfc3031-x-1)

**No test and no annotation (6):** [`RFC3031-3.14-1`](#rfc3031-3.14-1), [`RFC3031-3.16-1`](#rfc3031-3.16-1), [`RFC3031-3.16-2`](#rfc3031-3.16-2), [`RFC3031-3.16-3`](#rfc3031-3.16-3), [`RFC3031-3.23-1`](#rfc3031-3.23-1), [`RFC3031-4.1.2.2-1`](#rfc3031-4.1.2.2-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3031-3.8-1` | Label forwarding MUST use the Next Hop Label Forwarding Entry (NHLFE) for lookup (S3.8) | MUST | 3.8 - Label Retention Mode | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze is an MPLS control plane that programs the incoming-label-to-NHLFE mapping as kernel AF_MPLS routes (internal/plugins/fib/kernel/mplsentry_linux.go:21 addMPLSSwap) and VPP entries (internal/plugins/fib/vpp/mpls.go); the NHLFE lookup on a forwarded packet is executed by the kernel/VPP dataplane, and ze has no in-process MPLS packet-forwarding path |
| `RFC3031-3.14-1` | In all other cases, Rd MUST NOT distribute to Ru bindings of the same label value to two different FECs (S3.14) | MUST | 3.14 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-3.10-1` | If the packet's "next hop" is the current LSR, then the label stack operation MUST be to "pop the stack" (S3.10) | MUST | 3.10 - The NHLFE | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the NHLFE label stack operation is executed on a forwarded packet by the kernel AF_MPLS or VPP dataplane; ze programs the pop disposition as a route (internal/plugins/fib/kernel/mplsentry_linux.go:44-56) and has no in-process MPLS packet-forwarding path |
| `RFC3031-3.24-1` | TTL field MUST be decremented at each LSR (S3.24) | MUST | 3.24 - Loop Control | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** MPLS label-stack TTL decrement on a forwarded packet is performed by the kernel/VPP dataplane; ze sets only the initial push TTL (internal/plugins/fib/vpp/mpls.go) and has no transit packet-forwarding TTL code path |
| `RFC3031-3.24-2` | If TTL reaches 0, packet MUST be discarded (S3.24) | MUST | 3.24 - Loop Control | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** discarding a packet whose MPLS TTL reached zero is executed by the kernel/VPP dataplane; ze has no MPLS packet-forwarding path that could observe or act on the label-stack TTL |
| `RFC3031-3.10-2` | Top label only determines forwarding; lower labels are opaque to transit LSRs (S3.10) | MUST | 3.10 - The NHLFE | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keys each programmed swap entry on the top/incoming label (internal/plugins/fib/kernel/mplsentry_linux.go:24 MPLSDst), but the top-label-only forwarding decision on a packet is executed by the kernel dataplane; ze has no in-process forwarding path |
| `RFC3031-x-1` | Unknown label in lookup: discard packet (Validation) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** discarding a packet whose top label has no ILM entry is executed by the kernel/VPP dataplane on a lookup miss; ze programs the entries and has no MPLS packet-forwarding path that could observe an unknown-label miss |
| `RFC3031-3.16-1` | An LSR which is capable of popping the label stack at all MUST do penultimate hop popping when so requested by its downstream label distribution peer (S3.16) | MUST | 3.16 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-3.16-2` | Initial label distribution protocol negotiations MUST allow each LSR to determine whether its neighboring LSRS are capable of popping the label stack. (S3.16) | MUST | 3.16 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-3.16-3` | A LSR MUST NOT request a label distribution peer to pop the label stack unless it is capable of doing so (S3.16) | MUST | 3.16 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-3.23-1` | If the label values are encoded in a "shim" that sits between the data link and network layer headers, then this shim MUST have a TTL field that SHOULD be initially loaded from the network layer header TTL field, SHOULD be decremented at each LSR-hop, and SHOULD be copied into the network layer header TTL field when the packet emerges from its LSP (S3.23) | MUST | 3.23 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-4.1.2.2-1` | In order to use MPLS for the forwarding of packets according to the hop-by-hop route corresponding to any address prefix, each LSR MUST bind one or more labels to each address prefix that appears in its routing table, and for each such address prefix X use a label distribution protocol to distribute the binding of a label to X to each of its label distribution peers for X (S4.1.2.2) | MUST | 4.1.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-3.15-1` | An LSR SHOULD be able to support both liberal and conservative label retention modes (S3.15) | SHOULD | 3.15 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-3.14-2` | LSR MAY support label merging to reduce label consumption (S3.14) | MAY | 3.14 | **positive:** no positive test. **negative:** no negative test |
| `RFC3031-2.1-1` | Explicitly routed LSPs MAY be used for traffic engineering (S2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3031-3.8-1`](#rfc3031-3.8-1) Label forwarding MUST use the Next Hop Label Forwarding Entry (NHLFE) for lookup (S3.8) | no test | no test carries this requirement id; annotated {not-applicable}: ze is an MPLS control plane that programs the incoming-label-to-NHLFE mapping as kernel AF_MPLS routes (internal/plugins/fib/kernel/mplsentry_linux.go:21 addMPLSSwap) and VPP entries (internal/plugins/fib/vpp/mpls.go); the NHLFE lookup on a forwarded packet is executed by the kernel/VPP dataplane, and ze has no in-process MPLS packet-forwarding path |
| [`RFC3031-3.14-1`](#rfc3031-3.14-1) In all other cases, Rd MUST NOT distribute to Ru bindings of the same label value to two different FECs (S3.14) | no test | no test carries this requirement id |
| [`RFC3031-3.10-1`](#rfc3031-3.10-1) If the packet's "next hop" is the current LSR, then the label stack operation MUST be to "pop the stack" (S3.10) | no test | no test carries this requirement id; annotated {not-applicable}: the NHLFE label stack operation is executed on a forwarded packet by the kernel AF_MPLS or VPP dataplane; ze programs the pop disposition as a route (internal/plugins/fib/kernel/mplsentry_linux.go:44-56) and has no in-process MPLS packet-forwarding path |
| [`RFC3031-3.24-1`](#rfc3031-3.24-1) TTL field MUST be decremented at each LSR (S3.24) | no test | no test carries this requirement id; annotated {not-applicable}: MPLS label-stack TTL decrement on a forwarded packet is performed by the kernel/VPP dataplane; ze sets only the initial push TTL (internal/plugins/fib/vpp/mpls.go) and has no transit packet-forwarding TTL code path |
| [`RFC3031-3.24-2`](#rfc3031-3.24-2) If TTL reaches 0, packet MUST be discarded (S3.24) | no test | no test carries this requirement id; annotated {not-applicable}: discarding a packet whose MPLS TTL reached zero is executed by the kernel/VPP dataplane; ze has no MPLS packet-forwarding path that could observe or act on the label-stack TTL |
| [`RFC3031-3.10-2`](#rfc3031-3.10-2) Top label only determines forwarding; lower labels are opaque to transit LSRs (S3.10) | no test | no test carries this requirement id; annotated {not-applicable}: ze keys each programmed swap entry on the top/incoming label (internal/plugins/fib/kernel/mplsentry_linux.go:24 MPLSDst), but the top-label-only forwarding decision on a packet is executed by the kernel dataplane; ze has no in-process forwarding path |
| [`RFC3031-x-1`](#rfc3031-x-1) Unknown label in lookup: discard packet (Validation) | no test | no test carries this requirement id; annotated {not-applicable}: discarding a packet whose top label has no ILM entry is executed by the kernel/VPP dataplane on a lookup miss; ze programs the entries and has no MPLS packet-forwarding path that could observe an unknown-label miss |
| [`RFC3031-3.16-1`](#rfc3031-3.16-1) An LSR which is capable of popping the label stack at all MUST do penultimate hop popping when so requested by its downstream label distribution peer (S3.16) | no test | no test carries this requirement id |
| [`RFC3031-3.16-2`](#rfc3031-3.16-2) Initial label distribution protocol negotiations MUST allow each LSR to determine whether its neighboring LSRS are capable of popping the label stack. (S3.16) | no test | no test carries this requirement id |
| [`RFC3031-3.16-3`](#rfc3031-3.16-3) A LSR MUST NOT request a label distribution peer to pop the label stack unless it is capable of doing so (S3.16) | no test | no test carries this requirement id |
| [`RFC3031-3.23-1`](#rfc3031-3.23-1) If the label values are encoded in a "shim" that sits between the data link and network layer headers, then this shim MUST have a TTL field that SHOULD be initially loaded from the network layer header TTL field, SHOULD be decremented at each LSR-hop, and SHOULD be copied into the network layer header TTL field when the packet emerges from its LSP (S3.23) | no test | no test carries this requirement id |
| [`RFC3031-4.1.2.2-1`](#rfc3031-4.1.2.2-1) In order to use MPLS for the forwarding of packets according to the hop-by-hop route corresponding to any address prefix, each LSR MUST bind one or more labels to each address prefix that appears in its routing table, and for each such address prefix X use a label distribution protocol to distribute the binding of a label to X to each of its label distribution peers for X (S4.1.2.2) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3031-3.8-1`](#rfc3031-3.8-1)

Label forwarding MUST use the Next Hop Label Forwarding Entry (NHLFE) for lookup (S3.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.8-1, so no unit is bound to it.

### [`RFC3031-3.14-1`](#rfc3031-3.14-1)

In all other cases, Rd MUST NOT distribute to Ru bindings of the same label value to two different FECs (S3.14)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.14-1, so no unit is bound to it.

### [`RFC3031-3.10-1`](#rfc3031-3.10-1)

If the packet's "next hop" is the current LSR, then the label stack operation MUST be to "pop the stack" (S3.10)

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

### [`RFC3031-3.16-1`](#rfc3031-3.16-1)

An LSR which is capable of popping the label stack at all MUST do penultimate hop popping when so requested by its downstream label distribution peer (S3.16)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.16-1, so no unit is bound to it.

### [`RFC3031-3.16-2`](#rfc3031-3.16-2)

Initial label distribution protocol negotiations MUST allow each LSR to determine whether its neighboring LSRS are capable of popping the label stack. (S3.16)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.16-2, so no unit is bound to it.

### [`RFC3031-3.16-3`](#rfc3031-3.16-3)

A LSR MUST NOT request a label distribution peer to pop the label stack unless it is capable of doing so (S3.16)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.16-3, so no unit is bound to it.

### [`RFC3031-3.23-1`](#rfc3031-3.23-1)

If the label values are encoded in a "shim" that sits between the data link and network layer headers, then this shim MUST have a TTL field that SHOULD be initially loaded from the network layer header TTL field, SHOULD be decremented at each LSR-hop, and SHOULD be copied into the network layer header TTL field when the packet emerges from its LSP (S3.23)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-3.23-1, so no unit is bound to it.

### [`RFC3031-4.1.2.2-1`](#rfc3031-4.1.2.2-1)

In order to use MPLS for the forwarding of packets according to the hop-by-hop route corresponding to any address prefix, each LSR MUST bind one or more labels to each address prefix that appears in its routing table, and for each such address prefix X use a label distribution protocol to distribute the binding of a label to X to each of its label distribution peers for X (S4.1.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3031-4.1.2.2-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc3031.txt |
| Source fingerprint | da270cc6acad74d1 |
| Record | rfc/extraction/rfc3031.json |
| Mapped sentences | 8 |
| Declined as scope | 73 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 2 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 1 | walked | not stated |
| `3.4` | not stated | 0 | walked | not stated |
| `3.5` | not stated | 1 | walked | not stated |
| `3.6` | not stated | 0 | walked | not stated |
| `3.7` | not stated | 1 | walked | not stated |
| `3.8` | Label Retention Mode | 0 | walked | Label Retention Mode. Indicative prose with no capitalised keyword site. RFC3031-3.8-1 (label forwarding uses the NHLFE for lookup) was read from the indicative description in section 3.10, "The "Next Hop Label Forwarding Entry" (NHLFE) is used when forwarding a labeled packet", and section 3.8 states nothing about the NHLFE; the id keeps its section because ids are permanent. |
| `3.9` | not stated | 2 | walked | not stated |
| `3.10` | The NHLFE | 1 | walked | The NHLFE. One capitalised MUST site, mapped to RFC3031-3.10-1. RFC3031-3.10-2 (only the top label determines forwarding) was read from the indicative prose of section 3.9, "The processing is always based on the top label, without regard for the possibility that some number of other labels may have been "above it" in the past, or that some number of other labels may be below it at present." |
| `3.11` | not stated | 1 | walked | not stated |
| `3.12` | not stated | 1 | walked | not stated |
| `3.13` | not stated | 0 | walked | not stated |
| `3.14` | not stated | 3 | walked | not stated |
| `3.15` | not stated | 0 | walked | not stated |
| `3.16` | not stated | 6 | walked | not stated |
| `3.17` | not stated | 0 | walked | not stated |
| `3.18` | not stated | 1 | walked | not stated |
| `3.19` | not stated | 1 | walked | not stated |
| `3.20` | not stated | 0 | walked | not stated |
| `3.21` | not stated | 0 | walked | not stated |
| `3.22` | not stated | 0 | walked | not stated |
| `3.23` | not stated | 3 | walked | not stated |
| `3.24` | Loop Control | 1 | walked | Loop Control. Its one site is the lower-case loop-detection sentence, excluded. RFC3031-3.24-1 (TTL decremented at each LSR) and RFC3031-3.24-2 (discard at TTL 0) were read from the indicative prose of section 3.23, "Whenever a packet passes through a router, its TTL gets decremented by 1; if the TTL reaches 0 before the packet has reached its destination, the packet gets discarded." Section 3.23 states the shim TTL "SHOULD be decremented at each LSR-hop"; this document states no MUST for either row, and section 3.24 does not mention TTL handling. |
| `3.25` | not stated | 0 | walked | not stated |
| `3.25.1` | not stated | 0 | walked | not stated |
| `3.25.2` | not stated | 2 | walked | not stated |
| `3.25.3` | not stated | 1 | walked | not stated |
| `3.26` | not stated | 3 | walked | not stated |
| `3.26.1` | not stated | 0 | walked | not stated |
| `3.26.2` | not stated | 1 | walked | not stated |
| `3.26.3` | not stated | 0 | walked | not stated |
| `3.26.3.1` | not stated | 1 | walked | not stated |
| `3.26.3.2` | not stated | 1 | walked | not stated |
| `3.27` | not stated | 0 | walked | not stated |
| `3.27.1` | not stated | 0 | walked | not stated |
| `3.27.2` | not stated | 0 | walked | not stated |
| `3.27.3` | not stated | 1 | walked | not stated |
| `3.27.4` | not stated | 1 | walked | not stated |
| `3.27.5` | not stated | 1 | walked | not stated |
| `3.28` | not stated | 0 | walked | not stated |
| `3.29` | not stated | 0 | walked | not stated |
| `3.29.1` | not stated | 0 | walked | not stated |
| `3.29.2` | not stated | 0 | walked | not stated |
| `3.29.3` | not stated | 0 | walked | not stated |
| `3.30` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.1.1` | not stated | 0 | walked | not stated |
| `4.1.2` | not stated | 0 | walked | not stated |
| `4.1.2.1` | not stated | 0 | walked | not stated |
| `4.1.2.2` | not stated | 4 | walked | not stated |
| `4.1.3` | not stated | 2 | walked | not stated |
| `4.1.4` | not stated | 0 | walked | not stated |
| `4.1.5` | not stated | 1 | walked | not stated |
| `4.1.6` | not stated | 3 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 2 | walked | not stated |
| `4.3` | not stated | 3 | walked | not stated |
| `4.4` | not stated | 0 | walked | not stated |
| `4.5` | not stated | 1 | walked | not stated |
| `4.6` | not stated | 3 | walked | not stated |
| `4.7` | not stated | 2 | walked | not stated |
| `4.8` | not stated | 2 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 2 | walked | not stated |
| `5.1.1` | not stated | 1 | walked | not stated |
| `5.1.1.1` | not stated | 1 | walked | not stated |
| `5.1.1.2` | not stated | 0 | walked | not stated |
| `5.1.1.3` | not stated | 1 | walked | not stated |
| `5.1.1.4` | not stated | 4 | walked | not stated |
| `5.1.2` | not stated | 0 | walked | not stated |
| `5.1.2.1` | not stated | 0 | walked | not stated |
| `5.1.2.2` | not stated | 0 | walked | not stated |
| `5.1.2.3` | not stated | 1 | walked | not stated |
| `5.1.3` | not stated | 0 | walked | not stated |
| `5.1.3.1` | not stated | 0 | walked | not stated |
| `5.1.3.2` | not stated | 0 | walked | not stated |
| `5.1.4` | not stated | 0 | walked | not stated |
| `5.1.4.1` | not stated | 0 | walked | not stated |
| `5.1.4.2` | not stated | 0 | walked | not stated |
| `5.1.5` | not stated | 0 | walked | not stated |
| `5.1.5.1` | not stated | 0 | walked | not stated |
| `5.1.5.2` | not stated | 0 | walked | not stated |
| `5.1.6` | not stated | 4 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.2.1` | not stated | 1 | walked | not stated |
| `5.2.2` | not stated | 2 | walked | not stated |
| `5.2.3` | not stated | 3 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | (This will typically be the case when Ru and Rd are not direct neighbors.) In such cases, Rd must make sure that the binding from label to FEC is one-to-one. |
| `3.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.1 states the label-uniqueness prohibition in passing; Section 3.14, "Scope and Uniqueness of Labels", states it in full as "In all other cases, Rd MUST NOT distribute to Ru bindings of the same label value to two different FECs", which RFC3031-3.14-1 carries. | That is, Rd MUST NOT agree with Ru1 to bind L to FEC F1, while also agreeing with some other LSR Ru2 to bind L to a different FEC F2, UNLESS Rd can always tell, when it receives a packet with incoming label L, whether the label was put on the packet by Ru1 or whether it was put on by Ru2. |
| `3.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The particular encoding technique to be used must be agreed to by both the entity which encodes the label and the entity which decodes the label. |
| `3.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Ru, acting as a downstream LSR, also distributes a binding of a label to FEC F, then under certain conditions, it may be required to also distribute the corresponding attribute that it received from Rd. |
| `3.7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | On any given label distribution adjacency, the upstream LSR and the downstream LSR must agree on which technique is to be used. |
| `3.9:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | As we shall see, it is useful to have a more general model in which a labeled packet carries a number of labels, organized as a last-in, first-out stack. |
| `3.9:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Although, as we shall see, MPLS supports a hierarchy, the processing of a labeled packet is completely independent of the level of hierarchy. |
| `3.11:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If the ILM maps a particular label to a set of NHLFEs that contains more than one element, exactly one element of the set must be chosen before the packet is forwarded. |
| `3.12:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If the FTN maps a particular label to a set of NHLFEs that contains more than one element, exactly one element of the set must be chosen before the packet is forwarded. |
| `3.14:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When these conditions do not hold, the labels must be unique over the LSR which has assigned them, and we may say that the LSR is using a "per- platform label space." |
| `3.14:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | However, in such cases the LSR must have some means, not specified by the architecture, of determining, for a particular incoming label, which label space that label belongs to. |
| `3.16:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Then it must pop the stack, and examine what remains of the packet. |
| `3.16:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The creation of the forwarding "fastpath" in a label switching product may be greatly aided if it is known that only a single lookup is ever required: |
| `3.16:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | However, some hardware switching engines may not be able to pop the label stack, so this cannot be universally required. |
| `3.19:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If one wants to ensure that traffic in a particular FEC follows a path with some specified set of properties (e.g., that the traffic does not traverse any node twice, that a specified amount of resources are available to the traffic, that the traffic follows an explicitly specified path, etc.) ordered control must be used. |
| `3.23:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | In this case, the LSR at the ingress to the non-TTL LSP segment must not label switch the packet. |
| `3.23:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | This means that special procedures must be developed to support traceroute functionality, for example, traceroute packets may be forwarded using conventional hop by hop forwarding. |
| `3.24:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | All LSRs that may attach to non-TTL LSP segments will therefore be required to support a common technique for loop detection; however, use of the loop detection technique is optional. |
| `3.25.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | As we shall see in section 3.26, this enables us to do label merging, without running into any cell interleaving problems, on ATM switches which can provide multipoint-to-point VPs, but which do not have the VC merge capability. |
| `3.25.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If there are more labels on the stack than can be encoded in the ATM header, the ATM encodings must be combined with the generic encapsulation. |
| `3.25.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When a labeled packet is received, the LSR must decode it to determine the current value of the label stack, then must operate on the label stack to determine the new value of the stack, and then encode the new value appropriately before transmitting the labeled packet to its next hop. |
| `3.26:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Let us say that an LSR is not capable of label merging if, for any two packets which arrive from different interfaces, or with different labels, the packets must either be transmitted out different interfaces, or must have different labels. |
| `3.26:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If a particular LSR cannot perform label merging, then if two packets in the same FEC arrive with different incoming labels, they must be forwarded with different outgoing labels. |
| `3.26:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | In fact, it is difficult for an LSR to even determine how many such incoming labels it must support for a particular FEC. |
| `3.26.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When a downstream neighbor receives such a request from upstream, and the downstream neighbor does not itself support label merging, then it must in turn ask its downstream neighbor for another label for the FEC in question. |
| `3.26.3.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When VC merge is used, switches are required to buffer cells from one packet until the entire packet is received (this may be determined by looking for the AAL5 end of frame indicator). |
| `3.26.3.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The number required will be determined by allowing the upstream nodes to request additional VPI/VCIs from their downstream neighbors (this is again analogous to the method used with frame merge). |
| `3.27.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The set of packets which are to be sent though the LSP tunnel constitutes a FEC, and each LSR in the tunnel must assign a label to that FEC (i.e., must assign a label to the tunnel). |
| `3.27.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | R2, switching on the label, determines that P must enter the tunnel. |
| `3.27.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Note that in this example, R2 and R21 must be IGP neighbors, but R2 and R3 need not be. |
| `4.1.2.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | There is also one circumstance in which an LSR must distribute a label binding for an address prefix, even if it is not the LSR which bound that label to that address prefix: |
| `4.1.2.2:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If R1 uses BGP to distribute a route to X, naming some other LSR R2 as the BGP Next Hop to X, and if R1 knows that R2 has assigned label L to X, then R1 must distribute the binding between L and X to any BGP peer to which it distributes that route. |
| `4.1.2.2:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | These rules are intended only to indicate which label bindings must be distributed by a given LSR to which other LSRs. |
| `4.1.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | At that point, the LSP must end and the best match algorithm must be performed again. |
| `4.1.3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | In this situation, packet P can be label Switched until it reaches R2, but since R2 has performed route aggregation, it must execute the best match algorithm to find P's FEC. |
| `4.1.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If LSR Ru, by consulting its ILM, sees that labeled packet P must be forwarded next to Rd, but that Rd has distributed a binding of Implicit NULL to the corresponding address prefix, then instead of replacing the value of the label on top of the label stack, Ru pops the label stack, and then forwards the resulting packet to Rd. |
| `4.1.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | There are situations in which an LSP Ingress, Ri, knows that packets of several different FECs must all follow the same LSP, terminating at, say, LSP Egress Re. |
| `4.1.6:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | - If the network is running a link state routing algorithm, and all nodes in the area support MPLS, then the routing algorithm provides Ri with enough information to determine the routers through which packets in that FEC must leave the routing domain or area. |
| `4.1.6:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | - If the network is running BGP, Ri may be able to determine that the packets in a particular FEC must leave the network via some particular router which is the "BGP Next Hop" for that FEC. |
| `4.2.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If the transmit endpoint of the tunnel wishes to put a labeled packet into the tunnel, it must first replace the label value at the top of the stack with a label value that was distributed to it by the tunnel's receive endpoint. |
| `4.2.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Then it must push on the label which corresponds to the tunnel itself, as distributed to it by the next hop along the tunnel. |
| `4.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Ru must push L2 and then L1 onto the packet's label stack, and then forward the packet to Rd; |
| `4.3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When Ru distributes label bindings for X to its label distribution peers, it must include L2 as the stack attribute. |
| `4.3:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Whenever the stack attribute changes (possibly as a result of a change in Ru's LSP next hop for X), Ru must distribute the new stack attribute. |
| `4.5:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Since conventional ATM switches do not support multipoint-to-point connections, there must be procedures to ensure that each LSP is realized as a point-to-point VC. |
| `4.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | However, there must be some means of ensuring that the transit traffic will be delivered from Border Router to Border Router by the interior routers. |
| `4.6:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Then before sending packet P to I1, B1 must create a label stack for P, then push on label L1, and then push on label L2. |
| `4.6:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Then before sending packet P to I1, B1 must replace the label at the top of the label stack with L1, and then push on label L2. |
| `4.7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If the transmit endpoint of the tunnel wishes to put a labeled packet into the tunnel, it must first replace the label value at the top of the stack with a label value that was distributed to it by the tunnel's receive endpoint. |
| `4.7:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Then it must push on the label which corresponds to the tunnel itself, as distributed to it by the next hop along the tunnel. |
| `4.8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The tree along which a particular multicast packet must get forwarded depends in general on the packet's source address and its destination address. |
| `4.8:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | (If the node in question is on a LAN, and has siblings on that LAN, it must also distribute the binding to its siblings. |
| `5.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The downstream LSR must perform: |
| `5.1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | The upstream LSR must perform: |
| `5.1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Irrespective of the particular procedure that is used, if a label binding for a particular address prefix has been distributed by a downstream LSR Rd to an upstream LSR Ru, and if at any time the attributes (as defined above) of that binding change, then Rd must inform Ru of the new attributes. |
| `5.1.1.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Whenever these conditions hold, Rd must bind a label to X and distribute that binding to Ru. |
| `5.1.1.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Note that if X is not in Rd's routing table, or if Rd is not a label distribution peer of Ru with respect to X, then Rd must inform Ru that it cannot provide a binding at this time. |
| `5.1.1.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Note that if X is not in Rd's routing table and a binding for X is not obtainable via Rd's next hop for X, or if Rd is not a label distribution peer of Ru with respect to X, then Rd must inform Ru that it cannot provide a binding at this time. |
| `5.1.1.4:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | However, if the only condition that fails to hold is that Rn has not yet provided a label to Rd, then Rd must defer any response to Ru until such time as it has receiving a binding from Rn. |
| `5.1.1.4:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Rd has distributed a label binding for address prefix X to Ru, and at some later time, any attribute of the label binding changes, then Rd must redistribute the label binding to Ru, with the new attribute. |
| `5.1.1.4:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | It must do this even though Ru does not issue a new Request. |
| `5.1.2.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Rd receives such a request from Ru, for an address prefix for which Rd has already distributed Ru a label, Rd shall assign a new (distinct) label, bind it to X, and distribute that binding. |
| `5.1.6:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | When LSR Rd decides to break the binding between label L and address prefix X, then this unbinding must be distributed to all LSRs to which the binding was distributed. |
| `5.1.6:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | It is required that the unbinding of L from X be distributed by Rd to a LSR Ru before Rd distributes to Ru any new binding of L to any other address prefix Y, where X != Y. |
| `5.1.6:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If LSR R1 has a label distribution adjacency to LSR R2, and has received label bindings from LSR R2 via that adjacency, then if adjacency is brought down by either peer (whether as a result of failure or as a matter of normal operation), all bindings received over that adjacency must be considered to have been withdrawn. |
| `5.1.6:4` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | As long as the relevant label distribution adjacency remains in place, label bindings that are withdrawn must always be withdrawn explicitly. |
| `5.2.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Ru and Rd are label distribution peers, and both support label merging, one of the following schemes must be used: |
| `5.2.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Since there is no multipoint-to-point capability, the LSPs must be realized as point-to-point VCs, which means that there needs to be three such VCs for address prefix X: <R1, R2, R3, R4>, <R2, R3, R4>, and <R3, R4>. |
| `5.2.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Therefore, if R1 and R2 are MPLS peers, and either is an LSR which is implemented using conventional ATM switching hardware (i.e., no cell interleave suppression), or is otherwise incapable of performing label merging, the MPLS scheme in use between R1 and R2 must be one of the following: |
| `5.2.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | Each must state whether it supports label merging. |
| `5.2.3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Rd does not support label merging, Rd must choose either the PulledUnconditional procedure or the PulledConditional procedure. |
| `5.2.3:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 binds the normative reading to the capitalised key words: "The key words \\"MUST\\", \\"MUST NOT\\", \\"REQUIRED\\", \\"SHALL\\", \\"SHALL NOT\\", \\"SHOULD\\", \\"SHOULD NOT\\", \\"RECOMMENDED\\", \\"MAY\\", and \\"OPTIONAL\\" in this document are to be interpreted as described in [RFC2119]." This site's keyword is lower case, so it is descriptive architecture prose rather than a normative obligation. | If Ru does not support label merging, but Rd does, Ru must choose either the RequestRetry or RequestNoRetry procedure. |
| `10:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate: the RFC Editor's full copyright statement, which constrains republication of the document and states no protocol obligation. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 3031, so its obligations are stated where they were written.
