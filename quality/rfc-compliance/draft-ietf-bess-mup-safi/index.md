# DRAFT-IETF-BESS-MUP-SAFI - BGP Extensions for the Mobile User Plane (MUP) SAFI

Partial. Every requirement this repository extracted from DRAFT-IETF-BESS-MUP-SAFI, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 36.2% | 17 of 47 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 47 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 47 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 47 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 100.0% | 48 of 48 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 47 | of 67 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 47 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 47 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 47 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 47 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 63.8% | 30 of 47 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 17 | of 47 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 47 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 67 |
| Gated MUST-level | 47 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 30 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 50 |
| Tagged units | 48 |
| Recorded audit verdicts | 17 |
| Discrimination records | 48 |
| Summary | `rfc/short/draft-ietf-bess-mup-safi.md` |
| Requirement shard | `rfc/requirements/draft-ietf-bess-mup-safi.md` |
| RFC text | `rfc/drafts/draft-ietf-bess-mup-safi.txt` |

## Enrolment

Enrolled: BGP Extensions for Mobile User Plane (MUP) SAFI. Eleven MUST rows added 2026-09-21 from sentences the checklist had not carried, each untested: the TLV rules of Sections 3.1.3.1 and 3.1.4.1 (3.1.3.1-6 through 3.1.3.1-10, 3.1.4.1-3), the Section 3.1.5 rule that a TLV received in a route type for which it is not applicable MUST be ignored (3.1.5-1), and the four Section 3.3.12 route resolution rules that bind a PE receiving a Type 2 ST route to DSD or ISD routes by BGP MUP Extended Community (3.3.12-4 through 3.3.12-7). The MUP codec parses no ST route TLV and resolves no Type 2 ST route.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

The BGP-MUP NLRI codec only: ipv4/mup and ipv6/mup family registration, ISD/DSD/T1ST/T2ST encoding from config and route commands, full route type body decoding, the RFC 7606 Section 5.4 ruling that discards a route whose Architecture Type is not 1 or whose Route Type is outside 1..4 at ingress ([`internal/component/bgp/plugins/nlri/mup/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc7606.go), RecognizeNLRI), MUP extended-community config syntax, and family-generic MP_REACH announcement (internal/component/bgp/plugins/nlri/mup). Thirty MUST rows stay open, each a {gap} annotated per line in [`rfc/short/draft-ietf-bess-mup-safi.md`](https://github.com/ze-software/ze/blob/main/rfc/short/draft-ietf-bess-mup-safi.md). A received Type 1 ST route whose Length ends short of a mandatory field is treated as withdrawn, including one that ends exactly at its Endpoint Address, the draft-mpmz-bess-mup-safi-02 encoding with no Source Address Length octet: Section 3.1.3.1 says such an NLRI "is malformed; MUST be treated as Treat-as-withdraw" (parseBodyT1ST, [`internal/component/bgp/plugins/nlri/mup/types.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/types.go); owner ruling 2026-10-24, which reverses the 2026-10-22 decision to read it as Source Address Length 0). ExaBGP 5.0 sends that form for a route with no source address, so such a route from it reaches Ze as a withdrawal ([`docs/exabgp/exabgp-differences.md`](https://github.com/ze-software/ze/blob/main/docs/exabgp/exabgp-differences.md)). Withdrawal: no MUP NLRI reaches the family-generic MP_UNREACH encoder ([`internal/component/bgp/reactor/peer_rib_routes.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_rib_routes.go)) -- its callers read the PeerOpWithdraw queue, and neither withdrawal entry point parses SAFI 85 ([`internal/component/bgp/plugins/cmd/update/update_text_nlri.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/update/update_text_nlri.go), [`internal/component/bgp/plugins/cmd/announce/announce.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/cmd/announce/announce.go)) -- so a Type 1 ST or Type 2 ST withdrawal cannot be emitted (3.3.8-1, 3.3.11-1). Receive side: nlrisplit registers SplitMUP for SAFI 85 since 2026-08-04 ([`internal/core/bgp/nlri/nlrisplit/register.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/register.go)), so a received MUP route is stored as an opaque Adj-RIB-In entry (insertPoolNLRIs) and a withdrawal deletes exactly the NLRI it names (removePoolNLRIs, [`internal/component/bgp/plugins/rib/rib.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib.go)). Four routing-instance obligations stay open because ze models no MUP routing instance and no route-type-aware wildcard delete exists ([`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2`](#draft-ietf-bess-mup-safi-3.3.3-2), 3.3.6-2, 3.3.9-1, 3.3.9-2). ParseMUP validates the route type body and refuses an NLRI that does not add up ([`internal/component/bgp/plugins/nlri/mup/types.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/types.go)), and the same checks run at ingress (ValidateNLRI, called by the registered recognizer, rfc7606.go recognizerFor): a malformed route is treated as withdrawn, or discarded when its route key cannot form (3.1.1-1, 3.1.1-2, 3.1.2-1, 3.1.3-1, 3.1.3.1-1, 3.1.3.1-2, 3.1.3.1-3, 3.1.3.1-6, 3.1.3.1-7, 3.1.3.1-10, 3.1.4-1, 3.1.4.1-1). A non-3gpp-5g architecture type is discarded as an unimplemented type rather than treated as withdrawn (3.1.3.1-4, 3.1.4.1-2), and no treat-as-withdraw fires on an ISD route missing its Prefix-SID, a nexthop/locator mismatch, or a Type 2 ST route without the BGP MUP Extended Community (3.3.3-1, 3.3.3-3, 3.3.3-4, 3.3.6-1, 3.3.6-4). A DSD route announced in an UPDATE with no Prefix-SID attribute, counting one that RFC 7606 attribute-discard removes, is treated as withdrawn by the UPDATE-level check the plugin registers ([`internal/component/bgp/plugins/nlri/mup/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/rfc7606.go), CheckDSDPrefixSID; 3.3.6-5). Send side: ze runs no MUP PE or MUP Controller function, so route targets, the BGP MUP Extended Community, the Prefix-SID, the GTP4.E/GTP6.E function, the required T1ST TEID and Endpoint Address, and the PE or controller IPv6 nexthop are whatever the operator configures rather than derived (3.3.1-1, 3.3.1-2, 3.3.1-3, 3.3.1-4, 3.3.2-1, 3.3.4-1, 3.3.4-2, 3.3.4-3, 3.3.4-4, 3.3.5-2, 3.3.7-1, 3.3.10-1, 3.3.10-2).

**What the ledger says remains:**

-

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 17 | one part of the gated population |
| Annotated (including scoped evidence) | 30 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **47** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (17):** [`DRAFT-IETF-BESS-MUP-SAFI-3.1-1`](#draft-ietf-bess-mup-safi-3.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1`](#draft-ietf-bess-mup-safi-3.1.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2`](#draft-ietf-bess-mup-safi-3.1.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1`](#draft-ietf-bess-mup-safi-3.1.2-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1`](#draft-ietf-bess-mup-safi-3.1.3-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1`](#draft-ietf-bess-mup-safi-3.1.3.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2`](#draft-ietf-bess-mup-safi-3.1.3.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3`](#draft-ietf-bess-mup-safi-3.1.3.1-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6`](#draft-ietf-bess-mup-safi-3.1.3.1-6), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7`](#draft-ietf-bess-mup-safi-3.1.3.1-7), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9`](#draft-ietf-bess-mup-safi-3.1.3.1-9), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10`](#draft-ietf-bess-mup-safi-3.1.3.1-10), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1`](#draft-ietf-bess-mup-safi-3.1.4-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1`](#draft-ietf-bess-mup-safi-3.1.4.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-3`](#draft-ietf-bess-mup-safi-3.1.4.1-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.5-1`](#draft-ietf-bess-mup-safi-3.1.5-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5`](#draft-ietf-bess-mup-safi-3.3.6-5)

**Annotated (including scoped evidence) (30):** [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-1`](#draft-ietf-bess-mup-safi-3.3.1-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-2`](#draft-ietf-bess-mup-safi-3.3.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-3`](#draft-ietf-bess-mup-safi-3.3.1-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.2-1`](#draft-ietf-bess-mup-safi-3.3.2-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-1`](#draft-ietf-bess-mup-safi-3.3.4-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-2`](#draft-ietf-bess-mup-safi-3.3.4-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-3`](#draft-ietf-bess-mup-safi-3.3.4-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.4-4`](#draft-ietf-bess-mup-safi-3.3.4-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.7-1`](#draft-ietf-bess-mup-safi-3.3.7-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1`](#draft-ietf-bess-mup-safi-3.3.10-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2`](#draft-ietf-bess-mup-safi-3.3.10-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1`](#draft-ietf-bess-mup-safi-3.3.3-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2`](#draft-ietf-bess-mup-safi-3.3.3-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1`](#draft-ietf-bess-mup-safi-3.3.6-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2`](#draft-ietf-bess-mup-safi-3.3.6-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1`](#draft-ietf-bess-mup-safi-3.3.9-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4`](#draft-ietf-bess-mup-safi-3.1.3.1-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2`](#draft-ietf-bess-mup-safi-3.1.4.1-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3`](#draft-ietf-bess-mup-safi-3.3.3-3), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4`](#draft-ietf-bess-mup-safi-3.3.3-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4`](#draft-ietf-bess-mup-safi-3.3.6-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.1-4`](#draft-ietf-bess-mup-safi-3.3.1-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.5-2`](#draft-ietf-bess-mup-safi-3.3.5-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1`](#draft-ietf-bess-mup-safi-3.3.8-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.9-2`](#draft-ietf-bess-mup-safi-3.3.9-2), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.11-1`](#draft-ietf-bess-mup-safi-3.3.11-1), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-4`](#draft-ietf-bess-mup-safi-3.3.12-4), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-5`](#draft-ietf-bess-mup-safi-3.3.12-5), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-6`](#draft-ietf-bess-mup-safi-3.3.12-6), [`DRAFT-IETF-BESS-MUP-SAFI-3.3.12-7`](#draft-ietf-bess-mup-safi-3.3.12-7)

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
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.10-1` | The controller MUST attach a Route Target Extended Community of the routing instances in the PE accommodating the corresponding Interwork Segment. (§3.3.10) | MUST | 3.3.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseConfigRoute attaches only the extended communities the operator configures (internal/component/bgp/plugins/nlri/mup/config.go:66) and ze models no routing instance with export route targets, so a Type 2 ST advertisement carries a route target only when the operator writes one into the route |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.10-2` | The controller MUST set the nexthop of the route to the address of the MUP Controller. (§3.3.10) | MUST | 3.3.10 | **positive:** no positive test. **negative:** no negative test. **{gap}:** EncodeRoute takes the next-hop verbatim from the route command (internal/component/bgp/plugins/nlri/mup/encode.go:173-191) for a Type 2 ST route and ze runs no MUP Controller function that substitutes the controller address |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1-1` | Any other Route Types MUST be silently ignored upon a receipt if a BGP speaker supports only 3gpp-5G architecture type. (Section 3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestMUPOtherRouteTypesSilentlyIgnoredAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L64). **positive:** `unit/verify` [`TestRFCMUPUnknownRouteTypeIsNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_safi_test.go#L159). **negative:** `unit/verify` [`TestMUPDefinedRouteTypesSurviveIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L87) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-1` | However, the receiving BGP speaker MUST ensure that the value of Address filed in the NLRI is an address of the originator of the locator value in the prefix SID attribute. (Section 3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the ISD Address field is never checked against the originator of the prefix SID locator |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-2` | When a BGP speaker receives an MP_UNREACH_NLRI attribute update message it MUST delete the withdrawn Interwork Segment Discovery route from the routing instance table where it was created. (Section 3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives an ISD withdrawal through it |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-1` | However, the receiving BGP speaker MUST ensure that the received nexthop value in the MP_REACH_NLRI attribute is identical to the originator of the locator value in the prefix SID attribute. (Section 3.3.6) | MUST | 3.3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so the DSD nexthop is never compared with the originator of the prefix SID locator |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-2` | When a BGP speaker receives an MP_UNREACH_NLRI attribute update message it MUST delete the withdrawn Direct Segment Discovery route from the routing instance table where it was created. (Section 3.3.6) | MUST | 3.3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). What remains is that ze models no MUP routing instance: the entry lives in the peer's Adj-RIB-In alone, and no RFC requirement tagged test drives a DSD withdrawal through it |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.9-1` | The PE receiving Type 1 ST routes in MP_UNREACH_NLRI attribute MUST delete all the routes from the associated routing instance. (Section 3.3.9) | MUST | 3.3.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** nlrisplit now registers SplitMUP for SAFI 85 (internal/core/bgp/nlri/nlrisplit/register.go), so insertPoolNLRIs stores a received MUP NLRI as an opaque entry keyed on the whole NLRI and removePoolNLRIs deletes exactly the NLRI a withdrawal names (internal/component/bgp/plugins/rib/rib.go). The delete is one NLRI for one NLRI. Nothing reads the Type 1 ST route type to delete every route of the associated routing instance, and ze models no such instance |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.1-1` | If the AFI is IPv4, then the maximum value of the Prefix Length is 32 bits otherwise it is considered as a malformed NLRI. If the AFI is IPv6, then the maximum value of of the Prefix length is 128 bits otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.1) | MUST | 3.1.1 | **positive:** `unit/verify` [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L110). **negative:** `unit/verify` [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L199) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2` | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. (Section 3.1.1) | MUST | 3.1.1 | **positive:** `unit/verify` [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L111). **negative:** `unit/verify` [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L200) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1` | If the AFI is IPv4 then the address length is 4 octets otherwise it is considered as a malformed NLRI. If the AFI is IPv6 then the address length is 16 octets otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat- as-withdraw" [RFC7606]. (§3.1.2) | MUST | 3.1.2 | **positive:** `unit/verify` [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L112). **negative:** `unit/verify` [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L201) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1` | If the AFI is IPv4, then the maximum value of the Prefix Length field is 32. If the AFI is IPv6, then the maximum value of the Prefix Length field is 128. Any other length field is considered a a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat- as-withdraw" [RFC7606]. (§3.1.3) | MUST | 3.1.3 | **positive:** `unit/verify` [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L113). **negative:** `unit/verify` [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L202) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1` | The TEID value of 0 is considered as an invalid and a malformed TEID. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L114). **negative:** `unit/verify` [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L203) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2` | Endpoint Address field contains of an IPv4 address, then the value of the Endpoint Address Length field is 32. If the Endpoint Address field contains of an IPv6 Address, then the value of the Endpoint Address Length field is 128. Any other value is considered as an invalid and a malformed Endpoint Address. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L115). **positive:** `unit/verify` [`TestMUPT1STEndpointLengthIsTheOnlyFault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_t1st_address_length_test.go#L63). **negative:** `unit/verify` [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L204) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3` | Source Address Length is 0 bytes then the Source address is not carried within the NLRI. A BGP speaker MAY have a local configuration for using a Source address. The exact mechanism of local configuration is outside the scope of this document. Source Address field contains of an IPv4 address, then the value of the Source Address Length field is 32. If the Source Address field contains of an IPv6 Address, then the value of the Source Address Length field is 128. Any other value is considered as an invalid and a malformed Endpoint Address. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L116). **positive:** `unit/verify` [`TestMUPT1STSourceLengthIsTheOnlyFault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_t1st_address_length_test.go#L89). **negative:** `unit/verify` [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L205) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6` | - < 0: the mandatory fields exceed the declared Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw. (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPMalformedT1STTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_withdraw_test.go#L111). **positive:** `unit/verify` [`TestMUPT1ST02TreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_02_relay_test.go#L29). **positive:** `unit/verify` [`TestMUPT1STMandatoryFieldsWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L38). **negative:** `unit/verify` [`TestMUPMalformedT1STTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_withdraw_test.go#L137). **negative:** `unit/verify` [`TestMUPT1STFixedOctetsWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_t1st_truncation_test.go#L53). **negative:** `unit/verify` [`TestMUPT1STMandatoryFieldsWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L46). **negative:** `unit/verify` [`TestMUPT1STSourceAddressWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_t1st_truncation_test.go#L37) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7` | - 1: encoding is invalid (a valid TLV requires at minimum a Type byte and a Length byte); MUST be treated as Treat-as- withdraw. (Section 3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPMalformedT1STTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_withdraw_test.go#L113). **positive:** `unit/verify` [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L69). **negative:** `unit/verify` [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L77) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9` | unknown TLV types MUST be ignored for local processing and MUST be propagated unchanged when re- advertising the route to other BGP peers (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L103). **positive:** `unit/verify` [`TestMUPUnknownTLVPropagatedUnchangedAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L116). **negative:** `unit/verify` [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L123). **negative:** `unit/verify` [`TestMUPUnknownTLVNotStrippedAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L148) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10` | any TLV parsing error MUST result in Treat-as-withdraw (§3.1.3.1) | MUST | 3.1.3.1 | **positive:** `unit/verify` [`TestMUPMalformedT1STTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_withdraw_test.go#L115). **positive:** `unit/verify` [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L71). **negative:** `unit/verify` [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L83) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1` | If the AFI is IPv4, then the maximum Endpoint length is 64 otherwise it is considered as a malformed NLRI. If the AFI is IPv6, then the maximum Endpoint length is 160 otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.4) | MUST | 3.1.4 | **positive:** `unit/verify` [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L117). **negative:** `unit/verify` [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L206) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1` | The TEID value of 0 is considered as an invalid and a malformed TEID. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.4.1) | MUST | 3.1.4.1 | **positive:** `unit/verify` [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L118). **negative:** `unit/verify` [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L207) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-3` | The Endpoint Length MUST NOT extend beyond the TEID field. (§3.1.4.1) | MUST NOT | 3.1.4.1 | **positive:** `unit/verify` [`TestMUPT2STEncoderKeepsEndpointLengthWithinTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L179). **positive:** `unit/verify` [`TestMUPT2STEndpointLengthBoundedByTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L135). **negative:** `unit/verify` [`TestMUPT2STEncoderRefusesEndpointLengthBeyondTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L205). **negative:** `unit/verify` [`TestMUPT2STEndpointLengthBoundedByTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L145) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.5-1` | A TLV received in a route type for which it is not applicable MUST be ignored (Section 3.1.5) | MUST | 3.1.5 | **positive:** `unit/verify` [`TestMUPST1IgnoresST2OnlyTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_applicability_test.go#L10). **negative:** `unit/verify` [`TestMUPST1DoesNotInterpretInapplicableAddressTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_applicability_test.go#L24) |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4` | The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 1 ST route. Otherwise the NLRI is considered as a malformed. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (Section 3.1.3.1) | MUST | 3.1.3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go), but the ingress recognizer answers a received NLRI whose architecture is not 3gpp-5g as an unimplemented type and discards it (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a T1ST NLRI encoded for another architecture is not treated as withdraw and an earlier route of its key is not removed |
| `DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2` | The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 2 ST route. Otherwise the NLRI is considered as a malformed. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (Section 3.1.4.1) | MUST | 3.1.4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go), but the ingress recognizer answers a received NLRI whose architecture is not 3gpp-5g as an unimplemented type and discards it (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a T2ST NLRI encoded for another architecture is not treated as withdraw and an earlier route of its key is not removed |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3` | When a BGP speaker receives the Interwork Segment Dicovery routes with a MP_REACH_NLRI attribute without a prefix SID attribute, then it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied. (Section 3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) without consulting the UPDATE path attributes, so an ISD route arriving without a Prefix-SID attribute is not treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4` | If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied. (§3.3.3) | MUST | 3.3.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so a mismatch between the nexthop and the locator originator is never detected on ISD or DSD routes |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4` | If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied. (§3.3.6) | MUST | 3.3.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the Direct Segment Discovery form of the nexthop and locator check. The ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI) and ze reads no prefix SID locator anywhere in internal/component/bgp, so a DSD route whose MP_REACH_NLRI nexthop differs from the locator originator is stored rather than treated as withdrawn |
| `DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5` | When a BGP speaker receives a MP_REACH_NLRI attribute update message with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied. (§3.3.6) | MUST | 3.3.6 | **positive:** `unit/verify` [`TestMUPDSDWithoutPrefixSIDWithdrawnOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_dsd_prefix_sid_test.go#L76). **negative:** `unit/verify` [`TestMUPDSDWithPrefixSIDKeptOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_dsd_prefix_sid_test.go#L108) |
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
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4`](#draft-ietf-bess-mup-safi-3.1.3.1-4) The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 1 ST route. Otherwise the NLRI is considered as a malformed. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (Section 3.1.3.1) | {gap}, no test | writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go), but the ingress recognizer answers a received NLRI whose architecture is not 3gpp-5g as an unimplemented type and discards it (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a T1ST NLRI encoded for another architecture is not treated as withdraw and an earlier route of its key is not removed |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2`](#draft-ietf-bess-mup-safi-3.1.4.1-2) The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 2 ST route. Otherwise the NLRI is considered as a malformed. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (Section 3.1.4.1) | {gap}, no test | writeMUPNLRI always writes architecture type 1 (internal/component/bgp/plugins/nlri/mup/encode.go), but the ingress recognizer answers a received NLRI whose architecture is not 3gpp-5g as an unimplemented type and discards it (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI), so a T2ST NLRI encoded for another architecture is not treated as withdraw and an earlier route of its key is not removed |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3`](#draft-ietf-bess-mup-safi-3.3.3-3) When a BGP speaker receives the Interwork Segment Dicovery routes with a MP_REACH_NLRI attribute without a prefix SID attribute, then it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied. (Section 3.3.3) | {gap}, no test | DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) without consulting the UPDATE path attributes, so an ISD route arriving without a Prefix-SID attribute is not treated as withdrawn |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4`](#draft-ietf-bess-mup-safi-3.3.3-4) If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied. (§3.3.3) | {gap}, no test | DecodeNLRIHex surfaces only route type, architecture type and RD from a received MUP NLRI (internal/component/bgp/plugins/nlri/mup/mup.go:56-73) and ze reads no prefix SID locator anywhere in internal/component/bgp, so a mismatch between the nexthop and the locator originator is never detected on ISD or DSD routes |
| [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4`](#draft-ietf-bess-mup-safi-3.3.6-4) If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied. (§3.3.6) | {gap}, no test | the Direct Segment Discovery form of the nexthop and locator check. The ingress recognizer reads the architecture and route type alone (internal/component/bgp/plugins/nlri/mup/rfc7606.go, RecognizeNLRI) and ze reads no prefix SID locator anywhere in internal/component/bgp, so a DSD route whose MP_REACH_NLRI nexthop differs from the locator originator is stored rather than treated as withdrawn |
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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 32ff59085b). Section 3.1: "Any other Route Types MUST be silently ignored upon a receipt if a BGP speaker supports only 3gpp-5G architecture type." The earlier weak finding (retainMUP summed Discarded and WithdrawnCount, so a recognizer answering Withdraw for unknown route types still passed) is answered: TestMUPOtherRouteTypesSilentlyIgnoredAtIngress now reads retainMUPCounts and asserts discarded == 4 and withdrawn == 0 separately, with the ISD between them kept byte for byte, under both MUP families. keyMUP keys an unknown route type on its whole NLRI, so a Withdraw answer would land in WithdrawnCount and go red; a Keep answer fails the kept equality. The negative TestMUPDefinedRouteTypesSurviveIngress (types 1 to 4, kept whole, nothing dropped) changed only by the T1ST body gaining its mandatory zero Source Address Length octet. TestRFCMUPUnknownRouteTypeIsNotRejected adds that ParseMUP skips an unknown route by its Length. RecognizeNLRI revert records re-observed red today on both polarities. The optional "MAY log" is not tested and not owed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPDefinedRouteTypesSurviveIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestMUPOtherRouteTypesSilentlyIgnoredAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L64) | unit/verify | revert, verified |
| positive | [`TestRFCMUPUnknownRouteTypeIsNotRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_safi_test.go#L159) | unit/verify | revert, verified |

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

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 32ff59085b). Section 3.1.1 ISD Prefix Length bound (32 under IPv4, 128 under IPv6) with treat-as-withdraw. The changed unit is the negative TestMUPWellFormedRouteKeptAtIngress: its two T1ST cases gained the zero Source Address Length octet and are now complete routes under the corrected parser; every other case, and every assertion (section kept whole, Withdrawn and Unkeyable empty, Discarded zero), is unchanged. The positive ISD cases (IPv4 33 withdrawn exactly; IPv6 129 discarded as unkeyable) and the negative ISD 32/128 cases are byte-identical. Both revert records were re-taken today with ./le rfc discriminate-record over HEAD and observed red naming the unit. 

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L199) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L110) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.1-2`](#draft-ietf-bess-mup-safi-3.1.1-2)

A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. (Section 3.1.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 32ff59085b). Section 3.1.1: "A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message." Positive keeps exactly L then L2 around every malformed shape. The changed unit is the negative TestMUPWellFormedRouteKeptAtIngress: its two T1ST cases gained the zero Source Address Length octet and are now complete routes under the corrected parser; every other case, and every assertion (section kept whole, Withdrawn and Unkeyable empty, Discarded zero), is unchanged. 32ff59085b made the T1ST Source Address Length octet mandatory, matching Section 3.1.3.1 ("The Source Address Length has a fixed length of 1 octet." and the TLV remainder counted from "the offset of the end of the Source Address"); parseBodyT1ST now returns ErrMUPTruncated when nothing follows the endpoint. The T1ST malformed cases now use endpoint4NoSource, so the corrected parser does not confound them. Both revert records were re-taken today with ./le rfc discriminate-record over HEAD and observed red naming the unit. 

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L200) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L111) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.2-1`](#draft-ietf-bess-mup-safi-3.1.2-1)

If the AFI is IPv4 then the address length is 4 octets otherwise it is considered as a malformed NLRI. If the AFI is IPv6 then the address length is 16 octets otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat- as-withdraw" [RFC7606]. (§3.1.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 32ff59085b). Section 3.1.2 DSD address length (4 under IPv4, 16 under IPv6) with treat-as-withdraw. The changed unit is the negative TestMUPWellFormedRouteKeptAtIngress: its two T1ST cases gained the zero Source Address Length octet and are now complete routes under the corrected parser; every other case, and every assertion (section kept whole, Withdrawn and Unkeyable empty, Discarded zero), is unchanged. The DSD positive (16-octet under IPv4, 4-octet under IPv6, withdrawn exactly) and negative (4 and 16 kept) cases are byte-identical. Both revert records were re-taken today with ./le rfc discriminate-record over HEAD and observed red naming the unit. 

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L201) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L112) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3-1`](#draft-ietf-bess-mup-safi-3.1.3-1)

If the AFI is IPv4, then the maximum value of the Prefix Length field is 32. If the AFI is IPv6, then the maximum value of the Prefix Length field is 128. Any other length field is considered a a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat- as-withdraw" [RFC7606]. (§3.1.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 32ff59085b). Section 3.1.3: "If the AFI is IPv4, then the maximum value of the Prefix Length field is 32. If the AFI is IPv6, then the maximum value of the Prefix Length field is 128." 32ff59085b made the T1ST Source Address Length octet mandatory, matching Section 3.1.3.1 ("The Source Address Length has a fixed length of 1 octet." and the TLV remainder counted from "the offset of the end of the Source Address"); parseBodyT1ST now returns ErrMUPTruncated when nothing follows the endpoint. Positive T1ST IPv4 prefix length 33 now closes with endpoint4NoSource, so the prefix bound is the only fault and the route is withdrawn exactly; negative prefix 32 kept, now with its mandatory zero source length. The IPv6 128 clause has no T1ST case in the tagged units: parseBodyT1ST calls the shared AFI-indexed m.parsePrefixField, whose IPv6 bound the ISD 128/129 cases pin; a T1ST-only regression of that clause would stay green here, as the 2026-10-14 judgement already noted. Both revert records were re-taken today with ./le rfc discriminate-record over HEAD and observed red naming the unit. 

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L202) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L113) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-1`](#draft-ietf-bess-mup-safi-3.1.3.1-1)

The TEID value of 0 is considered as an invalid and a malformed TEID. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 32ff59085b). Section 3.1.3.1: "The TEID value of 0 is considered as an invalid and a malformed TEID.  A BGP speaker MUST handle such a malformed NLRI as a \"Treat-as-withdraw\" [RFC7606]." 32ff59085b made the T1ST Source Address Length octet mandatory, matching Section 3.1.3.1 ("The Source Address Length has a fixed length of 1 octet." and the TLV remainder counted from "the offset of the end of the Source Address"); parseBodyT1ST now returns ErrMUPTruncated when nothing follows the endpoint. The positive T1ST TEID 0 case now ends in endpoint4NoSource, so with the TEID check removed the route would be complete and kept: TEID 0 stays the only fault (withdrawn exactly, siblings kept). Negative TEID 1 kept. Both revert records were re-taken today with ./le rfc discriminate-record over HEAD and observed red naming the unit. 

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L203) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L114) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-2`](#draft-ietf-bess-mup-safi-3.1.3.1-2)

Endpoint Address field contains of an IPv4 address, then the value of the Endpoint Address Length field is 32. If the Endpoint Address field contains of an IPv6 Address, then the value of the Endpoint Address Length field is 128. Any other value is considered as an invalid and a malformed Endpoint Address. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 32ff59085b). Section 3.1.3.1 Endpoint Address Length 32 or 128, "Any other value is considered as an invalid and a malformed Endpoint Address." The changed unit is the negative TestMUPWellFormedRouteKeptAtIngress: its two T1ST cases gained the zero Source Address Length octet and are now complete routes under the corrected parser; every other case, and every assertion (section kept whole, Withdrawn and Unkeyable empty, Discarded zero), is unchanged. The ingress positive case (length 24) is unchanged and remains confounded (with 3 endpoint octets the trailing 0x01 reads as an invalid Source Address Length), which is why the isolating unit TestMUPT1STEndpointLengthIsTheOnlyFault (unchanged, its addressBitsOctets record verifies) carries this proof. Negative 32 and 128 kept, now complete routes. Re-taken parseBodyT1ST records observed red. 

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L204) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L115) | unit/verify | revert, verified |
| positive | [`TestMUPT1STEndpointLengthIsTheOnlyFault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_t1st_address_length_test.go#L63) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-3`](#draft-ietf-bess-mup-safi-3.1.3.1-3)

Source Address Length is 0 bytes then the Source address is not carried within the NLRI. A BGP speaker MAY have a local configuration for using a Source address. The exact mechanism of local configuration is outside the scope of this document. Source Address field contains of an IPv4 address, then the value of the Source Address Length field is 32. If the Source Address field contains of an IPv6 Address, then the value of the Source Address Length field is 128. Any other value is considered as an invalid and a malformed Endpoint Address. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 32ff59085b). Section 3.1.3.1 Source Address Length 0, 32 or 128, any other value treat-as-withdraw. 32ff59085b made the T1ST Source Address Length octet mandatory, matching Section 3.1.3.1 ("The Source Address Length has a fixed length of 1 octet." and the TLV remainder counted from "the offset of the end of the Source Address"); parseBodyT1ST now returns ErrMUPTruncated when nothing follows the endpoint. This is the field the fix concerns. Negative now pins length 0 (twice: the TEID 1 case and the Source Address Length 0 case are now the same octets), 32 with IPv4 and 128 with IPv6, all kept. Positive length 16 at ingress, and the isolating TestMUPT1STSourceLengthIsTheOnlyFault, withdraw. The "MAY have a local configuration" sentence binds nothing. parseBodyT1ST records (producer-changed) re-taken and observed red. 

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L205) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestMUPT1STSourceLengthIsTheOnlyFault`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_t1st_address_length_test.go#L89) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-6`](#draft-ietf-bess-mup-safi-3.1.3.1-6)

- < 0: the mandatory fields exceed the declared Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw. (§3.1.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent rejudgment at HEAD ff3feee994 after 3249a93545 and 38fb0c08fe, replacing the weak verdict of 8be6ddfbed. draft-ietf-bess-mup-safi Section 3.1.3.1 (rfc/drafts/draft-ietf-bess-mup-safi.txt): "The Source Address Length has a fixed length of 1 octet.", "compute the number of remaining octets by subtracting, from the Length field in the MUP NLRI common header, the offset of the end of the Source Address measured from the start of the Route Type specific field", and "- < 0:   the mandatory fields exceed the declared Length; the NLRI is malformed; MUST be treated as Treat-as-withdraw." Producer at HEAD, types.go::parseBodyT1ST: ErrMUPTruncated when fewer than the 6 fixed octets (TEID, QFI, Endpoint Address Length) remain, when the Endpoint Address is short, when nothing remains for the Source Address Length octet (the draft-mpmz-bess-mup-safi-02 form), and when a non-zero Source Address is short; ValidateNLRI runs the same decode at ingress. (1) Asserts the draft, not the code: the parser units assert ErrorIs ErrMUPTruncated, the session units assert RFC 7606 treat-as-withdraw itself (ActionNone, the route leaves MP_REACH_NLRI, MP_UNREACH_NLRI is exactly the held generation K0 and decodable, siblings byte for byte). (2) Fails on non-compliance, at every bound inside parseBodyT1ST: TestMUPT1STFixedOctetsWithinLength (new) cuts the Length inside the TEID, before the QFI and before the Endpoint Address Length; TestMUPT1STSourceAddressWithinLength (new) gives Source Address Length 32 and two source octets. Each is followed by a whole second NLRI, so the body slice has capacity past its Length and a parser that skipped the bound reads the next route's octets: I traced that deleting len(rest) < fixedLen panics on rest[4] or rest[5] and deleting len(rest) < sourceOctets panics on rest[sourceOctets:], both red; the recorded mutants (line 314 and line 359 to false) confirm it. The endpoint cut and the -02 cut in TestMUPT1STMandatoryFieldsWithinLength go red under a lenient len(rest)==0 return or a deleted endpoint bound (read; their records are revert-to-panic only). TestMUPT1ST02TreatAsWithdraw (now tagged) and TestMUPMalformedT1STTreatAsWithdraw carry the -02 and endpoint cuts through the receive path. (3) Isolated: TEID 0x3039 non-zero, Endpoint and Source Address Lengths 32, following NLRI well formed, so only the truncation can fire. (4) Whole requirement: every cut that leaves the route key (RD, Prefix Length, Prefix) intact is tagged and discriminated, with both polarities: a route ending exactly at its Length parses with nothing left and exact TEID and endpoint, and a well-formed UPDATE comes back byte-identical. Reading taken, stated openly: a Length ending inside the RD or the prefix is also below zero by the letter, but Section 3.1.3 makes RD, Prefix Length and Prefix the route key, so such a route has no key to withdraw and Ze discards it (TestMUPUnkeyableMalformedT1STDiscardedNotRelayed, "RD overruns the Length"; TestMUPParseErrors for the shared RD and parsePrefixField bounds). Those units are untagged for this row and earn it no credit; the unkeyable class is judged as the discard ruling, not here. Polarity labels are inverted between packages (mup: positive = well formed parses; reactor: positive = malformed withdrawn) but each file holds a genuine conforming and violating pair. Untagged corroboration: tlv_applicability_test parses a T1ST with a 32-bit source address and TLVs. Implemented and tested for all keyable truncations.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPT1STFixedOctetsWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_t1st_truncation_test.go#L53) | unit/verify | mutant, verified |
| negative | [`TestMUPT1STSourceAddressWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_t1st_truncation_test.go#L37) | unit/verify | mutant, verified |
| negative | [`TestMUPT1STMandatoryFieldsWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L46) | unit/verify | revert, verified |
| negative | [`TestMUPMalformedT1STTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_withdraw_test.go#L137) | unit/verify | revert, verified |
| positive | [`TestMUPT1STMandatoryFieldsWithinLength`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L38) | unit/verify | revert, verified |
| positive | [`TestMUPT1ST02TreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_02_relay_test.go#L29) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedT1STTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_withdraw_test.go#L111) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-7`](#draft-ietf-bess-mup-safi-3.1.3.1-7)

- 1: encoding is invalid (a valid TLV requires at minimum a Type byte and a Length byte); MUST be treated as Treat-as- withdraw. (Section 3.1.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment. draft-ietf-bess-mup-safi-01 Section 3.1.3.1 (rfc/drafts/draft-ietf-bess-mup-safi.txt:439-441): "- 1: encoding is invalid (a valid TLV requires at minimum a Type byte and a Length byte); MUST be treated as Treat-as-withdraw." Read both tagged units in full: TestMUPT1STTLVFraming in internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go accepts a complete TLV and requires ErrMUPTLV for exactly one byte after the zero Source Address Length; TestMUPMalformedT1STTreatAsWithdraw in internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_withdraw_test.go first receives valid K0, then the same route key with the lone TLV byte 0xf0. It requires exact ActionNone/no reset, exact surviving sibling bytes, and MP_UNREACH containing exactly K0, checked decodable, not merely disappearance of K. Producers read: mup/types.go ParseMUP, ValidateNLRI, decode, parseBodyT1ST and validateTLVs; mup/rfc7606.go recognizerFor; reactor/session_validation.go applyRFC7606; session_validation_nlritype.go applyTypedNLRIDiscard, typedNLRIEdit, typedNLRIRelayEdit and typedNLRIPlaceWithdrawals; session_validation_nlritype_held.go typedNLRIWalk.withdrawal. The complete mandatory body isolates the lone-byte framing fault; the parser rejects it, recognizer returns Withdraw and the held generation supplies the withdrawal. Section 3.1.3 makes RD/Prefix Length/Prefix the key, so K0 names the route being removed. Scope has no partial or exclusion annotation. No checks or native discrimination run by this auditor.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L77) | unit/verify | revert, verified |
| positive | [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L69) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedT1STTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_withdraw_test.go#L113) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-9`](#draft-ietf-bess-mup-safi-3.1.3.1-9)

unknown TLV types MUST be ignored for local processing and MUST be propagated unchanged when re- advertising the route to other BGP peers (§3.1.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 32ff59085b). Section 3.1.3.1: "unknown TLV types MUST be ignored for local processing and MUST be propagated unchanged when re- advertising the route to other BGP peers". Changed unit TestMUPUnknownTLVNotStrippedAtIngress builds the same octets as before (t1stBody now includes the zero source length the old code appended): the relayed route is five octets longer than the TLV-less one, its Length counts the TLV, its last five octets are TLV 0xF0. TestMUPUnknownTLVPropagatedUnchangedAtIngress and TestMUPT1STUnknownTLVIgnoredAndPropagated are likewise byte-identical in their wire. Retain, validateTLVs and ParseMUP records re-taken and observed red. 

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPUnknownTLVNotStrippedAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L148) | unit/verify | revert, verified |
| negative | [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L123) | unit/verify | revert, verified |
| positive | [`TestMUPUnknownTLVPropagatedUnchangedAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L116) | unit/verify | revert, verified |
| positive | [`TestMUPT1STUnknownTLVIgnoredAndPropagated`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L103) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-10`](#draft-ietf-bess-mup-safi-3.1.3.1-10)

any TLV parsing error MUST result in Treat-as-withdraw (§3.1.3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment. draft-ietf-bess-mup-safi-01 Section 3.1.3.1 (rfc/drafts/draft-ietf-bess-mup-safi.txt:443-457): "Parse TLVs one by one per Section 3.1.5; unknown TLV types MUST be ignored for local processing and MUST be propagated unchanged when re-advertising the route to other BGP peers; any TLV parsing error MUST result in Treat-as-withdraw; TLVs are not part of the NLRI key for route processing." Read Section 3.1.5 and all three defined TLVs: each applies to ST2, so T1ST validates their framing and ignores their values. Read both tagged units: TestMUPT1STTLVFraming in internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go accepts a complete TLV, rejects a lone header byte, and requires ErrMUPTLV when Length 5 exceeds the available two value bytes. TestMUPMalformedT1STTreatAsWithdraw in internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_withdraw_test.go isolates Length 5 with one value byte after complete mandatory fields, first seeds valid K0 of the same RD/prefix, and requires exact surviving sibling bytes plus exactly K0 in a decodable MP_UNREACH, with ActionNone/no reset and no extra withdrawal body. This catches retaining, merely dropping, or withdrawing the malformed bytes instead of the held generation. Producers read: mup/types.go ParseMUP, ValidateNLRI, decode, parseBodyT1ST and validateTLVs; mup/rfc7606.go recognizerFor; reactor/session_validation.go applyRFC7606; session_validation_nlritype.go applyTypedNLRIDiscard, typedNLRIEdit, typedNLRIRelayEdit and typedNLRIPlaceWithdrawals; session_validation_nlritype_held.go typedNLRIWalk.withdrawal. validateTLVs checks both header truncation and value overrun on each iteration; recognizer and session convert its error to the tested withdrawal. Scope has no partial or exclusion annotation. No checks or native discrimination run by this auditor.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L83) | unit/verify | revert, verified |
| positive | [`TestMUPT1STTLVFraming`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L71) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedT1STTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_t1st_withdraw_test.go#L115) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4-1`](#draft-ietf-bess-mup-safi-3.1.4-1)

If the AFI is IPv4, then the maximum Endpoint length is 64 otherwise it is considered as a malformed NLRI. If the AFI is IPv6, then the maximum Endpoint length is 160 otherwise it is considered as a malformed NLRI. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 32ff59085b). Section 3.1.4 Endpoint length bound (64 under IPv4, 160 under IPv6) with treat-as-withdraw. The changed unit is the negative TestMUPWellFormedRouteKeptAtIngress: its two T1ST cases gained the zero Source Address Length octet and are now complete routes under the corrected parser; every other case, and every assertion (section kept whole, Withdrawn and Unkeyable empty, Discarded zero), is unchanged. The T2ST positive (65 withdrawn, 161 discarded as unkeyable) and negative (64 and 160 kept) cases are byte-identical. Both revert records were re-taken today with ./le rfc discriminate-record over HEAD and observed red naming the unit. 

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L206) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L117) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-1`](#draft-ietf-bess-mup-safi-3.1.4.1-1)

The TEID value of 0 is considered as an invalid and a malformed TEID. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (§3.1.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21 (independent auditor, authored none of 32ff59085b). Section 3.1.4.1: "The TEID value of 0 is considered as an invalid and a malformed TEID." with treat-as-withdraw. The changed unit is the negative TestMUPWellFormedRouteKeptAtIngress: its two T1ST cases gained the zero Source Address Length octet and are now complete routes under the corrected parser; every other case, and every assertion (section kept whole, Withdrawn and Unkeyable empty, Discarded zero), is unchanged. The T2ST TEID 0 positive (withdrawn exactly) and the TEID 1 and no-TEID negatives are byte-identical. Both revert records were re-taken today with ./le rfc discriminate-record over HEAD and observed red naming the unit. 

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPWellFormedRouteKeptAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L207) | unit/verify | revert, verified |
| positive | [`TestMUPMalformedRouteTreatedAsWithdrawnAtIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_malformed_ingress_test.go#L118) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-3`](#draft-ietf-bess-mup-safi-3.1.4.1-3)

The Endpoint Length MUST NOT extend beyond the TEID field. (§3.1.4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Encoder (the sender side the MUST NOT binds): TestMUPT2STEncoderKeepsEndpointLengthWithinTEID asserts EncodeNLRIHex writes Endpoint Length address bits + 0/16/32 (32 default) for IPv4 and IPv6; TestMUPT2STEncoderRefusesEndpointLengthBeyondTEID asserts teid bits 33 and 40 (v4, v6), -8 and unreadable are refused with errTeidBits. Author observed all five negatives red on HEAD's parseTEIDWithBits (D-8 defect fixed in encode.go, re-checked at the producer: both callers propagate the error). Decode path: TestMUPT2STEndpointLengthBoundedByTEID, 64 parses, 72 refused ErrMUPEndpointTooLong. Not covered here: ingress (RecognizeNLRI) does not refuse an over-long Endpoint Length from a peer, so such a route is stored and relayed; that receive handling is the Section 3.1.4 malformed rule (3.1.4-1, a disclosed {gap}), not this sentence

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPT2STEncoderRefusesEndpointLengthBeyondTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L205) | unit/verify | revert, verified |
| negative | [`TestMUPT2STEndpointLengthBoundedByTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L145) | unit/verify | revert, verified |
| positive | [`TestMUPT2STEncoderKeepsEndpointLengthWithinTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_ingress_test.go#L179) | unit/verify | revert, verified |
| positive | [`TestMUPT2STEndpointLengthBoundedByTEID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_mup_test.go#L135) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.5-1`](#draft-ietf-bess-mup-safi-3.1.5-1)

A TLV received in a route type for which it is not applicable MUST be ignored (Section 3.1.5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPST1DoesNotInterpretInapplicableAddressTLV`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_applicability_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestMUPST1IgnoresST2OnlyTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/mup/draft_ietf_bess_mup_safi_tlv_applicability_test.go#L10) | unit/verify | revert, verified |

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4`](#draft-ietf-bess-mup-safi-3.1.3.1-4)

The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 1 ST route. Otherwise the NLRI is considered as a malformed. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (Section 3.1.3.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.3.1-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2`](#draft-ietf-bess-mup-safi-3.1.4.1-2)

The NLRI architecture field MUST be encoded as shown above if a BGP speaker receives 3gpp-5g specific BGP Type 2 ST route. Otherwise the NLRI is considered as a malformed. A BGP speaker MUST handle such a malformed NLRI as a "Treat-as-withdraw" [RFC7606]. (Section 3.1.4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.1.4.1-2, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3`](#draft-ietf-bess-mup-safi-3.3.3-3)

When a BGP speaker receives the Interwork Segment Dicovery routes with a MP_REACH_NLRI attribute without a prefix SID attribute, then it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied. (Section 3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.3-3, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4`](#draft-ietf-bess-mup-safi-3.3.3-4)

If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied. (§3.3.3)

Audit verdict: not audited: no reader has judged these tests

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.3-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4`](#draft-ietf-bess-mup-safi-3.3.6-4)

If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied. (§3.3.6)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-14 (independent auditor) after the receive recognizer changed. recognizerFor (mup/rfc7606.go) now runs RecognizeNLRI then ValidateNLRI, which checks the NLRI body alone: Retain hands it one NLRI and no path attributes, and no code in internal/component/bgp reads a Prefix-SID SRv6 locator or compares it with a DSD MP_REACH next hop, so a DSD route whose next hop and locator differ is still stored rather than treated as withdrawn. The {gap} annotation's phrase "reads the architecture and route type alone" is now out of date (the body is also validated), but its conclusion holds.

No test carries DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4, so no unit is bound to it.

### [`DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5`](#draft-ietf-bess-mup-safi-3.3.6-5)

When a BGP speaker receives a MP_REACH_NLRI attribute update message with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied. (§3.3.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-19 (independent auditor, after 691bd048e9). draft-ietf-bess-mup-safi Section 3.3.6: "When a BGP speaker receives a MP_REACH_NLRI attribute update message with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied.  A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message." The earlier unimplemented verdict is superseded: mup/rfc7606.go::CheckDSDPrefixSID is registered by mup/register.go for both ipv4/mup and ipv6/mup through nlritype.RegisterUpdateCheck, and the receive path (Session.applyRFC7606 -> applyTypedNLRIDiscard -> typedNLRIEdit -> nlritype.RetainUpdate) moves an announced DSD route with no Prefix-SID out of MP_REACH_NLRI into MP_UNREACH_NLRI per NLRI, keeping its siblings and the session; typedNLRIUpdateAttrs counts a Prefix-SID that RFC 7606 attribute-discard removes (the RFC 8669 Section 4 EBGP rule) as absent. Tests drive whole hex UPDATE bodies through Session.enforceRFC7606 and assert exact bytes: TestMUPDSDWithoutPrefixSIDWithdrawnOnReceive (kept MP_REACH NLRI equals the ISD sibling, MP_UNREACH NLRI equals the DSD route, no error, for both an absent and an EBGP-discarded Prefix-SID) and TestMUPDSDWithPrefixSIDKeptOnReceive (DSD with a Prefix-SID, and ISD plus Type 2 ST without one, return RFC7606ActionNone and the UPDATE byte for byte). The recorded discrimination breaks are panics, which prove reach only, so five overlay mutants were run (go test -overlay, tags ze_core ze_bgp): Keep instead of Withdraw when the Prefix-SID is absent (positive red, both cases); withdraw a DSD even with a Prefix-SID (negative DSD case red); drop the route-type test so every type is withdrawn (positive and negative ISD/T2ST case red); Discard instead of Withdraw (positive red, "withdrawn, not dropped"); ignore attribute-discard entries in typedNLRIUpdateAttrs (positive EBGP-discard case red). All five killed. Residual: only AFI 1 is exercised; the AFI 2 family is covered by the same registration loop, not by a test. test/plugin/mup-dsd-without-prefix-sid-withdraw.ci exists but is untagged and was not run here.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMUPDSDWithPrefixSIDKeptOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_dsd_prefix_sid_test.go#L108) | unit/verify | revert, verified |
| positive | [`TestMUPDSDWithoutPrefixSIDWithdrawnOnReceive`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/draft_ietf_bess_mup_safi_dsd_prefix_sid_test.go#L76) | unit/verify | revert, verified |

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
| Signed off | 2026-09-29 |
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
| `3.3:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence addresses BGP speakers acting as a MUP PE and the MUP Controller (Section 3.3); ze fills neither role. The role has no producer in this tree: the nlri/mup plugin registers the ipv4/mup and ipv6/mup codecs and RecognizeNLRI, and no ze function originates, resolves or controls MUP state as a PE or a controller. Row DRAFT-IETF-BESS-MUP-SAFI-3.3-1 retired 2026-09-29 (rfc/corrections/draft-ietf-bess-mup-safi.md); its generic capability negotiation tags moved to RFC4760-8-2. | BGP speakers acting as a PE, and a MUP Controller MUST establish a BGP session to exchange BGP-MUP NLRIs for both, IPv4 and IPv6 AFIs. |
| `3.3.3:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.3.3:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.3.6:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.3.6:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The document repeats one sentence after every malformed-NLRI ruling: a BGP speaker MUST skip such NLRIs and continue processing of the rest of the Update message. The checklist carries it once, at Section 3.1.1. | A BGP speaker MUST skip such NLRIs and continue processing of rest of the Update message. |
| `3.3.7:2` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | the sentence addresses the MUP Controller that originates the Type 1 ST route (Section 3.3.7); ze fills no MUP Controller role. The role has no producer in this tree: EncodeRoute encodes the route the operator's command names, and no ze function derives Type 1 ST routes from session state or selects the speakers of an SRv6 domain. Row DRAFT-IETF-BESS-MUP-SAFI-3.3.7-2 retired 2026-09-29 (rfc/corrections/draft-ietf-bess-mup-safi.md); its tags were dropped. | The controller MUST announce this route using a AFI of the route and the SAFI of BGP-MUP to all other BGP speakers within the SRv6 domain. |

## Superseded

No document obsoletes DRAFT-IETF-BESS-MUP-SAFI, so its obligations are stated where they were written.
