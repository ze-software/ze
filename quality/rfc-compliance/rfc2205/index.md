# RFC 2205 - Resource ReSerVation Protocol (RSVP) -- Version 1 Functional Specification

Experimental. Every requirement this repository extracted from RFC 2205, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4.3% | 3 of 70 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 2.9% | 2 of 70 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 70 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 36.4% | 4 of 11 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 70 | of 75 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 70 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 70 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 70 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 70 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 92.9% | 65 of 70 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 70 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
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
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 75 |
| Gated MUST-level | 70 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 2 |
| Gated with no test | 63 |
| Nightly-only evidence | 0 |
| Test tags | 15 |
| Tagged units | 11 |
| Recorded audit verdicts | 0 |
| Discrimination records | 4 |
| Summary | `rfc/short/rfc2205.md` |
| Requirement shard | `rfc/requirements/rfc2205.md` |
| RFC text | `rfc/full/rfc2205.txt` |

## Enrolment

Enrolled: RSVP version 1 base protocol (codec shared by ze's RSVP-TE plugin). The 2026-09-21 extraction walk read all 123 normative sites and the checklist now holds 75 rows: 70 MUST or MUST NOT, 2 SHOULD, 1 SHOULD NOT and 2 MAY. Seven MUST rows were enrolled first and are described below; the walk added 63 more, none of them bound to a test, covering the reservation model of Sections 1 and 2 (merging, teardown, blockade state, admission and policy control, non-RSVP clouds), the message rules of Section 3 (addressing, object order, port consistency, state matching, teardown routing, error reporting, refresh timing, loop avoidance, and the routing, interface and packet-diversion services RSVP requires of the node), and the object field and UDP encapsulation rules of the appendices. Of the seven: 3.1-1 (Version MUST be 1) is met with positive+negative tags (encode round-trip and bad-version decode reject, internal/plugins/rsvpte/wire.go). 3.1-2 (reserved octet 0 on send) and 3.1.2-1 (object length multiple of 4) are {single-polarity: positive} with send-side tests (wire.go and the object encoders). 3.1.3-1 (a received message carries every object its BNF writes unbracketed) is met with positive+negative tags: checkMandatoryObjects (internal/plugins/rsvpte/mandatory.go) refuses a Path, Resv, PathTear or PathErr that omits one, and handlePacket drops it with a log line and no ERROR_SPEC. 3.10-1 (reject an unknown Class-Num of the form 0bbbbbbb) is met with positive+negative tags: DecodeMessage classifies by the high-order bit (classifyUnknownClass, wire.go) and engine.rejectUnknownObject answers a PATH with Error Code 13. 3.1-3 (verify checksum on receipt) and x-1 (IP Router Alert in PATH) are {gap}: ze's receive path (wire.go DecodeHeader) and raw-socket send (transport_linux.go) omit these. Disclosed in the docs/features/rfc-status.md RFC 2205 row.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- RSVP base common-header and object codec used by RSVP-TE: Version-1 header enforced on decode, reserved octet zeroed on send, every emitted object length a multiple of 4, and a received Path, Resv, PathTear or PathErr dropped when it omits an object its Section 3.1 BNF writes unbracketed
- tests bound per requirement in [`rfc/requirements/rfc2205.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc2205.md).


**What the ledger says remains**

Two MUST gaps gated in [`rfc/short/rfc2205.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2205.md): the receive path does not verify the RSVP checksum or drop bad-checksum messages (3.1); and PATH is sent without the IP Router Alert option. The 63 rows the 2026-09-21 extraction walk added carry no bound test. Most of them describe the IntServ reservation model ze's RSVP-TE plugin does not implement: multicast merging, wildcard and shared reservation styles, blockade state, policy control, and the UDP encapsulation for hosts without raw network I/O.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated instead of tested | 4 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 63 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **70** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC2205-3.1-1`](#rfc2205-3.1-1), [`RFC2205-3.1.3-1`](#rfc2205-3.1.3-1), [`RFC2205-3.10-1`](#rfc2205-3.10-1)

**Annotated instead of tested (4):** [`RFC2205-3.1-2`](#rfc2205-3.1-2), [`RFC2205-3.1.2-1`](#rfc2205-3.1.2-1), [`RFC2205-3.1-3`](#rfc2205-3.1-3), [`RFC2205-x-1`](#rfc2205-x-1)

**No test and no annotation (63):** [`RFC2205-1-1`](#rfc2205-1-1), [`RFC2205-1-2`](#rfc2205-1-2), [`RFC2205-1-3`](#rfc2205-1-3), [`RFC2205-1-4`](#rfc2205-1-4), [`RFC2205-2-1`](#rfc2205-2-1), [`RFC2205-2-2`](#rfc2205-2-2), [`RFC2205-2-3`](#rfc2205-2-3), [`RFC2205-2-4`](#rfc2205-2-4), [`RFC2205-2-5`](#rfc2205-2-5), [`RFC2205-2-6`](#rfc2205-2-6), [`RFC2205-2-7`](#rfc2205-2-7), [`RFC2205-2-8`](#rfc2205-2-8), [`RFC2205-2-9`](#rfc2205-2-9), [`RFC2205-2-10`](#rfc2205-2-10), [`RFC2205-2-11`](#rfc2205-2-11), [`RFC2205-2-12`](#rfc2205-2-12), [`RFC2205-3-1`](#rfc2205-3-1), [`RFC2205-3-2`](#rfc2205-3-2), [`RFC2205-3-3`](#rfc2205-3-3), [`RFC2205-3-4`](#rfc2205-3-4), [`RFC2205-3-5`](#rfc2205-3-5), [`RFC2205-3-6`](#rfc2205-3-6), [`RFC2205-3-7`](#rfc2205-3-7), [`RFC2205-3-8`](#rfc2205-3-8), [`RFC2205-3-9`](#rfc2205-3-9), [`RFC2205-3-10`](#rfc2205-3-10), [`RFC2205-3-11`](#rfc2205-3-11), [`RFC2205-3-12`](#rfc2205-3-12), [`RFC2205-3-13`](#rfc2205-3-13), [`RFC2205-3-14`](#rfc2205-3-14), [`RFC2205-3-16`](#rfc2205-3-16), [`RFC2205-3-17`](#rfc2205-3-17), [`RFC2205-3-18`](#rfc2205-3-18), [`RFC2205-3-19`](#rfc2205-3-19), [`RFC2205-3-20`](#rfc2205-3-20), [`RFC2205-3-21`](#rfc2205-3-21), [`RFC2205-3-22`](#rfc2205-3-22), [`RFC2205-3-23`](#rfc2205-3-23), [`RFC2205-3-24`](#rfc2205-3-24), [`RFC2205-3-25`](#rfc2205-3-25), [`RFC2205-3-26`](#rfc2205-3-26), [`RFC2205-3-27`](#rfc2205-3-27), [`RFC2205-3-28`](#rfc2205-3-28), [`RFC2205-3-29`](#rfc2205-3-29), [`RFC2205-3-30`](#rfc2205-3-30), [`RFC2205-3-31`](#rfc2205-3-31), [`RFC2205-3-32`](#rfc2205-3-32), [`RFC2205-3-33`](#rfc2205-3-33), [`RFC2205-3-34`](#rfc2205-3-34), [`RFC2205-3-35`](#rfc2205-3-35), [`RFC2205-3-36`](#rfc2205-3-36), [`RFC2205-3-37`](#rfc2205-3-37), [`RFC2205-3-38`](#rfc2205-3-38), [`RFC2205-3-39`](#rfc2205-3-39), [`RFC2205-3-40`](#rfc2205-3-40), [`RFC2205-3-41`](#rfc2205-3-41), [`RFC2205-3-42`](#rfc2205-3-42), [`RFC2205-3-43`](#rfc2205-3-43), [`RFC2205-4-1`](#rfc2205-4-1), [`RFC2205-4-2`](#rfc2205-4-2), [`RFC2205-4-3`](#rfc2205-4-3), [`RFC2205-4-4`](#rfc2205-4-4), [`RFC2205-4-5`](#rfc2205-4-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2205-3.1-1` | Version field MUST be 1 (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRSVPHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L405). **negative:** `unit/verify` [`TestRSVPDecodeHeaderBadVersion`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L439) |
| `RFC2205-3.1-2` | Reserved field in common header MUST be zero (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRSVPReservedByteZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L581). **negative:** no negative test. **{single-polarity}:** ze sets the reserved byte to 0 on send (internal/plugins/rsvpte/wire.go:177) and the RFC does not require receivers to reject a nonzero reserved field, so no negative case exists |
| `RFC2205-3.1.2-1` | Object lengths MUST be a multiple of 4 (§3.1.2) | MUST | 3.1.2 | **positive:** `unit/verify` [`TestRSVPObjectLengthMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L602). **negative:** no negative test. **{single-polarity}:** every RSVP object encoder in internal/plugins/rsvpte/wire.go emits a length that is a multiple of 4; the receive path does not enforce %4, so the reject/negative polarity has no code path |
| `RFC2205-3.1.3-1` | A received message MUST carry every object its Section 3.1 BNF writes unbracketed -- SESSION, RSVP_HOP and TIME_VALUES in a Path, those three plus STYLE in a Resv, SESSION and RSVP_HOP in a PathTear, SESSION and ERROR_SPEC in a PathErr: each node is required to verify the correct construction of each message it receives, and a malformed message is logged locally rather than reported in an ERROR_SPEC (Appendix B) (§3.1.3) | MUST | 3.1.3 | **positive:** `unit/verify` [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L83). **positive:** `unit/verify` [`TestEnginePathWithTimeValuesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L131). **negative:** `unit/verify` [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L91). **negative:** `unit/verify` [`TestEnginePathWithoutTimeValuesDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L112) |
| `RFC2205-3.1-3` | Checksum MUST be verified on receipt; drop messages with bad checksum (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze computes the RFC 2205 checksum on send (internal/plugins/rsvpte/build.go:48 internetChecksum) but the receive path (internal/plugins/rsvpte/wire.go:190 DecodeHeader / DecodeMessage) does not verify it or drop bad-checksum messages |
| `RFC2205-x-1` | IP Router Alert option MUST be set in PATH messages (Transport) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze sends PATH over a raw protocol-46 socket (internal/plugins/rsvpte/transport_linux.go:35-69) and never sets the IP Router Alert option |
| `RFC2205-3.10-1` | Unknown Class-Num of the form 0bbbbbbb: reject the entire message and return an "Unknown Object Class" error (§3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L718). **negative:** `unit/verify` [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L725). **negative:** `unit/verify` [`TestEnginePathWithIgnorableObjectAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L935) |
| `RFC2205-3.7-1` | Refresh period SHOULD be jittered by +/- 50% of R to prevent synchronization (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-x-2` | Jitter: SHOULD randomize refresh timing (Soft-State Model) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3.10-2` | Unknown Class-Num of the form 10bbbbbb: ignore the object, neither forwarding it nor sending an error message (§3.10) | MAY | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3.10-3` | Unknown Class-Num of the form 11bbbbbb: ignore the object but forward it unexamined and unmodified in every message resulting from this one (§3.10) | MAY | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-1-1` | Each router must be able to examine the UDP/TCP port fields used for packet classification (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-1-2` | Where the link-layer technology implements its own QoS management capability, RSVP must negotiate with the link layer to obtain the requested QoS (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-1-3` | Reservations from different downstream branches of the multicast tree(s) from the same sender must be merged as reservations travel upstream (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-1-4` | In an explicit sender-selection reservation, each filter spec must match exactly one sender (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-1` | Resv messages must follow exactly the reverse of the path(s) the data packets will use, upstream to all the sender hosts included in the sender selection (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-2` | Resv messages must finally be delivered to the sender hosts themselves (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-3` | A Path message is required to carry a Sender Template (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-4` | A Path message is required to carry a Sender Tspec (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-5` | Where a state update modifies state to be forwarded in refresh messages, those refresh messages must be generated and forwarded immediately (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-6` | State forwarded out interface I* must be computed using only state that arrived on interfaces different from I* (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-7` | Once initiated, a teardown request must be forwarded hop-by-hop without delay (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-8` | A reservation error must be reported to all of the responsible receivers (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-9` | Blockade state must not deny service to a smaller reservation that would succeed, and must not remove the state or prevent its immediate refresh (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-10` | Each node must answer the admission control and the policy control questions, and both must be favorable for RSVP to make a reservation (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-11` | RSVP must provide correct protocol operation even when two RSVP-capable routers are joined by an arbitrary cloud of non-RSVP routers (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-2-12` | Where the destination address matches no local interface and the message is not a Path or PathTear, the message must be forwarded without further processing by this node (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-1` | An RSVP implementation must recognize the object classes Section 3 lists (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-2` | The IP source address of a Path message must be an address of the sender it describes, and its destination address must be the DestAddress for the session (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-3` | Where the INTEGRITY object is present it must immediately follow the common header (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-4` | Path messages and filter spec matching must satisfy the SrcPort and DstPort consistency rules: the DstPort values for one DestAddress and ProtocolId are all zero or all non-zero, a zero DstPort forces zero SrcPort, and a sender host must not send path state both with and without a zero SrcPort (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-5` | RSVP must be prepared to maintain path state for Path messages from the same sender arriving from more than one PHOP (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-6` | RSVP must not forward Path messages that arrive on an incoming interface different from that provided by routing (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-7` | The STYLE object followed by the flow descriptor list must occur at the end of the message, and objects within the flow descriptor list must follow the BNF (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-8` | The first FF flow descriptor must contain a FLOWSPEC (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-9` | Where a Resv message with wildcard sender selection is forwarded to more than one previous hop, a SCOPE object must be included in the message (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-10` | Matching path state must match the SESSION, SENDER_TEMPLATE and PHOP objects (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-11` | A unicast PathTear must not be forwarded if there is path state for the same (session, sender) pair but a different PHOP (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-12` | A PathTear message must be routed exactly like the corresponding Path message, with the session DestAddress as its IP destination address and the sender address as its IP source address (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-13` | A SENDER_TSPEC or ADSPEC object in a PathTear, and a SCOPE object in a ResvTear, must be ignored (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-14` | Deletion of path state by PathTear or timeout must also adjust related reservation state to maintain consistency in the local node (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-15` | The reservation changes a PathTear causes should not trigger an immediate Resv refresh message (§3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-16` | Matching reservation state must match the SESSION, STYLE and FILTER_SPEC objects and the LIH in the RSVP_HOP object (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-17` | A ResvTear message must be routed like the corresponding Resv message (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-18` | Each flow descriptor in an FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-19` | A ResvErr message must contain the information required to define the error and to route the error message in later hops (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-20` | Where admission control fails while increasing an existing reservation, the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-21` | An immediate state forward must not trigger a message out the interface through which the triggering message arrived (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-22` | Each RSVP message must occupy exactly one IP datagram (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-23` | Forwarding of RSVP messages must avoid looping (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-24` | Where reservation state from a NHOP carries no SCOPE object, a substitute sender list must be created and included in the union (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-25` | A ResvErr message forwarded out an outgoing interface must carry a SCOPE object holding only those senders that route to that interface (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-26` | RSVP must avoid refresh message synchronization and ensure that any synchronization that occurs is not stable (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-27` | The state lifetime L must satisfy L >= (K + 0.5)*1.5*R (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-28` | The ratio of two successive refresh periods R2/R1 must not exceed 1 + Slew.Max (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-29` | RSVP must indicate the merge points it knows to the traffic control mechanism (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-30` | RSVP must test for the presence of non-RSVP hops in the path and pass this information to traffic control (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-31` | Where the routing protocol uses IP encapsulating tunnels, it must inform RSVP when non-RSVP hops are included (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-32` | The RSVP process must be aware of the default sending interface, and an application that sets a specific interface must pass that information to RSVP (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-33` | The RSVP process must determine which incoming interface to use for sending Resv messages by examining the path state (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-34` | Path state that can match only a local application must be marked Local_Only by the RSVP process, and Local_Only path state must be ignored when Path and PathTear messages are forwarded (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-35` | A forwarded message carrying objects of unknown class must obey the general object order requirements for its message type (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-36` | RSVP must merge reservation requests from the corresponding next hops by computing the maximum of their flowspecs (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-37` | An RSVP process must be able to query the routing process(es) for routes in order to forward Path and PathTear messages (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-38` | RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-39` | Packets received for IP protocol 46 but not addressed to the node must be diverted to the RSVP program for processing, without being forwarded (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-40` | The identity of the interface on which a diverted message arrived, and the IP source address and IP TTL it arrived with, must be available to the RSVP process (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-41` | RSVP must be able to force a datagram to be sent on a specific outgoing real or virtual link, bypassing the normal routing mechanism (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-42` | RSVP must be able to specify the IP source address and IP TTL to be used when sending Path messages (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-43` | The service-dependent routines that manipulate the flowspec, Tspec and Adspec objects must be available to the RSVP process (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-4-1` | The SESSION object's DestAddress field must be non-zero (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-4-2` | The SESSION object's Protocol Id field must be non-zero (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-4-3` | The addresses in a SCOPE object must be listed in ascending numerical order (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-4-4` | Each node is required to verify the correct construction of each RSVP message it receives (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-4-5` | A host that cannot do raw network I/O must encapsulate RSVP messages in UDP, using a scheme that allows RSVP interoperation among an arbitrary topology of hosts and routers (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2205-3.1-3`](#rfc2205-3.1-3) Checksum MUST be verified on receipt; drop messages with bad checksum (§3.1) | {gap}, no test | ze computes the RFC 2205 checksum on send (internal/plugins/rsvpte/build.go:48 internetChecksum) but the receive path (internal/plugins/rsvpte/wire.go:190 DecodeHeader / DecodeMessage) does not verify it or drop bad-checksum messages |
| [`RFC2205-x-1`](#rfc2205-x-1) IP Router Alert option MUST be set in PATH messages (Transport) | {gap}, no test | ze sends PATH over a raw protocol-46 socket (internal/plugins/rsvpte/transport_linux.go:35-69) and never sets the IP Router Alert option |
| [`RFC2205-1-1`](#rfc2205-1-1) Each router must be able to examine the UDP/TCP port fields used for packet classification (§1) | no test | no test carries this requirement id |
| [`RFC2205-1-2`](#rfc2205-1-2) Where the link-layer technology implements its own QoS management capability, RSVP must negotiate with the link layer to obtain the requested QoS (§1) | no test | no test carries this requirement id |
| [`RFC2205-1-3`](#rfc2205-1-3) Reservations from different downstream branches of the multicast tree(s) from the same sender must be merged as reservations travel upstream (§1) | no test | no test carries this requirement id |
| [`RFC2205-1-4`](#rfc2205-1-4) In an explicit sender-selection reservation, each filter spec must match exactly one sender (§1) | no test | no test carries this requirement id |
| [`RFC2205-2-1`](#rfc2205-2-1) Resv messages must follow exactly the reverse of the path(s) the data packets will use, upstream to all the sender hosts included in the sender selection (§2) | no test | no test carries this requirement id |
| [`RFC2205-2-2`](#rfc2205-2-2) Resv messages must finally be delivered to the sender hosts themselves (§2) | no test | no test carries this requirement id |
| [`RFC2205-2-3`](#rfc2205-2-3) A Path message is required to carry a Sender Template (§2) | no test | no test carries this requirement id |
| [`RFC2205-2-4`](#rfc2205-2-4) A Path message is required to carry a Sender Tspec (§2) | no test | no test carries this requirement id |
| [`RFC2205-2-5`](#rfc2205-2-5) Where a state update modifies state to be forwarded in refresh messages, those refresh messages must be generated and forwarded immediately (§2) | no test | no test carries this requirement id |
| [`RFC2205-2-6`](#rfc2205-2-6) State forwarded out interface I* must be computed using only state that arrived on interfaces different from I* (§2) | no test | no test carries this requirement id |
| [`RFC2205-2-7`](#rfc2205-2-7) Once initiated, a teardown request must be forwarded hop-by-hop without delay (§2) | no test | no test carries this requirement id |
| [`RFC2205-2-8`](#rfc2205-2-8) A reservation error must be reported to all of the responsible receivers (§2) | no test | no test carries this requirement id |
| [`RFC2205-2-9`](#rfc2205-2-9) Blockade state must not deny service to a smaller reservation that would succeed, and must not remove the state or prevent its immediate refresh (§2) | no test | no test carries this requirement id |
| [`RFC2205-2-10`](#rfc2205-2-10) Each node must answer the admission control and the policy control questions, and both must be favorable for RSVP to make a reservation (§2) | no test | no test carries this requirement id |
| [`RFC2205-2-11`](#rfc2205-2-11) RSVP must provide correct protocol operation even when two RSVP-capable routers are joined by an arbitrary cloud of non-RSVP routers (§2) | no test | no test carries this requirement id |
| [`RFC2205-2-12`](#rfc2205-2-12) Where the destination address matches no local interface and the message is not a Path or PathTear, the message must be forwarded without further processing by this node (§2) | no test | no test carries this requirement id |
| [`RFC2205-3-1`](#rfc2205-3-1) An RSVP implementation must recognize the object classes Section 3 lists (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-2`](#rfc2205-3-2) The IP source address of a Path message must be an address of the sender it describes, and its destination address must be the DestAddress for the session (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-3`](#rfc2205-3-3) Where the INTEGRITY object is present it must immediately follow the common header (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-4`](#rfc2205-3-4) Path messages and filter spec matching must satisfy the SrcPort and DstPort consistency rules: the DstPort values for one DestAddress and ProtocolId are all zero or all non-zero, a zero DstPort forces zero SrcPort, and a sender host must not send path state both with and without a zero SrcPort (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-5`](#rfc2205-3-5) RSVP must be prepared to maintain path state for Path messages from the same sender arriving from more than one PHOP (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-6`](#rfc2205-3-6) RSVP must not forward Path messages that arrive on an incoming interface different from that provided by routing (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-7`](#rfc2205-3-7) The STYLE object followed by the flow descriptor list must occur at the end of the message, and objects within the flow descriptor list must follow the BNF (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-8`](#rfc2205-3-8) The first FF flow descriptor must contain a FLOWSPEC (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-9`](#rfc2205-3-9) Where a Resv message with wildcard sender selection is forwarded to more than one previous hop, a SCOPE object must be included in the message (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-10`](#rfc2205-3-10) Matching path state must match the SESSION, SENDER_TEMPLATE and PHOP objects (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-11`](#rfc2205-3-11) A unicast PathTear must not be forwarded if there is path state for the same (session, sender) pair but a different PHOP (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-12`](#rfc2205-3-12) A PathTear message must be routed exactly like the corresponding Path message, with the session DestAddress as its IP destination address and the sender address as its IP source address (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-13`](#rfc2205-3-13) A SENDER_TSPEC or ADSPEC object in a PathTear, and a SCOPE object in a ResvTear, must be ignored (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-14`](#rfc2205-3-14) Deletion of path state by PathTear or timeout must also adjust related reservation state to maintain consistency in the local node (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-16`](#rfc2205-3-16) Matching reservation state must match the SESSION, STYLE and FILTER_SPEC objects and the LIH in the RSVP_HOP object (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-17`](#rfc2205-3-17) A ResvTear message must be routed like the corresponding Resv message (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-18`](#rfc2205-3-18) Each flow descriptor in an FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-19`](#rfc2205-3-19) A ResvErr message must contain the information required to define the error and to route the error message in later hops (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-20`](#rfc2205-3-20) Where admission control fails while increasing an existing reservation, the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-21`](#rfc2205-3-21) An immediate state forward must not trigger a message out the interface through which the triggering message arrived (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-22`](#rfc2205-3-22) Each RSVP message must occupy exactly one IP datagram (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-23`](#rfc2205-3-23) Forwarding of RSVP messages must avoid looping (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-24`](#rfc2205-3-24) Where reservation state from a NHOP carries no SCOPE object, a substitute sender list must be created and included in the union (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-25`](#rfc2205-3-25) A ResvErr message forwarded out an outgoing interface must carry a SCOPE object holding only those senders that route to that interface (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-26`](#rfc2205-3-26) RSVP must avoid refresh message synchronization and ensure that any synchronization that occurs is not stable (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-27`](#rfc2205-3-27) The state lifetime L must satisfy L >= (K + 0.5)*1.5*R (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-28`](#rfc2205-3-28) The ratio of two successive refresh periods R2/R1 must not exceed 1 + Slew.Max (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-29`](#rfc2205-3-29) RSVP must indicate the merge points it knows to the traffic control mechanism (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-30`](#rfc2205-3-30) RSVP must test for the presence of non-RSVP hops in the path and pass this information to traffic control (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-31`](#rfc2205-3-31) Where the routing protocol uses IP encapsulating tunnels, it must inform RSVP when non-RSVP hops are included (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-32`](#rfc2205-3-32) The RSVP process must be aware of the default sending interface, and an application that sets a specific interface must pass that information to RSVP (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-33`](#rfc2205-3-33) The RSVP process must determine which incoming interface to use for sending Resv messages by examining the path state (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-34`](#rfc2205-3-34) Path state that can match only a local application must be marked Local_Only by the RSVP process, and Local_Only path state must be ignored when Path and PathTear messages are forwarded (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-35`](#rfc2205-3-35) A forwarded message carrying objects of unknown class must obey the general object order requirements for its message type (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-36`](#rfc2205-3-36) RSVP must merge reservation requests from the corresponding next hops by computing the maximum of their flowspecs (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-37`](#rfc2205-3-37) An RSVP process must be able to query the routing process(es) for routes in order to forward Path and PathTear messages (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-38`](#rfc2205-3-38) RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-39`](#rfc2205-3-39) Packets received for IP protocol 46 but not addressed to the node must be diverted to the RSVP program for processing, without being forwarded (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-40`](#rfc2205-3-40) The identity of the interface on which a diverted message arrived, and the IP source address and IP TTL it arrived with, must be available to the RSVP process (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-41`](#rfc2205-3-41) RSVP must be able to force a datagram to be sent on a specific outgoing real or virtual link, bypassing the normal routing mechanism (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-42`](#rfc2205-3-42) RSVP must be able to specify the IP source address and IP TTL to be used when sending Path messages (§3) | no test | no test carries this requirement id |
| [`RFC2205-3-43`](#rfc2205-3-43) The service-dependent routines that manipulate the flowspec, Tspec and Adspec objects must be available to the RSVP process (§3) | no test | no test carries this requirement id |
| [`RFC2205-4-1`](#rfc2205-4-1) The SESSION object's DestAddress field must be non-zero (§4) | no test | no test carries this requirement id |
| [`RFC2205-4-2`](#rfc2205-4-2) The SESSION object's Protocol Id field must be non-zero (§4) | no test | no test carries this requirement id |
| [`RFC2205-4-3`](#rfc2205-4-3) The addresses in a SCOPE object must be listed in ascending numerical order (§4) | no test | no test carries this requirement id |
| [`RFC2205-4-4`](#rfc2205-4-4) Each node is required to verify the correct construction of each RSVP message it receives (§4) | no test | no test carries this requirement id |
| [`RFC2205-4-5`](#rfc2205-4-5) A host that cannot do raw network I/O must encapsulate RSVP messages in UDP, using a scheme that allows RSVP interoperation among an arbitrary topology of hosts and routers (§4) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2205-3.1-1`](#rfc2205-3.1-1)

Version field MUST be 1 (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRSVPDecodeHeaderBadVersion`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L439) | unit/verify | unproven |
| positive | [`TestRSVPHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L405) | unit/verify | unproven |

### [`RFC2205-3.1-2`](#rfc2205-3.1-2)

Reserved field in common header MUST be zero (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPReservedByteZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L581) | unit/verify | unproven |

### [`RFC2205-3.1.2-1`](#rfc2205-3.1.2-1)

Object lengths MUST be a multiple of 4 (§3.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPObjectLengthMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L602) | unit/verify | unproven |

### [`RFC2205-3.1.3-1`](#rfc2205-3.1.3-1)

A received message MUST carry every object its Section 3.1 BNF writes unbracketed -- SESSION, RSVP_HOP and TIME_VALUES in a Path, those three plus STYLE in a Resv, SESSION and RSVP_HOP in a PathTear, SESSION and ERROR_SPEC in a PathErr: each node is required to verify the correct construction of each message it receives, and a malformed message is logged locally rather than reported in an ERROR_SPEC (Appendix B) (§3.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L91) | unit/verify | revert, verified |
| negative | [`TestEnginePathWithoutTimeValuesDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestEnginePathWithTimeValuesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L131) | unit/verify | revert, verified |

### [`RFC2205-3.1-3`](#rfc2205-3.1-3)

Checksum MUST be verified on receipt; drop messages with bad checksum (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3.1-3, so no unit is bound to it.

### [`RFC2205-x-1`](#rfc2205-x-1)

IP Router Alert option MUST be set in PATH messages (Transport)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-x-1, so no unit is bound to it.

### [`RFC2205-3.10-1`](#rfc2205-3.10-1)

Unknown Class-Num of the form 0bbbbbbb: reject the entire message and return an "Unknown Object Class" error (§3.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEnginePathWithIgnorableObjectAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L935) | unit/verify | unproven |
| negative | [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L725) | unit/verify | unproven |
| positive | [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L718) | unit/verify | unproven |

### [`RFC2205-1-1`](#rfc2205-1-1)

Each router must be able to examine the UDP/TCP port fields used for packet classification (§1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-1-1, so no unit is bound to it.

### [`RFC2205-1-2`](#rfc2205-1-2)

Where the link-layer technology implements its own QoS management capability, RSVP must negotiate with the link layer to obtain the requested QoS (§1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-1-2, so no unit is bound to it.

### [`RFC2205-1-3`](#rfc2205-1-3)

Reservations from different downstream branches of the multicast tree(s) from the same sender must be merged as reservations travel upstream (§1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-1-3, so no unit is bound to it.

### [`RFC2205-1-4`](#rfc2205-1-4)

In an explicit sender-selection reservation, each filter spec must match exactly one sender (§1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-1-4, so no unit is bound to it.

### [`RFC2205-2-1`](#rfc2205-2-1)

Resv messages must follow exactly the reverse of the path(s) the data packets will use, upstream to all the sender hosts included in the sender selection (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-1, so no unit is bound to it.

### [`RFC2205-2-2`](#rfc2205-2-2)

Resv messages must finally be delivered to the sender hosts themselves (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-2, so no unit is bound to it.

### [`RFC2205-2-3`](#rfc2205-2-3)

A Path message is required to carry a Sender Template (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-3, so no unit is bound to it.

### [`RFC2205-2-4`](#rfc2205-2-4)

A Path message is required to carry a Sender Tspec (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-4, so no unit is bound to it.

### [`RFC2205-2-5`](#rfc2205-2-5)

Where a state update modifies state to be forwarded in refresh messages, those refresh messages must be generated and forwarded immediately (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-5, so no unit is bound to it.

### [`RFC2205-2-6`](#rfc2205-2-6)

State forwarded out interface I* must be computed using only state that arrived on interfaces different from I* (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-6, so no unit is bound to it.

### [`RFC2205-2-7`](#rfc2205-2-7)

Once initiated, a teardown request must be forwarded hop-by-hop without delay (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-7, so no unit is bound to it.

### [`RFC2205-2-8`](#rfc2205-2-8)

A reservation error must be reported to all of the responsible receivers (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-8, so no unit is bound to it.

### [`RFC2205-2-9`](#rfc2205-2-9)

Blockade state must not deny service to a smaller reservation that would succeed, and must not remove the state or prevent its immediate refresh (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-9, so no unit is bound to it.

### [`RFC2205-2-10`](#rfc2205-2-10)

Each node must answer the admission control and the policy control questions, and both must be favorable for RSVP to make a reservation (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-10, so no unit is bound to it.

### [`RFC2205-2-11`](#rfc2205-2-11)

RSVP must provide correct protocol operation even when two RSVP-capable routers are joined by an arbitrary cloud of non-RSVP routers (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-11, so no unit is bound to it.

### [`RFC2205-2-12`](#rfc2205-2-12)

Where the destination address matches no local interface and the message is not a Path or PathTear, the message must be forwarded without further processing by this node (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-12, so no unit is bound to it.

### [`RFC2205-3-1`](#rfc2205-3-1)

An RSVP implementation must recognize the object classes Section 3 lists (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-1, so no unit is bound to it.

### [`RFC2205-3-2`](#rfc2205-3-2)

The IP source address of a Path message must be an address of the sender it describes, and its destination address must be the DestAddress for the session (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-2, so no unit is bound to it.

### [`RFC2205-3-3`](#rfc2205-3-3)

Where the INTEGRITY object is present it must immediately follow the common header (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-3, so no unit is bound to it.

### [`RFC2205-3-4`](#rfc2205-3-4)

Path messages and filter spec matching must satisfy the SrcPort and DstPort consistency rules: the DstPort values for one DestAddress and ProtocolId are all zero or all non-zero, a zero DstPort forces zero SrcPort, and a sender host must not send path state both with and without a zero SrcPort (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-4, so no unit is bound to it.

### [`RFC2205-3-5`](#rfc2205-3-5)

RSVP must be prepared to maintain path state for Path messages from the same sender arriving from more than one PHOP (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-5, so no unit is bound to it.

### [`RFC2205-3-6`](#rfc2205-3-6)

RSVP must not forward Path messages that arrive on an incoming interface different from that provided by routing (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-6, so no unit is bound to it.

### [`RFC2205-3-7`](#rfc2205-3-7)

The STYLE object followed by the flow descriptor list must occur at the end of the message, and objects within the flow descriptor list must follow the BNF (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-7, so no unit is bound to it.

### [`RFC2205-3-8`](#rfc2205-3-8)

The first FF flow descriptor must contain a FLOWSPEC (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-8, so no unit is bound to it.

### [`RFC2205-3-9`](#rfc2205-3-9)

Where a Resv message with wildcard sender selection is forwarded to more than one previous hop, a SCOPE object must be included in the message (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-9, so no unit is bound to it.

### [`RFC2205-3-10`](#rfc2205-3-10)

Matching path state must match the SESSION, SENDER_TEMPLATE and PHOP objects (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-10, so no unit is bound to it.

### [`RFC2205-3-11`](#rfc2205-3-11)

A unicast PathTear must not be forwarded if there is path state for the same (session, sender) pair but a different PHOP (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-11, so no unit is bound to it.

### [`RFC2205-3-12`](#rfc2205-3-12)

A PathTear message must be routed exactly like the corresponding Path message, with the session DestAddress as its IP destination address and the sender address as its IP source address (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-12, so no unit is bound to it.

### [`RFC2205-3-13`](#rfc2205-3-13)

A SENDER_TSPEC or ADSPEC object in a PathTear, and a SCOPE object in a ResvTear, must be ignored (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-13, so no unit is bound to it.

### [`RFC2205-3-14`](#rfc2205-3-14)

Deletion of path state by PathTear or timeout must also adjust related reservation state to maintain consistency in the local node (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-14, so no unit is bound to it.

### [`RFC2205-3-16`](#rfc2205-3-16)

Matching reservation state must match the SESSION, STYLE and FILTER_SPEC objects and the LIH in the RSVP_HOP object (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-16, so no unit is bound to it.

### [`RFC2205-3-17`](#rfc2205-3-17)

A ResvTear message must be routed like the corresponding Resv message (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-17, so no unit is bound to it.

### [`RFC2205-3-18`](#rfc2205-3-18)

Each flow descriptor in an FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-18, so no unit is bound to it.

### [`RFC2205-3-19`](#rfc2205-3-19)

A ResvErr message must contain the information required to define the error and to route the error message in later hops (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-19, so no unit is bound to it.

### [`RFC2205-3-20`](#rfc2205-3-20)

Where admission control fails while increasing an existing reservation, the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-20, so no unit is bound to it.

### [`RFC2205-3-21`](#rfc2205-3-21)

An immediate state forward must not trigger a message out the interface through which the triggering message arrived (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-21, so no unit is bound to it.

### [`RFC2205-3-22`](#rfc2205-3-22)

Each RSVP message must occupy exactly one IP datagram (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-22, so no unit is bound to it.

### [`RFC2205-3-23`](#rfc2205-3-23)

Forwarding of RSVP messages must avoid looping (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-23, so no unit is bound to it.

### [`RFC2205-3-24`](#rfc2205-3-24)

Where reservation state from a NHOP carries no SCOPE object, a substitute sender list must be created and included in the union (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-24, so no unit is bound to it.

### [`RFC2205-3-25`](#rfc2205-3-25)

A ResvErr message forwarded out an outgoing interface must carry a SCOPE object holding only those senders that route to that interface (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-25, so no unit is bound to it.

### [`RFC2205-3-26`](#rfc2205-3-26)

RSVP must avoid refresh message synchronization and ensure that any synchronization that occurs is not stable (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-26, so no unit is bound to it.

### [`RFC2205-3-27`](#rfc2205-3-27)

The state lifetime L must satisfy L >= (K + 0.5)*1.5*R (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-27, so no unit is bound to it.

### [`RFC2205-3-28`](#rfc2205-3-28)

The ratio of two successive refresh periods R2/R1 must not exceed 1 + Slew.Max (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-28, so no unit is bound to it.

### [`RFC2205-3-29`](#rfc2205-3-29)

RSVP must indicate the merge points it knows to the traffic control mechanism (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-29, so no unit is bound to it.

### [`RFC2205-3-30`](#rfc2205-3-30)

RSVP must test for the presence of non-RSVP hops in the path and pass this information to traffic control (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-30, so no unit is bound to it.

### [`RFC2205-3-31`](#rfc2205-3-31)

Where the routing protocol uses IP encapsulating tunnels, it must inform RSVP when non-RSVP hops are included (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-31, so no unit is bound to it.

### [`RFC2205-3-32`](#rfc2205-3-32)

The RSVP process must be aware of the default sending interface, and an application that sets a specific interface must pass that information to RSVP (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-32, so no unit is bound to it.

### [`RFC2205-3-33`](#rfc2205-3-33)

The RSVP process must determine which incoming interface to use for sending Resv messages by examining the path state (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-33, so no unit is bound to it.

### [`RFC2205-3-34`](#rfc2205-3-34)

Path state that can match only a local application must be marked Local_Only by the RSVP process, and Local_Only path state must be ignored when Path and PathTear messages are forwarded (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-34, so no unit is bound to it.

### [`RFC2205-3-35`](#rfc2205-3-35)

A forwarded message carrying objects of unknown class must obey the general object order requirements for its message type (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-35, so no unit is bound to it.

### [`RFC2205-3-36`](#rfc2205-3-36)

RSVP must merge reservation requests from the corresponding next hops by computing the maximum of their flowspecs (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-36, so no unit is bound to it.

### [`RFC2205-3-37`](#rfc2205-3-37)

An RSVP process must be able to query the routing process(es) for routes in order to forward Path and PathTear messages (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-37, so no unit is bound to it.

### [`RFC2205-3-38`](#rfc2205-3-38)

RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-38, so no unit is bound to it.

### [`RFC2205-3-39`](#rfc2205-3-39)

Packets received for IP protocol 46 but not addressed to the node must be diverted to the RSVP program for processing, without being forwarded (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-39, so no unit is bound to it.

### [`RFC2205-3-40`](#rfc2205-3-40)

The identity of the interface on which a diverted message arrived, and the IP source address and IP TTL it arrived with, must be available to the RSVP process (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-40, so no unit is bound to it.

### [`RFC2205-3-41`](#rfc2205-3-41)

RSVP must be able to force a datagram to be sent on a specific outgoing real or virtual link, bypassing the normal routing mechanism (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-41, so no unit is bound to it.

### [`RFC2205-3-42`](#rfc2205-3-42)

RSVP must be able to specify the IP source address and IP TTL to be used when sending Path messages (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-42, so no unit is bound to it.

### [`RFC2205-3-43`](#rfc2205-3-43)

The service-dependent routines that manipulate the flowspec, Tspec and Adspec objects must be available to the RSVP process (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-43, so no unit is bound to it.

### [`RFC2205-4-1`](#rfc2205-4-1)

The SESSION object's DestAddress field must be non-zero (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-1, so no unit is bound to it.

### [`RFC2205-4-2`](#rfc2205-4-2)

The SESSION object's Protocol Id field must be non-zero (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-2, so no unit is bound to it.

### [`RFC2205-4-3`](#rfc2205-4-3)

The addresses in a SCOPE object must be listed in ascending numerical order (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-3, so no unit is bound to it.

### [`RFC2205-4-4`](#rfc2205-4-4)

Each node is required to verify the correct construction of each RSVP message it receives (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-4, so no unit is bound to it.

### [`RFC2205-4-5`](#rfc2205-4-5)

A host that cannot do raw network I/O must encapsulate RSVP messages in UDP, using a scheme that allows RSVP interoperation among an arbitrary topology of hosts and routers (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-5, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc2205.txt |
| Source fingerprint | 81606f8d5072deae |
| Record | rfc/extraction/rfc2205.json |
| Mapped sentences | 67 |
| Declined as scope | 56 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 9 | walked | not stated |
| `2` | not stated | 23 | walked | not stated |
| `3` | not stated | 77 | walked | not stated |
| `4` | not stated | 14 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | an illustrative example about packetized audio and what a receiver might issue; it imposes nothing | Packetized audio is an example of an application suitable for shared reservations; since a limited number of people talk at once, each receiver might issue a WF or SE reservation request for twice the bandwidth required for one sender (to allow some over-speaking). |
| `1:6` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | authorial voice about the worked example: the authors must specify the multicast routes within Figure 4 for the example to be readable | We must also specify the multicast routes within the node of Figure 4. |
| `1:7` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a caption sentence describing what Figure 5 illustrates | Figure 5, showing the WF style, illustrates two distinct situations in which merging is required. |
| `1:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the worked example of Figure 5 restating the merging obligation for two next hops on one interface | (1) Each of the two next hops on interface (d) results in a separate RSVP reservation request, as shown; these two requests must be merged into the effective flowspec, 3B, that is used to make the reservation on interface (d). |
| `1:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the worked example of Figure 5 restating the merging obligation for forwarding upstream | (2) The reservations on the interfaces (c) and (d) must be merged in order to forward the reservation requests upstream; as a result, the larger flowspec 4B is forwarded upstream to each previous hop. \| Sends \| Reserves Receives \| \| _______ WF( *{4B} ) <- (a) \| (c) \| * {4B}\| (c) <- WF( *{4B} ) \| \|_______\| \| -----------------------\|---------------------------------------- \| _______ WF( *{4B} ) <- (b) \| (d) \| * {3B}\| (d) <- WF( *{3B} ) \| \|_______\| <- WF( *{2B} ) |
| `2:5` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | the sentence sends the obligation outside this RFC: flowspecs are opaque to RSVP and the comparison rules are defined by the integrated services specifications, not here | Since flowspecs are opaque to RSVP, the actual rules for comparing flowspecs must be defined and implemented outside RSVP proper. |
| `2:6` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | the obligation binds the service-specific merging routines that site 2:5 places outside RSVP proper; the least upper bound is defined by the integrated services specification | In such a case, instead of taking the larger, the service-specific merging routines must be able to return a third flowspec that is at least as large as each; mathematically, this is the "least upper bound" (LUB). |
| `2:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same blockade state rule, stated for a different receiver establishing a smaller reservation | This must not prevent a different receiver from now establishing a smaller reservation Q0 that would succeed if not merged with Q1. |
| `2:13` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same blockade state rule, stated for the downstream routers' state and its refresh | The blockade state in each downstream router must not remove the state or prevent its immediate refresh. |
| `2:14` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a deployment observation that back pressure 'will generally be required on users'; it names no protocol behaviour | To prevent abuse, some form of back pressure will generally be required on users who make reservations. |
| `2:15` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a definition of the term policy control | The term "policy control" is used for the mechanisms required to support access policies and back pressure for RSVP reservations. |
| `2:17` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | descriptive: policy data is opaque to RSVP, which passes it to policy control | Like flowspecs, policy data is opaque to RSVP, which simply passes it to policy control when required. |
| `2:18` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | the sentence assigns the obligation to the policy control mechanism 'rather than by RSVP itself'; RFC 2750 defines that mechanism | Similarly, merging of policy data must be done by the policy control mechanism rather than by RSVP itself. |
| `2:19` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a statement about key management infrastructure availability at the time of writing, not a protocol obligation | Until that infrastructure becomes available, manual key management will be required to secure RSVP message integrity. |
| `2:21` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the non-RSVP cloud obligation as a requirement on the RSVP process | An RSVP process must be prepared to handle either situation. |
| `2:23` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a precondition on the application and the operator: the session identification is assigned and communicated out of band before RSVP runs, so no RSVP behaviour is required | Before a session can be created, the session identification (DestAddress, ProtocolId [, DstPort]) must be assigned and communicated to all the senders and receivers by some out-of-band mechanism. |
| `3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates the object length rule of the same section | Its length must be at least 4, but can be any multiple of 4. |
| `3:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | one line of the per-object required-in table the mandatory object rule already covers | Required in every Path and Resv message. |
| `3:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | one line of the per-object required-in table the mandatory object rule already covers | Required in every Resv message. |
| `3:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | one line of the per-object required-in table the mandatory object rule already covers | Required in a Path message. |
| `3:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | one line of the per-object required-in table the mandatory object rule already covers | Required in a Path message. |
| `3:12` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | descriptive: the RSVP process forwards and replicates Path messages using routing information; the obligation on which interface it may use is site 3:14 | The RSVP process forwards Path messages and replicates them as required by multicast sessions, using routing information it obtains from the appropriate uni-/multicast routing process. |
| `3:15` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the INTEGRITY placement rule restated for the Resv message | If the INTEGRITY object is present, it must immediately follow the common header. |
| `3:17` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a field description of the RSVP_HOP object; 'the reservation is required' names the logical interface, it imposes nothing | The NHOP (i.e., the RSVP_HOP) object contains the IP address of the interface through which the Resv message was sent and the LIH for the logical interface on which the reservation is required. |
| `3:19` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same pointer at the Section 3.2 port rules, for filter spec matching | This match must follow the rules of Section 3.2. o Wildcard sender selection |
| `3:24` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the addressing consequence of routing a PathTear exactly like its Path, in the same paragraph | Therefore, its IP destination address must be the session DestAddress, and its IP source address must be the sender address from the path state being torn down. |
| `3:30` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same ignore-on-receipt rule for an object a teardown message may carry, stated for SCOPE in ResvTear | A ResvTear message may include a SCOPE object, but it must be ignored. |
| `3:34` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the heading of the destination port consistency rule the row carries | Destination ports must be consistent. |
| `3:35` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the all-zero or all-non-zero DstPort rule of the same Section 3.2 port usage rules | Path state and reservation state for the same DestAddress and ProtocolId must each have DstPort values that are all zero or all non-zero. |
| `3:36` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the zero DstPort implies zero SrcPort rule of the same Section 3.2 port usage rules | If DstPort in a session definition is zero, all SrcPort fields used for that session must also be zero. |
| `3:37` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the heading of the source port consistency rule the row carries | Source Ports must be consistent. |
| `3:38` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the sender host half of the same Section 3.2 port usage rules | A sender host must not send path state both with and without a zero SrcPort. |
| `3:40` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the immediate forwarding of a state change, stated in Section 2 and again here | Upon the arrival of an RSVP message M that changes the state, a node must forward the state modification immediately. |
| `3:43` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the sentence defers the work: additional machinery for reservations within a tunnel is 'to be defined in the future', so nothing is required here | To establish RSVP reservations within the tunnel, additional machinery will be required, to be defined in the future. |
| `3:45` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same rule that state received on an interface is never forwarded out of it | If the topology has no loops, then looping of Resv and ResvErr messages with wildcard sender selection can be avoided by simply enforcing the rule given earlier: state that is received through a particular interface must never be forwarded out the same interface. |
| `3:52` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | an example sentence about the state of deployed sender hosts, introduced by 'For example' | For example, sender hosts must implement RSVP but currently many of them do not implement traffic control. |
| `3:54` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | 'other means must sometimes be used' names no mechanism and no addressee; the mechanisms are the sentences that follow, sites 3:55 and 3:56 | However, the TTL is not always a reliable indicator of non-RSVP hops, and other means must sometimes be used. |
| `3:56` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a fallback statement that manual configuration will be required where no automatic mechanism works; it binds the operator, not the protocol | If no automatic mechanism will work, manual configuration will be required. |
| `3:58` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the parameter description of the same application-set interface rule | If it is set by the application, this parameter must be the interface address for sending the data packets; otherwise, the system default interface is implied. |
| `3:61` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the forwarding half of the same Local_Only rule | When Path and PathTear messages are forwarded, path state marked "Local_Only" must be ignored. |
| `3:63` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | advice to whoever defines a new object class, about examining scaling limitations before deployment | Forwarding objects with unknown class enables incremental deployment of new objects; however, the scaling limitations of doing so must be carefully examined before a new object class is deployed with both high bits on. |
| `3:64` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates why merging is required, which the Section 1 merging row carries | Merging of RSVP reservations is required because of multicast data delivery, which replicates data packets for delivery to different next-hop nodes. |
| `3:66` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same merging rule for reservations on corresponding outgoing interfaces | In this case, RSVP must merge the reservations that are in place on the corresponding outgoing interfaces in order to forward a request upstream. |
| `3:67` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same merging rule for several next hops on one outgoing interface | In these cases, RSVP must merge the reservations from the different next hops in order to make the reservation on the single outgoing interface. |
| `3:68` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same merging rule for forwarding a request upstream | It must also merge reservations requests from all outgoing interfaces in order to forward a request upstream. |
| `3:69` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a statement that these complexities do not change the protocol processing required | In general, these complexities do not impact the protocol processing that is required by RSVP, except to determine exactly what reservation requests need to be merged. |
| `3:76` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the interface requirement behind the Router Alert obligation the row already carries | RSVP must be able to cause Path, PathTear, and ResvConf message to be sent with the Router Alert IP option. 3.11.6 Service-Dependent Manipulations |
| `4:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the bare 'Must be non-zero.' of a later object field description in the same appendix, repeating the non-zero field constraint | Must be non-zero. |
| `4:5` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the description of an error code value: what Error Code 2 means, with credentials named as an example | Reservation or path message has been rejected for administrative reasons, for example, required credentials not submitted, insufficient quota or balance, or administrative preemption. |
| `4:6` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the description of an error code value carrying an API error, not an obligation on the protocol | Error Value field contains an API error code, for an API error that was detected asynchronously and must be reported via an upcall. o Error Code = 21: Traffic Control Error |
| `4:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | one bullet of the list of construction faults the verification at site 4:7 detects | o Required object class (specify) missing |
| `4:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | two more bullets of the same list of construction faults | o Violation of required object order o Flow descriptor count wrong for style or message type |
| `4:11` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the interoperation property of the same UDP encapsulation scheme | The UDP encapsulation scheme must allow RSVP interoperation among an arbitrary topology of Hr hosts, Hu hosts, and routers. |
| `4:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | a TTL detail of the same UDP encapsulation scheme | Here Ta must be the TTL to exactly reach R. |
| `4:13` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the configuration detail of the same UDP encapsulation scheme | The host Hu must be explicitly configured with Ra and Ta. |
| `4:14` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the definition of an RSVP session: one simplex unicast or multicast data flow | An RSVP session defines one simplex unicast or multicast data flow for which reservations are required. |

## Superseded

No document obsoletes RFC 2205, so its obligations are stated where they were written.
