# RFC 5575 - Dissemination of Flow Specification Rules

BGP Flowspec (obsoleted by RFC 8955). Every requirement listed for RFC 5575, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 7 | of 9 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 1 | of 9 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 9 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 9 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Tests shown to detect a fault | 13 | 13 of 31 tests linked to requirements, 0 without a fault test and 5 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 1 | of 9 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 9 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 9 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 9 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze verified: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 9 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 9 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no test verification credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 9 | of 16 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 9 mandatory requirements: they add up to 9. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

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
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP Flowspec (obsoleted by RFC 8955) |
| Automatic checks | Included |
| Requirements | 16 |
| Mandatory requirements (MUST level) | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 1 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 31 |
| Tests linked to requirements | 31 |
| Recorded audit verdicts | 8 |
| Fault test records | 18 |
| Summary | `rfc/short/rfc5575.md` |
| Requirement table | `rfc/requirements/rfc5575.md` |
| RFC text | `rfc/full/rfc5575.txt` |

## Automatic checks

Included in automatic checks: BGP Flowspec (obsoleted by RFC 8955): 8 single-polarity positive (capability/family/component-ordering/reserved-bits) + 4 rollup onto the RFC 8955 Section 6 validation rows that restate them

## Published support status

**Area:** BGP Flowspec (obsoleted by RFC 8955)

**Test verification:** 3 of 9 MUST proven

**Evidence:** unit 3

**Unmet:** RFC5575-6-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 7 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 2 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 3 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **9** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (7):** [`RFC5575-4-1`](#rfc5575-4-1), [`RFC5575-4-2`](#rfc5575-4-2), [`RFC5575-4-3`](#rfc5575-4-3), [`RFC5575-4-4`](#rfc5575-4-4), [`RFC5575-4-8`](#rfc5575-4-8), [`RFC5575-5.1-1`](#rfc5575-5.1-1), [`RFC5575-8-1`](#rfc5575-8-1)

**Annotated (including scoped evidence) (2):** [`RFC5575-6-1`](#rfc5575-6-1), [`RFC5575-7-1`](#rfc5575-7-1)

**Derived from other rows (3):** [`RFC5575-6-2`](#rfc5575-6-2), [`RFC5575-6-3`](#rfc5575-6-3), [`RFC5575-6-4`](#rfc5575-6-4)

## Figures

- **MUST bar**: Ze verified 3, Tests awaiting verification 5, Not built 1
- **Overview bar**: Ze MUST verified 3, Tests awaiting verification 5, Not assessed 4, Not built 1

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 9 |
| MUST rows kept (the MUST bar) | 9 |
| Ze verified | 3 |
| OS verified | 0 |
| Tests awaiting verification | 5 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 1 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 4 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 4 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc5575.json](../../../data/rfc/rfc5575.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC5575-4-1` | Implementations wishing to exchange flow specification rules MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) as defined in RFC 4760 [RFC4760]. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L902). **positive:** `unit/verify` [`TestMultiprotocolApplicationsAdvertiseLiteralCodeOne`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/multiprotocol_application_code_test.go#L27). **positive:** `unit/verify` [`TestRFC5575OpenCapabilityMatchesTransmittedFlowFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5575_capability_wire_test.go#L21). **negative:** `unit/verify` [`TestMultiprotocolApplicationsRejectUnrecognizedCapabilityCode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/multiprotocol_application_code_test.go#L106). **negative:** `unit/verify` [`TestRFC5575OpenCapabilityMatchesTransmittedFlowFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5575_capability_wire_test.go#L22) |
| `RFC5575-4-2` | The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be the same as the one used to identify a particular application that uses this NLRI-type. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecIPv4Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L421). **positive:** `unit/verify` [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L647). **positive:** `unit/verify` [`TestRFC5575OpenCapabilityMatchesTransmittedFlowFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5575_capability_wire_test.go#L23). **negative:** `unit/verify` [`TestRFC5575OpenCapabilityMatchesTransmittedFlowFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5575_capability_wire_test.go#L24) |
| `RFC5575-4-3` | Flow specification components must follow strict type ordering. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1227). **positive:** `unit/verify` [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1270). **negative:** `unit/verify` [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1307) |
| `RFC5575-4-4` | A given component type may or may not be present in the specification, but if present, it MUST precede any component of higher numeric type value. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1228). **negative:** `unit/verify` [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1335) |
| `RFC5575-6-1` | BGP implementations MUST also enforce that the AS_PATH attribute of a route received via the External Border Gateway Protocol (eBGP) contains the neighboring AS in the left-most position of the AS_PATH attribute. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{gap}:** Ze does not enforce this first-AS check for Flow Specification; RFC9117 Section 7 makes the original rule OPTIONAL again, and the owner chose disclosure rather than implementing that optional check on 2026-10-02 |
| `RFC5575-6-2` | A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC8955-6-1; RFC 5575 is obsoleted and RFC 8955 Section 6 restates the feasibility check as its validation procedure, so the row holds exactly when RFC8955-6-1 does. **derived:** met |
| `RFC5575-6-3` | A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC8955-6-1; RFC 5575 is obsoleted and originator matching is condition (b) of the RFC 8955 Section 6 procedure, so the row holds exactly when RFC8955-6-1 does. **derived:** met |
| `RFC5575-6-4` | A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | MUST | 6 | **positive:** no positive test. **negative:** no negative test. **{rollup}:** RFC8955-6-1; RFC 5575 is obsoleted and the more-specific-route rule is condition (c) of the RFC 8955 Section 6 procedure, so the row holds exactly when RFC8955-6-1 does. **derived:** met |
| `RFC5575-7-1` | This extended community is encoded as a sequence of 5 zero bytes followed by the DSCP value encoded in the 6 least significant bits of 6th byte. (§7) | MUST | 7 | **positive:** `unit/verify` [`TestFlowSpecTrafficMarkingReservedBytesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1508). **negative:** no negative test. **{single-polarity}:** the traffic-marking community is emitted with literal zero reserved bytes and only the trailing DSCP octet varies (internal/component/bgp/plugins/nlri/flowspec/encode.go:124-127) |
| `RFC5575-4-8` | Whenever the corresponding application does not require Next-Hop information, this shall be encoded as a 0-octet length Next Hop in the MP_REACH_NLRI attribute and ignored on receipt. (§4) | MUST | 4 | **positive:** `unit/verify` [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L33). **positive:** `unit/verify` [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L34). **positive:** `unit/verify` [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L29). **positive:** `unit/verify` [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L17). **positive:** `unit/verify` [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L514). **negative:** `unit/verify` [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L34). **negative:** `unit/verify` [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L35). **negative:** `unit/verify` [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L30). **negative:** `unit/verify` [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L18). **negative:** `unit/verify` [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L515) |
| `RFC5575-5.1-1` | This ordering function must be such that it must not depend on the arrival order of the flow specification's rules and must be constant in the network. (§5.1) | MUST | 5.1 | **positive:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L13). **positive:** `unit/verify` [`TestRFC5575PrecedenceIndependentOfArrival`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc5575_arrival_order_test.go#L14). **negative:** `unit/verify` [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L14). **negative:** `unit/verify` [`TestRFC5575PrecedenceIndependentOfArrival`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc5575_arrival_order_test.go#L15) |
| `RFC5575-8-1` | The NLRI length field shall include both the 8 bytes of the Route Distinguisher as well as the subsequent flow specification. (§8) | MUST | 8 | **positive:** `unit/verify` [`TestFlowSpecVPNUpdateCountsRouteDistinguisher`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L36). **negative:** `unit/verify` [`TestFlowSpecVPNUpdateCountsRouteDistinguisher`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L37) |
| `RFC5575-3-1` | Standard BGP policy mechanisms, such as UPDATE filtering by NLRI prefix and community matching, SHOULD apply to the newly defined NLRI-type. (§3) | SHOULD | 3 | **positive:** no positive test. **negative:** no negative test |
| `RFC5575-9-1` | While this is an implementation-specific choice, implementations SHOULD provide: o A mechanism to log the packet header of filtered traffic. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5575-9-2` | While this is an implementation-specific choice, implementations SHOULD provide: o A mechanism to log the packet header of filtered traffic. o A mechanism to count the number of matches for a given flow specification rule. (§9) | SHOULD | 9 | **positive:** no positive test. **negative:** no negative test |
| `RFC5575-7-2` | Rather than attempting to define it here, this can be accomplished by mapping a user-defined community value to platform-/network-specific behavior via user configuration. (§7) | MAY | 7 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC5575-6-1`](#rfc5575-6-1) BGP implementations MUST also enforce that the AS_PATH attribute of a route received via the External Border Gateway Protocol (eBGP) contains the neighboring AS in the left-most position of the AS_PATH attribute. (§6) | {gap}, no test | Ze does not enforce this first-AS check for Flow Specification; RFC9117 Section 7 makes the original rule OPTIONAL again, and the owner chose disclosure rather than implementing that optional check on 2026-10-02 |
| [`RFC5575-6-2`](#rfc5575-6-2) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | no test | no test carries this requirement id; annotated {rollup}: RFC8955-6-1; RFC 5575 is obsoleted and RFC 8955 Section 6 restates the feasibility check as its validation procedure, so the row holds exactly when RFC8955-6-1 does |
| [`RFC5575-6-3`](#rfc5575-6-3) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. (§6) | no test | no test carries this requirement id; annotated {rollup}: RFC8955-6-1; RFC 5575 is obsoleted and originator matching is condition (b) of the RFC 8955 Section 6 procedure, so the row holds exactly when RFC8955-6-1 does |
| [`RFC5575-6-4`](#rfc5575-6-4) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | no test | no test carries this requirement id; annotated {rollup}: RFC8955-6-1; RFC 5575 is obsoleted and the more-specific-route rule is condition (c) of the RFC 8955 Section 6 procedure, so the row holds exactly when RFC8955-6-1 does |

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC5575-4-1`](#rfc5575-4-1)

Implementations wishing to exchange flow specification rules MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) as defined in RFC 4760 [RFC4760]. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-read RFC 5575 Section 4's complete capability paragraph against RFC 8955 Section 4 and the current summary/derived row; the superseded/restated RFC8955-4-1 annotation remains accurate and unchanged. TestIPv4FlowSpecNegotiatesMultiprotocolCapability is supplementary family-registration evidence; its constructed object and shared code constant are not an independent numeric-wire proof. Both tagged occurrences of TestRFC5575OpenCapabilityMatchesTransmittedFlowFamily retain real configuration-derived OPEN capabilities, matching and crossed 133/134 SAFIs, an actual AnnounceNLRIBatch, exactly one UPDATE with the complete expected MP_REACH payload in matched cases, and exact ErrNoPeersAcceptedFamily plus no connection bytes in crossed cases. The same negotiated families reach both Peer and Session; no input or assertion is weakened. These exchange controls establish conditional use of the advertised application, while the literal pair supplies independent numeric identity. The configured OPEN fixture's test-only Session.buildOpen delegates directly to the current production buildOpenFromSnapshot with one captured plugin snapshot. In production prepareOpen retains the built message with its settings and declarations in one preparedOpen; sendOpen writes that message, and prepareNegotiation/publishNegotiation retain the advertised local/remote family intersection. The obsolete production buildOpen code key is replaced by buildOpenFromSnapshot; all other existing code anchors and tagged covers remain. The literal positive independently walks parameter and capability TLVs from the configured OPEN, requiring exactly one literal code 1, length 4 and exact AFI/reserved/SAFI payload for each exercised application (1/133, 1/134, 16388/71), then requires application and unicast negotiation against a separately handwritten code-1 peer stream. The negative provides a valid code-1 unicast control and the application payload under code 250, requires successful parsing, first excludes the application, and then requires surviving unicast. Thus a shared codec constant or rejection of every capability cannot satisfy the pair. Existing recorded breaks target Multiprotocol.WriteTo emitting 250 and parseCapability recognizing 250; the latter must fail for acceptance of the bad application, not solely loss of the unicast control. This independent current-source judgment credits only the capability-advertisement obligation. Main's final native inventories at HEAD 606a3ba2 report all existing records current: four for RFC5575-4-1, five for RFC7752-3.2-1, two for RFC8955-4-1 and five for RFC9552-5.2-7, with no stale bindings. This continuation read all sixteen actual record objects and every tagged test and reached producer before stamping. The eight numeric-code records and eight partner records remain intact; four supplementary registration/core-positive tagged covers have no historical record and are judged from their assertions without inventing execution. This verdict covers this capability-advertisement requirement alone, not whole-RFC or additional-NLRI conformance.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMultiprotocolApplicationsRejectUnrecognizedCapabilityCode`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/multiprotocol_application_code_test.go#L106) | unit/verify | mutant, verified |
| negative | [`TestRFC5575OpenCapabilityMatchesTransmittedFlowFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5575_capability_wire_test.go#L22) | unit/verify | revert, verified |
| positive | [`TestIPv4FlowSpecNegotiatesMultiprotocolCapability`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/plugin_test.go#L902) | unit/verify | not verified |
| positive | [`TestMultiprotocolApplicationsAdvertiseLiteralCodeOne`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/multiprotocol_application_code_test.go#L27) | unit/verify | mutant, verified |
| positive | [`TestRFC5575OpenCapabilityMatchesTransmittedFlowFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5575_capability_wire_test.go#L21) | unit/verify | revert, verified |

### [`RFC5575-4-2`](#rfc5575-4-2)

The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be the same as the one used to identify a particular application that uses this NLRI-type. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independently re-judged against full RFC 5575 Section 4: The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be the same as the one used to identify a particular application that uses this NLRI-type. TestFlowSpecIPv4Basic pins IPv4 flow to AFI 1 / SAFI 133; TestFlowSpecVPNFamily pins VPNv4 flow to AFI 1 / SAFI 134 (its additional IPv6 assertion is not evidence for RFC5575). TestRFC5575OpenCapabilityMatchesTransmittedFlowFamily and its second cover build and parse the configured OPEN, negotiate each exact application through capability.Negotiate, publish that same negotiation to both peer and session, and call AnnounceNLRIBatch. Each matched case requires exactly one UPDATE and the complete MP_REACH value, including the literal AFI/SAFI and NLRI bytes. Each crossed-SAFI case requires ErrNoPeersAcceptedFamily and zero connection bytes. The fixture additions retain all old expectations and make the session/peer encoding context agree with the actual negotiation. The producer's exact-family AnnounceNLRIBatch gate and batch writer therefore remain meaningfully exercised. All four prior test keys are preserved by the native writer; the prior audit has no code anchors. The independent Code-1 numeric oracle is a separate RFC5575-4-1 question and is not claimed here. This is a source re-judgment, not a claim that this agent ran tests or observed a red; main must finish and inspect the pending native renewal observations before stamping.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC5575OpenCapabilityMatchesTransmittedFlowFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5575_capability_wire_test.go#L24) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestFlowSpecIPv4Basic`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L421) | unit/verify | not verified |
| positive | [`TestFlowSpecVPNFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L647) | unit/verify | not verified |
| positive | [`TestRFC5575OpenCapabilityMatchesTransmittedFlowFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc5575_capability_wire_test.go#L23) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC5575-4-3`](#rfc5575-4-3)

Flow specification components must follow strict type ordering. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16 after the codec move; units changed only by package qualification. RFC 5575 Section 4: "Flow specification components must follow strict type ordering." TestFlowSpecComponentsAscendingOrder adds types 5,12,3,1 and requires the re-parsed wire order 1,3,5,12 (red on insertion-order emission by core writeComponentsSorted); TestFlowSpecJoinsRepeatedTypeIntoOneComponent requires one Type 4 component with exact merged bytes; TestParseFlowSpecRefusesRepeatedComponentType requires ErrFlowSpecDuplicateType from core parseComponents on a repeated type 4, with the merged control accepted.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParseFlowSpecRefusesRepeatedComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1307) | unit/verify | not verified |
| positive | [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1227) | unit/verify | not verified |
| positive | [`TestFlowSpecJoinsRepeatedTypeIntoOneComponent`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1270) | unit/verify | not verified |

### [`RFC5575-4-4`](#rfc5575-4-4)

A given component type may or may not be present in the specification, but if present, it MUST precede any component of higher numeric type value. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16 after the codec move; units changed only by package qualification. RFC 5575 Section 4: "if present, it MUST precede any component of higher numeric type value." Positive asserts strictly ascending emitted types and the exact order 1,3,5,12; negative TestParseFlowSpecRefusesDescendingComponentType requires ErrFlowSpecTypeOrder from core parseComponents on 5-then-3, with the same components ascending accepted, so the refusal is isolated to order.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestParseFlowSpecRefusesDescendingComponentType`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1335) | unit/verify | not verified |
| positive | [`TestFlowSpecComponentsAscendingOrder`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1228) | unit/verify | not verified |

### [`RFC5575-6-1`](#rfc5575-6-1)

BGP implementations MUST also enforce that the AS_PATH attribute of a route received via the External Border Gateway Protocol (eBGP) contains the neighboring AS in the left-most position of the AS_PATH attribute. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5575-6-1, so there is no linked test.

### [`RFC5575-6-2`](#rfc5575-6-2)

A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5575-6-2, so there is no linked test.

### [`RFC5575-6-3`](#rfc5575-6-3)

A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5575-6-3, so there is no linked test.

### [`RFC5575-6-4`](#rfc5575-6-4)

A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC5575-6-4, so there is no linked test.

### [`RFC5575-7-1`](#rfc5575-7-1)

This extended community is encoded as a sequence of 5 zero bytes followed by the DSCP value encoded in the 6 least significant bits of 6th byte. (§7)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: non-zero bytes in the five octets before the DSCP, or the DSCP elsewhere. TestFlowSpecTrafficMarkingReservedBytesZero asserts ec[2..6]==0 and ec[7]==dscp with type 0x80/0x09; red on either. {single-polarity} marker present.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| positive | [`TestFlowSpecTrafficMarkingReservedBytesZero`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/nlri/flowspec/types_test.go#L1508) | unit/verify | not verified |

### [`RFC5575-4-8`](#rfc5575-4-8)

Whenever the corresponding application does not require Next-Hop information, this shall be encoded as a 0-octet length Next Hop in the MP_REACH_NLRI attribute and ignored on receipt. (§4)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-14 by an independent auditor who authored neither 08fe903803 nor the uncommitted phase 2 of spec-single-owner-lifecycle-2-bgp-session. RFC 5575 Section 4 read: "Whenever the corresponding application does not require Next-Hop information, this shall be encoded as a 0-octet length Next Hop in the MP_REACH_NLRI attribute and ignored on receipt." Unit read: TestFlowSpecOriginationOmitsConfiguredNextHop (both tags); its subtests now close the gate on the session the fixture installed (setTestInitialSync(1, true)) where the Peer flags were set, and drive runInitialSyncNow. 08fe903803 moved the initial-sync gate (sendingInitialRoutes) and the End-of-RIB owed mark (initialSyncEOROwed) from Peer onto Session. In the tagged units named stale it replaced peer.sendInitialRoutes() with peer.runInitialSyncNow(), which is Peer.sendInitialRoutes(p.testInitialSync()) (initial_sync_helpers_test.go): the same production sync body, run on the session the fixture installed (or one built with NewSession(p.settings) and published through establishedSession as enterEstablished does); it replaced peer.<flag>.Store(v) with peer.testInitialSync().<flag>.Store(v) or setTestInitialSync, and drainAndCloseQueueGate now takes that session. git diff 08fe903803^ against the working tree shows no assertion, expected byte, driven input or tag prose changed in any of them, each under the RFC-approved line 08fe903803 carries. Questions 1, 3 and 4 answer exactly as the earlier judgement below; question 2 still rests on the same producers, whose discrimination records are retaken by ./le rfc record after this stamp. Verdict unchanged: enforced. Earlier judgement follows. Re-judged 2026-10-24 (independent auditor, authored none of the change). RFC 5575 Section 4: "Whenever the corresponding application does not require Next-Hop information, this shall be encoded as a 0-octet length Next Hop in the MP_REACH_NLRI attribute and ignored on receipt." 238bdb62e9 (spec-refuse-instead-of-defaulting) changed PeerInfo/StructuredEvent PeerAS from uint32 to rpc.RemoteAS; the unit now writes rpc.RemoteASOf(N) where it wrote N, the same known peer AS, and no assertion, input byte or expected value changed (TestRFC8955NextHopIgnoredForFlowSpec, both tags). Verdict unchanged: enforced. Earlier judgement follows. RFC 5575 Section 4: "Whenever the corresponding application does not require Next-Hop information, this shall be encoded as a 0-octet length Next Hop in the MP_REACH_NLRI attribute and ignored on receipt." Both clauses remain covered. Producers message/update_build_plugin.go::MPReachCarriesNextHop and reactor/forward_next_hop.go::applyNextHopFamily force zero hop for non-redirect FlowSpec; payloadNextHop does not use the received FlowSpec address as a forwarding gate. plugins/rib/rib_bestchange.go::entryNextHopAddr returns no forwarding address for FlowSpec; rib_flowspec_validation_test.go::TestRFC8955NextHopIgnoredForFlowSpec feeds empty, self and non-address MP bytes alongside a legacy self NEXT_HOP, then requires eligible retained candidates and exactly the same selected NLRI/action event. The legacy condition matters: draft-simpson-idr-flowspec-redirect-02 Section 3 identifies the redirect action target in MP_REACH, so an application requiring that address is outside RFC 5575 zero-hop antecedent. RFC 8955 Section 4 instead says "When advertising Flow Specifications, the Length of the Next-Hop Network Address MUST be set to 0." and separately "The Network Address of the Next-Hop field MUST be ignored." Its unconditional transmit rule is stronger; the legacy superseded relationship is navigation, not permission to import the old exception into RFC 8955. No whole-successor-conformance claim follows from this enforced legacy row. Independently read every tag carrier: message/flowspec_wire_contract_test.go::TestFlowSpecUpdateIgnoresConfiguredNextHop, reactor/flowspec_origin_wire_test.go::TestFlowSpecOriginationOmitsConfiguredNextHop, reactor/flowspec_forward_wire_test.go::TestFlowSpecForwardingOmitsNextHop and protected reactor/flowspec_rs_wire_test.go::TestFlowSpecRouteServerOmitsNextHop. The changed forward fixture calls negotiateForwardTestFamilies, which uses capability.Negotiate on explicit MP capabilities and publishes the same negotiated intersection to peer.session and peer.negotiated; it no longer seeds only the peer facade. forwardViaRIB sends non-FlowSpec sections live and relays individual FlowSpec StoredRoute values through reactorAPIAdapter.RelayStoredRoute to forwardUpdateCore. Exact recipient MP_REACH equals AFI/SAFI/zero length/reserved/NLRI, with exact traffic-rate bytes, no substituted withdrawal and the expected message count, for all four families, four rewrite modes and empty/IPv4/IPv6 received fields. Config, API batch and both queued origination writers compare whole TCP frames, including RD/NLRI/actions, with absent/IPv4/IPv6 configured addresses. The protected route-server carrier drives real RIB authorization and recipient TCP and remains unchanged. Native freshness inputs identify pending forward-carrier renewals; this source rejudgment does not claim they have run or passed. Parent must renew applicable proofs and stamp only after integration.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L18) | unit/verify | not verified |
| negative | [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L515) | unit/verify | revert, verified |
| negative | [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L34) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L35) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| negative | [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L30) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestFlowSpecUpdateIgnoresConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L17) | unit/verify | not verified |
| positive | [`TestRFC8955NextHopIgnoredForFlowSpec`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/rib/rib_flowspec_validation_test.go#L514) | unit/verify | revert, verified |
| positive | [`TestFlowSpecForwardingOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_forward_wire_test.go#L33) | unit/verify | revert, verified |
| positive | [`TestFlowSpecOriginationOmitsConfiguredNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_origin_wire_test.go#L34) | unit/verify | revert, verified |
| positive | [`TestFlowSpecRouteServerOmitsNextHop`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/flowspec_rs_wire_test.go#L29) | unit/verify | revert, verified |

### [`RFC5575-5.1-1`](#rfc5575-5.1-1)

This ordering function must be such that it must not depend on the arrival order of the flow specification's rules and must be constant in the network. (§5.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: both units moved verbatim to internal/core/bgp/nlri/flowspec, only the family identifier changed to the test-local ipv4FlowSpec literal (AFI 1, SAFI 133). RFC 5575 Section 5.1: "This ordering function must be such that it must not depend on the arrival order of the flow specification's rules and must be constant in the network." TestFlowSpecPrecedenceFromWire requires Compare(a,b)<0, Compare(b,a)>0 and Compare(a,a)==0 over overlap, disjoint, component-type, operator-byte, received-width and extra-component pairs; TestRFC5575PrecedenceIndependentOfArrival parses independent copies in both arrival orders, sorts with production Compare and requires identical exact byte order. Compare (core compare.go) reads only component content, no identity or arrival metadata.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L14) | unit/verify | revert, verified |
| negative | [`TestRFC5575PrecedenceIndependentOfArrival`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc5575_arrival_order_test.go#L15) | unit/verify | revert, verified |
| positive | [`TestFlowSpecPrecedenceFromWire`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/compare_rfc8955_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestRFC5575PrecedenceIndependentOfArrival`](https://github.com/ze-software/ze/blob/main/internal/core/bgp/nlri/flowspec/rfc5575_arrival_order_test.go#L14) | unit/verify | revert, verified |

### [`RFC5575-8-1`](#rfc5575-8-1)

The NLRI length field shall include both the 8 bytes of the Route Distinguisher as well as the subsequent flow specification. (§8)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-16: the unit changed only in how the VPN family is named (family.Family{AFIIPv4, SAFIFlowSpecVPN} instead of the plugin constant). RFC 5575 Section 8: "The NLRI length field shall include both the 8 bytes of the Route Distinguisher as well as the subsequent flow specification." TestFlowSpecVPNUpdateCountsRouteDistinguisher builds a VPN UPDATE through the real message builder and requires the exact MP_REACH NLRI 13|RD|5-octet value (red on a length of 5) with SAFI 134, re-parses it, and requires the core ParseFlowSpecVPN to error when the length octet is reduced by 8.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestFlowSpecVPNUpdateCountsRouteDistinguisher`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L37) | unit/verify | not verified |
| positive | [`TestFlowSpecVPNUpdateCountsRouteDistinguisher`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/flowspec_wire_contract_test.go#L36) | unit/verify | not verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc5575.txt |
| Source fingerprint | f57a08b1d5469245 |
| Record | rfc/extraction/rfc5575.json |
| Mapped sentences | 9 |
| Declined as scope | 8 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | reviewed | not stated |
| `1` | not stated | 3 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `4` | not stated | 5 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `5.1` | not stated | 1 | reviewed | not stated |
| `6` | not stated | 3 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 1 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `10` | not stated | 0 | reviewed | not stated |
| `11` | not stated | 3 | reviewed | not stated |
| `12` | not stated | 0 | reviewed | not stated |
| `13` | not stated | 0 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Abstract prose describing what the document defines; the lowercase 'required' is part of the phrase 'such as what is required in order to mitigate (distributed) denial-of-service attacks', not an obligation on an implementation. | Additionally, it defines two applications of that encoding format: one that can be used to automate inter-domain coordination of traffic filtering, such as what is required in order to mitigate (distributed) denial-of-service attacks, and a second application to provide traffic filtering in the context of a BGP/MPLS VPN service. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction prose stating the document's own scope ('we define the required mechanisms'); no obligation on an implementation. | Additionally, we define the required mechanisms to utilize this definition to the problem of immediate concern to the authors: intra- and inter-provider distribution of traffic filtering rules to filter (distributed) denial-of-service (DoS) attacks. |
| `1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction sentence heading a descriptive list of technology components; 'required to address the class of problems' describes the problem space, not an implementation obligation. | The key technology components required to address the class of problems targeted by this document are: |
| `1:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction prose stating the document's own scope ('defines required protocol extensions'); the obligations themselves are in Sections 4, 6 and 7. | This specification defines required protocol extensions to address most common applications of IPv4 unicast and VPNv4 unicast filtering. |
| `6:1` | `cross-document` (never bound Ze): the obligation belongs to another document that this one only cites | The obligation to run path selection is RFC 4271's, which the same paragraph cites: 'The first step of the BGP Route Selection procedure (Section 9.1.2 of [RFC4271])'. This sentence only states that flow specification routes are not exempt from it. | Although the forwarding attributes of two routes for the same flow specification prefix may be the same, BGP is still required to perform its path selection algorithm in order to select the correct set of attributes to advertise. |
| `11:1` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | IANA Considerations: the sentence directs IANA in its administration of the 'Flow Spec Component Types' registry ('Types must be assigned and interpreted uniquely'). The producer is IANA, and Ze holds no code for that role and could hold none: it consumes code points and operates no code-point registry. | Types must be assigned and interpreted uniquely. |
| `11:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | An IANA allocation-policy table (Invalid value / Defined by this specification / Specification Required / First Come First Served), not a sentence stating an obligation on a BGP speaker. | +--------------+-------------------------------+ \| Range \| Policy \| +--------------+-------------------------------+ \| 0 \| Invalid value \| \| [1 .. 12] \| Defined by this specification \| \| [13 .. 127] \| Specification Required \| \| [128 .. 255] \| First Come First Served \| +--------------+-------------------------------+ |
| `11:3` | `binds-another-role` (never bound Ze): the obligation is addressed to a role Ze never acts as. Presumed wrong until justified: Ze rarely implements one side of a protocol, so the reason beside this row must name the role, show Ze never acts as it, and cite the producer that would. | IANA Considerations: the sentence states what a registering document must say, so the producer is the author of a future specification registering a flow component type under the 'Specification Required' policy. Ze holds no code for that role: it registers no new component type and publishes no such specification. | The specification of a particular "flow component type" must clearly identify what the criteria used to match packets forwarded by the router is. |

## Superseded

RFC 5575 is obsoleted by RFC 8955.

| Requirement | Disposition | Now stated at | Reason |
|---|---|---|---|
| [`RFC5575-4-1`](#rfc5575-4-1) Implementations wishing to exchange flow specification rules MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) as defined in RFC 4760 [RFC4760]. (§4) | restated | RFC8955-4-1 | RFC 8955 Section 4 keeps the sentence, that implementations wishing to exchange Flow Specification MUST use BGP's Capability Advertisement facility to exchange the Multiprotocol Extension Capability Code (Code 1) |
| [`RFC5575-4-2`](#rfc5575-4-2) The (AFI, SAFI) pair carried in the Multiprotocol Extension Capability MUST be the same as the one used to identify a particular application that uses this NLRI-type. (§4) | restated | RFC8955-4-2 | RFC 8955 Section 4 makes the pair explicit rather than leaving it to the application: (AFI 1, SAFI 133) for IPv4 Flow Specification and (AFI 1, SAFI 134) for VPNv4 Flow Specification |
| [`RFC5575-4-3`](#rfc5575-4-3) Flow specification components must follow strict type ordering. (§4) | restated | RFC8955-4.2-1 | the NLRI value encoding moved from Section 4 to Section 4.2, which keeps the strict type ordering by increasing numerical order |
| [`RFC5575-4-4`](#rfc5575-4-4) A given component type may or may not be present in the specification, but if present, it MUST precede any component of higher numeric type value. (§4) | restated | RFC8955-4.2-2 | RFC 8955 Section 4.2 keeps the rule that a component, if present, MUST precede any component of higher numeric type value |
| [`RFC5575-6-1`](#rfc5575-6-1) BGP implementations MUST also enforce that the AS_PATH attribute of a route received via the External Border Gateway Protocol (eBGP) contains the neighboring AS in the left-most position of the AS_PATH attribute. (§6) | dropped | not stated | RFC 8955 Section 6 restated this sentence, and RFC 9117 Section 4.2, which updates RFC 8955, replaces it with a comparison against the left-most AS of the best-match unicast route (row RFC9117-4.2-1) and Section 7 makes the original rule optional again, so the documents in force state no equivalent obligation; RFC8955-6-2 was retired 2026-10-02 (rfc/corrections/rfc8955.md) |
| [`RFC5575-6-2`](#rfc5575-6-2) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | restated | RFC8955-6-1 | RFC 8955 Section 6 keeps the feasibility rule and scopes it, adding that it applies in the absence of explicit configuration and that SAFI 133 validates against SAFI 1 while SAFI 134 validates against SAFI 128 |
| [`RFC5575-6-3`](#rfc5575-6-3) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. (§6) | restated | RFC8955-6-1 | originator matching is condition (b) of RFC 8955 Section 6; RFC 9117 Section 4.1 adds the local-domain controller alternative |
| [`RFC5575-6-4`](#rfc5575-6-4) A flow specification NLRI must be validated such that it is considered feasible if and only if: a) The originator of the flow specification matches the originator of the best-match unicast route for the destination prefix embedded in the flow specification. b) There are no more specific unicast routes, when compared with the flow destination prefix, that have been received from a different neighboring AS than the best-match unicast route, which has been determined in step a). (§6) | restated | RFC8955-6-1 | the more-specific-route rule is condition (c) of RFC 8955 Section 6 |
| [`RFC5575-7-1`](#rfc5575-7-1) This extended community is encoded as a sequence of 5 zero bytes followed by the DSCP value encoded in the 6 least significant bits of 6th byte. (§7) | restated | RFC8955-7.5-1 | the traffic-marking action moved from Section 7 to Section 7.5, which keeps the reserved bits at 0 on encoding and adds the receive half, that they MUST be ignored during decoding |
| [`RFC5575-4-8`](#rfc5575-4-8) Whenever the corresponding application does not require Next-Hop information, this shall be encoded as a 0-octet length Next Hop in the MP_REACH_NLRI attribute and ignored on receipt. (§4) | restated | RFC8955-4-3 | RFC 8955 §4 splits the sentence into "Length of the Next-Hop Network Address MUST be set to 0" (RFC8955-4-3) and "Network Address of the Next-Hop field MUST be ignored" (RFC8955-4-4) |
| [`RFC5575-5.1-1`](#rfc5575-5.1-1) This ordering function must be such that it must not depend on the arrival order of the flow specification's rules and must be constant in the network. (§5.1) | restated | RFC8955-5.1-1 | RFC 8955 §5.1 keeps the ordering function that "does not depend on the arrival order of the Flow Specification via BGP" |
| [`RFC5575-8-1`](#rfc5575-8-1) The NLRI length field shall include both the 8 bytes of the Route Distinguisher as well as the subsequent flow specification. (§8) | restated | RFC8955-8-2 | RFC 8955 §8 keeps "The NLRI length field shall include both the 8 octets of the Route Distinguisher as well as the subsequent Flow Specification NLRI value" |
| [`RFC5575-3-1`](#rfc5575-3-1) Standard BGP policy mechanisms, such as UPDATE filtering by NLRI prefix and community matching, SHOULD apply to the newly defined NLRI-type. (§3) | unextracted | §3 | RFC 8955 Section 3 states the obligation with a lowercase keyword, that standard BGP policy mechanisms such as UPDATE filtering by NLRI prefix and community matching must apply to the Flow specification defined NLRI-type. rfc/short/rfc8955.md declares no row for it |
| [`RFC5575-9-1`](#rfc5575-9-1) While this is an implementation-specific choice, implementations SHOULD provide: o A mechanism to log the packet header of filtered traffic. (§9) | restated | RFC8955-9-1 | RFC 8955 Section 9 keeps the SHOULD to provide a mechanism to log the packet header of filtered traffic |
| [`RFC5575-9-2`](#rfc5575-9-2) While this is an implementation-specific choice, implementations SHOULD provide: o A mechanism to log the packet header of filtered traffic. o A mechanism to count the number of matches for a given flow specification rule. (§9) | restated | RFC8955-9-2 | RFC 8955 Section 9 keeps the SHOULD to provide a mechanism to count the number of matches for a given Flow Specification rule |
| [`RFC5575-7-2`](#rfc5575-7-2) Rather than attempting to define it here, this can be accomplished by mapping a user-defined community value to platform-/network-specific behavior via user configuration. (§7) | unextracted | §7.6 | RFC 8955 gives the paragraph its own section, 7.6, and keeps it word for word, that a user-defined community value can be mapped to platform-specific or network-specific behavior via user configuration. rfc/short/rfc8955.md declares no row for it |
