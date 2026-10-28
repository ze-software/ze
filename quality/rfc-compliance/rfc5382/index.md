# RFC 5382 - NAT Behavioral Requirements for TCP

Partial. Every requirement this repository extracted from RFC 5382, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 0.0% | 0 of 11 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 11 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 11 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 11 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 0.0% | 0 of 0 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 11 | of 22 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 11 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 11 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 45.5% | 5 of 11 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 11 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |
| Linux cannot do this | 0.0% | 0 of 11 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and remains unmet; it stays in every denominator and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 54.5% | 6 of 11 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 shares marked as a part above are the whole of the 11 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 22 |
| Gated MUST-level | 11 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 6 |
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

Enrolled: NAT behavioral requirements for TCP (BCP 142). Every MUST-level row is classified: the behaviors Linux netfilter performs on the NAT rules ze installs are {lower-layer}, and the behaviors neither ze nor the Linux defaults provide (endpoint-independent mapping, the unsolicited-SYN hold, the four-minute transitory timeout, hairpinning) are {gap}.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Source NAT, destination NAT and masquerade are configured through the `firewall` YANG tree and lowered into nftables NAT chains
- the nf_conntrack TCP tracker follows every valid TCP sequence including simultaneous open, keeps an established connection for the Linux default of five days, never overloads a port, and keeps a connection across ICMP.


**What the ledger says remains**

Endpoint-independent mapping is not guaranteed ([`RFC5382-4.1-1`](#rfc5382-4.1-1)). An unsolicited inbound SYN is not held for six seconds: the NAT's own TCP stack answers it with a RST unless an operator filter drops it ([`RFC5382-4.3-1`](#rfc5382-4.3-1), [`RFC5382-4.3-2`](#rfc5382-4.3-2), [`RFC5382-4.3-3`](#rfc5382-4.3-3)). Ze sets no conntrack timeout, so Linux's transitory TCP timeouts (10 to 120 seconds) stay below four minutes ([`RFC5382-5-2`](#rfc5382-5-2)). No hairpinning is configured ([`RFC5382-7.2-1`](#rfc5382-7.2-1)).

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

## Attainable ceiling

Mandatory ceiling: 6 of 11 (54.5%); excluded: Met by a lower layer: 5 of 11 (45.5%). Optional ceiling: 11 of 11 (100.0%). Lowest mandatory ceiling: rfc5382 6 of 11 (54.5%). Lowest optional ceiling: rfc5382 11 of 11 (100.0%). Ceilings describe current classifications, not an immutable Linux maximum. Not-applicable and feature-declined classifications can be revisited. Every gap remains work Ze owes.

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5382-4.1-1` | REQ-1: A NAT MUST have an "Endpoint-Independent Mapping" behavior for TCP. (§4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nf_nat preserves the internal source port only while the translated 5-tuple is unique and otherwise allocates another port for that destination, the masquerade random and fully-random flags ze lowers randomize per connection, and VPP NAT44-ED maps per endpoint; internal/plugins/firewall/nft/lower_linux.go lowerMasquerade |
| `RFC5382-4.2-1` | REQ-2: A NAT MUST support all valid sequences of TCP packets (defined in [RFC0793]) for connections initiated both internally as well as externally when the connection is permitted by the NAT. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux nf_conntrack TCP tracker; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and the tracker follows every RFC 793 state transition of the translated connection |
| `RFC5382-4.2-2` | In addition to handling the TCP 3-way handshake mode of connection initiation, A NAT MUST handle the TCP simultaneous- open mode of connection initiation. (§4.2) | MUST | 4.2 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux nf_conntrack TCP tracker; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and the tracker accepts a SYN answered by a SYN as a simultaneous open |
| `RFC5382-4.3-1` | REQ-4: A NAT MUST NOT respond to an unsolicited inbound SYN packet for at least 6 seconds after the packet is received. (§4.3) | MUST NOT | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze installs nothing that holds an unsolicited inbound SYN; one addressed to the NAT's external address matches no conntrack entry, reaches the NAT's own Linux TCP stack and is answered with a RST at once unless an operator input filter drops it |
| `RFC5382-4.3-2` | If during this interval the NAT receives and translates an outbound SYN for the connection the NAT MUST silently drop the original unsolicited inbound SYN packet. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no unsolicited SYN is held (RFC5382-4.3-1), so a later outbound SYN cannot cause it to be dropped |
| `RFC5382-4.3-3` | The NAT MUST silently drop the original SYN packet if sending a response violates the security policy of the NAT. (§4.3) | MUST | 4.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** whether the NAT's own TCP stack answers with a RST or an operator filter drops the SYN is decided by the input ruleset, and ze ties no NAT security policy to that choice |
| `RFC5382-5-1` | In such cases, the value of the "established connection idle-timeout" MUST NOT be less than 2 hours 4 minutes. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux nf_conntrack; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and no timeout policy, so an established connection keeps nf_conntrack_tcp_timeout_established, 432000 seconds |
| `RFC5382-5-2` | The value of the "transitory connection idle-timeout" MUST NOT be less than 4 minutes. (§5) | MUST NOT | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze installs no conntrack timeout policy, so Linux's transitory TCP timeouts apply, from 10 seconds (close) to 120 seconds (syn_sent, time_wait) |
| `RFC5382-7.1-1` | REQ-7: A NAT MUST NOT have a "Port assignment" behavior of "Port overloading" for TCP. (§7.1) | MUST NOT | 7.1 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux nf_nat; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the masquerade expression and nf_nat's unique-tuple allocation never gives two internal endpoints one external tuple |
| `RFC5382-7.2-1` | REQ-8: A NAT MUST support "hairpinning" for TCP. a) A NAT's hairpinning behavior MUST be of type "External source IP address and port". (§7.2) | MUST | 7.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze generates no hairpin rule, and a SYN from an internal host to another internal host's external mapping matches no conntrack reply tuple and is delivered to the NAT itself, so no hairpin exists and no hairpin source is translated |
| `RFC5382-7.3-1` | REQ-10: Receipt of any sort of ICMP message MUST NOT terminate the NAT mapping or TCP connection for which the ICMP was generated. (§7.3) | MUST NOT | 7.3 | **positive:** no positive test. **negative:** no negative test. **{lower-layer}:** Linux nf_conntrack; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and nf_conntrack classifies an ICMP error as RELATED to the connection without deleting it |
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
| [`RFC5382-4.2-1`](#rfc5382-4.2-1) REQ-2: A NAT MUST support all valid sequences of TCP packets (defined in [RFC0793]) for connections initiated both internally as well as externally when the connection is permitted by the NAT. (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux nf_conntrack TCP tracker; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and the tracker follows every RFC 793 state transition of the translated connection |
| [`RFC5382-4.2-2`](#rfc5382-4.2-2) In addition to handling the TCP 3-way handshake mode of connection initiation, A NAT MUST handle the TCP simultaneous- open mode of connection initiation. (§4.2) | no test | no test carries this requirement id; annotated {lower-layer}: Linux nf_conntrack TCP tracker; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and the tracker accepts a SYN answered by a SYN as a simultaneous open |
| [`RFC5382-4.3-1`](#rfc5382-4.3-1) REQ-4: A NAT MUST NOT respond to an unsolicited inbound SYN packet for at least 6 seconds after the packet is received. (§4.3) | {gap}, no test | ze installs nothing that holds an unsolicited inbound SYN; one addressed to the NAT's external address matches no conntrack entry, reaches the NAT's own Linux TCP stack and is answered with a RST at once unless an operator input filter drops it |
| [`RFC5382-4.3-2`](#rfc5382-4.3-2) If during this interval the NAT receives and translates an outbound SYN for the connection the NAT MUST silently drop the original unsolicited inbound SYN packet. (§4.3) | {gap}, no test | no unsolicited SYN is held (RFC5382-4.3-1), so a later outbound SYN cannot cause it to be dropped |
| [`RFC5382-4.3-3`](#rfc5382-4.3-3) The NAT MUST silently drop the original SYN packet if sending a response violates the security policy of the NAT. (§4.3) | {gap}, no test | whether the NAT's own TCP stack answers with a RST or an operator filter drops the SYN is decided by the input ruleset, and ze ties no NAT security policy to that choice |
| [`RFC5382-5-1`](#rfc5382-5-1) In such cases, the value of the "established connection idle-timeout" MUST NOT be less than 2 hours 4 minutes. (§5) | no test | no test carries this requirement id; annotated {lower-layer}: Linux nf_conntrack; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and no timeout policy, so an established connection keeps nf_conntrack_tcp_timeout_established, 432000 seconds |
| [`RFC5382-5-2`](#rfc5382-5-2) The value of the "transitory connection idle-timeout" MUST NOT be less than 4 minutes. (§5) | {gap}, no test | ze installs no conntrack timeout policy, so Linux's transitory TCP timeouts apply, from 10 seconds (close) to 120 seconds (syn_sent, time_wait) |
| [`RFC5382-7.1-1`](#rfc5382-7.1-1) REQ-7: A NAT MUST NOT have a "Port assignment" behavior of "Port overloading" for TCP. (§7.1) | no test | no test carries this requirement id; annotated {lower-layer}: Linux nf_nat; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the masquerade expression and nf_nat's unique-tuple allocation never gives two internal endpoints one external tuple |
| [`RFC5382-7.2-1`](#rfc5382-7.2-1) REQ-8: A NAT MUST support "hairpinning" for TCP. a) A NAT's hairpinning behavior MUST be of type "External source IP address and port". (§7.2) | {gap}, no test | ze generates no hairpin rule, and a SYN from an internal host to another internal host's external mapping matches no conntrack reply tuple and is delivered to the NAT itself, so no hairpin exists and no hairpin source is translated |
| [`RFC5382-7.3-1`](#rfc5382-7.3-1) REQ-10: Receipt of any sort of ICMP message MUST NOT terminate the NAT mapping or TCP connection for which the ICMP was generated. (§7.3) | no test | no test carries this requirement id; annotated {lower-layer}: Linux nf_conntrack; internal/plugins/firewall/nft/lower_linux.go::lowerMasquerade installs the NAT rule and nf_conntrack classifies an ICMP error as RELATED to the connection without deleting it |

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
