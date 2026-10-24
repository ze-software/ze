# RFC 6286 - Autonomous-System-Wide Unique BGP Identifier for BGP-4

Supported. Every requirement this repository extracted from RFC 6286, the tests bound to it, and what a reader has verified about them. This summary is enrolled and gated by ./le rfc check.

## Overview

### Positive

what Ze has

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Tested both ways | 75.0% | 3 of 4 gated MUSTs | a positive test proves Ze does what the requirement demands and a negative one proves it refuses what the requirement forbids |
| One polarity plus reason | 0.0% | 0 of 4 gated MUSTs | the requirement admits no counter-case, so one polarity plus a recorded reason is the whole proof available for it |
| One polarity, unexcused | 0.0% | 0 of 4 gated MUSTs | one direction is tested, the other is neither tested nor excused, and nothing states which |
| No test at all | 0.0% | 0 of 4 gated MUSTs | no test carries the requirement id, whether or not a gap states why |
| Proven by a recorded break | 31.2% | 5 of 16 tagged units | a red was observed once under a recorded procedure, and the unit, the claim and the producer it rested on still hash to what was recorded. The break is not re-run. A test pair is not a proof until one has been observed |

### Neutral

measures that are neither good news nor bad

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Gated MUSTs | 4 | of 5 this summary declares | MUST-level requirements the gate HOLDS. A population, not a result: the shares beside it are what says how Ze stands |
| Out of scope | 0 | of 4 gated MUSTs | an obligation that does not bind Ze. A {not-applicable} annotation says it never bound; a {feature-declined} annotation says its condition is an optional feature Ze does not offer, and quotes the RFC sentence that makes it optional. Scope, not coverage: it stays in the denominator every share on this page is taken over |
| Not applicable | 0.0% | 0 of 4 gated MUSTs | a {not-applicable} annotation says the obligation does not bind Ze, so no test is owed for it. It stays in the denominator every share here is taken over |
| Met below Ze | 0.0% | 0 of 4 gated MUSTs | a {lower-layer} annotation says a layer under Ze performs the behavior, on state Ze installs into that layer, and names the producer that installs it. The obligation binds Ze and is met; Ze proves none of it, because its own boundary carries no value the behavior reads |
| Optional feature declined | 0.0% | 0 of 4 gated MUSTs | a {feature-declined} annotation says the obligation is conditional on a feature the RFC makes optional and Ze does not offer, and it quotes the sentence that makes it optional. The condition is false, so nothing is owed and nothing is missing. It stays in the denominator every share here is taken over |

### Negative

what Ze owes

| Measure | Value | Count | What it means |
|---|---:|---|---|
| Partial proof; remaining gap | 25.0% | 1 of 4 gated MUSTs | scoped tests exist; the remaining obligation is unmet or unproven, with zero whole-requirement credit |
| Audit verdicts | 5 | of 4 gated MUSTs judged | 1 weak, wrong or unimplemented, 0 no longer current. Each is named below under its own requirement id |

The 8 shares marked as a part above are the whole of the 4 gated MUSTs: they add to 100%. Proven by a recorded break is a share of TAGGED UNITS, a different population, so it is not one of them.

A color names what the measure MEANS, not how well Ze scores on it. Green is a good outcome at any value, red is a bad one, and neither a population nor a scope count is an outcome, so both take no color. The number under the label is what says how far Ze has got.

| Card | Tone here | Why that color |
|---|---|---|
| Gated MUSTs | neutral | no color: a population is a scale, and a larger one is neither good news nor bad. It is the accounting total |
| Out of scope | neutral | no color: an obligation that never bound Ze is neither an achievement nor a failure, and counting it either way would be a claim |
| Tested both ways | ok | green at every value: a test pair is the outcome this gate exists to produce, and the share under the label is what says how far Ze has got |
| One polarity plus reason | ok | green at every value: where no counter-case exists, one polarity IS the complete answer, and a recorded reason is what the gate demands beside it |
| One polarity, unexcused | ok | green at zero, RED above it: half a proof with no reason for the other half |
| Partial proof; remaining gap | bad | green at zero, RED above it: a tested clause cannot prove the whole requirement |
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
| Requirements | 5 |
| Gated MUST-level | 4 |
| Not applicable, so out of scope | 0 |
| Declared gaps | 0 |
| Declared gaps a test demonstrates | 0 |
| Gated with no test | 0 |
| Nightly-only evidence | 0 |
| Test tags | 16 |
| Tagged units | 16 |
| Recorded audit verdicts | 5 |
| Discrimination records | 5 |
| Summary | `rfc/short/rfc6286.md` |
| Requirement shard | `rfc/requirements/rfc6286.md` |
| RFC text | `rfc/full/rfc6286.txt` |

## Enrolment

Enrolled: Autonomous-System-Wide Unique BGP Identifier for BGP-4: three short protocol revisions to RFC 4271 that ze now implements end to end -- the Section 2.2 reception check on both OPEN rails, the Section 2.1 AS-wide identifier claim (and its operator opt-out), and the Section 2.3 collision tie-break.

## What the public ledger says

**Status:** Supported

**What the ledger says is covered**

- All three protocol revisions. Section 2.1: the local identifier is rejected when zero, on the global leaf and on a per-peer override (`parseRouterID`, [`internal/component/bgp/reactor/config.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config.go)), and AS-wide uniqueness is enforced by default through a claim taken during OPEN validation and released on session teardown ([`internal/component/bgp/reactor/routerid_unique.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/routerid_unique.go)), with `bgp/session/allow-shared-router-id` as the operator opt-out the lowercase "should" permits. Section 2.2: a received OPEN whose BGP Identifier is zero, or equals ze's own identifier and comes from an internal peer, is answered with OPEN Message Error / Bad BGP Identifier on BOTH receive rails -- normal receive and the connection that won collision resolution ([`internal/component/bgp/reactor/session_open_validation.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session_open_validation.go), called from session_handlers.go and session_connection.go)
- the same identifier from an external peer is accepted, as Section 2.2 requires. The LOSING connection of a collision is closed before Section 2.2 runs, so it receives Cease / Connection Collision (6/7) rather than Bad BGP Identifier even when its identifier is zero (`rejectConnectionCollision`, [`internal/component/bgp/reactor/reactor_connection.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reactor_connection.go)). The connection closes either way and RFC 4271 Section 6.8 does not fix the ordering, but the subcode differs and this row does not claim otherwise. Section 2.3: identical identifiers in a connection collision preserve the connection initiated by the larger AS number ([`internal/component/bgp/reactor/session.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/session.go) `DetectCollision`). Requirements bound per line in [`rfc/short/rfc6286.md`](https://github.com/ze-software/ze/blob/main/rfc/short/rfc6286.md).


**What the ledger says remains**

Section 2.1: the identifier is the same for every peer only when no peer or group sets `session router-id`; the global `router-id` is the default and a peer or group may override it (`parsePeerSettings`, [`internal/component/bgp/reactor/config.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config.go)), an owner-approved deviation from Section 2.1 (owner, 2026-10-13, D4 of the BGP OPEN session RFC defects spec) that is not counted conformant. Section 2.1 also says the value "is determined on startup"; ze applies a changed global `router-id` while it runs: a SIGHUP reload stores it (`reactorAPIAdapter.applyGlobalsJournaled`, [`internal/component/bgp/reactor/reload_globals.go`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/reload_globals.go)) and the peer diff restarts every peer whose effective identifier changed, so each sends the new identifier in a new OPEN ([`test/reload/reload-global-router-id.ci`](https://github.com/ze-software/ze/blob/main/test/reload/reload-global-router-id.ci)); the owner kept that behaviour as it ships (2026-10-13, Q3 of the same spec: "Out of scope"), a second departure from Section 2.1 under the same [`RFC6286-2.1-1`](#rfc6286-2.1-1) row, which already leaves the conformance count. Section 2.3 applies only where RFC 4271 Section 6.8 collision detection already applies (OpenConfirm): ze does not implement the Section 6.8 OpenSent MAY, tracked under the RFC 4271 row.

## Coverage

| Bucket | Count | What it counts |
|---|---|---|
| Positive and negative tests | 3 | one part of the gated population |
| Annotated (including scoped evidence) | 1 | one part of the gated population |
| One polarity only | 0 | one part of the gated population |
| No test and no annotation | 0 | one part of the gated population |
| Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) | 1 | an overlay: each of these is also counted by the part it falls in |
| Evidence that runs nightly only | 0 | an overlay: each of these is also counted by the part it falls in |
| Derived from other rows | 0 | outside the gated population: each asserts nothing and derives its state from the rows it names, which the parts above already count |
| **Gated MUST-level requirements** | **4** | every gated MUST falls in exactly one bucket above |

**Positive and negative tests (3):** [`RFC6286-2.2-1`](#rfc6286-2.2-1), [`RFC6286-2.2-2`](#rfc6286-2.2-2), [`RFC6286-2.3-1`](#rfc6286-2.3-1)

**Annotated (including scoped evidence) (1):** [`RFC6286-2.1-1`](#rfc6286-2.1-1)

**Partial proof; remaining gap (subset of annotated; zero whole-requirement credit) (1):** [`RFC6286-2.1-1`](#rfc6286-2.1-1)

## Requirements

| Requirement | Text | Level | Section | Tests |
|---|---|---|---|---|
| `RFC6286-2.1-1` | The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer. (Section 2.1) | MUST | 2.1 - Definition of the BGP Identifier | **positive:** `unit/verify` [`TestParsePeerFromTreeInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L128). **positive:** `unit/verify` [`TestRFC6286GlobalZeroRouterIDRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L35). **negative:** `unit/verify` [`TestRFC6286GlobalZeroRouterIDRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L39). **negative:** `unit/verify` [`TestRFC6286NonZeroRouterIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L10). **{partial}:** tested "non-zero integer"; gap "is the same for every local interface and every BGP peer"; owner-approved deviation (spec-bgp-open-session-rfc-defects Q2, owner 2026-10-13: "we should have a global router-id which can be overwritten per peer which is over what the RFC allows"), not counted conformant: the global `router-id` is the default and a peer or group may override it with the `session router-id` leaf, and each OPEN carries its own peer's identifier, so a configured override gives that peer an identifier different from the others'; internal/component/bgp/reactor/config.go::parsePeerSettings reads the override. The non-zero clause is enforced: parseRouterID rejects 0.0.0.0 for the global leaf and a per-peer override. **Scoped evidence:** Tests and tag-claim records apply only to Tested; zero whole-requirement credit. partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Re-judged 2026-10-24 (independent auditor, authored none of the change). RFC 6286 Section 2.1: "The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer." 5ac5c3fc0d changed the Producer reactor/config.go::parsePeerSettings only to call applyASSetPolicy (session as-set accept on a migrated peer); the session router-id override the Gap names and the parseRouterID zero refusal are unchanged, so Tested 'non-zero integer' and Gap 'is the same for every local interface and every BGP peer' stand, Producer internal/component/bgp/reactor/config.go::parsePeerSettings. Partial, zero whole-requirement credit. Verdict unchanged: partial. Earlier judgement follows. Re-judged 2026-10-21 (Phase 7 agent, not the author of 72a56ddfbf) after 72a56ddfbf changed parsePeerSettings: the only change parses the peer shutdown presence container into ps.AdminShutdown; the session router-id override read the Gap names and the parseRouterID zero refusal are unchanged, so Tested 'non-zero integer' and Gap 'is the same for every local interface and every BGP peer' stand, Producer internal/component/bgp/reactor/config.go::parsePeerSettings. Verdict unchanged. Earlier judgement follows. Re-judged 2026-10-21 by an independent judge (spec-bgp-open-session-rfc-defects AC-4) against the owner-approved deviation record and rfc/full/rfc6286.txt Section 2.1: 'The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer.' Row is {partial}, not counted conformant. Tested: 'non-zero integer'. Gap: 'is the same for every local interface and every BGP peer', the owner deviation (Q2, 2026-10-13): Producer internal/component/bgp/reactor/config.go::parsePeerSettings reads the per-peer/group session router-id override, so an override gives that peer a different identifier. Within Tested: TestParsePeerFromTreeInvalid zero_router_id_rejected (peer-level 0.0.0.0 refused with the RFC 6286 error), TestRFC6286GlobalZeroRouterIDRejected (global 0.0.0.0 refused by PeersFromTree, 0.0.0.1 accepted and inherited as 1), TestRFC6286NonZeroRouterIDAccepted (peer-level 0.0.0.1 becomes 1); parseRouterID refuses id == 0 alone, and the four parseRouterID records verify. Second departing clause not named in Gap or Support remaining: 'determined on startup' -- reload_globals.go applyGlobalsJournaled applies a changed global router-id while ze runs (Q3, owner 2026-10-13: out of scope, ships as is), so the identifier is not fixed at startup; the whole requirement remains unproven and the disclosure owes that clause. Limit kept from the 2026-10-14 judgement: a config with no router-id at any level is refused only by the config mandatory check, which no tag here exercises. |
| `RFC6286-2.1-2` | The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. (Section 2.1) | SHOULD | 2.1 - Definition of the BGP Identifier | **positive:** `unit/verify` [`TestRFC6286LocalIdentifierUniqueWithinItsAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_unique_within_as_test.go#L24). **positive:** `unit/verify` [`TestRouterIDClaimConcurrentOnlyOneWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_routerid_unique_test.go#L453). **negative:** no negative test. **{single-polarity}:** the negative case -- ze accepting a duplicate -- is the operator-selected `bgp/session/allow-shared-router-id true` path, which is conformant precisely because the requirement is a lowercase "should", so there is no violation for a negative test to catch. Enforcement is proven by TestRouterIDClaimConcurrentOnlyOneWins and the TestRouterIDConflict* family; the opt-out by TestValidateOpenAllowSharedRouterID |
| `RFC6286-2.2-1` | If the BGP Identifier field of the OPEN message is zero, or if it is the same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then the Error Subcode is set to "Bad BGP Identifier". (Section 2.2) | MUST | 2.2 - Open Message Error Handling | **positive:** `unit/verify` [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L105). **positive:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L445). **positive:** `unit/verify` [`TestProcessOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L163). **negative:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L447) |
| `RFC6286-2.2-2` | If the BGP Identifier field of the OPEN message is zero, or if it is the same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then the Error Subcode is set to "Bad BGP Identifier". (Section 2.2) | MUST | 2.2 - Open Message Error Handling | **positive:** `unit/verify` [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L107). **positive:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L449). **negative:** `unit/verify` [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L109). **negative:** `unit/verify` [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L451) |
| `RFC6286-2.3-1` | If the BGP Identifiers of the peers involved in the connection collision are identical, then the connection initiated by the BGP speaker with the larger AS number is preserved. (Section 2.3) | MUST | 2.3 - Connection Collision Resolution | **positive:** `unit/verify` [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L600). **negative:** `unit/verify` [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L604) |

## Gaps and untested MUSTs

| Requirement | State | Reason |
|---|---|---|
| [`RFC6286-2.1-1`](#rfc6286-2.1-1) The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer. (Section 2.1) | {partial}, Partial proof; remaining gap | tested "non-zero integer"; gap "is the same for every local interface and every BGP peer"; owner-approved deviation (spec-bgp-open-session-rfc-defects Q2, owner 2026-10-13: "we should have a global router-id which can be overwritten per peer which is over what the RFC allows"), not counted conformant: the global `router-id` is the default and a peer or group may override it with the `session router-id` leaf, and each OPEN carries its own peer's identifier, so a configured override gives that peer an identifier different from the others'; internal/component/bgp/reactor/config.go::parsePeerSettings reads the override. The non-zero clause is enforced: parseRouterID rejects 0.0.0.0 for the global leaf and a per-peer override |

## Proof state

A tagged unit reads unproven where no discrimination record exists for it: nothing in this tree has been observed to break it, so the claim its tag makes is unproven.

### [`RFC6286-2.1-1`](#rfc6286-2.1-1)

The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer. (Section 2.1)

Scoped tag-claim records only: tested "non-zero integer"; gap "is the same for every local interface and every BGP peer"; owner-approved deviation (spec-bgp-open-session-rfc-defects Q2, owner 2026-10-13: "we should have a global router-id which can be overwritten per peer which is over what the RFC allows"), not counted conformant: the global `router-id` is the default and a peer or group may override it with the `session router-id` leaf, and each OPEN carries its own peer's identifier, so a configured override gives that peer an identifier different from the others'; internal/component/bgp/reactor/config.go::parsePeerSettings reads the override. The non-zero clause is enforced: parseRouterID rejects 0.0.0.0 for the global leaf and a per-peer override; zero whole-requirement credit.

Audit verdict: partial (the tests enforce only the declared tested scope; the whole requirement remains unmet), fresh. Re-judged 2026-10-24 (independent auditor, authored none of the change). RFC 6286 Section 2.1: "The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer." 5ac5c3fc0d changed the Producer reactor/config.go::parsePeerSettings only to call applyASSetPolicy (session as-set accept on a migrated peer); the session router-id override the Gap names and the parseRouterID zero refusal are unchanged, so Tested 'non-zero integer' and Gap 'is the same for every local interface and every BGP peer' stand, Producer internal/component/bgp/reactor/config.go::parsePeerSettings. Partial, zero whole-requirement credit. Verdict unchanged: partial. Earlier judgement follows. Re-judged 2026-10-21 (Phase 7 agent, not the author of 72a56ddfbf) after 72a56ddfbf changed parsePeerSettings: the only change parses the peer shutdown presence container into ps.AdminShutdown; the session router-id override read the Gap names and the parseRouterID zero refusal are unchanged, so Tested 'non-zero integer' and Gap 'is the same for every local interface and every BGP peer' stand, Producer internal/component/bgp/reactor/config.go::parsePeerSettings. Verdict unchanged. Earlier judgement follows. Re-judged 2026-10-21 by an independent judge (spec-bgp-open-session-rfc-defects AC-4) against the owner-approved deviation record and rfc/full/rfc6286.txt Section 2.1: 'The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer.' Row is {partial}, not counted conformant. Tested: 'non-zero integer'. Gap: 'is the same for every local interface and every BGP peer', the owner deviation (Q2, 2026-10-13): Producer internal/component/bgp/reactor/config.go::parsePeerSettings reads the per-peer/group session router-id override, so an override gives that peer a different identifier. Within Tested: TestParsePeerFromTreeInvalid zero_router_id_rejected (peer-level 0.0.0.0 refused with the RFC 6286 error), TestRFC6286GlobalZeroRouterIDRejected (global 0.0.0.0 refused by PeersFromTree, 0.0.0.1 accepted and inherited as 1), TestRFC6286NonZeroRouterIDAccepted (peer-level 0.0.0.1 becomes 1); parseRouterID refuses id == 0 alone, and the four parseRouterID records verify. Second departing clause not named in Gap or Support remaining: 'determined on startup' -- reload_globals.go applyGlobalsJournaled applies a changed global router-id while ze runs (Q3, owner 2026-10-13: out of scope, ships as is), so the identifier is not fixed at startup; the whole requirement remains unproven and the disclosure owes that clause. Limit kept from the 2026-10-14 judgement: a config with no router-id at any level is refused only by the config mandatory check, which no tag here exercises.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestRFC6286GlobalZeroRouterIDRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L39) | unit/verify | revert, verified |
| negative | [`TestRFC6286NonZeroRouterIDAccepted`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L10) | unit/verify | revert, verified |
| positive | [`TestParsePeerFromTreeInvalid`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/config_test.go#L128) | unit/verify | revert, verified |
| positive | [`TestRFC6286GlobalZeroRouterIDRejected`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_router_id_test.go#L35) | unit/verify | revert, verified |

### [`RFC6286-2.1-2`](#rfc6286-2.1-2)

The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. (Section 2.1)

Audit verdict: enforced (the tests do what the requirement demands), fresh. {single-polarity: positive} valid: lowercase should, the duplicate is the operator's allow-shared-router-id opt-out. Both halves of 'unique within an AS' now covered: remote peers by TestRouterIDClaimConcurrentOnlyOneWins (accepted == 1, routerIDConflictError); this speaker's own identifier by TestRFC6286LocalIdentifierUniqueWithinItsAS, which presents the local identifier 0x01020301 from an internal peer (AS 65001 both sides) on handleOpen and requires ErrBadBGPIdentifier, NOTIFICATION 2/3 on the wire and no OpenConfirm. Producer read: Open.ValidateBGPIdentifier (internal && id == localID). Judge break (overlay, open.go: the self check gated on !internal) turned the new unit red, the remote-peer unit green. Revert record on ValidateBGPIdentifier for the new unit; the HEAD unit still carries no record (proof by reading: the exact accepted == 1 assertion).

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| positive | [`TestRouterIDClaimConcurrentOnlyOneWins`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_routerid_unique_test.go#L453) | unit/verify | unproven |
| positive | [`TestRFC6286LocalIdentifierUniqueWithinItsAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_unique_within_as_test.go#L24) | unit/verify | revert, verified |

### [`RFC6286-2.2-1`](#rfc6286-2.2-1)

If the BGP Identifier field of the OPEN message is zero, or if it is the same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then the Error Subcode is set to "Bad BGP Identifier". (Section 2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) accepting an OPEN whose BGP Identifier is zero; (b) TestOpenValidateBGPIdentifier: zero from internal and from external wantErr with NotifyOpenMessage / NotifyOpenBadBGPID; negative: 1 and 0xFFFFFFFF accepted. The same unit also proves the sentence's second clause (local id from internal rejected, from external accepted). handleOpen and processOpen rails tagged positive too.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L447) | unit/verify | unproven |
| positive | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L445) | unit/verify | unproven |
| positive | [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L105) | unit/verify | unproven |
| positive | [`TestProcessOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L163) | unit/verify | unproven |

### [`RFC6286-2.2-2`](#rfc6286-2.2-2)

If the BGP Identifier field of the OPEN message is zero, or if it is the same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then the Error Subcode is set to "Bad BGP Identifier". (Section 2.2)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) accepting the local identifier from an internal peer, or rejecting it from an external one; (b) TestOpenValidateBGPIdentifier rows 'local identifier from internal peer' wantErr 2/3 and 'from external peer' no error, plus localID+-1 accepted; zero rows cover the first clause. TestHandleOpenRejectsBadBGPIdentifier repeats both polarities on the FSM rail.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L451) | unit/verify | unproven |
| negative | [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L109) | unit/verify | unproven |
| positive | [`TestOpenValidateBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/message/open_test.go#L449) | unit/verify | unproven |
| positive | [`TestHandleOpenRejectsBadBGPIdentifier`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/rfc6286_session_open_validation_test.go#L107) | unit/verify | unproven |

### [`RFC6286-2.3-1`](#rfc6286-2.3-1)

If the BGP Identifiers of the peers involved in the connection collision are identical, then the connection initiated by the BGP speaker with the larger AS number is preserved. (Section 2.3)

Audit verdict: enforced (the tests do what the requirement demands), fresh. (a) with identical identifiers, preserving the connection of the smaller AS; (b) TestDetectCollisionEqualIdentifierPrefersLargerAS asserts accept/close for peer AS > local AS and peer AS < local AS with equal identifiers; negative rows show unequal identifiers ignore the AS.

| Polarity | Test | Kind and tier | Proof state |
|---|---|---|---|
| negative | [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L604) | unit/verify | unproven |
| positive | [`TestDetectCollisionEqualIdentifierPrefersLargerAS`](https://github.com/ze-software/ze/blob/main/internal/component/bgp/reactor/collision_test.go#L600) | unit/verify | unproven |

## Extraction sign-off

| Field | Value |
|---|---|
| Reviewer | ze-work agent, spec-rfcgate-6, rfc6286 |
| Signed off | 2026-08-31 |
| Register | prose |
| Source | rfc/full/rfc6286.txt |
| Source fingerprint | c09b7508ad62e214 |
| Record | rfc/extraction/rfc6286.json |
| Mapped sentences | 0 |
| Declined as scope | 3 |
| Relocated to a spec, which Ze OWES | 0 |
| Unclassified | 0 |

### Sections

| Section | Name | Sites | Disposition | Reason |
|---|---|---|---|---|
| `front` | not stated | 2 | walked | Title block, Abstract, Status of This Memo and Copyright Notice. Walked rather than skipped because the site scan attributes two sites here, both excluded below: the Abstract's restatement of what the document does to RFC 4271, and the IETF Trust Legal Provisions boilerplate. Nothing before section 1 binds a BGP speaker. |
| `1` | Introduction | 1 | walked | Introduction. Reports that RFC 4271 specifies the BGP Identifier as a valid IPv4 host address assigned to the speaker, that deployed code requires two speakers to carry distinct Identifiers, and that this document relaxes both so an IPv6-only network can comply. Its one site, 1:1, is that report and is excluded below. The relaxed definition itself is stated in section 2.1. |
| `2` | Protocol Revisions, the parent heading | 0 | walked | Protocol Revisions, the parent heading. One sentence naming what the revisions cover: the definition of the BGP Identifier and the procedures for a speaker that supports the AS-wide Unique BGP Identifier. It directs nobody; 2.1, 2.2 and 2.3 carry the obligations. |
| `2.1` | Definition of the BGP Identifier | 0 | walked | Definition of the BGP Identifier. The obligation is the indented replacement text: 'The BGP Identifier is a 4-octet, unsigned, non-zero integer that should be unique within an AS. The value of the BGP Identifier for a BGP speaker is determined on startup and is the same for every local interface and every BGP peer.' It is written in the indicative with a lowercase 'should', so neither the capitalised scan nor the modal scan attributes a site to this section, and both ids read from it are declared unsourced here. RFC6286-2.1-1 is the 4-octet, unsigned, NON-ZERO definition, whose non-zero half ze enforces on reception through Open.ValidateBGPIdentifier and refuses to originate through parseRouterID (internal/component/bgp/reactor/config.go). RFC6286-2.1-2 is the lowercase 'should' of AS-wide uniqueness, enforced by default through the claim registry in internal/component/bgp/reactor/routerid_unique.go, which keys routerIDKey on (peer AS, identifier), with bgp/session/allow-shared-router-id as the operator opt-out the advisory level permits. |
| `2.2` | Open Message Error Handling | 0 | walked | Open Message Error Handling. The obligation is the indented replacement text for RFC 4271 section 6.2: 'If the BGP Identifier field of the OPEN message is zero, or if it is the same as the BGP Identifier of the local BGP speaker and the message is from an internal peer, then the Error Subcode is set to "Bad BGP Identifier".' Written in the indicative with no modal at all, so no scan attributes a site to this section. It states two rejection conditions and the second is gated on the peer being internal, which is why the summary declares two ids and both are declared unsourced here. Session.validateOpenIdentifier (internal/component/bgp/reactor/session_open_validation.go) is the producer for both, on the handleOpen and processOpen rails. |
| `2.3` | Connection Collision Resolution | 0 | walked | Connection Collision Resolution. The obligation is the indented extension to RFC 4271 section 6.8: 'If the BGP Identifiers of the peers involved in the connection collision are identical, then the connection initiated by the BGP speaker with the larger AS number is preserved.' Indicative, no modal, no site. Declared unsourced here as RFC6286-2.3-1. The lead-in scopes it to an external peer, because an internal peer presenting this speaker's identifier is already rejected by 2.2, and the closing sentence says the extension covers 4-octet AS numbers [RFC4893]. Both facts qualify the same single obligation and neither adds one. |
| `3` | Remarks | 0 | walked | Remarks. Five indicative observations, none of which directs a speaker: an Identifier allocated per RFC 4271 already fits the revised definition; a confederation counts as one AS for the purpose of AS-wide uniqueness; a speaker supporting this cannot share an Identifier with an external neighbor until that neighbor is upgraded, which is a consequence of the un-upgraded neighbor still applying RFC 4271's stricter test; and the Identifier is also used in AGGREGATOR, in the route-reflection attributes and in route selection. The section's own closing sentence states its purpose: to conclude that the revisions introduce no backward compatibility issue. The confederation observation scopes the lowercase 'should' of 2.1 and adds no obligation of its own, so it earns no id; rfc/short/rfc6286.md carries all five observations under Remarks. |
| `4` | Security Considerations | 0 | walked | Security Considerations. Two sentences: this extension introduces no new security considerations, and BGP's are discussed in RFC 4271. No countermeasure is directed at a speaker. |
| `5` | Acknowledgments | 0 | skipped (acknowledgements) | Acknowledgments. |
| `6` | Normative References: RFC 4271 and RFC 4893 | 0 | skipped (references) | Normative References: RFC 4271 and RFC 4893. |

### Excluded sentences

| Site | Excluded kind | Reason | Quote |
|---|---|---|---|
| `front:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Abstract, non-normative by RFC convention and word-for-word the second paragraph of section 1. Its lowercase 'required' is inside a report of the document's own editorial effect on RFC 4271 -- it 'relaxes the "uniqueness" requirement so that only Autonomous-System-wide (AS-wide) uniqueness of the BGP Identifiers is required' -- which states what scope the revised requirement has, not an obligation on a speaker. The obligation itself is the indented replacement text of section 2.1, invisible to both scans and declared as unsourced RFC6286-2.1-1 and RFC6286-2.1-2 there. | To accommodate situations where the current requirements for the BGP Identifier are not met, this document relaxes the definition of the BGP Identifier to be a 4-octet, unsigned, non-zero integer and relaxes the "uniqueness" requirement so that only Autonomous-System- wide (AS-wide) uniqueness of the BGP Identifiers is required. |
| `front:2` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | Boilerplate the extractor did not strip: the IETF Trust Legal Provisions paragraph of the Copyright Notice. Its lowercase 'must' binds a party extracting Code Components from the document text, and directs no protocol behavior at a BGP speaker. RFC 6286 contains no code component: it revises three prose paragraphs of RFC 4271 and defines no syntax. | Code Components extracted from this document must include Simplified BSD License text as described in Section 4.e of the Trust Legal Provisions and are provided without warranty as described in the Simplified BSD License. |
| `1:1` | `not-a-requirement` (never bound Ze): the sentence states a fact or describes another document, and directs no implementation | The Introduction's second paragraph, the same report as front:1 with the IPv6-only network named as the motivating case. It says what this document does to RFC 4271's text, so its lowercase 'required' describes the new scope of the uniqueness requirement rather than imposing one. Classified not-a-requirement rather than duplicate-of because a duplicate must name an id another site MAPS, and no site of this document maps anything: every obligation is stated in the indicative and is declared unsourced on 2.1, 2.2 and 2.3. | To accommodate situations where the current requirements for the BGP Identifier are not met (such as in the case of an IPv6-only network), this document relaxes the definition of the BGP Identifier to be a 4-octet, unsigned, non-zero integer and relaxes the "uniqueness" requirement so that only AS-wide uniqueness of the BGP Identifiers is required. |

## Superseded

No document obsoletes RFC 6286, so its obligations are stated where they were written.
