# RFC 2132 - DHCP Options and BOOTP Vendor Extensions

Partial. Every requirement this repository extracted from RFC 2132, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 9 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 88.9% | 8 of 9 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 8 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 34 | of 68 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 25 | of 34 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 11.1% | 1 of 9 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 9 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 68 |
| Gated MUST-level | 34 |
| Obligations that bind Ze | 9 |
| Not applicable, so out of scope | 25 |
| Declared gaps | 1 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 8 |
| Tagged units | 8 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2132.md` |
| Requirement shard | `rfc/requirements/rfc2132.md` |
| RFC text | `rfc/full/rfc2132.txt` |

## Enrolment

Enrolled: DHCP Options and BOOTP Vendor Extensions (RFC 2132): DHCP server option TLV encoding. 8 single-polarity positive (subnet mask before router, router/DNS length multiple of 4, every option carries a length octet, unrecognized vendor-specific/class-specific info ignored, PXE class prefix-match) + 1 gap (RFC2132-9.8-1 Parameter Request List option 55 not honored, fixed option order) + 25 not-applicable (length MUSTs for options ze never emits, client-identifier uniqueness is a client obligation)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- DHCP server encodes options 1, 3, 6, 15, 51, 53, 54, 58, 59 and PXE options 43/60/66/67
- it parses received message type (53), requested IP (50), server identifier (54), and PXE class/arch (60/77/93). Tests bound per requirement in [`rfc/requirements/rfc2132.md`](https://github.com/ze-software/ze/blob/main/rfc/requirements/rfc2132.md).


**What the ledger says remains**

One MUST gap, tracked in [`rfc/short/rfc2132.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2132.md): [`RFC2132-9.8-1`](#rfc2132-9.8-1) -- the server emits a fixed option set in a fixed order (buildReply [`internal/plugins/dhcpserver/handler.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler.go)) and never reads the client Parameter Request List (option 55), so it does not honor the client-requested option order.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 34 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **34** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (34):** [`RFC2132-3.3-1`](#rfc2132-3.3-1), [`RFC2132-3.5-1`](#rfc2132-3.5-1), [`RFC2132-3.6-1`](#rfc2132-3.6-1), [`RFC2132-3.7-1`](#rfc2132-3.7-1), [`RFC2132-3.8-1`](#rfc2132-3.8-1), [`RFC2132-3.9-1`](#rfc2132-3.9-1), [`RFC2132-3.10-1`](#rfc2132-3.10-1), [`RFC2132-3.11-1`](#rfc2132-3.11-1), [`RFC2132-3.12-1`](#rfc2132-3.12-1), [`RFC2132-3.13-1`](#rfc2132-3.13-1), [`RFC2132-8.2-1`](#rfc2132-8.2-1), [`RFC2132-8.3-1`](#rfc2132-8.3-1), [`RFC2132-8.9-1`](#rfc2132-8.9-1), [`RFC2132-8.10-1`](#rfc2132-8.10-1), [`RFC2132-8.12-1`](#rfc2132-8.12-1), [`RFC2132-8.13-1`](#rfc2132-8.13-1), [`RFC2132-8.14-1`](#rfc2132-8.14-1), [`RFC2132-8.15-1`](#rfc2132-8.15-1), [`RFC2132-8.16-1`](#rfc2132-8.16-1), [`RFC2132-8.17-1`](#rfc2132-8.17-1), [`RFC2132-8.18-1`](#rfc2132-8.18-1), [`RFC2132-8.19-1`](#rfc2132-8.19-1), [`RFC2132-8.20-1`](#rfc2132-8.20-1), [`RFC2132-8.21-1`](#rfc2132-8.21-1), [`RFC2132-4.7-1`](#rfc2132-4.7-1), [`RFC2132-4.3-1`](#rfc2132-4.3-1), [`RFC2132-5.8-1`](#rfc2132-5.8-1), [`RFC2132-2-1`](#rfc2132-2-1), [`RFC2132-2-2`](#rfc2132-2-2), [`RFC2132-2-3`](#rfc2132-2-3), [`RFC2132-8.4-1`](#rfc2132-8.4-1), [`RFC2132-9.13-1`](#rfc2132-9.13-1), [`RFC2132-9.8-1`](#rfc2132-9.8-1), [`RFC2132-9.14-1`](#rfc2132-9.14-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2132-3.3-1` | If both subnet mask and router option are specified in a DHCP reply, the subnet mask option MUST be first (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestOptionSubnetMaskBeforeRouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1331). **negative:** no negative test. **{single-polarity}:** ze constructs every OFFER/ACK with the subnet mask (option 1) emitted before the router (option 3) in fixed code order (buildReply internal/plugins/dhcpserver/handler.go:251 then :255); no code path emits them reversed, so there is no out-of-order output to assert as a negative |
| `RFC2132-3.5-1` | Router option length MUST always be a multiple of 4 (§3.5) | MUST | 3.5 | **positive:** `unit/verify` [`TestEmittedIPListOptionLengthsMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1354). **negative:** no negative test. **{single-polarity}:** ze emits option 3 from a single netip.Addr via As4() = exactly 4 octets (buildReply internal/plugins/dhcpserver/handler.go:255), so the router option length is always a multiple of 4 and no code path can emit a non-multiple-of-4 router option to test negatively |
| `RFC2132-3.6-1` | Time server option length MUST always be a multiple of 4 (§3.6) | MUST | 3.6 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Time Server option (code 4), so this sender length constraint binds no ze code path |
| `RFC2132-3.7-1` | Name server option length MUST always be a multiple of 4 (§3.7) | MUST | 3.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Name Server option (code 5), so this sender length constraint binds no ze code path |
| `RFC2132-3.8-1` | Domain name server option length MUST always be a multiple of 4 (§3.8) | MUST | 3.8 | **positive:** `unit/verify` [`TestEmittedIPListOptionLengthsMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1363). **negative:** no negative test. **{single-polarity}:** ze emits option 6 by concatenating 4-octet As4() addresses (buildReply internal/plugins/dhcpserver/handler.go:258-264), so the DNS option length is always 4*n and no code path can emit a non-multiple-of-4 DNS option to test negatively |
| `RFC2132-3.9-1` | Log server option length MUST always be a multiple of 4 (§3.9) | MUST | 3.9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Log Server option (code 7), so this sender length constraint binds no ze code path |
| `RFC2132-3.10-1` | Cookie server option length MUST always be a multiple of 4 (§3.10) | MUST | 3.10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Cookie Server option (code 8), so this sender length constraint binds no ze code path |
| `RFC2132-3.11-1` | LPR server option length MUST always be a multiple of 4 (§3.11) | MUST | 3.11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the LPR Server option (code 9), so this sender length constraint binds no ze code path |
| `RFC2132-3.12-1` | Impress server option length MUST always be a multiple of 4 (§3.12) | MUST | 3.12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Impress Server option (code 10), so this sender length constraint binds no ze code path |
| `RFC2132-3.13-1` | Resource location server option length MUST always be a multiple of 4 (§3.13) | MUST | 3.13 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Resource Location Server option (code 11), so this sender length constraint binds no ze code path |
| `RFC2132-8.2-1` | NIS servers option length MUST be a multiple of 4 (§8.2) | MUST | 8.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the NIS Servers option (code 41), so this sender length constraint binds no ze code path |
| `RFC2132-8.3-1` | NTP servers option length MUST be a multiple of 4 (§8.3) | MUST | 8.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the NTP Servers option (code 42), so this sender length constraint binds no ze code path |
| `RFC2132-8.9-1` | X Window Font Server option length MUST be a multiple of 4 (§8.9) | MUST | 8.9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the X Window Font Server option (code 48), so this sender length constraint binds no ze code path |
| `RFC2132-8.10-1` | X Window Display Manager option length MUST be a multiple of 4 (§8.10) | MUST | 8.10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the X Window Display Manager option (code 49), so this sender length constraint binds no ze code path |
| `RFC2132-8.12-1` | NIS+ Servers option length MUST be a multiple of 4 (§8.12) | MUST | 8.12 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the NIS+ Servers option (code 65), so this sender length constraint binds no ze code path |
| `RFC2132-8.13-1` | Mobile IP Home Agent option length MUST be a multiple of 4 (§8.13) | MUST | 8.13 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Mobile IP Home Agent option (code 68), so this sender length constraint binds no ze code path |
| `RFC2132-8.14-1` | SMTP server option length MUST always be a multiple of 4 (§8.14) | MUST | 8.14 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the SMTP Server option (code 69), so this sender length constraint binds no ze code path |
| `RFC2132-8.15-1` | POP3 server option length MUST always be a multiple of 4 (§8.15) | MUST | 8.15 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the POP3 Server option (code 70), so this sender length constraint binds no ze code path |
| `RFC2132-8.16-1` | NNTP server option length MUST always be a multiple of 4 (§8.16) | MUST | 8.16 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the NNTP Server option (code 71), so this sender length constraint binds no ze code path |
| `RFC2132-8.17-1` | WWW server option length MUST always be a multiple of 4 (§8.17) | MUST | 8.17 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the WWW Server option (code 72), so this sender length constraint binds no ze code path |
| `RFC2132-8.18-1` | Finger server option length MUST always be a multiple of 4 (§8.18) | MUST | 8.18 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Finger Server option (code 73), so this sender length constraint binds no ze code path |
| `RFC2132-8.19-1` | IRC server option length MUST always be a multiple of 4 (§8.19) | MUST | 8.19 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the IRC Server option (code 74), so this sender length constraint binds no ze code path |
| `RFC2132-8.20-1` | StreetTalk server option length MUST always be a multiple of 4 (§8.20) | MUST | 8.20 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the StreetTalk Server option (code 75), so this sender length constraint binds no ze code path |
| `RFC2132-8.21-1` | STDA server option length MUST always be a multiple of 4 (§8.21) | MUST | 8.21 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the STDA Server option (code 76), so this sender length constraint binds no ze code path |
| `RFC2132-4.7-1` | Path MTU plateau table option length MUST be a multiple of 2 (§4.7) | MUST | 4.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Path MTU Plateau Table option (code 25), so this sender length constraint binds no ze code path |
| `RFC2132-4.3-1` | Policy filter option length MUST be a multiple of 8 (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Policy Filter option (code 21), so this sender length constraint binds no ze code path |
| `RFC2132-5.8-1` | Static route option length MUST be a multiple of 8 (§5.8) | MUST | 5.8 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Static Route option (code 33), so this sender length constraint binds no ze code path |
| `RFC2132-2-1` | Any options defined subsequent to this document MUST contain a length octet even if fixed or zero (§2) | MUST | 2 | **positive:** `unit/verify` [`TestEveryEmittedOptionHasLengthOctet`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1413). **negative:** no negative test. **{single-polarity}:** every option ze emits goes through safeAppendOption, which always writes a length octet (internal/plugins/dhcpserver/handler.go:361-363); only the exempt Pad/End markers are written without one, so ze emits no length-octet-less option to test negatively |
| `RFC2132-2-2` | Receiver MUST be prepared to delete trailing nulls from ASCII options (§2) | MUST | 2 | **positive:** `unit/verify` [`TestASCIIOptionParsingTolerantOfTrailingNull`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1452). **negative:** no negative test. **{single-polarity}:** ze reads ASCII-carrying options 60 and 77 only by fixed-prefix match (isPXEClient internal/plugins/dhcpserver/handler.go:493, isIPXE handler.go:498), so a trailing NUL never corrupts interpretation; no ze receive path rejects or mishandles ASCII option data because of a trailing NUL, so there is no negative case |
| `RFC2132-2-3` | Receiver MUST NOT require that a trailing null be included in ASCII data (§2) | MUST NOT | 2 | **positive:** `unit/verify` [`TestASCIIOptionParsingTolerantOfTrailingNull`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1444). **negative:** no negative test. **{single-polarity}:** ze's ASCII-option receiver prefix-matches options 60 and 77 (isPXEClient internal/plugins/dhcpserver/handler.go:493, isIPXE handler.go:498) and so never requires a trailing NUL; option data without one is accepted, and there is no ze path that demands a trailing NUL to test negatively |
| `RFC2132-8.4-1` | Servers not equipped to interpret vendor-specific information MUST ignore it (§8.4) | MUST | 8.4 | **positive:** `unit/verify` [`TestIgnoresClientVendorSpecificOption43`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1501). **negative:** no negative test. **{single-polarity}:** ze never reads a received option 43 -- vendor-specific information appears only in ze's Tx path (handler.go:325) and the option-parse loop skips any code it does not request (parseMsgType/parseOptionBytes advance past unrequested options, internal/plugins/dhcpserver/handler.go:367-403), so ze ignores client vendor-specific info and no code path interprets or rejects it |
| `RFC2132-9.13-1` | Servers not equipped to interpret class-specific information MUST ignore it (§9.13) | MUST | 9.13 | **positive:** `unit/verify` [`TestIgnoresUnknownVendorClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1548). **negative:** no negative test. **{single-polarity}:** ze inspects option 60 only for the "PXEClient:" prefix (isPXEClient internal/plugins/dhcpserver/handler.go:493-496); any other vendor class yields false and no class-specific handling, so an unrecognized class is ignored and no code path rejects a packet on class content to test negatively |
| `RFC2132-9.8-1` | Server MUST try to insert requested options in the order requested by the client (§9.8) | MUST | 9.8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze emits a fixed option set in a fixed order (buildReply internal/plugins/dhcpserver/handler.go:245-285) and never reads the client Parameter Request List (option 55 is defined at handler.go:52 but parsed nowhere in production), so it does not try to insert requested options in the client's requested order |
| `RFC2132-9.14-1` | Each client's client-identifier MUST be unique among identifiers on the subnet (§9.14) | MUST | 9.14 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the uniqueness obligation binds the client's choice of client-identifier (option 61); ze is a server that keys leases and pool allocations by hardware address/chaddr (extractMAC internal/plugins/dhcpserver/handler.go:456, pool.allocate pool.go:64, leaseTable byMAC lease.go:23) and never reads or generates option 61, so it neither produces client-identifiers nor can enforce cross-client uniqueness |
| `RFC2132-2-4` | Options containing NVT ASCII data SHOULD NOT include a trailing NULL (§2) | SHOULD NOT | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-3.5-2` | Routers SHOULD be listed in order of preference (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-3.6-2` | Time servers SHOULD be listed in order of preference (§3.6) | SHOULD | 3.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-3.7-2` | Name servers SHOULD be listed in order of preference (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-3.8-2` | Domain name servers SHOULD be listed in order of preference (§3.8) | SHOULD | 3.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-3.9-2` | Log servers SHOULD be listed in order of preference (§3.9) | SHOULD | 3.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-3.10-2` | Cookie servers SHOULD be listed in order of preference (§3.10) | SHOULD | 3.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-3.11-2` | LPR servers SHOULD be listed in order of preference (§3.11) | SHOULD | 3.11 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-3.12-2` | Impress servers SHOULD be listed in order of preference (§3.12) | SHOULD | 3.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-3.13-2` | Resource location servers SHOULD be listed in order of preference (§3.13) | SHOULD | 3.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.2-2` | NIS servers SHOULD be listed in order of preference (§8.2) | SHOULD | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.3-2` | NTP servers SHOULD be listed in order of preference (§8.3) | SHOULD | 8.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.4-2` | Clients not receiving desired vendor-specific information SHOULD make an attempt to operate without it (§8.4) | SHOULD | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.9-2` | X Window Font servers SHOULD be listed in order of preference (§8.9) | SHOULD | 8.9 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.10-2` | X Window Display Manager addresses SHOULD be listed in order of preference (§8.10) | SHOULD | 8.10 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.12-2` | NIS+ servers SHOULD be listed in order of preference (§8.12) | SHOULD | 8.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.13-2` | Mobile IP home agents SHOULD be listed in order of preference (§8.13) | SHOULD | 8.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.14-2` | SMTP servers SHOULD be listed in order of preference (§8.14) | SHOULD | 8.14 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.15-2` | POP3 servers SHOULD be listed in order of preference (§8.15) | SHOULD | 8.15 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.16-2` | NNTP servers SHOULD be listed in order of preference (§8.16) | SHOULD | 8.16 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.17-2` | WWW servers SHOULD be listed in order of preference (§8.17) | SHOULD | 8.17 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.18-2` | Finger servers SHOULD be listed in order of preference (§8.18) | SHOULD | 8.18 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.19-2` | IRC servers SHOULD be listed in order of preference (§8.19) | SHOULD | 8.19 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.20-2` | StreetTalk servers SHOULD be listed in order of preference (§8.20) | SHOULD | 8.20 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.21-2` | STDA servers SHOULD be listed in order of preference (§8.21) | SHOULD | 8.21 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-9.13-2` | Servers responding to vendor class identifier SHOULD only use option 43 to return vendor-specific information (§9.13) | SHOULD | 9.13 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-9.14-2` | Client identifiers SHOULD be treated as opaque objects by DHCP servers (§9.14) | SHOULD | 9.14 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-9.14-3` | Client identifier type field SHOULD be one of the ARP hardware types from STD 2 (§9.14) | SHOULD | 9.14 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.4-3` | Vendor SHOULD encode multiple items in vendor-specific information using encapsulated vendor-specific options (§8.4) | SHOULD | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.4-4` | Encapsulated vendor-specific extensions SHOULD NOT contain a magic cookie field (§8.4) | SHOULD NOT | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.4-5` | Encapsulated vendor-specific option codes SHOULD conform to the tag-length-value syntax (§8.4) | SHOULD | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-9.8-2` | Client MAY list options in order of preference in Parameter Request List (§9.8) | MAY | 9.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-9.14-4` | Client identifier MAY consist of type-value pairs similar to htype/chaddr (§9.14) | MAY | 9.14 | **positive:** no positive test. **negative:** no negative test |
| `RFC2132-8.4-6` | Vendor-specific option codes other than 0 or 255 MAY be redefined within the encapsulated field (§8.4) | MAY | 8.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2132-3.6-1`](#rfc2132-3.6-1) Time server option length MUST always be a multiple of 4 (§3.6) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Time Server option (code 4), so this sender length constraint binds no ze code path |
| [`RFC2132-3.7-1`](#rfc2132-3.7-1) Name server option length MUST always be a multiple of 4 (§3.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Name Server option (code 5), so this sender length constraint binds no ze code path |
| [`RFC2132-3.9-1`](#rfc2132-3.9-1) Log server option length MUST always be a multiple of 4 (§3.9) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Log Server option (code 7), so this sender length constraint binds no ze code path |
| [`RFC2132-3.10-1`](#rfc2132-3.10-1) Cookie server option length MUST always be a multiple of 4 (§3.10) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Cookie Server option (code 8), so this sender length constraint binds no ze code path |
| [`RFC2132-3.11-1`](#rfc2132-3.11-1) LPR server option length MUST always be a multiple of 4 (§3.11) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the LPR Server option (code 9), so this sender length constraint binds no ze code path |
| [`RFC2132-3.12-1`](#rfc2132-3.12-1) Impress server option length MUST always be a multiple of 4 (§3.12) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Impress Server option (code 10), so this sender length constraint binds no ze code path |
| [`RFC2132-3.13-1`](#rfc2132-3.13-1) Resource location server option length MUST always be a multiple of 4 (§3.13) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Resource Location Server option (code 11), so this sender length constraint binds no ze code path |
| [`RFC2132-8.2-1`](#rfc2132-8.2-1) NIS servers option length MUST be a multiple of 4 (§8.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the NIS Servers option (code 41), so this sender length constraint binds no ze code path |
| [`RFC2132-8.3-1`](#rfc2132-8.3-1) NTP servers option length MUST be a multiple of 4 (§8.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the NTP Servers option (code 42), so this sender length constraint binds no ze code path |
| [`RFC2132-8.9-1`](#rfc2132-8.9-1) X Window Font Server option length MUST be a multiple of 4 (§8.9) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the X Window Font Server option (code 48), so this sender length constraint binds no ze code path |
| [`RFC2132-8.10-1`](#rfc2132-8.10-1) X Window Display Manager option length MUST be a multiple of 4 (§8.10) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the X Window Display Manager option (code 49), so this sender length constraint binds no ze code path |
| [`RFC2132-8.12-1`](#rfc2132-8.12-1) NIS+ Servers option length MUST be a multiple of 4 (§8.12) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the NIS+ Servers option (code 65), so this sender length constraint binds no ze code path |
| [`RFC2132-8.13-1`](#rfc2132-8.13-1) Mobile IP Home Agent option length MUST be a multiple of 4 (§8.13) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Mobile IP Home Agent option (code 68), so this sender length constraint binds no ze code path |
| [`RFC2132-8.14-1`](#rfc2132-8.14-1) SMTP server option length MUST always be a multiple of 4 (§8.14) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the SMTP Server option (code 69), so this sender length constraint binds no ze code path |
| [`RFC2132-8.15-1`](#rfc2132-8.15-1) POP3 server option length MUST always be a multiple of 4 (§8.15) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the POP3 Server option (code 70), so this sender length constraint binds no ze code path |
| [`RFC2132-8.16-1`](#rfc2132-8.16-1) NNTP server option length MUST always be a multiple of 4 (§8.16) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the NNTP Server option (code 71), so this sender length constraint binds no ze code path |
| [`RFC2132-8.17-1`](#rfc2132-8.17-1) WWW server option length MUST always be a multiple of 4 (§8.17) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the WWW Server option (code 72), so this sender length constraint binds no ze code path |
| [`RFC2132-8.18-1`](#rfc2132-8.18-1) Finger server option length MUST always be a multiple of 4 (§8.18) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Finger Server option (code 73), so this sender length constraint binds no ze code path |
| [`RFC2132-8.19-1`](#rfc2132-8.19-1) IRC server option length MUST always be a multiple of 4 (§8.19) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the IRC Server option (code 74), so this sender length constraint binds no ze code path |
| [`RFC2132-8.20-1`](#rfc2132-8.20-1) StreetTalk server option length MUST always be a multiple of 4 (§8.20) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the StreetTalk Server option (code 75), so this sender length constraint binds no ze code path |
| [`RFC2132-8.21-1`](#rfc2132-8.21-1) STDA server option length MUST always be a multiple of 4 (§8.21) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the STDA Server option (code 76), so this sender length constraint binds no ze code path |
| [`RFC2132-4.7-1`](#rfc2132-4.7-1) Path MTU plateau table option length MUST be a multiple of 2 (§4.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Path MTU Plateau Table option (code 25), so this sender length constraint binds no ze code path |
| [`RFC2132-4.3-1`](#rfc2132-4.3-1) Policy filter option length MUST be a multiple of 8 (§4.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Policy Filter option (code 21), so this sender length constraint binds no ze code path |
| [`RFC2132-5.8-1`](#rfc2132-5.8-1) Static route option length MUST be a multiple of 8 (§5.8) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCP server emits only options 1/3/6/15/51/53/54/58/59 and PXE 43/60/66/67 (buildReply internal/plugins/dhcpserver/handler.go:245-285, appendPXEOptions handler.go:298-325); it never emits the Static Route option (code 33), so this sender length constraint binds no ze code path |
| [`RFC2132-9.8-1`](#rfc2132-9.8-1) Server MUST try to insert requested options in the order requested by the client (§9.8) | {gap}, no test | ze emits a fixed option set in a fixed order (buildReply internal/plugins/dhcpserver/handler.go:245-285) and never reads the client Parameter Request List (option 55 is defined at handler.go:52 but parsed nowhere in production), so it does not try to insert requested options in the client's requested order |
| [`RFC2132-9.14-1`](#rfc2132-9.14-1) Each client's client-identifier MUST be unique among identifiers on the subnet (§9.14) | no test | no test carries this requirement id; annotated {not-applicable}: the uniqueness obligation binds the client's choice of client-identifier (option 61); ze is a server that keys leases and pool allocations by hardware address/chaddr (extractMAC internal/plugins/dhcpserver/handler.go:456, pool.allocate pool.go:64, leaseTable byMAC lease.go:23) and never reads or generates option 61, so it neither produces client-identifiers nor can enforce cross-client uniqueness |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2132-3.3-1`](#rfc2132-3.3-1)

If both subnet mask and router option are specified in a DHCP reply, the subnet mask option MUST be first (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOptionSubnetMaskBeforeRouter`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1331) | unit/verify | unproven |

### [`RFC2132-3.5-1`](#rfc2132-3.5-1)

Router option length MUST always be a multiple of 4 (§3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEmittedIPListOptionLengthsMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1354) | unit/verify | unproven |

### [`RFC2132-3.6-1`](#rfc2132-3.6-1)

Time server option length MUST always be a multiple of 4 (§3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-3.6-1, so no unit is bound to it.

### [`RFC2132-3.7-1`](#rfc2132-3.7-1)

Name server option length MUST always be a multiple of 4 (§3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-3.7-1, so no unit is bound to it.

### [`RFC2132-3.8-1`](#rfc2132-3.8-1)

Domain name server option length MUST always be a multiple of 4 (§3.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEmittedIPListOptionLengthsMultipleOfFour`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1363) | unit/verify | unproven |

### [`RFC2132-3.9-1`](#rfc2132-3.9-1)

Log server option length MUST always be a multiple of 4 (§3.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-3.9-1, so no unit is bound to it.

### [`RFC2132-3.10-1`](#rfc2132-3.10-1)

Cookie server option length MUST always be a multiple of 4 (§3.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-3.10-1, so no unit is bound to it.

### [`RFC2132-3.11-1`](#rfc2132-3.11-1)

LPR server option length MUST always be a multiple of 4 (§3.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-3.11-1, so no unit is bound to it.

### [`RFC2132-3.12-1`](#rfc2132-3.12-1)

Impress server option length MUST always be a multiple of 4 (§3.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-3.12-1, so no unit is bound to it.

### [`RFC2132-3.13-1`](#rfc2132-3.13-1)

Resource location server option length MUST always be a multiple of 4 (§3.13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-3.13-1, so no unit is bound to it.

### [`RFC2132-8.2-1`](#rfc2132-8.2-1)

NIS servers option length MUST be a multiple of 4 (§8.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.2-1, so no unit is bound to it.

### [`RFC2132-8.3-1`](#rfc2132-8.3-1)

NTP servers option length MUST be a multiple of 4 (§8.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.3-1, so no unit is bound to it.

### [`RFC2132-8.9-1`](#rfc2132-8.9-1)

X Window Font Server option length MUST be a multiple of 4 (§8.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.9-1, so no unit is bound to it.

### [`RFC2132-8.10-1`](#rfc2132-8.10-1)

X Window Display Manager option length MUST be a multiple of 4 (§8.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.10-1, so no unit is bound to it.

### [`RFC2132-8.12-1`](#rfc2132-8.12-1)

NIS+ Servers option length MUST be a multiple of 4 (§8.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.12-1, so no unit is bound to it.

### [`RFC2132-8.13-1`](#rfc2132-8.13-1)

Mobile IP Home Agent option length MUST be a multiple of 4 (§8.13)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.13-1, so no unit is bound to it.

### [`RFC2132-8.14-1`](#rfc2132-8.14-1)

SMTP server option length MUST always be a multiple of 4 (§8.14)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.14-1, so no unit is bound to it.

### [`RFC2132-8.15-1`](#rfc2132-8.15-1)

POP3 server option length MUST always be a multiple of 4 (§8.15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.15-1, so no unit is bound to it.

### [`RFC2132-8.16-1`](#rfc2132-8.16-1)

NNTP server option length MUST always be a multiple of 4 (§8.16)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.16-1, so no unit is bound to it.

### [`RFC2132-8.17-1`](#rfc2132-8.17-1)

WWW server option length MUST always be a multiple of 4 (§8.17)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.17-1, so no unit is bound to it.

### [`RFC2132-8.18-1`](#rfc2132-8.18-1)

Finger server option length MUST always be a multiple of 4 (§8.18)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.18-1, so no unit is bound to it.

### [`RFC2132-8.19-1`](#rfc2132-8.19-1)

IRC server option length MUST always be a multiple of 4 (§8.19)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.19-1, so no unit is bound to it.

### [`RFC2132-8.20-1`](#rfc2132-8.20-1)

StreetTalk server option length MUST always be a multiple of 4 (§8.20)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.20-1, so no unit is bound to it.

### [`RFC2132-8.21-1`](#rfc2132-8.21-1)

STDA server option length MUST always be a multiple of 4 (§8.21)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-8.21-1, so no unit is bound to it.

### [`RFC2132-4.7-1`](#rfc2132-4.7-1)

Path MTU plateau table option length MUST be a multiple of 2 (§4.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-4.7-1, so no unit is bound to it.

### [`RFC2132-4.3-1`](#rfc2132-4.3-1)

Policy filter option length MUST be a multiple of 8 (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-4.3-1, so no unit is bound to it.

### [`RFC2132-5.8-1`](#rfc2132-5.8-1)

Static route option length MUST be a multiple of 8 (§5.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-5.8-1, so no unit is bound to it.

### [`RFC2132-2-1`](#rfc2132-2-1)

Any options defined subsequent to this document MUST contain a length octet even if fixed or zero (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEveryEmittedOptionHasLengthOctet`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1413) | unit/verify | unproven |

### [`RFC2132-2-2`](#rfc2132-2-2)

Receiver MUST be prepared to delete trailing nulls from ASCII options (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestASCIIOptionParsingTolerantOfTrailingNull`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1452) | unit/verify | unproven |

### [`RFC2132-2-3`](#rfc2132-2-3)

Receiver MUST NOT require that a trailing null be included in ASCII data (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestASCIIOptionParsingTolerantOfTrailingNull`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1444) | unit/verify | unproven |

### [`RFC2132-8.4-1`](#rfc2132-8.4-1)

Servers not equipped to interpret vendor-specific information MUST ignore it (§8.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIgnoresClientVendorSpecificOption43`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1501) | unit/verify | unproven |

### [`RFC2132-9.13-1`](#rfc2132-9.13-1)

Servers not equipped to interpret class-specific information MUST ignore it (§9.13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestIgnoresUnknownVendorClass`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/handler_test.go#L1548) | unit/verify | unproven |

### [`RFC2132-9.8-1`](#rfc2132-9.8-1)

Server MUST try to insert requested options in the order requested by the client (§9.8)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-9.8-1, so no unit is bound to it.

### [`RFC2132-9.14-1`](#rfc2132-9.14-1)

Each client's client-identifier MUST be unique among identifiers on the subnet (§9.14)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2132-9.14-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 2132, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2132, so its obligations are stated where they were written.
