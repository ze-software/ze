# RFC 2516 - A Method for Transmitting PPP Over Ethernet (PPPoE)

Partial. Every requirement this repository extracted from RFC 2516, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 57.1% | 12 of 21 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 38.1% | 8 of 21 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 21 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 34 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 21 | of 25 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 21 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 4.8% | 1 of 21 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 21 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 25 |
| Gated MUST-level | 21 |
| Obligations that bind Ze | 21 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 34 |
| Tagged units | 34 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2516.md` |
| Requirement shard | `rfc/requirements/rfc2516.md` |
| RFC text | `rfc/full/rfc2516.txt` |

## Enrolment

Enrolled: A Method for Transmitting PPP Over Ethernet / PPPoE (RFC 2516): AC + client, both roles. 12 MET (VER/TYPE nibble=1 and discard ver/type!=0x11, reject broadcast/multicast source, Relay-Session-Id echo, Host-Uniq echo in PADO/PADS, AC-Cookie + Host-Uniq echo in PADR, no PADO for unservable Service-Name, PADT verified on SESSION_ID+source MAC) + 8 single-polarity positive (no non-zero End-Of-List, unicast destinations, unknown tags tolerated, client LCP MRU 1492, PADI one Service-Name to broadcast, PADR/PADS one Service-Name) + 1 gap (BuildPADO can emit a PADO with zero Service-Name tags under accept-any config)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered:**

Access concentrator (PADI/PADO/PADR/PADS/PADT discovery, AC-Cookie, session tables, AF_PPPOX kernel sessions) and PPPoE client/Host dialer, over the shared PPP driver. Tests bound per requirement in [`rfc/requirements/rfc2516.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc2516.md).

**What the ledger says remains**

One MUST gap, gated in [`rfc/short/rfc2516.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2516.md): [`RFC2516-5.2-2`](#rfc2516-5.2-2) -- BuildPADO ([`internal/component/l2tp/pppoe/discovery.go`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery.go)) can emit a PADO carrying only an AC-Name and no Service-Name tag when the AC has no configured service names (the documented accept-any config) and the client's PADI Service-Name is empty or absent, violating the one-or-more Service-Name MUST.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 12 | one part of the gated population |
| Annotated instead of tested | 9 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **21** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (12):** [`RFC2516-x-1`](#rfc2516-x-1), [`RFC2516-x-2`](#rfc2516-x-2), [`RFC2516-x-3`](#rfc2516-x-3), [`RFC2516-5.2-1`](#rfc2516-5.2-1), [`RFC2516-5.3-1`](#rfc2516-5.3-1), [`RFC2516-x-5`](#rfc2516-x-5), [`RFC2516-5.2-3`](#rfc2516-5.2-3), [`RFC2516-5.2-4`](#rfc2516-5.2-4), [`RFC2516-5.3-3`](#rfc2516-5.3-3), [`RFC2516-5.3-4`](#rfc2516-5.3-4), [`RFC2516-x-7`](#rfc2516-x-7), [`RFC2516-7-1`](#rfc2516-7-1)

**Annotated instead of tested (9):** [`RFC2516-x-4`](#rfc2516-x-4), [`RFC2516-5.1-1`](#rfc2516-5.1-1), [`RFC2516-5.1-2`](#rfc2516-5.1-2), [`RFC2516-5.2-2`](#rfc2516-5.2-2), [`RFC2516-5.3-2`](#rfc2516-5.3-2), [`RFC2516-5.4-1`](#rfc2516-5.4-1), [`RFC2516-x-6`](#rfc2516-x-6), [`RFC2516-x-8`](#rfc2516-x-8), [`RFC2516-x-9`](#rfc2516-x-9)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2516-x-1` | VER field MUST be 0x1 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L451). **negative:** `unit/verify` [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L302) |
| `RFC2516-x-2` | TYPE field MUST be 0x1 (Wire Format) | MUST | x | **positive:** `unit/verify` [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L452). **negative:** `unit/verify` [`TestParseBadType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L471) |
| `RFC2516-x-3` | Packets with VER or TYPE other than 0x1 MUST be silently discarded (Encoding Rules) | MUST | x | **positive:** `unit/verify` [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L46). **negative:** `unit/verify` [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L303) |
| `RFC2516-x-4` | End-Of-List tag TAG_LENGTH MUST be 0 (Tag Catalog) | MUST | x | **positive:** `unit/verify` [`TestParseEndOfListTag`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L362). **negative:** no negative test. **{single-polarity}:** ze never constructs an End-Of-List tag (builders set the payload length instead, discovery.go:291), and parseTags treats a zero-length End-Of-List as the list terminator (discovery.go:161), so no non-zero-length End-Of-List is ever produced and no negative case exists |
| `RFC2516-5.2-1` | AC MUST echo Host-Uniq unchanged in PADO and PADS if Host included it in PADI/PADR (Encoding Rules, §5.2, §5.3) | MUST | 5.2 | **positive:** `unit/verify` [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L125). **positive:** `unit/verify` [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L175). **negative:** `unit/verify` [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L579) |
| `RFC2516-5.3-1` | Host MUST echo AC-Cookie unchanged in PADR if AC included it in PADO (Encoding Rules, §5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L505). **negative:** `unit/verify` [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L548) |
| `RFC2516-x-5` | Relay-Session-Id, if present, MUST be included unchanged in all subsequent Discovery packets for the exchange (Encoding Rules) | MUST | x | **positive:** `unit/verify` [`TestRelaySessionIDEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L617). **negative:** `unit/verify` [`TestRelaySessionIDNoEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L660) |
| `RFC2516-5.1-1` | PADI MUST contain exactly one Service-Name tag (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L482). **negative:** no negative test. **{single-polarity}:** emit-side count: BuildPADI always writes exactly one Service-Name tag (discovery.go:307), and the AC does not enforce the count on receive (MatchServiceName tolerates zero or many, discovery.go:220) |
| `RFC2516-5.1-2` | PADI destination MUST be broadcast; non-broadcast PADI is silently discarded (§5.1, Encoding Rules) | MUST | 5.1 | **positive:** `unit/verify` [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L483). **negative:** no negative test. **{single-polarity}:** emit-side: BuildPADI addresses the PADI to the Ethernet broadcast MAC (discovery.go:305); the receive-side non-broadcast-PADI discard is not enforced (handlePADI checks only SID, server.go:53) |
| `RFC2516-5.2-2` | PADO MUST contain one AC-Name tag and one or more Service-Name tags (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** BuildPADO can emit a PADO carrying only an AC-Name and no Service-Name tag when the AC has no configured service names (the documented accept-any config, yang/ze-pppoe-conf.yang:35) and the PADI Service-Name is empty or absent (the discovery.go:339 loop adds none and discovery.go:344 skips the empty request), violating the one-or-more Service-Name MUST |
| `RFC2516-5.2-3` | PADO MUST echo Host-Uniq if present in PADI (§5.2) | MUST | 5.2 | **positive:** `unit/verify` [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L126). **negative:** `unit/verify` [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L580) |
| `RFC2516-5.2-4` | PADO MUST NOT be sent if AC cannot serve the requested Service-Name (§5.2) | MUST NOT | 5.2 | **positive:** `unit/verify` [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L242). **negative:** `unit/verify` [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L243) |
| `RFC2516-5.3-2` | PADR MUST contain exactly one Service-Name tag from the selected PADO (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L508). **negative:** no negative test. **{single-polarity}:** emit-side count: BuildPADR writes exactly one Service-Name tag (discovery.go:322); there is no receive-side count enforcement to give a negative |
| `RFC2516-5.3-3` | PADR MUST echo AC-Cookie if one was in the PADO (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L506). **negative:** `unit/verify` [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L549) |
| `RFC2516-5.3-4` | PADR MUST echo Host-Uniq if one was in the PADI (§5.3) | MUST | 5.3 | **positive:** `unit/verify` [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L507). **negative:** `unit/verify` [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L550) |
| `RFC2516-5.4-1` | PADS MUST contain exactly one Service-Name tag (§5.4) | MUST | 5.4 | **positive:** `unit/verify` [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L176). **negative:** no negative test. **{single-polarity}:** emit-side: BuildPADS echoes the single Service-Name from a conformant PADR (discovery.go:365); no negative case exists on the AC send path |
| `RFC2516-x-6` | PADO/PADR/PADS/PADT destination MUST be unicast; broadcast destination is silently discarded (Encoding Rules) | MUST | x | **positive:** `unit/verify` [`TestBuildPADT`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L219). **negative:** no negative test. **{single-polarity}:** emit-side: every builder addresses PADO/PADR/PADS/PADT to the peer unicast MAC derived from its unicast source (discovery.go:320, 336, 362, 387); the receive-side broadcast-destination discard is not enforced (ParseDiscovery validates only the source, discovery.go:108) |
| `RFC2516-x-7` | Source address MUST NOT be broadcast or multicast (Encoding Rules) | MUST NOT | x | **positive:** `unit/verify` [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L47). **negative:** `unit/verify` [`TestParseBroadcastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L329). **negative:** `unit/verify` [`TestParseMulticastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L339) |
| `RFC2516-x-8` | Unknown tags MUST NOT cause an error; silently ignore (Encoding Rules) | MUST | x | **positive:** `unit/verify` [`TestUnknownTagIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L680). **negative:** no negative test. **{single-polarity}:** tolerate requirement: parseTags stores unknown tag types generically and never errors on them (discovery.go:154); rejecting an unknown tag would itself violate the MUST NOT, so no negative exists |
| `RFC2516-x-9` | LCP MUST negotiate MRU to 1492 or lower unless both sides support RFC 4638 and the Ethernet path supports larger frames (MTU) | MUST | x | **positive:** `unit/verify` [`TestLCPConfigRequestMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L117). **negative:** no negative test. **{single-polarity}:** ceiling requirement: the client proposes MRU 1492 by default in its LCP Configure-Request (dialer.go:114, session.go:487) and the AC caps MaxMRU at PPPoEMaxMTU 1492 (server.go:203); 1492 is the maximum so there is no negative |
| `RFC2516-7-1` | AC MUST verify SESSION_ID + source MAC pair on every Session and PADT packet (Security, §7) | MUST | 7 | **positive:** `unit/verify` [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L9). **negative:** `unit/verify` [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L10) |
| `RFC2516-5.2-5` | PADO SHOULD contain AC-Cookie for DoS mitigation (§5.2) | SHOULD | 5.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.1-3` | PADI MAY contain Host-Uniq for client-side demux of PADO replies (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.5-1` | PADT MAY be sent by either side at any time after PADS (§5.5) | MAY | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2516-5.5-2` | PADT MAY contain Generic-Error tag (§5.5) | MAY | 5.5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2516-5.2-2`](#rfc2516-5.2-2) PADO MUST contain one AC-Name tag and one or more Service-Name tags (§5.2) | {gap}, no test | BuildPADO can emit a PADO carrying only an AC-Name and no Service-Name tag when the AC has no configured service names (the documented accept-any config, yang/ze-pppoe-conf.yang:35) and the PADI Service-Name is empty or absent (the discovery.go:339 loop adds none and discovery.go:344 skips the empty request), violating the one-or-more Service-Name MUST |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2516-x-1`](#rfc2516-x-1)

VER field MUST be 0x1 (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L302) | unit/verify | unproven |
| positive | [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L451) | unit/verify | unproven |

### [`RFC2516-x-2`](#rfc2516-x-2)

TYPE field MUST be 0x1 (Wire Format)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseBadType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L471) | unit/verify | unproven |
| positive | [`TestBuildFrameVerType`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L452) | unit/verify | unproven |

### [`RFC2516-x-3`](#rfc2516-x-3)

Packets with VER or TYPE other than 0x1 MUST be silently discarded (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseBadVersion`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L303) | unit/verify | unproven |
| positive | [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L46) | unit/verify | unproven |

### [`RFC2516-x-4`](#rfc2516-x-4)

End-Of-List tag TAG_LENGTH MUST be 0 (Tag Catalog)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParseEndOfListTag`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L362) | unit/verify | unproven |

### [`RFC2516-5.2-1`](#rfc2516-5.2-1)

AC MUST echo Host-Uniq unchanged in PADO and PADS if Host included it in PADI/PADR (Encoding Rules, §5.2, §5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L579) | unit/verify | unproven |
| positive | [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L125) | unit/verify | unproven |
| positive | [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L175) | unit/verify | unproven |

### [`RFC2516-5.3-1`](#rfc2516-5.3-1)

Host MUST echo AC-Cookie unchanged in PADR if AC included it in PADO (Encoding Rules, §5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L548) | unit/verify | unproven |
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L505) | unit/verify | unproven |

### [`RFC2516-x-5`](#rfc2516-x-5)

Relay-Session-Id, if present, MUST be included unchanged in all subsequent Discovery packets for the exchange (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRelaySessionIDNoEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L660) | unit/verify | unproven |
| positive | [`TestRelaySessionIDEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L617) | unit/verify | unproven |

### [`RFC2516-5.1-1`](#rfc2516-5.1-1)

PADI MUST contain exactly one Service-Name tag (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L482) | unit/verify | unproven |

### [`RFC2516-5.1-2`](#rfc2516-5.1-2)

PADI destination MUST be broadcast; non-broadcast PADI is silently discarded (§5.1, Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADIDiscovery`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L483) | unit/verify | unproven |

### [`RFC2516-5.2-2`](#rfc2516-5.2-2)

PADO MUST contain one AC-Name tag and one or more Service-Name tags (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2516-5.2-2, so no unit is bound to it.

### [`RFC2516-5.2-3`](#rfc2516-5.2-3)

PADO MUST echo Host-Uniq if present in PADI (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildNoHostUniqEcho`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L580) | unit/verify | unproven |
| positive | [`TestBuildPADO`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L126) | unit/verify | unproven |

### [`RFC2516-5.2-4`](#rfc2516-5.2-4)

PADO MUST NOT be sent if AC cannot serve the requested Service-Name (§5.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L243) | unit/verify | unproven |
| positive | [`TestServiceNameFilter`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L242) | unit/verify | unproven |

### [`RFC2516-5.3-2`](#rfc2516-5.3-2)

PADR MUST contain exactly one Service-Name tag from the selected PADO (§5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L508) | unit/verify | unproven |

### [`RFC2516-5.3-3`](#rfc2516-5.3-3)

PADR MUST echo AC-Cookie if one was in the PADO (§5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L549) | unit/verify | unproven |
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L506) | unit/verify | unproven |

### [`RFC2516-5.3-4`](#rfc2516-5.3-4)

PADR MUST echo Host-Uniq if one was in the PADI (§5.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestBuildPADRNoOptionalTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L550) | unit/verify | unproven |
| positive | [`TestBuildPADREchoesTags`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L507) | unit/verify | unproven |

### [`RFC2516-5.4-1`](#rfc2516-5.4-1)

PADS MUST contain exactly one Service-Name tag (§5.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADS`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L176) | unit/verify | unproven |

### [`RFC2516-x-6`](#rfc2516-x-6)

PADO/PADR/PADS/PADT destination MUST be unicast; broadcast destination is silently discarded (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestBuildPADT`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L219) | unit/verify | unproven |

### [`RFC2516-x-7`](#rfc2516-x-7)

Source address MUST NOT be broadcast or multicast (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestParseBroadcastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L329) | unit/verify | unproven |
| negative | [`TestParseMulticastSource`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L339) | unit/verify | unproven |
| positive | [`TestParsePADI`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L47) | unit/verify | unproven |

### [`RFC2516-x-8`](#rfc2516-x-8)

Unknown tags MUST NOT cause an error; silently ignore (Encoding Rules)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUnknownTagIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/discovery_test.go#L680) | unit/verify | unproven |

### [`RFC2516-x-9`](#rfc2516-x-9)

LCP MUST negotiate MRU to 1492 or lower unless both sides support RFC 4638 and the Ethernet path supports larger frames (MTU)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLCPConfigRequestMRU`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoeclient/session_test.go#L117) | unit/verify | unproven |

### [`RFC2516-7-1`](#rfc2516-7-1)

AC MUST verify SESSION_ID + source MAC pair on every Session and PADT packet (Security, §7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L10) | unit/verify | unproven |
| positive | [`TestHandlePADTVerifiesMACAndSID`](https://github.com/ze-software/ze/blob/main/internal/component/l2tp/pppoe/server_test.go#L9) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 2516, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2516, so its obligations are stated where they were written.
