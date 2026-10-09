# RFC 9252 - BGP Overlay Services Based on Segment Routing over IPv6 (SRv6)

Partial. Every requirement this repository extracted from RFC 9252, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 44.0% | 11 of 25 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.0% | 5 of 25 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 25 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 52.6% | 20 of 38 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 25 | of 30 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 25 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 25 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 25 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 25 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Partial proof; remaining gap | 4.0% | 1 of 25 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 32.0% | 8 of 25 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 18 | of 25 gated MUSTs judged | 8 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 25 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | bad | green at zero, RED above it: a tested clause cannot prove the whole requirement |
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
| Requirements | 30 |
| Gated MUST-level | 25 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 8 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 38 |
| Tagged units | 38 |
| Recorded audit verdicts | 18 |
| Discrimination records | 20 |
| Summary | `rfc/short/rfc9252.md` |
| Requirement shard | `rfc/requirements/rfc9252.md` |
| RFC text | `rfc/full/rfc9252.txt` |

## Enrolment

Enrolled: SRv6 BGP Overlay Services / Prefix-SID Service TLV (RFC 9252): receive-side SRv6 L3/L2 Service TLV codec and forwarding treatment based on the final effective next-hop identity. Unchanged identity preserves received Service TLVs; changed identity removes them while retaining unrelated top-level TLVs. The changed-next-hop obligation remains partial because locally allocated SID information is not produced to rebuild the service information. Remaining obligations are bounded by the requirement rows, not by a codec-wide conformance claim. The 2026-09-21 extraction walk read the document against this checklist and changed two things. It corrected the section reference on 21 rows, which carried the draft's numbering (S3.1, S3.2, S3.3, S3.4, S4.1, S5, S6.1, S6.2) rather than the published one: the Service TLV Reserved field is Section 2, the SID Information Sub-TLV is Section 3.1, the argument-validation obligations are Section 3.2.1, the VPN transposition bound is Sections 5.1 and 5.2, the EVPN bounds are Sections 6.1.1 through 6.5, and error handling is Section 7. And it added 6 MUST rows the checklist did not carry: RFC9252-3.2.1-4 (shifted-out bits zeroed in the SID value), RFC9252-3.2.1-5 (per-neighbor and per-service advertisement control), RFC9252-3.2.1-6 (AL zero where the Argument does not apply), RFC9252-5-3 (ingress PE encapsulates in IPv6 and inserts an SRH when required), and RFC9252-7-1 and -7-2 (a second SRv6 L3 or L2 Service TLV is ignored). None of the 6 carries a tagged test, so each is an open gap.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Prefix-SID attribute (code 40) SRv6 L3/L2 Service TLV parse, SID Information Sub-TLV and SID Structure Sub-Sub-TLV extraction with the errata-7817 sum bound ([`internal/component/bgp/plugins/rib/pool/srv6sid.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid.go))
- Section 3.2.1 transposition undone for the IPv4 and IPv6 VPN families, whose Section 5.1 label field is read out of the NLRI the route is keyed by ([`internal/core/bgp/nlri/nlrisplit/transposition.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/transposition.go)) and merged back into the partial SID ([`internal/component/bgp/plugins/rib/rib_bestchange.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange.go), srv6SIDFromResult)
- malformed-Service-TLV treat-as-withdraw ([`internal/component/bgp/message/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go))
- no-valid-SID best-path ineligibility ([`internal/component/bgp/plugins/rib/rib_bestchange.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange.go))
- received Service TLVs, Reserved fields and unknown nested fields preserved when the final effective next-hop identity is unchanged, and Service TLVs removed when it changes, while unrelated top-level TLVs survive ([`internal/component/bgp/reactor/forward_prefix_sid.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/forward_prefix_sid.go)::applyEgressPrefixSIDNextHop and prefixSIDNextHopHandler)
- and SRv6 Prefix-SID config encode with zeroed reserved/flags octets ([`internal/core/bgp/attribute/prefixsid.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/prefixsid.go)). The changed-next-hop obligation is partial, not whole-row conformance: local SID allocation and rebuilding are absent. Requirements bound per line in [`rfc/short/rfc9252.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9252.md).


**What the ledger says remains**

[`RFC9252-3.3-2`](#rfc9252-3.3-2) is partial: changed-next-hop forwarding removes unrecognized nested fields with the received Service TLVs, but does not produce locally allocated SRv6 SID information to rebuild them. Local SID allocation and rebuilding remain absent and uncommissioned.

- **EVPN transposition is not implemented:** Section 6 puts the label field at a different NLRI offset per route type, in the PMSI Tunnel Attribute for Route Type 3 and in the ESI Label extended community for Route Type 1 per-ES, and Route Type 2 has two label fields bound to different Service TLVs. An EVPN route whose Prefix-SID declares a transposition therefore yields no SID rather than the partial one. Nine MUST-level rows carry gap or partial annotations in this checklist, counted once per requirement rather than per protocol obligation: [`RFC9252-3.3-2`](#rfc9252-3.3-2) above; [`RFC9252-4.1-1`](#rfc9252-4.1-1)/6.1-1/6.2-1 -- Transposition Length is bounded against the 20-bit VPN and 24-bit EVPN label field widths but still not against FL or AL; [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1)/3.2.1-2 -- the zero-offset-when-length-zero and zero-when-scheme-not-applicable transposition constraints are not enforced; [`RFC9252-3.2-5`](#rfc9252-3.2-5)/3.2-6 -- ze keeps no SRv6 Endpoint Behavior registry, so it neither ignores SIDs with a non-zero Argument Length under an unknown behavior nor validates AL against a known behavior; and [`RFC9252-5-2`](#rfc9252-5-2) -- ze installs the received Service SID into the Linux FIB (kernel SEG6 encap at [`internal/plugins/fib/kernel/nexthop_linux.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/nexthop_linux.go)) but performs no Section 5 resolvability check on the SID locator before best-path computation (isSRv6Ineligible gates on extraction validity only). VPP policy installation and packet qualification belong to the separate SRv6 FIB work described in [`docs/architecture/fib/fib-depth-4-srv6.md`](https://github.com/ze-software/ze/blob/main/docs/architecture/fib/fib-depth-4-srv6.md); this BGP forwarding proof makes no VPP dataplane claim. The former eight-row figure omitted the newly partial [`RFC9252-3.3-2`](#rfc9252-3.3-2); the former fourteen figure added six historical extraction findings rather than counting current gap annotations. Neither figure is a current conformance total.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 11 | one part of the gated population |
| Annotated (including scoped evidence) | 14 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 1 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **25** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (11):** [`RFC9252-3.1-2`](#rfc9252-3.1-2), [`RFC9252-3.2.1-3`](#rfc9252-3.2.1-3), [`RFC9252-5-1`](#rfc9252-5-1), [`RFC9252-3.3-1`](#rfc9252-3.3-1), [`RFC9252-3.4-1`](#rfc9252-3.4-1), [`RFC9252-3.2.1-4`](#rfc9252-3.2.1-4), [`RFC9252-3.2.1-5`](#rfc9252-3.2.1-5), [`RFC9252-3.2.1-6`](#rfc9252-3.2.1-6), [`RFC9252-5-3`](#rfc9252-5-3), [`RFC9252-7-1`](#rfc9252-7-1), [`RFC9252-7-2`](#rfc9252-7-2)

**Annotated (including scoped evidence) (14):** [`RFC9252-3.1-1`](#rfc9252-3.1-1), [`RFC9252-3.2-1`](#rfc9252-3.2-1), [`RFC9252-3.2-2`](#rfc9252-3.2-2), [`RFC9252-3.2-3`](#rfc9252-3.2-3), [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1), [`RFC9252-3.2.1-2`](#rfc9252-3.2.1-2), [`RFC9252-4.1-1`](#rfc9252-4.1-1), [`RFC9252-6.1-1`](#rfc9252-6.1-1), [`RFC9252-6.2-1`](#rfc9252-6.2-1), [`RFC9252-3.2-4`](#rfc9252-3.2-4), [`RFC9252-3.2-5`](#rfc9252-3.2-5), [`RFC9252-3.2-6`](#rfc9252-3.2-6), [`RFC9252-5-2`](#rfc9252-5-2), [`RFC9252-3.3-2`](#rfc9252-3.3-2)

**Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) (1):** [`RFC9252-3.3-2`](#rfc9252-3.3-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9252-3.1-1` | This field is reserved; it MUST be set to 0 by the sender (§2) | MUST | 2 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L163). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes the Service TLV Reserved octet to 0 on encode and no code path emits a non-zero value, so there is no negative input to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC9252-3.1-2` | This field is reserved; it MUST be set to 0 by the sender and ignored by the receiver. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestExtractSRv6SID_ServiceReservedZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L255). **negative:** `unit/verify` [`TestExtractSRv6SID_ServiceReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L269) |
| `RFC9252-3.2-1` | RESERVED1 (1 octet): This field MUST be set to 0 by the sender (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L164). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes RESERVED1 to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC9252-3.2-2` | This field encodes SRv6 Service SID Flags -- none are currently defined. It MUST be set to 0 by the sender (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L165). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes the Service SID Flags octet to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC9252-3.2-3` | RESERVED2 (1 octet): This field MUST be set to 0 by the sender (§3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L166). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes RESERVED2 to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC9252-3.2.1-1` | In this case, the Transposition Offset MUST be set to 0. (S3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseSIDStructure returns no-transposition when Transposition Length is 0 and never marks the SID invalid for a non-zero Transposition Offset, and ParsePrefixSIDSRv6 passes the configured structure through without enforcing offset 0 (internal/component/bgp/plugins/rib/pool/srv6sid.go:132) |
| `RFC9252-3.2.1-2` | The transposition offset and length MUST be 0 when the Sub-Sub-TLV is advertised along with routes where the Transposition Scheme is not applicable (e.g., for global IPv6 service [RFC2545] where there is no label field). (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseSIDStructure has no family or label-field context, so it never enforces zero Transposition Offset and Length for SIDs advertised with routes where transposition does not apply (internal/component/bgp/plugins/rib/pool/srv6sid.go:106) |
| `RFC9252-3.2.1-3` | As defined in [RFC8986], the sum of the Locator Block Length (LBL), Locator Node Length (LNL), Function Length (FL), and Argument Length (AL) fields MUST be less than or equal to 128 and greater than or equal to the sum of Transposition Offset and Transposition Length. (S3.2.1, errata 7817) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestExtractSRv6SIDFull_WithTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L139). **negative:** `unit/verify` [`TestExtractSRv6SIDFull_InvalidSIDStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L217). **negative:** `unit/verify` [`TestExtractSRv6SIDFull_SumBelowTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L235) |
| `RFC9252-4.1-1` | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 20 and less than or equal to the FL. (§5.1, §5.2) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the 20-bit half is enforced -- srv6SIDFromResult refuses to reconstruct and isSRv6Ineligible makes the path ineligible when Transposition Length exceeds labelWidthForSAFI (internal/component/bgp/plugins/rib/rib_bestchange.go) -- but neither they nor parseSIDStructure bound it against the Function Length (internal/component/bgp/plugins/rib/pool/srv6sid.go) |
| `RFC9252-6.1-1` | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the AL. (§6.1.1) | MUST | 6.1.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the 24-bit half is enforced by labelWidthForSAFI through srv6SIDFromResult and isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go), but the Argument Length is never bounded, the ESI Label extended community that carries these bits is never read, and the EVPN encoder carries no SRv6 ESI-label SID (internal/component/bgp/plugins/rib/pool/srv6sid.go) |
| `RFC9252-6.2-1` | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. (§6.2, §6.3, §6.5) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the 24-bit half is enforced by labelWidthForSAFI through srv6SIDFromResult and isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go), but the Function Length is never bounded and no EVPN label field is read at all -- TranspositionLabel answers only the VPN families, so an EVPN transposition yields no SID rather than a reconstructed one (internal/core/bgp/nlri/nlrisplit/transposition.go) |
| `RFC9252-3.2-4` | An unrecognized SRv6 Endpoint Behavior MUST NOT be considered invalid by the receiver, except for behaviors that involve the use of arguments (§3.1) | MUST NOT | 3.1 | **positive:** `unit/verify` [`TestExtractSRv6SID_UnknownEndpointBehavior`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L285). **negative:** no negative test. **{single-polarity}:** extractSIDFromServiceTLV extracts the SID without inspecting or validating the SRv6 Endpoint Behavior, so an unrecognized behavior is never rejected and there is no behavior-based rejection path to drive negatively (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| `RFC9252-3.2-5` | A receiver is unable to validate the applicability of arguments for SRv6 Endpoint Behaviors that are unknown to it and hence MUST ignore SRv6 SIDs with arguments (indicated by a non-zero AL) with unknown SRv6 Endpoint Behaviors. (S3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze maintains no SRv6 Endpoint Behavior registry and extractSIDFromServiceTLV returns the SID regardless of a non-zero Argument Length or an unknown behavior, so such SIDs are used rather than ignored (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| `RFC9252-3.2-6` | For SIDs corresponding to an SRv6 Endpoint Behavior that is known, a receiver MUST validate that the consistency of the AL with the specific SRv6 Endpoint Behavior definition. (S3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no SRv6 Endpoint Behavior definitions, so it never validates the Argument Length against a known behavior's expected argument size (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| `RFC9252-5-1` | The path having any such Prefix-SID attribute without any valid SRv6 SID information MUST be considered ineligible during the selection of the best path for the corresponding prefix. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestIsSRv6Ineligible_ValidSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_ineligible_test.go#L72). **negative:** `unit/verify` [`TestIsSRv6Ineligible_InvalidSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_ineligible_test.go#L83). **negative:** `unit/verify` [`TestSRv6TranspositionWiderThanLabelFieldIsIneligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_transposition_test.go#L201) |
| `RFC9252-5-2` | Therefore, the ingress PE MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix for the BGP best path computation. (S5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze acts as an SRv6 ingress PE -- it extracts the received best-path Service SID (internal/component/bgp/plugins/rib/rib_bestchange.go:729,:882) and installs it into the FIB as a kernel SEG6 encap route (internal/plugins/fib/kernel/nexthop_linux.go:78) or a VPP SR steering policy (internal/plugins/fib/vpp/srv6.go:35) -- but performs no RFC 9252 Section 5 resolvability check: isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go:963) gates best-path on SID extraction validity only, never on locator reachability (no resolvability check exists in internal/component/bgp) |
| `RFC9252-3.3-1` | If the BGP next hop is unchanged during the advertisement, the SRv6 Service TLVs, including any unrecognized Types of Sub-TLV and Sub-Sub-TLV, SHOULD be propagated further. In addition, all Reserved fields in the TLV, Sub-TLV, or Sub-Sub-TLV MUST be propagated unchanged. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L338). **positive:** `unit/verify` [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L31). **positive:** `unit/verify` [`TestRFC9252ReservedFieldsPropagatedUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_reserved_propagated_test.go#L55). **negative:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L339). **negative:** `unit/verify` [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L32) |
| `RFC9252-3.3-2` | If the BGP next hop is changed, the TLVs, Sub-TLVs, and Sub-Sub- TLVs SHOULD be updated with the locally allocated SRv6 SID information. Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed. (§2) | MUST | 2 | **positive:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L340). **positive:** `unit/verify` [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L33). **positive:** `unit/verify` [`TestRFC9252ServiceOnlyPrefixSIDDroppedOnANextHopChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_nexthop_change_defect_test.go#L169). **negative:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L341). **negative:** `unit/verify` [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L34). **{partial}:** tested "Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed."; gap "SHOULD be updated with the locally allocated SRv6 SID information."; internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop compares the received and final effective next-hop entity and requests removal of the received SRv6 Service TLVs when that entity changes. The registered handler removes their unknown nested fields while retaining unrelated top-level TLVs. Locally allocated SRv6 SID information is not produced to rebuild the service information; only that allocation/rebuilding remains absent and uncommissioned.. **Scoped evidence:** Tests and tag-claim records apply only to Tested; zero whole-requirement credit. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Tested: ‘Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed.’ Gap: ‘SHOULD be updated with the locally allocated SRv6 SID information.’ Producer: internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop. Independent post-fix review confirms that original received identity is compared with final effective carrier identity after policy/configuration/scope on cached and RS paths, including raw-policy fallback and repaired intermediate addressless MP fields. The registered prefixSIDNextHopHandler consumes Remove of types 5 and 6 while retaining unrelated top-level TLVs. Migrated TestPrefixSIDPropagationNextHop and TestRFC9252EffectiveNextHopControlsServiceTLVs inspect actual recipient Session output: effective A-to-B removes Service TLVs and unknown nested fields while the route survives; A-to-A and restoration controls preserve them; mixed MP/legacy cases follow the applicable MP identity; unknown top-level TLVs and SRGB remain, with Label-Index transmit normalization tested separately. The older service-only test supplies complementary removal evidence, and the new actual-writer service-only cases forbid an empty attribute. The policy-only-change and equal-address defects are fixed, not scoped away. Locally allocated SRv6 SID information is still not produced to rebuild the service information, so the whole SHOULD-plus-MUST quotation remains partial. No local SID allocation is commissioned, and pending live FRR/SDK or native-record completion is not claimed by this source judgment. |
| `RFC9252-3.4-1` | The treat-as-withdraw action [RFC7606] MUST be performed when at least one malformed SRv6 Service TLV is present in the BGP Prefix-SID attribute. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestValidatePrefixSIDAttr_Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2404). **negative:** `unit/verify` [`TestValidateSRv6ServiceTLV_SIDInfoTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2473). **negative:** `unit/verify` [`TestValidateSRv6ServiceTLV_TrailingBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2433) |
| `RFC9252-3.2.1-4` | The bits that have been shifted out MUST be set to 0 in the SID value. (§3.2.1) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestSRv6OriginTranspositionKeepsOnlyUntransposedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L10). **negative:** `unit/verify` [`TestSRv6OriginRefusesNonzeroTransposedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L21) |
| `RFC9252-3.2.1-5` | Implementations supporting this specification MUST provide a mechanism to control the advertisement of SRv6-based BGP service routes on a per-neighbor and per-service basis. (S3.2.1) | MUST | 3.2.1 | **positive:** `functional/verify` [`srv6-service-export-control.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/srv6-service-export-control.ci#L2). **negative:** `functional/verify` [`srv6-service-export-control.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/srv6-service-export-control.ci#L3) |
| `RFC9252-3.2.1-6` | Arguments may be generally applicable for SIDs of only specific SRv6 Endpoint Behaviors (e.g., End.DT2M); therefore, the AL MUST be set to 0 for SIDs where the Argument is not applicable. (S3.2.1) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestSRv6OriginWithoutArguments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L30). **negative:** `unit/verify` [`TestSRv6OriginRefusesArgumentsForDT4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L41) |
| `RFC9252-5-3` | it indicates that the egress PE supports SRv6 overlay, and the BGP ingress PE receiving this route MUST perform IPv6 encapsulation and insert an SRH [RFC8754] when required (§5) | MUST | 5 | **positive:** `unit/verify` [`TestSRv6ServiceSIDSelectsIPv6Encapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc9252_nexthop_srv6_linux_test.go#L15). **negative:** `unit/verify` [`TestRouteWithoutServiceSIDKeepsMPLSEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc9252_nexthop_srv6_linux_test.go#L39) |
| `RFC9252-7-1` | If multiple instances of the SRv6 L3 Service TLV are encountered, all but the first instance MUST be ignored. (S7) | MUST | 7 | **positive:** `unit/verify` [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L31). **negative:** `unit/verify` [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L32) |
| `RFC9252-7-2` | If multiple instances of the SRv6 L2 Service TLV are encountered, all but the first instance MUST be ignored. (S7) | MUST | 7 | **positive:** `unit/verify` [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L33). **negative:** `unit/verify` [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L34) |
| `RFC9252-3.2-7` | When multiple SRv6 SID Information Sub-TLVs are present, the ingress PE SHOULD use the SRv6 SID from the first instance of the Sub-TLV. (§3.1) | SHOULD | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.3-3` | If the BGP next hop is unchanged during the advertisement, the SRv6 Service TLVs, including any unrecognized Types of Sub-TLV and Sub-Sub-TLV, SHOULD be propagated further. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.3-4` | If the BGP next hop is changed, the TLVs, Sub-TLVs, and Sub-Sub- TLVs SHOULD be updated with the locally allocated SRv6 SID information. (§2) | SHOULD | 2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.2-8` | When multiple SRv6 SID Information Sub-TLVs are present, the ingress PE SHOULD use the SRv6 SID from the first instance of the Sub-TLV. An implementation MAY provide a local policy to override this selection. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.2-9` | The opaque SRv6 Endpoint Behavior (i.e., value 0xFFFF) MAY be used when the advertising router wishes to abstract the actual behavior of its locally instantiated SRv6 SID. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1) In this case, the Transposition Offset MUST be set to 0. (S3.2.1) | {gap}, no test | parseSIDStructure returns no-transposition when Transposition Length is 0 and never marks the SID invalid for a non-zero Transposition Offset, and ParsePrefixSIDSRv6 passes the configured structure through without enforcing offset 0 (internal/component/bgp/plugins/rib/pool/srv6sid.go:132) |
| [`RFC9252-3.2.1-2`](#rfc9252-3.2.1-2) The transposition offset and length MUST be 0 when the Sub-Sub-TLV is advertised along with routes where the Transposition Scheme is not applicable (e.g., for global IPv6 service [RFC2545] where there is no label field). (§7) | {gap}, no test | parseSIDStructure has no family or label-field context, so it never enforces zero Transposition Offset and Length for SIDs advertised with routes where transposition does not apply (internal/component/bgp/plugins/rib/pool/srv6sid.go:106) |
| [`RFC9252-4.1-1`](#rfc9252-4.1-1) When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 20 and less than or equal to the FL. (§5.1, §5.2) | {gap}, no test | the 20-bit half is enforced -- srv6SIDFromResult refuses to reconstruct and isSRv6Ineligible makes the path ineligible when Transposition Length exceeds labelWidthForSAFI (internal/component/bgp/plugins/rib/rib_bestchange.go) -- but neither they nor parseSIDStructure bound it against the Function Length (internal/component/bgp/plugins/rib/pool/srv6sid.go) |
| [`RFC9252-6.1-1`](#rfc9252-6.1-1) When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the AL. (§6.1.1) | {gap}, no test | the 24-bit half is enforced by labelWidthForSAFI through srv6SIDFromResult and isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go), but the Argument Length is never bounded, the ESI Label extended community that carries these bits is never read, and the EVPN encoder carries no SRv6 ESI-label SID (internal/component/bgp/plugins/rib/pool/srv6sid.go) |
| [`RFC9252-6.2-1`](#rfc9252-6.2-1) When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. (§6.2, §6.3, §6.5) | {gap}, no test | the 24-bit half is enforced by labelWidthForSAFI through srv6SIDFromResult and isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go), but the Function Length is never bounded and no EVPN label field is read at all -- TranspositionLabel answers only the VPN families, so an EVPN transposition yields no SID rather than a reconstructed one (internal/core/bgp/nlri/nlrisplit/transposition.go) |
| [`RFC9252-3.2-5`](#rfc9252-3.2-5) A receiver is unable to validate the applicability of arguments for SRv6 Endpoint Behaviors that are unknown to it and hence MUST ignore SRv6 SIDs with arguments (indicated by a non-zero AL) with unknown SRv6 Endpoint Behaviors. (S3.2) | {gap}, no test | ze maintains no SRv6 Endpoint Behavior registry and extractSIDFromServiceTLV returns the SID regardless of a non-zero Argument Length or an unknown behavior, so such SIDs are used rather than ignored (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| [`RFC9252-3.2-6`](#rfc9252-3.2-6) For SIDs corresponding to an SRv6 Endpoint Behavior that is known, a receiver MUST validate that the consistency of the AL with the specific SRv6 Endpoint Behavior definition. (S3.2) | {gap}, no test | ze has no SRv6 Endpoint Behavior definitions, so it never validates the Argument Length against a known behavior's expected argument size (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| [`RFC9252-5-2`](#rfc9252-5-2) Therefore, the ingress PE MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix for the BGP best path computation. (S5) | {gap}, no test | ze acts as an SRv6 ingress PE -- it extracts the received best-path Service SID (internal/component/bgp/plugins/rib/rib_bestchange.go:729,:882) and installs it into the FIB as a kernel SEG6 encap route (internal/plugins/fib/kernel/nexthop_linux.go:78) or a VPP SR steering policy (internal/plugins/fib/vpp/srv6.go:35) -- but performs no RFC 9252 Section 5 resolvability check: isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go:963) gates best-path on SID extraction validity only, never on locator reachability (no resolvability check exists in internal/component/bgp) |
| [`RFC9252-3.3-2`](#rfc9252-3.3-2) If the BGP next hop is changed, the TLVs, Sub-TLVs, and Sub-Sub- TLVs SHOULD be updated with the locally allocated SRv6 SID information. Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed. (§2) | {partial}, Partial proof; remaining gap | tested "Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed."; gap "SHOULD be updated with the locally allocated SRv6 SID information."; internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop compares the received and final effective next-hop entity and requests removal of the received SRv6 Service TLVs when that entity changes. The registered handler removes their unknown nested fields while retaining unrelated top-level TLVs. Locally allocated SRv6 SID information is not produced to rebuild the service information; only that allocation/rebuilding remains absent and uncommissioned. |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9252-3.1-1`](#rfc9252-3.1-1)

This field is reserved; it MUST be set to 0 by the sender (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: the sender emits a non-zero Service TLV RESERVED octet. (b) TestEncodePrefixSIDSRv6_ReservedFieldsZero asserts ps[3]==0 on EncodePrefixSIDSRv6 output, with a non-zero behavior code so the zero is not a zero-filled buffer. Single-polarity marker present (no reject path on encode).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L163) | unit/verify | revert, verified |

### [`RFC9252-3.1-2`](#rfc9252-3.1-2)

This field is reserved; it MUST be set to 0 by the sender and ignored by the receiver. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a receiver that rejects or mis-parses a Service TLV whose RESERVED octet is non-zero. (b) TestExtractSRv6SID_ServiceReservedIgnored feeds RESERVED=0xFF and asserts ExtractSRv6SID still returns the SID; TestExtractSRv6SID_ServiceReservedZero is the positive. validateSRv6ServiceTLV (rfc7606.go) never reads the octet, so the UPDATE path does not reject it either.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtractSRv6SID_ServiceReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L269) | unit/verify | unproven |
| positive | [`TestExtractSRv6SID_ServiceReservedZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L255) | unit/verify | unproven |

### [`RFC9252-3.2-1`](#rfc9252-3.2-1)

RESERVED1 (1 octet): This field MUST be set to 0 by the sender (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: the sender emits a non-zero SID Information RESERVED1 octet. (b) TestEncodePrefixSIDSRv6_ReservedFieldsZero asserts ps[7]==0. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L164) | unit/verify | revert, verified |

### [`RFC9252-3.2-2`](#rfc9252-3.2-2)

This field encodes SRv6 Service SID Flags -- none are currently defined. It MUST be set to 0 by the sender (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: the sender emits a non-zero Service SID Flags octet. (b) TestEncodePrefixSIDSRv6_ReservedFieldsZero asserts ps[24]==0. The quote covers the sender half only; the receiver half (unknown flags MUST be ignored) has no row. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L165) | unit/verify | revert, verified |

### [`RFC9252-3.2-3`](#rfc9252-3.2-3)

RESERVED2 (1 octet): This field MUST be set to 0 by the sender (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: the sender emits a non-zero RESERVED2 octet. (b) TestEncodePrefixSIDSRv6_ReservedFieldsZero asserts ps[27]==0, next to a behavior of 0x003e. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L166) | unit/verify | revert, verified |

### [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1)

In this case, the Transposition Offset MUST be set to 0. (S3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-3.2.1-1, so no unit is bound to it.

### [`RFC9252-3.2.1-2`](#rfc9252-3.2.1-2)

The transposition offset and length MUST be 0 when the Sub-Sub-TLV is advertised along with routes where the Transposition Scheme is not applicable (e.g., for global IPv6 service [RFC2545] where there is no label field). (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-3.2.1-2, so no unit is bound to it.

### [`RFC9252-3.2.1-3`](#rfc9252-3.2.1-3)

As defined in [RFC8986], the sum of the Locator Block Length (LBL), Locator Node Length (LNL), Function Length (FL), and Argument Length (AL) fields MUST be less than or equal to 128 and greater than or equal to the sum of Transposition Offset and Transposition Length. (S3.2.1, errata 7817)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Re-judged 2026-09-27 by QF-2 after the row took the Corrected Text of verified erratum 7817 (rfc/errata/rfc9252/7817.txt): 'greater than or equal to'. Both bounds are detected: TestExtractSRv6SIDFull_InvalidSIDStructure (sum 200 > 128) and TestExtractSRv6SIDFull_SumBelowTransposition (64 < 76) assert HasTranspos false, and TestExtractSRv6SIDFull_WithTransposition accepts sum 64 == offset 48 + length 16, the equality the erratum allows. Weak: both negatives also assert the SID stays valid and ze uses it whole, while Section 7 makes such a SID invalid and the path ineligible, so no assertion goes red on the consequence of breaking this MUST.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtractSRv6SIDFull_InvalidSIDStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L217) | unit/verify | unproven |
| negative | [`TestExtractSRv6SIDFull_SumBelowTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L235) | unit/verify | unproven |
| positive | [`TestExtractSRv6SIDFull_WithTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L139) | unit/verify | unproven |

### [`RFC9252-4.1-1`](#rfc9252-4.1-1)

When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 20 and less than or equal to the FL. (§5.1, §5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-4.1-1, so no unit is bound to it.

### [`RFC9252-6.1-1`](#rfc9252-6.1-1)

When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the AL. (§6.1.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-6.1-1, so no unit is bound to it.

### [`RFC9252-6.2-1`](#rfc9252-6.2-1)

When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. (§6.2, §6.3, §6.5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-6.2-1, so no unit is bound to it.

### [`RFC9252-3.2-4`](#rfc9252-3.2-4)

An unrecognized SRv6 Endpoint Behavior MUST NOT be considered invalid by the receiver, except for behaviors that involve the use of arguments (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: a receiver treating a SID with an unrecognized, argument-free Endpoint Behavior as invalid. (b) TestExtractSRv6SID_UnknownEndpointBehavior feeds behavior 0xFEED with no SID Structure and asserts the SID is returned. validateSRv6ServiceTLV never reads the behavior. The argument exception is the gap row RFC9252-3.2-5. Single-polarity marker present.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExtractSRv6SID_UnknownEndpointBehavior`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_srv6sid_test.go#L285) | unit/verify | unproven |

### [`RFC9252-3.2-5`](#rfc9252-3.2-5)

A receiver is unable to validate the applicability of arguments for SRv6 Endpoint Behaviors that are unknown to it and hence MUST ignore SRv6 SIDs with arguments (indicated by a non-zero AL) with unknown SRv6 Endpoint Behaviors. (S3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-3.2-5, so no unit is bound to it.

### [`RFC9252-3.2-6`](#rfc9252-3.2-6)

For SIDs corresponding to an SRv6 Endpoint Behavior that is known, a receiver MUST validate that the consistency of the AL with the specific SRv6 Endpoint Behavior definition. (S3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-3.2-6, so no unit is bound to it.

### [`RFC9252-5-1`](#rfc9252-5-1)

The path having any such Prefix-SID attribute without any valid SRv6 SID information MUST be considered ineligible during the selection of the best path for the corresponding prefix. (§7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. RFC 9252 Section 7: 'The path having any such Prefix-SID attribute without any valid SRv6 SID information MUST be considered ineligible during the selection of the best path for the corresponding prefix.' Its preceding sentence defines invalidity: 'The SRv6 SID value in the SRv6 SID Information Sub-TLV is invalid when the SID Structure Sub-Sub-TLV transposition length is greater than the number of bits of the label field or if any of the conditions for the fields of the Sub-Sub-TLV, as specified in Section 3.2.1, is not met.' Section 7 also requires zero transposition offset/length on families where transposition does not apply. Section 3.2.1 was read with verified erratum 7817 (sum >= offset+length). All tagged tests were read: internal/component/bgp/plugins/rib/rfc9252_srv6_ineligible_test.go::TestIsSRv6Ineligible_ValidSID and ::TestIsSRv6Ineligible_InvalidSID, and internal/component/bgp/plugins/rib/rfc9252_srv6_transposition_test.go::TestSRv6TranspositionWiderThanLabelFieldIsIneligible. The positive helper test uses a valid extracted SID. The first negative instead supplies a five-octet SID Information Sub-TLV: this is structural malformation whose RFC action is treat-as-withdraw, not an isolated semantic-ineligibility case. The transposition negative supplies an otherwise valid VPNv4 SID Structure with FL=24, offset=48, length=24 and a 20-bit label field, but checks only storedSRv6SID -> storedPathSRv6SID -> srv6SIDFromResult returning no SID. Its final comment explicitly excludes candidate admission from the assertion. Thus its tag claims a selection result the test never checks. rib_bestchange.go::isSRv6Ineligible calls pool.ExtractSRv6SID, which discards the transposition result and returns the extracted SID even for this width violation; rib_commands.go::candidateAdmitted and rib_flowspec_validation.go::reconcileFlowSpecs use that predicate. pool/srv6sid.go::parseSIDStructure also returns no-transposition on invalid sum without invalidating the SID returned by extractSIDFromServiceTLV. These are source-backed existing-capability defects, not authorization to implement absent EVPN reconstruction or endpoint-behavior features. Proposed unrun reproduction: feed the existing tooWide=24 VPNv4 fixture and assert zero real candidates and no best-change install; pair with valid 20-bit boundary input. Current tests can stay green while the invalid route participates in selection. No build, test or mutation was run for this judgment; no matching discrimination record exists.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsSRv6Ineligible_InvalidSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_ineligible_test.go#L83) | unit/verify | unproven |
| negative | [`TestSRv6TranspositionWiderThanLabelFieldIsIneligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_transposition_test.go#L201) | unit/verify | unproven |
| positive | [`TestIsSRv6Ineligible_ValidSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rfc9252_srv6_ineligible_test.go#L72) | unit/verify | unproven |

### [`RFC9252-5-2`](#rfc9252-5-2)

Therefore, the ingress PE MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix for the BGP best path computation. (S5)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. {gap} confirmed at the producer. isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go) is the only best-path gate on the SRv6 Service SID, and it excludes a route only when an SRv6 Service TLV yields no extractable SID; it never asks whether the SID resolves. The one resolvability check ze runs, srv6SIDResolvable in sysRIB.fibEntry (internal/component/sysrib/sysrib.go), runs after both selections and only declines the FIB write, so a route whose SID reaches nothing still wins the prefix and the next-best route is not used. No tagged unit exists; the check before best path is plan/immediate/spec-srv6-bestpath-resolvability.md.

No test carries RFC9252-5-2, so no unit is bound to it.

### [`RFC9252-3.3-1`](#rfc9252-3.3-1)

If the BGP next hop is unchanged during the advertisement, the SRv6 Service TLVs, including any unrecognized Types of Sub-TLV and Sub-Sub-TLV, SHOULD be propagated further. In addition, all Reserved fields in the TLV, Sub-TLV, or Sub-Sub-TLV MUST be propagated unchanged. (§2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Rejudged the whole RFC 9252 Section 2 unchanged-next-hop propagation and Reserved-field obligation after the implemented identity-condition fix. applyFactsNextHop now records only next-hop operations; applyEgressPrefixSIDNextHop compares the original received entity with the final effective carrier after policy, configured rewriting and scope normalization on cached and RS paths. Equal identity records no Service-TLV removal, including explicit A-to-A and configured restoration of the original address. The migrated TestPrefixSIDPropagationNextHop and new TestRFC9252EffectiveNextHopControlsServiceTLVs inspect actual recipient Session output after validated receipt and cache publication, asserting exact Service-TLV Reserved and unknown nested bytes for unchanged identity, with changed-identity counter-controls. The matrix covers native IPv6/VPNv6 carriers, operation and raw-policy changes, RS export fallback, mixed legacy/MP output, pair trimming and repaired intermediate addressless MP fields. MP carrier presence is selected by base.mpFamily rather than address validity, so a legacy sibling or absent companion operation cannot decide the MP service identity. The earlier reserved-field fixture remains complementary materialized-body evidence, not the sole end-to-end proof. Legacy/mapped and extra L2 fixtures are propagation controls, not claims of service origination or EVPN support. The former explicit-equality defect is fixed, not retained as a scope gap.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L32) | unit/verify | revert, verified |
| negative | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L339) | unit/verify | revert, verified |
| positive | [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L338) | unit/verify | revert, verified |
| positive | [`TestRFC9252ReservedFieldsPropagatedUnchanged`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_reserved_propagated_test.go#L55) | unit/verify | revert, verified |

### [`RFC9252-3.3-2`](#rfc9252-3.3-2)

If the BGP next hop is changed, the TLVs, Sub-TLVs, and Sub-Sub- TLVs SHOULD be updated with the locally allocated SRv6 SID information. Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed. (§2)

Scoped tag-claim records only: tested "Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed."; gap "SHOULD be updated with the locally allocated SRv6 SID information."; internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop compares the received and final effective next-hop entity and requests removal of the received SRv6 Service TLVs when that entity changes. The registered handler removes their unknown nested fields while retaining unrelated top-level TLVs. Locally allocated SRv6 SID information is not produced to rebuild the service information; only that allocation/rebuilding remains absent and uncommissioned.; zero whole-requirement credit.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Tested: ‘Any received Sub-TLVs and Sub-Sub-TLVs that are unrecognized MUST be removed.’ Gap: ‘SHOULD be updated with the locally allocated SRv6 SID information.’ Producer: internal/component/bgp/reactor/forward_prefix_sid.go::applyEgressPrefixSIDNextHop. Independent post-fix review confirms that original received identity is compared with final effective carrier identity after policy/configuration/scope on cached and RS paths, including raw-policy fallback and repaired intermediate addressless MP fields. The registered prefixSIDNextHopHandler consumes Remove of types 5 and 6 while retaining unrelated top-level TLVs. Migrated TestPrefixSIDPropagationNextHop and TestRFC9252EffectiveNextHopControlsServiceTLVs inspect actual recipient Session output: effective A-to-B removes Service TLVs and unknown nested fields while the route survives; A-to-A and restoration controls preserve them; mixed MP/legacy cases follow the applicable MP identity; unknown top-level TLVs and SRGB remain, with Label-Index transmit normalization tested separately. The older service-only test supplies complementary removal evidence, and the new actual-writer service-only cases forbid an empty attribute. The policy-only-change and equal-address defects are fixed, not scoped away. Locally allocated SRv6 SID information is still not produced to rebuild the service information, so the whole SHOULD-plus-MUST quotation remains partial. No local SID allocation is commissioned, and pending live FRR/SDK or native-record completion is not claimed by this source judgment.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L34) | unit/verify | revert, verified |
| negative | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L341) | unit/verify | revert, verified |
| positive | [`TestRFC9252ServiceOnlyPrefixSIDDroppedOnANextHopChange`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc8669_nexthop_change_defect_test.go#L169) | unit/verify | revert, verified |
| positive | [`TestRFC9252EffectiveNextHopControlsServiceTLVs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_effective_next_hop_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc9252_peer_forward_facts_test.go#L340) | unit/verify | revert, verified |

### [`RFC9252-3.4-1`](#rfc9252-3.4-1)

The treat-as-withdraw action [RFC7606] MUST be performed when at least one malformed SRv6 Service TLV is present in the BGP Prefix-SID attribute. (§7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Section 7 lists the malformed cases. Asserted to give treat-as-withdraw: trailing bytes after the last Sub-TLV (TestValidateSRv6ServiceTLV_TrailingBytes) and a SID Information Sub-TLV under 21 octets (TestValidateSRv6ServiceTLV_SIDInfoTooShort). Not asserted by any tagged unit: TLV Length less than 1, and TLV Length inconsistent with the Prefix-SID attribute length. The second one gives the wrong action: validatePrefixSIDAttr answers RFC7606ActionAttributeDiscard for a type 5 or 6 TLV that overruns the attribute, not treat-as-withdraw.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateSRv6ServiceTLV_SIDInfoTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2473) | unit/verify | unproven |
| negative | [`TestValidateSRv6ServiceTLV_TrailingBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2433) | unit/verify | unproven |
| positive | [`TestValidatePrefixSIDAttr_Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2404) | unit/verify | unproven |

### [`RFC9252-3.2.1-4`](#rfc9252-3.2.1-4)

The bits that have been shifted out MUST be set to 0 in the SID value. (§3.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: advertising a SID with a set bit inside the transposed range. (b) TestSRv6OriginRefusesNonzeroTransposedBits configures bit 79 set within offset 65 length 15 and requires ParseRouteAttributes to fail (validateSRv6OriginStructure); TestSRv6OriginTranspositionKeepsOnlyUntransposedBits is the positive, keeping the adjacent bit 64.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSRv6OriginRefusesNonzeroTransposedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L21) | unit/verify | unproven |
| positive | [`TestSRv6OriginTranspositionKeepsOnlyUntransposedBits`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L10) | unit/verify | unproven |

### [`RFC9252-3.2.1-5`](#rfc9252-3.2.1-5)

Implementations supporting this specification MUST provide a mechanism to control the advertisement of SRv6-based BGP service routes on a per-neighbor and per-service basis. (S3.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: no way to withhold an SRv6 service route from one neighbour while sending another service to it, or while sending the same service to another neighbour. (b) test/plugin/srv6-service-export-control.ci: filtered-peer has reject=pattern for the RT10 SID and expects the RT20 SID (per service); control-peer expects both SIDs (per neighbour). Losing either control turns an expect or reject red. Re-read after the 2026-09-29 change: both peers now advertise Extended Next Hop for ipv4/mpls-vpn, because RFC 9252 Section 5.1 encodes IPv4 VPN over SRv6 per RFC 8950 and RFC 8950 Section 4 now withholds the IPv6 next hop from a peer without the pair; no expect or reject line changed. Observed-red records: positive on evaluateCommunities (the export filter fails closed, RT20 never reaches filtered-peer), negative on Peer.ExportFilters (a panic, so it proves reach only; the per-neighbour discrimination of the negative is by reading: a filter applied to every peer withholds RT10 from control-peer and its seq=2 expect goes red).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`srv6-service-export-control.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/srv6-service-export-control.ci#L3) | functional/verify | revert, verified |
| positive | [`srv6-service-export-control.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/srv6-service-export-control.ci#L2) | functional/verify | revert, verified |

### [`RFC9252-3.2.1-6`](#rfc9252-3.2.1-6)

Arguments may be generally applicable for SIDs of only specific SRv6 Endpoint Behaviors (e.g., End.DT2M); therefore, the AL MUST be set to 0 for SIDs where the Argument is not applicable. (S3.2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) forbidden: advertising a non-zero AL for a behavior with no argument. (b) TestSRv6OriginRefusesArgumentsForDT4 configures End.DT4 with AL=16 and requires ParseRouteAttributes to fail; TestSRv6OriginWithoutArguments asserts AL octet 0 for End.DT4. validateSRv6OriginStructure allows AL only for End.DT2M (0x0018).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestSRv6OriginRefusesArgumentsForDT4`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L41) | unit/verify | unproven |
| positive | [`TestSRv6OriginWithoutArguments`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc9252_routeattr_srv6_test.go#L30) | unit/verify | unproven |

### [`RFC9252-5-3`](#rfc9252-5-3)

it indicates that the egress PE supports SRv6 overlay, and the BGP ingress PE receiving this route MUST perform IPv6 encapsulation and insert an SRH [RFC8754] when required (§5)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. (a) forbidden: an ingress PE forwarding an SRv6 service route without IPv6 encapsulation. TestSRv6ServiceSIDSelectsIPv6Encapsulation asserts SEG6 encap mode 1 with the SID for the kernel backend (buildRichRoute). The VPP backend (internal/plugins/fib/vpp/srv6.go) has no test. The negative-tagged TestRouteWithoutServiceSIDKeepsMPLSEncapsulation proves a route without a SID keeps MPLS, which is not a violation of this row.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRouteWithoutServiceSIDKeepsMPLSEncapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc9252_nexthop_srv6_linux_test.go#L39) | unit/verify | unproven |
| positive | [`TestSRv6ServiceSIDSelectsIPv6Encapsulation`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/rfc9252_nexthop_srv6_linux_test.go#L15) | unit/verify | unproven |

### [`RFC9252-7-1`](#rfc9252-7-1)

If multiple instances of the SRv6 L3 Service TLV are encountered, all but the first instance MUST be ignored. (S7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. TestRFC9252FirstServiceTLVWins asserts the first of two L3 Service TLVs that both carry a valid SID is used. ExtractSRv6SIDFull returns the first TLV that yields a valid SID, so when the first instance carries none (for example a SID Information Sub-TLV under 21 octets, or no SID Information Sub-TLV) the second instance's SID is used instead of being ignored. No assertion covers that input.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L32) | unit/verify | revert, verified |
| positive | [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L31) | unit/verify | revert, verified |

### [`RFC9252-7-2`](#rfc9252-7-2)

If multiple instances of the SRv6 L2 Service TLV are encountered, all but the first instance MUST be ignored. (S7)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Same unit and same gap as RFC9252-7-1, for the L2 Service TLV: the first instance wins only when it holds a valid SID; otherwise ExtractSRv6SIDFull falls through to the second instance, which Section 7 says MUST be ignored.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestRFC9252FirstServiceTLVWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/rfc9252_first_service_tlv_test.go#L33) | unit/verify | revert, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc9252.txt |
| Source fingerprint | dd9dd72a8f831935 |
| Record | rfc/extraction/rfc9252.json |
| Mapped sentences | 24 |
| Declined as scope | 12 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | walked | not stated |
| `1` | not stated | 1 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `2` | not stated | 3 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 4 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.2.1` | not stated | 7 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `5` | not stated | 2 | walked | not stated |
| `5.1` | not stated | 1 | walked | not stated |
| `5.2` | not stated | 1 | walked | not stated |
| `5.3` | not stated | 0 | walked | not stated |
| `5.4` | not stated | 0 | walked | not stated |
| `6` | not stated | 2 | walked | not stated |
| `6.1` | not stated | 0 | walked | not stated |
| `6.1.1` | not stated | 1 | walked | not stated |
| `6.1.2` | not stated | 1 | walked | not stated |
| `6.2` | not stated | 2 | walked | not stated |
| `6.2.1` | not stated | 1 | walked | not stated |
| `6.2.2` | not stated | 2 | walked | not stated |
| `6.3` | not stated | 1 | walked | not stated |
| `6.4` | not stated | 0 | walked | not stated |
| `6.5` | not stated | 1 | walked | not stated |
| `6.6` | not stated | 0 | walked | not stated |
| `7` | not stated | 5 | walked | not stated |
| `8` | not stated | 0 | walked | not stated |
| `8.1` | not stated | 0 | walked | not stated |
| `8.2` | not stated | 0 | walked | not stated |
| `8.3` | not stated | 0 | walked | not stated |
| `8.4` | not stated | 0 | walked | not stated |
| `8.5` | not stated | 0 | walked | not stated |
| `9` | not stated | 0 | walked | not stated |
| `9.1` | not stated | 0 | walked | not stated |
| `9.2` | not stated | 0 | walked | not stated |
| `9.3` | not stated | 1 | walked | not stated |
| `10` | not stated | 0 | walked | not stated |
| `10.1` | not stated | 0 | walked | not stated |
| `10.2` | not stated | 0 | walked | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction applicability statement about the deployment: it says which underlay nodes a segment list may name, and binds no behavior of a BGP implementation of this document. The obligation on an SRv6 data-plane node is RFC 8986's, not this document's. | The underlay nodes whose SRv6 SIDs are part of the SRH segment list MUST support the SRv6 data plane. |
| `5.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 5.2 states the IPv6 VPN case in the same words as the Section 5.1 IPv4 VPN case that RFC9252-4.1-1 maps. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 20 and less than or equal to the FL. |
| `6:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 repeats for EVPN the ingress-device encapsulation obligation Section 5 states for L3 services and RFC9252-5-3 maps. | Signaling of the SRv6 Service SID(s) serves two purposes -- first, it indicates that the BGP egress device supports SRv6 overlay, and the BGP ingress device receiving this route MUST perform IPv6 encapsulation and insert an SRH [RFC8754] when required; second, it indicates the value of the Service SID(s) to be used in the encapsulation. |
| `6:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6 repeats for EVPN the ingress-PE resolvability check Section 5 states and RFC9252-5-2 maps. | Therefore, the ingress PE MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix for the BGP best path computation. |
| `6.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.2 repeats the 24-bit and Function Length transposition bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second MAC/IP label field repeats the same 24-bit and Function Length bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.2.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.2.1 repeats the same 24-bit and Function Length bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.2.2:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.2.2 repeats the same 24-bit and Function Length bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.2.2:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The second label field of the MAC+IP route repeats the same bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.3:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.3 repeats the same 24-bit and Function Length bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `6.5:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Section 6.5 repeats the same 24-bit and Function Length bound word for word. | When using the Transposition Scheme, the Transposition Length MUST be less than or equal to 24 and less than or equal to the FL. |
| `9.3:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The sentence points the operator at Section 7 of RFC 8754 and at BCP 38 (RFC 2827) and BCP 84 (RFC 3704); the infrastructure ACL and ingress filtering obligations are stated in those documents, and this sentence adds none of its own. | As such, an operator deploying SRv6 MUST follow the considerations described in Section 7 of [RFC8754] to implement the infrastructure Access Control Lists (ACLs) and the recommendations described in BCP 38 [RFC2827] and BCP 84 [RFC3704]. |

## Superseded

No document obsoletes RFC 9252, so its obligations are stated where they were written.
