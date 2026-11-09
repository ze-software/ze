# RFC 9234 - Route Leak Prevention and Detection Using Roles in UPDATE and OPEN Messages

BGP Role and OTC. Every requirement this repository extracted from RFC 9234, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 13 | of 19 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 2 | of 19 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| Partial proof; remaining gap | 0 | of 19 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Proven by a recorded break | 36 | 36 of 69 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 19 | of 24 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the counts beside it are what says how Ze stands |
| Out of scope | 0 | of 19 gated MUSTs | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as a Ze gap. Scope, not coverage: it is counted in the gated whole the cards on this page add up to |
| Not applicable | 0 | of 19 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Met below Ze | 0 | of 19 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 19 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to |
| Linux cannot do this | 0 | of 19 gated MUSTs | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as a Ze gap. It is counted in the gated whole the cards add up to and earns no proof credit |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| One polarity, unexcused | 1 | of 19 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 3 | of 19 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Audit verdicts | 17 | of 19 gated MUSTs judged | 1 weak, wrong or unimplemented, 1 no longer current. Each is named below under its own requirement id |

The 9 counts marked as a part above are the whole of the 19 gated MUSTs: they add up to 19. Proven by a recorded break counts TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | bad | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| No test at all | bad | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. It counts TAGGED UNITS, not obligations, so this count is not one of the parts above |
| Audit verdicts | bad | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public row | BGP Role and OTC |
| Enrolment | Enrolled |
| Requirements | 24 |
| Gated MUST-level | 19 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 3 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 74 |
| Tagged units | 69 |
| Recorded audit verdicts | 17 |
| Discrimination records | 36 |
| Summary | `rfc/short/rfc9234.md` |
| Requirement shard | `rfc/requirements/rfc9234.md` |
| RFC text | `rfc/full/rfc9234.txt` |

## Enrolment

Enrolled: BGP Open Policy / Roles + Only-To-Customer OTC attribute (RFC 9234): role plugin. 13 MET (Role capability advertise, Table-2 pair correspondence + code-2/subcode-11 Role Mismatch, OTC ingress leak rules for Customer/RS-Client/Peer, ingress/egress stamping, no-propagate-upstream, preserve-unchanged, unicast-only, Gao-Rexford prohibition, malformed-OTC treat-as-withdraw) + 3 single-polarity positive (one Role capability per peer, egress stamp uses internet-facing local AS, OTC procedures non-overridable) + 3 gaps (no AS Confederation for 5-7 and 5-8, no refusal of a role on a Complex peering for 6-1)

## What the public ledger says

**Area:** BGP Role and OTC

**Proof:** 11 of 19 MUST proven

**Evidence:** interop 1, functional 5, unit 5

**Unmet:** RFC9234-5-7, RFC9234-5-8, RFC9234-6-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 13 | one part of the gated population |
| Annotated (including scoped evidence) | 5 | one part of the gated population |
| One polarity only | 1 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **19** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (13):** [`RFC9234-4.1-1`](#rfc9234-4.1-1), [`RFC9234-4.2-1`](#rfc9234-4.2-1), [`RFC9234-4.2-2`](#rfc9234-4.2-2), [`RFC9234-4.2-3`](#rfc9234-4.2-3), [`RFC9234-5-1`](#rfc9234-5-1), [`RFC9234-5-2`](#rfc9234-5-2), [`RFC9234-5-3`](#rfc9234-5-3), [`RFC9234-5-4`](#rfc9234-5-4), [`RFC9234-5-5`](#rfc9234-5-5), [`RFC9234-5-6`](#rfc9234-5-6), [`RFC9234-5-10`](#rfc9234-5-10), [`RFC9234-3.1-1`](#rfc9234-3.1-1), [`RFC9234-5-12`](#rfc9234-5-12)

**Annotated (including scoped evidence) (5):** [`RFC9234-4.1-2`](#rfc9234-4.1-2), [`RFC9234-5-7`](#rfc9234-5-7), [`RFC9234-5-8`](#rfc9234-5-8), [`RFC9234-5-11`](#rfc9234-5-11), [`RFC9234-6-1`](#rfc9234-6-1)

**One polarity only (1):** [`RFC9234-5-9`](#rfc9234-5-9)

## Figures

- **MUST bar**: Ze proven 11, Ze correctness gap 8
- **Overview bar**: Ze MUST proven 11, SHOULD proven 1, SHOULD not decided 4, Ze correctness gap 8

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Not walked | 0 |
| Mandatory register | not complete |
| MUST rows | 19 |
| MUST rows kept (the MUST bar) | 19 |
| Ze proven | 11 |
| OS verified | 0 |
| OS testing gap | 0 |
| Ze correctness gap | 8 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 5 |
| SHOULD proven | 1 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD not ever | 0 |
| SHOULD not yet | 0 |
| SHOULD not decided | 4 |

The MUST bar counts the MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT rows of the walked RFCs that the stack owes: Ze proven means a recorded test break proves the row; a row a layer under Ze performs is OS verified when a test checks that layer on the state Ze installs, an OS testing gap until one does. The overview bar adds the MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT rows and the rows not owed, which do not bind Ze. An RFC that is not walked has no valid extraction sign-off, so its rows count in no figure. Every count is read from the RFC data files when the page loads.

Source: [data/rfc/rfc9234.json](../../../data/rfc/rfc9234.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC9234-4.1-1` | If the BGP Role is locally configured, the eBGP speaker MUST advertise the BGP Role Capability in the BGP OPEN message. (S4.1) | MUST | 4.1 - BGP Role Capability | **positive:** `unit/verify` [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L288). **negative:** `unit/verify` [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L289). **positive:** `functional/verify` [`dynamic-peer-gets-group-role-capability.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/dynamic-peer-gets-group-role-capability.ci#L21) |
| `RFC9234-4.1-2` | An eBGP speaker MUST NOT advertise multiple versions of the BGP Role Capability. (S4.1) | MUST NOT | 4.1 - BGP Role Capability | **positive:** `unit/verify` [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L290). **negative:** no negative test. **positive:** `functional/verify` [`dynamic-peer-gets-group-role-capability.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/dynamic-peer-gets-group-role-capability.ci#L24). **{single-polarity}:** parseRoleContainer (internal/component/bgp/plugins/role/config.go:66) reads a single import role per peer and extractRoleCapabilities (config.go:213) emits exactly one CapabilityDecl per peer, so no code path can advertise multiple Role capabilities and only the exactly-one assertion is constructible |
| `RFC9234-4.2-1` | If the BGP Role Capability is advertised, and one is also received from the peer, the Roles MUST correspond to the relationships in Table 2. (S4.2) | MUST | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L21). **negative:** `unit/verify` [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L123). **negative:** `unit/verify` [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L65) |
| `RFC9234-4.2-2` | If the Roles do not correspond, the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11). (S4.2) | MUST | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestAskOpenValidatorsLeavesASilentPluginPending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L269). **positive:** `unit/verify` [`TestBroadcastValidateOpenRefusesAnUnansweredPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L225). **positive:** `unit/verify` [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L124). **positive:** `unit/verify` [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L66). **negative:** `unit/verify` [`TestBroadcastValidateOpenAcceptsAPeerWithNoPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L252). **negative:** `unit/verify` [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L22). **positive:** `functional/verify` [`role-mismatch-notification.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-mismatch-notification.ci#L4). **negative:** `functional/verify` [`peer-open-role-complement.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/peer-open-role-complement.ci#L4) |
| `RFC9234-4.2-3` | If multiple BGP Role Capabilities are received and not all of them have the same value, then the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11). (§4.2) | MUST | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestValidateOpenRolePair_MultipleDifferentRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L166). **negative:** `unit/verify` [`TestValidateOpenRolePair_MultipleSameRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L196). **positive:** `functional/verify` [`role-multiple-roles-notification.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-multiple-roles-notification.ci#L4). **negative:** `functional/verify` [`role-multiple-same-roles.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-multiple-same-roles.ci#L4) |
| `RFC9234-5-1` | If a route with the OTC Attribute is received from a Customer or an RS-Client, then it is a route leak and MUST be considered ineligible (see Section 3). (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L247). **positive:** `unit/verify` [`TestOTCIngressGateRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L81). **negative:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L257) |
| `RFC9234-5-2` | If a route with the OTC Attribute is received from a Peer (i.e., remote AS with a Peer Role) and the Attribute has a value that is not equal to the remote (i.e., Peer's) AS number, then it is a route leak and MUST be considered ineligible. (§5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L417). **negative:** `unit/verify` [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L408) |
| `RFC9234-5-3` | If a route is received from a Provider, a Peer, or an RS and the OTC Attribute is not present, then it MUST be added with a value equal to the AS number of the remote AS. (§5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L395). **negative:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L242). **negative:** `unit/verify` [`TestOTCIngressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1964). **negative:** `unit/verify` [`TestOTCIngressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1935) |
| `RFC9234-5-4` | If a route is to be advertised to a Customer, a Peer, or an RS- Client (when the sender is an RS), and the OTC Attribute is not present, then when advertising the route, an OTC Attribute MUST be added with a value equal to the AS number of the local AS. (§5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L930). **positive:** `unit/verify` [`TestOTCEgressStampsMixedWithdrawAndAnnounce`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1902). **positive:** `unit/verify` [`TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1096). **positive:** `unit/verify` [`TestRFC9234OTCStampedToEveryDownstreamDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L53). **negative:** `unit/verify` [`TestOTCEgressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1861). **negative:** `unit/verify` [`TestOTCEgressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1840). **negative:** `unit/verify` [`TestOTCEgressNoStampProvider`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L970). **negative:** `unit/verify` [`TestRFC9234OTCNotStampedOutsideTheDownstreamScope`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L72). **positive:** `functional/verify` [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L8). **positive:** `functional/verify` [`role-otc-rs-client-dest-stamp-forked.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-client-dest-stamp-forked.ci#L22). **positive:** `functional/verify` [`role-otc-rs-client-dest-stamp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-client-dest-stamp.ci#L12). **positive:** `functional/verify` [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L14). **negative:** `functional/verify` [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L11). **negative:** `functional/verify` [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L17). **positive:** `interop/nightly` [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L893). **negative:** `interop/nightly` [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L894) |
| `RFC9234-5-5` | If a route already contains the OTC Attribute, it MUST NOT be propagated to Providers, Peers, or RSes. (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1700). **negative:** `unit/verify` [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1701) |
| `RFC9234-5-6` | Once the OTC Attribute has been set, it MUST be preserved unchanged (this also applies to an AS Confederation). (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCAttrModHandlerExistingPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1484). **negative:** `unit/verify` [`TestOTCAttrModHandlerNewAttr`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1463) |
| `RFC9234-5-7` | If an OTC Attribute is added on egress from the AS Confederation, its value MUST equal the AS Confederation Identifier. (S5) | MUST | 5 - BGP Only to Customer (OTC) Attribute | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no AS Confederation and no owner decision declines it: no confederation-identifier or member-AS leaf exists in the BGP YANG, and the egress OTC stamp (internal/component/bgp/plugins/role/otc.go) uses only the destination's local AS, so no confederation egress stamps the Confederation Identifier |
| `RFC9234-5-8` | Also, on egress from the AS Confederation, an UPDATE MUST NOT contain an OTC Attribute with a value corresponding to any Member-AS Number other than the AS Confederation Identifier. (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** no positive test. **negative:** no negative test. **{gap}:** ze implements no AS Confederation and no owner decision declines it: with no member-AS or confederation-identifier configuration, internal/component/bgp/plugins/role/otc.go has no confederation egress boundary at which to strip a Member-AS OTC value |
| `RFC9234-5-9` | On egress from the Internet- facing AS, the OTC Attribute MUST NOT contain a value other than the Internet-facing ASN. (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressStampLocalASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1307). **negative:** no negative test |
| `RFC9234-5-10` | The described ingress and egress procedures are applicable only for the address families AFI 1 (IPv4) and AFI 2 (IPv6) with SAFI 1 (unicast) in both cases and MUST NOT be applied to other address families by default. (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestIsPayloadUnicastMPUnreachFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1996). **positive:** `unit/verify` [`TestOTCEgressNonUnicastWithdrawalNotProcessed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L2039). **positive:** `unit/verify` [`TestOTCEgressUnicastOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1341). **positive:** `unit/verify` [`TestOTCNonUnicastWithdrawalSkipsOTCProcedures`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L2079). **negative:** `unit/verify` [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L931) |
| `RFC9234-5-11` | The operator MUST NOT have the ability to modify the procedures defined in this section. (S5) | MUST NOT | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1702). **positive:** `unit/verify` [`TestRFC9234OTCProceduresSurviveOperatorSettings`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L92). **negative:** no negative test. **{single-polarity}:** checkOTCIngress (internal/component/bgp/plugins/role/otc.go:164) and OTCEgressFilter (otc.go:384) take no operator override, peerRoleConfig (config.go:17) exposes no disable flag, and the wire-bytes OTC suppression at otc.go:384 runs before and independent of the export policy, so the procedures cannot be switched off by configuration and a modifiable negative is not constructible |
| `RFC9234-6-1` | Roles MUST NOT be configured on an eBGP session with a Complex peering relationship. (S6) | MUST NOT | 6 - Additional Considerations | **positive:** no positive test. **negative:** no negative test. **{gap}:** this MUST binds the operator's configuration, and ze cannot know whether a peering relationship is Complex (internal/component/bgp/plugins/role/config.go::parseRoleContainer reads one role per peer). What is owed is operator documentation stating that a role must not be configured on a Complex eBGP session, and no test asserts that the documentation carries it |
| `RFC9234-3.1-1` | Customer: MAY propagate any route learned from a Customer, or that is locally originated, to a Provider. All other routes MUST NOT be propagated. Route Server (RS): MAY propagate any available route to a Route Server Client (RS-Client). Route Server Client (RS-Client): MAY propagate any route learned from a Customer, or that is locally originated, to an RS. All other routes MUST NOT be propagated. Peer: MAY propagate any route learned from a Customer, or that is locally originated, to a Peer. All other routes MUST NOT be propagated. (§3.1) | MUST NOT | 3.1 - Peering Relationships | **positive:** `unit/verify` [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L578). **positive:** `unit/verify` [`TestOTCEgressSuppressProviderLearnedWithoutMeta`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1041). **negative:** `unit/verify` [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L588) |
| `RFC9234-5-12` | The OTC Attribute is considered malformed if the length value is not 4. An UPDATE message with a malformed OTC Attribute SHALL be handled using the approach of "treat-as-withdraw" [RFC7606]. (§5) | SHALL | 5 - BGP Only to Customer (OTC) Attribute | **positive:** `unit/verify` [`TestOTCIngressMalformedTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1755). **positive:** `unit/verify` [`TestRFC9234OTCOfAnyLengthButFourIsTreatedAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_malformed_test.go#L36). **negative:** `unit/verify` [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L258). **negative:** `unit/verify` [`TestRFC9234OTCOfLengthFourIsNotTreatedAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_malformed_test.go#L56). **positive:** `functional/verify` [`role-otc-malformed-treat-as-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-malformed-treat-as-withdraw.ci#L7). **negative:** `functional/verify` [`role-otc-malformed-treat-as-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-malformed-treat-as-withdraw.ci#L11) |
| `RFC9234-4-1` | One of the Roles described below SHOULD be configured at the local AS for each eBGP session (see definitions in Section 3) based on the local AS's knowledge of its Role. (§4) | SHOULD | 4 - BGP Role | **positive:** no positive test. **negative:** no negative test |
| `RFC9234-4.2-4` | For backward compatibility, if the BGP Role Capability is sent but one is not received, the BGP Speaker SHOULD ignore the absence of the BGP Role Capability and proceed with session establishment. (§4.2) | SHOULD | 4.2 - Role Correctness | **positive:** no positive test. **negative:** no negative test |
| `RFC9234-6-2` | If multiple eBGP sessions can segregate the Complex peering relationship into eBGP sessions with normal peering relationships, BGP Roles SHOULD be used on each of the resulting eBGP sessions. (§6) | SHOULD | 6 - Additional Considerations | **positive:** no positive test. **negative:** no negative test |
| `RFC9234-4.2-5` | An operator may choose to apply a "strict mode" in which the receipt of a BGP Role Capability from the remote AS is required. When operating in the "strict mode", if the BGP Role Capability is sent but one is not received, the connection is rejected using the Role Mismatch Notification (code 2, subcode 11). (§4.2) | MAY | 4.2 - Role Correctness | **positive:** `unit/verify` [`TestValidateOpenStrictModeRefusesADynamicGroupMemberWithNoRole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L176). **negative:** `unit/verify` [`TestValidateOpenRolePair_NoPeerRole_NoStrict`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L112). **positive:** `functional/verify` [`role-strict-enforcement.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-strict-enforcement.ci#L4) |
| `RFC9234-5-13` | The BGP Role negotiation and OTC-Attribute-based procedures specified in this document are NOT RECOMMENDED to be used between autonomous systems in an AS Confederation [RFC5065]. (§5) | NOT RECOMMENDED | 5 - BGP Only to Customer (OTC) Attribute | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC9234-5-7`](#rfc9234-5-7) If an OTC Attribute is added on egress from the AS Confederation, its value MUST equal the AS Confederation Identifier. (S5) | {gap}, no test | ze implements no AS Confederation and no owner decision declines it: no confederation-identifier or member-AS leaf exists in the BGP YANG, and the egress OTC stamp (internal/component/bgp/plugins/role/otc.go) uses only the destination's local AS, so no confederation egress stamps the Confederation Identifier |
| [`RFC9234-5-8`](#rfc9234-5-8) Also, on egress from the AS Confederation, an UPDATE MUST NOT contain an OTC Attribute with a value corresponding to any Member-AS Number other than the AS Confederation Identifier. (S5) | {gap}, no test | ze implements no AS Confederation and no owner decision declines it: with no member-AS or confederation-identifier configuration, internal/component/bgp/plugins/role/otc.go has no confederation egress boundary at which to strip a Member-AS OTC value |
| [`RFC9234-6-1`](#rfc9234-6-1) Roles MUST NOT be configured on an eBGP session with a Complex peering relationship. (S6) | {gap}, no test | this MUST binds the operator's configuration, and ze cannot know whether a peering relationship is Complex (internal/component/bgp/plugins/role/config.go::parseRoleContainer reads one role per peer). What is owed is operator documentation stating that a role must not be configured on a Complex eBGP session, and no test asserts that the documentation carries it |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC9234-4.1-1`](#rfc9234-4.1-1)

If the BGP Role is locally configured, the eBGP speaker MUST advertise the BGP Role Capability in the BGP OPEN message. (S4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-02 by an independent auditor who authored none of the bgp-role changes. RFC 9234 Section 4.1: "If the BGP Role is locally configured, the eBGP speaker MUST advertise the BGP Role Capability in the BGP OPEN message." The only change to the file-scoped .ci units it judged (655d07b59b, 3459b6e390) deletes the `policy { reject-asn NO-TRANSIT { indirect [ 3356 ] } }` block, its four comment lines, and `filters [ inactive:NO-TRANSIT ];` from each bound import and export chain, leaving `final accept;`. That filter was deactivated and never ran; it existed only because the config then required a role's bound chain to name a transit-refusing filter, and 655d07b59b made that check a filter the bgp-role plugin inserts itself. Every OPEN capability option, every action=send and every expect/reject line (the exact NOTIFICATION or UPDATE hex) is byte-identical, so each case still isolates the same condition and asserts the same exact outcome. Verdict unchanged: enforced. Earlier judgement follows. Rejudged 2026-10-31 by an auditor who authored none of the change. The only change to test/plugin/dynamic-peer-gets-group-role-capability.ci since the last stamp is cabb706c11 moving the policy block from inside bgp {} to the top-level policy root, the same lines re-indented; every action, expect, reject and option line is byte-identical, so the carriers drive the same inputs and assert the same outcomes. Verdict unchanged. RFC 9234 Section 4.1: "If the BGP Role is locally configured, the eBGP speaker MUST advertise the BGP Role Capability in the BGP OPEN message." Producer internal/component/bgp/plugins/role/config.go::extractRoleCapabilities emits code 9 and one-octet role values from extractPeerRoleConfigs; reactor/peer.go::getPluginCapabilities consumes the selector declarations, including dynamic groups. TestExtractRoleCapabilities_ParseBGPConfig checks all five values, exactly one declaration and peer, and no declaration for absent/empty/invalid roles. dynamic-peer-gets-group-role-capability.ci pins the entire 002E OPEN ending 090101, then the EoR and dynamic-session establishment, with local AS 65533 and peer AS 65010. A missing, duplicate or wrong-valued capability fails the wire assertion. Rejudged independently against RFC source and every tagged carrier on 2026-10-22. Functional judgments use the corresponding test/draft/plugin files, not the unpromoted old-syntax live files. Their inactive NO-TRANSIT import/export policies explicitly end in accept; reactor runEgressPolicyChain applies that final only after mandatory in-process OTC, and OPEN validation is independent. This is a source judgment, not a new runtime or discrimination result; parent owns promotion and native freshness before stamping.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L289) | unit/verify | revert, verified |
| positive | [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L288) | unit/verify | revert, verified |
| positive | [`dynamic-peer-gets-group-role-capability.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/dynamic-peer-gets-group-role-capability.ci#L21) | functional/verify | revert, verified |

### [`RFC9234-4.1-2`](#rfc9234-4.1-2)

An eBGP speaker MUST NOT advertise multiple versions of the BGP Role Capability. (S4.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-02 by an independent auditor who authored none of the bgp-role changes. RFC 9234 Section 4.1: "An eBGP speaker MUST NOT advertise multiple versions of the BGP Role Capability." The only change to the file-scoped .ci units it judged (655d07b59b, 3459b6e390) deletes the `policy { reject-asn NO-TRANSIT { indirect [ 3356 ] } }` block, its four comment lines, and `filters [ inactive:NO-TRANSIT ];` from each bound import and export chain, leaving `final accept;`. That filter was deactivated and never ran; it existed only because the config then required a role's bound chain to name a transit-refusing filter, and 655d07b59b made that check a filter the bgp-role plugin inserts itself. Every OPEN capability option, every action=send and every expect/reject line (the exact NOTIFICATION or UPDATE hex) is byte-identical, so each case still isolates the same condition and asserts the same exact outcome. Verdict unchanged: enforced. Earlier judgement follows. Rejudged 2026-10-31 by an auditor who authored none of the change. The only change to test/plugin/dynamic-peer-gets-group-role-capability.ci since the last stamp is cabb706c11 moving the policy block from inside bgp {} to the top-level policy root, the same lines re-indented; every action, expect, reject and option line is byte-identical, so the carriers drive the same inputs and assert the same outcomes. Verdict unchanged. RFC 9234 Section 4.1: "An eBGP speaker MUST NOT advertise multiple versions of the BGP Role Capability." The single-polarity annotation remains justified: config.go::parseRoleContainer reads one scalar import role and extractRoleCapabilities emits one declaration per selected peer; reactor/peer.go::getPluginCapabilities consumes those declarations for its selector set. TestExtractRoleCapabilities_ParseBGPConfig requires exactly one capability and one peer for every configured role. dynamic-peer-gets-group-role-capability.ci asserts the whole OPEN with exactly one 090101 TLV, including all enclosing lengths; duplicate emission cannot hide behind a substring assertion. No fabricated negative configuration is required. Rejudged independently against RFC source and every tagged carrier on 2026-10-22. Functional judgments use the corresponding test/draft/plugin files, not the unpromoted old-syntax live files. Their inactive NO-TRANSIT import/export policies explicitly end in accept; reactor runEgressPolicyChain applies that final only after mandatory in-process OTC, and OPEN validation is independent. This is a source judgment, not a new runtime or discrimination result; parent owns promotion and native freshness before stamping.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestExtractRoleCapabilities_ParseBGPConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_config_test.go#L290) | unit/verify | unproven |
| positive | [`dynamic-peer-gets-group-role-capability.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/dynamic-peer-gets-group-role-capability.ci#L24) | functional/verify | revert, verified |

### [`RFC9234-4.2-1`](#rfc9234-4.2-1)

If the BGP Role Capability is advertised, and one is also received from the peer, the Roles MUST correspond to the relationships in Table 2. (S4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. non-compliant: accepting a pair outside Table 2. TestValidateOpenRolePair_InvalidPairs assert.False(output.Accept) over customer/customer, provider/provider, rs/rs, rsclient/rsclient, provider/rs, customer/peer, and TestValidateOpenRolePairRunsForADynamicGroupMember asserts RS/RS refused; positive TestValidateOpenRolePair_ValidPairs assert.True over all five Table 2 pairs

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L123) | unit/verify | unproven |
| negative | [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L65) | unit/verify | unproven |
| positive | [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L21) | unit/verify | unproven |

### [`RFC9234-4.2-2`](#rfc9234-4.2-2)

If the Roles do not correspond, the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11). (S4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-02 by an independent auditor who authored none of the bgp-role changes. RFC 9234 Section 4.2: "If the Roles do not correspond, the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11)." The only change to the file-scoped .ci units it judged (655d07b59b, 3459b6e390) deletes the `policy { reject-asn NO-TRANSIT { indirect [ 3356 ] } }` block, its four comment lines, and `filters [ inactive:NO-TRANSIT ];` from each bound import and export chain, leaving `final accept;`. That filter was deactivated and never ran; it existed only because the config then required a role's bound chain to name a transit-refusing filter, and 655d07b59b made that check a filter the bgp-role plugin inserts itself. Every OPEN capability option, every action=send and every expect/reject line (the exact NOTIFICATION or UPDATE hex) is byte-identical, so each case still isolates the same condition and asserts the same exact outcome. The discrimination records on these .ci carriers (validateOpenRolePair) were re-taken over the changed units in this session. Verdict unchanged: enforced. Earlier judgement follows. Rejudged 2026-10-31 by an auditor who authored none of the change. The only change to test/plugin/peer-open-role-complement.ci and role-mismatch-notification.ci since the last stamp is cabb706c11 moving the policy block from inside bgp {} to the top-level policy root, the same lines re-indented; every action, expect, reject and option line is byte-identical, so the carriers drive the same inputs and assert the same outcomes. Verdict unchanged. RFC 9234 Section 4.2: "If the Roles do not correspond, the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11)." Producer role/validate.go::validateOpenRolePair checks Table 2; server/validate.go::askOpenValidators converts a rejected ValidateOpenOutput into OpenValidationError and reactor/session_open_validation.go::runOpenValidator sends its codes. role-mismatch-notification.ci supplies Provider/Provider and requires exact FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001503020B, rejecting EoR; peer-open-role-complement.ci changes the remote role to Customer, requires EoR and rejects 03020B. TestValidateOpenRolePair_ValidPairs and InvalidPairs pin accept/refuse and exact codes; TestValidateOpenRolePairRunsForADynamicGroupMember also checks group lookup, refusal and complementary acceptance. Server TestBroadcastValidateOpenRefusesAnUnansweredPerPeerPolicy, TestAskOpenValidatorsLeavesASilentPluginPending and TestBroadcastValidateOpenAcceptsAPeerWithNoPerPeerPolicy retain the owner-selected fail-closed plumbing: those are supplemental robustness evidence, not a claim that the RFC equates plugin silence with an observed role mismatch. Rejudged independently against RFC source and every tagged carrier on 2026-10-22. Functional judgments use the corresponding test/draft/plugin files, not the unpromoted old-syntax live files. Their inactive NO-TRANSIT import/export policies explicitly end in accept; reactor runEgressPolicyChain applies that final only after mandatory in-process OTC, and OPEN validation is independent. This is a source judgment, not a new runtime or discrimination result; parent owns promotion and native freshness before stamping.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateOpenRolePair_ValidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L22) | unit/verify | unproven |
| negative | [`TestBroadcastValidateOpenAcceptsAPeerWithNoPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L252) | unit/verify | unproven |
| negative | [`peer-open-role-complement.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/peer-open-role-complement.ci#L4) | functional/verify | revert, verified |
| positive | [`TestValidateOpenRolePairRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L124) | unit/verify | unproven |
| positive | [`TestValidateOpenRolePair_InvalidPairs`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L66) | unit/verify | unproven |
| positive | [`TestAskOpenValidatorsLeavesASilentPluginPending`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L269) | unit/verify | unproven |
| positive | [`TestBroadcastValidateOpenRefusesAnUnansweredPerPeerPolicy`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/server/rfc9234_validate_test.go#L225) | unit/verify | unproven |
| positive | [`role-mismatch-notification.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-mismatch-notification.ci#L4) | functional/verify | mutant, verified |

### [`RFC9234-4.2-3`](#rfc9234-4.2-3)

If multiple BGP Role Capabilities are received and not all of them have the same value, then the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11). (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-02 by an independent auditor who authored none of the bgp-role changes. RFC 9234 Section 4.2: "If multiple BGP Role Capabilities are received and not all of them have the same value, then the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11)." The only change to the file-scoped .ci units it judged (655d07b59b, 3459b6e390) deletes the `policy { reject-asn NO-TRANSIT { indirect [ 3356 ] } }` block, its four comment lines, and `filters [ inactive:NO-TRANSIT ];` from each bound import and export chain, leaving `final accept;`. That filter was deactivated and never ran; it existed only because the config then required a role's bound chain to name a transit-refusing filter, and 655d07b59b made that check a filter the bgp-role plugin inserts itself. Every OPEN capability option, every action=send and every expect/reject line (the exact NOTIFICATION or UPDATE hex) is byte-identical, so each case still isolates the same condition and asserts the same exact outcome. The discrimination records on these .ci carriers (validateOpenRolePair) were re-taken over the changed units in this session. Verdict unchanged: enforced. Earlier judgement follows. Rejudged 2026-10-31 by an auditor who authored none of the change. The only change to test/plugin/role-multiple-roles-notification.ci and role-multiple-same-roles.ci since the last stamp is cabb706c11 moving the policy block from inside bgp {} to the top-level policy root, the same lines re-indented; every action, expect, reject and option line is byte-identical, so the carriers drive the same inputs and assert the same outcomes. Verdict unchanged. RFC 9234 Section 4.2: "If multiple BGP Role Capabilities are received and not all of them have the same value, then the BGP speaker MUST reject the connection using the Role Mismatch Notification (code 2, subcode 11)." The complementary sentence is: "If an eBGP speaker receives multiple but identical BGP Role Capabilities with the same value in each, then the speaker considers them to be a single BGP Role Capability and proceeds [RFC5492]." Producer role/validate.go::validateOpenRolePair compares values, not just the count; server askOpenValidators and session runOpenValidator consume the refusal. role-multiple-roles-notification.ci sends Customer then Peer against Provider, requires exact NOTIFICATION 2/11 and no EoR; Customer alone corresponds, isolating the second differing value. role-multiple-same-roles.ci sends Customer twice, requires EoR and rejects 03020B. TestValidateOpenRolePair_MultipleDifferentRoles pins false, 2/11 and the multiple reason; MultipleSameRoles pins true. Rejudged independently against RFC source and every tagged carrier on 2026-10-22. Functional judgments use the corresponding test/draft/plugin files, not the unpromoted old-syntax live files. Their inactive NO-TRANSIT import/export policies explicitly end in accept; reactor runEgressPolicyChain applies that final only after mandatory in-process OTC, and OPEN validation is independent. This is a source judgment, not a new runtime or discrimination result; parent owns promotion and native freshness before stamping.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateOpenRolePair_MultipleSameRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L196) | unit/verify | unproven |
| negative | [`role-multiple-same-roles.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-multiple-same-roles.ci#L4) | functional/verify | revert, verified |
| positive | [`TestValidateOpenRolePair_MultipleDifferentRoles`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L166) | unit/verify | unproven |
| positive | [`role-multiple-roles-notification.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-multiple-roles-notification.ci#L4) | functional/verify | mutant, verified |

### [`RFC9234-5-1`](#rfc9234-5-1)

If a route with the OTC Attribute is received from a Customer or an RS-Client, then it is a route leak and MUST be considered ineligible (see Section 3). (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-02 by an independent auditor who authored none of db53ccc34d. RFC 9234 Section 5 (row text): "If a route with the OTC Attribute is received from a Customer or an RS-Client, then it is a route leak and MUST be considered ineligible (see Section 3)." db53ccc34d deleted OTCIngressFilter, which had no non-test caller since the ingress hook removal, and moved its test call sites onto the live bgp-role:otc-ingress filter answer through evaluateRoleFilter (otcIngressAnswer, answerOTC, answerMeta). Every input payload and source role is unchanged; an old reject maps to the FilterWithdraw verdict, an old accept-unmodified to a non-withdraw verdict with no otc set, and an old stamp to FilterModify with the otc set at the expected ASN. The tests now judge the decision the shipped filter returns rather than a dead wrapper, so for the decision this row binds they are stronger than before. The engine's application of a withdraw or modify verdict is the shared filter-chain mechanism, proven by reactor TestWithdrawVerdictReactorRails and TestBuildWithdrawalPayload_IPv4, which carry no tag for this row. Verdict unchanged: enforced. Earlier judgement follows. non-compliant: OTC route from Customer or RS-Client accepted. TestCheckOTCIngress reject_customer_with_otc and reject_rs_client_with_otc assert otcRejectLeak; TestOTCIngressGateRunsForADynamicGroupMember asserts accept=false for an RS-Client member; negative accept_provider_has_otc/accept_rs_has_otc assert otcAccept

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L257) | unit/verify | unproven |
| positive | [`TestOTCIngressGateRunsForADynamicGroupMember`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L81) | unit/verify | unproven |
| positive | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L247) | unit/verify | unproven |

### [`RFC9234-5-2`](#rfc9234-5-2)

If a route with the OTC Attribute is received from a Peer (i.e., remote AS with a Peer Role) and the Attribute has a value that is not equal to the remote (i.e., Peer's) AS number, then it is a route leak and MUST be considered ineligible. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-02 by an independent auditor who authored none of db53ccc34d. RFC 9234 Section 5 (row text): "If a route with the OTC Attribute is received from a Peer (i.e., remote AS with a Peer Role) and the Attribute has a value that is not equal to the remote (i.e., Peer's) AS number, then it is a route leak and MUST be considered ineligible." db53ccc34d deleted OTCIngressFilter, which had no non-test caller since the ingress hook removal, and moved its test call sites onto the live bgp-role:otc-ingress filter answer through evaluateRoleFilter (otcIngressAnswer, answerOTC, answerMeta). Every input payload and source role is unchanged; an old reject maps to the FilterWithdraw verdict, an old accept-unmodified to a non-withdraw verdict with no otc set, and an old stamp to FilterModify with the otc set at the expected ASN. The tests now judge the decision the shipped filter returns rather than a dead wrapper, so for the decision this row binds they are stronger than before. The engine's application of a withdraw or modify verdict is the shared filter-chain mechanism, proven by reactor TestWithdrawVerdictReactorRails and TestBuildWithdrawalPayload_IPv4, which carry no tag for this row. Verdict unchanged: enforced. Earlier judgement follows. non-compliant: Peer route whose OTC differs from the Peer's AS accepted. TestOTCIngressFilter reject_peer_otc_mismatch asserts accept=false (OTC 65099 vs PeerAS 65003); negative accept_peer_otc_matches asserts accept=true and no rewrite; TestCheckOTCIngress also pins reject_peer_otc_wrong and zero ASN

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L408) | unit/verify | unproven |
| positive | [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L417) | unit/verify | unproven |

### [`RFC9234-5-3`](#rfc9234-5-3)

If a route is received from a Provider, a Peer, or an RS and the OTC Attribute is not present, then it MUST be added with a value equal to the AS number of the remote AS. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-02 by an independent auditor who authored none of db53ccc34d. RFC 9234 Section 5 (row text): "If a route is received from a Provider, a Peer, or an RS and the OTC Attribute is not present, then it MUST be added with a value equal to the AS number of the remote AS." db53ccc34d deleted OTCIngressFilter, which had no non-test caller since the ingress hook removal, and moved its test call sites onto the live bgp-role:otc-ingress filter answer through evaluateRoleFilter (otcIngressAnswer, answerOTC, answerMeta). Every input payload and source role is unchanged; an old reject maps to the FilterWithdraw verdict, an old accept-unmodified to a non-withdraw verdict with no otc set, and an old stamp to FilterModify with the otc set at the expected ASN. The tests now judge the decision the shipped filter returns rather than a dead wrapper, so for the decision this row binds they are stronger than before. The engine's application of a withdraw or modify verdict is the shared filter-chain mechanism, proven by reactor TestWithdrawVerdictReactorRails and TestBuildWithdrawalPayload_IPv4, which carry no tag for this row. Verdict unchanged: enforced. Earlier judgement follows. non-compliant: a route without OTC from Provider, Peer or RS left unstamped or stamped with another ASN. TestOTCIngressFilter stamp_from_provider finds OTC == remote AS 65002 in the rewritten payload; TestCheckOTCIngress stamp_from_provider/peer/rs assert stampASN == remote ASN for each of the three roles. Negatives: Customer/RS-Client not stamped, pure withdrawal and MP_UNREACH-only not stamped

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L242) | unit/verify | unproven |
| negative | [`TestOTCIngressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1964) | unit/verify | unproven |
| negative | [`TestOTCIngressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1935) | unit/verify | unproven |
| positive | [`TestOTCIngressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L395) | unit/verify | unproven |

### [`RFC9234-5-4`](#rfc9234-5-4)

If a route is to be advertised to a Customer, a Peer, or an RS- Client (when the sender is an RS), and the OTC Attribute is not present, then when advertising the route, an OTC Attribute MUST be added with a value equal to the AS number of the local AS. (§5)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/le/interoplab/bgp/check_rfc.go::checkOTCWithdrawal, internal/le/interoplab/bgp/check_rfc.go::checkOTCWithdrawal#2 moved. Independent source rejudgment against the full Section 5 egress sentence and Section 4.2 local-role prerequisite. TestRFC9234OTCStampedToEveryDownstreamDestination checks Customer, Peer and RS-Client with reachable NLRI, no OTC, a nonzero destination local AS, and exact code-35/set/four-byte/local-AS operations. TestRFC9234OTCNotStampedOutsideTheDownstreamScope checks Provider/RS and existing-OTC controls. TestOTCEgressStampMod pins the same operation; TestOTCEgressNoStampProvider and the no-source-role-config case isolate destination scope. Pure withdrawal, MP_UNREACH-only and mixed withdraw/advertise units distinguish the route-advertisement condition. All tagged native fixtures were read: receiver exact wire expectations include the OTC advertisement and attribute-free withdrawals/EOR; their observer barriers require the corresponding destination updates before shutdown. Both RS-Client fixtures configure the sender as RS. checkOTCWithdrawal requires an established FRR session, an observed advertised route with exact OTC 65001, then observed withdrawal, no receiver attribute error and a surviving session; injector barriers prevent disappearance-by-disconnect from satisfying the negative. OTCEgressFilter, evaluateOTCEgress and PayloadAdvertisesNLRI implement those decisions. Added complementary destination configurations repair RFC applicability while retaining assertions and payloads. Three existing direct producer-break records require renewal; the unchanged interop tuple also needs runtime reproof through the changed OTCEgressFilter dependency. No current runtime or discrimination success is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9234OTCNotStampedOutsideTheDownstreamScope`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L72) | unit/verify | mutant, verified |
| negative | [`TestOTCEgressNoStampOnMPUnreachOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1861) | unit/verify | mutant, verified |
| negative | [`TestOTCEgressNoStampOnPureWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1840) | unit/verify | mutant, verified |
| negative | [`TestOTCEgressNoStampProvider`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L970) | unit/verify | mutant, verified |
| negative | [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L894) | interop/nightly | unproven |
| negative | [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L11) | functional/verify | unproven |
| negative | [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L17) | functional/verify | unproven |
| positive | [`TestRFC9234OTCStampedToEveryDownstreamDestination`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L53) | unit/verify | mutant, verified |
| positive | [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L930) | unit/verify | mutant, verified |
| positive | [`TestOTCEgressStampsMixedWithdrawAndAnnounce`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1902) | unit/verify | mutant, verified |
| positive | [`TestOTCEgressStampsToCustomerWhenSourceHasNoRoleConfig`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1096) | unit/verify | mutant, verified |
| positive | [`checkOTCWithdrawal`](https://github.com/ze-software/ze/blob/main/internal/le/interoplab/bgp/check_rfc.go#L893) | interop/nightly | mutant, verified |
| positive | [`role-otc-fwd-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-fwd-withdraw.ci#L8) | functional/verify | unproven |
| positive | [`role-otc-rs-client-dest-stamp-forked.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-client-dest-stamp-forked.ci#L22) | functional/verify | unproven |
| positive | [`role-otc-rs-client-dest-stamp.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-client-dest-stamp.ci#L12) | functional/verify | unproven |
| positive | [`role-otc-rs-withdraw-eor.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-rs-withdraw-eor.ci#L14) | functional/verify | unproven |

### [`RFC9234-5-5`](#rfc9234-5-5)

If a route already contains the OTC Attribute, it MUST NOT be propagated to Providers, Peers, or RSes. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment against full Section 5 and the Section 4.2 local-role prerequisite. TestOTCEgressWireBytesCheck supplies a real OTC attribute, a source with no role configuration and no role metadata, and complementary locally configured Provider/Peer/RS/Customer destination relationships. Exact false outcomes for remote Provider, Peer and RS therefore isolate the existing-wire-OTC rule from source export-set and metadata suppression. True Customer and OTC-absent Provider controls reject blanket dropping. OTCEgressFilter calls checkOTCEgress before operator export-set evaluation. The added local destination configurations establish applicability without changing wire input or outcome assertions. There is no existing discrimination record for this requirement to count as current proof; corrected runtime execution remains owed to the parent.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1701) | unit/verify | mutant, verified |
| positive | [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1700) | unit/verify | mutant, verified |

### [`RFC9234-5-6`](#rfc9234-5-6)

Once the OTC Attribute has been set, it MUST be preserved unchanged (this also applies to an AS Confederation). (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. non-compliant: an existing OTC value rewritten. TestOTCAttrModHandlerExistingPreserved plans OTC 65001 from source against a set op of 65000 and asserts the emitted value is 65001. Negative TestOTCAttrModHandlerNewAttr: no prior OTC, so the op's value is emitted. The confederation parenthetical has no ze code path (RFC9234-5-7/5-8 are not-applicable)

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCAttrModHandlerNewAttr`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1463) | unit/verify | unproven |
| positive | [`TestOTCAttrModHandlerExistingPreserved`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1484) | unit/verify | unproven |

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

Audit verdict: weak (the tests pass over code that does not enforce the requirement), fresh. Independent rejudgment against the complete private-AS paragraph of RFC 9234 Section 5: 'On egress from the Internet-facing AS, the OTC Attribute MUST NOT contain a value other than the Internet-facing ASN.' The requirement row is now unannotated: it retains the exact obligation, permanent id, level and section without the unsupported single-polarity exemption or a gap annotation incompatible with its existing positive tag. The sole tagged unit, TestOTCEgressStampLocalASN in internal/component/bgp/plugins/role/rfc9234_otc_test.go, supplies reachable unicast NLRI with no OTC and a locally configured downstream destination. It asserts exactly one operation whose decoded value is dest.LocalAS 64999, distinguished from source ASN 65001 and destination ASN 65002. This remains useful isolated proof of absent-OTC synthesis. OTCEgressFilter in internal/component/bgp/plugins/role/otc.go creates OTC only when it is absent and otherwise preserves the existing value. The tagged unit does not supply an already-present OTC carrying a wrong private or internal ASN at the Internet-facing boundary, so it cannot detect noncompliance in that part of the requirement. No negative polarity currently covers the requirement. The existing positive test and claim remain unchanged; removing the unsupported exemption exposes the genuine missing-negative finding rather than inventing coverage or retagging the test. This is a testing limitation and not an observed wire violation, a justification to rewrite arbitrary received OTC, or a whole-requirement conformance claim. No runtime or producer-break replay was run for this source rejudgment.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestOTCEgressStampLocalASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1307) | unit/verify | mutant, verified |

### [`RFC9234-5-10`](#rfc9234-5-10)

The described ingress and egress procedures are applicable only for the address families AFI 1 (IPv4) and AFI 2 (IPv6) with SAFI 1 (unicast) in both cases and MUST NOT be applied to other address families by default. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment against the complete Section 5 address-family restriction. TestIsPayloadUnicastMPUnreachFamily asserts both allowed AFI 1/2 SAFI 1 cases, multicast/VPN/flowspec/EVPN exclusions and MP_REACH precedence in both attribute orders. TestOTCEgressStampMod supplies an actual reachable unicast advertisement and pins its OTC operation. TestOTCNonUnicastWithdrawalSkipsOTCProcedures supplies an existing OTC toward a locally configured Provider destination with a VPNv4 withdrawal, and an EVPN ingress from a Customer carrying OTC: exact acceptance/no-withdrawal/no-OTC-set assertions distinguish family bypass from the wire-OTC egress suppression and Customer-leak ingress rejection that would otherwise fire. TestOTCEgressUnicastOnly and TestOTCEgressNonUnicastWithdrawalNotProcessed alone are not discriminating stamping negatives because they advertise no reachable NLRI; they are not the basis for whole-requirement credit. The aggregate predicate, positive stamp, and existing-OTC ingress/egress negatives isolate the rule. isPayloadUnicast gates OTCEgressFilter and decideOTCIngress before those procedures. Complementary destination config additions preserve outcomes while activating the intended RFC gate. No corrected runtime run or new discrimination record is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCEgressStampMod`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L931) | unit/verify | mutant, verified |
| positive | [`TestIsPayloadUnicastMPUnreachFamily`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1996) | unit/verify | unproven |
| positive | [`TestOTCEgressNonUnicastWithdrawalNotProcessed`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L2039) | unit/verify | mutant, verified |
| positive | [`TestOTCEgressUnicastOnly`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1341) | unit/verify | mutant, verified |
| positive | [`TestOTCNonUnicastWithdrawalSkipsOTCProcedures`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L2079) | unit/verify | mutant, verified |

### [`RFC9234-5-11`](#rfc9234-5-11)

The operator MUST NOT have the ability to modify the procedures defined in this section. (S5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment against full Section 5, including Section 4.2's locally configured role condition rather than treating a roleless session as an operator bypass of active RFC procedures. TestRFC9234OTCProceduresSurviveOperatorSettings uses strict role configuration and permissive all-role export sets, yet requires exact ingress Provider OTC stamping, Customer OTC withdrawal, upstream egress OTC suppression and downstream local-AS stamping. TestOTCEgressWireBytesCheck independently isolates all three wire-OTC prohibited destinations with no source role configuration or metadata and includes acceptance controls. peerRoleConfig exposes role/strict/export but no procedure override; checkOTCIngress has no operator-policy argument; OTCEgressFilter applies wire-OTC and source-role gates before export policy. otcInsertions marks the active ingress and any-peer egress members Required, and insertForPeer rejects explicit duplicates instead of allowing an operator-positioned member to skip the prepended mandatory filter. Thus the single-polarity rationale is justified for the configured-role procedures; omitting the role is the RFC applicability boundary, not a procedure-modification control. The added destination local role fixtures retain the same assertions. The existing checkOTCIngress producer-break tuple needs renewal because its test unit changed. No corrected runtime success is claimed.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC9234OTCProceduresSurviveOperatorSettings`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_procedures_test.go#L92) | unit/verify | mutant, verified |
| positive | [`TestOTCEgressWireBytesCheck`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1702) | unit/verify | mutant, verified |

### [`RFC9234-6-1`](#rfc9234-6-1)

Roles MUST NOT be configured on an eBGP session with a Complex peering relationship. (S6)

Audit verdict: not audited: no reader has judged these tests

No test carries RFC9234-6-1, so no unit is bound to it.

### [`RFC9234-3.1-1`](#rfc9234-3.1-1)

Customer: MAY propagate any route learned from a Customer, or that is locally originated, to a Provider. All other routes MUST NOT be propagated. Route Server (RS): MAY propagate any available route to a Route Server Client (RS-Client). Route Server Client (RS-Client): MAY propagate any route learned from a Customer, or that is locally originated, to an RS. All other routes MUST NOT be propagated. Peer: MAY propagate any route learned from a Customer, or that is locally originated, to a Peer. All other routes MUST NOT be propagated. (§3.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Independent source rejudgment against full RFC 9234 Sections 3.1, 4.2 and 5. TestOTCEgressFilter exercises exact suppression of non-Customer-learned routes toward Provider, Peer and RS and exact acceptance controls toward Customer/RS-Client and for Customer-learned transit. Its no-OTC inputs and config-less source cases isolate source-role suppression from the wire OTC gate and source export-set gate; TestOTCEgressSuppressProviderLearnedWithoutMeta additionally proves the configured source-role fallback without metadata. The destination local role additions make each RFC destination gate applicable under Section 4.2, complementary to the previously supplied remote role, without changing payloads or assertions. The Customer/local-origin and RS forwarding sentences are MAY permissions, not unconditional forwarding requirements that override operator policy. OTCEgressFilter and resolveSrcRole implement the asserted scope. Existing positive/negative TestOTCEgressFilter discrimination records require renewal for the changed unit/producer. This is source judgment, not a claim that corrected tests or producer-break proofs have run.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L588) | unit/verify | mutant, verified |
| positive | [`TestOTCEgressFilter`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L578) | unit/verify | mutant, verified |
| positive | [`TestOTCEgressSuppressProviderLearnedWithoutMeta`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1041) | unit/verify | mutant, verified |

### [`RFC9234-5-12`](#rfc9234-5-12)

The OTC Attribute is considered malformed if the length value is not 4. An UPDATE message with a malformed OTC Attribute SHALL be handled using the approach of "treat-as-withdraw" [RFC7606]. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-02 by an independent auditor who authored none of the changes named. RFC 9234 Section 5: "The OTC Attribute is considered malformed if the length value is not 4. An UPDATE message with a malformed OTC Attribute SHALL be handled using the approach of "treat-as-withdraw" [RFC7606]." The earlier weak finding (no tagged unit proved the handling, only the withdraw verdict) is answered by the new tag on test/plugin/role-otc-malformed-treat-as-withdraw.ci (b5990018f7): ze with bgp-role, a Provider source and a Customer dest. Negative: the source's 10.0.0.0/24 with OTC C0 23 04 0000FDE9 must reach the dest as an announcement still carrying that OTC (dest seq 1). Positive: the same prefix re-sent with OTC length 3 must reach the dest as exactly the withdrawal FF..001B020004180A00000000 (dest seq 2) with no attributes, so the earlier announcement is removed on the wire, the RFC 7606 treat-as-withdraw outcome; a reject would leave seq 2 absent, an accept would send an announcement. Isolation: the source is a Provider, so no leak rule rejects either UPDATE, and a length-3 OTC read as absent would be stamped and announced, so only the malformed rule yields the withdrawal; a barrier route makes the dest an established target first. Discrimination: otc.go checkOTCIngress isMalformed disabled reddens the positive, findOTC length check forced true reddens the negative (judged genuine by ze-test-judge this session). Lengths 0, 5 and 8 are proven at the decision level by the byte-identical Go units. Whole requirement now covered: decision and handling. Verdict: enforced (was weak).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC9234OTCOfLengthFourIsNotTreatedAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_malformed_test.go#L56) | unit/verify | revert, verified |
| negative | [`TestCheckOTCIngress`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L258) | unit/verify | unproven |
| negative | [`role-otc-malformed-treat-as-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-malformed-treat-as-withdraw.ci#L11) | functional/verify | mutant, verified |
| positive | [`TestRFC9234OTCOfAnyLengthButFourIsTreatedAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_malformed_test.go#L36) | unit/verify | mutant, verified |
| positive | [`TestOTCIngressMalformedTreatAsWithdraw`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_otc_test.go#L1755) | unit/verify | mutant, verified |
| positive | [`role-otc-malformed-treat-as-withdraw.ci`](https://github.com/ze-software/ze/blob/main/test/plugin/role-otc-malformed-treat-as-withdraw.ci#L7) | functional/verify | mutant, verified |

### [`RFC9234-4.2-5`](#rfc9234-4.2-5)

An operator may choose to apply a "strict mode" in which the receipt of a BGP Role Capability from the remote AS is required. When operating in the "strict mode", if the BGP Role Capability is sent but one is not received, the connection is rejected using the Role Mismatch Notification (code 2, subcode 11). (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-11-02 by an independent auditor who authored none of the bgp-role changes. RFC 9234 Section 4.2: "When operating in the "strict mode", if the BGP Role Capability is sent but one is not received, the connection is rejected using the Role Mismatch Notification (code 2, subcode 11)." The only change to the file-scoped .ci units it judged (655d07b59b, 3459b6e390) deletes the `policy { reject-asn NO-TRANSIT { indirect [ 3356 ] } }` block, its four comment lines, and `filters [ inactive:NO-TRANSIT ];` from each bound import and export chain, leaving `final accept;`. That filter was deactivated and never ran; it existed only because the config then required a role's bound chain to name a transit-refusing filter, and 655d07b59b made that check a filter the bgp-role plugin inserts itself. Every OPEN capability option, every action=send and every expect/reject line (the exact NOTIFICATION or UPDATE hex) is byte-identical, so each case still isolates the same condition and asserts the same exact outcome. The discrimination records on these .ci carriers (validateOpenRolePair) were re-taken over the changed units in this session. Verdict unchanged: enforced. Earlier judgement follows. Rejudged 2026-10-31 by an auditor who authored none of the change. The only change to test/plugin/role-strict-enforcement.ci since the last stamp is cabb706c11 moving the policy block from inside bgp {} to the top-level policy root, the same lines re-indented; every action, expect, reject and option line is byte-identical, so the carriers drive the same inputs and assert the same outcomes. Verdict unchanged. RFC 9234 Section 4.2: "An operator may choose to apply a "strict mode" in which the receipt of a BGP Role Capability from the remote AS is required." "When operating in the "strict mode", if the BGP Role Capability is sent but one is not received, the connection is rejected using the Role Mismatch Notification (code 2, subcode 11)." role/validate.go::validateOpenRolePair branches on cfg.strict when peerRoles is empty. role-strict-enforcement.ci configures customer and strict true, drops capability 9 from the remote OPEN and requires exact NOTIFICATION 2/11. TestValidateOpenStrictModeRefusesADynamicGroupMemberWithNoRole checks strict refusal with 2/11 and acceptance of the corresponding group member; TestValidateOpenRolePair_NoPeerRole_NoStrict accepts a missing remote role with strict false. The adjacent backward-compatibility sentence says SHOULD ignore absence, not MUST; the negative pins the chosen non-strict behavior without upgrading that SHOULD. The functional fixture remains ASN 1/1 as before; it is supplemented by the role/group unit controls, not described as an eBGP wire fixture. Rejudged independently against RFC source and every tagged carrier on 2026-10-22. Functional judgments use the corresponding test/draft/plugin files, not the unpromoted old-syntax live files. Their inactive NO-TRANSIT import/export policies explicitly end in accept; reactor runEgressPolicyChain applies that final only after mandatory in-process OTC, and OPEN validation is independent. This is a source judgment, not a new runtime or discrimination result; parent owns promotion and native freshness before stamping.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestValidateOpenRolePair_NoPeerRole_NoStrict`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_validate_test.go#L112) | unit/verify | revert, verified |
| positive | [`TestValidateOpenStrictModeRefusesADynamicGroupMemberWithNoRole`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/plugins/role/rfc9234_dynamic_group_test.go#L176) | unit/verify | unproven |
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
