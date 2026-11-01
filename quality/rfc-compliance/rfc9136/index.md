# RFC 9136 - IP Prefix Advertisement in Ethernet VPN (EVPN)

EVPN IP Prefix (RT-5). Every requirement this repository extracted from RFC 9136, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 14 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 14 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 14 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 14 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 7 | 7 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 14 | of 20 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 14 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 14 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 14 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 14 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 14 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 8 | of 14 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 9 counts marked as a part above are the whole of the 14 gated MUSTs: they add up to 14. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

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
| Public row | EVPN IP Prefix (RT-5) |
| Enrolment | Enrolled |
| Requirements | 20 |
| Gated MUST-level | 14 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 8 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 25 |
| Tagged units | 16 |
| Recorded audit verdicts | 5 |
| Discrimination records | 7 |
| Summary | `rfc/short/rfc9136.md` |
| Requirement shard | `rfc/requirements/rfc9136.md` |
| RFC text | `rfc/full/rfc9136.txt` |

## Enrolment

Enrolled: EVPN IP Prefix / RT-5 (RFC 9136): full RT-5 wire codec; 2 MET (length 34/58, prefix-length bound) + 2 single-polarity positive (same-family, RD/etag) + 8 gap (overlay-index validations, EVPN forwarding and install, Router's MAC EC)

## What the public ledger says

**Area:** EVPN IP Prefix (RT-5)

**Proof:** 3 of 14 MUST proven

**Evidence:** unit 3

**Unmet:** RFC9136-3-1, RFC9136-3.1-7, RFC9136-3.1-8, RFC9136-3.1-9, RFC9136-3.2-1, RFC9136-3.2-2, RFC9136-3.2-3, RFC9136-3.2-4


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the gated population |
| Annotated (including scoped evidence) | 10 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **14** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (4):** [`RFC9136-3.1-1`](#rfc9136-3.1-1), [`RFC9136-3.1-4`](#rfc9136-3.1-4), [`RFC9136-3.1-5`](#rfc9136-3.1-5), [`RFC9136-3.1-6`](#rfc9136-3.1-6)

**Annotated (including scoped evidence) (10):** [`RFC9136-3.1-2`](#rfc9136-3.1-2), [`RFC9136-3.1-3`](#rfc9136-3.1-3), [`RFC9136-3.2-1`](#rfc9136-3.2-1), [`RFC9136-3.1-7`](#rfc9136-3.1-7), [`RFC9136-3.1-8`](#rfc9136-3.1-8), [`RFC9136-3.1-9`](#rfc9136-3.1-9), [`RFC9136-3.2-2`](#rfc9136-3.2-2), [`RFC9136-3-1`](#rfc9136-3-1), [`RFC9136-3.2-3`](#rfc9136-3.2-3), [`RFC9136-3.2-4`](#rfc9136-3.2-4)

## Figures

- **MUST bar**: Ze proven 3, Ze correctness gap 11
- **Overview bar**: Ze MUST proven 3, SHOULD not decided 6, Ze correctness gap 11

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 14 |
| MUST rows kept (the MUST bar) | 14 |
| Ze proven | 3 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 11 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 6 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 6 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc9136.json](../../../data/rfc/rfc9136.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9136-3.1-1` | * The Length field of the BGP EVPN NLRI for an EVPN IP Prefix route MUST be either 34 (if IPv4 addresses are carried) or 58 (if IPv6 addresses are carried). (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L142). **positive:** `unit/verify` [`TestEVPNType5IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L177). **positive:** `unit/verify` [`TestRFC9136IPPrefixRouteSentWithLength34Or58`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_length_test.go#L22). **negative:** `unit/verify` [`TestEVPNType5InvalidLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L205). **negative:** `unit/verify` [`TestRFC9136IPPrefixRouteNeverSentWithAnotherLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_length_test.go#L53). **negative:** `unit/verify` [`TestRFC9136IPPrefixRouteReceivedWithAnotherLengthIsRefused`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_length_test.go#L71) |
| `RFC9136-3.1-2` | IP prefix and gateway IP address MUST be from the same IP address family (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEVPNType5RoundTripIPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L884). **positive:** `unit/verify` [`TestEVPNType5RoundTripIPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L915). **negative:** no negative test. **{single-polarity}:** the single Length field fixes both prefix and gateway to one family on decode and encode, so a cross-family pair is unrepresentable on the wire and has no negative case to reject (internal/component/bgp/plugins/nlri/evpn/types.go:780-800) |
| `RFC9136-3.1-3` | * The Route Distinguisher (RD) and Ethernet Tag ID MUST be used as defined in [RFC7432] and [RFC8365]. (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L144). **negative:** no negative test. **{single-polarity}:** RD (8 octets) and Ethernet Tag (uint32) are decoded, carried, and re-encoded per the RFC 7432/8365 wire layout, and any uint32 tag is valid so there is no malformed-input negative for the field ze handles (internal/component/bgp/plugins/nlri/evpn/types.go:763-774) |
| `RFC9136-3.1-4` | * The Ethernet Segment Identifier MUST be a non-zero 10-octet identifier if the ESI is used as an Overlay Index (see the definition of "Overlay Index" in Section 3.2). It MUST be all bytes zero otherwise. (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9136OverlayIndexESINonZeroOnlyAsIndex`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_overlay_index_test.go#L44). **negative:** `unit/verify` [`TestRFC9136OverlayIndexESIRefusedOtherwise`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_overlay_index_test.go#L83) |
| `RFC9136-3.1-5` | The value MUST NOT be greater than 128. (S3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestEVPNType5IPv4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L143). **positive:** `unit/verify` [`TestEVPNType5IPv6`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L178). **negative:** `unit/verify` [`TestEVPNType5PrefixLengthTooLong`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/types_test.go#L235) |
| `RFC9136-3.1-6` | The GW IP field MUST be all bytes zero if it is not used as an Overlay Index. (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFC9136OverlayIndexESINonZeroOnlyAsIndex`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_overlay_index_test.go#L50). **negative:** `unit/verify` [`TestRFC9136OverlayIndexESIRefusedOtherwise`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_overlay_index_test.go#L95) |
| `RFC9136-3.2-1` | However, they MUST NOT both be non-zero at the same time. (S3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseEVPNType5 reads both ESI and gateway without any mutual-exclusion validation, so it never treats a both-non-zero RT-5 as a withdraw (internal/component/bgp/plugins/nlri/evpn/types.go:770, :788-800) |
| `RFC9136-3.1-7` | If the received MPLS label value is zero, the route MUST contain an Overlay Index (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze parses the label and the overlay-candidate fields but never validates that a zero label is accompanied by an overlay index (internal/component/bgp/plugins/nlri/evpn/types.go:808-814) |
| `RFC9136-3.1-8` | If the received MPLS label value is zero, the route MUST contain an Overlay Index, and the ingress NVE/PE MUST perform a recursive resolution to find the egress NVE/ PE. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze resolves no RT-5 and installs none, so it performs no ingress NVE/PE recursive resolution to an egress NVE/PE: it originates and propagates RT-5 on the control plane only (internal/component/bgp/plugins/nlri/evpn/types.go), and internal/component/sysrib/sysrib.go keeps every family whose NLRI is not a CIDR prefix, EVPN included, out of the system RIB, so no FIB backend sees one. No owner decision declines the EVPN data plane, so this is owed with it |
| `RFC9136-3.1-9` | If the received label is zero and the route does not contain an Overlay Index, it MUST be "treat as withdraw" [RFC7606]. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this receive-side content validation could be applied to the parsed NLRI, but ze performs no treat-as-withdraw for a zero-label / no-overlay-index RT-5 (internal/component/bgp/plugins/nlri/evpn/types.go:754-814) |
| `RFC9136-3.2-2` | * Irrespective of the recursive resolution, if there is no IGP or BGP route to the BGP next hop of an RT-5, BGP MUST NOT install the RT-5 even if the Overlay Index can be resolved. (S3.2) | MUST NOT | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: ze meets it by installing no RT-5 at all: internal/component/sysrib/sysrib.go keeps EVPN out of the system RIB, so neither the kernel nor the VPP FIB backend receives one; no test asserts it |
| `RFC9136-3-1` | In case two or more NVEs are attached to different BDs of the same tenant, they MUST support the RT-5 for the proper inter-subnet forwarding operation of the tenant. (S3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze attaches no broadcast domain and runs no inter-subnet forwarding, so it is no NVE supporting RT-5 for that operation; its RT-5 support is the wire codec (internal/component/bgp/plugins/nlri/evpn/types.go). No owner decision declines the EVPN data plane, so this is owed with it |
| `RFC9136-3.2-3` | The encoding of a MAC address MUST be the 6-octet MAC address specified by [IEEE-802.1Q]. (S3.2, Table 1) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze neither encodes nor decodes the EVPN Router's MAC Extended Community, so it encodes no MAC address in the 6-octet form this row requires; owed with the MAC overlay index |
| `RFC9136-3.2-4` | The route MUST be treat as withdraw in case of an invalid MAC address. (S3.2, Table 1) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze does not interpret the EVPN Router's MAC Extended Community, so a route carrying an invalid MAC address is not treated as withdraw; owed with the MAC overlay index |
| `RFC9136-3.1-10` | When sending, the label value SHOULD be zero if a recursive resolution based on an Overlay Index is used. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-5` | A route containing a non-zero GW IP and a non-zero ESI (at the same time) SHOULD be treat as withdraw [RFC7606]. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-6` | A route where ESI, GW IP, MAC, and Label are all zero at the same time SHOULD be treat as withdraw. (§3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-7` | Note that the advertising NVE/PE that sets the Overlay Index SHOULD advertise an RT-2 for the MAC Overlay Index if there are receiving NVE/PEs configured to use the MAC as the Overlay Index. (§3.2, Table 1) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.1-11` | An IP Prefix route MAY be sent along with an EVPN Router's MAC Extended Community (defined in [RFC9135]) to carry the MAC address that is used as the Overlay Index. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9136-3.2-8` | The support of a MAC Overlay Index in this model is OPTIONAL. (§3.2, Table 1) | OPTIONAL | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9136-3.2-1`](#rfc9136-3.2-1) However, they MUST NOT both be non-zero at the same time. (S3.2) | {gap}, no test | parseEVPNType5 reads both ESI and gateway without any mutual-exclusion validation, so it never treats a both-non-zero RT-5 as a withdraw (internal/component/bgp/plugins/nlri/evpn/types.go:770, :788-800) |
| [`RFC9136-3.1-7`](#rfc9136-3.1-7) If the received MPLS label value is zero, the route MUST contain an Overlay Index (§3.1) | {gap}, no test | ze parses the label and the overlay-candidate fields but never validates that a zero label is accompanied by an overlay index (internal/component/bgp/plugins/nlri/evpn/types.go:808-814) |
| [`RFC9136-3.1-8`](#rfc9136-3.1-8) If the received MPLS label value is zero, the route MUST contain an Overlay Index, and the ingress NVE/PE MUST perform a recursive resolution to find the egress NVE/ PE. (§3.1) | {gap}, no test | ze resolves no RT-5 and installs none, so it performs no ingress NVE/PE recursive resolution to an egress NVE/PE: it originates and propagates RT-5 on the control plane only (internal/component/bgp/plugins/nlri/evpn/types.go), and internal/component/sysrib/sysrib.go keeps every family whose NLRI is not a CIDR prefix, EVPN included, out of the system RIB, so no FIB backend sees one. No owner decision declines the EVPN data plane, so this is owed with it |
| [`RFC9136-3.1-9`](#rfc9136-3.1-9) If the received label is zero and the route does not contain an Overlay Index, it MUST be "treat as withdraw" [RFC7606]. (§3.1) | {gap}, no test | this receive-side content validation could be applied to the parsed NLRI, but ze performs no treat-as-withdraw for a zero-label / no-overlay-index RT-5 (internal/component/bgp/plugins/nlri/evpn/types.go:754-814) |
| [`RFC9136-3.2-2`](#rfc9136-3.2-2) * Irrespective of the recursive resolution, if there is no IGP or BGP route to the BGP next hop of an RT-5, BGP MUST NOT install the RT-5 even if the Overlay Index can be resolved. (S3.2) | {gap}, no test | gap in proof: ze meets it by installing no RT-5 at all: internal/component/sysrib/sysrib.go keeps EVPN out of the system RIB, so neither the kernel nor the VPP FIB backend receives one; no test asserts it |
| [`RFC9136-3-1`](#rfc9136-3-1) In case two or more NVEs are attached to different BDs of the same tenant, they MUST support the RT-5 for the proper inter-subnet forwarding operation of the tenant. (S3) | {gap}, no test | ze attaches no broadcast domain and runs no inter-subnet forwarding, so it is no NVE supporting RT-5 for that operation; its RT-5 support is the wire codec (internal/component/bgp/plugins/nlri/evpn/types.go). No owner decision declines the EVPN data plane, so this is owed with it |
| [`RFC9136-3.2-3`](#rfc9136-3.2-3) The encoding of a MAC address MUST be the 6-octet MAC address specified by [IEEE-802.1Q]. (S3.2, Table 1) | {gap}, no test | ze neither encodes nor decodes the EVPN Router's MAC Extended Community, so it encodes no MAC address in the 6-octet form this row requires; owed with the MAC overlay index |
| [`RFC9136-3.2-4`](#rfc9136-3.2-4) The route MUST be treat as withdraw in case of an invalid MAC address. (S3.2, Table 1) | {gap}, no test | ze does not interpret the EVPN Router's MAC Extended Community, so a route carrying an invalid MAC address is not treated as withdraw; owed with the MAC overlay index |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-24 (independent auditor, authored none of the change). RFC 9136 Section 3.1: "* The Ethernet Segment Identifier MUST be a non-zero 10-octet identifier if the ESI is used as an Overlay Index (see the definition of "Overlay Index" in Section 3.2). It MUST be all bytes zero otherwise." 238bdb62e9 only renamed ParseESIString to parseESIString in the cited encode.go::l2vpnRouteToEVPNParams; the overlay-index guard and the ESI handling the verdict rests on are unchanged, and the tagged carriers are byte-identical. Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-10-19 (independent auditor, after 4283f8d2e3). Verdict stands: enforced. RFC 9136 Section 3.1: "The Ethernet Segment Identifier MUST be a non-zero 10-octet identifier if the ESI is used as an Overlay Index (see the definition of "Overlay Index" in Section 3.2).  It MUST be all bytes zero otherwise." No producer change since 0411b53e91 (git log over internal/component/bgp/plugins/nlri/evpn and update_text_evpn.go shows only 4283f8d2e3, which touches internal/le/interoplab/bgp/check_extras.go alone). checkOverlayIndex is unchanged and still the only gate before newEVPNType5, whose two non-test callers (plugin.go buildEVPNFromParams, encode.go l2vpnRouteToEVPNParams) both call it first. Tagged units pass at HEAD (go test -run 'TestRFC9136Overlay|EVPNType5ESIGateway' over nlri/evpn and cmd/update, ok). Interop, closing the prior note's owed item: 4283f8d2e3 makes bgp-evpn-gobgp originate through update text ('... ip-prefix rd 1:1 esi 00:01:02:03:04:05:06:07:08:09 etag 0 prefix 10.77.0.0/24 overlay-index esi label 0', internal/le/interoplab/bgp/helper.go:202, path parseEVPNSection -> buildEVPNFromParams -> checkOverlayIndex) and wait for GoBGP's JSON NLRI to read "esi":"ESI_ARBITRARY | 01:02:03:04:05:06:07:08:09","etag":0,"prefix":"10.77.0.0/24","gateway":"0.0.0.0". bgp-evpn-frr now only asserts FRR keyed the route [5]:[0]:[24]:[10.77.0.0] (FRR 10.3.1 prints no type-5 ESI), so FRR proves acceptance, not the octets. Both scenarios reported 1 passed in the commit body; not re-run by this judge. Red recorded in the commit body: a GOFLAGS overlay passing ESI{} to newEVPNType5 in buildEVPNFromParams made bgp-evpn-gobgp fail its assertion 3 (GoBGP read esi single-homed), so the interop discriminates the non-zero-when-index clause at a second implementation. The zero-otherwise clause and every refusal remain unit-proven only (TestRFC9136OverlayIndexESIRefusedOtherwise, four mutants each red on its own subtest per the 2026-10-18 note); an interop peer cannot observe a route Ze refuses to send, so no interop negative is owed. The interop check is untagged reach/goal evidence and adds no tag; it carries no discrimination record of its own under ./le rfc discriminate-record, only the commit-body red.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9136OverlayIndexESIRefusedOtherwise`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_overlay_index_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestRFC9136OverlayIndexESINonZeroOnlyAsIndex`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_overlay_index_test.go#L44) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-24 (independent auditor, authored none of the change). RFC 9136 Section 3.1: "The GW IP field MUST be all bytes zero if it is not used as an Overlay Index." 238bdb62e9 only renamed ParseESIString to parseESIString in the cited encode.go::l2vpnRouteToEVPNParams; the GW IP handling and overlay-index guard are unchanged, and the tagged carriers are byte-identical. Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-10-19 (independent auditor, after 4283f8d2e3). Verdict stands: enforced. RFC 9136 Section 3.1: "The GW IP Address field is a 4- or 16-octet field (IPv4 or IPv6) and will encode a valid IP address as an Overlay Index for the IP prefixes.  The GW IP field MUST be all bytes zero if it is not used as an Overlay Index." No producer change since 0411b53e91 (git log over internal/component/bgp/plugins/nlri/evpn and update_text_evpn.go shows only 4283f8d2e3, which touches internal/le/interoplab/bgp/check_extras.go alone). checkOverlayIndex is unchanged and still the only gate before newEVPNType5, whose two non-test callers (plugin.go buildEVPNFromParams, encode.go l2vpnRouteToEVPNParams) both call it first. Tagged units pass at HEAD (go test -run 'TestRFC9136Overlay|EVPNType5ESIGateway' over nlri/evpn and cmd/update, ok). Interop, closing the prior note's owed item: 4283f8d2e3 makes bgp-evpn-gobgp originate through update text ('... ip-prefix rd 1:1 esi 00:01:02:03:04:05:06:07:08:09 etag 0 prefix 10.77.0.0/24 overlay-index esi label 0', internal/le/interoplab/bgp/helper.go:202, path parseEVPNSection -> buildEVPNFromParams -> checkOverlayIndex) and wait for GoBGP's JSON NLRI to read "esi":"ESI_ARBITRARY | 01:02:03:04:05:06:07:08:09","etag":0,"prefix":"10.77.0.0/24","gateway":"0.0.0.0". bgp-evpn-frr now only asserts FRR keyed the route [5]:[0]:[24]:[10.77.0.0] (FRR 10.3.1 prints no type-5 ESI), so FRR proves acceptance, not the octets. Both scenarios reported 1 passed in the commit body; not re-run by this judge. The GoBGP needle includes "gateway":"0.0.0.0", so GoBGP reads an all-zero IPv4 GW IP under overlay-index esi: positive interop evidence for this MUST. The recorded red broke the ESI only; no red was forced on the gateway (for example passing a non-zero gateway past checkOverlayIndex), so the interop's discrimination of the GW IP clause is unproven and the needle's gateway half is a positive-only observation. That does not lower the verdict, which rests on the unit proof: TestRFC9136OverlayIndexESINonZeroOnlyAsIndex pins GW 00000000 under none and esi, TestRFC9136OverlayIndexESIRefusedOtherwise refuses gateway 10.1.2.1 under esi, none and no keyword, three mutants red on their own subtests (2026-10-18 note).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9136OverlayIndexESIRefusedOtherwise`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_overlay_index_test.go#L95) | unit/verify | revert, verified |
| positive | [`TestRFC9136OverlayIndexESINonZeroOnlyAsIndex`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/evpn/rfc9136_overlay_index_test.go#L50) | unit/verify | revert, verified |

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
