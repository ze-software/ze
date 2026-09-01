# RFC 9234 - Route Leak Prevention and Detection Using Roles in UPDATE and OPEN Messages

Supported. Every requirement this repository extracted from RFC 9234, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 81.2% | 13 of 16 binding obligations | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 18.8% | 3 of 16 binding obligations | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 16 binding obligations | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 16 binding obligations | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 1.9% | 1 of 53 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 19 | of 24 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 3 | of 19 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. Scope, not coverage: it is in no share below |

The 4 shares marked as a part above are the whole of the 16 obligations that bind Ze: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 24 |
| Gated MUST-level | 19 |
| Obligations that bind Ze | 16 |
| Not applicable, so out of scope | 3 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 53 |
| Tagged units | 53 |
| Recorded audit verdicts | 0 |
| Discrimination records | 1 |
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
| Annotated instead of tested | 6 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **19** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC9234-4.1-1`](#rfc9234-4.1-1), [`RFC9234-4.2-1`](#rfc9234-4.2-1), [`RFC9234-4.2-2`](#rfc9234-4.2-2), [`RFC9234-4.2-3`](#rfc9234-4.2-3), [`RFC9234-5-1`](#rfc9234-5-1), [`RFC9234-5-2`](#rfc9234-5-2), [`RFC9234-5-3`](#rfc9234-5-3), [`RFC9234-5-4`](#rfc9234-5-4), [`RFC9234-5-5`](#rfc9234-5-5), [`RFC9234-5-6`](#rfc9234-5-6), [`RFC9234-5-10`](#rfc9234-5-10), [`RFC9234-3.1-1`](#rfc9234-3.1-1), [`RFC9234-5-12`](#rfc9234-5-12)

**Annotated instead of tested (6):** [`RFC9234-4.1-2`](#rfc9234-4.1-2), [`RFC9234-5-7`](#rfc9234-5-7), [`RFC9234-5-8`](#rfc9234-5-8), [`RFC9234-5-9`](#rfc9234-5-9), [`RFC9234-5-11`](#rfc9234-5-11), [`RFC9234-6-1`](#rfc9234-6-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9234-4.1-1` | If BGP Role is locally configured, eBGP speaker MUST advertise BGP Role Capability in OPEN (S4.1) | MUST | 4.1 - BGP Role Capability | **positive:** `unit/verify` [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/config_test.go#L288). **negative:** `unit/verify` [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/config_test.go#L289) |
| `RFC9234-4.1-2` | eBGP speaker MUST NOT advertise multiple versions of BGP Role Capability (S4.1) | MUST NOT | 4.1 - BGP Role Capability | **positive:** `unit/verify` [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/config_test.go#L290). **negative:** no negative test. **{single-polarity}:** parseRoleContainer (internal/component/bgp/plugins/role/config.go:66) reads a single import role per peer and extractRoleCapabilities (config.go:213) emits exactly one CapabilityDecl per peer, so no code path can advertise multiple Role capabilities and only the exactly-one assertion is constructible |
| `RFC9234-4.2-1` | If Role Capability advertised and received, Roles MUST correspond to Table 2 relationships (S4.2) | MUST | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L21). **negative:** `unit/verify` [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/dynamic_group_test.go#L122). **negative:** `unit/verify` [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L65) |
| `RFC9234-4.2-2` | If Roles do not correspond, MUST reject connection with Role Mismatch Notification (code 2, subcode 11) (S4.2) | MUST | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestAskOpenValidatorsLeavesASilentPluginPending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/validate_test.go#L267). **positive:** `unit/verify` [`TestBroadcastValidateOpenRefusesAnUnansweredPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/validate_test.go#L223). **positive:** `unit/verify` [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/dynamic_group_test.go#L123). **positive:** `unit/verify` [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L66). **negative:** `unit/verify` [`TestBroadcastValidateOpenAcceptsAPeerWithNoPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/validate_test.go#L250). **negative:** `unit/verify` [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L22) |
| `RFC9234-4.2-3` | If multiple Role Capabilities received with different values, MUST reject with Role Mismatch Notification (S4.2) | MUST | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestValidateOpenRolePair_MultipleDifferentRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L163). **negative:** `unit/verify` [`TestValidateOpenRolePair_MultipleSameRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L193) |
| `RFC9234-5-1` | Route with OTC received from Customer or RS-Client MUST be considered ineligible (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L335). **positive:** `unit/verify` [`TestOTCIngressGateRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/dynamic_group_test.go#L81). **negative:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L345) |
| `RFC9234-5-2` | Route with OTC from Peer where OTC value != Peer's ASN MUST be considered ineligible (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L505). **negative:** `unit/verify` [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L497) |
| `RFC9234-5-3` | Route from Provider/Peer/RS without OTC: OTC MUST be added with remote AS number (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L481). **negative:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L330). **negative:** `unit/verify` [`TestOTCIngressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1989). **negative:** `unit/verify` [`TestOTCIngressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1961) |
| `RFC9234-5-4` | Route to Customer/Peer/RS-Client without OTC: OTC MUST be added with local AS number (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L964). **positive:** `unit/verify` [`TestOTCEgressStampsMixedWithdrawAndAnnounce`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1928). **positive:** `unit/verify` [`TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1125). **negative:** `unit/verify` [`TestOTCEgressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1887). **negative:** `unit/verify` [`TestOTCEgressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1866). **negative:** `unit/verify` [`TestOTCEgressNoStampProvider`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1003). **positive:** `functional/verify` [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L8). **positive:** `functional/verify` [`role-otc-rs-client-dest-stamp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-client-dest-stamp.ci#L12). **positive:** `functional/verify` [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L14). **negative:** `functional/verify` [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L11). **negative:** `functional/verify` [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L17). **positive:** `interop/nightly` [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L857). **negative:** `interop/nightly` [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L858) |
| `RFC9234-5-5` | Route with OTC MUST NOT be propagated to Providers, Peers, or RSes (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1723). **negative:** `unit/verify` [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1724) |
| `RFC9234-5-6` | Once OTC Attribute is set, it MUST be preserved unchanged (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCAttrModHandlerExistingPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1511). **negative:** `unit/verify` [`TestOTCAttrModHandlerNewAttr`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1490) |
| `RFC9234-5-7` | OTC added on egress from AS Confederation MUST equal AS Confederation Identifier (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze does not operate as an AS Confederation (no confederation-identifier or member-AS config exists anywhere in config or the role plugin); the egress OTC stamp at internal/component/bgp/plugins/role/otc.go:432 uses only dest.LocalAS, so there is no confederation egress boundary at which a confederation identifier could be stamped |
| `RFC9234-5-8` | On egress from AS Confederation, UPDATE MUST NOT contain OTC with Member-AS Number other than Confederation Identifier (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no AS Confederation membership (no member-AS or confederation-identifier config); the egress OTC stamp at internal/component/bgp/plugins/role/otc.go:432 uses only dest.LocalAS, so an UPDATE never carries a member-AS OTC value at a confederation boundary |
| `RFC9234-5-9` | On egress from Internet-facing AS, OTC MUST NOT contain value other than Internet-facing ASN (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressStampLocalASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1336). **negative:** no negative test. **{single-polarity}:** the egress OTC stamp at internal/component/bgp/plugins/role/otc.go:432 uses dest.LocalAS, the effective per-peer internet-facing local AS supplied by the reactor at internal/component/bgp/reactor/peer_forward_facts.go:133, and no code path stamps any other value, so a wrong-ASN negative is not constructible |
| `RFC9234-5-10` | OTC procedures MUST NOT be applied to address families other than AFI 1/2, SAFI 1 by default (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestIsPayloadUnicastMPUnreachFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L2020). **positive:** `unit/verify` [`TestOTCEgressNonUnicastWithdrawalNotProcessed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L2063). **positive:** `unit/verify` [`TestOTCEgressUnicastOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1369). **positive:** `unit/verify` [`TestOTCNonUnicastWithdrawalSkipsOTCProcedures`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L2103). **negative:** `unit/verify` [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L965) |
| `RFC9234-5-11` | Operator MUST NOT have ability to modify ingress/egress OTC procedures (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1725). **negative:** no negative test. **{single-polarity}:** checkOTCIngress (internal/component/bgp/plugins/role/otc.go:164) and OTCEgressFilter (otc.go:384) take no operator override, peerRoleConfig (config.go:17) exposes no disable flag, and the wire-bytes OTC suppression at otc.go:384 runs before and independent of the export policy, so the procedures cannot be switched off by configuration and a modifiable negative is not constructible |
| `RFC9234-6-1` | Roles MUST NOT be configured on eBGP session with Complex peering relationship (S6) | MUST NOT | 6 - Additional Considerations | **positive:** no positive test. **negative:** no negative test. **{not-applicable}:** ze has no representation of a Complex peering relationship; parseRoleContainer (internal/component/bgp/plugins/role/config.go:66) parses one role per peer with no relationship-complexity classification, so there is no ze code path that could place a role on a complex session to guard against |
| `RFC9234-3.1-1` | Customer/RS-Client/Peer: routes from Provider/Peer/RS MUST NOT be propagated (S3.1) | MUST NOT | 3.1 - Peering Relationships | **positive:** `unit/verify` [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L661). **positive:** `unit/verify` [`TestOTCEgressSuppressProviderLearnedWithoutMeta`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1071). **negative:** `unit/verify` [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L671) |
| `RFC9234-5-12` | UPDATE with malformed OTC Attribute (length != 4) SHALL be handled as treat-as-withdraw (S5) | SHALL | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCIngressMalformedTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1773). **negative:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L346) |
| `RFC9234-4-1` | One of the defined Roles SHOULD be configured for each eBGP session (S4) | SHOULD | 4 - BGP Role | **positive:** no positive test. **negative:** no negative test |
| `RFC9234-4.2-4` | If Role Capability sent but not received, SHOULD ignore absence and proceed (S4.2) | SHOULD | 4.2 - Role Correctness | **positive:** no positive test. **negative:** no negative test |
| `RFC9234-6-2` | If Complex peering can be segregated into multiple sessions, BGP Roles SHOULD be used on each (S6) | SHOULD | 6 - Additional Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9234-4.2-5` | Operator may apply strict mode requiring Role capability from peer (S4.2) | MAY | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestValidateOpenStrictModeRefusesADynamicGroupMemberWithNoRole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/dynamic_group_test.go#L175). **negative:** no negative test |
| `RFC9234-5-13` | BGP Role negotiation and OTC procedures NOT RECOMMENDED between ASes in AS Confederation (S5) | NOT RECOMMENDED | 5 - BGP Only to Customer (OTC) Attribute | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9234-5-7`](#rfc9234-5-7) OTC added on egress from AS Confederation MUST equal AS Confederation Identifier (S5) | no test | no test carries this requirement id; annotated {not-applicable}: ze does not operate as an AS Confederation (no confederation-identifier or member-AS config exists anywhere in config or the role plugin); the egress OTC stamp at internal/component/bgp/plugins/role/otc.go:432 uses only dest.LocalAS, so there is no confederation egress boundary at which a confederation identifier could be stamped |
| [`RFC9234-5-8`](#rfc9234-5-8) On egress from AS Confederation, UPDATE MUST NOT contain OTC with Member-AS Number other than Confederation Identifier (S5) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no AS Confederation membership (no member-AS or confederation-identifier config); the egress OTC stamp at internal/component/bgp/plugins/role/otc.go:432 uses only dest.LocalAS, so an UPDATE never carries a member-AS OTC value at a confederation boundary |
| [`RFC9234-6-1`](#rfc9234-6-1) Roles MUST NOT be configured on eBGP session with Complex peering relationship (S6) | no test | no test carries this requirement id; annotated {not-applicable}: ze has no representation of a Complex peering relationship; parseRoleContainer (internal/component/bgp/plugins/role/config.go:66) parses one role per peer with no relationship-complexity classification, so there is no ze code path that could place a role on a complex session to guard against |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9234-4.1-1`](#rfc9234-4.1-1)

If BGP Role is locally configured, eBGP speaker MUST advertise BGP Role Capability in OPEN (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/config_test.go#L289) | unit/verify | unproven |
| positive | [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/config_test.go#L288) | unit/verify | mutant, verified |

### [`RFC9234-4.1-2`](#rfc9234-4.1-2)

eBGP speaker MUST NOT advertise multiple versions of BGP Role Capability (S4.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/config_test.go#L290) | unit/verify | unproven |

### [`RFC9234-4.2-1`](#rfc9234-4.2-1)

If Role Capability advertised and received, Roles MUST correspond to Table 2 relationships (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/dynamic_group_test.go#L122) | unit/verify | unproven |
| negative | [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L65) | unit/verify | unproven |
| positive | [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L21) | unit/verify | unproven |

### [`RFC9234-4.2-2`](#rfc9234-4.2-2)

If Roles do not correspond, MUST reject connection with Role Mismatch Notification (code 2, subcode 11) (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L22) | unit/verify | unproven |
| negative | [`TestBroadcastValidateOpenAcceptsAPeerWithNoPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/validate_test.go#L250) | unit/verify | unproven |
| positive | [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/dynamic_group_test.go#L123) | unit/verify | unproven |
| positive | [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L66) | unit/verify | unproven |
| positive | [`TestAskOpenValidatorsLeavesASilentPluginPending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/validate_test.go#L267) | unit/verify | unproven |
| positive | [`TestBroadcastValidateOpenRefusesAnUnansweredPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/validate_test.go#L223) | unit/verify | unproven |

### [`RFC9234-4.2-3`](#rfc9234-4.2-3)

If multiple Role Capabilities received with different values, MUST reject with Role Mismatch Notification (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateOpenRolePair_MultipleSameRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L193) | unit/verify | unproven |
| positive | [`TestValidateOpenRolePair_MultipleDifferentRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/validate_test.go#L163) | unit/verify | unproven |

### [`RFC9234-5-1`](#rfc9234-5-1)

Route with OTC received from Customer or RS-Client MUST be considered ineligible (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L345) | unit/verify | unproven |
| positive | [`TestOTCIngressGateRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/dynamic_group_test.go#L81) | unit/verify | unproven |
| positive | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L335) | unit/verify | unproven |

### [`RFC9234-5-2`](#rfc9234-5-2)

Route with OTC from Peer where OTC value != Peer's ASN MUST be considered ineligible (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L497) | unit/verify | unproven |
| positive | [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L505) | unit/verify | unproven |

### [`RFC9234-5-3`](#rfc9234-5-3)

Route from Provider/Peer/RS without OTC: OTC MUST be added with remote AS number (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L330) | unit/verify | unproven |
| negative | [`TestOTCIngressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1989) | unit/verify | unproven |
| negative | [`TestOTCIngressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1961) | unit/verify | unproven |
| positive | [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L481) | unit/verify | unproven |

### [`RFC9234-5-4`](#rfc9234-5-4)

Route to Customer/Peer/RS-Client without OTC: OTC MUST be added with local AS number (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCEgressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1887) | unit/verify | unproven |
| negative | [`TestOTCEgressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1866) | unit/verify | unproven |
| negative | [`TestOTCEgressNoStampProvider`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1003) | unit/verify | unproven |
| negative | [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L858) | interop/nightly | unproven |
| negative | [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L11) | functional/verify | unproven |
| negative | [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L17) | functional/verify | unproven |
| positive | [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L964) | unit/verify | unproven |
| positive | [`TestOTCEgressStampsMixedWithdrawAndAnnounce`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1928) | unit/verify | unproven |
| positive | [`TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1125) | unit/verify | unproven |
| positive | [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L857) | interop/nightly | unproven |
| positive | [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L8) | functional/verify | unproven |
| positive | [`role-otc-rs-client-dest-stamp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-client-dest-stamp.ci#L12) | functional/verify | unproven |
| positive | [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L14) | functional/verify | unproven |

### [`RFC9234-5-5`](#rfc9234-5-5)

Route with OTC MUST NOT be propagated to Providers, Peers, or RSes (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1724) | unit/verify | unproven |
| positive | [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1723) | unit/verify | unproven |

### [`RFC9234-5-6`](#rfc9234-5-6)

Once OTC Attribute is set, it MUST be preserved unchanged (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCAttrModHandlerNewAttr`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1490) | unit/verify | unproven |
| positive | [`TestOTCAttrModHandlerExistingPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1511) | unit/verify | unproven |

### [`RFC9234-5-7`](#rfc9234-5-7)

OTC added on egress from AS Confederation MUST equal AS Confederation Identifier (S5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9234-5-7, so no unit is bound to it.

### [`RFC9234-5-8`](#rfc9234-5-8)

On egress from AS Confederation, UPDATE MUST NOT contain OTC with Member-AS Number other than Confederation Identifier (S5)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9234-5-8, so no unit is bound to it.

### [`RFC9234-5-9`](#rfc9234-5-9)

On egress from Internet-facing AS, OTC MUST NOT contain value other than Internet-facing ASN (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOTCEgressStampLocalASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1336) | unit/verify | unproven |

### [`RFC9234-5-10`](#rfc9234-5-10)

OTC procedures MUST NOT be applied to address families other than AFI 1/2, SAFI 1 by default (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L965) | unit/verify | unproven |
| positive | [`TestIsPayloadUnicastMPUnreachFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L2020) | unit/verify | unproven |
| positive | [`TestOTCEgressNonUnicastWithdrawalNotProcessed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L2063) | unit/verify | unproven |
| positive | [`TestOTCEgressUnicastOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1369) | unit/verify | unproven |
| positive | [`TestOTCNonUnicastWithdrawalSkipsOTCProcedures`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L2103) | unit/verify | unproven |

### [`RFC9234-5-11`](#rfc9234-5-11)

Operator MUST NOT have ability to modify ingress/egress OTC procedures (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1725) | unit/verify | unproven |

### [`RFC9234-6-1`](#rfc9234-6-1)

Roles MUST NOT be configured on eBGP session with Complex peering relationship (S6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9234-6-1, so no unit is bound to it.

### [`RFC9234-3.1-1`](#rfc9234-3.1-1)

Customer/RS-Client/Peer: routes from Provider/Peer/RS MUST NOT be propagated (S3.1)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L671) | unit/verify | unproven |
| positive | [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L661) | unit/verify | unproven |
| positive | [`TestOTCEgressSuppressProviderLearnedWithoutMeta`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1071) | unit/verify | unproven |

### [`RFC9234-5-12`](#rfc9234-5-12)

UPDATE with malformed OTC Attribute (length != 4) SHALL be handled as treat-as-withdraw (S5)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L346) | unit/verify | unproven |
| positive | [`TestOTCIngressMalformedTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/otc_test.go#L1773) | unit/verify | unproven |

### [`RFC9234-4.2-5`](#rfc9234-4.2-5)

Operator may apply strict mode requiring Role capability from peer (S4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestValidateOpenStrictModeRefusesADynamicGroupMemberWithNoRole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/dynamic_group_test.go#L175) | unit/verify | unproven |

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
