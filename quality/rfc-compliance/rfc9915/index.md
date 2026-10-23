# RFC 9915 - Dynamic Host Configuration Protocol for IPv6 (DHCPv6)

Partial. Every requirement this repository extracted from RFC 9915, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 3.2% | 4 of 126 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 62.7% | 79 of 126 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 126 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 126 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 91 of 91 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 126 | of 126 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 21 | of 126 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 3.2% | 4 of 126 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 2.4% | 3 of 126 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 13.5% | 17 of 126 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 15.1% | 19 of 126 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 126 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 126 |
| Gated MUST-level | 126 |
| Not applicable, so out of scope | 4 |
| Declared gaps | 19 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 91 |
| Tagged units | 91 |
| Recorded audit verdicts | 0 |
| Discrimination records | 91 |
| Summary | `rfc/short/rfc9915.md` |
| Requirement shard | `rfc/requirements/rfc9915.md` |
| RFC text | `rfc/full/rfc9915.txt` |

## Enrolment

Enrolled: DHCPv6 client of the iface-dhcp plugin; RFC 8415 to RFC 9915 put in scope by the owner as an exception to the RFC scope freeze, 2026-10-18

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Client role only. The `iface-dhcp` plugin (`internal/plugins/iface/dhcp/`) runs a long-lived stateful DHCPv6 client per interface unit: Solicit with Rapid Commit and Advertise selection by preference, Request, Renew at T1 to the leasing server and Rebind at T2 without a Server ID, a Confirm after a device down then up for a lease of addresses alone (NotOnLink drops the addresses and solicits again), the section 15 retransmission engine with a per-interface rate limit, SOL_MAX_RT processing, Reply and Advertise validation, IA_NA addresses installed as /128 with the lease's lifetimes, IA_PD prefix delegation published as an event, expiry teardown and Release on Stop. Tests bound per requirement in [`rfc/short/rfc9915.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9915.md).

**What the ledger says remains**

- **Nineteen MUST gaps:** Information-request (stateless DHCPv6), Reconfigure and the Authentication option are in scope by owner decision 2026-10-19 and not implemented yet, and the requirements only those features meet are annotated `{gap}` ([`plan/post-release/features/spec-dhcpv6-client-0-umbrella.md`](https://github.com/ze-software/ze/blob/main/plan/post-release/features/spec-dhcpv6-client-0-umbrella.md)). On the VPP backend, which reports no interface addresses, the client reads no duplicate address detection outcome and declines nothing, so [`RFC9915-18.2.10.1-1`](#rfc9915-18.2.10.1-1) is unmet there ([`plan/journal/zero-value-as-valid-answer.md`](https://github.com/ze-software/ze/blob/main/plan/journal/zero-value-as-valid-answer.md)).
- **Not offered:** the User Class, Vendor Class, Vendor-specific and NTP server options, any use of a delegated prefix (ze publishes it and neither subnets it onto other interfaces nor advertises it in Router Advertisements), and a configurable transmission rate limit; the client always uses a DUID-LL unless a duid is configured. Server and relay roles are not implemented.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 122 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **126** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC9915-18.2.3-3`](#rfc9915-18.2.3-3), [`RFC9915-18.2.10.1-1`](#rfc9915-18.2.10.1-1), [`RFC9915-18.2.12-1`](#rfc9915-18.2.12-1), [`RFC9915-18.2.12-2`](#rfc9915-18.2.12-2)

**Annotated (including scoped evidence) (122):** [`RFC9915-6.3-1`](#rfc9915-6.3-1), [`RFC9915-6.3-2`](#rfc9915-6.3-2), [`RFC9915-6.3-3`](#rfc9915-6.3-3), [`RFC9915-7.2-1`](#rfc9915-7.2-1), [`RFC9915-7.2-3`](#rfc9915-7.2-3), [`RFC9915-10-1`](#rfc9915-10-1), [`RFC9915-11-1`](#rfc9915-11-1), [`RFC9915-11-2`](#rfc9915-11-2), [`RFC9915-11.2-1`](#rfc9915-11.2-1), [`RFC9915-11.2-2`](#rfc9915-11.2-2), [`RFC9915-11.2-3`](#rfc9915-11.2-3), [`RFC9915-11.2-4`](#rfc9915-11.2-4), [`RFC9915-11.2-5`](#rfc9915-11.2-5), [`RFC9915-11.3-1`](#rfc9915-11.3-1), [`RFC9915-11.4-1`](#rfc9915-11.4-1), [`RFC9915-12-1`](#rfc9915-12-1), [`RFC9915-12-2`](#rfc9915-12-2), [`RFC9915-14.1-1`](#rfc9915-14.1-1), [`RFC9915-14.1-2`](#rfc9915-14.1-2), [`RFC9915-14.2-1`](#rfc9915-14.2-1), [`RFC9915-14.2-2`](#rfc9915-14.2-2), [`RFC9915-15-1`](#rfc9915-15-1), [`RFC9915-15-2`](#rfc9915-15-2), [`RFC9915-16-1`](#rfc9915-16-1), [`RFC9915-16-2`](#rfc9915-16-2), [`RFC9915-16.1-1`](#rfc9915-16.1-1), [`RFC9915-16.2-1`](#rfc9915-16.2-1), [`RFC9915-16.3-1`](#rfc9915-16.3-1), [`RFC9915-16.4-1`](#rfc9915-16.4-1), [`RFC9915-16.5-1`](#rfc9915-16.5-1), [`RFC9915-16.6-1`](#rfc9915-16.6-1), [`RFC9915-16.7-1`](#rfc9915-16.7-1), [`RFC9915-16.8-1`](#rfc9915-16.8-1), [`RFC9915-16.9-1`](#rfc9915-16.9-1), [`RFC9915-16.10-1`](#rfc9915-16.10-1), [`RFC9915-16.10-2`](#rfc9915-16.10-2), [`RFC9915-16.10-3`](#rfc9915-16.10-3), [`RFC9915-16.11-2`](#rfc9915-16.11-2), [`RFC9915-16.12-1`](#rfc9915-16.12-1), [`RFC9915-16.13-1`](#rfc9915-16.13-1), [`RFC9915-16.14-1`](#rfc9915-16.14-1), [`RFC9915-18-1`](#rfc9915-18-1), [`RFC9915-18.2-1`](#rfc9915-18.2-1), [`RFC9915-18.2.1-1`](#rfc9915-18.2.1-1), [`RFC9915-18.2.1-2`](#rfc9915-18.2.1-2), [`RFC9915-18.2.1-3`](#rfc9915-18.2.1-3), [`RFC9915-18.2.1-4`](#rfc9915-18.2.1-4), [`RFC9915-18.2.1-5`](#rfc9915-18.2.1-5), [`RFC9915-18.2.1-6`](#rfc9915-18.2.1-6), [`RFC9915-18.2.1-7`](#rfc9915-18.2.1-7), [`RFC9915-18.2.2-1`](#rfc9915-18.2.2-1), [`RFC9915-18.2.2-2`](#rfc9915-18.2.2-2), [`RFC9915-18.2.2-3`](#rfc9915-18.2.2-3), [`RFC9915-18.2.2-4`](#rfc9915-18.2.2-4), [`RFC9915-18.2.3-1`](#rfc9915-18.2.3-1), [`RFC9915-18.2.3-2`](#rfc9915-18.2.3-2), [`RFC9915-18.2.4-1`](#rfc9915-18.2.4-1), [`RFC9915-18.2.4-2`](#rfc9915-18.2.4-2), [`RFC9915-18.2.4-4`](#rfc9915-18.2.4-4), [`RFC9915-18.2.4-5`](#rfc9915-18.2.4-5), [`RFC9915-18.2.4-6`](#rfc9915-18.2.4-6), [`RFC9915-18.2.4-7`](#rfc9915-18.2.4-7), [`RFC9915-18.2.6-1`](#rfc9915-18.2.6-1), [`RFC9915-18.2.6-2`](#rfc9915-18.2.6-2), [`RFC9915-18.2.6-3`](#rfc9915-18.2.6-3), [`RFC9915-18.2.6-4`](#rfc9915-18.2.6-4), [`RFC9915-18.2.7-1`](#rfc9915-18.2.7-1), [`RFC9915-18.2.7-2`](#rfc9915-18.2.7-2), [`RFC9915-18.2.7-3`](#rfc9915-18.2.7-3), [`RFC9915-18.2.7-4`](#rfc9915-18.2.7-4), [`RFC9915-18.2.7-5`](#rfc9915-18.2.7-5), [`RFC9915-18.2.7-6`](#rfc9915-18.2.7-6), [`RFC9915-18.2.7-7`](#rfc9915-18.2.7-7), [`RFC9915-18.2.7-8`](#rfc9915-18.2.7-8), [`RFC9915-18.2.8-1`](#rfc9915-18.2.8-1), [`RFC9915-18.2.8-2`](#rfc9915-18.2.8-2), [`RFC9915-18.2.8-3`](#rfc9915-18.2.8-3), [`RFC9915-18.2.8-4`](#rfc9915-18.2.8-4), [`RFC9915-18.2.8-5`](#rfc9915-18.2.8-5), [`RFC9915-18.2.8-6`](#rfc9915-18.2.8-6), [`RFC9915-18.2.9-1`](#rfc9915-18.2.9-1), [`RFC9915-18.2.9-2`](#rfc9915-18.2.9-2), [`RFC9915-18.2.9-3`](#rfc9915-18.2.9-3), [`RFC9915-18.2.9-4`](#rfc9915-18.2.9-4), [`RFC9915-18.2.9-5`](#rfc9915-18.2.9-5), [`RFC9915-18.2.10-1`](#rfc9915-18.2.10-1), [`RFC9915-18.2.10.1-3`](#rfc9915-18.2.10.1-3), [`RFC9915-18.2.10.5-1`](#rfc9915-18.2.10.5-1), [`RFC9915-18.2.11-1`](#rfc9915-18.2.11-1), [`RFC9915-18.2.11-2`](#rfc9915-18.2.11-2), [`RFC9915-18.2.12-3`](#rfc9915-18.2.12-3), [`RFC9915-18.2.12-4`](#rfc9915-18.2.12-4), [`RFC9915-18.3.5-1`](#rfc9915-18.3.5-1), [`RFC9915-20.2-1`](#rfc9915-20.2-1), [`RFC9915-20.3-3`](#rfc9915-20.3-3), [`RFC9915-20.3-4`](#rfc9915-20.3-4), [`RFC9915-21-1`](#rfc9915-21-1), [`RFC9915-21.4-2`](#rfc9915-21.4-2), [`RFC9915-21.6-3`](#rfc9915-21.6-3), [`RFC9915-21.6-4`](#rfc9915-21.6-4), [`RFC9915-21.7-1`](#rfc9915-21.7-1), [`RFC9915-21.7-2`](#rfc9915-21.7-2), [`RFC9915-21.9-1`](#rfc9915-21.9-1), [`RFC9915-21.15-1`](#rfc9915-21.15-1), [`RFC9915-21.15-2`](#rfc9915-21.15-2), [`RFC9915-21.16-1`](#rfc9915-21.16-1), [`RFC9915-21.17-1`](#rfc9915-21.17-1), [`RFC9915-21.17-2`](#rfc9915-21.17-2), [`RFC9915-21.17-3`](#rfc9915-21.17-3), [`RFC9915-21.22-2`](#rfc9915-21.22-2), [`RFC9915-21.23-1`](#rfc9915-21.23-1), [`RFC9915-21.23-2`](#rfc9915-21.23-2), [`RFC9915-21.23-5`](#rfc9915-21.23-5), [`RFC9915-21.23-6`](#rfc9915-21.23-6), [`RFC9915-21.23-7`](#rfc9915-21.23-7), [`RFC9915-21.24-3`](#rfc9915-21.24-3), [`RFC9915-21.24-4`](#rfc9915-21.24-4), [`RFC9915-21.25-2`](#rfc9915-21.25-2), [`RFC9915-21.25-3`](#rfc9915-21.25-3), [`RFC9915-21.25-4`](#rfc9915-21.25-4), [`RFC9915-22.3-1`](#rfc9915-22.3-1), [`RFC9915-A-1`](#rfc9915-a-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9915-6.3-1` | If the client uses a delegated prefix to configure addresses on interfaces on itself or other nodes behind it, the preferred and valid lifetimes of those addresses MUST be no longer than the remaining preferred and valid lifetimes, respectively, for the delegated prefix at any time. (§6.3) | MUST | 6.3 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the client might assign a subnet from a delegated prefix to one of its interfaces and begin sending Router Advertisements for the prefix on that link"; ze uses no delegated prefix itself: internal/plugins/iface/dhcp/dhcp_v6_linux.go::effect records the IA_PD prefix and publishes it as a dhcp-acquired event, and its subscribers, handleDHCPLeaseEvent in internal/component/iface/register.go and syncWorker.handleDHCPEvent in internal/plugins/ntp/ntp.go, configure no address and send no Router Advertisement from it |
| `RFC9915-6.3-2` | In particular, if the delegated prefix or a prefix derived from it is advertised for stateless address autoconfiguration [RFC4862], the advertised preferred and valid lifetimes MUST NOT exceed the corresponding remaining lifetimes of the delegated prefix. (§6.3) | MUST NOT | 6.3 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the client might assign a subnet from a delegated prefix to one of its interfaces and begin sending Router Advertisements for the prefix on that link"; ze uses no delegated prefix itself: internal/plugins/iface/dhcp/dhcp_v6_linux.go::effect records the IA_PD prefix and publishes it as a dhcp-acquired event, and its subscribers, handleDHCPLeaseEvent in internal/component/iface/register.go and syncWorker.handleDHCPEvent in internal/plugins/ntp/ntp.go, configure no address and send no Router Advertisement from it |
| `RFC9915-6.3-3` | A client that has delegated any of the address space received through DHCP Prefix Delegation MUST NOT issue a DHCP Release on the relevant delegated prefix while any of the address space is outstanding. (§6.3) | MUST NOT | 6.3 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the client might assign a subnet from a delegated prefix to one of its interfaces and begin sending Router Advertisements for the prefix on that link"; ze uses no delegated prefix itself: internal/plugins/iface/dhcp/dhcp_v6_linux.go::effect records the IA_PD prefix and publishes it as a dhcp-acquired event, and its subscribers, handleDHCPLeaseEvent in internal/component/iface/register.go and syncWorker.handleDHCPEvent in internal/plugins/ntp/ntp.go, configure no address and send no Router Advertisement from it |
| `RFC9915-7.2-1` | Clients MUST listen for DHCP messages on UDP port 546. (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** vendored insomniacslk/dhcp nclient6; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::dialV6Link opens nclient6.New on the device and passes no port, and the library binds UDP port 546 |
| `RFC9915-7.2-3` | Therefore, clients MUST send DHCP messages to UDP destination port 547. (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** vendored insomniacslk/dhcp nclient6; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::dialV6Link passes no destination, and the library sends every client message to All_DHCP_Relay_Agents_and_Servers port 547 |
| `RFC9915-10-1` | The message compression scheme in Section 4.1.4 of [RFC1035] MUST NOT be used. (§10) | MUST NOT | 10 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's client encodes no domain name: the options it sends (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease) carry DUIDs, codes, IAs and timers, never a DNS-encoded name |
| `RFC9915-11-1` | Clients and servers MUST treat DUIDs as opaque values and MUST only compare DUIDs for equality. (§11) | MUST | 11 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section1610ReplyValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L209). **{single-polarity}:** the conforming case, a Reply naming this client's DUID, is the positive control of the same test, and a DUID compared for equality has no other observable outcome |
| `RFC9915-11-2` | Clients and servers MUST NOT restrict DUIDs to the types defined in this document, as additional DUID types may be defined in the future. (§11) | MUST NOT | 11 | **positive:** `unit/verify` [`TestRFC9915Section163AdvertiseValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L92). **negative:** no negative test. **{single-polarity}:** a DUID type cannot be restricted by an input ze refuses, so the conforming acceptance is the only observable outcome |
| `RFC9915-11.2-1` | The hardware type MUST be a valid hardware type assigned by IANA; see [IANA-HARDWARE-TYPES]. (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| `RFC9915-11.2-2` | Clients and servers using this type of DUID MUST store the DUID-LLT in stable storage and MUST continue to use this DUID-LLT even if the network interface used to generate the DUID-LLT is removed. (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| `RFC9915-11.2-3` | Clients and servers that do not have any stable storage MUST NOT use this type of DUID. (§11.2) | MUST NOT | 11.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| `RFC9915-11.2-4` | Clients and servers that use this DUID SHOULD attempt to configure the time prior to generating the DUID, if that is possible, and MUST use some sort of time source (for example, a real-time clock) in generating the DUID, even if that time source could not be configured prior to generating the DUID. (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| `RFC9915-11.2-5` | A DHCP client that generates a DUID-LLT using this mechanism MUST provide an administrative interface that replaces the existing DUID with a newly generated DUID-LLT. (§11.2) | MUST | 11.2 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| `RFC9915-11.3-1` | The source of the identifier is left up to the vendor defining it, but each identifier part of each DUID-EN MUST be unique to the device that is using it, and MUST be assigned to the device no later than at the first usage and stored in some form of non-volatile storage. (§11.3) | MUST | 11.3 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| `RFC9915-11.4-1` | The hardware type MUST be a valid hardware type assigned by IANA; see [IANA-HARDWARE-TYPES]. (§11.4) | MUST | 11.4 | **positive:** `unit/verify` [`TestRFC9915Section1824RenewAtT1CarriesLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L362). **negative:** no negative test. **{single-polarity}:** the hardware type is a constant ze writes, so no input yields another value to assert negatively |
| `RFC9915-12-1` | The IAID uniquely identifies the IA and MUST be chosen to be unique among the IAIDs for that IA type on the client (e.g., an IA_NA with an IAID of 0 and an IA_PD with an IAID of 0 are each considered unique). (§12) | MUST | 12 | **positive:** `unit/verify` [`TestDHCPv6PDLengthRequestsDelegatedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L341). **negative:** no negative test. **{single-polarity}:** ze holds one IA_NA and one IA_PD per client, so no input yields two IAs of one type to assert negatively |
| `RFC9915-12-2` | For any given use of an IA by the client, the IAID for that IA MUST be consistent across restarts of the DHCP client. (§12) | MUST | 12 | **positive:** `unit/verify` [`TestRFC9915Section12IAIDStable`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L384). **negative:** no negative test. **{single-polarity}:** the IAID is a pure function of the MAC, so no input makes a restart change it |
| `RFC9915-14.1-1` | A DHCPv6 client MUST limit the rate of DHCP messages it transmits or retransmits. (§14.1) | MUST | 14.1 | **positive:** `unit/verify` [`TestRFC9915Section141PerInterfaceRateLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L238). **negative:** no negative test. **{single-polarity}:** the limit is the client's own schedule, so no peer input makes it exceed the limit to assert negatively |
| `RFC9915-14.1-2` | For a device that has multiple interfaces, the limit MUST be enforced on a per-interface basis. (§14.1) | MUST | 14.1 | **positive:** `unit/verify` [`TestRFC9915Section141LimitIsPerInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L361). **negative:** no negative test. **{single-polarity}:** one client runs per interface, so no input makes two interfaces share a window to assert negatively |
| `RFC9915-14.2-1` | When T1 and/or T2 values are set to 0, the client MUST choose a time to avoid message storms. (§14.2) | MUST | 14.2 | **positive:** `unit/verify` [`TestRFC9915Section142ZeroTimersChosenByClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L779). **negative:** no negative test. **{single-polarity}:** the timers are chosen by the client, so the zero T1/T2 input is itself the case asserted |
| `RFC9915-14.2-2` | In particular, it MUST NOT transmit immediately. (§14.2) | MUST NOT | 14.2 | **positive:** `unit/verify` [`TestRFC9915Section142ZeroTimersChosenByClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L780). **negative:** no negative test. **{single-polarity}:** the timers are chosen by the client, so the zero T1/T2 input is itself the case asserted |
| `RFC9915-15-1` | The client MUST update an "elapsed-time" value within an Elapsed Time option (see Section 21.9) in the retransmitted message. (§15) | MUST | 15 | **positive:** `unit/verify` [`TestRFC9915Section15ElapsedTimeUpdated`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L61). **negative:** no negative test. **{single-polarity}:** the Elapsed Time is computed by the client, so no input yields a stale value to assert negatively |
| `RFC9915-15-2` | Of course, a client MUST listen for a Reconfigure if it has negotiated for its use with the server. (§15) | MUST | 15 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Reconfigure is in scope by owner decision 2026-10-19 and not yet implemented: ze is never willing to accept Reconfigure messages: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit sends no Reconfigure Accept option, and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher discards every Reconfigure; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| `RFC9915-16-1` | Clients, relay agents, and servers MUST NOT discard messages that contain unknown options (or instances of vendor options with unknown enterprise-number values). (§16) | MUST NOT | 16 | **positive:** `unit/verify` [`TestRFC9915Section1610ReplyValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L205). **negative:** no negative test. **{single-polarity}:** the conforming outcome is the acceptance, and the unknown-message-type refusal is the separate row RFC9915-16-2 |
| `RFC9915-16-2` | A client or server MUST discard any received DHCP messages with an unknown message type. (§16) | MUST | 16 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section1610ReplyValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L206). **{single-polarity}:** a known message type taken as an answer is the ordinary path every exchange test asserts |
| `RFC9915-16.1-1` | A client MUST leave the transaction ID unchanged in retransmissions of a message. (§16.1) | MUST | 16.1 | **positive:** `unit/verify` [`TestRFC9915Section15ElapsedTimeUpdated`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L62). **negative:** no negative test. **{single-polarity}:** the transaction ID is chosen by the client, so no input makes a retransmission change it |
| `RFC9915-16.2-1` | Clients MUST discard any received Solicit messages. (§16.2) | MUST | 16.2 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L29). **{single-polarity}:** the obligation is a discard, and a client has no Solicit to accept |
| `RFC9915-16.3-1` | Clients MUST discard any received Advertise message that meets any of the following conditions: (§16.3) | MUST | 16.3 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section163AdvertiseValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L88). **{single-polarity}:** the accepted Advertise is the positive control in RFC9915-18.2.1-6 |
| `RFC9915-16.4-1` | Clients MUST discard any received Request messages. (§16.4) | MUST | 16.4 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L30). **{single-polarity}:** the obligation is a discard, and a client has no Request to accept |
| `RFC9915-16.5-1` | Clients MUST discard any received Confirm messages. (§16.5) | MUST | 16.5 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L31). **{single-polarity}:** the obligation is a discard, and a client has no Confirm to accept |
| `RFC9915-16.6-1` | Clients MUST discard any received Renew messages. (§16.6) | MUST | 16.6 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L32). **{single-polarity}:** the obligation is a discard, and a client has no Renew to accept |
| `RFC9915-16.7-1` | Clients MUST discard any received Rebind messages. (§16.7) | MUST | 16.7 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L33). **{single-polarity}:** the obligation is a discard, and a client has no Rebind to accept |
| `RFC9915-16.8-1` | Clients MUST discard any received Decline messages. (§16.8) | MUST | 16.8 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L34). **{single-polarity}:** the obligation is a discard, and a client has no Decline to accept |
| `RFC9915-16.9-1` | Clients MUST discard any received Release messages. (§16.9) | MUST | 16.9 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L35). **{single-polarity}:** the obligation is a discard, and a client has no Release to accept |
| `RFC9915-16.10-1` | Clients MUST discard any received Reply message that meets any of the following conditions: (§16.10) | MUST | 16.10 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section1610ReplyValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L207). **{single-polarity}:** the accepted Reply is the control of the same test and of every exchange test |
| `RFC9915-16.10-2` | If the client included a Client Identifier option (see Section 21.2) in the original message, the Reply message MUST include a Client Identifier option, and the contents of the Client Identifier option MUST match the DUID of the client. (§16.10) | MUST | 16.10 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section1610ReplyValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L208). **{single-polarity}:** the matching Client ID is the control of the same test |
| `RFC9915-16.10-3` | If the client did not include a Client Identifier option in the original message, the Reply message MUST NOT include a Client Identifier option. (§16.10) | MUST NOT | 16.10 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The client SHOULD include a Client Identifier option (see Section 21.2) to identify itself to the server"; ze never omits it: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message adds a Client Identifier option to every message, so the condition never holds |
| `RFC9915-16.11-2` | Clients MUST discard any Reconfigure message that meets any of the following conditions: (§16.11) | MUST | 16.11 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L39). **{single-polarity}:** ze accepts no Reconfigure at all, so no conforming Reconfigure exists to assert positively |
| `RFC9915-16.12-1` | Clients MUST discard any received Information-request messages. (§16.12) | MUST | 16.12 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L36). **{single-polarity}:** the obligation is a discard, and a client has no Information-request to accept |
| `RFC9915-16.13-1` | Clients MUST discard any received Relay-forward messages. (§16.13) | MUST | 16.13 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L37). **{single-polarity}:** the obligation is a discard, and a client has no Relay-forward to accept |
| `RFC9915-16.14-1` | Clients and servers MUST discard any received Relay-reply messages. (§16.14) | MUST | 16.14 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L38). **{single-polarity}:** the obligation is a discard, and a client has no Relay-reply to accept |
| `RFC9915-18-1` | The client MUST NOT be using any of the addresses or delegated prefixes for which it tries to obtain the bindings by sending the Solicit message. (§18) | MUST NOT | 18 | **positive:** `unit/verify` [`TestRFC9915Section1825ExpiryRemovesAndSolicits`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L486). **negative:** no negative test. **{single-polarity}:** the client starts a Solicit only from an empty or expired lease, so no input yields a Solicit for an address in use to assert negatively |
| `RFC9915-18.2-1` | The client SHOULD be suspicious of the Reconfigure message (they may be faked), and it MUST NOT abandon any resources it might have already obtained. (§18.2) | MUST NOT | 18.2 | **positive:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L42). **negative:** no negative test. **{single-polarity}:** ze discards every Reconfigure, so no input makes it abandon a resource on one |
| `RFC9915-18.2.1-1` | The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.1) | MUST | 18.2.1 | **positive:** `unit/verify` [`TestRFC9915Section1821SolicitContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L148). **negative:** no negative test. **{single-polarity}:** the Client ID is written by the client, so no input removes it |
| `RFC9915-18.2.1-2` | The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.1) | MUST | 18.2.1 | **positive:** `unit/verify` [`TestRFC9915Section1821SolicitContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L149). **negative:** no negative test. **{single-polarity}:** the Elapsed Time is written by the client, so no input removes it |
| `RFC9915-18.2.1-3` | The client MUST include an Option Request option (ORO) (see Section 21.7) to request the SOL_MAX_RT option (see Section 21.24) and any other options the client is interested in receiving. (§18.2.1) | MUST | 18.2.1 | **positive:** `unit/verify` [`TestRFC9915Section1821FirstSolicitRTAboveIRT`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L91). **negative:** no negative test. **{single-polarity}:** the ORO is a constant the client writes, so no input removes SOL_MAX_RT from it |
| `RFC9915-18.2.1-4` | The client MUST NOT include any other options in the Solicit message, except as specifically allowed in the definition of individual options. (§18.2.1) | MUST NOT | 18.2.1 | **positive:** `unit/verify` [`TestRFC9915Section1821SolicitContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L150). **negative:** no negative test. **{single-polarity}:** the option set is fixed by the builder, so no input adds an option |
| `RFC9915-18.2.1-5` | Also, the first RT MUST be selected to be strictly greater than IRT by choosing RAND to be strictly greater than 0. (§18.2.1) | MUST | 18.2.1 | **positive:** `unit/verify` [`TestRFC9915Section1821FirstSolicitRTAboveIRT`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L92). **negative:** no negative test. **{single-polarity}:** RAND is drawn by the client, so no input makes the first RT reach IRT |
| `RFC9915-18.2.1-6` | A client MUST collect valid Advertise messages for the first RT seconds, unless it receives a valid Advertise message with a preference value of 255. (§18.2.1) | MUST | 18.2.1 | **positive:** `unit/verify` [`TestRFC9915Section1821AdvertiseSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L276). **negative:** no negative test. **{single-polarity}:** the collection window is the client's own schedule, so no input ends it early other than the 255 case asserted |
| `RFC9915-18.2.1-7` | If the DHCP client is configured with either MRC or MRD set to a value other than 0, it MUST stop trying to configure the interface if the message exchange fails. (§18.2.1) | MUST | 18.2.1 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "A DHCP client SHOULD choose MRC and MRD values of 0"; ze does: internal/plugins/iface/dhcp/dhcp_v6_linux.go::solicit runs the Solicit exchange with MRC and MRD 0 from Table 1, and no configuration leaf sets them |
| `RFC9915-18.2.2-1` | The client MUST include the identifier of the destination server in a Server Identifier option (see Section 21.3). (§18.2.2) | MUST | 18.2.2 | **positive:** `unit/verify` [`TestRFC9915Section1822RequestContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L185). **negative:** no negative test. **{single-polarity}:** the Server ID is copied by the builder, so no input removes it |
| `RFC9915-18.2.2-2` | The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.2) | MUST | 18.2.2 | **positive:** `unit/verify` [`TestRFC9915Section1822RequestContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L186). **negative:** no negative test. **{single-polarity}:** the Client ID is written by the client, so no input removes it |
| `RFC9915-18.2.2-3` | The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.2) | MUST | 18.2.2 | **positive:** `unit/verify` [`TestRFC9915Section1822RequestContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L187). **negative:** no negative test. **{single-polarity}:** the Elapsed Time is written by the client, so no input removes it |
| `RFC9915-18.2.2-4` | The client MUST include an Option Request option (see Section 21.7) to request the SOL_MAX_RT option (see Section 21.24) and any other options the client is interested in receiving. (§18.2.2) | MUST | 18.2.2 | **positive:** `unit/verify` [`TestRFC9915Section1822RequestContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L188). **negative:** no negative test. **{single-polarity}:** the ORO is a constant the client writes, so no input removes SOL_MAX_RT from it |
| `RFC9915-18.2.3-1` | The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.3) | MUST | 18.2.3 | **positive:** `unit/verify` [`TestRFC9915Section18212ConfirmOnMove`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L616). **negative:** no negative test. **{single-polarity}:** the Client ID is written by the client, so no input removes it |
| `RFC9915-18.2.3-2` | The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.3) | MUST | 18.2.3 | **positive:** `unit/verify` [`TestRFC9915Section1823ConfirmRetransmission`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L725). **negative:** no negative test. **{single-polarity}:** the Elapsed Time is written by the client, so no input removes it |
| `RFC9915-18.2.3-3` | The first Confirm message from the client on the interface MUST be delayed by a random amount of time between 0 and CNF_MAX_DELAY. (§18.2.3) | MUST | 18.2.3 | **positive:** `unit/verify` [`TestRFC9915Section18212ConfirmOnMove`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L617). **negative:** `unit/verify` [`TestDHCPv6ConfirmStopDuringDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L913) |
| `RFC9915-18.2.4-1` | However, as the client SHOULD renew/ rebind all IAs from the server at the same time, the client MUST select T1 and T2 times from all IA options that will guarantee that the client initiates transmissions of Renew/Rebind messages not later than at the T1/T2 times associated with any of the client's bindings (earliest T1/T2). (§18.2.4) | MUST | 18.2.4 | **positive:** `unit/verify` [`TestRFC9915Section1824EarliestT1AcrossIAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L804). **negative:** no negative test. **{single-polarity}:** the earliest-T1 selection is the client's own arithmetic, so the multi-IA input is itself the case asserted |
| `RFC9915-18.2.4-2` | A client MUST also initiate a Renew/Reply message exchange before time T1 if the client's link-local address used in previous interactions with the server is no longer valid and it is willing to receive Reconfigure messages. (§18.2.4) | MUST | 18.2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Reconfigure is in scope by owner decision 2026-10-19 and not yet implemented: ze is never willing to accept Reconfigure messages: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit sends no Reconfigure Accept option, and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher discards every Reconfigure; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| `RFC9915-18.2.4-4` | The client MUST include a Server Identifier option (see Section 21.3) in the Renew message, identifying the server with which the client most recently communicated. (§18.2.4) | MUST | 18.2.4 | **positive:** `unit/verify` [`TestRFC9915Section1824RenewAtT1CarriesLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L358). **negative:** no negative test. **{single-polarity}:** the Server ID is written by the client, so no input removes it |
| `RFC9915-18.2.4-5` | The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.4) | MUST | 18.2.4 | **positive:** `unit/verify` [`TestRFC9915Section1824RenewAtT1CarriesLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L359). **negative:** no negative test. **{single-polarity}:** the Client ID is written by the client, so no input removes it |
| `RFC9915-18.2.4-6` | The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.4) | MUST | 18.2.4 | **positive:** `unit/verify` [`TestRFC9915Section1824RenewAtT1CarriesLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L360). **negative:** no negative test. **{single-polarity}:** the Elapsed Time is written by the client, so no input removes it |
| `RFC9915-18.2.4-7` | The client MUST NOT include addresses and prefixes in any IA option that the client did not obtain from the server or that are no longer valid (that have a valid lifetime of 0). (§18.2.4) | MUST NOT | 18.2.4 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section1824RenewOmitsExpiredLeases`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L309). **{single-polarity}:** the address still valid is the control of the same test |
| `RFC9915-18.2.6-1` | The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.6) | MUST | 18.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-18.2.6-2` | The client MUST include an Option Request option (see Section 21.7) to request the INF_MAX_RT option (see Section 21.25), the Information Refresh Time option (see Section 21.23), and any other options the client is interested in receiving. (§18.2.6) | MUST | 18.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-18.2.6-3` | When responding to a Reconfigure, the client MUST include a Server Identifier option (see Section 21.3) with the identifier from the Reconfigure message to which the client is responding. (§18.2.6) | MUST | 18.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Reconfigure and stateless DHCPv6 are in scope by owner decision 2026-10-19 and not yet implemented: ze never responds to a Reconfigure, because internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher discards every Reconfigure, and every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request is sent; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| `RFC9915-18.2.6-4` | The first Information-request message from the client on the interface MUST be delayed by a random amount of time between 0 and INF_MAX_DELAY. (§18.2.6) | MUST | 18.2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-18.2.7-1` | The client MUST include a Server Identifier option (see Section 21.3) in the Renew message, identifying the server that allocated the lease(s). (§18.2.7) | MUST | 18.2.7 | **positive:** `unit/verify` [`TestRFC9915Section1827ReleaseContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L276). **negative:** no negative test. **{single-polarity}:** the Server ID is written by the client, so no input removes it |
| `RFC9915-18.2.7-2` | The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.7) | MUST | 18.2.7 | **positive:** `unit/verify` [`TestRFC9915Section1827ReleaseContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L277). **negative:** no negative test. **{single-polarity}:** the Client ID is written by the client, so no input removes it |
| `RFC9915-18.2.7-3` | The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.7) | MUST | 18.2.7 | **positive:** `unit/verify` [`TestRFC9915Section1827ReleaseContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L278). **negative:** no negative test. **{single-polarity}:** the Elapsed Time is written by the client, so no input removes it |
| `RFC9915-18.2.7-4` | The leases to be released MUST be included in the IAs. (§18.2.7) | MUST | 18.2.7 | **positive:** `unit/verify` [`TestRFC9915Section1827ReleaseContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L279). **negative:** no negative test. **{single-polarity}:** the Release carries every held lease by construction, so no input drops one |
| `RFC9915-18.2.7-5` | Any leases for the IAs the client wishes to continue to use MUST NOT be added to the IAs. (§18.2.7) | MUST NOT | 18.2.7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze sends Release only from internal/plugins/iface/dhcp/dhcp_v6_linux.go::shutdown, on Stop, when it keeps no lease, so no lease it wishes to keep exists |
| `RFC9915-18.2.7-6` | The client MUST stop using all of the leases being released before the client begins the Release message exchange process. (§18.2.7) | MUST | 18.2.7 | **positive:** `unit/verify` [`TestDHCPv6ReleaseOnStop`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L646). **negative:** no negative test. **{single-polarity}:** the removal order is fixed in internal/plugins/iface/dhcp/dhcp_v6_linux.go::shutdown, so no input reorders it |
| `RFC9915-18.2.7-7` | For an address, this means the address MUST have been removed from the interface. (§18.2.7) | MUST | 18.2.7 | **positive:** `unit/verify` [`TestDHCPv6ReleaseOnStop`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L647). **negative:** no negative test. **{single-polarity}:** the removal order is fixed in internal/plugins/iface/dhcp/dhcp_v6_linux.go::shutdown, so no input reorders it |
| `RFC9915-18.2.7-8` | For a delegated prefix, this means the prefix MUST have been advertised with a Preferred Lifetime and a Valid Lifetime of 0 in a Router Advertisement message as described in part (e) of Section 5.5.3 of [RFC4862]; also see requirement L-13 in Section 4.3 of [RFC7084]. (§18.2.7) | MUST | 18.2.7 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the client might assign a subnet from a delegated prefix to one of its interfaces and begin sending Router Advertisements for the prefix on that link"; ze uses no delegated prefix itself: internal/plugins/iface/dhcp/dhcp_v6_linux.go::effect records the IA_PD prefix and publishes it as a dhcp-acquired event, and its subscribers, handleDHCPLeaseEvent in internal/component/iface/register.go and syncWorker.handleDHCPEvent in internal/plugins/ntp/ntp.go, configure no address and send no Router Advertisement from it |
| `RFC9915-18.2.8-1` | The Decline message is not used in prefix delegation; thus, the client MUST NOT include IA_PD options (see Section 21.21) in the Decline message. (§18.2.8) | MUST NOT | 18.2.8 | **positive:** `unit/verify` [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L98). **negative:** no negative test. **{single-polarity}:** the IAs of a Decline are written by the client from its declined addresses, so no input adds an IA_PD |
| `RFC9915-18.2.8-2` | The client MUST include a Server Identifier option (see Section 21.3) in the Decline message, identifying the server that allocated the lease(s). (§18.2.8) | MUST | 18.2.8 | **positive:** `unit/verify` [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L99). **negative:** no negative test. **{single-polarity}:** the Server ID is written by the client, so no input removes it |
| `RFC9915-18.2.8-3` | The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.8) | MUST | 18.2.8 | **positive:** `unit/verify` [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L100). **negative:** no negative test. **{single-polarity}:** the Client ID is written by the client, so no input removes it |
| `RFC9915-18.2.8-4` | The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.8) | MUST | 18.2.8 | **positive:** `unit/verify` [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L101). **negative:** no negative test. **{single-polarity}:** the Elapsed Time is written by the client, so no input removes it |
| `RFC9915-18.2.8-5` | The addresses to be declined MUST be included in the IAs. (§18.2.8) | MUST | 18.2.8 | **positive:** `unit/verify` [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L102). **negative:** no negative test. **{single-polarity}:** the declined addresses are written by the client, so no input removes them |
| `RFC9915-18.2.8-6` | The client MUST NOT use any of the addresses it is declining as the source address in the Decline message or in any subsequently transmitted message. (§18.2.8) | MUST NOT | 18.2.8 | **positive:** `unit/verify` [`TestRFC9915DuplicateAddressDeclinedOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc9915_dhcp_v6_dad_integration_linux_test.go#L36). **positive:** `unit/verify` [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L103). **negative:** no negative test. **{single-polarity}:** the declined address leaves the device before the Decline is built, so no input puts it back |
| `RFC9915-18.2.9-1` | The client MUST process any SOL_MAX_RT option (§18.2.9) | MUST | 18.2.9 | **positive:** `unit/verify` [`TestRFC9915Section163AdvertiseValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L90). **negative:** no negative test. **{single-polarity}:** applying a valid SOL_MAX_RT has no wrong input but the out-of-range one, row RFC9915-21.24-3 |
| `RFC9915-18.2.9-2` | The client MUST ignore any Advertise message that contains no addresses (IA Address options (see Section 21.6) encapsulated in IA_NA options (see Section 21.4)) and no delegated prefixes (IA Prefix options (see Section 21.22) encapsulated in IA_PD options (see Section 21.21)), with the exception that the client: (§18.2.9) | MUST | 18.2.9 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section163AdvertiseValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L89). **{single-polarity}:** the Advertise carrying a lease is the control in RFC9915-18.2.1-6 |
| `RFC9915-18.2.9-3` | MUST process an included SOL_MAX_RT option and (§18.2.9) | MUST | 18.2.9 | **positive:** `unit/verify` [`TestRFC9915Section163AdvertiseValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L91). **negative:** no negative test. **{single-polarity}:** applying a valid SOL_MAX_RT has no wrong input but the out-of-range one, row RFC9915-21.24-3 |
| `RFC9915-18.2.9-4` | MUST process an included INF_MAX_RT option. (§18.2.9) | MUST | 18.2.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-18.2.9-5` | The client ignoring an Advertise message MUST NOT restart the Solicit retransmission timer. (§18.2.9) | MUST NOT | 18.2.9 | **positive:** `unit/verify` [`TestDHCPv6LinkOneDatagramPerExchange`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_link_linux_test.go#L109). **negative:** no negative test. **{single-polarity}:** the retransmission timer is the client's own, so no input restarts it |
| `RFC9915-18.2.10-1` | The client MUST process any SOL_MAX_RT option (§18.2.10) | MUST | 18.2.10 | **positive:** `unit/verify` [`TestRFC9915Section2124SolMaxRTRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L181). **negative:** no negative test. **{single-polarity}:** applying a valid SOL_MAX_RT has no wrong input but the out-of-range one, row RFC9915-21.24-3 |
| `RFC9915-18.2.10.1-1` | The client MUST perform duplicate address detection as per Section 5.4 of [RFC4862], which does list some exceptions, on each of the received addresses in any IAs on which it has not performed duplicate address detection during processing of any of the previous Reply messages from the server. (§18.2.10.1) | MUST | 18.2.10.1 | **positive:** `unit/verify` [`TestRFC9915DuplicateAddressDeclinedOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc9915_dhcp_v6_dad_integration_linux_test.go#L35). **positive:** `unit/verify` [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L97). **negative:** `unit/verify` [`TestRFC9915Section182101UniqueAddressKept`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L183) |
| `RFC9915-18.2.10.1-3` | Addresses obtained from an IA Address option MUST NOT be used to form an implicit prefix with a length other than 128. (§18.2.10.1) | MUST NOT | 18.2.10.1 | **positive:** `unit/verify` [`TestDHCPv6LeaseHeldAcrossFiveRenewals`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L414). **negative:** no negative test. **{single-polarity}:** the prefix length is a constant ze installs, so no input yields another length |
| `RFC9915-18.2.10.5-1` | As a counterexample, a client that previously received an NTP server address from the DHCP server and does not receive it anymore MUST stop using the configured NTP server address. (§18.2.10.5) | MUST | 18.2.10.5 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "The client MUST include an Option Request option (ORO) (see Section 21.7) to request the SOL_MAX_RT option (see Section 21.24) and any other options the client is interested in receiving"; ze asks for no NTP server option: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::requestOptions requests DNS Recursive Name Server and SOL_MAX_RT only, so no NTP server address is ever configured from DHCPv6; the DNS case the sentence illustrates is met: a Reply without the DNS option empties the lease's list and setV6Resolvers in internal/plugins/iface/dhcp/resolv_linux.go withdraws the servers |
| `RFC9915-18.2.11-1` | The message MUST be dropped if it doesn't pass the validation, as explained in Section 16.11, particularly in cases where the authentication is missing or fails. (§18.2.11) | MUST | 18.2.11 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L40). **{single-polarity}:** ze accepts no Reconfigure at all, so no validated Reconfigure exists to assert positively |
| `RFC9915-18.2.11-2` | Once the client has received a Reconfigure, the client proceeds with the message exchange (retransmitting the Renew, Rebind, or Information-request message if necessary); the client MUST ignore any additional Reconfigure messages until the exchange is complete. (§18.2.11) | MUST | 18.2.11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Reconfigure is in scope by owner decision 2026-10-19 and not yet implemented: ze is never willing to accept Reconfigure messages: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit sends no Reconfigure Accept option, and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher discards every Reconfigure; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| `RFC9915-18.2.12-1` | The client MUST include any IAs assigned to the interface that may have moved to a new link, along with the addresses associated with those IAs, in its Confirm message. (§18.2.12) | MUST | 18.2.12 | **positive:** `unit/verify` [`TestRFC9915Section18212ConfirmOnMove`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L618). **negative:** `unit/verify` [`TestRFC9915Section18212NoMoveNoConfirm`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L694) |
| `RFC9915-18.2.12-2` | If the client has any valid delegated prefixes obtained from the DHCP server, the client MUST initiate a Rebind/Reply message exchange as described in Section 18.2.5, with the exception that the retransmission parameters should be set as for the Confirm message (see Section 18.2.3). (§18.2.12) | MUST | 18.2.12 | **positive:** `unit/verify` [`TestRFC9915LinkMoveRebindsThroughTheMonitor`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc9915_dhcp_v6_link_move_integration_linux_test.go#L49). **positive:** `unit/verify` [`TestRFC9915Section18212MoveRebindsWithConfirmParameters`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L398). **negative:** `unit/verify` [`TestRFC9915LinkChangeWithoutDownSendsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc9915_dhcp_v6_link_move_integration_linux_test.go#L96). **negative:** `unit/verify` [`TestRFC9915Section18212NoMoveNoRebind`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L446) |
| `RFC9915-18.2.12-3` | If the client has only obtained network information using Information-request/Reply message exchanges, the client MUST initiate an Information-request/Reply message exchange as described in Section 18.2.6. (§18.2.12) | MUST | 18.2.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-18.2.12-4` | However, a client MUST rate- limit such initiation attempts to avoid flooding a server with requests when there are link issues (for example, only doing one of these at most every 30 seconds). (§18.2.12) | MUST | 18.2.12 | **positive:** `unit/verify` [`TestRFC9915Section18212MoveRebindRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L492). **negative:** no negative test. **{single-polarity}:** a link-change Rebind before the interval is the only wrong outcome, and the delay is fixed by the client |
| `RFC9915-18.3.5-1` | The Reply with lifetimes of 0 constitutes an explicit notification to the client that the specific addresses and delegated prefixes are no longer valid and MUST NOT be used by the client. (§18.3.5) | MUST NOT | 18.3.5 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section182101ValidZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L731). **{single-polarity}:** a lease with a non-zero valid lifetime is recorded in every exchange test |
| `RFC9915-20.2-1` | Any DHCP message MUST NOT include more than one Authentication option. (§20.2) | MUST NOT | 20.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Authentication option is in scope by owner decision 2026-10-19 and not yet implemented: ze's client sends and reads no Authentication option (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message builds none, and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher accepts only Reply messages, judged without authentication); disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| `RFC9915-20.3-3` | A client that receives a message with the RDM field set to 0x00 MUST compare its replay detection field with the previous value sent by that same server (based on the Server Identifier option; see Section 21.3) and only accept the message if the received value is greater and record this as the new value. (§20.3) | MUST | 20.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Authentication option (RKAP) is in scope by owner decision 2026-10-19 and not yet implemented: ze never negotiates Reconfigure: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit sends no Reconfigure Accept option, so no Authentication option is processed and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher judges Replies without one; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| `RFC9915-20.3-4` | If this is the first time a client processes an Authentication option sent by a server, the client MUST record the replay detection value and skip the replay detection check. (§20.3) | MUST | 20.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Authentication option (RKAP) is in scope by owner decision 2026-10-19 and not yet implemented: ze never negotiates Reconfigure: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit sends no Reconfigure Accept option, so no Authentication option is processed and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher judges Replies without one; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| `RFC9915-21-1` | If an option does appear multiple times, each instance is considered separate and the data areas of the options MUST NOT be concatenated or otherwise combined. (§21) | MUST NOT | 21 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** vendored insomniacslk/dhcp dhcpv6 option decoder; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::v6Identified reads each option through the library's per-code accessors, and the decoder keeps every instance separate, so ze combines no option data |
| `RFC9915-21.4-2` | In a message sent by a server to a client, the client MUST use the values in the T1 and T2 fields for the T1 and T2 times, unless values in those fields are 0. (§21.4) | MUST | 21.4 | **positive:** `unit/verify` [`TestRFC9915Section1824RenewAtT1CarriesLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L361). **negative:** no negative test. **{single-polarity}:** using the server's T1 has no wrong input but zero, the separate row RFC9915-14.2-1 |
| `RFC9915-21.6-3` | In a message sent by a server to a client, the client MUST use the values in the preferred-lifetime and valid-lifetime fields for the preferred and valid lifetimes. (§21.6) | MUST | 21.6 | **positive:** `unit/verify` [`TestDHCPv6LeaseHeldAcrossFiveRenewals`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L415). **negative:** no negative test. **{single-polarity}:** installing the lease's lifetimes has no wrong input but preferred above valid, the separate row RFC9915-21.6-4 |
| `RFC9915-21.6-4` | The client MUST discard any addresses for which the preferred lifetime is greater than the valid lifetime. (§21.6) | MUST | 21.6 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section216PreferredAboveValidDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L749). **{single-polarity}:** the valid address is recorded in every exchange test |
| `RFC9915-21.7-1` | A client MUST include an Option Request option in a Solicit, Request, Renew, Rebind, or Information-request message to inform the server about options the client wants the server to send to the client. (§21.7) | MUST | 21.7 | **positive:** `unit/verify` [`TestRFC9915Section217OptionRequestEveryMessage`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L223). **negative:** no negative test. **{single-polarity}:** the ORO is a constant the client writes, so no input removes it |
| `RFC9915-21.7-2` | The Option Request option MUST NOT include the following option codes: (§21.7) | MUST NOT | 21.7 | **positive:** `unit/verify` [`TestRFC9915Section217OptionRequestEveryMessage`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L224). **negative:** no negative test. **{single-polarity}:** the ORO is a constant the client writes, so no input adds a forbidden code |
| `RFC9915-21.9-1` | A client MUST include an Elapsed Time option in messages to indicate how long the client has been trying to complete a DHCP message exchange. (§21.9) | MUST | 21.9 | **positive:** `unit/verify` [`TestRFC9915Section217OptionRequestEveryMessage`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L226). **negative:** no negative test. **{single-polarity}:** the Elapsed Time is written by the client, so no input removes it |
| `RFC9915-21.15-1` | The user class information carried in this option MUST be configurable on the client. (§21.15) | MUST | 21.15 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the User Class option can be used to configure all clients of people in the accounting department"; ze sends no User Class option: the messages internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease build carry only Client ID, Server ID, Elapsed Time, ORO, Rapid Commit, IA_NA and IA_PD |
| `RFC9915-21.15-2` | The data area of the User Class option MUST contain one or more instances of user-class-data information. (§21.15) | MUST | 21.15 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "the User Class option can be used to configure all clients of people in the accounting department"; ze sends no User Class option: the messages internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease build carry only Client ID, Server ID, Elapsed Time, ORO, Rapid Commit, IA_NA and IA_PD |
| `RFC9915-21.16-1` | Servers and clients MUST NOT include more than one instance of OPTION_VENDOR_CLASS with the same Enterprise Number. (§21.16) | MUST NOT | 21.16 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's client sends no Vendor Class option: the messages it builds carry only Client ID, Server ID, Elapsed Time, ORO, Rapid Commit, IA_NA and IA_PD (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease) |
| `RFC9915-21.17-1` | The vendor-option-data field MUST be encoded as a sequence of code/length/value fields of format identical to the DHCP options (see Section 21.1). (§21.17) | MUST | 21.17 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "A client that is interested in receiving a Vendor-specific Information option"; ze is not: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::requestOptions requests DNS Recursive Name Server and SOL_MAX_RT only, and the messages internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message builds carry no Vendor-specific Information option |
| `RFC9915-21.17-2` | Servers and clients MUST NOT send more than one instance of the Vendor-specific Information option with the same Enterprise Number. (§21.17) | MUST NOT | 21.17 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze's client sends no Vendor-specific Information option: the messages it builds carry only Client ID, Server ID, Elapsed Time, ORO, Rapid Commit, IA_NA and IA_PD (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease) |
| `RFC9915-21.17-3` | MUST specify the Vendor-specific Information option in an Option Request option. (§21.17) | MUST | 21.17 | **positive:** no positive test. **negative:** no negative test. **{feature-declined}:** "A client that is interested in receiving a Vendor-specific Information option"; ze is not: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::requestOptions requests DNS Recursive Name Server and SOL_MAX_RT only, and the messages internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message builds carry no Vendor-specific Information option |
| `RFC9915-21.22-2` | The client MUST discard any prefixes for which the preferred lifetime is greater than the valid lifetime. (§21.22) | MUST | 21.22 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section2122PrefixPreferredAboveValidDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L335). **{single-polarity}:** the valid prefix is the control of the same test |
| `RFC9915-21.23-1` | A DHCP client MUST request this option in the Option Request option (see Section 21.7) when sending Information-request messages. (§21.23) | MUST | 21.23 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-21.23-2` | A client MUST NOT request this option in the Option Request option in any other messages. (§21.23) | MUST NOT | 21.23 | **positive:** `unit/verify` [`TestRFC9915Section217OptionRequestEveryMessage`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L225). **negative:** no negative test. **{single-polarity}:** the ORO is a constant the client writes, so no input adds the option |
| `RFC9915-21.23-5` | If the Reply to an Information-request message does not contain this option, the client MUST behave as if the option with the value IRT_DEFAULT was provided. (§21.23) | MUST | 21.23 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-21.23-6` | A client MUST use the refresh time IRT_MINIMUM if it receives the option with a value less than IRT_MINIMUM. (§21.23) | MUST | 21.23 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-21.23-7` | When the client detects that the refresh time has expired, it SHOULD try to update its configuration data by sending an Information- request as specified in Section 18.2.6, except that the client MUST delay sending the first Information-request by a random amount of time between 0 and INF_MAX_DELAY. (§21.23) | MUST | 21.23 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-21.24-3` | A DHCP client MUST ignore any SOL_MAX_RT option values that are less than 60 or more than 86400. (§21.24) | MUST | 21.24 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section2124SolMaxRTRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L179). **{single-polarity}:** the in-range value is the separate row RFC9915-21.24-4 |
| `RFC9915-21.24-4` | If a DHCP client receives a message containing a SOL_MAX_RT option that has a valid value for SOL_MAX_RT, the client MUST set its internal SOL_MAX_RT parameter to the value contained in the SOL_MAX_RT option. (§21.24) | MUST | 21.24 | **positive:** `unit/verify` [`TestRFC9915Section2124SolMaxRTRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L180). **negative:** no negative test. **{single-polarity}:** the out-of-range value is the separate row RFC9915-21.24-3 |
| `RFC9915-21.25-2` | A DHCP client MUST include the INF_MAX_RT option code in any Option Request option (see Section 21.7) it sends in an Information-request message. (§21.25) | MUST | 21.25 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-21.25-3` | A DHCP client MUST ignore any INF_MAX_RT option values that are less than 60 or more than 86400. (§21.25) | MUST | 21.25 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-21.25-4` | If a DHCP client receives a message containing an INF_MAX_RT option that has a valid value for INF_MAX_RT, the client MUST set its internal INF_MAX_RT parameter to the value contained in the INF_MAX_RT option. (§21.25) | MUST | 21.25 | **positive:** no positive test. **negative:** no negative test. **{gap}:** stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| `RFC9915-22.3-1` | Because of the opportunity for attack through the Reconfigure message, a DHCP client MUST discard any Reconfigure message that does not include authentication or that does not pass the validation process for the authentication protocol. (§22.3) | MUST | 22.3 | **positive:** no positive test. **negative:** `unit/verify` [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L41). **{single-polarity}:** ze accepts no Reconfigure at all, so no authenticated Reconfigure exists to assert positively |
| `RFC9915-A-1` | Legacy implementations MAY support them, but implementations conformant to this document MUST NOT rely on them. (§A) | MUST NOT | A | **positive:** `unit/verify` [`TestRFC9915Section1821SolicitContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L151). **negative:** no negative test. **{single-polarity}:** the IA set is fixed by the builder, so no input adds an IA_TA |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9915-6.3-1`](#rfc9915-6.3-1) If the client uses a delegated prefix to configure addresses on interfaces on itself or other nodes behind it, the preferred and valid lifetimes of those addresses MUST be no longer than the remaining preferred and valid lifetimes, respectively, for the delegated prefix at any time. (§6.3) | no test | no test carries this requirement id; annotated {feature-declined}: "the client might assign a subnet from a delegated prefix to one of its interfaces and begin sending Router Advertisements for the prefix on that link"; ze uses no delegated prefix itself: internal/plugins/iface/dhcp/dhcp_v6_linux.go::effect records the IA_PD prefix and publishes it as a dhcp-acquired event, and its subscribers, handleDHCPLeaseEvent in internal/component/iface/register.go and syncWorker.handleDHCPEvent in internal/plugins/ntp/ntp.go, configure no address and send no Router Advertisement from it |
| [`RFC9915-6.3-2`](#rfc9915-6.3-2) In particular, if the delegated prefix or a prefix derived from it is advertised for stateless address autoconfiguration [RFC4862], the advertised preferred and valid lifetimes MUST NOT exceed the corresponding remaining lifetimes of the delegated prefix. (§6.3) | no test | no test carries this requirement id; annotated {feature-declined}: "the client might assign a subnet from a delegated prefix to one of its interfaces and begin sending Router Advertisements for the prefix on that link"; ze uses no delegated prefix itself: internal/plugins/iface/dhcp/dhcp_v6_linux.go::effect records the IA_PD prefix and publishes it as a dhcp-acquired event, and its subscribers, handleDHCPLeaseEvent in internal/component/iface/register.go and syncWorker.handleDHCPEvent in internal/plugins/ntp/ntp.go, configure no address and send no Router Advertisement from it |
| [`RFC9915-6.3-3`](#rfc9915-6.3-3) A client that has delegated any of the address space received through DHCP Prefix Delegation MUST NOT issue a DHCP Release on the relevant delegated prefix while any of the address space is outstanding. (§6.3) | no test | no test carries this requirement id; annotated {feature-declined}: "the client might assign a subnet from a delegated prefix to one of its interfaces and begin sending Router Advertisements for the prefix on that link"; ze uses no delegated prefix itself: internal/plugins/iface/dhcp/dhcp_v6_linux.go::effect records the IA_PD prefix and publishes it as a dhcp-acquired event, and its subscribers, handleDHCPLeaseEvent in internal/component/iface/register.go and syncWorker.handleDHCPEvent in internal/plugins/ntp/ntp.go, configure no address and send no Router Advertisement from it |
| [`RFC9915-7.2-1`](#rfc9915-7.2-1) Clients MUST listen for DHCP messages on UDP port 546. (§7.2) | no test | no test carries this requirement id; annotated {lower-layer}: vendored insomniacslk/dhcp nclient6; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::dialV6Link opens nclient6.New on the device and passes no port, and the library binds UDP port 546 |
| [`RFC9915-7.2-3`](#rfc9915-7.2-3) Therefore, clients MUST send DHCP messages to UDP destination port 547. (§7.2) | no test | no test carries this requirement id; annotated {lower-layer}: vendored insomniacslk/dhcp nclient6; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::dialV6Link passes no destination, and the library sends every client message to All_DHCP_Relay_Agents_and_Servers port 547 |
| [`RFC9915-10-1`](#rfc9915-10-1) The message compression scheme in Section 4.1.4 of [RFC1035] MUST NOT be used. (§10) | no test | no test carries this requirement id; annotated {not-applicable}: ze's client encodes no domain name: the options it sends (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease) carry DUIDs, codes, IAs and timers, never a DNS-encoded name |
| [`RFC9915-11.2-1`](#rfc9915-11.2-1) The hardware type MUST be a valid hardware type assigned by IANA; see [IANA-HARDWARE-TYPES]. (§11.2) | no test | no test carries this requirement id; annotated {feature-declined}: "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| [`RFC9915-11.2-2`](#rfc9915-11.2-2) Clients and servers using this type of DUID MUST store the DUID-LLT in stable storage and MUST continue to use this DUID-LLT even if the network interface used to generate the DUID-LLT is removed. (§11.2) | no test | no test carries this requirement id; annotated {feature-declined}: "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| [`RFC9915-11.2-3`](#rfc9915-11.2-3) Clients and servers that do not have any stable storage MUST NOT use this type of DUID. (§11.2) | no test | no test carries this requirement id; annotated {feature-declined}: "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| [`RFC9915-11.2-4`](#rfc9915-11.2-4) Clients and servers that use this DUID SHOULD attempt to configure the time prior to generating the DUID, if that is possible, and MUST use some sort of time source (for example, a real-time clock) in generating the DUID, even if that time source could not be configured prior to generating the DUID. (§11.2) | no test | no test carries this requirement id; annotated {feature-declined}: "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| [`RFC9915-11.2-5`](#rfc9915-11.2-5) A DHCP client that generates a DUID-LLT using this mechanism MUST provide an administrative interface that replaces the existing DUID with a newly generated DUID-LLT. (§11.2) | no test | no test carries this requirement id; annotated {feature-declined}: "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| [`RFC9915-11.3-1`](#rfc9915-11.3-1) The source of the identifier is left up to the vendor defining it, but each identifier part of each DUID-EN MUST be unique to the device that is using it, and MUST be assigned to the device no later than at the first usage and stored in some form of non-volatile storage. (§11.3) | no test | no test carries this requirement id; annotated {feature-declined}: "the ways in which a DUID may be constructed"; ze generates no DUID of this type: internal/plugins/iface/dhcp/dhcp_v6_linux.go::identify builds a DUID-LL from the device MAC, or takes the operator's configured duid octets as given |
| [`RFC9915-15-2`](#rfc9915-15-2) Of course, a client MUST listen for a Reconfigure if it has negotiated for its use with the server. (§15) | {gap}, no test | Reconfigure is in scope by owner decision 2026-10-19 and not yet implemented: ze is never willing to accept Reconfigure messages: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit sends no Reconfigure Accept option, and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher discards every Reconfigure; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| [`RFC9915-16.10-3`](#rfc9915-16.10-3) If the client did not include a Client Identifier option in the original message, the Reply message MUST NOT include a Client Identifier option. (§16.10) | no test | no test carries this requirement id; annotated {feature-declined}: "The client SHOULD include a Client Identifier option (see Section 21.2) to identify itself to the server"; ze never omits it: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message adds a Client Identifier option to every message, so the condition never holds |
| [`RFC9915-18.2.1-7`](#rfc9915-18.2.1-7) If the DHCP client is configured with either MRC or MRD set to a value other than 0, it MUST stop trying to configure the interface if the message exchange fails. (§18.2.1) | no test | no test carries this requirement id; annotated {feature-declined}: "A DHCP client SHOULD choose MRC and MRD values of 0"; ze does: internal/plugins/iface/dhcp/dhcp_v6_linux.go::solicit runs the Solicit exchange with MRC and MRD 0 from Table 1, and no configuration leaf sets them |
| [`RFC9915-18.2.4-2`](#rfc9915-18.2.4-2) A client MUST also initiate a Renew/Reply message exchange before time T1 if the client's link-local address used in previous interactions with the server is no longer valid and it is willing to receive Reconfigure messages. (§18.2.4) | {gap}, no test | Reconfigure is in scope by owner decision 2026-10-19 and not yet implemented: ze is never willing to accept Reconfigure messages: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit sends no Reconfigure Accept option, and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher discards every Reconfigure; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| [`RFC9915-18.2.6-1`](#rfc9915-18.2.6-1) The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.6) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| [`RFC9915-18.2.6-2`](#rfc9915-18.2.6-2) The client MUST include an Option Request option (see Section 21.7) to request the INF_MAX_RT option (see Section 21.25), the Information Refresh Time option (see Section 21.23), and any other options the client is interested in receiving. (§18.2.6) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| [`RFC9915-18.2.6-3`](#rfc9915-18.2.6-3) When responding to a Reconfigure, the client MUST include a Server Identifier option (see Section 21.3) with the identifier from the Reconfigure message to which the client is responding. (§18.2.6) | {gap}, no test | Reconfigure and stateless DHCPv6 are in scope by owner decision 2026-10-19 and not yet implemented: ze never responds to a Reconfigure, because internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher discards every Reconfigure, and every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request is sent; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| [`RFC9915-18.2.6-4`](#rfc9915-18.2.6-4) The first Information-request message from the client on the interface MUST be delayed by a random amount of time between 0 and INF_MAX_DELAY. (§18.2.6) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| [`RFC9915-18.2.7-5`](#rfc9915-18.2.7-5) Any leases for the IAs the client wishes to continue to use MUST NOT be added to the IAs. (§18.2.7) | no test | no test carries this requirement id; annotated {not-applicable}: ze sends Release only from internal/plugins/iface/dhcp/dhcp_v6_linux.go::shutdown, on Stop, when it keeps no lease, so no lease it wishes to keep exists |
| [`RFC9915-18.2.7-8`](#rfc9915-18.2.7-8) For a delegated prefix, this means the prefix MUST have been advertised with a Preferred Lifetime and a Valid Lifetime of 0 in a Router Advertisement message as described in part (e) of Section 5.5.3 of [RFC4862]; also see requirement L-13 in Section 4.3 of [RFC7084]. (§18.2.7) | no test | no test carries this requirement id; annotated {feature-declined}: "the client might assign a subnet from a delegated prefix to one of its interfaces and begin sending Router Advertisements for the prefix on that link"; ze uses no delegated prefix itself: internal/plugins/iface/dhcp/dhcp_v6_linux.go::effect records the IA_PD prefix and publishes it as a dhcp-acquired event, and its subscribers, handleDHCPLeaseEvent in internal/component/iface/register.go and syncWorker.handleDHCPEvent in internal/plugins/ntp/ntp.go, configure no address and send no Router Advertisement from it |
| [`RFC9915-18.2.9-4`](#rfc9915-18.2.9-4) MUST process an included INF_MAX_RT option. (§18.2.9) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| [`RFC9915-18.2.10.5-1`](#rfc9915-18.2.10.5-1) As a counterexample, a client that previously received an NTP server address from the DHCP server and does not receive it anymore MUST stop using the configured NTP server address. (§18.2.10.5) | no test | no test carries this requirement id; annotated {feature-declined}: "The client MUST include an Option Request option (ORO) (see Section 21.7) to request the SOL_MAX_RT option (see Section 21.24) and any other options the client is interested in receiving"; ze asks for no NTP server option: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::requestOptions requests DNS Recursive Name Server and SOL_MAX_RT only, so no NTP server address is ever configured from DHCPv6; the DNS case the sentence illustrates is met: a Reply without the DNS option empties the lease's list and setV6Resolvers in internal/plugins/iface/dhcp/resolv_linux.go withdraws the servers |
| [`RFC9915-18.2.11-2`](#rfc9915-18.2.11-2) Once the client has received a Reconfigure, the client proceeds with the message exchange (retransmitting the Renew, Rebind, or Information-request message if necessary); the client MUST ignore any additional Reconfigure messages until the exchange is complete. (§18.2.11) | {gap}, no test | Reconfigure is in scope by owner decision 2026-10-19 and not yet implemented: ze is never willing to accept Reconfigure messages: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit sends no Reconfigure Accept option, and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher discards every Reconfigure; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| [`RFC9915-18.2.12-3`](#rfc9915-18.2.12-3) If the client has only obtained network information using Information-request/Reply message exchanges, the client MUST initiate an Information-request/Reply message exchange as described in Section 18.2.6. (§18.2.12) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| [`RFC9915-20.2-1`](#rfc9915-20.2-1) Any DHCP message MUST NOT include more than one Authentication option. (§20.2) | {gap}, no test | the Authentication option is in scope by owner decision 2026-10-19 and not yet implemented: ze's client sends and reads no Authentication option (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message builds none, and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher accepts only Reply messages, judged without authentication); disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| [`RFC9915-20.3-3`](#rfc9915-20.3-3) A client that receives a message with the RDM field set to 0x00 MUST compare its replay detection field with the previous value sent by that same server (based on the Server Identifier option; see Section 21.3) and only accept the message if the received value is greater and record this as the new value. (§20.3) | {gap}, no test | the Authentication option (RKAP) is in scope by owner decision 2026-10-19 and not yet implemented: ze never negotiates Reconfigure: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit sends no Reconfigure Accept option, so no Authentication option is processed and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher judges Replies without one; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| [`RFC9915-20.3-4`](#rfc9915-20.3-4) If this is the first time a client processes an Authentication option sent by a server, the client MUST record the replay detection value and skip the replay detection check. (§20.3) | {gap}, no test | the Authentication option (RKAP) is in scope by owner decision 2026-10-19 and not yet implemented: ze never negotiates Reconfigure: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit sends no Reconfigure Accept option, so no Authentication option is processed and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::replyMatcher judges Replies without one; disclosed in Support remaining; plan/spec-dhcpv6-client-4-reconfigure.md |
| [`RFC9915-21-1`](#rfc9915-21-1) If an option does appear multiple times, each instance is considered separate and the data areas of the options MUST NOT be concatenated or otherwise combined. (§21) | no test | no test carries this requirement id; annotated {lower-layer}: vendored insomniacslk/dhcp dhcpv6 option decoder; internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::v6Identified reads each option through the library's per-code accessors, and the decoder keeps every instance separate, so ze combines no option data |
| [`RFC9915-21.15-1`](#rfc9915-21.15-1) The user class information carried in this option MUST be configurable on the client. (§21.15) | no test | no test carries this requirement id; annotated {feature-declined}: "the User Class option can be used to configure all clients of people in the accounting department"; ze sends no User Class option: the messages internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease build carry only Client ID, Server ID, Elapsed Time, ORO, Rapid Commit, IA_NA and IA_PD |
| [`RFC9915-21.15-2`](#rfc9915-21.15-2) The data area of the User Class option MUST contain one or more instances of user-class-data information. (§21.15) | no test | no test carries this requirement id; annotated {feature-declined}: "the User Class option can be used to configure all clients of people in the accounting department"; ze sends no User Class option: the messages internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit and internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease build carry only Client ID, Server ID, Elapsed Time, ORO, Rapid Commit, IA_NA and IA_PD |
| [`RFC9915-21.16-1`](#rfc9915-21.16-1) Servers and clients MUST NOT include more than one instance of OPTION_VENDOR_CLASS with the same Enterprise Number. (§21.16) | no test | no test carries this requirement id; annotated {not-applicable}: ze's client sends no Vendor Class option: the messages it builds carry only Client ID, Server ID, Elapsed Time, ORO, Rapid Commit, IA_NA and IA_PD (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease) |
| [`RFC9915-21.17-1`](#rfc9915-21.17-1) The vendor-option-data field MUST be encoded as a sequence of code/length/value fields of format identical to the DHCP options (see Section 21.1). (§21.17) | no test | no test carries this requirement id; annotated {feature-declined}: "A client that is interested in receiving a Vendor-specific Information option"; ze is not: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::requestOptions requests DNS Recursive Name Server and SOL_MAX_RT only, and the messages internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message builds carry no Vendor-specific Information option |
| [`RFC9915-21.17-2`](#rfc9915-21.17-2) Servers and clients MUST NOT send more than one instance of the Vendor-specific Information option with the same Enterprise Number. (§21.17) | no test | no test carries this requirement id; annotated {not-applicable}: ze's client sends no Vendor-specific Information option: the messages it builds carry only Client ID, Server ID, Elapsed Time, ORO, Rapid Commit, IA_NA and IA_PD (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease) |
| [`RFC9915-21.17-3`](#rfc9915-21.17-3) MUST specify the Vendor-specific Information option in an Option Request option. (§21.17) | no test | no test carries this requirement id; annotated {feature-declined}: "A client that is interested in receiving a Vendor-specific Information option"; ze is not: internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::requestOptions requests DNS Recursive Name Server and SOL_MAX_RT only, and the messages internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::newV6Message builds carry no Vendor-specific Information option |
| [`RFC9915-21.23-1`](#rfc9915-21.23-1) A DHCP client MUST request this option in the Option Request option (see Section 21.7) when sending Information-request messages. (§21.23) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| [`RFC9915-21.23-5`](#rfc9915-21.23-5) If the Reply to an Information-request message does not contain this option, the client MUST behave as if the option with the value IRT_DEFAULT was provided. (§21.23) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| [`RFC9915-21.23-6`](#rfc9915-21.23-6) A client MUST use the refresh time IRT_MINIMUM if it receives the option with a value less than IRT_MINIMUM. (§21.23) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| [`RFC9915-21.23-7`](#rfc9915-21.23-7) When the client detects that the refresh time has expired, it SHOULD try to update its configuration data by sending an Information- request as specified in Section 18.2.6, except that the client MUST delay sending the first Information-request by a random amount of time between 0 and INF_MAX_DELAY. (§21.23) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| [`RFC9915-21.25-2`](#rfc9915-21.25-2) A DHCP client MUST include the INF_MAX_RT option code in any Option Request option (see Section 21.7) it sends in an Information-request message. (§21.25) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| [`RFC9915-21.25-3`](#rfc9915-21.25-3) A DHCP client MUST ignore any INF_MAX_RT option values that are less than 60 or more than 86400. (§21.25) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |
| [`RFC9915-21.25-4`](#rfc9915-21.25-4) If a DHCP client receives a message containing an INF_MAX_RT option that has a valid value for INF_MAX_RT, the client MUST set its internal INF_MAX_RT parameter to the value contained in the INF_MAX_RT option. (§21.25) | {gap}, no test | stateless DHCPv6 is in scope by owner decision 2026-10-19 and not yet implemented: every exchange the client starts is a Solicit, Request, Renew, Rebind, Decline or Release (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildSolicit, internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::buildFromLease), so no Information-request, INF_MAX_RT or Information Refresh Time state exists; disclosed in Support remaining; plan/spec-dhcpv6-client-3-stateless.md |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9915-6.3-1`](#rfc9915-6.3-1)

If the client uses a delegated prefix to configure addresses on interfaces on itself or other nodes behind it, the preferred and valid lifetimes of those addresses MUST be no longer than the remaining preferred and valid lifetimes, respectively, for the delegated prefix at any time. (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-6.3-1, so no unit is bound to it.

### [`RFC9915-6.3-2`](#rfc9915-6.3-2)

In particular, if the delegated prefix or a prefix derived from it is advertised for stateless address autoconfiguration [RFC4862], the advertised preferred and valid lifetimes MUST NOT exceed the corresponding remaining lifetimes of the delegated prefix. (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-6.3-2, so no unit is bound to it.

### [`RFC9915-6.3-3`](#rfc9915-6.3-3)

A client that has delegated any of the address space received through DHCP Prefix Delegation MUST NOT issue a DHCP Release on the relevant delegated prefix while any of the address space is outstanding. (§6.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-6.3-3, so no unit is bound to it.

### [`RFC9915-7.2-1`](#rfc9915-7.2-1)

Clients MUST listen for DHCP messages on UDP port 546. (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-7.2-1, so no unit is bound to it.

### [`RFC9915-7.2-3`](#rfc9915-7.2-3)

Therefore, clients MUST send DHCP messages to UDP destination port 547. (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-7.2-3, so no unit is bound to it.

### [`RFC9915-10-1`](#rfc9915-10-1)

The message compression scheme in Section 4.1.4 of [RFC1035] MUST NOT be used. (§10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-10-1, so no unit is bound to it.

### [`RFC9915-11-1`](#rfc9915-11-1)

Clients and servers MUST treat DUIDs as opaque values and MUST only compare DUIDs for equality. (§11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section1610ReplyValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L209) | unit/verify | revert, verified |

### [`RFC9915-11-2`](#rfc9915-11-2)

Clients and servers MUST NOT restrict DUIDs to the types defined in this document, as additional DUID types may be defined in the future. (§11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section163AdvertiseValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L92) | unit/verify | revert, verified |

### [`RFC9915-11.2-1`](#rfc9915-11.2-1)

The hardware type MUST be a valid hardware type assigned by IANA; see [IANA-HARDWARE-TYPES]. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-11.2-1, so no unit is bound to it.

### [`RFC9915-11.2-2`](#rfc9915-11.2-2)

Clients and servers using this type of DUID MUST store the DUID-LLT in stable storage and MUST continue to use this DUID-LLT even if the network interface used to generate the DUID-LLT is removed. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-11.2-2, so no unit is bound to it.

### [`RFC9915-11.2-3`](#rfc9915-11.2-3)

Clients and servers that do not have any stable storage MUST NOT use this type of DUID. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-11.2-3, so no unit is bound to it.

### [`RFC9915-11.2-4`](#rfc9915-11.2-4)

Clients and servers that use this DUID SHOULD attempt to configure the time prior to generating the DUID, if that is possible, and MUST use some sort of time source (for example, a real-time clock) in generating the DUID, even if that time source could not be configured prior to generating the DUID. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-11.2-4, so no unit is bound to it.

### [`RFC9915-11.2-5`](#rfc9915-11.2-5)

A DHCP client that generates a DUID-LLT using this mechanism MUST provide an administrative interface that replaces the existing DUID with a newly generated DUID-LLT. (§11.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-11.2-5, so no unit is bound to it.

### [`RFC9915-11.3-1`](#rfc9915-11.3-1)

The source of the identifier is left up to the vendor defining it, but each identifier part of each DUID-EN MUST be unique to the device that is using it, and MUST be assigned to the device no later than at the first usage and stored in some form of non-volatile storage. (§11.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-11.3-1, so no unit is bound to it.

### [`RFC9915-11.4-1`](#rfc9915-11.4-1)

The hardware type MUST be a valid hardware type assigned by IANA; see [IANA-HARDWARE-TYPES]. (§11.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1824RenewAtT1CarriesLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L362) | unit/verify | revert, verified |

### [`RFC9915-12-1`](#rfc9915-12-1)

The IAID uniquely identifies the IA and MUST be chosen to be unique among the IAIDs for that IA type on the client (e.g., an IA_NA with an IAID of 0 and an IA_PD with an IAID of 0 are each considered unique). (§12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDHCPv6PDLengthRequestsDelegatedPrefix`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L341) | unit/verify | revert, verified |

### [`RFC9915-12-2`](#rfc9915-12-2)

For any given use of an IA by the client, the IAID for that IA MUST be consistent across restarts of the DHCP client. (§12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section12IAIDStable`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L384) | unit/verify | revert, verified |

### [`RFC9915-14.1-1`](#rfc9915-14.1-1)

A DHCPv6 client MUST limit the rate of DHCP messages it transmits or retransmits. (§14.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section141PerInterfaceRateLimit`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L238) | unit/verify | revert, verified |

### [`RFC9915-14.1-2`](#rfc9915-14.1-2)

For a device that has multiple interfaces, the limit MUST be enforced on a per-interface basis. (§14.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section141LimitIsPerInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L361) | unit/verify | revert, verified |

### [`RFC9915-14.2-1`](#rfc9915-14.2-1)

When T1 and/or T2 values are set to 0, the client MUST choose a time to avoid message storms. (§14.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section142ZeroTimersChosenByClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L779) | unit/verify | revert, verified |

### [`RFC9915-14.2-2`](#rfc9915-14.2-2)

In particular, it MUST NOT transmit immediately. (§14.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section142ZeroTimersChosenByClient`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L780) | unit/verify | revert, verified |

### [`RFC9915-15-1`](#rfc9915-15-1)

The client MUST update an "elapsed-time" value within an Elapsed Time option (see Section 21.9) in the retransmitted message. (§15)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section15ElapsedTimeUpdated`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L61) | unit/verify | revert, verified |

### [`RFC9915-15-2`](#rfc9915-15-2)

Of course, a client MUST listen for a Reconfigure if it has negotiated for its use with the server. (§15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-15-2, so no unit is bound to it.

### [`RFC9915-16-1`](#rfc9915-16-1)

Clients, relay agents, and servers MUST NOT discard messages that contain unknown options (or instances of vendor options with unknown enterprise-number values). (§16)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1610ReplyValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L205) | unit/verify | revert, verified |

### [`RFC9915-16-2`](#rfc9915-16-2)

A client or server MUST discard any received DHCP messages with an unknown message type. (§16)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section1610ReplyValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L206) | unit/verify | revert, verified |

### [`RFC9915-16.1-1`](#rfc9915-16.1-1)

A client MUST leave the transaction ID unchanged in retransmissions of a message. (§16.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section15ElapsedTimeUpdated`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L62) | unit/verify | revert, verified |

### [`RFC9915-16.2-1`](#rfc9915-16.2-1)

Clients MUST discard any received Solicit messages. (§16.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L29) | unit/verify | revert, verified |

### [`RFC9915-16.3-1`](#rfc9915-16.3-1)

Clients MUST discard any received Advertise message that meets any of the following conditions: (§16.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section163AdvertiseValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L88) | unit/verify | revert, verified |

### [`RFC9915-16.4-1`](#rfc9915-16.4-1)

Clients MUST discard any received Request messages. (§16.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L30) | unit/verify | revert, verified |

### [`RFC9915-16.5-1`](#rfc9915-16.5-1)

Clients MUST discard any received Confirm messages. (§16.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L31) | unit/verify | revert, verified |

### [`RFC9915-16.6-1`](#rfc9915-16.6-1)

Clients MUST discard any received Renew messages. (§16.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L32) | unit/verify | revert, verified |

### [`RFC9915-16.7-1`](#rfc9915-16.7-1)

Clients MUST discard any received Rebind messages. (§16.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L33) | unit/verify | revert, verified |

### [`RFC9915-16.8-1`](#rfc9915-16.8-1)

Clients MUST discard any received Decline messages. (§16.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L34) | unit/verify | revert, verified |

### [`RFC9915-16.9-1`](#rfc9915-16.9-1)

Clients MUST discard any received Release messages. (§16.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L35) | unit/verify | revert, verified |

### [`RFC9915-16.10-1`](#rfc9915-16.10-1)

Clients MUST discard any received Reply message that meets any of the following conditions: (§16.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section1610ReplyValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L207) | unit/verify | revert, verified |

### [`RFC9915-16.10-2`](#rfc9915-16.10-2)

If the client included a Client Identifier option (see Section 21.2) in the original message, the Reply message MUST include a Client Identifier option, and the contents of the Client Identifier option MUST match the DUID of the client. (§16.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section1610ReplyValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L208) | unit/verify | revert, verified |

### [`RFC9915-16.10-3`](#rfc9915-16.10-3)

If the client did not include a Client Identifier option in the original message, the Reply message MUST NOT include a Client Identifier option. (§16.10)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-16.10-3, so no unit is bound to it.

### [`RFC9915-16.11-2`](#rfc9915-16.11-2)

Clients MUST discard any Reconfigure message that meets any of the following conditions: (§16.11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L39) | unit/verify | revert, verified |

### [`RFC9915-16.12-1`](#rfc9915-16.12-1)

Clients MUST discard any received Information-request messages. (§16.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L36) | unit/verify | revert, verified |

### [`RFC9915-16.13-1`](#rfc9915-16.13-1)

Clients MUST discard any received Relay-forward messages. (§16.13)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L37) | unit/verify | revert, verified |

### [`RFC9915-16.14-1`](#rfc9915-16.14-1)

Clients and servers MUST discard any received Relay-reply messages. (§16.14)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L38) | unit/verify | revert, verified |

### [`RFC9915-18-1`](#rfc9915-18-1)

The client MUST NOT be using any of the addresses or delegated prefixes for which it tries to obtain the bindings by sending the Solicit message. (§18)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1825ExpiryRemovesAndSolicits`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L486) | unit/verify | revert, verified |

### [`RFC9915-18.2-1`](#rfc9915-18.2-1)

The client SHOULD be suspicious of the Reconfigure message (they may be faked), and it MUST NOT abandon any resources it might have already obtained. (§18.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L42) | unit/verify | revert, verified |

### [`RFC9915-18.2.1-1`](#rfc9915-18.2.1-1)

The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1821SolicitContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L148) | unit/verify | revert, verified |

### [`RFC9915-18.2.1-2`](#rfc9915-18.2.1-2)

The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1821SolicitContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L149) | unit/verify | revert, verified |

### [`RFC9915-18.2.1-3`](#rfc9915-18.2.1-3)

The client MUST include an Option Request option (ORO) (see Section 21.7) to request the SOL_MAX_RT option (see Section 21.24) and any other options the client is interested in receiving. (§18.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1821FirstSolicitRTAboveIRT`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L91) | unit/verify | revert, verified |

### [`RFC9915-18.2.1-4`](#rfc9915-18.2.1-4)

The client MUST NOT include any other options in the Solicit message, except as specifically allowed in the definition of individual options. (§18.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1821SolicitContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L150) | unit/verify | revert, verified |

### [`RFC9915-18.2.1-5`](#rfc9915-18.2.1-5)

Also, the first RT MUST be selected to be strictly greater than IRT by choosing RAND to be strictly greater than 0. (§18.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1821FirstSolicitRTAboveIRT`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L92) | unit/verify | revert, verified |

### [`RFC9915-18.2.1-6`](#rfc9915-18.2.1-6)

A client MUST collect valid Advertise messages for the first RT seconds, unless it receives a valid Advertise message with a preference value of 255. (§18.2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1821AdvertiseSelection`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L276) | unit/verify | revert, verified |

### [`RFC9915-18.2.1-7`](#rfc9915-18.2.1-7)

If the DHCP client is configured with either MRC or MRD set to a value other than 0, it MUST stop trying to configure the interface if the message exchange fails. (§18.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.1-7, so no unit is bound to it.

### [`RFC9915-18.2.2-1`](#rfc9915-18.2.2-1)

The client MUST include the identifier of the destination server in a Server Identifier option (see Section 21.3). (§18.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1822RequestContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L185) | unit/verify | revert, verified |

### [`RFC9915-18.2.2-2`](#rfc9915-18.2.2-2)

The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1822RequestContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L186) | unit/verify | revert, verified |

### [`RFC9915-18.2.2-3`](#rfc9915-18.2.2-3)

The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1822RequestContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L187) | unit/verify | revert, verified |

### [`RFC9915-18.2.2-4`](#rfc9915-18.2.2-4)

The client MUST include an Option Request option (see Section 21.7) to request the SOL_MAX_RT option (see Section 21.24) and any other options the client is interested in receiving. (§18.2.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1822RequestContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L188) | unit/verify | revert, verified |

### [`RFC9915-18.2.3-1`](#rfc9915-18.2.3-1)

The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section18212ConfirmOnMove`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L616) | unit/verify | revert, verified |

### [`RFC9915-18.2.3-2`](#rfc9915-18.2.3-2)

The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1823ConfirmRetransmission`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L725) | unit/verify | revert, verified |

### [`RFC9915-18.2.3-3`](#rfc9915-18.2.3-3)

The first Confirm message from the client on the interface MUST be delayed by a random amount of time between 0 and CNF_MAX_DELAY. (§18.2.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDHCPv6ConfirmStopDuringDelay`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L913) | unit/verify | revert, verified |
| positive | [`TestRFC9915Section18212ConfirmOnMove`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L617) | unit/verify | revert, verified |

### [`RFC9915-18.2.4-1`](#rfc9915-18.2.4-1)

However, as the client SHOULD renew/ rebind all IAs from the server at the same time, the client MUST select T1 and T2 times from all IA options that will guarantee that the client initiates transmissions of Renew/Rebind messages not later than at the T1/T2 times associated with any of the client's bindings (earliest T1/T2). (§18.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1824EarliestT1AcrossIAs`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L804) | unit/verify | revert, verified |

### [`RFC9915-18.2.4-2`](#rfc9915-18.2.4-2)

A client MUST also initiate a Renew/Reply message exchange before time T1 if the client's link-local address used in previous interactions with the server is no longer valid and it is willing to receive Reconfigure messages. (§18.2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.4-2, so no unit is bound to it.

### [`RFC9915-18.2.4-4`](#rfc9915-18.2.4-4)

The client MUST include a Server Identifier option (see Section 21.3) in the Renew message, identifying the server with which the client most recently communicated. (§18.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1824RenewAtT1CarriesLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L358) | unit/verify | revert, verified |

### [`RFC9915-18.2.4-5`](#rfc9915-18.2.4-5)

The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1824RenewAtT1CarriesLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L359) | unit/verify | revert, verified |

### [`RFC9915-18.2.4-6`](#rfc9915-18.2.4-6)

The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1824RenewAtT1CarriesLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L360) | unit/verify | revert, verified |

### [`RFC9915-18.2.4-7`](#rfc9915-18.2.4-7)

The client MUST NOT include addresses and prefixes in any IA option that the client did not obtain from the server or that are no longer valid (that have a valid lifetime of 0). (§18.2.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section1824RenewOmitsExpiredLeases`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L309) | unit/verify | revert, verified |

### [`RFC9915-18.2.6-1`](#rfc9915-18.2.6-1)

The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.6-1, so no unit is bound to it.

### [`RFC9915-18.2.6-2`](#rfc9915-18.2.6-2)

The client MUST include an Option Request option (see Section 21.7) to request the INF_MAX_RT option (see Section 21.25), the Information Refresh Time option (see Section 21.23), and any other options the client is interested in receiving. (§18.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.6-2, so no unit is bound to it.

### [`RFC9915-18.2.6-3`](#rfc9915-18.2.6-3)

When responding to a Reconfigure, the client MUST include a Server Identifier option (see Section 21.3) with the identifier from the Reconfigure message to which the client is responding. (§18.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.6-3, so no unit is bound to it.

### [`RFC9915-18.2.6-4`](#rfc9915-18.2.6-4)

The first Information-request message from the client on the interface MUST be delayed by a random amount of time between 0 and INF_MAX_DELAY. (§18.2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.6-4, so no unit is bound to it.

### [`RFC9915-18.2.7-1`](#rfc9915-18.2.7-1)

The client MUST include a Server Identifier option (see Section 21.3) in the Renew message, identifying the server that allocated the lease(s). (§18.2.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1827ReleaseContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L276) | unit/verify | revert, verified |

### [`RFC9915-18.2.7-2`](#rfc9915-18.2.7-2)

The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1827ReleaseContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L277) | unit/verify | revert, verified |

### [`RFC9915-18.2.7-3`](#rfc9915-18.2.7-3)

The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1827ReleaseContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L278) | unit/verify | revert, verified |

### [`RFC9915-18.2.7-4`](#rfc9915-18.2.7-4)

The leases to be released MUST be included in the IAs. (§18.2.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1827ReleaseContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L279) | unit/verify | revert, verified |

### [`RFC9915-18.2.7-5`](#rfc9915-18.2.7-5)

Any leases for the IAs the client wishes to continue to use MUST NOT be added to the IAs. (§18.2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.7-5, so no unit is bound to it.

### [`RFC9915-18.2.7-6`](#rfc9915-18.2.7-6)

The client MUST stop using all of the leases being released before the client begins the Release message exchange process. (§18.2.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDHCPv6ReleaseOnStop`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L646) | unit/verify | revert, verified |

### [`RFC9915-18.2.7-7`](#rfc9915-18.2.7-7)

For an address, this means the address MUST have been removed from the interface. (§18.2.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDHCPv6ReleaseOnStop`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L647) | unit/verify | revert, verified |

### [`RFC9915-18.2.7-8`](#rfc9915-18.2.7-8)

For a delegated prefix, this means the prefix MUST have been advertised with a Preferred Lifetime and a Valid Lifetime of 0 in a Router Advertisement message as described in part (e) of Section 5.5.3 of [RFC4862]; also see requirement L-13 in Section 4.3 of [RFC7084]. (§18.2.7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.7-8, so no unit is bound to it.

### [`RFC9915-18.2.8-1`](#rfc9915-18.2.8-1)

The Decline message is not used in prefix delegation; thus, the client MUST NOT include IA_PD options (see Section 21.21) in the Decline message. (§18.2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L98) | unit/verify | revert, verified |

### [`RFC9915-18.2.8-2`](#rfc9915-18.2.8-2)

The client MUST include a Server Identifier option (see Section 21.3) in the Decline message, identifying the server that allocated the lease(s). (§18.2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L99) | unit/verify | revert, verified |

### [`RFC9915-18.2.8-3`](#rfc9915-18.2.8-3)

The client MUST include a Client Identifier option (see Section 21.2) to identify itself to the server. (§18.2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L100) | unit/verify | revert, verified |

### [`RFC9915-18.2.8-4`](#rfc9915-18.2.8-4)

The client MUST include an Elapsed Time option (see Section 21.9) to indicate how long the client has been trying to complete the current DHCP message exchange. (§18.2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L101) | unit/verify | revert, verified |

### [`RFC9915-18.2.8-5`](#rfc9915-18.2.8-5)

The addresses to be declined MUST be included in the IAs. (§18.2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L102) | unit/verify | revert, verified |

### [`RFC9915-18.2.8-6`](#rfc9915-18.2.8-6)

The client MUST NOT use any of the addresses it is declining as the source address in the Decline message or in any subsequently transmitted message. (§18.2.8)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915DuplicateAddressDeclinedOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc9915_dhcp_v6_dad_integration_linux_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L103) | unit/verify | revert, verified |

### [`RFC9915-18.2.9-1`](#rfc9915-18.2.9-1)

The client MUST process any SOL_MAX_RT option (§18.2.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section163AdvertiseValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L90) | unit/verify | revert, verified |

### [`RFC9915-18.2.9-2`](#rfc9915-18.2.9-2)

The client MUST ignore any Advertise message that contains no addresses (IA Address options (see Section 21.6) encapsulated in IA_NA options (see Section 21.4)) and no delegated prefixes (IA Prefix options (see Section 21.22) encapsulated in IA_PD options (see Section 21.21)), with the exception that the client: (§18.2.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section163AdvertiseValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L89) | unit/verify | revert, verified |

### [`RFC9915-18.2.9-3`](#rfc9915-18.2.9-3)

MUST process an included SOL_MAX_RT option and (§18.2.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section163AdvertiseValidation`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L91) | unit/verify | revert, verified |

### [`RFC9915-18.2.9-4`](#rfc9915-18.2.9-4)

MUST process an included INF_MAX_RT option. (§18.2.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.9-4, so no unit is bound to it.

### [`RFC9915-18.2.9-5`](#rfc9915-18.2.9-5)

The client ignoring an Advertise message MUST NOT restart the Solicit retransmission timer. (§18.2.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDHCPv6LinkOneDatagramPerExchange`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_link_linux_test.go#L109) | unit/verify | revert, verified |

### [`RFC9915-18.2.10-1`](#rfc9915-18.2.10-1)

The client MUST process any SOL_MAX_RT option (§18.2.10)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section2124SolMaxRTRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L181) | unit/verify | revert, verified |

### [`RFC9915-18.2.10.1-1`](#rfc9915-18.2.10.1-1)

The client MUST perform duplicate address detection as per Section 5.4 of [RFC4862], which does list some exceptions, on each of the received addresses in any IAs on which it has not performed duplicate address detection during processing of any of the previous Reply messages from the server. (§18.2.10.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section182101UniqueAddressKept`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L183) | unit/verify | revert, verified |
| positive | [`TestRFC9915DuplicateAddressDeclinedOnTheLink`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc9915_dhcp_v6_dad_integration_linux_test.go#L35) | unit/verify | revert, verified |
| positive | [`TestRFC9915Section182101DuplicateAddressDeclined`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L97) | unit/verify | revert, verified |

### [`RFC9915-18.2.10.1-3`](#rfc9915-18.2.10.1-3)

Addresses obtained from an IA Address option MUST NOT be used to form an implicit prefix with a length other than 128. (§18.2.10.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDHCPv6LeaseHeldAcrossFiveRenewals`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L414) | unit/verify | revert, verified |

### [`RFC9915-18.2.10.5-1`](#rfc9915-18.2.10.5-1)

As a counterexample, a client that previously received an NTP server address from the DHCP server and does not receive it anymore MUST stop using the configured NTP server address. (§18.2.10.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.10.5-1, so no unit is bound to it.

### [`RFC9915-18.2.11-1`](#rfc9915-18.2.11-1)

The message MUST be dropped if it doesn't pass the validation, as explained in Section 16.11, particularly in cases where the authentication is missing or fails. (§18.2.11)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L40) | unit/verify | revert, verified |

### [`RFC9915-18.2.11-2`](#rfc9915-18.2.11-2)

Once the client has received a Reconfigure, the client proceeds with the message exchange (retransmitting the Renew, Rebind, or Information-request message if necessary); the client MUST ignore any additional Reconfigure messages until the exchange is complete. (§18.2.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.11-2, so no unit is bound to it.

### [`RFC9915-18.2.12-1`](#rfc9915-18.2.12-1)

The client MUST include any IAs assigned to the interface that may have moved to a new link, along with the addresses associated with those IAs, in its Confirm message. (§18.2.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section18212NoMoveNoConfirm`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L694) | unit/verify | revert, verified |
| positive | [`TestRFC9915Section18212ConfirmOnMove`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L618) | unit/verify | revert, verified |

### [`RFC9915-18.2.12-2`](#rfc9915-18.2.12-2)

If the client has any valid delegated prefixes obtained from the DHCP server, the client MUST initiate a Rebind/Reply message exchange as described in Section 18.2.5, with the exception that the retransmission parameters should be set as for the Confirm message (see Section 18.2.3). (§18.2.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915LinkChangeWithoutDownSendsNothing`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc9915_dhcp_v6_link_move_integration_linux_test.go#L96) | unit/verify | mutant, verified |
| negative | [`TestRFC9915Section18212NoMoveNoRebind`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L446) | unit/verify | revert, verified |
| positive | [`TestRFC9915LinkMoveRebindsThroughTheMonitor`](https://github.com/ze-software/ze/blob/main/internal/component/iface/rfc9915_dhcp_v6_link_move_integration_linux_test.go#L49) | unit/verify | revert, verified |
| positive | [`TestRFC9915Section18212MoveRebindsWithConfirmParameters`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L398) | unit/verify | revert, verified |

### [`RFC9915-18.2.12-3`](#rfc9915-18.2.12-3)

If the client has only obtained network information using Information-request/Reply message exchanges, the client MUST initiate an Information-request/Reply message exchange as described in Section 18.2.6. (§18.2.12)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-18.2.12-3, so no unit is bound to it.

### [`RFC9915-18.2.12-4`](#rfc9915-18.2.12-4)

However, a client MUST rate- limit such initiation attempts to avoid flooding a server with requests when there are link issues (for example, only doing one of these at most every 30 seconds). (§18.2.12)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section18212MoveRebindRateLimited`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_attach_linux_test.go#L492) | unit/verify | revert, verified |

### [`RFC9915-18.3.5-1`](#rfc9915-18.3.5-1)

The Reply with lifetimes of 0 constitutes an explicit notification to the client that the specific addresses and delegated prefixes are no longer valid and MUST NOT be used by the client. (§18.3.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section182101ValidZeroDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L731) | unit/verify | revert, verified |

### [`RFC9915-20.2-1`](#rfc9915-20.2-1)

Any DHCP message MUST NOT include more than one Authentication option. (§20.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-20.2-1, so no unit is bound to it.

### [`RFC9915-20.3-3`](#rfc9915-20.3-3)

A client that receives a message with the RDM field set to 0x00 MUST compare its replay detection field with the previous value sent by that same server (based on the Server Identifier option; see Section 21.3) and only accept the message if the received value is greater and record this as the new value. (§20.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-20.3-3, so no unit is bound to it.

### [`RFC9915-20.3-4`](#rfc9915-20.3-4)

If this is the first time a client processes an Authentication option sent by a server, the client MUST record the replay detection value and skip the replay detection check. (§20.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-20.3-4, so no unit is bound to it.

### [`RFC9915-21-1`](#rfc9915-21-1)

If an option does appear multiple times, each instance is considered separate and the data areas of the options MUST NOT be concatenated or otherwise combined. (§21)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21-1, so no unit is bound to it.

### [`RFC9915-21.4-2`](#rfc9915-21.4-2)

In a message sent by a server to a client, the client MUST use the values in the T1 and T2 fields for the T1 and T2 times, unless values in those fields are 0. (§21.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1824RenewAtT1CarriesLease`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L361) | unit/verify | revert, verified |

### [`RFC9915-21.6-3`](#rfc9915-21.6-3)

In a message sent by a server to a client, the client MUST use the values in the preferred-lifetime and valid-lifetime fields for the preferred and valid lifetimes. (§21.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestDHCPv6LeaseHeldAcrossFiveRenewals`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L415) | unit/verify | revert, verified |

### [`RFC9915-21.6-4`](#rfc9915-21.6-4)

The client MUST discard any addresses for which the preferred lifetime is greater than the valid lifetime. (§21.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section216PreferredAboveValidDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_lease_linux_test.go#L749) | unit/verify | revert, verified |

### [`RFC9915-21.7-1`](#rfc9915-21.7-1)

A client MUST include an Option Request option in a Solicit, Request, Renew, Rebind, or Information-request message to inform the server about options the client wants the server to send to the client. (§21.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section217OptionRequestEveryMessage`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L223) | unit/verify | revert, verified |

### [`RFC9915-21.7-2`](#rfc9915-21.7-2)

The Option Request option MUST NOT include the following option codes: (§21.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section217OptionRequestEveryMessage`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L224) | unit/verify | revert, verified |

### [`RFC9915-21.9-1`](#rfc9915-21.9-1)

A client MUST include an Elapsed Time option in messages to indicate how long the client has been trying to complete a DHCP message exchange. (§21.9)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section217OptionRequestEveryMessage`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L226) | unit/verify | revert, verified |

### [`RFC9915-21.15-1`](#rfc9915-21.15-1)

The user class information carried in this option MUST be configurable on the client. (§21.15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.15-1, so no unit is bound to it.

### [`RFC9915-21.15-2`](#rfc9915-21.15-2)

The data area of the User Class option MUST contain one or more instances of user-class-data information. (§21.15)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.15-2, so no unit is bound to it.

### [`RFC9915-21.16-1`](#rfc9915-21.16-1)

Servers and clients MUST NOT include more than one instance of OPTION_VENDOR_CLASS with the same Enterprise Number. (§21.16)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.16-1, so no unit is bound to it.

### [`RFC9915-21.17-1`](#rfc9915-21.17-1)

The vendor-option-data field MUST be encoded as a sequence of code/length/value fields of format identical to the DHCP options (see Section 21.1). (§21.17)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.17-1, so no unit is bound to it.

### [`RFC9915-21.17-2`](#rfc9915-21.17-2)

Servers and clients MUST NOT send more than one instance of the Vendor-specific Information option with the same Enterprise Number. (§21.17)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.17-2, so no unit is bound to it.

### [`RFC9915-21.17-3`](#rfc9915-21.17-3)

MUST specify the Vendor-specific Information option in an Option Request option. (§21.17)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.17-3, so no unit is bound to it.

### [`RFC9915-21.22-2`](#rfc9915-21.22-2)

The client MUST discard any prefixes for which the preferred lifetime is greater than the valid lifetime. (§21.22)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section2122PrefixPreferredAboveValidDiscarded`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L335) | unit/verify | revert, verified |

### [`RFC9915-21.23-1`](#rfc9915-21.23-1)

A DHCP client MUST request this option in the Option Request option (see Section 21.7) when sending Information-request messages. (§21.23)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.23-1, so no unit is bound to it.

### [`RFC9915-21.23-2`](#rfc9915-21.23-2)

A client MUST NOT request this option in the Option Request option in any other messages. (§21.23)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section217OptionRequestEveryMessage`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L225) | unit/verify | revert, verified |

### [`RFC9915-21.23-5`](#rfc9915-21.23-5)

If the Reply to an Information-request message does not contain this option, the client MUST behave as if the option with the value IRT_DEFAULT was provided. (§21.23)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.23-5, so no unit is bound to it.

### [`RFC9915-21.23-6`](#rfc9915-21.23-6)

A client MUST use the refresh time IRT_MINIMUM if it receives the option with a value less than IRT_MINIMUM. (§21.23)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.23-6, so no unit is bound to it.

### [`RFC9915-21.23-7`](#rfc9915-21.23-7)

When the client detects that the refresh time has expired, it SHOULD try to update its configuration data by sending an Information- request as specified in Section 18.2.6, except that the client MUST delay sending the first Information-request by a random amount of time between 0 and INF_MAX_DELAY. (§21.23)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.23-7, so no unit is bound to it.

### [`RFC9915-21.24-3`](#rfc9915-21.24-3)

A DHCP client MUST ignore any SOL_MAX_RT option values that are less than 60 or more than 86400. (§21.24)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section2124SolMaxRTRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L179) | unit/verify | revert, verified |

### [`RFC9915-21.24-4`](#rfc9915-21.24-4)

If a DHCP client receives a message containing a SOL_MAX_RT option that has a valid value for SOL_MAX_RT, the client MUST set its internal SOL_MAX_RT parameter to the value contained in the SOL_MAX_RT option. (§21.24)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section2124SolMaxRTRange`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_exchange_linux_test.go#L180) | unit/verify | revert, verified |

### [`RFC9915-21.25-2`](#rfc9915-21.25-2)

A DHCP client MUST include the INF_MAX_RT option code in any Option Request option (see Section 21.7) it sends in an Information-request message. (§21.25)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.25-2, so no unit is bound to it.

### [`RFC9915-21.25-3`](#rfc9915-21.25-3)

A DHCP client MUST ignore any INF_MAX_RT option values that are less than 60 or more than 86400. (§21.25)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.25-3, so no unit is bound to it.

### [`RFC9915-21.25-4`](#rfc9915-21.25-4)

If a DHCP client receives a message containing an INF_MAX_RT option that has a valid value for INF_MAX_RT, the client MUST set its internal INF_MAX_RT parameter to the value contained in the INF_MAX_RT option. (§21.25)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9915-21.25-4, so no unit is bound to it.

### [`RFC9915-22.3-1`](#rfc9915-22.3-1)

Because of the opportunity for attack through the Reconfigure message, a DHCP client MUST discard any Reconfigure message that does not include authentication or that does not pass the validation process for the authentication protocol. (§22.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9915Section16ClientDiscardsOtherMessageTypes`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L41) | unit/verify | revert, verified |

### [`RFC9915-A-1`](#rfc9915-a-1)

Legacy implementations MAY support them, but implementations conformant to this document MUST NOT rely on them. (§A)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9915Section1821SolicitContents`](https://github.com/ze-software/ze/blob/main/internal/plugins/iface/dhcp/rfc9915_dhcp_v6_messages_linux_test.go#L151) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-19 |
| Register | rfc2119 |
| Source | rfc/full/rfc9915.txt |
| Source fingerprint | b882e4301dde4616 |
| Record | rfc/extraction/rfc9915.json |
| Mapped sentences | 126 |
| Declined as scope | 92 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 0 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.2` | not stated | 0 | walked | not stated |
| `6.3` | not stated | 3 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 0 | walked | not stated |
| `6.6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 5 | walked | not stated |
| `7.3` | not stated | 0 | walked | not stated |
| `7.4` | not stated | 0 | walked | not stated |
| `7.5` | not stated | 0 | walked | not stated |
| `7.6` | not stated | 0 | walked | not stated |
| `7.7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 1 | walked | not stated |
| `9.2` | not stated | 1 | walked | not stated |
| `10` | not stated | 1 | walked | not stated |
| `11` | not stated | 2 | walked | not stated |
| `11.1` | not stated | 0 | walked | not stated |
| `11.2` | not stated | 5 | walked | not stated |
| `11.3` | not stated | 1 | walked | not stated |
| `11.4` | not stated | 1 | walked | not stated |
| `11.5` | not stated | 0 | walked | not stated |
| `12` | not stated | 2 | walked | not stated |
| `12.1` | not stated | 0 | walked | not stated |
| `12.2` | not stated | 0 | walked | not stated |
| `13` | not stated | 0 | walked | not stated |
| `13.1` | not stated | 3 | walked | not stated |
| `13.2` | not stated | 0 | walked | not stated |
| `14` | not stated | 0 | walked | not stated |
| `14.1` | not stated | 2 | walked | not stated |
| `14.2` | not stated | 3 | walked | not stated |
| `15` | not stated | 2 | walked | not stated |
| `16` | not stated | 2 | walked | not stated |
| `16.1` | not stated | 1 | walked | not stated |
| `16.2` | not stated | 2 | walked | not stated |
| `16.3` | not stated | 2 | walked | not stated |
| `16.4` | not stated | 2 | walked | not stated |
| `16.5` | not stated | 2 | walked | not stated |
| `16.6` | not stated | 2 | walked | not stated |
| `16.7` | not stated | 2 | walked | not stated |
| `16.8` | not stated | 2 | walked | not stated |
| `16.9` | not stated | 2 | walked | not stated |
| `16.10` | not stated | 4 | walked | not stated |
| `16.11` | not stated | 2 | walked | not stated |
| `16.12` | not stated | 2 | walked | not stated |
| `16.13` | not stated | 1 | walked | not stated |
| `16.14` | not stated | 1 | walked | not stated |
| `17` | not stated | 0 | walked | not stated |
| `17.1` | not stated | 0 | walked | not stated |
| `17.2` | not stated | 0 | walked | not stated |
| `18` | not stated | 2 | walked | not stated |
| `18.1` | not stated | 2 | walked | not stated |
| `18.2` | not stated | 1 | walked | not stated |
| `18.2.1` | not stated | 7 | walked | not stated |
| `18.2.2` | not stated | 4 | walked | not stated |
| `18.2.3` | not stated | 3 | walked | not stated |
| `18.2.4` | not stated | 7 | walked | not stated |
| `18.2.5` | not stated | 0 | walked | not stated |
| `18.2.6` | not stated | 4 | walked | not stated |
| `18.2.7` | not stated | 9 | walked | not stated |
| `18.2.8` | not stated | 6 | walked | not stated |
| `18.2.9` | not stated | 5 | walked | not stated |
| `18.2.10` | not stated | 2 | walked | not stated |
| `18.2.10.1` | not stated | 6 | walked | not stated |
| `18.2.10.2` | not stated | 0 | walked | not stated |
| `18.2.10.3` | not stated | 0 | walked | not stated |
| `18.2.10.4` | not stated | 0 | walked | not stated |
| `18.2.10.5` | not stated | 1 | walked | not stated |
| `18.2.11` | not stated | 2 | walked | not stated |
| `18.2.12` | not stated | 4 | walked | not stated |
| `18.2.13` | not stated | 0 | walked | not stated |
| `18.3` | not stated | 2 | walked | not stated |
| `18.3.1` | not stated | 1 | walked | not stated |
| `18.3.2` | not stated | 8 | walked | not stated |
| `18.3.3` | not stated | 2 | walked | not stated |
| `18.3.4` | not stated | 3 | walked | not stated |
| `18.3.5` | not stated | 5 | walked | not stated |
| `18.3.6` | not stated | 1 | walked | not stated |
| `18.3.7` | not stated | 0 | walked | not stated |
| `18.3.8` | not stated | 0 | walked | not stated |
| `18.3.9` | not stated | 7 | walked | not stated |
| `18.3.10` | not stated | 1 | walked | not stated |
| `18.3.11` | not stated | 3 | walked | not stated |
| `19` | not stated | 2 | walked | not stated |
| `19.1` | not stated | 0 | walked | not stated |
| `19.1.1` | not stated | 1 | walked | not stated |
| `19.1.2` | not stated | 0 | walked | not stated |
| `19.1.3` | not stated | 0 | walked | not stated |
| `19.2` | not stated | 2 | walked | not stated |
| `19.3` | not stated | 1 | walked | not stated |
| `19.4` | not stated | 1 | walked | not stated |
| `20` | not stated | 0 | walked | not stated |
| `20.1` | not stated | 0 | walked | not stated |
| `20.2` | not stated | 1 | walked | not stated |
| `20.3` | not stated | 5 | walked | not stated |
| `20.4` | not stated | 0 | walked | not stated |
| `20.4.1` | not stated | 0 | walked | not stated |
| `20.4.2` | not stated | 1 | walked | not stated |
| `20.4.3` | not stated | 0 | walked | not stated |
| `21` | not stated | 1 | walked | not stated |
| `21.1` | not stated | 0 | walked | not stated |
| `21.2` | not stated | 0 | walked | not stated |
| `21.3` | not stated | 0 | walked | not stated |
| `21.4` | not stated | 3 | walked | not stated |
| `21.5` | not stated | 0 | walked | not stated |
| `21.6` | not stated | 4 | walked | not stated |
| `21.7` | not stated | 3 | walked | not stated |
| `21.8` | not stated | 0 | walked | not stated |
| `21.9` | not stated | 1 | walked | not stated |
| `21.10` | not stated | 0 | walked | not stated |
| `21.11` | not stated | 0 | walked | not stated |
| `21.12` | not stated | 1 | walked | not stated |
| `21.13` | not stated | 1 | walked | not stated |
| `21.14` | not stated | 1 | walked | not stated |
| `21.15` | not stated | 2 | walked | not stated |
| `21.16` | not stated | 1 | walked | not stated |
| `21.17` | not stated | 3 | walked | not stated |
| `21.18` | not stated | 2 | walked | not stated |
| `21.19` | not stated | 0 | walked | not stated |
| `21.20` | not stated | 0 | walked | not stated |
| `21.21` | not stated | 3 | walked | not stated |
| `21.22` | not stated | 2 | walked | not stated |
| `21.23` | not stated | 7 | walked | not stated |
| `21.24` | not stated | 4 | walked | not stated |
| `21.25` | not stated | 4 | walked | not stated |
| `22` | not stated | 0 | walked | not stated |
| `22.1` | not stated | 0 | walked | not stated |
| `22.2` | not stated | 0 | walked | not stated |
| `22.3` | not stated | 1 | walked | not stated |
| `22.4` | not stated | 0 | walked | not stated |
| `23` | not stated | 0 | walked | not stated |
| `24` | not stated | 0 | walked | not stated |
| `25` | not stated | 0 | walked | not stated |
| `25.1` | not stated | 0 | walked | not stated |
| `25.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 1 | walked | not stated |
| `B` | not stated | 0 | walked | not stated |
| `C` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `7.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | Servers and relay agents MUST listen for DHCP messages on UDP port 547. |
| `7.2:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | Servers MUST send Relay-reply messages to UDP destination port 547 and client messages to UDP destination port 546. |
| `7.2:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 relay agent. | Relay agents MUST send Relay-forward and Relay-reply messages to UDP destination port 547 and client messages to UDP destination port 546. |
| `9.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 relay agent. | options: MUST include a Relay Message option (see Section 21.10); MAY include other options, such as the Interface-Id option (see Section 21.18), added by the relay agent. |
| `9.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 relay agent. | options: MUST include a Relay Message option (see Section 21.10); MAY include other options, such as the Interface-Id option (see Section 21.18). |
| `13.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | According to [RFC6221], the server MUST ignore any link-address field whose value is zero. |
| `13.1:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | A server MUST NOT assign an address that is otherwise reserved for some other purpose. |
| `13.1:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | For example, a server MUST NOT assign addresses that use a reserved IPv6 Interface Identifier [RFC5453] [RFC7136] [IANA-RESERVED-IID]. |
| `14.2:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-14.1-1: this sentence applies the section 14.1 limit to the renewal schedule; every Renew and Rebind passes the same per-interface limiter (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::admitTransmission). | The client MUST choose renew and rebind times to not violate rate-limiting restrictions as defined in Section 14.1. |
| `16.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | Servers MUST discard any Solicit messages that do not include a Client Identifier option or that do include a Server Identifier option. |
| `16.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server and relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server and relay agent. | Servers and relay agents MUST discard any received Advertise messages. |
| `16.4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | Servers MUST discard any received Request message that meets any of the following conditions: |
| `16.5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | Servers MUST discard any received Confirm messages that do not include a Client Identifier option (see Section 21.2) or that do include a Server Identifier option (see Section 21.3). |
| `16.6:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | Servers MUST discard any received Renew message that meets any of the following conditions: |
| `16.7:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | Servers MUST discard any received Rebind messages that do not include a Client Identifier option (see Section 21.2) or that do include a Server Identifier option (see Section 21.3). |
| `16.8:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | Servers MUST discard any received Decline message that meets any of the following conditions: |
| `16.9:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | Servers MUST discard any received Release message that meets any of the following conditions: |
| `16.10:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server and relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server and relay agent. | Servers and relay agents MUST discard any received Reply messages. |
| `16.11:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server and relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server and relay agent. | Servers and relay agents MUST discard any received Reconfigure messages. |
| `16.12:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | Servers MUST discard any received Information-request message that meets any of the following conditions: |
| `18:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-18-1: this sentence is the case of the same prohibition where bindings were valid; ze starts a Solicit only once every lease has expired (internal/plugins/iface/dhcp/dhcp_v6_linux.go::expireDue). | In particular, if the client had some valid bindings and has chosen to start the server discovery process to obtain the same bindings from a different server, the client MUST stop using the addresses and delegated prefixes for the bindings that it had obtained from the previous server (see Section 18.2.7 for more details on what "stop using" means in this context) and that it is now trying to obtain from a new server. |
| `18.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | To facilitate a client's use of a single transaction for all IA options, servers MUST return the same T1/T2 values for all IA options in a Reply (see Sections 18.3.2, 18.3.4, and 18.3.5) so that the client will generate a single transaction when renewing or rebinding its leases. |
| `18.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-18.2.4-1: this sentence names the section 18.2.4 rule that row RFC9915-18.2.4-1 carries. | However, because some servers may not yet conform to this requirement, a client MUST be prepared to select appropriate T1/T2 times as described in Section 18.2.4. |
| `18.2.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-14.2-1: this sentence names the section 14.2 rule that rows RFC9915-14.2-1 and RFC9915-14.2-2 carry. | The client MUST follow the rules defined in Section 14.2. |
| `18.2.7:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-18.2.7-7: this sentence follows from it: internal/plugins/iface/dhcp/dhcp_v6_linux.go::shutdown removes every released address from the interface before the Release socket opens, so a released address cannot be a source. | The client MUST NOT use any of the addresses it is releasing as the source address in the Release message or in any subsequently transmitted message. |
| `18.2.10:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-14.1-1: this sentence applies the section 14.1 limit to a retried exchange; every retransmission passes the same per-interface limiter (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::admitTransmission). | If the client retransmits the original message to the same server to retry the desired operation, the client MUST limit the rate at which it retransmits the message and limit the duration of the time during which it retransmits the message (see Section 14.1). |
| `18.2.10.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-18.2.10.1-3: this sentence is met by the same /128 installation: an address installed as a /128 carries no on-link prefix. | * For each assigned address that does not have any associated reachability information (see the definition of "on-link" in Section 2.1 of [RFC4861]), in order to avoid the problems described in [RFC4943], the client MUST NOT assume that any addresses are reachable on-link as a result of receiving an IA Address option (see Section 21.6). |
| `18.2.10.1:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-6.3-1: this sentence repeats section 6.3 word for word. | If the client uses a delegated prefix to configure addresses on interfaces on itself or other nodes behind it, the preferred and valid lifetimes of those addresses MUST be no longer than the remaining preferred and valid lifetimes, respectively, for the delegated prefix at any time. |
| `18.2.10.1:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-6.3-2: this sentence repeats section 6.3 word for word. | In particular, if the delegated prefix or a prefix derived from it is advertised for stateless address autoconfiguration [RFC4862], the advertised preferred and valid lifetimes MUST NOT exceed the corresponding remaining lifetimes of the delegated prefix. |
| `18.2.10.1:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-14.1-1: this sentence applies the section 14.1 limit to the UseMulticast retry; every retransmission passes the same per-interface limiter (internal/plugins/iface/dhcp/dhcp_v6_exchange_linux.go::admitTransmission). | Therefore, the client MUST rate-limit its transmissions (see Section 14.1) and MAY just wait for the normal retransmission time (as if the Reply message had not been received). |
| `18.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | These Advertise and Reply messages MUST always contain the Server Identifier option (see Section 21.3) containing the server's DUID and the Client Identifier option (see Section 21.2) from the client message if one was present. |
| `18.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST use the received message to construct the proper Relay-reply message to allow the response to the received message to be relayed through the same relay agents (in reverse order) as the original client message; see Section 19.3 for more details. |
| `18.3.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST commit the assignment of any addresses and delegated prefixes or other configuration information before sending a Reply message to a client. |
| `18.3.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST include in the Reply message a Server Identifier option (see Section 21.3) containing the server's DUID and the Client Identifier option (see Section 21.2) from the Request message. |
| `18.3.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | If any of the prefixes of the included addresses are not appropriate for the link to which the client is connected, the server MUST return the IA to the client with a Status Code option (see Section 21.13) with the value NotOnLink. |
| `18.3.2:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | If the server does not send the NotOnLink status code but it cannot assign any IP addresses to an IA, the server MUST return the IA option in the Reply message with no addresses in the IA and a Status Code option containing status code NoAddrsAvail in the IA. |
| `18.3.2:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | For any IA_PD option (see Section 21.21) in the Request message to which the server cannot assign any delegated prefixes, the server MUST return the IA_PD option in the Reply message with no prefixes in the IA_PD and with a Status Code option containing status code NoPrefixAvail in the IA_PD. |
| `18.3.2:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST NOT include any addresses or delegated prefixes in the IA that the server does not assign to the client. |
| `18.3.2:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The T1/T2 times set in each applicable IA option for a Reply MUST be the same values across all IAs. |
| `18.3.2:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST determine the T1/T2 times across all of the applicable client's bindings in the Reply. |
| `18.3.2:8` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST NOT cache its responses. |
| `18.3.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | If the server is unable to perform this test (for example, the server does not have information about prefixes on the link to which the client is connected) or there were no addresses in any of the IAs sent by the client, the server MUST NOT send a Reply to the client. |
| `18.3.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST include in the Reply message a Server Identifier option (see Section 21.3) containing the server's DUID and the Client Identifier option (see Section 21.2) from the Confirm message. |
| `18.3.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST include in the Reply message a Server Identifier option (see Section 21.3) containing the server's DUID and the Client Identifier option (see Section 21.2) from the Renew message. |
| `18.3.4:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The T1/T2 values set in each applicable IA option for a Reply MUST be the same across all IAs. |
| `18.3.4:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST determine the T1/T2 values across all of the applicable client's bindings in the Reply. |
| `18.3.5:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST NOT return any addresses or delegated prefixes in the IA that the server does not assign to the client. |
| `18.3.5:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST include in the Reply message a Server Identifier option (see Section 21.3) containing the server's DUID and the Client Identifier option (see Section 21.2) from the Rebind message. |
| `18.3.5:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The T1 or T2 values set in each applicable IA option for a Reply MUST be the same values across all IAs. |
| `18.3.5:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST determine the T1 or T2 values across all of the applicable client's bindings in the Reply. |
| `18.3.6:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST include a Server Identifier option (see Section 21.3) containing the server's DUID in the Reply message. |
| `18.3.9:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server preference value MUST default to 0 unless otherwise configured by the server administrator. |
| `18.3.9:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST include options in the Advertise message containing configuration parameters for all of the options identified in the Option Request option (see Section 21.7) in the Solicit message that the server has been configured to return to the client. |
| `18.3.9:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | If the Option Request option includes a container option, the server MUST include all the options that are eligible to be encapsulated in the container. |
| `18.3.9:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST include IA options in the Advertise message containing any addresses and/or delegated prefixes that would be assigned to IAs contained in the Solicit message from the client. |
| `18.3.9:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | If the server is not going to assign an address or delegated prefix received as a hint in the Solicit message, the server MUST NOT include this address or delegated prefix in the Advertise message. |
| `18.3.9:6` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | If the server will not assign any addresses to an IA_NA in subsequent Request messages from the client, the server MUST include the IA option in the Advertise message with no addresses in that IA and a Status Code option (see Section 21.13) encapsulated in the IA option containing status code NoAddrsAvail. |
| `18.3.9:7` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | If the server will not assign any prefixes to an IA_PD in subsequent Request messages from the client, the server MUST include the IA_PD option (see Section 21.21) in the Advertise message with no prefixes in the IA_PD option and a Status Code option encapsulated in the IA_PD containing status code NoPrefixAvail. |
| `18.3.10:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The Advertise or Reply message MUST be unicast through the interface on which the original message was received. |
| `18.3.11:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST use DHCP authentication in the Reconfigure message (see Section 20.4). |
| `18.3.11:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST include a Reconfigure Message option (see Section 21.19) to select whether the client responds with a Renew message, a Rebind message, or an Information-request message. |
| `18.3.11:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST NOT include any other options in the Reconfigure message, except as specifically allowed in the definition of individual options. |
| `19:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 relay agent. | If the relay agent has not been explicitly configured, it MUST use the All_DHCP_Servers multicast address as the default. |
| `19:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 relay agent. | If the relay agent receives a message other than Relay-forward and Relay-reply and the relay agent does not recognize its message type, it MUST forward the message as described in Section 19.1.1. |
| `19.1.1:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 relay agent. | If the relay agent cannot use the address in the link-address field to identify the interface through which the response to the client will be relayed, the relay agent MUST include an Interface-Id option (see Section 21.18) in the Relay-forward message. |
| `19.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 relay agent. | Relay agents MUST NOT modify the message. |
| `19.2:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 relay agent. | If the relay agent receives a Relay-reply message, it MUST process the message as defined above, regardless of the type of message encapsulated in the Relay Message option. |
| `19.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 relay agent. | A response to the client MUST be relayed through the same relay agents as the original client message. |
| `19.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 relay agent. | Each relay is allowed to insert additional options on the encapsulation level it added but MUST NOT change anything in the message being encapsulated. |
| `20.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | If the RDM field contains 0x00, the replay detection field MUST be set to the value of a strictly monotonically increasing 64-bit unsigned integer (modulo 2^64). |
| `20.3:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the Authentication option protocol designer, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the Authentication option protocol designer. | This method MUST be supported by all Authentication option protocols. |
| `20.3:5` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | Servers that support the reconfigure mechanism MUST ensure that the replay detection value is retained between restarts. |
| `20.4.2:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The reconfigure key is 128 bits long and MUST be a cryptographically strong random or pseudorandom number that cannot easily be predicted. |
| `21.4:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST ignore any values in these fields in messages received from a client. |
| `21.4:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-14.2-1: this sentence names the section 14.2 rule that rows RFC9915-14.2-1 and RFC9915-14.2-2 carry. | The client MUST follow the rules defined in Section 14.2. |
| `21.6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-18.2.10.1-3: this sentence states the same /128 rule as section 18.2.10.1. | A client MUST NOT form an implicit prefix with a length other than 128 for this address. |
| `21.6:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST ignore any received values. |
| `21.7:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-21.7-1: this sentence states the consequence of the client obligation row RFC9915-21.7-1 carries. | Other top-level option codes MUST appear in the Option Request option or they will not be sent by the server. |
| `21.12:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST NOT send this option. |
| `21.13:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | MUST NOT be NUL-terminated. |
| `21.14:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | A server MUST include this option in a Reply message sent in response to a Solicit message when completing the Solicit/Reply message exchange. |
| `21.18:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST copy the Interface-Id option from the Relay-forward message into the Relay-reply message the server sends to the relay agent in response to the Relay-forward message. |
| `21.18:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 relay agent, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 relay agent. | This option MUST NOT appear in any message except a Relay-forward or Relay-reply message. |
| `21.21:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST ignore any values in these fields in messages received from a client. |
| `21.21:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-21.4-2: this sentence states for IA_PD the T1/T2 rule section 21.4 states for IA_NA; internal/plugins/iface/dhcp/dhcp_v6_lease_linux.go::iaTimes reads both through one path. | In a message sent by a server to a client, the client MUST use the values in the T1 and T2 fields for the T1 and T2 timers, unless values in those fields are 0. |
| `21.21:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-14.2-1: this sentence names the section 14.2 rule that rows RFC9915-14.2-1 and RFC9915-14.2-2 carry. | The client MUST follow the rules defined in Section 14.2. |
| `21.22:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The server MUST ignore any received values in these lifetime fields. |
| `21.23:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | The option value MUST NOT be smaller than IRT_MINIMUM. |
| `21.23:4` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | This option MUST only appear in the top-level options area of Reply messages. |
| `21.24:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | SOL_MAX_RT value: Overriding value for SOL_MAX_RT in seconds; MUST be in this range: 60 <= "value" <= 86400 (1 day). |
| `21.24:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates RFC9915-18.2.1-3: this sentence states the SOL_MAX_RT request row RFC9915-18.2.1-3 carries. | A DHCP client MUST include the SOL_MAX_RT option code in any Option Request option (see Section 21.7) it sends in a Solicit message. |
| `21.25:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | Binds the DHCPv6 server, not the client; ze implements the client role only: internal/plugins/iface/dhcp/ holds the DHCPv6 client and internal/plugins/dhcpserver/ answers DHCPv4 alone, so no ze code acts as the DHCPv6 server. | INF_MAX_RT value: Overriding value for INF_MAX_RT in seconds; MUST be in this range: 60 <= "value" <= 86400 (1 day). |

## Superseded

No document obsoletes RFC 9915, so its obligations are stated where they were written.
