# RFC 2205 - Resource ReSerVation Protocol (RSVP) -- Version 1 Functional Specification

Experimental. Every requirement this repository extracted from RFC 2205, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 33.3% | 25 of 75 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 8.0% | 6 of 75 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 75 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 75 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 91.8% | 78 of 85 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 75 | of 80 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 75 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 75 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 1.3% | 1 of 75 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 75 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 57.3% | 43 of 75 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 75 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Experimental |
| Enrolment | Enrolled |
| Requirements | 80 |
| Gated MUST-level | 75 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 43 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 89 |
| Tagged units | 85 |
| Recorded audit verdicts | 19 |
| Discrimination records | 78 |
| Summary | `rfc/short/rfc2205.md` |
| Requirement shard | `rfc/requirements/rfc2205.md` |
| RFC text | `rfc/full/rfc2205.txt` |

## Enrolment

Enrolled: RSVP version 1 base protocol (codec shared by ze's RSVP-TE plugin). The 2026-09-21 extraction walk read all 123 normative sites and the checklist now holds 75 rows: 70 MUST or MUST NOT, 2 SHOULD, 1 SHOULD NOT and 2 MAY. Seven MUST rows were enrolled first and are described below; the walk added 63 more, none of them bound to a test, covering the reservation model of Sections 1 and 2 (merging, teardown, blockade state, admission and policy control, non-RSVP clouds), the message rules of Section 3 (addressing, object order, port consistency, state matching, teardown routing, error reporting, refresh timing, loop avoidance, and the routing, interface and packet-diversion services RSVP requires of the node), and the object field and UDP encapsulation rules of the appendices. Of the seven: 3.1-1 (Version MUST be 1) is met with positive+negative tags (encode round-trip and bad-version decode reject, internal/plugins/rsvpte/wire.go). 3.1-2 (reserved octet 0 on send) and 3.1.2-1 (object length multiple of 4) are {single-polarity: positive} with send-side tests (wire.go and the object encoders). 3.1.3-1 (a received message carries every object its BNF writes unbracketed) is met with positive+negative tags: checkMandatoryObjects (internal/plugins/rsvpte/mandatory.go) refuses a Path, Resv, PathTear or PathErr that omits one, and handlePacket drops it with a log line and no ERROR_SPEC. 3.10-1 (reject an unknown Class-Num of the form 0bbbbbbb) is met with positive+negative tags: DecodeMessage classifies by the high-order bit (classifyUnknownClass, wire.go) and engine.rejectUnknownObject answers a PATH with Error Code 13. 3.1-3 (verify checksum on receipt) and x-1 (IP Router Alert in PATH) are {gap}: ze's receive path (wire.go DecodeHeader) and raw-socket send (transport_linux.go) omit these. Disclosed in the docs/features/rfc-status.md RFC 2205 row.

## What the public ledger says

**Status:** Experimental

**What the ledger says is covered**

- RSVP base common-header and object codec used by RSVP-TE: Version-1 header enforced on decode, reserved octet zeroed on send, every emitted object length a multiple of 4, and a received Path, Resv, PathTear, PathErr or ResvConf dropped when it omits an object its Section 3.1 BNF writes unbracketed
- tests bound per requirement in [`rfc/requirements/rfc2205.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc2205.md). Reservation control: a ResvErr originated per failing FF descriptor and with InPlace on a refused increase, a ResvErr relayed toward the receiver, and a ResvTear matched on SESSION, STYLE, FILTER_SPEC and RSVP_HOP and relayed toward the previous hop ([`internal/plugins/rsvpte/reservation.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/reservation.go) -- acceptReservation, handleResvErr, handleResvTear). Against freeRouter, `./le test integration interop-rsvpte` shows a freeRouter egress receiving a Ze transit's InPlace ResvErr and its per-descriptor ResvErr for an unknown sender, and freeRouter relaying a Ze-originated ResvErr and ResvTear to another Ze node ([`internal/le/interoplab/rsvpte/checkers.go`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/rsvpte/checkers.go)).


**What the ledger says remains**

Forty-three MUST rows carry {gap}. Checksum validation, supported FF/SE message checks, ResvErr/ResvTear, local address-based policy and native transport are implemented; the ResvErr, ResvTear, FF per-descriptor error, InPlace and PathTear-object rows are proven by tagged tests, and complete requirement-level proof of the other implemented rows remains open. Missing enforcement includes refresh synchronization avoidance, the lifetime floor, bounded period increases and non-RSVP-hop detection. Received-interface loop checks and reverse-interface retention also remain absent. IntServ host reservations, multicast/WF signaling, authenticated POLICY_DATA interpretation and UDP encapsulation are absent capabilities, not implementation authorization. The baseline records bounded native Ze-to-Ze evidence separately from full conformance; independent-peer evidence is freeRouter only and limited to the reservation-control scenarios named under Support coverage.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 25 | one part of the gated population |
| Annotated (including scoped evidence) | 50 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **75** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (25):** [`RFC2205-3.1-1`](#rfc2205-3.1-1), [`RFC2205-3.1.2-1`](#rfc2205-3.1.2-1), [`RFC2205-3.1.3-1`](#rfc2205-3.1.3-1), [`RFC2205-3.1.1-1`](#rfc2205-3.1.1-1), [`RFC2205-x-1`](#rfc2205-x-1), [`RFC2205-3.10-1`](#rfc2205-3.10-1), [`RFC2205-2-1`](#rfc2205-2-1), [`RFC2205-2-2`](#rfc2205-2-2), [`RFC2205-2-3`](#rfc2205-2-3), [`RFC2205-2-4`](#rfc2205-2-4), [`RFC2205-2.3-1`](#rfc2205-2.3-1), [`RFC2205-2-7`](#rfc2205-2-7), [`RFC2205-2-8`](#rfc2205-2-8), [`RFC2205-3-1`](#rfc2205-3-1), [`RFC2205-3-2`](#rfc2205-3-2), [`RFC2205-3-12`](#rfc2205-3-12), [`RFC2205-3.1.5-1`](#rfc2205-3.1.5-1), [`RFC2205-3-13`](#rfc2205-3-13), [`RFC2205-3.1.6-1`](#rfc2205-3.1.6-1), [`RFC2205-3-14`](#rfc2205-3-14), [`RFC2205-3-17`](#rfc2205-3-17), [`RFC2205-3-18`](#rfc2205-3-18), [`RFC2205-3-19`](#rfc2205-3-19), [`RFC2205-3-20`](#rfc2205-3-20), [`RFC2205-4-4`](#rfc2205-4-4)

**Annotated (including scoped evidence) (50):** [`RFC2205-3.1-2`](#rfc2205-3.1-2), [`RFC2205-1-1`](#rfc2205-1-1), [`RFC2205-1-2`](#rfc2205-1-2), [`RFC2205-1-3`](#rfc2205-1-3), [`RFC2205-1-4`](#rfc2205-1-4), [`RFC2205-2-5`](#rfc2205-2-5), [`RFC2205-2-6`](#rfc2205-2-6), [`RFC2205-2-9`](#rfc2205-2-9), [`RFC2205-2.5-1`](#rfc2205-2.5-1), [`RFC2205-2-10`](#rfc2205-2-10), [`RFC2205-2-11`](#rfc2205-2-11), [`RFC2205-2-12`](#rfc2205-2-12), [`RFC2205-3-3`](#rfc2205-3-3), [`RFC2205-3-4`](#rfc2205-3-4), [`RFC2205-3-5`](#rfc2205-3-5), [`RFC2205-3-6`](#rfc2205-3-6), [`RFC2205-3-7`](#rfc2205-3-7), [`RFC2205-3-8`](#rfc2205-3-8), [`RFC2205-3-9`](#rfc2205-3-9), [`RFC2205-3-10`](#rfc2205-3-10), [`RFC2205-3-11`](#rfc2205-3-11), [`RFC2205-3-16`](#rfc2205-3-16), [`RFC2205-3-21`](#rfc2205-3-21), [`RFC2205-3-22`](#rfc2205-3-22), [`RFC2205-3-23`](#rfc2205-3-23), [`RFC2205-3-24`](#rfc2205-3-24), [`RFC2205-3-25`](#rfc2205-3-25), [`RFC2205-3-26`](#rfc2205-3-26), [`RFC2205-3-27`](#rfc2205-3-27), [`RFC2205-3-28`](#rfc2205-3-28), [`RFC2205-3-29`](#rfc2205-3-29), [`RFC2205-3-30`](#rfc2205-3-30), [`RFC2205-3-31`](#rfc2205-3-31), [`RFC2205-3-32`](#rfc2205-3-32), [`RFC2205-3-33`](#rfc2205-3-33), [`RFC2205-3-34`](#rfc2205-3-34), [`RFC2205-3.9-1`](#rfc2205-3.9-1), [`RFC2205-3-35`](#rfc2205-3-35), [`RFC2205-3-36`](#rfc2205-3-36), [`RFC2205-3-37`](#rfc2205-3-37), [`RFC2205-3-38`](#rfc2205-3-38), [`RFC2205-3-39`](#rfc2205-3-39), [`RFC2205-3-40`](#rfc2205-3-40), [`RFC2205-3-41`](#rfc2205-3-41), [`RFC2205-3-42`](#rfc2205-3-42), [`RFC2205-3-43`](#rfc2205-3-43), [`RFC2205-4-1`](#rfc2205-4-1), [`RFC2205-4-2`](#rfc2205-4-2), [`RFC2205-4-3`](#rfc2205-4-3), [`RFC2205-4-5`](#rfc2205-4-5)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2205-3.1-1` | Protocol version number.  This is version 1. (§3.1.1) | MUST | 3.1.1 | **positive:** `unit/verify` [`TestRFC2205EmittedVersionIsOne`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L20). **positive:** `unit/verify` [`TestRSVPHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L409). **negative:** `unit/verify` [`TestRSVPDecodeHeaderBadVersion`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L443) |
| `RFC2205-3.1-2` | All unused fields should be sent as zero and ignored on receipt. (§A) | MUST | A - Appendix A | **positive:** `unit/verify` [`TestRFC2205UnusedHeaderFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L35). **positive:** `unit/verify` [`TestRSVPReservedByteZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L585). **negative:** no negative test. **{single-polarity}:** ze sets the reserved byte to 0 on send (internal/plugins/rsvpte/wire.go:177) and the RFC does not require receivers to reject a nonzero reserved field, so no negative case exists |
| `RFC2205-3.1.2-1` | A 16-bit field containing the total object length in bytes.  Must always be a multiple of 4 (§3.1.2) | MUST | 3.1.2 | **positive:** `unit/verify` [`TestRSVPObjectLengthMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L606). **negative:** `unit/verify` [`TestRFC2205ReceivedObjectLengthNotMultipleOfFourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L48) |
| `RFC2205-3.1.3-1` | Similarly, each node is required to verify the correct construction of each RSVP message it receives.  Should a programming error allow an RSVP to create a malformed message, the error is not generally reported to end systems in an ERROR_SPEC object; instead, the error is simply logged locally, and perhaps reported through network management mechanisms. (§B) | MUST | B - Appendix B | **positive:** `unit/verify` [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_mandatory_test.go#L90). **positive:** `unit/verify` [`TestEnginePathWithTimeValuesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_mandatory_test.go#L139). **positive:** `unit/verify` [`TestRFC2205WellFormedMessageNotLogged`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L212). **negative:** `unit/verify` [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_mandatory_test.go#L98). **negative:** `unit/verify` [`TestEnginePathWithoutTimeValuesDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_mandatory_test.go#L120). **negative:** `unit/verify` [`TestRFC2205MalformedMessageLoggedLocally`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L196) |
| `RFC2205-3.1.1-1` | An all-zero value means that no checksum was transmitted. (§3.1.1) | MUST | 3.1.1 | **positive:** `unit/verify` [`TestRFC2205ZeroChecksumMeansNoneTransmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L515). **negative:** `unit/verify` [`TestRFC2205NonzeroChecksumStillVerified`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L533) |
| `RFC2205-x-1` | RSVP must be able to cause Path, PathTear, and ResvConf message to be sent with the Router Alert IP option. (§3.11.5) | MUST | 3.11.5 | **positive:** `unit/verify` [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L390). **negative:** `unit/verify` [`TestRFC2205PathNeverLeavesWithoutRouterAlertCarrier`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L14). **negative:** `unit/verify` [`TestRFC2205ResvConfNeverLeavesWithoutRouterAlertCarrier`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L400) |
| `RFC2205-3.10-1` | Class-Num = 0bbbbbbb The entire message should be rejected and an "Unknown Object Class" error returned. (§3.10) | MUST | 3.10 | **positive:** `unit/verify` [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L724). **negative:** `unit/verify` [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L731). **negative:** `unit/verify` [`TestEnginePathWithIgnorableObjectAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L949). **negative:** `unit/verify` [`TestRFC2205UnknownClassPathRejectedWithError`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L230). **negative:** `unit/verify` [`TestRFC2205UnknownClassResvRejectedWithError`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L250) |
| `RFC2205-3.7-1` | For this reason, the refresh timer should be randomly set to a value in the range [0.5R, 1.5R]. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-x-2` | For this reason, the refresh timer should be randomly set to a value in the range [0.5R, 1.5R]. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3.10-2` | Class-Num = 10bbbbbb The node should ignore the object, neither forwarding it nor sending an error message. (§3.10) | MAY | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3.10-3` | Class-Num = 11bbbbbb The node should ignore the object but forward it, unexamined and unmodified, in all messages resulting from this message. (§3.10) | MAY | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-1-1` | Because the UDP/TCP port numbers are used for packet classification, each router must be able to examine these fields. (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-1-2` | If the link-layer technology implements its own QoS management capability, then RSVP must negotiate with the link layer to obtain the requested QoS. (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-1-3` | More importantly, reservations from different downstream branches of the multicast tree(s) from the same sender (or set of senders) must be " merged" as reservations travel upstream. (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-1-4` | In an explicit sender-selection reservation, each filter spec must match exactly one sender (§1) | MUST | 1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** acceptReservation in internal/plugins/rsvpte/reservation.go resolves each supported tunnel filter to sender state before admission. Complete requirement-level proof remains open. |
| `RFC2205-2-1` | These messages must follow exactly the reverse of the path(s) the data packets will use, upstream to all the sender hosts included in the sender selection. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC2205ResvFollowsEachSendersReversePath`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L376). **positive:** `unit/verify` [`TestRFC2205ResvSentToPreviousHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L91). **negative:** `unit/verify` [`TestRFC2205ResvNeverTakesAnotherSendersPath`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L392). **negative:** `unit/verify` [`TestRFC2205ResvWithoutPathStateNotRelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L105) |
| `RFC2205-2-2` | Resv messages must finally be delivered to the sender hosts themselves (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC2205ResvDeliveredToSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L122). **negative:** `unit/verify` [`TestRFC2205ResvForUnknownSenderIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L139) |
| `RFC2205-2-3` | A Path message is required to carry a Sender Template (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC2205PathCarriesSenderTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L61). **negative:** `unit/verify` [`TestRFC2205PathWithoutSenderTemplateDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L80) |
| `RFC2205-2-4` | A Path message is required to carry a Sender Tspec (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC2205PathCarriesSenderTSpec`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L71). **negative:** `unit/verify` [`TestRFC2205PathWithoutSenderTSpecDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L121). **negative:** `unit/verify` [`TestRFC2205PathWithoutSenderTSpecRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_path_test.go#L13) |
| `RFC2205-2-5` | If this update results in modification of state to be forwarded in refresh messages, these refresh messages must be generated and forwarded immediately, so that state changes can be propagated end-to-end without delay. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handlePathTransit and acceptReservation relay supported state immediately; forwarding is not confined to refresh ticks. Proof covering every required state-update case remains open. |
| `RFC2205-2.3-1` | RSVP soft state is created and periodically refreshed by Path and Resv messages. The state is deleted if no matching refresh messages arrive before the expiration of a "cleanup timeout" interval. (§2.3) | MUST | 2.3 | **positive:** `unit/verify` [`TestRFC2205PathStateDeletedAtCleanupTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L68). **positive:** `unit/verify` [`TestRFC2205RefreshRecursEachTick`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L54). **positive:** `unit/verify` [`TestRFC2205ResvStateDeletedAtCleanupTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L128). **positive:** `unit/verify` [`TestRefreshResendsPathAndResv`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_test.go#L56). **negative:** `unit/verify` [`TestRFC2205RefreshedPathStateKept`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L81). **negative:** `unit/verify` [`TestRFC2205RefreshedResvStateKept`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L167). **negative:** `unit/verify` [`TestRefreshDoesNotStampEgressPSB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_test.go#L21) |
| `RFC2205-2-6` | Conversely, state that is forwarded out interface I* must be computed using only state that arrived on interfaces different from I*. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-2-7` | Once initiated, a teardown request must be forwarded hop-by-hop without delay (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC2205TransitPathTearRelayedWithoutDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L166). **negative:** `unit/verify` [`TestRFC2205PathTearWithoutStateNotRelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L181) |
| `RFC2205-2-8` | Since a request that fails may be the result of merging a number of requests, a reservation error must be reported to all of the responsible receivers. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestRFC2205ResvErrRelayedToReceiver`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L105). **negative:** `unit/verify` [`TestRFC2205ResvErrForOtherSenderNotRelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L128) |
| `RFC2205-2-9` | The blockade state in each downstream router must not remove the state or prevent its immediate refresh. (§2) | MUST NOT | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-2.5-1` | This must not prevent a different receiver from now establishing a smaller reservation Q0 that would succeed if not merged with Q1. (§2.5) | MUST NOT | 2.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-2-10` | When a new reservation is requested, each node must answer two questions: "Are enough resources available to meet this request?" and "Is this user allowed to make this reservation?" These two decisions are termed the "admission control" decision and the "policy control" decision, respectively, and both must be favorable in order for RSVP to make a reservation. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** acceptReservation in internal/plugins/rsvpte/reservation.go requires local address-based policy and bandwidth admission before native installation. Authenticated POLICY_DATA interpretation is a separate absent capability; complete requirement-level proof remains open. |
| `RFC2205-2-11` | RSVP must therefore provide correct protocol operation even when two RSVP-capable routers are joined by an arbitrary "cloud" of non-RSVP routers. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** receive IP TTL is available, but non-RSVP-cloud detection and state are not implemented; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-2-12` | If the destination address does not match any local interface and the message is not a Path or PathTear, the message must be forwarded without further processing by this node. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ResvConf is relayed toward its receiver, but other nonlocal message types lack this forwarding path; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-3-1` | An RSVP implementation must recognize the following classes: (§3.1.2) | MUST | 3.1.2 | **positive:** `unit/verify` [`TestRFC2205ErrorSpecClassRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L132). **positive:** `unit/verify` [`TestRFC2205KnownClassesRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_wire_test.go#L64). **positive:** `unit/verify` [`TestRFC2205NullObjectRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L94). **negative:** `unit/verify` [`TestRFC2205NullObjectNotRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L107). **negative:** `unit/verify` [`TestRFC2205UnlistedClassNotRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_wire_test.go#L124) |
| `RFC2205-3-2` | The IP source address of a Path message must be an address of the sender it describes, while the destination address must be the DestAddress for the session. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC2205PathIPAddressesAreSenderAndSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L93). **negative:** `unit/verify` [`TestRFC2205PathIPAddressesNeverHopAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L106) |
| `RFC2205-3-3` | If the INTEGRITY object is present, it must immediately follow the common header. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** checkObjectPlacement in internal/plugins/rsvpte/message_validation.go enforces this position. Complete requirement-level proof remains open. |
| `RFC2205-3-4` | Path state and reservation state for the same DestAddress and ProtocolId must each have DstPort values that are all zero or all non-zero.  Violation of this condition in a node is a "Conflicting Dest Ports" error.  2.   Destination ports rule.  If DstPort in a session definition is zero, all SrcPort fields used for that session must also be zero.  The assumption here is that the protocol does not have UDP/TCP- like ports.   Violation of this condition in a node is a "Bad Src Ports" error.  3.   Source Ports must be consistent.  A sender host must not send path state both with and without a zero SrcPort. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-3-5` | Multicast routing allows a stable distribution tree in which Path messages from the same sender arrive from more than one PHOP, and RSVP must be prepared to maintain all such path state. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-6` | RSVP must not forward (according to the rules of Section 3.9) Path messages that arrive on an incoming interface different from that provided by routing. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** PATH processing does not compare the received interface index with the routing-derived incoming interface; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-3-7` | The STYLE object followed by the flow descriptor list must occur at the end of the message, and objects within the flow descriptor list must follow the BNF (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** checkObjectPlacement and checkFlowDescriptors enforce supported FF/SE ordering. Complete requirement-level proof remains open. |
| `RFC2205-3-8` | A FLOWSPEC object can be omitted if it is identical to the most recent such object that appeared in the list; the first FF flow descriptor must contain a FLOWSPEC. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** appendFilter in internal/plugins/rsvpte/message_validation.go rejects a first Resv filter without FLOWSPEC. Complete requirement-level proof remains open. |
| `RFC2205-3-9` | Whenever a Resv message with wildcard sender selection is forwarded to more than one previous hop, a SCOPE object must be included in the message (see Section 3.4 below); in this case, the scope for forwarding the reservation is constrained to just the sender IP addresses explicitly listed in the SCOPE object. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-10` | Matching state must have match the SESSION, SENDER_TEMPLATE, and PHOP objects. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handlePathTear in internal/plugins/rsvpte/engine.go now matches tunnel/sender identity and the stored RSVP_HOP before removal. Complete requirement-level proof remains open. |
| `RFC2205-3-11` | A unicast PathTear must not be forwarded if there is path state for the same (session, sender) pair but a different PHOP (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handlePathTear now rejects a different stored hop for ordinary and merged tunnel state. Complete requirement-level proof remains open. |
| `RFC2205-3-12` | A PathTear message must be routed exactly like the corresponding Path message. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC2205PathTearRouteEqualsPathRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L277). **positive:** `unit/verify` [`TestRFC2205PathTearRoutedLikePath`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L118). **negative:** `unit/verify` [`TestRFC2205PathTearNotAddressedHopByHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L132) |
| `RFC2205-3.1.5-1` | Therefore, its IP destination address must be the session DestAddress, and its IP source address must be the sender address from the path state being torn down. (§3.1.5) | MUST | 3.1.5 | **positive:** `unit/verify` [`TestRFC2205PathTearAddressedFromSenderToSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L300). **negative:** `unit/verify` [`TestRFC2205PathTearSourceNotTakenFromCarrier`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L324) |
| `RFC2205-3-13` | A PathTear message may include a SENDER_TSPEC or ADSPEC object in its sender descriptor, but these must be ignored. (§3.1.5) | MUST | 3.1.5 | **positive:** `unit/verify` [`TestRFC2205PathTearObjectsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L152). **negative:** `unit/verify` [`TestRFC2205PathTearTSpecNotUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L181) |
| `RFC2205-3.1.6-1` | A ResvTear message may include a SCOPE object, but it must be ignored. (§3.1.6) | MUST | 3.1.6 | **positive:** `unit/verify` [`TestRFC2205ResvTearWithScopeTearsDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L455). **negative:** `unit/verify` [`TestRFC2205ResvTearScopeNotObeyed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L481) |
| `RFC2205-3-14` | Deletion of path state as the result of a PathTear message or a timeout must also adjust related reservation state as required to maintain consistency in the local node. (§3.1.5) | MUST | 3.1.5 | **positive:** `unit/verify` [`TestRFC2205PathStateDeletedAtCleanupTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L69). **positive:** `unit/verify` [`TestRFC2205PathTearReleasesReservation`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L206). **negative:** `unit/verify` [`TestRFC2205PathTearForOtherLSPLeavesReservation`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L217). **negative:** `unit/verify` [`TestRFC2205RefreshedPathStateKept`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L82) |
| `RFC2205-3-15` | These reservation changes should not trigger an immediate Resv refresh message, since the PathTear message has already made the required changes upstream. (§3.1.5) | SHOULD NOT | 3.1.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2205-3-16` | Matching reservation state must match the SESSION, STYLE, and FILTER_SPEC objects as well as the LIH in the RSVP_HOP object. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** handleResvTear in internal/plugins/rsvpte/reservation.go now checks supported tunnel identity, style and the stored RSVP_HOP. Complete requirement-level proof remains open. |
| `RFC2205-3-17` | A ResvTear message must be routed like the corresponding Resv message (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC2205ResvTearRoutedLikeResv`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L236). **negative:** `unit/verify` [`TestRFC2205ResvTearForOtherHopNotRouted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L262) |
| `RFC2205-3-18` | Each flow descriptor in a FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC2205FixedFilterErrorPerDescriptor`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L282). **negative:** `unit/verify` [`TestRFC2205FixedFilterGoodDescriptorKept`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L308) |
| `RFC2205-3-19` | This ResvErr message must contain the information required to define the error and to route the error message in later hops. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC2205ResvErrCarriesErrorAndRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L339). **negative:** `unit/verify` [`TestRFC2205ResvErrWrongStyleNotRouted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L368) |
| `RFC2205-3-20` | If the error is an admission control failure while attempting to increase an existing reservation, then the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC of the ResvErr message. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestRFC2205FailedIncreaseLeavesReservationInPlace`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L384). **negative:** `unit/verify` [`TestRFC2205FailedFirstReservationNotInPlace`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L420) |
| `RFC2205-3-21` | However, this must not trigger sending a message out the interface through which M arrived (which could happen if the implementation simply triggered an immediate refresh of all state for the session). (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** immediate forwarding does not exclude the triggering packet's incoming interface; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-3-22` | In this version of the spec, each RSVP message must occupy exactly one IP datagram. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux raw IPv4 socket; internal/plugins/rsvpte/transport_linux.go::Send and SendPath issue one SendmsgBuffers call per encoded message, and Linux combines its buffers into one datagram |
| `RFC2205-3-23` | Forwarding of RSVP messages must avoid looping (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** RRO and hop-limit handling do not supply the missing routing-derived incoming-interface checks; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-3-24` | If reservation state from some NHOP does not contain a SCOPE object, a substitute sender list must be created and included in the union. (§3.4) | MUST | 3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-25` | However, the ResvErr message forwarded out OI must contain a SCOPE object derived from L by including only those senders that route to OI. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-26` | Since RSVP sends periodic refresh messages, it must avoid message synchronization and ensure that any synchronization that may occur is not stable. (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** runRefreshLoop in internal/plugins/rsvpte/register.go uses a fixed ticker without synchronization avoidance; plan/pre-release/spec-rsvp-refresh-timing.md |
| `RFC2205-3-27` | To avoid premature loss of state, L must satisfy L >= (K + 0.5)*1.5*R, where K is a small integer. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** expiredPSBs in internal/plugins/rsvpte/fsm.go uses the received period multiplied by the configured factor, without enforcing this floor; plan/pre-release/spec-rsvp-refresh-timing.md |
| `RFC2205-3-28` | Specifically, the ratio of two successive values R2/R1 must not exceed 1 + Slew.Max. (§3) | MUST NOT | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** adoptedRefreshPeriod in internal/plugins/rsvpte/register.go adopts the configured period in one step without limiting its ratio; plan/pre-release/spec-rsvp-refresh-timing.md |
| `RFC2205-3-29` | RSVP knows where such points occur and must so indicate to the traffic control mechanism. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-30` | RSVP must also test for the presence of non-RSVP hops in the path and pass this information to traffic control. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze holds no RSVP-capability knowledge of a neighbor or a hop; plan/spec-rsvpte-non-rsvp-neighbors.md |
| `RFC2205-3-31` | For example, if the routing protocol uses IP encapsulating tunnels, then the routing protocol must inform RSVP when non-RSVP hops are included. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze holds no RSVP-capability knowledge of a neighbor or a hop; plan/spec-rsvpte-non-rsvp-neighbors.md |
| `RFC2205-3-32` | The RSVP process must be aware of the default, and if an application sets a specific interface, it must also pass that information to RSVP. o Sending Data (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-3-33` | The RSVP process must determine which case holds by examining the path state, to decide which incoming interface to use for sending Resv messages. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** PSB retains PHOP but not the received interface index, and RESV relies on a route lookup toward PHOP; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| `RFC2205-3-34` | The path state on Iapp should only match a reservation from the local application; it must be marked "Local_only" by the RSVP process. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-3.9-1` | When Path and PathTear messages are forwarded, path state marked "Local_Only" must be ignored. (§3.9) | MUST | 3.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-3-35` | The original order of such unknown-class objects need not be retained; however, the message that is forwarded must obey the general order requirements for its message type. (§3.10) | MUST | 3.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** receive-side placement checks now exist in message_validation.go; full proof of forwarded unknown-object placement remains open. |
| `RFC2205-3-36` | At each such replication point, RSVP must merge reservation requests from the corresponding next hops by computing the "maximum" of their flowspecs. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-3-37` | To forward Path and PathTear messages, an RSVP process must be able to query the routing process(s) for routes. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIntegrationRSVPPathAvoidsDataFEC`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L478). **positive:** `unit/verify` [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L391). **negative:** no negative test. **{single-polarity}:** a capability obligation ("must be able to"): the RFC names no behavior the node must avoid, so a negative case has nothing to show absent. The integration tests show the RSVP process resolving native routes for PATH and PathTear |
| `RFC2205-3-38` | RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** rawTransport.LocalAddresses in internal/plugins/rsvpte/transport_linux.go joins native addresses to interface index, state and MTU. Complete requirement-level proof, including virtual interfaces, remains open. |
| `RFC2205-3-39` | Packets received for IP protocol 46 but not addressed to the node must be diverted to the RSVP program for processing, without being forwarded (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIntegrationRSVPPathAvoidsDataFEC`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L479). **negative:** no negative test. **{single-polarity}:** the integration test shows a nonlocal PATH diverted to the RSVP receiver. The "without being forwarded" half is Linux behavior: a packet delivered to an IP_ROUTER_ALERT raw socket (internal/plugins/rsvpte/transport_linux.go openSockets) is consumed by the kernel router-alert chain before ip_forward, and no value at Ze's boundary decides it, so a negative case at that boundary would assert the kernel, not Ze |
| `RFC2205-3-40` | On a router or multi-homed host, the identity of the interface (real or virtual) on which a diverted message is received, as well as the IP source address and IP TTL with which it arrived, must also be available to the RSVP process. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L392). **negative:** no negative test. **{single-polarity}:** a capability obligation ("must be able to"): the RFC names no behavior the node must avoid, so a negative case has nothing to show absent. The integration test shows the receiver observing the arrival interface, IP source and IP TTL of a diverted message |
| `RFC2205-3-41` | RSVP must be able to force a (multicast) datagram to be sent on a specific outgoing real or virtual link, bypassing the normal routing mechanism. (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L393). **negative:** no negative test. **{single-polarity}:** a capability obligation ("must be able to"): the RFC names no behavior the node must avoid, so a negative case has nothing to show absent. The integration test shows SendPath pinning the explicit hop link through IP_PKTINFO against the ordinary route |
| `RFC2205-3-42` | RSVP must be able to specify the IP source address and IP TTL to be used when sending Path messages (§3) | MUST | 3 | **positive:** `unit/verify` [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L394). **negative:** no negative test. **{single-polarity}:** a capability obligation ("must be able to"): the RFC names no behavior the node must avoid, so a negative case has nothing to show absent. The integration test shows the requested source and Send_TTL in the captured IPv4 header |
| `RFC2205-3-43` | In order to manipulate these objects, RSVP process must have available to it the following service-dependent routines. (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-4-1` | This field must be non-zero. (§A) | MUST | A - Appendix A | **positive:** no positive test. **negative:** no negative test. **{gap}:** decodeSessionIPv4 in internal/plugins/rsvpte/wire.go rejects an unspecified tunnel endpoint. Complete requirement-level proof remains open. |
| `RFC2205-4-2` | This field must be non-zero. (§A) | MUST | A - Appendix A | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| `RFC2205-4-3` | The addresses must be listed in ascending numerical order. (§A) | MUST | A - Appendix A | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| `RFC2205-4-4` | Similarly, each node is required to verify the correct construction of each RSVP message it receives. (§B) | MUST | B - Appendix B | **positive:** `unit/verify` [`TestRFC2205EachMessageTypeConstructionAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L200). **positive:** `unit/verify` [`TestRFC2205WellFormedMessageVerified`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_wire_test.go#L133). **positive:** `unit/verify` [`TestRFC3209ObjectsAcceptedInAnyOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L249). **negative:** `unit/verify` [`TestRFC2205EachMessageTypeMalformedRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L209). **negative:** `unit/verify` [`TestRFC2205MissingRequiredObjectRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_wire_test.go#L142). **negative:** `unit/verify` [`TestRFC3209ObjectsAcceptedInAnyOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L280) |
| `RFC2205-4-5` | However, some important classes of host systems may not support raw network I/O.  To use RSVP, such hosts must encapsulate RSVP messages in UDP.  The basic UDP encapsulation scheme makes two assumptions:  1.   All hosts are capable of sending and receiving multicast packets if multicast destinations are to be supported.  2.   The first/last-hop routers are RSVP-capable.  A method of relaxing the second assumption is given later.  Let Hu be a "UDP-only" host that requires UDP encapsulation, and Hr a host that can do raw network I/O.  The UDP encapsulation scheme must allow RSVP interoperation among an arbitrary topology of Hr hosts, Hu hosts, and routers. (§C) | MUST | C - Appendix C | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze fills no host-without-raw-I/O role; plan/spec-rsvp-udp-encapsulation.md |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2205-1-1`](#rfc2205-1-1) Because the UDP/TCP port numbers are used for packet classification, each router must be able to examine these fields. (§1) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-1-2`](#rfc2205-1-2) If the link-layer technology implements its own QoS management capability, then RSVP must negotiate with the link layer to obtain the requested QoS. (§1) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-1-3`](#rfc2205-1-3) More importantly, reservations from different downstream branches of the multicast tree(s) from the same sender (or set of senders) must be " merged" as reservations travel upstream. (§1) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-1-4`](#rfc2205-1-4) In an explicit sender-selection reservation, each filter spec must match exactly one sender (§1) | {gap}, no test | acceptReservation in internal/plugins/rsvpte/reservation.go resolves each supported tunnel filter to sender state before admission. Complete requirement-level proof remains open. |
| [`RFC2205-2-5`](#rfc2205-2-5) If this update results in modification of state to be forwarded in refresh messages, these refresh messages must be generated and forwarded immediately, so that state changes can be propagated end-to-end without delay. (§2) | {gap}, no test | handlePathTransit and acceptReservation relay supported state immediately; forwarding is not confined to refresh ticks. Proof covering every required state-update case remains open. |
| [`RFC2205-2-6`](#rfc2205-2-6) Conversely, state that is forwarded out interface I* must be computed using only state that arrived on interfaces different from I*. (§2) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-2-9`](#rfc2205-2-9) The blockade state in each downstream router must not remove the state or prevent its immediate refresh. (§2) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-2.5-1`](#rfc2205-2.5-1) This must not prevent a different receiver from now establishing a smaller reservation Q0 that would succeed if not merged with Q1. (§2.5) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-2-10`](#rfc2205-2-10) When a new reservation is requested, each node must answer two questions: "Are enough resources available to meet this request?" and "Is this user allowed to make this reservation?" These two decisions are termed the "admission control" decision and the "policy control" decision, respectively, and both must be favorable in order for RSVP to make a reservation. (§2) | {gap}, no test | acceptReservation in internal/plugins/rsvpte/reservation.go requires local address-based policy and bandwidth admission before native installation. Authenticated POLICY_DATA interpretation is a separate absent capability; complete requirement-level proof remains open. |
| [`RFC2205-2-11`](#rfc2205-2-11) RSVP must therefore provide correct protocol operation even when two RSVP-capable routers are joined by an arbitrary "cloud" of non-RSVP routers. (§2) | {gap}, no test | receive IP TTL is available, but non-RSVP-cloud detection and state are not implemented; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-2-12`](#rfc2205-2-12) If the destination address does not match any local interface and the message is not a Path or PathTear, the message must be forwarded without further processing by this node. (§2) | {gap}, no test | ResvConf is relayed toward its receiver, but other nonlocal message types lack this forwarding path; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-3-3`](#rfc2205-3-3) If the INTEGRITY object is present, it must immediately follow the common header. (§3) | {gap}, no test | checkObjectPlacement in internal/plugins/rsvpte/message_validation.go enforces this position. Complete requirement-level proof remains open. |
| [`RFC2205-3-4`](#rfc2205-3-4) Path state and reservation state for the same DestAddress and ProtocolId must each have DstPort values that are all zero or all non-zero.  Violation of this condition in a node is a "Conflicting Dest Ports" error.  2.   Destination ports rule.  If DstPort in a session definition is zero, all SrcPort fields used for that session must also be zero.  The assumption here is that the protocol does not have UDP/TCP- like ports.   Violation of this condition in a node is a "Bad Src Ports" error.  3.   Source Ports must be consistent.  A sender host must not send path state both with and without a zero SrcPort. (§3.2) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-3-5`](#rfc2205-3-5) Multicast routing allows a stable distribution tree in which Path messages from the same sender arrive from more than one PHOP, and RSVP must be prepared to maintain all such path state. (§3) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-6`](#rfc2205-3-6) RSVP must not forward (according to the rules of Section 3.9) Path messages that arrive on an incoming interface different from that provided by routing. (§3) | {gap}, no test | PATH processing does not compare the received interface index with the routing-derived incoming interface; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-3-7`](#rfc2205-3-7) The STYLE object followed by the flow descriptor list must occur at the end of the message, and objects within the flow descriptor list must follow the BNF (§3) | {gap}, no test | checkObjectPlacement and checkFlowDescriptors enforce supported FF/SE ordering. Complete requirement-level proof remains open. |
| [`RFC2205-3-8`](#rfc2205-3-8) A FLOWSPEC object can be omitted if it is identical to the most recent such object that appeared in the list; the first FF flow descriptor must contain a FLOWSPEC. (§3) | {gap}, no test | appendFilter in internal/plugins/rsvpte/message_validation.go rejects a first Resv filter without FLOWSPEC. Complete requirement-level proof remains open. |
| [`RFC2205-3-9`](#rfc2205-3-9) Whenever a Resv message with wildcard sender selection is forwarded to more than one previous hop, a SCOPE object must be included in the message (see Section 3.4 below); in this case, the scope for forwarding the reservation is constrained to just the sender IP addresses explicitly listed in the SCOPE object. (§3) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-10`](#rfc2205-3-10) Matching state must have match the SESSION, SENDER_TEMPLATE, and PHOP objects. (§3) | {gap}, no test | handlePathTear in internal/plugins/rsvpte/engine.go now matches tunnel/sender identity and the stored RSVP_HOP before removal. Complete requirement-level proof remains open. |
| [`RFC2205-3-11`](#rfc2205-3-11) A unicast PathTear must not be forwarded if there is path state for the same (session, sender) pair but a different PHOP (§3) | {gap}, no test | handlePathTear now rejects a different stored hop for ordinary and merged tunnel state. Complete requirement-level proof remains open. |
| [`RFC2205-3-16`](#rfc2205-3-16) Matching reservation state must match the SESSION, STYLE, and FILTER_SPEC objects as well as the LIH in the RSVP_HOP object. (§3) | {gap}, no test | handleResvTear in internal/plugins/rsvpte/reservation.go now checks supported tunnel identity, style and the stored RSVP_HOP. Complete requirement-level proof remains open. |
| [`RFC2205-3-21`](#rfc2205-3-21) However, this must not trigger sending a message out the interface through which M arrived (which could happen if the implementation simply triggered an immediate refresh of all state for the session). (§3) | {gap}, no test | immediate forwarding does not exclude the triggering packet's incoming interface; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-3-22`](#rfc2205-3-22) In this version of the spec, each RSVP message must occupy exactly one IP datagram. (§3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux raw IPv4 socket; internal/plugins/rsvpte/transport_linux.go::Send and SendPath issue one SendmsgBuffers call per encoded message, and Linux combines its buffers into one datagram |
| [`RFC2205-3-23`](#rfc2205-3-23) Forwarding of RSVP messages must avoid looping (§3) | {gap}, no test | RRO and hop-limit handling do not supply the missing routing-derived incoming-interface checks; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-3-24`](#rfc2205-3-24) If reservation state from some NHOP does not contain a SCOPE object, a substitute sender list must be created and included in the union. (§3.4) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-25`](#rfc2205-3-25) However, the ResvErr message forwarded out OI must contain a SCOPE object derived from L by including only those senders that route to OI. (§3) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-26`](#rfc2205-3-26) Since RSVP sends periodic refresh messages, it must avoid message synchronization and ensure that any synchronization that may occur is not stable. (§3.7) | {gap}, no test | runRefreshLoop in internal/plugins/rsvpte/register.go uses a fixed ticker without synchronization avoidance; plan/pre-release/spec-rsvp-refresh-timing.md |
| [`RFC2205-3-27`](#rfc2205-3-27) To avoid premature loss of state, L must satisfy L >= (K + 0.5)*1.5*R, where K is a small integer. (§3) | {gap}, no test | expiredPSBs in internal/plugins/rsvpte/fsm.go uses the received period multiplied by the configured factor, without enforcing this floor; plan/pre-release/spec-rsvp-refresh-timing.md |
| [`RFC2205-3-28`](#rfc2205-3-28) Specifically, the ratio of two successive values R2/R1 must not exceed 1 + Slew.Max. (§3) | {gap}, no test | adoptedRefreshPeriod in internal/plugins/rsvpte/register.go adopts the configured period in one step without limiting its ratio; plan/pre-release/spec-rsvp-refresh-timing.md |
| [`RFC2205-3-29`](#rfc2205-3-29) RSVP knows where such points occur and must so indicate to the traffic control mechanism. (§3) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-30`](#rfc2205-3-30) RSVP must also test for the presence of non-RSVP hops in the path and pass this information to traffic control. (§3) | {gap}, no test | Ze holds no RSVP-capability knowledge of a neighbor or a hop; plan/spec-rsvpte-non-rsvp-neighbors.md |
| [`RFC2205-3-31`](#rfc2205-3-31) For example, if the routing protocol uses IP encapsulating tunnels, then the routing protocol must inform RSVP when non-RSVP hops are included. (§3) | {gap}, no test | Ze holds no RSVP-capability knowledge of a neighbor or a hop; plan/spec-rsvpte-non-rsvp-neighbors.md |
| [`RFC2205-3-32`](#rfc2205-3-32) The RSVP process must be aware of the default, and if an application sets a specific interface, it must also pass that information to RSVP. o Sending Data (§3) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-3-33`](#rfc2205-3-33) The RSVP process must determine which case holds by examining the path state, to decide which incoming interface to use for sending Resv messages. (§3) | {gap}, no test | PSB retains PHOP but not the received interface index, and RESV relies on a route lookup toward PHOP; plan/pre-release/spec-rsvp-routing-and-interface-integration.md |
| [`RFC2205-3-34`](#rfc2205-3-34) The path state on Iapp should only match a reservation from the local application; it must be marked "Local_only" by the RSVP process. (§3) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-3.9-1`](#rfc2205-3.9-1) When Path and PathTear messages are forwarded, path state marked "Local_Only" must be ignored. (§3.9) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-3-35`](#rfc2205-3-35) The original order of such unknown-class objects need not be retained; however, the message that is forwarded must obey the general order requirements for its message type. (§3.10) | {gap}, no test | receive-side placement checks now exist in message_validation.go; full proof of forwarded unknown-object placement remains open. |
| [`RFC2205-3-36`](#rfc2205-3-36) At each such replication point, RSVP must merge reservation requests from the corresponding next hops by computing the "maximum" of their flowspecs. (§3) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-3-38`](#rfc2205-3-38) RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses (§3) | {gap}, no test | rawTransport.LocalAddresses in internal/plugins/rsvpte/transport_linux.go joins native addresses to interface index, state and MTU. Complete requirement-level proof, including virtual interfaces, remains open. |
| [`RFC2205-3-43`](#rfc2205-3-43) In order to manipulate these objects, RSVP process must have available to it the following service-dependent routines. (§3) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-4-1`](#rfc2205-4-1) This field must be non-zero. (§A) | {gap}, no test | decodeSessionIPv4 in internal/plugins/rsvpte/wire.go rejects an unspecified tunnel endpoint. Complete requirement-level proof remains open. |
| [`RFC2205-4-2`](#rfc2205-4-2) This field must be non-zero. (§A) | {gap}, no test | Ze reserves for LSP tunnels, not IntServ flows; plan/spec-rsvp-intserv-host-flows.md |
| [`RFC2205-4-3`](#rfc2205-4-3) The addresses must be listed in ascending numerical order. (§A) | {gap}, no test | Ze signals unicast LSPs with FF and SE only; plan/spec-rsvp-multicast-wildcard.md |
| [`RFC2205-4-5`](#rfc2205-4-5) However, some important classes of host systems may not support raw network I/O.  To use RSVP, such hosts must encapsulate RSVP messages in UDP.  The basic UDP encapsulation scheme makes two assumptions:  1.   All hosts are capable of sending and receiving multicast packets if multicast destinations are to be supported.  2.   The first/last-hop routers are RSVP-capable.  A method of relaxing the second assumption is given later.  Let Hu be a "UDP-only" host that requires UDP encapsulation, and Hr a host that can do raw network I/O.  The UDP encapsulation scheme must allow RSVP interoperation among an arbitrary topology of Hr hosts, Hu hosts, and routers. (§C) | {gap}, no test | Ze fills no host-without-raw-I/O role; plan/spec-rsvp-udp-encapsulation.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2205-3.1-1`](#rfc2205-3.1-1)

Protocol version number.  This is version 1. (§3.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-10-08 against RFC 2205 Section 3.1.1 ('Protocol version number.  This is version 1.'). DecodeHeader still refuses Version != rsvpVersion (1) with errBadVersion; TestRSVPDecodeHeaderBadVersion feeds Vers 2 and requires the error, TestRSVPHeaderRoundTrip and TestRFC2205EmittedVersionIsOne pin the emitted Vers nibble at 1. Units byte-identical since the 2026-09-29 judgement; only the files around them moved. Both polarities hold.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRSVPDecodeHeaderBadVersion`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L443) | unit/verify | revert, verified |
| positive | [`TestRFC2205EmittedVersionIsOne`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestRSVPHeaderRoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L409) | unit/verify | revert, verified |

### [`RFC2205-3.1-2`](#rfc2205-3.1-2)

All unused fields should be sent as zero and ignored on receipt. (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-10-08 against RFC 2205 Appendix A ('All unused fields should be sent as zero and ignored on receipt.'). Single-polarity row, both clauses covered: send, TestRSVPReservedByteZeroOnSend requires raw[5]==0 in a built PATH, plus the Flags nibble sent zero in TestRFC2205UnusedHeaderFieldsIgnoredOnReceipt; receive, the same unit feeds Reserved 0xA5 and all four Flags bits set, re-checksummed, and the egress still installs state and answers with a RESV. Units byte-identical; covers the common-header unused fields only.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2205UnusedHeaderFieldsIgnoredOnReceipt`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRSVPReservedByteZeroOnSend`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L585) | unit/verify | revert, verified |

### [`RFC2205-3.1.2-1`](#rfc2205-3.1.2-1)

A 16-bit field containing the total object length in bytes.  Must always be a multiple of 4 (§3.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-10-08 against RFC 2205 Section 3.1.2 ('Must always be a multiple of 4, and at least 4.'). The unit change since the last judgement (9b8bfe250c) only passes nil for buildPathErr's new adspec argument; the assertions are untouched and the PathErr walked carries the same object set as before. Send: TestRSVPObjectLengthMultipleOfFour errors on objLen%4 != 0 over every object of a built PATH, RESV and PathErr. Receive: decodeObjectHeader refuses Length%4 != 0 with errBadObjLen and TestRFC2205ReceivedObjectLengthNotMultipleOfFourRejected requires it for a Length-6 object. An ADSPEC in a PathErr is copied raw from a PATH that already passed that decode check. The positive tag's prose ('Decode does not enforce %4') is still stale.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205ReceivedObjectLengthNotMultipleOfFourRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestRSVPObjectLengthMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L606) | unit/verify | unproven |

### [`RFC2205-3.1.3-1`](#rfc2205-3.1.3-1)

Similarly, each node is required to verify the correct construction of each RSVP message it receives.  Should a programming error allow an RSVP to create a malformed message, the error is not generally reported to end systems in an ERROR_SPEC object; instead, the error is simply logged locally, and perhaps reported through network management mechanisms. (§B)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Verify clause: TestDecodeMandatoryObjects and the engine TIME_VALUES pair. Not-in-ERROR_SPEC and logged-locally clauses: TestRFC2205MalformedMessageLoggedLocally asserts a WARN "decode failed" record naming the source and the error, nothing sent, no state; TestRFC2205WellFormedMessageNotLogged asserts no WARN for a conforming PATH.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_mandatory_test.go#L98) | unit/verify | mutant, verified |
| negative | [`TestEnginePathWithoutTimeValuesDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_mandatory_test.go#L120) | unit/verify | revert, verified |
| negative | [`TestRFC2205MalformedMessageLoggedLocally`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L196) | unit/verify | revert, verified |
| positive | [`TestDecodeMandatoryObjects`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_mandatory_test.go#L90) | unit/verify | mutant, verified |
| positive | [`TestEnginePathWithTimeValuesAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_mandatory_test.go#L139) | unit/verify | revert, verified |
| positive | [`TestRFC2205WellFormedMessageNotLogged`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L212) | unit/verify | revert, verified |

### [`RFC2205-3.1.1-1`](#rfc2205-3.1.1-1)

An all-zero value means that no checksum was transmitted. (§3.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Positive TestRFC2205ZeroChecksumMeansNoneTransmitted: an all-zero checksum over a body that would fail verification is accepted at handlePacket (PSB installed, PATH relayed). Negative TestRFC2205NonzeroChecksumStillVerified bounds the rule: a nonzero failing checksum is dropped. Records break the whole DecodeMessage; a targeted mutant of the zero test would be sharper.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205NonzeroChecksumStillVerified`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L533) | unit/verify | revert, verified |
| positive | [`TestRFC2205ZeroChecksumMeansNoneTransmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L515) | unit/verify | revert, verified |

### [`RFC2205-x-1`](#rfc2205-x-1)

RSVP must be able to cause Path, PathTear, and ResvConf message to be sent with the Router Alert IP option. (§3.11.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Positive TestIntegrationRSVPPathCarriage proves SendPath writes Router Alert. Negatives: TestRFC2205PathNeverLeavesWithoutRouterAlertCarrier for Path/PathTear and TestRFC2205ResvConfNeverLeavesWithoutRouterAlertCarrier for both the originated and the relayed ResvConf going through SendPath, closing the vacuous ResvConf arm.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathNeverLeavesWithoutRouterAlertCarrier`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L14) | unit/verify | revert, verified |
| negative | [`TestRFC2205ResvConfNeverLeavesWithoutRouterAlertCarrier`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L400) | unit/verify | revert, verified |
| positive | [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L390) | unit/verify | revert, verified |

### [`RFC2205-3.10-1`](#rfc2205-3.10-1)

Class-Num = 0bbbbbbb The entire message should be rejected and an "Unknown Object Class" error returned. (§3.10)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-10-08 against RFC 2205 Section 3.10 ('Class-Num = 0bbbbbbb The entire message should be rejected and an "Unknown Object Class" error returned.'). classifyUnknownClass still rejects a 0bbbbbbb class ze does not know; the 1d457b0d52 change to DecodeMessage skips only SENDER_TSPEC and ADSPEC in a PathTear (pathTearIgnored), both known classes, so it does not reach this path. Reject and error clauses at the engine: TestRFC2205UnknownClassPathRejectedWithError (no PSB, no RESV, PathErr code 13, value Class<<8|C-Type to the PHOP) and TestRFC2205UnknownClassResvRejectedWithError (no relay, no RSB, ResvErr 13). Decode-level cases in TestDecodeUnknownObjectClass and the ignorable-class positive in TestEnginePathWithIgnorableObjectAccepted are byte-identical.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEnginePathWithIgnorableObjectAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/frr_test.go#L949) | unit/verify | revert, verified |
| negative | [`TestRFC2205UnknownClassPathRejectedWithError`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L230) | unit/verify | revert, verified |
| negative | [`TestRFC2205UnknownClassResvRejectedWithError`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L250) | unit/verify | revert, verified |
| negative | [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L731) | unit/verify | revert, verified |
| positive | [`TestDecodeUnknownObjectClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_test.go#L724) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Positive TestRFC2205ResvFollowsEachSendersReversePath: two senders of one session through two PHOPs each get their RESV at their own PHOP, and a transit relays the RESV on to the ingress. Negative TestRFC2205ResvNeverTakesAnotherSendersPath asserts neither sender's reservation leaves on the other's hop. Both records observed red (sendResv, handlePathEgress).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205ResvWithoutPathStateNotRelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L105) | unit/verify | mutant, verified |
| negative | [`TestRFC2205ResvNeverTakesAnotherSendersPath`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L392) | unit/verify | revert, verified |
| positive | [`TestRFC2205ResvSentToPreviousHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestRFC2205ResvFollowsEachSendersReversePath`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L376) | unit/verify | revert, verified |

### [`RFC2205-2-2`](#rfc2205-2-2)

Resv messages must finally be delivered to the sender hosts themselves (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205ResvForUnknownSenderIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L139) | unit/verify | mutant, verified |
| positive | [`TestRFC2205ResvDeliveredToSender`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L122) | unit/verify | mutant, verified |

### [`RFC2205-2-3`](#rfc2205-2-3)

A Path message is required to carry a Sender Template (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathWithoutSenderTemplateDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L80) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathCarriesSenderTemplate`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L61) | unit/verify | revert, verified |

### [`RFC2205-2-4`](#rfc2205-2-4)

A Path message is required to carry a Sender Tspec (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathWithoutSenderTSpecDropped`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L121) | unit/verify | revert, verified |
| negative | [`TestRFC2205PathWithoutSenderTSpecRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_path_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathCarriesSenderTSpec`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L71) | unit/verify | revert, verified |

### [`RFC2205-2-5`](#rfc2205-2-5)

If this update results in modification of state to be forwarded in refresh messages, these refresh messages must be generated and forwarded immediately, so that state changes can be propagated end-to-end without delay. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-5, so no unit is bound to it.

### [`RFC2205-2.3-1`](#rfc2205-2.3-1)

RSVP soft state is created and periodically refreshed by Path and Resv messages. The state is deleted if no matching refresh messages arrive before the expiration of a "cleanup timeout" interval. (§2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Refresh: TestRFC2205RefreshRecursEachTick (three refreshPaths ticks, three PATHs) and TestRefreshResendsPathAndResv (RESV re-sent). Deletion: TestRFC2205PathStateDeletedAtCleanupTimeout (PSB) and TestRFC2205ResvStateDeletedAtCleanupTimeout (transit RSB dropped with ResvTear, ingress RSB dropped Up->PathSent, PSB kept). Negatives keep refreshed PSB and RSB. D-1 producer read: cleanupTick withdraws expiredRSBs through removeReservation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRefreshDoesNotStampEgressPSB`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_test.go#L21) | unit/verify | revert, verified |
| negative | [`TestRFC2205RefreshedPathStateKept`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L81) | unit/verify | revert, verified |
| negative | [`TestRFC2205RefreshedResvStateKept`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L167) | unit/verify | revert, verified |
| positive | [`TestRefreshResendsPathAndResv`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_test.go#L56) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathStateDeletedAtCleanupTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L68) | unit/verify | revert, verified |
| positive | [`TestRFC2205RefreshRecursEachTick`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L54) | unit/verify | revert, verified |
| positive | [`TestRFC2205ResvStateDeletedAtCleanupTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L128) | unit/verify | revert, verified |

### [`RFC2205-2-6`](#rfc2205-2-6)

Conversely, state that is forwarded out interface I* must be computed using only state that arrived on interfaces different from I*. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-6, so no unit is bound to it.

### [`RFC2205-2-7`](#rfc2205-2-7)

Once initiated, a teardown request must be forwarded hop-by-hop without delay (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathTearWithoutStateNotRelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L181) | unit/verify | revert, verified |
| positive | [`TestRFC2205TransitPathTearRelayedWithoutDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L166) | unit/verify | revert, verified |

### [`RFC2205-2-8`](#rfc2205-2-8)

Since a request that fails may be the result of merging a number of requests, a reservation error must be reported to all of the responsible receivers. (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205ResvErrForOtherSenderNotRelayed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC2205ResvErrRelayedToReceiver`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L105) | unit/verify | revert, verified |

### [`RFC2205-2-9`](#rfc2205-2-9)

The blockade state in each downstream router must not remove the state or prevent its immediate refresh. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2-9, so no unit is bound to it.

### [`RFC2205-2.5-1`](#rfc2205-2.5-1)

This must not prevent a different receiver from now establishing a smaller reservation Q0 that would succeed if not merged with Q1. (§2.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-2.5-1, so no unit is bound to it.

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

An RSVP implementation must recognize the following classes: (§3.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 (routing child). The missing ERROR_SPEC is now proven: TestRFC2205ErrorSpecClassRecognized decodes a PathErr with no unknown object and the error node, code and value intact. Other listed classes are covered by TestRFC2205KnownClassesRecognized and the NULL tests; negative TestRFC2205UnlistedClassNotRecognized. Class numbers are compared through the package constants, as for every other class in this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205NullObjectNotRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L107) | unit/verify | revert, verified |
| negative | [`TestRFC2205UnlistedClassNotRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_wire_test.go#L124) | unit/verify | revert, verified |
| positive | [`TestRFC2205ErrorSpecClassRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestRFC2205NullObjectRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/repairs_rfc2205_test.go#L94) | unit/verify | revert, verified |
| positive | [`TestRFC2205KnownClassesRecognized`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_wire_test.go#L64) | unit/verify | mutant, verified |

### [`RFC2205-3-2`](#rfc2205-3-2)

The IP source address of a Path message must be an address of the sender it describes, while the destination address must be the DestAddress for the session. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a PATH whose IP source is not the sender, or whose IP destination is not the session DestAddress. TestRFC2205PathIPAddressesAreSenderAndSession requires Source == sender and Destination == tunnel endpoint for an originated and a relayed PATH; TestRFC2205PathIPAddressesNeverHopAddresses requires neither to be a hop address.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathIPAddressesNeverHopAddresses`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L106) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathIPAddressesAreSenderAndSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L93) | unit/verify | revert, verified |

### [`RFC2205-3-3`](#rfc2205-3-3)

If the INTEGRITY object is present, it must immediately follow the common header. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-3, so no unit is bound to it.

### [`RFC2205-3-4`](#rfc2205-3-4)

Path state and reservation state for the same DestAddress and ProtocolId must each have DstPort values that are all zero or all non-zero.  Violation of this condition in a node is a "Conflicting Dest Ports" error.  2.   Destination ports rule.  If DstPort in a session definition is zero, all SrcPort fields used for that session must also be zero.  The assumption here is that the protocol does not have UDP/TCP- like ports.   Violation of this condition in a node is a "Bad Src Ports" error.  3.   Source Ports must be consistent.  A sender host must not send path state both with and without a zero SrcPort. (§3.2)

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). TestRFC2205PathTearRouteEqualsPathRoute compares the whole PathRoute of the relayed and the ingress-originated PathTear with the PATH's, closing both earlier gaps. Negative TestRFC2205PathTearNotAddressedHopByHop: a PathTear sent hop-by-hop through Send would not follow the PATH route, which is this rule.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathTearNotAddressedHopByHop`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L132) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathTearRoutedLikePath`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_carrier_test.go#L118) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathTearRouteEqualsPathRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L277) | unit/verify | revert, verified |

### [`RFC2205-3.1.5-1`](#rfc2205-3.1.5-1)

Therefore, its IP destination address must be the session DestAddress, and its IP source address must be the sender address from the path state being torn down. (§3.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Positive: relayed and ingress-originated PathTear both carry IP dst = session endpoint and IP src = sender address. Negative TestRFC2205PathTearSourceNotTakenFromCarrier: a PathTear arriving from another source is still relayed from the sender address, not the carrier or the router ID.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathTearSourceNotTakenFromCarrier`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L324) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathTearAddressedFromSenderToSession`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L300) | unit/verify | revert, verified |

### [`RFC2205-3-13`](#rfc2205-3-13)

A PathTear message may include a SENDER_TSPEC or ADSPEC object in its sender descriptor, but these must be ignored. (§3.1.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathTearTSpecNotUsed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L181) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathTearObjectsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L152) | unit/verify | revert, verified |

### [`RFC2205-3.1.6-1`](#rfc2205-3.1.6-1)

A ResvTear message may include a SCOPE object, but it must be ignored. (§3.1.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). Positive: a ResvTear with a SCOPE naming the sender tears the RSB, keeps the PSB, draws no ResvErr and goes upstream. Negative TestRFC2205ResvTearScopeNotObeyed: a SCOPE that lists only another sender does not narrow the tear, so a node obeying SCOPE goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205ResvTearScopeNotObeyed`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L481) | unit/verify | revert, verified |
| positive | [`TestRFC2205ResvTearWithScopeTearsDown`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L455) | unit/verify | revert, verified |

### [`RFC2205-3-14`](#rfc2205-3-14)

Deletion of path state as the result of a PathTear message or a timeout must also adjust related reservation state as required to maintain consistency in the local node. (§3.1.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Judged 2026-09-29 (routing child, continuation judge). PathTear clause: TestRFC2205PathTearReleasesReservation / ...ForOtherLSPLeavesReservation. Timeout clause: TestRFC2205PathStateDeletedAtCleanupTimeout asserts the reservation bandwidth released with the expired path state; TestRFC2205RefreshedPathStateKept keeps it for refreshed state.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205PathTearForOtherLSPLeavesReservation`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L217) | unit/verify | revert, verified |
| negative | [`TestRFC2205RefreshedPathStateKept`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestRFC2205PathTearReleasesReservation`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_engine_test.go#L206) | unit/verify | mutant, verified |
| positive | [`TestRFC2205PathStateDeletedAtCleanupTimeout`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_softstate_timeout_test.go#L69) | unit/verify | revert, verified |

### [`RFC2205-3-16`](#rfc2205-3-16)

Matching reservation state must match the SESSION, STYLE, and FILTER_SPEC objects as well as the LIH in the RSVP_HOP object. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-16, so no unit is bound to it.

### [`RFC2205-3-17`](#rfc2205-3-17)

A ResvTear message must be routed like the corresponding Resv message (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205ResvTearForOtherHopNotRouted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L262) | unit/verify | revert, verified |
| positive | [`TestRFC2205ResvTearRoutedLikeResv`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L236) | unit/verify | revert, verified |

### [`RFC2205-3-18`](#rfc2205-3-18)

Each flow descriptor in a FF-style Resv message must be processed independently, and a separate ResvErr message must be generated for each one that is in error. (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205FixedFilterGoodDescriptorKept`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L308) | unit/verify | revert, verified |
| positive | [`TestRFC2205FixedFilterErrorPerDescriptor`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L282) | unit/verify | revert, verified |

### [`RFC2205-3-19`](#rfc2205-3-19)

This ResvErr message must contain the information required to define the error and to route the error message in later hops. (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205ResvErrWrongStyleNotRouted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L368) | unit/verify | revert, verified |
| positive | [`TestRFC2205ResvErrCarriesErrorAndRoute`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L339) | unit/verify | revert, verified |

### [`RFC2205-3-20`](#rfc2205-3-20)

If the error is an admission control failure while attempting to increase an existing reservation, then the existing reservation must be left in place and the InPlace flag bit must be on in the ERROR_SPEC of the ResvErr message. (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205FailedFirstReservationNotInPlace`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L420) | unit/verify | revert, verified |
| positive | [`TestRFC2205FailedIncreaseLeavesReservationInPlace`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_resv_error_test.go#L384) | unit/verify | revert, verified |

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

If reservation state from some NHOP does not contain a SCOPE object, a substitute sender list must be created and included in the union. (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-24, so no unit is bound to it.

### [`RFC2205-3-25`](#rfc2205-3-25)

However, the ResvErr message forwarded out OI must contain a SCOPE object derived from L by including only those senders that route to OI. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-25, so no unit is bound to it.

### [`RFC2205-3-26`](#rfc2205-3-26)

Since RSVP sends periodic refresh messages, it must avoid message synchronization and ensure that any synchronization that may occur is not stable. (§3.7)

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

### [`RFC2205-3.9-1`](#rfc2205-3.9-1)

When Path and PathTear messages are forwarded, path state marked "Local_Only" must be ignored. (§3.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3.9-1, so no unit is bound to it.

### [`RFC2205-3-35`](#rfc2205-3-35)

The original order of such unknown-class objects need not be retained; however, the message that is forwarded must obey the general order requirements for its message type. (§3.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-35, so no unit is bound to it.

### [`RFC2205-3-36`](#rfc2205-3-36)

At each such replication point, RSVP must merge reservation requests from the corresponding next hops by computing the "maximum" of their flowspecs. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-36, so no unit is bound to it.

### [`RFC2205-3-37`](#rfc2205-3-37)

To forward Path and PathTear messages, an RSVP process must be able to query the routing process(s) for routes. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity capability. Forbidden: forwarding PATH or PathTear without a routing query. TestIntegrationRSVPPathCarriage sends PATH and PathTear through SendPath, which calls RouteGetWithOptions, and fails unless the frame's L2 destination is the explicit hop; TestIntegrationRSVPPathAvoidsDataFEC resolves through ResolveRoute and asserts the PATH reaches the selected hop.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationRSVPPathAvoidsDataFEC`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L478) | unit/verify | unproven |
| positive | [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L391) | unit/verify | unproven |

### [`RFC2205-3-38`](#rfc2205-3-38)

RSVP must be able to learn what real and virtual interfaces are active, with their IP addresses (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-38, so no unit is bound to it.

### [`RFC2205-3-39`](#rfc2205-3-39)

Packets received for IP protocol 46 but not addressed to the node must be diverted to the RSVP program for processing, without being forwarded (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationRSVPPathAvoidsDataFEC`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L479) | unit/verify | unproven |

### [`RFC2205-3-40`](#rfc2205-3-40)

On a router or multi-homed host, the identity of the interface (real or virtual) on which a diverted message is received, as well as the IP source address and IP TTL with which it arrived, must also be available to the RSVP process. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity capability. Forbidden: the RSVP process not seeing arrival interface, IP source or IP TTL of a diverted message. TestIntegrationRSVPPathCarriage fails unless packet.Src, Dst, TTL and IfIndex match the sent route, TTL and receiving link, for a destination not local to the receiver.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L392) | unit/verify | unproven |

### [`RFC2205-3-41`](#rfc2205-3-41)

RSVP must be able to force a (multicast) datagram to be sent on a specific outgoing real or virtual link, bypassing the normal routing mechanism. (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity capability. Forbidden: a PATH following the ordinary route instead of the forced link. TestIntegrationRSVPPathCarriage fails unless the frame's L2 destination is link 0's peer while the endpoint's ordinary route uses link 1, as the Send control on link 1 shows.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L393) | unit/verify | unproven |

### [`RFC2205-3-42`](#rfc2205-3-42)

RSVP must be able to specify the IP source address and IP TTL to be used when sending Path messages (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIntegrationRSVPPathCarriage`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_transport_integration_linux_test.go#L394) | unit/verify | unproven |

### [`RFC2205-3-43`](#rfc2205-3-43)

In order to manipulate these objects, RSVP process must have available to it the following service-dependent routines. (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-3-43, so no unit is bound to it.

### [`RFC2205-4-1`](#rfc2205-4-1)

This field must be non-zero. (§A)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-1, so no unit is bound to it.

### [`RFC2205-4-2`](#rfc2205-4-2)

This field must be non-zero. (§A)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-2, so no unit is bound to it.

### [`RFC2205-4-3`](#rfc2205-4-3)

The addresses must be listed in ascending numerical order. (§A)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-3, so no unit is bound to it.

### [`RFC2205-4-4`](#rfc2205-4-4)

Similarly, each node is required to verify the correct construction of each RSVP message it receives. (§B)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged 2026-09-29 (routing child). Each message type is now covered: TestRFC2205EachMessageTypeConstructionAccepted decodes a conformant PathTear, ResvTear, PathErr, ResvErr and ResvConf; TestRFC2205EachMessageTypeMalformedRefused omits each unbracketed BNF object in turn, asserts DecodeMessage refuses it (errObjectAbsent except STYLE), and that a transit with matching path state sends nothing and keeps its state. PATH and RESV are covered by the existing units.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2205EachMessageTypeMalformedRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L209) | unit/verify | revert, verified |
| negative | [`TestRFC2205MissingRequiredObjectRefused`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_wire_test.go#L142) | unit/verify | revert, verified |
| negative | [`TestRFC3209ObjectsAcceptedInAnyOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L280) | unit/verify | mutant, verified |
| positive | [`TestRFC2205EachMessageTypeConstructionAccepted`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/literals_rfc_test.go#L200) | unit/verify | revert, verified |
| positive | [`TestRFC2205WellFormedMessageVerified`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/rfc2205_wire_test.go#L133) | unit/verify | revert, verified |
| positive | [`TestRFC3209ObjectsAcceptedInAnyOrder`](https://github.com/ze-software/ze/blob/main/internal/plugins/rsvpte/wire_rfc3209_test.go#L249) | unit/verify | mutant, verified |

### [`RFC2205-4-5`](#rfc2205-4-5)

However, some important classes of host systems may not support raw network I/O.  To use RSVP, such hosts must encapsulate RSVP messages in UDP.  The basic UDP encapsulation scheme makes two assumptions:  1.   All hosts are capable of sending and receiving multicast packets if multicast destinations are to be supported.  2.   The first/last-hop routers are RSVP-capable.  A method of relaxing the second assumption is given later.  Let Hu be a "UDP-only" host that requires UDP encapsulation, and Hr a host that can do raw network I/O.  The UDP encapsulation scheme must allow RSVP interoperation among an arbitrary topology of Hr hosts, Hu hosts, and routers. (§C)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2205-4-5, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-27 |
| Register | prose |
| Source | rfc/full/rfc2205.txt |
| Source fingerprint | 81606f8d5072deae |
| Record | rfc/extraction/rfc2205.json |
| Mapped sentences | 71 |
| Declined as scope | 52 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 9 | walked | not stated |
| `2` | not stated | 23 | walked | not stated |
| `3` | not stated | 77 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `A` | Appendix A | 4 | walked | Appendix A. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of section 4, where its 4 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `B` | Appendix B | 5 | walked | Appendix B. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of section 4, where its 5 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `C` | Appendix C | 4 | walked | Appendix C. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of section 4, where its 4 site(s) were walked; every decision is carried forward by its verbatim quote. |
| `D` | Appendix D | 1 | walked | Appendix D. Until 2026-09-27 the heading reader did not read this appendix's heading, so its text was read as part of section 4, where its 1 site(s) were walked; every decision is carried forward by its verbatim quote. |

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
| `3:63` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | advice to whoever defines a new object class, about examining scaling limitations before deployment | Forwarding objects with unknown class enables incremental deployment of new objects; however, the scaling limitations of doing so must be carefully examined before a new object class is deployed with both high bits on. |
| `3:64` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | restates why merging is required, which the Section 1 merging row carries | Merging of RSVP reservations is required because of multicast data delivery, which replicates data packets for delivery to different next-hop nodes. |
| `3:66` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same merging rule for reservations on corresponding outgoing interfaces | In this case, RSVP must merge the reservations that are in place on the corresponding outgoing interfaces in order to forward a request upstream. |
| `3:67` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same merging rule for several next hops on one outgoing interface | In these cases, RSVP must merge the reservations from the different next hops in order to make the reservation on the single outgoing interface. |
| `3:68` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same merging rule for forwarding a request upstream | It must also merge reservations requests from all outgoing interfaces in order to forward a request upstream. |
| `3:69` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | a statement that these complexities do not change the protocol processing required | In general, these complexities do not impact the protocol processing that is required by RSVP, except to determine exactly what reservation requests need to be merged. |
| `3:76` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the interface requirement behind the Router Alert obligation the row already carries | RSVP must be able to cause Path, PathTear, and ResvConf message to be sent with the Router Alert IP option. 3.11.6 Service-Dependent Manipulations |
| `A:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the bare 'Must be non-zero.' of a later object field description in the same appendix, repeating the non-zero field constraint | Must be non-zero. |
| `B:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the description of an error code value: what Error Code 2 means, with credentials named as an example | Reservation or path message has been rejected for administrative reasons, for example, required credentials not submitted, insufficient quota or balance, or administrative preemption. |
| `B:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the description of an error code value carrying an API error, not an obligation on the protocol | Error Value field contains an API error code, for an API error that was detected asynchronously and must be reported via an upcall. o Error Code = 21: Traffic Control Error |
| `B:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | one bullet of the list of construction faults the verification at site 4:7 detects | o Required object class (specify) missing |
| `B:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | two more bullets of the same list of construction faults | o Violation of required object order o Flow descriptor count wrong for style or message type |
| `C:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the interoperation property of the same UDP encapsulation scheme | The UDP encapsulation scheme must allow RSVP interoperation among an arbitrary topology of Hr hosts, Hu hosts, and routers. |
| `C:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | a TTL detail of the same UDP encapsulation scheme | Here Ta must be the TTL to exactly reach R. |
| `C:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the configuration detail of the same UDP encapsulation scheme | The host Hu must be explicitly configured with Ra and Ta. |
| `D:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the definition of an RSVP session: one simplex unicast or multicast data flow | An RSVP session defines one simplex unicast or multicast data flow for which reservations are required. |

## Superseded

No document obsoletes RFC 2205, so its obligations are stated where they were written.
