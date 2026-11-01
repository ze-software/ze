# RFC 5382 - NAT Behavioral Requirements for TCP

NAT behavior for TCP. Every requirement this repository extracted from RFC 5382, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 11 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0 | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 22 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 11 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 11 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 11 | of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 11 gated MUSTs: they add up to 11. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | NAT behavior for TCP |
| Enrolment | Enrolled |
| Requirements | 22 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 11 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tagged units | 0 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc5382.md` |
| Requirement shard | `rfc/requirements/rfc5382.md` |
| RFC text | `rfc/full/rfc5382.txt` |

## Enrolment

Enrolled: NAT behavioral requirements for TCP (BCP 142). Every MUST-level row is classified: all 11 are {gap}: the behaviors neither ze nor the Linux defaults provide (endpoint-independent mapping, no port overloading, the unsolicited-SYN hold, the four-minute transitory timeout, a floor on an operator-set established timeout, hairpinning), and the three nf_conntrack performs on the nft firewall backend (RFC5382-4.2-1, RFC5382-4.2-2, RFC5382-7.3-1), which stay unverified because an operator may select the vpp backend, whose VPP NAT44-ED path (internal/plugins/firewall/vpp/nat_linux.go, applyMasquerade, applySNAT, applyDNAT) no test or source reading has checked.

## What the public ledger says

**Area:** NAT behavior for TCP

**Proof:** 0 of 11 MUST proven

**Evidence:** -

**Unmet:** RFC5382-4.1-1, RFC5382-4.2-1, RFC5382-4.2-2, RFC5382-4.3-1, RFC5382-4.3-2, RFC5382-4.3-3, RFC5382-5-1, RFC5382-5-2, RFC5382-7.1-1, RFC5382-7.2-1, RFC5382-7.3-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the gated population |
| Annotated (including scoped evidence) | 11 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **11** | every gated MUST falls in exactly one bucket above |

**Annotated (including scoped evidence) (11):** [`RFC5382-4.1-1`](#rfc5382-4.1-1), [`RFC5382-4.2-1`](#rfc5382-4.2-1), [`RFC5382-4.2-2`](#rfc5382-4.2-2), [`RFC5382-4.3-1`](#rfc5382-4.3-1), [`RFC5382-4.3-2`](#rfc5382-4.3-2), [`RFC5382-4.3-3`](#rfc5382-4.3-3), [`RFC5382-5-1`](#rfc5382-5-1), [`RFC5382-5-2`](#rfc5382-5-2), [`RFC5382-7.1-1`](#rfc5382-7.1-1), [`RFC5382-7.2-1`](#rfc5382-7.2-1), [`RFC5382-7.3-1`](#rfc5382-7.3-1)

## Figures

- **MUST bar**: Ze correctness gap 11
- **Overview bar**: SHOULD not decided 11, Ze correctness gap 11

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 11 |
| MUST rows kept (the MUST bar) | 11 |
| Ze proven | 0 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 11 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 11 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 11 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc5382.json](../../../data/rfc/rfc5382.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5382-4.1-1` | REQ-1: A NAT MUST have an "Endpoint-Independent Mapping" behavior for TCP. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nf_nat preserves the internal source port only while the translated 5-tuple is unique and otherwise allocates another port for that destination, the masquerade random and fully-random flags ze lowers randomize per connection, and VPP NAT44-ED maps per endpoint; internal/plugins/firewall/nft/lower_linux.go lowerMasquerade |
| `RFC5382-4.2-1` | REQ-2: A NAT MUST support all valid sequences of TCP packets (defined in [RFC0793]) for connections initiated both internally as well as externally when the connection is permitted by the NAT. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** on the nft backend the kernel performs it: internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade (with lowerSNAT and lowerDNAT) installs the NAT rule and the Linux nf_conntrack TCP tracker (nf_conntrack_proto_tcp.c, linux:CONFIG_NF_CONNTRACK) follows every RFC 793 state transition of the translated connection in nf_conntrack_tcp_packet over its tcp_conntracks table; the be_liberal and loose values internal/component/config/system/conntrack.go::addTCPBehavior can write widen what the tracker accepts and remove no transition; the firewall backend leaf is operator-set, and on the vpp backend internal/plugins/firewall/vpp/nat_linux.go::applyMasquerade (with applySNAT and applyDNAT) programs VPP NAT44-ED instead, whose TCP state tracking no test or source reading has verified |
| `RFC5382-4.2-2` | In addition to handling the TCP 3-way handshake mode of connection initiation, A NAT MUST handle the TCP simultaneous- open mode of connection initiation. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** on the nft backend the kernel performs it: internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade (with lowerSNAT and lowerDNAT) installs the NAT rule and the Linux nf_conntrack TCP tracker (nf_conntrack_proto_tcp.c, linux:CONFIG_NF_CONNTRACK) accepts a SYN answered by a SYN as a simultaneous open, the TCP_CONNTRACK_SYN_SENT2 state of its tcp_conntracks table; the firewall backend leaf is operator-set, and on the vpp backend internal/plugins/firewall/vpp/nat_linux.go::applyMasquerade (with applySNAT and applyDNAT) programs VPP NAT44-ED instead, whose handling of a simultaneous open no test or source reading has verified |
| `RFC5382-4.3-1` | REQ-4: A NAT MUST NOT respond to an unsolicited inbound SYN packet for at least 6 seconds after the packet is received. (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze installs nothing that holds an unsolicited inbound SYN; one addressed to the NAT's external address matches no conntrack entry, reaches the NAT's own Linux TCP stack and is answered with a RST at once unless an operator input filter drops it |
| `RFC5382-4.3-2` | If during this interval the NAT receives and translates an outbound SYN for the connection the NAT MUST silently drop the original unsolicited inbound SYN packet. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no unsolicited SYN is held (RFC5382-4.3-1), so a later outbound SYN cannot cause it to be dropped |
| `RFC5382-4.3-3` | The NAT MUST silently drop the original SYN packet if sending a response violates the security policy of the NAT. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** whether the NAT's own TCP stack answers with a RST or an operator filter drops the SYN is decided by the input ruleset, and ze ties no NAT security policy to that choice |
| `RFC5382-5-1` | In such cases, the value of the "established connection idle-timeout" MUST NOT be less than 2 hours 4 minutes. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the established idle timeout is a value Ze writes: the `system conntrack timeout tcp established` leaf accepts 1 to 604800 seconds and internal/component/config/system/conntrack.go::addTCPTimeouts writes it to net.netfilter.nf_conntrack_tcp_timeout_established, so an operator value below 7440 seconds breaks this MUST NOT and nothing refuses it; the Linux default of five days holds only while the leaf is unset |
| `RFC5382-5-2` | The value of the "transitory connection idle-timeout" MUST NOT be less than 4 minutes. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze writes a transitory TCP conntrack timeout only when the operator sets one under `system conntrack timeout tcp`, which internal/component/config/system/conntrack.go::addTCPTimeouts writes to the net.netfilter.nf_conntrack_tcp_timeout_* sysctls from leaves that accept values down to 1 second; with the leaves unset Linux's transitory TCP timeouts apply, from 10 seconds (close) to 120 seconds (syn_sent, time_wait), and an operator value below 4 minutes is not refused; on the vpp backend the NAT44-ED transitory timeout is unverified |
| `RFC5382-7.1-1` | REQ-7: A NAT MUST NOT have a "Port assignment" behavior of "Port overloading" for TCP. (§7.1) | MUST NOT | 7.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nf_nat checks a candidate translation for collision on the full reply tuple (net/netfilter/nf_nat_core.c nf_nat_used_tuple inverts the tuple and asks nf_conntrack_tuple_taken), so two internal endpoints can be given one external address and port toward different remote endpoints, which is the port overloading RFC 4787 Section 4.2.1 describes and this section forbids for TCP; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade and lowerSNAT install no option that keeps an external port to one internal endpoint, and the firewall backend leaf is operator-set, and on the vpp backend internal/plugins/firewall/vpp/nat_linux.go::applyMasquerade (with applySNAT and applyDNAT) programs VPP NAT44-ED instead, whose port assignment no test or source reading has verified |
| `RFC5382-7.2-1` | REQ-8: A NAT MUST support "hairpinning" for TCP. a) A NAT's hairpinning behavior MUST be of type "External source IP address and port". (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze generates no hairpin rule, and a SYN from an internal host to another internal host's external mapping matches no conntrack reply tuple and is delivered to the NAT itself, so no hairpin exists and no hairpin source is translated |
| `RFC5382-7.3-1` | REQ-10: Receipt of any sort of ICMP message MUST NOT terminate the NAT mapping or TCP connection for which the ICMP was generated. (§7.3) | MUST NOT | 7.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** on the nft backend the kernel performs it: internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade (with lowerSNAT and lowerDNAT) installs the NAT rule and Linux nf_conntrack (linux:CONFIG_NF_CONNTRACK) classifies an ICMP error as RELATED to the connection without deleting it, in icmp_error_message (nf_conntrack_proto_icmp.c); the firewall backend leaf is operator-set, and on the vpp backend internal/plugins/firewall/vpp/nat_linux.go::applyMasquerade (with applySNAT and applyDNAT) programs VPP NAT44-ED instead, whose handling of a received ICMP message no test or source reading has verified |
| `RFC5382-4.3-4` | REQ-3: If application transparency is most important, it is RECOMMENDED that a NAT have an "Endpoint-Independent Filtering" behavior for TCP. (§4.3) | RECOMMENDED | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5382-4.3-5` | If a more stringent filtering behavior is most important, it is RECOMMENDED that a NAT have an "Address-Dependent Filtering" behavior. (§4.3) | RECOMMENDED | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5382-4.3-6` | Otherwise, the NAT SHOULD send an ICMP Port Unreachable error (Type 3, Code 3) for the original SYN, unless REQ-4a applies. (§4.3) | SHOULD | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5382-6-1` | REQ-6: If a NAT includes ALGs that affect TCP, it is RECOMMENDED that all of those ALGs (except for FTP [RFC0959]) be disabled by default. (§6) | RECOMMENDED | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC5382-7.3-2` | REQ-9: If a NAT translates TCP, it SHOULD translate ICMP Destination Unreachable (Type 3) messages. (§7.3) | SHOULD | 7.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5382-4.3-7` | a) The filtering behavior MAY be an option configurable by the administrator of the NAT. (§4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5382-4.3-8` | b) The filtering behavior for TCP MAY be independent of the filtering behavior for UDP. (§4.3) | MAY | 4.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5382-5-3` | REQ-5: If a NAT cannot determine whether the endpoints of a TCP connection are active, it MAY abandon the session if it has been idle for some time. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5382-5-4` | a) The value of the NAT idle-timeouts MAY be configurable. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5382-5-5` | A NAT MAY hold state for a connection in TIME_WAIT state to accommodate retransmissions of the last ACK. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC5382-5-6` | When a NAT abandons a live connection, for example due to a timeout expiring, the NAT MAY either send TCP RST packets to the endpoints or MAY silently abandon the connection. (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5382-4.1-1`](#rfc5382-4.1-1) REQ-1: A NAT MUST have an "Endpoint-Independent Mapping" behavior for TCP. (§4.1) | {gap}, no test | nf_nat preserves the internal source port only while the translated 5-tuple is unique and otherwise allocates another port for that destination, the masquerade random and fully-random flags ze lowers randomize per connection, and VPP NAT44-ED maps per endpoint; internal/plugins/firewall/nft/lower_linux.go lowerMasquerade |
| [`RFC5382-4.2-1`](#rfc5382-4.2-1) REQ-2: A NAT MUST support all valid sequences of TCP packets (defined in [RFC0793]) for connections initiated both internally as well as externally when the connection is permitted by the NAT. (§4.2) | {gap}, no test | on the nft backend the kernel performs it: internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade (with lowerSNAT and lowerDNAT) installs the NAT rule and the Linux nf_conntrack TCP tracker (nf_conntrack_proto_tcp.c, linux:CONFIG_NF_CONNTRACK) follows every RFC 793 state transition of the translated connection in nf_conntrack_tcp_packet over its tcp_conntracks table; the be_liberal and loose values internal/component/config/system/conntrack.go::addTCPBehavior can write widen what the tracker accepts and remove no transition; the firewall backend leaf is operator-set, and on the vpp backend internal/plugins/firewall/vpp/nat_linux.go::applyMasquerade (with applySNAT and applyDNAT) programs VPP NAT44-ED instead, whose TCP state tracking no test or source reading has verified |
| [`RFC5382-4.2-2`](#rfc5382-4.2-2) In addition to handling the TCP 3-way handshake mode of connection initiation, A NAT MUST handle the TCP simultaneous- open mode of connection initiation. (§4.2) | {gap}, no test | on the nft backend the kernel performs it: internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade (with lowerSNAT and lowerDNAT) installs the NAT rule and the Linux nf_conntrack TCP tracker (nf_conntrack_proto_tcp.c, linux:CONFIG_NF_CONNTRACK) accepts a SYN answered by a SYN as a simultaneous open, the TCP_CONNTRACK_SYN_SENT2 state of its tcp_conntracks table; the firewall backend leaf is operator-set, and on the vpp backend internal/plugins/firewall/vpp/nat_linux.go::applyMasquerade (with applySNAT and applyDNAT) programs VPP NAT44-ED instead, whose handling of a simultaneous open no test or source reading has verified |
| [`RFC5382-4.3-1`](#rfc5382-4.3-1) REQ-4: A NAT MUST NOT respond to an unsolicited inbound SYN packet for at least 6 seconds after the packet is received. (§4.3) | {gap}, no test | ze installs nothing that holds an unsolicited inbound SYN; one addressed to the NAT's external address matches no conntrack entry, reaches the NAT's own Linux TCP stack and is answered with a RST at once unless an operator input filter drops it |
| [`RFC5382-4.3-2`](#rfc5382-4.3-2) If during this interval the NAT receives and translates an outbound SYN for the connection the NAT MUST silently drop the original unsolicited inbound SYN packet. (§4.3) | {gap}, no test | no unsolicited SYN is held (RFC5382-4.3-1), so a later outbound SYN cannot cause it to be dropped |
| [`RFC5382-4.3-3`](#rfc5382-4.3-3) The NAT MUST silently drop the original SYN packet if sending a response violates the security policy of the NAT. (§4.3) | {gap}, no test | whether the NAT's own TCP stack answers with a RST or an operator filter drops the SYN is decided by the input ruleset, and ze ties no NAT security policy to that choice |
| [`RFC5382-5-1`](#rfc5382-5-1) In such cases, the value of the "established connection idle-timeout" MUST NOT be less than 2 hours 4 minutes. (§5) | {gap}, no test | the established idle timeout is a value Ze writes: the `system conntrack timeout tcp established` leaf accepts 1 to 604800 seconds and internal/component/config/system/conntrack.go::addTCPTimeouts writes it to net.netfilter.nf_conntrack_tcp_timeout_established, so an operator value below 7440 seconds breaks this MUST NOT and nothing refuses it; the Linux default of five days holds only while the leaf is unset |
| [`RFC5382-5-2`](#rfc5382-5-2) The value of the "transitory connection idle-timeout" MUST NOT be less than 4 minutes. (§5) | {gap}, no test | Ze writes a transitory TCP conntrack timeout only when the operator sets one under `system conntrack timeout tcp`, which internal/component/config/system/conntrack.go::addTCPTimeouts writes to the net.netfilter.nf_conntrack_tcp_timeout_* sysctls from leaves that accept values down to 1 second; with the leaves unset Linux's transitory TCP timeouts apply, from 10 seconds (close) to 120 seconds (syn_sent, time_wait), and an operator value below 4 minutes is not refused; on the vpp backend the NAT44-ED transitory timeout is unverified |
| [`RFC5382-7.1-1`](#rfc5382-7.1-1) REQ-7: A NAT MUST NOT have a "Port assignment" behavior of "Port overloading" for TCP. (§7.1) | {gap}, no test | nf_nat checks a candidate translation for collision on the full reply tuple (net/netfilter/nf_nat_core.c nf_nat_used_tuple inverts the tuple and asks nf_conntrack_tuple_taken), so two internal endpoints can be given one external address and port toward different remote endpoints, which is the port overloading RFC 4787 Section 4.2.1 describes and this section forbids for TCP; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade and lowerSNAT install no option that keeps an external port to one internal endpoint, and the firewall backend leaf is operator-set, and on the vpp backend internal/plugins/firewall/vpp/nat_linux.go::applyMasquerade (with applySNAT and applyDNAT) programs VPP NAT44-ED instead, whose port assignment no test or source reading has verified |
| [`RFC5382-7.2-1`](#rfc5382-7.2-1) REQ-8: A NAT MUST support "hairpinning" for TCP. a) A NAT's hairpinning behavior MUST be of type "External source IP address and port". (§7.2) | {gap}, no test | ze generates no hairpin rule, and a SYN from an internal host to another internal host's external mapping matches no conntrack reply tuple and is delivered to the NAT itself, so no hairpin exists and no hairpin source is translated |
| [`RFC5382-7.3-1`](#rfc5382-7.3-1) REQ-10: Receipt of any sort of ICMP message MUST NOT terminate the NAT mapping or TCP connection for which the ICMP was generated. (§7.3) | {gap}, no test | on the nft backend the kernel performs it: internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade (with lowerSNAT and lowerDNAT) installs the NAT rule and Linux nf_conntrack (linux:CONFIG_NF_CONNTRACK) classifies an ICMP error as RELATED to the connection without deleting it, in icmp_error_message (nf_conntrack_proto_icmp.c); the firewall backend leaf is operator-set, and on the vpp backend internal/plugins/firewall/vpp/nat_linux.go::applyMasquerade (with applySNAT and applyDNAT) programs VPP NAT44-ED instead, whose handling of a received ICMP message no test or source reading has verified |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC5382-4.1-1`](#rfc5382-4.1-1)

REQ-1: A NAT MUST have an "Endpoint-Independent Mapping" behavior for TCP. (§4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5382-4.1-1, so no unit is bound to it.

### [`RFC5382-4.2-1`](#rfc5382-4.2-1)

REQ-2: A NAT MUST support all valid sequences of TCP packets (defined in [RFC0793]) for connections initiated both internally as well as externally when the connection is permitted by the NAT. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5382-4.2-1, so no unit is bound to it.

### [`RFC5382-4.2-2`](#rfc5382-4.2-2)

In addition to handling the TCP 3-way handshake mode of connection initiation, A NAT MUST handle the TCP simultaneous- open mode of connection initiation. (§4.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5382-4.2-2, so no unit is bound to it.

### [`RFC5382-4.3-1`](#rfc5382-4.3-1)

REQ-4: A NAT MUST NOT respond to an unsolicited inbound SYN packet for at least 6 seconds after the packet is received. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5382-4.3-1, so no unit is bound to it.

### [`RFC5382-4.3-2`](#rfc5382-4.3-2)

If during this interval the NAT receives and translates an outbound SYN for the connection the NAT MUST silently drop the original unsolicited inbound SYN packet. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5382-4.3-2, so no unit is bound to it.

### [`RFC5382-4.3-3`](#rfc5382-4.3-3)

The NAT MUST silently drop the original SYN packet if sending a response violates the security policy of the NAT. (§4.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5382-4.3-3, so no unit is bound to it.

### [`RFC5382-5-1`](#rfc5382-5-1)

In such cases, the value of the "established connection idle-timeout" MUST NOT be less than 2 hours 4 minutes. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5382-5-1, so no unit is bound to it.

### [`RFC5382-5-2`](#rfc5382-5-2)

The value of the "transitory connection idle-timeout" MUST NOT be less than 4 minutes. (§5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5382-5-2, so no unit is bound to it.

### [`RFC5382-7.1-1`](#rfc5382-7.1-1)

REQ-7: A NAT MUST NOT have a "Port assignment" behavior of "Port overloading" for TCP. (§7.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5382-7.1-1, so no unit is bound to it.

### [`RFC5382-7.2-1`](#rfc5382-7.2-1)

REQ-8: A NAT MUST support "hairpinning" for TCP. a) A NAT's hairpinning behavior MUST be of type "External source IP address and port". (§7.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5382-7.2-1, so no unit is bound to it.

### [`RFC5382-7.3-1`](#rfc5382-7.3-1)

REQ-10: Receipt of any sort of ICMP message MUST NOT terminate the NAT mapping or TCP connection for which the ICMP was generated. (§7.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC5382-7.3-1, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-10-26 |
| Register | rfc2119 |
| Source | rfc/full/rfc5382.txt |
| Source fingerprint | 45e5af46d41ae22a |
| Record | rfc/extraction/rfc5382.json |
| Mapped sentences | 11 |
| Declined as scope | 12 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 2 | walked | not stated |
| `4.3` | not stated | 3 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 1 | walked | not stated |
| `7.2` | not stated | 1 | walked | not stated |
| `7.3` | not stated | 1 | walked | not stated |
| `8` | not stated | 12 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `11` | not stated | 0 | walked | not stated |
| `11.1` | not stated | 0 | walked | not stated |
| `11.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Section 8 opens by defining what 'compliant with this specification' means; the MUST names the set of requirements and imposes none of its own. | A NAT that supports all of the mandatory requirements of this specification (i.e., the "MUST") and is compliant with [BEHAVE-UDP], is "compliant with this specification". |
| `8:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 summary repeats REQ-1 of Section 4.1. | REQ-1: A NAT MUST have an "Endpoint-Independent Mapping" behavior for TCP. |
| `8:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 summary repeats REQ-2 of Section 4.2. | REQ-2: A NAT MUST support all valid sequences of TCP packets (defined in [RFC0793]) for connections initiated both internally as well as externally when the connection is permitted by the NAT. |
| `8:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 summary repeats REQ-2a of Section 4.2. | In particular: a) In addition to handling the TCP 3-way handshake mode of connection initiation, A NAT MUST handle the TCP simultaneous- open mode of connection initiation. |
| `8:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 summary repeats REQ-4 of Section 4.3. | REQ-4: A NAT MUST NOT respond to an unsolicited inbound SYN packet for at least 6 seconds after the packet is received. |
| `8:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 summary repeats the REQ-4 drop of Section 4.3. | If during this interval the NAT receives and translates an outbound SYN for the connection the NAT MUST silently drop the original unsolicited inbound SYN packet. |
| `8:7` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 summary repeats REQ-4a of Section 4.3. | Otherwise, the NAT SHOULD send an ICMP Port Unreachable error (Type 3, Code 3) for the original SYN, unless REQ-4a applies. a) The NAT MUST silently drop the original SYN packet if sending a response violates the security policy of the NAT. |
| `8:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 summary repeats the REQ-5 established timeout of Section 5. | In such cases, the value of the "established connection idle-timeout" MUST NOT be less than 2 hours 4 minutes. |
| `8:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 summary repeats the REQ-5 transitory timeout of Section 5. | The value of the "transitory connection idle-timeout" MUST NOT be less than 4 minutes. a) The value of the NAT idle-timeouts MAY be configurable. |
| `8:10` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 summary repeats REQ-7 of Section 7.1. | REQ-7: A NAT MUST NOT have a "Port assignment" behavior of "Port overloading" for TCP. |
| `8:11` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 summary repeats REQ-8 of Section 7.2. | REQ-8: A NAT MUST support "hairpinning" for TCP. a) A NAT's hairpinning behavior MUST be of type "External source IP address and port". |
| `8:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 8 summary repeats REQ-10 of Section 7.3. | REQ-10: Receipt of any sort of ICMP message MUST NOT terminate the NAT mapping or TCP connection for which the ICMP was generated. |

## Superseded

No document obsoletes RFC 5382, so its obligations are stated where they were written.
