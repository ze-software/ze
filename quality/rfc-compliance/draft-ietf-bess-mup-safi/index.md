# DRAFT-IETF-BESS-MUP-SAFI - BGP Extensions for the Mobile User Plane (MUP) SAFI

Partial. Every requirement this repository extracted from DRAFT-IETF-BESS-MUP-SAFI, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 16.7% | 8 of 48 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 4.2% | 2 of 48 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 48 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 77.8% | 14 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 48 | of 68 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 48 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 48 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 48 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 48 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 79.2% | 38 of 48 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 9 | of 48 gated MUSTs judged | 9 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 7 shares marked as a part above are the whole of the 48 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 68 |
| Gated MUST-level | 48 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 38 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 9 |
| Discrimination records | 14 |
| Summary | `rfc/short/draft-ietf-bess-mup-safi.md` |
| Requirement shard | `rfc/requirements/draft-ietf-bess-mup-safi.md` |
| RFC text | `rfc/drafts/draft-ietf-bess-mup-safi.txt` |

## Enrolment

Enrolled: BGP Extensions for Mobile User Plane (MUP) SAFI. Eleven MUST rows added 2026-09-21 from sentences the checklist had not carried, each untested: the TLV rules of Sections 3.1.3.1 and 3.1.4.1 (3.1.3.1-6 through 3.1.3.1-10, 3.1.4.1-3), the Section 3.1.5 rule that a TLV received in a route type for which it is not applicable MUST be ignored (3.1.5-1), and the four Section 3.3.12 route resolution rules that bind a PE receiving a Type 2 ST route to DSD or ISD routes by BGP MUP Extended Community (3.3.12-4 through 3.3.12-7). The MUP codec parses no ST route TLV and resolves no Type 2 ST route.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

The BGP-MUP NLRI codec only: ipv4/mup and ipv6/mup family registration, ISD/DSD/T1ST/T2ST encoding from config and route commands, full route type body decoding, the RFC 7606 Section 5.4 ruling that discards a route whose Architecture Type is not 1 or whose Route Type is outside 1..4 at ingress ([`internal/component/bgp/plugins/nlri/mup/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc7606.go), RecognizeNLRI), MUP extended-community config syntax, and family-generic MP_REACH announcement (internal/component/bgp/plugins/nlri/mup). Thirty-four MUST gaps are annotated per line in [`rfc/short/draft-ietf-bess-mup-safi.md`](https://github.com/ze-software/ze/blob/main/rfc/short/draft-ietf-bess-mup-safi.md). Withdrawal: no MUP NLRI reaches the family-generic MP_UNREACH encoder ([`internal/component/bgp/reactor/peer_rib_routes.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_rib_routes.go)) -- its callers read the PeerOpWithdraw queue, and neither withdrawal entry point parses SAFI 85 ([`internal/component/bgp/plugins/cmd/update/update_text_nlri.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/update_text_nlri.go), [`internal/component/bgp/plugins/cmd/announce/announce.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/announce.go)) -- so a Type 1 ST or Type 2 ST withdrawal cannot be emitted (3.3.8-1, 3.3.11-1). Receive side: nlrisplit registers SplitMUP for SAFI 85 since 2026-08-04 ([`internal/core/bgp/nlri/nlrisplit/register.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/register.go)), so a received MUP route is stored as an opaque Adj-RIB-In entry (insertPoolNLRIs) and a withdrawal deletes exactly the NLRI it names (removePoolNLRIs, [`internal/component/bgp/plugins/rib/rib.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib.go)). Four routing-instance obligations stay open because ze models no MUP routing instance and no route-type-aware wildcard delete exists ([`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2`](#draft-ietf-bess-mup-safi-3.3.3-2), 3.3.6-2, 3.3.9-1, 3.3.9-2). ParseMUP validates the route type body and refuses an NLRI that does not add up ([`internal/component/bgp/plugins/nlri/mup/types.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/types.go)), but it serves the JSON decode path and the CLI: the ingress recognizer reads the architecture and route type alone (rfc7606.go, RecognizeNLRI), so no RFC 7606 treat-as-withdraw fires on an out-of-range prefix length, wrong-size address, zero TEID, invalid endpoint or source length, over-long T2ST endpoint length, or non-3gpp-5g architecture type (3.1.1-1, 3.1.1-2, 3.1.2-1, 3.1.3-1, 3.1.3.1-1, 3.1.3.1-2, 3.1.3.1-3, 3.1.3.1-4, 3.1.4-1, 3.1.4.1-1, 3.1.4.1-2), nor on a missing Prefix-SID, a nexthop/locator mismatch, or a Type 2 ST route without the BGP MUP Extended Community (3.3.3-1, 3.3.3-3, 3.3.3-4, 3.3.6-1). Send side: ze runs no MUP PE or MUP Controller function, so route targets, the BGP MUP Extended Community, the Prefix-SID, the GTP4.E/GTP6.E function, the required T1ST TEID and Endpoint Address, and the PE or controller IPv6 nexthop are whatever the operator configures rather than derived (3.3.1-1, 3.3.1-2, 3.3.1-3, 3.3.1-4, 3.3.2-1, 3.3.4-1, 3.3.4-2, 3.3.4-3, 3.3.4-4, 3.3.5-2, 3.3.7-1, 3.3.10-1, 3.3.10-2).

**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 8 | one part of the gated population |
| Annotated instead of tested | 40 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **48** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (8):** [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6`](#draft-ietf-bess-mup-safi-3.1.3.1-6), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7`](#draft-ietf-bess-mup-safi-3.1.3.1-7), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-8`](#draft-ietf-bess-mup-safi-3.1.3.1-8), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9`](#draft-ietf-bess-mup-safi-3.1.3.1-9), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10`](#draft-ietf-bess-mup-safi-3.1.3.1-10), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-3`](#draft-ietf-bess-mup-safi-3.1.4.1-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.5-1`](#draft-ietf-bess-mup-safi-3.1.5-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3-1`](#draft-ietf-bess-mup-safi-3.3-1)

**Annotated instead of tested (40):** [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1`](#draft-ietf-bess-mup-safi-3.3.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2`](#draft-ietf-bess-mup-safi-3.3.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3`](#draft-ietf-bess-mup-safi-3.3.1-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1`](#draft-ietf-bess-mup-safi-3.3.2-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1`](#draft-ietf-bess-mup-safi-3.3.4-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2`](#draft-ietf-bess-mup-safi-3.3.4-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3`](#draft-ietf-bess-mup-safi-3.3.4-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4`](#draft-ietf-bess-mup-safi-3.3.4-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1`](#draft-ietf-bess-mup-safi-3.3.7-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-2`](#draft-ietf-bess-mup-safi-3.3.7-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1`](#draft-ietf-bess-mup-safi-3.3.10-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2`](#draft-ietf-bess-mup-safi-3.3.10-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.1-1`](#draft-ietf-bess-mup-safi-3.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1`](#draft-ietf-bess-mup-safi-3.3.3-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2`](#draft-ietf-bess-mup-safi-3.3.3-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1`](#draft-ietf-bess-mup-safi-3.3.6-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2`](#draft-ietf-bess-mup-safi-3.3.6-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1`](#draft-ietf-bess-mup-safi-3.3.9-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1`](#draft-ietf-bess-mup-safi-3.1.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2`](#draft-ietf-bess-mup-safi-3.1.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1`](#draft-ietf-bess-mup-safi-3.1.2-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1`](#draft-ietf-bess-mup-safi-3.1.3-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1`](#draft-ietf-bess-mup-safi-3.1.3.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2`](#draft-ietf-bess-mup-safi-3.1.3.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3`](#draft-ietf-bess-mup-safi-3.1.3.1-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1`](#draft-ietf-bess-mup-safi-3.1.4-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1`](#draft-ietf-bess-mup-safi-3.1.4.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4`](#draft-ietf-bess-mup-safi-3.1.3.1-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2`](#draft-ietf-bess-mup-safi-3.1.4.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3`](#draft-ietf-bess-mup-safi-3.3.3-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4`](#draft-ietf-bess-mup-safi-3.3.3-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4`](#draft-ietf-bess-mup-safi-3.3.1-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2`](#draft-ietf-bess-mup-safi-3.3.5-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1`](#draft-ietf-bess-mup-safi-3.3.8-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2`](#draft-ietf-bess-mup-safi-3.3.9-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1`](#draft-ietf-bess-mup-safi-3.3.11-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-4`](#draft-ietf-bess-mup-safi-3.3.12-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-5`](#draft-ietf-bess-mup-safi-3.3.12-5), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-6`](#draft-ietf-bess-mup-safi-3.3.12-6), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-7`](#draft-ietf-bess-mup-safi-3.3.12-7)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1` | When advertising the Interwork Segment Discovery route, a PE MUST attach the export BGP Route Target Extended Community of the associated routing instance. (Section 3.3.1) | MUST | 3.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66) and ze models no routing instance with export route targets, so no export route target is derived for an ISD advertisement |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2` | When advertising the Interwork Segment Discovery route, a PE MUST use the IPv6 address of the PE as the nexthop address in the MP_REACH_NLRI attribute. (Section 3.3.1) | MUST | 3.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) and accepts an IPv4 next-hop for an ISD route, so nothing requires the IPv6 address of the PE |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3` | The Interwork Segment Discovery route update MUST have a prefix SID attribute which the SID consists of the PE locater followed by a function. (Section 3.3.1) | MUST | 3.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71), so an ISD route configured without a prefix SID is advertised without one |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1` | When withdrawing the Interwork Segment Discovery route, a PE MUST attach the export BGP Route Target Extended Community of the associated routing instance. (Section 3.3.2) | MUST | 3.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a MUP withdrawal is built as a bare MP_UNREACH_NLRI with no path attributes (internal/component/bgp/reactor/peer_rib_routes.go:182-197), so an ISD withdrawal carries no export route target |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1` | The address in the BGP-MUP NLRI MUST be a unique PE identifier. (Section 3.3.4) | MUST | 3.3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseDSDFields accepts any parsable address as the DSD NLRI address (internal/component/bgp/plugins/nlri/mup/encode.go:301-310) and never checks that it identifies the PE uniquely |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2` | When announcing the Direct Segment Discovery route, a PE MUST attach a BGP MUP Extended community of the associated routing instance. (Section 3.3.4) | MUST | 3.3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66), so a DSD advertisement carries a BGP MUP Extended Community only when the operator writes one into the route |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3` | When advertising the Direct Segment Discovery route, a PE MUST use the IPv6 address of the PE as the nexthop address in the MP_REACH_NLRI attribute. (Section 3.3.4) | MUST | 3.3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) and accepts an IPv4 next-hop for a DSD route, so nothing requires the IPv6 address of the PE |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4` | The Direct Segment Discovery route update MUST have a prefix SID attribute which the SID consists of the PE locator followed by a function. (Section 3.3.4) | MUST | 3.3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71), so a DSD route configured without a prefix SID is advertised without one |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1` | The MUP Controller MUST set the nexthop of the route to the address of the controller. (§3.3.7) | MUST | 3.3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) for a Type 1 ST route and ze runs no MUP Controller function that substitutes the controller address |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.7-2` | The controller MUST announce this route using a AFI of the route and the SAFI of BGP-MUP to all other BGP speakers within the SRv6 domain. (§3.3.7) | MUST | 3.3.7 | **positive:** `unit/verify` [`TestRFCMUPAnnounceUsesRouteAFIWithMUPSAFI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L207). **negative:** no negative test. **{single-polarity}:** EncodeRoute emits MUP NLRI only under SAFI 85 with the AFI taken from the route family (internal/component/bgp/plugins/nlri/mup/encode.go:183-199), so no non-conformant AFI/SAFI emission exists to reject |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1` | The controller MUST attach a Route Target Extended Community of the routing instances in the PE accommodating the corresponding Interwork Segment. (§3.3.10) | MUST | 3.3.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66) and ze models no routing instance with export route targets, so a Type 2 ST advertisement carries a route target only when the operator writes one into the route |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2` | The controller MUST set the nexthop of the route to the address of the MUP Controller. (§3.3.10) | MUST | 3.3.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) for a Type 2 ST route and ze runs no MUP Controller function that substitutes the controller address |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1-1` | Any other Route Types MUST be silently ignored upon a receipt if a BGP speaker supports only 3gpp-5G architecture type. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFCMUPUnknownRouteTypeIsNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L161). **negative:** no negative test. **{single-polarity}:** ParseMUP accepts any route type under the 3gpp-5g architecture type and returns the bytes after the declared Length (internal/component/bgp/plugins/nlri/mup/types.go:118-152), so there is no rejection path for an unknown route type to drive negatively |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1` | However, the receiving BGP speaker MUST ensure that the value of Address filed in the NLRI is an address of the originator of the locator value in the prefix SID attribute. (Section 3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the ISD Address field is never checked against the originator of the prefix SID locator |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2` | When a BGP speaker receives an MP_UNREACH_NLRI attribute update message it MUST delete the withdrawn Interwork Segment Discovery route from the routing instance table where it was created. (Section 3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives an ISD withdrawal through it |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1` | However, the receiving BGP speaker MUST ensure that the received nexthop value in the MP_REACH_NLRI attribute is identical to the originator of the locator value in the prefix SID attribute. (Section 3.3.6) | MUST | 3.3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the DSD nexthop is never compared with the originator of the prefix SID locator |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2` | When a BGP speaker receives an MP_UNREACH_NLRI attribute update message it MUST delete the withdrawn Direct Segment Discovery route from the routing instance table where it was created. (Section 3.3.6) | MUST | 3.3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives a DSD withdrawal through it |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1` | The PE receiving Type 1 ST routes in MP_UNREACH_NLRI attribute MUST delete all the routes from the associated routing instance. (Section 3.3.9) | MUST | 3.3.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). The delete is one NLRI for one NLRI. Nothing reads the Type 1 ST route type to delete every route of the associated routing instance, and ze models no such instance |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1` | If the AFI is IPv4, then the maximum value of the Prefix Length is 32 bits otherwise it is considered as a malformed NLRI. If the AFI is IPv6, then the maximum value of of the Prefix length is 128 bits otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP reads the ISD prefix length and returns an error above 32 for AFI 1 or 128 for AFI 2 (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyISD), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2` | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. (Section 3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP validates the whole route type body and returns an error for a malformed one (internal/component/bgp/plugins/nlri/mup/types.go, parseBody), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn, and no code skips it and continues |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1` | If the AFI is IPv4 then the address length is 4 octets otherwise it is considered as a malformed NLRI. If the AFI is IPv6 then the address length is 16 octets otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat- as-withdraw" [RFC7606]. (§3.1.2) | MUST | 3.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP measures the DSD address against the AFI and returns an error for any other size (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyDSD), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1` | If the AFI is IPv4, then the maximum value of the Prefix Length field is 32. If the AFI is IPv6, then the maximum value of the Prefix Length field is 128. Any other length field is considered a a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat- as-withdraw" [RFC7606]. (§3.1.3) | MUST | 3.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP reads the T1ST prefix length and returns an error above 32 for AFI 1 or 128 for AFI 2 (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT1ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1` | The TEID value of 0 is considered as an invalid and a malformed TEID. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP reads the T1ST TEID and returns an error when it is zero (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT1ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn. On the encode side parseTEIDWithBits still maps an absent TEID to zero bits, which writeT1STData omits from the NLRI (internal/component/bgp/plugins/nlri/mup/encode.go) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2` | Endpoint Address field contains of an IPv4 address, then the value of the Endpoint Address Length field is 32. If the Endpoint Address field contains of an IPv6 Address, then the value of the Endpoint Address Length field is 128. Any other value is considered as an invalid and a malformed Endpoint Address. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP reads the T1ST Endpoint Address Length and returns an error for a value other than 32 or 128 (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT1ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3` | Source Address Length is 0 bytes then the Source address is not carried within the NLRI. A BGP speaker MAY have a local configuration for using a Source address. The exact mechanism of local configuration is outside the scope of this document. Source Address field contains of an IPv4 address, then the value of the Source Address Length field is 32. If the Source Address field contains of an IPv6 Address, then the value of the Source Address Length field is 128. Any other value is considered as an invalid and a malformed Endpoint Address. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP reads the T1ST Source Address Length and returns an error for a value other than 0, 32 or 128 (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT1ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6` | - < 0: the mandatory fields exceed the declared Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw. (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPT1STMandatoryFieldsWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L34). **negative:** `unit/verify` [`TestMUPT1STMandatoryFieldsWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L42) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7` | - 1: encoding is invalid (a valid TLV requires at minimum a Type byte and a Length byte); MUST be treated as Treat-as- withdraw. (Section 3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L59). **negative:** `unit/verify` [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L67) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-8` | unknown TLV types MUST be ignored for local processing (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L93). **negative:** `unit/verify` [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L105) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9` | unknown TLV types MUST be ignored for local processing and MUST be propagated unchanged when re- advertising the route to other BGP peers (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L111). **negative:** `unit/verify` [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L115) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10` | any TLV parsing error MUST result in Treat-as-withdraw (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L61). **negative:** `unit/verify` [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L73) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1` | If the AFI is IPv4, then the maximum Endpoint length is 64 otherwise it is considered as a malformed NLRI. If the AFI is IPv6, then the maximum Endpoint length is 160 otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.4) | MUST | 3.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP reads the T2ST Endpoint Length and returns an error above 64 for AFI 1 or 160 for AFI 2 (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT2ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1` | The TEID value of 0 is considered as an invalid and a malformed TEID. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.4.1) | MUST | 3.1.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP reads the T2ST TEID and returns an error when a present TEID is zero (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT2ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-3` | The Endpoint Length MUST NOT extend beyond the TEID field. (§3.1.4.1) | MUST NOT | 3.1.4.1 | **positive:** `unit/verify` [`TestMUPT2STEndpointLengthBoundedByTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L127). **negative:** `unit/verify` [`TestMUPT2STEndpointLengthBoundedByTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L137) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.5-1` | A TLV received in a route type for which it is not applicable MUST be ignored (Section 3.1.5) | MUST | 3.1.5 | **positive:** `unit/verify` [`TestMUPST1IgnoresST2OnlyTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_applicability_test.go#L10). **negative:** `unit/verify` [`TestMUPST1DoesNotInterpretInapplicableAddressTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_applicability_test.go#L24) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4` | The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 1 ST route. (Section 3.1.3.1) | MUST | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go:246) but ParseMUP accepts any architecture byte and keeps parsing (internal/component/bgp/plugins/nlri/mup/types.go:123), so a T1ST NLRI encoded for another architecture is not treated as withdraw |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2` | The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 2 ST route. (Section 3.1.4.1) | MUST | 3.1.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go:246) but ParseMUP accepts any architecture byte and keeps parsing (internal/component/bgp/plugins/nlri/mup/types.go:123), so a T2ST NLRI encoded for another architecture is not treated as withdraw |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3` | When a BGP speaker receives the Interwork Segment Dicovery routes with a MP_REACH_NLRI attribute without a prefix SID attribute, then it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied. (Section 3.3.3, 3.3.6) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) without consulting the UPDATE path attributes, so an ISD or DSD route arriving without a Prefix-SID attribute is not treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4` | If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied. (§3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so a mismatch between the nexthop and the locator originator is never detected on ISD or DSD routes |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3-1` | BGP speakers acting as a PE, and a MUP Controller MUST establish a BGP session to exchange BGP-MUP NLRIs for both, IPv4 and IPv6 AFIs. (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFCMUPFamiliesCoverBothAFIs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L84). **negative:** `unit/verify` [`TestRFCMUPRejectsNonMUPFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L133) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4` | In 3GPP 5G specific case, if the BGP AFI is IPv4, the function MUST be GTP4.E [I-D.ietf-dmm-srv6-mobile-uplane], or MUST be GTP6.E [I-D.ietf-dmm-srv6-mobile-uplane] if the BGP AFI is IPv6. (Section 3.3.1) | MUST | 3.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71) and passes its bytes through unchanged, and ze decodes no SRv6 endpoint function, so the GTP4.E/GTP6.E function is never tied to the BGP AFI |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2` | When withdrawing the Direct Segment Discovery route, a BGP speaker MUST attach a BGP MUP Extended community of the associated routing instance. (Section 3.3.5) | MUST | 3.3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a MUP withdrawal is built as a bare MP_UNREACH_NLRI with no path attributes (internal/component/bgp/reactor/peer_rib_routes.go:182-197), so a DSD withdrawal carries no BGP MUP Extended Community |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1` | The controller MUST advertise the withdraws of the Type 1 ST route. (Section 3.3.8) | MUST | 3.3.8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no MUP NLRI can reach the family-generic MP_UNREACH encoder (internal/component/bgp/reactor/peer_rib_routes.go:170). Its only callers take the NLRI from a PeerOpWithdraw queue entry (internal/component/bgp/reactor/peer_initial_sync.go:237, :377) filled by QueueWithdraw (internal/component/bgp/reactor/peer.go:886-893), and the two withdrawal entry points that feed it parse no SAFI 85: text mode rejects the family in isSupportedFamily, whose list stops at SAFI 73 (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:375-403), and the announce/withdraw registry builds only unicast and FlowSpec NLRIs (internal/component/bgp/plugins/cmd/announce/announce.go:257, :304, :415). NewMUP and NewMUPFull (internal/component/bgp/plugins/nlri/mup/types.go:93, :103) have no non-test caller, and nlrisplit registers no SAFI 85 splitter (internal/core/bgp/nlri/nlrisplit/register.go:9-24) so no received MUP route is stored to be withdrawn either |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2` | In an event where the received Type 1 ST route in MP_UNREACH_NLRI attribute does not have a Source address a receiving PE MUST delete all the matching Type 1 ST Routes with different Source addresses. (§3.3.9) | MUST | 3.3.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). The opaque key is the whole NLRI, Source address included, so a Source-less Type 1 ST withdrawal matches no stored entry. No wildcard delete over differing Source addresses exists |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1` | The controller MUST advertise the withdraws of the Type 2 ST route. (Section 3.3.11) | MUST | 3.3.11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same missing path as DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1 -- the family-generic MP_UNREACH encoder (internal/component/bgp/reactor/peer_rib_routes.go:170) is reachable only through PeerOpWithdraw (internal/component/bgp/reactor/peer.go:886-893, internal/component/bgp/reactor/peer_initial_sync.go:237, :377), and neither withdrawal entry point produces a SAFI 85 NLRI: isSupportedFamily omits it (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:375-403) and the announce registry builds only unicast and FlowSpec NLRIs (internal/component/bgp/plugins/cmd/announce/announce.go:257, :304, :415) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-5` | The BGP speaker receiving the Interwork Segment Discovery routes SHOULD ignore the nexthop in the MP_REACH_NLRI attribute. (§3.3.3) | SHOULD | 3.3.3 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.7-4` | When advertising the Type 1 ST route, the controller SHOULD attach a Route Target Extended community which the PEs are importing into the routing instance for the corresponding Direct segment. (§3.3.7) | SHOULD | 3.3.7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-3` | In case of a BGP speaker receiving a Type 1 ST routes is a PE, the PE SHOULD use the received Tunnel Endpoint Address in this NLRI as a key to lookup the associated Interwork Segment Discovery route and extract the locator and the function in the prefix SID attribute of the Interwork route. (§3.3.9) | SHOULD | 3.3.9 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-3` | The BGP speaker receiving the Direct Segment Discovery routes SHOULD ignore the nexthop in the MP_REACH_NLRI attribute. (§3.3.6) | SHOULD | 3.3.6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-4` | The PE receiving Type 1 ST routes SHOULD ignore the received nexthop in the MP_REACH_NLRI attribute. (§3.3.9) | SHOULD | 3.3.9 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-5` | The PE SHOULD generate the forwarding SID for GTP4/6.E based on the procedures mentioned in the [I-D.ietf-dmm-srv6-mobile-uplane]. (§3.3.9) | SHOULD | 3.3.9 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-6` | If the PE cannot generate the prefix SID, then it SHOULD mark the received Type 1 ST route as an invalid route. (§3.3.9) | SHOULD | 3.3.9 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.12-2` | The BGP speaker receiving the Type 2 ST routes SHOULD ignore the received nexthop in the MP_REACH_NLRI attribute. (§3.3.12) | SHOULD | 3.3.12 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.12-4` | A PE receiving a Type 2 ST route with a Direct Segment type BGP MUP Extended Community MUST resolve the route using the DSD routes matching that community (Section 3.3.12) | MUST | 3.3.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze resolves no received Type 2 ST route against DSD or ISD routes; the MUP plugin only encodes, decodes and originates MUP NLRIs; internal/component/bgp/plugins/nlri/mup/mup.go |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.12-5` | DSD routes MUST NOT be used to resolve Type 2 ST routes that do not carry a Direct Segment type BGP MUP Extended Community (Section 3.3.12) | MUST NOT | 3.3.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze resolves no received Type 2 ST route against DSD or ISD routes; the MUP plugin only encodes, decodes and originates MUP NLRIs; internal/component/bgp/plugins/nlri/mup/mup.go |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.12-6` | A PE receiving a Type 2 ST route with an Interwork Segment type BGP MUP Extended Community MUST resolve the route using ISD routes that carry a matching Interwork Segment type BGP MUP Extended Community (Section 3.3.12) | MUST | 3.3.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze resolves no received Type 2 ST route against DSD or ISD routes; the MUP plugin only encodes, decodes and originates MUP NLRIs; internal/component/bgp/plugins/nlri/mup/mup.go |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.12-7` | A PE receiving a Type 2 ST route without a BGP MUP Extended Community MUST resolve the route using ISD routes for the default Interwork Segment, i.e., ISD routes that carry no BGP MUP Extended Community. (Section 3.3.12) | MUST | 3.3.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze resolves no received Type 2 ST route against DSD or ISD routes; the MUP plugin only encodes, decodes and originates MUP NLRIs; internal/component/bgp/plugins/nlri/mup/mup.go |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.8-2` | When withdrawing the Type 1 ST route, the controller SHOULD attach the Route Target Extended community which the PEs are importing into the routing instance accomodating the corresponding Direct segment to the Route Target Extended community. (§3.3.8) | SHOULD | 3.3.8 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.10-3` | When advertising the Type 2 ST route for a Direct Segment, the controller SHOULD attach a Direct Segment type BGP MUP Extended Community. (§3.3.10) | SHOULD | 3.3.10 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.11-2` | When withdrawing the Type 2 ST route, the controller SHOULD attach the same BGP MUP Extended Community that was attached when the route was advertised, together with the Route Target Extended Community of the corresponding routing instance. (§3.3.11) | SHOULD | 3.3.11 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-4-1` | The method defined in [RFC5925] SHOULD be used where authentication of BGP control packets is needed. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-4-2` | The PEs and MUP Controller SHOULD NOT establish BGP sessions with other BGP speakers in the domains which are not trusted without any explicit configuration or an operator intervention. (§4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-4-3` | Usage of procedures defined in [RFC5925] SHOULD be enforced at such boundaries to ensure the proper authentication of BGP control packets. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-4-4` | To protect the BGP messages exchanged between BGP speakers from eavesdrop, establishing BGP sessions over encrypted paths SHOULD be considered. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-4-5` | PEs SHOULD impose an upper bound on number of routes they should store to protect their control plane load. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1-2` | An implementation MAY log an error when such Route Types are ignored. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-5` | Source Address Length is 0 bytes then the Source address is not carried within the NLRI. A BGP speaker MAY have a local configuration for using a Source address. (§3.1.3.1) | MAY | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.1-5` | The IP prefix MAY include a gNodeB address which is connecting to the PE. (§3.3.1) | MAY | 3.3.1 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.4-5` | The function MAY be End.DT4/6 or End.DX4/6. (§3.3.4) | MAY | 3.3.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1`](#draft-ietf-bess-mup-safi-3.3.1-1) When advertising the Interwork Segment Discovery route, a PE MUST attach the export BGP Route Target Extended Community of the associated routing instance. (Section 3.3.1) | {gap}, no test | parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66) and ze models no routing instance with export route targets, so no export route target is derived for an ISD advertisement |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2`](#draft-ietf-bess-mup-safi-3.3.1-2) When advertising the Interwork Segment Discovery route, a PE MUST use the IPv6 address of the PE as the nexthop address in the MP_REACH_NLRI attribute. (Section 3.3.1) | {gap}, no test | EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) and accepts an IPv4 next-hop for an ISD route, so nothing requires the IPv6 address of the PE |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3`](#draft-ietf-bess-mup-safi-3.3.1-3) The Interwork Segment Discovery route update MUST have a prefix SID attribute which the SID consists of the PE locater followed by a function. (Section 3.3.1) | {gap}, no test | parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71), so an ISD route configured without a prefix SID is advertised without one |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1`](#draft-ietf-bess-mup-safi-3.3.2-1) When withdrawing the Interwork Segment Discovery route, a PE MUST attach the export BGP Route Target Extended Community of the associated routing instance. (Section 3.3.2) | {gap}, no test | a MUP withdrawal is built as a bare MP_UNREACH_NLRI with no path attributes (internal/component/bgp/reactor/peer_rib_routes.go:182-197), so an ISD withdrawal carries no export route target |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1`](#draft-ietf-bess-mup-safi-3.3.4-1) The address in the BGP-MUP NLRI MUST be a unique PE identifier. (Section 3.3.4) | {gap}, no test | parseDSDFields accepts any parsable address as the DSD NLRI address (internal/component/bgp/plugins/nlri/mup/encode.go:301-310) and never checks that it identifies the PE uniquely |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2`](#draft-ietf-bess-mup-safi-3.3.4-2) When announcing the Direct Segment Discovery route, a PE MUST attach a BGP MUP Extended community of the associated routing instance. (Section 3.3.4) | {gap}, no test | parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66), so a DSD advertisement carries a BGP MUP Extended Community only when the operator writes one into the route |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3`](#draft-ietf-bess-mup-safi-3.3.4-3) When advertising the Direct Segment Discovery route, a PE MUST use the IPv6 address of the PE as the nexthop address in the MP_REACH_NLRI attribute. (Section 3.3.4) | {gap}, no test | EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) and accepts an IPv4 next-hop for a DSD route, so nothing requires the IPv6 address of the PE |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4`](#draft-ietf-bess-mup-safi-3.3.4-4) The Direct Segment Discovery route update MUST have a prefix SID attribute which the SID consists of the PE locator followed by a function. (Section 3.3.4) | {gap}, no test | parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71), so a DSD route configured without a prefix SID is advertised without one |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1`](#draft-ietf-bess-mup-safi-3.3.7-1) The MUP Controller MUST set the nexthop of the route to the address of the controller. (§3.3.7) | {gap}, no test | EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) for a Type 1 ST route and ze runs no MUP Controller function that substitutes the controller address |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1`](#draft-ietf-bess-mup-safi-3.3.10-1) The controller MUST attach a Route Target Extended Community of the routing instances in the PE accommodating the corresponding Interwork Segment. (§3.3.10) | {gap}, no test | parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66) and ze models no routing instance with export route targets, so a Type 2 ST advertisement carries a route target only when the operator writes one into the route |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2`](#draft-ietf-bess-mup-safi-3.3.10-2) The controller MUST set the nexthop of the route to the address of the MUP Controller. (§3.3.10) | {gap}, no test | EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) for a Type 2 ST route and ze runs no MUP Controller function that substitutes the controller address |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1`](#draft-ietf-bess-mup-safi-3.3.3-1) However, the receiving BGP speaker MUST ensure that the value of Address filed in the NLRI is an address of the originator of the locator value in the prefix SID attribute. (Section 3.3.3) | {gap}, no test | DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the ISD Address field is never checked against the originator of the prefix SID locator |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2`](#draft-ietf-bess-mup-safi-3.3.3-2) When a BGP speaker receives an MP_UNREACH_NLRI attribute update message it MUST delete the withdrawn Interwork Segment Discovery route from the routing instance table where it was created. (Section 3.3.3) | {gap}, no test | nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives an ISD withdrawal through it |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1`](#draft-ietf-bess-mup-safi-3.3.6-1) However, the receiving BGP speaker MUST ensure that the received nexthop value in the MP_REACH_NLRI attribute is identical to the originator of the locator value in the prefix SID attribute. (Section 3.3.6) | {gap}, no test | DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the DSD nexthop is never compared with the originator of the prefix SID locator |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2`](#draft-ietf-bess-mup-safi-3.3.6-2) When a BGP speaker receives an MP_UNREACH_NLRI attribute update message it MUST delete the withdrawn Direct Segment Discovery route from the routing instance table where it was created. (Section 3.3.6) | {gap}, no test | nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives a DSD withdrawal through it |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1`](#draft-ietf-bess-mup-safi-3.3.9-1) The PE receiving Type 1 ST routes in MP_UNREACH_NLRI attribute MUST delete all the routes from the associated routing instance. (Section 3.3.9) | {gap}, no test | nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). The delete is one NLRI for one NLRI. Nothing reads the Type 1 ST route type to delete every route of the associated routing instance, and ze models no such instance |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1`](#draft-ietf-bess-mup-safi-3.1.1-1) If the AFI is IPv4, then the maximum value of the Prefix Length is 32 bits otherwise it is considered as a malformed NLRI. If the AFI is IPv6, then the maximum value of of the Prefix length is 128 bits otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.1) | {gap}, no test | ParseMUP reads the ISD prefix length and returns an error above 32 for AFI 1 or 128 for AFI 2 (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyISD), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2`](#draft-ietf-bess-mup-safi-3.1.1-2) A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. (Section 3.1.1) | {gap}, no test | ParseMUP validates the whole route type body and returns an error for a malformed one (internal/component/bgp/plugins/nlri/mup/types.go, parseBody), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn, and no code skips it and continues |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1`](#draft-ietf-bess-mup-safi-3.1.2-1) If the AFI is IPv4 then the address length is 4 octets otherwise it is considered as a malformed NLRI. If the AFI is IPv6 then the address length is 16 octets otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat- as-withdraw" [RFC7606]. (§3.1.2) | {gap}, no test | ParseMUP measures the DSD address against the AFI and returns an error for any other size (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyDSD), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1`](#draft-ietf-bess-mup-safi-3.1.3-1) If the AFI is IPv4, then the maximum value of the Prefix Length field is 32. If the AFI is IPv6, then the maximum value of the Prefix Length field is 128. Any other length field is considered a a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat- as-withdraw" [RFC7606]. (§3.1.3) | {gap}, no test | ParseMUP reads the T1ST prefix length and returns an error above 32 for AFI 1 or 128 for AFI 2 (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT1ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1`](#draft-ietf-bess-mup-safi-3.1.3.1-1) The TEID value of 0 is considered as an invalid and a malformed TEID. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1) | {gap}, no test | ParseMUP reads the T1ST TEID and returns an error when it is zero (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT1ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn. On the encode side parseTEIDWithBits still maps an absent TEID to zero bits, which writeT1STData omits from the NLRI (internal/component/bgp/plugins/nlri/mup/encode.go) |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2`](#draft-ietf-bess-mup-safi-3.1.3.1-2) Endpoint Address field contains of an IPv4 address, then the value of the Endpoint Address Length field is 32. If the Endpoint Address field contains of an IPv6 Address, then the value of the Endpoint Address Length field is 128. Any other value is considered as an invalid and a malformed Endpoint Address. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1) | {gap}, no test | ParseMUP reads the T1ST Endpoint Address Length and returns an error for a value other than 32 or 128 (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT1ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3`](#draft-ietf-bess-mup-safi-3.1.3.1-3) Source Address Length is 0 bytes then the Source address is not carried within the NLRI. A BGP speaker MAY have a local configuration for using a Source address. The exact mechanism of local configuration is outside the scope of this document. Source Address field contains of an IPv4 address, then the value of the Source Address Length field is 32. If the Source Address field contains of an IPv6 Address, then the value of the Source Address Length field is 128. Any other value is considered as an invalid and a malformed Endpoint Address. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1) | {gap}, no test | ParseMUP reads the T1ST Source Address Length and returns an error for a value other than 0, 32 or 128 (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT1ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1`](#draft-ietf-bess-mup-safi-3.1.4-1) If the AFI is IPv4, then the maximum Endpoint length is 64 otherwise it is considered as a malformed NLRI. If the AFI is IPv6, then the maximum Endpoint length is 160 otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.4) | {gap}, no test | ParseMUP reads the T2ST Endpoint Length and returns an error above 64 for AFI 1 or 160 for AFI 2 (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT2ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1`](#draft-ietf-bess-mup-safi-3.1.4.1-1) The TEID value of 0 is considered as an invalid and a malformed TEID. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.4.1) | {gap}, no test | ParseMUP reads the T2ST TEID and returns an error when a present TEID is zero (internal/component/bgp/plugins/nlri/mup/types.go, parseBodyT2ST), but that parse serves the JSON decode path (DecodeNLRIHex) and the CLI decoder, while the ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a received route carrying it is still stored rather than treated as withdrawn |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4`](#draft-ietf-bess-mup-safi-3.1.3.1-4) The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 1 ST route. (Section 3.1.3.1) | {gap}, no test | writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go:246) but ParseMUP accepts any architecture byte and keeps parsing (internal/component/bgp/plugins/nlri/mup/types.go:123), so a T1ST NLRI encoded for another architecture is not treated as withdraw |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2`](#draft-ietf-bess-mup-safi-3.1.4.1-2) The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 2 ST route. (Section 3.1.4.1) | {gap}, no test | writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go:246) but ParseMUP accepts any architecture byte and keeps parsing (internal/component/bgp/plugins/nlri/mup/types.go:123), so a T2ST NLRI encoded for another architecture is not treated as withdraw |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3`](#draft-ietf-bess-mup-safi-3.3.3-3) When a BGP speaker receives the Interwork Segment Dicovery routes with a MP_REACH_NLRI attribute without a prefix SID attribute, then it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied. (Section 3.3.3, 3.3.6) | {gap}, no test | DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) without consulting the UPDATE path attributes, so an ISD or DSD route arriving without a Prefix-SID attribute is not treated as withdrawn |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4`](#draft-ietf-bess-mup-safi-3.3.3-4) If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied. (§3.3.3) | {gap}, no test | DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so a mismatch between the nexthop and the locator originator is never detected on ISD or DSD routes |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4`](#draft-ietf-bess-mup-safi-3.3.1-4) In 3GPP 5G specific case, if the BGP AFI is IPv4, the function MUST be GTP4.E [I-D.ietf-dmm-srv6-mobile-uplane], or MUST be GTP6.E [I-D.ietf-dmm-srv6-mobile-uplane] if the BGP AFI is IPv6. (Section 3.3.1) | {gap}, no test | parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71) and passes its bytes through unchanged, and ze decodes no SRv6 endpoint function, so the GTP4.E/GTP6.E function is never tied to the BGP AFI |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2`](#draft-ietf-bess-mup-safi-3.3.5-2) When withdrawing the Direct Segment Discovery route, a BGP speaker MUST attach a BGP MUP Extended community of the associated routing instance. (Section 3.3.5) | {gap}, no test | a MUP withdrawal is built as a bare MP_UNREACH_NLRI with no path attributes (internal/component/bgp/reactor/peer_rib_routes.go:182-197), so a DSD withdrawal carries no BGP MUP Extended Community |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1`](#draft-ietf-bess-mup-safi-3.3.8-1) The controller MUST advertise the withdraws of the Type 1 ST route. (Section 3.3.8) | {gap}, no test | no MUP NLRI can reach the family-generic MP_UNREACH encoder (internal/component/bgp/reactor/peer_rib_routes.go:170). Its only callers take the NLRI from a PeerOpWithdraw queue entry (internal/component/bgp/reactor/peer_initial_sync.go:237, :377) filled by QueueWithdraw (internal/component/bgp/reactor/peer.go:886-893), and the two withdrawal entry points that feed it parse no SAFI 85: text mode rejects the family in isSupportedFamily, whose list stops at SAFI 73 (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:375-403), and the announce/withdraw registry builds only unicast and FlowSpec NLRIs (internal/component/bgp/plugins/cmd/announce/announce.go:257, :304, :415). NewMUP and NewMUPFull (internal/component/bgp/plugins/nlri/mup/types.go:93, :103) have no non-test caller, and nlrisplit registers no SAFI 85 splitter (internal/core/bgp/nlri/nlrisplit/register.go:9-24) so no received MUP route is stored to be withdrawn either |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2`](#draft-ietf-bess-mup-safi-3.3.9-2) In an event where the received Type 1 ST route in MP_UNREACH_NLRI attribute does not have a Source address a receiving PE MUST delete all the matching Type 1 ST Routes with different Source addresses. (§3.3.9) | {gap}, no test | nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). The opaque key is the whole NLRI, Source address included, so a Source-less Type 1 ST withdrawal matches no stored entry. No wildcard delete over differing Source addresses exists |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1`](#draft-ietf-bess-mup-safi-3.3.11-1) The controller MUST advertise the withdraws of the Type 2 ST route. (Section 3.3.11) | {gap}, no test | the same missing path as DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1 -- the family-generic MP_UNREACH encoder (internal/component/bgp/reactor/peer_rib_routes.go:170) is reachable only through PeerOpWithdraw (internal/component/bgp/reactor/peer.go:886-893, internal/component/bgp/reactor/peer_initial_sync.go:237, :377), and neither withdrawal entry point produces a SAFI 85 NLRI: isSupportedFamily omits it (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:375-403) and the announce registry builds only unicast and FlowSpec NLRIs (internal/component/bgp/plugins/cmd/announce/announce.go:257, :304, :415) |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-4`](#draft-ietf-bess-mup-safi-3.3.12-4) A PE receiving a Type 2 ST route with a Direct Segment type BGP MUP Extended Community MUST resolve the route using the DSD routes matching that community (Section 3.3.12) | {gap}, no test | ze resolves no received Type 2 ST route against DSD or ISD routes; the MUP plugin only encodes, decodes and originates MUP NLRIs; internal/component/bgp/plugins/nlri/mup/mup.go |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-5`](#draft-ietf-bess-mup-safi-3.3.12-5) DSD routes MUST NOT be used to resolve Type 2 ST routes that do not carry a Direct Segment type BGP MUP Extended Community (Section 3.3.12) | {gap}, no test | ze resolves no received Type 2 ST route against DSD or ISD routes; the MUP plugin only encodes, decodes and originates MUP NLRIs; internal/component/bgp/plugins/nlri/mup/mup.go |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-6`](#draft-ietf-bess-mup-safi-3.3.12-6) A PE receiving a Type 2 ST route with an Interwork Segment type BGP MUP Extended Community MUST resolve the route using ISD routes that carry a matching Interwork Segment type BGP MUP Extended Community (Section 3.3.12) | {gap}, no test | ze resolves no received Type 2 ST route against DSD or ISD routes; the MUP plugin only encodes, decodes and originates MUP NLRIs; internal/component/bgp/plugins/nlri/mup/mup.go |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-7`](#draft-ietf-bess-mup-safi-3.3.12-7) A PE receiving a Type 2 ST route without a BGP MUP Extended Community MUST resolve the route using ISD routes for the default Interwork Segment, i.e., ISD routes that carry no BGP MUP Extended Community. (Section 3.3.12) | {gap}, no test | ze resolves no received Type 2 ST route against DSD or ISD routes; the MUP plugin only encodes, decodes and originates MUP NLRIs; internal/component/bgp/plugins/nlri/mup/mup.go |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1`](#draft-ietf-bess-mup-safi-3.3.1-1)

When advertising the Interwork Segment Discovery route, a PE MUST attach the export BGP Route Target Extended Community of the associated routing instance. (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2`](#draft-ietf-bess-mup-safi-3.3.1-2)

When advertising the Interwork Segment Discovery route, a PE MUST use the IPv6 address of the PE as the nexthop address in the MP_REACH_NLRI attribute. (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3`](#draft-ietf-bess-mup-safi-3.3.1-3)

The Interwork Segment Discovery route update MUST have a prefix SID attribute which the SID consists of the PE locater followed by a function. (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1`](#draft-ietf-bess-mup-safi-3.3.2-1)

When withdrawing the Interwork Segment Discovery route, a PE MUST attach the export BGP Route Target Extended Community of the associated routing instance. (Section 3.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1`](#draft-ietf-bess-mup-safi-3.3.4-1)

The address in the BGP-MUP NLRI MUST be a unique PE identifier. (Section 3.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2`](#draft-ietf-bess-mup-safi-3.3.4-2)

When announcing the Direct Segment Discovery route, a PE MUST attach a BGP MUP Extended community of the associated routing instance. (Section 3.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3`](#draft-ietf-bess-mup-safi-3.3.4-3)

When advertising the Direct Segment Discovery route, a PE MUST use the IPv6 address of the PE as the nexthop address in the MP_REACH_NLRI attribute. (Section 3.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4`](#draft-ietf-bess-mup-safi-3.3.4-4)

The Direct Segment Discovery route update MUST have a prefix SID attribute which the SID consists of the PE locator followed by a function. (Section 3.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1`](#draft-ietf-bess-mup-safi-3.3.7-1)

The MUP Controller MUST set the nexthop of the route to the address of the controller. (§3.3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-2`](#draft-ietf-bess-mup-safi-3.3.7-2)

The controller MUST announce this route using a AFI of the route and the SAFI of BGP-MUP to all other BGP speakers within the SRv6 domain. (§3.3.7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts EncodeRoute puts a T1ST in MP_REACH_NLRI with the route's AFI and SAFI 85; the clause 'to all other BGP speakers within the SRv6 domain' is not asserted, so the unit proves one clause of the row

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFCMUPAnnounceUsesRouteAFIWithMUPSAFI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L207) | unit/verify | unproven |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1`](#draft-ietf-bess-mup-safi-3.3.10-1)

The controller MUST attach a Route Target Extended Community of the routing instances in the PE accommodating the corresponding Interwork Segment. (§3.3.10)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2`](#draft-ietf-bess-mup-safi-3.3.10-2)

The controller MUST set the nexthop of the route to the address of the MUP Controller. (§3.3.10)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1-1`](#draft-ietf-bess-mup-safi-3.1-1)

Any other Route Types MUST be silently ignored upon a receipt if a BGP speaker supports only 3gpp-5G architecture type. (Section 3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the tagged unit drives ParseMUP, the decode-path parser, and asserts an unknown route type is returned as a parsed MUP value and the next NLRI is reached; it never asserts the unknown route is ignored on receipt. The ingress decision is RecognizeNLRI (rfc7606.go), which no tag of this row exercises

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFCMUPUnknownRouteTypeIsNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L161) | unit/verify | unproven |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1`](#draft-ietf-bess-mup-safi-3.3.3-1)

However, the receiving BGP speaker MUST ensure that the value of Address filed in the NLRI is an address of the originator of the locator value in the prefix SID attribute. (Section 3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2`](#draft-ietf-bess-mup-safi-3.3.3-2)

When a BGP speaker receives an MP_UNREACH_NLRI attribute update message it MUST delete the withdrawn Interwork Segment Discovery route from the routing instance table where it was created. (Section 3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1`](#draft-ietf-bess-mup-safi-3.3.6-1)

However, the receiving BGP speaker MUST ensure that the received nexthop value in the MP_REACH_NLRI attribute is identical to the originator of the locator value in the prefix SID attribute. (Section 3.3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2`](#draft-ietf-bess-mup-safi-3.3.6-2)

When a BGP speaker receives an MP_UNREACH_NLRI attribute update message it MUST delete the withdrawn Direct Segment Discovery route from the routing instance table where it was created. (Section 3.3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1`](#draft-ietf-bess-mup-safi-3.3.9-1)

The PE receiving Type 1 ST routes in MP_UNREACH_NLRI attribute MUST delete all the routes from the associated routing instance. (Section 3.3.9)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1`](#draft-ietf-bess-mup-safi-3.1.1-1)

If the AFI is IPv4, then the maximum value of the Prefix Length is 32 bits otherwise it is considered as a malformed NLRI. If the AFI is IPv6, then the maximum value of of the Prefix length is 128 bits otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2`](#draft-ietf-bess-mup-safi-3.1.1-2)

A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. (Section 3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1`](#draft-ietf-bess-mup-safi-3.1.2-1)

If the AFI is IPv4 then the address length is 4 octets otherwise it is considered as a malformed NLRI. If the AFI is IPv6 then the address length is 16 octets otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat- as-withdraw" [RFC7606]. (§3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1`](#draft-ietf-bess-mup-safi-3.1.3-1)

If the AFI is IPv4, then the maximum value of the Prefix Length field is 32. If the AFI is IPv6, then the maximum value of the Prefix Length field is 128. Any other length field is considered a a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat- as-withdraw" [RFC7606]. (§3.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1`](#draft-ietf-bess-mup-safi-3.1.3.1-1)

The TEID value of 0 is considered as an invalid and a malformed TEID. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2`](#draft-ietf-bess-mup-safi-3.1.3.1-2)

Endpoint Address field contains of an IPv4 address, then the value of the Endpoint Address Length field is 32. If the Endpoint Address field contains of an IPv6 Address, then the value of the Endpoint Address Length field is 128. Any other value is considered as an invalid and a malformed Endpoint Address. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3`](#draft-ietf-bess-mup-safi-3.1.3.1-3)

Source Address Length is 0 bytes then the Source address is not carried within the NLRI. A BGP speaker MAY have a local configuration for using a Source address. The exact mechanism of local configuration is outside the scope of this document. Source Address field contains of an IPv4 address, then the value of the Source Address Length field is 32. If the Source Address field contains of an IPv6 Address, then the value of the Source Address Length field is 128. Any other value is considered as an invalid and a malformed Endpoint Address. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6`](#draft-ietf-bess-mup-safi-3.1.3.1-6)

- < 0: the mandatory fields exceed the declared Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw. (§3.1.3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts ParseMUP returns ErrMUPTruncated, which is the decode path (DecodeNLRIHex, RunCLIDecode); nothing asserts Treat-as-withdraw, and ingress (SplitMUP framing, RecognizeNLRI arch/route type) stores such a route, so the test would stay green with the MUST unmet

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPT1STMandatoryFieldsWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L42) | unit/verify | revert, verified |
| positive | [`TestMUPT1STMandatoryFieldsWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L34) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7`](#draft-ietf-bess-mup-safi-3.1.3.1-7)

- 1: encoding is invalid (a valid TLV requires at minimum a Type byte and a Length byte); MUST be treated as Treat-as- withdraw. (Section 3.1.3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts ParseMUP returns ErrMUPTLV for one leftover octet on the decode path; nothing asserts Treat-as-withdraw, and ingress (SplitMUP framing, RecognizeNLRI arch/route type) never validates TLVs, so the route is stored

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L67) | unit/verify | revert, verified |
| positive | [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L59) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-8`](#draft-ietf-bess-mup-safi-3.1.3.1-8)

unknown TLV types MUST be ignored for local processing (§3.1.3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The positive drives ParseMUP (the decode path) with unknown TLV 0xF0 and asserts every mandatory field equals the TLV-less route, so a refusing decoder goes red. The negative tag sits on a truncated TLV, which is the -10 framing rule, not a violation of this row, and no {single-polarity} marker stands in for it. Ingress (SplitMUP framing, RecognizeNLRI) is never exercised, so a future ingress TLV check that refused unknown types would stay green.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L105) | unit/verify | revert, verified |
| positive | [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L93) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9`](#draft-ietf-bess-mup-safi-3.1.3.1-9)

unknown TLV types MUST be ignored for local processing and MUST be propagated unchanged when re- advertising the route to other BGP peers (§3.1.3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts MUP.Bytes() reproduces the parsed NLRI with the unknown TLV; re-advertisement does not re-encode through Bytes, it forwards the NLRI stored opaque by SplitMUP/insertPoolNLRIs, so the path the row names is not exercised

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L115) | unit/verify | revert, verified |
| positive | [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L111) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10`](#draft-ietf-bess-mup-safi-3.1.3.1-10)

any TLV parsing error MUST result in Treat-as-withdraw (§3.1.3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. asserts ParseMUP returns ErrMUPTLV for a TLV Length overrun on the decode path; nothing asserts Treat-as-withdraw, and ingress never walks TLVs, so the route is stored

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L73) | unit/verify | revert, verified |
| positive | [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L61) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1`](#draft-ietf-bess-mup-safi-3.1.4-1)

If the AFI is IPv4, then the maximum Endpoint length is 64 otherwise it is considered as a malformed NLRI. If the AFI is IPv6, then the maximum Endpoint length is 160 otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1`](#draft-ietf-bess-mup-safi-3.1.4.1-1)

The TEID value of 0 is considered as an invalid and a malformed TEID. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-3`](#draft-ietf-bess-mup-safi-3.1.4.1-3)

The Endpoint Length MUST NOT extend beyond the TEID field. (§3.1.4.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. The sentence binds the encoding of Endpoint Length, and ze's encoder (encode.go EncodeRoute, writeTEIDWithBits, parseTEIDWithBits) is driven by no tagged assertion, so an encoder that wrote address bits plus more than 32 stays green. The unit proves only receive-side refusal on the decode path: IPv4 Endpoint Length 64 parses with TEID 0x3039 and 72 is refused ErrMUPEndpointTooLong (types.go, one AFI-independent check). Ingress (SplitMUP, RecognizeNLRI) never applies the bound, so such a route is stored and forwarded.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPT2STEndpointLengthBoundedByTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L137) | unit/verify | revert, verified |
| positive | [`TestMUPT2STEndpointLengthBoundedByTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_mup_test.go#L127) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.5-1`](#draft-ietf-bess-mup-safi-3.1.5-1)

A TLV received in a route type for which it is not applicable MUST be ignored (Section 3.1.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPST1DoesNotInterpretInapplicableAddressTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_applicability_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestMUPST1IgnoresST2OnlyTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/tlv_applicability_test.go#L10) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4`](#draft-ietf-bess-mup-safi-3.1.3.1-4)

The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 1 ST route. (Section 3.1.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2`](#draft-ietf-bess-mup-safi-3.1.4.1-2)

The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 2 ST route. (Section 3.1.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3`](#draft-ietf-bess-mup-safi-3.3.3-3)

When a BGP speaker receives the Interwork Segment Dicovery routes with a MP_REACH_NLRI attribute without a prefix SID attribute, then it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied. (Section 3.3.3, 3.3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4`](#draft-ietf-bess-mup-safi-3.3.3-4)

If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied. (§3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3-1`](#draft-ietf-bess-mup-safi-3.3-1)

BGP speakers acting as a PE, and a MUP Controller MUST establish a BGP session to exchange BGP-MUP NLRIs for both, IPv4 and IPv6 AFIs. (Section 3.3)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. positive checks the ipv4/mup and ipv6/mup family constants and codec round trip, not a session negotiating and exchanging MUP NLRI for both AFIs; negative asserts the codec rejects non-MUP families, which the sentence does not state

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFCMUPRejectsNonMUPFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L133) | unit/verify | unproven |
| positive | [`TestRFCMUPFamiliesCoverBothAFIs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L84) | unit/verify | unproven |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4`](#draft-ietf-bess-mup-safi-3.3.1-4)

In 3GPP 5G specific case, if the BGP AFI is IPv4, the function MUST be GTP4.E [I-D.ietf-dmm-srv6-mobile-uplane], or MUST be GTP6.E [I-D.ietf-dmm-srv6-mobile-uplane] if the BGP AFI is IPv6. (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2`](#draft-ietf-bess-mup-safi-3.3.5-2)

When withdrawing the Direct Segment Discovery route, a BGP speaker MUST attach a BGP MUP Extended community of the associated routing instance. (Section 3.3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1`](#draft-ietf-bess-mup-safi-3.3.8-1)

The controller MUST advertise the withdraws of the Type 1 ST route. (Section 3.3.8)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2`](#draft-ietf-bess-mup-safi-3.3.9-2)

In an event where the received Type 1 ST route in MP_UNREACH_NLRI attribute does not have a Source address a receiving PE MUST delete all the matching Type 1 ST Routes with different Source addresses. (§3.3.9)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1`](#draft-ietf-bess-mup-safi-3.3.11-1)

The controller MUST advertise the withdraws of the Type 2 ST route. (Section 3.3.11)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-4`](#draft-ietf-bess-mup-safi-3.3.12-4)

A PE receiving a Type 2 ST route with a Direct Segment type BGP MUP Extended Community MUST resolve the route using the DSD routes matching that community (Section 3.3.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.12-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-5`](#draft-ietf-bess-mup-safi-3.3.12-5)

DSD routes MUST NOT be used to resolve Type 2 ST routes that do not carry a Direct Segment type BGP MUP Extended Community (Section 3.3.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.12-5, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-6`](#draft-ietf-bess-mup-safi-3.3.12-6)

A PE receiving a Type 2 ST route with an Interwork Segment type BGP MUP Extended Community MUST resolve the route using ISD routes that carry a matching Interwork Segment type BGP MUP Extended Community (Section 3.3.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.12-6, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-7`](#draft-ietf-bess-mup-safi-3.3.12-7)

A PE receiving a Type 2 ST route without a BGP MUP Extended Community MUST resolve the route using ISD routes for the default Interwork Segment, i.e., ISD routes that carry no BGP MUP Extended Community. (Section 3.3.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.12-7, so no unit is bound to it.

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/drafts/draft-ietf-bess-mup-safi.txt |
| Source fingerprint | cc8da36f0b20e05b |
| Record | rfc/extraction/draft-ietf-bess-mup-safi.json |
| Mapped sentences | 46 |
| Declined as scope | 20 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 0 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 1 | walked | not stated |
| `3.1.1` | not stated | 2 | walked | not stated |
| `3.1.2` | not stated | 2 | walked | not stated |
| `3.1.3` | not stated | 2 | walked | not stated |
| `3.1.3.1` | not stated | 12 | walked | not stated |
| `3.1.4` | not stated | 2 | walked | not stated |
| `3.1.4.1` | not stated | 9 | walked | not stated |
| `3.1.5` | not stated | 1 | walked | not stated |
| `3.1.5.1` | not stated | 0 | walked | not stated |
| `3.1.5.2` | not stated | 0 | walked | not stated |
| `3.1.5.3` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.2.1` | not stated | 0 | walked | not stated |
| `3.2.2` | not stated | 0 | walked | not stated |
| `3.2.3` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 1 | walked | not stated |
| `3.3.1` | not stated | 4 | walked | not stated |
| `3.3.2` | not stated | 1 | walked | not stated |
| `3.3.3` | not stated | 6 | walked | not stated |
| `3.3.4` | not stated | 4 | walked | not stated |
| `3.3.5` | not stated | 1 | walked | not stated |
| `3.3.6` | not stated | 6 | walked | not stated |
| `3.3.7` | not stated | 2 | walked | not stated |
| `3.3.8` | not stated | 1 | walked | not stated |
| `3.3.9` | not stated | 2 | walked | not stated |
| `3.3.10` | not stated | 2 | walked | not stated |
| `3.3.11` | not stated | 1 | walked | not stated |
| `3.3.12` | not stated | 4 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
| `7` | not stated | 0 | walked | not stated |
| `7.1` | not stated | 0 | walked | not stated |
| `7.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.1.3:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.1.3.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.1.3.1:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.1.3.1:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.1.3.1:11` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Treat-as-withdraw consequence of the Type 1 ST architecture encoding rule mapped at site 3.1.3.1:10, which the row already carries as its otherwise clause. | A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. |
| `3.1.3.1:12` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.1.4:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.1.4.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.1.4.1:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.1.4.1 repeats the Section 3.1.3.1 TLV length computation word for word; the negative-remainder rule is mapped at site 3.1.3.1:7. | - < 0: the mandatory fields exceed the declared Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw. |
| `3.1.4.1:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.1.4.1 repeats the Section 3.1.3.1 TLV length computation word for word; the one-octet-remainder rule is mapped at site 3.1.3.1:8. | - 1: encoding is invalid (a valid TLV requires at minimum a Type byte and a Length byte); MUST be treated as Treat-as- withdraw. |
| `3.1.4.1:6` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 3.1.4.1 repeats the Section 3.1.3.1 TLV parsing sentence word for word; it is mapped at site 3.1.3.1:9. | Parse TLVs one by one per Section 3.1.5; unknown TLV types MUST be ignored for local processing and MUST be propagated unchanged when re- advertising the route to other BGP peers; any TLV parsing error MUST result in Treat-as-withdraw; TLVs are not part of the NLRI key for route processing. |
| `3.1.4.1:8` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates the Treat-as-withdraw consequence of the Type 2 ST architecture encoding rule mapped at site 3.1.4.1:7, which the row already carries as its otherwise clause. | A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. |
| `3.1.4.1:9` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.3.3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.3.3:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.3.6:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates for the Direct Segment Discovery route the nexthop-locator mismatch ruling mapped at site 3.3.3:2; the row cites both Section 3.3.3 and Section 3.3.6. | If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied. |
| `3.3.6:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.3.6:4` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Restates for the Direct Segment Discovery route the missing-prefix-SID ruling mapped at site 3.3.3:4; the row cites both Section 3.3.3 and Section 3.3.6. | When a BGP speaker receives a MP_REACH_NLRI attribute update message with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied. |
| `3.3.6:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |

## Superseded

No document obsoletes DRAFT-IETF-BESS-MUP-SAFI, so its obligations are stated where they were written.
