# RFC 4555 - IKEv2 Mobility and Multihoming Protocol (MOBIKE)

Unsupported. Every requirement this repository extracted from RFC 4555, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 6 | of 16 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 6 | of 6 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Public status | Unsupported |
| Enrolment | Enrolled |
| Requirements | 16 |
| Gated MUST-level | 6 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 6 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4555.md` |
| Requirement shard | `rfc/requirements/rfc4555.md` |
| RFC text | `rfc/full/rfc4555.txt` |

## Enrolment

Enrolled: IKEv2 Mobility and Multihoming Protocol (MOBIKE): six MUST-level requirements, all {not-applicable} to Ze. Ze does not implement the RFC 4555 MOBIKE extension -- its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path (no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, no address-update handling; grep for MOBIKE/UPDATE_SA_ADDRESSES across internal/component/ike/ finds nothing). RFC4555-x-1 (MOBIKE_SUPPORTED in IKE_AUTH), RFC4555-x-2 (switch to port 4500 for MOBIKE+NAT-T), RFC4555-3.9-1 (NO_NATS_ALLOWED in address-updating messages), RFC4555-3.7-1 (responder copies COOKIE2 verbatim), RFC4555-3.8-1 (no dynamic address updates when not behind NAT), RFC4555-3.8-2 (echo NAT_DETECTION in MOBIKE INFORMATIONAL) all govern MOBIKE behaviors Ze does not implement. No SHOULD/MAY requirements are gated.

## What the public ledger says

**Status:** Unsupported

**What the ledger says is covered:**

- None
- the IKE engine does not define the MOBIKE notify types (16396/16400) and does not handle UPDATE_SA_ADDRESSES.


**What the ledger says remains:**

Responder role (announce MOBIKE_SUPPORTED, accept UPDATE_SA_ADDRESSES, migrate XFRM endpoints) is not implemented.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **6** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (6):** [`RFC4555-x-1`](#rfc4555-x-1), [`RFC4555-x-2`](#rfc4555-x-2), [`RFC4555-3.9-1`](#rfc4555-3.9-1), [`RFC4555-3.7-1`](#rfc4555-3.7-1), [`RFC4555-3.8-1`](#rfc4555-3.8-1), [`RFC4555-3.8-2`](#rfc4555-3.8-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4555-x-1` | Both peers MUST include N(MOBIKE_SUPPORTED) in IKE_AUTH to enable MOBIKE for that IKE SA (Capability Negotiation, Sections 3.1-3.2) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |
| `RFC4555-x-2` | Implementations supporting both MOBIKE and NAT Traversal MUST switch to port 4500 during IKE_AUTH even if no NAT is detected (Capability Negotiation, Sections 3.1-3.2) | MUST | x | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |
| `RFC4555-3.9-1` | When NAT Traversal is NOT enabled, address-updating messages MUST include NO_NATS_ALLOWED containing actual source/destination IP and ports (NAT Prohibition, Section 3.9) | MUST | 3.9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |
| `RFC4555-3.7-1` | The exchange responder MUST copy COOKIE2 verbatim into the response (Return Routability Check, Section 3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |
| `RFC4555-3.8-1` | When MOBIKE is active, the host not behind a NAT MUST NOT use dynamic IKEv2 address updates for IKE packets (NAT Mapping Changes, Section 3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |
| `RFC4555-3.8-2` | The responder MUST echo NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP if present in any INFORMATIONAL request (NAT Mapping Changes, Section 3.8) | MUST | 3.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |
| `RFC4555-3.7-2` | Return routability check SHOULD be performed by default (Return Routability Check, Section 3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.8-3` | The initiator behind a NAT SHOULD include NAT detection payloads in DPD messages and compare with previous values (NAT Mapping Changes, Section 3.8) | SHOULD | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.9-2` | The initiator SHOULD retry several times on UNEXPECTED_NAT_DETECTED (NAT Prohibition, Section 3.9) | SHOULD | 3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.11-1` | The responder SHOULD use long timeout intervals (at least 5 minutes for retransmission) to give the initiator time to detect problems and switch paths (Failure Recovery, Section 3.11) | SHOULD | 3.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.12-1` | MOBIKE nodes SHOULD verify that incoming IPsec packets use expected addresses (Dead Peer Detection, Section 3.12) | SHOULD | 3.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.12-2` | Packets from stale addresses SHOULD NOT count as evidence that the peer is alive and synchronized (Dead Peer Detection, Section 3.12) | SHOULD NOT | 3.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-x-3` | SPD cache links to SAD entries should NOT use (remote IP, remote SPI) as the key (Implementation Considerations, Appendix A) | SHOULD NOT | x | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-x-4` | Both peers MAY advertise extra addresses in IKE_AUTH and later in INFORMATIONAL requests (Additional Addresses, Sections 3.4, 3.6) | MAY | x | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.7-3` | Return routability check MAY be omitted in trusted environments (Return Routability Check, Section 3.7) | MAY | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC4555-3.8-4` | The host not behind a NAT MAY use dynamic IKEv2 address updates for ESP packets when MOBIKE is active (NAT Mapping Changes, Section 3.8) | MAY | 3.8 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4555-x-1`](#rfc4555-x-1) Both peers MUST include N(MOBIKE_SUPPORTED) in IKE_AUTH to enable MOBIKE for that IKE SA (Capability Negotiation, Sections 3.1-3.2) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |
| [`RFC4555-x-2`](#rfc4555-x-2) Implementations supporting both MOBIKE and NAT Traversal MUST switch to port 4500 during IKE_AUTH even if no NAT is detected (Capability Negotiation, Sections 3.1-3.2) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |
| [`RFC4555-3.9-1`](#rfc4555-3.9-1) When NAT Traversal is NOT enabled, address-updating messages MUST include NO_NATS_ALLOWED containing actual source/destination IP and ports (NAT Prohibition, Section 3.9) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |
| [`RFC4555-3.7-1`](#rfc4555-3.7-1) The exchange responder MUST copy COOKIE2 verbatim into the response (Return Routability Check, Section 3.7) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |
| [`RFC4555-3.8-1`](#rfc4555-3.8-1) When MOBIKE is active, the host not behind a NAT MUST NOT use dynamic IKEv2 address updates for IKE packets (NAT Mapping Changes, Section 3.8) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |
| [`RFC4555-3.8-2`](#rfc4555-3.8-2) The responder MUST echo NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP if present in any INFORMATIONAL request (NAT Mapping Changes, Section 3.8) | no test | no test carries this requirement id; annotated {not-applicable}: Ze does not implement the RFC 4555 MOBIKE (IKEv2 Mobility and Multihoming) extension. Its IKE engine (internal/component/ike/) implements base IKEv2 (RFC 7296) and NAT-Traversal but has no MOBIKE code path -- no N(MOBIKE_SUPPORTED) negotiation, no UPDATE_SA_ADDRESSES exchange, and no address-update handling (grep for MOBIKE / UPDATE_SA_ADDRESSES / MOBIKE_SUPPORTED across internal/component/ike/ finds nothing), so this MOBIKE requirement has no applicable code path. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4555-x-1`](#rfc4555-x-1)

Both peers MUST include N(MOBIKE_SUPPORTED) in IKE_AUTH to enable MOBIKE for that IKE SA (Capability Negotiation, Sections 3.1-3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4555-x-1, so no unit is bound to it.

### [`RFC4555-x-2`](#rfc4555-x-2)

Implementations supporting both MOBIKE and NAT Traversal MUST switch to port 4500 during IKE_AUTH even if no NAT is detected (Capability Negotiation, Sections 3.1-3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4555-x-2, so no unit is bound to it.

### [`RFC4555-3.9-1`](#rfc4555-3.9-1)

When NAT Traversal is NOT enabled, address-updating messages MUST include NO_NATS_ALLOWED containing actual source/destination IP and ports (NAT Prohibition, Section 3.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4555-3.9-1, so no unit is bound to it.

### [`RFC4555-3.7-1`](#rfc4555-3.7-1)

The exchange responder MUST copy COOKIE2 verbatim into the response (Return Routability Check, Section 3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4555-3.7-1, so no unit is bound to it.

### [`RFC4555-3.8-1`](#rfc4555-3.8-1)

When MOBIKE is active, the host not behind a NAT MUST NOT use dynamic IKEv2 address updates for IKE packets (NAT Mapping Changes, Section 3.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4555-3.8-1, so no unit is bound to it.

### [`RFC4555-3.8-2`](#rfc4555-3.8-2)

The responder MUST echo NAT_DETECTION_SOURCE_IP and NAT_DETECTION_DESTINATION_IP if present in any INFORMATIONAL request (NAT Mapping Changes, Section 3.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4555-3.8-2, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 4555, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4555, so its obligations are stated where they were written.
