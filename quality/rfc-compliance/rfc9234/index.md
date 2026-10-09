# RFC 9234 - Route Leak Prevention and Detection Using Roles in UPDATE and OPEN Messages

Supported. Every requirement this repository extracted from RFC 9234, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 68.4% | 13 of 19 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 15.8% | 3 of 19 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 19 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0.0% | 0 of 19 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| No test at all | 0.0% | 0 of 19 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 24.2% | 16 of 66 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 19 | of 24 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 19 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 15.8% | 3 of 19 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 19 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 19 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Audit verdicts | 17 | of 19 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 19 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 24 |
| Gated MUST-level | 19 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 66 |
| Tagged units | 66 |
| Recorded audit verdicts | 17 |
| Discrimination records | 16 |
| Summary | `rfc/short/rfc9234.md` |
| Requirement shard | `rfc/requirements/rfc9234.md` |
| RFC text | `rfc/full/rfc9234.txt` |

## Enrolment

Enrolled: BGP Open Policy / Roles + Only-To-Customer OTC attribute (RFC 9234): role plugin. 13 MET (Role capability advertise, Table-2 pair correspondence + code-2/subcode-11 Role Mismatch, OTC ingress leak rules for Customer/RS-Client/Peer, ingress/egress stamping, no-propagate-upstream, preserve-unchanged, unicast-only, Gao-Rexford prohibition, malformed-OTC treat-as-withdraw) + 3 single-polarity positive (one Role capability per peer, egress stamp uses internet-facing local AS, OTC procedures non-overridable) + 3 not-applicable (no AS Confederation, no Complex peering)

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

Role capability negotiation, role mismatch NOTIFICATION, OTC egress stamping, OTC ingress leak detection and treat-as-withdraw, unicast-only (AFI 1/2, SAFI 1) OTC scoping read from MP_REACH_NLRI or, for a withdrawal, MP_UNREACH_NLRI. Both stamping rules are conditioned on the UPDATE advertising reachable NLRI, per Section 5's "if a route is to be advertised" / "if a route is received", so a withdrawal, an MP_UNREACH-only UPDATE and an End-of-RIB marker (both RFC 4724 encodings, which an added attribute would stop being a marker at all) are never stamped ([`internal/component/bgp/plugins/role/otc.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc.go) `payloadAdvertisesNLRI`, `isPayloadUnicast`).

**What the ledger says remains**

No known gap. The coverage gap disclosed here until 2026-08-05 is closed: [`test/plugin/role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci) asserts byte for byte that a withdraw-only UPDATE relayed to a Customer leaves the wire with `attrLen=0000` and no attribute of any code, [`test/plugin/role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci) does the same for an End-of-RIB marker, and `test/interop/scenarios/bgp-role-otc-withdraw-frr` runs a conforming FRR 10.3.1 receiver against the shape RFC 7606 Section 5.2 escalates to "session reset". The producer all three drive is `payloadAdvertisesNLRI` in [`internal/component/bgp/plugins/role/otc.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc.go).

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the gated population |
| Annotated (including scoped evidence) | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **19** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC9234-4.1-1`](#rfc9234-4.1-1), [`RFC9234-4.2-1`](#rfc9234-4.2-1), [`RFC9234-4.2-2`](#rfc9234-4.2-2), [`RFC9234-4.2-3`](#rfc9234-4.2-3), [`RFC9234-5-1`](#rfc9234-5-1), [`RFC9234-5-2`](#rfc9234-5-2), [`RFC9234-5-3`](#rfc9234-5-3), [`RFC9234-5-4`](#rfc9234-5-4), [`RFC9234-5-5`](#rfc9234-5-5), [`RFC9234-5-6`](#rfc9234-5-6), [`RFC9234-5-10`](#rfc9234-5-10), [`RFC9234-3.1-1`](#rfc9234-3.1-1), [`RFC9234-5-12`](#rfc9234-5-12)

**Annotated (including scoped evidence) (6):** [`RFC9234-4.1-2`](#rfc9234-4.1-2), [`RFC9234-5-7`](#rfc9234-5-7), [`RFC9234-5-8`](#rfc9234-5-8), [`RFC9234-5-9`](#rfc9234-5-9), [`RFC9234-5-11`](#rfc9234-5-11), [`RFC9234-6-1`](#rfc9234-6-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9234-4.1-1` | If the BGP Role is locally configured, the eBGP speaker MUST advertise the BGP Role Capability in the BGP OPEN message. (S4.1) | MUST | 4.1 - BGP Role Capability | **positive:** `unit/verify` [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L288). **negative:** `unit/verify` [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L289). **positive:** `functional/verify` [`dynamic-peer-gets-group-role-capability.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/dynamic-peer-gets-group-role-capability.ci#L21) |
| `RFC9234-4.1-2` | An eBGP speaker MUST NOT advertise multiple versions of the BGP Role Capability. (S4.1) | MUST NOT | 4.1 - BGP Role Capability | **positive:** `unit/verify` [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L290). **negative:** no negative test. **positive:** `functional/verify` [`dynamic-peer-gets-group-role-capability.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/dynamic-peer-gets-group-role-capability.ci#L24). **{single-polarity}:** parseRoleContainer (internal/component/bgp/plugins/role/config.go:66) reads a single import role per peer and extractRoleCapabilities (config.go:213) emits exactly one CapabilityDecl per peer, so no code path can advertise multiple Role capabilities and only the exactly-one assertion is constructible |
| `RFC9234-4.2-1` | If the BGP Role Capability is advertised, and one is also received from the peer, the Roles MUST correspond to the relationships in Table 2. (S4.2) | MUST | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L21). **negative:** `unit/verify` [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L122). **negative:** `unit/verify` [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L65) |
| `RFC9234-4.2-2` | If the Roles do not correspond, the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11). (S4.2) | MUST | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestAskOpenValidatorsLeavesASilentPluginPending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L267). **positive:** `unit/verify` [`TestBroadcastValidateOpenRefusesAnUnansweredPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L223). **positive:** `unit/verify` [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L123). **positive:** `unit/verify` [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L66). **negative:** `unit/verify` [`TestBroadcastValidateOpenAcceptsAPeerWithNoPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L250). **negative:** `unit/verify` [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L22). **positive:** `functional/verify` [`role-mismatch-notification.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-mismatch-notification.ci#L4). **negative:** `functional/verify` [`peer-open-role-complement.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/peer-open-role-complement.ci#L4) |
| `RFC9234-4.2-3` | If multiple BGP Role Capabilities are received and not all of them have the same value, then the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11). (§4.2) | MUST | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestValidateOpenRolePair_MultipleDifferentRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L166). **negative:** `unit/verify` [`TestValidateOpenRolePair_MultipleSameRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L196). **positive:** `functional/verify` [`role-multiple-roles-notification.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-multiple-roles-notification.ci#L4). **negative:** `functional/verify` [`role-multiple-same-roles.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-multiple-same-roles.ci#L4) |
| `RFC9234-5-1` | If a route with the OTC Attribute is received from a Customer or an RS-Client, then it is a route leak and MUST be considered ineligible (see Section 3). (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L335). **positive:** `unit/verify` [`TestOTCIngressGateRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L81). **negative:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L345) |
| `RFC9234-5-2` | If a route with the OTC Attribute is received from a Peer (i.e., remote AS with a Peer Role) and the Attribute has a value that is not equal to the remote (i.e., Peer's) AS number, then it is a route leak and MUST be considered ineligible. (§5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L505). **negative:** `unit/verify` [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L497) |
| `RFC9234-5-3` | If a route is received from a Provider, a Peer, or an RS and the OTC Attribute is not present, then it MUST be added with a value equal to the AS number of the remote AS. (§5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L481). **negative:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L330). **negative:** `unit/verify` [`TestOTCIngressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1991). **negative:** `unit/verify` [`TestOTCIngressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1963) |
| `RFC9234-5-4` | If a route is to be advertised to a Customer, a Peer, or an RS- Client (when the sender is an RS), and the OTC Attribute is not present, then when advertising the route, an OTC Attribute MUST be added with a value equal to the AS number of the local AS. (§5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L964). **positive:** `unit/verify` [`TestOTCEgressStampsMixedWithdrawAndAnnounce`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1930). **positive:** `unit/verify` [`TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1127). **positive:** `unit/verify` [`TestRFC9234OTCStampedToEveryDownstreamDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L48). **negative:** `unit/verify` [`TestOTCEgressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1889). **negative:** `unit/verify` [`TestOTCEgressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1868). **negative:** `unit/verify` [`TestOTCEgressNoStampProvider`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1003). **negative:** `unit/verify` [`TestRFC9234OTCNotStampedOutsideTheDownstreamScope`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L67). **positive:** `functional/verify` [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L8). **positive:** `functional/verify` [`role-otc-rs-client-dest-stamp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-client-dest-stamp.ci#L12). **positive:** `functional/verify` [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L14). **negative:** `functional/verify` [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L11). **negative:** `functional/verify` [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L17). **positive:** `interop/nightly` [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L850). **negative:** `interop/nightly` [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L851) |
| `RFC9234-5-5` | If a route already contains the OTC Attribute, it MUST NOT be propagated to Providers, Peers, or RSes. (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1725). **negative:** `unit/verify` [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1726) |
| `RFC9234-5-6` | Once the OTC Attribute has been set, it MUST be preserved unchanged (this also applies to an AS Confederation). (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCAttrModHandlerExistingPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1513). **negative:** `unit/verify` [`TestOTCAttrModHandlerNewAttr`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1492) |
| `RFC9234-5-7` | If an OTC Attribute is added on egress from the AS Confederation, its value MUST equal the AS Confederation Identifier. (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not operate as an AS Confederation (no confederation-identifier or member-AS config exists anywhere in config or the role plugin); the egress OTC stamp at internal/component/bgp/plugins/role/otc.go:432 uses only dest.LocalAS, so there is no confederation egress boundary at which a confederation identifier could be stamped |
| `RFC9234-5-8` | Also, on egress from the AS Confederation, an UPDATE MUST NOT contain an OTC Attribute with a value corresponding to any Member-AS Number other than the AS Confederation Identifier. (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no AS Confederation membership (no member-AS or confederation-identifier config); the egress OTC stamp at internal/component/bgp/plugins/role/otc.go:432 uses only dest.LocalAS, so an UPDATE never carries a member-AS OTC value at a confederation boundary |
| `RFC9234-5-9` | On egress from the Internet- facing AS, the OTC Attribute MUST NOT contain a value other than the Internet-facing ASN. (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressStampLocalASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1338). **negative:** no negative test. **{single-polarity}:** the egress OTC stamp at internal/component/bgp/plugins/role/otc.go:432 uses dest.LocalAS, the effective per-peer internet-facing local AS supplied by the reactor at internal/component/bgp/reactor/peer_forward_facts.go:133, and no code path stamps any other value, so a wrong-ASN negative is not constructible |
| `RFC9234-5-10` | The described ingress and egress procedures are applicable only for the address families AFI 1 (IPv4) and AFI 2 (IPv6) with SAFI 1 (unicast) in both cases and MUST NOT be applied to other address families by default. (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestIsPayloadUnicastMPUnreachFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L2022). **positive:** `unit/verify` [`TestOTCEgressNonUnicastWithdrawalNotProcessed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L2065). **positive:** `unit/verify` [`TestOTCEgressUnicastOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1371). **positive:** `unit/verify` [`TestOTCNonUnicastWithdrawalSkipsOTCProcedures`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L2105). **negative:** `unit/verify` [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L965) |
| `RFC9234-5-11` | The operator MUST NOT have the ability to modify the procedures defined in this section. (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1727). **positive:** `unit/verify` [`TestRFC9234OTCProceduresSurviveOperatorSettings`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L87). **negative:** no negative test. **{single-polarity}:** checkOTCIngress (internal/component/bgp/plugins/role/otc.go:164) and OTCEgressFilter (otc.go:384) take no operator override, peerRoleConfig (config.go:17) exposes no disable flag, and the wire-bytes OTC suppression at otc.go:384 runs before and independent of the export policy, so the procedures cannot be switched off by configuration and a modifiable negative is not constructible |
| `RFC9234-6-1` | Roles MUST NOT be configured on an eBGP session with a Complex peering relationship. (S6) | MUST NOT | 6 - Additional Considerations | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no representation of a Complex peering relationship; parseRoleContainer (internal/component/bgp/plugins/role/config.go:66) parses one role per peer with no relationship-complexity classification, so there is no ze code path that could place a role on a complex session to guard against |
| `RFC9234-3.1-1` | Customer: MAY propagate any route learned from a Customer, or that is locally originated, to a Provider. All other routes MUST NOT be propagated. Route Server (RS): MAY propagate any available route to a Route Server Client (RS-Client). Route Server Client (RS-Client): MAY propagate any route learned from a Customer, or that is locally originated, to an RS. All other routes MUST NOT be propagated. Peer: MAY propagate any route learned from a Customer, or that is locally originated, to a Peer. All other routes MUST NOT be propagated. (§3.1) | MUST NOT | 3.1 - Peering Relationships | **positive:** `unit/verify` [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L661). **positive:** `unit/verify` [`TestOTCEgressSuppressProviderLearnedWithoutMeta`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1073). **negative:** `unit/verify` [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L671) |
| `RFC9234-5-12` | The OTC Attribute is considered malformed if the length value is not 4. An UPDATE message with a malformed OTC Attribute SHALL be handled using the approach of "treat-as-withdraw" [RFC7606]. (§5) | SHALL | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCIngressMalformedTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1775). **positive:** `unit/verify` [`TestRFC9234OTCOfAnyLengthButFourIsTreatedAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_malformed_test.go#L36). **negative:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L346). **negative:** `unit/verify` [`TestRFC9234OTCOfLengthFourIsNotTreatedAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_malformed_test.go#L56) |
| `RFC9234-4-1` | One of the Roles described below SHOULD be configured at the local AS for each eBGP session (see definitions in Section 3) based on the local AS's knowledge of its Role. (§4) | SHOULD | 4 - BGP Role | **positive:** no positive test. **negative:** no negative test |
| `RFC9234-4.2-4` | For backward compatibility, if the BGP Role Capability is sent but one is not received, the BGP Speaker SHOULD ignore the absence of the BGP Role Capability and proceed with session establishment. (§4.2) | SHOULD | 4.2 - Role Correctness | **positive:** no positive test. **negative:** no negative test |
| `RFC9234-6-2` | If multiple eBGP sessions can segregate the Complex peering relationship into eBGP sessions with normal peering relationships, BGP Roles SHOULD be used on each of the resulting eBGP sessions. (§6) | SHOULD | 6 - Additional Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9234-4.2-5` | An operator may choose to apply a "strict mode" in which the receipt of a BGP Role Capability from the remote AS is required. When operating in the "strict mode", if the BGP Role Capability is sent but one is not received, the connection is rejected using the Role Mismatch Notification (code 2, subcode 11). (§4.2) | MAY | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestValidateOpenStrictModeRefusesADynamicGroupMemberWithNoRole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L175). **negative:** `unit/verify` [`TestValidateOpenRolePair_NoPeerRole_NoStrict`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L112). **positive:** `functional/verify` [`role-strict-enforcement.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-strict-enforcement.ci#L4) |
| `RFC9234-5-13` | The BGP Role negotiation and OTC-Attribute-based procedures specified in this document are NOT RECOMMENDED to be used between autonomous systems in an AS Confederation [RFC5065]. (§5) | NOT RECOMMENDED | 5 - BGP Only to Customer (OTC) Attribute | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9234-5-7`](#rfc9234-5-7) If an OTC Attribute is added on egress from the AS Confederation, its value MUST equal the AS Confederation Identifier. (S5) | no test | no test carries this requirement id; annotated {not-applicable}: ze does not operate as an AS Confederation (no confederation-identifier or member-AS config exists anywhere in config or the role plugin); the egress OTC stamp at internal/component/bgp/plugins/role/otc.go:432 uses only dest.LocalAS, so there is no confederation egress boundary at which a confederation identifier could be stamped |
| [`RFC9234-5-8`](#rfc9234-5-8) Also, on egress from the AS Confederation, an UPDATE MUST NOT contain an OTC Attribute with a value corresponding to any Member-AS Number other than the AS Confederation Identifier. (S5) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no AS Confederation membership (no member-AS or confederation-identifier config); the egress OTC stamp at internal/component/bgp/plugins/role/otc.go:432 uses only dest.LocalAS, so an UPDATE never carries a member-AS OTC value at a confederation boundary |
| [`RFC9234-6-1`](#rfc9234-6-1) Roles MUST NOT be configured on an eBGP session with a Complex peering relationship. (S6) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no representation of a Complex peering relationship; parseRoleContainer (internal/component/bgp/plugins/role/config.go:66) parses one role per peer with no relationship-complexity classification, so there is no ze code path that could place a role on a complex session to guard against |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9234-4.1-1`](#rfc9234-4.1-1)

If the BGP Role is locally configured, the eBGP speaker MUST advertise the BGP Role Capability in the BGP OPEN message. (S4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive on the wire: dynamic-peer-gets-group-role-capability.ci asserts ze's whole OPEN octet for octet ending 09 01 01 (role rs configured); revert record on extractRoleCapabilities, plus a mutant record on extractPeerRoleConfigs for TestExtractRoleCapabilities_ParseBGPConfig. Negative (no, invalid or empty role: no capability 9) is a table case in that same unit, recorded 2026-09-29 by the judge (revert extractRoleCapabilities, observed red).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L289) | unit/verify | revert, verified |
| positive | [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L288) | unit/verify | mutant, verified |
| positive | [`dynamic-peer-gets-group-role-capability.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/dynamic-peer-gets-group-role-capability.ci#L21) | functional/verify | revert, verified |

### [`RFC9234-4.1-2`](#rfc9234-4.1-2)

An eBGP speaker MUST NOT advertise multiple versions of the BGP Role Capability. (S4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive, argument re-checked: parseRoleContainer reads one role per peer and extractRoleCapabilities emits one CapabilityDecl per peer. The .ci now asserts the OPEN ze sends octet for octet, so a second Role capability changes the length fields and goes red; revert record on extractRoleCapabilities. The marker cites config.go line numbers that have shifted; the argument holds.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L290) | unit/verify | unproven |
| positive | [`dynamic-peer-gets-group-role-capability.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/dynamic-peer-gets-group-role-capability.ci#L24) | functional/verify | revert, verified |

### [`RFC9234-4.2-1`](#rfc9234-4.2-1)

If the BGP Role Capability is advertised, and one is also received from the peer, the Roles MUST correspond to the relationships in Table 2. (S4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. non-compliant: accepting a pair outside Table 2. TestValidateOpenRolePair_InvalidPairs assert.False(output.Accept) over customer/customer, provider/provider, rs/rs, rsclient/rsclient, provider/rs, customer/peer, and TestValidateOpenRolePairRunsForADynamicGroupMember asserts RS/RS refused; positive TestValidateOpenRolePair_ValidPairs assert.True over all five Table 2 pairs

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L122) | unit/verify | unproven |
| negative | [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L65) | unit/verify | unproven |
| positive | [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L21) | unit/verify | unproven |

### [`RFC9234-4.2-2`](#rfc9234-4.2-2)

If the Roles do not correspond, the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11). (S4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive role-mismatch-notification.ci: ze Provider, ze-peer Provider, first message on the wire is NOTIFICATION 03020B and no EoR; mutant record removing ! from !isValidRolePair (revert cannot discriminate: a crashed plugin also fails closed with 2/11). Negative peer-open-role-complement.ci: same config, ze-peer answers Customer, EoR sent and 03020B rejected; revert record on validateOpenRolePair. Only the peer's role octet differs, so the pair isolates the correspondence check.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L22) | unit/verify | unproven |
| negative | [`TestBroadcastValidateOpenAcceptsAPeerWithNoPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L250) | unit/verify | unproven |
| negative | [`peer-open-role-complement.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/peer-open-role-complement.ci#L4) | functional/verify | revert, verified |
| positive | [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L123) | unit/verify | unproven |
| positive | [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L66) | unit/verify | unproven |
| positive | [`TestAskOpenValidatorsLeavesASilentPluginPending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L267) | unit/verify | unproven |
| positive | [`TestBroadcastValidateOpenRefusesAnUnansweredPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L223) | unit/verify | unproven |
| positive | [`role-mismatch-notification.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-mismatch-notification.ci#L4) | functional/verify | mutant, verified |

### [`RFC9234-4.2-3`](#rfc9234-4.2-3)

If multiple BGP Role Capabilities are received and not all of them have the same value, then the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11). (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive role-multiple-roles-notification.ci: Customer then Peer; Customer alone pairs with ze's Provider, so only the differing second value can refuse; wire 03020B, no EoR; mutant record on len(peerRoles) > 1. Negative role-multiple-same-roles.ci: Customer twice establishes (EoR, 03020B rejected); revert record. The unit pair in validate_test.go stays as plugin-tier cover.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateOpenRolePair_MultipleSameRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L196) | unit/verify | unproven |
| negative | [`role-multiple-same-roles.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-multiple-same-roles.ci#L4) | functional/verify | revert, verified |
| positive | [`TestValidateOpenRolePair_MultipleDifferentRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L166) | unit/verify | unproven |
| positive | [`role-multiple-roles-notification.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-multiple-roles-notification.ci#L4) | functional/verify | mutant, verified |

### [`RFC9234-5-1`](#rfc9234-5-1)

If a route with the OTC Attribute is received from a Customer or an RS-Client, then it is a route leak and MUST be considered ineligible (see Section 3). (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. non-compliant: OTC route from Customer or RS-Client accepted. TestCheckOTCIngress reject_customer_with_otc and reject_rs_client_with_otc assert otcRejectLeak; TestOTCIngressGateRunsForADynamicGroupMember asserts accept=false for an RS-Client member; negative accept_provider_has_otc/accept_rs_has_otc assert otcAccept

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L345) | unit/verify | unproven |
| positive | [`TestOTCIngressGateRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L81) | unit/verify | unproven |
| positive | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L335) | unit/verify | unproven |

### [`RFC9234-5-2`](#rfc9234-5-2)

If a route with the OTC Attribute is received from a Peer (i.e., remote AS with a Peer Role) and the Attribute has a value that is not equal to the remote (i.e., Peer's) AS number, then it is a route leak and MUST be considered ineligible. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. non-compliant: Peer route whose OTC differs from the Peer's AS accepted. TestOTCIngressFilter reject_peer_otc_mismatch asserts accept=false (OTC 65099 vs PeerAS 65003); negative accept_peer_otc_matches asserts accept=true and no rewrite; TestCheckOTCIngress also pins reject_peer_otc_wrong and zero ASN

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L497) | unit/verify | unproven |
| positive | [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L505) | unit/verify | unproven |

### [`RFC9234-5-3`](#rfc9234-5-3)

If a route is received from a Provider, a Peer, or an RS and the OTC Attribute is not present, then it MUST be added with a value equal to the AS number of the remote AS. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. non-compliant: a route without OTC from Provider, Peer or RS left unstamped or stamped with another ASN. TestOTCIngressFilter stamp_from_provider finds OTC == remote AS 65002 in the rewritten payload; TestCheckOTCIngress stamp_from_provider/peer/rs assert stampASN == remote ASN for each of the three roles. Negatives: Customer/RS-Client not stamped, pure withdrawal and MP_UNREACH-only not stamped

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L330) | unit/verify | unproven |
| negative | [`TestOTCIngressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1991) | unit/verify | unproven |
| negative | [`TestOTCIngressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1963) | unit/verify | unproven |
| positive | [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L481) | unit/verify | unproven |

### [`RFC9234-5-4`](#rfc9234-5-4)

If a route is to be advertised to a Customer, a Peer, or an RS- Client (when the sender is an RS), and the OTC Attribute is not present, then when advertising the route, an OTC Attribute MUST be added with a value equal to the AS number of the local AS. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive TestRFC9234OTCStampedToEveryDownstreamDestination: an NLRI-bearing route with no OTC toward Customer, Peer and RS-Client each gets exactly one set op, code 35, value local AS 65000; plus the existing .ci and FRR interop positives. Negatives TestRFC9234OTCNotStampedOutsideTheDownstreamScope: the same NLRI-bearing route toward Provider and RS gets no op (the destination gate refuses, not the empty-NLRI gate), and a route already carrying OTC toward a Customer gets no op. TestOTCEgressNoStampProvider was vacuous (no NLRI, refused before the destination gate) and now carries NLRI (D-15 correction, not a weakening). Revert records on OTCEgressFilter for all three.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9234OTCNotStampedOutsideTheDownstreamScope`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L67) | unit/verify | revert, verified |
| negative | [`TestOTCEgressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1889) | unit/verify | unproven |
| negative | [`TestOTCEgressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1868) | unit/verify | unproven |
| negative | [`TestOTCEgressNoStampProvider`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1003) | unit/verify | revert, verified |
| negative | [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L851) | interop/nightly | unproven |
| negative | [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L11) | functional/verify | unproven |
| negative | [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L17) | functional/verify | unproven |
| positive | [`TestRFC9234OTCStampedToEveryDownstreamDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L48) | unit/verify | revert, verified |
| positive | [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L964) | unit/verify | unproven |
| positive | [`TestOTCEgressStampsMixedWithdrawAndAnnounce`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1930) | unit/verify | unproven |
| positive | [`TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1127) | unit/verify | unproven |
| positive | [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L850) | interop/nightly | unproven |
| positive | [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L8) | functional/verify | unproven |
| positive | [`role-otc-rs-client-dest-stamp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-client-dest-stamp.ci#L12) | functional/verify | unproven |
| positive | [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L14) | functional/verify | unproven |

### [`RFC9234-5-5`](#rfc9234-5-5)

If a route already contains the OTC Attribute, it MUST NOT be propagated to Providers, Peers, or RSes. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. non-compliant: an OTC-carrying route sent to a Provider, Peer or RS. TestOTCEgressWireBytesCheck asserts OTCEgressFilter false for each of the three destinations with no source config, true for an OTC route to a Customer and for a no-OTC route to a Provider

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1726) | unit/verify | unproven |
| positive | [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1725) | unit/verify | unproven |

### [`RFC9234-5-6`](#rfc9234-5-6)

Once the OTC Attribute has been set, it MUST be preserved unchanged (this also applies to an AS Confederation). (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. non-compliant: an existing OTC value rewritten. TestOTCAttrModHandlerExistingPreserved plans OTC 65001 from source against a set op of 65000 and asserts the emitted value is 65001. Negative TestOTCAttrModHandlerNewAttr: no prior OTC, so the op's value is emitted. The confederation parenthetical has no ze code path (RFC9234-5-7/5-8 are not-applicable)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCAttrModHandlerNewAttr`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1492) | unit/verify | unproven |
| positive | [`TestOTCAttrModHandlerExistingPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1513) | unit/verify | unproven |

### [`RFC9234-5-7`](#rfc9234-5-7)

If an OTC Attribute is added on egress from the AS Confederation, its value MUST equal the AS Confederation Identifier. (S5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9234-5-7, so no unit is bound to it.

### [`RFC9234-5-8`](#rfc9234-5-8)

Also, on egress from the AS Confederation, an UPDATE MUST NOT contain an OTC Attribute with a value corresponding to any Member-AS Number other than the AS Confederation Identifier. (S5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9234-5-8, so no unit is bound to it.

### [`RFC9234-5-9`](#rfc9234-5-9)

On egress from the Internet- facing AS, the OTC Attribute MUST NOT contain a value other than the Internet-facing ASN. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. single-polarity positive. non-compliant: egress OTC carrying a value other than the local (internet-facing) AS. TestOTCEgressStampLocalASN asserts the stamped value is dest.LocalAS 64999, not src AS 65001 nor dest peer AS 65002

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOTCEgressStampLocalASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1338) | unit/verify | unproven |

### [`RFC9234-5-10`](#rfc9234-5-10)

The described ingress and egress procedures are applicable only for the address families AFI 1 (IPv4) and AFI 2 (IPv6) with SAFI 1 (unicast) in both cases and MUST NOT be applied to other address families by default. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. non-compliant: OTC procedures applied to a non AFI1/2 SAFI1 family. Egress stamp: TestOTCEgressUnicastOnly (ipv4 multicast, mods.Len()==0) and TestOTCEgressNonUnicastWithdrawalNotProcessed (VPNv4); egress rule 2: TestOTCNonUnicastWithdrawalSkipsOTCProcedures egress_rule_2_does_not_suppress; ingress rule 1: its ingress_rule_1_does_not_reject subtest; family gate: TestIsPayloadUnicastMPUnreachFamily. Negative TestOTCEgressStampMod stamps IPv4 unicast

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L965) | unit/verify | unproven |
| positive | [`TestIsPayloadUnicastMPUnreachFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L2022) | unit/verify | unproven |
| positive | [`TestOTCEgressNonUnicastWithdrawalNotProcessed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L2065) | unit/verify | unproven |
| positive | [`TestOTCEgressUnicastOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1371) | unit/verify | unproven |
| positive | [`TestOTCNonUnicastWithdrawalSkipsOTCProcedures`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L2105) | unit/verify | unproven |

### [`RFC9234-5-11`](#rfc9234-5-11)

The operator MUST NOT have the ability to modify the procedures defined in this section. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Single-polarity positive, argument re-checked: peerRoleConfig carries only role, strict and export; TestRFC9234OTCProceduresSurviveOperatorSettings sets strict on and export naming every role on the source configs (the export set the egress filter reads, srcCfg.resolvedExport) and shows ingress still stamps OTC 64500 from a Provider, still refuses an OTC route from a Customer, egress still withholds an OTC route from a Provider and still stamps 65000 toward a Customer. Revert record on checkOTCIngress. The marker's otc.go:164/:384 line citations are stale (now checkOTCIngress and OTCEgressFilter moved); the argument holds.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9234OTCProceduresSurviveOperatorSettings`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1727) | unit/verify | unproven |

### [`RFC9234-6-1`](#rfc9234-6-1)

Roles MUST NOT be configured on an eBGP session with a Complex peering relationship. (S6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9234-6-1, so no unit is bound to it.

### [`RFC9234-3.1-1`](#rfc9234-3.1-1)

Customer: MAY propagate any route learned from a Customer, or that is locally originated, to a Provider. All other routes MUST NOT be propagated. Route Server (RS): MAY propagate any available route to a Route Server Client (RS-Client). Route Server Client (RS-Client): MAY propagate any route learned from a Customer, or that is locally originated, to an RS. All other routes MUST NOT be propagated. Peer: MAY propagate any route learned from a Customer, or that is locally originated, to a Peer. All other routes MUST NOT be propagated. (§3.1)

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. the quote holds three MUST NOTs: Customer, RS-Client and Peer roles propagate only Customer-learned or local routes. TestOTCEgressFilter asserts suppression for Provider-learned->Provider, Peer-learned->Peer, Peer-learned->RS, RS-learned->Provider; TestOTCEgressSuppressProviderLearnedWithoutMeta for Provider-learned->Provider with no meta. Provider-learned->Peer/RS, RS-learned->Peer/RS and Peer-learned->Provider have no assertion (question 4). The same unit's untagged subtest src_role_rs_to_provider_accept asserts an RS-Client-learned route IS sent to a Provider, which the Customer rule's 'All other routes MUST NOT be propagated' forbids (question 1; see findings)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L671) | unit/verify | unproven |
| positive | [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L661) | unit/verify | unproven |
| positive | [`TestOTCEgressSuppressProviderLearnedWithoutMeta`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1073) | unit/verify | unproven |

### [`RFC9234-5-12`](#rfc9234-5-12)

The OTC Attribute is considered malformed if the length value is not 4. An UPDATE message with a malformed OTC Attribute SHALL be handled using the approach of "treat-as-withdraw" [RFC7606]. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive TestRFC9234OTCOfAnyLengthButFourIsTreatedAsWithdraw: OTC lengths 0, 5 and 8 from a Provider (attributes ORIGIN + OTC only, isolated) move 10.0.0.0/24 to withdrawn, clear the attributes and keep the message. Negative TestRFC9234OTCOfLengthFourIsNotTreatedAsWithdraw: length 4 accepted unchanged. Revert records on findOTC for both.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9234OTCOfLengthFourIsNotTreatedAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_malformed_test.go#L56) | unit/verify | revert, verified |
| negative | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L346) | unit/verify | unproven |
| positive | [`TestRFC9234OTCOfAnyLengthButFourIsTreatedAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_malformed_test.go#L36) | unit/verify | revert, verified |
| positive | [`TestOTCIngressMalformedTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1775) | unit/verify | unproven |

### [`RFC9234-4.2-5`](#rfc9234-4.2-5)

An operator may choose to apply a "strict mode" in which the receipt of a BGP Role Capability from the remote AS is required. When operating in the "strict mode", if the BGP Role Capability is sent but one is not received, the connection is rejected using the Role Mismatch Notification (code 2, subcode 11). (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Positive role-strict-enforcement.ci: strict true, ze-peer drops capability 9, first message is 03020B; mutant record cfg.strict -> false turns it red, so strict mode is what refuses. Negative TestValidateOpenRolePair_NoPeerRole_NoStrict: non-strict accepts a missing role; revert record. Note: the .ci session is iBGP (asn 1/1); the strict sentence is not eBGP-scoped in RFC 9234, so this does not void it. The .ci has a 10s budget and flaked once (journal row).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateOpenRolePair_NoPeerRole_NoStrict`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestValidateOpenStrictModeRefusesADynamicGroupMemberWithNoRole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L175) | unit/verify | unproven |
| positive | [`role-strict-enforcement.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-strict-enforcement.ci#L4) | functional/verify | mutant, verified |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6 phase 3, rfc9234 |
| Signed off | 2026-08-31 |
| Register | rfc2119 |
| Source | rfc/full/rfc9234.txt |
| Source fingerprint | 34079d5254c6a473 |
| Record | rfc/extraction/rfc9234.json |
| Mapped sentences | 19 |
| Declined as scope | 2 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | skipped (front-matter) | Title block, Status of This Memo, Copyright Notice, Abstract and Table of Contents. The Abstract restates section 1 and directs no speaker. |
| `1` | Introduction | 0 | walked | Introduction. Indicative prose: what a route leak is, with RFC 7908 named for the taxonomy, why configuration-based prevention is unchecked, and what this document adds. Its one modal, "An eBGP speaker may require the use of this capability and confirmation of the BGP Role with a neighbor for the BGP OPEN to succeed", is lowercase, so under this document's own section 2 it carries no RFC 2119 level; it previews the strict mode that section 4.2 states and that rfc/short/rfc9234.md carries as RFC9234-4.2-5. |
| `2` | Requirements Language | 0 | walked | Requirements Language. The BCP 14 key-words paragraph, which states that the key words bind only when they appear in all capitals. It tells a reader how to read the other sections and binds no speaker, which is why the derivation excludes it from the site inventory. |
| `3` | Terminology | 0 | walked | Terminology. Defines "local AS" and "remote AS", and imports RFC 4271's meaning for "route is ineligible" ("ineligible to be installed in Loc-RIB and will be excluded from the next phase of route selection"). Definitions, no directive. That imported meaning is what the ingress leak rejection owes: checkOTCIngress (internal/component/bgp/plugins/role/otc.go) answers otcRejectLeak rather than dropping the session. |
| `3.1` | Peering Relationships | 3 | walked | Peering Relationships. Five role bullets giving the Gao-Rexford propagation rules. Three identical sentences, "All other routes MUST NOT be propagated.", close the Customer, the RS-Client and the Peer bullet; rfc/short/rfc9234.md states one row, RFC9234-3.1-1, whose text covers all three roles, so the first site maps it and the two later ones are duplicates of it. The Provider and RS bullets, and every "MAY propagate" clause, are permissions rather than obligations. "A BGP speaker may apply policy to reduce what is announced", "Violation of the route propagation rules listed above may result in route leaks" and the closing pointer to section 5 are lowercase and indicative. |
| `4` | BGP Role | 0 | walked | BGP Role. No capitalised MUST-level site. Its one directive is the SHOULD to configure a Role at the local AS for each eBGP session, with the Complex peering exception, carried as the unsourced id below. The five allowed Role definitions are terminology, and "BGP Roles are mutually confirmed using the BGP Role Capability" is a pointer to sections 4.1 and 4.2. |
| `4.1` | BGP Role Capability | 2 | walked | BGP Role Capability. Capability Code 9, Length 1 and Table 1's role values 0 through 4 with 5-255 unassigned are value assignments, stated indicatively, and are carried by the Wire Formats and Constants tables of rfc/short/rfc9234.md rather than by a requirement row. Two capitalised sites, mapped below to RFC9234-4.1-1 and RFC9234-4.1-2. The closing sentence, "The error handling when multiple BGP Role Capabilities are received is described in Section 4.2", is a pointer. |
| `4.2` | Role Correctness | 3 | walked | Role Correctness. Three capitalised sites, mapped below to RFC9234-4.2-1 through RFC9234-4.2-3, plus Table 2's allowed pairs, which the sites reference rather than state. Three sentences the site scan cannot see: the backward-compatibility SHOULD to ignore a missing Role Capability and proceed (unsourced id below); the "strict mode" paragraph, written indicatively ("the connection is rejected using the Role Mismatch Notification") and carried as the MAY row RFC9234-4.2-5, whose local behaviour is the strict leaf defaulting to false in internal/component/bgp/plugins/role/yang/ze-role.yang; and "If an eBGP speaker receives multiple but identical BGP Role Capabilities with the same value in each, then the speaker considers them to be a single BGP Role Capability and proceeds [RFC5492]", which states no RFC 2119 level and defers the merge to RFC 5492, so it adds no obligation of this document. |
| `5` | BGP Only to Customer (OTC) Attribute | 12 | walked | BGP Only to Customer (OTC) Attribute. The document's main normative section: twelve capitalised MUST-level sites, all mapped below. Its remaining sentences are definition ("an optional transitive Path Attribute of the UPDATE message with Attribute Type Code 35 and a length of 4 octets", and "The OTC Attribute is considered malformed if the length value is not 4", which supplies the condition site 5:6 acts on), rationale (the leak prevention and detection paragraphs, and the early-adopter paragraph observing that the OTC value is the same whether the remote AS or the local AS sets it), and the NOT RECOMMENDED sentence about AS Confederations, which is the unsourced id below. Two lead-ins scope the sites rather than add a requirement: "The following ingress procedure applies to the processing of the OTC Attribute on route receipt" and "The following egress procedure applies to the processing of the OTC Attribute on route advertisement". Both are indicative, and both are why payloadAdvertisesNLRI (internal/component/bgp/plugins/role/otc.go) gates the two stamping rules on the UPDATE carrying reachable NLRI: an UPDATE that only withdraws, and an End-of-RIB marker, are neither a route received nor a route advertised. |
| `6` | Additional Considerations | 1 | walked | Additional Considerations. One capitalised site, mapped below to RFC9234-6-1. Its one further directive is the SHOULD to use Roles on each session once a Complex relationship is segregated, the unsourced id below. The rest is commentary: per-prefix policy as an alternative with no in-band check, the effect of an incorrect Role or OTC value, AS migration under RFC 7705 where a router sets OTC to the ASN it currently represents, and a pointer to RFC 7606 section 6 for the negative impacts of treat-as-withdraw. |
| `7` | IANA Considerations | 0 | skipped (iana) | IANA Considerations. Records Capability Code 9, the new "BGP Role Value" subregistry with Table 3, OPEN Message Error subcode 11 "Role Mismatch", the deprecation of subcodes 8-10, and Path Attribute code 35. Binds IANA, not a speaker. |
| `8` | Security Considerations | 0 | walked | Security Considerations. States that the RFC 4271 and RFC 4272 considerations apply, describes what a misconfigured Role does to prefix propagation, discourages strict mode as a default in lowercase ("Implementations with such default behavior are strongly discouraged"), and describes OTC removal by an on-path attacker and OTC addition by a Customer as threats BGPsec does not cover. No capitalised keyword and no countermeasure directed at a speaker. The discouraged default is met: the strict leaf defaults to false (internal/component/bgp/plugins/role/yang/ze-role.yang). |
| `9` | References | 0 | skipped (references) | References. The heading only; its entries are in 9.1 and 9.2. |
| `9.1` | not stated | 0 | skipped (references) | Normative References: RFC 2119, RFC 4271, RFC 5065, RFC 5492, RFC 7606, RFC 7908, RFC 8126, RFC 8174. |
| `9.2` | not stated | 0 | skipped (references) | Informative References: GAO-REXFORD, RFC 4272, RFC 7705, RFC 7938, RFC 8205. The Acknowledgments, Contributors and Authors' Addresses blocks that close the document fall in this section's body and state no obligation. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `3.1:2` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same sentence closing the RS-Client bullet. It restates for the RS-Client Role the prohibition RFC9234-3.1-1 already carries for all three Roles, and site 3.1:1 maps that id. OTCEgressFilter (internal/component/bgp/plugins/role/otc.go) enforces all three in one expression: a destination resolving to Provider, Peer or RS is refused a route whose source resolves to Customer, Peer or RS-Client. | All other routes MUST NOT be propagated. |
| `3.1:3` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | The same sentence closing the Peer bullet. It restates for the Peer Role the prohibition RFC9234-3.1-1 already carries for all three Roles, and site 3.1:1 maps that id. | All other routes MUST NOT be propagated. |

## Superseded

No document obsoletes RFC 9234, so its obligations are stated where they were written.
