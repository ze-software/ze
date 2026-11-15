# RFC 3623 - Graceful OSPF Restart

OSPFv2 Graceful Restart. Every requirement listed for RFC 3623, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 4 | of 13 mandatory requirements | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 5 | of 13 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0 | of 13 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial proof; remaining gap | 0 | of 13 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Tests shown to detect a fault | 28 | 28 of 30 tests linked to requirements | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests counts as proof only after this check has demonstrated that they can detect a fault |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Declared gap or no test | 4 | of 13 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Out of scope | 0 | of 13 mandatory requirements | an obligation that does not bind Ze, counted once its evidence passes. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. An annotation whose evidence fails is not counted here: the figures count it as exclusion unchecked. This classification records applicability and forms part of the total accounted for by the cards on this page |
| Not applicable | 0 | of 13 mandatory requirements | a {not-applicable} annotation says the obligation does not bind Ze. No test is owed only once its evidence passes; one whose evidence fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Met below Ze | 0 | of 13 mandatory requirements | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and stays in the MUST bar, outside Ze proven: it is OS verified once a tagged test checks that layer's behavior on the state Ze installs, with a recorded break, and an OS testing gap until then |
| Optional feature declined | 0 | of 13 mandatory requirements | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed once that quote passes its evidence check; one whose quote fails stays owed, and the figures count it as exclusion unchecked. It remains in the total that these cards add up to |
| Linux cannot do this | 0 | of 13 mandatory requirements | Linux cannot provide the primitive this requirement needs. The obligation still binds Ze and stays unmet, but as a limit of the kernel, not of Ze's code: once its limit entry is verified, the row is not owed (OS blocked) and is left out of the MUST bar; one whose entry fails stays owed, and the figures count it as not built. It remains in the total that these cards add up to and earns no proof credit |

### Totals

what the cards above are counted against

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Mandatory requirements (MUST level) | 13 | of 26 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 13 mandatory requirements: they add up to 13. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | ok | green at zero, RED above it: a tested clause cannot prove the whole requirement |
| Declared gap or no test | bad | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green counts above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Linux cannot do this | neutral | neutral at every value: an evidenced platform limit is neither a proof nor a scope exemption |
| Tests shown to detect a fault | ok | green at every value: these tests have been shown to detect a deliberate fault. This card counts individual tests separately, so its total cannot be added to the requirement counts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a mandatory requirement is unreviewed, green when every one is judged sound and current |
| Mandatory requirements (MUST level) | outlined | outlined, with no colored edge: it is the total the other cards are counted against, not a measure, and a larger one is neither good news nor bad |

## At a glance

| Field | Value |
|---|---|
| Public row | OSPFv2 Graceful Restart |
| Automatic checks | Included |
| Requirements | 26 |
| Mandatory requirements (MUST level) | 13 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 4 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 30 |
| Tests linked to requirements | 30 |
| Recorded audit verdicts | 9 |
| Fault test records | 28 |
| Summary | `rfc/short/rfc3623.md` |
| Requirement table | `rfc/requirements/rfc3623.md` |
| RFC text | `rfc/full/rfc3623.txt` |

## Automatic checks

Included in automatic checks: Graceful OSPF Restart (RFC 3623): restarter + helper roles; 4 MET (Grace-LSA mandatory TLVs, shared-media type-3, unplanned toggle) + 5 single-polarity positive + 4 gap (virtual-link V-bit, changed-retx-list refusal, the Appendix A process rule on additional TLVs, the aggregated-adjacency exit ze meets with no test)

## Published support status

**Area:** OSPFv2 Graceful Restart

**Proof:** 8 of 13 MUST proven

**Evidence:** unit 8

**Unmet:** RFC3623-3-1, RFC3623-3.1-1, RFC3623-3.2-1, RFC3623-A-1


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 4 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 9 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **13** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (4):** [`RFC3623-A-2`](#rfc3623-a-2), [`RFC3623-A-3`](#rfc3623-a-3), [`RFC3623-A-4`](#rfc3623-a-4), [`RFC3623-5-1`](#rfc3623-5-1)

**Annotated (including scoped evidence) (9):** [`RFC3623-A-1`](#rfc3623-a-1), [`RFC3623-A-5`](#rfc3623-a-5), [`RFC3623-2.1-1`](#rfc3623-2.1-1), [`RFC3623-5-2`](#rfc3623-5-2), [`RFC3623-5-3`](#rfc3623-5-3), [`RFC3623-5-4`](#rfc3623-5-4), [`RFC3623-3-1`](#rfc3623-3-1), [`RFC3623-3.1-1`](#rfc3623-3.1-1), [`RFC3623-3.2-1`](#rfc3623-3.2-1)

## Figures

- **MUST bar**: Ze proven 8, Proof pending 1, Not built 4
- **Overview bar**: Ze MUST proven 8, Proof pending 1, Not assessed 13, Not built 4

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | not complete |
| MUST rows | 13 |
| MUST rows kept (the MUST bar) | 13 |
| Ze proven | 8 |
| OS verified | 0 |
| Proof pending | 1 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 4 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 13 |
| SHOULD proven | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD proof pending | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 13 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze proven means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Proof pending counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc3623.json](../../../data/rfc/rfc3623.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC3623-A-1` | Additional Grace-LSA TLVs must be described in an Internet Draft and will be subject to the expert review of the OSPF Working Group. (§A) | MUST | A | **positive:** no positive test. **negative:** no negative test. **{gap}:** an IETF process rule on additional Grace-LSA TLVs with no implementer role the RFC names; ze originates only TLV types 1, 2 and 3 and its decoder skips unknown types (internal/plugins/ospf/packet/grace_lsa.go), and no test asserts that ze emits no other TLV |
| `RFC3623-A-2` | Grace Period (Type=1, length=4). The number of seconds that the router's neighbors should continue to advertise the router as fully adjacent, regardless of the state of database synchronization between the router and its neighbors. Since this time period began when grace-LSA's LS age was equal to 0, the grace period terminates when either: a) the LS age of the grace-LSA exceeds the value of a Grace Period or b) the grace-LSA is flushed. See Section 3.2 for other conditions that terminate graceful restart. This TLV must always appear in a grace-LSA. (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L13). **positive:** `unit/verify` [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L41). **negative:** `unit/verify` [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L62). **negative:** `unit/verify` [`TestRFC3623HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L82) |
| `RFC3623-A-3` | Graceful restart reason (Type=2, length=1). Encodes the reason for the router restart as one of the following: 0 (unknown), 1 (software restart), 2 (software reload/upgrade) or 3 (switch to redundant control processor). This TLV must always appear in a grace-LSA. (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L17). **positive:** `unit/verify` [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L44). **negative:** `unit/verify` [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L59). **negative:** `unit/verify` [`TestRFC3623HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L87) |
| `RFC3623-A-4` | IP interface address (Type=3, length=4). The router's IP interface address on the subnet associated with the grace-LSA. Required on broadcast, NBMA and Point-to-MultiPoint segments, where the helper uses the IP interface address to identify the restarting router (see Section 3.1). (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_lsa_test.go#L19). **positive:** `unit/verify` [`TestRFC3623HelperFlushExitsNamedRestarter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_helper_identity_test.go#L201). **positive:** `unit/verify` [`TestRFC3623HelperIdentifiesRestarterByInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_helper_identity_test.go#L131). **positive:** `unit/verify` [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L48). **positive:** `unit/verify` [`TestRFC3623SharedMediaGraceLSACarriesInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L58). **negative:** `unit/verify` [`TestRFC3623GraceLSANeverOmitsAddressOnSharedMedia`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_lsa_test.go#L60). **negative:** `unit/verify` [`TestRFC3623HelperRefusesUnmatchedInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_helper_identity_test.go#L169) |
| `RFC3623-A-5` | DoNotAge is never set in a grace-LSA, even if the grace-LSA is flooded over a demand circuit [7]. (§A) | MUST | A | **positive:** `unit/verify` [`TestGraceLSANeverSetsDoNotAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L62). **negative:** no negative test. **{single-polarity}:** Grace-LSAs originate through OriginateOpaque, whose input struct has no DoNotAge field and starts LS age at 0 with normal aging, so the bit is never set and no negative behavior exists to test (internal/plugins/ospf/lsdb/opaque_as.go:73, gr_restarter.go:314, lsdb/entry.go:87) |
| `RFC3623-2.1-1` | In preparation for the graceful restart, Router X must perform the following actions before its software is restarted/reloaded: (Note that common OSPF shutdown procedures are *not* performed, since we want the other OSPF routers to act as if Router X remains in continuous service. For example, Router X does not flush its locally originated LSAs, since we want them to remain in other routers' link-state databases throughout the restart period.) 1) Router X must ensure that its forwarding table(s) is/are up- to-date and will remain in place across the restart. (§2.1) | MUST | 2.1 | **positive:** `unit/verify` [`TestPrepareRestartRetainsFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L91). **positive:** `unit/verify` [`TestRFC3623PrepareInstallsPendingSPFAndKeepsIt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_prepare_fib_test.go#L106). **positive:** `unit/verify` [`TestRFC3623PreparedRestartKeepsRoutesSPFNoLongerComputes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_install_wiring_test.go#L85). **negative:** no negative test. **{single-polarity}:** prepareRestart raises gracefulStop so suppressInstall makes the ensuing engine stop skip RemoveAll and retain the pre-restart FIB; the meaningful assertion is retention, with no negative behavior the requirement forbids (internal/plugins/ospf/gr_restarter.go:49, gr.go:235) |
| `RFC3623-5-1` | In any event, implementors providing the option to recover gracefully from unplanned outages must allow a network operator to turn the option off. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3623OperatorTurnsUnplannedRecoveryOff`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L119). **positive:** `unit/verify` [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L40). **negative:** `unit/verify` [`TestRFC3623OperatorTurnsUnplannedRecoveryOff`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L122). **negative:** `unit/verify` [`TestUnplannedDisabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L16) |
| `RFC3623-5-2` | The grace-LSAs must be originated and be sent *before* the restarted router sends any OSPF Hello Packets. On broadcast networks, this LSA must be flooded to the AllSPFRouters multicast address (224.0.0.5) since the restarting router is not aware of its previous DR state. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L45). **positive:** `unit/verify` [`TestRFC3623UnplannedColdStartOriginatesGraceLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_unplanned_cold_start_test.go#L131). **positive:** `unit/verify` [`TestUnplannedGraceLSAFloodsToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L154). **negative:** no negative test. **{single-polarity}:** maybeUnplannedRestart enters in-restart then originates one Grace-LSA per interface before interface Hellos begin, and link-local LSAs flood to AllSPFRouters via the standard link-scope path (internal/plugins/ospf/gr_restarter.go:105, lsdb_flooding_test.go:47) |
| `RFC3623-5-3` | The grace-LSAs are encapsulated in Link State Update Packets and sent out to all interfaces, even though the restarted router has no adjacencies and no knowledge of previous adjacencies. (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L43). **positive:** `unit/verify` [`TestUnplannedGraceLSAPerActiveInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L196). **negative:** no negative test. **{single-polarity}:** grOriginateGraceLSAs walks every active (non-passive) interface and floods each Grace-LSA via OriginateOpaque's standard LSU flooding (internal/plugins/ospf/gr_restarter.go:296, :315) |
| `RFC3623-5-4` | o The restart reason in the grace-LSAs must be set to 0 (unknown) or 3 (switch to redundant control processor). (§5) | MUST | 5 | **positive:** `unit/verify` [`TestRFC3623UnplannedGraceLSACarriesUnplannedReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L80). **positive:** `unit/verify` [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L58). **negative:** no negative test. **{single-polarity}:** grUnplannedReason returns the constant 3 (redundant control processor), an in-range unplanned reason, and there is no receive-side reason rejection to test negatively (internal/plugins/ospf/gr_restarter.go:88, gr.go:36) |
| `RFC3623-3-1` | When helping over a virtual link, the helper must also continue to set bit V in its router-LSA for the virtual link's transit area (Section 12.4.1 of [1]). (§3) | MUST | 3 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the helper role keeps X advertised in Router/Network-LSAs but neither gr_helper.go nor gr.go has any virtual-link/transit-area handling, so the transit-area V-bit is not preserved while helping over a virtual link (internal/plugins/ospf/gr_helper.go:279) |
| `RFC3623-3.1-1` | Router Y examines the link-state retransmission list for X over the associated network segment. - If there are any LSAs with LS types 1-5,7 on the list, then they all must be periodic refreshes. - If there are instead LSAs on the list whose contents have changed (see Section 3.3 of [7]), Y must refuse to enter helper mode. (§3.1) | MUST | 3.1 | **positive:** no positive test. **negative:** no negative test. **{gap}:** the helper entry decision has an lsdb-changed branch, but the production caller hardcodes lsdbUnchanged=true, so no production path refuses entry on a changed retransmission-list LSA; the permissive entry is only mitigated by the Section 3.2 strict-checking exit (internal/plugins/ospf/gr_helper.go:40, :128) |
| `RFC3623-3.2-1` | If Router Y aggregated adjacencies with Router X when entering helper mode (as described in section 3.1), it must also exit helper mode for all adjacencies with Router X when any one of the exit events occurs for an adjacency with Router X. (§3.2) | MUST | 3.2 | **positive:** no positive test. **negative:** no negative test. **{gap}:** gap in proof: internal/plugins/ospf/gr.go::helperKey meets it, since ze keys each helper relationship by interface and restarting router and never aggregates adjacencies across segments, so no aggregated exit is owed; no test asserts it |
| `RFC3623-2.1-2` | In order to avoid the restarting router's LSAs from aging out, the grace period should not exceed LSRefreshTime (1800 second) [1]. (§2.1) | SHOULD NOT | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.1-3` | If Router X wants to ensure that its neighbors receive the grace- LSAs, it should retransmit the grace-LSAs until they are acknowledged (i.e., perform standard OSPF reliable flooding of the grace-LSAs). (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.1-4` | After the grace-LSAs have been sent, the router should store the fact that it is performing graceful restart along with the length of the requested grace period in non-volatile storage. (§2.1) | SHOULD | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-1` | The router should reoriginate its router-LSAs for all attached areas in order to make sure they have the correct contents. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-2` | The router should reoriginate network-LSAs on all segments where it is the Designated Router. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-3` | Any remnant entries in the system forwarding table that were installed before the restart, but that are no longer valid, should be removed. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-4` | Any received self-originated LSAs that are no longer valid should be flushed. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.3-5` | Any grace-LSAs that the router originated should be flushed. (§2.3) | SHOULD | 2.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-2.1-5` | The router may need to preserve the cryptographic sequence numbers being used on each interface in non-volatile storage. An alternative is to use the router's clock for cryptographic sequence number generation and ensure that the clock is preserved across restarts (either on the same or redundant route processors). (§2.1) | MAY | 2.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-5-5` | To improve the probability that grace-LSAs will be delivered, an implementation may send them multiple times (see for example the Robustness Variable in [8]). (§5) | MAY | 5 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-3.1-2` | Router Y may optionally disallow graceful restart with Router X on other network segments. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-3.1-3` | Alternately, Router Y may choose to enter helper mode when a grace- LSA is received and the above checks pass for all adjacencies with Router X. (§3.1) | MAY | 3.1 | **positive:** no positive test. **negative:** no negative test |
| `RFC3623-3.2-2` | An implementation MAY provide a configuration option to disable link-state database options from terminating graceful restart. Such an option will, however, increase the risk of transient routing loops and black holes. (§3.2) | MAY | 3.2 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC3623-A-1`](#rfc3623-a-1) Additional Grace-LSA TLVs must be described in an Internet Draft and will be subject to the expert review of the OSPF Working Group. (§A) | {gap}, no test | an IETF process rule on additional Grace-LSA TLVs with no implementer role the RFC names; ze originates only TLV types 1, 2 and 3 and its decoder skips unknown types (internal/plugins/ospf/packet/grace_lsa.go), and no test asserts that ze emits no other TLV |
| [`RFC3623-3-1`](#rfc3623-3-1) When helping over a virtual link, the helper must also continue to set bit V in its router-LSA for the virtual link's transit area (Section 12.4.1 of [1]). (§3) | {gap}, no test | the helper role keeps X advertised in Router/Network-LSAs but neither gr_helper.go nor gr.go has any virtual-link/transit-area handling, so the transit-area V-bit is not preserved while helping over a virtual link (internal/plugins/ospf/gr_helper.go:279) |
| [`RFC3623-3.1-1`](#rfc3623-3.1-1) Router Y examines the link-state retransmission list for X over the associated network segment. - If there are any LSAs with LS types 1-5,7 on the list, then they all must be periodic refreshes. - If there are instead LSAs on the list whose contents have changed (see Section 3.3 of [7]), Y must refuse to enter helper mode. (§3.1) | {gap}, no test | the helper entry decision has an lsdb-changed branch, but the production caller hardcodes lsdbUnchanged=true, so no production path refuses entry on a changed retransmission-list LSA; the permissive entry is only mitigated by the Section 3.2 strict-checking exit (internal/plugins/ospf/gr_helper.go:40, :128) |
| [`RFC3623-3.2-1`](#rfc3623-3.2-1) If Router Y aggregated adjacencies with Router X when entering helper mode (as described in section 3.1), it must also exit helper mode for all adjacencies with Router X when any one of the exit events occurs for an adjacency with Router X. (§3.2) | {gap}, no test | gap in proof: internal/plugins/ospf/gr.go::helperKey meets it, since ze keys each helper relationship by interface and restarting router and never aggregates adjacencies across segments, so no aggregated exit is owed; no test asserts it |

## Proof state

A test reads unproven when no record shows it failing after a deliberate fault. Until then, its claim to check the requirement remains unproven.

### [`RFC3623-A-1`](#rfc3623-a-1)

Additional Grace-LSA TLVs must be described in an Internet Draft and will be subject to the expert review of the OSPF Working Group. (§A)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3623-A-1, so there is no linked test.

### [`RFC3623-A-2`](#rfc3623-a-2)

Grace Period (Type=1, length=4). The number of seconds that the router's neighbors should continue to advertise the router as fully adjacent, regardless of the state of database synchronization between the router and its neighbors. Since this time period began when grace-LSA's LS age was equal to 0, the grace period terminates when either: a) the LS age of the grace-LSA exceeds the value of a Grace Period or b) the grace-LSA is flushed. See Section 3.2 for other conditions that terminate graceful restart. This TLV must always appear in a grace-LSA. (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c23 re-judge (independent judge). Positive: TestRFC3623OriginatedGraceLSATLVOctets drives grOriginateGraceLSAs (the OSPFv2 origination, grV4Body) over running p2p eth0 and broadcast eth1 and compares the installed link-scope body with literal octets 00 01 00 04 00 00 00 78 (Type 1, Length 4, 120 s), read without the codec; an overlay GraceTLVPeriod=4 reds it while TestGraceLSARoundTrip and TestGraceLSAv4* stay green (scratch/c23/o1). Negative under owner ruling 2 (RFC 3623 prescribes no handling; the helper cannot run a window it was not given, so ignore): TestRFC3623HelperIgnoresGraceLSAMissingMandatoryTLV feeds a reason-only body through graceOnReceive while helping X and asserts the session survives with an unchanged grace end; the control with both TLVs moves it to now+600, so the input is isolated to the missing TLV. Decoder overlay without the hasPeriod check reds only missing-grace-period (o3). Both units hold observed-red revert records (EncodeGraceLSA, DecodeGraceLSA). The HEAD codec units stay supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L62) | unit/verify | revert, verified |
| negative | [`TestRFC3623HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L13) | unit/verify | revert, verified |
| positive | [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L41) | unit/verify | revert, verified |

### [`RFC3623-A-3`](#rfc3623-a-3)

Graceful restart reason (Type=2, length=1). Encodes the reason for the router restart as one of the following: 0 (unknown), 1 (software restart), 2 (software reload/upgrade) or 3 (switch to redundant control processor). This TLV must always appear in a grace-LSA. (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. c23 re-judge (independent judge). Positive: TestRFC3623OriginatedGraceLSATLVOctets asserts the originated OSPFv2 Grace-LSA carries 00 02 00 01 02 00 00 00 (Type 2, Length 1, reason 2, padded) at octets 8..15 on both p2p and broadcast, and the p2p body is exactly these 16 octets; overlay GraceTLVReason=5 reds it, old units green (o2). Negative under owner ruling 2: a period-only body (600) through graceOnReceive is ignored, same session, same grace end, control with both TLVs applied; decoder overlay without the hasReason check reds only missing-restart-reason (o4). Observed-red revert records on EncodeGraceLSA and DecodeGraceLSA. HEAD codec units supplementary.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestGraceLSADecodeMissingMandatory`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L59) | unit/verify | revert, verified |
| negative | [`TestRFC3623HelperIgnoresGraceLSAMissingMandatoryTLV`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L87) | unit/verify | revert, verified |
| positive | [`TestGraceLSARoundTrip`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/packet/rfc3623_grace_lsa_test.go#L17) | unit/verify | revert, verified |
| positive | [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L44) | unit/verify | revert, verified |

### [`RFC3623-A-4`](#rfc3623-a-4)

IP interface address (Type=3, length=4). The router's IP interface address on the subnet associated with the grace-LSA. Required on broadcast, NBMA and Point-to-MultiPoint segments, where the helper uses the IP interface address to identify the restarting router (see Section 3.1). (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21. Origination: TestRFC3623GraceLSANeverOmitsAddressOnSharedMedia requires the type-3 TLV with the address for broadcast, NBMA and P2MP through grSharedMedia. Use: gr_helper.go graceRestarterV4 matches body.InterfaceAddr against FloodNeighbors on shared media, refuses a missing TLV or unknown address, and uses the Advertising Router elsewhere. TestRFC3623HelperIdentifiesRestarterByInterfaceAddress now loops broadcast, NBMA and P2MP: Grace-LSA from A naming B's address helps B, not A, exactly one session; on P2P it helps A. TestRFC3623HelperRefusesUnmatchedInterfaceAddress: unknown address or no TLV gives zero sessions on all three. TestRFC3623HelperFlushExitsNamedRestarter: a flush from A naming B's address ends B's session and keeps A's, on all three. Keying on the Advertising Router reddens every one; the earlier broadcast-only limitation is gone.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC3623GraceLSANeverOmitsAddressOnSharedMedia`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_lsa_test.go#L60) | unit/verify | revert, verified |
| negative | [`TestRFC3623HelperRefusesUnmatchedInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_helper_identity_test.go#L169) | unit/verify | revert, verified |
| positive | [`TestGraceLSAv4BodyBuild`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_lsa_test.go#L19) | unit/verify | revert, verified |
| positive | [`TestRFC3623HelperFlushExitsNamedRestarter`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_helper_identity_test.go#L201) | unit/verify | revert, verified |
| positive | [`TestRFC3623HelperIdentifiesRestarterByInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_helper_identity_test.go#L131) | unit/verify | revert, verified |
| positive | [`TestRFC3623SharedMediaGraceLSACarriesInterfaceAddress`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestRFC3623OriginatedGraceLSATLVOctets`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_tlv_octets_test.go#L48) | unit/verify | revert, verified |

### [`RFC3623-A-5`](#rfc3623-a-5)

DoNotAge is never set in a grace-LSA, even if the grace-LSA is flooded over a demand circuit [7]. (§A)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden behaviour (a): a grace-LSA whose LS age has the DoNotAge bit set. (b) TestGraceLSANeverSetsDoNotAge drives OriginateOpaque with the restarter's body and asserts grace.Header.Age.DoNotAge() is false and RawBytes[0]&0x80 == 0, so an origination that set the bit goes red. The demand-circuit clause has no path in ze: no production code sets DoNotAgeBit (only types/lsage.go declares and reads it) and flooding copies the installed RawBytes, so the asserted bytes are the flooded bytes. Single-polarity marker holds.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestGraceLSANeverSetsDoNotAge`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L62) | unit/verify | unproven |

### [`RFC3623-2.1-1`](#rfc3623-2.1-1)

In preparation for the graceful restart, Router X must perform the following actions before its software is restarted/reloaded: (Note that common OSPF shutdown procedures are *not* performed, since we want the other OSPF routers to act as if Router X remains in continuous service. For example, Router X does not flush its locally originated LSAs, since we want them to remain in other routers' link-state databases throughout the restart period.) 1) Router X must ensure that its forwarding table(s) is/are up- to-date and will remain in place across the restart. (§2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21. TestPrepareRestartRetainsFIB now claims only the gate (suppressInstall after prepareRestart) and its record is on prepareRestart. Retention proof: TestRFC3623PreparedRestartKeepsRoutesSPFNoLongerComputes installs a route through SPF, prepares, withdraws it from the LSDB, reruns SPF and still finds it in the Loc-RIB; TestRFC3623PrepareInstallsPendingSPFAndKeepsIt (record grRefreshFIB) makes the table up to date first. Single-polarity positive.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3623PreparedRestartKeepsRoutesSPFNoLongerComputes`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_install_wiring_test.go#L85) | unit/verify | revert, verified |
| positive | [`TestPrepareRestartRetainsFIB`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L91) | unit/verify | revert, verified |
| positive | [`TestRFC3623PrepareInstallsPendingSPFAndKeepsIt`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_prepare_fib_test.go#L106) | unit/verify | revert, verified |

### [`RFC3623-5-1`](#rfc3623-5-1)

In any event, implementors providing the option to recover gracefully from unplanned outages must allow a network operator to turn the option off. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge). The operator surface is now driven: TestRFC3623OperatorTurnsUnplannedRecoveryOff parses 'graceful-restart restarter support' from configuration text and runs maybeUnplannedRestart: planned-and-unplanned enters in-restart, planned and disabled do not. A parser mapping every value to planned-and-unplanned reddens the negative (revert records on parseGRSupport).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestUnplannedDisabledByDefault`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L16) | unit/verify | revert, verified |
| negative | [`TestRFC3623OperatorTurnsUnplannedRecoveryOff`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L122) | unit/verify | revert, verified |
| positive | [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L40) | unit/verify | revert, verified |
| positive | [`TestRFC3623OperatorTurnsUnplannedRecoveryOff`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L119) | unit/verify | revert, verified |

### [`RFC3623-5-2`](#rfc3623-5-2)

The grace-LSAs must be originated and be sent *before* the restarted router sends any OSPF Hello Packets. On broadcast networks, this LSA must be flooded to the AllSPFRouters multicast address (224.0.0.5) since the restarting router is not aware of its previous DR state. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-21. TestUnplannedGraceLSAFloodsToAllSPFRouters now makes the router a restarting DROther (self-flush suppressed) with a Full neighbor that is DR, so the ordinary destination would be AllDRouters; only the restart-grace branch of floodLink sends to 224.0.0.5. Before-Hello ordering: TestRFC3623UnplannedColdStartOriginatesGraceLSA (record openInterfaces) and the lsdb restart flood unit. Single-polarity positive.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L45) | unit/verify | revert, verified |
| positive | [`TestUnplannedGraceLSAFloodsToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L154) | unit/verify | revert, verified |
| positive | [`TestRFC3623UnplannedColdStartOriginatesGraceLSA`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_unplanned_cold_start_test.go#L131) | unit/verify | revert, verified |

### [`RFC3623-5-3`](#rfc3623-5-3)

The grace-LSAs are encapsulated in Link State Update Packets and sent out to all interfaces, even though the restarted router has no adjacencies and no knowledge of previous adjacencies. (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Producer change re-read: floodLink now sends floodCopy(lsa, TransmitDelay) (RFC 2328 13.3 (5) age bump) on the restart branch instead of the database copy; the branch condition and destination (allSPFRoutersFor) are unchanged. floodLink sends a Grace-LSA originated in restart in a Link State Update even when no neighbor qualified for the retransmit list. TestRFC3623RestartGraceLSAFloodedToAllSPFRouters asserts one send with no neighbor while restarting and none outside restart; revert record on lsdb/link_scope.go::floodLink re-recorded against the new body and observed red. TestUnplannedGraceLSAPerActiveInterface pins one Grace-LSA per active interface. Single-polarity positive per the row annotation.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRFC3623RestartGraceLSAFloodedToAllSPFRouters`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/lsdb/rfc3623_restart_grace_flood_test.go#L43) | unit/verify | revert, verified |
| positive | [`TestUnplannedGraceLSAPerActiveInterface`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_positive_test.go#L196) | unit/verify | unproven |

### [`RFC3623-5-4`](#rfc3623-5-4)

o The restart reason in the grace-LSAs must be set to 0 (unknown) or 3 (switch to redundant control processor). (§5)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-09-29 (independent judge). TestRFC3623UnplannedGraceLSACarriesUnplannedReason configures planned-and-unplanned, runs maybeUnplannedRestart on a running interface and decodes the installed Grace-LSA: reason 3, equal to the recorded e.gr.reason (revert record on maybeUnplannedRestart). The {single-polarity} annotation stands: the unplanned reason is a constant and there is no receive-side reason rejection.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestUnplannedGraceBeforeHello`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_gr_unplanned_test.go#L58) | unit/verify | revert, verified |
| positive | [`TestRFC3623UnplannedGraceLSACarriesUnplannedReason`](https://github.com/ze-software/ze/blob/main/internal/plugins/ospf/rfc3623_grace_origination_test.go#L80) | unit/verify | revert, verified |

### [`RFC3623-3-1`](#rfc3623-3-1)

When helping over a virtual link, the helper must also continue to set bit V in its router-LSA for the virtual link's transit area (Section 12.4.1 of [1]). (§3)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3623-3-1, so there is no linked test.

### [`RFC3623-3.1-1`](#rfc3623-3.1-1)

Router Y examines the link-state retransmission list for X over the associated network segment. - If there are any LSAs with LS types 1-5,7 on the list, then they all must be periodic refreshes. - If there are instead LSAs on the list whose contents have changed (see Section 3.3 of [7]), Y must refuse to enter helper mode. (§3.1)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3623-3.1-1, so there is no linked test.

### [`RFC3623-3.2-1`](#rfc3623-3.2-1)

If Router Y aggregated adjacencies with Router X when entering helper mode (as described in section 3.1), it must also exit helper mode for all adjacencies with Router X when any one of the exit events occurs for an adjacency with Router X. (§3.2)

Audit verdict: not reviewed: no reviewer has checked whether these tests enforce the requirement

No test carries RFC3623-3.2-1, so there is no linked test.

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | claude |
| Signed off | 2026-09-21 |
| Register | prose |
| Source | rfc/full/rfc3623.txt |
| Source fingerprint | 8c64b96d2a23301e |
| Record | rfc/extraction/rfc3623.json |
| Mapped sentences | 12 |
| Declined as scope | 8 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 0 | reviewed | not stated |
| `1` | not stated | 1 | reviewed | not stated |
| `2` | not stated | 1 | reviewed | not stated |
| `2.1` | not stated | 2 | reviewed | not stated |
| `2.2` | not stated | 0 | reviewed | not stated |
| `2.3` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 1 | reviewed | not stated |
| `3.1` | not stated | 2 | reviewed | not stated |
| `3.2` | not stated | 1 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `5` | not stated | 6 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | not stated | 0 | reviewed | not stated |
| `8` | not stated | 1 | reviewed | not stated |
| `9` | not stated | 0 | reviewed | not stated |
| `9.1` | not stated | 0 | reviewed | not stated |
| `9.2` | not stated | 0 | reviewed | not stated |
| `A` | not stated | 4 | reviewed | not stated |
| `B` | not stated | 0 | reviewed | not stated |
| `B.1` | not stated | 0 | reviewed | not stated |
| `B.2` | not stated | 1 | reviewed | not stated |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction sentence describing what the mechanism needs from the neighbors ('which must cooperate in order for the restart to be graceful'); the helper obligations themselves are in Section 3. | Then there are the router's neighbors, which must cooperate in order for the restart to be graceful. |
| `2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Overview sentence announcing that Sections 2.1-2.3 follow ('it must change its OSPF processing somewhat'); it names no specific behavior. | After the router restarts/reloads, it must change its OSPF processing somewhat until it re-establishes full adjacencies with all its former fully-adjacent neighbors. |
| `2.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Lead-in to the numbered list that follows ('must perform the following actions'); the obligations are the list items, decided below. | In preparation for the graceful restart, Router X must perform the following actions before its software is restarted/reloaded: |
| `3.1:1` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | First half of the helper's retransmission-list check: RFC3623-3.1-1 states the same condition as its refusal ('LSAs whose contents have changed' rather than periodic refreshes). | - If there are any LSAs with LS types 1-5,7 on the list, then they all must be periodic refreshes. |
| `5:3` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Lead-in to the bullet list that follows ('The following points must be observed'); the obligations are the bullets, decided below. | The following points must be observed during this grace-LSA origination. |
| `5:5` | `duplicate-of` (never bound Ze): the same obligation is already captured under another requirement id | Second sentence of the same bullet; RFC3623-5-2 carries both the before-Hellos ordering and the AllSPFRouters (224.0.0.5) flood target. | On broadcast networks, this LSA must be flooded to the AllSPFRouters multicast address (224.0.0.5) since the restarting router is not aware of its previous DR state. |
| `8:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | RFC boilerplate: the IETF's standard intellectual-property notice inviting parties to disclose patents. It binds no implementation. | The IETF invites any interested party to bring to its attention any copyrights, patents or patent applications, or other proprietary rights which may cover technology that may be required to practice this standard. |
| `B.2:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | RFC Editor full-copyright statement ('this document itself may not be modified in any way'); it is a licence term, not a protocol requirement. | However, this document itself may not be modified in any way, such as by removing the copyright notice or references to the Internet Society or other Internet organizations, except as needed for the purpose of developing Internet standards in which case the procedures for copyrights defined in the Internet Standards process must be followed, or as required to translate it into languages other than English. |

## Superseded

No document obsoletes RFC 3623, so its obligations are stated where they were written.
