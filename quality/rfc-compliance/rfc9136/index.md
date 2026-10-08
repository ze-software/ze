# RFC 9136 - IP Prefix Advertisement in Ethernet VPN (EVPN)

Partial. Every requirement this repository extracted from RFC 9136, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 14.3% | 2 of 14 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 14.3% | 2 of 14 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 14 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 14 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 25.0% | 3 of 12 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 14 | of 20 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 5 | of 14 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 35.7% | 5 of 14 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 14 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 14 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 35.7% | 5 of 14 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 8 shares marked as a part above are the whole of the 14 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 20 |
| Gated MUST-level | 14 |
| Not applicable, so out of scope | 5 |
| Declared gaps | 5 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 13 |
| Tagged units | 12 |
| Recorded audit verdicts | 3 |
| Discrimination records | 3 |
| Summary | `rfc/short/rfc9136.md` |
| Requirement shard | `rfc/requirements/rfc9136.md` |
| RFC text | `rfc/full/rfc9136.txt` |

## Enrolment

Enrolled: EVPN IP Prefix / RT-5 (RFC 9136): full RT-5 wire codec; 2 MET (length 34/58, prefix-length bound) + 2 single-polarity positive (same-family, RD/etag) + 5 gap (overlay-index validations) + 5 not-applicable (forwarding/install/Router's-MAC EC)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

RT-5 (IP Prefix) NLRI encode/decode for IPv4 (len 34) and IPv6 (len 58): RD, ESI, Ethernet Tag, prefix-length bounds (<=32/<=128), prefix, gateway, and MPLS label stack, with length-field and prefix-length validation ([`internal/component/bgp/plugins/nlri/evpn/types.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types.go)).

**What the ledger says remains**

Five MUST gaps annotated in [`rfc/short/rfc9136.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9136.md) ([`RFC9136-3.1-4`](#rfc9136-3.1-4)/3.1-6 ESI/GW-IP zero-unless-overlay-index, 3.2-1 ESI/GW mutual exclusion, 3.1-7/3.1-9 zero-label-without-overlay-index treat-as-withdraw): ze models no overlay index. Five further MUSTs (3.1-8, 3.2-2, 3-1, 3.2-3, 3.2-4) bind the ingress-NVE/PE forwarding, IP-VRF install, and EVPN Router's-MAC Extended Community roles ze does not play.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 2 | one part of the gated population |
| Annotated (including scoped evidence) | 12 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **14** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC9136-3.1-1`](#rfc9136-3.1-1), [`RFC9136-3.1-5`](#rfc9136-3.1-5)

**Annotated (including scoped evidence) (12):** [`RFC9136-3.1-2`](#rfc9136-3.1-2), [`RFC9136-3.1-3`](#rfc9136-3.1-3), [`RFC9136-3.1-4`](#rfc9136-3.1-4), [`RFC9136-3.1-6`](#rfc9136-3.1-6), [`RFC9136-3.2-1`](#rfc9136-3.2-1), [`RFC9136-3.1-7`](#rfc9136-3.1-7), [`RFC9136-3.1-8`](#rfc9136-3.1-8), [`RFC9136-3.1-9`](#rfc9136-3.1-9), [`RFC9136-3.2-2`](#rfc9136-3.2-2), [`RFC9136-3-1`](#rfc9136-3-1), [`RFC9136-3.2-3`](#rfc9136-3.2-3), [`RFC9136-3.2-4`](#rfc9136-3.2-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9136-3.1-1` | * The Length field of the BGP EVPN NLRI for an EVPN IP Prefix route MUST be either 34 (if IPv4 addresses are carried) or 58 (if IPv6 addresses are carried). (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L142). **positive:** `unit/verify` [`TestEVPNType5IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L177). **positive:** `unit/verify` [`TestRFC9136IPPrefixRouteSentWithLength34Or58`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_length_test.go#L22). **negative:** `unit/verify` [`TestEVPNType5InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L205). **negative:** `unit/verify` [`TestRFC9136IPPrefixRouteNeverSentWithAnotherLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_length_test.go#L53). **negative:** `unit/verify` [`TestRFC9136IPPrefixRouteReceivedWithAnotherLengthIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_length_test.go#L71) |
| `RFC9136-3.1-2` | IP prefix and gateway IP address MUST be from the same IP address family (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEVPNType5RoundTripIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L884). **positive:** `unit/verify` [`TestEVPNType5RoundTripIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L915). **negative:** no negative test. **{single-polarity}:** the single Length field fixes both prefix and gateway to one family on decode and encode, so a cross-family pair is unrepresentable on the wire and has no negative case to reject (internal/component/bgp/plugins/nlri/evpn/types.go:780-800) |
| `RFC9136-3.1-3` | * The Route Distinguisher (RD) and Ethernet Tag ID MUST be used as defined in [RFC7432] and [RFC8365]. (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L144). **negative:** no negative test. **{single-polarity}:** RD (8 octets) and Ethernet Tag (uint32) are decoded, carried, and re-encoded per the RFC 7432/8365 wire layout, and any uint32 tag is valid so there is no malformed-input negative for the field ze handles (internal/component/bgp/plugins/nlri/evpn/types.go:763-774) |
| `RFC9136-3.1-4` | * The Ethernet Segment Identifier MUST be a non-zero 10-octet identifier if the ESI is used as an Overlay Index (see the definition of "Overlay Index" in Section 3.2). It MUST be all bytes zero otherwise. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes and carries the 10-octet ESI but models no overlay-index concept, so it never enforces the zero-unless-used-as-overlay-index constraint (internal/component/bgp/plugins/nlri/evpn/types.go::parseEVPNType5 and EVPNType5.WriteTo). |
| `RFC9136-3.1-5` | The value MUST NOT be greater than 128. (S3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L143). **positive:** `unit/verify` [`TestEVPNType5IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L178). **negative:** `unit/verify` [`TestEVPNType5PrefixLengthTooLong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L235) |
| `RFC9136-3.1-6` | The GW IP field MUST be all bytes zero if it is not used as an Overlay Index. (S3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes and re-encodes the gateway field verbatim but has no overlay-index model, so it never enforces gateway-zero-unless-used-as-overlay-index (internal/component/bgp/plugins/nlri/evpn/types.go:788, :798, :854, :863) |
| `RFC9136-3.2-1` | However, they MUST NOT both be non-zero at the same time. (S3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseEVPNType5 reads both ESI and gateway without any mutual-exclusion validation, so it never treats a both-non-zero RT-5 as a withdraw (internal/component/bgp/plugins/nlri/evpn/types.go:770, :788-800) |
| `RFC9136-3.1-7` | If the received MPLS label value is zero, the route MUST contain an Overlay Index (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses the label and the overlay-candidate fields but never validates that a zero label is accompanied by an overlay index (internal/component/bgp/plugins/nlri/evpn/types.go:808-814) |
| `RFC9136-3.1-8` | If the received MPLS label value is zero, the route MUST contain an Overlay Index, and the ingress NVE/PE MUST perform a recursive resolution to find the egress NVE/ PE. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** recursive resolution to an egress NVE/PE is a forwarding/IP-VRF role ze does not play; ze propagates RT-5 without resolving or installing it (sysrib/fib carry no EVPN handling) |
| `RFC9136-3.1-9` | If the received label is zero and the route does not contain an Overlay Index, it MUST be "treat as withdraw" [RFC7606]. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this receive-side content validation could be applied to the parsed NLRI, but ze performs no treat-as-withdraw for a zero-label / no-overlay-index RT-5 (internal/component/bgp/plugins/nlri/evpn/types.go:754-814) |
| `RFC9136-3.2-2` | * Irrespective of the recursive resolution, if there is no IGP or BGP route to the BGP next hop of an RT-5, BGP MUST NOT install the RT-5 even if the Overlay Index can be resolved. (S3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never installs RT-5 into a FIB/IP-VRF, so a next-hop-reachability install gate binds a role it does not play (internal/component/sysrib and internal/plugins/fib carry no EVPN handling) |
| `RFC9136-3-1` | In case two or more NVEs are attached to different BDs of the same tenant, they MUST support the RT-5 for the proper inter-subnet forwarding operation of the tenant. (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the requirement binds NVEs performing inter-subnet forwarding between broadcast domains; ze supports RT-5 on the wire but performs no such forwarding (internal/component/bgp/plugins/nlri/evpn/types.go:743) |
| `RFC9136-3.2-3` | The encoding of a MAC address MUST be the 6-octet MAC address specified by [IEEE-802.1Q]. (S3.2, Table 1) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this constrains the MAC inside the EVPN Router's MAC Extended Community (the optional MAC overlay-index feature), which ze does not implement or interpret |
| `RFC9136-3.2-4` | The route MUST be treat as withdraw in case of an invalid MAC address. (S3.2, Table 1) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** detecting an invalid MAC requires interpreting the Router's MAC Extended Community, an optional feature ze neither parses nor validates |
| `RFC9136-3.1-10` | When sending, the label value SHOULD be zero if a recursive resolution based on an Overlay Index is used. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-5` | A route containing a non-zero GW IP and a non-zero ESI (at the same time) SHOULD be treat as withdraw [RFC7606]. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-6` | A route where ESI, GW IP, MAC, and Label are all zero at the same time SHOULD be treat as withdraw. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-7` | Note that the advertising NVE/PE that sets the Overlay Index SHOULD advertise an RT-2 for the MAC Overlay Index if there are receiving NVE/PEs configured to use the MAC as the Overlay Index. (§3.2, Table 1) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.1-11` | An IP Prefix route MAY be sent along with an EVPN Router's MAC Extended Community (defined in [RFC9135]) to carry the MAC address that is used as the Overlay Index. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-8` | The support of a MAC Overlay Index in this model is OPTIONAL. (§3.2, Table 1) | OPTIONAL | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9136-3.1-4`](#rfc9136-3.1-4) * The Ethernet Segment Identifier MUST be a non-zero 10-octet identifier if the ESI is used as an Overlay Index (see the definition of "Overlay Index" in Section 3.2). It MUST be all bytes zero otherwise. (§3.1) | {gap}, no test | ze decodes and carries the 10-octet ESI but models no overlay-index concept, so it never enforces the zero-unless-used-as-overlay-index constraint (internal/component/bgp/plugins/nlri/evpn/types.go::parseEVPNType5 and EVPNType5.WriteTo). |
| [`RFC9136-3.1-6`](#rfc9136-3.1-6) The GW IP field MUST be all bytes zero if it is not used as an Overlay Index. (S3.1) | {gap}, no test | ze decodes and re-encodes the gateway field verbatim but has no overlay-index model, so it never enforces gateway-zero-unless-used-as-overlay-index (internal/component/bgp/plugins/nlri/evpn/types.go:788, :798, :854, :863) |
| [`RFC9136-3.2-1`](#rfc9136-3.2-1) However, they MUST NOT both be non-zero at the same time. (S3.2) | {gap}, no test | parseEVPNType5 reads both ESI and gateway without any mutual-exclusion validation, so it never treats a both-non-zero RT-5 as a withdraw (internal/component/bgp/plugins/nlri/evpn/types.go:770, :788-800) |
| [`RFC9136-3.1-7`](#rfc9136-3.1-7) If the received MPLS label value is zero, the route MUST contain an Overlay Index (§3.1) | {gap}, no test | ze parses the label and the overlay-candidate fields but never validates that a zero label is accompanied by an overlay index (internal/component/bgp/plugins/nlri/evpn/types.go:808-814) |
| [`RFC9136-3.1-8`](#rfc9136-3.1-8) If the received MPLS label value is zero, the route MUST contain an Overlay Index, and the ingress NVE/PE MUST perform a recursive resolution to find the egress NVE/ PE. (§3.1) | no test | no test carries this requirement id; annotated {not-applicable}: recursive resolution to an egress NVE/PE is a forwarding/IP-VRF role ze does not play; ze propagates RT-5 without resolving or installing it (sysrib/fib carry no EVPN handling) |
| [`RFC9136-3.1-9`](#rfc9136-3.1-9) If the received label is zero and the route does not contain an Overlay Index, it MUST be "treat as withdraw" [RFC7606]. (§3.1) | {gap}, no test | this receive-side content validation could be applied to the parsed NLRI, but ze performs no treat-as-withdraw for a zero-label / no-overlay-index RT-5 (internal/component/bgp/plugins/nlri/evpn/types.go:754-814) |
| [`RFC9136-3.2-2`](#rfc9136-3.2-2) * Irrespective of the recursive resolution, if there is no IGP or BGP route to the BGP next hop of an RT-5, BGP MUST NOT install the RT-5 even if the Overlay Index can be resolved. (S3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never installs RT-5 into a FIB/IP-VRF, so a next-hop-reachability install gate binds a role it does not play (internal/component/sysrib and internal/plugins/fib carry no EVPN handling) |
| [`RFC9136-3-1`](#rfc9136-3-1) In case two or more NVEs are attached to different BDs of the same tenant, they MUST support the RT-5 for the proper inter-subnet forwarding operation of the tenant. (S3) | no test | no test carries this requirement id; annotated {not-applicable}: the requirement binds NVEs performing inter-subnet forwarding between broadcast domains; ze supports RT-5 on the wire but performs no such forwarding (internal/component/bgp/plugins/nlri/evpn/types.go:743) |
| [`RFC9136-3.2-3`](#rfc9136-3.2-3) The encoding of a MAC address MUST be the 6-octet MAC address specified by [IEEE-802.1Q]. (S3.2, Table 1) | no test | no test carries this requirement id; annotated {not-applicable}: this constrains the MAC inside the EVPN Router's MAC Extended Community (the optional MAC overlay-index feature), which ze does not implement or interpret |
| [`RFC9136-3.2-4`](#rfc9136-3.2-4) The route MUST be treat as withdraw in case of an invalid MAC address. (S3.2, Table 1) | no test | no test carries this requirement id; annotated {not-applicable}: detecting an invalid MAC requires interpreting the Router's MAC Extended Community, an optional feature ze neither parses nor validates |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9136-3.1-1`](#rfc9136-3.1-1)

* The Length field of the BGP EVPN NLRI for an EVPN IP Prefix route MUST be either 34 (if IPv4 addresses are carried) or 58 (if IPv6 addresses are carried). (S3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-02 (BGP c31 judge). RFC 9136 Section 3.1: "The Length field of the BGP EVPN NLRI for an EVPN IP Prefix route MUST be either 34 (if IPv4 addresses are carried) or 58 (if IPv6 addresses are carried)." Send: evpn rfc9136_length_test.go::TestRFC9136IPPrefixRouteSentWithLength34Or58 (EncodeNLRIHex type5, IPv4 with and without label -> 05 22 + 34 octets, IPv6 -> 05 3A + 58) and ::TestRFC9136IPPrefixRouteNeverSentWithAnotherLength (two labels refused, nothing encoded). Receive: ::TestRFC9136IPPrefixRouteReceivedWithAnotherLengthIsRefused (Length 33/35/57/59 -> ErrorIs ErrEVPNInvalidAddress) beside the HEAD 34/58 decode positives. The InProcessRouteEncoder path (encode.go EncodeRoute) builds type 5 with exactly one label and shares (*EVPNType5).WriteTo, so the same Length holds there by construction; it is not driven by these units. Judge overlay: the parse guard reduced to `len(data) < 34` turns only the receive negative red. Records on buildEVPNFromParams (+/-) and parseEVPNType5 (-).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9136IPPrefixRouteNeverSentWithAnotherLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_length_test.go#L53) | unit/verify | revert, verified |
| negative | [`TestRFC9136IPPrefixRouteReceivedWithAnotherLengthIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_length_test.go#L71) | unit/verify | revert, verified |
| negative | [`TestEVPNType5InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L205) | unit/verify | unproven |
| positive | [`TestRFC9136IPPrefixRouteSentWithLength34Or58`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_length_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L142) | unit/verify | unproven |
| positive | [`TestEVPNType5IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L177) | unit/verify | unproven |

### [`RFC9136-3.1-2`](#rfc9136-3.1-2)

IP prefix and gateway IP address MUST be from the same IP address family (S3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEVPNType5RoundTripIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L884) | unit/verify | unproven |
| positive | [`TestEVPNType5RoundTripIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L915) | unit/verify | unproven |

### [`RFC9136-3.1-3`](#rfc9136-3.1-3)

* The Route Distinguisher (RD) and Ethernet Tag ID MUST be used as defined in [RFC7432] and [RFC8365]. (S3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive (row marker). (a) forbidden: decoding the RD or the Ethernet Tag ID at other than the RFC 7432 layout (8-octet RD, 4-octet tag after the 10-octet ESI); (b) TestEVPNType5IPv4 assert.Equal("0:65000:100", evpn.RD().String()) and assert.Equal(uint32(10), evpn.EthernetTag()) go red on a misplaced or misdecoded field. Only a type-0 RD is driven; RD type decoding is the shared ParseRouteDistinguisher.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L144) | unit/verify | unproven |

### [`RFC9136-3.1-4`](#rfc9136-3.1-4)

* The Ethernet Segment Identifier MUST be a non-zero 10-octet identifier if the ESI is used as an Overlay Index (see the definition of "Overlay Index" in Section 3.2). It MUST be all bytes zero otherwise. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.1-4, so no unit is bound to it.

### [`RFC9136-3.1-5`](#rfc9136-3.1-5)

The value MUST NOT be greater than 128. (S3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: accepting an IP Prefix Length above 128; (b) TestEVPNType5PrefixLengthTooLong/ipv6 require.ErrorIs(err, ErrEVPNInvalidPrefix) on length 129 goes red if parseEVPNType5 accepts it; positive TestEVPNType5IPv6 decodes 64. The send side cannot exceed 128 (netip.Prefix). The ipv4 subtest's tag (33 > 32) proves the non-normative 'can be set to a value between 0 and 32' sentence, not this MUST NOT: a misplaced tag, listed as a finding.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEVPNType5PrefixLengthTooLong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L235) | unit/verify | unproven |
| positive | [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L143) | unit/verify | unproven |
| positive | [`TestEVPNType5IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L178) | unit/verify | unproven |

### [`RFC9136-3.1-6`](#rfc9136-3.1-6)

The GW IP field MUST be all bytes zero if it is not used as an Overlay Index. (S3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.1-6, so no unit is bound to it.

### [`RFC9136-3.2-1`](#rfc9136-3.2-1)

However, they MUST NOT both be non-zero at the same time. (S3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.2-1, so no unit is bound to it.

### [`RFC9136-3.1-7`](#rfc9136-3.1-7)

If the received MPLS label value is zero, the route MUST contain an Overlay Index (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.1-7, so no unit is bound to it.

### [`RFC9136-3.1-8`](#rfc9136-3.1-8)

If the received MPLS label value is zero, the route MUST contain an Overlay Index, and the ingress NVE/PE MUST perform a recursive resolution to find the egress NVE/ PE. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.1-8, so no unit is bound to it.

### [`RFC9136-3.1-9`](#rfc9136-3.1-9)

If the received label is zero and the route does not contain an Overlay Index, it MUST be "treat as withdraw" [RFC7606]. (§3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.1-9, so no unit is bound to it.

### [`RFC9136-3.2-2`](#rfc9136-3.2-2)

* Irrespective of the recursive resolution, if there is no IGP or BGP route to the BGP next hop of an RT-5, BGP MUST NOT install the RT-5 even if the Overlay Index can be resolved. (S3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.2-2, so no unit is bound to it.

### [`RFC9136-3-1`](#rfc9136-3-1)

In case two or more NVEs are attached to different BDs of the same tenant, they MUST support the RT-5 for the proper inter-subnet forwarding operation of the tenant. (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3-1, so no unit is bound to it.

### [`RFC9136-3.2-3`](#rfc9136-3.2-3)

The encoding of a MAC address MUST be the 6-octet MAC address specified by [IEEE-802.1Q]. (S3.2, Table 1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.2-3, so no unit is bound to it.

### [`RFC9136-3.2-4`](#rfc9136-3.2-4)

The route MUST be treat as withdraw in case of an invalid MAC address. (S3.2, Table 1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.2-4, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc9136.txt |
| Source fingerprint | 3cd1f114ac072151 |
| Record | rfc/extraction/rfc9136.json |
| Mapped sentences | 13 |
| Declined as scope | 1 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `2.1` | not stated | 0 | walked | not stated |
| `2.2` | not stated | 0 | walked | not stated |
| `3` | not stated | 1 | walked | not stated |
| `3.1` | not stated | 9 | walked | not stated |
| `3.2` | not stated | 4 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 0 | walked | not stated |
| `4.2` | not stated | 0 | walked | not stated |
| `4.3` | not stated | 0 | walked | not stated |
| `4.4` | not stated | 0 | walked | not stated |
| `4.4.1` | not stated | 0 | walked | not stated |
| `4.4.2` | not stated | 0 | walked | not stated |
| `4.4.3` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The restored RFC9136-3.1-4 quotation includes the ESI antecedent and this complete second sentence: "It MUST be all bytes zero otherwise." Both sites therefore target the same two-part ESI constraint; its Overlay Index implementation gap remains. | It MUST be all bytes zero otherwise. |

## Superseded

No document obsoletes RFC 9136, so its obligations are stated where they were written.
