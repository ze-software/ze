# RFC 5382 - NAT Behavioral Requirements for TCP

NAT behavior for TCP. Every requirement listed for RFC 5382, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0 | of 11 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 11 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 11 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 11 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 0 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 11 | of 11 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 11 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 11 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 11 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 11 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 11 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 11 | of 22 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 11 mandatory requirements: they add up to 11. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | NAT behavior for TCP |
| Automatic checks | Included |
| Requirements | 22 |
| Mandatory requirements (MUST level) | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 11 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 0 |
| Tests linked to requirements | 0 |
| Recorded audit verdicts | 0 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc5382.md` |
| Requirement table | `rfc/requirements/rfc5382.md` |
| RFC text | `rfc/full/rfc5382.txt` |

## Automatic checks

Included in automatic checks: NAT behavioral requirements for TCP (BCP 142). Every MUST-level row is classified: all 11 are {gap}: the behaviors neither ze nor the Linux defaults provide (endpoint-independent mapping, no port overloading, the unsolicited-SYN hold, the four-minute transitory timeout, a floor on an operator-set established timeout, hairpinning), and the three nf_conntrack performs on the nft firewall backend (RFC5382-4.2-1, RFC5382-4.2-2, RFC5382-7.3-1), which stay unverified because an operator may select the vpp backend, whose VPP NAT44-ED path (internal/plugins/firewall/vpp/nat_linux.go, applyMasquerade, applySNAT, applyDNAT) no test or source reading has checked.

## Published support status

**Area:** NAT behavior for TCP

**Proof:** 0 of 11 MUST proven

**Evidence:** -

**Unmet:** RFC5382-4.1-1, RFC5382-4.2-1, RFC5382-4.2-2, RFC5382-4.3-1, RFC5382-4.3-2, RFC5382-4.3-3, RFC5382-5-1, RFC5382-5-2, RFC5382-7.1-1, RFC5382-7.2-1, RFC5382-7.3-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 0 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 11 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **11** | every mandatory requirement appears in exactly one group above |

**Annotated (including scoped evidence) (11):** [`RFC5382-4.1-1`](#rfc5382-4.1-1), [`RFC5382-4.2-1`](#rfc5382-4.2-1), [`RFC5382-4.2-2`](#rfc5382-4.2-2), [`RFC5382-4.3-1`](#rfc5382-4.3-1), [`RFC5382-4.3-2`](#rfc5382-4.3-2), [`RFC5382-4.3-3`](#rfc5382-4.3-3), [`RFC5382-5-1`](#rfc5382-5-1), [`RFC5382-5-2`](#rfc5382-5-2), [`RFC5382-7.1-1`](#rfc5382-7.1-1), [`RFC5382-7.2-1`](#rfc5382-7.2-1), [`RFC5382-7.3-1`](#rfc5382-7.3-1)

## Figures

- **MUST bar**: Not built 11
- **Overview bar**: Not assessed 11, Not built 11

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 11 |
| MUST rows kept (the MUST bar) | 11 |
| Ze proven | 0 |
| OS verified | 0 |
| Proof pending | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 11 |
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
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 11 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

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

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC5382-4.1-1`](#rfc5382-4.1-1)

REQ-1: A NAT MUST have an "Endpoint-Independent Mapping" behavior for TCP. (§4.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5382-4.1-1, so there is no linked test.

### [`RFC5382-4.2-1`](#rfc5382-4.2-1)

REQ-2: A NAT MUST support all valid sequences of TCP packets (defined in [RFC0793]) for connections initiated both internally as well as externally when the connection is permitted by the NAT. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5382-4.2-1, so there is no linked test.

### [`RFC5382-4.2-2`](#rfc5382-4.2-2)

In addition to handling the TCP 3-way handshake mode of connection initiation, A NAT MUST handle the TCP simultaneous- open mode of connection initiation. (§4.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5382-4.2-2, so there is no linked test.

### [`RFC5382-4.3-1`](#rfc5382-4.3-1)

REQ-4: A NAT MUST NOT respond to an unsolicited inbound SYN packet for at least 6 seconds after the packet is received. (§4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5382-4.3-1, so there is no linked test.

### [`RFC5382-4.3-2`](#rfc5382-4.3-2)

If during this interval the NAT receives and translates an outbound SYN for the connection the NAT MUST silently drop the original unsolicited inbound SYN packet. (§4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5382-4.3-2, so there is no linked test.

### [`RFC5382-4.3-3`](#rfc5382-4.3-3)

The NAT MUST silently drop the original SYN packet if sending a response violates the security policy of the NAT. (§4.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5382-4.3-3, so there is no linked test.

### [`RFC5382-5-1`](#rfc5382-5-1)

In such cases, the value of the "established connection idle-timeout" MUST NOT be less than 2 hours 4 minutes. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5382-5-1, so there is no linked test.

### [`RFC5382-5-2`](#rfc5382-5-2)

The value of the "transitory connection idle-timeout" MUST NOT be less than 4 minutes. (§5)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5382-5-2, so there is no linked test.

### [`RFC5382-7.1-1`](#rfc5382-7.1-1)

REQ-7: A NAT MUST NOT have a "Port assignment" behavior of "Port overloading" for TCP. (§7.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5382-7.1-1, so there is no linked test.

### [`RFC5382-7.2-1`](#rfc5382-7.2-1)

REQ-8: A NAT MUST support "hairpinning" for TCP. a) A NAT's hairpinning behavior MUST be of type "External source IP address and port". (§7.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5382-7.2-1, so there is no linked test.

### [`RFC5382-7.3-1`](#rfc5382-7.3-1)

REQ-10: Receipt of any sort of ICMP message MUST NOT terminate the NAT mapping or TCP connection for which the ICMP was generated. (§7.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5382-7.3-1, so there is no linked test.

## Requirement list review

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
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 1 | reviewed | not stated |
| `4.2` | not stated | 2 | reviewed | not stated |
| `4.3` | not stated | 3 | reviewed | not stated |
| `5` | not stated | 2 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `7.1` | not stated | 1 | reviewed | not stated |
| `7.2` | not stated | 1 | reviewed | not stated |
| `7.3` | not stated | 1 | reviewed | not stated |
| `8` | not stated | 12 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `11.1` | not stated | 0 | reviewed | not stated |
| `11.2` | not stated | 0 | reviewed | not stated |

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
