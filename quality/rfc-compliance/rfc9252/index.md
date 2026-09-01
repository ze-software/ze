# RFC 9252 - BGP Overlay Services Based on Segment Routing over IPv6 (SRv6)

Partial. Every requirement this repository extracted from RFC 9252, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 31.6% | 6 of 19 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 26.3% | 5 of 19 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 19 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 20 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 19 | of 24 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 19 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 42.1% | 8 of 19 binding obligations | no test carries the requirement id, whether or not a gap states why |

The 4 shares marked as a part above are the whole of the 19 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Requirements | 24 |
| Gated MUST-level | 19 |
| Obligations that bind Ze | 19 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 8 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 20 |
| Tagged units | 20 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9252.md` |
| Requirement shard | `rfc/requirements/rfc9252.md` |
| RFC text | `rfc/full/rfc9252.txt` |

## Enrolment

Enrolled: SRv6 BGP Overlay Services / Prefix-SID Service TLV (RFC 9252): receive-side SRv6 L3/L2 Service TLV codec. 6 MET (Service Reserved ignored, SID-Structure sum bound via errata 7817, no-valid-SID best-path ineligibility, next-hop-unchanged preserve + next-hop-changed strip of Prefix-SID, malformed-TLV treat-as-withdraw) + 5 single-polarity positive (sender zeroes Reserved/Flags x4, unknown endpoint-behavior extracted) + 8 gap (Transposition Length unbounded vs VPN/EVPN label + FL/AL, zero-offset/zero-scheme transposition constraints unenforced, no Endpoint-Behavior registry, ingress-PE Service-SID installed to FIB but no Section 5 resolvability check)

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Prefix-SID attribute (code 40) SRv6 L3/L2 Service TLV parse, SID Information Sub-TLV and SID Structure Sub-Sub-TLV extraction with the errata-7817 sum bound ([`internal/component/bgp/plugins/rib/pool/srv6sid.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid.go))
- Section 3.2.1 transposition undone for the IPv4 and IPv6 VPN families, whose Section 5.1 label field is read out of the NLRI the route is keyed by ([`internal/core/bgp/nlri/nlrisplit/transposition.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/transposition.go)) and merged back into the partial SID ([`internal/component/bgp/plugins/rib/rib_bestchange.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange.go), srv6SIDFromResult)
- malformed-Service-TLV treat-as-withdraw ([`internal/component/bgp/message/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go))
- no-valid-SID best-path ineligibility ([`internal/component/bgp/plugins/rib/rib_bestchange.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange.go))
- next-hop-change Prefix-SID strip vs next-hop-unchanged preserve ([`internal/component/bgp/reactor/peer_forward_facts.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_forward_facts.go))
- and SRv6 Prefix-SID config encode with zeroed reserved/flags octets ([`internal/component/bgp/config/routeattr_prefixsid.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_prefixsid.go)). Requirements bound per line in [`rfc/short/rfc9252.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9252.md).


**What the ledger says remains**

EVPN transposition is not implemented: Section 6 puts the label field at a different NLRI offset per route type, in the PMSI Tunnel Attribute for Route Type 3 and in the ESI Label extended community for Route Type 1 per-ES, and Route Type 2 has two label fields bound to different Service TLVs. An EVPN route whose Prefix-SID declares a transposition therefore yields no SID rather than the partial one. Eight MUST gaps annotated in [`rfc/short/rfc9252.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9252.md): [`RFC9252-4.1-1`](#rfc9252-4.1-1)/6.1-1/6.2-1 -- Transposition Length is now bounded against the 20-bit VPN and 24-bit EVPN label field widths but still not against FL or AL; [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1)/3.2.1-2 -- the zero-offset-when-length-zero and zero-when-scheme-not-applicable transposition constraints are not enforced; [`RFC9252-3.2-5`](#rfc9252-3.2-5)/3.2-6 -- ze keeps no SRv6 Endpoint Behavior registry, so it neither ignores SIDs with a non-zero Argument Length under an unknown behavior nor validates AL against a known behavior; and [`RFC9252-5-2`](#rfc9252-5-2) -- ze installs the received Service SID into the FIB (kernel SEG6 encap at [`internal/plugins/fib/kernel/nexthop_linux.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/nexthop_linux.go), VPP SR steering at [`internal/plugins/fib/vpp/srv6.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/vpp/srv6.go)) but performs no Section 5 resolvability check on the SID locator before best-path computation (isSRv6Ineligible gates on extraction validity only).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 13 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **19** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC9252-3.1-2`](#rfc9252-3.1-2), [`RFC9252-3.2.1-3`](#rfc9252-3.2.1-3), [`RFC9252-5-1`](#rfc9252-5-1), [`RFC9252-3.3-1`](#rfc9252-3.3-1), [`RFC9252-3.3-2`](#rfc9252-3.3-2), [`RFC9252-3.4-1`](#rfc9252-3.4-1)

**Annotated instead of tested (13):** [`RFC9252-3.1-1`](#rfc9252-3.1-1), [`RFC9252-3.2-1`](#rfc9252-3.2-1), [`RFC9252-3.2-2`](#rfc9252-3.2-2), [`RFC9252-3.2-3`](#rfc9252-3.2-3), [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1), [`RFC9252-3.2.1-2`](#rfc9252-3.2.1-2), [`RFC9252-4.1-1`](#rfc9252-4.1-1), [`RFC9252-6.1-1`](#rfc9252-6.1-1), [`RFC9252-6.2-1`](#rfc9252-6.2-1), [`RFC9252-3.2-4`](#rfc9252-3.2-4), [`RFC9252-3.2-5`](#rfc9252-3.2-5), [`RFC9252-3.2-6`](#rfc9252-3.2-6), [`RFC9252-5-2`](#rfc9252-5-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9252-3.1-1` | Service TLV Reserved field MUST be set to 0 by sender (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestParsePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L161). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes the Service TLV Reserved octet to 0 on encode and no code path emits a non-zero value, so there is no negative input to reject (internal/component/bgp/config/routeattr_prefixsid.go:339) |
| `RFC9252-3.1-2` | Service TLV Reserved field MUST be ignored by receiver (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestExtractSRv6SID_ServiceReservedZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L255). **negative:** `unit/verify` [`TestExtractSRv6SID_ServiceReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L269) |
| `RFC9252-3.2-1` | SID Information Sub-TLV RESERVED1 MUST be set to 0 (S3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestParsePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L162). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes RESERVED1 to 0 on encode with no non-zero path to reject (internal/component/bgp/config/routeattr_prefixsid.go:323) |
| `RFC9252-3.2-2` | SID Information Sub-TLV Service SID Flags MUST be set to 0 (S3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestParsePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L163). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes the Service SID Flags octet to 0 on encode with no non-zero path to reject (internal/component/bgp/config/routeattr_prefixsid.go:325) |
| `RFC9252-3.2-3` | SID Information Sub-TLV RESERVED2 MUST be set to 0 (S3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestParsePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L164). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes RESERVED2 to 0 on encode with no non-zero path to reject (internal/component/bgp/config/routeattr_prefixsid.go:325) |
| `RFC9252-3.2.1-1` | Transposition Offset MUST be 0 when Transposition Length is 0 (S3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseSIDStructure returns no-transposition when Transposition Length is 0 and never marks the SID invalid for a non-zero Transposition Offset, and ParsePrefixSIDSRv6 passes the configured structure through without enforcing offset 0 (internal/component/bgp/plugins/rib/pool/srv6sid.go:132) |
| `RFC9252-3.2.1-2` | Transposition Offset and Length MUST be 0 when Transposition Scheme is not applicable (S3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseSIDStructure has no family or label-field context, so it never enforces zero Transposition Offset and Length for SIDs advertised with routes where transposition does not apply (internal/component/bgp/plugins/rib/pool/srv6sid.go:106) |
| `RFC9252-3.2.1-3` | LBL+LNL+FL+AL MUST be <= 128 and >= Transposition Offset + Transposition Length (S3.2.1, errata 7817) | MUST | 3.2.1 | **positive:** `unit/verify` [`TestExtractSRv6SIDFull_WithTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L139). **negative:** `unit/verify` [`TestExtractSRv6SIDFull_InvalidSIDStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L217). **negative:** `unit/verify` [`TestExtractSRv6SIDFull_SumBelowTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L235) |
| `RFC9252-4.1-1` | IPv4/IPv6 VPN: Transposition Length MUST be <= 20 and <= FL (S4.1) | MUST | 4.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the 20-bit half is enforced -- srv6SIDFromResult refuses to reconstruct and isSRv6Ineligible makes the path ineligible when Transposition Length exceeds labelWidthForSAFI (internal/component/bgp/plugins/rib/rib_bestchange.go) -- but neither they nor parseSIDStructure bound it against the Function Length (internal/component/bgp/plugins/rib/pool/srv6sid.go) |
| `RFC9252-6.1-1` | EVPN ESI Label: Transposition Length MUST be <= 24 and <= AL (S6.1) | MUST | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the 24-bit half is enforced by labelWidthForSAFI through srv6SIDFromResult and isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go), but the Argument Length is never bounded, the ESI Label extended community that carries these bits is never read, and the EVPN encoder carries no SRv6 ESI-label SID (internal/component/bgp/plugins/rib/pool/srv6sid.go) |
| `RFC9252-6.2-1` | EVPN routes 2/3/5: Transposition Length MUST be <= 24 and <= FL (S6.2, S6.3, S6.4) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the 24-bit half is enforced by labelWidthForSAFI through srv6SIDFromResult and isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go), but the Function Length is never bounded and no EVPN label field is read at all -- TranspositionLabel answers only the VPN families, so an EVPN transposition yields no SID rather than a reconstructed one (internal/core/bgp/nlri/nlrisplit/transposition.go) |
| `RFC9252-3.2-4` | Unrecognized SRv6 Endpoint Behavior MUST NOT be considered invalid (unless involves arguments) (S3.2) | MUST NOT | 3.2 | **positive:** `unit/verify` [`TestExtractSRv6SID_UnknownEndpointBehavior`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L285). **negative:** no negative test. **{single-polarity}:** extractSIDFromServiceTLV extracts the SID without inspecting or validating the SRv6 Endpoint Behavior, so an unrecognized behavior is never rejected and there is no behavior-based rejection path to drive negatively (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| `RFC9252-3.2-5` | Receiver MUST ignore SRv6 SIDs with non-zero AL and unknown Endpoint Behaviors (S3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze maintains no SRv6 Endpoint Behavior registry and extractSIDFromServiceTLV returns the SID regardless of a non-zero Argument Length or an unknown behavior, so such SIDs are used rather than ignored (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| `RFC9252-3.2-6` | Receiver MUST validate AL consistency with known SRv6 Endpoint Behavior (S3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze has no SRv6 Endpoint Behavior definitions, so it never validates the Argument Length against a known behavior's expected argument size (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| `RFC9252-5-1` | Path with no valid SRv6 SID MUST be considered ineligible for best-path selection (S5) | MUST | 5 | **positive:** `unit/verify` [`TestIsSRv6Ineligible_ValidSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/srv6_ineligible_test.go#L72). **negative:** `unit/verify` [`TestIsSRv6Ineligible_InvalidSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/srv6_ineligible_test.go#L83). **negative:** `unit/verify` [`TestSRv6TranspositionWiderThanLabelFieldIsIneligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/srv6_transposition_test.go#L168) |
| `RFC9252-5-2` | Ingress PE MUST perform resolvability check for SRv6 Service SID before best-path computation (S5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze acts as an SRv6 ingress PE -- it extracts the received best-path Service SID (internal/component/bgp/plugins/rib/rib_bestchange.go:729,:882) and installs it into the FIB as a kernel SEG6 encap route (internal/plugins/fib/kernel/nexthop_linux.go:78) or a VPP SR steering policy (internal/plugins/fib/vpp/srv6.go:35) -- but performs no RFC 9252 Section 5 resolvability check: isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go:963) gates best-path on SID extraction validity only, never on locator reachability (no resolvability check exists in internal/component/bgp) |
| `RFC9252-3.3-1` | When next-hop unchanged, all Reserved fields MUST be propagated unchanged (S3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_forward_facts_test.go#L336). **negative:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_forward_facts_test.go#L339) |
| `RFC9252-3.3-2` | When next-hop changed, unrecognized Sub-TLVs and Sub-Sub-TLVs MUST be removed (S3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_forward_facts_test.go#L338). **negative:** `unit/verify` [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_forward_facts_test.go#L337) |
| `RFC9252-3.4-1` | treat-as-withdraw MUST be performed when at least one malformed SRv6 Service TLV is present (S3.4) | MUST | 3.4 | **positive:** `unit/verify` [`TestValidatePrefixSIDAttr_Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2397). **negative:** `unit/verify` [`TestValidateSRv6ServiceTLV_SIDInfoTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2466). **negative:** `unit/verify` [`TestValidateSRv6ServiceTLV_TrailingBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2426) |
| `RFC9252-3.2-7` | When multiple SRv6 SID Information Sub-TLVs present, ingress PE SHOULD use the first instance (S3.2) | SHOULD | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.3-3` | When next-hop unchanged, SRv6 Service TLVs SHOULD be propagated further (S3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.3-4` | When next-hop changed, TLVs/Sub-TLVs/Sub-Sub-TLVs SHOULD be updated with locally allocated SRv6 SID info (S3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.2-8` | Implementation MAY provide local policy to override SID selection (S3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.2-9` | Endpoint Behavior 0xFFFF MAY be used to abstract actual behavior (S3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1) Transposition Offset MUST be 0 when Transposition Length is 0 (S3.2.1) | {gap}, no test | parseSIDStructure returns no-transposition when Transposition Length is 0 and never marks the SID invalid for a non-zero Transposition Offset, and ParsePrefixSIDSRv6 passes the configured structure through without enforcing offset 0 (internal/component/bgp/plugins/rib/pool/srv6sid.go:132) |
| [`RFC9252-3.2.1-2`](#rfc9252-3.2.1-2) Transposition Offset and Length MUST be 0 when Transposition Scheme is not applicable (S3.2.1) | {gap}, no test | parseSIDStructure has no family or label-field context, so it never enforces zero Transposition Offset and Length for SIDs advertised with routes where transposition does not apply (internal/component/bgp/plugins/rib/pool/srv6sid.go:106) |
| [`RFC9252-4.1-1`](#rfc9252-4.1-1) IPv4/IPv6 VPN: Transposition Length MUST be <= 20 and <= FL (S4.1) | {gap}, no test | the 20-bit half is enforced -- srv6SIDFromResult refuses to reconstruct and isSRv6Ineligible makes the path ineligible when Transposition Length exceeds labelWidthForSAFI (internal/component/bgp/plugins/rib/rib_bestchange.go) -- but neither they nor parseSIDStructure bound it against the Function Length (internal/component/bgp/plugins/rib/pool/srv6sid.go) |
| [`RFC9252-6.1-1`](#rfc9252-6.1-1) EVPN ESI Label: Transposition Length MUST be <= 24 and <= AL (S6.1) | {gap}, no test | the 24-bit half is enforced by labelWidthForSAFI through srv6SIDFromResult and isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go), but the Argument Length is never bounded, the ESI Label extended community that carries these bits is never read, and the EVPN encoder carries no SRv6 ESI-label SID (internal/component/bgp/plugins/rib/pool/srv6sid.go) |
| [`RFC9252-6.2-1`](#rfc9252-6.2-1) EVPN routes 2/3/5: Transposition Length MUST be <= 24 and <= FL (S6.2, S6.3, S6.4) | {gap}, no test | the 24-bit half is enforced by labelWidthForSAFI through srv6SIDFromResult and isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go), but the Function Length is never bounded and no EVPN label field is read at all -- TranspositionLabel answers only the VPN families, so an EVPN transposition yields no SID rather than a reconstructed one (internal/core/bgp/nlri/nlrisplit/transposition.go) |
| [`RFC9252-3.2-5`](#rfc9252-3.2-5) Receiver MUST ignore SRv6 SIDs with non-zero AL and unknown Endpoint Behaviors (S3.2) | {gap}, no test | ze maintains no SRv6 Endpoint Behavior registry and extractSIDFromServiceTLV returns the SID regardless of a non-zero Argument Length or an unknown behavior, so such SIDs are used rather than ignored (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| [`RFC9252-3.2-6`](#rfc9252-3.2-6) Receiver MUST validate AL consistency with known SRv6 Endpoint Behavior (S3.2) | {gap}, no test | ze has no SRv6 Endpoint Behavior definitions, so it never validates the Argument Length against a known behavior's expected argument size (internal/component/bgp/plugins/rib/pool/srv6sid.go:84) |
| [`RFC9252-5-2`](#rfc9252-5-2) Ingress PE MUST perform resolvability check for SRv6 Service SID before best-path computation (S5) | {gap}, no test | ze acts as an SRv6 ingress PE -- it extracts the received best-path Service SID (internal/component/bgp/plugins/rib/rib_bestchange.go:729,:882) and installs it into the FIB as a kernel SEG6 encap route (internal/plugins/fib/kernel/nexthop_linux.go:78) or a VPP SR steering policy (internal/plugins/fib/vpp/srv6.go:35) -- but performs no RFC 9252 Section 5 resolvability check: isSRv6Ineligible (internal/component/bgp/plugins/rib/rib_bestchange.go:963) gates best-path on SID extraction validity only, never on locator reachability (no resolvability check exists in internal/component/bgp) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9252-3.1-1`](#rfc9252-3.1-1)

Service TLV Reserved field MUST be set to 0 by sender (S3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParsePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L161) | unit/verify | unproven |

### [`RFC9252-3.1-2`](#rfc9252-3.1-2)

Service TLV Reserved field MUST be ignored by receiver (S3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtractSRv6SID_ServiceReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L269) | unit/verify | unproven |
| positive | [`TestExtractSRv6SID_ServiceReservedZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L255) | unit/verify | unproven |

### [`RFC9252-3.2-1`](#rfc9252-3.2-1)

SID Information Sub-TLV RESERVED1 MUST be set to 0 (S3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParsePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L162) | unit/verify | unproven |

### [`RFC9252-3.2-2`](#rfc9252-3.2-2)

SID Information Sub-TLV Service SID Flags MUST be set to 0 (S3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParsePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L163) | unit/verify | unproven |

### [`RFC9252-3.2-3`](#rfc9252-3.2-3)

SID Information Sub-TLV RESERVED2 MUST be set to 0 (S3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestParsePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L164) | unit/verify | unproven |

### [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1)

Transposition Offset MUST be 0 when Transposition Length is 0 (S3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-3.2.1-1, so no unit is bound to it.

### [`RFC9252-3.2.1-2`](#rfc9252-3.2.1-2)

Transposition Offset and Length MUST be 0 when Transposition Scheme is not applicable (S3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-3.2.1-2, so no unit is bound to it.

### [`RFC9252-3.2.1-3`](#rfc9252-3.2.1-3)

LBL+LNL+FL+AL MUST be <= 128 and >= Transposition Offset + Transposition Length (S3.2.1, errata 7817)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtractSRv6SIDFull_InvalidSIDStructure`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L217) | unit/verify | unproven |
| negative | [`TestExtractSRv6SIDFull_SumBelowTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L235) | unit/verify | unproven |
| positive | [`TestExtractSRv6SIDFull_WithTransposition`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L139) | unit/verify | unproven |

### [`RFC9252-4.1-1`](#rfc9252-4.1-1)

IPv4/IPv6 VPN: Transposition Length MUST be <= 20 and <= FL (S4.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-4.1-1, so no unit is bound to it.

### [`RFC9252-6.1-1`](#rfc9252-6.1-1)

EVPN ESI Label: Transposition Length MUST be <= 24 and <= AL (S6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-6.1-1, so no unit is bound to it.

### [`RFC9252-6.2-1`](#rfc9252-6.2-1)

EVPN routes 2/3/5: Transposition Length MUST be <= 24 and <= FL (S6.2, S6.3, S6.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-6.2-1, so no unit is bound to it.

### [`RFC9252-3.2-4`](#rfc9252-3.2-4)

Unrecognized SRv6 Endpoint Behavior MUST NOT be considered invalid (unless involves arguments) (S3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExtractSRv6SID_UnknownEndpointBehavior`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L285) | unit/verify | unproven |

### [`RFC9252-3.2-5`](#rfc9252-3.2-5)

Receiver MUST ignore SRv6 SIDs with non-zero AL and unknown Endpoint Behaviors (S3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-3.2-5, so no unit is bound to it.

### [`RFC9252-3.2-6`](#rfc9252-3.2-6)

Receiver MUST validate AL consistency with known SRv6 Endpoint Behavior (S3.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-3.2-6, so no unit is bound to it.

### [`RFC9252-5-1`](#rfc9252-5-1)

Path with no valid SRv6 SID MUST be considered ineligible for best-path selection (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestIsSRv6Ineligible_InvalidSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/srv6_ineligible_test.go#L83) | unit/verify | unproven |
| negative | [`TestSRv6TranspositionWiderThanLabelFieldIsIneligible`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/srv6_transposition_test.go#L168) | unit/verify | unproven |
| positive | [`TestIsSRv6Ineligible_ValidSID`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/srv6_ineligible_test.go#L72) | unit/verify | unproven |

### [`RFC9252-5-2`](#rfc9252-5-2)

Ingress PE MUST perform resolvability check for SRv6 Service SID before best-path computation (S5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-5-2, so no unit is bound to it.

### [`RFC9252-3.3-1`](#rfc9252-3.3-1)

When next-hop unchanged, all Reserved fields MUST be propagated unchanged (S3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_forward_facts_test.go#L339) | unit/verify | unproven |
| positive | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_forward_facts_test.go#L336) | unit/verify | unproven |

### [`RFC9252-3.3-2`](#rfc9252-3.3-2)

When next-hop changed, unrecognized Sub-TLVs and Sub-Sub-TLVs MUST be removed (S3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_forward_facts_test.go#L337) | unit/verify | unproven |
| positive | [`TestPrefixSIDPropagationNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_forward_facts_test.go#L338) | unit/verify | unproven |

### [`RFC9252-3.4-1`](#rfc9252-3.4-1)

treat-as-withdraw MUST be performed when at least one malformed SRv6 Service TLV is present (S3.4)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateSRv6ServiceTLV_SIDInfoTooShort`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2466) | unit/verify | unproven |
| negative | [`TestValidateSRv6ServiceTLV_TrailingBytes`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2426) | unit/verify | unproven |
| positive | [`TestValidatePrefixSIDAttr_Valid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606_test.go#L2397) | unit/verify | unproven |

## Extraction sign-off

No extraction sign-off exists for RFC 9252, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9252, so its obligations are stated where they were written.
