# RFC 9136 - IP Prefix Advertisement in Ethernet VPN (EVPN)

Partial. Every requirement this repository extracted from RFC 9136, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 22.2% | 2 of 9 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 22.2% | 2 of 9 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 9 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 14 | of 20 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 5 | of 14 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 55.6% | 5 of 9 binding obligations | no test carries the requirement id, whether or not a gap states why |

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
| Requirements | 20 |
| Gated MUST-level | 14 |
| Obligations that bind Ze | 9 |
| Not applicable, so out of scope | 5 |
| Declared gaps | 5 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 10 |
| Tagged units | 9 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
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
| Annotated instead of tested | 12 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **14** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (2):** [`RFC9136-3.1-1`](#rfc9136-3.1-1), [`RFC9136-3.1-5`](#rfc9136-3.1-5)

**Annotated instead of tested (12):** [`RFC9136-3.1-2`](#rfc9136-3.1-2), [`RFC9136-3.1-3`](#rfc9136-3.1-3), [`RFC9136-3.1-4`](#rfc9136-3.1-4), [`RFC9136-3.1-6`](#rfc9136-3.1-6), [`RFC9136-3.2-1`](#rfc9136-3.2-1), [`RFC9136-3.1-7`](#rfc9136-3.1-7), [`RFC9136-3.1-8`](#rfc9136-3.1-8), [`RFC9136-3.1-9`](#rfc9136-3.1-9), [`RFC9136-3.2-2`](#rfc9136-3.2-2), [`RFC9136-3-1`](#rfc9136-3-1), [`RFC9136-3.2-3`](#rfc9136-3.2-3), [`RFC9136-3.2-4`](#rfc9136-3.2-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9136-3.1-1` | Length field MUST be either 34 (IPv4) or 58 (IPv6) (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L142). **positive:** `unit/verify` [`TestEVPNType5IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L177). **negative:** `unit/verify` [`TestEVPNType5InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L205) |
| `RFC9136-3.1-2` | IP prefix and gateway IP address MUST be from the same IP address family (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEVPNType5RoundTripIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L849). **positive:** `unit/verify` [`TestEVPNType5RoundTripIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L880). **negative:** no negative test. **{single-polarity}:** the single Length field fixes both prefix and gateway to one family on decode and encode, so a cross-family pair is unrepresentable on the wire and has no negative case to reject (internal/component/bgp/plugins/nlri/evpn/types.go:780-800) |
| `RFC9136-3.1-3` | Route Distinguisher (RD) and Ethernet Tag ID MUST be used as defined in RFC 7432 and RFC 8365 (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L144). **negative:** no negative test. **{single-polarity}:** RD (8 octets) and Ethernet Tag (uint32) are decoded, carried, and re-encoded per the RFC 7432/8365 wire layout, and any uint32 tag is valid so there is no malformed-input negative for the field ze handles (internal/component/bgp/plugins/nlri/evpn/types.go:763-774) |
| `RFC9136-3.1-4` | ESI MUST be a non-zero 10-octet identifier if used as Overlay Index; MUST be all zeros otherwise (S3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes and carries the 10-octet ESI but models no overlay-index concept, so it never enforces the zero-unless-used-as-overlay-index constraint (internal/component/bgp/plugins/nlri/evpn/types.go:770) |
| `RFC9136-3.1-5` | IP Prefix Length value MUST NOT be greater than 128 (S3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L143). **positive:** `unit/verify` [`TestEVPNType5IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L178). **negative:** `unit/verify` [`TestEVPNType5PrefixLengthTooLong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L235) |
| `RFC9136-3.1-6` | GW IP field MUST be all bytes zero if not used as an Overlay Index (S3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze decodes and re-encodes the gateway field verbatim but has no overlay-index model, so it never enforces gateway-zero-unless-used-as-overlay-index (internal/component/bgp/plugins/nlri/evpn/types.go:788, :798, :854, :863) |
| `RFC9136-3.2-1` | ESI and GW IP MUST NOT both be non-zero simultaneously (S3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseEVPNType5 reads both ESI and gateway without any mutual-exclusion validation, so it never treats a both-non-zero RT-5 as a withdraw (internal/component/bgp/plugins/nlri/evpn/types.go:770, :788-800) |
| `RFC9136-3.1-7` | If received label is zero, route MUST contain an Overlay Index (S3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses the label and the overlay-candidate fields but never validates that a zero label is accompanied by an overlay index (internal/component/bgp/plugins/nlri/evpn/types.go:808-814) |
| `RFC9136-3.1-8` | If received label is zero, ingress NVE/PE MUST perform recursive resolution (S3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** recursive resolution to an egress NVE/PE is a forwarding/IP-VRF role ze does not play; ze propagates RT-5 without resolving or installing it (sysrib/fib carry no EVPN handling) |
| `RFC9136-3.1-9` | If received label is zero and no Overlay Index, MUST treat as withdraw (S3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this receive-side content validation could be applied to the parsed NLRI, but ze performs no treat-as-withdraw for a zero-label / no-overlay-index RT-5 (internal/component/bgp/plugins/nlri/evpn/types.go:754-814) |
| `RFC9136-3.2-2` | If no IGP or BGP route to BGP next hop of RT-5, MUST NOT install even if Overlay Index resolves (S3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze never installs RT-5 into a FIB/IP-VRF, so a next-hop-reachability install gate binds a role it does not play (internal/component/sysrib and internal/plugins/fib carry no EVPN handling) |
| `RFC9136-3-1` | NVEs attached to different BDs of same tenant MUST support RT-5 for proper inter-subnet forwarding (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the requirement binds NVEs performing inter-subnet forwarding between broadcast domains; ze supports RT-5 on the wire but performs no such forwarding (internal/component/bgp/plugins/nlri/evpn/types.go:743) |
| `RFC9136-3.2-3` | MAC address encoding MUST be 6-octet MAC address per IEEE 802.1Q (S3.2, Table 1) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** this constrains the MAC inside the EVPN Router's MAC Extended Community (the optional MAC overlay-index feature), which ze does not implement or interpret |
| `RFC9136-3.2-4` | Route MUST be treat as withdraw if MAC address is broadcast or multicast (S3.2, Table 1) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** detecting a broadcast/multicast MAC requires interpreting the Router's MAC Extended Community, an optional feature ze neither parses nor validates |
| `RFC9136-3.1-10` | Label value SHOULD be zero if recursive resolution via Overlay Index is used (S3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-5` | Route with non-zero GW IP and non-zero ESI simultaneously SHOULD be treat as withdraw (S3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-6` | Route where ESI, GW IP, MAC, and Label are all zero SHOULD be treat as withdraw (S3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-7` | Advertising NVE/PE SHOULD advertise RT-2 for MAC Overlay Index if receivers use MAC as Overlay Index (S3.2, Table 1) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.1-11` | IP Prefix route MAY be sent with EVPN Router's MAC Extended Community (S3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-8` | Support of MAC Overlay Index in IP-VRF-to-IP-VRF model is OPTIONAL (S3.2, Table 1) | OPTIONAL | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9136-3.1-4`](#rfc9136-3.1-4) ESI MUST be a non-zero 10-octet identifier if used as Overlay Index; MUST be all zeros otherwise (S3.1) | {gap}, no test | ze decodes and carries the 10-octet ESI but models no overlay-index concept, so it never enforces the zero-unless-used-as-overlay-index constraint (internal/component/bgp/plugins/nlri/evpn/types.go:770) |
| [`RFC9136-3.1-6`](#rfc9136-3.1-6) GW IP field MUST be all bytes zero if not used as an Overlay Index (S3.1) | {gap}, no test | ze decodes and re-encodes the gateway field verbatim but has no overlay-index model, so it never enforces gateway-zero-unless-used-as-overlay-index (internal/component/bgp/plugins/nlri/evpn/types.go:788, :798, :854, :863) |
| [`RFC9136-3.2-1`](#rfc9136-3.2-1) ESI and GW IP MUST NOT both be non-zero simultaneously (S3.2) | {gap}, no test | parseEVPNType5 reads both ESI and gateway without any mutual-exclusion validation, so it never treats a both-non-zero RT-5 as a withdraw (internal/component/bgp/plugins/nlri/evpn/types.go:770, :788-800) |
| [`RFC9136-3.1-7`](#rfc9136-3.1-7) If received label is zero, route MUST contain an Overlay Index (S3.1) | {gap}, no test | ze parses the label and the overlay-candidate fields but never validates that a zero label is accompanied by an overlay index (internal/component/bgp/plugins/nlri/evpn/types.go:808-814) |
| [`RFC9136-3.1-8`](#rfc9136-3.1-8) If received label is zero, ingress NVE/PE MUST perform recursive resolution (S3.1) | no test | no test carries this requirement id; annotated {not-applicable}: recursive resolution to an egress NVE/PE is a forwarding/IP-VRF role ze does not play; ze propagates RT-5 without resolving or installing it (sysrib/fib carry no EVPN handling) |
| [`RFC9136-3.1-9`](#rfc9136-3.1-9) If received label is zero and no Overlay Index, MUST treat as withdraw (S3.1) | {gap}, no test | this receive-side content validation could be applied to the parsed NLRI, but ze performs no treat-as-withdraw for a zero-label / no-overlay-index RT-5 (internal/component/bgp/plugins/nlri/evpn/types.go:754-814) |
| [`RFC9136-3.2-2`](#rfc9136-3.2-2) If no IGP or BGP route to BGP next hop of RT-5, MUST NOT install even if Overlay Index resolves (S3.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze never installs RT-5 into a FIB/IP-VRF, so a next-hop-reachability install gate binds a role it does not play (internal/component/sysrib and internal/plugins/fib carry no EVPN handling) |
| [`RFC9136-3-1`](#rfc9136-3-1) NVEs attached to different BDs of same tenant MUST support RT-5 for proper inter-subnet forwarding (S3) | no test | no test carries this requirement id; annotated {not-applicable}: the requirement binds NVEs performing inter-subnet forwarding between broadcast domains; ze supports RT-5 on the wire but performs no such forwarding (internal/component/bgp/plugins/nlri/evpn/types.go:743) |
| [`RFC9136-3.2-3`](#rfc9136-3.2-3) MAC address encoding MUST be 6-octet MAC address per IEEE 802.1Q (S3.2, Table 1) | no test | no test carries this requirement id; annotated {not-applicable}: this constrains the MAC inside the EVPN Router's MAC Extended Community (the optional MAC overlay-index feature), which ze does not implement or interpret |
| [`RFC9136-3.2-4`](#rfc9136-3.2-4) Route MUST be treat as withdraw if MAC address is broadcast or multicast (S3.2, Table 1) | no test | no test carries this requirement id; annotated {not-applicable}: detecting a broadcast/multicast MAC requires interpreting the Router's MAC Extended Community, an optional feature ze neither parses nor validates |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9136-3.1-1`](#rfc9136-3.1-1)

Length field MUST be either 34 (IPv4) or 58 (IPv6) (S3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEVPNType5InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L205) | unit/verify | unproven |
| positive | [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L142) | unit/verify | unproven |
| positive | [`TestEVPNType5IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L177) | unit/verify | unproven |

### [`RFC9136-3.1-2`](#rfc9136-3.1-2)

IP prefix and gateway IP address MUST be from the same IP address family (S3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEVPNType5RoundTripIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L849) | unit/verify | unproven |
| positive | [`TestEVPNType5RoundTripIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L880) | unit/verify | unproven |

### [`RFC9136-3.1-3`](#rfc9136-3.1-3)

Route Distinguisher (RD) and Ethernet Tag ID MUST be used as defined in RFC 7432 and RFC 8365 (S3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L144) | unit/verify | unproven |

### [`RFC9136-3.1-4`](#rfc9136-3.1-4)

ESI MUST be a non-zero 10-octet identifier if used as Overlay Index; MUST be all zeros otherwise (S3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.1-4, so no unit is bound to it.

### [`RFC9136-3.1-5`](#rfc9136-3.1-5)

IP Prefix Length value MUST NOT be greater than 128 (S3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestEVPNType5PrefixLengthTooLong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L235) | unit/verify | unproven |
| positive | [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L143) | unit/verify | unproven |
| positive | [`TestEVPNType5IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L178) | unit/verify | unproven |

### [`RFC9136-3.1-6`](#rfc9136-3.1-6)

GW IP field MUST be all bytes zero if not used as an Overlay Index (S3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.1-6, so no unit is bound to it.

### [`RFC9136-3.2-1`](#rfc9136-3.2-1)

ESI and GW IP MUST NOT both be non-zero simultaneously (S3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.2-1, so no unit is bound to it.

### [`RFC9136-3.1-7`](#rfc9136-3.1-7)

If received label is zero, route MUST contain an Overlay Index (S3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.1-7, so no unit is bound to it.

### [`RFC9136-3.1-8`](#rfc9136-3.1-8)

If received label is zero, ingress NVE/PE MUST perform recursive resolution (S3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.1-8, so no unit is bound to it.

### [`RFC9136-3.1-9`](#rfc9136-3.1-9)

If received label is zero and no Overlay Index, MUST treat as withdraw (S3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.1-9, so no unit is bound to it.

### [`RFC9136-3.2-2`](#rfc9136-3.2-2)

If no IGP or BGP route to BGP next hop of RT-5, MUST NOT install even if Overlay Index resolves (S3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.2-2, so no unit is bound to it.

### [`RFC9136-3-1`](#rfc9136-3-1)

NVEs attached to different BDs of same tenant MUST support RT-5 for proper inter-subnet forwarding (S3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3-1, so no unit is bound to it.

### [`RFC9136-3.2-3`](#rfc9136-3.2-3)

MAC address encoding MUST be 6-octet MAC address per IEEE 802.1Q (S3.2, Table 1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.2-3, so no unit is bound to it.

### [`RFC9136-3.2-4`](#rfc9136-3.2-4)

Route MUST be treat as withdraw if MAC address is broadcast or multicast (S3.2, Table 1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9136-3.2-4, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 9136, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9136, so its obligations are stated where they were written.
