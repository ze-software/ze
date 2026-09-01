# RFC 4862 - IPv6 Stateless Address Autoconfiguration

No row in the public ledger. Every requirement this repository extracted from RFC 4862, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

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
| Gated MUSTs | 16 | of 30 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 16 | of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

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
| Public status | No row in the public ledger |
| Enrolment | Enrolled |
| Requirements | 30 |
| Gated MUST-level | 16 |
| Obligations that bind Ze | 0 |
| Not applicable, so out of scope | 16 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc4862.md` |
| Requirement shard | `rfc/requirements/rfc4862.md` |
| RFC text | `rfc/full/rfc4862.txt` |

## Enrolment

Enrolled: IPv6 Stateless Address Autoconfiguration (RFC 4862): all 16 gated MUST/MUST-NOTs not-applicable -- host SLAAC (address formation, RA/PIO, DAD, deprecated/invalid source selection) is kernel addrconf; ze enables it via sysctls and classifies kernel-assigned addresses

## What the public ledger says

No row in the public ledger, so its summary declares `| Support | - |` and docs/features/rfc-status.md carries no row for RFC 4862.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated instead of tested | 16 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (16):** [`RFC4862-5.1-1`](#rfc4862-5.1-1), [`RFC4862-5.4-1`](#rfc4862-5.4-1), [`RFC4862-5.4-2`](#rfc4862-5.4-2), [`RFC4862-5.4-5`](#rfc4862-5.4-5), [`RFC4862-5.4.1-1`](#rfc4862-5.4.1-1), [`RFC4862-5.4.2-1`](#rfc4862-5.4.2-1), [`RFC4862-5.4.2-4`](#rfc4862-5.4.2-4), [`RFC4862-5.4.3-1`](#rfc4862-5.4.3-1), [`RFC4862-5.4.5-1`](#rfc4862-5.4.5-1), [`RFC4862-5.5-2`](#rfc4862-5.5-2), [`RFC4862-5.5.3-2`](#rfc4862-5.5.3-2), [`RFC4862-5.5.4-3`](#rfc4862-5.5.4-3), [`RFC4862-5.5.4-5`](#rfc4862-5.5.4-5), [`RFC4862-5.5.4-6`](#rfc4862-5.5.4-6), [`RFC4862-5.5.4-7`](#rfc4862-5.5.4-7), [`RFC4862-5.5.4-8`](#rfc4862-5.5.4-8)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4862-5.1-1` | A node MUST allow the DupAddrDetectTransmits autoconfiguration variable to be configured by system management for each multicast-capable interface (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** DAD is run by the kernel, so DupAddrDetectTransmits is the kernel's net.ipv6.conf.<if>.dad_transmits sysctl; ze configures kernel IPv6 conf keys and owns no DAD variable of its own (internal/component/iface/config_sysctl.go:73-79) |
| `RFC4862-5.4-1` | Duplicate Address Detection MUST be performed on all unicast addresses prior to assigning them to an interface, regardless of whether they are obtained through stateless autoconfiguration, DHCPv6, or manual configuration (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** DAD is executed by the kernel addrconf engine; ze only observes the kernel's tentative/assigned state via netlink and never performs DAD (internal/plugins/iface/netlink/show_linux.go:201) |
| `RFC4862-5.4-2` | Duplicate Address Detection MUST NOT be performed on anycast addresses (§5.4) | MUST NOT | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no DAD at all, so the anycast exclusion is enforced by the kernel that performs DAD |
| `RFC4862-5.4-5` | New implementations MUST NOT skip DAD for a global address that reuses the link-local interface identifier (the DAD "optimization") (§5.4) | MUST NOT | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the kernel decides which addresses undergo DAD; ze forms no addresses and applies no DAD optimization |
| `RFC4862-5.4.1-1` | A node MUST silently discard any Neighbor Solicitation or Advertisement message that does not pass the validity checks specified in [RFC4861] (§5.4.1) | MUST | 5.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Neighbor Discovery message validation is performed by the kernel neighbor subsystem; ze sends and parses no NS/NA on the SLAAC path |
| `RFC4862-5.4.2-1` | Before sending a Neighbor Solicitation, an interface MUST join the all-nodes multicast address and the solicited-node multicast address of the tentative address (§5.4.2) | MUST | 5.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** multicast group membership for DAD and the tentative NS are issued by the kernel; ze joins no such groups for autoconfiguration |
| `RFC4862-5.4.2-4` | An interface MUST receive and process datagrams sent to the all-nodes multicast address or solicited-node multicast address of the tentative address during the delay period (§5.4.2) | MUST | 5.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** receiving DAD-window multicast datagrams is a kernel IPv6 input action; ze has no per-packet SLAAC receive path |
| `RFC4862-5.4.3-1` | In all cases, a node MUST NOT respond to a Neighbor Solicitation for a tentative address (§5.4.3) | MUST NOT | 5.4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** NS response suppression for tentative addresses is enforced by the kernel neighbor subsystem that owns the tentative state; ze answers no NS |
| `RFC4862-5.4.5-1` | A tentative address that is determined to be a duplicate MUST NOT be assigned to an interface (§5.4.5) | MUST NOT | 5.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the kernel decides DAD outcome and refuses assignment on collision; ze only observes the resulting non-tentative address via netlink (internal/plugins/iface/netlink/slaac_linux.go:30) |
| `RFC4862-5.5-2` | The global-address creation processing described in this section MUST be enabled by default (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** global-address formation from RAs is the kernel's addrconf, on by default via net.ipv6.conf.*.autoconf; ze forwards this knob to the kernel and forms no addresses itself (internal/component/iface/config_sysctl.go:74-75) |
| `RFC4862-5.5.3-2` | If the sum of the prefix length and interface identifier length does not equal 128 bits, the Prefix Information option MUST be ignored (§5.5.3) | MUST | 5.5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** PIO parsing and the 128-bit consistency check happen inside the kernel RA processor; ze parses no Prefix Information options |
| `RFC4862-5.5.4-3` | IP and higher layers (e.g., TCP, UDP) MUST continue to accept and process datagrams destined to a deprecated address as normal (§5.5.4) | MUST | 5.5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** datagram acceptance for a deprecated address is a kernel IP-stack behavior; ze runs no IP forwarding/receive datapath |
| `RFC4862-5.5.4-5` | If an implementation prevents new communication from using a deprecated address, system management MUST have the ability to disable that facility (§5.5.4) | MUST | 5.5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no facility that prevents new communication on a deprecated address; deprecated-address source selection is the kernel's, configured via kernel sysctls ze passes through (internal/component/iface/config_sysctl.go:73-79) |
| `RFC4862-5.5.4-6` | The facility preventing new communication on a deprecated address MUST be disabled by default (§5.5.4) | MUST | 5.5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** any such default lives in the kernel source-address-selection logic; ze provides no deprecated-address-blocking facility to default off |
| `RFC4862-5.5.4-7` | An invalid address MUST NOT be used as a source address in outgoing communications (§5.5.4) | MUST NOT | 5.5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** source-address selection excluding invalid (expired) addresses is a kernel function; ze originates no traffic bound by SLAAC source selection and only observes lifetimes via netlink (internal/plugins/iface/netlink/slaac_linux.go:46) |
| `RFC4862-5.5.4-8` | An invalid address MUST NOT be recognized as a destination on a receiving interface (§5.5.4) | MUST NOT | 5.5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** destination-address acceptance on input is a kernel IP-stack decision; ze has no IPv6 receive datapath that recognizes destinations |
| `RFC4862-5.4-3` | Each individual unicast address SHOULD be tested for uniqueness (§5.4) | SHOULD | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4-4` | Performing DAD only for the link-local address and skipping the global address that reuses its interface identifier is NOT RECOMMENDED (§5.4) | NOT RECOMMENDED | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.2-2` | If the Neighbor Solicitation is the first message sent after interface (re)initialization, the node SHOULD delay joining the solicited-node multicast address by a random delay between 0 and MAX_RTR_SOLICITATION_DELAY (§5.4.2) | SHOULD | 5.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.2-3` | Even when not the first message, the node SHOULD delay joining the solicited-node multicast address by a random delay between 0 and MAX_RTR_SOLICITATION_DELAY if the address being checked is configured by a multicasted router advertisement (§5.4.2) | SHOULD | 5.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.5-2` | On DAD failure, the node SHOULD log a system management error (§5.4.5) | SHOULD | 5.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.5-3` | If the duplicate link-local address is formed from a hardware-based interface identifier (e.g., EUI-64), IP operation on the interface SHOULD be disabled (§5.4.5) | SHOULD | 5.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5-1` | Creation of global addresses as described in this section SHOULD be locally configurable (§5.5) | SHOULD | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.4-1` | A deprecated address SHOULD continue to be used as a source address in existing communications (§5.5.4) | SHOULD | 5.5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.4-2` | A deprecated address SHOULD NOT be used to initiate new communications if an alternate (non-deprecated) address of sufficient scope can easily be used instead (§5.5.4) | SHOULD NOT | 5.5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.6-1` | If there is no security difference, the most recently obtained values SHOULD have precedence over information learned earlier (§5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.5-4` | If the duplicate link-local address is not formed from a hardware-based interface identifier, IP operation on the interface MAY be continued (§5.4.5) | MAY | 5.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.3-1` | A node MAY wish to log a system management error when the preferred lifetime is greater than the valid lifetime (§5.5.3) | MAY | 5.5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.3-3` | An implementation MAY wish to log a system management error when the prefix length plus interface identifier length is not 128 bits (§5.5.3) | MAY | 5.5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.4-4` | An implementation MAY prevent any new communication from using a deprecated address (§5.5.4) | MAY | 5.5.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4862-5.1-1`](#rfc4862-5.1-1) A node MUST allow the DupAddrDetectTransmits autoconfiguration variable to be configured by system management for each multicast-capable interface (§5.1) | no test | no test carries this requirement id; annotated {not-applicable}: DAD is run by the kernel, so DupAddrDetectTransmits is the kernel's net.ipv6.conf.<if>.dad_transmits sysctl; ze configures kernel IPv6 conf keys and owns no DAD variable of its own (internal/component/iface/config_sysctl.go:73-79) |
| [`RFC4862-5.4-1`](#rfc4862-5.4-1) Duplicate Address Detection MUST be performed on all unicast addresses prior to assigning them to an interface, regardless of whether they are obtained through stateless autoconfiguration, DHCPv6, or manual configuration (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: DAD is executed by the kernel addrconf engine; ze only observes the kernel's tentative/assigned state via netlink and never performs DAD (internal/plugins/iface/netlink/show_linux.go:201) |
| [`RFC4862-5.4-2`](#rfc4862-5.4-2) Duplicate Address Detection MUST NOT be performed on anycast addresses (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no DAD at all, so the anycast exclusion is enforced by the kernel that performs DAD |
| [`RFC4862-5.4-5`](#rfc4862-5.4-5) New implementations MUST NOT skip DAD for a global address that reuses the link-local interface identifier (the DAD "optimization") (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: the kernel decides which addresses undergo DAD; ze forms no addresses and applies no DAD optimization |
| [`RFC4862-5.4.1-1`](#rfc4862-5.4.1-1) A node MUST silently discard any Neighbor Solicitation or Advertisement message that does not pass the validity checks specified in [RFC4861] (§5.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: Neighbor Discovery message validation is performed by the kernel neighbor subsystem; ze sends and parses no NS/NA on the SLAAC path |
| [`RFC4862-5.4.2-1`](#rfc4862-5.4.2-1) Before sending a Neighbor Solicitation, an interface MUST join the all-nodes multicast address and the solicited-node multicast address of the tentative address (§5.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: multicast group membership for DAD and the tentative NS are issued by the kernel; ze joins no such groups for autoconfiguration |
| [`RFC4862-5.4.2-4`](#rfc4862-5.4.2-4) An interface MUST receive and process datagrams sent to the all-nodes multicast address or solicited-node multicast address of the tentative address during the delay period (§5.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: receiving DAD-window multicast datagrams is a kernel IPv6 input action; ze has no per-packet SLAAC receive path |
| [`RFC4862-5.4.3-1`](#rfc4862-5.4.3-1) In all cases, a node MUST NOT respond to a Neighbor Solicitation for a tentative address (§5.4.3) | no test | no test carries this requirement id; annotated {not-applicable}: NS response suppression for tentative addresses is enforced by the kernel neighbor subsystem that owns the tentative state; ze answers no NS |
| [`RFC4862-5.4.5-1`](#rfc4862-5.4.5-1) A tentative address that is determined to be a duplicate MUST NOT be assigned to an interface (§5.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: the kernel decides DAD outcome and refuses assignment on collision; ze only observes the resulting non-tentative address via netlink (internal/plugins/iface/netlink/slaac_linux.go:30) |
| [`RFC4862-5.5-2`](#rfc4862-5.5-2) The global-address creation processing described in this section MUST be enabled by default (§5.5) | no test | no test carries this requirement id; annotated {not-applicable}: global-address formation from RAs is the kernel's addrconf, on by default via net.ipv6.conf.*.autoconf; ze forwards this knob to the kernel and forms no addresses itself (internal/component/iface/config_sysctl.go:74-75) |
| [`RFC4862-5.5.3-2`](#rfc4862-5.5.3-2) If the sum of the prefix length and interface identifier length does not equal 128 bits, the Prefix Information option MUST be ignored (§5.5.3) | no test | no test carries this requirement id; annotated {not-applicable}: PIO parsing and the 128-bit consistency check happen inside the kernel RA processor; ze parses no Prefix Information options |
| [`RFC4862-5.5.4-3`](#rfc4862-5.5.4-3) IP and higher layers (e.g., TCP, UDP) MUST continue to accept and process datagrams destined to a deprecated address as normal (§5.5.4) | no test | no test carries this requirement id; annotated {not-applicable}: datagram acceptance for a deprecated address is a kernel IP-stack behavior; ze runs no IP forwarding/receive datapath |
| [`RFC4862-5.5.4-5`](#rfc4862-5.5.4-5) If an implementation prevents new communication from using a deprecated address, system management MUST have the ability to disable that facility (§5.5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no facility that prevents new communication on a deprecated address; deprecated-address source selection is the kernel's, configured via kernel sysctls ze passes through (internal/component/iface/config_sysctl.go:73-79) |
| [`RFC4862-5.5.4-6`](#rfc4862-5.5.4-6) The facility preventing new communication on a deprecated address MUST be disabled by default (§5.5.4) | no test | no test carries this requirement id; annotated {not-applicable}: any such default lives in the kernel source-address-selection logic; ze provides no deprecated-address-blocking facility to default off |
| [`RFC4862-5.5.4-7`](#rfc4862-5.5.4-7) An invalid address MUST NOT be used as a source address in outgoing communications (§5.5.4) | no test | no test carries this requirement id; annotated {not-applicable}: source-address selection excluding invalid (expired) addresses is a kernel function; ze originates no traffic bound by SLAAC source selection and only observes lifetimes via netlink (internal/plugins/iface/netlink/slaac_linux.go:46) |
| [`RFC4862-5.5.4-8`](#rfc4862-5.5.4-8) An invalid address MUST NOT be recognized as a destination on a receiving interface (§5.5.4) | no test | no test carries this requirement id; annotated {not-applicable}: destination-address acceptance on input is a kernel IP-stack decision; ze has no IPv6 receive datapath that recognizes destinations |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4862-5.1-1`](#rfc4862-5.1-1)

A node MUST allow the DupAddrDetectTransmits autoconfiguration variable to be configured by system management for each multicast-capable interface (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.1-1, so no unit is bound to it.

### [`RFC4862-5.4-1`](#rfc4862-5.4-1)

Duplicate Address Detection MUST be performed on all unicast addresses prior to assigning them to an interface, regardless of whether they are obtained through stateless autoconfiguration, DHCPv6, or manual configuration (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.4-1, so no unit is bound to it.

### [`RFC4862-5.4-2`](#rfc4862-5.4-2)

Duplicate Address Detection MUST NOT be performed on anycast addresses (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.4-2, so no unit is bound to it.

### [`RFC4862-5.4-5`](#rfc4862-5.4-5)

New implementations MUST NOT skip DAD for a global address that reuses the link-local interface identifier (the DAD "optimization") (§5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.4-5, so no unit is bound to it.

### [`RFC4862-5.4.1-1`](#rfc4862-5.4.1-1)

A node MUST silently discard any Neighbor Solicitation or Advertisement message that does not pass the validity checks specified in [RFC4861] (§5.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.4.1-1, so no unit is bound to it.

### [`RFC4862-5.4.2-1`](#rfc4862-5.4.2-1)

Before sending a Neighbor Solicitation, an interface MUST join the all-nodes multicast address and the solicited-node multicast address of the tentative address (§5.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.4.2-1, so no unit is bound to it.

### [`RFC4862-5.4.2-4`](#rfc4862-5.4.2-4)

An interface MUST receive and process datagrams sent to the all-nodes multicast address or solicited-node multicast address of the tentative address during the delay period (§5.4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.4.2-4, so no unit is bound to it.

### [`RFC4862-5.4.3-1`](#rfc4862-5.4.3-1)

In all cases, a node MUST NOT respond to a Neighbor Solicitation for a tentative address (§5.4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.4.3-1, so no unit is bound to it.

### [`RFC4862-5.4.5-1`](#rfc4862-5.4.5-1)

A tentative address that is determined to be a duplicate MUST NOT be assigned to an interface (§5.4.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.4.5-1, so no unit is bound to it.

### [`RFC4862-5.5-2`](#rfc4862-5.5-2)

The global-address creation processing described in this section MUST be enabled by default (§5.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.5-2, so no unit is bound to it.

### [`RFC4862-5.5.3-2`](#rfc4862-5.5.3-2)

If the sum of the prefix length and interface identifier length does not equal 128 bits, the Prefix Information option MUST be ignored (§5.5.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.5.3-2, so no unit is bound to it.

### [`RFC4862-5.5.4-3`](#rfc4862-5.5.4-3)

IP and higher layers (e.g., TCP, UDP) MUST continue to accept and process datagrams destined to a deprecated address as normal (§5.5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.5.4-3, so no unit is bound to it.

### [`RFC4862-5.5.4-5`](#rfc4862-5.5.4-5)

If an implementation prevents new communication from using a deprecated address, system management MUST have the ability to disable that facility (§5.5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.5.4-5, so no unit is bound to it.

### [`RFC4862-5.5.4-6`](#rfc4862-5.5.4-6)

The facility preventing new communication on a deprecated address MUST be disabled by default (§5.5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.5.4-6, so no unit is bound to it.

### [`RFC4862-5.5.4-7`](#rfc4862-5.5.4-7)

An invalid address MUST NOT be used as a source address in outgoing communications (§5.5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.5.4-7, so no unit is bound to it.

### [`RFC4862-5.5.4-8`](#rfc4862-5.5.4-8)

An invalid address MUST NOT be recognized as a destination on a receiving interface (§5.5.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC4862-5.5.4-8, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 4862, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 4862, so its obligations are stated where they were written.
