# DRAFT-IETF-BESS-MUP-SAFI - BGP Extensions for the Mobile User Plane (MUP) SAFI

Partial. Every requirement this repository extracted from DRAFT-IETF-BESS-MUP-SAFI, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 2.5% | 1 of 40 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 5.0% | 2 of 40 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 40 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 40 | of 61 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 40 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 92.5% | 37 of 40 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 40 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 61 |
| Gated MUST-level | 40 |
| Obligations that bind Ze | 40 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 37 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/draft-ietf-bess-mup-safi.md` |
| Requirement shard | `rfc/requirements/draft-ietf-bess-mup-safi.md` |
| RFC text | `rfc/drafts/draft-ietf-bess-mup-safi.txt` |

## Enrolment

Enrolled: BGP Extensions for Mobile User Plane (MUP) SAFI

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

The BGP-MUP NLRI codec only: ipv4/mup and ipv6/mup family registration, ISD/DSD/T1ST/T2ST encoding from config and route commands, header plus RD decoding, the RFC 7606 Section 5.4 ruling that discards a route whose Architecture Type is not 1 or whose Route Type is outside 1..4 at ingress ([`internal/component/bgp/plugins/nlri/mup/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc7606.go), RecognizeNLRI), MUP extended-community config syntax, and family-generic MP_REACH announcement (internal/component/bgp/plugins/nlri/mup). Thirty-seven MUST gaps are annotated per line in [`rfc/short/draft-ietf-bess-mup-safi.md`](https://github.com/ze-software/ze/blob/main/rfc/short/draft-ietf-bess-mup-safi.md). Withdrawal: no MUP NLRI reaches the family-generic MP_UNREACH encoder ([`internal/component/bgp/reactor/peer_rib_routes.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_rib_routes.go)) -- its callers read the PeerOpWithdraw queue, and neither withdrawal entry point parses SAFI 85 ([`internal/component/bgp/plugins/cmd/update/update_text_nlri.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/update_text_nlri.go), [`internal/component/bgp/plugins/cmd/announce/announce.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/announce.go)) -- so a Type 1 ST or Type 2 ST withdrawal cannot be emitted (3.3.8-1, 3.3.11-1). Receive side: nlrisplit registers SplitMUP for SAFI 85 since 2026-08-04 ([`internal/core/bgp/nlri/nlrisplit/register.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/register.go)), so a received MUP route is stored as an opaque Adj-RIB-In entry (insertPoolNLRIs) and a withdrawal deletes exactly the NLRI it names (removePoolNLRIs, [`internal/component/bgp/plugins/rib/rib.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib.go)). Four routing-instance obligations stay open because ze models no MUP routing instance and no route-type-aware wildcard delete exists ([`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2`](#draft-ietf-bess-mup-safi-3.3.3-2), 3.3.6-2, 3.3.9-1, 3.3.9-2). ParseMUP keeps the route-type body opaque ([`internal/component/bgp/plugins/nlri/mup/types.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/types.go)), so no RFC 7606 treat-as-withdraw fires on an out-of-range prefix length, wrong-size address, zero TEID, invalid endpoint or source length, over-long T2ST endpoint length, or non-3gpp-5g architecture type (3.1.1-1, 3.1.1-2, 3.1.2-1, 3.1.3-1, 3.1.3.1-1, 3.1.3.1-2, 3.1.3.1-3, 3.1.3.1-4, 3.1.4-1, 3.1.4.1-1, 3.1.4.1-2), nor on a missing Prefix-SID, a nexthop/locator mismatch, or a Type 2 ST route without the BGP MUP Extended Community (3.3.3-1, 3.3.3-3, 3.3.3-4, 3.3.6-1, 3.3.12-1). Send side: ze runs no MUP PE or MUP Controller function, so route targets, the BGP MUP Extended Community, the Prefix-SID, the GTP4.E/GTP6.E function, the required T1ST TEID and Endpoint Address, and the PE or controller IPv6 nexthop are whatever the operator configures rather than derived (3.3.1-1, 3.3.1-2, 3.3.1-3, 3.3.1-4, 3.3.2-1, 3.3.4-1, 3.3.4-2, 3.3.4-3, 3.3.4-4, 3.3.5-1, 3.3.5-2, 3.3.7-1, 3.3.7-3, 3.3.10-1, 3.3.10-2).

**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 39 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **40** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`DRAFT-IETF-BESS-MUP-SAFI-3.3-1`](#draft-ietf-bess-mup-safi-3.3-1)

**Annotated instead of tested (39):** [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1`](#draft-ietf-bess-mup-safi-3.3.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2`](#draft-ietf-bess-mup-safi-3.3.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3`](#draft-ietf-bess-mup-safi-3.3.1-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1`](#draft-ietf-bess-mup-safi-3.3.2-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1`](#draft-ietf-bess-mup-safi-3.3.4-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2`](#draft-ietf-bess-mup-safi-3.3.4-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3`](#draft-ietf-bess-mup-safi-3.3.4-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4`](#draft-ietf-bess-mup-safi-3.3.4-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.5-1`](#draft-ietf-bess-mup-safi-3.3.5-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1`](#draft-ietf-bess-mup-safi-3.3.7-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-2`](#draft-ietf-bess-mup-safi-3.3.7-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1`](#draft-ietf-bess-mup-safi-3.3.10-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2`](#draft-ietf-bess-mup-safi-3.3.10-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.1-1`](#draft-ietf-bess-mup-safi-3.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1`](#draft-ietf-bess-mup-safi-3.3.3-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2`](#draft-ietf-bess-mup-safi-3.3.3-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1`](#draft-ietf-bess-mup-safi-3.3.6-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2`](#draft-ietf-bess-mup-safi-3.3.6-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1`](#draft-ietf-bess-mup-safi-3.3.9-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-1`](#draft-ietf-bess-mup-safi-3.3.12-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1`](#draft-ietf-bess-mup-safi-3.1.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2`](#draft-ietf-bess-mup-safi-3.1.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1`](#draft-ietf-bess-mup-safi-3.1.2-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1`](#draft-ietf-bess-mup-safi-3.1.3-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1`](#draft-ietf-bess-mup-safi-3.1.3.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2`](#draft-ietf-bess-mup-safi-3.1.3.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3`](#draft-ietf-bess-mup-safi-3.1.3.1-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1`](#draft-ietf-bess-mup-safi-3.1.4-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1`](#draft-ietf-bess-mup-safi-3.1.4.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4`](#draft-ietf-bess-mup-safi-3.1.3.1-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2`](#draft-ietf-bess-mup-safi-3.1.4.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3`](#draft-ietf-bess-mup-safi-3.3.3-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4`](#draft-ietf-bess-mup-safi-3.3.3-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4`](#draft-ietf-bess-mup-safi-3.3.1-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2`](#draft-ietf-bess-mup-safi-3.3.5-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1`](#draft-ietf-bess-mup-safi-3.3.8-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-3`](#draft-ietf-bess-mup-safi-3.3.7-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2`](#draft-ietf-bess-mup-safi-3.3.9-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1`](#draft-ietf-bess-mup-safi-3.3.11-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1` | PE advertising ISD route must attach export BGP Route Target Extended Community of the associated routing instance (Section 3.3.1) | MUST | 3.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66) and ze models no routing instance with export route targets, so no export route target is derived for an ISD advertisement |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2` | PE advertising ISD route must use IPv6 address of PE as nexthop in MP_REACH_NLRI (Section 3.3.1) | MUST | 3.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) and accepts an IPv4 next-hop for an ISD route, so nothing requires the IPv6 address of the PE |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3` | ISD route update must have a prefix SID attribute (Section 3.3.1) | MUST | 3.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71), so an ISD route configured without a prefix SID is advertised without one |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1` | PE withdrawing ISD route must attach export BGP Route Target Extended Community (Section 3.3.2) | MUST | 3.3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a MUP withdrawal is built as a bare MP_UNREACH_NLRI with no path attributes (internal/component/bgp/reactor/peer_rib_routes.go:182-197), so an ISD withdrawal carries no export route target |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1` | DSD address in NLRI must be a unique PE identifier (Section 3.3.4) | MUST | 3.3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseDSDFields accepts any parsable address as the DSD NLRI address (internal/component/bgp/plugins/nlri/mup/encode.go:301-310) and never checks that it identifies the PE uniquely |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2` | PE announcing DSD route must attach a BGP MUP Extended Community (Section 3.3.4) | MUST | 3.3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66), so a DSD advertisement carries a BGP MUP Extended Community only when the operator writes one into the route |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3` | PE advertising DSD route must use IPv6 address of PE as nexthop in MP_REACH_NLRI (Section 3.3.4) | MUST | 3.3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) and accepts an IPv4 next-hop for a DSD route, so nothing requires the IPv6 address of the PE |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4` | DSD route update must have a prefix SID attribute (Section 3.3.4) | MUST | 3.3.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71), so a DSD route configured without a prefix SID is advertised without one |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.5-1` | BGP speaker announcing T1ST must attach a BGP MUP Extended Community (Section 3.3.5) | MUST | 3.3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66), so a Type 1 ST advertisement carries a BGP MUP Extended Community only when the operator writes one into the route |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1` | MUP Controller must set nexthop of T1ST route to the controller address (Section 3.3.7) | MUST | 3.3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) for a Type 1 ST route and ze runs no MUP Controller function that substitutes the controller address |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.7-2` | Controller must announce T1ST route using AFI of the route and SAFI BGP-MUP to all BGP speakers in SRv6 domain (Section 3.3.7) | MUST | 3.3.7 | **positive:** `unit/verify` [`TestRFCMUPAnnounceUsesRouteAFIWithMUPSAFI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L207). **negative:** no negative test. **{single-polarity}:** EncodeRoute emits MUP NLRI only under SAFI 85 with the AFI taken from the route family (internal/component/bgp/plugins/nlri/mup/encode.go:183-199), so no non-conformant AFI/SAFI emission exists to reject |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1` | Controller must attach Route Target Extended Community of routing instances in the PE for T2ST (Section 3.3.10) | MUST | 3.3.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66) and ze models no routing instance with export route targets, so a Type 2 ST advertisement carries a route target only when the operator writes one into the route |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2` | Controller must set nexthop of T2ST route to MUP Controller address (Section 3.3.10) | MUST | 3.3.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) for a Type 2 ST route and ze runs no MUP Controller function that substitutes the controller address |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1-1` | Unknown Route Types for supported Architecture Types must be silently ignored (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestRFCMUPUnknownRouteTypeIsNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L161). **negative:** no negative test. **{single-polarity}:** ParseMUP accepts any route type under the 3gpp-5g architecture type and returns the bytes after the declared Length (internal/component/bgp/plugins/nlri/mup/types.go:118-152), so there is no rejection path for an unknown route type to drive negatively |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1` | Receiver must ensure ISD Address field value is address of originator of locator in prefix SID attribute (Section 3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the ISD Address field is never checked against the originator of the prefix SID locator |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2` | On MP_UNREACH_NLRI, receiver must delete withdrawn ISD route from routing instance table (Section 3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives an ISD withdrawal through it |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1` | Receiver must ensure DSD nexthop in MP_REACH_NLRI is identical to originator of locator in prefix SID attribute (Section 3.3.6) | MUST | 3.3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the DSD nexthop is never compared with the originator of the prefix SID locator |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2` | On MP_UNREACH_NLRI, receiver must delete withdrawn DSD route from routing instance table (Section 3.3.6) | MUST | 3.3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives a DSD withdrawal through it |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1` | PE receiving T1ST routes in MP_UNREACH_NLRI must delete all routes from associated routing instance (Section 3.3.9) | MUST | 3.3.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). The delete is one NLRI for one NLRI. Nothing reads the Type 1 ST route type to delete every route of the associated routing instance, and ze models no such instance |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.12-1` | PE must handle T2ST without MUP Extended Community as treat-as-withdraw (Section 3.3.12) | MUST | 3.3.12 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) without consulting the UPDATE extended communities, so a Type 2 ST route arriving without the BGP MUP Extended Community is not treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1` | ISD with prefix length exceeding max for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the ISD prefix length, so a prefix length above 32 for AFI 1 or 128 for AFI 2 is accepted as a valid route |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2` | Speaker must skip malformed NLRIs and continue processing rest of Update message (Section 3.1.1) | MUST | 3.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP applies no semantic validation (internal/component/bgp/plugins/nlri/mup/types.go:118-152), so no NLRI is ever classed malformed and skipped, and a truncated NLRI aborts the parse with ErrMUPTruncated (internal/component/bgp/plugins/nlri/mup/types.go:127-129) instead of continuing with the rest of the Update |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1` | DSD with wrong address size for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.2) | MUST | 3.1.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never measures the DSD address against the AFI, so a 4-octet address under AFI 2 or a 16-octet address under AFI 1 is accepted |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1` | T1ST with prefix length exceeding max for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.3) | MUST | 3.1.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST prefix length, so a prefix length above 32 for AFI 1 or 128 for AFI 2 is accepted as a valid route |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1` | T1ST with TEID=0: treat-as-withdraw per RFC 7606 (Section 3.1.3.1) | MUST | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST TEID, and parseTEIDWithBits maps an absent TEID to zero bits which writeT1STData then omits from the NLRI (internal/component/bgp/plugins/nlri/mup/encode.go:435-450, internal/component/bgp/plugins/nlri/mup/encode.go:396-399) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2` | T1ST with invalid Endpoint Address Length (not 32 or 128): treat-as-withdraw (Section 3.1.3.1) | MUST | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST Endpoint Address Length, while writeT1STData derives it from the configured address (internal/component/bgp/plugins/nlri/mup/encode.go:404-409), so a received length other than 32 or 128 is accepted |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3` | T1ST with invalid Source Address Length (not 0, 32, or 128): treat-as-withdraw (Section 3.1.3.1) | MUST | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST Source Address Length, while writeT1STData derives it from the configured address (internal/component/bgp/plugins/nlri/mup/encode.go:411-416), so a received length other than 0, 32 or 128 is accepted |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1` | T2ST with Endpoint Length exceeding max for AFI: treat-as-withdraw (Section 3.1.4) | MUST | 3.1.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T2ST Endpoint Length, so a combined length above 64 for AFI 1 or 160 for AFI 2 is accepted |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1` | T2ST with TEID=0: treat-as-withdraw per RFC 7606 (Section 3.1.4.1) | MUST | 3.1.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T2ST TEID, while writeT2STData writes whatever parseTEIDWithBits yields, zero included (internal/component/bgp/plugins/nlri/mup/encode.go:422-431) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4` | T1ST NLRI architecture field MUST be encoded as specified for 3gpp-5g; otherwise treat-as-withdraw (Section 3.1.3.1) | MUST | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go:246) but ParseMUP accepts any architecture byte and keeps parsing (internal/component/bgp/plugins/nlri/mup/types.go:123), so a T1ST NLRI encoded for another architecture is not treated as withdraw |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2` | T2ST NLRI architecture field MUST be encoded as specified for 3gpp-5g; otherwise treat-as-withdraw (Section 3.1.4.1) | MUST | 3.1.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go:246) but ParseMUP accepts any architecture byte and keeps parsing (internal/component/bgp/plugins/nlri/mup/types.go:123), so a T2ST NLRI encoded for another architecture is not treated as withdraw |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3` | ISD/DSD without prefix SID attribute: treat-as-withdraw (Section 3.3.3, 3.3.6) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) without consulting the UPDATE path attributes, so an ISD or DSD route arriving without a Prefix-SID attribute is not treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4` | Nexthop/locator mismatch on ISD/DSD: treat-as-withdraw (Section 3.3.3, 3.3.6) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so a mismatch between the nexthop and the locator originator is never detected on ISD or DSD routes |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3-1` | PE and MUP Controller MUST establish a BGP session to exchange BGP-MUP NLRIs for both IPv4 and IPv6 AFIs (Section 3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestRFCMUPFamiliesCoverBothAFIs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L84). **negative:** `unit/verify` [`TestRFCMUPRejectsNonMUPFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L133) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4` | ISD prefix SID function MUST be GTP4.E if BGP AFI is IPv4, or MUST be GTP6.E if BGP AFI is IPv6 (Section 3.3.1) | MUST | 3.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71) and passes its bytes through unchanged, and ze decodes no SRv6 endpoint function, so the GTP4.E/GTP6.E function is never tied to the BGP AFI |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2` | When withdrawing DSD route, BGP speaker MUST attach a BGP MUP Extended community of the associated routing instance (Section 3.3.5) | MUST | 3.3.5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** a MUP withdrawal is built as a bare MP_UNREACH_NLRI with no path attributes (internal/component/bgp/reactor/peer_rib_routes.go:182-197), so a DSD withdrawal carries no BGP MUP Extended Community |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1` | Controller MUST advertise the withdrawal of the Type 1 ST route (Section 3.3.8) | MUST | 3.3.8 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no MUP NLRI can reach the family-generic MP_UNREACH encoder (internal/component/bgp/reactor/peer_rib_routes.go:170). Its only callers take the NLRI from a PeerOpWithdraw queue entry (internal/component/bgp/reactor/peer_initial_sync.go:237, :377) filled by QueueWithdraw (internal/component/bgp/reactor/peer.go:886-893), and the two withdrawal entry points that feed it parse no SAFI 85: text mode rejects the family in isSupportedFamily, whose list stops at SAFI 73 (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:375-403), and the announce/withdraw registry builds only unicast and FlowSpec NLRIs (internal/component/bgp/plugins/cmd/announce/announce.go:257, :304, :415). NewMUP and NewMUPFull (internal/component/bgp/plugins/nlri/mup/types.go:93, :103) have no non-test caller, and nlrisplit registers no SAFI 85 splitter (internal/core/bgp/nlri/nlrisplit/register.go:9-24) so no received MUP route is stored to be withdrawn either |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.7-3` | Controller MUST advertise the Type 1 ST route with Destination prefix, TEID, QFI, Endpoint Address, and optionally Source Address (Section 3.3.7) | MUST | 3.3.7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseT1STFields requires only the destination prefix (internal/component/bgp/plugins/nlri/mup/encode.go:313-316) and writeT1STData omits the TEID field when no TEID is configured and the Endpoint Address field when no endpoint is configured (internal/component/bgp/plugins/nlri/mup/encode.go:396-409), so a Type 1 ST route is advertised without them |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2` | PE receiving T1ST in MP_UNREACH_NLRI without Source address MUST delete all matching T1ST routes with different Source addresses (Section 3.3.9) | MUST | 3.3.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). The opaque key is the whole NLRI, Source address included, so a Source-less Type 1 ST withdrawal matches no stored entry. No wildcard delete over differing Source addresses exists |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1` | Controller MUST advertise the withdrawal of the Type 2 ST route (Section 3.3.11) | MUST | 3.3.11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the same missing path as DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1 -- the family-generic MP_UNREACH encoder (internal/component/bgp/reactor/peer_rib_routes.go:170) is reachable only through PeerOpWithdraw (internal/component/bgp/reactor/peer.go:886-893, internal/component/bgp/reactor/peer_initial_sync.go:237, :377), and neither withdrawal entry point produces a SAFI 85 NLRI: isSupportedFamily omits it (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:375-403) and the announce registry builds only unicast and FlowSpec NLRIs (internal/component/bgp/plugins/cmd/announce/announce.go:257, :304, :415) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-5` | Receiver of ISD routes should ignore nexthop in MP_REACH_NLRI and use prefix SID locator instead (Section 3.3.3) | SHOULD | 3.3.3 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.7-4` | Controller should attach Route Target Extended Community which PEs are importing for T1ST (Section 3.3.7) | SHOULD | 3.3.7 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-3` | PE should use received Tunnel Endpoint Address in T1ST NLRI as key to lookup associated ISD route (Section 3.3.9) | SHOULD | 3.3.9 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-3` | Receiver of DSD routes should ignore the nexthop in MP_REACH_NLRI attribute (Section 3.3.6) | SHOULD | 3.3.6 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-4` | PE receiving T1ST routes should ignore the received nexthop in MP_REACH_NLRI (Section 3.3.9) | SHOULD | 3.3.9 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-5` | PE should generate forwarding SID for GTP4/6.E based on SRv6 MUP procedures (Section 3.3.9) | SHOULD | 3.3.9 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-6` | If PE cannot generate prefix SID, it should mark the received T1ST route as invalid (Section 3.3.9) | SHOULD | 3.3.9 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.12-2` | Receiver of T2ST routes should ignore received nexthop in MP_REACH_NLRI (Section 3.3.12) | SHOULD | 3.3.12 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.12-3` | PE receiving T2ST without BGP MUP Extended community should consider the route malformed (Section 3.3.12) | SHOULD | 3.3.12 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.8-2` | When withdrawing T1ST, controller should attach Route Target Extended community for the corresponding Direct segment (Section 3.3.8) | SHOULD | 3.3.8 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.10-3` | When advertising T2ST, controller should attach BGP MUP Extended community for the Direct segment (Section 3.3.10) | SHOULD | 3.3.10 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.11-2` | When withdrawing T2ST, controller should attach BGP MUP Extended community and Route Target Extended community (Section 3.3.11) | SHOULD | 3.3.11 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-4-1` | RFC 5925 authentication should be used where authentication of BGP control packets is needed (Section 4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-4-2` | PEs and MUP Controller should not establish BGP sessions with untrusted domains without explicit configuration (Section 4) | SHOULD NOT | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-4-3` | RFC 5925 procedures should be enforced at untrusted domain boundaries (Section 4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-4-4` | Establishing BGP sessions over encrypted paths should be considered to protect from eavesdropping (Section 4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-4-5` | PEs should impose an upper bound on number of routes stored to protect control plane load (Section 4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1-2` | Implementation may log an error when unknown Route Types are ignored (Section 3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-5` | BGP speaker may have local configuration for using a Source address when Source Address Length is 0 in T1ST (Section 3.1.3.1) | MAY | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.1-5` | ISD IP prefix may include a gNodeB address connecting to the PE (Section 3.3.1) | MAY | 3.3.1 | **positive:** no positive test. **negative:** no negative test |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.4-5` | DSD prefix SID function may be End.DT4/6 or End.DX4/6 (Section 3.3.4) | MAY | 3.3.4 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1`](#draft-ietf-bess-mup-safi-3.3.1-1) PE advertising ISD route must attach export BGP Route Target Extended Community of the associated routing instance (Section 3.3.1) | {gap}, no test | parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66) and ze models no routing instance with export route targets, so no export route target is derived for an ISD advertisement |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2`](#draft-ietf-bess-mup-safi-3.3.1-2) PE advertising ISD route must use IPv6 address of PE as nexthop in MP_REACH_NLRI (Section 3.3.1) | {gap}, no test | EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) and accepts an IPv4 next-hop for an ISD route, so nothing requires the IPv6 address of the PE |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3`](#draft-ietf-bess-mup-safi-3.3.1-3) ISD route update must have a prefix SID attribute (Section 3.3.1) | {gap}, no test | parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71), so an ISD route configured without a prefix SID is advertised without one |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1`](#draft-ietf-bess-mup-safi-3.3.2-1) PE withdrawing ISD route must attach export BGP Route Target Extended Community (Section 3.3.2) | {gap}, no test | a MUP withdrawal is built as a bare MP_UNREACH_NLRI with no path attributes (internal/component/bgp/reactor/peer_rib_routes.go:182-197), so an ISD withdrawal carries no export route target |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1`](#draft-ietf-bess-mup-safi-3.3.4-1) DSD address in NLRI must be a unique PE identifier (Section 3.3.4) | {gap}, no test | parseDSDFields accepts any parsable address as the DSD NLRI address (internal/component/bgp/plugins/nlri/mup/encode.go:301-310) and never checks that it identifies the PE uniquely |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2`](#draft-ietf-bess-mup-safi-3.3.4-2) PE announcing DSD route must attach a BGP MUP Extended Community (Section 3.3.4) | {gap}, no test | parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66), so a DSD advertisement carries a BGP MUP Extended Community only when the operator writes one into the route |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3`](#draft-ietf-bess-mup-safi-3.3.4-3) PE advertising DSD route must use IPv6 address of PE as nexthop in MP_REACH_NLRI (Section 3.3.4) | {gap}, no test | EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) and accepts an IPv4 next-hop for a DSD route, so nothing requires the IPv6 address of the PE |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4`](#draft-ietf-bess-mup-safi-3.3.4-4) DSD route update must have a prefix SID attribute (Section 3.3.4) | {gap}, no test | parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71), so a DSD route configured without a prefix SID is advertised without one |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.5-1`](#draft-ietf-bess-mup-safi-3.3.5-1) BGP speaker announcing T1ST must attach a BGP MUP Extended Community (Section 3.3.5) | {gap}, no test | parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66), so a Type 1 ST advertisement carries a BGP MUP Extended Community only when the operator writes one into the route |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1`](#draft-ietf-bess-mup-safi-3.3.7-1) MUP Controller must set nexthop of T1ST route to the controller address (Section 3.3.7) | {gap}, no test | EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) for a Type 1 ST route and ze runs no MUP Controller function that substitutes the controller address |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1`](#draft-ietf-bess-mup-safi-3.3.10-1) Controller must attach Route Target Extended Community of routing instances in the PE for T2ST (Section 3.3.10) | {gap}, no test | parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66) and ze models no routing instance with export route targets, so a Type 2 ST advertisement carries a route target only when the operator writes one into the route |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2`](#draft-ietf-bess-mup-safi-3.3.10-2) Controller must set nexthop of T2ST route to MUP Controller address (Section 3.3.10) | {gap}, no test | EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) for a Type 2 ST route and ze runs no MUP Controller function that substitutes the controller address |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1`](#draft-ietf-bess-mup-safi-3.3.3-1) Receiver must ensure ISD Address field value is address of originator of locator in prefix SID attribute (Section 3.3.3) | {gap}, no test | DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the ISD Address field is never checked against the originator of the prefix SID locator |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2`](#draft-ietf-bess-mup-safi-3.3.3-2) On MP_UNREACH_NLRI, receiver must delete withdrawn ISD route from routing instance table (Section 3.3.3) | {gap}, no test | nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives an ISD withdrawal through it |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1`](#draft-ietf-bess-mup-safi-3.3.6-1) Receiver must ensure DSD nexthop in MP_REACH_NLRI is identical to originator of locator in prefix SID attribute (Section 3.3.6) | {gap}, no test | DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the DSD nexthop is never compared with the originator of the prefix SID locator |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2`](#draft-ietf-bess-mup-safi-3.3.6-2) On MP_UNREACH_NLRI, receiver must delete withdrawn DSD route from routing instance table (Section 3.3.6) | {gap}, no test | nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives a DSD withdrawal through it |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1`](#draft-ietf-bess-mup-safi-3.3.9-1) PE receiving T1ST routes in MP_UNREACH_NLRI must delete all routes from associated routing instance (Section 3.3.9) | {gap}, no test | nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). The delete is one NLRI for one NLRI. Nothing reads the Type 1 ST route type to delete every route of the associated routing instance, and ze models no such instance |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-1`](#draft-ietf-bess-mup-safi-3.3.12-1) PE must handle T2ST without MUP Extended Community as treat-as-withdraw (Section 3.3.12) | {gap}, no test | DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) without consulting the UPDATE extended communities, so a Type 2 ST route arriving without the BGP MUP Extended Community is not treated as withdrawn |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1`](#draft-ietf-bess-mup-safi-3.1.1-1) ISD with prefix length exceeding max for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.1) | {gap}, no test | ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the ISD prefix length, so a prefix length above 32 for AFI 1 or 128 for AFI 2 is accepted as a valid route |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2`](#draft-ietf-bess-mup-safi-3.1.1-2) Speaker must skip malformed NLRIs and continue processing rest of Update message (Section 3.1.1) | {gap}, no test | ParseMUP applies no semantic validation (internal/component/bgp/plugins/nlri/mup/types.go:118-152), so no NLRI is ever classed malformed and skipped, and a truncated NLRI aborts the parse with ErrMUPTruncated (internal/component/bgp/plugins/nlri/mup/types.go:127-129) instead of continuing with the rest of the Update |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1`](#draft-ietf-bess-mup-safi-3.1.2-1) DSD with wrong address size for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.2) | {gap}, no test | ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never measures the DSD address against the AFI, so a 4-octet address under AFI 2 or a 16-octet address under AFI 1 is accepted |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1`](#draft-ietf-bess-mup-safi-3.1.3-1) T1ST with prefix length exceeding max for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.3) | {gap}, no test | ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST prefix length, so a prefix length above 32 for AFI 1 or 128 for AFI 2 is accepted as a valid route |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1`](#draft-ietf-bess-mup-safi-3.1.3.1-1) T1ST with TEID=0: treat-as-withdraw per RFC 7606 (Section 3.1.3.1) | {gap}, no test | ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST TEID, and parseTEIDWithBits maps an absent TEID to zero bits which writeT1STData then omits from the NLRI (internal/component/bgp/plugins/nlri/mup/encode.go:435-450, internal/component/bgp/plugins/nlri/mup/encode.go:396-399) |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2`](#draft-ietf-bess-mup-safi-3.1.3.1-2) T1ST with invalid Endpoint Address Length (not 32 or 128): treat-as-withdraw (Section 3.1.3.1) | {gap}, no test | ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST Endpoint Address Length, while writeT1STData derives it from the configured address (internal/component/bgp/plugins/nlri/mup/encode.go:404-409), so a received length other than 32 or 128 is accepted |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3`](#draft-ietf-bess-mup-safi-3.1.3.1-3) T1ST with invalid Source Address Length (not 0, 32, or 128): treat-as-withdraw (Section 3.1.3.1) | {gap}, no test | ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T1ST Source Address Length, while writeT1STData derives it from the configured address (internal/component/bgp/plugins/nlri/mup/encode.go:411-416), so a received length other than 0, 32 or 128 is accepted |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1`](#draft-ietf-bess-mup-safi-3.1.4-1) T2ST with Endpoint Length exceeding max for AFI: treat-as-withdraw (Section 3.1.4) | {gap}, no test | ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T2ST Endpoint Length, so a combined length above 64 for AFI 1 or 160 for AFI 2 is accepted |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1`](#draft-ietf-bess-mup-safi-3.1.4.1-1) T2ST with TEID=0: treat-as-withdraw per RFC 7606 (Section 3.1.4.1) | {gap}, no test | ParseMUP keeps the route-type-specific bytes opaque after the RD (internal/component/bgp/plugins/nlri/mup/types.go:137-149) and never reads the T2ST TEID, while writeT2STData writes whatever parseTEIDWithBits yields, zero included (internal/component/bgp/plugins/nlri/mup/encode.go:422-431) |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4`](#draft-ietf-bess-mup-safi-3.1.3.1-4) T1ST NLRI architecture field MUST be encoded as specified for 3gpp-5g; otherwise treat-as-withdraw (Section 3.1.3.1) | {gap}, no test | writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go:246) but ParseMUP accepts any architecture byte and keeps parsing (internal/component/bgp/plugins/nlri/mup/types.go:123), so a T1ST NLRI encoded for another architecture is not treated as withdraw |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2`](#draft-ietf-bess-mup-safi-3.1.4.1-2) T2ST NLRI architecture field MUST be encoded as specified for 3gpp-5g; otherwise treat-as-withdraw (Section 3.1.4.1) | {gap}, no test | writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go:246) but ParseMUP accepts any architecture byte and keeps parsing (internal/component/bgp/plugins/nlri/mup/types.go:123), so a T2ST NLRI encoded for another architecture is not treated as withdraw |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3`](#draft-ietf-bess-mup-safi-3.3.3-3) ISD/DSD without prefix SID attribute: treat-as-withdraw (Section 3.3.3, 3.3.6) | {gap}, no test | DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) without consulting the UPDATE path attributes, so an ISD or DSD route arriving without a Prefix-SID attribute is not treated as withdrawn |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4`](#draft-ietf-bess-mup-safi-3.3.3-4) Nexthop/locator mismatch on ISD/DSD: treat-as-withdraw (Section 3.3.3, 3.3.6) | {gap}, no test | DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so a mismatch between the nexthop and the locator originator is never detected on ISD or DSD routes |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4`](#draft-ietf-bess-mup-safi-3.3.1-4) ISD prefix SID function MUST be GTP4.E if BGP AFI is IPv4, or MUST be GTP6.E if BGP AFI is IPv6 (Section 3.3.1) | {gap}, no test | parseConfigRoute adds the Prefix-SID attribute only when the config supplies one (internal/component/bgp/plugins/nlri/mup/config.go:71) and passes its bytes through unchanged, and ze decodes no SRv6 endpoint function, so the GTP4.E/GTP6.E function is never tied to the BGP AFI |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2`](#draft-ietf-bess-mup-safi-3.3.5-2) When withdrawing DSD route, BGP speaker MUST attach a BGP MUP Extended community of the associated routing instance (Section 3.3.5) | {gap}, no test | a MUP withdrawal is built as a bare MP_UNREACH_NLRI with no path attributes (internal/component/bgp/reactor/peer_rib_routes.go:182-197), so a DSD withdrawal carries no BGP MUP Extended Community |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1`](#draft-ietf-bess-mup-safi-3.3.8-1) Controller MUST advertise the withdrawal of the Type 1 ST route (Section 3.3.8) | {gap}, no test | no MUP NLRI can reach the family-generic MP_UNREACH encoder (internal/component/bgp/reactor/peer_rib_routes.go:170). Its only callers take the NLRI from a PeerOpWithdraw queue entry (internal/component/bgp/reactor/peer_initial_sync.go:237, :377) filled by QueueWithdraw (internal/component/bgp/reactor/peer.go:886-893), and the two withdrawal entry points that feed it parse no SAFI 85: text mode rejects the family in isSupportedFamily, whose list stops at SAFI 73 (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:375-403), and the announce/withdraw registry builds only unicast and FlowSpec NLRIs (internal/component/bgp/plugins/cmd/announce/announce.go:257, :304, :415). NewMUP and NewMUPFull (internal/component/bgp/plugins/nlri/mup/types.go:93, :103) have no non-test caller, and nlrisplit registers no SAFI 85 splitter (internal/core/bgp/nlri/nlrisplit/register.go:9-24) so no received MUP route is stored to be withdrawn either |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-3`](#draft-ietf-bess-mup-safi-3.3.7-3) Controller MUST advertise the Type 1 ST route with Destination prefix, TEID, QFI, Endpoint Address, and optionally Source Address (Section 3.3.7) | {gap}, no test | parseT1STFields requires only the destination prefix (internal/component/bgp/plugins/nlri/mup/encode.go:313-316) and writeT1STData omits the TEID field when no TEID is configured and the Endpoint Address field when no endpoint is configured (internal/component/bgp/plugins/nlri/mup/encode.go:396-409), so a Type 1 ST route is advertised without them |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2`](#draft-ietf-bess-mup-safi-3.3.9-2) PE receiving T1ST in MP_UNREACH_NLRI without Source address MUST delete all matching T1ST routes with different Source addresses (Section 3.3.9) | {gap}, no test | nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). The opaque key is the whole NLRI, Source address included, so a Source-less Type 1 ST withdrawal matches no stored entry. No wildcard delete over differing Source addresses exists |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1`](#draft-ietf-bess-mup-safi-3.3.11-1) Controller MUST advertise the withdrawal of the Type 2 ST route (Section 3.3.11) | {gap}, no test | the same missing path as DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1 -- the family-generic MP_UNREACH encoder (internal/component/bgp/reactor/peer_rib_routes.go:170) is reachable only through PeerOpWithdraw (internal/component/bgp/reactor/peer.go:886-893, internal/component/bgp/reactor/peer_initial_sync.go:237, :377), and neither withdrawal entry point produces a SAFI 85 NLRI: isSupportedFamily omits it (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:375-403) and the announce registry builds only unicast and FlowSpec NLRIs (internal/component/bgp/plugins/cmd/announce/announce.go:257, :304, :415) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1`](#draft-ietf-bess-mup-safi-3.3.1-1)

PE advertising ISD route must attach export BGP Route Target Extended Community of the associated routing instance (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2`](#draft-ietf-bess-mup-safi-3.3.1-2)

PE advertising ISD route must use IPv6 address of PE as nexthop in MP_REACH_NLRI (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3`](#draft-ietf-bess-mup-safi-3.3.1-3)

ISD route update must have a prefix SID attribute (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1`](#draft-ietf-bess-mup-safi-3.3.2-1)

PE withdrawing ISD route must attach export BGP Route Target Extended Community (Section 3.3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1`](#draft-ietf-bess-mup-safi-3.3.4-1)

DSD address in NLRI must be a unique PE identifier (Section 3.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2`](#draft-ietf-bess-mup-safi-3.3.4-2)

PE announcing DSD route must attach a BGP MUP Extended Community (Section 3.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3`](#draft-ietf-bess-mup-safi-3.3.4-3)

PE advertising DSD route must use IPv6 address of PE as nexthop in MP_REACH_NLRI (Section 3.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4`](#draft-ietf-bess-mup-safi-3.3.4-4)

DSD route update must have a prefix SID attribute (Section 3.3.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.5-1`](#draft-ietf-bess-mup-safi-3.3.5-1)

BGP speaker announcing T1ST must attach a BGP MUP Extended Community (Section 3.3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.5-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1`](#draft-ietf-bess-mup-safi-3.3.7-1)

MUP Controller must set nexthop of T1ST route to the controller address (Section 3.3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-2`](#draft-ietf-bess-mup-safi-3.3.7-2)

Controller must announce T1ST route using AFI of the route and SAFI BGP-MUP to all BGP speakers in SRv6 domain (Section 3.3.7)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFCMUPAnnounceUsesRouteAFIWithMUPSAFI`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L207) | unit/verify | unproven |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1`](#draft-ietf-bess-mup-safi-3.3.10-1)

Controller must attach Route Target Extended Community of routing instances in the PE for T2ST (Section 3.3.10)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2`](#draft-ietf-bess-mup-safi-3.3.10-2)

Controller must set nexthop of T2ST route to MUP Controller address (Section 3.3.10)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1-1`](#draft-ietf-bess-mup-safi-3.1-1)

Unknown Route Types for supported Architecture Types must be silently ignored (Section 3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFCMUPUnknownRouteTypeIsNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L161) | unit/verify | unproven |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1`](#draft-ietf-bess-mup-safi-3.3.3-1)

Receiver must ensure ISD Address field value is address of originator of locator in prefix SID attribute (Section 3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2`](#draft-ietf-bess-mup-safi-3.3.3-2)

On MP_UNREACH_NLRI, receiver must delete withdrawn ISD route from routing instance table (Section 3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1`](#draft-ietf-bess-mup-safi-3.3.6-1)

Receiver must ensure DSD nexthop in MP_REACH_NLRI is identical to originator of locator in prefix SID attribute (Section 3.3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2`](#draft-ietf-bess-mup-safi-3.3.6-2)

On MP_UNREACH_NLRI, receiver must delete withdrawn DSD route from routing instance table (Section 3.3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1`](#draft-ietf-bess-mup-safi-3.3.9-1)

PE receiving T1ST routes in MP_UNREACH_NLRI must delete all routes from associated routing instance (Section 3.3.9)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-1`](#draft-ietf-bess-mup-safi-3.3.12-1)

PE must handle T2ST without MUP Extended Community as treat-as-withdraw (Section 3.3.12)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.12-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1`](#draft-ietf-bess-mup-safi-3.1.1-1)

ISD with prefix length exceeding max for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2`](#draft-ietf-bess-mup-safi-3.1.1-2)

Speaker must skip malformed NLRIs and continue processing rest of Update message (Section 3.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1`](#draft-ietf-bess-mup-safi-3.1.2-1)

DSD with wrong address size for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.2)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1`](#draft-ietf-bess-mup-safi-3.1.3-1)

T1ST with prefix length exceeding max for AFI: treat-as-withdraw per RFC 7606 (Section 3.1.3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1`](#draft-ietf-bess-mup-safi-3.1.3.1-1)

T1ST with TEID=0: treat-as-withdraw per RFC 7606 (Section 3.1.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2`](#draft-ietf-bess-mup-safi-3.1.3.1-2)

T1ST with invalid Endpoint Address Length (not 32 or 128): treat-as-withdraw (Section 3.1.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3`](#draft-ietf-bess-mup-safi-3.1.3.1-3)

T1ST with invalid Source Address Length (not 0, 32, or 128): treat-as-withdraw (Section 3.1.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1`](#draft-ietf-bess-mup-safi-3.1.4-1)

T2ST with Endpoint Length exceeding max for AFI: treat-as-withdraw (Section 3.1.4)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1`](#draft-ietf-bess-mup-safi-3.1.4.1-1)

T2ST with TEID=0: treat-as-withdraw per RFC 7606 (Section 3.1.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4`](#draft-ietf-bess-mup-safi-3.1.3.1-4)

T1ST NLRI architecture field MUST be encoded as specified for 3gpp-5g; otherwise treat-as-withdraw (Section 3.1.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2`](#draft-ietf-bess-mup-safi-3.1.4.1-2)

T2ST NLRI architecture field MUST be encoded as specified for 3gpp-5g; otherwise treat-as-withdraw (Section 3.1.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3`](#draft-ietf-bess-mup-safi-3.3.3-3)

ISD/DSD without prefix SID attribute: treat-as-withdraw (Section 3.3.3, 3.3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4`](#draft-ietf-bess-mup-safi-3.3.3-4)

Nexthop/locator mismatch on ISD/DSD: treat-as-withdraw (Section 3.3.3, 3.3.6)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3-1`](#draft-ietf-bess-mup-safi-3.3-1)

PE and MUP Controller MUST establish a BGP session to exchange BGP-MUP NLRIs for both IPv4 and IPv6 AFIs (Section 3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFCMUPRejectsNonMUPFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L133) | unit/verify | unproven |
| positive | [`TestRFCMUPFamiliesCoverBothAFIs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc_mup_safi_test.go#L84) | unit/verify | unproven |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4`](#draft-ietf-bess-mup-safi-3.3.1-4)

ISD prefix SID function MUST be GTP4.E if BGP AFI is IPv4, or MUST be GTP6.E if BGP AFI is IPv6 (Section 3.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2`](#draft-ietf-bess-mup-safi-3.3.5-2)

When withdrawing DSD route, BGP speaker MUST attach a BGP MUP Extended community of the associated routing instance (Section 3.3.5)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1`](#draft-ietf-bess-mup-safi-3.3.8-1)

Controller MUST advertise the withdrawal of the Type 1 ST route (Section 3.3.8)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-3`](#draft-ietf-bess-mup-safi-3.3.7-3)

Controller MUST advertise the Type 1 ST route with Destination prefix, TEID, QFI, Endpoint Address, and optionally Source Address (Section 3.3.7)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.7-3, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2`](#draft-ietf-bess-mup-safi-3.3.9-2)

PE receiving T1ST in MP_UNREACH_NLRI without Source address MUST delete all matching T1ST routes with different Source addresses (Section 3.3.9)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1`](#draft-ietf-bess-mup-safi-3.3.11-1)

Controller MUST advertise the withdrawal of the Type 2 ST route (Section 3.3.11)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for DRAFT-IETF-BESS-MUP-SAFI, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes DRAFT-IETF-BESS-MUP-SAFI, so its obligations are stated where they were written.
