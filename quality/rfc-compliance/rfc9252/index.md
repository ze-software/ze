# RFC 9252 - BGP Overlay Services Based on Segment Routing over IPv6 (SRv6)

Partial. Every requirement this repository extracted from RFC 9252, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 24.0% | 6 of 25 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 20.0% | 5 of 25 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 25 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 20.0% | 4 of 20 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

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
| No test at all | 56.0% | 14 of 25 gated MUSTs | no test carries the requirement id, whether or not a gap states why |

The 7 shares marked as a part above are the whole of the 25 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Partial |
| Enrolment | Enrolled |
| Requirements | 30 |
| Gated MUST-level | 25 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 8 |
| Gated with no test | 6 |
| Nightly-only evidence | 0 |
| Test tags | 20 |
| Tagged units | 20 |
| Recorded audit verdicts | 0 |
| Discrimination records | 4 |
| Summary | `rfc/short/rfc9252.md` |
| Requirement shard | `rfc/requirements/rfc9252.md` |
| RFC text | `rfc/full/rfc9252.txt` |

## Enrolment

Enrolled: SRv6 BGP Overlay Services / Prefix-SID Service TLV (RFC 9252): receive-side SRv6 L3/L2 Service TLV codec. 6 MET (Service Reserved ignored, SID-Structure sum bound via errata 7817, no-valid-SID best-path ineligibility, next-hop-unchanged preserve + next-hop-changed strip of Prefix-SID, malformed-TLV treat-as-withdraw) + 5 single-polarity positive (sender zeroes Reserved/Flags x4, unknown endpoint-behavior extracted) + 8 gap (Transposition Length unbounded vs VPN/EVPN label + FL/AL, zero-offset/zero-scheme transposition constraints unenforced, no Endpoint-Behavior registry, ingress-PE Service-SID installed to FIB but no Section 5 resolvability check). The 2026-09-21 extraction walk read the document against this checklist and changed two things. It corrected the section reference on 21 rows, which carried the draft's numbering (S3.1, S3.2, S3.3, S3.4, S4.1, S5, S6.1, S6.2) rather than the published one: the Service TLV Reserved field is Section 2, the SID Information Sub-TLV is Section 3.1, the argument-validation obligations are Section 3.2.1, the VPN transposition bound is Sections 5.1 and 5.2, the EVPN bounds are Sections 6.1.1 through 6.5, and error handling is Section 7. And it added 6 MUST rows the checklist did not carry: RFC9252-3.2.1-4 (shifted-out bits zeroed in the SID value), RFC9252-3.2.1-5 (per-neighbor and per-service advertisement control), RFC9252-3.2.1-6 (AL zero where the Argument does not apply), RFC9252-5-3 (ingress PE encapsulates in IPv6 and inserts an SRH when required), and RFC9252-7-1 and -7-2 (a second SRv6 L3 or L2 Service TLV is ignored). None of the 6 carries a tagged test, so each is an open gap.

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

- Prefix-SID attribute (code 40) SRv6 L3/L2 Service TLV parse, SID Information Sub-TLV and SID Structure Sub-Sub-TLV extraction with the errata-7817 sum bound ([`internal/component/bgp/plugins/rib/pool/srv6sid.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid.go))
- Section 3.2.1 transposition undone for the IPv4 and IPv6 VPN families, whose Section 5.1 label field is read out of the NLRI the route is keyed by ([`internal/core/bgp/nlri/nlrisplit/transposition.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/nlrisplit/transposition.go)) and merged back into the partial SID ([`internal/component/bgp/plugins/rib/rib_bestchange.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange.go), srv6SIDFromResult)
- malformed-Service-TLV treat-as-withdraw ([`internal/component/bgp/message/rfc7606.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/rfc7606.go))
- no-valid-SID best-path ineligibility ([`internal/component/bgp/plugins/rib/rib_bestchange.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_bestchange.go))
- next-hop-change Prefix-SID strip vs next-hop-unchanged preserve ([`internal/component/bgp/reactor/peer_forward_facts.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/peer_forward_facts.go))
- and SRv6 Prefix-SID config encode with zeroed reserved/flags octets ([`internal/core/bgp/attribute/prefixsid.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/prefixsid.go)). Requirements bound per line in [`rfc/short/rfc9252.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9252.md).


**What the ledger says remains**

EVPN transposition is not implemented: Section 6 puts the label field at a different NLRI offset per route type, in the PMSI Tunnel Attribute for Route Type 3 and in the ESI Label extended community for Route Type 1 per-ES, and Route Type 2 has two label fields bound to different Service TLVs. An EVPN route whose Prefix-SID declares a transposition therefore yields no SID rather than the partial one. Eight MUST gaps annotated in [`rfc/short/rfc9252.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9252.md): [`RFC9252-4.1-1`](#rfc9252-4.1-1)/6.1-1/6.2-1 -- Transposition Length is now bounded against the 20-bit VPN and 24-bit EVPN label field widths but still not against FL or AL; [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1)/3.2.1-2 -- the zero-offset-when-length-zero and zero-when-scheme-not-applicable transposition constraints are not enforced; [`RFC9252-3.2-5`](#rfc9252-3.2-5)/3.2-6 -- ze keeps no SRv6 Endpoint Behavior registry, so it neither ignores SIDs with a non-zero Argument Length under an unknown behavior nor validates AL against a known behavior; and [`RFC9252-5-2`](#rfc9252-5-2) -- ze installs the received Service SID into the FIB (kernel SEG6 encap at [`internal/plugins/fib/kernel/nexthop_linux.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/kernel/nexthop_linux.go), VPP SR steering at [`internal/plugins/fib/vpp/srv6.go`](https://github.com/ze-software/ze/blob/main/internal/plugins/fib/vpp/srv6.go)) but performs no Section 5 resolvability check on the SID locator before best-path computation (isSRv6Ineligible gates on extraction validity only). Six further MUST gaps were added by the 2026-09-21 extraction walk and none is proven: [`RFC9252-3.2.1-4`](#rfc9252-3.2.1-4), [`RFC9252-3.2.1-5`](#rfc9252-3.2.1-5), [`RFC9252-3.2.1-6`](#rfc9252-3.2.1-6), [`RFC9252-5-3`](#rfc9252-5-3), [`RFC9252-7-1`](#rfc9252-7-1) and [`RFC9252-7-2`](#rfc9252-7-2), so the count of MUST gaps on this page is fourteen, not eight.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 6 | one part of the gated population |
| Annotated instead of tested | 13 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 6 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **25** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (6):** [`RFC9252-3.1-2`](#rfc9252-3.1-2), [`RFC9252-3.2.1-3`](#rfc9252-3.2.1-3), [`RFC9252-5-1`](#rfc9252-5-1), [`RFC9252-3.3-1`](#rfc9252-3.3-1), [`RFC9252-3.3-2`](#rfc9252-3.3-2), [`RFC9252-3.4-1`](#rfc9252-3.4-1)

**Annotated instead of tested (13):** [`RFC9252-3.1-1`](#rfc9252-3.1-1), [`RFC9252-3.2-1`](#rfc9252-3.2-1), [`RFC9252-3.2-2`](#rfc9252-3.2-2), [`RFC9252-3.2-3`](#rfc9252-3.2-3), [`RFC9252-3.2.1-1`](#rfc9252-3.2.1-1), [`RFC9252-3.2.1-2`](#rfc9252-3.2.1-2), [`RFC9252-4.1-1`](#rfc9252-4.1-1), [`RFC9252-6.1-1`](#rfc9252-6.1-1), [`RFC9252-6.2-1`](#rfc9252-6.2-1), [`RFC9252-3.2-4`](#rfc9252-3.2-4), [`RFC9252-3.2-5`](#rfc9252-3.2-5), [`RFC9252-3.2-6`](#rfc9252-3.2-6), [`RFC9252-5-2`](#rfc9252-5-2)

**No test and no annotation (6):** [`RFC9252-3.2.1-4`](#rfc9252-3.2.1-4), [`RFC9252-3.2.1-5`](#rfc9252-3.2.1-5), [`RFC9252-3.2.1-6`](#rfc9252-3.2.1-6), [`RFC9252-5-3`](#rfc9252-5-3), [`RFC9252-7-1`](#rfc9252-7-1), [`RFC9252-7-2`](#rfc9252-7-2)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9252-3.1-1` | Service TLV Reserved field MUST be set to 0 by sender (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L163). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes the Service TLV Reserved octet to 0 on encode and no code path emits a non-zero value, so there is no negative input to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC9252-3.1-2` | Service TLV Reserved field MUST be ignored by receiver (S3.1) | MUST | 3.1 | **positive:** `unit/verify` [`TestExtractSRv6SID_ServiceReservedZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L255). **negative:** `unit/verify` [`TestExtractSRv6SID_ServiceReservedIgnored`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/pool/srv6sid_test.go#L269) |
| `RFC9252-3.2-1` | SID Information Sub-TLV RESERVED1 MUST be set to 0 (S3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L164). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes RESERVED1 to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC9252-3.2-2` | SID Information Sub-TLV Service SID Flags MUST be set to 0 (S3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L165). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes the Service SID Flags octet to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
| `RFC9252-3.2-3` | SID Information Sub-TLV RESERVED2 MUST be set to 0 (S3.2) | MUST | 3.2 | **positive:** `unit/verify` [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L166). **negative:** no negative test. **{single-polarity}:** ParsePrefixSIDSRv6 hardcodes RESERVED2 to 0 on encode with no non-zero path to reject (internal/core/bgp/attribute/prefixsid.go) |
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
| `RFC9252-3.2.1-4` | When the Transposition Scheme is used, "The bits that have been shifted out MUST be set to 0 in the SID value" (S3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.2.1-5` | "Implementations supporting this specification MUST provide a mechanism to control the advertisement of SRv6-based BGP service routes on a per-neighbor and per-service basis" (S3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-3.2.1-6` | "the AL MUST be set to 0 for SIDs where the Argument is not applicable" (S3.2.1) | MUST | 3.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-5-3` | The SRv6 Service TLV "indicates that the egress PE supports SRv6 overlay, and the BGP ingress PE receiving this route MUST perform IPv6 encapsulation and insert an SRH [RFC8754] when required" (S5) | MUST | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-7-1` | "If multiple instances of the SRv6 L3 Service TLV are encountered, all but the first instance MUST be ignored" (S7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9252-7-2` | "If multiple instances of the SRv6 L2 Service TLV are encountered, all but the first instance MUST be ignored" (S7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test |
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
| [`RFC9252-3.2.1-4`](#rfc9252-3.2.1-4) When the Transposition Scheme is used, "The bits that have been shifted out MUST be set to 0 in the SID value" (S3.2.1) | no test | no test carries this requirement id |
| [`RFC9252-3.2.1-5`](#rfc9252-3.2.1-5) "Implementations supporting this specification MUST provide a mechanism to control the advertisement of SRv6-based BGP service routes on a per-neighbor and per-service basis" (S3.2.1) | no test | no test carries this requirement id |
| [`RFC9252-3.2.1-6`](#rfc9252-3.2.1-6) "the AL MUST be set to 0 for SIDs where the Argument is not applicable" (S3.2.1) | no test | no test carries this requirement id |
| [`RFC9252-5-3`](#rfc9252-5-3) The SRv6 Service TLV "indicates that the egress PE supports SRv6 overlay, and the BGP ingress PE receiving this route MUST perform IPv6 encapsulation and insert an SRH [RFC8754] when required" (S5) | no test | no test carries this requirement id |
| [`RFC9252-7-1`](#rfc9252-7-1) "If multiple instances of the SRv6 L3 Service TLV are encountered, all but the first instance MUST be ignored" (S7) | no test | no test carries this requirement id |
| [`RFC9252-7-2`](#rfc9252-7-2) "If multiple instances of the SRv6 L2 Service TLV are encountered, all but the first instance MUST be ignored" (S7) | no test | no test carries this requirement id |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9252-3.1-1`](#rfc9252-3.1-1)

Service TLV Reserved field MUST be set to 0 by sender (S3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L163) | unit/verify | revert, verified |

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
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L164) | unit/verify | revert, verified |

### [`RFC9252-3.2-2`](#rfc9252-3.2-2)

SID Information Sub-TLV Service SID Flags MUST be set to 0 (S3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L165) | unit/verify | revert, verified |

### [`RFC9252-3.2-3`](#rfc9252-3.2-3)

SID Information Sub-TLV RESERVED2 MUST be set to 0 (S3.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestEncodePrefixSIDSRv6_ReservedFieldsZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/routeattr_test.go#L166) | unit/verify | revert, verified |

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

### [`RFC9252-3.2.1-4`](#rfc9252-3.2.1-4)

When the Transposition Scheme is used, "The bits that have been shifted out MUST be set to 0 in the SID value" (S3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-3.2.1-4, so no unit is bound to it.

### [`RFC9252-3.2.1-5`](#rfc9252-3.2.1-5)

"Implementations supporting this specification MUST provide a mechanism to control the advertisement of SRv6-based BGP service routes on a per-neighbor and per-service basis" (S3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-3.2.1-5, so no unit is bound to it.

### [`RFC9252-3.2.1-6`](#rfc9252-3.2.1-6)

"the AL MUST be set to 0 for SIDs where the Argument is not applicable" (S3.2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-3.2.1-6, so no unit is bound to it.

### [`RFC9252-5-3`](#rfc9252-5-3)

The SRv6 Service TLV "indicates that the egress PE supports SRv6 overlay, and the BGP ingress PE receiving this route MUST perform IPv6 encapsulation and insert an SRH [RFC8754] when required" (S5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-5-3, so no unit is bound to it.

### [`RFC9252-7-1`](#rfc9252-7-1)

"If multiple instances of the SRv6 L3 Service TLV are encountered, all but the first instance MUST be ignored" (S7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-7-1, so no unit is bound to it.

### [`RFC9252-7-2`](#rfc9252-7-2)

"If multiple instances of the SRv6 L2 Service TLV are encountered, all but the first instance MUST be ignored" (S7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9252-7-2, so no unit is bound to it.

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
