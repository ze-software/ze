# RFC 2131 - Dynamic Host Configuration Protocol

Partial. Every requirement this repository extracted from RFC 2131, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 40.5% | 15 of 37 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 10.8% | 4 of 37 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 37 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 34 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 64 | of 101 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 27 | of 64 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 48.6% | 18 of 37 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 37 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 101 |
| Gated MUST-level | 64 |
| Obligations that bind Ze | 37 |
| Not applicable, so out of scope | 27 |
| Declared gaps | 18 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 34 |
| Tagged units | 34 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc2131.md` |
| Requirement shard | `rfc/requirements/rfc2131.md` |
| RFC text | `rfc/full/rfc2131.txt` |

## Enrolment

Enrolled: Dynamic Host Configuration Protocol

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Both roles. Server: DORA, leases, static mappings, PXE support -- every OFFER/ACK/NAK carries the server identifier, OFFER and ACK carry the lease time and ordered T1/T2 timers, no client-only option (50, 55, 57, 61) is ever echoed back, a NAK carries nothing beyond the message type and server identifier, the magic cookie is required on input and emitted on output, and options are read and written strictly inside the options field. Client: the `iface-dhcp` plugin (`internal/plugins/iface/dhcp/`) runs a long-lived DHCPv4 client per interface unit -- ze owns the lease state machine (T1/T2 arithmetic, renewal, expiry teardown, address and default-route installation) and authors options 12 and 61, while DORA and RENEWING message construction belong to the vendored `nclient4` library. Tests bound per requirement in [`rfc/short/rfc2131.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2131.md).

**What the ledger says remains**

Eighteen MUST gaps, each annotated in [`rfc/short/rfc2131.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc2131.md).

- **Server:** DHCPNAK delivery ([`RFC2131-4.3.2-1`](#rfc2131-4.3.2-1) unicasts to a non-zero ciaddr instead of broadcasting, [`RFC2131-4.3.2-2`](#rfc2131-4.3.2-2) never forces the broadcast bit behind a relay); INIT-REBOOT for an unknown client draws an ACK rather than silence ([`RFC2131-4.3.2-3`](#rfc2131-4.3.2-3)); a declined address is handed back to the client that declined it ([`RFC2131-4.3.3-1`](#rfc2131-4.3.3-1)); only one address per subnet is accepted as the server identifier ([`RFC2131-4.1-1`](#rfc2131-4.1-1)); with no default-router configured the server identifier falls back to a pool address or the subnet network address, neither of which the server answers on ([`RFC2131-4.1-2`](#rfc2131-4.1-2)); the client identifier option 61 is never read, so every client is keyed by chaddr ([`RFC2131-4.2-1`](#rfc2131-4.2-1)); the Parameter Request List is never parsed, so the section 4.3.1 selection rules govern no code path ([`RFC2131-4.3.1-1`](#rfc2131-4.3.1-1), 4.3.1-3, 4.3.1-5); and the vendor class is matched by "PXEClient:" prefix rather than exactly ([`RFC2131-4.3.1-7`](#rfc2131-4.3.1-7)).
- **Client:** option 61 is sent at acquisition and omitted on every renewal ([`RFC2131-2-2`](#rfc2131-2-2)); the configured client-id is emitted with no uniqueness check ([`RFC2131-2-1`](#rfc2131-2-1)); an address found in use is kept rather than declined, and no DHCPDECLINE path exists ([`RFC2131-3.1-7`](#rfc2131-3.1-7)); retransmission doubles the timeout with no randomization ([`RFC2131-4.1-8`](#rfc2131-4.1-8)); the renewal is broadcast rather than unicast to the server identifier ([`RFC2131-4.1-10`](#rfc2131-4.1-10)); the lease default route survives around 70 seconds past expiry because blocking renewal attempts add to a fixed sleep budget ([`RFC2131-4.4.5-1`](#rfc2131-4.4.5-1)); and a renewal ACK carrying a different yiaddr leaves the previous address installed ([`RFC2131-4.4.5-5`](#rfc2131-4.4.5-5)). DHCPINFORM is answered by no code path and sent by none, option overload (52) is neither emitted nor honored, and the remaining client-role MUSTs are not-applicable because ze produces none of the governed bytes: the vendored nclient4 library constructs them, or ze never enters the state (INIT-REBOOT, DECLINE, RELEASE, INFORM).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 15 | one part of the gated population |
| Annotated instead of tested | 49 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **64** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (15):** [`RFC2131-4.3-4`](#rfc2131-4.3-4), [`RFC2131-4.3-5`](#rfc2131-4.3-5), [`RFC2131-4.3-6`](#rfc2131-4.3-6), [`RFC2131-4.3-7`](#rfc2131-4.3-7), [`RFC2131-4.3-8`](#rfc2131-4.3-8), [`RFC2131-4.3-9`](#rfc2131-4.3-9), [`RFC2131-4.2-2`](#rfc2131-4.2-2), [`RFC2131-3-1`](#rfc2131-3-1), [`RFC2131-4.1-3`](#rfc2131-4.1-3), [`RFC2131-4.1-6`](#rfc2131-4.1-6), [`RFC2131-2-3`](#rfc2131-2-3), [`RFC2131-4.4.5-2`](#rfc2131-4.4.5-2), [`RFC2131-4.4.5-3`](#rfc2131-4.4.5-3), [`RFC2131-4.3.1-4`](#rfc2131-4.3.1-4), [`RFC2131-4.3.1-6`](#rfc2131-4.3.1-6)

**Annotated instead of tested (49):** [`RFC2131-4.3-1`](#rfc2131-4.3-1), [`RFC2131-4.3-2`](#rfc2131-4.3-2), [`RFC2131-4.3-3`](#rfc2131-4.3-3), [`RFC2131-4.3.5-1`](#rfc2131-4.3.5-1), [`RFC2131-4.3.2-1`](#rfc2131-4.3.2-1), [`RFC2131-4.3.2-2`](#rfc2131-4.3.2-2), [`RFC2131-4.3.2-3`](#rfc2131-4.3.2-3), [`RFC2131-4.3.3-1`](#rfc2131-4.3.3-1), [`RFC2131-4.1-1`](#rfc2131-4.1-1), [`RFC2131-4.1-2`](#rfc2131-4.1-2), [`RFC2131-4.2-1`](#rfc2131-4.2-1), [`RFC2131-2-1`](#rfc2131-2-1), [`RFC2131-2-2`](#rfc2131-2-2), [`RFC2131-3.1-1`](#rfc2131-3.1-1), [`RFC2131-3.1-2`](#rfc2131-3.1-2), [`RFC2131-3.1-3`](#rfc2131-3.1-3), [`RFC2131-3.1-4`](#rfc2131-3.1-4), [`RFC2131-3.1-5`](#rfc2131-3.1-5), [`RFC2131-3.2-1`](#rfc2131-3.2-1), [`RFC2131-3.1-6`](#rfc2131-3.1-6), [`RFC2131-3.1-7`](#rfc2131-3.1-7), [`RFC2131-4.4.5-1`](#rfc2131-4.4.5-1), [`RFC2131-4.1-4`](#rfc2131-4.1-4), [`RFC2131-4.1-5`](#rfc2131-4.1-5), [`RFC2131-4.1-7`](#rfc2131-4.1-7), [`RFC2131-4.1-8`](#rfc2131-4.1-8), [`RFC2131-4.1-9`](#rfc2131-4.1-9), [`RFC2131-4.1-10`](#rfc2131-4.1-10), [`RFC2131-3.4-1`](#rfc2131-3.4-1), [`RFC2131-2-4`](#rfc2131-2-4), [`RFC2131-3.5-1`](#rfc2131-3.5-1), [`RFC2131-3.1-8`](#rfc2131-3.1-8), [`RFC2131-4.3.1-1`](#rfc2131-4.3.1-1), [`RFC2131-4.3.1-2`](#rfc2131-4.3.1-2), [`RFC2131-4.3.1-3`](#rfc2131-4.3.1-3), [`RFC2131-4.3.1-5`](#rfc2131-4.3.1-5), [`RFC2131-4.3.1-7`](#rfc2131-4.3.1-7), [`RFC2131-4.4.1-1`](#rfc2131-4.4.1-1), [`RFC2131-4.4.2-1`](#rfc2131-4.4.2-1), [`RFC2131-4.4.2-2`](#rfc2131-4.4.2-2), [`RFC2131-4.4.3-1`](#rfc2131-4.4.3-1), [`RFC2131-4.3.2-5`](#rfc2131-4.3.2-5), [`RFC2131-4.4.5-5`](#rfc2131-4.4.5-5), [`RFC2131-4.4.5-6`](#rfc2131-4.4.5-6), [`RFC2131-4.4.5-7`](#rfc2131-4.4.5-7), [`RFC2131-3.2-3`](#rfc2131-3.2-3), [`RFC2131-4.4-1`](#rfc2131-4.4-1), [`RFC2131-4.4-2`](#rfc2131-4.4-2), [`RFC2131-4.4-3`](#rfc2131-4.4-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2131-4.3-1` | Server identifier MUST be included in DHCPOFFER, DHCPACK, and DHCPNAK (§4.3, Table 3) | MUST | 4.3 | **positive:** `unit/verify` [`TestServerIdentifierInEveryReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L169). **negative:** no negative test. **{single-polarity}:** every reply path appends option 54 unconditionally -- buildReply at internal/plugins/dhcpserver/handler.go:246 and buildNak at handler.go:349 -- so no input suppresses the server identifier and there is no omission to assert negatively |
| `RFC2131-4.3-2` | IP address lease time MUST be included in DHCPOFFER (§4.3, Table 3) | MUST | 4.3 | **positive:** `unit/verify` [`TestLeaseTimeInOfferAndAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L194). **negative:** no negative test. **{single-polarity}:** the lease time is appended to every OFFER and ACK unconditionally (buildReply internal/plugins/dhcpserver/handler.go:271-273, inside the OFFER/ACK branch opened at handler.go:248), so no input yields a DHCPOFFER without option 51 to assert negatively |
| `RFC2131-4.3-3` | IP address lease time MUST be included in DHCPACK for DHCPREQUEST (§4.3, Table 3) | MUST | 4.3 | **positive:** `unit/verify` [`TestLeaseTimeInOfferAndAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L198). **negative:** no negative test. **{single-polarity}:** the lease time is appended to every OFFER and ACK unconditionally (buildReply internal/plugins/dhcpserver/handler.go:271-273, inside the OFFER/ACK branch opened at handler.go:248), so no DHCPREQUEST yields a DHCPACK without option 51 to assert negatively |
| `RFC2131-4.3-4` | IP address lease time MUST NOT be included in DHCPNAK (§4.3, Table 3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L245). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L333) |
| `RFC2131-4.3-5` | Requested IP address option MUST NOT be included in DHCPOFFER, DHCPACK, or DHCPNAK (§4.3, Table 3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L224). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L312) |
| `RFC2131-4.3-6` | Client identifier MUST NOT be included in DHCPOFFER or DHCPACK (§4.3, Table 3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L239). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L327) |
| `RFC2131-4.3-7` | Parameter request list MUST NOT be included in DHCPOFFER, DHCPACK, or DHCPNAK (§4.3, Table 3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L228). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L316) |
| `RFC2131-4.3-8` | Maximum message size MUST NOT be included in DHCPOFFER, DHCPACK, or DHCPNAK (§4.3, Table 3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L232). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L320) |
| `RFC2131-4.3-9` | Options other than message MUST NOT be included in DHCPNAK (§4.3, Table 3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L249). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L337) |
| `RFC2131-4.3.5-1` | Server MUST NOT send lease expiration time for DHCPINFORM (§4.3.5) | MUST NOT | 4.3.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze answers no DHCPINFORM at all -- the message-type switch in handle (internal/plugins/dhcpserver/handler.go:121-134) dispatches DISCOVER, REQUEST, RELEASE and DECLINE, and every other type including msgInform (handler.go:37) falls to the default branch that returns nil, so ze builds no DHCPINFORM response at all |
| `RFC2131-4.3.2-1` | When giaddr is 0x0, server MUST broadcast DHCPNAK to 0xFFFFFFFF (§4.3.2, §3.2) | MUST | 4.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with giaddr zero the delivery path unicasts a DHCPNAK to a non-zero ciaddr instead of broadcasting -- responseAddr tests giaddr, then ciaddr, before it ever reaches the broadcast decision (internal/plugins/dhcpserver/register.go:275-298) and never special-cases the NAK message type, so a REQUEST whose ciaddr is on-subnet and whose requested address is off-subnet is NAKed to ciaddr:68 |
| `RFC2131-4.3.2-2` | When giaddr is set in DHCPNAK, server MUST set the broadcast bit (§4.3.2) | MUST | 4.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** buildNak copies the client's flags word verbatim (internal/plugins/dhcpserver/handler.go:339) and never forces the BROADCAST bit, so a DHCPNAK relayed through a non-zero giaddr leaves the server with the broadcast bit clear whenever the client left it clear |
| `RFC2131-4.3.2-3` | If server has no record of client in INIT-REBOOT, server MUST remain silent (§4.3.2) | MUST | 4.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the INIT-REBOOT branch commits a binding for any in-subnet requested address without consulting the lease table (handleRequest internal/plugins/dhcpserver/handler.go:178-183 calls commitBinding at handler.go:194), so a client the server holds no record of draws a DHCPACK rather than silence |
| `RFC2131-4.3.3-1` | Server MUST mark the network address as unavailable on DHCPDECLINE (§4.3.3) | MUST | 4.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** markUnavailable sets the pool bit and the static set for the declined address (internal/plugins/dhcpserver/pool.go:144-163), but the declining client's MAC-to-address cache entry survives -- pool.release returns early for a staticSet address (pool.go:174-177) -- so pool.allocate hands that same declined address back to that client on its next DISCOVER (pool.go:68-72) |
| `RFC2131-4.1-1` | A server with multiple network addresses MUST be prepared to accept any of its addresses as identifying that server (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** each handler accepts exactly one address as its server identifier -- handleRequest discards a REQUEST whose option 54 differs from its own serverIP (internal/plugins/dhcpserver/handler.go:167-169) -- and that address is the single per-subnet value derived at register.go:93-101, so a REQUEST naming another of the server's own addresses is silently dropped |
| `RFC2131-4.1-2` | Server MUST choose a server identifier address reachable from the client (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with no default-router configured for a subnet the server identifier is not an address the server answers on -- register.go:93 prefers sub.DefaultRouter, which parseSubnet leaves unset when the leaf is absent (internal/plugins/dhcpserver/config.go:189-195), and falls back to the first pool address (internal/plugins/dhcpserver/register.go:95-97), which the server itself hands out to a client, or to the subnet network address (register.go:99-101), which is no host at all; buildReply then emits that value as option 54 (internal/plugins/dhcpserver/handler.go:246), so a client unicasting to it reaches nothing |
| `RFC2131-4.2-1` | If client supplies a client identifier, server MUST use it to identify the client (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never reads the client identifier -- option 61 is absent from the option codes at internal/plugins/dhcpserver/handler.go:41-61 and no parse call asks for it -- and keys every binding on the hardware address instead (extractMAC handler.go:456, leaseTable byMAC lease.go:23, pool.macToAddr pool.go:22), so a client that supplies a client identifier is still identified by chaddr |
| `RFC2131-4.2-2` | If client does not provide a client identifier, server MUST use chaddr to identify the client (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestChaddrIdentifiesClientWithoutClientIdentifier`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L366). **negative:** `unit/verify` [`TestUnidentifiableClientRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L390) |
| `RFC2131-2-1` | Client identifier MUST be unique within the subnet (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's client emits the operator-configured client identifier verbatim and nothing derives or checks it -- v4RequestModifiers appends OptClientIdentifier([]byte(c.config.ClientID)) (internal/plugins/iface/dhcp/dhcp_v4_linux.go:304-306) from the config leaf read at internal/component/iface/config.go:1267-1269 and carried at internal/component/iface/register.go:891, and no validator constrains the value, so two units on one subnet configured with the same client-id emit colliding identifiers |
| `RFC2131-2-2` | If client uses a client identifier in one message, it MUST use the same identifier in all subsequent messages (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's client sends option 61 at acquisition and omits it on every renewal -- the DORA call passes v4RequestModifiers() (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, built at :299-308) while renewV4 calls client.Renew(ctx, lease) with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143), and the renewal REQUEST is built by NewRenewFromAck (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290) whose WithReply copies only opcode, HW type, xid, chaddr and flags (vendor/github.com/insomniacslk/dhcp/dhcpv4/modifiers.go:56-68) and never an option, so the identifier the server bound at acquisition is absent from every later message |
| `RFC2131-3.1-1` | DHCPREQUEST in SELECTING state MUST include the server identifier option (§3.1, Table 4) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze hands SELECTING message construction to the vendored library -- runV4 calls client.Request (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48), which builds the REQUEST in NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) where the server identifier is copied from the OFFER at dhcpv4.go:263; ze contributes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308) and authors none of those bytes |
| `RFC2131-3.1-2` | DHCPREQUEST in SELECTING state MUST include the requested IP address option (§3.1, Table 4) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze hands SELECTING message construction to the vendored library -- runV4 calls client.Request (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48), which builds the REQUEST in NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) where the requested IP address option is set from the OFFER's yiaddr at dhcpv4.go:261; ze contributes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308) and authors none of those bytes |
| `RFC2131-3.1-3` | DHCPREQUEST in INIT-REBOOT state MUST include the requested IP address option (§3.1, Table 4) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| `RFC2131-3.1-4` | DHCPREQUEST in INIT-REBOOT state MUST NOT include the server identifier option (§3.1, Table 4) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| `RFC2131-3.1-5` | DHCPREQUEST in RENEWING/REBINDING MUST NOT include server identifier or requested IP address (§3.1, Table 4) | MUST NOT | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| `RFC2131-3.2-1` | Client in INIT-REBOOT MUST NOT fill in ciaddr (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| `RFC2131-3.1-6` | Client MUST use same client identifier in DHCPRELEASE as used to obtain the lease (§3.1, §3.2) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPRELEASE, so no ze path builds the message whose fields this governs -- runV4 ends a lease by removing the address locally (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 211-235), and nclient4's Release (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:26) is called nowhere in ze |
| `RFC2131-3.1-7` | If client detects allocated address is already in use, it MUST send DHCPDECLINE (§3.1, §3.2) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's client installs the ACKed address without checking it is free and has no DHCPDECLINE path -- handleV4Lease goes straight from the ACK to ReplaceAddressWithLifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) with no probe of the address, and neither ze nor the vendored nclient4 client contains a DECLINE producer, so an address found in use is kept rather than declined |
| `RFC2131-4.4.5-1` | Client MUST stop network processing when lease expires (§4.4.5) | MUST | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze tears the lease down late -- runV4 budgets fixed sleeps of lease/2, 3*lease/8 and lease/8 (internal/plugins/iface/dhcp/dhcp_v4_linux.go:80-118) and calls renewV4 between them, so the blocking renewal attempts add to that budget: each failed renewal spends the vendored retry schedule of 5 seconds doubling over 3 tries (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:32, 35, retryFn 649-669), leaving the lease's default route (internal/plugins/iface/dhcp/dhcp_v4_linux.go:190) installed for around 70 seconds past expiry before removeV4Addr runs (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 228-235); only the address itself leaves on time, because the kernel holds the lease duration as its valid lifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:180, internal/plugins/iface/netlink/manage_linux.go:268-286) |
| `RFC2131-3-1` | Options field MUST start with magic cookie 0x63825363 (§3) | MUST | 3 | **positive:** `unit/verify` [`TestMagicCookieRequiredAndEmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L423). **negative:** `unit/verify` [`TestMagicCookieRequiredAndEmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L431) |
| `RFC2131-4.1-3` | Options field MUST end with end option (255) (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestReplyOptionsTerminatedByEnd`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L453). **negative:** `unit/verify` [`TestReplyOptionsTerminatedByEnd`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L471) |
| `RFC2131-4.1-4` | If option overload is used, it MUST appear in the options field (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field |
| `RFC2131-4.1-5` | Options in sname/file fields MUST begin with the first octet, be terminated by end option, and be followed by pad (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field |
| `RFC2131-4.1-6` | Each option MUST be entirely contained in its field (options/sname/file) (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestOptionsContainedWithinTheirField`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L492). **negative:** `unit/verify` [`TestOptionsContainedWithinTheirField`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L515) |
| `RFC2131-4.1-7` | Options field MUST be interpreted first, then file, then sname (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field; ze reads options from the options field only, so it holds no second option stream to order against it |
| `RFC2131-4.1-8` | Client MUST adopt a retransmission strategy with randomized exponential backoff (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's client retransmits on a doubling timeout with no randomization -- runV4 and renewV4 build the client with no ClientOpt (internal/plugins/iface/dhcp/dhcp_v4_linux.go:37, 128), so it runs the vendored defaults of a 5 second timeout over 3 tries (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:32, 35, 183) through retryFn, which only doubles the timeout between tries (client.go:649-669) |
| `RFC2131-4.1-9` | Client MUST choose xid values to minimize collision with other clients (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the transaction ID is drawn inside the vendored library -- every message ze's client sends is built through dhcpv4.New (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:144-150), which takes the xid from crypto/rand via GenerateTransactionID (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:121-139); ze calls only client.Request and client.Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) and authors no xid |
| `RFC2131-4.1-10` | Client MUST use the IP address from the server identifier option for unicast requests (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's renewal is not unicast to the server identifier -- renewV4 creates the client with nclient4.New(c.ifaceName) and no WithServerAddr (internal/plugins/iface/dhcp/dhcp_v4_linux.go:128), so serverAddr keeps the library default of 255.255.255.255:67 (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:50-53, assigned at client.go:183) and Renew sends the RENEWING REQUEST to that broadcast address (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:60) instead of to the server identifier the ACK carried |
| `RFC2131-3.4-1` | Server MUST NOT check for existing lease when responding to DHCPINFORM (§3.4) | MUST NOT | 3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze answers no DHCPINFORM at all -- the message-type switch in handle (internal/plugins/dhcpserver/handler.go:121-134) dispatches DISCOVER, REQUEST, RELEASE and DECLINE, and every other type including msgInform (handler.go:37) falls to the default branch that returns nil, so ze builds no DHCPINFORM response at all, so no response path exists that could consult an existing lease |
| `RFC2131-2-3` | Flags field bits 1-15 MUST be set to zero by clients and ignored by servers/relay agents (§2) | MUST | 2 | **positive:** `unit/verify` [`TestFlagsReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L549). **negative:** `unit/verify` [`TestFlagsReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L560) |
| `RFC2131-2-4` | Client MUST be prepared to receive DHCP messages with options field of at least 312 octets (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze reads no DHCP bytes itself -- the receive path is the vendored library's receiveLoop, which reads each datagram into a MaxMessageSize (1500 octet) buffer before decoding (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:256-271, MaxMessageSize at client.go:38), and ze only consumes the decoded result of client.Request and client.Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) |
| `RFC2131-3.5-1` | If client includes a parameter request list in DHCPDISCOVER, it MUST include that list in subsequent DHCPREQUEST messages (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the parameter request list is written by the vendored library, the same list in both messages -- NewDiscovery (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:198-209) and NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) each add the identical WithRequestedOptions set, and ze passes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308), so it authors no request list |
| `RFC2131-4.4.5-2` | T1 MUST be earlier than T2 (§4.4.5) | MUST | 4.4.5 | **positive:** `unit/verify` [`TestRenewalTimersOrderedWithinLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L603). **negative:** `unit/verify` [`TestShortLeaseTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L630) |
| `RFC2131-4.4.5-3` | T2 MUST be earlier than lease expiry (§4.4.5) | MUST | 4.4.5 | **positive:** `unit/verify` [`TestRenewalTimersOrderedWithinLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L607). **negative:** `unit/verify` [`TestShortLeaseTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L635) |
| `RFC2131-3.1-8` | DHCPREQUEST in SELECTING MUST use the same secs field and broadcast address as the original DHCPDISCOVER (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the secs field and the broadcast flag of the SELECTING REQUEST are the vendored library's -- it leaves secs zero in both the DISCOVER and the REQUEST (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:164-180 sets no secs) and copies the flags word across with WithReply (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:258, vendor/github.com/insomniacslk/dhcp/dhcpv4/modifiers.go:56-68), while ze's DORA call and its modifiers touch neither (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 299-308) |
| `RFC2131-3.1-9` | Server SHOULD probe offered address (e.g., ICMP Echo) before allocating (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.4-1` | Server SHOULD retain released client parameters for possible reuse (§4.3.4) | SHOULD | 4.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-10` | Server SHOULD mark offered address as available if no DHCPREQUEST received (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.2-4` | Server SHOULD respond with DHCPNAK to wrong-subnet INIT-REBOOT client (§4.3.2, §3.2) | SHOULD | 4.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.3-2` | Server SHOULD notify administrator on DHCPDECLINE (§4.3.3) | SHOULD | 4.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.5-2` | DHCPINFORM response SHOULD NOT fill in yiaddr (§4.3.5) | SHOULD NOT | 4.3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.4-2` | Server SHOULD unicast DHCPACK for DHCPINFORM to ciaddr (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.4-3` | Server SHOULD check network address in DHCPINFORM for consistency (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-11` | DHCPACK parameters SHOULD NOT conflict with earlier DHCPOFFER (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-12` | Server SHOULD NOT check offered network address again at DHCPREQUEST time (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-13` | Client SHOULD perform final check on parameters (e.g., ARP) after DHCPACK (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-14` | Client SHOULD wait minimum 10 seconds before restarting after DHCPDECLINE (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-15` | Client SHOULD notify the user when initialization fails and restarts (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.5-2` | Client SHOULD include maximum DHCP message size option (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.7-1` | Client SHOULD use DHCP to reacquire/verify IP address at boot time or after disconnection (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-2-5` | TCP/IP software SHOULD accept and forward IP packets to the IP layer before address is configured (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.1-11` | Client unable to receive unicast SHOULD set BROADCAST bit (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.1-12` | Server/relay SHOULD examine BROADCAST bit and broadcast when set (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.5-4` | T1 and T2 SHOULD include random fuzz to avoid synchronization (§4.4.5) | SHOULD | 4.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.1-13` | Retransmission delay SHOULD start at 4 seconds with +/-1 randomization and double up to 64 seconds (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-2.2-1` | Allocating server SHOULD probe reused address before allocating (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-2.2-2` | Client SHOULD probe newly received address (e.g., with ARP) (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-16` | DHCPDISCOVER MAY include options suggesting address and lease duration (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-17` | Server MAY choose to mark offered addresses as unavailable (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.2-3` | Server MAY refuse to allocate even when addresses are available (§4.2) | MAY | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.1-14` | Server MAY use any of its network addresses in outgoing DHCP messages (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.2-2` | Client MAY choose to use previously allocated address for remainder of unexpired lease if no response (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.1-1` | Server MUST select configuration parameters by applying rules in specified order (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze applies no parameter-selection procedure -- buildReply emits one fixed option set in one fixed order (internal/plugins/dhcpserver/handler.go:245-285) and the parameter request list is never parsed (optParamReqList is defined at handler.go:52 and read nowhere in production), so the ordered rules of Section 4.3.1 govern no ze code path |
| `RFC2131-4.3.1-2` | If server has an explicitly configured default value for a requested parameter, it MUST include that value (§4.3.1) | MUST | 4.3.1 | **positive:** `unit/verify` [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L697). **negative:** no negative test. **{single-polarity}:** ze emits its configured values unconditionally (buildReply internal/plugins/dhcpserver/handler.go:245-285), so a configured value is present whether or not the client asked for it and no input suppresses one to assert negatively |
| `RFC2131-4.3.1-3` | If server recognizes a parameter defined in the Host Requirements Document, it MUST include the default value (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze returns only the values its own subnet configuration holds (buildReply internal/plugins/dhcpserver/handler.go:245-285); it reads no parameter request list (optParamReqList handler.go:52 is parsed nowhere in production) and carries no Host Requirements defaults, so a recognized parameter the client requests draws no default value |
| `RFC2131-4.3.1-4` | If server has no value for a requested parameter, it MUST NOT return a value for that parameter (§4.3.1) | MUST NOT | 4.3.1 | **positive:** `unit/verify` [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L668). **negative:** `unit/verify` [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L682) |
| `RFC2131-4.3.1-5` | Server MUST supply as many of the requested parameters as possible and MUST omit any it cannot provide (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the parameter request list is never parsed (optParamReqList internal/plugins/dhcpserver/handler.go:52 is read nowhere in production), so ze supplies its fixed configured option set (buildReply handler.go:245-285) rather than as many of the client's requested parameters as it can |
| `RFC2131-4.3.1-6` | Server MUST include each requested parameter only once unless explicitly allowed (§4.3.1) | MUST | 4.3.1 | **positive:** `unit/verify` [`TestEachParameterEmittedOnce`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L723). **negative:** `unit/verify` [`TestEachParameterEmittedOnce`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L744) |
| `RFC2131-4.3.1-7` | Vendor class identifier parameters MUST be identified by an exact match between client and server class identifiers (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze identifies the vendor class by a ten-octet prefix comparison against the string PXEClient: (isPXEClient internal/plugins/dhcpserver/handler.go:493-496) rather than by an exact match against a configured class identifier, and that prefix match is what selects the class-specific options appended at handler.go:292-330 |
| `RFC2131-4.4.1-1` | Client MUST include its hardware address in the 'chaddr' field if necessary for DHCP reply delivery (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** chaddr is filled by the vendored library from the bound interface -- nclient4.New resolves the interface MAC (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:199-209) and NewDiscovery/NewRequestFromOffer set it with WithHwAddr (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:198-209, 256-270); ze only names the interface to bind (internal/plugins/iface/dhcp/dhcp_v4_linux.go:37) |
| `RFC2131-4.4.2-1` | Client sending DHCPDECLINE MUST insert its known network address as 'requested IP address' option (§4.4.2) | MUST | 4.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| `RFC2131-4.4.2-2` | Client sending DHCPDECLINE/DHCPRELEASE of requested IP MUST NOT include 'server identifier' in INIT-REBOOT (§4.4.2) | MUST NOT | 4.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| `RFC2131-4.4.3-1` | DHCPINFORM messages MUST be directed to the 'DHCP server' UDP port (§4.4.3) | MUST | 4.4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's client sends no DHCPINFORM -- nclient4 exposes Inform (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:483-498) and ze calls it nowhere; runV4 and renewV4 use only Request and Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) |
| `RFC2131-4.3.2-5` | DHCPREQUEST in REBINDING state MUST be broadcast to 0xFFFFFFFF (§4.3.2) | MUST | 4.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's T2 attempt reuses the same vendored Renew call as its T1 attempt (internal/plugins/iface/dhcp/dhcp_v4_linux.go:103 and 143), so both the message and its destination are the library's: NewRenewFromAck builds it (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290) and it goes to the default serverAddr of 255.255.255.255:67 (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:50-53, 183, used at nclient4/lease.go:60); ze chooses neither |
| `RFC2131-4.4.5-5` | Client given a new network address after lease expiry MUST NOT continue using the previous address (§4.4.5) | MUST NOT | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a renewal that returns a different yiaddr leaves the previous address in use -- renewV4 hands the new ACK to handleV4Lease (internal/plugins/iface/dhcp/dhcp_v4_linux.go:154), which installs the new address with ReplaceAddressWithLifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:180), a per-address replace that touches no other address (internal/plugins/iface/netlink/manage_linux.go:268-286), and runV4 then overwrites its ack reference (internal/plugins/iface/dhcp/dhcp_v4_linux.go:87-88) so the previous address is never passed to removeV4Addr; both addresses stay configured on the interface |
| `RFC2131-4.4.5-6` | Client in RENEWING state MUST NOT include 'server identifier' in the DHCPREQUEST (§4.4.5) | MUST NOT | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| `RFC2131-4.4.5-7` | Client in REBINDING state MUST NOT include 'server identifier' in the DHCPREQUEST (§4.4.5) | MUST NOT | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| `RFC2131-3.2-3` | Client MUST NOT fill in 'ciaddr' when it has not received its network address (INIT-REBOOT) (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| `RFC2131-4.4-1` | DHCPDECLINE/DHCPRELEASE: vendor class identifier MUST NOT be included (§4.4, Table 5) | MUST NOT | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| `RFC2131-4.4-2` | DHCPDECLINE: requested IP address MUST be included (§4.4, Table 5) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| `RFC2131-4.4-3` | DHCPRELEASE: server identifier MUST be included (§4.4, Table 5) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPRELEASE, so no ze path builds the message whose fields this governs -- runV4 ends a lease by removing the address locally (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 211-235), and nclient4's Release (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:26) is called nowhere in ze |
| `RFC2131-4.4.3-2` | Client SHOULD NOT request lease time parameters in DHCPINFORM (§4.4.3) | SHOULD NOT | 4.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.1-2` | Client SHOULD wait a random time between one and ten seconds before first DHCPDISCOVER (§4.4.1) | SHOULD | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.1-3` | Client SHOULD broadcast ARP reply to announce new IP address after DHCPACK (§4.4.1) | SHOULD | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.2-6` | Server SHOULD check 'ciaddr' for correctness before replying to REBINDING DHCPREQUEST (§4.3.2) | SHOULD | 4.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.2-4` | Client SHOULD provide mechanism for user to select vendor class identifier values (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.5-8` | Client SHOULD wait one-half of remaining time before retransmitting DHCPREQUEST in RENEWING/REBINDING (§4.4.5) | SHOULD | 4.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.5-9` | Client SHOULD continue network processing if given its previous address after lease expiry (§4.4.5) | SHOULD | 4.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.5-10` | Client SHOULD notify the local users if given a new address after lease expiry (§4.4.5) | SHOULD | 4.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.5-3` | Server SHOULD respond with DHCPNAK if 'requested IP address' is invalid (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.1-8` | Address selection for new allocation SHOULD follow specified priority order (§4.3.1) | SHOULD | 4.3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2131-4.3.5-1`](#rfc2131-4.3.5-1) Server MUST NOT send lease expiration time for DHCPINFORM (§4.3.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze answers no DHCPINFORM at all -- the message-type switch in handle (internal/plugins/dhcpserver/handler.go:121-134) dispatches DISCOVER, REQUEST, RELEASE and DECLINE, and every other type including msgInform (handler.go:37) falls to the default branch that returns nil, so ze builds no DHCPINFORM response at all |
| [`RFC2131-4.3.2-1`](#rfc2131-4.3.2-1) When giaddr is 0x0, server MUST broadcast DHCPNAK to 0xFFFFFFFF (§4.3.2, §3.2) | {gap}, no test | with giaddr zero the delivery path unicasts a DHCPNAK to a non-zero ciaddr instead of broadcasting -- responseAddr tests giaddr, then ciaddr, before it ever reaches the broadcast decision (internal/plugins/dhcpserver/register.go:275-298) and never special-cases the NAK message type, so a REQUEST whose ciaddr is on-subnet and whose requested address is off-subnet is NAKed to ciaddr:68 |
| [`RFC2131-4.3.2-2`](#rfc2131-4.3.2-2) When giaddr is set in DHCPNAK, server MUST set the broadcast bit (§4.3.2) | {gap}, no test | buildNak copies the client's flags word verbatim (internal/plugins/dhcpserver/handler.go:339) and never forces the BROADCAST bit, so a DHCPNAK relayed through a non-zero giaddr leaves the server with the broadcast bit clear whenever the client left it clear |
| [`RFC2131-4.3.2-3`](#rfc2131-4.3.2-3) If server has no record of client in INIT-REBOOT, server MUST remain silent (§4.3.2) | {gap}, no test | the INIT-REBOOT branch commits a binding for any in-subnet requested address without consulting the lease table (handleRequest internal/plugins/dhcpserver/handler.go:178-183 calls commitBinding at handler.go:194), so a client the server holds no record of draws a DHCPACK rather than silence |
| [`RFC2131-4.3.3-1`](#rfc2131-4.3.3-1) Server MUST mark the network address as unavailable on DHCPDECLINE (§4.3.3) | {gap}, no test | markUnavailable sets the pool bit and the static set for the declined address (internal/plugins/dhcpserver/pool.go:144-163), but the declining client's MAC-to-address cache entry survives -- pool.release returns early for a staticSet address (pool.go:174-177) -- so pool.allocate hands that same declined address back to that client on its next DISCOVER (pool.go:68-72) |
| [`RFC2131-4.1-1`](#rfc2131-4.1-1) A server with multiple network addresses MUST be prepared to accept any of its addresses as identifying that server (§4.1) | {gap}, no test | each handler accepts exactly one address as its server identifier -- handleRequest discards a REQUEST whose option 54 differs from its own serverIP (internal/plugins/dhcpserver/handler.go:167-169) -- and that address is the single per-subnet value derived at register.go:93-101, so a REQUEST naming another of the server's own addresses is silently dropped |
| [`RFC2131-4.1-2`](#rfc2131-4.1-2) Server MUST choose a server identifier address reachable from the client (§4.1) | {gap}, no test | with no default-router configured for a subnet the server identifier is not an address the server answers on -- register.go:93 prefers sub.DefaultRouter, which parseSubnet leaves unset when the leaf is absent (internal/plugins/dhcpserver/config.go:189-195), and falls back to the first pool address (internal/plugins/dhcpserver/register.go:95-97), which the server itself hands out to a client, or to the subnet network address (register.go:99-101), which is no host at all; buildReply then emits that value as option 54 (internal/plugins/dhcpserver/handler.go:246), so a client unicasting to it reaches nothing |
| [`RFC2131-4.2-1`](#rfc2131-4.2-1) If client supplies a client identifier, server MUST use it to identify the client (§4.2) | {gap}, no test | ze never reads the client identifier -- option 61 is absent from the option codes at internal/plugins/dhcpserver/handler.go:41-61 and no parse call asks for it -- and keys every binding on the hardware address instead (extractMAC handler.go:456, leaseTable byMAC lease.go:23, pool.macToAddr pool.go:22), so a client that supplies a client identifier is still identified by chaddr |
| [`RFC2131-2-1`](#rfc2131-2-1) Client identifier MUST be unique within the subnet (§2) | {gap}, no test | ze's client emits the operator-configured client identifier verbatim and nothing derives or checks it -- v4RequestModifiers appends OptClientIdentifier([]byte(c.config.ClientID)) (internal/plugins/iface/dhcp/dhcp_v4_linux.go:304-306) from the config leaf read at internal/component/iface/config.go:1267-1269 and carried at internal/component/iface/register.go:891, and no validator constrains the value, so two units on one subnet configured with the same client-id emit colliding identifiers |
| [`RFC2131-2-2`](#rfc2131-2-2) If client uses a client identifier in one message, it MUST use the same identifier in all subsequent messages (§2) | {gap}, no test | ze's client sends option 61 at acquisition and omits it on every renewal -- the DORA call passes v4RequestModifiers() (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, built at :299-308) while renewV4 calls client.Renew(ctx, lease) with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143), and the renewal REQUEST is built by NewRenewFromAck (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290) whose WithReply copies only opcode, HW type, xid, chaddr and flags (vendor/github.com/insomniacslk/dhcp/dhcpv4/modifiers.go:56-68) and never an option, so the identifier the server bound at acquisition is absent from every later message |
| [`RFC2131-3.1-1`](#rfc2131-3.1-1) DHCPREQUEST in SELECTING state MUST include the server identifier option (§3.1, Table 4) | no test | no test carries this requirement id; annotated {not-applicable}: ze hands SELECTING message construction to the vendored library -- runV4 calls client.Request (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48), which builds the REQUEST in NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) where the server identifier is copied from the OFFER at dhcpv4.go:263; ze contributes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308) and authors none of those bytes |
| [`RFC2131-3.1-2`](#rfc2131-3.1-2) DHCPREQUEST in SELECTING state MUST include the requested IP address option (§3.1, Table 4) | no test | no test carries this requirement id; annotated {not-applicable}: ze hands SELECTING message construction to the vendored library -- runV4 calls client.Request (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48), which builds the REQUEST in NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) where the requested IP address option is set from the OFFER's yiaddr at dhcpv4.go:261; ze contributes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308) and authors none of those bytes |
| [`RFC2131-3.1-3`](#rfc2131-3.1-3) DHCPREQUEST in INIT-REBOOT state MUST include the requested IP address option (§3.1, Table 4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| [`RFC2131-3.1-4`](#rfc2131-3.1-4) DHCPREQUEST in INIT-REBOOT state MUST NOT include the server identifier option (§3.1, Table 4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| [`RFC2131-3.1-5`](#rfc2131-3.1-5) DHCPREQUEST in RENEWING/REBINDING MUST NOT include server identifier or requested IP address (§3.1, Table 4) | no test | no test carries this requirement id; annotated {not-applicable}: the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| [`RFC2131-3.2-1`](#rfc2131-3.2-1) Client in INIT-REBOOT MUST NOT fill in ciaddr (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| [`RFC2131-3.1-6`](#rfc2131-3.1-6) Client MUST use same client identifier in DHCPRELEASE as used to obtain the lease (§3.1, §3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPRELEASE, so no ze path builds the message whose fields this governs -- runV4 ends a lease by removing the address locally (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 211-235), and nclient4's Release (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:26) is called nowhere in ze |
| [`RFC2131-3.1-7`](#rfc2131-3.1-7) If client detects allocated address is already in use, it MUST send DHCPDECLINE (§3.1, §3.2) | {gap}, no test | ze's client installs the ACKed address without checking it is free and has no DHCPDECLINE path -- handleV4Lease goes straight from the ACK to ReplaceAddressWithLifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) with no probe of the address, and neither ze nor the vendored nclient4 client contains a DECLINE producer, so an address found in use is kept rather than declined |
| [`RFC2131-4.4.5-1`](#rfc2131-4.4.5-1) Client MUST stop network processing when lease expires (§4.4.5) | {gap}, no test | ze tears the lease down late -- runV4 budgets fixed sleeps of lease/2, 3*lease/8 and lease/8 (internal/plugins/iface/dhcp/dhcp_v4_linux.go:80-118) and calls renewV4 between them, so the blocking renewal attempts add to that budget: each failed renewal spends the vendored retry schedule of 5 seconds doubling over 3 tries (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:32, 35, retryFn 649-669), leaving the lease's default route (internal/plugins/iface/dhcp/dhcp_v4_linux.go:190) installed for around 70 seconds past expiry before removeV4Addr runs (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 228-235); only the address itself leaves on time, because the kernel holds the lease duration as its valid lifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:180, internal/plugins/iface/netlink/manage_linux.go:268-286) |
| [`RFC2131-4.1-4`](#rfc2131-4.1-4) If option overload is used, it MUST appear in the options field (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field |
| [`RFC2131-4.1-5`](#rfc2131-4.1-5) Options in sname/file fields MUST begin with the first octet, be terminated by end option, and be followed by pad (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field |
| [`RFC2131-4.1-7`](#rfc2131-4.1-7) Options field MUST be interpreted first, then file, then sname (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field; ze reads options from the options field only, so it holds no second option stream to order against it |
| [`RFC2131-4.1-8`](#rfc2131-4.1-8) Client MUST adopt a retransmission strategy with randomized exponential backoff (§4.1) | {gap}, no test | ze's client retransmits on a doubling timeout with no randomization -- runV4 and renewV4 build the client with no ClientOpt (internal/plugins/iface/dhcp/dhcp_v4_linux.go:37, 128), so it runs the vendored defaults of a 5 second timeout over 3 tries (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:32, 35, 183) through retryFn, which only doubles the timeout between tries (client.go:649-669) |
| [`RFC2131-4.1-9`](#rfc2131-4.1-9) Client MUST choose xid values to minimize collision with other clients (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: the transaction ID is drawn inside the vendored library -- every message ze's client sends is built through dhcpv4.New (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:144-150), which takes the xid from crypto/rand via GenerateTransactionID (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:121-139); ze calls only client.Request and client.Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) and authors no xid |
| [`RFC2131-4.1-10`](#rfc2131-4.1-10) Client MUST use the IP address from the server identifier option for unicast requests (§4.1) | {gap}, no test | ze's renewal is not unicast to the server identifier -- renewV4 creates the client with nclient4.New(c.ifaceName) and no WithServerAddr (internal/plugins/iface/dhcp/dhcp_v4_linux.go:128), so serverAddr keeps the library default of 255.255.255.255:67 (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:50-53, assigned at client.go:183) and Renew sends the RENEWING REQUEST to that broadcast address (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:60) instead of to the server identifier the ACK carried |
| [`RFC2131-3.4-1`](#rfc2131-3.4-1) Server MUST NOT check for existing lease when responding to DHCPINFORM (§3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze answers no DHCPINFORM at all -- the message-type switch in handle (internal/plugins/dhcpserver/handler.go:121-134) dispatches DISCOVER, REQUEST, RELEASE and DECLINE, and every other type including msgInform (handler.go:37) falls to the default branch that returns nil, so ze builds no DHCPINFORM response at all, so no response path exists that could consult an existing lease |
| [`RFC2131-2-4`](#rfc2131-2-4) Client MUST be prepared to receive DHCP messages with options field of at least 312 octets (§2) | no test | no test carries this requirement id; annotated {not-applicable}: ze reads no DHCP bytes itself -- the receive path is the vendored library's receiveLoop, which reads each datagram into a MaxMessageSize (1500 octet) buffer before decoding (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:256-271, MaxMessageSize at client.go:38), and ze only consumes the decoded result of client.Request and client.Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) |
| [`RFC2131-3.5-1`](#rfc2131-3.5-1) If client includes a parameter request list in DHCPDISCOVER, it MUST include that list in subsequent DHCPREQUEST messages (§3.5) | no test | no test carries this requirement id; annotated {not-applicable}: the parameter request list is written by the vendored library, the same list in both messages -- NewDiscovery (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:198-209) and NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) each add the identical WithRequestedOptions set, and ze passes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308), so it authors no request list |
| [`RFC2131-3.1-8`](#rfc2131-3.1-8) DHCPREQUEST in SELECTING MUST use the same secs field and broadcast address as the original DHCPDISCOVER (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: the secs field and the broadcast flag of the SELECTING REQUEST are the vendored library's -- it leaves secs zero in both the DISCOVER and the REQUEST (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:164-180 sets no secs) and copies the flags word across with WithReply (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:258, vendor/github.com/insomniacslk/dhcp/dhcpv4/modifiers.go:56-68), while ze's DORA call and its modifiers touch neither (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 299-308) |
| [`RFC2131-4.3.1-1`](#rfc2131-4.3.1-1) Server MUST select configuration parameters by applying rules in specified order (§4.3.1) | {gap}, no test | ze applies no parameter-selection procedure -- buildReply emits one fixed option set in one fixed order (internal/plugins/dhcpserver/handler.go:245-285) and the parameter request list is never parsed (optParamReqList is defined at handler.go:52 and read nowhere in production), so the ordered rules of Section 4.3.1 govern no ze code path |
| [`RFC2131-4.3.1-3`](#rfc2131-4.3.1-3) If server recognizes a parameter defined in the Host Requirements Document, it MUST include the default value (§4.3.1) | {gap}, no test | ze returns only the values its own subnet configuration holds (buildReply internal/plugins/dhcpserver/handler.go:245-285); it reads no parameter request list (optParamReqList handler.go:52 is parsed nowhere in production) and carries no Host Requirements defaults, so a recognized parameter the client requests draws no default value |
| [`RFC2131-4.3.1-5`](#rfc2131-4.3.1-5) Server MUST supply as many of the requested parameters as possible and MUST omit any it cannot provide (§4.3.1) | {gap}, no test | the parameter request list is never parsed (optParamReqList internal/plugins/dhcpserver/handler.go:52 is read nowhere in production), so ze supplies its fixed configured option set (buildReply handler.go:245-285) rather than as many of the client's requested parameters as it can |
| [`RFC2131-4.3.1-7`](#rfc2131-4.3.1-7) Vendor class identifier parameters MUST be identified by an exact match between client and server class identifiers (§4.3.1) | {gap}, no test | ze identifies the vendor class by a ten-octet prefix comparison against the string PXEClient: (isPXEClient internal/plugins/dhcpserver/handler.go:493-496) rather than by an exact match against a configured class identifier, and that prefix match is what selects the class-specific options appended at handler.go:292-330 |
| [`RFC2131-4.4.1-1`](#rfc2131-4.4.1-1) Client MUST include its hardware address in the 'chaddr' field if necessary for DHCP reply delivery (§4.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: chaddr is filled by the vendored library from the bound interface -- nclient4.New resolves the interface MAC (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:199-209) and NewDiscovery/NewRequestFromOffer set it with WithHwAddr (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:198-209, 256-270); ze only names the interface to bind (internal/plugins/iface/dhcp/dhcp_v4_linux.go:37) |
| [`RFC2131-4.4.2-1`](#rfc2131-4.4.2-1) Client sending DHCPDECLINE MUST insert its known network address as 'requested IP address' option (§4.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| [`RFC2131-4.4.2-2`](#rfc2131-4.4.2-2) Client sending DHCPDECLINE/DHCPRELEASE of requested IP MUST NOT include 'server identifier' in INIT-REBOOT (§4.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| [`RFC2131-4.4.3-1`](#rfc2131-4.4.3-1) DHCPINFORM messages MUST be directed to the 'DHCP server' UDP port (§4.4.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's client sends no DHCPINFORM -- nclient4 exposes Inform (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:483-498) and ze calls it nowhere; runV4 and renewV4 use only Request and Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) |
| [`RFC2131-4.3.2-5`](#rfc2131-4.3.2-5) DHCPREQUEST in REBINDING state MUST be broadcast to 0xFFFFFFFF (§4.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's T2 attempt reuses the same vendored Renew call as its T1 attempt (internal/plugins/iface/dhcp/dhcp_v4_linux.go:103 and 143), so both the message and its destination are the library's: NewRenewFromAck builds it (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290) and it goes to the default serverAddr of 255.255.255.255:67 (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:50-53, 183, used at nclient4/lease.go:60); ze chooses neither |
| [`RFC2131-4.4.5-5`](#rfc2131-4.4.5-5) Client given a new network address after lease expiry MUST NOT continue using the previous address (§4.4.5) | {gap}, no test | a renewal that returns a different yiaddr leaves the previous address in use -- renewV4 hands the new ACK to handleV4Lease (internal/plugins/iface/dhcp/dhcp_v4_linux.go:154), which installs the new address with ReplaceAddressWithLifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:180), a per-address replace that touches no other address (internal/plugins/iface/netlink/manage_linux.go:268-286), and runV4 then overwrites its ack reference (internal/plugins/iface/dhcp/dhcp_v4_linux.go:87-88) so the previous address is never passed to removeV4Addr; both addresses stay configured on the interface |
| [`RFC2131-4.4.5-6`](#rfc2131-4.4.5-6) Client in RENEWING state MUST NOT include 'server identifier' in the DHCPREQUEST (§4.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| [`RFC2131-4.4.5-7`](#rfc2131-4.4.5-7) Client in REBINDING state MUST NOT include 'server identifier' in the DHCPREQUEST (§4.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| [`RFC2131-3.2-3`](#rfc2131-3.2-3) Client MUST NOT fill in 'ciaddr' when it has not received its network address (INIT-REBOOT) (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| [`RFC2131-4.4-1`](#rfc2131-4.4-1) DHCPDECLINE/DHCPRELEASE: vendor class identifier MUST NOT be included (§4.4, Table 5) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| [`RFC2131-4.4-2`](#rfc2131-4.4-2) DHCPDECLINE: requested IP address MUST be included (§4.4, Table 5) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| [`RFC2131-4.4-3`](#rfc2131-4.4-3) DHCPRELEASE: server identifier MUST be included (§4.4, Table 5) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPRELEASE, so no ze path builds the message whose fields this governs -- runV4 ends a lease by removing the address locally (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 211-235), and nclient4's Release (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:26) is called nowhere in ze |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2131-4.3-1`](#rfc2131-4.3-1)

Server identifier MUST be included in DHCPOFFER, DHCPACK, and DHCPNAK (§4.3, Table 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestServerIdentifierInEveryReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L169) | unit/verify | unproven |

### [`RFC2131-4.3-2`](#rfc2131-4.3-2)

IP address lease time MUST be included in DHCPOFFER (§4.3, Table 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLeaseTimeInOfferAndAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L194) | unit/verify | unproven |

### [`RFC2131-4.3-3`](#rfc2131-4.3-3)

IP address lease time MUST be included in DHCPACK for DHCPREQUEST (§4.3, Table 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLeaseTimeInOfferAndAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L198) | unit/verify | unproven |

### [`RFC2131-4.3-4`](#rfc2131-4.3-4)

IP address lease time MUST NOT be included in DHCPNAK (§4.3, Table 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L333) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L245) | unit/verify | unproven |

### [`RFC2131-4.3-5`](#rfc2131-4.3-5)

Requested IP address option MUST NOT be included in DHCPOFFER, DHCPACK, or DHCPNAK (§4.3, Table 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L312) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L224) | unit/verify | unproven |

### [`RFC2131-4.3-6`](#rfc2131-4.3-6)

Client identifier MUST NOT be included in DHCPOFFER or DHCPACK (§4.3, Table 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L327) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L239) | unit/verify | unproven |

### [`RFC2131-4.3-7`](#rfc2131-4.3-7)

Parameter request list MUST NOT be included in DHCPOFFER, DHCPACK, or DHCPNAK (§4.3, Table 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L316) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L228) | unit/verify | unproven |

### [`RFC2131-4.3-8`](#rfc2131-4.3-8)

Maximum message size MUST NOT be included in DHCPOFFER, DHCPACK, or DHCPNAK (§4.3, Table 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L320) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L232) | unit/verify | unproven |

### [`RFC2131-4.3-9`](#rfc2131-4.3-9)

Options other than message MUST NOT be included in DHCPNAK (§4.3, Table 3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L337) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L249) | unit/verify | unproven |

### [`RFC2131-4.3.5-1`](#rfc2131-4.3.5-1)

Server MUST NOT send lease expiration time for DHCPINFORM (§4.3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.5-1, so no unit is bound to it.

### [`RFC2131-4.3.2-1`](#rfc2131-4.3.2-1)

When giaddr is 0x0, server MUST broadcast DHCPNAK to 0xFFFFFFFF (§4.3.2, §3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.2-1, so no unit is bound to it.

### [`RFC2131-4.3.2-2`](#rfc2131-4.3.2-2)

When giaddr is set in DHCPNAK, server MUST set the broadcast bit (§4.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.2-2, so no unit is bound to it.

### [`RFC2131-4.3.2-3`](#rfc2131-4.3.2-3)

If server has no record of client in INIT-REBOOT, server MUST remain silent (§4.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.2-3, so no unit is bound to it.

### [`RFC2131-4.3.3-1`](#rfc2131-4.3.3-1)

Server MUST mark the network address as unavailable on DHCPDECLINE (§4.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.3-1, so no unit is bound to it.

### [`RFC2131-4.1-1`](#rfc2131-4.1-1)

A server with multiple network addresses MUST be prepared to accept any of its addresses as identifying that server (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-1, so no unit is bound to it.

### [`RFC2131-4.1-2`](#rfc2131-4.1-2)

Server MUST choose a server identifier address reachable from the client (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-2, so no unit is bound to it.

### [`RFC2131-4.2-1`](#rfc2131-4.2-1)

If client supplies a client identifier, server MUST use it to identify the client (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.2-1, so no unit is bound to it.

### [`RFC2131-4.2-2`](#rfc2131-4.2-2)

If client does not provide a client identifier, server MUST use chaddr to identify the client (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUnidentifiableClientRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L390) | unit/verify | unproven |
| positive | [`TestChaddrIdentifiesClientWithoutClientIdentifier`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L366) | unit/verify | unproven |

### [`RFC2131-2-1`](#rfc2131-2-1)

Client identifier MUST be unique within the subnet (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-2-1, so no unit is bound to it.

### [`RFC2131-2-2`](#rfc2131-2-2)

If client uses a client identifier in one message, it MUST use the same identifier in all subsequent messages (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-2-2, so no unit is bound to it.

### [`RFC2131-3.1-1`](#rfc2131-3.1-1)

DHCPREQUEST in SELECTING state MUST include the server identifier option (§3.1, Table 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-1, so no unit is bound to it.

### [`RFC2131-3.1-2`](#rfc2131-3.1-2)

DHCPREQUEST in SELECTING state MUST include the requested IP address option (§3.1, Table 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-2, so no unit is bound to it.

### [`RFC2131-3.1-3`](#rfc2131-3.1-3)

DHCPREQUEST in INIT-REBOOT state MUST include the requested IP address option (§3.1, Table 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-3, so no unit is bound to it.

### [`RFC2131-3.1-4`](#rfc2131-3.1-4)

DHCPREQUEST in INIT-REBOOT state MUST NOT include the server identifier option (§3.1, Table 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-4, so no unit is bound to it.

### [`RFC2131-3.1-5`](#rfc2131-3.1-5)

DHCPREQUEST in RENEWING/REBINDING MUST NOT include server identifier or requested IP address (§3.1, Table 4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-5, so no unit is bound to it.

### [`RFC2131-3.2-1`](#rfc2131-3.2-1)

Client in INIT-REBOOT MUST NOT fill in ciaddr (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.2-1, so no unit is bound to it.

### [`RFC2131-3.1-6`](#rfc2131-3.1-6)

Client MUST use same client identifier in DHCPRELEASE as used to obtain the lease (§3.1, §3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-6, so no unit is bound to it.

### [`RFC2131-3.1-7`](#rfc2131-3.1-7)

If client detects allocated address is already in use, it MUST send DHCPDECLINE (§3.1, §3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-7, so no unit is bound to it.

### [`RFC2131-4.4.5-1`](#rfc2131-4.4.5-1)

Client MUST stop network processing when lease expires (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.5-1, so no unit is bound to it.

### [`RFC2131-3-1`](#rfc2131-3-1)

Options field MUST start with magic cookie 0x63825363 (§3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMagicCookieRequiredAndEmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L431) | unit/verify | unproven |
| positive | [`TestMagicCookieRequiredAndEmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L423) | unit/verify | unproven |

### [`RFC2131-4.1-3`](#rfc2131-4.1-3)

Options field MUST end with end option (255) (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyOptionsTerminatedByEnd`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L471) | unit/verify | unproven |
| positive | [`TestReplyOptionsTerminatedByEnd`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L453) | unit/verify | unproven |

### [`RFC2131-4.1-4`](#rfc2131-4.1-4)

If option overload is used, it MUST appear in the options field (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-4, so no unit is bound to it.

### [`RFC2131-4.1-5`](#rfc2131-4.1-5)

Options in sname/file fields MUST begin with the first octet, be terminated by end option, and be followed by pad (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-5, so no unit is bound to it.

### [`RFC2131-4.1-6`](#rfc2131-4.1-6)

Each option MUST be entirely contained in its field (options/sname/file) (§4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOptionsContainedWithinTheirField`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L515) | unit/verify | unproven |
| positive | [`TestOptionsContainedWithinTheirField`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L492) | unit/verify | unproven |

### [`RFC2131-4.1-7`](#rfc2131-4.1-7)

Options field MUST be interpreted first, then file, then sname (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-7, so no unit is bound to it.

### [`RFC2131-4.1-8`](#rfc2131-4.1-8)

Client MUST adopt a retransmission strategy with randomized exponential backoff (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-8, so no unit is bound to it.

### [`RFC2131-4.1-9`](#rfc2131-4.1-9)

Client MUST choose xid values to minimize collision with other clients (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-9, so no unit is bound to it.

### [`RFC2131-4.1-10`](#rfc2131-4.1-10)

Client MUST use the IP address from the server identifier option for unicast requests (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-10, so no unit is bound to it.

### [`RFC2131-3.4-1`](#rfc2131-3.4-1)

Server MUST NOT check for existing lease when responding to DHCPINFORM (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.4-1, so no unit is bound to it.

### [`RFC2131-2-3`](#rfc2131-2-3)

Flags field bits 1-15 MUST be set to zero by clients and ignored by servers/relay agents (§2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlagsReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L560) | unit/verify | unproven |
| positive | [`TestFlagsReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L549) | unit/verify | unproven |

### [`RFC2131-2-4`](#rfc2131-2-4)

Client MUST be prepared to receive DHCP messages with options field of at least 312 octets (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-2-4, so no unit is bound to it.

### [`RFC2131-3.5-1`](#rfc2131-3.5-1)

If client includes a parameter request list in DHCPDISCOVER, it MUST include that list in subsequent DHCPREQUEST messages (§3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.5-1, so no unit is bound to it.

### [`RFC2131-4.4.5-2`](#rfc2131-4.4.5-2)

T1 MUST be earlier than T2 (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestShortLeaseTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L630) | unit/verify | unproven |
| positive | [`TestRenewalTimersOrderedWithinLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L603) | unit/verify | unproven |

### [`RFC2131-4.4.5-3`](#rfc2131-4.4.5-3)

T2 MUST be earlier than lease expiry (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestShortLeaseTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L635) | unit/verify | unproven |
| positive | [`TestRenewalTimersOrderedWithinLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L607) | unit/verify | unproven |

### [`RFC2131-3.1-8`](#rfc2131-3.1-8)

DHCPREQUEST in SELECTING MUST use the same secs field and broadcast address as the original DHCPDISCOVER (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-8, so no unit is bound to it.

### [`RFC2131-4.3.1-1`](#rfc2131-4.3.1-1)

Server MUST select configuration parameters by applying rules in specified order (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.1-1, so no unit is bound to it.

### [`RFC2131-4.3.1-2`](#rfc2131-4.3.1-2)

If server has an explicitly configured default value for a requested parameter, it MUST include that value (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L697) | unit/verify | unproven |

### [`RFC2131-4.3.1-3`](#rfc2131-4.3.1-3)

If server recognizes a parameter defined in the Host Requirements Document, it MUST include the default value (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.1-3, so no unit is bound to it.

### [`RFC2131-4.3.1-4`](#rfc2131-4.3.1-4)

If server has no value for a requested parameter, it MUST NOT return a value for that parameter (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L682) | unit/verify | unproven |
| positive | [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L668) | unit/verify | unproven |

### [`RFC2131-4.3.1-5`](#rfc2131-4.3.1-5)

Server MUST supply as many of the requested parameters as possible and MUST omit any it cannot provide (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.1-5, so no unit is bound to it.

### [`RFC2131-4.3.1-6`](#rfc2131-4.3.1-6)

Server MUST include each requested parameter only once unless explicitly allowed (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEachParameterEmittedOnce`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L744) | unit/verify | unproven |
| positive | [`TestEachParameterEmittedOnce`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L723) | unit/verify | unproven |

### [`RFC2131-4.3.1-7`](#rfc2131-4.3.1-7)

Vendor class identifier parameters MUST be identified by an exact match between client and server class identifiers (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.1-7, so no unit is bound to it.

### [`RFC2131-4.4.1-1`](#rfc2131-4.4.1-1)

Client MUST include its hardware address in the 'chaddr' field if necessary for DHCP reply delivery (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.1-1, so no unit is bound to it.

### [`RFC2131-4.4.2-1`](#rfc2131-4.4.2-1)

Client sending DHCPDECLINE MUST insert its known network address as 'requested IP address' option (§4.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.2-1, so no unit is bound to it.

### [`RFC2131-4.4.2-2`](#rfc2131-4.4.2-2)

Client sending DHCPDECLINE/DHCPRELEASE of requested IP MUST NOT include 'server identifier' in INIT-REBOOT (§4.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.2-2, so no unit is bound to it.

### [`RFC2131-4.4.3-1`](#rfc2131-4.4.3-1)

DHCPINFORM messages MUST be directed to the 'DHCP server' UDP port (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.3-1, so no unit is bound to it.

### [`RFC2131-4.3.2-5`](#rfc2131-4.3.2-5)

DHCPREQUEST in REBINDING state MUST be broadcast to 0xFFFFFFFF (§4.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.2-5, so no unit is bound to it.

### [`RFC2131-4.4.5-5`](#rfc2131-4.4.5-5)

Client given a new network address after lease expiry MUST NOT continue using the previous address (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.5-5, so no unit is bound to it.

### [`RFC2131-4.4.5-6`](#rfc2131-4.4.5-6)

Client in RENEWING state MUST NOT include 'server identifier' in the DHCPREQUEST (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.5-6, so no unit is bound to it.

### [`RFC2131-4.4.5-7`](#rfc2131-4.4.5-7)

Client in REBINDING state MUST NOT include 'server identifier' in the DHCPREQUEST (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.5-7, so no unit is bound to it.

### [`RFC2131-3.2-3`](#rfc2131-3.2-3)

Client MUST NOT fill in 'ciaddr' when it has not received its network address (INIT-REBOOT) (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.2-3, so no unit is bound to it.

### [`RFC2131-4.4-1`](#rfc2131-4.4-1)

DHCPDECLINE/DHCPRELEASE: vendor class identifier MUST NOT be included (§4.4, Table 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4-1, so no unit is bound to it.

### [`RFC2131-4.4-2`](#rfc2131-4.4-2)

DHCPDECLINE: requested IP address MUST be included (§4.4, Table 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4-2, so no unit is bound to it.

### [`RFC2131-4.4-3`](#rfc2131-4.4-3)

DHCPRELEASE: server identifier MUST be included (§4.4, Table 5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4-3, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 2131, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 2131, so its obligations are stated where they were written.
