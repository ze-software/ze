# RFC 2131 - Dynamic Host Configuration Protocol

Partial. Every requirement this repository extracted from RFC 2131, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 24.6% | 16 of 65 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 6.2% | 4 of 65 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 65 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 5.6% | 2 of 36 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 65 | of 102 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 27 | of 65 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 41.5% | 27 of 65 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 65 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 65 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 27.7% | 18 of 65 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 19 | of 65 gated MUSTs judged | 4 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 65 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 102 |
| Gated MUST-level | 65 |
| Not applicable, so out of scope | 27 |
| Declared gaps | 18 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 36 |
| Tagged units | 36 |
| Recorded audit verdicts | 19 |
| Discrimination records | 2 |
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
| Positive and negative tests | 16 | one part of the gated population |
| Annotated instead of tested | 49 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **65** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (16):** [`RFC2131-4.3-4`](#rfc2131-4.3-4), [`RFC2131-4.3-5`](#rfc2131-4.3-5), [`RFC2131-4.3-6`](#rfc2131-4.3-6), [`RFC2131-4.3-7`](#rfc2131-4.3-7), [`RFC2131-4.3-8`](#rfc2131-4.3-8), [`RFC2131-4.3-9`](#rfc2131-4.3-9), [`RFC2131-3.2-4`](#rfc2131-3.2-4), [`RFC2131-4.2-2`](#rfc2131-4.2-2), [`RFC2131-3-1`](#rfc2131-3-1), [`RFC2131-4.1-3`](#rfc2131-4.1-3), [`RFC2131-4.1-6`](#rfc2131-4.1-6), [`RFC2131-2-3`](#rfc2131-2-3), [`RFC2131-4.4.5-2`](#rfc2131-4.4.5-2), [`RFC2131-4.4.5-3`](#rfc2131-4.4.5-3), [`RFC2131-4.3.1-4`](#rfc2131-4.3.1-4), [`RFC2131-4.3.1-6`](#rfc2131-4.3.1-6)

**Annotated instead of tested (49):** [`RFC2131-4.3-1`](#rfc2131-4.3-1), [`RFC2131-4.3-2`](#rfc2131-4.3-2), [`RFC2131-4.3-3`](#rfc2131-4.3-3), [`RFC2131-4.3.5-1`](#rfc2131-4.3.5-1), [`RFC2131-4.3.2-1`](#rfc2131-4.3.2-1), [`RFC2131-4.3.2-2`](#rfc2131-4.3.2-2), [`RFC2131-4.3.2-3`](#rfc2131-4.3.2-3), [`RFC2131-4.3.3-1`](#rfc2131-4.3.3-1), [`RFC2131-4.1-1`](#rfc2131-4.1-1), [`RFC2131-4.1-2`](#rfc2131-4.1-2), [`RFC2131-4.2-1`](#rfc2131-4.2-1), [`RFC2131-2-1`](#rfc2131-2-1), [`RFC2131-2-2`](#rfc2131-2-2), [`RFC2131-3.1-1`](#rfc2131-3.1-1), [`RFC2131-3.1-2`](#rfc2131-3.1-2), [`RFC2131-3.1-3`](#rfc2131-3.1-3), [`RFC2131-3.1-4`](#rfc2131-3.1-4), [`RFC2131-3.1-5`](#rfc2131-3.1-5), [`RFC2131-3.2-1`](#rfc2131-3.2-1), [`RFC2131-3.1-6`](#rfc2131-3.1-6), [`RFC2131-3.1-7`](#rfc2131-3.1-7), [`RFC2131-4.4.5-1`](#rfc2131-4.4.5-1), [`RFC2131-4.1-4`](#rfc2131-4.1-4), [`RFC2131-4.1-5`](#rfc2131-4.1-5), [`RFC2131-4.1-7`](#rfc2131-4.1-7), [`RFC2131-4.1-8`](#rfc2131-4.1-8), [`RFC2131-4.1-9`](#rfc2131-4.1-9), [`RFC2131-4.1-10`](#rfc2131-4.1-10), [`RFC2131-3.4-1`](#rfc2131-3.4-1), [`RFC2131-2-4`](#rfc2131-2-4), [`RFC2131-3.5-1`](#rfc2131-3.5-1), [`RFC2131-3.1-8`](#rfc2131-3.1-8), [`RFC2131-4.3.1-1`](#rfc2131-4.3.1-1), [`RFC2131-4.3.1-2`](#rfc2131-4.3.1-2), [`RFC2131-4.3.1-3`](#rfc2131-4.3.1-3), [`RFC2131-4.3.1-5`](#rfc2131-4.3.1-5), [`RFC2131-4.3.1-7`](#rfc2131-4.3.1-7), [`RFC2131-4.4.1-1`](#rfc2131-4.4.1-1), [`RFC2131-4.4.2-1`](#rfc2131-4.4.2-1), [`RFC2131-4.4.2-2`](#rfc2131-4.4.2-2), [`RFC2131-4.4.3-1`](#rfc2131-4.4.3-1), [`RFC2131-4.3.2-5`](#rfc2131-4.3.2-5), [`RFC2131-4.4.5-5`](#rfc2131-4.4.5-5), [`RFC2131-4.4.5-6`](#rfc2131-4.4.5-6), [`RFC2131-4.4.5-7`](#rfc2131-4.4.5-7), [`RFC2131-3.2-3`](#rfc2131-3.2-3), [`RFC2131-4.4-1`](#rfc2131-4.4-1), [`RFC2131-4.4-2`](#rfc2131-4.4-2), [`RFC2131-4.4-3`](#rfc2131-4.4-3)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC2131-4.3-1` | Server identifier MUST MUST MUST (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestServerIdentifierInEveryReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L169). **negative:** no negative test. **{single-polarity}:** every reply path appends option 54 unconditionally -- buildReply at internal/plugins/dhcpserver/handler.go:246 and buildNak at handler.go:349 -- so no input suppresses the server identifier and there is no omission to assert negatively |
| `RFC2131-4.3-2` | IP address lease time MUST (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestLeaseTimeInOfferAndAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L194). **negative:** no negative test. **{single-polarity}:** the lease time is appended to every OFFER and ACK unconditionally (buildReply internal/plugins/dhcpserver/handler.go:271-273, inside the OFFER/ACK branch opened at handler.go:248), so no input yields a DHCPOFFER without option 51 to assert negatively |
| `RFC2131-4.3-3` | IP address lease time MUST MUST (DHCPREQUEST) (§4.3) | MUST | 4.3 | **positive:** `unit/verify` [`TestLeaseTimeInOfferAndAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L198). **negative:** no negative test. **{single-polarity}:** the lease time is appended to every OFFER and ACK unconditionally (buildReply internal/plugins/dhcpserver/handler.go:271-273, inside the OFFER/ACK branch opened at handler.go:248), so no DHCPREQUEST yields a DHCPACK without option 51 to assert negatively |
| `RFC2131-4.3-4` | IP address lease time MUST MUST (DHCPREQUEST) MUST NOT (§4.3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L245). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L333) |
| `RFC2131-4.3-5` | Requested IP address MUST NOT MUST NOT MUST NOT (§4.3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L224). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L312) |
| `RFC2131-4.3-6` | Client identifier MUST NOT MUST NOT (§4.3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L239). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L327) |
| `RFC2131-4.3-7` | Parameter request list MUST NOT MUST NOT MUST NOT (§4.3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L228). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L316) |
| `RFC2131-4.3-8` | Maximum message size MUST NOT MUST NOT MUST NOT (§4.3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L232). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L320) |
| `RFC2131-4.3-9` | All others MAY MAY MUST NOT (§4.3) | MUST NOT | 4.3 | **positive:** `unit/verify` [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L249). **negative:** `unit/verify` [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L337) |
| `RFC2131-4.3.5-1` | The server MUST NOT send a lease expiration time to the client (§4.3.5) | MUST NOT | 4.3.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze answers no DHCPINFORM at all -- the message-type switch in handle (internal/plugins/dhcpserver/handler.go:121-134) dispatches DISCOVER, REQUEST, RELEASE and DECLINE, and every other type including msgInform (handler.go:37) falls to the default branch that returns nil, so ze builds no DHCPINFORM response at all |
| `RFC2131-4.3.2-1` | If 'giaddr' is 0x0 in the DHCPREQUEST message, the client is on the same subnet as the server. The server MUST broadcast the DHCPNAK message to the 0xffffffff broadcast address (§4.3.2) | MUST | 4.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with giaddr zero the delivery path unicasts a DHCPNAK to a non-zero ciaddr instead of broadcasting -- responseAddr tests giaddr, then ciaddr, before it ever reaches the broadcast decision (internal/plugins/dhcpserver/register.go:275-298) and never special-cases the NAK message type, so a REQUEST whose ciaddr is on-subnet and whose requested address is off-subnet is NAKed to ciaddr:68 |
| `RFC2131-4.3.2-2` | If 'giaddr' is set in the DHCPREQUEST message, the client is on a different subnet. The server MUST set the broadcast bit in the DHCPNAK, so that the relay agent will broadcast the DHCPNAK to the client (§4.3.2) | MUST | 4.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** buildNak copies the client's flags word verbatim (internal/plugins/dhcpserver/handler.go:339) and never forces the BROADCAST bit, so a DHCPNAK relayed through a non-zero giaddr leaves the server with the broadcast bit clear whenever the client left it clear |
| `RFC2131-3.2-4` | Otherwise, the server MUST send the DHCPNAK message to the IP address of the BOOTP relay agent, as recorded in 'giaddr'. (§3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestNakDeliveredToRelayAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/nak_relay_rfc2131_test.go#L19). **negative:** `unit/verify` [`TestNakDeliveredToRelayAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/nak_relay_rfc2131_test.go#L20) |
| `RFC2131-4.3.2-3` | If the DHCP server has no record of this client, then it MUST remain silent (§4.3.2) | MUST | 4.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the INIT-REBOOT branch commits a binding for any in-subnet requested address without consulting the lease table (handleRequest internal/plugins/dhcpserver/handler.go:178-183 calls commitBinding at handler.go:194), so a client the server holds no record of draws a DHCPACK rather than silence |
| `RFC2131-4.3.3-1` | The server MUST mark the network address as not available (§4.3.3) | MUST | 4.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** markUnavailable sets the pool bit and the static set for the declined address (internal/plugins/dhcpserver/pool.go:144-163), but the declining client's MAC-to-address cache entry survives -- pool.release returns early for a staticSet address (pool.go:174-177) -- so pool.allocate hands that same declined address back to that client on its next DISCOVER (pool.go:68-72) |
| `RFC2131-4.1-1` | A server with multiple network addresses MUST be prepared to to accept any of its network addresses as identifying that server in a DHCP message. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** each handler accepts exactly one address as its server identifier -- handleRequest discards a REQUEST whose option 54 differs from its own serverIP (internal/plugins/dhcpserver/handler.go:167-169) -- and that address is the single per-subnet value derived at register.go:93-101, so a REQUEST naming another of the server's own addresses is silently dropped |
| `RFC2131-4.1-2` | To accommodate potentially incomplete network connectivity, a server MUST choose an address as a 'server identifier' that, to the best of the server's knowledge, is reachable from the client. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** with no default-router configured for a subnet the server identifier is not an address the server answers on -- register.go:93 prefers sub.DefaultRouter, which parseSubnet leaves unset when the leaf is absent (internal/plugins/dhcpserver/config.go:189-195), and falls back to the first pool address (internal/plugins/dhcpserver/register.go:95-97), which the server itself hands out to a client, or to the subnet network address (register.go:99-101), which is no host at all; buildReply then emits that value as option 54 (internal/plugins/dhcpserver/handler.go:246), so a client unicasting to it reaches nothing |
| `RFC2131-4.2-1` | If the client supplies a 'client identifier', the client MUST use the same 'client identifier' in all subsequent messages, and the server MUST use that identifier to identify the client. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze never reads the client identifier -- option 61 is absent from the option codes at internal/plugins/dhcpserver/handler.go:41-61 and no parse call asks for it -- and keys every binding on the hardware address instead (extractMAC handler.go:456, leaseTable byMAC lease.go:23, pool.macToAddr pool.go:22), so a client that supplies a client identifier is still identified by chaddr |
| `RFC2131-4.2-2` | If the client does not provide a 'client identifier' option, the server MUST use the contents of the 'chaddr' field to identify the client. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestChaddrIdentifiesClientWithoutClientIdentifier`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L366). **negative:** `unit/verify` [`TestUnidentifiableClientRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L390) |
| `RFC2131-2-1` | The 'client identifier' chosen by a DHCP client MUST be unique to that client within the subnet to which the client is attached. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's client emits the operator-configured client identifier verbatim and nothing derives or checks it -- v4RequestModifiers appends OptClientIdentifier([]byte(c.config.ClientID)) (internal/plugins/iface/dhcp/dhcp_v4_linux.go:304-306) from the config leaf read at internal/component/iface/config.go:1267-1269 and carried at internal/component/iface/register.go:891, and no validator constrains the value, so two units on one subnet configured with the same client-id emit colliding identifiers |
| `RFC2131-2-2` | If the client uses a 'client identifier' in one message, it MUST use that same identifier in all subsequent messages, to ensure that all servers correctly identify the client. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's client sends option 61 at acquisition and omits it on every renewal -- the DORA call passes v4RequestModifiers() (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, built at :299-308) while renewV4 calls client.Renew(ctx, lease) with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143), and the renewal REQUEST is built by NewRenewFromAck (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290) whose WithReply copies only opcode, HW type, xid, chaddr and flags (vendor/github.com/insomniacslk/dhcp/dhcpv4/modifiers.go:56-68) and never an option, so the identifier the server bound at acquisition is absent from every later message |
| `RFC2131-3.1-1` | The client broadcasts a DHCPREQUEST message that MUST include the 'server identifier' option to indicate which server it has selected (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze hands SELECTING message construction to the vendored library -- runV4 calls client.Request (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48), which builds the REQUEST in NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) where the server identifier is copied from the OFFER at dhcpv4.go:263; ze contributes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308) and authors none of those bytes |
| `RFC2131-3.1-2` | The 'requested IP address' option MUST be set to the value of 'yiaddr' in the DHCPOFFER message from the server. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze hands SELECTING message construction to the vendored library -- runV4 calls client.Request (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48), which builds the REQUEST in NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) where the requested IP address option is set from the OFFER's yiaddr at dhcpv4.go:261; ze contributes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308) and authors none of those bytes |
| `RFC2131-3.1-3` | 'requested IP address' option MUST be filled in with client's notion of its previously assigned address. (§4.3.2) | MUST | 4.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| `RFC2131-3.1-4` | DHCPREQUEST generated during INIT-REBOOT state: 'server identifier' MUST NOT be filled in (§4.3.2) | MUST NOT | 4.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| `RFC2131-3.1-5` | 'server identifier' MUST NOT be filled in, 'requested IP address' option MUST NOT be filled in (§4.3.2) | MUST NOT | 4.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| `RFC2131-3.2-1` | As the client has not received its network address, it MUST NOT fill in the 'ciaddr' field. (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| `RFC2131-3.1-6` | If the client used a 'client identifier' when it obtained the lease, it MUST use the same 'client identifier' in the DHCPRELEASE message. (§3.1, §3.2) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPRELEASE, so no ze path builds the message whose fields this governs -- runV4 ends a lease by removing the address locally (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 211-235), and nclient4's Release (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:26) is called nowhere in ze |
| `RFC2131-3.1-7` | If the client detects that the address is already in use (e.g., through the use of ARP), the client MUST send a DHCPDECLINE message to the server and restarts the configuration process. (§3.1, §3.2) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's client installs the ACKed address without checking it is free and has no DHCPDECLINE path -- handleV4Lease goes straight from the ACK to ReplaceAddressWithLifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) with no probe of the address, and neither ze nor the vendored nclient4 client contains a DECLINE producer, so an address found in use is kept rather than declined |
| `RFC2131-4.4.5-1` | If the lease expires before the client receives a DHCPACK, the client moves to INIT state, MUST immediately stop any other network processing and requests network initialization parameters as if the client were uninitialized. (§4.4.5) | MUST | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze tears the lease down late -- runV4 budgets fixed sleeps of lease/2, 3*lease/8 and lease/8 (internal/plugins/iface/dhcp/dhcp_v4_linux.go:80-118) and calls renewV4 between them, so the blocking renewal attempts add to that budget: each failed renewal spends the vendored retry schedule of 5 seconds doubling over 3 tries (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:32, 35, retryFn 649-669), leaving the lease's default route (internal/plugins/iface/dhcp/dhcp_v4_linux.go:190) installed for around 70 seconds past expiry before removeV4Addr runs (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 228-235); only the address itself leaves on time, because the kernel holds the lease duration as its valid lifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:180, internal/plugins/iface/netlink/manage_linux.go:268-286) |
| `RFC2131-3-1` | The first four octets of the 'options' field of the DHCP message contain the (decimal) values 99, 130, 83 and 99, respectively (§3) | MUST | 3 | **positive:** `unit/verify` [`TestMagicCookieRequiredAndEmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L423). **negative:** `unit/verify` [`TestMagicCookieRequiredAndEmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L431) |
| `RFC2131-4.1-3` | The last option must always be the 'end' option. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestReplyOptionsTerminatedByEnd`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L453). **negative:** `unit/verify` [`TestReplyOptionsTerminatedByEnd`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L471) |
| `RFC2131-4.1-4` | If the options in a DHCP message extend into the 'sname' and 'file' fields, the 'option overload' option MUST appear in the 'options' field, with value 1, 2 or 3, as specified in RFC 1533. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field |
| `RFC2131-4.1-5` | The options in the 'sname' and 'file' fields (if in use as indicated by the 'options overload' option) MUST begin with the first octet of the field, MUST be terminated by an 'end' option, and MUST be followed by 'pad' options to fill the remainder of the field. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field |
| `RFC2131-4.1-6` | Any individual option in the 'options', 'sname' and 'file' fields MUST be entirely contained in that field. (§4.1) | MUST | 4.1 | **positive:** `unit/verify` [`TestOptionsContainedWithinTheirField`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L492). **negative:** `unit/verify` [`TestOptionsContainedWithinTheirField`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L515) |
| `RFC2131-4.1-7` | The options in the 'options' field MUST be interpreted first, so that any 'option overload' options may be interpreted. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field; ze reads options from the options field only, so it holds no second option stream to order against it |
| `RFC2131-4.1-8` | The client MUST adopt a retransmission strategy that incorporates a randomized exponential backoff algorithm to determine the delay between retransmissions. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's client retransmits on a doubling timeout with no randomization -- runV4 and renewV4 build the client with no ClientOpt (internal/plugins/iface/dhcp/dhcp_v4_linux.go:37, 128), so it runs the vendored defaults of a 5 second timeout over 3 tries (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:32, 35, 183) through retryFn, which only doubles the timeout between tries (client.go:649-669) |
| `RFC2131-4.1-9` | A DHCP client MUST choose 'xid's in such a way as to minimize the chance of using an 'xid' identical to one used by another client. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the transaction ID is drawn inside the vendored library -- every message ze's client sends is built through dhcpv4.New (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:144-150), which takes the xid from crypto/rand via GenerateTransactionID (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:121-139); ze calls only client.Request and client.Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) and authors no xid |
| `RFC2131-4.1-10` | DHCP clients MUST use the IP address provided in the 'server identifier' option for any unicast requests to the DHCP server. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze's renewal is not unicast to the server identifier -- renewV4 creates the client with nclient4.New(c.ifaceName) and no WithServerAddr (internal/plugins/iface/dhcp/dhcp_v4_linux.go:128), so serverAddr keeps the library default of 255.255.255.255:67 (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:50-53, assigned at client.go:183) and Renew sends the RENEWING REQUEST to that broadcast address (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:60) instead of to the server identifier the ACK carried |
| `RFC2131-3.4-1` | The server SHOULD check the network address in a DHCPINFORM message for consistency, but MUST NOT check for an existing lease. (§3.4) | MUST NOT | 3.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze answers no DHCPINFORM at all -- the message-type switch in handle (internal/plugins/dhcpserver/handler.go:121-134) dispatches DISCOVER, REQUEST, RELEASE and DECLINE, and every other type including msgInform (handler.go:37) falls to the default branch that returns nil, so ze builds no DHCPINFORM response at all, so no response path exists that could consult an existing lease |
| `RFC2131-2-3` | The remaining bits of the flags field are reserved for future use. They MUST be set to zero by clients and ignored by servers and relay agents. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestFlagsReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L549). **negative:** `unit/verify` [`TestFlagsReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L560) |
| `RFC2131-2-4` | A DHCP client must be prepared to receive DHCP messages with an 'options' field of at least length 312 octets. (§2) | MUST | 2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze reads no DHCP bytes itself -- the receive path is the vendored library's receiveLoop, which reads each datagram into a MaxMessageSize (1500 octet) buffer before decoding (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:256-271, MaxMessageSize at client.go:38), and ze only consumes the decoded result of client.Request and client.Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) |
| `RFC2131-3.5-1` | If the client includes a list of parameters in a DHCPDISCOVER message, it MUST include that list in any subsequent DHCPREQUEST messages. (§3.5) | MUST | 3.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the parameter request list is written by the vendored library, the same list in both messages -- NewDiscovery (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:198-209) and NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) each add the identical WithRequestedOptions set, and ze passes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308), so it authors no request list |
| `RFC2131-4.4.5-2` | T1 MUST be earlier than T2 (§4.4.5) | MUST | 4.4.5 | **positive:** `unit/verify` [`TestRenewalTimersOrderedWithinLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L603). **negative:** `unit/verify` [`TestShortLeaseTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L630) |
| `RFC2131-4.4.5-3` | T2, which, in turn, MUST be earlier than the time at which the client's lease will expire. (§4.4.5) | MUST | 4.4.5 | **positive:** `unit/verify` [`TestRenewalTimersOrderedWithinLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L607). **negative:** `unit/verify` [`TestShortLeaseTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L635) |
| `RFC2131-3.1-8` | To help ensure that any BOOTP relay agents forward the DHCPREQUEST message to the same set of DHCP servers that received the original DHCPDISCOVER message, the DHCPREQUEST message MUST use the same value in the DHCP message header's 'secs' field and be sent to the same IP broadcast address as the original DHCPDISCOVER message. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the secs field and the broadcast flag of the SELECTING REQUEST are the vendored library's -- it leaves secs zero in both the DISCOVER and the REQUEST (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:164-180 sets no secs) and copies the flags word across with WithReply (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:258, vendor/github.com/insomniacslk/dhcp/dhcpv4/modifiers.go:56-68), while ze's DORA call and its modifiers touch neither (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 299-308) |
| `RFC2131-3.1-9` | When allocating a new address, servers SHOULD check that the offered network address is not already in use; e.g., the server may probe the offered address with an ICMP Echo Request. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.4-1` | The server SHOULD retain a record of the client's initialization parameters for possible reuse in response to subsequent requests from the client. (§4.3.4) | SHOULD | 4.3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-10` | The server SHOULD mark an address offered to a client in a DHCPOFFER message as available if the server receives no DHCPREQUEST message from that client. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.2-4` | If the DHCP server detects that the client is on the wrong net (i.e., the result of applying the local subnet mask or remote subnet mask (if 'giaddr' is not zero) to 'requested IP address' option value doesn't match reality), then the server SHOULD send a DHCPNAK message to the client. (§4.3.2) | SHOULD | 4.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.3-2` | SHOULD notify the local system administrator of a possible configuration problem. (§4.3.3) | SHOULD | 4.3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.5-2` | SHOULD NOT fill in 'yiaddr'. (§4.3.5) | SHOULD NOT | 4.3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.4-2` | The servers SHOULD unicast the DHCPACK reply to the address given in the 'ciaddr' field of the DHCPINFORM message. (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.4-3` | The server SHOULD check the network address in a DHCPINFORM message for consistency (§3.4) | SHOULD | 3.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-11` | Any configuration parameters in the DHCPACK message SHOULD NOT conflict with those in the earlier DHCPOFFER message to which the client is responding. (§3.1) | SHOULD NOT | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-12` | The server SHOULD NOT check the offered network address at this point. (§3.1) | SHOULD NOT | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-13` | The client SHOULD perform a final check on the parameters (e.g., ARP for allocated network address) (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-14` | The client SHOULD wait a minimum of ten seconds before restarting the configuration process to avoid excessive network traffic in case of looping. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-15` | The client SHOULD notify the user that the initialization process has failed and is restarting. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.5-2` | The client SHOULD include the 'maximum DHCP message size' option to let the server know how large the server may make its DHCP messages. (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.7-1` | A client SHOULD use DHCP to reacquire or verify its IP address and network parameters whenever the local network parameters may have changed; e.g., at system boot time or after a disconnection from the local network, as the local network configuration may change without the client's or user's knowledge. (§3.7) | SHOULD | 3.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-2-5` | The TCP/IP software SHOULD accept and forward to the IP layer any IP packets delivered to the client's hardware address before the IP address is configured (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.1-11` | A client that cannot receive unicast IP datagrams until its protocol software has been configured with an IP address SHOULD set the BROADCAST bit in the 'flags' field to 1 in any DHCPDISCOVER or DHCPREQUEST messages that client sends. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.1-12` | A server or relay agent sending or relaying a DHCP message directly to a DHCP client (i.e., not to a relay agent specified in the 'giaddr' field) SHOULD examine the BROADCAST bit in the 'flags' field. If this bit is set to 1, the DHCP message SHOULD be sent as an IP broadcast using an IP broadcast address (preferably 0xffffffff) as the IP destination address and the link-layer broadcast address as the link-layer destination address. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.5-4` | Times T1 and T2 SHOULD be chosen with some random "fuzz" around a fixed value, to avoid synchronization of client reacquisition. (§4.4.5) | SHOULD | 4.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.1-13` | the delay before the first retransmission SHOULD be 4 seconds randomized by the value of a uniform random number chosen from the range -1 to +1. Clients with clocks that provide resolution granularity of less than one second may choose a non-integer randomization value. The delay before the next retransmission SHOULD be 8 seconds randomized by the value of a uniform number chosen from the range -1 to +1. The retransmission delay SHOULD be doubled with subsequent retransmissions up to a maximum of 64 seconds. (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-2.2-1` | As a consistency check, the allocating server SHOULD probe the reused address before allocating the address, e.g., with an ICMP echo request (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-2.2-2` | the client SHOULD probe the newly received address, e.g., with ARP. (§2.2) | SHOULD | 2.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-16` | The DHCPDISCOVER message MAY include options that suggest values for the network address and lease duration. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.1-17` | A server MAY choose to mark addresses offered to clients in DHCPOFFER messages as unavailable. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.2-3` | a server MAY, for administrative reasons, assign an address other than the one requested, or may refuse to allocate an address to a particular client even though free addresses are available. (§4.3.1) | MAY | 4.3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.1-14` | A server with multiple network address (e.g., a multi-homed host) MAY use any of its network addresses in outgoing DHCP messages. (§4.1) | MAY | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.2-2` | If the client receives neither a DHCPACK or a DHCPNAK message after employing the retransmission algorithm, the client MAY choose to use the previously allocated network address and configuration parameters for the remainder of the unexpired lease. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.1-1` | The configuration parameters MUST be selected by applying the following rules in the order given below. (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze applies no parameter-selection procedure -- buildReply emits one fixed option set in one fixed order (internal/plugins/dhcpserver/handler.go:245-285) and the parameter request list is never parsed (optParamReqList is defined at handler.go:52 and read nowhere in production), so the ordered rules of Section 4.3.1 govern no ze code path |
| `RFC2131-4.3.1-2` | -- IF the server has been explicitly configured with a default value for the parameter, the server MUST include that value in an appropriate option in the 'option' field, ELSE (§4.3.1) | MUST | 4.3.1 | **positive:** `unit/verify` [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L697). **negative:** no negative test. **{single-polarity}:** ze emits its configured values unconditionally (buildReply internal/plugins/dhcpserver/handler.go:245-285), so a configured value is present whether or not the client asked for it and no input suppresses one to assert negatively |
| `RFC2131-4.3.1-3` | -- IF the server recognizes the parameter as a parameter defined in the Host Requirements Document, the server MUST include the default value for that parameter as given in the Host Requirements Document in an appropriate option in the 'option' field, ELSE (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze returns only the values its own subnet configuration holds (buildReply internal/plugins/dhcpserver/handler.go:245-285); it reads no parameter request list (optParamReqList handler.go:52 is parsed nowhere in production) and carries no Host Requirements defaults, so a recognized parameter the client requests draws no default value |
| `RFC2131-4.3.1-4` | The server MUST NOT return a value for that parameter, (§4.3.1) | MUST NOT | 4.3.1 | **positive:** `unit/verify` [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L668). **negative:** `unit/verify` [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L682) |
| `RFC2131-4.3.1-5` | The server MUST supply as many of the requested parameters as possible and MUST omit any parameters it cannot provide. (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the parameter request list is never parsed (optParamReqList internal/plugins/dhcpserver/handler.go:52 is read nowhere in production), so ze supplies its fixed configured option set (buildReply handler.go:245-285) rather than as many of the client's requested parameters as it can |
| `RFC2131-4.3.1-6` | The server MUST include each requested parameter only once unless explicitly allowed in the DHCP Options and BOOTP Vendor Extensions document. (§4.3.1) | MUST | 4.3.1 | **positive:** `unit/verify` [`TestEachParameterEmittedOnce`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L723). **negative:** `unit/verify` [`TestEachParameterEmittedOnce`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L744) |
| `RFC2131-4.3.1-7` | o Any parameters specific to this client's class (as identified by the contents of the 'vendor class identifier' option in the DHCPDISCOVER or DHCPREQUEST message), e.g., as configured by the network administrator; the parameters MUST be identified by an exact match between the client's vendor class identifiers and the client's classes identified in the server, (§4.3.1) | MUST | 4.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze identifies the vendor class by a ten-octet prefix comparison against the string PXEClient: (isPXEClient internal/plugins/dhcpserver/handler.go:493-496) rather than by an exact match against a configured class identifier, and that prefix match is what selects the class-specific options appended at handler.go:292-330 |
| `RFC2131-4.4.1-1` | The client MUST include its hardware address in the 'chaddr' field, if necessary for delivery of DHCP reply messages. (§4.4.1) | MUST | 4.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** chaddr is filled by the vendored library from the bound interface -- nclient4.New resolves the interface MAC (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:199-209) and NewDiscovery/NewRequestFromOffer set it with WithHwAddr (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:198-209, 256-270); ze only names the interface to bind (internal/plugins/iface/dhcp/dhcp_v4_linux.go:37) |
| `RFC2131-4.4.2-1` | The client MUST insert its known network address as a 'requested IP address' option in the DHCPREQUEST message. (§4.4.2) | MUST | 4.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| `RFC2131-4.4.2-2` | The client MUST NOT include a 'server identifier' in the DHCPREQUEST message. (§4.4.2) | MUST NOT | 4.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| `RFC2131-4.4.3-1` | DHCPINFORM messages MUST be directed to the 'DHCP server' UDP port (§4.4.3) | MUST | 4.4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's client sends no DHCPINFORM -- nclient4 exposes Inform (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:483-498) and ze calls it nowhere; runV4 and renewV4 use only Request and Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) |
| `RFC2131-4.3.2-5` | This message MUST be broadcast to the 0xffffffff IP broadcast address. (§4.3.2) | MUST | 4.3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's T2 attempt reuses the same vendored Renew call as its T1 attempt (internal/plugins/iface/dhcp/dhcp_v4_linux.go:103 and 143), so both the message and its destination are the library's: NewRenewFromAck builds it (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290) and it goes to the default serverAddr of 255.255.255.255:67 (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:50-53, 183, used at nclient4/lease.go:60); ze chooses neither |
| `RFC2131-4.4.5-5` | If the client is given a new network address, it MUST NOT continue using the previous network address (§4.4.5) | MUST NOT | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a renewal that returns a different yiaddr leaves the previous address in use -- renewV4 hands the new ACK to handleV4Lease (internal/plugins/iface/dhcp/dhcp_v4_linux.go:154), which installs the new address with ReplaceAddressWithLifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:180), a per-address replace that touches no other address (internal/plugins/iface/netlink/manage_linux.go:268-286), and runV4 then overwrites its ack reference (internal/plugins/iface/dhcp/dhcp_v4_linux.go:87-88) so the previous address is never passed to removeV4Addr; both addresses stay configured on the interface |
| `RFC2131-4.4.5-6` | The client MUST NOT include a 'server identifier' in the DHCPREQUEST message. (§4.4.5) | MUST NOT | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| `RFC2131-4.4.5-7` | The client MUST NOT include a 'server identifier' in the DHCPREQUEST message. (§4.4.5) | MUST NOT | 4.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| `RFC2131-3.2-3` | As the client has not received its network address, it MUST NOT fill in the 'ciaddr' field. (§3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| `RFC2131-4.4-1` | Vendor class identifier MAY MAY MUST NOT (§4.4) | MUST NOT | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| `RFC2131-4.4-2` | Requested IP address MAY MUST (in MUST (DISCOVER) SELECTING or (DHCPDECLINE), (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| `RFC2131-4.4-3` | Server identifier MUST NOT MUST (after MUST SELECTING) (§4.4) | MUST | 4.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's DHCPv4 client sends no DHCPRELEASE, so no ze path builds the message whose fields this governs -- runV4 ends a lease by removing the address locally (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 211-235), and nclient4's Release (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:26) is called nowhere in ze |
| `RFC2131-4.4.3-2` | The client SHOULD NOT request lease time parameters. (§4.4.3) | SHOULD NOT | 4.4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.1-2` | The client SHOULD wait a random time between one and ten seconds to desynchronize the use of DHCP at startup. (§4.4.1) | SHOULD | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.1-3` | The client SHOULD broadcast an ARP reply to announce the client's new IP address and clear any outdated ARP cache entries in hosts on the client's subnet. (§4.4.1) | SHOULD | 4.4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.2-6` | The DHCP server SHOULD check 'ciaddr' for correctness before replying to the DHCPREQUEST. (§4.3.2) | SHOULD | 4.3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.2-4` | The client implementation of DHCP SHOULD provide a mechanism for the user to select directly the 'vendor class identifier' values. (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.5-8` | In both RENEWING and REBINDING states, if the client receives no response to its DHCPREQUEST message, the client SHOULD wait one-half of the remaining time until T2 (in RENEWING state) and one-half of the remaining lease time (in REBINDING state), down to a minimum of 60 seconds, before retransmitting the DHCPREQUEST message. (§4.4.5) | SHOULD | 4.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.5-9` | If the client then receives a DHCPACK allocating that client its previous network address, the client SHOULD continue network processing. (§4.4.5) | SHOULD | 4.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.4.5-10` | If the client is given a new network address, it MUST NOT continue using the previous network address and SHOULD notify the local users of the problem. (§4.4.5) | SHOULD | 4.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-3.5-3` | If a server receives a DHCPREQUEST message with an invalid 'requested IP address', the server SHOULD respond to the client with a DHCPNAK message (§3.5) | SHOULD | 3.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC2131-4.3.1-8` | If an address is available, the new address SHOULD be chosen as follows: (§4.3.1) | SHOULD | 4.3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC2131-4.3.5-1`](#rfc2131-4.3.5-1) The server MUST NOT send a lease expiration time to the client (§4.3.5) | no test | no test carries this requirement id; annotated {not-applicable}: ze answers no DHCPINFORM at all -- the message-type switch in handle (internal/plugins/dhcpserver/handler.go:121-134) dispatches DISCOVER, REQUEST, RELEASE and DECLINE, and every other type including msgInform (handler.go:37) falls to the default branch that returns nil, so ze builds no DHCPINFORM response at all |
| [`RFC2131-4.3.2-1`](#rfc2131-4.3.2-1) If 'giaddr' is 0x0 in the DHCPREQUEST message, the client is on the same subnet as the server. The server MUST broadcast the DHCPNAK message to the 0xffffffff broadcast address (§4.3.2) | {gap}, no test | with giaddr zero the delivery path unicasts a DHCPNAK to a non-zero ciaddr instead of broadcasting -- responseAddr tests giaddr, then ciaddr, before it ever reaches the broadcast decision (internal/plugins/dhcpserver/register.go:275-298) and never special-cases the NAK message type, so a REQUEST whose ciaddr is on-subnet and whose requested address is off-subnet is NAKed to ciaddr:68 |
| [`RFC2131-4.3.2-2`](#rfc2131-4.3.2-2) If 'giaddr' is set in the DHCPREQUEST message, the client is on a different subnet. The server MUST set the broadcast bit in the DHCPNAK, so that the relay agent will broadcast the DHCPNAK to the client (§4.3.2) | {gap}, no test | buildNak copies the client's flags word verbatim (internal/plugins/dhcpserver/handler.go:339) and never forces the BROADCAST bit, so a DHCPNAK relayed through a non-zero giaddr leaves the server with the broadcast bit clear whenever the client left it clear |
| [`RFC2131-4.3.2-3`](#rfc2131-4.3.2-3) If the DHCP server has no record of this client, then it MUST remain silent (§4.3.2) | {gap}, no test | the INIT-REBOOT branch commits a binding for any in-subnet requested address without consulting the lease table (handleRequest internal/plugins/dhcpserver/handler.go:178-183 calls commitBinding at handler.go:194), so a client the server holds no record of draws a DHCPACK rather than silence |
| [`RFC2131-4.3.3-1`](#rfc2131-4.3.3-1) The server MUST mark the network address as not available (§4.3.3) | {gap}, no test | markUnavailable sets the pool bit and the static set for the declined address (internal/plugins/dhcpserver/pool.go:144-163), but the declining client's MAC-to-address cache entry survives -- pool.release returns early for a staticSet address (pool.go:174-177) -- so pool.allocate hands that same declined address back to that client on its next DISCOVER (pool.go:68-72) |
| [`RFC2131-4.1-1`](#rfc2131-4.1-1) A server with multiple network addresses MUST be prepared to to accept any of its network addresses as identifying that server in a DHCP message. (§4.1) | {gap}, no test | each handler accepts exactly one address as its server identifier -- handleRequest discards a REQUEST whose option 54 differs from its own serverIP (internal/plugins/dhcpserver/handler.go:167-169) -- and that address is the single per-subnet value derived at register.go:93-101, so a REQUEST naming another of the server's own addresses is silently dropped |
| [`RFC2131-4.1-2`](#rfc2131-4.1-2) To accommodate potentially incomplete network connectivity, a server MUST choose an address as a 'server identifier' that, to the best of the server's knowledge, is reachable from the client. (§4.1) | {gap}, no test | with no default-router configured for a subnet the server identifier is not an address the server answers on -- register.go:93 prefers sub.DefaultRouter, which parseSubnet leaves unset when the leaf is absent (internal/plugins/dhcpserver/config.go:189-195), and falls back to the first pool address (internal/plugins/dhcpserver/register.go:95-97), which the server itself hands out to a client, or to the subnet network address (register.go:99-101), which is no host at all; buildReply then emits that value as option 54 (internal/plugins/dhcpserver/handler.go:246), so a client unicasting to it reaches nothing |
| [`RFC2131-4.2-1`](#rfc2131-4.2-1) If the client supplies a 'client identifier', the client MUST use the same 'client identifier' in all subsequent messages, and the server MUST use that identifier to identify the client. (§4.2) | {gap}, no test | ze never reads the client identifier -- option 61 is absent from the option codes at internal/plugins/dhcpserver/handler.go:41-61 and no parse call asks for it -- and keys every binding on the hardware address instead (extractMAC handler.go:456, leaseTable byMAC lease.go:23, pool.macToAddr pool.go:22), so a client that supplies a client identifier is still identified by chaddr |
| [`RFC2131-2-1`](#rfc2131-2-1) The 'client identifier' chosen by a DHCP client MUST be unique to that client within the subnet to which the client is attached. (§2) | {gap}, no test | ze's client emits the operator-configured client identifier verbatim and nothing derives or checks it -- v4RequestModifiers appends OptClientIdentifier([]byte(c.config.ClientID)) (internal/plugins/iface/dhcp/dhcp_v4_linux.go:304-306) from the config leaf read at internal/component/iface/config.go:1267-1269 and carried at internal/component/iface/register.go:891, and no validator constrains the value, so two units on one subnet configured with the same client-id emit colliding identifiers |
| [`RFC2131-2-2`](#rfc2131-2-2) If the client uses a 'client identifier' in one message, it MUST use that same identifier in all subsequent messages, to ensure that all servers correctly identify the client. (§2) | {gap}, no test | ze's client sends option 61 at acquisition and omits it on every renewal -- the DORA call passes v4RequestModifiers() (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, built at :299-308) while renewV4 calls client.Renew(ctx, lease) with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143), and the renewal REQUEST is built by NewRenewFromAck (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290) whose WithReply copies only opcode, HW type, xid, chaddr and flags (vendor/github.com/insomniacslk/dhcp/dhcpv4/modifiers.go:56-68) and never an option, so the identifier the server bound at acquisition is absent from every later message |
| [`RFC2131-3.1-1`](#rfc2131-3.1-1) The client broadcasts a DHCPREQUEST message that MUST include the 'server identifier' option to indicate which server it has selected (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze hands SELECTING message construction to the vendored library -- runV4 calls client.Request (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48), which builds the REQUEST in NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) where the server identifier is copied from the OFFER at dhcpv4.go:263; ze contributes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308) and authors none of those bytes |
| [`RFC2131-3.1-2`](#rfc2131-3.1-2) The 'requested IP address' option MUST be set to the value of 'yiaddr' in the DHCPOFFER message from the server. (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze hands SELECTING message construction to the vendored library -- runV4 calls client.Request (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48), which builds the REQUEST in NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) where the requested IP address option is set from the OFFER's yiaddr at dhcpv4.go:261; ze contributes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308) and authors none of those bytes |
| [`RFC2131-3.1-3`](#rfc2131-3.1-3) 'requested IP address' option MUST be filled in with client's notion of its previously assigned address. (§4.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| [`RFC2131-3.1-4`](#rfc2131-3.1-4) DHCPREQUEST generated during INIT-REBOOT state: 'server identifier' MUST NOT be filled in (§4.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| [`RFC2131-3.1-5`](#rfc2131-3.1-5) 'server identifier' MUST NOT be filled in, 'requested IP address' option MUST NOT be filled in (§4.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| [`RFC2131-3.2-1`](#rfc2131-3.2-1) As the client has not received its network address, it MUST NOT fill in the 'ciaddr' field. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| [`RFC2131-3.1-6`](#rfc2131-3.1-6) If the client used a 'client identifier' when it obtained the lease, it MUST use the same 'client identifier' in the DHCPRELEASE message. (§3.1, §3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPRELEASE, so no ze path builds the message whose fields this governs -- runV4 ends a lease by removing the address locally (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 211-235), and nclient4's Release (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:26) is called nowhere in ze |
| [`RFC2131-3.1-7`](#rfc2131-3.1-7) If the client detects that the address is already in use (e.g., through the use of ARP), the client MUST send a DHCPDECLINE message to the server and restarts the configuration process. (§3.1, §3.2) | {gap}, no test | ze's client installs the ACKed address without checking it is free and has no DHCPDECLINE path -- handleV4Lease goes straight from the ACK to ReplaceAddressWithLifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) with no probe of the address, and neither ze nor the vendored nclient4 client contains a DECLINE producer, so an address found in use is kept rather than declined |
| [`RFC2131-4.4.5-1`](#rfc2131-4.4.5-1) If the lease expires before the client receives a DHCPACK, the client moves to INIT state, MUST immediately stop any other network processing and requests network initialization parameters as if the client were uninitialized. (§4.4.5) | {gap}, no test | ze tears the lease down late -- runV4 budgets fixed sleeps of lease/2, 3*lease/8 and lease/8 (internal/plugins/iface/dhcp/dhcp_v4_linux.go:80-118) and calls renewV4 between them, so the blocking renewal attempts add to that budget: each failed renewal spends the vendored retry schedule of 5 seconds doubling over 3 tries (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:32, 35, retryFn 649-669), leaving the lease's default route (internal/plugins/iface/dhcp/dhcp_v4_linux.go:190) installed for around 70 seconds past expiry before removeV4Addr runs (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 228-235); only the address itself leaves on time, because the kernel holds the lease duration as its valid lifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:180, internal/plugins/iface/netlink/manage_linux.go:268-286) |
| [`RFC2131-4.1-4`](#rfc2131-4.1-4) If the options in a DHCP message extend into the 'sname' and 'file' fields, the 'option overload' option MUST appear in the 'options' field, with value 1, 2 or 3, as specified in RFC 1533. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field |
| [`RFC2131-4.1-5`](#rfc2131-4.1-5) The options in the 'sname' and 'file' fields (if in use as indicated by the 'options overload' option) MUST begin with the first octet of the field, MUST be terminated by an 'end' option, and MUST be followed by 'pad' options to fill the remainder of the field. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field |
| [`RFC2131-4.1-7`](#rfc2131-4.1-7) The options in the 'options' field MUST be interpreted first, so that any 'option overload' options may be interpreted. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze neither emits nor honors option overload (52) -- buildReply and buildNak leave sname and file zeroed (internal/plugins/dhcpserver/handler.go:220-290 and 333-354) and every option reader starts at pkt[240:], the options field alone (parseMsgType handler.go:367, parseOptionAddr handler.go:386, parseOptionBytes handler.go:471), so no option ever lives in the sname or file field; ze reads options from the options field only, so it holds no second option stream to order against it |
| [`RFC2131-4.1-8`](#rfc2131-4.1-8) The client MUST adopt a retransmission strategy that incorporates a randomized exponential backoff algorithm to determine the delay between retransmissions. (§4.1) | {gap}, no test | ze's client retransmits on a doubling timeout with no randomization -- runV4 and renewV4 build the client with no ClientOpt (internal/plugins/iface/dhcp/dhcp_v4_linux.go:37, 128), so it runs the vendored defaults of a 5 second timeout over 3 tries (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:32, 35, 183) through retryFn, which only doubles the timeout between tries (client.go:649-669) |
| [`RFC2131-4.1-9`](#rfc2131-4.1-9) A DHCP client MUST choose 'xid's in such a way as to minimize the chance of using an 'xid' identical to one used by another client. (§4.1) | no test | no test carries this requirement id; annotated {not-applicable}: the transaction ID is drawn inside the vendored library -- every message ze's client sends is built through dhcpv4.New (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:144-150), which takes the xid from crypto/rand via GenerateTransactionID (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:121-139); ze calls only client.Request and client.Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) and authors no xid |
| [`RFC2131-4.1-10`](#rfc2131-4.1-10) DHCP clients MUST use the IP address provided in the 'server identifier' option for any unicast requests to the DHCP server. (§4.1) | {gap}, no test | ze's renewal is not unicast to the server identifier -- renewV4 creates the client with nclient4.New(c.ifaceName) and no WithServerAddr (internal/plugins/iface/dhcp/dhcp_v4_linux.go:128), so serverAddr keeps the library default of 255.255.255.255:67 (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:50-53, assigned at client.go:183) and Renew sends the RENEWING REQUEST to that broadcast address (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:60) instead of to the server identifier the ACK carried |
| [`RFC2131-3.4-1`](#rfc2131-3.4-1) The server SHOULD check the network address in a DHCPINFORM message for consistency, but MUST NOT check for an existing lease. (§3.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze answers no DHCPINFORM at all -- the message-type switch in handle (internal/plugins/dhcpserver/handler.go:121-134) dispatches DISCOVER, REQUEST, RELEASE and DECLINE, and every other type including msgInform (handler.go:37) falls to the default branch that returns nil, so ze builds no DHCPINFORM response at all, so no response path exists that could consult an existing lease |
| [`RFC2131-2-4`](#rfc2131-2-4) A DHCP client must be prepared to receive DHCP messages with an 'options' field of at least length 312 octets. (§2) | no test | no test carries this requirement id; annotated {not-applicable}: ze reads no DHCP bytes itself -- the receive path is the vendored library's receiveLoop, which reads each datagram into a MaxMessageSize (1500 octet) buffer before decoding (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:256-271, MaxMessageSize at client.go:38), and ze only consumes the decoded result of client.Request and client.Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) |
| [`RFC2131-3.5-1`](#rfc2131-3.5-1) If the client includes a list of parameters in a DHCPDISCOVER message, it MUST include that list in any subsequent DHCPREQUEST messages. (§3.5) | no test | no test carries this requirement id; annotated {not-applicable}: the parameter request list is written by the vendored library, the same list in both messages -- NewDiscovery (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:198-209) and NewRequestFromOffer (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:256-270) each add the identical WithRequestedOptions set, and ze passes only hostname and client-id modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:299-308), so it authors no request list |
| [`RFC2131-3.1-8`](#rfc2131-3.1-8) To help ensure that any BOOTP relay agents forward the DHCPREQUEST message to the same set of DHCP servers that received the original DHCPDISCOVER message, the DHCPREQUEST message MUST use the same value in the DHCP message header's 'secs' field and be sent to the same IP broadcast address as the original DHCPDISCOVER message. (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: the secs field and the broadcast flag of the SELECTING REQUEST are the vendored library's -- it leaves secs zero in both the DISCOVER and the REQUEST (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:164-180 sets no secs) and copies the flags word across with WithReply (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:258, vendor/github.com/insomniacslk/dhcp/dhcpv4/modifiers.go:56-68), while ze's DORA call and its modifiers touch neither (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 299-308) |
| [`RFC2131-4.3.1-1`](#rfc2131-4.3.1-1) The configuration parameters MUST be selected by applying the following rules in the order given below. (§4.3.1) | {gap}, no test | ze applies no parameter-selection procedure -- buildReply emits one fixed option set in one fixed order (internal/plugins/dhcpserver/handler.go:245-285) and the parameter request list is never parsed (optParamReqList is defined at handler.go:52 and read nowhere in production), so the ordered rules of Section 4.3.1 govern no ze code path |
| [`RFC2131-4.3.1-3`](#rfc2131-4.3.1-3) -- IF the server recognizes the parameter as a parameter defined in the Host Requirements Document, the server MUST include the default value for that parameter as given in the Host Requirements Document in an appropriate option in the 'option' field, ELSE (§4.3.1) | {gap}, no test | ze returns only the values its own subnet configuration holds (buildReply internal/plugins/dhcpserver/handler.go:245-285); it reads no parameter request list (optParamReqList handler.go:52 is parsed nowhere in production) and carries no Host Requirements defaults, so a recognized parameter the client requests draws no default value |
| [`RFC2131-4.3.1-5`](#rfc2131-4.3.1-5) The server MUST supply as many of the requested parameters as possible and MUST omit any parameters it cannot provide. (§4.3.1) | {gap}, no test | the parameter request list is never parsed (optParamReqList internal/plugins/dhcpserver/handler.go:52 is read nowhere in production), so ze supplies its fixed configured option set (buildReply handler.go:245-285) rather than as many of the client's requested parameters as it can |
| [`RFC2131-4.3.1-7`](#rfc2131-4.3.1-7) o Any parameters specific to this client's class (as identified by the contents of the 'vendor class identifier' option in the DHCPDISCOVER or DHCPREQUEST message), e.g., as configured by the network administrator; the parameters MUST be identified by an exact match between the client's vendor class identifiers and the client's classes identified in the server, (§4.3.1) | {gap}, no test | ze identifies the vendor class by a ten-octet prefix comparison against the string PXEClient: (isPXEClient internal/plugins/dhcpserver/handler.go:493-496) rather than by an exact match against a configured class identifier, and that prefix match is what selects the class-specific options appended at handler.go:292-330 |
| [`RFC2131-4.4.1-1`](#rfc2131-4.4.1-1) The client MUST include its hardware address in the 'chaddr' field, if necessary for delivery of DHCP reply messages. (§4.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: chaddr is filled by the vendored library from the bound interface -- nclient4.New resolves the interface MAC (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:199-209) and NewDiscovery/NewRequestFromOffer set it with WithHwAddr (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:198-209, 256-270); ze only names the interface to bind (internal/plugins/iface/dhcp/dhcp_v4_linux.go:37) |
| [`RFC2131-4.4.2-1`](#rfc2131-4.4.2-1) The client MUST insert its known network address as a 'requested IP address' option in the DHCPREQUEST message. (§4.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| [`RFC2131-4.4.2-2`](#rfc2131-4.4.2-2) The client MUST NOT include a 'server identifier' in the DHCPREQUEST message. (§4.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| [`RFC2131-4.4.3-1`](#rfc2131-4.4.3-1) DHCPINFORM messages MUST be directed to the 'DHCP server' UDP port (§4.4.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze's client sends no DHCPINFORM -- nclient4 exposes Inform (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:483-498) and ze calls it nowhere; runV4 and renewV4 use only Request and Renew (internal/plugins/iface/dhcp/dhcp_v4_linux.go:48, 143) |
| [`RFC2131-4.3.2-5`](#rfc2131-4.3.2-5) This message MUST be broadcast to the 0xffffffff IP broadcast address. (§4.3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's T2 attempt reuses the same vendored Renew call as its T1 attempt (internal/plugins/iface/dhcp/dhcp_v4_linux.go:103 and 143), so both the message and its destination are the library's: NewRenewFromAck builds it (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290) and it goes to the default serverAddr of 255.255.255.255:67 (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/client.go:50-53, 183, used at nclient4/lease.go:60); ze chooses neither |
| [`RFC2131-4.4.5-5`](#rfc2131-4.4.5-5) If the client is given a new network address, it MUST NOT continue using the previous network address (§4.4.5) | {gap}, no test | a renewal that returns a different yiaddr leaves the previous address in use -- renewV4 hands the new ACK to handleV4Lease (internal/plugins/iface/dhcp/dhcp_v4_linux.go:154), which installs the new address with ReplaceAddressWithLifetime (internal/plugins/iface/dhcp/dhcp_v4_linux.go:180), a per-address replace that touches no other address (internal/plugins/iface/netlink/manage_linux.go:268-286), and runV4 then overwrites its ack reference (internal/plugins/iface/dhcp/dhcp_v4_linux.go:87-88) so the previous address is never passed to removeV4Addr; both addresses stay configured on the interface |
| [`RFC2131-4.4.5-6`](#rfc2131-4.4.5-6) The client MUST NOT include a 'server identifier' in the DHCPREQUEST message. (§4.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| [`RFC2131-4.4.5-7`](#rfc2131-4.4.5-7) The client MUST NOT include a 'server identifier' in the DHCPREQUEST message. (§4.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: the RENEWING/REBINDING REQUEST is constructed entirely inside the vendored library -- renewV4 calls client.Renew with no modifiers (internal/plugins/iface/dhcp/dhcp_v4_linux.go:143) and NewRenewFromAck sets message type, ciaddr, the broadcast flag and a requested-options list only (vendor/github.com/insomniacslk/dhcp/dhcpv4/dhcpv4.go:275-290), adding neither a server identifier nor a requested IP address; ze authors none of those bytes |
| [`RFC2131-3.2-3`](#rfc2131-3.2-3) As the client has not received its network address, it MUST NOT fill in the 'ciaddr' field. (§3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client never enters INIT-REBOOT -- every acquisition is a full DORA (client.Request internal/plugins/iface/dhcp/dhcp_v4_linux.go:48) and every renewal is a RENEWING request built from the stored ACK (client.Renew internal/plugins/iface/dhcp/dhcp_v4_linux.go:143); no ze code path caches an address to reboot with, so ze produces no INIT-REBOOT REQUEST |
| [`RFC2131-4.4-1`](#rfc2131-4.4-1) Vendor class identifier MAY MAY MUST NOT (§4.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| [`RFC2131-4.4-2`](#rfc2131-4.4-2) Requested IP address MAY MUST (in MUST (DISCOVER) SELECTING or (DHCPDECLINE), (§4.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPDECLINE, so no ze path builds the message whose fields this governs -- runV4 installs the ACKed address directly (handleV4Lease internal/plugins/iface/dhcp/dhcp_v4_linux.go:158-184) and neither ze nor the vendored nclient4 client holds a DECLINE producer; that absence is itself recorded as the gap on RFC2131-3.1-7 |
| [`RFC2131-4.4-3`](#rfc2131-4.4-3) Server identifier MUST NOT MUST (after MUST SELECTING) (§4.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze's DHCPv4 client sends no DHCPRELEASE, so no ze path builds the message whose fields this governs -- runV4 ends a lease by removing the address locally (internal/plugins/iface/dhcp/dhcp_v4_linux.go:120, 211-235), and nclient4's Release (vendor/github.com/insomniacslk/dhcp/dhcpv4/nclient4/lease.go:26) is called nowhere in ze |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC2131-4.3-1`](#rfc2131-4.3-1)

Server identifier MUST MUST MUST (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: option 54 in OFFER, ACK and NAK must equal the server address, so omitting or mis-setting it goes red

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestServerIdentifierInEveryReply`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L169) | unit/verify | unproven |

### [`RFC2131-4.3-2`](#rfc2131-4.3-2)

IP address lease time MUST (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: the OFFER's option 51 must be four octets equal to the configured lease seconds

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLeaseTimeInOfferAndAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L194) | unit/verify | unproven |

### [`RFC2131-4.3-3`](#rfc2131-4.3-3)

IP address lease time MUST MUST (DHCPREQUEST) (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive: the ACK answering a DHCPREQUEST must carry option 51 equal to the configured lease seconds

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestLeaseTimeInOfferAndAck`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L198) | unit/verify | unproven |

### [`RFC2131-4.3-4`](#rfc2131-4.3-4)

IP address lease time MUST MUST (DHCPREQUEST) MUST NOT (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: the NAK's option walk holds no 51; negative: a REQUEST carrying option 51 that is NAKed still yields a NAK without 51

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L333) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L245) | unit/verify | unproven |

### [`RFC2131-4.3-5`](#rfc2131-4.3-5)

Requested IP address MUST NOT MUST NOT MUST NOT (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: no OFFER/ACK/NAK carries option 50; negative: requests carrying option 50 still draw OFFER/ACK/NAK without it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L312) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L224) | unit/verify | unproven |

### [`RFC2131-4.3-6`](#rfc2131-4.3-6)

Client identifier MUST NOT MUST NOT (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: neither OFFER nor ACK carries option 61; negative: requests carrying option 61 still draw OFFER/ACK without it. The NAK column (MAY) is correctly left unasserted

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L327) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L239) | unit/verify | unproven |

### [`RFC2131-4.3-7`](#rfc2131-4.3-7)

Parameter request list MUST NOT MUST NOT MUST NOT (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: no OFFER/ACK/NAK carries option 55; negative: requests carrying option 55 still draw replies without it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L316) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L228) | unit/verify | unproven |

### [`RFC2131-4.3-8`](#rfc2131-4.3-8)

Maximum message size MUST NOT MUST NOT MUST NOT (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: no OFFER/ACK/NAK carries option 57; negative: requests carrying option 57 still draw replies without it

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L320) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L232) | unit/verify | unproven |

### [`RFC2131-4.3-9`](#rfc2131-4.3-9)

All others MAY MAY MUST NOT (§4.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive and negative both require every NAK option code to be 53 or 54, so any option Table 3 marks 'All others' goes red. The assertion is stricter than Table 3: it would also go red on the Message option (56, SHOULD), Client identifier (61, MAY) and Vendor class identifier (60, MAY), which Table 3 allows in a DHCPNAK

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyDoesNotEchoClientOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L337) | unit/verify | unproven |
| positive | [`TestReplyOmitsClientOnlyOptions`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L249) | unit/verify | unproven |

### [`RFC2131-4.3.5-1`](#rfc2131-4.3.5-1)

The server MUST NOT send a lease expiration time to the client (§4.3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.5-1, so no unit is bound to it.

### [`RFC2131-4.3.2-1`](#rfc2131-4.3.2-1)

If 'giaddr' is 0x0 in the DHCPREQUEST message, the client is on the same subnet as the server. The server MUST broadcast the DHCPNAK message to the 0xffffffff broadcast address (§4.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.2-1, so no unit is bound to it.

### [`RFC2131-4.3.2-2`](#rfc2131-4.3.2-2)

If 'giaddr' is set in the DHCPREQUEST message, the client is on a different subnet. The server MUST set the broadcast bit in the DHCPNAK, so that the relay agent will broadcast the DHCPNAK to the client (§4.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.2-2, so no unit is bound to it.

### [`RFC2131-3.2-4`](#rfc2131-3.2-4)

Otherwise, the server MUST send the DHCPNAK message to the IP address of the BOOTP relay agent, as recorded in 'giaddr'. (§3.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive relays a SELECTING REQUEST with giaddr 192.168.1.254 and asserts responseAddr gives 192.168.1.254:67 for the resulting NAK; negative asserts the same NAK with giaddr zero goes to 255.255.255.255:68, not to a relay

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestNakDeliveredToRelayAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/nak_relay_rfc2131_test.go#L20) | unit/verify | revert, verified |
| positive | [`TestNakDeliveredToRelayAgent`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/nak_relay_rfc2131_test.go#L19) | unit/verify | revert, verified |

### [`RFC2131-4.3.2-3`](#rfc2131-4.3.2-3)

If the DHCP server has no record of this client, then it MUST remain silent (§4.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.2-3, so no unit is bound to it.

### [`RFC2131-4.3.3-1`](#rfc2131-4.3.3-1)

The server MUST mark the network address as not available (§4.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.3-1, so no unit is bound to it.

### [`RFC2131-4.1-1`](#rfc2131-4.1-1)

A server with multiple network addresses MUST be prepared to to accept any of its network addresses as identifying that server in a DHCP message. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-1, so no unit is bound to it.

### [`RFC2131-4.1-2`](#rfc2131-4.1-2)

To accommodate potentially incomplete network connectivity, a server MUST choose an address as a 'server identifier' that, to the best of the server's knowledge, is reachable from the client. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-2, so no unit is bound to it.

### [`RFC2131-4.2-1`](#rfc2131-4.2-1)

If the client supplies a 'client identifier', the client MUST use the same 'client identifier' in all subsequent messages, and the server MUST use that identifier to identify the client. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.2-1, so no unit is bound to it.

### [`RFC2131-4.2-2`](#rfc2131-4.2-2)

If the client does not provide a 'client identifier' option, the server MUST use the contents of the 'chaddr' field to identify the client. (§4.2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive is sound: two DISCOVERs from one chaddr with different xids are offered the same address. The negative sends hlen=0 and asserts no reply; the RFC says nothing about a zero-length chaddr, so that tag asserts what the code does, not a violation of this requirement. The untagged assertion at the end of TestUnidentifiableClientRejected (two distinct chaddrs sharing one xid get different addresses) is the real negative

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUnidentifiableClientRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L390) | unit/verify | unproven |
| positive | [`TestChaddrIdentifiesClientWithoutClientIdentifier`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L366) | unit/verify | unproven |

### [`RFC2131-2-1`](#rfc2131-2-1)

The 'client identifier' chosen by a DHCP client MUST be unique to that client within the subnet to which the client is attached. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-2-1, so no unit is bound to it.

### [`RFC2131-2-2`](#rfc2131-2-2)

If the client uses a 'client identifier' in one message, it MUST use that same identifier in all subsequent messages, to ensure that all servers correctly identify the client. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-2-2, so no unit is bound to it.

### [`RFC2131-3.1-1`](#rfc2131-3.1-1)

The client broadcasts a DHCPREQUEST message that MUST include the 'server identifier' option to indicate which server it has selected (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-1, so no unit is bound to it.

### [`RFC2131-3.1-2`](#rfc2131-3.1-2)

The 'requested IP address' option MUST be set to the value of 'yiaddr' in the DHCPOFFER message from the server. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-2, so no unit is bound to it.

### [`RFC2131-3.1-3`](#rfc2131-3.1-3)

'requested IP address' option MUST be filled in with client's notion of its previously assigned address. (§4.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-3, so no unit is bound to it.

### [`RFC2131-3.1-4`](#rfc2131-3.1-4)

DHCPREQUEST generated during INIT-REBOOT state: 'server identifier' MUST NOT be filled in (§4.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-4, so no unit is bound to it.

### [`RFC2131-3.1-5`](#rfc2131-3.1-5)

'server identifier' MUST NOT be filled in, 'requested IP address' option MUST NOT be filled in (§4.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-5, so no unit is bound to it.

### [`RFC2131-3.2-1`](#rfc2131-3.2-1)

As the client has not received its network address, it MUST NOT fill in the 'ciaddr' field. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.2-1, so no unit is bound to it.

### [`RFC2131-3.1-6`](#rfc2131-3.1-6)

If the client used a 'client identifier' when it obtained the lease, it MUST use the same 'client identifier' in the DHCPRELEASE message. (§3.1, §3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-6, so no unit is bound to it.

### [`RFC2131-3.1-7`](#rfc2131-3.1-7)

If the client detects that the address is already in use (e.g., through the use of ARP), the client MUST send a DHCPDECLINE message to the server and restarts the configuration process. (§3.1, §3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-7, so no unit is bound to it.

### [`RFC2131-4.4.5-1`](#rfc2131-4.4.5-1)

If the lease expires before the client receives a DHCPACK, the client moves to INIT state, MUST immediately stop any other network processing and requests network initialization parameters as if the client were uninitialized. (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.5-1, so no unit is bound to it.

### [`RFC2131-3-1`](#rfc2131-3-1)

The first four octets of the 'options' field of the DHCP message contain the (decimal) values 99, 130, 83 and 99, respectively (§3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive reads octets 236..239 of OFFER, ACK and NAK and pins 0x63825363; negative replaces the cookie with 0xDEADBEEF and asserts h.handle returns nil

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMagicCookieRequiredAndEmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L431) | unit/verify | unproven |
| positive | [`TestMagicCookieRequiredAndEmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L423) | unit/verify | unproven |

### [`RFC2131-4.1-3`](#rfc2131-4.1-3)

The last option must always be the 'end' option. (§4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive walks each OFFER/ACK/NAK option by its length octets and requires reaching End (255); negative fills a request's options with 0x01 so it carries no End, and requires the reply to be End-terminated

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestReplyOptionsTerminatedByEnd`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L471) | unit/verify | unproven |
| positive | [`TestReplyOptionsTerminatedByEnd`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L453) | unit/verify | unproven |

### [`RFC2131-4.1-4`](#rfc2131-4.1-4)

If the options in a DHCP message extend into the 'sname' and 'file' fields, the 'option overload' option MUST appear in the 'options' field, with value 1, 2 or 3, as specified in RFC 1533. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-4, so no unit is bound to it.

### [`RFC2131-4.1-5`](#rfc2131-4.1-5)

The options in the 'sname' and 'file' fields (if in use as indicated by the 'options overload' option) MUST begin with the first octet of the field, MUST be terminated by an 'end' option, and MUST be followed by 'pad' options to fill the remainder of the field. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-5, so no unit is bound to it.

### [`RFC2131-4.1-6`](#rfc2131-4.1-6)

Any individual option in the 'options', 'sname' and 'file' fields MUST be entirely contained in that field. (§4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: an emitted option whose declared length runs past its field. TestOptionsContainedWithinTheirField walks the OFFER and ACK with replyOptionCodes (fails on overrun), but not the DHCPNAK, which a separate producer builds (buildNak, handler.go:349), so an overrunning NAK option stays green in this unit. The 'sname' and 'file' clauses have no producer (ze emits no option overload, option 52, and never reads options from those fields) and the note cannot name an assertion for them. The negative (parseOptionAddr refuses a truncated received option) is receive-side robustness, a neighbouring rule, not the sender obligation

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOptionsContainedWithinTheirField`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L515) | unit/verify | unproven |
| positive | [`TestOptionsContainedWithinTheirField`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L492) | unit/verify | unproven |

### [`RFC2131-4.1-7`](#rfc2131-4.1-7)

The options in the 'options' field MUST be interpreted first, so that any 'option overload' options may be interpreted. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-7, so no unit is bound to it.

### [`RFC2131-4.1-8`](#rfc2131-4.1-8)

The client MUST adopt a retransmission strategy that incorporates a randomized exponential backoff algorithm to determine the delay between retransmissions. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-8, so no unit is bound to it.

### [`RFC2131-4.1-9`](#rfc2131-4.1-9)

A DHCP client MUST choose 'xid's in such a way as to minimize the chance of using an 'xid' identical to one used by another client. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-9, so no unit is bound to it.

### [`RFC2131-4.1-10`](#rfc2131-4.1-10)

DHCP clients MUST use the IP address provided in the 'server identifier' option for any unicast requests to the DHCP server. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.1-10, so no unit is bound to it.

### [`RFC2131-3.4-1`](#rfc2131-3.4-1)

The server SHOULD check the network address in a DHCPINFORM message for consistency, but MUST NOT check for an existing lease. (§3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.4-1, so no unit is bound to it.

### [`RFC2131-2-3`](#rfc2131-2-3)

The remaining bits of the flags field are reserved for future use. They MUST be set to zero by clients and ignored by servers and relay agents. (§2)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the quote holds two clauses. The server clause is enforced: TestFlagsReservedBitsIgnored sends flags 0x7fff and asserts responseAddr still unicasts to yiaddr:68 exactly as for flags 0. The client clause (bits 1-15 set to zero by clients) has no tagged test although ze runs a DHCPv4 client (internal/plugins/iface/dhcp, messages built by the vendored nclient4); question 4 fails

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestFlagsReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L560) | unit/verify | unproven |
| positive | [`TestFlagsReservedBitsIgnored`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L549) | unit/verify | unproven |

### [`RFC2131-2-4`](#rfc2131-2-4)

A DHCP client must be prepared to receive DHCP messages with an 'options' field of at least length 312 octets. (§2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-2-4, so no unit is bound to it.

### [`RFC2131-3.5-1`](#rfc2131-3.5-1)

If the client includes a list of parameters in a DHCPDISCOVER message, it MUST include that list in any subsequent DHCPREQUEST messages. (§3.5)

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

T2, which, in turn, MUST be earlier than the time at which the client's lease will expire. (§4.4.5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. positive: for leases 60, 3600, 86400 and 604800 the OFFER's option 59 (T2) must be strictly below the lease seconds; negative: parseConfig must reject lease-time 0

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestShortLeaseTimeRejected`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L635) | unit/verify | unproven |
| positive | [`TestRenewalTimersOrderedWithinLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L607) | unit/verify | unproven |

### [`RFC2131-3.1-8`](#rfc2131-3.1-8)

To help ensure that any BOOTP relay agents forward the DHCPREQUEST message to the same set of DHCP servers that received the original DHCPDISCOVER message, the DHCPREQUEST message MUST use the same value in the DHCP message header's 'secs' field and be sent to the same IP broadcast address as the original DHCPDISCOVER message. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.1-8, so no unit is bound to it.

### [`RFC2131-4.3.1-1`](#rfc2131-4.3.1-1)

The configuration parameters MUST be selected by applying the following rules in the order given below. (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.1-1, so no unit is bound to it.

### [`RFC2131-4.3.1-2`](#rfc2131-4.3.1-2)

-- IF the server has been explicitly configured with a default value for the parameter, the server MUST include that value in an appropriate option in the 'option' field, ELSE (§4.3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Forbidden: a requested, explicitly configured parameter omitted or carrying a value other than the configured one. TestUnconfiguredParametersOmitted pins the router option byte for byte and the domain-name option by string, but the DNS option only by length (4 x configured servers), so a wrong DNS address of the right count passes, and the subnet mask the same DISCOVER requests is never asserted. The quote covers every configured parameter, so two of the four requested are unproven

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L697) | unit/verify | unproven |

### [`RFC2131-4.3.1-3`](#rfc2131-4.3.1-3)

-- IF the server recognizes the parameter as a parameter defined in the Host Requirements Document, the server MUST include the default value for that parameter as given in the Host Requirements Document in an appropriate option in the 'option' field, ELSE (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.1-3, so no unit is bound to it.

### [`RFC2131-4.3.1-4`](#rfc2131-4.3.1-4)

The server MUST NOT return a value for that parameter, (§4.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: returning a value for a parameter the server has no value for (neither configured nor a Host Requirements default). TestUnconfiguredParametersOmitted builds a handler with no router, DNS or domain name configured; the negative tag sends a DISCOVER whose parameter request list explicitly names optRouter, optDNS and optDomainName and asserts askedCodes[code] is false for each, so any emitted option for them (including an empty or zero-valued one, since replyOptionCodes records presence by code) goes red. The positive tag asserts the same absence for an unrequested DISCOVER. None of the three has a Host Requirements default, so the ELSE branch applies.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L682) | unit/verify | unproven |
| positive | [`TestUnconfiguredParametersOmitted`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L668) | unit/verify | unproven |

### [`RFC2131-4.3.1-5`](#rfc2131-4.3.1-5)

The server MUST supply as many of the requested parameters as possible and MUST omit any parameters it cannot provide. (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.1-5, so no unit is bound to it.

### [`RFC2131-4.3.1-6`](#rfc2131-4.3.1-6)

The server MUST include each requested parameter only once unless explicitly allowed in the DHCP Options and BOOTP Vendor Extensions document. (§4.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: a requested parameter included more than once in a reply (no option ze emits is one the options document allows to repeat). TestEachParameterEmittedOnce counts every option code in the OFFER and ACK with replyOptionCounts and fails on n != 1 (positive); the negative tag sends a DISCOVER whose parameter request list repeats router, DNS and subnet mask, carries a second PRL and an echoed router option, to both a plain and a PXE server, and fails on any option count other than 1, so a duplicated parameter goes red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEachParameterEmittedOnce`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L744) | unit/verify | unproven |
| positive | [`TestEachParameterEmittedOnce`](https://github.com/ze-software/ze/blob/main/internal/plugins/dhcpserver/rfc2131_test.go#L723) | unit/verify | unproven |

### [`RFC2131-4.3.1-7`](#rfc2131-4.3.1-7)

o Any parameters specific to this client's class (as identified by the contents of the 'vendor class identifier' option in the DHCPDISCOVER or DHCPREQUEST message), e.g., as configured by the network administrator; the parameters MUST be identified by an exact match between the client's vendor class identifiers and the client's classes identified in the server, (§4.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.1-7, so no unit is bound to it.

### [`RFC2131-4.4.1-1`](#rfc2131-4.4.1-1)

The client MUST include its hardware address in the 'chaddr' field, if necessary for delivery of DHCP reply messages. (§4.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.1-1, so no unit is bound to it.

### [`RFC2131-4.4.2-1`](#rfc2131-4.4.2-1)

The client MUST insert its known network address as a 'requested IP address' option in the DHCPREQUEST message. (§4.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.2-1, so no unit is bound to it.

### [`RFC2131-4.4.2-2`](#rfc2131-4.4.2-2)

The client MUST NOT include a 'server identifier' in the DHCPREQUEST message. (§4.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.2-2, so no unit is bound to it.

### [`RFC2131-4.4.3-1`](#rfc2131-4.4.3-1)

DHCPINFORM messages MUST be directed to the 'DHCP server' UDP port (§4.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.3-1, so no unit is bound to it.

### [`RFC2131-4.3.2-5`](#rfc2131-4.3.2-5)

This message MUST be broadcast to the 0xffffffff IP broadcast address. (§4.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.3.2-5, so no unit is bound to it.

### [`RFC2131-4.4.5-5`](#rfc2131-4.4.5-5)

If the client is given a new network address, it MUST NOT continue using the previous network address (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.5-5, so no unit is bound to it.

### [`RFC2131-4.4.5-6`](#rfc2131-4.4.5-6)

The client MUST NOT include a 'server identifier' in the DHCPREQUEST message. (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.5-6, so no unit is bound to it.

### [`RFC2131-4.4.5-7`](#rfc2131-4.4.5-7)

The client MUST NOT include a 'server identifier' in the DHCPREQUEST message. (§4.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4.5-7, so no unit is bound to it.

### [`RFC2131-3.2-3`](#rfc2131-3.2-3)

As the client has not received its network address, it MUST NOT fill in the 'ciaddr' field. (§3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-3.2-3, so no unit is bound to it.

### [`RFC2131-4.4-1`](#rfc2131-4.4-1)

Vendor class identifier MAY MAY MUST NOT (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4-1, so no unit is bound to it.

### [`RFC2131-4.4-2`](#rfc2131-4.4-2)

Requested IP address MAY MUST (in MUST (DISCOVER) SELECTING or (DHCPDECLINE), (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4-2, so no unit is bound to it.

### [`RFC2131-4.4-3`](#rfc2131-4.4-3)

Server identifier MUST NOT MUST (after MUST SELECTING) (§4.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC2131-4.4-3, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc2131.txt |
| Source fingerprint | 034052024822fc6a |
| Record | rfc/extraction/rfc2131.json |
| Mapped sentences | 49 |
| Declined as scope | 16 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `1.3` | not stated | 0 | walked | not stated |
| `1.4` | not stated | 3 | walked | not stated |
| `1.5` | not stated | 0 | walked | not stated |
| `1.6` | not stated | 0 | walked | not stated |
| `2` | not stated | 4 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 5 | walked | not stated |
| `3.2` | not stated | 5 | walked | not stated |
| `3.3` | not stated | 0 | walked | not stated |
| `3.4` | not stated | 1 | walked | not stated |
| `3.5` | not stated | 1 | walked | not stated |
| `3.6` | not stated | 0 | walked | not stated |
| `3.7` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 11 | walked | not stated |
| `4.2` | not stated | 2 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.3.1` | not stated | 8 | walked | not stated |
| `4.3.2` | not stated | 10 | walked | not stated |
| `4.3.3` | not stated | 1 | walked | not stated |
| `4.3.4` | not stated | 0 | walked | not stated |
| `4.3.5` | not stated | 1 | walked | not stated |
| `4.3.6` | not stated | 1 | walked | not stated |
| `4.4` | not stated | 0 | walked | not stated |
| `4.4.1` | not stated | 4 | walked | not stated |
| `4.4.2` | not stated | 2 | walked | not stated |
| `4.4.3` | not stated | 1 | walked | not stated |
| `4.4.4` | not stated | 0 | walked | not stated |
| `4.4.5` | not stated | 5 | walked | not stated |
| `4.4.6` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `A` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the bullet heading of the RFC 2119 keyword glossary, quoting the word itself rather than stating an obligation | o "MUST" |
| `1.4:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | RFC 2119 boilerplate defining what the word MUST means in this document | This word or the adjective "REQUIRED" means that the item is an absolute requirement of this specification. |
| `1.4:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | the bullet heading of the RFC 2119 keyword glossary, quoting the words MUST NOT rather than stating an obligation | o "MUST NOT" |
| `2:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the figure legend for the 'flags' field restates the same obligation as the section 2 sentence 'They MUST be set to zero by clients and ignored by servers and relay agents' | MBZ: MUST BE ZERO (reserved for future use) |
| `3.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | section 3.2 restates for the INIT-REBOOT DHCPREQUEST the section 2 obligation that a client which uses a 'client identifier' in one message uses that same identifier in all subsequent messages | If the client used a 'client identifier' to obtain its address, the client MUST use the same 'client identifier' in the DHCPREQUEST message. |
| `3.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | section 3.2 and section 4.3.2 carry the same sentence word for word; site 4.3.2:6 maps it | The server MUST broadcast the DHCPNAK message to the 0xffffffff broadcast address because the client may not have a correct network address or subnet mask, and the client may not be answering ARP requests. |
| `3.2:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | section 3.2 restates the section 3.1 obligation to send a DHCPDECLINE when the client detects the assigned address is already in use; site 3.1:4 maps it | If the client detects that the IP address in the DHCPACK message is already in use, the client MUST send a DHCPDECLINE message to the server and restarts the configuration process by requesting a new network address. |
| `4.1:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the same interpretation-order sentence the previous site states; site 4.1:8 maps it | The 'file' field MUST be interpreted next (if the 'option overload' option indicates that the 'file' field contains DHCP options), followed by the 'sname' field. |
| `4.3.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | 'The server MUST return to the client:' is the lead-in to the ordered selection rules the previous sentence already binds; site 4.3.1:2 maps it | The server MUST return to the client: |
| `4.3.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | section 4.3.2 restates the section 2 'same client identifier in all subsequent messages' obligation; site 2:2 maps it | If the client uses a 'client identifier' in a DHCPREQUEST message, it MUST use that same 'client identifier' in all subsequent messages. |
| `4.3.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | section 4.3.2 restates the section 3.5 obligation to carry the requested-parameter list into subsequent messages; site 3.5:1 maps it | If the client included a list of requested parameters in a DHCPDISCOVER message, it MUST include that list in all subsequent messages. |
| `4.3.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the SELECTING row of the section 4.3.2 field list restates Table 4, whose SELECTING obligations RFC2131-3.1-1 and RFC2131-3.1-2 already carry, and whose 'ciaddr MUST be zero' clause RFC2131-3.2-1 carries | Client inserts the address of the selected server in 'server identifier', 'ciaddr' MUST be zero, 'requested IP address' MUST be filled in with the yiaddr value from the chosen DHCPOFFER. |
| `4.3.2:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | the REBINDING row repeats the RENEWING row word for word; site 4.3.2:8 maps it | 'server identifier' MUST NOT be filled in, 'requested IP address' option MUST NOT be filled in, 'ciaddr' MUST be filled in with client's IP address. |
| `4.3.6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Table 4 itself, whose per-state obligations RFC2131-3.1-1 through RFC2131-3.1-5 already carry | --------------------------------------------------------------------- \| \|INIT-REBOOT \|SELECTING \|RENEWING \|REBINDING \| --------------------------------------------------------------------- \|broad/unicast \|broadcast \|broadcast \|unicast \|broadcast \| \|server-ip \|MUST NOT \|MUST \|MUST NOT \|MUST NOT \| \|requested-ip \|MUST \|MUST \|MUST NOT \|MUST NOT \| \|ciaddr \|zero \|zero \|IP address \|IP address\| --------------------------------------------------------------------- |
| `4.4.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | section 4.4.1 restates the section 3.5 obligation to carry the requested-parameter list into subsequent messages; site 3.5:1 maps it | If the client included a list of requested parameters in a DHCPDISCOVER message, it MUST include that list in all subsequent messages. |
| `4.4.1:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | section 4.4.1 restates the section 3.1 obligation to send a DHCPDECLINE when the address appears to be in use; site 3.1:4 maps it | If the network address appears to be in use, the client MUST send a DHCPDECLINE message to the server. |

## Superseded

No document obsoletes RFC 2131, so its obligations are stated where they were written.
