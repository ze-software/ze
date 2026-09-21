# RFC 2516 - A Method for Transmitting PPP Over Ethernet (PPPoE)

Partial. Every requirement this repository extracted from RFC 2516, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 36.6% | 15 of 41 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 17.1% | 7 of 41 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 41 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 12.8% | 5 of 39 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 41 | of 45 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 41 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 41 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 41 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 41 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 46.3% | 19 of 41 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 41 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 45 |
| Gated MUST-level | 41 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 19 |
| Nightly-only evidence | 0 |
| Test tags | 39 |
| Tagged units | 39 |
| Recorded audit verdicts | 0 |
| Discrimination records | 5 |
| Summary | `rfc/short/rfc2516.md` |
| Requirement shard | `rfc/requirements/rfc2516.md` |
| RFC text | `rfc/full/rfc2516.txt` |

## Enrolment

Enrolled: A Method for Transmitting PPP Over Ethernet / PPPoE (RFC 2516): AC + client, both roles. 40 MUST-level rows after the 2026-09-21 extraction walk. 20 carry tests: 13 with both polarities (VER and TYPE nibble = 1, Relay-Session-Id echo, Host-Uniq echo in PADO/PADS, AC-Cookie and Host-Uniq echo in PADR, no PADO for an unservable Service-Name, the session identity of SESSION_ID with the two MAC addresses, SOURCE_ADDR carrying a source-device MAC with broadcast and multicast sources refused on parse, exactly one Service-Name tag in PADO and in PADS, a PADR the AC will not serve refused with a Service-Name-Error tag and SESSION_ID 0x0000) and 7 single-polarity positive (no non-zero End-Of-List, unicast destination for session traffic, unknown tags tolerated, client LCP MRU 1492, PADI one Service-Name to broadcast, PADR one Service-Name with presence checked but not count on receipt). The other 20 rows the walk added from sentences this checklist had never carried, and NONE of them has a test: §3 resource allocation, §4 reserved SESSION_ID 0xffff, the per-packet SESSION_ID values of §5.1 to §5.5, the 1484-octet PADI ceiling, silence after a PADT, §6 CODE 0x00, the §7 LCP option refusals and post-termination rules, and the Relay-Session-Id and error-TAG rules of the tag catalog. RFC2516-x-3 (a packet with VER or TYPE other than 0x1 is silently discarded) was deleted by the walk: "discard" and "multicast" appear zero times in rfc/full/rfc2516.txt, so neither that rule nor a broadcast/multicast source rule is in this document.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Access concentrator (PADI/PADO/PADR/PADS/PADT discovery, AC-Cookie, session tables, AF_PPPOX kernel sessions) and PPPoE client/Host dialer, over the shared PPP driver. Tests are bound per requirement in [`rfc/requirements/rfc2516.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc2516.md) for the 20 MUST-level rows that carry one
- the 20 rows the 2026-09-21 extraction walk added carry none.


**What the ledger says remains**

Twenty MUST rows the 2026-09-21 extraction walk added carry no test, so each is an open obligation on this ledger: §3 resource allocation, §4 reserved SESSION_ID 0xffff, the per-packet SESSION_ID values of §5.1 to §5.5, the 1484-octet PADI ceiling, silence after a PADT, §6 CODE 0x00, the §7 LCP option refusals and post-termination rules, and the Relay-Session-Id and error-TAG rules of the tag catalog. [`RFC2516-5.2-2`](#rfc2516-5.2-2) (BuildPADO omitting Service-Name under the accept-any config) closed 2026-09-09: BuildPADO and BuildPADS now write the Service-Name tag through AddTagString unconditionally, so an absent or zero-length source value still yields exactly one tag. The receive-side tolerance that remains -- a PADI or PADR with no Service-Name tag is served/admitted rather than refused for count, and a duplicate tag resolves first-wins -- is a deliberate robustness choice recorded in [`docs/architecture/l2tp/bng-5-pppoe.md`](https://github.com/ze-software/ze/blob/main/docs/architecture/l2tp/bng-5-pppoe.md), not an unmet MUST: Sections 5.1 and 5.3 bind the sending host, and no reference AC (accel-ppp, FreeBSD) enforces those counts on receipt either. The extraction sign-off this sentence used to wait for landed on 2026-09-21 at [`rfc/extraction/rfc2516.json`](https://github.com/ze-software/ze/blob/main/rfc/extraction/rfc2516.json), and it is what found the twenty untested rows above, so status stays `Partial` for those rows rather than for an unbounded extraction.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 15 | one part of the gated population |
| Annotated instead of tested | 7 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 19 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **41** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (15):** [`RFC2516-x-1`](#rfc2516-x-1), [`RFC2516-x-2`](#rfc2516-x-2), [`RFC2516-x-3`](#rfc2516-x-3), [`RFC2516-5.2-1`](#rfc2516-5.2-1), [`RFC2516-5.3-1`](#rfc2516-5.3-1), [`RFC2516-x-5`](#rfc2516-x-5), [`RFC2516-5.2-2`](#rfc2516-5.2-2), [`RFC2516-5.2-3`](#rfc2516-5.2-3), [`RFC2516-5.2-4`](#rfc2516-5.2-4), [`RFC2516-5.3-3`](#rfc2516-5.3-3), [`RFC2516-5.3-4`](#rfc2516-5.3-4), [`RFC2516-5.4-1`](#rfc2516-5.4-1), [`RFC2516-5.4-2`](#rfc2516-5.4-2), [`RFC2516-x-7`](#rfc2516-x-7), [`RFC2516-7-1`](#rfc2516-7-1)

**Annotated instead of tested (7):** [`RFC2516-x-4`](#rfc2516-x-4), [`RFC2516-5.1-1`](#rfc2516-5.1-1), [`RFC2516-5.1-2`](#rfc2516-5.1-2), [`RFC2516-5.3-2`](#rfc2516-5.3-2), [`RFC2516-x-6`](#rfc2516-x-6), [`RFC2516-x-8`](#rfc2516-x-8), [`RFC2516-x-9`](#rfc2516-x-9)

**No test and no annotation (19):** [`RFC2516-3-1`](#rfc2516-3-1), [`RFC2516-4-1`](#rfc2516-4-1), [`RFC2516-5.1-4`](#rfc2516-5.1-4), [`RFC2516-5.1-5`](#rfc2516-5.1-5), [`RFC2516-5.2-6`](#rfc2516-5.2-6), [`RFC2516-5.3-5`](#rfc2516-5.3-5), [`RFC2516-5.4-3`](#rfc2516-5.4-3), [`RFC2516-5.5-3`](#rfc2516-5.5-3), [`RFC2516-5.5-4`](#rfc2516-5.5-4), [`RFC2516-6-1`](#rfc2516-6-1), [`RFC2516-7-2`](#rfc2516-7-2), [`RFC2516-7-3`](#rfc2516-7-3), [`RFC2516-7-4`](#rfc2516-7-4), [`RFC2516-x-10`](#rfc2516-x-10), [`RFC2516-x-11`](#rfc2516-x-11), [`RFC2516-x-12`](#rfc2516-x-12), [`RFC2516-x-13`](#rfc2516-x-13), [`RFC2516-x-14`](#rfc2516-x-14), [`RFC2516-x-15`](#rfc2516-x-15)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2516-x-1` | VER field MUST be 0x1 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L452). **negative:** `unit/verify` [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L303) |
| `RFC2516-x-2` | TYPE field MUST be 0x1 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L453). **negative:** `unit/verify` [`TestParseBadType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L472) |
| `RFC2516-x-3` | Packets with VER or TYPE other than 0x1 MUST be silently discarded (Encoding Rules) | MUST | x | **positive:** `unit/verify` [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L46). **negative:** `unit/verify` [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L304) |
| `RFC2516-x-4` | End-Of-List tag TAG_LENGTH MUST be 0 (Tag Catalog) | MUST | x | **positive:** `unit/verify` [`TestParseEndOfListTag`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L363). **negative:** no negative test. **{single-polarity}:** ze never constructs an End-Of-List tag (builders set the payload length instead, discovery.go:291), and parseTags treats a zero-length End-Of-List as the list terminator (discovery.go:161), so no non-zero-length End-Of-List is ever produced and no negative case exists |
| `RFC2516-5.2-1` | AC MUST echo Host-Uniq unchanged in PADO and PADS if Host included it in PADI/PADR (Encoding Rules, §5.2, §5.3) | MUST | 5.2 | **positive:** `unit/verify` [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L125). **positive:** `unit/verify` [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L175). **negative:** `unit/verify` [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L580) |
| `RFC2516-5.3-1` | Host MUST echo AC-Cookie unchanged in PADR if AC included it in PADO (Encoding Rules, §5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L506). **negative:** `unit/verify` [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L549) |
| `RFC2516-x-5` | Relay-Session-Id, if present, MUST be included unchanged in all subsequent Discovery packets for the exchange (Encoding Rules) | MUST | x | **positive:** `unit/verify` [`TestRelaySessionIDEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L618). **negative:** `unit/verify` [`TestRelaySessionIDNoEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L661) |
| `RFC2516-5.1-1` | PADI MUST contain exactly one Service-Name tag (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L483). **negative:** no negative test. **{single-polarity}:** emit-side count: BuildPADI always writes exactly one Service-Name tag (discovery.go:307). The AC does not enforce the count on receive (MatchServiceName tolerates zero or many, discovery.go:220): this is a deliberate choice, not an omission, per the owner decision of 2026-09-08 recorded in docs/architecture/l2tp/bng-5-pppoe.md ("Ze refuses a PADR with no Service-Name tag and serves a PADI with none"), because Section 5.1 binds the sending host and neither accel-ppp nor FreeBSD enforces this count on receipt either |
| `RFC2516-5.1-2` | PADI destination MUST be broadcast; non-broadcast PADI is silently discarded (§5.1, Encoding Rules) | MUST | 5.1 | **positive:** `unit/verify` [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L484). **negative:** no negative test. **{single-polarity}:** emit-side: BuildPADI addresses the PADI to the Ethernet broadcast MAC (discovery.go:305); the receive-side non-broadcast-PADI discard is not enforced (handlePADI checks only SID, server.go:53) |
| `RFC2516-5.2-2` | PADO MUST contain one AC-Name tag and one or more Service-Name tags (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L728). **negative:** `unit/verify` [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L729) |
| `RFC2516-5.2-3` | PADO MUST echo Host-Uniq if present in PADI (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L126). **negative:** `unit/verify` [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L581) |
| `RFC2516-5.2-4` | PADO MUST NOT be sent if AC cannot serve the requested Service-Name (§5.2) | MUST NOT | 5.2 | **positive:** `unit/verify` [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L243). **negative:** `unit/verify` [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L244) |
| `RFC2516-5.3-2` | PADR MUST contain exactly one Service-Name tag from the selected PADO (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L509). **negative:** no negative test. **{single-polarity}:** emit-side count: BuildPADR writes exactly one Service-Name tag (discovery.go:322). The AC's requireServiceNameTag (server.go) now refuses a PADR carrying zero Service-Name tags on receipt, but that enforces PRESENCE, not COUNT: a PADR carrying two Service-Name tags is accepted, first-wins (TestDuplicateServiceNameTagTakesTheFirst), so there is still no receive-side enforcement of "exactly one, not more" to give a negative for this requirement |
| `RFC2516-5.3-3` | PADR MUST echo AC-Cookie if one was in the PADO (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L507). **negative:** `unit/verify` [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L550) |
| `RFC2516-5.3-4` | PADR MUST echo Host-Uniq if one was in the PADI (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L508). **negative:** `unit/verify` [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L551) |
| `RFC2516-5.4-1` | PADS MUST contain exactly one Service-Name tag (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L176). **negative:** `unit/verify` [`TestPADSAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L785) |
| `RFC2516-5.4-2` | AC that does not like the PADR's Service-Name MUST reply with a PADS carrying a Service-Name-Error tag, and MUST set SESSION_ID to 0x0000 in that reply (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestPADRWithoutServiceNameGetsServiceNameError`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L156). **negative:** `unit/verify` [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L177) |
| `RFC2516-x-6` | For PPP session traffic the DESTINATION_ADDR field MUST contain the peer's unicast address as determined from the Discovery stage (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestBuildPADT`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L220). **negative:** no negative test. **{single-polarity}:** emit-side: every builder addresses PADO/PADR/PADS/PADT to the peer unicast MAC derived from its unicast source (discovery.go:320, 336, 362, 387); the receive-side broadcast-destination discard is not enforced (ParseDiscovery validates only the source, discovery.go:108) |
| `RFC2516-x-7` | The SOURCE_ADDR field MUST contain the Ethernet MAC address of the source device (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L47). **negative:** `unit/verify` [`TestParseBroadcastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L330). **negative:** `unit/verify` [`TestParseMulticastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L340) |
| `RFC2516-x-8` | Unknown tags MUST NOT cause an error; silently ignore (Encoding Rules) | MUST | x | **positive:** `unit/verify` [`TestUnknownTagIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L681). **negative:** no negative test. **{single-polarity}:** tolerate requirement: parseTags stores unknown tag types generically and never errors on them (discovery.go:154); rejecting an unknown tag would itself violate the MUST NOT, so no negative exists |
| `RFC2516-x-9` | LCP MUST negotiate MRU to 1492 or lower unless both sides support RFC 4638 and the Ethernet path supports larger frames (MTU) | MUST | x | **positive:** `unit/verify` [`TestLCPConfigRequestMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L117). **negative:** no negative test. **{single-polarity}:** ceiling requirement: the client proposes MRU 1492 by default in its LCP Configure-Request (dialer.go:114, session.go:487) and the AC caps MaxMRU at PPPoEMaxMTU 1492 (server.go:203); 1492 is the maximum so there is no negative |
| `RFC2516-7-1` | The SESSION_ID value is fixed for a given PPP session and, in fact, defines a PPP session along with the Ethernet SOURCE_ADDR and DESTINATION_ADDR; in the PPP Session stage it MUST NOT change and MUST be the value assigned in the Discovery stage (§7) | MUST | 7 | **positive:** `unit/verify` [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L12). **negative:** `unit/verify` [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L13) |
| `RFC2516-3-1` | Once a PPP session is established, both the Host and the Access Concentrator MUST allocate the resources for a PPP virtual interface (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-4-1` | A SESSION_ID value of 0xffff is reserved for future use and MUST NOT be used (§4) | MUST NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.1-4` | In a PADI the SESSION_ID MUST be set to 0x0000 (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.1-5` | An entire PADI packet, including the PPPoE header, MUST NOT exceed 1484 octets, so as to leave sufficient room for a relay agent to add a Relay-Session-Id TAG (§5.1) | MUST NOT | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.2-6` | In a PADO the SESSION_ID MUST be set to 0x0000 (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.3-5` | In a PADR the SESSION_ID MUST be set to 0x0000 (§5.3) | MUST | 5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.4-3` | In a PADS the SESSION_ID MUST be set to the unique value generated for this PPPoE session (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.5-3` | In a PADT the SESSION_ID MUST be set to indicate which session is to be terminated (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.5-4` | Even normal PPP termination packets MUST NOT be sent after sending or receiving a PADT (§5.5) | MUST NOT | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-6-1` | In the PPP Session stage the PPPoE CODE MUST be set to 0x00 (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-7-2` | An implementation MUST NOT request FCS Alternatives, ACFC or ACCM, and MUST reject a request for such an option (§7) | MUST NOT | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-7-3` | When LCP terminates, the Host and Access Concentrator MUST stop using that PPPoE session (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-7-4` | A Host that wishes to start another PPP session MUST return to the PPPoE Discovery stage (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-x-10` | All PADI packets MUST guarantee sufficient room for the addition of a Relay-Session-Id TAG with a TAG_VALUE length of 12 octets (Tag Catalog) | MUST | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-x-11` | A Relay-Session-Id TAG MUST NOT be added if the discovery packet already contains one (Tag Catalog) | MUST NOT | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-x-12` | Where a Service-Name-Error TAG carries data whose first octet is nonzero, that data MUST be a printable UTF-8 string which explains why the request was denied (Tag Catalog) | MUST | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-x-13` | Where an AC-System-Error TAG carries data whose first octet is nonzero, that data MUST be a printable UTF-8 string which explains the nature of the error (Tag Catalog) | MUST | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-x-14` | Where a Generic-Error TAG carries data, that data MUST be a UTF-8 string which explains the nature of the error (Tag Catalog) | MUST | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-x-15` | The Generic-Error string MUST NOT be NULL terminated (Tag Catalog) | MUST NOT | x | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.2-5` | PADO SHOULD contain AC-Cookie for DoS mitigation (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.1-3` | PADI MAY contain Host-Uniq for client-side demux of PADO replies (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.5-1` | PADT MAY be sent by either side at any time after PADS (§5.5) | MAY | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.5-2` | PADT MAY contain Generic-Error tag (§5.5) | MAY | 5.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2516-3-1`](#rfc2516-3-1) Once a PPP session is established, both the Host and the Access Concentrator MUST allocate the resources for a PPP virtual interface (§3) | no test | no test carries this requirement id |
| [`RFC2516-4-1`](#rfc2516-4-1) A SESSION_ID value of 0xffff is reserved for future use and MUST NOT be used (§4) | no test | no test carries this requirement id |
| [`RFC2516-5.1-4`](#rfc2516-5.1-4) In a PADI the SESSION_ID MUST be set to 0x0000 (§5.1) | no test | no test carries this requirement id |
| [`RFC2516-5.1-5`](#rfc2516-5.1-5) An entire PADI packet, including the PPPoE header, MUST NOT exceed 1484 octets, so as to leave sufficient room for a relay agent to add a Relay-Session-Id TAG (§5.1) | no test | no test carries this requirement id |
| [`RFC2516-5.2-6`](#rfc2516-5.2-6) In a PADO the SESSION_ID MUST be set to 0x0000 (§5.2) | no test | no test carries this requirement id |
| [`RFC2516-5.3-5`](#rfc2516-5.3-5) In a PADR the SESSION_ID MUST be set to 0x0000 (§5.3) | no test | no test carries this requirement id |
| [`RFC2516-5.4-3`](#rfc2516-5.4-3) In a PADS the SESSION_ID MUST be set to the unique value generated for this PPPoE session (§5.4) | no test | no test carries this requirement id |
| [`RFC2516-5.5-3`](#rfc2516-5.5-3) In a PADT the SESSION_ID MUST be set to indicate which session is to be terminated (§5.5) | no test | no test carries this requirement id |
| [`RFC2516-5.5-4`](#rfc2516-5.5-4) Even normal PPP termination packets MUST NOT be sent after sending or receiving a PADT (§5.5) | no test | no test carries this requirement id |
| [`RFC2516-6-1`](#rfc2516-6-1) In the PPP Session stage the PPPoE CODE MUST be set to 0x00 (§6) | no test | no test carries this requirement id |
| [`RFC2516-7-2`](#rfc2516-7-2) An implementation MUST NOT request FCS Alternatives, ACFC or ACCM, and MUST reject a request for such an option (§7) | no test | no test carries this requirement id |
| [`RFC2516-7-3`](#rfc2516-7-3) When LCP terminates, the Host and Access Concentrator MUST stop using that PPPoE session (§7) | no test | no test carries this requirement id |
| [`RFC2516-7-4`](#rfc2516-7-4) A Host that wishes to start another PPP session MUST return to the PPPoE Discovery stage (§7) | no test | no test carries this requirement id |
| [`RFC2516-x-10`](#rfc2516-x-10) All PADI packets MUST guarantee sufficient room for the addition of a Relay-Session-Id TAG with a TAG_VALUE length of 12 octets (Tag Catalog) | no test | no test carries this requirement id |
| [`RFC2516-x-11`](#rfc2516-x-11) A Relay-Session-Id TAG MUST NOT be added if the discovery packet already contains one (Tag Catalog) | no test | no test carries this requirement id |
| [`RFC2516-x-12`](#rfc2516-x-12) Where a Service-Name-Error TAG carries data whose first octet is nonzero, that data MUST be a printable UTF-8 string which explains why the request was denied (Tag Catalog) | no test | no test carries this requirement id |
| [`RFC2516-x-13`](#rfc2516-x-13) Where an AC-System-Error TAG carries data whose first octet is nonzero, that data MUST be a printable UTF-8 string which explains the nature of the error (Tag Catalog) | no test | no test carries this requirement id |
| [`RFC2516-x-14`](#rfc2516-x-14) Where a Generic-Error TAG carries data, that data MUST be a UTF-8 string which explains the nature of the error (Tag Catalog) | no test | no test carries this requirement id |
| [`RFC2516-x-15`](#rfc2516-x-15) The Generic-Error string MUST NOT be NULL terminated (Tag Catalog) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2516-x-1`](#rfc2516-x-1)

VER field MUST be 0x1 (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L303) | unit/verify | unproven |
| positive | [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L452) | unit/verify | unproven |

### [`RFC2516-x-2`](#rfc2516-x-2)

TYPE field MUST be 0x1 (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseBadType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L472) | unit/verify | unproven |
| positive | [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L453) | unit/verify | unproven |

### [`RFC2516-x-3`](#rfc2516-x-3)

Packets with VER or TYPE other than 0x1 MUST be silently discarded (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L304) | unit/verify | unproven |
| positive | [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L46) | unit/verify | unproven |

### [`RFC2516-x-4`](#rfc2516-x-4)

End-Of-List tag TAG_LENGTH MUST be 0 (Tag Catalog)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParseEndOfListTag`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L363) | unit/verify | unproven |

### [`RFC2516-5.2-1`](#rfc2516-5.2-1)

AC MUST echo Host-Uniq unchanged in PADO and PADS if Host included it in PADI/PADR (Encoding Rules, §5.2, §5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L580) | unit/verify | unproven |
| positive | [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L125) | unit/verify | unproven |
| positive | [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L175) | unit/verify | unproven |

### [`RFC2516-5.3-1`](#rfc2516-5.3-1)

Host MUST echo AC-Cookie unchanged in PADR if AC included it in PADO (Encoding Rules, §5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L549) | unit/verify | unproven |
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L506) | unit/verify | unproven |

### [`RFC2516-x-5`](#rfc2516-x-5)

Relay-Session-Id, if present, MUST be included unchanged in all subsequent Discovery packets for the exchange (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRelaySessionIDNoEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L661) | unit/verify | unproven |
| positive | [`TestRelaySessionIDEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L618) | unit/verify | unproven |

### [`RFC2516-5.1-1`](#rfc2516-5.1-1)

PADI MUST contain exactly one Service-Name tag (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L483) | unit/verify | unproven |

### [`RFC2516-5.1-2`](#rfc2516-5.1-2)

PADI destination MUST be broadcast; non-broadcast PADI is silently discarded (§5.1, Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L484) | unit/verify | unproven |

### [`RFC2516-5.2-2`](#rfc2516-5.2-2)

PADO MUST contain one AC-Name tag and one or more Service-Name tags (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L729) | unit/verify | revert, verified |
| positive | [`TestPADOAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L728) | unit/verify | revert, verified |

### [`RFC2516-5.2-3`](#rfc2516-5.2-3)

PADO MUST echo Host-Uniq if present in PADI (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L581) | unit/verify | unproven |
| positive | [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L126) | unit/verify | unproven |

### [`RFC2516-5.2-4`](#rfc2516-5.2-4)

PADO MUST NOT be sent if AC cannot serve the requested Service-Name (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L244) | unit/verify | unproven |
| positive | [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L243) | unit/verify | unproven |

### [`RFC2516-5.3-2`](#rfc2516-5.3-2)

PADR MUST contain exactly one Service-Name tag from the selected PADO (§5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L509) | unit/verify | unproven |

### [`RFC2516-5.3-3`](#rfc2516-5.3-3)

PADR MUST echo AC-Cookie if one was in the PADO (§5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L550) | unit/verify | unproven |
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L507) | unit/verify | unproven |

### [`RFC2516-5.3-4`](#rfc2516-5.3-4)

PADR MUST echo Host-Uniq if one was in the PADI (§5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L551) | unit/verify | unproven |
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L508) | unit/verify | unproven |

### [`RFC2516-5.4-1`](#rfc2516-5.4-1)

PADS MUST contain exactly one Service-Name tag (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPADSAlwaysCarriesServiceName`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L785) | unit/verify | revert, verified |
| positive | [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L176) | unit/verify | unproven |

### [`RFC2516-5.4-2`](#rfc2516-5.4-2)

AC that does not like the PADR's Service-Name MUST reply with a PADS carrying a Service-Name-Error tag, and MUST set SESSION_ID to 0x0000 in that reply (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L177) | unit/verify | revert, verified |
| positive | [`TestPADRWithoutServiceNameGetsServiceNameError`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L156) | unit/verify | revert, verified |

### [`RFC2516-x-6`](#rfc2516-x-6)

For PPP session traffic the DESTINATION_ADDR field MUST contain the peer's unicast address as determined from the Discovery stage (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADT`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L220) | unit/verify | unproven |

### [`RFC2516-x-7`](#rfc2516-x-7)

The SOURCE_ADDR field MUST contain the Ethernet MAC address of the source device (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseBroadcastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L330) | unit/verify | unproven |
| negative | [`TestParseMulticastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L340) | unit/verify | unproven |
| positive | [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L47) | unit/verify | unproven |

### [`RFC2516-x-8`](#rfc2516-x-8)

Unknown tags MUST NOT cause an error; silently ignore (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUnknownTagIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L681) | unit/verify | unproven |

### [`RFC2516-x-9`](#rfc2516-x-9)

LCP MUST negotiate MRU to 1492 or lower unless both sides support RFC 4638 and the Ethernet path supports larger frames (MTU)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLCPConfigRequestMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L117) | unit/verify | unproven |

### [`RFC2516-7-1`](#rfc2516-7-1)

The SESSION_ID value is fixed for a given PPP session and, in fact, defines a PPP session along with the Ethernet SOURCE_ADDR and DESTINATION_ADDR; in the PPP Session stage it MUST NOT change and MUST be the value assigned in the Discovery stage (§7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L13) | unit/verify | unproven |
| positive | [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L12) | unit/verify | unproven |

### [`RFC2516-3-1`](#rfc2516-3-1)

Once a PPP session is established, both the Host and the Access Concentrator MUST allocate the resources for a PPP virtual interface (§3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-3-1, so no unit is bound to it.

### [`RFC2516-4-1`](#rfc2516-4-1)

A SESSION_ID value of 0xffff is reserved for future use and MUST NOT be used (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-4-1, so no unit is bound to it.

### [`RFC2516-5.1-4`](#rfc2516-5.1-4)

In a PADI the SESSION_ID MUST be set to 0x0000 (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-5.1-4, so no unit is bound to it.

### [`RFC2516-5.1-5`](#rfc2516-5.1-5)

An entire PADI packet, including the PPPoE header, MUST NOT exceed 1484 octets, so as to leave sufficient room for a relay agent to add a Relay-Session-Id TAG (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-5.1-5, so no unit is bound to it.

### [`RFC2516-5.2-6`](#rfc2516-5.2-6)

In a PADO the SESSION_ID MUST be set to 0x0000 (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-5.2-6, so no unit is bound to it.

### [`RFC2516-5.3-5`](#rfc2516-5.3-5)

In a PADR the SESSION_ID MUST be set to 0x0000 (§5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-5.3-5, so no unit is bound to it.

### [`RFC2516-5.4-3`](#rfc2516-5.4-3)

In a PADS the SESSION_ID MUST be set to the unique value generated for this PPPoE session (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-5.4-3, so no unit is bound to it.

### [`RFC2516-5.5-3`](#rfc2516-5.5-3)

In a PADT the SESSION_ID MUST be set to indicate which session is to be terminated (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-5.5-3, so no unit is bound to it.

### [`RFC2516-5.5-4`](#rfc2516-5.5-4)

Even normal PPP termination packets MUST NOT be sent after sending or receiving a PADT (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-5.5-4, so no unit is bound to it.

### [`RFC2516-6-1`](#rfc2516-6-1)

In the PPP Session stage the PPPoE CODE MUST be set to 0x00 (§6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-6-1, so no unit is bound to it.

### [`RFC2516-7-2`](#rfc2516-7-2)

An implementation MUST NOT request FCS Alternatives, ACFC or ACCM, and MUST reject a request for such an option (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-7-2, so no unit is bound to it.

### [`RFC2516-7-3`](#rfc2516-7-3)

When LCP terminates, the Host and Access Concentrator MUST stop using that PPPoE session (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-7-3, so no unit is bound to it.

### [`RFC2516-7-4`](#rfc2516-7-4)

A Host that wishes to start another PPP session MUST return to the PPPoE Discovery stage (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-7-4, so no unit is bound to it.

### [`RFC2516-x-10`](#rfc2516-x-10)

All PADI packets MUST guarantee sufficient room for the addition of a Relay-Session-Id TAG with a TAG_VALUE length of 12 octets (Tag Catalog)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-x-10, so no unit is bound to it.

### [`RFC2516-x-11`](#rfc2516-x-11)

A Relay-Session-Id TAG MUST NOT be added if the discovery packet already contains one (Tag Catalog)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-x-11, so no unit is bound to it.

### [`RFC2516-x-12`](#rfc2516-x-12)

Where a Service-Name-Error TAG carries data whose first octet is nonzero, that data MUST be a printable UTF-8 string which explains why the request was denied (Tag Catalog)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-x-12, so no unit is bound to it.

### [`RFC2516-x-13`](#rfc2516-x-13)

Where an AC-System-Error TAG carries data whose first octet is nonzero, that data MUST be a printable UTF-8 string which explains the nature of the error (Tag Catalog)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-x-13, so no unit is bound to it.

### [`RFC2516-x-14`](#rfc2516-x-14)

Where a Generic-Error TAG carries data, that data MUST be a UTF-8 string which explains the nature of the error (Tag Catalog)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-x-14, so no unit is bound to it.

### [`RFC2516-x-15`](#rfc2516-x-15)

The Generic-Error string MUST NOT be NULL terminated (Tag Catalog)

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
| Mapped sentences | 38 |
| Declined as scope | 5 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `2` | not stated | 1 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 5 | walked | not stated |
| `5` | not stated | 1 | walked | not stated |
| `5.1` | not stated | 3 | walked | not stated |
| `5.2` | not stated | 3 | walked | not stated |
| `5.3` | not stated | 2 | walked | not stated |
| `5.4` | not stated | 3 | walked | not stated |
| `5.5` | not stated | 3 | walked | not stated |
| `6` | not stated | 2 | walked | not stated |
| `7` | not stated | 5 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 12 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 1 is the Introduction. The lowercase 'must learn' describes why the Discovery stage exists; the obligations it names are stated normatively in sections 4 and 5 and are mapped from those sections. | To provide a point-to-point connection over Ethernet, each PPP session must learn the Ethernet address of the remote peer, as well as establish a unique session identifier. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The RFC 2119 conventions boilerplate of section 2. It tells the reader how to read the keywords and states no obligation of its own. | The keywords MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT, RECOMMENDED, MAY, and OPTIONAL, when they appear in this document, are to be interpreted as described in [2]. |
| `5.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second half of the Service-Name-Error reply rule the previous sentence states. RFC2516-5.4-2 already carries it: the Access Concentrator 'MUST reply with a PADS carrying a Service-Name-Error tag, and MUST set SESSION_ID to 0x0000 in that reply'. | In this case the SESSION_ID MUST be set to 0x0000. |
| `7:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same 1492 ceiling the previous sentence sets on the MRU option, restated as the PPP MTU with its arithmetic: 1500 octets of Ethernet payload less the 6-octet PPPoE header and the 2-octet PPP Protocol ID. | Since Ethernet has a maximum payload size of 1500 octets, the PPPoE header is 6 octets and the PPP Protocol ID is 2 octets, the PPP MTU MUST NOT be greater than 1492. |
| `11:12` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The RFC's Full Copyright Statement. It is document boilerplate the site scan did not strip, and it states no protocol obligation. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 2516, so its obligations are stated where they were written.
