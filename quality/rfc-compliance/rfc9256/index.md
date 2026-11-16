# RFC 9256 - Segment Routing Policy Architecture

Segment Routing Policy Architecture. Every requirement listed for RFC 9256, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 1 | of 22 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 22 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 22 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 22 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 0 | 0 of 4 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 19 | of 22 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Audit verdicts | 3 | of 22 mandatory requirements reviewed | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 22 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 22 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 22 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 22 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 22 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 22 | of 69 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 22 mandatory requirements: they add up to 22. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze's own tests verify |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither test verification nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | Segment Routing Policy Architecture |
| Automatic checks | Included |
| Requirements | 69 |
| Mandatory requirements (MUST level) | 22 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 20 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 4 |
| Tests linked to requirements | 4 |
| Recorded audit verdicts | 3 |
| Fault test records | 0 |
| Summary | `rfc/short/rfc9256.md` |
| Requirement table | `rfc/requirements/rfc9256.md` |
| RFC text | `rfc/full/rfc9256.txt` |

## Automatic checks

Included in automatic checks: Segment Routing Policy Architecture

## Published support status

**Area:** Segment Routing Policy Architecture

**Test verification:** 0 of 22 MUST proven

**Evidence:** -

**Unmet:** RFC9256-2.1-1, RFC9256-2.1-3, RFC9256-2.11-1, RFC9256-2.2-1, RFC9256-2.2-2, RFC9256-2.2-3, RFC9256-2.4-1, RFC9256-2.4-2, RFC9256-2.6-1, RFC9256-2.9-1, RFC9256-4-6, RFC9256-4-7, RFC9256-5.1-2, RFC9256-5.1-5, RFC9256-5.2-1, RFC9256-6.1-3, RFC9256-6.2-4, RFC9256-6.2.3-3, RFC9256-6.2.3-4, RFC9256-7-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 1 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 21 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **22** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (1):** [`RFC9256-2.6-3`](#rfc9256-2.6-3)

**Annotated (including scoped evidence) (21):** [`RFC9256-2.1-1`](#rfc9256-2.1-1), [`RFC9256-2.1-2`](#rfc9256-2.1-2), [`RFC9256-2.1-3`](#rfc9256-2.1-3), [`RFC9256-2.2-1`](#rfc9256-2.2-1), [`RFC9256-2.2-2`](#rfc9256-2.2-2), [`RFC9256-2.2-3`](#rfc9256-2.2-3), [`RFC9256-2.4-1`](#rfc9256-2.4-1), [`RFC9256-2.4-2`](#rfc9256-2.4-2), [`RFC9256-2.6-1`](#rfc9256-2.6-1), [`RFC9256-2.9-1`](#rfc9256-2.9-1), [`RFC9256-2.11-1`](#rfc9256-2.11-1), [`RFC9256-4-6`](#rfc9256-4-6), [`RFC9256-5.1-2`](#rfc9256-5.1-2), [`RFC9256-5.1-4`](#rfc9256-5.1-4), [`RFC9256-5.1-5`](#rfc9256-5.1-5), [`RFC9256-5.2-1`](#rfc9256-5.2-1), [`RFC9256-6.1-3`](#rfc9256-6.1-3), [`RFC9256-6.2-4`](#rfc9256-6.2-4), [`RFC9256-6.2.3-3`](#rfc9256-6.2.3-3), [`RFC9256-6.2.3-4`](#rfc9256-6.2.3-4), [`RFC9256-7-1`](#rfc9256-7-1)

## Figures

- **MUST bar**: Tests awaiting verification 3, Not built 19
- **Overview bar**: Tests awaiting verification 3, Not assessed 46, Not built 20

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 22 |
| MUST rows kept (the MUST bar) | 22 |
| Ze verified | 0 |
| OS verified | 0 |
| Tests awaiting verification | 3 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 19 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 47 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 46 |
| SHOULD not built | 1 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc9256.json](../../../data/rfc/rfc9256.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9256-2.1-1` | An SR Policy MUST be identified through the tuple <Headend, Color, Endpoint> (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** SRPolicy (internal/component/bgp/plugins/nlri/srpolicy/types.go) carries distinguisher, color and endpoint and no headend, so no SR Policy is identified by the full tuple. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-2.1-2` | In the context of a specific headend, an SR Policy MUST be identified by the <Color, Endpoint> tuple (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestRFC9256IdentificationTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L48). **negative:** no negative test. **{single-polarity}:** every <Color, Endpoint> pair identifies a policy, so no conforming input exists that identification must REFUSE, and a counter-pole would have to assert a non-identification that the requirement never mandates. The discrimination against a key that ignores its inputs is inside the positive test: a different color and a different endpoint each change the key, while a changed preference does not |
| `RFC9256-2.1-3` | The headend is specified as an IPv4 or IPv6 address and MUST resolve to a unique node in the SR domain (§2.1) | MUST | 2.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze specifies no headend address and resolves none to a node in the SR domain. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-2.2-1` | The endpoints of the constituent SR Policies and the parent SR Policy MUST be identical (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze builds no composite candidate path: parseConfigRoute has no constituent-policy keyword, so no parent and constituent endpoints are compared. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-2.2-2` | The colors of each of the constituent SR Policies and the parent SR Policy MUST be different (§2.2) | MUST | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze builds no composite candidate path: parseConfigRoute has no constituent-policy keyword, so no parent and constituent colors are compared. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-2.2-3` | The constituent SR Policies MUST NOT use composite candidate paths (§2.2) | MUST NOT | 2.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze builds no composite candidate path or constituent SR Policy, so no constituent is checked for one. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-2.4-1` | If 2-byte ASNs are in use, the low-order 16 bits MUST be used, and the high-order bits MUST be set to 0. (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze encodes no Originator: internal/component/bgp/plugins/nlri/srpolicy/config.go::buildTunnelEncap writes only the preference, priority, binding SID, SRv6 binding SID, segment list and name sub-TLVs, and no candidate path carries an Originator ASN. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-2.4-2` | IPv4 addresses MUST be encoded in the lowest 32 bits, and the high-order bits MUST be set to 0. (§2.4) | MUST | 2.4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze encodes no Originator: internal/component/bgp/plugins/nlri/srpolicy/config.go::buildTunnelEncap writes only the preference, priority, binding SID, SRv6 binding SID, segment list and name sub-TLVs, and no candidate path carries an Originator node address. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-2.6-1` | The identity of a candidate path MUST be uniquely established in the context of an SR Policy <Headend, Color, Endpoint> to handle add, delete, or modify operations on them in an unambiguous manner regardless of their source(s). (§2.6) | MUST | 2.6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the only candidate-path identity ze produces is the RFC 9830 distinguisher in the NLRI key, written by (*SRPolicy).WriteTo (internal/component/bgp/plugins/nlri/srpolicy/types.go:91); ze keeps no candidate-path store, so an add, delete or modify is never resolved against a candidate-path identity, and neither a protocol origin nor an originator is carried |
| `RFC9256-2.6-3` | Such symbolic names MUST NOT be considered as identifiers for a candidate path. (§2.6) | MUST NOT | 2.6 | **positive:** `unit/verify` [`TestRFC9256SymbolicNamesAreNotIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L106). **negative:** `unit/verify` [`TestRFC9256SymbolicNamesAreNotIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L117) |
| `RFC9256-2.9-1` | Whenever a new path is learned or an active path is deleted, the validity of an existing path changes, or an existing path is changed, the selection process MUST be re-executed. (§2.9) | MUST | 2.9 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze runs no candidate-path selection, so nothing is re-executed when a path changes. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-2.11-1` | Only the active candidate path MUST be used for forwarding traffic that is being steered onto that policy except for certain scenarios such as fast reroute where a backup candidate path may be used as described in Section 9.3. (§2.11) | MUST | 2.11 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no SR Policy reaches forwarding: neither FIB backend (internal/plugins/fib/kernel, internal/plugins/fib/vpp) installs a SAFI 73 candidate path; the VPP SRv6 steering in internal/plugins/fib/vpp/srv6.go serves RFC 9252 service routes, not SR Policies. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-4-6` | When building the MPLS label stack or the SRv6 SID list from the segment list, the node instantiating the policy MUST interpret the set of Segments as follows: * The first Segment represents the topmost MPLS label or the first SRv6 SID. It identifies the active segment the traffic will be directed toward along the explicit SR path. * The last segment represents the bottommost MPLS label or the last SRv6 SID the traffic will be directed toward along the explicit SR path. (§4) | MUST | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** no code converts an SR Policy segment list into a label stack or an SRv6 SID list; buildSegmentListSubTLV only serializes the configured segments in their configured order (internal/component/bgp/plugins/nlri/srpolicy/config.go:428), and a received Tunnel Encapsulation attribute is kept as raw TLV bytes of which only the Preference sub-TLV is ever decoded (internal/core/bgp/attribute/tunnel_encap.go:39 and :137) |
| `RFC9256-5.1-2` | A segment list of an explicit candidate path MUST be declared invalid when any of the following is true: * It is empty. * Its weight is 0. * It comprises a mix of SR-MPLS and SRv6 segment types. * The headend is unable to perform path resolution for the first SID into one or more outgoing interface(s) and next-hop(s). * The headend is unable to perform SID resolution for any non-first SID of type C through K into an MPLS label or an SRv6 SID. * The headend verification fails for any SID for which verification has been explicitly requested. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** parseSegmentList builds a segment list with no validity determination; it accepts a list with zero segments, accepts weight 0, and appends Type A and Type B segments into one list without a data-plane check (internal/component/bgp/plugins/nlri/srpolicy/config.go:267), and ze resolves no SID to an outgoing interface or next hop |
| `RFC9256-5.1-4` | Types A or B MUST be used for the SIDs for which the reachability cannot be verified. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestRFC9256SegmentTypesAreAOrB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L130). **negative:** no negative test. **{single-polarity}:** the segment encoder accepts only type-a and type-b and returns an error for every other segment spelling (internal/component/bgp/plugins/nlri/srpolicy/config.go:276), so every SID ze emits already carries its value outright; ze holds no SID verification state whose failure could produce a violating segment to reject |
| `RFC9256-5.1-5` | Note that the first SID MUST always be reachable regardless of its type. (§5.1) | MUST | 5.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze resolves no SID; the first segment of a segment list is encoded straight from configuration with no reachability lookup (internal/component/bgp/plugins/nlri/srpolicy/config.go:276), and grep for SAFISRPolicy over internal/component/resolve and internal/plugins/fib matches nothing, so first-SID reachability is never established |
| `RFC9256-5.2-1` | If no solution is found to the optimization objective and constraints, then the dynamic candidate path MUST be declared invalid. (§5.2) | MUST | 5.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze signals explicit segment lists only (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseSegmentList) and computes no dynamic candidate path. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-6.1-3` | Candidate paths of different SR Policies MUST NOT have the same BSID (§6.1) | MUST NOT | 6.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** buildBindingSIDSubTLV encodes whatever label the configuration names (internal/component/bgp/plugins/nlri/srpolicy/config.go:381) and ze keeps no BSID allocation table, so two SR Policies configured with the same BSID are both encoded and advertised |
| `RFC9256-6.2-4` | When the specified BSID is not available (optionally is not in the SRLB), an alert message MUST be generated via mechanisms like syslog. (§6.2) | MUST | 6.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze performs no BSID availability check; buildBindingSIDSubTLV encodes the configured label unconditionally (internal/component/bgp/plugins/nlri/srpolicy/config.go:381) and the only SRLB allocator in ze belongs to OSPF Adj-SIDs (internal/plugins/ospf/sr_adjsid.go:115), which no SR Policy code consults, so no alert is raised |
| `RFC9256-6.2.3-3` | When this restrictive behavior is enabled, if the candidate path has an unspecified BSID or if the specified BSID is not available when the candidate path becomes active, then no BSID is bound to it and the candidate path is considered invalid. An alert MUST be triggered for this error via mechanisms like syslog. (§6.2.3) | MUST | 6.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no Specified-BSID-only behavior, binds no BSID and raises no alert. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-6.2.3-4` | Other candidate paths MUST then be evaluated for becoming the active candidate path. (§6.2.3) | MUST | 6.2.3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no Specified-BSID-only behavior and runs no candidate-path selection, so no other candidate path is evaluated. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-7-1` | The SR Policy state MUST also reflect the reason when a policy and/or its candidate path is not active due to validation errors or not being preferred (§7) | MUST | 7 | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze keeps no SR Policy state, so no reason for an inactive policy or candidate path is reported. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| `RFC9256-2.1-4` | The endpoint is specified as an IPv4 or IPv6 address and SHOULD resolve to a unique node in the domain (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.3-1` | The table below specifies the RECOMMENDED default values of Protocol-Origin: +=================+===================+ \| Protocol-Origin \| Description \| +=================+===================+ \| 10 \| PCEP \| +-----------------+-------------------+ \| 20 \| BGP SR Policy \| +-----------------+-------------------+ \| 30 \| Via Configuration \| (§2.3) | RECOMMENDED | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.7-1` | It is RECOMMENDED that each candidate path of a given SR Policy has a different Preference (§2.7) | RECOMMENDED | 2.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.8-1` | The RECOMMENDED candidate path validity criterion is the validity of at least one of its constituent segment lists. (§2.8) | RECOMMENDED | 2.8 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4-4` | When the algorithm is not specified for the SID types above which optionally allow for it, the headend SHOULD use the Strict Shortest Path algorithm if available and otherwise, it SHOULD use the default Shortest Path algorithm. (§4) | SHOULD | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4.1-1` | When an "IPv6 Explicit NULL label" is not present as the bottom label, the headend SHOULD automatically impose one (§4.1) | SHOULD | 4.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.1-2` | Candidate paths of the same SR Policy SHOULD have the same BSID (§6.1) | SHOULD | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-6` | Dynamically bound BSIDs SHOULD use an available SID outside the SRLB (§6.2) | SHOULD | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-9` | the BSID SHOULD NOT be used as an identification of an SR Policy. (§6.2) | SHOULD NOT | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.1-5` | An implementation MAY allow the assignment of a symbolic name comprising printable ASCII [RFC0020] characters (i.e., 0x20 to 0x7E) to an SR Policy to serve as a user-friendly attribute for debugging and troubleshooting purposes. (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.1-6` | The SR Policy name MAY also be signaled along with a candidate path of the SR Policy (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.1-7` | An SR Policy MAY have multiple names associated with it in the scenario where the headend receives different SR Policy names along with different candidate paths for the same SR Policy via the same or different sources. (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.3-2` | implementations MAY allow modifications of these default values assigned to protocols on the headend along similar lines as a routing administrative distance. (§2.3) | MAY | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.4-3` | When provisioning is via configuration, the ASN and node address MAY be set to either the headend or the provisioning controller/node ASN and address. (§2.4) | MAY | 2.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.6-2` | Candidate paths MAY also be assigned or signaled with a symbolic name comprising printable ASCII [RFC0020] characters (i.e., 0x20 to 0x7E) to serve as a user-friendly attribute for debugging and troubleshooting purposes. (§2.6) | MAY | 2.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.12-1` | An implementation MAY provide a per-policy priority configuration. The operator may set this field to indicate the order in which the policies should be re-computed. (§2.12) | MAY | 2.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-2.12-2` | A candidate path MAY be signaled with a priority value (§2.12) | MAY | 2.12 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4-1` | Additionally, special purpose labels like explicit-null or in general any MPLS label MAY also be used. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4-2` | Optionally, the SRv6 SID behavior (as defined in [RFC8986] or other SRv6 specifications) and structure (as defined in [RFC8986]) MAY also be provided for the headend to perform validation of the SID when using it for building the segment list. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4-3` | The SR algorithm (refer to Section 3.1.1 of [RFC8402]) to be used MAY also be provided. Type D: IPv6 Global Prefix with optional SR Algorithm for SR-MPLS: In this case, the headend is required to resolve the specified IPv6 Global Prefix Address to the SR-MPLS label corresponding to its Prefix SID segment (as defined in [RFC8402]). The SR Algorithm (refer to Section 3.1.1 of [RFC8402]) to be used MAY also be provided. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-4-7` | The SR Algorithm (refer to Section 3.1.1 of [RFC8402]), the SRv6 SID behavior (as defined in [RFC8986] or other SRv6 specifications), and structure (as defined in [RFC8986]) MAY also be provided. (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test. **{gap}:** this sentence applies to each of segment types I, J and K. Their optional SR Algorithm, SRv6 SID behavior and SID structure are not implemented by the SR Policy originator: internal/component/bgp/plugins/nlri/srpolicy/config.go::parseSegmentList accepts only type-a and type-b, and buildSegmentSubSubTLV emits only those types. Type B endpoint-behavior support does not supply any of the missing I/J/K fields. The absent optional segment types are not authorized for implementation by this audit. |
| `RFC9256-4-5` | For SID types C through K, a SID value MAY also be optionally provided to the headend for verification (§4) | MAY | 4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-5.1-1` | An explicit candidate path MAY consist of a single explicit segment list containing only an implicit-null label to indicate pop-and- forward behavior. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-5.1-3` | Implementations MAY provide a local configuration option to enable verification on a global or per-policy or per- candidate path basis. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-5.1-6` | Additionally, a segment list MAY be declared invalid when both of the conditions below are met : * Its last segment is not a Prefix SID (including BGP Peer Node-SID) advertised by the node specified as the endpoint of the corresponding SR Policy. * Its last segment is not an Adjacency SID (including BGP Peer Adjacency SID) of any of the links present on neighbor nodes and that terminate on the node specified as the endpoint of the corresponding SR Policy. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-5.1-7` | Additionally, an explicit candidate path MAY be declared invalid when its constituent segment lists (valid or invalid) are using segment types of different SR data planes. (§5.1) | MAY | 5.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.1-1` | Each candidate path MAY be defined with a BSID (§6.1) | MAY | 6.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-1` | In the case of SR-MPLS, SRv6 BSIDs (e.g., with the behavior End.BM [RFC8986]) MAY be associated with the SR Policy in addition to the MPLS BSID. (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-2` | In the case of SRv6, multiple SRv6 BSIDs (e.g., with different behaviors like End.B6.Encaps and End.B6.Encaps.Red [RFC8986]) MAY be associated with the SR Policy. (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-3` | Optionally, instead of only checking that the BSID of the active path is available, a headend MAY check that it is available within the given SID range i.e., Segment Routing Local Block (SRLB) as specified in [RFC8402]. (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-5` | In the cases (as described above) where SR Policy does not have a BSID available, the SR Policy MAY dynamically bind a BSID to itself. (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-7` | Assuming that at time t the BSID of the SR Policy is B1, if at time t+dt a different candidate path becomes active and this new active path does not have a specified BSID or its BSID is specified but is not available (e.g., it is in use by something else), then the SR Policy MAY keep the previous BSID B1. (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2-8` | The association of an SR Policy with a BSID thus MAY change over the life of the SR Policy (e.g., upon active path change). (§6.2) | MAY | 6.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2.1-1` | All the candidate paths of the same SR Policy can have an unspecified BSID. In such a case, a BSID MAY be dynamically bound to the SR Policy as soon as the first valid candidate path is received. (§6.2.1) | MAY | 6.2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2.3-1` | An implementation MAY support the configuration of the Specified- BSID-only restrictive behavior on the headend for all SR Policies or individual SR Policies. (§6.2.3) | MAY | 6.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.2.3-2` | An implementation MAY support the configuration of the Specified- BSID-only restrictive behavior on the headend for all SR Policies or individual SR Policies. Further, this restrictive behavior MAY also be signaled on a per-SR-Policy basis to the headend. (§6.2.3) | MAY | 6.2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-6.4-1` | An implementation MAY choose to associate a Binding SID with any type of interface (e.g., a layer 3 termination of an Optical Circuit) or a tunnel (e.g., IP tunnel, GRE tunnel, IP/UDP tunnel, MPLS RSVP-TE tunnel, etc). (§6.4) | MAY | 6.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-7-2` | Implementations MAY support an administrative state to control locally provisioned policies via mechanisms like command-line interface (CLI) or NETCONF. (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-8.2-1` | An SR Policy MAY be enabled for the Drop-Upon-Invalid behavior (§8.2) | MAY | 8.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-8.4-1` | In a BGP multi-path scenario, the BGP route MAY be resolved over a mix of paths that include those that are steered over SR Policies and others resolved via the normal BGP next-hop resolution. (§8.4) | MAY | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-8.4-2` | Implementations MAY provide options to prefer one type over the other or other forms of local policy to determine the paths that are selected. (§8.4) | MAY | 8.4 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-8.6-1` | A headend MAY support options to apply per-flow steering only for traffic matching specific prefixes (§8.6) | MAY | 8.6 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-8.7-1` | headend H MAY be configured with a local routing policy that overrides any BGP/IGP path and steers a specified packet on an SR Policy. (§8.7) | MAY | 8.7 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-9.3-1` | another (lower preference) candidate path MAY be designated as the backup for a specific or all (active) candidate path(s). (§9.3) | MAY | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-9.3-2` | The headend MAY compute a priori and validate such backup candidate paths as well as provision them into the forwarding plane as a backup for the active path. (§9.3) | MAY | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-9.3-3` | A fast-reroute mechanism MAY then be used to trigger sub-50 msec switchover from the active to the backup candidate path in the forwarding plane. (§9.3) | MAY | 9.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC9256-9.3-4` | Mechanisms like Bidirectional Forwarding Detection (BFD) MAY be used for fast detection of such failures. (§9.3) | MAY | 9.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9256-2.1-1`](#rfc9256-2.1-1) An SR Policy MUST be identified through the tuple <Headend, Color, Endpoint> (§2.1) | {gap}, no test | SRPolicy (internal/component/bgp/plugins/nlri/srpolicy/types.go) carries distinguisher, color and endpoint and no headend, so no SR Policy is identified by the full tuple. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-2.1-3`](#rfc9256-2.1-3) The headend is specified as an IPv4 or IPv6 address and MUST resolve to a unique node in the SR domain (§2.1) | {gap}, no test | ze specifies no headend address and resolves none to a node in the SR domain. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-2.2-1`](#rfc9256-2.2-1) The endpoints of the constituent SR Policies and the parent SR Policy MUST be identical (§2.2) | {gap}, no test | ze builds no composite candidate path: parseConfigRoute has no constituent-policy keyword, so no parent and constituent endpoints are compared. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-2.2-2`](#rfc9256-2.2-2) The colors of each of the constituent SR Policies and the parent SR Policy MUST be different (§2.2) | {gap}, no test | ze builds no composite candidate path: parseConfigRoute has no constituent-policy keyword, so no parent and constituent colors are compared. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-2.2-3`](#rfc9256-2.2-3) The constituent SR Policies MUST NOT use composite candidate paths (§2.2) | {gap}, no test | ze builds no composite candidate path or constituent SR Policy, so no constituent is checked for one. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-2.4-1`](#rfc9256-2.4-1) If 2-byte ASNs are in use, the low-order 16 bits MUST be used, and the high-order bits MUST be set to 0. (§2.4) | {gap}, no test | ze encodes no Originator: internal/component/bgp/plugins/nlri/srpolicy/config.go::buildTunnelEncap writes only the preference, priority, binding SID, SRv6 binding SID, segment list and name sub-TLVs, and no candidate path carries an Originator ASN. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-2.4-2`](#rfc9256-2.4-2) IPv4 addresses MUST be encoded in the lowest 32 bits, and the high-order bits MUST be set to 0. (§2.4) | {gap}, no test | ze encodes no Originator: internal/component/bgp/plugins/nlri/srpolicy/config.go::buildTunnelEncap writes only the preference, priority, binding SID, SRv6 binding SID, segment list and name sub-TLVs, and no candidate path carries an Originator node address. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-2.6-1`](#rfc9256-2.6-1) The identity of a candidate path MUST be uniquely established in the context of an SR Policy <Headend, Color, Endpoint> to handle add, delete, or modify operations on them in an unambiguous manner regardless of their source(s). (§2.6) | {gap}, no test | the only candidate-path identity ze produces is the RFC 9830 distinguisher in the NLRI key, written by (*SRPolicy).WriteTo (internal/component/bgp/plugins/nlri/srpolicy/types.go:91); ze keeps no candidate-path store, so an add, delete or modify is never resolved against a candidate-path identity, and neither a protocol origin nor an originator is carried |
| [`RFC9256-2.9-1`](#rfc9256-2.9-1) Whenever a new path is learned or an active path is deleted, the validity of an existing path changes, or an existing path is changed, the selection process MUST be re-executed. (§2.9) | {gap}, no test | ze runs no candidate-path selection, so nothing is re-executed when a path changes. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-2.11-1`](#rfc9256-2.11-1) Only the active candidate path MUST be used for forwarding traffic that is being steered onto that policy except for certain scenarios such as fast reroute where a backup candidate path may be used as described in Section 9.3. (§2.11) | {gap}, no test | no SR Policy reaches forwarding: neither FIB backend (internal/plugins/fib/kernel, internal/plugins/fib/vpp) installs a SAFI 73 candidate path; the VPP SRv6 steering in internal/plugins/fib/vpp/srv6.go serves RFC 9252 service routes, not SR Policies. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-4-6`](#rfc9256-4-6) When building the MPLS label stack or the SRv6 SID list from the segment list, the node instantiating the policy MUST interpret the set of Segments as follows: * The first Segment represents the topmost MPLS label or the first SRv6 SID. It identifies the active segment the traffic will be directed toward along the explicit SR path. * The last segment represents the bottommost MPLS label or the last SRv6 SID the traffic will be directed toward along the explicit SR path. (§4) | {gap}, no test | no code converts an SR Policy segment list into a label stack or an SRv6 SID list; buildSegmentListSubTLV only serializes the configured segments in their configured order (internal/component/bgp/plugins/nlri/srpolicy/config.go:428), and a received Tunnel Encapsulation attribute is kept as raw TLV bytes of which only the Preference sub-TLV is ever decoded (internal/core/bgp/attribute/tunnel_encap.go:39 and :137) |
| [`RFC9256-5.1-2`](#rfc9256-5.1-2) A segment list of an explicit candidate path MUST be declared invalid when any of the following is true: * It is empty. * Its weight is 0. * It comprises a mix of SR-MPLS and SRv6 segment types. * The headend is unable to perform path resolution for the first SID into one or more outgoing interface(s) and next-hop(s). * The headend is unable to perform SID resolution for any non-first SID of type C through K into an MPLS label or an SRv6 SID. * The headend verification fails for any SID for which verification has been explicitly requested. (§5.1) | {gap}, no test | parseSegmentList builds a segment list with no validity determination; it accepts a list with zero segments, accepts weight 0, and appends Type A and Type B segments into one list without a data-plane check (internal/component/bgp/plugins/nlri/srpolicy/config.go:267), and ze resolves no SID to an outgoing interface or next hop |
| [`RFC9256-5.1-5`](#rfc9256-5.1-5) Note that the first SID MUST always be reachable regardless of its type. (§5.1) | {gap}, no test | ze resolves no SID; the first segment of a segment list is encoded straight from configuration with no reachability lookup (internal/component/bgp/plugins/nlri/srpolicy/config.go:276), and grep for SAFISRPolicy over internal/component/resolve and internal/plugins/fib matches nothing, so first-SID reachability is never established |
| [`RFC9256-5.2-1`](#rfc9256-5.2-1) If no solution is found to the optimization objective and constraints, then the dynamic candidate path MUST be declared invalid. (§5.2) | {gap}, no test | ze signals explicit segment lists only (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseSegmentList) and computes no dynamic candidate path. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-6.1-3`](#rfc9256-6.1-3) Candidate paths of different SR Policies MUST NOT have the same BSID (§6.1) | {gap}, no test | buildBindingSIDSubTLV encodes whatever label the configuration names (internal/component/bgp/plugins/nlri/srpolicy/config.go:381) and ze keeps no BSID allocation table, so two SR Policies configured with the same BSID are both encoded and advertised |
| [`RFC9256-6.2-4`](#rfc9256-6.2-4) When the specified BSID is not available (optionally is not in the SRLB), an alert message MUST be generated via mechanisms like syslog. (§6.2) | {gap}, no test | ze performs no BSID availability check; buildBindingSIDSubTLV encodes the configured label unconditionally (internal/component/bgp/plugins/nlri/srpolicy/config.go:381) and the only SRLB allocator in ze belongs to OSPF Adj-SIDs (internal/plugins/ospf/sr_adjsid.go:115), which no SR Policy code consults, so no alert is raised |
| [`RFC9256-6.2.3-3`](#rfc9256-6.2.3-3) When this restrictive behavior is enabled, if the candidate path has an unspecified BSID or if the specified BSID is not available when the candidate path becomes active, then no BSID is bound to it and the candidate path is considered invalid. An alert MUST be triggered for this error via mechanisms like syslog. (§6.2.3) | {gap}, no test | ze implements no Specified-BSID-only behavior, binds no BSID and raises no alert. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-6.2.3-4`](#rfc9256-6.2.3-4) Other candidate paths MUST then be evaluated for becoming the active candidate path. (§6.2.3) | {gap}, no test | ze implements no Specified-BSID-only behavior and runs no candidate-path selection, so no other candidate path is evaluated. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-7-1`](#rfc9256-7-1) The SR Policy state MUST also reflect the reason when a policy and/or its candidate path is not active due to validation errors or not being preferred (§7) | {gap}, no test | ze keeps no SR Policy state, so no reason for an inactive policy or candidate path is reported. ze fills the SR Policy controller role only: it originates candidate paths from configuration (internal/component/bgp/plugins/nlri/srpolicy/config.go::parseConfigRoute) and instantiates no SR Policy at a headend, keeps no candidate-path state and steers no traffic. No owner decision declines the headend role, so this is owed with it |
| [`RFC9256-4-7`](#rfc9256-4-7) The SR Algorithm (refer to Section 3.1.1 of [RFC8402]), the SRv6 SID behavior (as defined in [RFC8986] or other SRv6 specifications), and structure (as defined in [RFC8986]) MAY also be provided. (§4) | {gap} | this sentence applies to each of segment types I, J and K. Their optional SR Algorithm, SRv6 SID behavior and SID structure are not implemented by the SR Policy originator: internal/component/bgp/plugins/nlri/srpolicy/config.go::parseSegmentList accepts only type-a and type-b, and buildSegmentSubSubTLV emits only those types. Type B endpoint-behavior support does not supply any of the missing I/J/K fields. The absent optional segment types are not authorized for implementation by this audit. |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC9256-2.1-1`](#rfc9256-2.1-1)

An SR Policy MUST be identified through the tuple <Headend, Color, Endpoint> (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-2.1-1, so there is no linked test.

### [`RFC9256-2.1-2`](#rfc9256-2.1-2)

In the context of a specific headend, an SR Policy MUST be identified by the <Color, Endpoint> tuple (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC9256IdentificationTuple`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L48) | unit/verify | not verified |

### [`RFC9256-2.1-3`](#rfc9256-2.1-3)

The headend is specified as an IPv4 or IPv6 address and MUST resolve to a unique node in the SR domain (§2.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-2.1-3, so there is no linked test.

### [`RFC9256-2.2-1`](#rfc9256-2.2-1)

The endpoints of the constituent SR Policies and the parent SR Policy MUST be identical (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-2.2-1, so there is no linked test.

### [`RFC9256-2.2-2`](#rfc9256-2.2-2)

The colors of each of the constituent SR Policies and the parent SR Policy MUST be different (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-2.2-2, so there is no linked test.

### [`RFC9256-2.2-3`](#rfc9256-2.2-3)

The constituent SR Policies MUST NOT use composite candidate paths (§2.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-2.2-3, so there is no linked test.

### [`RFC9256-2.4-1`](#rfc9256-2.4-1)

If 2-byte ASNs are in use, the low-order 16 bits MUST be used, and the high-order bits MUST be set to 0. (§2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-2.4-1, so there is no linked test.

### [`RFC9256-2.4-2`](#rfc9256-2.4-2)

IPv4 addresses MUST be encoded in the lowest 32 bits, and the high-order bits MUST be set to 0. (§2.4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-2.4-2, so there is no linked test.

### [`RFC9256-2.6-1`](#rfc9256-2.6-1)

The identity of a candidate path MUST be uniquely established in the context of an SR Policy <Headend, Color, Endpoint> to handle add, delete, or modify operations on them in an unambiguous manner regardless of their source(s). (§2.6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-2.6-1, so there is no linked test.

### [`RFC9256-2.6-3`](#rfc9256-2.6-3)

Such symbolic names MUST NOT be considered as identifiers for a candidate path. (§2.6)

Audit verdict: enforced (the tests do what the requirement demands), fresh. TestRFC9256SymbolicNamesAreNotIdentifiers drives parseConfigRoute: differing policy and candidate-path names give byte-identical NLRI keys while the names still reach the Tunnel Encapsulation attribute (positive), and a shared name with a different distinguisher gives distinct keys (negative), so the name never takes part in identity.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC9256SymbolicNamesAreNotIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L117) | unit/verify | not verified |
| positive | [`TestRFC9256SymbolicNamesAreNotIdentifiers`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L106) | unit/verify | not verified |

### [`RFC9256-2.9-1`](#rfc9256-2.9-1)

Whenever a new path is learned or an active path is deleted, the validity of an existing path changes, or an existing path is changed, the selection process MUST be re-executed. (§2.9)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-2.9-1, so there is no linked test.

### [`RFC9256-2.11-1`](#rfc9256-2.11-1)

Only the active candidate path MUST be used for forwarding traffic that is being steered onto that policy except for certain scenarios such as fast reroute where a backup candidate path may be used as described in Section 9.3. (§2.11)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-2.11-1, so there is no linked test.

### [`RFC9256-4-6`](#rfc9256-4-6)

When building the MPLS label stack or the SRv6 SID list from the segment list, the node instantiating the policy MUST interpret the set of Segments as follows: * The first Segment represents the topmost MPLS label or the first SRv6 SID. It identifies the active segment the traffic will be directed toward along the explicit SR path. * The last segment represents the bottommost MPLS label or the last SRv6 SID the traffic will be directed toward along the explicit SR path. (§4)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-4-6, so there is no linked test.

### [`RFC9256-5.1-2`](#rfc9256-5.1-2)

A segment list of an explicit candidate path MUST be declared invalid when any of the following is true: * It is empty. * Its weight is 0. * It comprises a mix of SR-MPLS and SRv6 segment types. * The headend is unable to perform path resolution for the first SID into one or more outgoing interface(s) and next-hop(s). * The headend is unable to perform SID resolution for any non-first SID of type C through K into an MPLS label or an SRv6 SID. * The headend verification fails for any SID for which verification has been explicitly requested. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-5.1-2, so there is no linked test.

### [`RFC9256-5.1-4`](#rfc9256-5.1-4)

Types A or B MUST be used for the SIDs for which the reachability cannot be verified. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. ze verifies no SID reachability, so every SID must be Type A or B; TestRFC9256SegmentTypesAreAOrB shows parseSegmentList and buildSegmentSubSubTLV emit only sub-TLV 1 and 13 and that type-c, type-d, type-i and type-k are refused; the row carries its single-polarity marker.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestRFC9256SegmentTypesAreAOrB`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/srpolicy/rfc9256_test.go#L130) | unit/verify | not verified |

### [`RFC9256-5.1-5`](#rfc9256-5.1-5)

Note that the first SID MUST always be reachable regardless of its type. (§5.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-5.1-5, so there is no linked test.

### [`RFC9256-5.2-1`](#rfc9256-5.2-1)

If no solution is found to the optimization objective and constraints, then the dynamic candidate path MUST be declared invalid. (§5.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-5.2-1, so there is no linked test.

### [`RFC9256-6.1-3`](#rfc9256-6.1-3)

Candidate paths of different SR Policies MUST NOT have the same BSID (§6.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-6.1-3, so there is no linked test.

### [`RFC9256-6.2-4`](#rfc9256-6.2-4)

When the specified BSID is not available (optionally is not in the SRLB), an alert message MUST be generated via mechanisms like syslog. (§6.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-6.2-4, so there is no linked test.

### [`RFC9256-6.2.3-3`](#rfc9256-6.2.3-3)

When this restrictive behavior is enabled, if the candidate path has an unspecified BSID or if the specified BSID is not available when the candidate path becomes active, then no BSID is bound to it and the candidate path is considered invalid. An alert MUST be triggered for this error via mechanisms like syslog. (§6.2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-6.2.3-3, so there is no linked test.

### [`RFC9256-6.2.3-4`](#rfc9256-6.2.3-4)

Other candidate paths MUST then be evaluated for becoming the active candidate path. (§6.2.3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-6.2.3-4, so there is no linked test.

### [`RFC9256-7-1`](#rfc9256-7-1)

The SR Policy state MUST also reflect the reason when a policy and/or its candidate path is not active due to validation errors or not being preferred (§7)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC9256-7-1, so there is no linked test.

### [`RFC9256-4-7`](#rfc9256-4-7)

The SR Algorithm (refer to Section 3.1.1 of [RFC8402]), the SRv6 SID behavior (as defined in [RFC8986] or other SRv6 specifications), and structure (as defined in [RFC8986]) MAY also be provided. (§4)

Audit verdict: unimplemented (no code path enforces the requirement), fresh. Re-judged 2026-10-14 (independent auditor) after parseSegmentList gained the 20-bit type-a label check. Gap unchanged: parseSegmentList still accepts only type-a and type-b ('unknown segment type' otherwise) and buildSegmentSubSubTLV emits only Types A and B, so the optional SR Algorithm, SRv6 SID behavior and structure of segment types I, J and K are not produced. Type B endpoint-behavior support is RFC9256-4-2, not evidence here. No tag; the gap authorizes no implementation.

No test carries RFC9256-4-7, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | rfc2119 |
| Source | rfc/full/rfc9256.txt |
| Source fingerprint | 765585ef86ff8d01 |
| Record | rfc/extraction/rfc9256.json |
| Mapped sentences | 22 |
| Declined as scope | 0 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 0 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `2.1` | not stated | 3 | reviewed | not stated |
| `2.2` | not stated | 3 | reviewed | not stated |
| `2.3` | not stated | 0 | reviewed | not stated |
| `2.4` | not stated | 2 | reviewed | not stated |
| `2.5` | not stated | 0 | reviewed | not stated |
| `2.6` | not stated | 2 | reviewed | not stated |
| `2.7` | not stated | 0 | reviewed | not stated |
| `2.8` | not stated | 0 | reviewed | not stated |
| `2.9` | not stated | 1 | reviewed | not stated |
| `2.10` | not stated | 0 | reviewed | not stated |
| `2.11` | not stated | 1 | reviewed | not stated |
| `2.12` | not stated | 0 | reviewed | not stated |
| `2.13` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 1 | reviewed | not stated |
| `4.1` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 3 | reviewed | not stated |
| `5.2` | not stated | 1 | reviewed | not stated |
| `5.3` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `6.1` | not stated | 1 | reviewed | not stated |
| `6.2` | not stated | 1 | reviewed | not stated |
| `6.2.1` | not stated | 0 | reviewed | not stated |
| `6.2.2` | not stated | 0 | reviewed | not stated |
| `6.2.3` | not stated | 2 | reviewed | not stated |
| `6.3` | not stated | 0 | reviewed | not stated |
| `6.4` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 1 | reviewed | not stated |
| `8` | not stated | 0 | reviewed | not stated |
| `8.1` | not stated | 0 | reviewed | not stated |
| `8.2` | not stated | 0 | reviewed | not stated |
| `8.3` | not stated | 0 | reviewed | not stated |
| `8.4` | not stated | 0 | reviewed | not stated |
| `8.4.1` | not stated | 0 | reviewed | not stated |
| `8.5` | not stated | 0 | reviewed | not stated |
| `8.5.1` | not stated | 0 | reviewed | not stated |
| `8.6` | not stated | 0 | reviewed | not stated |
| `8.7` | not stated | 0 | reviewed | not stated |
| `8.8` | not stated | 0 | reviewed | not stated |
| `8.8.1` | not stated | 0 | reviewed | not stated |
| `8.8.2` | not stated | 0 | reviewed | not stated |
| `8.8.3` | not stated | 0 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |
| `9.3` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 0 | reviewed | not stated |
| `12` | not stated | 0 | reviewed | not stated |
| `12.1` | not stated | 0 | reviewed | not stated |
| `13` | not stated | 0 | reviewed | not stated |
| `13.1` | not stated | 0 | reviewed | not stated |
| `13.2` | not stated | 0 | reviewed | not stated |

### Excluded sentences

The review of RFC 9256 excluded no sentences: every sentence identified as an obligation is linked to a requirement.

## Superseded

No document obsoletes RFC 9256, so its obligations are stated where they were written.
