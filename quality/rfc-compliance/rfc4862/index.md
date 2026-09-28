# RFC 4862 - IPv6 Stateless Address Autoconfiguration

No row in the public ledger. Every requirement this repository extracted from RFC 4862, the tests bound to it, and what a reader has verified about them. This summary is not enrolled.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 16 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 16 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 16 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 16 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| MUSTs declared | 16 | of 30 this summary declares | MUST-level requirements this summary DECLARES. The gate holds none of them, because this RFC is not enrolled (third-party), so every share below reads what the summary records rather than what the gate enforces |
| Out of scope | 16 | of 16 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 100.0% | 16 of 16 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 16 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 16 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 16 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| MUSTs declared | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | No row in the public ledger |
| Enrolment | Not enrolled (third-party) |
| Requirements | 30 |
| Gated MUST-level | 16 |
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

Not enrolled (third-party, a layer under or beside Ze performs the document and Ze holds no Go code for it, so the reason beside this kind names the component that does): Linux addrconf performs stateless address autoconfiguration. Ze sets the sysctls at internal/component/iface/config_sysctl.go and reads the resulting addresses over netlink.

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
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **16** | every gated MUST falls in exactly one bucket above |

**Annotated instead of tested (16):** [`RFC4862-5.1-1`](#rfc4862-5.1-1), [`RFC4862-5.4-1`](#rfc4862-5.4-1), [`RFC4862-5.4-2`](#rfc4862-5.4-2), [`RFC4862-5.4-5`](#rfc4862-5.4-5), [`RFC4862-5.4.1-1`](#rfc4862-5.4.1-1), [`RFC4862-5.4.2-1`](#rfc4862-5.4.2-1), [`RFC4862-5.4.2-4`](#rfc4862-5.4.2-4), [`RFC4862-5.4.3-1`](#rfc4862-5.4.3-1), [`RFC4862-5.4.5-1`](#rfc4862-5.4.5-1), [`RFC4862-5.5-2`](#rfc4862-5.5-2), [`RFC4862-5.5.3-2`](#rfc4862-5.5.3-2), [`RFC4862-5.5.4-3`](#rfc4862-5.5.4-3), [`RFC4862-5.5.4-5`](#rfc4862-5.5.4-5), [`RFC4862-5.5.4-6`](#rfc4862-5.5.4-6), [`RFC4862-5.5.4-7`](#rfc4862-5.5.4-7), [`RFC4862-5.5.4-8`](#rfc4862-5.5.4-8)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC4862-5.1-1` | A node MUST allow the following autoconfiguration-related variable to be configured by system management for each multicast-capable interface: DupAddrDetectTransmits The number of consecutive Neighbor Solicitation messages sent while performing Duplicate Address Detection on a tentative address. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** DAD is run by the kernel, so DupAddrDetectTransmits is the kernel's net.ipv6.conf.<if>.dad_transmits sysctl; ze configures kernel IPv6 conf keys and owns no DAD variable of its own (internal/component/iface/config_sysctl.go:73-79) |
| `RFC4862-5.4-1` | Duplicate Address Detection MUST be performed on all unicast addresses prior to assigning them to an interface, regardless of whether they are obtained through stateless autoconfiguration, DHCPv6, or manual configuration (§5.4) | MUST | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** DAD is executed by the kernel addrconf engine; ze only observes the kernel's tentative/assigned state via netlink and never performs DAD (internal/plugins/iface/netlink/show_linux.go:201) |
| `RFC4862-5.4-2` | Duplicate Address Detection MUST NOT be performed on anycast addresses (§5.4) | MUST NOT | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no DAD at all, so the anycast exclusion is enforced by the kernel that performs DAD |
| `RFC4862-5.4-5` | New implementations MUST NOT skip DAD for a global address that reuses the link-local interface identifier (the DAD "optimization") (§5.4) | MUST NOT | 5.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the kernel decides which addresses undergo DAD; ze forms no addresses and applies no DAD optimization |
| `RFC4862-5.4.1-1` | A node MUST silently discard any Neighbor Solicitation or Advertisement message that does not pass the validity checks specified in [RFC4861] (§5.4.1) | MUST | 5.4.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** Neighbor Discovery message validation is performed by the kernel neighbor subsystem; ze sends and parses no NS/NA on the SLAAC path |
| `RFC4862-5.4.2-1` | Before sending a Neighbor Solicitation, an interface MUST join the all-nodes multicast address and the solicited-node multicast address of the tentative address (§5.4.2) | MUST | 5.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** multicast group membership for DAD and the tentative NS are issued by the kernel; ze joins no such groups for autoconfiguration |
| `RFC4862-5.4.2-4` | In order to improve the robustness of the Duplicate Address Detection algorithm, an interface MUST receive and process datagrams sent to the all-nodes multicast address or solicited-node multicast address of the tentative address during the delay period. (§5.4.2) | MUST | 5.4.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** receiving DAD-window multicast datagrams is a kernel IPv6 input action; ze has no per-packet SLAAC receive path |
| `RFC4862-5.4.3-1` | In all cases, a node MUST NOT respond to a Neighbor Solicitation for a tentative address (§5.4.3) | MUST NOT | 5.4.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** NS response suppression for tentative addresses is enforced by the kernel neighbor subsystem that owns the tentative state; ze answers no NS |
| `RFC4862-5.4.5-1` | A tentative address that is determined to be a duplicate MUST NOT be assigned to an interface (§5.4.5) | MUST NOT | 5.4.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the kernel decides DAD outcome and refuses assignment on collision; ze only observes the resulting non-tentative address via netlink (internal/plugins/iface/netlink/slaac_linux.go:30) |
| `RFC4862-5.5-2` | However, the processing described below MUST be enabled by default. (§5.5) | MUST | 5.5 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** global-address formation from RAs is the kernel's addrconf, on by default via net.ipv6.conf.*.autoconf; ze forwards this knob to the kernel and forms no addresses itself (internal/component/iface/config_sysctl.go:74-75) |
| `RFC4862-5.5.3-2` | If the sum of the prefix length and interface identifier length does not equal 128 bits, the Prefix Information option MUST be ignored (§5.5.3) | MUST | 5.5.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** PIO parsing and the 128-bit consistency check happen inside the kernel RA processor; ze parses no Prefix Information options |
| `RFC4862-5.5.4-3` | IP and higher layers (e.g., TCP, UDP) MUST continue to accept and process datagrams destined to a deprecated address as normal (§5.5.4) | MUST | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** datagram acceptance for a deprecated address is a kernel IP-stack behavior; ze runs no IP forwarding/receive datapath |
| `RFC4862-5.5.4-5` | If an implementation prevents new communication from using a deprecated address, system management MUST have the ability to disable that facility (§5.5.4) | MUST | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no facility that prevents new communication on a deprecated address; deprecated-address source selection is the kernel's, configured via kernel sysctls ze passes through (internal/component/iface/config_sysctl.go:73-79) |
| `RFC4862-5.5.4-6` | The facility preventing new communication on a deprecated address MUST be disabled by default (§5.5.4) | MUST | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** any such default lives in the kernel source-address-selection logic; ze provides no deprecated-address-blocking facility to default off |
| `RFC4862-5.5.4-7` | An invalid address MUST NOT be used as a source address in outgoing communications (§5.5.4) | MUST NOT | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** source-address selection excluding invalid (expired) addresses is a kernel function; ze originates no traffic bound by SLAAC source selection and only observes lifetimes via netlink (internal/plugins/iface/netlink/slaac_linux.go:46) |
| `RFC4862-5.5.4-8` | An invalid address MUST NOT be recognized as a destination on a receiving interface (§5.5.4) | MUST NOT | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** destination-address acceptance on input is a kernel IP-stack decision; ze has no IPv6 receive datapath that recognizes destinations |
| `RFC4862-5.4-3` | Each individual unicast address SHOULD be tested for uniqueness (§5.4) | SHOULD | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4-4` | Performing DAD only for the link-local address and skipping the global address that reuses its interface identifier is NOT RECOMMENDED (§5.4) | NOT RECOMMENDED | 5.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.2-2` | If the Neighbor Solicitation is the first message sent after interface (re)initialization, the node SHOULD delay joining the solicited-node multicast address by a random delay between 0 and MAX_RTR_SOLICITATION_DELAY (§5.4.2) | SHOULD | 5.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.2-3` | Even when not the first message, the node SHOULD delay joining the solicited-node multicast address by a random delay between 0 and MAX_RTR_SOLICITATION_DELAY if the address being checked is configured by a multicasted router advertisement (§5.4.2) | SHOULD | 5.4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.5-2` | On DAD failure, the node SHOULD log a system management error (§5.4.5) | SHOULD | 5.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.5-3` | If the duplicate link-local address is formed from a hardware-based interface identifier (e.g., EUI-64), IP operation on the interface SHOULD be disabled (§5.4.5) | SHOULD | 5.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5-1` | Creation of global addresses as described in this section SHOULD be locally configurable (§5.5) | SHOULD | 5.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.4-1` | A deprecated address SHOULD continue to be used as a source address in existing communications (§5.5.4) | SHOULD | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.4-2` | A deprecated address SHOULD NOT be used to initiate new communications if an alternate (non-deprecated) address of sufficient scope can easily be used instead (§5.5.4) | SHOULD NOT | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.6-1` | If there is no security difference, the most recently obtained values SHOULD have precedence over information learned earlier (§5.6) | SHOULD | 5.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.4.5-4` | If the duplicate link-local address is not formed from a hardware-based interface identifier, IP operation on the interface MAY be continued (§5.4.5) | MAY | 5.4.5 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.3-1` | A node MAY wish to log a system management error when the preferred lifetime is greater than the valid lifetime (§5.5.3) | MAY | 5.5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.3-3` | An implementation MAY wish to log a system management error when the prefix length plus interface identifier length is not 128 bits (§5.5.3) | MAY | 5.5.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC4862-5.5.4-4` | An implementation MAY prevent any new communication from using a deprecated address (§5.5.4) | MAY | 5.5.4 - Four sites | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC4862-5.1-1`](#rfc4862-5.1-1) A node MUST allow the following autoconfiguration-related variable to be configured by system management for each multicast-capable interface: DupAddrDetectTransmits The number of consecutive Neighbor Solicitation messages sent while performing Duplicate Address Detection on a tentative address. (§5.1) | no test | no test carries this requirement id; annotated {not-applicable}: DAD is run by the kernel, so DupAddrDetectTransmits is the kernel's net.ipv6.conf.<if>.dad_transmits sysctl; ze configures kernel IPv6 conf keys and owns no DAD variable of its own (internal/component/iface/config_sysctl.go:73-79) |
| [`RFC4862-5.4-1`](#rfc4862-5.4-1) Duplicate Address Detection MUST be performed on all unicast addresses prior to assigning them to an interface, regardless of whether they are obtained through stateless autoconfiguration, DHCPv6, or manual configuration (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: DAD is executed by the kernel addrconf engine; ze only observes the kernel's tentative/assigned state via netlink and never performs DAD (internal/plugins/iface/netlink/show_linux.go:201) |
| [`RFC4862-5.4-2`](#rfc4862-5.4-2) Duplicate Address Detection MUST NOT be performed on anycast addresses (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no DAD at all, so the anycast exclusion is enforced by the kernel that performs DAD |
| [`RFC4862-5.4-5`](#rfc4862-5.4-5) New implementations MUST NOT skip DAD for a global address that reuses the link-local interface identifier (the DAD "optimization") (§5.4) | no test | no test carries this requirement id; annotated {not-applicable}: the kernel decides which addresses undergo DAD; ze forms no addresses and applies no DAD optimization |
| [`RFC4862-5.4.1-1`](#rfc4862-5.4.1-1) A node MUST silently discard any Neighbor Solicitation or Advertisement message that does not pass the validity checks specified in [RFC4861] (§5.4.1) | no test | no test carries this requirement id; annotated {not-applicable}: Neighbor Discovery message validation is performed by the kernel neighbor subsystem; ze sends and parses no NS/NA on the SLAAC path |
| [`RFC4862-5.4.2-1`](#rfc4862-5.4.2-1) Before sending a Neighbor Solicitation, an interface MUST join the all-nodes multicast address and the solicited-node multicast address of the tentative address (§5.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: multicast group membership for DAD and the tentative NS are issued by the kernel; ze joins no such groups for autoconfiguration |
| [`RFC4862-5.4.2-4`](#rfc4862-5.4.2-4) In order to improve the robustness of the Duplicate Address Detection algorithm, an interface MUST receive and process datagrams sent to the all-nodes multicast address or solicited-node multicast address of the tentative address during the delay period. (§5.4.2) | no test | no test carries this requirement id; annotated {not-applicable}: receiving DAD-window multicast datagrams is a kernel IPv6 input action; ze has no per-packet SLAAC receive path |
| [`RFC4862-5.4.3-1`](#rfc4862-5.4.3-1) In all cases, a node MUST NOT respond to a Neighbor Solicitation for a tentative address (§5.4.3) | no test | no test carries this requirement id; annotated {not-applicable}: NS response suppression for tentative addresses is enforced by the kernel neighbor subsystem that owns the tentative state; ze answers no NS |
| [`RFC4862-5.4.5-1`](#rfc4862-5.4.5-1) A tentative address that is determined to be a duplicate MUST NOT be assigned to an interface (§5.4.5) | no test | no test carries this requirement id; annotated {not-applicable}: the kernel decides DAD outcome and refuses assignment on collision; ze only observes the resulting non-tentative address via netlink (internal/plugins/iface/netlink/slaac_linux.go:30) |
| [`RFC4862-5.5-2`](#rfc4862-5.5-2) However, the processing described below MUST be enabled by default. (§5.5) | no test | no test carries this requirement id; annotated {not-applicable}: global-address formation from RAs is the kernel's addrconf, on by default via net.ipv6.conf.*.autoconf; ze forwards this knob to the kernel and forms no addresses itself (internal/component/iface/config_sysctl.go:74-75) |
| [`RFC4862-5.5.3-2`](#rfc4862-5.5.3-2) If the sum of the prefix length and interface identifier length does not equal 128 bits, the Prefix Information option MUST be ignored (§5.5.3) | no test | no test carries this requirement id; annotated {not-applicable}: PIO parsing and the 128-bit consistency check happen inside the kernel RA processor; ze parses no Prefix Information options |
| [`RFC4862-5.5.4-3`](#rfc4862-5.5.4-3) IP and higher layers (e.g., TCP, UDP) MUST continue to accept and process datagrams destined to a deprecated address as normal (§5.5.4) | no test | no test carries this requirement id; annotated {not-applicable}: datagram acceptance for a deprecated address is a kernel IP-stack behavior; ze runs no IP forwarding/receive datapath |
| [`RFC4862-5.5.4-5`](#rfc4862-5.5.4-5) If an implementation prevents new communication from using a deprecated address, system management MUST have the ability to disable that facility (§5.5.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no facility that prevents new communication on a deprecated address; deprecated-address source selection is the kernel's, configured via kernel sysctls ze passes through (internal/component/iface/config_sysctl.go:73-79) |
| [`RFC4862-5.5.4-6`](#rfc4862-5.5.4-6) The facility preventing new communication on a deprecated address MUST be disabled by default (§5.5.4) | no test | no test carries this requirement id; annotated {not-applicable}: any such default lives in the kernel source-address-selection logic; ze provides no deprecated-address-blocking facility to default off |
| [`RFC4862-5.5.4-7`](#rfc4862-5.5.4-7) An invalid address MUST NOT be used as a source address in outgoing communications (§5.5.4) | no test | no test carries this requirement id; annotated {not-applicable}: source-address selection excluding invalid (expired) addresses is a kernel function; ze originates no traffic bound by SLAAC source selection and only observes lifetimes via netlink (internal/plugins/iface/netlink/slaac_linux.go:46) |
| [`RFC4862-5.5.4-8`](#rfc4862-5.5.4-8) An invalid address MUST NOT be recognized as a destination on a receiving interface (§5.5.4) | no test | no test carries this requirement id; annotated {not-applicable}: destination-address acceptance on input is a kernel IP-stack decision; ze has no IPv6 receive datapath that recognizes destinations |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC4862-5.1-1`](#rfc4862-5.1-1)

A node MUST allow the following autoconfiguration-related variable to be configured by system management for each multicast-capable interface: DupAddrDetectTransmits The number of consecutive Neighbor Solicitation messages sent while performing Duplicate Address Detection on a tentative address. (§5.1)

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

In order to improve the robustness of the Duplicate Address Detection algorithm, an interface MUST receive and process datagrams sent to the all-nodes multicast address or solicited-node multicast address of the tentative address during the delay period. (§5.4.2)

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

However, the processing described below MUST be enabled by default. (§5.5)

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

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc4862.txt |
| Source fingerprint | d3b85654e334cbd7 |
| Record | rfc/extraction/rfc4862.json |
| Mapped sentences | 14 |
| Declined as scope | 18 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 2 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `3` | not stated | 2 | walked | not stated |
| `4` | not stated | 2 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `5.2` | not stated | 0 | walked | not stated |
| `5.3` | not stated | 1 | walked | not stated |
| `5.4` | not stated | 5 | walked | not stated |
| `5.4.1` | not stated | 1 | walked | not stated |
| `5.4.2` | not stated | 3 | walked | not stated |
| `5.4.3` | not stated | 1 | walked | not stated |
| `5.4.4` | not stated | 0 | walked | not stated |
| `5.4.5` | not stated | 1 | walked | not stated |
| `5.5` | not stated | 1 | walked | not stated |
| `5.5.1` | not stated | 0 | walked | not stated |
| `5.5.2` | not stated | 1 | walked | not stated |
| `5.5.3` | not stated | 1 | walked | not stated |
| `5.5.4` | Four sites | 4 | walked | Four sites. Two of them carry two independent obligations each and a site maps to one id. Site 5.5.4:3 maps to RFC4862-5.5.4-5 (management MUST be able to disable the facility) and its second clause, "and the facility MUST be disabled by default", is RFC4862-5.5.4-6. Site 5.5.4:4 maps to RFC4862-5.5.4-7 (MUST NOT be used as a source address) and its second clause, "and MUST NOT be recognized as a destination on a receiving interface", is RFC4862-5.5.4-8. |
| `5.6` | not stated | 0 | walked | not stated |
| `5.7` | not stated | 1 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `A` | not stated | 2 | walked | not stated |
| `B` | not stated | 1 | walked | not stated |
| `C` | not stated | 1 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 2 is Terminology; the sentence is part of the definition of an address lifetime and carries no RFC 2119 keyword. | The valid lifetime must be greater than or equal to the preferred lifetime. |
| `2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Terminology note reconciling this document's definitions with RFC 4291; descriptive, no RFC 2119 keyword. | Note that the address architecture [RFC4291] also defines the length of the interface identifiers for some set of addresses, but the two sets of definitions must be consistent. |
| `3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 3 is Design Goals; the bullet states a goal for the mechanism, not an obligation on an implementation. | o Manual configuration of individual machines before connecting them to the network should not be required. |
| `3:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 3 Design Goals; the sentence motivates the need for router advertisements and states no implementation obligation. | In order to generate global addresses, hosts must determine the prefixes that identify the subnets to which they attach. |
| `4:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 4 is the Protocol Overview; this sentence restates the Duplicate Address Detection obligation Section 5.4 states normatively. | Before the link-local address can be assigned to an interface and used, however, a node must attempt to verify that this "tentative" address is not already in use by another node on the link. |
| `4:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 4 Protocol Overview; indicative prose describing the outcome of a link-local DAD failure, with no RFC 2119 keyword. | If a node determines that its tentative link-local address is not unique, autoconfiguration stops and manual configuration of the interface is required. |
| `4.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 4.1 discusses site renumbering; the sentence explains why addresses stay stable during a packet exchange and binds no implementation. | Even when applications use UDP as a transport protocol, addresses must generally remain the same during a packet exchange. |
| `5.3:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Indicative outcome prose with no RFC 2119 keyword: it states that autoconfiguration fails, and Section 2.1 confines this document's keywords to the normative statements. | If the sum of the link-local prefix length and N is larger than 128, autoconfiguration fails and manual configuration is required. |
| `5.4:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The sentence explains tentative-address packet handling that Section 5.4.2 states normatively as the MUST receive-and-process obligation. | That is, the interface must accept Neighbor Solicitation and Advertisement messages containing the tentative address in the Target Address field, but processes such packets differently from those whose Target Address matches an address assigned to the interface. |
| `5.4:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | "It should also be noted that" reintroduces the Section 5.4 DAD-before-assignment MUST already mapped. | It should also be noted that Duplicate Address Detection must be performed prior to assigning an address to an interface in order to prevent multiple nodes from using the same address simultaneously. |
| `5.4.2:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Explanatory note about why an MLD report is sent; it describes the purpose of a message RFC 4861 and RFC 2710 require, not an obligation of this document. | In the case of Duplicate Address Detection, the MLD report message is required in order to inform MLD- snooping switches, rather than routers, to forward multicast packets. |
| `5.5.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 5.5.2 explains the consequence of having no Router Advertisement; descriptive prose about operator action, no RFC 2119 keyword. | In this case, the forwarding node's address must be manually configured in hosts to be able to send packets off-link, since the only mechanism to configure the default router's address automatically is the one using Router Advertisements. |
| `5.5.4:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An example illustrating the preceding SHOULD about deprecated source addresses; the sentence gives the reason for the SHOULD and states no separate obligation. | For example, if an application explicitly specifies that the protocol stack use a deprecated address as a source address, the protocol stack must accept that; the application might request it because that IP address is used in higher-level communication and there might be a requirement that the multiple connections in such a grouping use the same pair of IP addresses. |
| `5.7:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 5.7 describes an optional stable-storage extension ("An implementation that has stable storage may want to retain addresses in the storage") and the sentence opens "it should also be noted that"; it is advisory, and the section closes "Further details on this kind of extension are beyond the scope of this document." | When this technique is used, it should also be noted that the expiration times of the preferred and valid lifetimes must be retained, in order to prevent the use of an address after it has become deprecated or invalid. |
| `A:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A is informational: it discusses loopback suppression and DAD interaction and states no obligation. | o If a node performing Duplicate Address Detection discards received packets that have the same source link-layer address as the receiving interface, it will also discard packets from other nodes that also use the same link-layer address, including Neighbor Advertisement and Neighbor Solicitation messages required to make Duplicate Address Detection work correctly. |
| `A:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix A informational discussion of multicast loopback semantics; no RFC 2119 keyword and no obligation of this document. | Thus, to perform Duplicate Address Detection correctly in the case where two interfaces are using the same link-layer address, an implementation must have a good understanding of the interface's multicast loopback semantics, and the interface cannot discard received packets simply because the source link-layer address is the same as the interface's. |
| `B:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Appendix B is the change log against RFC 2462; it describes an editorial clarification, not an obligation. | o Clarified wording in Section 5.5.4 to make clear that all upper layer protocols must process (i.e., send and receive) packets sent to deprecated addresses. |
| `C:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF IPR boilerplate. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights that may cover technology that may be required to implement this standard. |

## Superseded

No document obsoletes RFC 4862, so its obligations are stated where they were written.
