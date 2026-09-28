# RFC 2205 - Resource ReSerVation Protocol (RSVP) -- Version 1 Functional Specification

Experimental. Every requirement this repository extracted from RFC 2205, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 21.4% | 15 of 70 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 8.6% | 6 of 70 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 70 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 68.9% | 31 of 45 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 70 | of 75 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 70 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 70 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 1.4% | 1 of 70 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 70 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 68.6% | 48 of 70 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

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
| Declared gaps | 48 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 49 |
| Tagged units | 45 |
| Recorded audit verdicts | 0 |
| Discrimination records | 31 |
| Summary | `rfc/short/rfc2205.md` |
| Requirement shard | `rfc/requirements/rfc2205.md` |
| RFC text | `rfc/full/rfc2205.txt` |

## Enrolment

Enrolled: RSVP version 1 base protocol (codec shared by ze's RSVP-TE plugin). The 2026-09-21 extraction walk read all 123 normative sites and the checklist now holds 75 rows: 70 MUST or MUST NOT, 2 SHOULD, 1 SHOULD NOT and 2 MAY. Seven MUST rows were enrolled first and are described below; the walk added 63 more, none of them bound to a test, covering the reservation model of Sections 1 and 2 (merging, teardown, blockade state, admission and policy control, non-RSVP clouds), the message rules of Section 3 (addressing, object order, port consistency, state matching, teardown routing, error reporting, refresh timing, loop avoidance, and the routing, interface and packet-diversion services RSVP requires of the node), and the object field and UDP encapsulation rules of the appendices. Of the seven: 3.1-1 (Version MUST be 1) is met with positive+negative tags (encode round-trip and bad-version decode reject, internal/plugins/rsvpte/wire.go). 3.1-2 (reserved octet 0 on send) and 3.1.2-1 (object length multiple of 4) are {single-polarity: positive} with send-side tests (wire.go and the object encoders). 3.1.3-1 (a received message carries every object its BNF writes unbracketed) is met with positive+negative tags: checkMandatoryObjects (internal/plugins/rsvpte/mandatory.go) refuses a Path, Resv, PathTear or PathErr that omits one, and handlePacket drops it with a log line and no ERROR_SPEC. 3.10-1 (reject an unknown Class-Num of the form 0bbbbbbb) is met with positive+negative tags: DecodeMessage classifies by the high-order bit (classifyUnknownClass, wire.go) and engine.rejectUnknownObject answers a PATH with Error Code 13. 3.1-3 (verify checksum on receipt) and x-1 (IP Router Alert in PATH) are {gap}: ze's receive path (wire.go DecodeHeader) and raw-socket send (transport_linux.go) omit these. Disclosed in the docs/features/rfc-status.md RFC 2205 row.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- RSVP base common-header and object codec used by RSVP-TE: Version-1 header enforced on decode, reserved octet zeroed on send, every emitted object length a multiple of 4, and a received Path, Resv, PathTear, PathErr or ResvConf dropped when it omits an object its Section 3.1 BNF writes unbracketed
- tests bound per requirement in [`rfc/requirements/rfc2205.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc2205.md).


**What the ledger says remains**

Forty-eight MUST rows carry {gap}. Checksum validation, supported FF/SE message checks, ResvErr/ResvTear, local address-based policy and native transport are implemented. Complete requirement-level proof remains open. Missing enforcement includes refresh synchronization avoidance, the lifetime floor, bounded period increases and non-RSVP-hop detection. Received-interface loop checks and reverse-interface retention also remain absent. IntServ host reservations, multicast/WF signaling, authenticated POLICY_DATA interpretation and UDP encapsulation are absent capabilities, not implementation authorization. The baseline records bounded native Ze-to-Ze evidence separately from full conformance and independent-peer interoperability.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 15 | one part of the gated population |
| Annotated instead of tested | 55 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **70** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (15):** [`RFC2205-3.1-1`](#rfc2205-3.1-1), [`RFC2205-3.1.2-1`](#rfc2205-3.1.2-1), [`RFC2205-3.1.3-1`](#rfc2205-3.1.3-1), [`RFC2205-x-1`](#rfc2205-x-1), [`RFC2205-3.10-1`](#rfc2205-3.10-1), [`RFC2205-2-1`](#rfc2205-2-1), [`RFC2205-2-2`](#rfc2205-2-2), [`RFC2205-2-3`](#rfc2205-2-3), [`RFC2205-2-4`](#rfc2205-2-4), [`RFC2205-2-7`](#rfc2205-2-7), [`RFC2205-3-1`](#rfc2205-3-1), [`RFC2205-3-2`](#rfc2205-3-2), [`RFC2205-3-12`](#rfc2205-3-12), [`RFC2205-3-14`](#rfc2205-3-14), [`RFC2205-4-4`](#rfc2205-4-4)

**Annotated instead of tested (55):** [`RFC2205-3.1-2`](#rfc2205-3.1-2), [`RFC2205-3.1-3`](#rfc2205-3.1-3), [`RFC2205-1-1`](#rfc2205-1-1), [`RFC2205-1-2`](#rfc2205-1-2), [`RFC2205-1-3`](#rfc2205-1-3), [`RFC2205-1-4`](#rfc2205-1-4), [`RFC2205-2-5`](#rfc2205-2-5), [`RFC2205-2-6`](#rfc2205-2-6), [`RFC2205-2-8`](#rfc2205-2-8), [`RFC2205-2-9`](#rfc2205-2-9), [`RFC2205-2-10`](#rfc2205-2-10), [`RFC2205-2-11`](#rfc2205-2-11), [`RFC2205-2-12`](#rfc2205-2-12), [`RFC2205-3-3`](#rfc2205-3-3), [`RFC2205-3-4`](#rfc2205-3-4), [`RFC2205-3-5`](#rfc2205-3-5), [`RFC2205-3-6`](#rfc2205-3-6), [`RFC2205-3-7`](#rfc2205-3-7), [`RFC2205-3-8`](#rfc2205-3-8), [`RFC2205-3-9`](#rfc2205-3-9), [`RFC2205-3-10`](#rfc2205-3-10), [`RFC2205-3-11`](#rfc2205-3-11), [`RFC2205-3-13`](#rfc2205-3-13), [`RFC2205-3-16`](#rfc2205-3-16), [`RFC2205-3-17`](#rfc2205-3-17), [`RFC2205-3-18`](#rfc2205-3-18), [`RFC2205-3-19`](#rfc2205-3-19), [`RFC2205-3-20`](#rfc2205-3-20), [`RFC2205-3-21`](#rfc2205-3-21), [`RFC2205-3-22`](#rfc2205-3-22), [`RFC2205-3-23`](#rfc2205-3-23), [`RFC2205-3-24`](#rfc2205-3-24), [`RFC2205-3-25`](#rfc2205-3-25), [`RFC2205-3-26`](#rfc2205-3-26), [`RFC2205-3-27`](#rfc2205-3-27), [`RFC2205-3-28`](#rfc2205-3-28), [`RFC2205-3-29`](#rfc2205-3-29), [`RFC2205-3-30`](#rfc2205-3-30), [`RFC2205-3-31`](#rfc2205-3-31), [`RFC2205-3-32`](#rfc2205-3-32), [`RFC2205-3-33`](#rfc2205-3-33), [`RFC2205-3-34`](#rfc2205-3-34), [`RFC2205-3-35`](#rfc2205-3-35), [`RFC2205-3-36`](#rfc2205-3-36), [`RFC2205-3-37`](#rfc2205-3-37), [`RFC2205-3-38`](#rfc2205-3-38), [`RFC2205-3-39`](#rfc2205-3-39), [`RFC2205-3-40`](#rfc2205-3-40), [`RFC2205-3-41`](#rfc2205-3-41), [`RFC2205-3-42`](#rfc2205-3-42), [`RFC2205-3-43`](#rfc2205-3-43), [`RFC2205-4-1`](#rfc2205-4-1), [`RFC2205-4-2`](#rfc2205-4-2), [`RFC2205-4-3`](#rfc2205-4-3), [`RFC2205-4-5`](#rfc2205-4-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2205-3.1-1` | Version field MUST be 1 (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRSVPHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L409). **negative:** `unit/verify` [`TestRSVPDecodeHeaderBadVersion`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L443) |
| `RFC2205-3.1-2` | Reserved field in common header MUST be zero (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRSVPReservedByteZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L585). **negative:** no negative test. **{single-polarity}:** ze sets the reserved byte to 0 on send (internal/plugins/rsvpte/wire.go:177) and the RFC does not require receivers to reject a nonzero reserved field, so no negative case exists |
| `RFC2205-3.1.2-1` | Object lengths MUST be a multiple of 4 (§3.1.2) | MUST | 3.1.2 | **positive:** `unit/verify` [`TestRSVPObjectLengthMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L606). **negative:** `unit/verify` [`TestRFC2205ReceivedObjectLengthNotMultipleOfFourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L49) |
| `RFC2205-3.1.3-1` | A received message MUST carry every object its Section 3.1 BNF writes unbracketed -- SESSION, RSVP_HOP and TIME_VALUES in a Path, those three plus STYLE in a Resv, SESSION and RSVP_HOP in a PathTear, SESSION and ERROR_SPEC in a PathErr, and SESSION, ERROR_SPEC, RESV_CONFIRM and STYLE in a ResvConf: each node is required to verify the correct construction of each message it receives, and a malformed message is logged locally rather than reported in an ERROR_SPEC (Appendix B) (§3.1.3) | MUST | 3.1.3 | **positive:** `unit/verify` [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L90). **positive:** `unit/verify` [`TestEnginePathWithTimeValuesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L139). **negative:** `unit/verify` [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L98). **negative:** `unit/verify` [`TestEnginePathWithoutTimeValuesDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L120) |
| `RFC2205-3.1-3` | A nonzero checksum MUST be verified on receipt; drop messages with bad checksum (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeMessage in internal/plugins/rsvpte/wire.go verifies the declared message when its checksum is nonzero and handlePacket drops decode errors. Section 3.1.1 permits zero to mean no checksum was transmitted. Complete requirement-level proof remains open. |
| `RFC2205-x-1` | IP Router Alert option MUST be set in PATH messages (Transport) | MUST | x | **positive:** `unit/verify` [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L390). **negative:** `unit/verify` [`TestRFC2205PathNeverLeavesWithoutRouterAlertCarrier`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L14) |
| `RFC2205-3.10-1` | Unknown Class-Num of the form 0bbbbbbb: reject the entire message and return an "Unknown Object Class" error (§3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L724). **negative:** `unit/verify` [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L731). **negative:** `unit/verify` [`TestEnginePathWithIgnorableObjectAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L954) |
| `RFC2205-3.7-1` | Refresh period SHOULD be jittered by +/- 50% of R to prevent synchronization (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-x-2` | Jitter: SHOULD randomize refresh timing (Soft-State Model) | SHOULD | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3.10-2` | Unknown Class-Num of the form 10bbbbbb: ignore the object, neither forwarding it nor sending an error message (§3.10) | MAY | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3.10-3` | Unknown Class-Num of the form 11bbbbbb: ignore the object but forward it unexamined and unmodified in every message resulting from this one (§3.10) | MAY | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-1-1` | Because the UDP/TCP port numbers are used for packet classification, each router must be able to examine these fields. (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-1-2` | If the link-layer technology implements its own QoS management capability, then RSVP must negotiate with the link layer to obtain the requested QoS. (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-1-3` | More importantly, reservations from different downstream branches of the multicast tree(s) from the same sender (or set of senders) must be " merged" as reservations travel upstream. (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-1-4` | In an explicit sender-selection reservation, each filter spec must match exactly one sender (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** acceptReservation in internal/plugins/rsvpte/reservation.go resolves each supported tunnel filter to sender state before admission. Complete requirement-level proof remains open. |
| `RFC2205-2-1` | These messages must follow exactly the reverse of the path(s) the data packets will use, upstream to all the sender hosts included in the sender selection. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC2205ResvSentToPreviousHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L91). **negative:** `unit/verify` [`TestRFC2205ResvWithoutPathStateNotRelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L105) |
| `RFC2205-2-2` | Resv messages must finally be delivered to the sender hosts themselves (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC2205ResvDeliveredToSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L122). **negative:** `unit/verify` [`TestRFC2205ResvForUnknownSenderIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L139) |
| `RFC2205-2-3` | A Path message is required to carry a Sender Template (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC2205PathCarriesSenderTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L61). **negative:** `unit/verify` [`TestRFC2205PathWithoutSenderTemplateDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L80) |
| `RFC2205-2-4` | A Path message is required to carry a Sender Tspec (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC2205PathCarriesSenderTSpec`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L71). **negative:** `unit/verify` [`TestRFC2205PathWithoutSenderTSpecDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L121). **negative:** `unit/verify` [`TestRFC2205PathWithoutSenderTSpecRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/path_rfc2205_test.go#L13) |
| `RFC2205-2-5` | If this update results in modification of state to be forwarded in refresh messages, these refresh messages must be generated and forwarded immediately, so that state changes can be propagated end-to-end without delay. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handlePathTransit and acceptReservation relay supported state immediately; forwarding is not confined to refresh ticks. Proof covering every required state-update case remains open. |
| `RFC2205-2-6` | Conversely, state that is forwarded out interface I* must be computed using only state that arrived on interfaces different from I*. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-2-7` | Once initiated, a teardown request must be forwarded hop-by-hop without delay (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC2205TransitPathTearRelayedWithoutDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L166). **negative:** `unit/verify` [`TestRFC2205PathTearWithoutStateNotRelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L181) |
| `RFC2205-2-8` | A reservation error must be reported to all of the responsible receivers (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** reservation.go and reservation_build.go now produce and relay ResvErr for supported tunnel reservations. Complete receiver coverage and requirement-level proof remain open. |
| `RFC2205-2-9` | Blockade state must not deny service to a smaller reservation that would succeed, and must not remove the state or prevent its immediate refresh (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-2-10` | When a new reservation is requested, each node must answer two questions: "Are enough resources available to meet this request?" and "Is this user allowed to make this reservation?" These two decisions are termed the "admission control" decision and the "policy control" decision, respectively, and both must be favorable in order for RSVP to make a reservation. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** acceptReservation in internal/plugins/rsvpte/reservation.go requires local address-based policy and bandwidth admission before native installation. Authenticated POLICY_DATA interpretation is a separate absent capability; complete requirement-level proof remains open. |
| `RFC2205-2-11` | RSVP must therefore provide correct protocol operation even when two RSVP-capable routers are joined by an arbitrary "cloud" of non-RSVP routers. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** receive IP TTL is available, but non-RSVP-cloud detection and state are not implemented; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-2-12` | If the destination address does not match any local interface and the message is not a Path or PathTear, the message must be forwarded without further processing by this node. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ResvConf is relayed toward its receiver, but other nonlocal message types lack this forwarding path; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-3-1` | An RSVP implementation must recognize the object classes Section 3 lists (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC2205KnownClassesRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc2205_test.go#L64). **positive:** `unit/verify` [`TestRFC2205NullObjectRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L94). **negative:** `unit/verify` [`TestRFC2205NullObjectNotRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L107). **negative:** `unit/verify` [`TestRFC2205UnlistedClassNotRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc2205_test.go#L124) |
| `RFC2205-3-2` | The IP source address of a Path message must be an address of the sender it describes, while the destination address must be the DestAddress for the session. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC2205PathIPAddressesAreSenderAndSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L94). **negative:** `unit/verify` [`TestRFC2205PathIPAddressesNeverHopAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L107) |
| `RFC2205-3-3` | If the INTEGRITY object is present, it must immediately follow the common header. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** checkObjectPlacement in internal/plugins/rsvpte/message_validation.go enforces this position. Complete requirement-level proof remains open. |
| `RFC2205-3-4` | Path messages and filter spec matching must satisfy the SrcPort and DstPort consistency rules: the DstPort values for one DestAddress and ProtocolId are all zero or all non-zero, a zero DstPort forces zero SrcPort, and a sender host must not send path state both with and without a zero SrcPort (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-3-5` | Multicast routing allows a stable distribution tree in which Path messages from the same sender arrive from more than one PHOP, and RSVP must be prepared to maintain all such path state. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-6` | RSVP must not forward (according to the rules of Section 3.9) Path messages that arrive on an incoming interface different from that provided by routing. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** PATH processing does not compare the received interface index with the routing-derived incoming interface; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-3-7` | The STYLE object followed by the flow descriptor list must occur at the end of the message, and objects within the flow descriptor list must follow the BNF (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** checkObjectPlacement and checkFlowDescriptors enforce supported FF/SE ordering. Complete requirement-level proof remains open. |
| `RFC2205-3-8` | A FLOWSPEC object can be omitted if it is identical to the most recent such object that appeared in the list; the first FF flow descriptor must contain a FLOWSPEC. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** appendFilter in internal/plugins/rsvpte/message_validation.go rejects a first Resv filter without FLOWSPEC. Complete requirement-level proof remains open. |
| `RFC2205-3-9` | Whenever a Resv message with wildcard sender selection is forwarded to more than one previous hop, a SCOPE object must be included in the message (see Section 3.4 below); in this case, the scope for forwarding the reservation is constrained to just the sender IP addresses explicitly listed in the SCOPE object. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-10` | Matching state must have match the SESSION, SENDER_TEMPLATE, and PHOP objects. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handlePathTear in internal/plugins/rsvpte/engine.go now matches tunnel/sender identity and the stored RSVP_HOP before removal. Complete requirement-level proof remains open. |
| `RFC2205-3-11` | A unicast PathTear must not be forwarded if there is path state for the same (session, sender) pair but a different PHOP (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handlePathTear now rejects a different stored hop for ordinary and merged tunnel state. Complete requirement-level proof remains open. |
| `RFC2205-3-12` | A PathTear message must be routed exactly like the corresponding Path message. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC2205PathTearRoutedLikePath`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L119). **negative:** `unit/verify` [`TestRFC2205PathTearNotAddressedHopByHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L133) |
| `RFC2205-3-13` | A SENDER_TSPEC or ADSPEC object in a PathTear, and a SCOPE object in a ResvTear, must be ignored (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeMessage now skips PathTear TSPEC/ADSPEC bodies and ResvTear handling exists. Complete proof of the stated ignored-object cases remains open. |
| `RFC2205-3-14` | Deletion of path state by PathTear or timeout must also adjust related reservation state to maintain consistency in the local node (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC2205PathTearReleasesReservation`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L206). **negative:** `unit/verify` [`TestRFC2205PathTearForOtherLSPLeavesReservation`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L217) |
| `RFC2205-3-15` | The reservation changes a PathTear causes should not trigger an immediate Resv refresh message (§3) | SHOULD NOT | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-16` | Matching reservation state must match the SESSION, STYLE, and FILTER_SPEC objects as well as the LIH in the RSVP_HOP object. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleResvTear in internal/plugins/rsvpte/reservation.go now checks supported tunnel identity, style and the stored RSVP_HOP. Complete requirement-level proof remains open. |
| `RFC2205-3-17` | A ResvTear message must be routed like the corresponding Resv message (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** removeReservation relays ResvTear toward stored previous hops. The received-interface retention gap and complete requirement-level proof remain open. |
| `RFC2205-3-18` | Each flow descriptor in a FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** supported descriptors reach acceptReservation independently and rejection produces ResvErr. Complete requirement-level proof remains open. |
| `RFC2205-3-19` | This ResvErr message must contain the information required to define the error and to route the error message in later hops. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** reservation_build.go now encodes ResvErr and handleResvErr relays it. Complete requirement-level proof remains open. |
| `RFC2205-3-20` | If the error is an admission control failure while attempting to increase an existing reservation, then the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC of the ResvErr message. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** acceptReservation retains the accepted RSB on failed admission and passes existing-state presence to rejectReservation. Complete requirement-level proof remains open. |
| `RFC2205-3-21` | However, this must not trigger sending a message out the interface through which M arrived (which could happen if the implementation simply triggered an immediate refresh of all state for the session). (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** immediate forwarding does not exclude the triggering packet's incoming interface; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-3-22` | In this version of the spec, each RSVP message must occupy exactly one IP datagram. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux raw IPv4 socket; internal/plugins/rsvpte/transport_linux.go::Send and SendPath issue one SendmsgBuffers call per encoded message, and Linux combines its buffers into one datagram |
| `RFC2205-3-23` | Forwarding of RSVP messages must avoid looping (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** RRO and hop-limit handling do not supply the missing routing-derived incoming-interface checks; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-3-24` | Where reservation state from a NHOP carries no SCOPE object, a substitute sender list must be created and included in the union (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-25` | However, the ResvErr message forwarded out OI must contain a SCOPE object derived from L by including only those senders that route to OI. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-26` | RSVP must avoid refresh message synchronization and ensure that any synchronization that occurs is not stable (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** runRefreshLoop in internal/plugins/rsvpte/register.go uses a fixed ticker without synchronization avoidance; plan/pre-release/spec-rsvp-refresh-timing.md |
| `RFC2205-3-27` | To avoid premature loss of state, L must satisfy L >= (K + 0.5)*1.5*R, where K is a small integer. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** expiredPSBs in internal/plugins/rsvpte/fsm.go uses the received period multiplied by the configured factor, without enforcing this floor; plan/pre-release/spec-rsvp-refresh-timing.md |
| `RFC2205-3-28` | Specifically, the ratio of two successive values R2/R1 must not exceed 1 + Slew.Max. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** adoptedRefreshPeriod in internal/plugins/rsvpte/register.go adopts the configured period in one step without limiting its ratio; plan/pre-release/spec-rsvp-refresh-timing.md |
| `RFC2205-3-29` | RSVP knows where such points occur and must so indicate to the traffic control mechanism. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-30` | RSVP must also test for the presence of non-RSVP hops in the path and pass this information to traffic control. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze holds no RSVP-capability knowledge of a neighbor or a hop; plan/spec-rsvpte-non-rsvp-neighbors.md |
| `RFC2205-3-31` | For example, if the routing protocol uses IP encapsulating tunnels, then the routing protocol must inform RSVP when non-RSVP hops are included. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze holds no RSVP-capability knowledge of a neighbor or a hop; plan/spec-rsvpte-non-rsvp-neighbors.md |
| `RFC2205-3-32` | The RSVP process must be aware of the default, and if an application sets a specific interface, it must also pass that information to RSVP. o Sending Data (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-3-33` | The RSVP process must determine which case holds by examining the path state, to decide which incoming interface to use for sending Resv messages. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** PSB retains PHOP but not the received interface index, and RESV relies on a route lookup toward PHOP; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-3-34` | The path state on Iapp should only match a reservation from the local application; it must be marked "Local_only" by the RSVP process. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-3-35` | A forwarded message carrying objects of unknown class must obey the general object order requirements for its message type (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** receive-side placement checks now exist in message_validation.go; full proof of forwarded unknown-object placement remains open. |
| `RFC2205-3-36` | At each such replication point, RSVP must merge reservation requests from the corresponding next hops by computing the "maximum" of their flowspecs. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-37` | To forward Path and PathTear messages, an RSVP process must be able to query the routing process(s) for routes. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIntegrationRSVPPathAvoidsDataFEC`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L479). **positive:** `unit/verify` [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L392). **negative:** no negative test. **{single-polarity}:** a capability obligation ("must be able to"): the RFC names no behavior the node must avoid, so a negative case has nothing to show absent. The integration tests show the RSVP process resolving native routes for PATH and PathTear |
| `RFC2205-3-38` | RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** rawTransport.LocalAddresses in internal/plugins/rsvpte/transport_linux.go joins native addresses to interface index, state and MTU. Complete requirement-level proof, including virtual interfaces, remains open. |
| `RFC2205-3-39` | Packets received for IP protocol 46 but not addressed to the node must be diverted to the RSVP program for processing, without being forwarded (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIntegrationRSVPPathAvoidsDataFEC`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L480). **negative:** no negative test. **{single-polarity}:** the integration test shows a nonlocal PATH diverted to the RSVP receiver. The "without being forwarded" half is Linux behavior: a packet delivered to an IP_ROUTER_ALERT raw socket (internal/plugins/rsvpte/transport_linux.go openSockets) is consumed by the kernel router-alert chain before ip_forward, and no value at Ze's boundary decides it, so a negative case at that boundary would assert the kernel, not Ze |
| `RFC2205-3-40` | On a router or multi-homed host, the identity of the interface (real or virtual) on which a diverted message is received, as well as the IP source address and IP TTL with which it arrived, must also be available to the RSVP process. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L393). **negative:** no negative test. **{single-polarity}:** a capability obligation ("must be able to"): the RFC names no behavior the node must avoid, so a negative case has nothing to show absent. The integration test shows the receiver observing the arrival interface, IP source and IP TTL of a diverted message |
| `RFC2205-3-41` | RSVP must be able to force a (multicast) datagram to be sent on a specific outgoing real or virtual link, bypassing the normal routing mechanism. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L394). **negative:** no negative test. **{single-polarity}:** a capability obligation ("must be able to"): the RFC names no behavior the node must avoid, so a negative case has nothing to show absent. The integration test shows SendPath pinning the explicit hop link through IP_PKTINFO against the ordinary route |
| `RFC2205-3-42` | RSVP must be able to specify the IP source address and IP TTL to be used when sending Path messages (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L395). **negative:** no negative test. **{single-polarity}:** a capability obligation ("must be able to"): the RFC names no behavior the node must avoid, so a negative case has nothing to show absent. The integration test shows the requested source and Send_TTL in the captured IPv4 header |
| `RFC2205-3-43` | In order to manipulate these objects, RSVP process must have available to it the following service-dependent routines. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-4-1` | This field must be non-zero. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** decodeSessionIPv4 in internal/plugins/rsvpte/wire.go rejects an unspecified tunnel endpoint. Complete requirement-level proof remains open. |
| `RFC2205-4-2` | This field must be non-zero. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-4-3` | The addresses must be listed in ascending numerical order. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-4-4` | Similarly, each node is required to verify the correct construction of each RSVP message it receives. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestRFC2205WellFormedMessageVerified`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc2205_test.go#L133). **positive:** `unit/verify` [`TestRFC3209ObjectsAcceptedInAnyOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L249). **negative:** `unit/verify` [`TestRFC2205MissingRequiredObjectRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc2205_test.go#L142). **negative:** `unit/verify` [`TestRFC3209ObjectsAcceptedInAnyOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L280) |
| `RFC2205-4-5` | A host that cannot do raw network I/O must encapsulate RSVP messages in UDP, using a scheme that allows RSVP interoperation among an arbitrary topology of hosts and routers (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze fills no host-without-raw-I/O role; plan/spec-rsvp-udp-encapsulation.md |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2205-3.1-3`](#rfc2205-3.1-3) A nonzero checksum MUST be verified on receipt; drop messages with bad checksum (§3.1) | {gap}, no test | DecodeMessage in internal/plugins/rsvpte/wire.go verifies the declared message when its checksum is nonzero and handlePacket drops decode errors. Section 3.1.1 permits zero to mean no checksum was transmitted. Complete requirement-level proof remains open. |
| [`RFC2205-1-1`](#rfc2205-1-1) Because the UDP/TCP port numbers are used for packet classification, each router must be able to examine these fields. (§1) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-1-2`](#rfc2205-1-2) If the link-layer technology implements its own QoS management capability, then RSVP must negotiate with the link layer to obtain the requested QoS. (§1) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-1-3`](#rfc2205-1-3) More importantly, reservations from different downstream branches of the multicast tree(s) from the same sender (or set of senders) must be " merged" as reservations travel upstream. (§1) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-1-4`](#rfc2205-1-4) In an explicit sender-selection reservation, each filter spec must match exactly one sender (§1) | {gap}, no test | acceptReservation in internal/plugins/rsvpte/reservation.go resolves each supported tunnel filter to sender state before admission. Complete requirement-level proof remains open. |
| [`RFC2205-2-5`](#rfc2205-2-5) If this update results in modification of state to be forwarded in refresh messages, these refresh messages must be generated and forwarded immediately, so that state changes can be propagated end-to-end without delay. (§2) | {gap}, no test | handlePathTransit and acceptReservation relay supported state immediately; forwarding is not confined to refresh ticks. Proof covering every required state-update case remains open. |
| [`RFC2205-2-6`](#rfc2205-2-6) Conversely, state that is forwarded out interface I* must be computed using only state that arrived on interfaces different from I*. (§2) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-2-8`](#rfc2205-2-8) A reservation error must be reported to all of the responsible receivers (§2) | {gap}, no test | reservation.go and reservation_build.go now produce and relay ResvErr for supported tunnel reservations. Complete receiver coverage and requirement-level proof remain open. |
| [`RFC2205-2-9`](#rfc2205-2-9) Blockade state must not deny service to a smaller reservation that would succeed, and must not remove the state or prevent its immediate refresh (§2) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-2-10`](#rfc2205-2-10) When a new reservation is requested, each node must answer two questions: "Are enough resources available to meet this request?" and "Is this user allowed to make this reservation?" These two decisions are termed the "admission control" decision and the "policy control" decision, respectively, and both must be favorable in order for RSVP to make a reservation. (§2) | {gap}, no test | acceptReservation in internal/plugins/rsvpte/reservation.go requires local address-based policy and bandwidth admission before native installation. Authenticated POLICY_DATA interpretation is a separate absent capability; complete requirement-level proof remains open. |
| [`RFC2205-2-11`](#rfc2205-2-11) RSVP must therefore provide correct protocol operation even when two RSVP-capable routers are joined by an arbitrary "cloud" of non-RSVP routers. (§2) | {gap}, no test | receive IP TTL is available, but non-RSVP-cloud detection and state are not implemented; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-2-12`](#rfc2205-2-12) If the destination address does not match any local interface and the message is not a Path or PathTear, the message must be forwarded without further processing by this node. (§2) | {gap}, no test | ResvConf is relayed toward its receiver, but other nonlocal message types lack this forwarding path; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-3-3`](#rfc2205-3-3) If the INTEGRITY object is present, it must immediately follow the common header. (§3) | {gap}, no test | checkObjectPlacement in internal/plugins/rsvpte/message_validation.go enforces this position. Complete requirement-level proof remains open. |
| [`RFC2205-3-4`](#rfc2205-3-4) Path messages and filter spec matching must satisfy the SrcPort and DstPort consistency rules: the DstPort values for one DestAddress and ProtocolId are all zero or all non-zero, a zero DstPort forces zero SrcPort, and a sender host must not send path state both with and without a zero SrcPort (§3) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-3-5`](#rfc2205-3-5) Multicast routing allows a stable distribution tree in which Path messages from the same sender arrive from more than one PHOP, and RSVP must be prepared to maintain all such path state. (§3) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-6`](#rfc2205-3-6) RSVP must not forward (according to the rules of Section 3.9) Path messages that arrive on an incoming interface different from that provided by routing. (§3) | {gap}, no test | PATH processing does not compare the received interface index with the routing-derived incoming interface; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-3-7`](#rfc2205-3-7) The STYLE object followed by the flow descriptor list must occur at the end of the message, and objects within the flow descriptor list must follow the BNF (§3) | {gap}, no test | checkObjectPlacement and checkFlowDescriptors enforce supported FF/SE ordering. Complete requirement-level proof remains open. |
| [`RFC2205-3-8`](#rfc2205-3-8) A FLOWSPEC object can be omitted if it is identical to the most recent such object that appeared in the list; the first FF flow descriptor must contain a FLOWSPEC. (§3) | {gap}, no test | appendFilter in internal/plugins/rsvpte/message_validation.go rejects a first Resv filter without FLOWSPEC. Complete requirement-level proof remains open. |
| [`RFC2205-3-9`](#rfc2205-3-9) Whenever a Resv message with wildcard sender selection is forwarded to more than one previous hop, a SCOPE object must be included in the message (see Section 3.4 below); in this case, the scope for forwarding the reservation is constrained to just the sender IP addresses explicitly listed in the SCOPE object. (§3) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-10`](#rfc2205-3-10) Matching state must have match the SESSION, SENDER_TEMPLATE, and PHOP objects. (§3) | {gap}, no test | handlePathTear in internal/plugins/rsvpte/engine.go now matches tunnel/sender identity and the stored RSVP_HOP before removal. Complete requirement-level proof remains open. |
| [`RFC2205-3-11`](#rfc2205-3-11) A unicast PathTear must not be forwarded if there is path state for the same (session, sender) pair but a different PHOP (§3) | {gap}, no test | handlePathTear now rejects a different stored hop for ordinary and merged tunnel state. Complete requirement-level proof remains open. |
| [`RFC2205-3-13`](#rfc2205-3-13) A SENDER_TSPEC or ADSPEC object in a PathTear, and a SCOPE object in a ResvTear, must be ignored (§3) | {gap}, no test | DecodeMessage now skips PathTear TSPEC/ADSPEC bodies and ResvTear handling exists. Complete proof of the stated ignored-object cases remains open. |
| [`RFC2205-3-16`](#rfc2205-3-16) Matching reservation state must match the SESSION, STYLE, and FILTER_SPEC objects as well as the LIH in the RSVP_HOP object. (§3) | {gap}, no test | handleResvTear in internal/plugins/rsvpte/reservation.go now checks supported tunnel identity, style and the stored RSVP_HOP. Complete requirement-level proof remains open. |
| [`RFC2205-3-17`](#rfc2205-3-17) A ResvTear message must be routed like the corresponding Resv message (§3) | {gap}, no test | removeReservation relays ResvTear toward stored previous hops. The received-interface retention gap and complete requirement-level proof remain open. |
| [`RFC2205-3-18`](#rfc2205-3-18) Each flow descriptor in a FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error. (§3) | {gap}, no test | supported descriptors reach acceptReservation independently and rejection produces ResvErr. Complete requirement-level proof remains open. |
| [`RFC2205-3-19`](#rfc2205-3-19) This ResvErr message must contain the information required to define the error and to route the error message in later hops. (§3) | {gap}, no test | reservation_build.go now encodes ResvErr and handleResvErr relays it. Complete requirement-level proof remains open. |
| [`RFC2205-3-20`](#rfc2205-3-20) If the error is an admission control failure while attempting to increase an existing reservation, then the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC of the ResvErr message. (§3) | {gap}, no test | acceptReservation retains the accepted RSB on failed admission and passes existing-state presence to rejectReservation. Complete requirement-level proof remains open. |
| [`RFC2205-3-21`](#rfc2205-3-21) However, this must not trigger sending a message out the interface through which M arrived (which could happen if the implementation simply triggered an immediate refresh of all state for the session). (§3) | {gap}, no test | immediate forwarding does not exclude the triggering packet's incoming interface; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-3-22`](#rfc2205-3-22) In this version of the spec, each RSVP message must occupy exactly one IP datagram. (§3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux raw IPv4 socket; internal/plugins/rsvpte/transport_linux.go::Send and SendPath issue one SendmsgBuffers call per encoded message, and Linux combines its buffers into one datagram |
| [`RFC2205-3-23`](#rfc2205-3-23) Forwarding of RSVP messages must avoid looping (§3) | {gap}, no test | RRO and hop-limit handling do not supply the missing routing-derived incoming-interface checks; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-3-24`](#rfc2205-3-24) Where reservation state from a NHOP carries no SCOPE object, a substitute sender list must be created and included in the union (§3) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-25`](#rfc2205-3-25) However, the ResvErr message forwarded out OI must contain a SCOPE object derived from L by including only those senders that route to OI. (§3) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-26`](#rfc2205-3-26) RSVP must avoid refresh message synchronization and ensure that any synchronization that occurs is not stable (§3) | {gap}, no test | runRefreshLoop in internal/plugins/rsvpte/register.go uses a fixed ticker without synchronization avoidance; plan/pre-release/spec-rsvp-refresh-timing.md |
| [`RFC2205-3-27`](#rfc2205-3-27) To avoid premature loss of state, L must satisfy L >= (K + 0.5)*1.5*R, where K is a small integer. (§3) | {gap}, no test | expiredPSBs in internal/plugins/rsvpte/fsm.go uses the received period multiplied by the configured factor, without enforcing this floor; plan/pre-release/spec-rsvp-refresh-timing.md |
| [`RFC2205-3-28`](#rfc2205-3-28) Specifically, the ratio of two successive values R2/R1 must not exceed 1 + Slew.Max. (§3) | {gap}, no test | adoptedRefreshPeriod in internal/plugins/rsvpte/register.go adopts the configured period in one step without limiting its ratio; plan/pre-release/spec-rsvp-refresh-timing.md |
| [`RFC2205-3-29`](#rfc2205-3-29) RSVP knows where such points occur and must so indicate to the traffic control mechanism. (§3) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-30`](#rfc2205-3-30) RSVP must also test for the presence of non-RSVP hops in the path and pass this information to traffic control. (§3) | {gap}, no test | Ze holds no RSVP-capability knowledge of a neighbor or a hop; plan/spec-rsvpte-non-rsvp-neighbors.md |
| [`RFC2205-3-31`](#rfc2205-3-31) For example, if the routing protocol uses IP encapsulating tunnels, then the routing protocol must inform RSVP when non-RSVP hops are included. (§3) | {gap}, no test | Ze holds no RSVP-capability knowledge of a neighbor or a hop; plan/spec-rsvpte-non-rsvp-neighbors.md |
| [`RFC2205-3-32`](#rfc2205-3-32) The RSVP process must be aware of the default, and if an application sets a specific interface, it must also pass that information to RSVP. o Sending Data (§3) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-3-33`](#rfc2205-3-33) The RSVP process must determine which case holds by examining the path state, to decide which incoming interface to use for sending Resv messages. (§3) | {gap}, no test | PSB retains PHOP but not the received interface index, and RESV relies on a route lookup toward PHOP; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-3-34`](#rfc2205-3-34) The path state on Iapp should only match a reservation from the local application; it must be marked "Local_only" by the RSVP process. (§3) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-3-35`](#rfc2205-3-35) A forwarded message carrying objects of unknown class must obey the general object order requirements for its message type (§3) | {gap}, no test | receive-side placement checks now exist in message_validation.go; full proof of forwarded unknown-object placement remains open. |
| [`RFC2205-3-36`](#rfc2205-3-36) At each such replication point, RSVP must merge reservation requests from the corresponding next hops by computing the "maximum" of their flowspecs. (§3) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-38`](#rfc2205-3-38) RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses (§3) | {gap}, no test | rawTransport.LocalAddresses in internal/plugins/rsvpte/transport_linux.go joins native addresses to interface index, state and MTU. Complete requirement-level proof, including virtual interfaces, remains open. |
| [`RFC2205-3-43`](#rfc2205-3-43) In order to manipulate these objects, RSVP process must have available to it the following service-dependent routines. (§3) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-4-1`](#rfc2205-4-1) This field must be non-zero. (§4) | {gap}, no test | decodeSessionIPv4 in internal/plugins/rsvpte/wire.go rejects an unspecified tunnel endpoint. Complete requirement-level proof remains open. |
| [`RFC2205-4-2`](#rfc2205-4-2) This field must be non-zero. (§4) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-4-3`](#rfc2205-4-3) The addresses must be listed in ascending numerical order. (§4) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-4-5`](#rfc2205-4-5) A host that cannot do raw network I/O must encapsulate RSVP messages in UDP, using a scheme that allows RSVP interoperation among an arbitrary topology of hosts and routers (§4) | {gap}, no test | Ze fills no host-without-raw-I/O role; plan/spec-rsvp-udp-encapsulation.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2205-3.1-1`](#rfc2205-3.1-1)

Version field MUST be 1 (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRSVPDecodeHeaderBadVersion`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L443) | unit/verify | unproven |
| positive | [`TestRSVPHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L409) | unit/verify | unproven |

### [`RFC2205-3.1-2`](#rfc2205-3.1-2)

Reserved field in common header MUST be zero (§3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRSVPReservedByteZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L585) | unit/verify | unproven |

### [`RFC2205-3.1.2-1`](#rfc2205-3.1.2-1)

Object lengths MUST be a multiple of 4 (§3.1.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205ReceivedObjectLengthNotMultipleOfFourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestRSVPObjectLengthMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L606) | unit/verify | unproven |

### [`RFC2205-3.1.3-1`](#rfc2205-3.1.3-1)

A received message MUST carry every object its Section 3.1 BNF writes unbracketed -- SESSION, RSVP_HOP and TIME_VALUES in a Path, those three plus STYLE in a Resv, SESSION and RSVP_HOP in a PathTear, SESSION and ERROR_SPEC in a PathErr, and SESSION, ERROR_SPEC, RESV_CONFIRM and STYLE in a ResvConf: each node is required to verify the correct construction of each message it receives, and a malformed message is logged locally rather than reported in an ERROR_SPEC (Appendix B) (§3.1.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L98) | unit/verify | mutant, verified |
| negative | [`TestEnginePathWithoutTimeValuesDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L120) | unit/verify | revert, verified |
| positive | [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L90) | unit/verify | mutant, verified |
| positive | [`TestEnginePathWithTimeValuesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/mandatory_test.go#L139) | unit/verify | revert, verified |

### [`RFC2205-3.1-3`](#rfc2205-3.1-3)

A nonzero checksum MUST be verified on receipt; drop messages with bad checksum (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3.1-3, so no unit is bound to it.

### [`RFC2205-x-1`](#rfc2205-x-1)

IP Router Alert option MUST be set in PATH messages (Transport)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathNeverLeavesWithoutRouterAlertCarrier`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L14) | unit/verify | revert, verified |
| positive | [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L390) | unit/verify | unproven |

### [`RFC2205-3.10-1`](#rfc2205-3.10-1)

Unknown Class-Num of the form 0bbbbbbb: reject the entire message and return an "Unknown Object Class" error (§3.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEnginePathWithIgnorableObjectAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L954) | unit/verify | unproven |
| negative | [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L731) | unit/verify | unproven |
| positive | [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L724) | unit/verify | unproven |

### [`RFC2205-1-1`](#rfc2205-1-1)

Because the UDP/TCP port numbers are used for packet classification, each router must be able to examine these fields. (§1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-1-1, so no unit is bound to it.

### [`RFC2205-1-2`](#rfc2205-1-2)

If the link-layer technology implements its own QoS management capability, then RSVP must negotiate with the link layer to obtain the requested QoS. (§1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-1-2, so no unit is bound to it.

### [`RFC2205-1-3`](#rfc2205-1-3)

More importantly, reservations from different downstream branches of the multicast tree(s) from the same sender (or set of senders) must be " merged" as reservations travel upstream. (§1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-1-3, so no unit is bound to it.

### [`RFC2205-1-4`](#rfc2205-1-4)

In an explicit sender-selection reservation, each filter spec must match exactly one sender (§1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-1-4, so no unit is bound to it.

### [`RFC2205-2-1`](#rfc2205-2-1)

These messages must follow exactly the reverse of the path(s) the data packets will use, upstream to all the sender hosts included in the sender selection. (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205ResvWithoutPathStateNotRelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L105) | unit/verify | mutant, verified |
| positive | [`TestRFC2205ResvSentToPreviousHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L91) | unit/verify | revert, verified |

### [`RFC2205-2-2`](#rfc2205-2-2)

Resv messages must finally be delivered to the sender hosts themselves (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205ResvForUnknownSenderIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L139) | unit/verify | mutant, verified |
| positive | [`TestRFC2205ResvDeliveredToSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L122) | unit/verify | mutant, verified |

### [`RFC2205-2-3`](#rfc2205-2-3)

A Path message is required to carry a Sender Template (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathWithoutSenderTemplateDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathCarriesSenderTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L61) | unit/verify | revert, verified |

### [`RFC2205-2-4`](#rfc2205-2-4)

A Path message is required to carry a Sender Tspec (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathWithoutSenderTSpecRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/path_rfc2205_test.go#L13) | unit/verify | revert, verified |
| negative | [`TestRFC2205PathWithoutSenderTSpecDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L121) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathCarriesSenderTSpec`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L71) | unit/verify | revert, verified |

### [`RFC2205-2-5`](#rfc2205-2-5)

If this update results in modification of state to be forwarded in refresh messages, these refresh messages must be generated and forwarded immediately, so that state changes can be propagated end-to-end without delay. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-5, so no unit is bound to it.

### [`RFC2205-2-6`](#rfc2205-2-6)

Conversely, state that is forwarded out interface I* must be computed using only state that arrived on interfaces different from I*. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-6, so no unit is bound to it.

### [`RFC2205-2-7`](#rfc2205-2-7)

Once initiated, a teardown request must be forwarded hop-by-hop without delay (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathTearWithoutStateNotRelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L181) | unit/verify | revert, verified |
| positive | [`TestRFC2205TransitPathTearRelayedWithoutDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L166) | unit/verify | revert, verified |

### [`RFC2205-2-8`](#rfc2205-2-8)

A reservation error must be reported to all of the responsible receivers (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-8, so no unit is bound to it.

### [`RFC2205-2-9`](#rfc2205-2-9)

Blockade state must not deny service to a smaller reservation that would succeed, and must not remove the state or prevent its immediate refresh (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-9, so no unit is bound to it.

### [`RFC2205-2-10`](#rfc2205-2-10)

When a new reservation is requested, each node must answer two questions: "Are enough resources available to meet this request?" and "Is this user allowed to make this reservation?" These two decisions are termed the "admission control" decision and the "policy control" decision, respectively, and both must be favorable in order for RSVP to make a reservation. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-10, so no unit is bound to it.

### [`RFC2205-2-11`](#rfc2205-2-11)

RSVP must therefore provide correct protocol operation even when two RSVP-capable routers are joined by an arbitrary "cloud" of non-RSVP routers. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-11, so no unit is bound to it.

### [`RFC2205-2-12`](#rfc2205-2-12)

If the destination address does not match any local interface and the message is not a Path or PathTear, the message must be forwarded without further processing by this node. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-12, so no unit is bound to it.

### [`RFC2205-3-1`](#rfc2205-3-1)

An RSVP implementation must recognize the object classes Section 3 lists (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205NullObjectNotRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L107) | unit/verify | revert, verified |
| negative | [`TestRFC2205UnlistedClassNotRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc2205_test.go#L124) | unit/verify | revert, verified |
| positive | [`TestRFC2205NullObjectRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC2205KnownClassesRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc2205_test.go#L64) | unit/verify | mutant, verified |

### [`RFC2205-3-2`](#rfc2205-3-2)

The IP source address of a Path message must be an address of the sender it describes, while the destination address must be the DestAddress for the session. (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathIPAddressesNeverHopAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L107) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathIPAddressesAreSenderAndSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L94) | unit/verify | revert, verified |

### [`RFC2205-3-3`](#rfc2205-3-3)

If the INTEGRITY object is present, it must immediately follow the common header. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-3, so no unit is bound to it.

### [`RFC2205-3-4`](#rfc2205-3-4)

Path messages and filter spec matching must satisfy the SrcPort and DstPort consistency rules: the DstPort values for one DestAddress and ProtocolId are all zero or all non-zero, a zero DstPort forces zero SrcPort, and a sender host must not send path state both with and without a zero SrcPort (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-4, so no unit is bound to it.

### [`RFC2205-3-5`](#rfc2205-3-5)

Multicast routing allows a stable distribution tree in which Path messages from the same sender arrive from more than one PHOP, and RSVP must be prepared to maintain all such path state. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-5, so no unit is bound to it.

### [`RFC2205-3-6`](#rfc2205-3-6)

RSVP must not forward (according to the rules of Section 3.9) Path messages that arrive on an incoming interface different from that provided by routing. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-6, so no unit is bound to it.

### [`RFC2205-3-7`](#rfc2205-3-7)

The STYLE object followed by the flow descriptor list must occur at the end of the message, and objects within the flow descriptor list must follow the BNF (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-7, so no unit is bound to it.

### [`RFC2205-3-8`](#rfc2205-3-8)

A FLOWSPEC object can be omitted if it is identical to the most recent such object that appeared in the list; the first FF flow descriptor must contain a FLOWSPEC. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-8, so no unit is bound to it.

### [`RFC2205-3-9`](#rfc2205-3-9)

Whenever a Resv message with wildcard sender selection is forwarded to more than one previous hop, a SCOPE object must be included in the message (see Section 3.4 below); in this case, the scope for forwarding the reservation is constrained to just the sender IP addresses explicitly listed in the SCOPE object. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-9, so no unit is bound to it.

### [`RFC2205-3-10`](#rfc2205-3-10)

Matching state must have match the SESSION, SENDER_TEMPLATE, and PHOP objects. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-10, so no unit is bound to it.

### [`RFC2205-3-11`](#rfc2205-3-11)

A unicast PathTear must not be forwarded if there is path state for the same (session, sender) pair but a different PHOP (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-11, so no unit is bound to it.

### [`RFC2205-3-12`](#rfc2205-3-12)

A PathTear message must be routed exactly like the corresponding Path message. (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathTearNotAddressedHopByHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L133) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathTearRoutedLikePath`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/carrier_rfc2205_test.go#L119) | unit/verify | revert, verified |

### [`RFC2205-3-13`](#rfc2205-3-13)

A SENDER_TSPEC or ADSPEC object in a PathTear, and a SCOPE object in a ResvTear, must be ignored (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-13, so no unit is bound to it.

### [`RFC2205-3-14`](#rfc2205-3-14)

Deletion of path state by PathTear or timeout must also adjust related reservation state to maintain consistency in the local node (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathTearForOtherLSPLeavesReservation`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L217) | unit/verify | mutant, verified |
| positive | [`TestRFC2205PathTearReleasesReservation`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/engine_rfc2205_test.go#L206) | unit/verify | mutant, verified |

### [`RFC2205-3-16`](#rfc2205-3-16)

Matching reservation state must match the SESSION, STYLE, and FILTER_SPEC objects as well as the LIH in the RSVP_HOP object. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-16, so no unit is bound to it.

### [`RFC2205-3-17`](#rfc2205-3-17)

A ResvTear message must be routed like the corresponding Resv message (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-17, so no unit is bound to it.

### [`RFC2205-3-18`](#rfc2205-3-18)

Each flow descriptor in a FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-18, so no unit is bound to it.

### [`RFC2205-3-19`](#rfc2205-3-19)

This ResvErr message must contain the information required to define the error and to route the error message in later hops. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-19, so no unit is bound to it.

### [`RFC2205-3-20`](#rfc2205-3-20)

If the error is an admission control failure while attempting to increase an existing reservation, then the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC of the ResvErr message. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-20, so no unit is bound to it.

### [`RFC2205-3-21`](#rfc2205-3-21)

However, this must not trigger sending a message out the interface through which M arrived (which could happen if the implementation simply triggered an immediate refresh of all state for the session). (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-21, so no unit is bound to it.

### [`RFC2205-3-22`](#rfc2205-3-22)

In this version of the spec, each RSVP message must occupy exactly one IP datagram. (§3)

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

However, the ResvErr message forwarded out OI must contain a SCOPE object derived from L by including only those senders that route to OI. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-25, so no unit is bound to it.

### [`RFC2205-3-26`](#rfc2205-3-26)

RSVP must avoid refresh message synchronization and ensure that any synchronization that occurs is not stable (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-26, so no unit is bound to it.

### [`RFC2205-3-27`](#rfc2205-3-27)

To avoid premature loss of state, L must satisfy L >= (K + 0.5)*1.5*R, where K is a small integer. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-27, so no unit is bound to it.

### [`RFC2205-3-28`](#rfc2205-3-28)

Specifically, the ratio of two successive values R2/R1 must not exceed 1 + Slew.Max. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-28, so no unit is bound to it.

### [`RFC2205-3-29`](#rfc2205-3-29)

RSVP knows where such points occur and must so indicate to the traffic control mechanism. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-29, so no unit is bound to it.

### [`RFC2205-3-30`](#rfc2205-3-30)

RSVP must also test for the presence of non-RSVP hops in the path and pass this information to traffic control. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-30, so no unit is bound to it.

### [`RFC2205-3-31`](#rfc2205-3-31)

For example, if the routing protocol uses IP encapsulating tunnels, then the routing protocol must inform RSVP when non-RSVP hops are included. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-31, so no unit is bound to it.

### [`RFC2205-3-32`](#rfc2205-3-32)

The RSVP process must be aware of the default, and if an application sets a specific interface, it must also pass that information to RSVP. o Sending Data (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-32, so no unit is bound to it.

### [`RFC2205-3-33`](#rfc2205-3-33)

The RSVP process must determine which case holds by examining the path state, to decide which incoming interface to use for sending Resv messages. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-33, so no unit is bound to it.

### [`RFC2205-3-34`](#rfc2205-3-34)

The path state on Iapp should only match a reservation from the local application; it must be marked "Local_only" by the RSVP process. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-34, so no unit is bound to it.

### [`RFC2205-3-35`](#rfc2205-3-35)

A forwarded message carrying objects of unknown class must obey the general object order requirements for its message type (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-35, so no unit is bound to it.

### [`RFC2205-3-36`](#rfc2205-3-36)

At each such replication point, RSVP must merge reservation requests from the corresponding next hops by computing the "maximum" of their flowspecs. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-36, so no unit is bound to it.

### [`RFC2205-3-37`](#rfc2205-3-37)

To forward Path and PathTear messages, an RSVP process must be able to query the routing process(s) for routes. (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationRSVPPathAvoidsDataFEC`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L479) | unit/verify | unproven |
| positive | [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L392) | unit/verify | unproven |

### [`RFC2205-3-38`](#rfc2205-3-38)

RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-38, so no unit is bound to it.

### [`RFC2205-3-39`](#rfc2205-3-39)

Packets received for IP protocol 46 but not addressed to the node must be diverted to the RSVP program for processing, without being forwarded (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationRSVPPathAvoidsDataFEC`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L480) | unit/verify | unproven |

### [`RFC2205-3-40`](#rfc2205-3-40)

On a router or multi-homed host, the identity of the interface (real or virtual) on which a diverted message is received, as well as the IP source address and IP TTL with which it arrived, must also be available to the RSVP process. (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L393) | unit/verify | unproven |

### [`RFC2205-3-41`](#rfc2205-3-41)

RSVP must be able to force a (multicast) datagram to be sent on a specific outgoing real or virtual link, bypassing the normal routing mechanism. (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L394) | unit/verify | unproven |

### [`RFC2205-3-42`](#rfc2205-3-42)

RSVP must be able to specify the IP source address and IP TTL to be used when sending Path messages (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/transport_integration_linux_test.go#L395) | unit/verify | unproven |

### [`RFC2205-3-43`](#rfc2205-3-43)

In order to manipulate these objects, RSVP process must have available to it the following service-dependent routines. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-43, so no unit is bound to it.

### [`RFC2205-4-1`](#rfc2205-4-1)

This field must be non-zero. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-1, so no unit is bound to it.

### [`RFC2205-4-2`](#rfc2205-4-2)

This field must be non-zero. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-2, so no unit is bound to it.

### [`RFC2205-4-3`](#rfc2205-4-3)

The addresses must be listed in ascending numerical order. (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-3, so no unit is bound to it.

### [`RFC2205-4-4`](#rfc2205-4-4)

Similarly, each node is required to verify the correct construction of each RSVP message it receives. (§4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205MissingRequiredObjectRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc2205_test.go#L142) | unit/verify | revert, verified |
| negative | [`TestRFC3209ObjectsAcceptedInAnyOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L280) | unit/verify | mutant, verified |
| positive | [`TestRFC2205WellFormedMessageVerified`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc2205_test.go#L133) | unit/verify | revert, verified |
| positive | [`TestRFC3209ObjectsAcceptedInAnyOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L249) | unit/verify | mutant, verified |

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
