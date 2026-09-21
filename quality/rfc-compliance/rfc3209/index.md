# RFC 3209 - RSVP-TE: Extensions to RSVP for LSP Tunnels

Experimental. Every requirement this repository extracted from RFC 3209, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 6.5% | 4 of 62 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 8.1% | 5 of 62 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| Proven by a recorded break | 0.0% | 0 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 62 | of 65 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 62 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 62 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 62 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 62 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| One polarity, unexcused | 1.6% | 1 of 62 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 83.9% | 52 of 62 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 62 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | bad | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 65 |
| Gated MUST-level | 62 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 51 |
| Nightly-only evidence | 0 |
| Test tags | 16 |
| Tagged units | 16 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc3209.md` |
| Requirement shard | `rfc/requirements/rfc3209.md` |
| RFC text | `rfc/full/rfc3209.txt` |

## Enrolment

Enrolled: RSVP-TE (RFC 3209): PATH/RESV signaling, ERO, LABEL, SE make-before-break, soft-state. The 2026-09-21 extraction walk added the 50 MUST-level and SHALL-level rows the earlier summary did not list, so the checklist now carries 65 rows and every one of them is sourced to a sentence of the document. The added rows carry no test.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered:**

PATH and RESV signaling, ERO routing, bandwidth admission, SE-style make-before-break, soft-state refresh/expiry, teardown.

**What the ledger says remains**

The strict-hop gap moved with its sentence: RFC 3209 states the strict-node adjacency rule in Section 4.3.3.1, so it is now [`RFC3209-4.3.3.1-1`](#rfc3209-4.3.3.1-1), and the transit next-hop selector (engine.go) still ignores the ERO L-bit and does no adjacency check, so a non-adjacent strict hop is not rejected. RFC3209-x-1 names an IP Router Alert option that RFC 3209 does not mention at any point; the raw IP transport (transport_linux.go) does send PATH without it, but that is the RFC 2205 transport gap and RFC 3209 states no such requirement. The 50 rows the 2026-09-21 walk added are untested and are gaps until tests exist. Cross-vendor interop remains constrained by available open daemons.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated instead of tested | 6 | one part of the gated population |
| One polarity only | 1 | one part of the gated population |
| No test and no annotation | 51 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **62** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC3209-4.1-1`](#rfc3209-4.1-1), [`RFC3209-4.3.4-1`](#rfc3209-4.3.4-1), [`RFC3209-6-1`](#rfc3209-6-1), [`RFC3209-2.5-1`](#rfc3209-2.5-1)

**Annotated instead of tested (6):** [`RFC3209-4.6.1-1`](#rfc3209-4.6.1-1), [`RFC3209-4.6.2-1`](#rfc3209-4.6.2-1), [`RFC3209-4.2-1`](#rfc3209-4.2-1), [`RFC3209-4.1-3`](#rfc3209-4.1-3), [`RFC3209-6-2`](#rfc3209-6-2), [`RFC3209-x-1`](#rfc3209-x-1)

**One polarity only (1):** [`RFC3209-4.6.1-2`](#rfc3209-4.6.1-2)

**No test and no annotation (51):** [`RFC3209-4.3.4.1-1`](#rfc3209-4.3.4.1-1), [`RFC3209-2.6-1`](#rfc3209-2.6-1), [`RFC3209-2.6-2`](#rfc3209-2.6-2), [`RFC3209-2.6-3`](#rfc3209-2.6-3), [`RFC3209-2.6-4`](#rfc3209-2.6-4), [`RFC3209-2.6-5`](#rfc3209-2.6-5), [`RFC3209-3-1`](#rfc3209-3-1), [`RFC3209-3-2`](#rfc3209-3-2), [`RFC3209-4.1.1.1-1`](#rfc3209-4.1.1.1-1), [`RFC3209-4.1.1.1-2`](#rfc3209-4.1.1.1-2), [`RFC3209-4.1.1.1-3`](#rfc3209-4.1.1.1-3), [`RFC3209-4.2.1-1`](#rfc3209-4.2.1-1), [`RFC3209-4.2.2-1`](#rfc3209-4.2.2-1), [`RFC3209-4.2.2-2`](#rfc3209-4.2.2-2), [`RFC3209-4.2.2-3`](#rfc3209-4.2.2-3), [`RFC3209-4.2.3-1`](#rfc3209-4.2.3-1), [`RFC3209-4.2.3-2`](#rfc3209-4.2.3-2), [`RFC3209-4.2.4-1`](#rfc3209-4.2.4-1), [`RFC3209-4.2.4-2`](#rfc3209-4.2.4-2), [`RFC3209-4.2.4-3`](#rfc3209-4.2.4-3), [`RFC3209-4.2.4-4`](#rfc3209-4.2.4-4), [`RFC3209-4.2.5-1`](#rfc3209-4.2.5-1), [`RFC3209-4.3.3-1`](#rfc3209-4.3.3-1), [`RFC3209-4.3.3.1-1`](#rfc3209-4.3.3.1-1), [`RFC3209-4.3.4.2-1`](#rfc3209-4.3.4.2-1), [`RFC3209-4.4.1-1`](#rfc3209-4.4.1-1), [`RFC3209-4.4.3-1`](#rfc3209-4.4.3-1), [`RFC3209-4.4.3-2`](#rfc3209-4.4.3-2), [`RFC3209-4.4.3-3`](#rfc3209-4.4.3-3), [`RFC3209-4.4.3-4`](#rfc3209-4.4.3-4), [`RFC3209-4.6.2-2`](#rfc3209-4.6.2-2), [`RFC3209-4.7.3-1`](#rfc3209-4.7.3-1), [`RFC3209-4.7.4-1`](#rfc3209-4.7.4-1), [`RFC3209-4.7.4-2`](#rfc3209-4.7.4-2), [`RFC3209-4.7.4-3`](#rfc3209-4.7.4-3), [`RFC3209-5.2.2-1`](#rfc3209-5.2.2-1), [`RFC3209-5.2.2-2`](#rfc3209-5.2.2-2), [`RFC3209-5.2.2-3`](#rfc3209-5.2.2-3), [`RFC3209-5.3-1`](#rfc3209-5.3-1), [`RFC3209-5.3-2`](#rfc3209-5.3-2), [`RFC3209-5.3-3`](#rfc3209-5.3-3), [`RFC3209-5.3-4`](#rfc3209-5.3-4), [`RFC3209-5.3-5`](#rfc3209-5.3-5), [`RFC3209-5.3-6`](#rfc3209-5.3-6), [`RFC3209-5.3-7`](#rfc3209-5.3-7), [`RFC3209-5.3-8`](#rfc3209-5.3-8), [`RFC3209-5.3-9`](#rfc3209-5.3-9), [`RFC3209-5.3-10`](#rfc3209-5.3-10), [`RFC3209-5.3-11`](#rfc3209-5.3-11), [`RFC3209-5.4-1`](#rfc3209-5.4-1), [`RFC3209-5.4-2`](#rfc3209-5.4-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3209-4.6.1-1` | The LSP_TUNNEL_IPv4 SESSION object's reserved field MUST be zero (S4.6.1, Wire Format) | MUST | 4.6.1 | **positive:** `unit/verify` [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L338). **negative:** no negative test. **{single-polarity}:** the encoder writes the SESSION reserved octet as 0 on every SESSION it emits, but the decoder never reads that octet, so a non-zero-reserved reject test is not meaningful (internal/plugins/rsvpte/wire.go:238, :243) |
| `RFC3209-4.6.2-1` | The LSP_TUNNEL_IPv4 SENDER_TEMPLATE object's reserved field MUST be zero (S4.6.2, Wire Format) | MUST | 4.6.2 | **positive:** `unit/verify` [`TestRSVPSenderTemplateReservedZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L368). **negative:** no negative test. **{single-polarity}:** the encoder zeroes the 2-byte reserved field before LSP ID on every SENDER_TEMPLATE, and decodeSenderTemplate never inspects it (internal/plugins/rsvpte/wire.go:270, :275-276) |
| `RFC3209-4.6.1-2` | The LSP_TUNNEL_IPv6 SESSION object's reserved field MUST be zero (S4.6.1, Wire Format) | MUST | 4.6.1 | **positive:** `unit/verify` [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L337). **negative:** no negative test |
| `RFC3209-4.2-1` | LABEL_REQUEST object MUST be present in PATH messages to request label allocation (S4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestBuildPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L45). **negative:** no negative test. **{single-polarity}:** buildPath unconditionally appends a LABEL_REQUEST to every PATH it composes, and handlePath does not reject a received PATH lacking one, so only the positive is meaningful (internal/plugins/rsvpte/build.go:92) |
| `RFC3209-4.1-1` | The label for a sender MUST immediately follow the FILTER_SPEC for that sender in the Resv message (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L75). **negative:** `unit/verify` [`TestEngineResvWithoutLabelRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L364) |
| `RFC3209-4.1-2` | Labels MAY be carried in Resv messages (S4.1) | MAY | 4.1 | **positive:** `unit/verify` [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L296). **negative:** `unit/verify` [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L297) |
| `RFC3209-4.3.4-1` | ERO processing: transit node MUST remove itself (first subobject) from ERO before forwarding PATH (S4.3.4) | MUST | 4.3.4 | **positive:** `unit/verify` [`TestEngineTransitForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L195). **negative:** `unit/verify` [`TestEngineTransitNoUsableERONextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L406) |
| `RFC3209-4.3.4.1-1` | The node receiving an RSVP message with an EXPLICIT_ROUTE object MUST first evaluate the first subobject (S4.3.4.1) | MUST | 4.3.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.1-3` | Reserved label values 0-15: only label 3 (Implicit NULL) MAY be allocated for penultimate hop popping (S4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestLSPTableAllocateSkipsReservedLabels`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/fsm_test.go#L70). **negative:** no negative test. **{single-polarity}:** the local label allocator starts at firstDynamicLabel=1000 and wraps back to 1000, so ze never hands out any label in 0-15 and there is no receive path that would allocate a reserved label (internal/plugins/rsvpte/fsm.go:184, :205, :215-217) |
| `RFC3209-2.6-1` | An LSR MUST execute the labeled-datagram forwarding algorithm of Section 2.6 (S2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-2.6-2` | When the datagram size exceeds M and the datagram may be fragmented, the datagram MUST be broken into fragments, each of whose size is no greater than M (S2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-2.6-3` | Each fragment of such a datagram MUST be labeled and then forwarded (S2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-2.6-4` | When the datagram size exceeds M and the datagram may not be fragmented, the datagram MUST NOT be forwarded (S2.6) | MUST NOT | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-2.6-5` | When the size of an IPv6 datagram without labels exceeds M, the datagram MUST NOT be forwarded (S2.6) | MUST NOT | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-3-1` | In Resv messages the new objects MUST appear after the associated FILTER_SPEC and prior to any subsequent FILTER_SPEC (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-3-2` | The ordering of these objects is not important, so an implementation MUST be prepared to accept objects in any order (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.1.1.1-1` | If a label range has been specified in the label request, the label MUST be drawn from that range (S4.1.1.1) | MUST | 4.1.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.1.1.1-2` | A node that intends to police individual senders to a session MUST assign unique labels to those senders (S4.1.1.1) | MUST | 4.1.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.1.1.1-3` | If for any senders the M-bit is not set, the downstream node MUST assign unique labels to those senders (S4.1.1.1) | MUST | 4.1.1.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.2.1-1` | The Reserved field of a Label Request without label range MUST be set to zero on transmission and MUST be ignored on receipt (S4.2.1) | MUST | 4.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.2.2-1` | The Reserved field of a Label Request with ATM label range MUST be set to zero on transmission and MUST be ignored on receipt (S4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.2.2-2` | If the VPI is less than 12 bits it MUST be right justified in its field and preceding bits MUST be set to zero (S4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.2.2-3` | If the VCI is less than 16 bits it MUST be right justified in its field and preceding bits MUST be set to zero (S4.2.2) | MUST | 4.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.2.3-1` | The Reserved field of a Label Request with Frame Relay label range MUST be set to zero on transmission and ignored on receipt (S4.2.3) | MUST | 4.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.2.3-2` | The DLCI MUST be right justified in its field and unused bits MUST be set to 0 (S4.2.3) | MUST | 4.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.2.4-1` | A receiver that accepts a LABEL_REQUEST object MUST include a LABEL object in Resv messages pertaining to that Path message (S4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.2.4-2` | If a LABEL_REQUEST object was not present in the Path message, a node MUST NOT include a LABEL object in a Resv message for that Path message's session and PHOP (S4.2.4) | MUST NOT | 4.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.2.4-3` | A node that sends a LABEL_REQUEST object MUST be ready to accept and correctly process a LABEL object in the corresponding Resv messages (S4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.2.4-4` | A node which receives and forwards a Path message with a LABEL_REQUEST object MUST copy the L3PID from the received LABEL_REQUEST object to the forwarded LABEL_REQUEST object (S4.2.4) | MUST | 4.2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.2.5-1` | A router with a neighbor known not to be RSVP capable MUST NOT advertise the LABEL_REQUEST object when sending messages that pass through the non-RSVP routers (S4.2.5) | MUST NOT | 4.2.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.3.3-1` | An EXPLICIT_ROUTE subobject Length MUST be at least 4, and MUST be a multiple of 4 (S4.3.3) | MUST | 4.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.3.3.1-1` | The path between a strict node and its preceding node MUST include only network nodes from the strict node and its preceding abstract node (S4.3.3.1) | MUST | 4.3.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.3.4.2-1` | Each subobject added to the EXPLICIT_ROUTE object MUST denote an abstract node that is a subset of the current abstract node (S4.3.4.2) | MUST | 4.3.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.4.1-1` | A RECORD_ROUTE subobject Length MUST always be a multiple of 4, and at least 4 (S4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.4.3-1` | The subobject a node newly adds to the RRO MUST be that router's IP address (S4.4.3) | MUST | 4.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.4.3-2` | A node MUST NOT push on a Label Record subobject without also pushing on an IPv4 or IPv6 subobject (S4.4.3) | MUST NOT | 4.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.4.3-3` | If the newly added subobject causes the RRO to be too big to fit in a Path or Resv message, the RRO object SHALL be dropped from the message and message processing continues as normal (S4.4.3) | SHALL | 4.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.4.3-4` | Subsequent Resv messages SHALL NOT contain an RRO (S4.4.3) | SHALL NOT | 4.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.6.2-2` | The LSP_TUNNEL_IPv6 SENDER_TEMPLATE object's reserved field MUST be zero (S4.6.2) | MUST | 4.6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.7.3-1` | A SESSION_ATTRIBUTE object Length MUST always be a multiple of 4 and MUST be at least 8 (S4.7.3) | MUST | 4.7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.7.4-1` | In order to be validated a link MUST pass the three resource-affinity tests (S4.7.4) | MUST | 4.7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.7.4-2` | When a node is choosing links in order to extend a loose node of an ERO, the node MUST validate the resource classes of those links against the resource affinities (S4.7.4) | MUST | 4.7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.7.4-3` | All RSVP routers, whether they support the SESSION_ATTRIBUTE object or not, SHALL forward the object unmodified (S4.7.4) | SHALL | 4.7.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.2.2-1` | The Src_Instance value MUST change when the sender is reset, when the node reboots, or when communication is lost to the neighboring node, and otherwise remains the same (S5.2.2) | MUST | 5.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.2.2-2` | The Src_Instance field MUST NOT be set to zero (S5.2.2) | MUST NOT | 5.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.2.2-3` | The Dst_Instance field MUST be set to zero when no value has ever been seen from the neighbor (S5.2.2) | MUST | 5.2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.3-1` | The Src_Instance value MUST NOT change while the agent is exchanging Hellos with the corresponding neighbor (S5.3) | MUST NOT | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.3-2` | On receipt of a message containing a HELLO REQUEST object, the receiver MUST generate a Hello message containing a HELLO ACK object (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.3-3` | If the neighbor's Src_Instance value differs from the value previously received, or the Src_Instance field is zero, the node MUST treat the neighbor as if communication has been lost (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.3-4` | If the neighbor continues to advertise a wrong non-zero value after a configured number of intervals, the node MUST treat the neighbor as if communication has been lost (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.3-5` | On receipt of a message containing a HELLO ACK object, the receiver MUST verify that the neighbor has not reset (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.3-6` | The receiver of a HELLO ACK object MUST also verify that the neighbor is reflecting back the receiver's Instance value (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.3-7` | If the neighbor advertises a wrong value in the Dst_Instance field, a node MUST treat the neighbor as if communication has been lost (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.3-8` | If no Instance values are received, via either REQUEST or ACK objects, from a neighbor within a configured number of hello_intervals, a node MUST presume that it cannot communicate with the neighbor (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.3-9` | A node that re-initiates Hellos MUST use a Src_Instance value different than the one advertised in the previous HELLO message (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.3-10` | The new Src_Instance value MUST continue to be advertised to the corresponding neighbor until a reset or reboot occurs, or until another communication failure is detected (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.3-11` | If a new instance value has not been received from the neighbor, the node MUST advertise zero in the Dst_Instance value field (S5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.4-1` | When the links between neighbors are numbered, Hellos MUST be run on each link (S5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-5.4-2` | When the links are unnumbered, link failure detection MUST be provided by some means other than Hellos (S5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-6-1` | Make-before-break: old and new LSPs MUST have the same SESSION (same Tunnel Endpoint, Tunnel ID, Extended Tunnel ID); only LSP ID differs (S6) | MUST | 6 | **positive:** `unit/verify` [`TestEngineMakeBeforeBreak`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/reroute_test.go#L41). **negative:** `unit/verify` [`TestSEAdmissionDistinctSessionsDoNotShare`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/admission_se_test.go#L35) |
| `RFC3209-6-2` | SE (Shared Explicit) style MUST be used for make-before-break rerouting (S6) | MUST | 6 | **positive:** `unit/verify` [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L78). **negative:** no negative test. **{single-polarity}:** every RESV ze originates carries STYLE = Shared Explicit (18) and admission always applies SE sharing semantics; ze never emits a Fixed-Filter style for an LSP tunnel (internal/plugins/rsvpte/engine.go:272, build.go:126, wire.go:680) |
| `RFC3209-x-1` | PATH messages MUST include the Router Alert IP option (Transport) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** the raw IPv4 socket (proto 46) is opened and used for Sendto with no IP_ROUTER_ALERT socket option and no per-packet IP option, so emitted PATH datagrams carry no Router Alert (the same transport gap as RFC2205-x-1) (internal/plugins/rsvpte/transport_linux.go:35-58, :60-69) |
| `RFC3209-2.5-1` | Both PATH and RESV messages MUST be refreshed periodically for soft-state maintenance (S2.5) | MUST | 2.5 | **positive:** `unit/verify` [`TestRefreshResendsPathAndResv`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/softstate_test.go#L56). **negative:** `unit/verify` [`TestRefreshDoesNotStampEgressPSB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/softstate_test.go#L21) |
| `RFC3209-x-2` | Admission control failure SHOULD generate PathErr with Error Code 1 (Admission Control) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC3209-4.4-1` | RRO MAY be included in PATH and RESV to record the actual path taken (S4.4) | MAY | 4.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3209-4.3.4.1-1`](#rfc3209-4.3.4.1-1) The node receiving an RSVP message with an EXPLICIT_ROUTE object MUST first evaluate the first subobject (S4.3.4.1) | no test | no test carries this requirement id |
| [`RFC3209-2.6-1`](#rfc3209-2.6-1) An LSR MUST execute the labeled-datagram forwarding algorithm of Section 2.6 (S2.6) | no test | no test carries this requirement id |
| [`RFC3209-2.6-2`](#rfc3209-2.6-2) When the datagram size exceeds M and the datagram may be fragmented, the datagram MUST be broken into fragments, each of whose size is no greater than M (S2.6) | no test | no test carries this requirement id |
| [`RFC3209-2.6-3`](#rfc3209-2.6-3) Each fragment of such a datagram MUST be labeled and then forwarded (S2.6) | no test | no test carries this requirement id |
| [`RFC3209-2.6-4`](#rfc3209-2.6-4) When the datagram size exceeds M and the datagram may not be fragmented, the datagram MUST NOT be forwarded (S2.6) | no test | no test carries this requirement id |
| [`RFC3209-2.6-5`](#rfc3209-2.6-5) When the size of an IPv6 datagram without labels exceeds M, the datagram MUST NOT be forwarded (S2.6) | no test | no test carries this requirement id |
| [`RFC3209-3-1`](#rfc3209-3-1) In Resv messages the new objects MUST appear after the associated FILTER_SPEC and prior to any subsequent FILTER_SPEC (S3) | no test | no test carries this requirement id |
| [`RFC3209-3-2`](#rfc3209-3-2) The ordering of these objects is not important, so an implementation MUST be prepared to accept objects in any order (S3) | no test | no test carries this requirement id |
| [`RFC3209-4.1.1.1-1`](#rfc3209-4.1.1.1-1) If a label range has been specified in the label request, the label MUST be drawn from that range (S4.1.1.1) | no test | no test carries this requirement id |
| [`RFC3209-4.1.1.1-2`](#rfc3209-4.1.1.1-2) A node that intends to police individual senders to a session MUST assign unique labels to those senders (S4.1.1.1) | no test | no test carries this requirement id |
| [`RFC3209-4.1.1.1-3`](#rfc3209-4.1.1.1-3) If for any senders the M-bit is not set, the downstream node MUST assign unique labels to those senders (S4.1.1.1) | no test | no test carries this requirement id |
| [`RFC3209-4.2.1-1`](#rfc3209-4.2.1-1) The Reserved field of a Label Request without label range MUST be set to zero on transmission and MUST be ignored on receipt (S4.2.1) | no test | no test carries this requirement id |
| [`RFC3209-4.2.2-1`](#rfc3209-4.2.2-1) The Reserved field of a Label Request with ATM label range MUST be set to zero on transmission and MUST be ignored on receipt (S4.2.2) | no test | no test carries this requirement id |
| [`RFC3209-4.2.2-2`](#rfc3209-4.2.2-2) If the VPI is less than 12 bits it MUST be right justified in its field and preceding bits MUST be set to zero (S4.2.2) | no test | no test carries this requirement id |
| [`RFC3209-4.2.2-3`](#rfc3209-4.2.2-3) If the VCI is less than 16 bits it MUST be right justified in its field and preceding bits MUST be set to zero (S4.2.2) | no test | no test carries this requirement id |
| [`RFC3209-4.2.3-1`](#rfc3209-4.2.3-1) The Reserved field of a Label Request with Frame Relay label range MUST be set to zero on transmission and ignored on receipt (S4.2.3) | no test | no test carries this requirement id |
| [`RFC3209-4.2.3-2`](#rfc3209-4.2.3-2) The DLCI MUST be right justified in its field and unused bits MUST be set to 0 (S4.2.3) | no test | no test carries this requirement id |
| [`RFC3209-4.2.4-1`](#rfc3209-4.2.4-1) A receiver that accepts a LABEL_REQUEST object MUST include a LABEL object in Resv messages pertaining to that Path message (S4.2.4) | no test | no test carries this requirement id |
| [`RFC3209-4.2.4-2`](#rfc3209-4.2.4-2) If a LABEL_REQUEST object was not present in the Path message, a node MUST NOT include a LABEL object in a Resv message for that Path message's session and PHOP (S4.2.4) | no test | no test carries this requirement id |
| [`RFC3209-4.2.4-3`](#rfc3209-4.2.4-3) A node that sends a LABEL_REQUEST object MUST be ready to accept and correctly process a LABEL object in the corresponding Resv messages (S4.2.4) | no test | no test carries this requirement id |
| [`RFC3209-4.2.4-4`](#rfc3209-4.2.4-4) A node which receives and forwards a Path message with a LABEL_REQUEST object MUST copy the L3PID from the received LABEL_REQUEST object to the forwarded LABEL_REQUEST object (S4.2.4) | no test | no test carries this requirement id |
| [`RFC3209-4.2.5-1`](#rfc3209-4.2.5-1) A router with a neighbor known not to be RSVP capable MUST NOT advertise the LABEL_REQUEST object when sending messages that pass through the non-RSVP routers (S4.2.5) | no test | no test carries this requirement id |
| [`RFC3209-4.3.3-1`](#rfc3209-4.3.3-1) An EXPLICIT_ROUTE subobject Length MUST be at least 4, and MUST be a multiple of 4 (S4.3.3) | no test | no test carries this requirement id |
| [`RFC3209-4.3.3.1-1`](#rfc3209-4.3.3.1-1) The path between a strict node and its preceding node MUST include only network nodes from the strict node and its preceding abstract node (S4.3.3.1) | no test | no test carries this requirement id |
| [`RFC3209-4.3.4.2-1`](#rfc3209-4.3.4.2-1) Each subobject added to the EXPLICIT_ROUTE object MUST denote an abstract node that is a subset of the current abstract node (S4.3.4.2) | no test | no test carries this requirement id |
| [`RFC3209-4.4.1-1`](#rfc3209-4.4.1-1) A RECORD_ROUTE subobject Length MUST always be a multiple of 4, and at least 4 (S4.4.1) | no test | no test carries this requirement id |
| [`RFC3209-4.4.3-1`](#rfc3209-4.4.3-1) The subobject a node newly adds to the RRO MUST be that router's IP address (S4.4.3) | no test | no test carries this requirement id |
| [`RFC3209-4.4.3-2`](#rfc3209-4.4.3-2) A node MUST NOT push on a Label Record subobject without also pushing on an IPv4 or IPv6 subobject (S4.4.3) | no test | no test carries this requirement id |
| [`RFC3209-4.4.3-3`](#rfc3209-4.4.3-3) If the newly added subobject causes the RRO to be too big to fit in a Path or Resv message, the RRO object SHALL be dropped from the message and message processing continues as normal (S4.4.3) | no test | no test carries this requirement id |
| [`RFC3209-4.4.3-4`](#rfc3209-4.4.3-4) Subsequent Resv messages SHALL NOT contain an RRO (S4.4.3) | no test | no test carries this requirement id |
| [`RFC3209-4.6.2-2`](#rfc3209-4.6.2-2) The LSP_TUNNEL_IPv6 SENDER_TEMPLATE object's reserved field MUST be zero (S4.6.2) | no test | no test carries this requirement id |
| [`RFC3209-4.7.3-1`](#rfc3209-4.7.3-1) A SESSION_ATTRIBUTE object Length MUST always be a multiple of 4 and MUST be at least 8 (S4.7.3) | no test | no test carries this requirement id |
| [`RFC3209-4.7.4-1`](#rfc3209-4.7.4-1) In order to be validated a link MUST pass the three resource-affinity tests (S4.7.4) | no test | no test carries this requirement id |
| [`RFC3209-4.7.4-2`](#rfc3209-4.7.4-2) When a node is choosing links in order to extend a loose node of an ERO, the node MUST validate the resource classes of those links against the resource affinities (S4.7.4) | no test | no test carries this requirement id |
| [`RFC3209-4.7.4-3`](#rfc3209-4.7.4-3) All RSVP routers, whether they support the SESSION_ATTRIBUTE object or not, SHALL forward the object unmodified (S4.7.4) | no test | no test carries this requirement id |
| [`RFC3209-5.2.2-1`](#rfc3209-5.2.2-1) The Src_Instance value MUST change when the sender is reset, when the node reboots, or when communication is lost to the neighboring node, and otherwise remains the same (S5.2.2) | no test | no test carries this requirement id |
| [`RFC3209-5.2.2-2`](#rfc3209-5.2.2-2) The Src_Instance field MUST NOT be set to zero (S5.2.2) | no test | no test carries this requirement id |
| [`RFC3209-5.2.2-3`](#rfc3209-5.2.2-3) The Dst_Instance field MUST be set to zero when no value has ever been seen from the neighbor (S5.2.2) | no test | no test carries this requirement id |
| [`RFC3209-5.3-1`](#rfc3209-5.3-1) The Src_Instance value MUST NOT change while the agent is exchanging Hellos with the corresponding neighbor (S5.3) | no test | no test carries this requirement id |
| [`RFC3209-5.3-2`](#rfc3209-5.3-2) On receipt of a message containing a HELLO REQUEST object, the receiver MUST generate a Hello message containing a HELLO ACK object (S5.3) | no test | no test carries this requirement id |
| [`RFC3209-5.3-3`](#rfc3209-5.3-3) If the neighbor's Src_Instance value differs from the value previously received, or the Src_Instance field is zero, the node MUST treat the neighbor as if communication has been lost (S5.3) | no test | no test carries this requirement id |
| [`RFC3209-5.3-4`](#rfc3209-5.3-4) If the neighbor continues to advertise a wrong non-zero value after a configured number of intervals, the node MUST treat the neighbor as if communication has been lost (S5.3) | no test | no test carries this requirement id |
| [`RFC3209-5.3-5`](#rfc3209-5.3-5) On receipt of a message containing a HELLO ACK object, the receiver MUST verify that the neighbor has not reset (S5.3) | no test | no test carries this requirement id |
| [`RFC3209-5.3-6`](#rfc3209-5.3-6) The receiver of a HELLO ACK object MUST also verify that the neighbor is reflecting back the receiver's Instance value (S5.3) | no test | no test carries this requirement id |
| [`RFC3209-5.3-7`](#rfc3209-5.3-7) If the neighbor advertises a wrong value in the Dst_Instance field, a node MUST treat the neighbor as if communication has been lost (S5.3) | no test | no test carries this requirement id |
| [`RFC3209-5.3-8`](#rfc3209-5.3-8) If no Instance values are received, via either REQUEST or ACK objects, from a neighbor within a configured number of hello_intervals, a node MUST presume that it cannot communicate with the neighbor (S5.3) | no test | no test carries this requirement id |
| [`RFC3209-5.3-9`](#rfc3209-5.3-9) A node that re-initiates Hellos MUST use a Src_Instance value different than the one advertised in the previous HELLO message (S5.3) | no test | no test carries this requirement id |
| [`RFC3209-5.3-10`](#rfc3209-5.3-10) The new Src_Instance value MUST continue to be advertised to the corresponding neighbor until a reset or reboot occurs, or until another communication failure is detected (S5.3) | no test | no test carries this requirement id |
| [`RFC3209-5.3-11`](#rfc3209-5.3-11) If a new instance value has not been received from the neighbor, the node MUST advertise zero in the Dst_Instance value field (S5.3) | no test | no test carries this requirement id |
| [`RFC3209-5.4-1`](#rfc3209-5.4-1) When the links between neighbors are numbered, Hellos MUST be run on each link (S5.4) | no test | no test carries this requirement id |
| [`RFC3209-5.4-2`](#rfc3209-5.4-2) When the links are unnumbered, link failure detection MUST be provided by some means other than Hellos (S5.4) | no test | no test carries this requirement id |
| [`RFC3209-x-1`](#rfc3209-x-1) PATH messages MUST include the Router Alert IP option (Transport) | {gap}, no test | the raw IPv4 socket (proto 46) is opened and used for Sendto with no IP_ROUTER_ALERT socket option and no per-packet IP option, so emitted PATH datagrams carry no Router Alert (the same transport gap as RFC2205-x-1) (internal/plugins/rsvpte/transport_linux.go:35-58, :60-69) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC3209-4.6.1-1`](#rfc3209-4.6.1-1)

The LSP_TUNNEL_IPv4 SESSION object's reserved field MUST be zero (S4.6.1, Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L338) | unit/verify | unproven |

### [`RFC3209-4.6.2-1`](#rfc3209-4.6.2-1)

The LSP_TUNNEL_IPv4 SENDER_TEMPLATE object's reserved field MUST be zero (S4.6.2, Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPSenderTemplateReservedZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L368) | unit/verify | unproven |

### [`RFC3209-4.6.1-2`](#rfc3209-4.6.1-2)

The LSP_TUNNEL_IPv6 SESSION object's reserved field MUST be zero (S4.6.1, Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPSessionObjectEncoding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L337) | unit/verify | unproven |

### [`RFC3209-4.2-1`](#rfc3209-4.2-1)

LABEL_REQUEST object MUST be present in PATH messages to request label allocation (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPathRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L45) | unit/verify | unproven |

### [`RFC3209-4.1-1`](#rfc3209-4.1-1)

The label for a sender MUST immediately follow the FILTER_SPEC for that sender in the Resv message (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineResvWithoutLabelRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L364) | unit/verify | unproven |
| positive | [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L75) | unit/verify | unproven |

### [`RFC3209-4.1-2`](#rfc3209-4.1-2)

Labels MAY be carried in Resv messages (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L297) | unit/verify | unproven |
| positive | [`TestRSVPLabelObject`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L296) | unit/verify | unproven |

### [`RFC3209-4.3.4-1`](#rfc3209-4.3.4-1)

ERO processing: transit node MUST remove itself (first subobject) from ERO before forwarding PATH (S4.3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEngineTransitNoUsableERONextHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L406) | unit/verify | unproven |
| positive | [`TestEngineTransitForwarding`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_test.go#L195) | unit/verify | unproven |

### [`RFC3209-4.3.4.1-1`](#rfc3209-4.3.4.1-1)

The node receiving an RSVP message with an EXPLICIT_ROUTE object MUST first evaluate the first subobject (S4.3.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.3.4.1-1, so no unit is bound to it.

### [`RFC3209-4.1-3`](#rfc3209-4.1-3)

Reserved label values 0-15: only label 3 (Implicit NULL) MAY be allocated for penultimate hop popping (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLSPTableAllocateSkipsReservedLabels`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/fsm_test.go#L70) | unit/verify | unproven |

### [`RFC3209-2.6-1`](#rfc3209-2.6-1)

An LSR MUST execute the labeled-datagram forwarding algorithm of Section 2.6 (S2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-2.6-1, so no unit is bound to it.

### [`RFC3209-2.6-2`](#rfc3209-2.6-2)

When the datagram size exceeds M and the datagram may be fragmented, the datagram MUST be broken into fragments, each of whose size is no greater than M (S2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-2.6-2, so no unit is bound to it.

### [`RFC3209-2.6-3`](#rfc3209-2.6-3)

Each fragment of such a datagram MUST be labeled and then forwarded (S2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-2.6-3, so no unit is bound to it.

### [`RFC3209-2.6-4`](#rfc3209-2.6-4)

When the datagram size exceeds M and the datagram may not be fragmented, the datagram MUST NOT be forwarded (S2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-2.6-4, so no unit is bound to it.

### [`RFC3209-2.6-5`](#rfc3209-2.6-5)

When the size of an IPv6 datagram without labels exceeds M, the datagram MUST NOT be forwarded (S2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-2.6-5, so no unit is bound to it.

### [`RFC3209-3-1`](#rfc3209-3-1)

In Resv messages the new objects MUST appear after the associated FILTER_SPEC and prior to any subsequent FILTER_SPEC (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-3-1, so no unit is bound to it.

### [`RFC3209-3-2`](#rfc3209-3-2)

The ordering of these objects is not important, so an implementation MUST be prepared to accept objects in any order (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-3-2, so no unit is bound to it.

### [`RFC3209-4.1.1.1-1`](#rfc3209-4.1.1.1-1)

If a label range has been specified in the label request, the label MUST be drawn from that range (S4.1.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.1.1.1-1, so no unit is bound to it.

### [`RFC3209-4.1.1.1-2`](#rfc3209-4.1.1.1-2)

A node that intends to police individual senders to a session MUST assign unique labels to those senders (S4.1.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.1.1.1-2, so no unit is bound to it.

### [`RFC3209-4.1.1.1-3`](#rfc3209-4.1.1.1-3)

If for any senders the M-bit is not set, the downstream node MUST assign unique labels to those senders (S4.1.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.1.1.1-3, so no unit is bound to it.

### [`RFC3209-4.2.1-1`](#rfc3209-4.2.1-1)

The Reserved field of a Label Request without label range MUST be set to zero on transmission and MUST be ignored on receipt (S4.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.1-1, so no unit is bound to it.

### [`RFC3209-4.2.2-1`](#rfc3209-4.2.2-1)

The Reserved field of a Label Request with ATM label range MUST be set to zero on transmission and MUST be ignored on receipt (S4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.2-1, so no unit is bound to it.

### [`RFC3209-4.2.2-2`](#rfc3209-4.2.2-2)

If the VPI is less than 12 bits it MUST be right justified in its field and preceding bits MUST be set to zero (S4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.2-2, so no unit is bound to it.

### [`RFC3209-4.2.2-3`](#rfc3209-4.2.2-3)

If the VCI is less than 16 bits it MUST be right justified in its field and preceding bits MUST be set to zero (S4.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.2-3, so no unit is bound to it.

### [`RFC3209-4.2.3-1`](#rfc3209-4.2.3-1)

The Reserved field of a Label Request with Frame Relay label range MUST be set to zero on transmission and ignored on receipt (S4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.3-1, so no unit is bound to it.

### [`RFC3209-4.2.3-2`](#rfc3209-4.2.3-2)

The DLCI MUST be right justified in its field and unused bits MUST be set to 0 (S4.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.3-2, so no unit is bound to it.

### [`RFC3209-4.2.4-1`](#rfc3209-4.2.4-1)

A receiver that accepts a LABEL_REQUEST object MUST include a LABEL object in Resv messages pertaining to that Path message (S4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.4-1, so no unit is bound to it.

### [`RFC3209-4.2.4-2`](#rfc3209-4.2.4-2)

If a LABEL_REQUEST object was not present in the Path message, a node MUST NOT include a LABEL object in a Resv message for that Path message's session and PHOP (S4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.4-2, so no unit is bound to it.

### [`RFC3209-4.2.4-3`](#rfc3209-4.2.4-3)

A node that sends a LABEL_REQUEST object MUST be ready to accept and correctly process a LABEL object in the corresponding Resv messages (S4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.4-3, so no unit is bound to it.

### [`RFC3209-4.2.4-4`](#rfc3209-4.2.4-4)

A node which receives and forwards a Path message with a LABEL_REQUEST object MUST copy the L3PID from the received LABEL_REQUEST object to the forwarded LABEL_REQUEST object (S4.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.4-4, so no unit is bound to it.

### [`RFC3209-4.2.5-1`](#rfc3209-4.2.5-1)

A router with a neighbor known not to be RSVP capable MUST NOT advertise the LABEL_REQUEST object when sending messages that pass through the non-RSVP routers (S4.2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.2.5-1, so no unit is bound to it.

### [`RFC3209-4.3.3-1`](#rfc3209-4.3.3-1)

An EXPLICIT_ROUTE subobject Length MUST be at least 4, and MUST be a multiple of 4 (S4.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.3.3-1, so no unit is bound to it.

### [`RFC3209-4.3.3.1-1`](#rfc3209-4.3.3.1-1)

The path between a strict node and its preceding node MUST include only network nodes from the strict node and its preceding abstract node (S4.3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.3.3.1-1, so no unit is bound to it.

### [`RFC3209-4.3.4.2-1`](#rfc3209-4.3.4.2-1)

Each subobject added to the EXPLICIT_ROUTE object MUST denote an abstract node that is a subset of the current abstract node (S4.3.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.3.4.2-1, so no unit is bound to it.

### [`RFC3209-4.4.1-1`](#rfc3209-4.4.1-1)

A RECORD_ROUTE subobject Length MUST always be a multiple of 4, and at least 4 (S4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.4.1-1, so no unit is bound to it.

### [`RFC3209-4.4.3-1`](#rfc3209-4.4.3-1)

The subobject a node newly adds to the RRO MUST be that router's IP address (S4.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.4.3-1, so no unit is bound to it.

### [`RFC3209-4.4.3-2`](#rfc3209-4.4.3-2)

A node MUST NOT push on a Label Record subobject without also pushing on an IPv4 or IPv6 subobject (S4.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.4.3-2, so no unit is bound to it.

### [`RFC3209-4.4.3-3`](#rfc3209-4.4.3-3)

If the newly added subobject causes the RRO to be too big to fit in a Path or Resv message, the RRO object SHALL be dropped from the message and message processing continues as normal (S4.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.4.3-3, so no unit is bound to it.

### [`RFC3209-4.4.3-4`](#rfc3209-4.4.3-4)

Subsequent Resv messages SHALL NOT contain an RRO (S4.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.4.3-4, so no unit is bound to it.

### [`RFC3209-4.6.2-2`](#rfc3209-4.6.2-2)

The LSP_TUNNEL_IPv6 SENDER_TEMPLATE object's reserved field MUST be zero (S4.6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.6.2-2, so no unit is bound to it.

### [`RFC3209-4.7.3-1`](#rfc3209-4.7.3-1)

A SESSION_ATTRIBUTE object Length MUST always be a multiple of 4 and MUST be at least 8 (S4.7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.7.3-1, so no unit is bound to it.

### [`RFC3209-4.7.4-1`](#rfc3209-4.7.4-1)

In order to be validated a link MUST pass the three resource-affinity tests (S4.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.7.4-1, so no unit is bound to it.

### [`RFC3209-4.7.4-2`](#rfc3209-4.7.4-2)

When a node is choosing links in order to extend a loose node of an ERO, the node MUST validate the resource classes of those links against the resource affinities (S4.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.7.4-2, so no unit is bound to it.

### [`RFC3209-4.7.4-3`](#rfc3209-4.7.4-3)

All RSVP routers, whether they support the SESSION_ATTRIBUTE object or not, SHALL forward the object unmodified (S4.7.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-4.7.4-3, so no unit is bound to it.

### [`RFC3209-5.2.2-1`](#rfc3209-5.2.2-1)

The Src_Instance value MUST change when the sender is reset, when the node reboots, or when communication is lost to the neighboring node, and otherwise remains the same (S5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.2.2-1, so no unit is bound to it.

### [`RFC3209-5.2.2-2`](#rfc3209-5.2.2-2)

The Src_Instance field MUST NOT be set to zero (S5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.2.2-2, so no unit is bound to it.

### [`RFC3209-5.2.2-3`](#rfc3209-5.2.2-3)

The Dst_Instance field MUST be set to zero when no value has ever been seen from the neighbor (S5.2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.2.2-3, so no unit is bound to it.

### [`RFC3209-5.3-1`](#rfc3209-5.3-1)

The Src_Instance value MUST NOT change while the agent is exchanging Hellos with the corresponding neighbor (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-1, so no unit is bound to it.

### [`RFC3209-5.3-2`](#rfc3209-5.3-2)

On receipt of a message containing a HELLO REQUEST object, the receiver MUST generate a Hello message containing a HELLO ACK object (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-2, so no unit is bound to it.

### [`RFC3209-5.3-3`](#rfc3209-5.3-3)

If the neighbor's Src_Instance value differs from the value previously received, or the Src_Instance field is zero, the node MUST treat the neighbor as if communication has been lost (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-3, so no unit is bound to it.

### [`RFC3209-5.3-4`](#rfc3209-5.3-4)

If the neighbor continues to advertise a wrong non-zero value after a configured number of intervals, the node MUST treat the neighbor as if communication has been lost (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-4, so no unit is bound to it.

### [`RFC3209-5.3-5`](#rfc3209-5.3-5)

On receipt of a message containing a HELLO ACK object, the receiver MUST verify that the neighbor has not reset (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-5, so no unit is bound to it.

### [`RFC3209-5.3-6`](#rfc3209-5.3-6)

The receiver of a HELLO ACK object MUST also verify that the neighbor is reflecting back the receiver's Instance value (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-6, so no unit is bound to it.

### [`RFC3209-5.3-7`](#rfc3209-5.3-7)

If the neighbor advertises a wrong value in the Dst_Instance field, a node MUST treat the neighbor as if communication has been lost (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-7, so no unit is bound to it.

### [`RFC3209-5.3-8`](#rfc3209-5.3-8)

If no Instance values are received, via either REQUEST or ACK objects, from a neighbor within a configured number of hello_intervals, a node MUST presume that it cannot communicate with the neighbor (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-8, so no unit is bound to it.

### [`RFC3209-5.3-9`](#rfc3209-5.3-9)

A node that re-initiates Hellos MUST use a Src_Instance value different than the one advertised in the previous HELLO message (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-9, so no unit is bound to it.

### [`RFC3209-5.3-10`](#rfc3209-5.3-10)

The new Src_Instance value MUST continue to be advertised to the corresponding neighbor until a reset or reboot occurs, or until another communication failure is detected (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-10, so no unit is bound to it.

### [`RFC3209-5.3-11`](#rfc3209-5.3-11)

If a new instance value has not been received from the neighbor, the node MUST advertise zero in the Dst_Instance value field (S5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.3-11, so no unit is bound to it.

### [`RFC3209-5.4-1`](#rfc3209-5.4-1)

When the links between neighbors are numbered, Hellos MUST be run on each link (S5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.4-1, so no unit is bound to it.

### [`RFC3209-5.4-2`](#rfc3209-5.4-2)

When the links are unnumbered, link failure detection MUST be provided by some means other than Hellos (S5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-5.4-2, so no unit is bound to it.

### [`RFC3209-6-1`](#rfc3209-6-1)

Make-before-break: old and new LSPs MUST have the same SESSION (same Tunnel Endpoint, Tunnel ID, Extended Tunnel ID); only LSP ID differs (S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSEAdmissionDistinctSessionsDoNotShare`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/admission_se_test.go#L35) | unit/verify | unproven |
| positive | [`TestEngineMakeBeforeBreak`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/reroute_test.go#L41) | unit/verify | unproven |

### [`RFC3209-6-2`](#rfc3209-6-2)

SE (Shared Explicit) style MUST be used for make-before-break rerouting (S6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildResvRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/build_test.go#L78) | unit/verify | unproven |

### [`RFC3209-x-1`](#rfc3209-x-1)

PATH messages MUST include the Router Alert IP option (Transport)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC3209-x-1, so no unit is bound to it.

### [`RFC3209-2.5-1`](#rfc3209-2.5-1)

Both PATH and RESV messages MUST be refreshed periodically for soft-state maintenance (S2.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRefreshDoesNotStampEgressPSB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/softstate_test.go#L21) | unit/verify | unproven |
| positive | [`TestRefreshResendsPathAndResv`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/softstate_test.go#L56) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc3209.txt |
| Source fingerprint | 976eb5437f567b6f |
| Record | rfc/extraction/rfc3209.json |
| Mapped sentences | 55 |
| Declined as scope | 8 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `2.3` | not stated | 0 | walked | not stated |
| `2.4` | not stated | 0 | walked | not stated |
| `2.4.1` | not stated | 0 | walked | not stated |
| `2.4.2` | not stated | 0 | walked | not stated |
| `2.4.3` | not stated | 0 | walked | not stated |
| `2.5` | not stated | 0 | walked | not stated |
| `2.6` | not stated | 5 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.1.1` | not stated | 0 | walked | not stated |
| `4.1.1.1` | not stated | 3 | walked | not stated |
| `4.1.1.2` | not stated | 0 | walked | not stated |
| `4.1.2` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.2.1` | not stated | 1 | walked | not stated |
| `4.2.2` | not stated | 5 | walked | not stated |
| `4.2.3` | not stated | 3 | walked | not stated |
| `4.2.4` | not stated | 5 | walked | not stated |
| `4.2.5` | not stated | 1 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.3.1` | not stated | 0 | walked | not stated |
| `4.3.2` | not stated | 0 | walked | not stated |
| `4.3.3` | not stated | 1 | walked | not stated |
| `4.3.3.1` | not stated | 1 | walked | not stated |
| `4.3.3.2` | not stated | 0 | walked | not stated |
| `4.3.3.3` | not stated | 0 | walked | not stated |
| `4.3.3.4` | not stated | 0 | walked | not stated |
| `4.3.4` | not stated | 0 | walked | not stated |
| `4.3.4.1` | not stated | 1 | walked | not stated |
| `4.3.4.2` | not stated | 1 | walked | not stated |
| `4.3.5` | not stated | 0 | walked | not stated |
| `4.3.6` | not stated | 0 | walked | not stated |
| `4.3.7` | not stated | 0 | walked | not stated |
| `4.4` | not stated | 0 | walked | not stated |
| `4.4.1` | not stated | 1 | walked | not stated |
| `4.4.1.1` | not stated | 0 | walked | not stated |
| `4.4.1.2` | not stated | 0 | walked | not stated |
| `4.4.1.3` | not stated | 0 | walked | not stated |
| `4.4.2` | not stated | 0 | walked | not stated |
| `4.4.3` | not stated | 4 | walked | not stated |
| `4.4.4` | not stated | 0 | walked | not stated |
| `4.4.5` | not stated | 0 | walked | not stated |
| `4.4.6` | not stated | 0 | walked | not stated |
| `4.5` | not stated | 0 | walked | not stated |
| `4.6` | not stated | 0 | walked | not stated |
| `4.6.1` | not stated | 0 | walked | not stated |
| `4.6.1.1` | not stated | 1 | walked | not stated |
| `4.6.1.2` | not stated | 1 | walked | not stated |
| `4.6.2` | not stated | 0 | walked | not stated |
| `4.6.2.1` | not stated | 1 | walked | not stated |
| `4.6.2.2` | not stated | 1 | walked | not stated |
| `4.6.3` | not stated | 0 | walked | not stated |
| `4.6.3.1` | not stated | 0 | walked | not stated |
| `4.6.3.2` | not stated | 0 | walked | not stated |
| `4.6.4` | not stated | 0 | walked | not stated |
| `4.7` | not stated | 0 | walked | not stated |
| `4.7.1` | not stated | 0 | walked | not stated |
| `4.7.2` | not stated | 0 | walked | not stated |
| `4.7.3` | not stated | 1 | walked | not stated |
| `4.7.4` | not stated | 5 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.2.1` | not stated | 0 | walked | not stated |
| `5.2.2` | not stated | 3 | walked | not stated |
| `5.3` | not stated | 13 | walked | not stated |
| `5.4` | not stated | 2 | walked | not stated |
| `5.5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `12` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `4.2.2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Maximum VPI field repeats the right-justification rule the Minimum VPI field states; site 4.2.2:2 maps it | If the VPI is less than 12-bits it MUST be right justified in this field and preceding bits MUST be set to zero. |
| `4.2.2:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Maximum VCI field repeats the right-justification rule the Minimum VCI field states; site 4.2.2:3 maps it | If the VCI is less than 16-bits it MUST be right justified in this field and preceding bits MUST be set to zero. |
| `4.2.3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the Maximum DLCI field repeats the right-justification rule the Minimum DLCI field states; site 4.2.3:2 maps it | The DLCI MUST be right justified in this field and unused bits MUST be set to 0. |
| `4.2.4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | section 4.2.4 restates the section 4.1.1.1 rule that a label is allocated from a specified label range; site 4.1.1.1:1 maps it | If a label range was specified, the label MUST be allocated from that range. |
| `4.7.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the second statement of the same three-test validation rule, for the second of the two resource-affinity cases; site 4.7.4:1 maps it | In order to be validated a link MUST pass the following three tests. |
| `4.7.4:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | 'For a link to be acceptable, all three tests MUST pass' restates the validation rule the preceding sentence states; site 4.7.4:1 maps it | For a link to be acceptable, all three tests MUST pass. |
| `5.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a scoping statement about how the RFC 2119 keywords in section 5.3 are to be read ('the use of MUST and SHOULD with respect to the receiver applies only to a node that supports Hello message processing'), not an obligation of its own | In particular, the use of MUST and SHOULD with respect to the receiver applies only to a node that supports Hello message processing. |
| `5.3:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the HELLO ACK path repeats the HELLO REQUEST sentence word for word; site 5.3:4 maps it | If the value differs or the Src_Instance field is zero, then the node MUST treat the neighbor as if communication has been lost. |

## Superseded

No document obsoletes RFC 3209, so its obligations are stated where they were written.
