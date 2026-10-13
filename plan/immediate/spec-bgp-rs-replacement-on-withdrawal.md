# Spec: Route Server Sends the Replacement Route on Withdrawal

<!-- DESIGN-TIME template: everything that must exist BEFORE code is written.
     The closure half (Implementation Summary, Audit, Goal Validation, Review
     Gate, Pre-Commit Verification, Mistake Log) lives in
     plan/TEMPLATE-CLOSURE.md and is APPENDED by /ze-close at step 1.
     Do not copy it in advance: sections copied 300 lines ahead of their use
     reach closure untouched, the ones created when needed get filled. -->

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Handoff | verify |
| Updated | 2026-10-13 |

<!-- Handoff: `verify` splits the work over two sessions -- the implementation session commits and stops at Status `verification`, a later session reviews that commit and closes, on Opus when the model is an Anthropic one. `-` closes in the same session. -->

<!-- Scope drives which optional blocks below apply. Say which one this is, so
     an absent section reads as "inapplicable" rather than "skipped".
     The file's DIRECTORY carries the release bucket: plan/immediate/ for a defect
     an operator meets, plan/pre-release/ for work the release cannot go out
     without, plan/ for everything else (plan/README.md). -->

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Record and resolve one RFC 4271 Section 9.2 defect in the route-server relay
(`bgp-rs`). When two sources announce the same prefix and one of them
withdraws it, a route-server client that did not negotiate ADD-PATH must be
sent the remaining source's route as its new best path. Today the relay
forwards the withdrawal and nothing else, so the client loses a prefix that is
still reachable.

Found by spec-rfc-verdict-test-fix-pass (child spec-rfc-verdict-fix-bgp,
continuation 19, 2026-10-01). Main-thread ruling R57(b) in
`plan/handover/rfc-verdict-test-fix-pass/RULINGS.md` homes it here because the
fix needs a per-source path store in the relay, which is larger than a verdict
fix.

## Evidence

| Item | Value |
|------|-------|
| Failing probe | `internal/component/bgp/plugins/rs/rfc4271_replacement_red_test.go`, `TestRouteServerWithdrawalKeepsTheReplacementRoute` (untracked, red on purpose) |
| Shape | sources 10.0.0.1 (A) and 10.0.0.2 (B) announce 10.9.0.0/24; A withdraws; client 10.0.0.3 (no ADD-PATH) receives the forwarded A announce, B announce and A withdrawal, and no re-announcement of B |
| Producer | the route-server forward path in `internal/component/bgp/plugins/rs/` (forward cache keyed per message, no per-source path store) |
| Requirement | RFC4271-9.2-7 (rfc/short/rfc4271.md), weak; Blocked-by row in plan/pre-release/spec-rfc-verdict-fix-bgp.md |

The RFC sentences (RFC 4271 Section 9.2 and RFC 7947 Section 2.3.2 on per-client
best path) must be read from `rfc/full/` and quoted at design time; this
skeleton quotes none of them and claims none.

### Design-time re-check (2026-10-13)

| Item | Finding at HEAD `0ed8cfb2c6` |
|------|------------------------------|
| Other session on this spec | None. `ls -t tmp/session/*/state/` holds no state file for this stem; `git log` since 2026-10-01 has no commit naming it |
| Red probe | ABSENT. `rfc4271_replacement_red_test.go` exists nowhere in the tree and never reached history (`git log --all` over the path is empty). Ruling R57(b) said it "moves to that spec (never committed here)"; it was lost with the untracked tree. Phase 1 recreates it from the Evidence Shape row |
| Defect still open | Yes, for one ordering. Commit `b880c5a9a7` (2026-10-07) added final-writer ownership: `adjOutWrite.allow` (`internal/component/bgp/reactor/session_ownership.go`) drops a forwarded withdrawal whose source is not the destination's current owner. So A announce, B announce, A withdraw now leaves the client with B (B owns it; read at the producer, not run, Phase 1 runs it). B announce, A announce, A withdraw still sends the bare withdrawal: A is the owner, the writer admits it, and no rail sends B. The route-server guide says so itself: "This is not best-path selection" (`docs/guide/route-reflection.md`, Forward-All Model) |
| Replacement machinery that exists | Only for source DEPARTURE. `handleStateDown` (`rs/server_handlers.go`) sends each departed source's NLRIs through `recoverRouteSel` with `recovery-source` meta; the engine's `recoverNLRIBatch` / `relayRecovery` (`reactor/relay_recovery.go`) drains publication, drains RIB application, asks `RIBManager.recoveryRoutes` (`plugins/rib/rib_recovery.go`) for a survivor, and writes the replacement or the withdrawal under the destination's session/sequence fence. A per-prefix withdrawal from a source that stays up never enters this rail |

### RFC text read (rfc/full/)

| RFC | Section | Whole sentence quoted |
|-----|---------|-----------------------|
| 4271 | 9.2 | "As part of Phase 3 of the route selection process, the BGP speaker has updated its Adj-RIBs-Out.  All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message." |
| 4271 | 3.1 | "Changing the attribute(s) of a route is accomplished by advertising a replacement route.  The replacement route carries new (changed) attributes and has the same address prefix as the original route." (method b of the three withdrawal methods: "a replacement route with the same NLRI can be advertised") |
| 7947 | 2.1 | "The route server SHOULD perform one or more BGP Decision Processes to select routes for subsequent advertisement to its clients, taking into account possible configuration to provide multiple Network Layer Reachability Information (NLRI) paths to a particular client as described in Section 2.3.2.2 or multiple Loc-RIBs as described in Section 2.3.2.1." |
| 7947 | 2.3 | "Neither Section 2.3 nor its subsections form part of the normative specification of this document; they are included for information purposes only." |
| 7947 | 2.3.2.2.2 | "As [RFC4271] specifies that a BGP listener must implement an implicit withdraw when it receives an UPDATE message for a prefix that already exists in its Adj-RIB-In, this approach requires explicit support for the feature both on the route server and on its clients." |

Reading: the skeleton's pointer to "RFC 7947 Section 2.3.2 on per-client best
path" carries no obligation, because Section 2.3 declares itself informative.
The binding text is RFC 4271 Section 9.2 (RFC4271-9.2-7), and RFC 7947 Section
2.1's SHOULD (row RFC7947-x-5, untagged, no audit verdict at HEAD). The
withdrawn route is "newly unfeasible", and a retained route from another source
IS a replacement route, so the bare withdrawal is owed only when none exists.

### Audit verdicts at HEAD

| Requirement | Verdict | Carriers | Bearing on this spec |
|-------------|---------|----------|----------------------|
| RFC4271-9.2-7 | weak (`rfc/audit/rfc4271.json`) | `TestRIBBestChangeWithdraw` (+), `TestRIBBestChangeNoPublishSameBest` (-), both `plugins/rib/rfc4271_rib_bestchange_test.go`; no discrimination record in `rfc/discrimination/rfc4271.json` | Note: "no UPDATE to a peer is asserted". Both carriers assert a RIB event, never a wire UPDATE. AC-3 adds wire-level carriers |
| RFC7947-x-1, x-2, x-3, x-4, 2.2-1, 2.2.2.2-1 | recorded in `rfc/audit/rfc7947.json` | transparency and per-client policy tests | Unaffected: a replacement carries the survivor's own received attributes (`recoveryCandidate`), so transparency holds by construction; AC-6 pins it |
| RFC7947-x-5 | no verdict, no tag | none | Not claimed by this spec (see Known Limitations and Q-3). Forward-all is not "one or more Decision Processes" |

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/guide/route-reflection.md` - Forward-All Model and source-departure sections, the operator-facing statement of what the route server sends
  → Constraint: the page states "The latest eligible advertisement still wins at a destination without ADD-PATH" and "This is not best-path selection". Steady-state announcements keep forward-all; only the owner-withdrawal case changes, and that paragraph is edited in the same phase as the code
  → Decision: a departing source already gets "an eligible surviving best route replaces the departed source with its own attributes; a route without a usable replacement is withdrawn". A per-prefix owner withdrawal reuses that semantics, so the operator learns one rule for both
- [ ] `docs/architecture/plugin/rib-storage-design.md` - "Source-DOWN replacement and sent ordering", the recovery rail this spec reuses
  → Constraint: "There is no timer retry and no second route inventory." The replacement MUST come from the selecting RIB (`bgp-rib`, a hard dependency of `bgp-rs` in `rs/register.go`), never from a new per-source store in the RS plugin. The skeleton's "per-source path store in the relay" is the rejected design
  → Constraint: no IPC or RIB lookup under a destination `writeMu`; publication drain then `DrainEventsApplied` then lookup; the destination worker re-checks session, causal send sequence, source generations and publication cuts under `writeMu` and retries on change
  → Constraint: a hard projection, lookup or writer failure retires the destination session through AutomaticStop/Cease, never leaves a known-invalid advertisement installed
- [ ] `docs/architecture/testing/interop.md` - BGP suite, scenario structure, `opExec` and session/route checker operations, the revert-to-red walk
  → Constraint: a new scenario is a named directory under `test/interop/scenarios/` plus a typed checker registered in `internal/le/interoplab/bgp/check_special.go` and listed in `bgp_test.go`; assertions never live in the scenario directory

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - row RFC4271-9.2-7 (verbatim span of Section 9.2)
  → Constraint: tagged claims MUST state only what each test asserts on the wire; the positive carrier asserts a replacement UPDATE and no withdrawal, the negative asserts a withdrawal when no replacement exists
- [ ] `rfc/short/rfc7947.md` - route-server summary, Forward-All and Path hiding rows
  → Constraint: RFC scope is frozen (owner, 2026-10-08): no new row is enrolled. RFC7947-x-5 stays untagged; its Support text must not be upgraded by this spec
  → Decision: the summary's "Support remaining" may gain one sentence stating that owner withdrawals now carry the RIB-elected replacement, with a source anchor (Documentation row 9)

**Key insights:** (minimal context to resume after compaction)
- Defect open at HEAD only when the withdrawing source is the destination's CURRENT owner (B, A, A-withdraw). Ownership fence `b880c5a9a7` already covers A, B, A-withdraw.
- The survivor election and the fenced writer already exist for source DEPARTURE (`recoverRouteSel` -> `recoverNLRIBatch` -> `request bgp rib recovery`). The fix is to route owner withdrawals through that rail in a "withdrawal" mode, not to build a store.
- Departure mode excludes EVERY candidate of the source (`candidate.PeerIP == request.Source`); withdrawal mode must exclude only the withdrawn path, which the RIB has already removed after `DrainEventsApplied`.
- Two forward rails reach a destination writer: plugin cached forwarding (`reactor_api_forward.go`, shared with bgp-rr) and the reactor RS fast path (`forward_rs.go`, RS only). Both set authority `adjOutForwarded`.
- The original red probe is lost and must be recreated.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/plugins/rs/server_withdrawal.go` (190L) - `processForward`: extracts families and NLRI records, forwards the WHOLE UPDATE (announcements and withdrawals alike) through `batchForwardUpdate`, or only releases the cache when `ReactorForwarded` is set, then updates the per-source `withdrawals` map used at peer-down
- [ ] `internal/component/bgp/plugins/rs/server.go` (922L) - `routeServer.withdrawals` is a key-only inventory (sourcePeer -> withdrawalKey -> withdrawalEntry) for peer-down; it holds no attributes, so it cannot supply a replacement. `recoverRouteSel` sends a command with `recovery-source` and `recovery-cut` meta through `UpdateRouteWithMeta`
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` (790L) - `handleStateDown` batches the departed source's NLRIs as `update text|hex nlri <fam> del ...` and calls `recoverRouteSel(ExcludeAddr(source), ..., cut)`
- [ ] `internal/component/bgp/reactor/session_ownership.go` (~600L) - `adjOutWrite.allow`: under `adjOutForwarded`, a withdrawal is admitted only when the destination's recorded owner equals the forwarded owner (`adjOutSameOwner`); an absent entry admits only a synthesized withdrawal. No replacement is considered
- [ ] `internal/component/bgp/reactor/relay_recovery.go` (~300L) - `recoverNLRIBatch` per destination: snapshot session and send sequence, capture source receipts, drain publication, `DrainEventsApplied`, then `relayRecovery` asks `ribevents.RecoveryProvider()` in process or forwards `request bgp rib recovery`; retries when the fence moved
- [ ] `internal/component/bgp/plugins/rib/rib_recovery.go` (301L) - `recoveryRoutes`: ADD-PATH destination -> withdraw only the source's sent identifiers at or below the cut; otherwise `recoveryRouteLocked` gathers candidates, skips the source and down peers, runs `SelectBest`, returns the survivor with its own attributes and next hop, or a withdrawal only if the sent owner is the source
- [ ] `internal/component/bgp/reactor/forward_rs.go` (853L) - `reactorForwardRS`: the RS fast path writes the UPDATE (withdrawals included) to eligible destinations before the plugin sees it, with authority `adjOutForwarded`
- [ ] `internal/core/bgp/ribevents/recovery.go` - `RecoveryRequest{Source, Destination, Family, NLRIs, AddPath, PrefixOnly, SentAddPath, Cut}`; no field says why recovery runs

**Behavior to preserve:** (unless the user explicitly said to change it)
- Steady-state announcements stay forward-all: the latest eligible advertisement wins at a non-ADD-PATH destination; no Decision Process runs on an announcement.
- ADD-PATH destinations: an owner withdrawal still withdraws exactly the advertised identifier; surviving paths are already present there.
- A withdrawal from a non-owner is still dropped at the writer (`b880c5a9a7`), and a synthesized policy withdrawal keeps its permission.
- Source-departure recovery semantics and its tests (`rib_recovery_test.go`, `relay_recovery_test.go`, `forward_publication_ownership_test.go`) unchanged.
- Peer-down `withdrawals` inventory maintenance in `processForward`.
- RS transparency: AS_PATH, NEXT_HOP, MED of the replacement are the survivor's received values (RFC7947-x-1/x-2/x-3 carriers stay green).
- bgp-rr forwarding behaviour (unless Q-2 widens scope).

**Behavior to change:** (only what the user asked for)
- A withdrawal from the destination's current owner, sent to a non-ADD-PATH destination, becomes the RIB-elected surviving route (an UPDATE carrying it) when one exists, and stays a withdrawal only when none exists.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Wire: an UPDATE from source peer A whose Withdrawn Routes field or MP_UNREACH_NLRI names a prefix A previously announced, received on an Established session.
- Format at entry: raw UPDATE bytes held in the reactor message cache, delivered to bgp-rs as a structured `update-received` event (`item.msg`) or text in fork mode.

### Transformation Path
1. Reactor receive: the UPDATE is accepted, cached, and either written by the RS fast path (`reactorForwardRS`) or handed to bgp-rs.
2. bgp-rs worker (`processForward`): forwards the cached UPDATE (`batchForwardUpdate` -> `ForwardCached`) to selected targets.
3. Destination forward worker -> session writer: `adjOutWrite.allow` per NLRI. NEW: for a forwarded withdrawal whose recorded owner is this source, on a destination that did not negotiate ADD-PATH send for the family, the writer does not emit the withdrawal; it records the NLRI as owed to recovery for this destination.
4. NEW: after the write releases `writeMu`, the owed NLRIs (per destination, per family, one batch per UPDATE) run through the existing `recoverNLRIBatch` path with a withdrawal-mode request and the withdrawal's message ID as cut.
5. bgp-rib `recoveryRoutes` in withdrawal mode: after the applied-event drain the withdrawn path is already gone from the candidates; elect over the rest without excluding the source's other paths; answer the survivor or a withdrawal.
6. Destination writer: the replacement passes ordinary egress policy and is written with the survivor as the new owner; a denied replacement or no survivor writes the withdrawal.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor -> bgp-rs | structured `update-received` event (DirectBridge) or text event | Yes, read `processForward` |
| bgp-rs -> Reactor | `ForwardCached(ids, destinations)` | Yes, read `batchForwardUpdate` |
| Reactor -> bgp-rib | in-process `ribevents.RecoveryProvider().Lookup`, else hidden command `request bgp rib recovery` | Yes, read `relayRecovery` |
| bgp-rib -> Reactor | `[]ribevents.RecoveryRoute` (owned bytes) | Yes, read `recoveryCandidate` |

### Integration Points
- `adjOutWrite.allow` / `adjOutWrite.record` (`reactor/session_ownership.go`) - the single decision point every forward rail shares; gains the "owed to recovery" outcome.
- `recoverNLRIBatch` / `relayRecovery` (`reactor/relay_recovery.go`) - reused per destination with a withdrawal-mode request; no second copy.
- `ribevents.RecoveryRequest` - gains a field naming the mode (departure vs withdrawal); `RIBManager.recoveryRouteLocked` reads it to decide whether the source's other candidates stay eligible.
- `rs/server_withdrawal.go` - unchanged unless the owner picks Approach C.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | Replacement selected by bgp-rib, exported through the ordinary forward rail and egress policy, as departure recovery does |
| No unintended coupling (components stay isolated) | Yes (design) | Reactor reaches the RIB through the existing `ribevents` provider or registered command, never by import |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | Reuses `recoverNLRIBatch`, `recoveryRoutes`, `SelectBest`; no RS-side store |
| Zero-copy preserved where applicable (refs, not copies) | Yes (design) | The forwarded UPDATE's announced NLRIs keep the cached zero-copy write; only owner-withdrawn NLRIs take the cold recovery copy |
| Registration over hardcoding, outbound | Yes (design) | No new command, family or plugin; the recovery provider is already published by bgp-rib via `ribevents.PublishRecovery` |
| Registration over hardcoding, inbound | Yes (design) | No switch learns a name; searched `relay_recovery.go`, `rib_recovery.go`, `session_ownership.go` for mode lists: none exists, the new mode is a field on the existing request |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | After `DrainEventsApplied`, bgp-rib no longer holds A's withdrawn path, so withdrawal-mode election need exclude nothing | `rib-storage-design.md` "this receipt proves successful application"; `recoveryRouteLocked` gathers from live peer RIBs | A stale candidate re-elects A's dead route and the client keeps a withdrawn path | Unit test: withdrawal-mode lookup after the RIB removed A's path returns B; reactor test drives the drain | unvalidated |
| A-2 | Every forward rail to an RS destination passes `adjOutWrite.allow` with authority `adjOutForwarded` | `reactorForwardRSSection` (`forward_rs.go`) and `forwardUpdateSection` (`reactor_api_forward.go`) both set it; guide: "All forwarding paths share the destination session's final writer ownership check" | A rail that bypasses the writer still emits the bare withdrawal | Wiring tests on both rails (cached and fast path) | unvalidated |
| A-3 | The survivor the RIB elects (`SelectBest`) is an acceptable replacement under forward-all, rather than "the latest remaining advertisement" | Departure recovery already uses `SelectBest` and the guide documents it | Owner may want latest-remaining semantics, which the RIB does not track | Owner answer Q-1 | unvalidated |
| A-4 | `forwardUpdateSection` serves every plugin that forwards a cached UPDATE (`ForwardUpdate` takes a `pluginName`), so a writer change gated on authority alone reaches non-RS forwarders | `reactorAPIAdapter.ForwardUpdate` signature | Silently changes another forwarder's behaviour | Gate on the RS forwarding role carried with the forward item; a test that a non-RS forward keeps today's withdrawal; or Q-2 widens scope | unvalidated |
| A-5 | `bgp-rib` is always loaded with `bgp-rs` | `rs/register.go` `Dependencies` names `bgp-rib` | Lookup has no provider; recovery returns "requires the BGP RIB command owner" and the destination is retired | Existing dependency resolution; a test with the provider absent asserts the error path, not silence | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Hot-path cost: every owner withdrawal to a non-ADD-PATH destination now pays a publication drain, a RIB applied-event drain and a lookup. For a single-source prefix (the common IXP case) the lookup finds nothing and answers a withdrawal anyway | `./le perf` withdrawal-heavy run regresses; RS backpressure `.ci` slows | Batch per destination per UPDATE (recovery takes an NLRI batch); in-process provider; owner decides acceptable cost (Q-1). Perf numbers are measured on the owner's Mac, not here |
| R-2 | Head-of-line blocking: running recovery on the destination's forward worker blocks that destination's queue while bgp-rib applies events | Destination forward queue depth grows during withdrawal storms | Recovery runs after `writeMu` release; the existing fence retries rather than holding locks; measure with the backpressure `.ci` |
| R-3 | Reordering: a later forward to the same destination for the same prefix (C announces) is written before the deferred recovery | Client ends on B instead of C | Existing fence: sequence moved -> retry; `entry.SourceMessageID > Cut` -> skip, so the newer owner C is never overwritten. Unit test for this interleaving |
| R-4 | Replacement denied by egress policy for this destination | Client keeps A's dead route | Existing departure behaviour: "a policy-denied replacement withdraws the obsolete advertised identity"; AC-5 asserts it |
| R-5 | Recovery hard failure retires the destination session (Cease) for one withdrawal | Unexpected Cease on a client during withdrawals | Same fail-closed policy as departure; documented; logged with the cause. Owner informed (Q-1) |
| R-6 | Withdrawal-mode request reaches the departure branch and excludes the source's other received paths (A sent two ADD-PATH paths, withdraws one) | The client loses a prefix A still announces | Explicit mode field; unit test with A holding two received paths, withdrawing one |
| R-7 | Interop scenario passes vacuously because BIRD already holds B (A, B order) | Red phase does not go red on revert | Checker asserts BIRD's path is A's before the withdrawal; revert-to-red walk recorded |
| R-8 | RFC-tagged claims on new carriers wider than their assertions | `./le rfc check` or the judge refuses | TDD plan fixes the claim text; discrimination records via `./le rfc discriminate-record` |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | RS clients without ADD-PATH: a wrong replacement installs a route the source no longer has (blackhole), or a missed withdrawal leaves a stale route; a recovery failure resets the client session |
| How is it reverted? | Single commit revert; no config or wire format change |
| Who else touches this path? | `spec-rfc-verdict-fix-bgp` (Blocked-by row for 9.2-7), `spec-rfc7947-adj-rib-in-accepts-filtered-updates` (RS ingress), the ownership work behind `b880c5a9a7`; other cached forwarders share `forwardUpdateSection` |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Wire UPDATE withdrawing A's prefix, RS cached rail (`processForward` -> `ForwardCached`) | → | `adjOutWrite.allow` owed-to-recovery outcome -> `recoverNLRIBatch` (withdrawal mode) -> `RIBManager.recoveryRoutes` | `TestRSOwnerWithdrawalRecoversOnCachedRail` (`internal/component/bgp/reactor/rs_withdrawal_replacement_test.go`) |
| Same UPDATE on the RS reactor fast path (`reactorForwardRS`) | → | same writer outcome and recovery | `TestRSOwnerWithdrawalRecoversOnFastPath` (same file) |
| Operator config: bgp-rs with three peers, one client without ADD-PATH | → | the whole chain from wire to wire | `test/plugin/bgp-rs-withdrawal-replacement.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Sources A and B announce one prefix in the order B then A; A withdraws; client C did not negotiate ADD-PATH send for the family | C's wire receives one UPDATE whose NLRI is the prefix and whose path attributes are B's received attributes (AS_PATH, NEXT_HOP, MED unchanged); C's wire carries no Withdrawn Routes or MP_UNREACH entry for the prefix after A's withdrawal. Proven on both the cached rail and the fast path |
| AC-2 | A is the only source; A withdraws | C's wire receives the withdrawal of the prefix, exactly once |
| AC-3 | RFC4271-9.2-7 | New wire-level carriers tagged in both polarities with the claim text fixed in the TDD plan, each with a discrimination record written by `./le rfc discriminate-record`; the verdict is re-judged; the 9.2-7 row is removed from the Blocked-by list of `plan/pre-release/spec-rfc-verdict-fix-bgp.md` |
| AC-4 | Interop | Scenario `bgp-route-server-replacement-frr`: FRR and GoBGP announce one prefix (GoBGP first, FRR second), BIRD (no ADD-PATH) holds FRR's path, FRR stops announcing; BIRD then holds GoBGP's path for the prefix and never loses it. Reverting the fix makes the checker fail at the "still present" assertion, recorded |
| AC-5 | B's route is denied by C's egress policy | C receives the withdrawal of the prefix (no stale A route, no B route) |
| AC-6 | A, B announce (B then A); C announces after A's withdrawal is deferred but before recovery writes | C's wire ends on C's route; recovery never overwrites it with B (fence: cut and send sequence) |
| AC-7 | Client C negotiated ADD-PATH send for the family | Unchanged: C receives the withdrawal of A's advertised path identifier only; B's path already present stays |
| AC-8 | A non-owner withdraws (A, B announce in that order; A withdraws) | Unchanged from `b880c5a9a7`: nothing is written to C for the prefix |
| AC-9 | A announced two received paths for the prefix over ADD-PATH and withdraws one; C has no ADD-PATH | C receives A's remaining path (or the RIB's elected survivor including it), never a withdrawal |
| AC-10 | Non-RS forwarder on the cached rail | Unchanged: an owner withdrawal is written as a withdrawal (unless the owner answers Q-2 otherwise) |
| AC-11 | Recovery lookup fails (no provider, lookup error) | Same as departure: the destination session is retired through AutomaticStop/Cease and the cause is logged; never a silent stale route |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | IXP member A withdraws a prefix another member B still announces; member C keeps reachability | wire -> reactor receive -> bgp-rs forward (or fast path) -> destination writer -> recovery -> bgp-rib election -> writer -> C's wire | `test/plugin/bgp-rs-withdrawal-replacement.ci`, interop `bgp-route-server-replacement-frr` |
| 2 | Last member withdraws the prefix; C loses it | same path, election answers no survivor -> withdrawal | `test/plugin/bgp-rs-withdrawal-replacement.ci` (second phase) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRouteServerWithdrawalKeepsTheReplacementRoute` | `internal/component/bgp/plugins/rs/rfc4271_replacement_red_test.go` (recreated red probe, then renamed to its final carrier name in `reactor/`) | Evidence shape: A 10.0.0.1, B 10.0.0.2, client 10.0.0.3, prefix 10.9.0.0/24; order B, A, A-withdraw (the order that is red at HEAD) | |
| `TestRecoveryWithdrawalModeKeepsSourceOtherPaths` | `internal/component/bgp/plugins/rib/rib_recovery_test.go` | Withdrawal mode does not exclude the source's remaining candidates (AC-9, R-6) | |
| `TestRecoveryWithdrawalModeAnswersWithdrawWhenNoSurvivor` | same | No candidate left -> withdrawal with the destination-framed identity (AC-2) | |
| `TestRecoveryDepartureModeUnchanged` | same | Departure mode still excludes every candidate of the source (preservation) | |
| `TestAdjOutOwnerWithdrawalOwedToRecovery` | `internal/component/bgp/reactor/session_ownership_test.go` | Owner withdrawal on a non-ADD-PATH RS destination is not written and is recorded as owed; non-owner withdrawal still dropped (AC-8); ADD-PATH destination still written (AC-7); non-RS forwarder still written (AC-10) | |
| `TestRSOwnerWithdrawalRecoversOnCachedRail` | `internal/component/bgp/reactor/rs_withdrawal_replacement_test.go` | AC-1 on the cached rail, real sessions over `net.Pipe`, wire bytes parsed | |
| `TestRSOwnerWithdrawalRecoversOnFastPath` | same | AC-1 on `reactorForwardRS` | |
| `TestRSOwnerWithdrawalNewerOwnerWins` | same | AC-6 interleaving | |
| `TestRSOwnerWithdrawalPolicyDeniedReplacement` | same | AC-5 | |
| `TestRSOwnerWithdrawalLookupFailureRetiresDestination` | same | AC-11 | |

### RFC-tagged tests that change (AC-3)

| Test | File | Change | Tag and claim (no wider than the assertion) |
|------|------|--------|---------------------------------------------|
| `TestRFC4271RouteServerOwnerWithdrawalSendsReplacement` | `internal/component/bgp/reactor/rfc4271_rs_replacement_test.go` (new) | add | `RFC4271-9.2-7 negative` -- when the source of the route last sent to a non-ADD-PATH route-server client withdraws it and another source's route for the same prefix is retained, the client's wire receives an UPDATE carrying that prefix with the retained route's attributes and no withdrawal of the prefix |
| `TestRFC4271RouteServerLastSourceWithdrawalIsSent` | same (new) | add | `RFC4271-9.2-7 positive` -- when the only source of a prefix withdraws it, the route-server client's wire receives an UPDATE withdrawing that prefix |
| `TestRIBBestChangeWithdraw` | `internal/component/bgp/plugins/rib/rfc4271_rib_bestchange_test.go` | claim text only, IF the owner approves Q-4 | today: "a newly unfeasible route with no replacement is advertised as a withdrawal"; the body asserts a `BestChangeWithdraw` event, not an UPDATE. Proposed: drop the 9.2-7 tag from this unit (the new wire carrier replaces it); the 9-3 and 9.1.2-2 tags stay |
| `TestRIBBestChangeNoPublishSameBest` | same | claim text only, IF Q-4 approved | today: "a route that is still feasible is not advertised as unfeasible"; body asserts no event. Proposed: drop the 9.2-7 tag; 9-3 and 9.2-8 stay |

No other RFC-tagged test changes. Tags on `forward_withdrawal_ownership_test.go`
(`TestRSWithdrawalOwnershipRails` and siblings) are read for regressions only;
AC-8 keeps them green. RFC7947 carriers are unchanged. The "newly installed
routes" half of the 9.2-7 sentence is not claimed by any new carrier.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| recovery cut vs sent entry message ID | uint64 | entry ID == cut (recovered) | N/A | entry ID == cut + 1 (skipped, newer owner kept) |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bgp-rs-withdrawal-replacement` | `test/plugin/bgp-rs-withdrawal-replacement.ci` | Three ze-peer connections (pattern of `bgp-rs-relay-aspath-transparency.ci`, conn=1..3), bgp-rs + bgp-rib. conn 2 (B) announces 10.9.0.0/24, conn 1 (A) announces it, conn 1 withdraws; conn 3 expects B's exact UPDATE hex and no withdrawal; then conn 2 withdraws and conn 3 expects the withdrawal hex | |
| `bgp-rs-withdrawal-replacement-fastpath` | `test/plugin/bgp-rs-withdrawal-replacement-fastpath.ci` | Same with the reactor fast path selected (as `bgp-rs-reactor-fastpath.ci` does) | |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-route-server-replacement-frr` | `test/interop/scenarios/bgp-route-server-replacement-frr/` (ze.conf, frr.conf, gobgp config, bird.conf) + checker `checkRouteServerReplacement` registered in `internal/le/interoplab/bgp/check_special.go`, listed in `bgp_test.go` | FRR and GoBGP as sources, BIRD as the non-ADD-PATH client | Checker: sessions up; GoBGP's path at BIRD; FRR announces (`opExec` vtysh `network`); BIRD's path for the prefix shows FRR's AS; FRR withdraws (`opExec` vtysh `no network`); BIRD still holds the prefix with GoBGP's AS_PATH and holds it for a stability window. Revert-to-red: rebuild the Ze image without the fix, the checker fails at the "still present" assertion; record the red run | |

### Goal Validation (planned evidence)

| Goal (Task section) | Evidence that will prove it |
|---------------------|-----------------------------|
| Client without ADD-PATH receives the remaining source's route after a withdrawal | `bgp-rs-withdrawal-replacement.ci` exact-hex assertion on conn 3, and interop `bgp-route-server-replacement-frr` against BIRD with the recorded red |
| Withdrawal still sent when nothing remains | `.ci` second phase exact withdrawal hex; `TestRFC4271RouteServerLastSourceWithdrawalIsSent` |
| RFC4271-9.2-7 verdict enforced | discrimination records in `rfc/discrimination/rfc4271.json` for both new carriers; re-judged audit verdict |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/bgp/reactor/session_ownership.go` - owner withdrawal on a non-ADD-PATH RS destination becomes "owed to recovery" instead of written (Design: `docs/architecture/update-building.md`)
- `internal/component/bgp/reactor/relay_recovery.go` - accept a withdrawal-mode batch from the writer's owed set; run after `writeMu` release on the existing destination worker (Design: `docs/architecture/plugin/rib-storage-design.md`)
- `internal/component/bgp/reactor/forward_pool.go` - hand the owed set from the write to the recovery run (`fwdBatchHandler`)
- `internal/core/bgp/ribevents/recovery.go` - `RecoveryRequest` gains the mode field
- `internal/component/bgp/plugins/rib/rib_recovery.go` - withdrawal mode keeps the source's other candidates; RFC 4271 Section 9.2 quote above the statement that answers survivor-or-withdrawal
- `internal/component/bgp/plugins/rib/rib_commands.go` (or wherever `request bgp rib recovery` parses its arguments) - carry the mode over the IPC form
- `internal/component/bgp/plugins/rib/rfc4271_rib_bestchange_test.go` - only if Q-4 approved
- `docs/guide/route-reflection.md` - Forward-All Model paragraph
- `docs/architecture/plugin/rib-storage-design.md` - recovery section gains the withdrawal mode
- `docs/architecture/update-building.md` - writer ownership outcome
- `rfc/short/rfc4271.md`, `rfc/short/rfc7947.md` Support remaining sentence, `rfc/audit/rfc4271.json` (re-judge), `rfc/discrimination/rfc4271.json`
- `plan/pre-release/spec-rfc-verdict-fix-bgp.md` - remove the 9.2-7 Blocked-by row
- `internal/le/interoplab/bgp/check_special.go`, `internal/le/interoplab/bgp/bgp_test.go` - register the new scenario

## Files to Create
- `internal/component/bgp/reactor/rs_withdrawal_replacement_test.go` - rail tests
- `internal/component/bgp/reactor/rfc4271_rs_replacement_test.go` - RFC-tagged carriers
- `internal/le/interoplab/bgp/check_route_server_replacement.go` - typed checker
- `test/interop/scenarios/bgp-route-server-replacement-frr/` - scenario inputs
- `test/plugin/bgp-rs-withdrawal-replacement.ci`, `test/plugin/bgp-rs-withdrawal-replacement-fastpath.ci` - functional tests

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No config leaf: the behaviour is the route server's default (no knob proposed; see Q-1) |
| YANG validation constraints | N-A | No leaf added |
| YANG custom validators | N-A | No leaf added |
| CLI commands/flags | No | No operator command; the hidden `request bgp rib recovery` only gains an argument |
| CLI grammar (keyword before value) | N-A | No operator-visible command changes |
| Editor autocomplete | N-A | No leaf or command added |
| Functional test for new RPC/API | Yes | `test/plugin/bgp-rs-withdrawal-replacement.ci` and its fast-path twin |
| Pipe completeness | N-A | No output-producing command |
| Env var registration | No | No env var |
| Doctor check for runtime dependencies | No | No new file, socket, port, module, binary or certificate; bgp-rib is already a declared dependency of bgp-rs |
| Prometheus counters/metrics | No | Recovery outcomes are logged as departure recovery logs them; no counter proposed |
| BGP family surface (new SAFI / capability / attribute) | N-A | No SAFI, capability or attribute; the families the recovery rail already supports are covered by it |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | Defect fix in an existing feature; `docs/features.md` unchanged |
| 2 | Config syntax changed? | No | No config |
| 3 | CLI command added/changed? | No | No operator command |
| 4 | API/RPC added/changed? | No | Hidden `request bgp rib recovery` gains one argument; grep `docs/architecture/api/commands.md` in phase 2 and edit if it is listed |
| 5 | Plugin added/changed? | Yes | `docs/guide/route-reflection.md` Forward-All Model paragraph (bgp-rs behaviour) |
| 6 | Has a user guide page? | Yes | `docs/guide/route-reflection.md` (same edit as row 5) |
| 7 | Wire format changed? | No | Same UPDATE format; only which UPDATE is sent changes |
| 8 | Plugin SDK/protocol changed? | No | `ribevents.RecoveryRequest` is internal, not SDK |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` (9.2-7 anchors), `rfc/short/rfc7947.md` Support remaining sentence, `docs/features/rfc-status.md` regenerated from Meta rows |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/interop.md` Scenario Inventory gains `bgp-route-server-replacement-frr` |
| 11 | Affects daemon comparison? | No | Grep `docs/comparison.md` for RS withdrawal claims in phase 4; none expected |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/rib-storage-design.md` (withdrawal mode), `docs/architecture/update-building.md` (writer outcome), `docs/architecture/bgp/structural-forwarding.md` |
| 13 | Route metadata keys added/changed? | No | The mode travels in `RecoveryRequest`; if the implementation adds a meta key beside `recovery-source` instead, `docs/architecture/meta/` is updated |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | Nothing registered changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors` (2026-10-13): 16 advisory mentions, none declared-blocking. Edited: `docs/architecture/bgp/structural-forwarding.md`; reviewed for the writer ownership statement: `docs/guide/bgp-resilience.md`. Unaffected (forward-pool queueing, not withdrawal choice): `docs/DESIGN.md`, `docs/architecture.md`, `docs/architecture/behavior/fsm-established.md`, `docs/architecture/buffer-architecture.md`, `docs/architecture/congestion-industry.md`, `docs/architecture/forward-congestion-pool.md`, `docs/features/configuration.md`, `docs/why-ze.md`. Unaffected (replay, not live withdrawal): `docs/architecture/bgp/replay-cursor.md`. Declared by `rs/server.go` and `rs/server_withdrawal.go` (`// Design:`), which this spec reads but does not change under Approach B: `docs/architecture/core-design.md`, unaffected because it describes the RS plugin's forward-all orchestration, which stays; it becomes affected only if the owner picks Approach C. Unaffected unless they document the hidden recovery command's arguments (re-check in phase 2): `docs/architecture/route-selection.md`, `docs/features/api-commands.md`, `docs/guide/graceful-restart.md`, `docs/guide/looking-glass.md`, `docs/guide/route-injection.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/route-reflection.md` RS config example unchanged; verify it against the new `.ci` config |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- recreate the red probe, write the failing rail tests
   - Tests: `TestRouteServerWithdrawalKeepsTheReplacementRoute` (recreated from the Evidence Shape, order B, A, A-withdraw), `TestRSOwnerWithdrawalRecoversOnCachedRail`, `TestRSOwnerWithdrawalRecoversOnFastPath`
   - Files: `reactor/rs_withdrawal_replacement_test.go`
   - Verify: all three red at HEAD with the client receiving a bare withdrawal (paste output); also run order A, B, A-withdraw and record that it is already green (the ownership fence)
2. **Phase: RIB withdrawal mode** -- `RecoveryRequest` mode field; `recoveryRouteLocked` keeps the source's other candidates in withdrawal mode; IPC argument in the hidden command
   - Tests: `TestRecoveryWithdrawalModeKeepsSourceOtherPaths`, `TestRecoveryWithdrawalModeAnswersWithdrawWhenNoSurvivor`, `TestRecoveryDepartureModeUnchanged`
   - Files: `ribevents/recovery.go`, `plugins/rib/rib_recovery.go`, `plugins/rib/rib_commands.go`, `docs/architecture/plugin/rib-storage-design.md`
   - Verify: tests fail, implement, pass; existing `rib_recovery_test.go` green
3. **Phase: Writer outcome and recovery run** -- `adjOutWrite.allow` returns owed-to-recovery for an RS owner withdrawal on a non-ADD-PATH destination; the destination worker runs the owed batch through `recoverNLRIBatch` after `writeMu` release
   - Tests: `TestAdjOutOwnerWithdrawalOwedToRecovery`, the three Phase 1 tests, `TestRSOwnerWithdrawalNewerOwnerWins`, `TestRSOwnerWithdrawalPolicyDeniedReplacement`, `TestRSOwnerWithdrawalLookupFailureRetiresDestination`; regression `forward_withdrawal_ownership_test.go`, `relay_recovery_test.go`, `forward_publication_ownership_test.go`
   - Files: `reactor/session_ownership.go`, `reactor/forward_pool.go`, `reactor/relay_recovery.go`, `docs/architecture/update-building.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/guide/route-reflection.md`
   - Verify: package tests under `./le job run` with `-race`
4. **Phase: Functional and RFC carriers** -- the two `.ci` files; the two RFC-tagged carriers with the exact claim text; discrimination records; Q-4 claim edits if approved
   - Tests: `bgp-rs-withdrawal-replacement`, `bgp-rs-withdrawal-replacement-fastpath`, `TestRFC4271RouteServerOwnerWithdrawalSendsReplacement`, `TestRFC4271RouteServerLastSourceWithdrawalIsSent`
   - Files: `test/plugin/*.ci`, `reactor/rfc4271_rs_replacement_test.go`, `rfc/short/rfc4271.md`, `rfc/short/rfc7947.md`, `rfc/discrimination/rfc4271.json`
   - Verify: `./le rfc discriminate-record` observes red for each tag when the writer outcome is reverted; `./le rfc check` clean
5. **Phase: Interop** -- scenario `bgp-route-server-replacement-frr` and its checker
   - Tests: `./le test integration interop bgp-route-server-replacement-frr`
   - Files: scenario directory, `internal/le/interoplab/bgp/check_route_server_replacement.go`, `check_special.go`, `bgp_test.go`, `docs/architecture/testing/interop.md`
   - Verify: green; revert-to-red walk with a rebuilt Ze image recorded; then remove the 9.2-7 row from `spec-rfc-verdict-fix-bgp` Blocked-by

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Both user stories have a passing `.ci` and the interop run |
| Correctness | A replacement carries the survivor's own attributes and next hop, never A's; the replacement's sent owner is the survivor so a later B withdrawal is admitted; withdrawal mode never excludes A's other paths |
| Correctness | The owed set is never dropped: every owed NLRI ends as a written replacement, a written withdrawal, a skip proven by a newer owner, or a retired session |
| Naming | The mode field names what it decides (departure vs withdrawal), not the caller |
| Data flow | No RIB lookup or IPC under `writeMu`; no new RS-side route store; reactor reaches the RIB only through `ribevents` or the registered command |
| Rule: goroutine-lifecycle | No new goroutine per withdrawal; recovery runs on the existing destination worker |
| Rule: performance | Announced NLRIs keep the zero-copy cached write; allocations confined to the cold recovery copy; withdrawal-heavy perf run handed to the owner (Mac) |
| Rule: rfc-compliance | RFC 4271 Section 9.2 quote above the statement choosing replacement or withdrawal; callers carry `// RFC 4271 Section 9.2` |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| Withdrawal mode in the RIB lookup | `go test` of `plugins/rib` under `./le job run`, the three Phase 2 tests |
| Writer outcome on both rails | `go test -race` of `reactor` under `./le job run`, Phase 1 and 3 tests |
| Functional proof | `./le test plugin bgp-rs-withdrawal-replacement` and the fast-path twin |
| RFC carriers | `./le rfc check`; `./le rfc discriminate id RFC4271-9.2-7` shows both polarities recorded |
| Interop | `./le test integration interop bgp-route-server-replacement-frr` green, red run recorded |
| Blocked-by cleared | grep `RFC4271-9.2-7` in `plan/pre-release/spec-rfc-verdict-fix-bgp.md` finds no Blocked-by row |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | Withdrawn NLRIs come from a peer; recovery re-parses them through the existing `recoveryIdentity` and refuses malformed framing with an error, never a silent skip |
| Resource exhaustion | A withdrawal storm from one source must not queue unbounded recovery work: the owed set is bounded by one UPDATE's NLRIs per destination and processed before the next item on that worker |
| Fail open | A lookup failure must never leave the owed withdrawal unsent with the stale route installed: AC-11 |
| Authority | Only the current owner's withdrawal can trigger a replacement; a non-owner still cannot change the client's route (AC-8) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. Route each lesson to its
     governing surface under ai/rules/planning.md. A problem-class journal row
     is appropriate only when no surface governs the lesson yet; closure alone
     requires no lesson artifact. -->
- 2026-10-13: the skeleton's premise ("the fix needs a per-source path store in the relay") is superseded. bgp-rib, a hard dependency of bgp-rs, already holds every source's retained paths and already answers survivor-or-withdrawal for source departure; the store exists, the rail does not reach it for a per-prefix withdrawal.
- 2026-10-13: half the defect was closed by `b880c5a9a7` without anyone recording it: a non-owner withdrawal is already dropped. A test must pin the order B, A, A-withdraw, because A, B, A-withdraw is green at HEAD and would prove nothing.
- 2026-10-13: an untracked red probe named in a ruling was lost. A probe a spec depends on belongs in the spec's Evidence as a shape (done here), not only as a path.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Recommended: Approach B, the final writer defers an RS owner withdrawal on a non-ADD-PATH destination to the existing recovery rail in a new withdrawal mode | A: per-source path store inside bgp-rs (the skeleton's wording). C: bgp-rs strips every withdrawal from forwarding and submits it through `recoverRouteSel`; the fast path stops forwarding withdrawals. D: bgp-rs subscribes to bgp-rib best-change events and re-announces the RIB best | B decides at the one point every rail shares and only for the owner case, so non-owner withdrawals keep costing nothing and the fast path needs no new exclusion. A duplicates bgp-rib's Adj-RIB-In and violates "no second route inventory". C puts a RIB round trip on EVERY withdrawal (owner or not) and needs a fast-path change. D races the forward-all writes (best-change and forward are unordered) and turns RS into best-path selection for announcements too |
| Survivor = the RIB's `SelectBest` over retained candidates | "Latest remaining advertisement" | The RIB records no advertisement order across sources; departure recovery already uses `SelectBest` and the guide documents it. Owner confirms in Q-1 |
| Withdrawal mode is a field on `RecoveryRequest`, not a second lookup function | Separate lookup for withdrawals | One producer, one election; the mode only toggles source exclusion |
| No config knob | `rs replacement on-withdrawal enable` leaf | A correct-by-default behaviour needs no switch; a knob would leave the RFC 4271 Section 9.2 defect reachable |

### Triple challenge

| Challenge | Answer |
|-----------|--------|
| Simplicity | Minimum change that reaches every rail: one writer outcome, one request field, one RIB branch. Nothing new is stored. Simpler C was rejected because it costs every withdrawal a RIB round trip and needs a fast-path change |
| Uniformity | Same pattern as source-departure recovery (same lookup, same fence, same fail-closed policy); the operator gets one rule for "a source stops offering a route" |
| Performance | Announcements and non-owner withdrawals unchanged (zero-copy). Owner withdrawals to non-ADD-PATH clients take the cold recovery path: copies of the survivor's NLRI and attributes (`recoveryCandidate`) and two drains. This is the R-1 cost the owner is asked to accept |

### Owner questions

| ID | Question | Recommended answer | If answered otherwise |
|----|----------|--------------------|-----------------------|
| Q-1 | Approach B costs a publication drain, a RIB applied-event drain and a lookup for every withdrawal the destination's current owner sends to a non-ADD-PATH client, including the common case where no other source exists and the answer is a plain withdrawal. A recovery hard failure resets that client session, as departure recovery does today. Is that cost and that fail-closed policy acceptable, and is the RIB-elected best the survivor you want? | Yes to all three | C or D changes Phases 3 to 5; "latest remaining" needs per-source ordering the RIB lacks (larger spec). **Answered by the owner, 2026-10-13: "RFC 4271 §9.2 requires the replacement: yes anything else in unacceptable". Approach B and its cost are accepted** |
| Q-2 | Other cached forwarders reach the same writer. bgp-rr may share the shape: nothing under `plugins/rr` uses `recovery-source` or `recoverRouteSel`, but its withdrawal behaviour was NOT traced at its producer in this design pass. Widen this spec to it (after tracing), or journal it? | Keep this spec to bgp-rs; journal row for bgp-rr | Widening adds RR ACs, RR `.ci` and RFC 4456 reading. **Reading taken from the owner's answer to Q-1 ("anything else in unacceptable"), not a separate answer: Phase 1 traces bgp-rr's withdrawal path first; if it sends a bare withdrawal while a replacement exists, the RR case is in this spec's scope (ACs, `.ci`, RFC 4456 reading). The owner may still narrow this** |
| Q-3 | RFC 7947 Section 2.1's SHOULD (row RFC7947-x-5, "SHOULD perform one or more BGP Decision Processes") stays untagged and unclaimed: forward-all remains. Confirm this spec claims nothing on x-5 | Confirm | Tagging x-5 needs a decision process on announcements, a different spec |
| Q-4 | The two existing 9.2-7 carriers in `rfc4271_rib_bestchange_test.go` claim an advertisement but assert a RIB event. Drop the 9.2-7 tag from them once the wire carriers land (their 9-3, 9.1.2-2 and 9.2-8 tags stay)? This changes RFC-tagged tests | Yes, drop | Keep both; their claim text must still be narrowed to "the RIB publishes a withdraw best-change event" to stop overclaiming, which also stales their audit entry |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- Steady-state announcements stay forward-all: when A re-announces with worse attributes while B's route is better, the client still gets A's. That is RFC7947-x-5 territory, not claimed here (Q-3).
- The "newly installed routes" half of the 9.2-7 sentence carries no new carrier.
- bgp-rr's same-shaped gap is out of scope pending Q-2.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
