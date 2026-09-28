# RFC 2516 - A Method for Transmitting PPP Over Ethernet (PPPoE)

Partial. Every requirement this repository extracted from RFC 2516, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 62.2% | 23 of 37 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 24.3% | 9 of 37 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 37 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 42.9% | 27 of 63 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 37 | of 40 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 37 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 37 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 2.7% | 1 of 37 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 37 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 10.8% | 4 of 37 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 31 | of 37 gated MUSTs judged | 13 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 37 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 40 |
| Gated MUST-level | 37 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 63 |
| Tagged units | 63 |
| Recorded audit verdicts | 31 |
| Discrimination records | 27 |
| Summary | `rfc/short/rfc2516.md` |
| Requirement shard | `rfc/requirements/rfc2516.md` |
| RFC text | `rfc/full/rfc2516.txt` |

## Enrolment

Enrolled: PPPoE access-concentrator and client roles: discovery framing and tags, session identity, kernel transport setup, PADT lifetime, and transport-specific LCP negotiation. Requirement bindings come from the tagged tests; the checklist records remaining kernel-path evidence and conditional feature decisions.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Access concentrator (PADI/PADO/PADR/PADS/PADT discovery, AC-Cookie, session tables, AF_PPPOX kernel sessions) and PPPoE client/Host dialer share PPP codecs and option validation. The AC uses the PPP driver
- the client runs its own negotiation loop. Requirement bindings are derived from the tagged tests. The 2026-09-21 PADT lifetime and prohibited-option regressions have been added but not run or discriminated during the concurrent source batch.


**What the ledger says remains**

Four MUST rows carry {gap}. `BuildPADI` limits the PPPoE header and payload to 1484 octets, excluding the Ethernet header; the returned frame can reach 1498 octets and an oversized request returns nil. Tests exercise that boundary and the 16-octet relay-tag reservation. The Linux pppoe module writes session CODE 0x00. Both roles now stop PPP transport on PADT and reject ACCM, ACFC and FCS Alternatives (type 9); verification and discrimination for these changes remain owed. PFC is NOT RECOMMENDED rather than prohibited by Section 7. Remaining annotations below cover kernel resource-allocation evidence and the conditional relay-agent and Generic-Error features proposed in existing specs. Error tags currently carry no data. The session table reserves SESSION_ID 0xffff, and PADO/PADS emit Service-Name even when its value is empty. Receive-side tolerance of absent or repeated Service-Name is documented in [`docs/architecture/l2tp/bng-5-pppoe.md`](https://github.com/ze-software/ze/blob/main/docs/architecture/l2tp/bng-5-pppoe.md); Sections 5.1 and 5.3 bind the sender. No optional-feature decision is implied by the backlog specs.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 23 | one part of the gated population |
| Annotated instead of tested | 14 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **37** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (23):** [`RFC2516-x-1`](#rfc2516-x-1), [`RFC2516-x-2`](#rfc2516-x-2), [`RFC2516-5.2-1`](#rfc2516-5.2-1), [`RFC2516-5.3-1`](#rfc2516-5.3-1), [`RFC2516-x-5`](#rfc2516-x-5), [`RFC2516-5.2-2`](#rfc2516-5.2-2), [`RFC2516-5.2-4`](#rfc2516-5.2-4), [`RFC2516-5.4-1`](#rfc2516-5.4-1), [`RFC2516-5.4-2`](#rfc2516-5.4-2), [`RFC2516-x-7`](#rfc2516-x-7), [`RFC2516-7-1`](#rfc2516-7-1), [`RFC2516-4-1`](#rfc2516-4-1), [`RFC2516-5.1-4`](#rfc2516-5.1-4), [`RFC2516-5.1-5`](#rfc2516-5.1-5), [`RFC2516-5.2-6`](#rfc2516-5.2-6), [`RFC2516-5.3-5`](#rfc2516-5.3-5), [`RFC2516-5.4-3`](#rfc2516-5.4-3), [`RFC2516-5.5-3`](#rfc2516-5.5-3), [`RFC2516-5.5-4`](#rfc2516-5.5-4), [`RFC2516-7-2`](#rfc2516-7-2), [`RFC2516-7-3`](#rfc2516-7-3), [`RFC2516-7-4`](#rfc2516-7-4), [`RFC2516-x-10`](#rfc2516-x-10)

**Annotated instead of tested (14):** [`RFC2516-x-4`](#rfc2516-x-4), [`RFC2516-5.1-1`](#rfc2516-5.1-1), [`RFC2516-5.1-2`](#rfc2516-5.1-2), [`RFC2516-5.3-2`](#rfc2516-5.3-2), [`RFC2516-x-6`](#rfc2516-x-6), [`RFC2516-x-8`](#rfc2516-x-8), [`RFC2516-x-9`](#rfc2516-x-9), [`RFC2516-3-1`](#rfc2516-3-1), [`RFC2516-6-1`](#rfc2516-6-1), [`RFC2516-x-11`](#rfc2516-x-11), [`RFC2516-x-12`](#rfc2516-x-12), [`RFC2516-x-13`](#rfc2516-x-13), [`RFC2516-x-14`](#rfc2516-x-14), [`RFC2516-x-15`](#rfc2516-x-15)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2516-x-1` | The VER field is four bits and MUST be set to 0x1 for this version of the PPPoE specification. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L449). **negative:** `unit/verify` [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L301) |
| `RFC2516-x-2` | The TYPE field is four bits and MUST be set to 0x1 for this version of the PPPoE specification. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L450). **negative:** `unit/verify` [`TestParseBadType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L469) |
| `RFC2516-x-4` | This TAG indicates that there are no further TAGs in the list. The TAG_LENGTH of this TAG MUST always be zero. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestParseEndOfListTag`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L360). **negative:** no negative test. **{single-polarity}:** ze never constructs an End-Of-List tag (builders set the payload length instead, discovery.go:291), and parseTags treats a zero-length End-Of-List as the list terminator (discovery.go:161), so no non-zero-length End-Of-List is ever produced and no negative case exists |
| `RFC2516-5.2-1` | If the Access Concentrator receives this TAG, it MUST include the TAG unmodified in the associated PADO or PADS response. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L124). **positive:** `unit/verify` [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L173). **negative:** `unit/verify` [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L573) |
| `RFC2516-5.3-1` | If a Host receives this TAG, it MUST return the TAG unmodified in the following PADR. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L503). **negative:** `unit/verify` [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L544) |
| `RFC2516-x-5` | If either the Host or Access Concentrator receives this TAG they MUST include it unmodified in any discovery packet they send as a response. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestRelaySessionIDEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L610). **negative:** `unit/verify` [`TestRelaySessionIDNoEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L653) |
| `RFC2516-5.1-1` | The PADI packet MUST contain exactly one TAG of TAG_TYPE Service- Name, indicating the service the Host is requesting, and any number of other TAG types. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L480). **negative:** no negative test. **{single-polarity}:** emit-side count: BuildPADI always writes exactly one Service-Name tag (discovery.go:307). The AC does not enforce the count on receive (MatchServiceName tolerates zero or many, discovery.go:220): this is a deliberate choice, not an omission, per the owner decision of 2026-09-08 recorded in docs/architecture/l2tp/bng-5-pppoe.md ("Ze refuses a PADR with no Service-Name tag and serves a PADI with none"), because Section 5.1 binds the sending host and neither accel-ppp nor FreeBSD enforces this count on receipt either |
| `RFC2516-5.1-2` | The Host sends the PADI packet with the DESTINATION_ADDR set to the broadcast address. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L481). **negative:** no negative test. **{single-polarity}:** emit-side: BuildPADI addresses the PADI to the Ethernet broadcast MAC (discovery.go:305); the receive-side non-broadcast-PADI discard is not enforced (handlePADI checks only SID, server.go:53) |
| `RFC2516-5.2-2` | The PADO packet MUST contain one AC-Name TAG containing the Access Concentrator's name, a Service-Name TAG identical to the one in the PADI, and any number of other Service-Name TAGs indicating other services that the Access Concentrator offers. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L720). **negative:** `unit/verify` [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L721) |
| `RFC2516-5.2-4` | If the Access Concentrator can not serve the PADI it MUST NOT respond with a PADO. (§5.2) | MUST NOT | 5.2 | **positive:** `unit/verify` [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L241). **negative:** `unit/verify` [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L242) |
| `RFC2516-5.3-2` | The PADR packet MUST contain exactly one TAG of TAG_TYPE Service- Name, indicating the service the Host is requesting, and any number of other TAG types. (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L504). **negative:** no negative test. **{single-polarity}:** emit-side count: BuildPADR writes exactly one Service-Name tag (discovery.go:322). The AC's requireServiceNameTag (server.go) now refuses a PADR carrying zero Service-Name tags on receipt, but that enforces PRESENCE, not COUNT: a PADR carrying two Service-Name tags is accepted, first-wins (TestDuplicateServiceNameTagTakesTheFirst), so there is still no receive-side enforcement of "exactly one, not more" to give a negative for this requirement |
| `RFC2516-5.4-1` | The PADS packet contains exactly one TAG of TAG_TYPE Service-Name, indicating the service under which Access Concentrator has accepted the PPPoE session, and any number of other TAG types. (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L174). **negative:** `unit/verify` [`TestPADSAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L777) |
| `RFC2516-5.4-2` | If the Access Concentrator does not like the Service-Name in the PADR, then it MUST reply with a PADS containing a TAG of TAG_TYPE Service-Name-Error (and any number of other TAG types). In this case the SESSION_ID MUST be set to 0x0000. (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestPADRWithoutServiceNameGetsServiceNameError`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L156). **negative:** `unit/verify` [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L175) |
| `RFC2516-x-6` | For Discovery packets, the value is either a unicast or broadcast address as defined in the Discovery section. For PPP session traffic, this field MUST contain the peer's unicast address as determined from the Discovery stage. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestBuildPADT`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L218). **negative:** no negative test. **{single-polarity}:** emit-side: every builder addresses PADO/PADR/PADS/PADT to the peer unicast MAC derived from its unicast source (discovery.go:320, 336, 362, 387); the receive-side broadcast-destination discard is not enforced (ParseDiscovery validates only the source, discovery.go:108) |
| `RFC2516-x-7` | The SOURCE_ADDR field MUST contains the Ethernet MAC address of the source device. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L46). **negative:** `unit/verify` [`TestParseBroadcastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L327). **negative:** `unit/verify` [`TestParseMulticastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L337) |
| `RFC2516-x-8` | If a discovery packet is received with a TAG of unknown TAG_TYPE, the TAG MUST be ignored unless otherwise specified in this document. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestUnknownTagIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L673). **negative:** no negative test. **{single-polarity}:** tolerate requirement: parseTags stores unknown tag types generically and never errors on them (discovery.go:154); rejecting an unknown tag would itself violate the MUST NOT, so no negative exists |
| `RFC2516-x-9` | The Maximum-Receive-Unit (MRU) option MUST NOT be negotiated to a larger size than 1492. (§7) | MUST NOT | 7 | **positive:** `unit/verify` [`TestLCPConfigRequestMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L117). **negative:** no negative test. **{single-polarity}:** ceiling requirement: the client proposes MRU 1492 by default in its LCP Configure-Request (dialer.go:114, session.go:487) and the AC caps MaxMRU at PPPoEMaxMTU 1492 (server.go:203); 1492 is the maximum so there is no negative |
| `RFC2516-7-1` | The SESSION_ID MUST NOT change for that PPPoE session and MUST be the value assigned in the Discovery stage. (§6) | MUST | 6 | **positive:** `unit/verify` [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L12). **negative:** `unit/verify` [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L13) |
| `RFC2516-3-1` | Once a PPP session is established, both the Host and the Access Concentrator MUST allocate the resources for a PPP virtual interface (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** both roles allocate the interface (server.go::handlePADR, pppoeclient/dialer.go::Dial) but no tagged test observes the ppp unit, which only a QEMU run can; plan/pre-release/spec-pppoe-virtual-interface-proof.md |
| `RFC2516-4-1` | A value of 0xffff is reserved for future use and MUST NOT be used (§4) | MUST NOT | 4 | **positive:** `unit/verify` [`TestRFC2516AllocSIDReturnsUsableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L46). **negative:** `unit/verify` [`TestRFC2516SessionID0xffffIsNeverAllocated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L58) |
| `RFC2516-5.1-4` | The CODE field is set to 0x09 and the SESSION_ID MUST be set to 0x0000. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC2516PADISessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L79). **negative:** `unit/verify` [`TestRFC2516PADIWithNonZeroSessionIDIsDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L95) |
| `RFC2516-5.1-5` | An entire PADI packet (including the PPPoE header) MUST NOT exceed 1484 octets so as to leave sufficient room for a relay agent to add a Relay-Session-Id TAG. (§5.1) | MUST NOT | 5.1 | **positive:** `unit/verify` [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padi_length_rfc2516_test.go#L17). **negative:** `unit/verify` [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padi_length_rfc2516_test.go#L18) |
| `RFC2516-5.2-6` | The CODE field is set to 0x07 and the SESSION_ID MUST be set to 0x0000. (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestRFC2516PADOSessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L115). **negative:** `unit/verify` [`TestRFC2516PADOWithNonZeroSessionIDIsNotAnOffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/discovery_rfc2516_test.go#L37) |
| `RFC2516-5.3-5` | The CODE field is set to 0x19 and the SESSION_ID MUST be set to 0x0000. (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestRFC2516PADRSessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L132). **negative:** `unit/verify` [`TestRFC2516PADRWithNonZeroSessionIDIsDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L149) |
| `RFC2516-5.4-3` | The CODE field is set to 0x65 and the SESSION_ID MUST be set to the unique value generated for this PPPoE session. (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestRFC2516PADSCarriesTheSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L178). **negative:** `unit/verify` [`TestRFC2516PADSWithZeroSessionIDIsARefusal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/discovery_rfc2516_test.go#L61) |
| `RFC2516-5.5-3` | The DESTINATION_ADDR field is a unicast Ethernet address, the CODE field is set to 0xa7 and the SESSION_ID MUST be set to indicate which session is to be terminated. (§5.5) | MUST | 5.5 | **positive:** `unit/verify` [`TestRFC2516SessionDownSendsPADTForThatSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L219). **negative:** `unit/verify` [`TestRFC2516UnknownSessionIDTerminatesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L253) |
| `RFC2516-5.5-4` | Even normal PPP termination packets MUST NOT be sent after sending or receiving a PADT (§5.5) | MUST NOT | 5.5 | **positive:** `unit/verify` [`TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padt_lifetime_linux_test.go#L45). **positive:** `unit/verify` [`TestRFC2516ACSentPADTClosesTransportBeforeSend`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padt_lifetime_linux_test.go#L79). **positive:** `unit/verify` [`TestRFC2516ClientPADTStopsOnlyMatchingSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/padt_lifetime_test.go#L29). **positive:** `unit/verify` [`TestRFC2516ClientSentPADTStopsPPPBeforeSend`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/padt_lifetime_test.go#L127). **negative:** `unit/verify` [`TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padt_lifetime_linux_test.go#L46). **negative:** `unit/verify` [`TestRFC2516ClientPADTStopsOnlyMatchingSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/padt_lifetime_test.go#L30) |
| `RFC2516-6-1` | The PPPoE CODE MUST be set to 0x00. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux pppoe (PX_PROTO_OE); internal/component/l2tp/pppoe/kernel_linux.go::pppoeCreate connects the AF_PPPOX socket to the session ID, the peer MAC and the device, and the module writes the CODE byte of every session frame itself, so no value Ze writes decides this field |
| `RFC2516-7-2` | An implementation MUST NOT request any of the following options, and MUST reject a request for such an option: Field Check Sequence (FCS) Alternatives, Address-and-Control-Field-Compression (ACFC), Asynchronous-Control-Character-Map (ACCM) (§7) | MUST NOT | 7 | **positive:** `unit/verify` [`TestRFC2516ACRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc2516_options_test.go#L11). **positive:** `unit/verify` [`TestRFC2516ClientRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L101). **negative:** `unit/verify` [`TestRFC2516ACRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc2516_options_test.go#L12). **negative:** `unit/verify` [`TestRFC2516ClientRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L102) |
| `RFC2516-7-3` | When LCP terminates, the Host and Access concentrator MUST stop using that PPPoE session. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRFC2516SessionDownSendsPADTForThatSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L220). **negative:** `unit/verify` [`TestRFC2516UnknownSessionIDTerminatesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L254) |
| `RFC2516-7-4` | If the Host wishes to start another PPP session, it MUST return to the PPPoE Discovery stage. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestRFC2516HostReturnsToDiscoveryForTheNextSession`](https://github.com/ze-software/ze/blob/main/internal/component/iface/pppoe_client_rfc2516_test.go#L59). **negative:** `unit/verify` [`TestRFC2516HostReturnsToDiscoveryForTheNextSession`](https://github.com/ze-software/ze/blob/main/internal/component/iface/pppoe_client_rfc2516_test.go#L60) |
| `RFC2516-x-10` | All PADI packets MUST guarantee sufficient room for the addition of a Relay-Session-Id TAG with a TAG_VALUE length of 12 octets. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padi_length_rfc2516_test.go#L19). **negative:** `unit/verify` [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padi_length_rfc2516_test.go#L20) |
| `RFC2516-x-11` | A Relay-Session-Id TAG MUST NOT be added if the discovery packet already contains one. (Appendix A, §11) | MUST NOT | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze fills no PPPoE relay agent; plan/spec-pppoe-relay-agent.md |
| `RFC2516-x-12` | This TAG (typically with a zero-length data section) indicates that for one reason or another, the requested Service-Name request could not be honored. If there is data, and the first octet of the data is nonzero, then it MUST be a printable UTF-8 string which explains why the request was denied. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestRFC2516ErrorTagsCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L285). **negative:** no negative test. **{single-polarity}:** discovery.go::BuildPADSError writes the tag with no data, so the conditional never binds and no violating input exists |
| `RFC2516-x-13` | It MAY be included in PADS packets. If there is data, and the first octet of the data is nonzero, then it MUST be a printable UTF-8 string which explains the nature of the error. (Appendix A, §11) | MUST | 11 | **positive:** `unit/verify` [`TestRFC2516ErrorTagsCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L286). **negative:** no negative test. **{single-polarity}:** discovery.go::BuildPADSError writes the tag with no data, so the conditional never binds and no violating input exists |
| `RFC2516-x-14` | It can be added to PADO, PADR or PADS packets when an unrecoverable error occurs and no other error TAG is appropriate. If there is data then it MUST be an UTF-8 string which explains the nature of the error. (Appendix A, §11) | MUST | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze emits no Generic-Error TAG (discovery.go::BuildPADT, BuildPADSError); plan/spec-pppoe-generic-error-tag.md |
| `RFC2516-x-15` | If there is data then it MUST be an UTF-8 string which explains the nature of the error. This string MUST NOT be NULL terminated. (Appendix A, §11) | MUST NOT | 11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze emits no Generic-Error TAG (discovery.go::BuildPADT, BuildPADSError); plan/spec-pppoe-generic-error-tag.md |
| `RFC2516-5.2-5` | The Access Concentrator MAY include this TAG in a PADO packet. (Appendix A, §11) | MAY | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.1-3` | The Host MAY include a Host-Uniq TAG in a PADI or PADR. (Appendix A, §11) | MAY | 11 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.5-1` | This packet may be sent anytime after a session is established to indicate that a PPPoE session has been terminated. It may be sent by either the Host or the Access Concentrator. (§5.5) | MAY | 5.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2516-3-1`](#rfc2516-3-1) Once a PPP session is established, both the Host and the Access Concentrator MUST allocate the resources for a PPP virtual interface (§3) | {gap}, no test | both roles allocate the interface (server.go::handlePADR, pppoeclient/dialer.go::Dial) but no tagged test observes the ppp unit, which only a QEMU run can; plan/pre-release/spec-pppoe-virtual-interface-proof.md |
| [`RFC2516-6-1`](#rfc2516-6-1) The PPPoE CODE MUST be set to 0x00. (§6) | no test | no test carries this requirement id; annotated {lower-layer}: Linux pppoe (PX_PROTO_OE); internal/component/l2tp/pppoe/kernel_linux.go::pppoeCreate connects the AF_PPPOX socket to the session ID, the peer MAC and the device, and the module writes the CODE byte of every session frame itself, so no value Ze writes decides this field |
| [`RFC2516-x-11`](#rfc2516-x-11) A Relay-Session-Id TAG MUST NOT be added if the discovery packet already contains one. (Appendix A, §11) | {gap}, no test | Ze fills no PPPoE relay agent; plan/spec-pppoe-relay-agent.md |
| [`RFC2516-x-14`](#rfc2516-x-14) It can be added to PADO, PADR or PADS packets when an unrecoverable error occurs and no other error TAG is appropriate. If there is data then it MUST be an UTF-8 string which explains the nature of the error. (Appendix A, §11) | {gap}, no test | Ze emits no Generic-Error TAG (discovery.go::BuildPADT, BuildPADSError); plan/spec-pppoe-generic-error-tag.md |
| [`RFC2516-x-15`](#rfc2516-x-15) If there is data then it MUST be an UTF-8 string which explains the nature of the error. This string MUST NOT be NULL terminated. (Appendix A, §11) | {gap}, no test | Ze emits no Generic-Error TAG (discovery.go::BuildPADT, BuildPADSError); plan/spec-pppoe-generic-error-tag.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2516-x-1`](#rfc2516-x-1)

The VER field is four bits and MUST be set to 0x1 for this version of the PPPoE specification. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestBuildFrameVerType asserts BuildPADI writes VER=1; TestParseBadVersion shows a VER=2 frame is refused

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L301) | unit/verify | unproven |
| positive | [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L449) | unit/verify | unproven |

### [`RFC2516-x-2`](#rfc2516-x-2)

The TYPE field is four bits and MUST be set to 0x1 for this version of the PPPoE specification. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestBuildFrameVerType asserts BuildPADI writes TYPE=1; TestParseBadType shows a TYPE=2 frame is refused

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseBadType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L469) | unit/verify | unproven |
| positive | [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L450) | unit/verify | unproven |

### [`RFC2516-x-4`](#rfc2516-x-4)

This TAG indicates that there are no further TAGs in the list. The TAG_LENGTH of this TAG MUST always be zero. (Appendix A, §11)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestParseEndOfListTag proves only the first quoted sentence (a zero-length End-Of-List ends the list on parse); nothing asserts the TAG_LENGTH MUST, and Ze never builds an End-Of-List

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParseEndOfListTag`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L360) | unit/verify | unproven |

### [`RFC2516-5.2-1`](#rfc2516-5.2-1)

If the Access Concentrator receives this TAG, it MUST include the TAG unmodified in the associated PADO or PADS response. (Appendix A, §11)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. PADO half enforced (TestBuildPADO compares the echoed value, TestBuildNoHostUniqEcho the absence); the PADS unit TestBuildPADS checks only len==2, so a PADS echoing a modified Host-Uniq of the same length passes

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L573) | unit/verify | unproven |
| positive | [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L124) | unit/verify | unproven |
| positive | [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L173) | unit/verify | unproven |

### [`RFC2516-5.3-1`](#rfc2516-5.3-1)

If a Host receives this TAG, it MUST return the TAG unmodified in the following PADR. (Appendix A, §11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestBuildPADREchoesTags compares the echoed AC-Cookie bytes; TestBuildPADRNoOptionalTags shows none is invented

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L544) | unit/verify | unproven |
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L503) | unit/verify | unproven |

### [`RFC2516-x-5`](#rfc2516-x-5)

If either the Host or Access Concentrator receives this TAG they MUST include it unmodified in any discovery packet they send as a response. (Appendix A, §11)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. PADO, PADR and PADS echo the Relay-Session-Id bytes; the refusing PADS (BuildPADSError, discovery.go:407) is also a response and no unit asserts its echo, and the negative checks only BuildPADO invents none

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRelaySessionIDNoEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L653) | unit/verify | unproven |
| positive | [`TestRelaySessionIDEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L610) | unit/verify | unproven |

### [`RFC2516-5.1-1`](#rfc2516-5.1-1)

The PADI packet MUST contain exactly one TAG of TAG_TYPE Service- Name, indicating the service the Host is requesting, and any number of other TAG types. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity; TestBuildPADIDiscovery counts exactly one Service-Name tag

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L480) | unit/verify | unproven |

### [`RFC2516-5.1-2`](#rfc2516-5.1-2)

The Host sends the PADI packet with the DESTINATION_ADDR set to the broadcast address. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity; TestBuildPADIDiscovery asserts the PADI DstMAC is the broadcast address

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L481) | unit/verify | unproven |

### [`RFC2516-5.2-2`](#rfc2516-5.2-2)

The PADO packet MUST contain one AC-Name TAG containing the Access Concentrator's name, a Service-Name TAG identical to the one in the PADI, and any number of other Service-Name TAGs indicating other services that the Access Concentrator offers. (§5.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestPADOAlwaysCarriesServiceName asserts the Service-Name tags exactly but never the AC-Name TAG the sentence also requires (TestBuildPADO checks it, untagged)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L721) | unit/verify | revert, verified |
| positive | [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L720) | unit/verify | revert, verified |

### [`RFC2516-5.2-4`](#rfc2516-5.2-4)

If the Access Concentrator can not serve the PADI it MUST NOT respond with a PADO. (§5.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestServiceNameFilter asserts only the MatchServiceName predicate; no unit drives handlePADI and observes that no PADO is sent for an unserved name, though the recording server harness in session_id_rfc2516_test.go could

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L242) | unit/verify | unproven |
| positive | [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L241) | unit/verify | unproven |

### [`RFC2516-5.3-2`](#rfc2516-5.3-2)

The PADR packet MUST contain exactly one TAG of TAG_TYPE Service- Name, indicating the service the Host is requesting, and any number of other TAG types. (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity; TestBuildPADREchoesTags counts exactly one Service-Name tag

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L504) | unit/verify | unproven |

### [`RFC2516-5.4-1`](#rfc2516-5.4-1)

The PADS packet contains exactly one TAG of TAG_TYPE Service-Name, indicating the service under which Access Concentrator has accepted the PPPoE session, and any number of other TAG types. (§5.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestBuildPADS and TestPADSAlwaysCarriesServiceName both count exactly one Service-Name tag in the PADS, including for a PADR carrying none

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPADSAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L777) | unit/verify | revert, verified |
| positive | [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L174) | unit/verify | unproven |

### [`RFC2516-5.4-2`](#rfc2516-5.4-2)

If the Access Concentrator does not like the Service-Name in the PADR, then it MUST reply with a PADS containing a TAG of TAG_TYPE Service-Name-Error (and any number of other TAG types). In this case the SESSION_ID MUST be set to 0x0000. (§5.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the positive drives handlePADR only with a PADR carrying NO Service-Name tag (the requireServiceNameTag branch, server.go:153); the sentence's case, a present Service-Name the AC does not serve (the MatchServiceName branch, server.go:164), is never driven by a tagged unit, so deleting that branch leaves both units green. The TestBuildPADS negative calls the builder, not handlePADR, and does not check the error tag is absent

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L175) | unit/verify | revert, verified |
| positive | [`TestPADRWithoutServiceNameGetsServiceNameError`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L156) | unit/verify | revert, verified |

### [`RFC2516-x-6`](#rfc2516-x-6)

For Discovery packets, the value is either a unicast or broadcast address as defined in the Discovery section. For PPP session traffic, this field MUST contain the peer's unicast address as determined from the Discovery stage. (§4)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. the sentence binds PPP session traffic (0x8864); TestBuildPADT asserts the destination of a PADT, which is a Discovery packet. Session-stage addressing is set by pppoeCreate connecting the pppox socket to the peer MAC (kernel_linux.go) and no tagged unit observes it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADT`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L218) | unit/verify | unproven |

### [`RFC2516-x-7`](#rfc2516-x-7)

The SOURCE_ADDR field MUST contains the Ethernet MAC address of the source device. (§4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tagged units prove receive-side validation (a unicast source parses, broadcast and multicast sources are refused); none asserts that Ze's builders write the sending device's MAC in SOURCE_ADDR (TestBuildPADO asserts it but is not tagged)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseBroadcastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L327) | unit/verify | unproven |
| negative | [`TestParseMulticastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L337) | unit/verify | unproven |
| positive | [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L46) | unit/verify | unproven |

### [`RFC2516-x-8`](#rfc2516-x-8)

If a discovery packet is received with a TAG of unknown TAG_TYPE, the TAG MUST be ignored unless otherwise specified in this document. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity; TestUnknownTagIgnored parses a frame with an unknown tag without error and keeps known tags findable

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUnknownTagIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L673) | unit/verify | unproven |

### [`RFC2516-x-9`](#rfc2516-x-9)

The Maximum-Receive-Unit (MRU) option MUST NOT be negotiated to a larger size than 1492. (§7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestLCPConfigRequestMRU passes 1492 into sendLCPConfigRequest itself, so a dialer default above 1492 would not turn it red; no unit shows a peer's MRU above 1492 is refused (Nak) or the AC cap applies

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLCPConfigRequestMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L117) | unit/verify | unproven |

### [`RFC2516-7-1`](#rfc2516-7-1)

The SESSION_ID MUST NOT change for that PPPoE session and MUST be the value assigned in the Discovery stage. (§6)

Audit verdict: wrong (the tests assert something other than what the requirement demands), fresh. the row now quotes Section 6 (SESSION_ID constant in the session stage and equal to the Discovery value); TestHandlePADTVerifiesMACAndSID asserts PADT source-MAC validation, a Discovery-packet rule the sentence does not state. The session-stage SESSION_ID is set by pppoeCreate (kernel_linux.go) and no tagged unit observes it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L13) | unit/verify | unproven |
| positive | [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L12) | unit/verify | unproven |

### [`RFC2516-3-1`](#rfc2516-3-1)

Once a PPP session is established, both the Host and the Access Concentrator MUST allocate the resources for a PPP virtual interface (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-3-1, so no unit is bound to it.

### [`RFC2516-4-1`](#rfc2516-4-1)

A value of 0xffff is reserved for future use and MUST NOT be used (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2516SessionID0xffffIsNeverAllocated exhausts the table and shows 0xffff is never handed out, even after freeSID

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2516SessionID0xffffIsNeverAllocated`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestRFC2516AllocSIDReturnsUsableValue`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L46) | unit/verify | revert, verified |

### [`RFC2516-5.1-4`](#rfc2516-5.1-4)

The CODE field is set to 0x09 and the SESSION_ID MUST be set to 0x0000. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2516PADISessionIDIsZero asserts CODE 0x09 and SESSION_ID 0; the negative shows handlePADI drops a non-zero SESSION_ID PADI

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2516PADIWithNonZeroSessionIDIsDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestRFC2516PADISessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L79) | unit/verify | revert, verified |

### [`RFC2516-5.1-5`](#rfc2516-5.1-5)

An entire PADI packet (including the PPPoE header) MUST NOT exceed 1484 octets so as to leave sufficient room for a relay agent to add a Relay-Session-Id TAG. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestPADINeverExceeds1484Octets accepts exactly 1484 octets and refuses 1485

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padi_length_rfc2516_test.go#L18) | unit/verify | revert, verified |
| positive | [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padi_length_rfc2516_test.go#L17) | unit/verify | revert, verified |

### [`RFC2516-5.2-6`](#rfc2516-5.2-6)

The CODE field is set to 0x07 and the SESSION_ID MUST be set to 0x0000. (§5.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2516PADOSessionIDIsZero asserts CODE 0x07 and SESSION_ID 0; the client refuses a PADO with a non-zero SESSION_ID

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2516PADOWithNonZeroSessionIDIsNotAnOffer`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/discovery_rfc2516_test.go#L37) | unit/verify | revert, verified |
| positive | [`TestRFC2516PADOSessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L115) | unit/verify | revert, verified |

### [`RFC2516-5.3-5`](#rfc2516-5.3-5)

The CODE field is set to 0x19 and the SESSION_ID MUST be set to 0x0000. (§5.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2516PADRSessionIDIsZero asserts CODE 0x19 and SESSION_ID 0; handlePADR drops a non-zero SESSION_ID PADR

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2516PADRWithNonZeroSessionIDIsDropped`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L149) | unit/verify | revert, verified |
| positive | [`TestRFC2516PADRSessionIDIsZero`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L132) | unit/verify | revert, verified |

### [`RFC2516-5.4-3`](#rfc2516-5.4-3)

The CODE field is set to 0x65 and the SESSION_ID MUST be set to the unique value generated for this PPPoE session. (§5.4)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the positive drives only the replay path (matchLiveCookie, server.go:191) with a pre-added session; the PADS that admits a fresh session (server.go:276, after AllocSID) is never observed, so a wrong SESSION_ID there stays green. CODE 0x65 and the client's refusal of 0x0000 are asserted

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2516PADSWithZeroSessionIDIsARefusal`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/discovery_rfc2516_test.go#L61) | unit/verify | revert, verified |
| positive | [`TestRFC2516PADSCarriesTheSessionsID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L178) | unit/verify | revert, verified |

### [`RFC2516-5.5-3`](#rfc2516-5.5-3)

The DESTINATION_ADDR field is a unicast Ethernet address, the CODE field is set to 0xa7 and the SESSION_ID MUST be set to indicate which session is to be terminated. (§5.5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. AC-sent PADT is asserted (CODE, SESSION_ID, unicast destination via handleSessionDown); the Host-sent PADT (pppoeclient/dialer.go::sendPADT, its own dstMAC and sid) has no tagged assertion, so a broadcast destination or wrong SESSION_ID there stays green. The negative proves receive-side matching, not the sent fields

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2516UnknownSessionIDTerminatesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L253) | unit/verify | revert, verified |
| positive | [`TestRFC2516SessionDownSendsPADTForThatSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L219) | unit/verify | revert, verified |

### [`RFC2516-5.5-4`](#rfc2516-5.5-4)

Even normal PPP termination packets MUST NOT be sent after sending or receiving a PADT (§5.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padt_lifetime_linux_test.go#L46) | unit/verify | unproven |
| negative | [`TestRFC2516ClientPADTStopsOnlyMatchingSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/padt_lifetime_test.go#L30) | unit/verify | unproven |
| positive | [`TestRFC2516ACReceivedPADTClosesOnlyMatchingTransport`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padt_lifetime_linux_test.go#L45) | unit/verify | unproven |
| positive | [`TestRFC2516ACSentPADTClosesTransportBeforeSend`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padt_lifetime_linux_test.go#L79) | unit/verify | unproven |
| positive | [`TestRFC2516ClientPADTStopsOnlyMatchingSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/padt_lifetime_test.go#L29) | unit/verify | unproven |
| positive | [`TestRFC2516ClientSentPADTStopsPPPBeforeSend`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/padt_lifetime_test.go#L127) | unit/verify | unproven |

### [`RFC2516-6-1`](#rfc2516-6-1)

The PPPoE CODE MUST be set to 0x00. (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-6-1, so no unit is bound to it.

### [`RFC2516-7-2`](#rfc2516-7-2)

An implementation MUST NOT request any of the following options, and MUST reject a request for such an option: Field Check Sequence (FCS) Alternatives, Address-and-Control-Field-Compression (ACFC), Asynchronous-Control-Character-Map (ACCM) (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2516ACRejectsForbiddenLCPOptions and TestRFC2516ClientRejectsForbiddenLCPOptions assert Configure-Reject of ACCM, ACFC and FCS Alternatives verbatim, that neither role requests them, and Ack for MRU/PFC

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2516ACRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc2516_options_test.go#L12) | unit/verify | unproven |
| negative | [`TestRFC2516ClientRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L102) | unit/verify | unproven |
| positive | [`TestRFC2516ACRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/ppp/rfc2516_options_test.go#L11) | unit/verify | unproven |
| positive | [`TestRFC2516ClientRejectsForbiddenLCPOptions`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/rfc1661_client_negotiation_test.go#L101) | unit/verify | unproven |

### [`RFC2516-7-3`](#rfc2516-7-3)

When LCP terminates, the Host and Access concentrator MUST stop using that PPPoE session. (§7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the AC half is asserted (handleSessionDown removes the session and sends its PADT; an unknown SID leaves it). The sentence binds the Host too, and no tagged unit shows the client stops using the session when LCP terminates

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2516UnknownSessionIDTerminatesNothing`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L254) | unit/verify | revert, verified |
| positive | [`TestRFC2516SessionDownSendsPADTForThatSession`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L220) | unit/verify | revert, verified |

### [`RFC2516-7-4`](#rfc2516-7-4)

If the Host wishes to start another PPP session, it MUST return to the PPPoE Discovery stage. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC2516HostReturnsToDiscoveryForTheNextSession shows the client redials (Discovery) after Done with SESSION_ID 0 and the old session cleaned up

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC2516HostReturnsToDiscoveryForTheNextSession`](https://github.com/ze-software/ze/blob/main/internal/component/iface/pppoe_client_rfc2516_test.go#L60) | unit/verify | revert, verified |
| positive | [`TestRFC2516HostReturnsToDiscoveryForTheNextSession`](https://github.com/ze-software/ze/blob/main/internal/component/iface/pppoe_client_rfc2516_test.go#L59) | unit/verify | revert, verified |

### [`RFC2516-x-10`](#rfc2516-x-10)

All PADI packets MUST guarantee sufficient room for the addition of a Relay-Session-Id TAG with a TAG_VALUE length of 12 octets. (Appendix A, §11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestPADINeverExceeds1484Octets asserts exactly 16 octets are left at the largest PADI and that one octet more is refused

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padi_length_rfc2516_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestPADINeverExceeds1484Octets`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/padi_length_rfc2516_test.go#L19) | unit/verify | revert, verified |

### [`RFC2516-x-11`](#rfc2516-x-11)

A Relay-Session-Id TAG MUST NOT be added if the discovery packet already contains one. (Appendix A, §11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-x-11, so no unit is bound to it.

### [`RFC2516-x-12`](#rfc2516-x-12)

This TAG (typically with a zero-length data section) indicates that for one reason or another, the requested Service-Name request could not be honored. If there is data, and the first octet of the data is nonzero, then it MUST be a printable UTF-8 string which explains why the request was denied. (Appendix A, §11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity; TestRFC2516ErrorTagsCarryNoData drives handlePADR and asserts the Service-Name-Error TAG_LENGTH is 0

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2516ErrorTagsCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L285) | unit/verify | revert, verified |

### [`RFC2516-x-13`](#rfc2516-x-13)

It MAY be included in PADS packets. If there is data, and the first octet of the data is nonzero, then it MUST be a printable UTF-8 string which explains the nature of the error. (Appendix A, §11)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity; TestRFC2516ErrorTagsCarryNoData drives a resource refusal and asserts the AC-System-Error TAG_LENGTH is 0

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC2516ErrorTagsCarryNoData`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/session_id_rfc2516_test.go#L286) | unit/verify | revert, verified |

### [`RFC2516-x-14`](#rfc2516-x-14)

It can be added to PADO, PADR or PADS packets when an unrecoverable error occurs and no other error TAG is appropriate. If there is data then it MUST be an UTF-8 string which explains the nature of the error. (Appendix A, §11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-x-14, so no unit is bound to it.

### [`RFC2516-x-15`](#rfc2516-x-15)

If there is data then it MUST be an UTF-8 string which explains the nature of the error. This string MUST NOT be NULL terminated. (Appendix A, §11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-x-15, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc2516.txt |
| Source fingerprint | 260dccfe0a8a3e8c |
| Record | rfc/extraction/rfc2516.json |
| Mapped sentences | 35 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `4` | not stated | 5 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `5.1` | not stated | 3 | walked | not stated |
| `5.2` | not stated | 3 | walked | not stated |
| `5.3` | not stated | 2 | walked | not stated |
| `5.4` | not stated | 3 | walked | not stated |
| `5.5` | not stated | 2 | walked | not stated |
| `6` | not stated | 2 | walked | not stated |
| `7` | not stated | 5 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 10 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The RFC 2119 conventions boilerplate of section 2. It tells the reader how to read the keywords and states no obligation of its own. | The keywords MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL, when they appear in this document, are to be interpreted as described in [2]. |
| `5.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second half of the Service-Name-Error reply rule the previous sentence states. RFC2516-5.4-2 already carries it: the Access Concentrator 'MUST reply with a PADS carrying a Service-Name-Error tag, and MUST set SESSION_ID to 0x0000 in that reply'. | In this case the SESSION_ID MUST be set to 0x0000. |
| `7:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same 1492 ceiling the previous sentence sets on the MRU option, restated as the PPP MTU with its arithmetic: 1500 octets of Ethernet payload less the 6-octet PPPoE header and the 2-octet PPP Protocol ID. | Since Ethernet has a maximum payload size of 1500 octets, the PPPoE header is 6 octets and the PPP Protocol ID is 2 octets, the PPP MTU MUST NOT be greater than 1492. |

## Superseded

No document obsoletes RFC 2516, so its obligations are stated where they were written.
