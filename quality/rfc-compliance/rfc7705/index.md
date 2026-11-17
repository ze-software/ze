# RFC 7705 - Autonomous System Migration Mechanisms and Their Effects on the BGP AS_PATH Attribute

AS migration: Local AS, No Prepend Inbound, Replace Old AS, Internal BGP AS Migration. Every requirement listed for RFC 7705, the tests linked to it, and their recorded reviews. This summary is included in the automatic compliance check, ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 9 | of 9 mandatory requirements | a positive test checks that Ze does what the requirement demands and a negative one checks that it refuses what the requirement forbids |
| One polarity plus reason | 0 | of 9 mandatory requirements | the requirement admits no counter-case, so one polarity plus a recorded reason supplies the complete test coverage required |
| One polarity, unexcused | 0 | of 9 mandatory requirements | one direction is tested, the other is neither tested nor excused, and nothing states which |
| Partial test coverage | 0 | of 9 mandatory requirements | scoped tests exist; the remaining obligation is unmet or unverified, with zero whole-requirement credit |
| Declared gap or no test | 0 | of 9 mandatory requirements | the requirements the summary declares a gap for, which say what Ze does not do yet, plus the ones no test carries and no annotation excuses. The at-a-glance table lists them apart: declared gaps (at every level), and mandatory requirements with no test and no annotation |
| Tests shown to detect a fault | 18 | 18 of 20 tests linked to requirements, 0 without a fault test and 2 with outdated fault tests | each test was observed to fail when the code it checks was deliberately broken. The test, its stated claim, and the relevant code are checked against the saved record for changes; the deliberate fault is not run again. Having both positive and negative tests verifies a requirement only after both tests have been shown to detect a fault |

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
| Mandatory requirements (MUST level) | 9 | of 17 requirements this summary lists, at all levels | The RFC's MUST, MUST NOT, SHALL, SHALL NOT and REQUIRED requirements included in the automatic checks: the compliance check (./le rfc check) holds Ze to each of them. This is the total being assessed; the counts above show the recorded status of those requirements. |

The 9 counts marked as a part above are the whole of the 9 mandatory requirements: they add up to 9. Tests shown to detect a fault counts individual tests rather than requirements, so its count is separate.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got. An outlined card with no colored edge, under Totals, is a total the other cards are counted against, not a measure.

| Card | Tone here | Why that color |
|---|---|---|
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the count under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: one test direction is missing without a recorded reason |
| Partial test coverage | ok | green at zero, RED above it: testing one clause does not verify the whole requirement |
| Declared gap or no test | ok | green at zero, RED above it: a binding obligation that nothing shows Ze meets is owed, whether or not a reason is stated |
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
| Public row | AS migration: Local AS, No Prepend Inbound, Replace Old AS, Internal BGP AS Migration |
| Automatic checks | Included |
| Requirements | 17 |
| Mandatory requirements (MUST level) | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Mandatory requirements with no test and no annotation | 0 |
| Nightly-only evidence | 0 |
| Test tags | 20 |
| Tests linked to requirements | 20 |
| Recorded audit verdicts | 9 |
| Fault test records | 20 |
| Summary | `rfc/short/rfc7705.md` |
| Requirement table | `rfc/requirements/rfc7705.md` |
| RFC text | `rfc/full/rfc7705.txt` |

## Automatic checks

Included in automatic checks: Autonomous System migration: the four de facto mechanisms a renumbering speaker runs, all four implemented, and all nine of this document's MUST-level requirements proven by a tagged test in both polarities. Section 3.3 "Local AS", "No Prepend Inbound" and "Replace Old AS" reach the AS_PATH through `secondaryPrependAS` and `localASPrependFor` (internal/component/bgp/reactor/peer_forward_facts.go). Section 4.2 "Internal BGP AS Migration" is `setMigrationAS`, `isIBGPWith`, `peerASAccepted`, `openLocalAS` and `noteASMigrationRejection` (internal/component/bgp/reactor/session_as_migration.go). The extraction sign-off is rfc/extraction/rfc7705.json: eight derived sites, all mapped, none excluded.

## Published support status

**Area:** AS migration: Local AS, No Prepend Inbound, Replace Old AS, Internal BGP AS Migration

**Test verification:** 9 of 9 MUST proven

**Evidence:** unit 9

**Unmet:** -


## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the mandatory requirements |
| Annotated (including scoped evidence) | 0 | one part of the mandatory requirements |
| Only a positive or negative test | 0 | one part of the mandatory requirements |
| No test and no annotation | 0 | one part of the mandatory requirements |
| Partial test coverage (subset of annotated; zero whole-requirement credit) | 0 | also counted in one of the groups above |
| Evidence that runs nightly only | 0 | also counted in one of the groups above |
| Derived from other rows | 0 | excluded from the total: each summarizes the status of other requirements, which are already counted above |
| **Mandatory requirements (MUST level)** | **9** | every mandatory requirement appears in exactly one group above |

**Positive and negative tests (9):** [`RFC7705-3.3-1`](#rfc7705-3.3-1), [`RFC7705-3.3-2`](#rfc7705-3.3-2), [`RFC7705-3.3-3`](#rfc7705-3.3-3), [`RFC7705-3.3-4`](#rfc7705-3.3-4), [`RFC7705-3.3-5`](#rfc7705-3.3-5), [`RFC7705-4.2-1`](#rfc7705-4.2-1), [`RFC7705-4.2-2`](#rfc7705-4.2-2), [`RFC7705-4.2-3`](#rfc7705-4.2-3), [`RFC7705-4.2-4`](#rfc7705-4.2-4)

## Figures

- **MUST bar**: Ze verified 9
- **Overview bar**: Ze MUST verified 9, Not assessed 8

| Figure | Rows |
|---|---|
| RFCs | 1 |
| Requirements not reviewed | 0 |
| Mandatory requirements complete | complete |
| MUST rows | 9 |
| MUST rows kept (the MUST bar) | 9 |
| Ze verified | 9 |
| OS verified | 0 |
| Tests awaiting verification | 0 |
| OS testing gap | 0 |
| Exclusion unchecked | 0 |
| Not assessed | 0 |
| Not built | 0 |
| MUST rows not owed | 0 |
| MUST not owed: feature-declined | 0 |
| MUST not owed: linux-limit | 0 |
| MUST not owed: not-applicable | 0 |
| SHOULD / MAY rows | 8 |
| SHOULD verified | 0 |
| SHOULD OS verified | 0 |
| SHOULD OS testing gap | 0 |
| SHOULD not owed | 0 |
| SHOULD not owed: feature-declined | 0 |
| SHOULD not owed: linux-limit | 0 |
| SHOULD not owed: not-applicable | 0 |
| SHOULD tests awaiting verification | 0 |
| SHOULD exclusion unchecked | 0 |
| SHOULD not assessed | 8 |
| SHOULD not built | 0 |

The MUST bar shows mandatory requirements (MUST, MUST NOT, REQUIRED, SHALL and SHALL NOT) that Ze and its operating system must meet. It includes only RFCs whose requirement lists have been checked against the original text. Ze verified means a test has been shown to fail when the required behavior is deliberately broken. Behavior provided by the operating system or a library is OS verified when a test checks it using the settings Ze installs and has also been shown to detect a deliberate fault; until then, it is an OS testing gap. Tests awaiting verification counts requirements a tagged test covers that no deliberate fault has yet been shown to break and that nothing records as unmet. Exclusion unchecked counts requirements claimed not to apply to Ze whose evidence is still to be checked. Not assessed counts requirements no tagged test covers and that nothing records as unmet, so nothing yet says whether Ze meets them. Not built counts requirements the RFC summary or a test review says Ze does not meet, in full or in part. The overview bar adds optional requirements (MAY, NOT RECOMMENDED, OPTIONAL, RECOMMENDED, SHOULD and SHOULD NOT) and requirements that do not apply to Ze. An RFC marked requirements not reviewed is excluded from these figures because its requirement list has not been checked against the original text. Counts are loaded from the RFC data files.

Source: [data/rfc/rfc7705.json](../../../data/rfc/rfc7705.json).

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7705-3.3-1` | The mechanisms introduced in this section MUST be configurable on a per-neighbor or per-neighbor-group basis to allow for maximum flexibility. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7705_peers_test.go#L702). **negative:** `unit/verify` [`TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7705_peers_test.go#L707) |
| `RFC7705-3.3-2` | "No Prepend Inbound": When the BGP router receives inbound BGP UPDATEs from its eBGP neighbor configured with this option, it MUST NOT append the "Local AS" ASN value in the AS_PATH attribute when installing the route or advertising that UPDATE to iBGP neighbors (§3.3) | MUST NOT | 3.3 | **positive:** `unit/verify` [`TestRFC7705NoPrependInstalledAndAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_live_behavior_test.go#L264). **negative:** `unit/verify` [`TestRFC7705NoPrependInstalledAndAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_live_behavior_test.go#L265) |
| `RFC7705-3.3-3` | it MUST still append the globally configured ASN as normal when advertising the UPDATE to other local eBGP neighbors (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L383). **negative:** `unit/verify` [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L386) |
| `RFC7705-3.3-4` | "Replace Old AS" (outbound): When the BGP router generates outbound BGP UPDATEs toward an eBGP neighbor configured with this option, the BGP speaker MUST NOT append the globally configured ASN from the AS_PATH attribute. (§3.3) | MUST NOT | 3.3 | **positive:** `unit/verify` [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L331). **negative:** `unit/verify` [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L334) |
| `RFC7705-3.3-5` | The BGP router MUST append only the configured "Local AS" ASN value to the AS_PATH attribute before sending the BGP UPDATEs outbound to the eBGP neighbor. (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L338). **negative:** `unit/verify` [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L341) |
| `RFC7705-4.2-1` | The mechanism introduced in this section MUST be configurable on a per-neighbor or per-neighbor-group basis to allow for maximum flexibility. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestPeersFromConfigTree_ASMigrationPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7705_as_migration_test.go#L27). **negative:** `unit/verify` [`TestPeersFromConfigTree_ASMigrationPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7705_as_migration_test.go#L31) |
| `RFC7705-4.2-2` | When configured with this mechanism, a BGP speaker MUST accept BGP OPEN and establish an iBGP session from configured iBGP peers if the ASN value in "My Autonomous System" is either the globally configured ASN or a locally configured ASN provided when this capability is utilized. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestMigrationAcceptsEitherASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_session_as_migration_test.go#L81). **positive:** `unit/verify` [`TestRFC7705MigratingPeerEstablishesAsIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_reactor_b_test.go#L24). **negative:** `unit/verify` [`TestMigrationAcceptsEitherASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_session_as_migration_test.go#L84). **negative:** `unit/verify` [`TestRFC7705MigratingPeerEstablishesAsIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_reactor_b_test.go#L25) |
| `RFC7705-4.2-3` | Additionally, a BGP router configured with this mechanism MUST send its own BGP OPEN [RFC4271] (see Section 4.2) using either the globally configured or the locally configured ASN in "My Autonomous System" as follows. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestMigrationOpenCarriesResolvedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_session_as_migration_test.go#L141). **negative:** `unit/verify` [`TestMigrationOpenCarriesResolvedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_session_as_migration_test.go#L145) |
| `RFC7705-4.2-4` | In each case, the BGP speaker MUST treat UPDATEs sent and received to this peer as if this was a natively configured iBGP session, as defined by [RFC4271] and [RFC4456]. (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestRFC7705MigrationWireSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_live_behavior_test.go#L290). **negative:** `unit/verify` [`TestRFC7705MigrationWireSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_live_behavior_test.go#L291) |
| `RFC7705-3.3-6` | To implement this mechanism, a BGP speaker SHOULD send BGP OPEN [RFC4271] (see Section 4.2) messages to the configured eBGP peer(s) using the local ASN configured for this session as the value sent in "My Autonomous System". (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-3.3-7` | The BGP router SHOULD NOT use the ASN configured globally within the BGP process as the value sent in "My Autonomous System" in the OPEN message. (§3.3) | SHOULD NOT | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-3.3-8` | If the session is successfully established to the globally configured ASN, then the modifications to AS_PATH described in this document SHOULD NOT be performed, as they are unnecessary. (§3.3) | SHOULD NOT | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-3.3-9` | The router SHOULD append the configured "Local AS" ASN in the AS_PATH attribute before installing the route or advertising the UPDATE to an iBGP neighbor. (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-3.3-10` | The BGP router SHOULD first append the globally configured ASN to the AS_PATH immediately followed by the "Local AS" value before advertising the UPDATE to an eBGP neighbor. (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-4.2-5` | To avoid potential deadlocks when two BGP speakers are attempting to establish a BGP peering session and are both configured with this mechanism, the speaker SHOULD send BGP OPEN using the globally configured ASN first, and only send a BGP OPEN using the locally configured ASN as a fallback if the remote neighbor responds with the BGP error "Bad Peer AS". (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-6-1` | Additionally, BGP sessions SHOULD be protected using TCP Authentication Option [RFC5925] and the Generalized TTL Security Mechanism [RFC5082] (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-3.3-11` | Implementations MAY support a more flexible model where the eBGP speaker attempts to open the BGP session using either the ASN configured as "Local AS" or the globally configured AS as discussed in BGP Alias (Section 4.2). (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7705 lists no missing behavior, and every mandatory requirement included in automatic checks has a linked test.

## Test status

A test reads not verified when no record shows it failing after a deliberate fault. Until then, the test has not been shown to detect a fault in the required behavior.

### [`RFC7705-3.3-1`](#rfc7705-3.3-1)

The mechanisms introduced in this section MUST be configurable on a per-neighbor or per-neighbor-group basis to allow for maximum flexibility. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/config/rfc7705_peers_test.go::TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup, internal/component/bgp/config/rfc7705_peers_test.go::TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup#2 moved. Forbidden: a Section 3.3 mechanism settable on neither a neighbor nor a neighbor group. TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup drives PeersFromConfigTree: the group's local (65010) and replace-as reach the inheriting peer (assert.Equal LocalAS, assert.True LocalASReplaceAS), and no-prepend set on a peer reaches that peer (assert.True LocalASNoPrepend); each of the three mechanisms goes red if it cannot be configured on the basis the test uses. Negative: the group options do not reach the peer outside the group or override the peer's own leaf-list.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7705_peers_test.go#L707) | unit/verify | revert, verified |
| positive | [`TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7705_peers_test.go#L702) | unit/verify | revert, verified |

### [`RFC7705-3.3-2`](#rfc7705-3.3-2)

"No Prepend Inbound": When the BGP router receives inbound BGP UPDATEs from its eBGP neighbor configured with this option, it MUST NOT append the "Local AS" ASN value in the AS_PATH attribute when installing the route or advertising that UPDATE to iBGP neighbors (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Re-judged 2026-10-31 by an independent auditor who authored none of the change. RFC text (rfc7705): ": When the BGP router receives inbound BGP UPDATEs from its eBGP neighbor configured with this option, it MUST NOT append the" be95c10053 replaced lowLiveSettings with acceptingLiveSettings for source and destination (FinalAccept import and export, no refs). An accepting policy neither refuses nor rewrites AS_PATH, so the absence of the prepended Local AS on installation and advertisement still comes from the No Prepend producer. Assertions unchanged. Verdict unchanged: enforced. Earlier judgements are in this file's git history.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7705NoPrependInstalledAndAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_live_behavior_test.go#L265) | unit/verify | revert, verified |
| positive | [`TestRFC7705NoPrependInstalledAndAdvertised`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_live_behavior_test.go#L264) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |

### [`RFC7705-3.3-3`](#rfc7705-3.3-3)

it MUST still append the globally configured ASN as normal when advertising the UPDATE to other local eBGP neighbors (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc7705_local_as_test.go::TestLocalASNoPrependLeavesEveryOutboundPathAlone, internal/component/bgp/reactor/rfc7705_local_as_test.go::TestLocalASNoPrependLeavesEveryOutboundPathAlone#2 moved. RFC 7705 Section 3.3: ""No Prepend Inbound": When the BGP router receives inbound BGP UPDATEs from its eBGP neighbor configured with this option, it MUST NOT append the "Local AS" ASN value in the AS_PATH attribute when installing the route or advertising that UPDATE to iBGP neighbors, but it MUST still append the globally configured ASN as normal when advertising the UPDATE to other local eBGP neighbors (i.e., those natively peering with the globally configured ASN)." The row owns the latter MUST, while 3.3-2 owns the inbound clause. TestLocalASNoPrependLeavesEveryOutboundPathAlone calls forwardLocalAS with noPrepend on the source, forwards actual NLRI, and compares decoded AS_PATH exactly: native eBGP [65000,65002], iBGP [65002], local-AS/no-prepend destination [legacy,global,source]. The first two independently pin append and its boundary. Producers peer_forward_facts.go secondaryPrependAS and session_as_migration.go isIBGPWith select the destination facts, not an outbound interpretation of the source's no-prepend option. A missing global ASN or blanket prepend fails. Independent source rejudgment only; no test, mutation, build or gate was executed by this auditor. Native observed-red renewal is a separate parent step.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L386) | unit/verify | revert, verified |
| positive | [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L383) | unit/verify | revert, verified |

### [`RFC7705-3.3-4`](#rfc7705-3.3-4)

"Replace Old AS" (outbound): When the BGP router generates outbound BGP UPDATEs toward an eBGP neighbor configured with this option, the BGP speaker MUST NOT append the globally configured ASN from the AS_PATH attribute. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc7705_local_as_test.go::TestLocalASReplaceASSendsOnlyTheLocalAS, internal/component/bgp/reactor/rfc7705_local_as_test.go::TestLocalASReplaceASSendsOnlyTheLocalAS#2 moved. Forbidden: appending the globally configured ASN toward a Replace Old AS eBGP neighbor. TestLocalASReplaceASSendsOnlyTheLocalAS asserts the replace-as destinations receive exactly [65010 65002] (assert.Equal), red if 65000 appears; TestLocalASOptionsProduceDifferentASPaths adds assert.NotContains 65000. Negative: the no-option destination receives [65010 65000 65002].

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L334) | unit/verify | revert, verified |
| positive | [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L331) | unit/verify | revert, verified |

### [`RFC7705-3.3-5`](#rfc7705-3.3-5)

The BGP router MUST append only the configured "Local AS" ASN value to the AS_PATH attribute before sending the BGP UPDATEs outbound to the eBGP neighbor. (§3.3)

Audit verdict: enforced (the tests do what the requirement demands), shifted: internal/component/bgp/reactor/rfc7705_local_as_test.go::TestLocalASReplaceASSendsOnlyTheLocalAS, internal/component/bgp/reactor/rfc7705_local_as_test.go::TestLocalASReplaceASSendsOnlyTheLocalAS#2 moved. Forbidden: toward a Replace Old AS eBGP neighbor, appending anything other than the Local AS value, or omitting it. TestLocalASReplaceASSendsOnlyTheLocalAS asserts exact equality with [65010 65002], red on a missing 65010 or any extra ASN. Negative: the no-option destination's [65010 65000 65002].

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L341) | unit/verify | revert, verified |
| positive | [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L338) | unit/verify | revert, verified |

### [`RFC7705-4.2-1`](#rfc7705-4.2-1)

The mechanism introduced in this section MUST be configurable on a per-neighbor or per-neighbor-group basis to allow for maximum flexibility. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: the Internal BGP AS Migration mechanism settable on neither a neighbor nor a neighbor group. TestPeersFromConfigTree_ASMigrationPerNeighborGroup drives PeersFromConfigTree: the group's migration 64510 reaches the inheriting peer and the peer's own migration 64520 reaches that peer (assert.Equal MigrationAS), red if either basis is not read. Negative: the peer outside the group keeps MigrationAS zero and stays eBGP.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestPeersFromConfigTree_ASMigrationPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7705_as_migration_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestPeersFromConfigTree_ASMigrationPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/rfc7705_as_migration_test.go#L27) | unit/verify | revert, verified |

### [`RFC7705-4.2-2`](#rfc7705-4.2-2)

When configured with this mechanism, a BGP speaker MUST accept BGP OPEN and establish an iBGP session from configured iBGP peers if the ASN value in "My Autonomous System" is either the globally configured ASN or a locally configured ASN provided when this capability is utilized. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: refusing the OPEN of a configured iBGP peer presenting the global or the locally configured ASN, or not establishing an iBGP session from it. TestRFC7705MigratingPeerEstablishesAsIBGP drives handleOpen then handleKeepalive for both ASNs and asserts Established, no NOTIFICATION and isIBGPWith(advertised) true; negative: a third ASN draws OPEN Message Error / Bad Peer AS and never reaches Established even after a KEEPALIVE. TestMigrationAcceptsEitherASN remains as OpenConfirm-level support.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7705MigratingPeerEstablishesAsIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_reactor_b_test.go#L25) | unit/verify | revert, verified |
| negative | [`TestMigrationAcceptsEitherASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_session_as_migration_test.go#L84) | unit/verify | revert, verified |
| positive | [`TestRFC7705MigratingPeerEstablishesAsIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_reactor_b_test.go#L24) | unit/verify | revert, verified |
| positive | [`TestMigrationAcceptsEitherASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_session_as_migration_test.go#L81) | unit/verify | revert, verified |

### [`RFC7705-4.2-3`](#rfc7705-4.2-3)

Additionally, a BGP router configured with this mechanism MUST send its own BGP OPEN [RFC4271] (see Section 4.2) using either the globally configured or the locally configured ASN in "My Autonomous System" as follows. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. Forbidden: sending an OPEN whose My Autonomous System is neither the global nor the locally configured ASN. TestMigrationOpenCarriesResolvedASN reads the OPEN buildOpen produces and asserts My AS and the Four-octet AS capability equal the global ASN before a Bad Peer AS and the locally configured ASN after it (assert.Equal MyAS, ASN4, capability ASN), red on any other AS. Negative: without migration configured the local AS is sent whatever the fallback flag holds.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestMigrationOpenCarriesResolvedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_session_as_migration_test.go#L145) | unit/verify | revert, verified |
| positive | [`TestMigrationOpenCarriesResolvedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_session_as_migration_test.go#L141) | unit/verify | revert, verified |

### [`RFC7705-4.2-4`](#rfc7705-4.2-4)

In each case, the BGP speaker MUST treat UPDATEs sent and received to this peer as if this was a natively configured iBGP session, as defined by [RFC4271] and [RFC4456]. (§4.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. RFC 7705 section 4.2 says: In each case, the BGP speaker MUST treat UPDATEs sent and received to this peer as if this was a natively configured iBGP session, as defined by RFC4271 and RFC4456. TestRFC7705MigrationWireSemantics compares native, retained-AS migration and legacy-AS migration sessions. Real ingress must install a third-party AS_PATH and LOCAL_PREF 231 unchanged; actual reflection must retain them without eBGP prepend and add exact ORIGINATOR_ID and CLUSTER_LIST. The distinct non-client/non-client forwarding case must return an error and emit no wire bytes. That negative case supplies resolved internal source facts, so it proves the forwarding boundary rather than independently proving source classification; the live positive case exercises source classification through isIBGPWith. Reading forwardUpdateSection confirms the destination classification and non-client suppression gate, reflection attributes and production forwarding path. These observations replace the old classification-only proof and address its named missing UPDATE behaviors. The startup commit changes formatting and fixture startup ordering without weakening assertions. Existing unit-changed discrimination records require renewal; no new mutant execution is claimed. Post-lint rejudgment 2026-10-04: lowLiveRouter no longer returns the discarded Reactor; the same running router and peers remain owned by cleanup. lowInstalledAttributes now embeds the identical source address 192.0.2.1 formerly passed by every caller, and still requires peer index, AFI/SAFI, prefix length and prefix bytes. Exact installed and transmitted attribute assertions and independent countercases are unchanged.

| Polarity | Test | Kind and tier | Test status |
|---|---|---|---|
| negative | [`TestRFC7705MigrationWireSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_live_behavior_test.go#L291) | unit/verify | revert, producer-changed (the producer's behavior changed since the break was applied to it) |
| positive | [`TestRFC7705MigrationWireSemantics`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_live_behavior_test.go#L290) | unit/verify | revert, verified |

## Requirement list review

| Field | Value |
|---|---|
| Reviewer | ze-implement agent, plan/immediate/spec-bgp-as-migration.md phase 8, rfc7705 enrolment walk |
| Signed off | 2026-09-14 |
| Register | prose |
| Source | rfc/full/rfc7705.txt |
| Source fingerprint | f7503ea9d187b305 |
| Record | rfc/extraction/rfc7705.json |
| Mapped sentences | 8 |
| Declined as scope | 4 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 1 | skipped (front-matter) | Title block, Abstract, Status of This Memo, Copyright Notice and the Table of Contents. It states what the document discusses and under which IETF process it was published, and it binds no implementation. |
| `1` | not stated | 2 | reviewed | not stated |
| `1.1` | not stated | 0 | reviewed | not stated |
| `1.2` | not stated | 0 | reviewed | not stated |
| `2` | not stated | 0 | reviewed | not stated |
| `3` | not stated | 0 | reviewed | not stated |
| `3.1` | not stated | 0 | reviewed | not stated |
| `3.2` | not stated | 0 | reviewed | not stated |
| `3.3` | not stated | 4 | reviewed | not stated |
| `4` | not stated | 0 | reviewed | not stated |
| `4.1` | not stated | 1 | reviewed | not stated |
| `4.2` | not stated | 4 | reviewed | not stated |
| `5` | not stated | 0 | reviewed | not stated |
| `6` | not stated | 0 | reviewed | not stated |
| `7` | References | 0 | skipped (references) | References. The heading over the two reference lists below it. |
| `7.1` | Normative References | 0 | skipped (references) | Normative References. Citation entries for RFC 2119, RFC 4271, RFC 4456, RFC 5082, RFC 5925 and RFC 6793. Each says where a cited document lives and binds nothing. |
| `7.2` | Informative References | 0 | skipped (references) | Informative References. Citation entries for RFC 5065, RFC 5398 and the AS migration drafts. Each says where a cited document lives and binds nothing. |
| `A` | Appendix A, Implementation Report | 0 | skipped (appendix-non-normative) | Appendix A, Implementation Report. Names the vendors whose shipping implementations the document describes (Cisco, Juniper, Alcatel-Lucent) and says the list is not exhaustive. It records what other implementations do and imposes nothing. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | IETF Trust Legal Provisions boilerplate the splitter did not strip: "Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions". It binds whoever copies code out of the document into another work, and this document contains no code component. It states nothing a BGP speaker does. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction, describing the base protocol this document then relaxes: "By default, the BGP protocol requires an operator to configure a router to use a single remote ASN for the BGP neighbor, and the ASN must match on both ends of the peering in order to successfully negotiate and establish a BGP session." Indicative, and about BGP-4 rather than about this document. The obligation it describes is RFC 4271 Section 6.2 Bad Peer AS, which rfc/short/rfc4271.md owns; this document's own normative sentence about it is site 4.2:2, which carves the migration exception out of that check and is mapped to RFC7705-4.2-2. | By default, the BGP protocol requires an operator to configure a router to use a single remote ASN for the BGP neighbor, and the ASN must match on both ends of the peering in order to successfully negotiate and establish a BGP session. |
| `1:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Introduction, a counterfactual about the world before these mechanisms existed: "Prior to the existence of these migration mechanisms, it would have required an ISP to coordinate an ASN change with, in some cases, tens of thousands of customers." The word 'required' sits inside 'it would have required', a past conditional describing what an ISP would have had to do, and it states no obligation on anybody now. | Prior to the existence of these migration mechanisms, it would have required an ISP to coordinate an ASN change with, in some cases, tens of thousands of customers. |
| `4.1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Step 2 of the section 4.1 migration recipe: "The 'Internal BGP AS Migration' mechanism is still required on all RRs for all RR Client iBGP sessions." It tells the operator carrying out the five-step renumbering which sessions must keep the mechanism configured while the walk is in progress, immediately after saying it 'is no longer needed' on the Non-Client sessions. It is a deployment instruction to the person running the migration, not a rule an implementation enforces: section 4.1 states outright that the mechanism 'can be enabled independent of the use of Route Reflectors'. The obligation this document places on the implementation is that the mechanism be configurable per neighbor or per neighbor group, which is site 4.2:1, mapped to RFC7705-4.2-1. | The "Internal BGP AS Migration" mechanism is still required on all RRs for all RR Client iBGP sessions. |

## Superseded

No document obsoletes RFC 7705, so its obligations are stated where they were written.
