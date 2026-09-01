# RFC 9256 - Segment Routing Policy Architecture

Partial. Every requirement this repository extracted from RFC 9256, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 11.1% | 1 of 9 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 22.2% | 2 of 9 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Proven by a recorded break | 0.0% | 0 of 4 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 22 | of 68 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 13 | of 22 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| No test at all | 66.7% | 6 of 9 binding obligations | no test carries the requirement id, whether or not a gap states why |

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
| Requirements | 68 |
| Gated MUST-level | 22 |
| Obligations that bind Ze | 9 |
| Not applicable, so out of scope | 13 |
| Declared gaps | 6 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tagged units | 4 |
| Recorded audit verdicts | 0 |
| Discrimination records | 0 |
| Summary | `rfc/short/rfc9256.md` |
| Requirement shard | `rfc/requirements/rfc9256.md` |
| RFC text | `rfc/full/rfc9256.txt` |

## Enrolment

Enrolled: Segment Routing Policy Architecture

## What the public ledger says

**Status:** Partial

**What the ledger says is covered**

Ze is an SR Policy originator, not a headend: it builds the SR Policy identification tuple into the NLRI key ([`internal/component/bgp/plugins/nlri/srpolicy/types.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/types.go)), parses it back (types.go), and encodes a candidate path's preference, priority, MPLS and SRv6 binding SID, weighted segment lists of Type A and Type B segments, and the policy and candidate-path names into the Tunnel Encapsulation attribute (config.go). Symbolic names stay out of the NLRI key, and the segment encoder admits only Type A and Type B. Requirements bound per line in [`rfc/short/rfc9256.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9256.md).

**What the ledger says remains**

Six MUST gaps annotated in [`rfc/short/rfc9256.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc9256.md), all from ze holding no SR Policy state: [`RFC9256-2.6-1`](#rfc9256-2.6-1) -- the only candidate-path identity is the RFC 9830 distinguisher in the NLRI key and no candidate-path store resolves add, delete or modify; [`RFC9256-4-6`](#rfc9256-4-6) -- no code turns a segment list into a label stack or an SRv6 SID list, and a received Tunnel Encapsulation attribute stays raw TLV bytes with only the Preference sub-TLV decoded ([`internal/core/bgp/attribute/tunnel_encap.go`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/attribute/tunnel_encap.go)); [`RFC9256-5.1-2`](#rfc9256-5.1-2) and [`RFC9256-5.1-5`](#rfc9256-5.1-5) -- parseSegmentList determines no segment-list validity (empty list, weight 0 and mixed SR-MPLS/SRv6 lists are all accepted) and ze resolves no SID, so first-SID reachability is never established; and [`RFC9256-6.1-3`](#rfc9256-6.1-3) and [`RFC9256-6.2-4`](#rfc9256-6.2-4) -- the binding SID is encoded from configuration with no allocation table, no SRLB availability check and no alert. Headend obligations (composite candidate paths, Originator sub-TLV, candidate-path selection, active-path forwarding, dynamic candidate paths, Specified-BSID-only, policy state reporting) are annotated not-applicable: ze instantiates no policy.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the gated population |
| Annotated instead of tested | 21 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **22** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (1):** [`RFC9256-2.6-3`](#rfc9256-2.6-3)

**Annotated instead of tested (21):** [`RFC9256-2.1-1`](#rfc9256-2.1-1), [`RFC9256-2.1-2`](#rfc9256-2.1-2), [`RFC9256-2.1-3`](#rfc9256-2.1-3), [`RFC9256-2.2-1`](#rfc9256-2.2-1), [`RFC9256-2.2-2`](#rfc9256-2.2-2), [`RFC9256-2.2-3`](#rfc9256-2.2-3), [`RFC9256-2.4-1`](#rfc9256-2.4-1), [`RFC9256-2.4-2`](#rfc9256-2.4-2), [`RFC9256-2.6-1`](#rfc9256-2.6-1), [`RFC9256-2.9-1`](#rfc9256-2.9-1), [`RFC9256-2.11-1`](#rfc9256-2.11-1), [`RFC9256-4-6`](#rfc9256-4-6), [`RFC9256-5.1-2`](#rfc9256-5.1-2), [`RFC9256-5.1-4`](#rfc9256-5.1-4), [`RFC9256-5.1-5`](#rfc9256-5.1-5), [`RFC9256-5.2-1`](#rfc9256-5.2-1), [`RFC9256-6.1-3`](#rfc9256-6.1-3), [`RFC9256-6.2-4`](#rfc9256-6.2-4), [`RFC9256-6.2.3-3`](#rfc9256-6.2.3-3), [`RFC9256-6.2.3-4`](#rfc9256-6.2.3-4), [`RFC9256-7-1`](#rfc9256-7-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9256-2.1-1` | An SR Policy MUST be identified through the tuple <Headend, Color, Endpoint> (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** the three-element tuple is headend-scoped identification, and ze holds no headend. SRPolicy carries distinguisher, color, endpoint and afi and no headend field (internal/component/bgp/plugins/nlri/srpolicy/types.go:35-40), and grep -rni headend --include=*.go over internal/ and pkg/ matches only internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go. What ze does implement -- identification by <Color, Endpoint> within one headend's context -- is the separate RFC9256-2.1-2, which the NLRI key does satisfy; this line is the same not-applicable shape as RFC9256-2.1-3, RFC9256-2.9-1 and RFC9256-7-1, which already record that ze instantiates no policy at a headend |
| `RFC9256-2.1-2` | In the context of a specific headend, an SR Policy MUST be identified by the <Color, Endpoint> tuple (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC9256IdentificationTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L48). **negative:** no negative test. **{single-polarity}:** every <Color, Endpoint> pair identifies a policy, so no conforming input exists that identification must REFUSE, and a counter-pole would have to assert a non-identification that the requirement never mandates. The discrimination against a key that ignores its inputs is inside the positive test: a different color and a different endpoint each change the key, while a changed preference does not |
| `RFC9256-2.1-3` | The headend is specified as an IPv4 or IPv6 address and MUST resolve to a unique node in the SR domain (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze originates and parses SR Policy NLRI but instantiates no policy at a headend; grep for headend or head-end across internal/component/bgp and internal/component/mpls matches nothing, and the SR Policy plugin carries no headend field to specify or resolve (internal/component/bgp/plugins/nlri/srpolicy/types.go:35) |
| `RFC9256-2.2-1` | The endpoints of the constituent SR Policies and the parent SR Policy MUST be identical (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** composite candidate paths have no producer; grep -rniE composite over internal, pkg and cmd matches only report subjects and capability sub-components, and parseConfigRoute accepts no constituent-policy keyword (internal/component/bgp/plugins/nlri/srpolicy/config.go:72) |
| `RFC9256-2.2-2` | The colors of each of the constituent SR Policies and the parent SR Policy MUST be different (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** composite candidate paths have no producer; parseConfigRoute's keyword switch has no constituent-policy or composite spelling, so ze never forms a parent policy whose constituents could share a color (internal/component/bgp/plugins/nlri/srpolicy/config.go:72) |
| `RFC9256-2.2-3` | The constituent SR Policies MUST NOT use composite candidate paths (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** composite candidate paths have no producer; grep -rniE composite over internal, pkg and cmd finds no SR Policy match, so ze forms no constituent policy that could carry one (internal/component/bgp/plugins/nlri/srpolicy/config.go:72) |
| `RFC9256-2.4-1` | For the Originator ASN, if 2-byte ASNs are in use the low-order 16 bits MUST be used and the high-order bits MUST be set to 0 (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze encodes no Originator sub-TLV; the SR Policy sub-TLV constant set is preference, binding-sid, priority, srv6-binding-sid, segment-list and the two name sub-TLVs only, and grep for ProtocolOrigin or OriginatorASN over internal, pkg and cmd matches nothing (internal/component/bgp/plugins/nlri/srpolicy/config.go:23) |
| `RFC9256-2.4-2` | For the Originator node address, IPv4 addresses MUST be encoded in the lowest 32 bits and the high-order bits MUST be set to 0 (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze encodes no Originator sub-TLV, so it writes no originator node address; buildTunnelEncap emits only the preference, BSID, priority, segment-list and name sub-TLVs (internal/component/bgp/plugins/nlri/srpolicy/config.go:339) |
| `RFC9256-2.6-1` | The identity of a candidate path MUST be uniquely established in the context of an SR Policy <Headend, Color, Endpoint> to handle add, delete, or modify operations unambiguously (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the only candidate-path identity ze produces is the RFC 9830 distinguisher in the NLRI key, written by (*SRPolicy).WriteTo (internal/component/bgp/plugins/nlri/srpolicy/types.go:91); ze keeps no candidate-path store, so an add, delete or modify is never resolved against a candidate-path identity, and neither a protocol origin nor an originator is carried |
| `RFC9256-2.6-3` | Symbolic candidate-path names MUST NOT be considered as identifiers for a candidate path (§2.6) | MUST NOT | 2.6 | **positive:** `unit/verify` [`TestRFC9256SymbolicNamesAreNotIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L106). **negative:** `unit/verify` [`TestRFC9256SymbolicNamesAreNotIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L117) |
| `RFC9256-2.9-1` | Whenever a new path is learned, an active path is deleted, an existing path's validity changes, or an existing path is changed, the candidate-path selection process MUST be re-executed (§2.9) | MUST | 2.9 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze runs no candidate-path selection; there is no candidate-path store and no preference comparison anywhere (grep for protocol-origin or active candidate over internal matches nothing), and the SR Policy plugin registers only an NLRI codec and a config route encoder (internal/component/bgp/plugins/nlri/srpolicy/register.go:29) |
| `RFC9256-2.11-1` | Only the active candidate path MUST be used for forwarding traffic steered onto that policy, except in scenarios such as fast reroute where a backup candidate path may be used (§2.11) | MUST | 2.11 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** no SR Policy reaches forwarding; SAFI 73 appears only in the family registry and the NLRI codec (internal/core/family/family.go:93, internal/component/bgp/plugins/nlri/srpolicy/types.go:74), and grep for SAFISRPolicy over internal/plugins/fib and internal/component/resolve matches nothing |
| `RFC9256-4-6` | When building the label stack or SRv6 SID list, the node instantiating the policy MUST interpret the first segment as the topmost MPLS label / first SRv6 SID and the last segment as the bottommost label / last SID (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code converts an SR Policy segment list into a label stack or an SRv6 SID list; buildSegmentListSubTLV only serializes the configured segments in their configured order (internal/component/bgp/plugins/nlri/srpolicy/config.go:428), and a received Tunnel Encapsulation attribute is kept as raw TLV bytes of which only the Preference sub-TLV is ever decoded (internal/core/bgp/attribute/tunnel_encap.go:39 and :137) |
| `RFC9256-5.1-2` | A segment list of an explicit candidate path MUST be declared invalid when it is empty, its weight is 0, it mixes SR-MPLS and SRv6 segment types, the headend cannot resolve the first SID to outgoing interface(s)/next-hop(s), it cannot resolve any non-first SID of type C-K to an MPLS label or SRv6 SID, or verification fails for any SID for which verification was requested (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseSegmentList builds a segment list with no validity determination; it accepts a list with zero segments, accepts weight 0, and appends Type A and Type B segments into one list without a data-plane check (internal/component/bgp/plugins/nlri/srpolicy/config.go:267), and ze resolves no SID to an outgoing interface or next hop |
| `RFC9256-5.1-4` | Types A or B MUST be used for the SIDs whose reachability cannot be verified (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC9256SegmentTypesAreAOrB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L130). **negative:** no negative test. **{single-polarity}:** the segment encoder accepts only type-a and type-b and returns an error for every other segment spelling (internal/component/bgp/plugins/nlri/srpolicy/config.go:276), so every SID ze emits already carries its value outright; ze holds no SID verification state whose failure could produce a violating segment to reject |
| `RFC9256-5.1-5` | The first SID MUST always be reachable regardless of its type (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze resolves no SID; the first segment of a segment list is encoded straight from configuration with no reachability lookup (internal/component/bgp/plugins/nlri/srpolicy/config.go:276), and grep for SAFISRPolicy over internal/component/resolve and internal/plugins/fib matches nothing, so first-SID reachability is never established |
| `RFC9256-5.2-1` | If no solution is found to the optimization objective and constraints, the dynamic candidate path MUST be declared invalid (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze signals only explicit segment lists and has no dynamic candidate path; parseSegmentList takes literal MPLS labels and SRv6 SIDs (internal/component/bgp/plugins/nlri/srpolicy/config.go:276) and grep for OptimizationObjective over internal, pkg and cmd matches nothing |
| `RFC9256-6.1-3` | Candidate paths of different SR Policies MUST NOT have the same BSID (§6.1) | MUST NOT | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** buildBindingSIDSubTLV encodes whatever label the configuration names (internal/component/bgp/plugins/nlri/srpolicy/config.go:381) and ze keeps no BSID allocation table, so two SR Policies configured with the same BSID are both encoded and advertised |
| `RFC9256-6.2-4` | When the specified BSID is not available (optionally not in the SRLB), an alert message MUST be generated via mechanisms like syslog (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze performs no BSID availability check; buildBindingSIDSubTLV encodes the configured label unconditionally (internal/component/bgp/plugins/nlri/srpolicy/config.go:381) and the only SRLB allocator in ze belongs to OSPF Adj-SIDs (internal/plugins/ospf/sr_adjsid.go:115), which no SR Policy code consults, so no alert is raised |
| `RFC9256-6.2.3-3` | Under Specified-BSID-only behavior, when a candidate path has an unspecified or unavailable BSID it is considered invalid and an alert MUST be triggered via mechanisms like syslog (§6.2.3) | MUST | 6.2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no Specified-BSID-only behavior; grep for specified-bsid or SpecifiedBSID over internal, pkg and cmd matches nothing and parseConfigRoute's keyword switch has no spelling for it (internal/component/bgp/plugins/nlri/srpolicy/config.go:72) |
| `RFC9256-6.2.3-4` | Under Specified-BSID-only behavior, other candidate paths MUST then be evaluated for becoming the active candidate path (§6.2.3) | MUST | 6.2.3 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze implements no Specified-BSID-only behavior and no candidate-path selection, so there is no evaluation of other candidate paths to perform; the SR Policy plugin registers only an NLRI codec and a config route encoder (internal/component/bgp/plugins/nlri/srpolicy/register.go:29) |
| `RFC9256-7-1` | The SR Policy state MUST also reflect the reason when a policy and/or its candidate path is not active due to validation errors or not being preferred (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze keeps no SR Policy state; grep for SRPolicyState or policyState over internal, pkg and cmd matches nothing, grep for sr-policy over internal/component/cli matches nothing, and an SR Policy route is an ordinary BGP NLRI carried in the RIB (internal/component/bgp/plugins/nlri/srpolicy/register.go:29) |
| `RFC9256-2.1-4` | The endpoint is specified as an IPv4 or IPv6 address and SHOULD resolve to a unique node in the domain (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.3-1` | The RECOMMENDED default Protocol-Origin values are PCEP=10, BGP SR Policy=20, and Via Configuration=30 (§2.3) | RECOMMENDED | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.7-1` | It is RECOMMENDED that each candidate path of a given SR Policy has a different Preference (§2.7) | RECOMMENDED | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.8-1` | The RECOMMENDED candidate-path validity criterion is the validity of at least one of its constituent segment lists (§2.8) | RECOMMENDED | 2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4-4` | When the algorithm is not specified for SID types that allow it, the headend SHOULD use the Strict Shortest Path algorithm if available and otherwise SHOULD use the default Shortest Path algorithm (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4.1-1` | When an "IPv6 Explicit NULL label" is not present as the bottom label, the headend SHOULD automatically impose one (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.1-2` | Candidate paths of the same SR Policy SHOULD have the same BSID (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-6` | Dynamically bound BSIDs SHOULD use an available SID outside the SRLB (§6.2) | SHOULD | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-9` | The BSID SHOULD NOT be used as an identification of an SR Policy (§6.2) | SHOULD NOT | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.1-5` | An implementation MAY allow the assignment of a symbolic name of printable ASCII characters to an SR Policy for debugging/troubleshooting (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.1-6` | The SR Policy name MAY also be signaled along with a candidate path of the SR Policy (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.1-7` | An SR Policy MAY have multiple names associated with it when different names arrive with different candidate paths (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.3-2` | Implementations MAY allow modifications of the default Protocol-Origin values, similar to a routing administrative distance (§2.3) | MAY | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.4-3` | When provisioning is via configuration, the Originator ASN and node address MAY be set to either the headend or the provisioning controller/node ASN and address (§2.4) | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.6-2` | Candidate paths MAY also be assigned or signaled with a symbolic name of printable ASCII characters for debugging/troubleshooting (§2.6) | MAY | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.12-1` | An implementation MAY provide a per-policy priority configuration governing the order in which policies are re-computed (§2.12) | MAY | 2.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.12-2` | A candidate path MAY be signaled with a priority value (§2.12) | MAY | 2.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4-1` | A Type A SID MAY be any MPLS label, including special-purpose labels such as explicit-null (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4-2` | For SRv6 SID types (B, I, J, K), the SRv6 SID behavior and structure MAY also be provided for the headend to validate the SID (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4-3` | For SID types that resolve to a Prefix/Adjacency SID (C, D, I, J, K), the SR Algorithm to be used MAY also be provided (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4-5` | For SID types C through K, a SID value MAY also be optionally provided to the headend for verification (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-5.1-1` | An explicit candidate path MAY consist of a single explicit segment list containing only an implicit-null label to indicate pop-and-forward behavior (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-5.1-3` | Implementations MAY provide a local configuration option to enable SID verification on a global, per-policy, or per-candidate-path basis (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-5.1-6` | A segment list MAY be declared invalid when its last segment is neither a Prefix SID advertised by the endpoint node nor an Adjacency SID of a link terminating on the endpoint node (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-5.1-7` | An explicit candidate path MAY be declared invalid when its constituent segment lists use segment types of different SR data planes (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.1-1` | Each candidate path MAY be defined with a BSID (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-1` | In the case of SR-MPLS, SRv6 BSIDs (e.g., End.BM) MAY be associated with the SR Policy in addition to the MPLS BSID (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-2` | In the case of SRv6, multiple SRv6 BSIDs (e.g., End.B6.Encaps and End.B6.Encaps.Red) MAY be associated with the SR Policy (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-3` | A headend MAY optionally check that the BSID is available within the given SID range, i.e., the SRLB (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-5` | When no BSID is available, the SR Policy MAY dynamically bind a BSID to itself (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-7` | When a new active path has no specified or available BSID, the SR Policy MAY keep the previous BSID (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-8` | The association of an SR Policy with a BSID MAY change over the life of the SR Policy (e.g., upon active path change) (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2.1-1` | When all candidate paths have an unspecified BSID, a BSID MAY be dynamically bound to the SR Policy as soon as the first valid candidate path is received (§6.2.1) | MAY | 6.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2.3-1` | An implementation MAY support configuring the Specified-BSID-only restrictive behavior for all or individual SR Policies (§6.2.3) | MAY | 6.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2.3-2` | The Specified-BSID-only restrictive behavior MAY also be signaled on a per-SR-Policy basis to the headend (§6.2.3) | MAY | 6.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.4-1` | An implementation MAY choose to associate a Binding SID with any type of interface or tunnel (§6.4) | MAY | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-7-2` | Implementations MAY support an administrative state to control locally provisioned policies via mechanisms like CLI or NETCONF (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-8.2-1` | An SR Policy MAY be enabled for the Drop-Upon-Invalid behavior (§8.2) | MAY | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-8.4-1` | In a BGP multi-path scenario, the BGP route MAY be resolved over a mix of paths steered over SR Policies and paths resolved via normal BGP next-hop resolution (§8.4) | MAY | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-8.4-2` | Implementations MAY provide options to prefer one type of path over the other, or other local policy, to select the paths (§8.4) | MAY | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-8.6-1` | A headend MAY support options to apply per-flow steering only for traffic matching specific prefixes (§8.6) | MAY | 8.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-8.7-1` | Headend H MAY be configured with a local routing policy that overrides any BGP/IGP path and steers a specified packet on an SR Policy (§8.7) | MAY | 8.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-9.3-1` | A lower-preference candidate path MAY be designated as the backup for a specific or all active candidate path(s) (§9.3) | MAY | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-9.3-2` | The headend MAY compute a priori and validate backup candidate paths and provision them into the forwarding plane as backup for the active path (§9.3) | MAY | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-9.3-3` | A fast-reroute mechanism MAY be used to trigger sub-50 msec switchover from the active to the backup candidate path (§9.3) | MAY | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-9.3-4` | Mechanisms like BFD MAY be used for fast detection of such failures (§9.3) | MAY | 9.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9256-2.1-1`](#rfc9256-2.1-1) An SR Policy MUST be identified through the tuple <Headend, Color, Endpoint> (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: the three-element tuple is headend-scoped identification, and ze holds no headend. SRPolicy carries distinguisher, color, endpoint and afi and no headend field (internal/component/bgp/plugins/nlri/srpolicy/types.go:35-40), and grep -rni headend --include=*.go over internal/ and pkg/ matches only internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go. What ze does implement -- identification by <Color, Endpoint> within one headend's context -- is the separate RFC9256-2.1-2, which the NLRI key does satisfy; this line is the same not-applicable shape as RFC9256-2.1-3, RFC9256-2.9-1 and RFC9256-7-1, which already record that ze instantiates no policy at a headend |
| [`RFC9256-2.1-3`](#rfc9256-2.1-3) The headend is specified as an IPv4 or IPv6 address and MUST resolve to a unique node in the SR domain (§2.1) | no test | no test carries this requirement id; annotated {not-applicable}: ze originates and parses SR Policy NLRI but instantiates no policy at a headend; grep for headend or head-end across internal/component/bgp and internal/component/mpls matches nothing, and the SR Policy plugin carries no headend field to specify or resolve (internal/component/bgp/plugins/nlri/srpolicy/types.go:35) |
| [`RFC9256-2.2-1`](#rfc9256-2.2-1) The endpoints of the constituent SR Policies and the parent SR Policy MUST be identical (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: composite candidate paths have no producer; grep -rniE composite over internal, pkg and cmd matches only report subjects and capability sub-components, and parseConfigRoute accepts no constituent-policy keyword (internal/component/bgp/plugins/nlri/srpolicy/config.go:72) |
| [`RFC9256-2.2-2`](#rfc9256-2.2-2) The colors of each of the constituent SR Policies and the parent SR Policy MUST be different (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: composite candidate paths have no producer; parseConfigRoute's keyword switch has no constituent-policy or composite spelling, so ze never forms a parent policy whose constituents could share a color (internal/component/bgp/plugins/nlri/srpolicy/config.go:72) |
| [`RFC9256-2.2-3`](#rfc9256-2.2-3) The constituent SR Policies MUST NOT use composite candidate paths (§2.2) | no test | no test carries this requirement id; annotated {not-applicable}: composite candidate paths have no producer; grep -rniE composite over internal, pkg and cmd finds no SR Policy match, so ze forms no constituent policy that could carry one (internal/component/bgp/plugins/nlri/srpolicy/config.go:72) |
| [`RFC9256-2.4-1`](#rfc9256-2.4-1) For the Originator ASN, if 2-byte ASNs are in use the low-order 16 bits MUST be used and the high-order bits MUST be set to 0 (§2.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze encodes no Originator sub-TLV; the SR Policy sub-TLV constant set is preference, binding-sid, priority, srv6-binding-sid, segment-list and the two name sub-TLVs only, and grep for ProtocolOrigin or OriginatorASN over internal, pkg and cmd matches nothing (internal/component/bgp/plugins/nlri/srpolicy/config.go:23) |
| [`RFC9256-2.4-2`](#rfc9256-2.4-2) For the Originator node address, IPv4 addresses MUST be encoded in the lowest 32 bits and the high-order bits MUST be set to 0 (§2.4) | no test | no test carries this requirement id; annotated {not-applicable}: ze encodes no Originator sub-TLV, so it writes no originator node address; buildTunnelEncap emits only the preference, BSID, priority, segment-list and name sub-TLVs (internal/component/bgp/plugins/nlri/srpolicy/config.go:339) |
| [`RFC9256-2.6-1`](#rfc9256-2.6-1) The identity of a candidate path MUST be uniquely established in the context of an SR Policy <Headend, Color, Endpoint> to handle add, delete, or modify operations unambiguously (§2.6) | {gap}, no test | the only candidate-path identity ze produces is the RFC 9830 distinguisher in the NLRI key, written by (*SRPolicy).WriteTo (internal/component/bgp/plugins/nlri/srpolicy/types.go:91); ze keeps no candidate-path store, so an add, delete or modify is never resolved against a candidate-path identity, and neither a protocol origin nor an originator is carried |
| [`RFC9256-2.9-1`](#rfc9256-2.9-1) Whenever a new path is learned, an active path is deleted, an existing path's validity changes, or an existing path is changed, the candidate-path selection process MUST be re-executed (§2.9) | no test | no test carries this requirement id; annotated {not-applicable}: ze runs no candidate-path selection; there is no candidate-path store and no preference comparison anywhere (grep for protocol-origin or active candidate over internal matches nothing), and the SR Policy plugin registers only an NLRI codec and a config route encoder (internal/component/bgp/plugins/nlri/srpolicy/register.go:29) |
| [`RFC9256-2.11-1`](#rfc9256-2.11-1) Only the active candidate path MUST be used for forwarding traffic steered onto that policy, except in scenarios such as fast reroute where a backup candidate path may be used (§2.11) | no test | no test carries this requirement id; annotated {not-applicable}: no SR Policy reaches forwarding; SAFI 73 appears only in the family registry and the NLRI codec (internal/core/family/family.go:93, internal/component/bgp/plugins/nlri/srpolicy/types.go:74), and grep for SAFISRPolicy over internal/plugins/fib and internal/component/resolve matches nothing |
| [`RFC9256-4-6`](#rfc9256-4-6) When building the label stack or SRv6 SID list, the node instantiating the policy MUST interpret the first segment as the topmost MPLS label / first SRv6 SID and the last segment as the bottommost label / last SID (§4) | {gap}, no test | no code converts an SR Policy segment list into a label stack or an SRv6 SID list; buildSegmentListSubTLV only serializes the configured segments in their configured order (internal/component/bgp/plugins/nlri/srpolicy/config.go:428), and a received Tunnel Encapsulation attribute is kept as raw TLV bytes of which only the Preference sub-TLV is ever decoded (internal/core/bgp/attribute/tunnel_encap.go:39 and :137) |
| [`RFC9256-5.1-2`](#rfc9256-5.1-2) A segment list of an explicit candidate path MUST be declared invalid when it is empty, its weight is 0, it mixes SR-MPLS and SRv6 segment types, the headend cannot resolve the first SID to outgoing interface(s)/next-hop(s), it cannot resolve any non-first SID of type C-K to an MPLS label or SRv6 SID, or verification fails for any SID for which verification was requested (§5.1) | {gap}, no test | parseSegmentList builds a segment list with no validity determination; it accepts a list with zero segments, accepts weight 0, and appends Type A and Type B segments into one list without a data-plane check (internal/component/bgp/plugins/nlri/srpolicy/config.go:267), and ze resolves no SID to an outgoing interface or next hop |
| [`RFC9256-5.1-5`](#rfc9256-5.1-5) The first SID MUST always be reachable regardless of its type (§5.1) | {gap}, no test | ze resolves no SID; the first segment of a segment list is encoded straight from configuration with no reachability lookup (internal/component/bgp/plugins/nlri/srpolicy/config.go:276), and grep for SAFISRPolicy over internal/component/resolve and internal/plugins/fib matches nothing, so first-SID reachability is never established |
| [`RFC9256-5.2-1`](#rfc9256-5.2-1) If no solution is found to the optimization objective and constraints, the dynamic candidate path MUST be declared invalid (§5.2) | no test | no test carries this requirement id; annotated {not-applicable}: ze signals only explicit segment lists and has no dynamic candidate path; parseSegmentList takes literal MPLS labels and SRv6 SIDs (internal/component/bgp/plugins/nlri/srpolicy/config.go:276) and grep for OptimizationObjective over internal, pkg and cmd matches nothing |
| [`RFC9256-6.1-3`](#rfc9256-6.1-3) Candidate paths of different SR Policies MUST NOT have the same BSID (§6.1) | {gap}, no test | buildBindingSIDSubTLV encodes whatever label the configuration names (internal/component/bgp/plugins/nlri/srpolicy/config.go:381) and ze keeps no BSID allocation table, so two SR Policies configured with the same BSID are both encoded and advertised |
| [`RFC9256-6.2-4`](#rfc9256-6.2-4) When the specified BSID is not available (optionally not in the SRLB), an alert message MUST be generated via mechanisms like syslog (§6.2) | {gap}, no test | ze performs no BSID availability check; buildBindingSIDSubTLV encodes the configured label unconditionally (internal/component/bgp/plugins/nlri/srpolicy/config.go:381) and the only SRLB allocator in ze belongs to OSPF Adj-SIDs (internal/plugins/ospf/sr_adjsid.go:115), which no SR Policy code consults, so no alert is raised |
| [`RFC9256-6.2.3-3`](#rfc9256-6.2.3-3) Under Specified-BSID-only behavior, when a candidate path has an unspecified or unavailable BSID it is considered invalid and an alert MUST be triggered via mechanisms like syslog (§6.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no Specified-BSID-only behavior; grep for specified-bsid or SpecifiedBSID over internal, pkg and cmd matches nothing and parseConfigRoute's keyword switch has no spelling for it (internal/component/bgp/plugins/nlri/srpolicy/config.go:72) |
| [`RFC9256-6.2.3-4`](#rfc9256-6.2.3-4) Under Specified-BSID-only behavior, other candidate paths MUST then be evaluated for becoming the active candidate path (§6.2.3) | no test | no test carries this requirement id; annotated {not-applicable}: ze implements no Specified-BSID-only behavior and no candidate-path selection, so there is no evaluation of other candidate paths to perform; the SR Policy plugin registers only an NLRI codec and a config route encoder (internal/component/bgp/plugins/nlri/srpolicy/register.go:29) |
| [`RFC9256-7-1`](#rfc9256-7-1) The SR Policy state MUST also reflect the reason when a policy and/or its candidate path is not active due to validation errors or not being preferred (§7) | no test | no test carries this requirement id; annotated {not-applicable}: ze keeps no SR Policy state; grep for SRPolicyState or policyState over internal, pkg and cmd matches nothing, grep for sr-policy over internal/component/cli matches nothing, and an SR Policy route is an ordinary BGP NLRI carried in the RIB (internal/component/bgp/plugins/nlri/srpolicy/register.go:29) |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9256-2.1-1`](#rfc9256-2.1-1)

An SR Policy MUST be identified through the tuple <Headend, Color, Endpoint> (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-2.1-1, so no unit is bound to it.

### [`RFC9256-2.1-2`](#rfc9256-2.1-2)

In the context of a specific headend, an SR Policy MUST be identified by the <Color, Endpoint> tuple (§2.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9256IdentificationTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L48) | unit/verify | unproven |

### [`RFC9256-2.1-3`](#rfc9256-2.1-3)

The headend is specified as an IPv4 or IPv6 address and MUST resolve to a unique node in the SR domain (§2.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-2.1-3, so no unit is bound to it.

### [`RFC9256-2.2-1`](#rfc9256-2.2-1)

The endpoints of the constituent SR Policies and the parent SR Policy MUST be identical (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-2.2-1, so no unit is bound to it.

### [`RFC9256-2.2-2`](#rfc9256-2.2-2)

The colors of each of the constituent SR Policies and the parent SR Policy MUST be different (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-2.2-2, so no unit is bound to it.

### [`RFC9256-2.2-3`](#rfc9256-2.2-3)

The constituent SR Policies MUST NOT use composite candidate paths (§2.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-2.2-3, so no unit is bound to it.

### [`RFC9256-2.4-1`](#rfc9256-2.4-1)

For the Originator ASN, if 2-byte ASNs are in use the low-order 16 bits MUST be used and the high-order bits MUST be set to 0 (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-2.4-1, so no unit is bound to it.

### [`RFC9256-2.4-2`](#rfc9256-2.4-2)

For the Originator node address, IPv4 addresses MUST be encoded in the lowest 32 bits and the high-order bits MUST be set to 0 (§2.4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-2.4-2, so no unit is bound to it.

### [`RFC9256-2.6-1`](#rfc9256-2.6-1)

The identity of a candidate path MUST be uniquely established in the context of an SR Policy <Headend, Color, Endpoint> to handle add, delete, or modify operations unambiguously (§2.6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-2.6-1, so no unit is bound to it.

### [`RFC9256-2.6-3`](#rfc9256-2.6-3)

Symbolic candidate-path names MUST NOT be considered as identifiers for a candidate path (§2.6)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9256SymbolicNamesAreNotIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L117) | unit/verify | unproven |
| positive | [`TestRFC9256SymbolicNamesAreNotIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L106) | unit/verify | unproven |

### [`RFC9256-2.9-1`](#rfc9256-2.9-1)

Whenever a new path is learned, an active path is deleted, an existing path's validity changes, or an existing path is changed, the candidate-path selection process MUST be re-executed (§2.9)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-2.9-1, so no unit is bound to it.

### [`RFC9256-2.11-1`](#rfc9256-2.11-1)

Only the active candidate path MUST be used for forwarding traffic steered onto that policy, except in scenarios such as fast reroute where a backup candidate path may be used (§2.11)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-2.11-1, so no unit is bound to it.

### [`RFC9256-4-6`](#rfc9256-4-6)

When building the label stack or SRv6 SID list, the node instantiating the policy MUST interpret the first segment as the topmost MPLS label / first SRv6 SID and the last segment as the bottommost label / last SID (§4)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-4-6, so no unit is bound to it.

### [`RFC9256-5.1-2`](#rfc9256-5.1-2)

A segment list of an explicit candidate path MUST be declared invalid when it is empty, its weight is 0, it mixes SR-MPLS and SRv6 segment types, the headend cannot resolve the first SID to outgoing interface(s)/next-hop(s), it cannot resolve any non-first SID of type C-K to an MPLS label or SRv6 SID, or verification fails for any SID for which verification was requested (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-5.1-2, so no unit is bound to it.

### [`RFC9256-5.1-4`](#rfc9256-5.1-4)

Types A or B MUST be used for the SIDs whose reachability cannot be verified (§5.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9256SegmentTypesAreAOrB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L130) | unit/verify | unproven |

### [`RFC9256-5.1-5`](#rfc9256-5.1-5)

The first SID MUST always be reachable regardless of its type (§5.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-5.1-5, so no unit is bound to it.

### [`RFC9256-5.2-1`](#rfc9256-5.2-1)

If no solution is found to the optimization objective and constraints, the dynamic candidate path MUST be declared invalid (§5.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-5.2-1, so no unit is bound to it.

### [`RFC9256-6.1-3`](#rfc9256-6.1-3)

Candidate paths of different SR Policies MUST NOT have the same BSID (§6.1)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-6.1-3, so no unit is bound to it.

### [`RFC9256-6.2-4`](#rfc9256-6.2-4)

When the specified BSID is not available (optionally not in the SRLB), an alert message MUST be generated via mechanisms like syslog (§6.2)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-6.2-4, so no unit is bound to it.

### [`RFC9256-6.2.3-3`](#rfc9256-6.2.3-3)

Under Specified-BSID-only behavior, when a candidate path has an unspecified or unavailable BSID it is considered invalid and an alert MUST be triggered via mechanisms like syslog (§6.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-6.2.3-3, so no unit is bound to it.

### [`RFC9256-6.2.3-4`](#rfc9256-6.2.3-4)

Under Specified-BSID-only behavior, other candidate paths MUST then be evaluated for becoming the active candidate path (§6.2.3)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-6.2.3-4, so no unit is bound to it.

### [`RFC9256-7-1`](#rfc9256-7-1)

The SR Policy state MUST also reflect the reason when a policy and/or its candidate path is not active due to validation errors or not being preferred (§7)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9256-7-1, so no unit is bound to it.

## Extraction sign-off

No extraction sign-off exists for RFC 9256, so no reviewer has walked its text sentence by sentence.

## Superseded

No document obsoletes RFC 9256, so its obligations are stated where they were written.
