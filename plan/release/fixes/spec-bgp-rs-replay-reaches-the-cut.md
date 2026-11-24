# Spec: bgp-rs-replay-reaches-the-cut

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Incorrect shipped BGP behaviour: a route-server client that comes up late can keep a route its source has withdrawn, or miss a route, until its next session. Nothing retracts or completes it |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-24 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| Replay catch-up failure / timeout (`bgp-adj-rib-in` still behind cut after 2-minute replay context timeout) | Keep waiting up to the 2-minute replay context timeout; if `bgp-adj-rib-in` still has not reached the cut (or delta fails unrecoverably), reset the client BGP session with a Cease NOTIFICATION so it reconnects and receives a complete table rather than leaving stale or missing routes. |

## Task

When a peer of the route server comes up, `bgp-rs` splits the UPDATE stream in
two at a cut, the newest MessageID it had seen at that instant. UPDATEs at or
below the cut reach the new peer through a replay of what `bgp-adj-rib-in` has
stored. UPDATEs above it reach the peer through the live forward rail, held
behind the replay. The two plugins consume events on separate queues, so the
store routinely trails `bgp-rs`.

The replay handles that lag wrongly in two ways. Both were read at the
producer on 2026-11-14 and neither has been reproduced on demand.

| # | Defect | What the peer ends up with |
|---|--------|----------------------------|
| 1 | The first replay walk runs before the wait for the store to reach the cut. A route announced and then withdrawn at or below the cut is relayed by that walk when the store has ingested the announcement only. The store then ingests the withdrawal and drops the route, and a replay carries stored routes only, so nothing retracts what was sent | A stale route, kept until the session ends |
| 2 | When the store has not reached the cut after `replayCatchUpBudget` (5 s), or a delta call fails, the replay logs a warning, sends End-of-RIB and reports the session ready. The live rail has already dropped every UPDATE at or below the cut for this peer | A missing route, or a stale one when the lost UPDATE was a withdrawal |

Defect 1 is the missing withdrawal that `test/plugin/rfc7606-relay-one-field.ci`
recorded on 2026-07-25 and that stayed open as "the destination arm" in the
known-failures record of that test (archived in
`plan/known-failures/RESOLVED.md`). It needs no give-up and no load beyond an
ordinary lag between the two plugins.

The goal: after a peer-up replay ends, the new peer holds exactly the routes
the route server's sources hold at the cut, then every later change in order.
A slow store delays that, and never changes it.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/bgp-plugins.md`, `ai/doors/bgp-engine.md`
  → Constraint: a plugin runs internal or forked, so the replay reaches the store only through a dispatched command, never a Go call
  → Constraint: "A barrier or a report is about timing, never about permission" (bgp-plugins door)
  → Constraint: a forwarder keeps per-source FIFO order and never drops a cached UPDATE
- [ ] `docs/architecture/plugin/plugin-system.md`, "Peer-up barrier" and "Session-ready reports"
  → Constraint: the engine does not hold a peer's End-of-RIB for a session-ready report (owner ruling, 2026-09-18). The report "orders the plugin's initial contribution; it does not assert successful route delivery"
  → Constraint: `bgp-rs` declares the barrier so that an UPDATE taken delivery of before the peer is `Up` lands at or below the cut and belongs to the replay. The page states the consequence this spec fixes: "its withdrawals never reach the peer"
- [ ] `docs/architecture/api/architecture.md`, the send half of an attach block and the owner-relayed peer-up replay
  → Constraint: the replay owner relays each page under its own name, because the destination's `send [ update ]` grant is the owner's. The store answers pages and relays nothing
- [ ] `docs/architecture/forward-congestion-pool.md`, the replay fence
  → Constraint: live forwards to a peer are held behind its peer-up replay and released by the session-ready report, so a live change never overtakes a replayed route
- [ ] `docs/architecture/core-design.md`, the route-server rails and "All output rails converge at the final session writer's ownership admission"
  → Constraint: both route-server rails ask the forwarder only for targets "subject to the peer-up replay cut"
  → Constraint: a withdrawal "is written only where the destination's adj-out holds a record for that route". A retraction this plugin synthesizes for a replayed route passes that admission like any other withdrawal
  → Constraint: a replay-only completion marker reaches the writer before the hold is released, "so a same-session withdrawal follows, rather than precedes, a retained page that still contains its route"
- [ ] `docs/architecture/plugin/rib-storage-design.md`, "Received source-policy provenance"
  → Constraint: a replayed route carries its stored `source-policy-epoch`. A change to what the store answers keeps that field
- [ ] `docs/architecture/bgp/replay-cursor.md`
  → Decision: that page covers the `bgp-rib` Adj-RIB-Out replay, a different replay. It is read to confirm that readiness "acknowledges that this producer's attempts finished, not successful delivery of every historical route"

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Section 9.2, read in `rfc/full/rfc4271.txt`
  → Constraint: "All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message." A route the source withdrew and the peer still holds violates it
- [ ] `rfc/short/rfc7947.md` - the route server role
  → Constraint: owed at the design gate, not yet read for this spec
- [ ] `rfc/short/rfc4724.md` - Section 2, the End-of-RIB marker
  → Constraint: the marker indicates "the completion of the initial routing update" (quoted in `sendEOR`). Owed at the design gate: read the section before deciding what the marker may follow

**Key insights:**
- The cut makes the two rails disjoint by MessageID. It says nothing about when the store's content is consistent with the cut. That second condition is what both defects miss.
- A replay carries announcements only. Any design that lets the replay send a route the store is about to delete needs a way to retract it, or must not send it.
- The store's ingest position is a high-water mark, not a count. Whether "position at or above the cut" proves that every UPDATE at or below the cut was ingested is assumption A-5, and it decides the design.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` - `handleStateUp` sets `Up`, captures `ForwardFrom` from `seenMsgID` as the cut and starts `replayForPeer`. `replayForPeer` calls `ribevents.RelayReplay` from index 0 first, relaying every page. Only then does it loop on `coversCut`, polling `deltaReplay` every `replayConvergenceDelay` until the store's ingest position reaches the cut or `replayCatchUpBudget` passes. On the budget, or on a delta error, it breaks out, drains, then `replayFlowSpecs`, `sendEOR` and `signalSessionReady`. Clock seams: `rs.now`, `rs.sleep`
- [ ] `internal/component/bgp/plugins/rs/server_forward.go` - `selectForwardTargets` and `fastPathOwedTarget` skip a peer that is not `Up`, and skip a peer for every UPDATE whose MessageID is at or below its `ForwardFrom`
- [ ] `internal/component/bgp/plugins/rs/server.go` - `replayConvergenceMax` 10, `replayConvergenceDelay` 20 ms, `replayCatchUpBudget` 5 s. Test seams `dispatchCommandHook` and `relayStoredRouteHook`
- [ ] `internal/core/bgp/ribevents/replay.go` - `RelayReplay` reads `ReplayRoutesCommand` pages and relays each before asking for the next. `ReplayPage.IngestedMsgID` is the store's ingest position, a pointer so that untracked stays distinct from 0
- [ ] `internal/component/bgp/plugins/adj_rib_in/replay_page.go` - `replayRoutesCommand` answers one page from what is stored now and does not wait for anything
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - `walkReplayRoutes` visits stored routes only, skipping FlowSpec, ineligible routes, routes an import policy denied, and routes whose MessageID is above the cut. `noteIngested` advances the position as a maximum: its comment says MessageIDs "arrive out of numeric order". `ingestPosition` reports it
- [ ] `internal/component/bgp/plugins/adj_rib_in/replay_cut.go` - the cut is presence plus value; 0 is a real cut
- [ ] `internal/component/bgp/plugins/rr/rr.go` - `replayForPeer` of the route reflector replays unbounded, with no cut and no catch-up wait. It is a different shape and is not changed here (Known Limitations)
- [ ] `internal/component/bgp/plugins/rs/replay_catchup_test.go` - `TestReplayProgressCoversCut` and `TestDeltaReplayDistinguishesAbsentFromZero` pin the predicate and the page decoding. Nothing drives `replayForPeer` through a lagging store

**Behavior to preserve:**
- The cut and its meaning: at or below is the replay's, above is the live rail's. No UPDATE reaches a peer by both rails
- A store that tracks no ingest position (`IngestedMsgID` absent) is never waited for
- The replay owner relays pages under its own name; the store relays nothing
- End-of-RIB timing as ruled on 2026-09-18: no new hold of the engine's marker
- A stale replay goroutine (the peer reconnected) sends nothing more
- The unknown-command fallback when `bgp-adj-rib-in` is not loaded

**Behavior to change:**
- The replay never relays a route from a store that does not yet cover the cut
- A store that does not reach the cut in the catch-up budget no longer ends in a peer reported complete with routes knowingly missing

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The peer-up cut | `rs/server_handlers.go` `handleStateUp`, `PeerState.ForwardFrom`; `adj_rib_in/replay_cut.go` `replayCut` | Splits the stream by MessageID between replay and live rail | Does not apply: kept as is. The defect is when the store is read, not where the cut falls |
| Ingest position | `adj_rib_in/rib.go` `noteIngested`, `ingestPosition`; `ribevents.ReplayPage.IngestedMsgID` | Tells the replay owner the highest MessageID the store has consumed | Extends: read before the first relay, where today it is read after it. Replaced by a stronger signal if A-5 breaks |
| Catch-up wait | `rs/server_handlers.go` `replayForPeer` phase 1, `replayProgress.coversCut` | Polls until the store covers the cut, after the first walk | Extends: the wait moves ahead of the first walk, so the walk reads a store consistent with the cut |
| Paged replay | `ribevents/replay.go` `RelayReplay`, `adj_rib_in/replay_page.go` `replayRoutesCommand` | Answers and relays the stored routes page by page | Extends: the owner needs the position without relaying a page. The page answer already carries it |
| Replay fence | `docs/architecture/forward-congestion-pool.md`; `rs` `signalSessionReady`, `endReplay` | Holds live forwards to the peer until the replay reports | Extends: it already orders live changes after the replay, which is what makes a consistent first walk sufficient |
| Give-up warning | `rs/server_handlers.go` `replayForPeer`, the `gave up waiting` line | Says the replay is incomplete, then completes the peer anyway | Replaces: the outcome after the budget is decided in Key Design Decisions |
| Doctor check for an incomplete replay | `plan/release/validation/spec-doctor-coverage-7-bgp-plugins.md`, AC-17, code `doctor-bgp-peer-up-replay-incomplete` | Planned, not implemented | Does not apply: that spec reports the state. This one removes the route loss. The code name is reused if this spec raises the condition first |
| Route reflector replay | `rr/rr.go` `replayForPeer` | Unbounded replay, no cut | Does not apply: no cut, so neither defect has this shape there |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A peer-up state event delivered to `bgp-rs` for a route-server client, while other clients have sent UPDATEs the store has not all ingested

### Transformation Path
1. `handleStateUp` marks the peer `Up`, captures the cut, shuts the replay gate, starts `replayForPeer`
2. `replayForPeer` asks the store for pages through the dispatcher and relays them with `RelayStoredRoute`
3. The engine rebuilds each stored route through the forward path under the replay's initial-update authority and writes it to the peer
4. `sendEOR` asks for the End-of-RIB; `signalSessionReady` lowers the replay fence, releasing the held live forwards

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `bgp-rs` to `bgp-adj-rib-in` | `request bgp adj-rib-in replay-routes` through the dispatcher, on both transports | Yes, read: `ribevents.RelayReplay`, `replayRoutesCommand` |
| `bgp-rs` to the engine | `RelayStoredRoute`, `plugin session ready` | Yes, read: `relayReplayRoutes`, `signalSessionReady` |

### Changed-Wire Census

**Search evidence:** searched `internal/` for `RelayReplay(`, `ReplayRoutesCommand`, `ingested-msg-id` and `replay-routes` on 2026-11-14. Positive control: the search finds `rs/server_handlers.go` `replayForPeer`, the known caller. The recommended design changes when the owner reads the page answer, not the answer's shape. If the design gate settles on a position-only request, or A-5 breaks and the answer needs a stronger signal, the rows below are the population to migrate.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `request bgp adj-rib-in replay-routes` answer (`ReplayPage` JSON) | writer | `adj_rib_in/replay_page.go` `replayRoutesCommand` | builds the page and sets `IngestedMsgID` when tracked | Preserve: no field added or removed |
| same | reader | `ribevents/replay.go` `RelayReplay`, called by `rs` `replayForPeer` and `deltaReplay` | decodes the page, relays its routes, returns the position | Preserve the shape; `rs` changes which calls relay |
| same | reader | `rr/rr.go` `replayForPeer` | two unbounded calls, ignores the position | Preserve: unchanged |
| same | reader | `reactor/policy_reload.go`, the `RelayReplay` call | unbounded, ignores the position | Preserve: unchanged |
| same | reader | a third-party replay owner outside the repository | the command is a published plugin RPC | Preserve: unchanged answer |

### Integration Points
- `handleStateUp` - unchanged entry; the goroutine it starts changes order
- `ribevents.RelayReplay` - the owner needs a way to learn the position without relaying; settled at the design gate

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The store is still reached only through the dispatched command |
| No unintended coupling (components stay isolated) | Yes | No import between `rs` and `adj_rib_in`; `ribevents` stays the shared core leaf |
| No duplicated functionality (extends existing, does not recreate) | Yes | Existing Mechanisms: the wait and the position exist and are reordered |
| Zero-copy preserved where applicable (refs, not copies) | Yes | The relay path is not changed |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command in the recommended design |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | The design introduces no name. If a doctor code is raised, it registers through the plugin's Stage 1 `DoctorChecks` |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Defect 1's sequence is reachable as read: the first walk can see a route whose withdrawal, at or below the cut, the store has not ingested | `replayForPeer` calls `RelayReplay` before the `coversCut` loop; `replayRoutesCommand` does not wait | The spec fixes a sequence that cannot happen | `TestReplayNeverRelaysBeforeStoreCoversCut`, red on today's code | unvalidated |
| A-2 | Once the store has ingested every UPDATE at or below the cut, a walk bounded at the cut plus the held live rail gives the peer a consistent view | `walkReplayRoutes` excludes MessageIDs above the cut; the replay fence holds live forwards until the report | A second ordering defect remains between a page and a live change | A unit test that deletes a route between two pages and checks the live withdrawal follows the replay; `rfc7606-relay-one-field.ci` under stress | unvalidated |
| A-3 | The store's ingest position can be read without relaying a page, through the existing page answer | `ReplayPage.IngestedMsgID` is set on every answer | A position-only request is needed, which changes a published RPC (census above) | Read `buildReplayPage` for a request that returns no route; decide at the design gate | unvalidated |
| A-4 | A legacy-ingest route (MessageID 0) is unaffected | `replayCut.excludes` never excludes MessageID 0, and a store with no tracking is never waited for | Those routes would need their own rule | Existing `TestReplayProgressCoversCut` "no signal" case stays green | unvalidated |
| A-5 | A position at or above the cut means the store has ingested every UPDATE at or below the cut | None proven. `noteIngested` keeps a maximum and its comment says MessageIDs "arrive out of numeric order". The cut is the maximum `bgp-rs` had seen, so the store can reach that number with a lower-numbered UPDATE still queued | The existing wait is not a sufficient condition today, for the missing-route case too, and moving it earlier does not close defect 1. The store then needs a signal that counts what it consumed (a delivery sequence shared by both plugins), which changes the page answer | Read how the reactor allocates MessageIDs and in what order one UPDATE reaches the two plugins' queues. This is the first research step, before the design is presented | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Waiting before the first walk adds peer-up latency for every route-server client | Peer-up time in the perf lab's route-server pass | The wait is the one that already runs, moved earlier; it normally ends in one or two 20 ms polls |
| R-2 | The live rail to the new peer stays held for as long as the replay waits, so a stuck store stalls that peer | The fence held past the budget | Decided in Key Design Decisions: the bound and what follows it |
| R-3 | The fix changes replay order in a way an existing `.ci` asserts | `forward-overflow-two-tier.ci`, `rfc7606-relay-one-field.ci`, `llgr-readvertise-multipeer.ci` red | Fix the code. A test edit goes through `ze-test-judge` |
| R-4 | A deterministic functional test needs the store to lag on demand, and no such seam exists | The `.ci` cannot be made to fail on today's code | The unit tests carry the proof through `dispatchCommandHook`; the `.ci` is a regression run under stress, stated as such |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A route-server client's initial table: routes missing, stale, or late. No effect on a peer already up |
| How is it reverted? | One commit revert; no config, no wire shape change in the recommended design |
| Who else touches this path? | `spec-bgp-one-off-defects` (AC-5 relay determinism, AC-38 relay pool dry), `spec-doctor-coverage-7-bgp-plugins` (AC-17), `spec-perf-rs-fast-path-regression-and-pass-labels` (route-server convergence figures) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer-up state event for a route-server client while the store trails the cut | → | `handleStateUp` → `replayForPeer` | `TestReplayNeverRelaysBeforeStoreCoversCut` in `internal/component/bgp/plugins/rs/replay_catchup_test.go` |
| The same, with a store that never reaches the cut | → | `replayForPeer` past `replayCatchUpBudget` | `TestReplayPastBudgetDoesNotReportComplete` in the same file |
| Two route-server clients, one late, a route announced then withdrawn | → | the daemon's `bgp-rs` and `bgp-adj-rib-in` | `test/plugin/rfc7606-relay-one-field.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A peer comes up with cut C. The store has ingested an announcement of P (MessageID below C) and not yet the withdrawal of P (MessageID at or below C) | The replay waits until the store has ingested every UPDATE at or below C, then relays the store's content, in which P is absent. The peer's table holds the other stored routes and not P | Routes still stored at or below C are relayed: they are the peer's initial table |
| AC-2 | The same peer-up, the store already level with C | The replay relays the stored routes at once, after one position read | The single position read: it is what proves the store is level |
| AC-3 | The store tracks no ingest position | The replay relays every stored route at once, as today | Every stored route: an untracked store is unbounded and has nothing to wait for |
| AC-4 | The store does not reach C within the catch-up budget | The outcome chosen in Key Design Decisions holds, and in every option the peer ends with the routes its sources hold | A delay of the peer's initial table: slowness is permitted, loss is not |
| AC-5 | A delta or position call to the store fails during the replay | The same outcome as AC-4, and the log line names the failed call and its cause | The peer's session may be reset if the design chooses it: a reset rebuilds a correct table |
| AC-6 | The peer reconnects while its replay waits | The newer replay owns the peer and delivers its table; the older one ends | Nothing from the older replay: its generation is stale |
| AC-7 | `rfc7606-relay-one-field.ci`, `forward-overflow-two-tier.ci` and `llgr-readvertise-multipeer.ci` under `./le test stress-repro` | Every iteration reports pass: the late client receives the withdrawal ahead of the announcement, all fifty prefixes in send order, and one copy of each UPDATE | The assertions of the three tests as they stand: they are what recorded the symptoms |
| AC-8 | `docs/architecture/plugin/plugin-system.md`, "Peer-up barrier" | The page states what the replay guarantees for an UPDATE at or below the cut, in place of "its withdrawals never reach the peer", and describes the replay order where the catch-up wait is documented | The rest of the section: the barrier itself is unchanged |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator's route-server client reconnects while another client is withdrawing routes | peer-up → cut → wait for the store → replay → End-of-RIB → live rail | `TestReplayNeverRelaysBeforeStoreCoversCut`, `rfc7606-relay-one-field.ci` |
| 2 | A client connects while `bgp-adj-rib-in` is far behind (a full-table burst, a slow forked plugin) | peer-up → wait past the budget → the AC-4 outcome | `TestReplayPastBudgetDoesNotReportComplete` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReplayNeverRelaysBeforeStoreCoversCut` | `internal/component/bgp/plugins/rs/replay_catchup_test.go` | AC-1, AC-2: a fake store answers P with a position below the cut on the first read and nothing with a position at the cut afterwards; the relay hook must never see P | |
| `TestReplayPastBudgetDoesNotReportComplete` | same | AC-4, AC-5: a fake clock moves past the budget with the store behind; the peer is not reported complete with routes missing | |
| `TestReplayUntrackedStoreRelaysAtOnce` | same | AC-3 | |
| `TestReplayStaleGenerationSendsNothing` | same | AC-6 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Ingest position against the cut | 0 to the cut and above | position equal to the cut is covered | position one below the cut is not covered | N/A |
| Cut | 0 upward | 0 is a real cut | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc7606-relay-one-field` | `test/plugin/rfc7606-relay-one-field.ci` | A late client receives the withdrawal that precedes an announcement | existing; regression under stress, not a deterministic proof (R-4) |
| `forward-overflow-two-tier` | `test/plugin/forward-overflow-two-tier.ci` | A late client receives every route, in order | existing |
| `llgr-readvertise-multipeer` | `test/plugin/llgr-readvertise-multipeer.ci` | No UPDATE reaches a client twice | existing |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-route-server-replacement-frr` | `test/interop/scenarios/` | FRR | A route-server client run by FRR still receives replacements and withdrawals after the replay order changes. Regression evidence: the scenario does not force the lag, which only the unit tests can | existing; run owed by the main thread |

## Files to Modify
- `internal/component/bgp/plugins/rs/server_handlers.go` - `replayForPeer`: position before the first relay, and the outcome past the budget
- `internal/component/bgp/plugins/rs/server.go` - the comment on `replayCatchUpBudget`, which describes the outcome this spec changes
- `internal/core/bgp/ribevents/replay.go` - only if A-3 breaks: a way to read the position without relaying
- `internal/component/bgp/plugins/adj_rib_in/rib.go`, `replay_page.go` - only if A-5 breaks: the signal the store reports
- `internal/component/bgp/plugins/rs/replay_catchup_test.go` - the four tests
- `docs/architecture/plugin/plugin-system.md` - AC-8
- `docs/architecture/api/architecture.md` - the owner-relayed peer-up replay, if it states the order

## Files to Create
- None in the recommended design

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new command in the recommended design; revisit if A-3 breaks |
| YANG validation constraints | N-A | No config |
| YANG custom validators | N-A | No config |
| CLI commands/flags | No | None |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No YANG |
| Functional test for new RPC/API | N-A | No new RPC |
| Pipe completeness | N-A | No command output |
| Env var registration | No | The budget stays a constant |
| Doctor check for runtime dependencies | No | No new runtime dependency |
| Doctor code per problem class | Yes | A replay that passes the budget is a problem an operator can hit. Its code is `doctor-bgp-peer-up-replay-incomplete`, planned by `spec-doctor-coverage-7-bgp-plugins` AC-17. Whichever spec lands first registers it; the other reuses it |
| Prometheus counters/metrics | No | None added |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | A defect fix |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | Yes only if A-3 or A-5 breaks: `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | Yes only if A-3 or A-5 breaks: `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md`, Section 9.2 row, if a test of this spec carries its tag |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/plugin-system.md`, "Peer-up barrier" (AC-8) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Derived at implementation | `./le spec citation anchors spec plan/release/fixes/spec-bgp-rs-replay-reaches-the-cut.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | - |

## Implementation Steps

Before phase 1, the design gate: settle A-5 by reading MessageID allocation and
delivery order, read FRR and BIRD as Key Design Decisions requires, and put the
design to Thomas. The spec moves to `ready` only then.

1. **Phase: Wiring (MANDATORY FIRST)** -- the two tests that drive `replayForPeer` through a lagging fake store
   - Tests: `TestReplayNeverRelaysBeforeStoreCoversCut`, `TestReplayPastBudgetDoesNotReportComplete`
   - Files: `replay_catchup_test.go`
   - Verify: both red on today's code, for the reason each names. The first red validates A-1
2. **Phase: position before the first relay** -- defect 1
   - Tests: `TestReplayNeverRelaysBeforeStoreCoversCut`, `TestReplayUntrackedStoreRelaysAtOnce`, `TestReplayStaleGenerationSendsNothing`
   - Files: `server_handlers.go`, and `ribevents/replay.go` if A-3 breaks
   - Verify: green; the three functional tests of AC-7 pass
3. **Phase: the outcome past the budget** -- defect 2, as decided
   - Tests: `TestReplayPastBudgetDoesNotReportComplete`
   - Files: `server_handlers.go`, `server.go`
   - Verify: green; the doctor code decision recorded
4. **Phase: pages and regression** -- AC-7 under stress, AC-8, the FRR scenario

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Both user stories have a working path |
| Correctness | No relay call can run while the store is known to be behind the cut. A position that is absent is never read as 0 |
| Correctness | The replay still ends in exactly one session-ready report per establishment, on every exit |
| Data flow | The store is reached only through the dispatcher, so the fix holds for a forked `bgp-adj-rib-in` |
| Operator errors and doctor codes | The warning past the budget names its code and the command that shows the store's position |
| Rule: `ai/rules/no-layering.md` | The first-walk-then-wait order is deleted, not kept as a fallback |
| Rule: `ai/rules/stale-comments.md` | The comments in `replayForPeer`, on `replayCatchUpBudget` and in `replay_page.go` that describe the old order are rewritten |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The four unit tests | `./le test unit pkg path ./internal/component/bgp/plugins/rs/ run TestReplay` |
| Regression under stress | `./le test stress-repro run suite plugin test rfc7606-relay-one-field any-failure`, and the same for the other two tests of AC-7 |
| FRR route-server scenario | `./le test integration interop` selecting `bgp-route-server-replacement-frr` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A peer that flaps while the store is behind must not accumulate waiting replay goroutines: the generation check ends the older one |
| Authorization that could fail open | A store that never answers must not leave the peer reported complete, and must not hold the live rail without bound |

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

- Two fixes of 2026-07-26 closed the duplicate UPDATE that `llgr-readvertise-multipeer.ci` recorded: `a588ac5277` (a cut of 0 read as no cut) and `e4ee32b034` (the replay index read as inclusive, so the boundary route was relayed on every delta pass). The known-failures record never credited them. A symptom that stops reproducing is traced to the commits between the last red and the first green before it is called unexplained.
- `81dd09dfa8` (2026-07-27) added the catch-up wait and put it after the first walk. It closed the missing-route case it was written for and left the stale-route case, because the first walk is the one that can relay a route the store is about to drop.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `suppression-decision-not-deterministic.md` | 2026-11-14 | bgp-rs `replayForPeer` walks the store before it covers the cut, so a withdrawn route is replayed |
| `transient-failure-treated-as-fatal.md` | 2026-11-14 | bgp-rs `replayForPeer` gives up after `replayCatchUpBudget` and reports the peer complete |

## Key Design Decisions

Recommended, not yet approved. The design gate presents these to Thomas.

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Defect 1: wait for the store to cover the cut, then walk | Walk first and retract afterwards: after the wait, compare what was relayed with the store and send a withdrawal for each route that is gone | Waiting first needs no record of what was relayed and no synthesized withdrawal, and reuses the wait that exists. Retracting keeps today's latency but adds a per-peer set of relayed keys and a second way for this plugin to originate withdrawals. Both depend on A-5 |
| Defect 2: past the budget, keep the peer's live rail held and keep waiting, bounded by the replay's existing two-minute context; if the store has still not reached the cut, reset the session with a Cease so it starts again | Proceed as today, with the doctor check as the only signal. Or wait without bound | A peer reported complete with routes knowingly missing is the silent wrong value `ai/rules/principles.md` forbids. An unbounded wait stalls one peer for ever on a stuck plugin. A reset costs that peer its session and rebuilds a correct table. This is the choice that is Thomas's: it trades a session reset against a table known to be wrong |
| The engine's End-of-RIB is not held | Hold the marker until the replay ends | Ruled out by the owner ruling of 2026-09-18 in `plugin-system.md`, "Session-ready reports" |
| The route reflector's replay is not changed | Give `bgp-rr` a cut and the same wait | It has no cut, so it cannot lose a route to one. Whether its unbounded replay can resurrect a withdrawn route is a separate question (Known Limitations) |

Owed before approval (`ai/META-DECISIONS.md`, "Goal and scope"): read how FRR
and BIRD build a new peer's initial table from a route server's view, and
quote the deciding function. Both are single-process and have no second store
to trail, so the expected finding is that they give no guidance on the
budget. That has to be read, not assumed.

## Known Limitations

- `bgp-rr` replays without a cut while its live rail also forwards. Whether a live withdrawal can be followed by a replayed announcement of the same route there was not examined. It is the open question of the 2026-09-24 row in `plan/journal/suppression-decision-not-deterministic.md`, owned by `spec-bgp-one-off-defects`.
- A deterministic `.ci` for the lag needs a way to make `bgp-adj-rib-in` trail on demand. None exists, and this spec does not add one: the unit tests carry the proof.

## RFC Documentation (Scope: protocol)

Add `// RFC 4271 Section 9.2: "All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message."` above the statement that keeps the replay from relaying before the store covers the cut.

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
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-rs-replay-reaches-the-cut.md` only, in the same `./le commit create` script
