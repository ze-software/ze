# RFC 7705 - Autonomous System Migration Mechanisms and Their Effects on the BGP AS_PATH Attribute

Supported. Every requirement this repository extracted from RFC 7705, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 100.0% | 9 of 9 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 9 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 9 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 9 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 100.0% | 18 of 18 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 9 | of 17 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 9 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 9 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 9 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 9 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

The 7 shares marked as a part above are the whole of the 9 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| No test at all | ok | green at zero, RED above it: a binding obligation nothing exercises is a claim with nothing behind it, whether or not a reason is stated |
| Not applicable | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Met below Ze | neutral | no color: an obligation met below Ze is neither a test Ze wrote nor work Ze owes, and the two green shares above are what says how much Ze proves itself |
| Optional feature declined | neutral | no color: an obligation whose condition Ze never meets is neither an achievement nor a failure. The absent FEATURE is disclosed on the RFC's own status row, as an implementation gap a later scope decision can revisit |
| Proven by a recorded break | ok | green at every value: an observed break is the outcome the discrimination gate exists to produce. The denominator is TAGGED UNITS, not obligations, so this share is not one of the parts above |
| Audit verdicts | warn | RED on the first weak, wrong or unimplemented verdict, amber while a verdict is no longer current or a gated MUST is unjudged, green when every one is judged sound and current |

## At a glance

| Field | Value |
|---|---|
| Public status | Supported |
| Enrolment | Enrolled |
| Requirements | 17 |
| Gated MUST-level | 9 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 18 |
| Tagged units | 18 |
| Recorded audit verdicts | 0 |
| Discrimination records | 18 |
| Summary | `rfc/short/rfc7705.md` |
| Requirement shard | `rfc/requirements/rfc7705.md` |
| RFC text | `rfc/full/rfc7705.txt` |

## Enrolment

Enrolled: Autonomous System migration: the four de facto mechanisms a renumbering speaker runs, all four implemented, and all nine of this document's MUST-level requirements proven by a tagged test in both polarities. Section 3.3 "Local AS", "No Prepend Inbound" and "Replace Old AS" reach the AS_PATH through `secondaryPrependAS` and `localASPrependFor` (internal/component/bgp/reactor/peer_forward_facts.go). Section 4.2 "Internal BGP AS Migration" is `setMigrationAS`, `isIBGPWith`, `peerASAccepted`, `openLocalAS` and `noteASMigrationRejection` (internal/component/bgp/reactor/session_as_migration.go). The extraction sign-off is rfc/extraction/rfc7705.json: eight derived sites, all mapped, none excluded.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

- All four mechanisms. Configuration is one `session > asn` container read by `parsePeerSettings` ([`internal/component/bgp/reactor/config.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config.go)) out of the peer-fields grouping that a group and a peer both use, so `local`, `local-options` and `migration` are each settable per neighbor and per neighbor group ([`RFC7705-3.3-1`](#rfc7705-3.3-1), [`RFC7705-4.2-1`](#rfc7705-4.2-1)). Section 3.3: `local-options no-prepend` keeps the Local AS value out of the AS_PATH of a route received from that peer, and leaves the globally configured AS number on the path every other external neighbor receives ([`RFC7705-3.3-2`](#rfc7705-3.3-2), [`RFC7705-3.3-3`](#rfc7705-3.3-3))
- `local-options replace-as` sends the Local AS value alone toward that peer, with the globally configured AS number absent from AS_PATH and from AS4_PATH ([`RFC7705-3.3-4`](#rfc7705-3.3-4), [`RFC7705-3.3-5`](#rfc7705-3.3-5)). Section 4.2: `session > asn > migration` widens one iBGP session to exactly two of this speaker's AS numbers. An OPEN carrying either is accepted and any third AS is answered with OPEN Message Error / Bad Peer AS, which ze originates for the first time (`validateOpenPeerAS`, [`internal/component/bgp/reactor/session_open_as.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_as.go), [`RFC7705-4.2-2`](#rfc7705-4.2-2)). The OPEN ze sends carries the globally configured AS number first and the locally configured one after a peer answers Bad Peer AS, in My Autonomous System and in the Four-octet AS capability alike (`openLocalAS`, [`RFC7705-4.2-3`](#rfc7705-4.2-3), and the Section 4.2 SHOULD [`RFC7705-4.2-5`](#rfc7705-4.2-5) through `noteASMigrationRejection`). The session is internal under either AS number, from one rule every AS-scoped decision reads (`isIBGPWith`, [`RFC7705-4.2-4`](#rfc7705-4.2-4)), so no external AS_PATH prepend runs and the RFC 4456 reflection rules apply. Requirements bound per line below.


**What the ledger says remains**

Two advisory items, neither of them a MUST. [`RFC7705-3.3-9`](#rfc7705-3.3-9), the Section 3.3 SHOULD that the Local AS value be appended inbound before a route is installed or advertised to an internal neighbor, is not performed by the engine: an operator writes it as policy, and [`plan/spec-bgp-local-as-inbound-append.md`](https://github.com/ze-software/ze/blob/main/plan/spec-bgp-local-as-inbound-append.md) holds the ruling. [`RFC7705-3.3-11`](#rfc7705-3.3-11), the MAY letting an EXTERNAL speaker attempt the session under either AS number, is not offered: the two-AS session is iBGP only, because `setMigrationAS` refuses a remote AS that is neither of the pair. [`RFC7705-3.3-8`](#rfc7705-3.3-8), the SHOULD NOT that follows from that MAY, is therefore met by the scope decision rather than by a branch: no external session of ze establishes on the globally configured AS number while a Local AS is set for it.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 9 | one part of the gated population |
| Annotated instead of tested | 0 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| **Gated MUST-level requirements** | **9** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (9):** [`RFC7705-3.3-1`](#rfc7705-3.3-1), [`RFC7705-3.3-2`](#rfc7705-3.3-2), [`RFC7705-3.3-3`](#rfc7705-3.3-3), [`RFC7705-3.3-4`](#rfc7705-3.3-4), [`RFC7705-3.3-5`](#rfc7705-3.3-5), [`RFC7705-4.2-1`](#rfc7705-4.2-1), [`RFC7705-4.2-2`](#rfc7705-4.2-2), [`RFC7705-4.2-3`](#rfc7705-4.2-3), [`RFC7705-4.2-4`](#rfc7705-4.2-4)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC7705-3.3-1` | "The mechanisms introduced in this section MUST be configurable on a per-neighbor or per-neighbor-group basis to allow for maximum flexibility." (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/peers_test.go#L673). **negative:** `unit/verify` [`TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/peers_test.go#L678) |
| `RFC7705-3.3-2` | With "No Prepend Inbound", on inbound UPDATEs from the configured eBGP neighbour the router "MUST NOT append the 'Local AS' ASN value in the AS_PATH attribute when installing the route or advertising that UPDATE to iBGP neighbors" (§3.3) | MUST NOT | 3.3 | **positive:** `unit/verify` [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L380). **negative:** `unit/verify` [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L383) |
| `RFC7705-3.3-3` | With "No Prepend Inbound", the router "MUST still append the globally configured ASN as normal when advertising the UPDATE to other local eBGP neighbors" (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L388). **negative:** `unit/verify` [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L391) |
| `RFC7705-3.3-4` | With "Replace Old AS", on outbound UPDATEs toward the configured eBGP neighbour "the BGP speaker MUST NOT append the globally configured ASN from the AS_PATH attribute" (§3.3) | MUST NOT | 3.3 | **positive:** `unit/verify` [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L328). **negative:** `unit/verify` [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L331) |
| `RFC7705-3.3-5` | With "Replace Old AS", "The BGP router MUST append only the configured 'Local AS' ASN value to the AS_PATH attribute before sending the BGP UPDATEs outbound to the eBGP neighbor." (§3.3) | MUST | 3.3 | **positive:** `unit/verify` [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L335). **negative:** `unit/verify` [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L338) |
| `RFC7705-4.2-1` | "The mechanism introduced in this section MUST be configurable on a per-neighbor or per-neighbor-group basis to allow for maximum flexibility." (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestPeersFromConfigTree_ASMigrationPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/as_migration_test.go#L27). **negative:** `unit/verify` [`TestPeersFromConfigTree_ASMigrationPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/as_migration_test.go#L31) |
| `RFC7705-4.2-2` | A speaker configured with "Internal BGP AS Migration" "MUST accept BGP OPEN and establish an iBGP session from configured iBGP peers if the ASN value in 'My Autonomous System' is either the globally configured ASN or a locally configured ASN" (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestMigrationAcceptsEitherASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L79). **negative:** `unit/verify` [`TestMigrationAcceptsEitherASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L82) |
| `RFC7705-4.2-3` | A router configured with "Internal BGP AS Migration" "MUST send its own BGP OPEN ... using either the globally configured or the locally configured ASN in 'My Autonomous System'" (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestMigrationOpenCarriesResolvedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L139). **negative:** `unit/verify` [`TestMigrationOpenCarriesResolvedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L143) |
| `RFC7705-4.2-4` | "In each case, the BGP speaker MUST treat UPDATEs sent and received to this peer as if this was a natively configured iBGP session, as defined by [RFC4271] and [RFC4456]." (§4.2) | MUST | 4.2 | **positive:** `unit/verify` [`TestMigrationSessionIsIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L219). **negative:** `unit/verify` [`TestMigrationSessionIsIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L222) |
| `RFC7705-3.3-6` | To implement "Local AS", "a BGP speaker SHOULD send BGP OPEN ... messages to the configured eBGP peer(s) using the local ASN configured for this session as the value sent in 'My Autonomous System'" (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-3.3-7` | "The BGP router SHOULD NOT use the ASN configured globally within the BGP process as the value sent in 'My Autonomous System' in the OPEN message." (§3.3) | SHOULD NOT | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-3.3-8` | "If the session is successfully established to the globally configured ASN, then the modifications to AS_PATH described in this document SHOULD NOT be performed, as they are unnecessary." (§3.3) | SHOULD NOT | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-3.3-9` | Internal direction: "The router SHOULD append the configured 'Local AS' ASN in the AS_PATH attribute before installing the route or advertising the UPDATE to an iBGP neighbor." (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-3.3-10` | External direction: "The BGP router SHOULD first append the globally configured ASN to the AS_PATH immediately followed by the 'Local AS' value before advertising the UPDATE to an eBGP neighbor." (§3.3) | SHOULD | 3.3 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-4.2-5` | To avoid deadlock when both speakers run the mechanism, "the speaker SHOULD send BGP OPEN using the globally configured ASN first, and only send a BGP OPEN using the locally configured ASN as a fallback if the remote neighbor responds with the BGP error 'Bad Peer AS'" (§4.2) | SHOULD | 4.2 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-6-1` | "BGP sessions SHOULD be protected using TCP Authentication Option [RFC5925] and the Generalized TTL Security Mechanism [RFC5082]" (§6) | SHOULD | 6 | **positive:** no positive test. **negative:** no negative test |
| `RFC7705-3.3-11` | "Implementations MAY support a more flexible model where the eBGP speaker attempts to open the BGP session using either the ASN configured as 'Local AS' or the globally configured AS as discussed in BGP Alias" (§3.3) | MAY | 3.3 | **positive:** no positive test. **negative:** no negative test |

## Gaps and untested MUSTs

RFC 7705 declares no gap, and every gated MUST it carries has a test bound to it.

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC7705-3.3-1`](#rfc7705-3.3-1)

"The mechanisms introduced in this section MUST be configurable on a per-neighbor or per-neighbor-group basis to allow for maximum flexibility." (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/peers_test.go#L678) | unit/verify | revert, verified |
| positive | [`TestPeersFromConfigTree_LocalASOptionsPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/peers_test.go#L673) | unit/verify | revert, verified |

### [`RFC7705-3.3-2`](#rfc7705-3.3-2)

With "No Prepend Inbound", on inbound UPDATEs from the configured eBGP neighbour the router "MUST NOT append the 'Local AS' ASN value in the AS_PATH attribute when installing the route or advertising that UPDATE to iBGP neighbors" (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L383) | unit/verify | revert, verified |
| positive | [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L380) | unit/verify | revert, verified |

### [`RFC7705-3.3-3`](#rfc7705-3.3-3)

With "No Prepend Inbound", the router "MUST still append the globally configured ASN as normal when advertising the UPDATE to other local eBGP neighbors" (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L391) | unit/verify | revert, verified |
| positive | [`TestLocalASNoPrependLeavesEveryOutboundPathAlone`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L388) | unit/verify | revert, verified |

### [`RFC7705-3.3-4`](#rfc7705-3.3-4)

With "Replace Old AS", on outbound UPDATEs toward the configured eBGP neighbour "the BGP speaker MUST NOT append the globally configured ASN from the AS_PATH attribute" (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L331) | unit/verify | revert, verified |
| positive | [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L328) | unit/verify | revert, verified |

### [`RFC7705-3.3-5`](#rfc7705-3.3-5)

With "Replace Old AS", "The BGP router MUST append only the configured 'Local AS' ASN value to the AS_PATH attribute before sending the BGP UPDATEs outbound to the eBGP neighbor." (§3.3)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L338) | unit/verify | revert, verified |
| positive | [`TestLocalASReplaceASSendsOnlyTheLocalAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc7705_local_as_test.go#L335) | unit/verify | revert, verified |

### [`RFC7705-4.2-1`](#rfc7705-4.2-1)

"The mechanism introduced in this section MUST be configurable on a per-neighbor or per-neighbor-group basis to allow for maximum flexibility." (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestPeersFromConfigTree_ASMigrationPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/as_migration_test.go#L31) | unit/verify | revert, verified |
| positive | [`TestPeersFromConfigTree_ASMigrationPerNeighborGroup`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/config/as_migration_test.go#L27) | unit/verify | revert, verified |

### [`RFC7705-4.2-2`](#rfc7705-4.2-2)

A speaker configured with "Internal BGP AS Migration" "MUST accept BGP OPEN and establish an iBGP session from configured iBGP peers if the ASN value in 'My Autonomous System' is either the globally configured ASN or a locally configured ASN" (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMigrationAcceptsEitherASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L82) | unit/verify | revert, verified |
| positive | [`TestMigrationAcceptsEitherASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L79) | unit/verify | revert, verified |

### [`RFC7705-4.2-3`](#rfc7705-4.2-3)

A router configured with "Internal BGP AS Migration" "MUST send its own BGP OPEN ... using either the globally configured or the locally configured ASN in 'My Autonomous System'" (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMigrationOpenCarriesResolvedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L143) | unit/verify | revert, verified |
| positive | [`TestMigrationOpenCarriesResolvedASN`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L139) | unit/verify | revert, verified |

### [`RFC7705-4.2-4`](#rfc7705-4.2-4)

"In each case, the BGP speaker MUST treat UPDATEs sent and received to this peer as if this was a natively configured iBGP session, as defined by [RFC4271] and [RFC4456]." (§4.2)

Audit verdict: not audited: no reader has judged these tests

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestMigrationSessionIsIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L222) | unit/verify | revert, verified |
| positive | [`TestMigrationSessionIsIBGP`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_as_migration_test.go#L219) | unit/verify | revert, verified |

## Extraction sign-off

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
| `1` | not stated | 2 | walked | not stated |
| `1.1` | not stated | 0 | walked | not stated |
| `1.2` | not stated | 0 | walked | not stated |
| `2` | not stated | 0 | walked | not stated |
| `3` | not stated | 0 | walked | not stated |
| `3.1` | not stated | 0 | walked | not stated |
| `3.2` | not stated | 0 | walked | not stated |
| `3.3` | not stated | 4 | walked | not stated |
| `4` | not stated | 0 | walked | not stated |
| `4.1` | not stated | 1 | walked | not stated |
| `4.2` | not stated | 4 | walked | not stated |
| `5` | not stated | 0 | walked | not stated |
| `6` | not stated | 0 | walked | not stated |
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
