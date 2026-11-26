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
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Owner decisions (2026-11-24)

| Topic | Decision |
|-------|----------|
| Replay catch-up failure / timeout (`bgp-adj-rib-in` still behind cut after 2-minute replay context timeout) | Keep waiting up to the 2-minute replay context timeout; if `bgp-adj-rib-in` still has not reached the cut (or delta fails unrecoverably), reset the client BGP session with a Cease NOTIFICATION so it reconnects and receives a complete table rather than leaving stale or missing routes. |

## Task

When a route-server client establishes, `bgp-rs` splits the UPDATE stream between the initial peer-up replay from `bgp-adj-rib-in` and the live forward rail (held behind the peer's replay fence). Both plugins consume events on independent delivery goroutines and queues (`internal/component/plugin/process/delivery.go`), so `bgp-adj-rib-in` routinely trails `bgp-rs`.

Three coupled defects remain in `bgp-rs`'s peer-up replay on HEAD:

| # | Defect | What the peer ends up with |
|---|--------|----------------------------|
| 1 | `replayForPeer` walks and relays `bgp-adj-rib-in` from index 0 *before* waiting for the store to catch up. A route announced and then withdrawn at or below the cut is relayed during that first walk when the store has ingested only the announcement. The store later ingests the withdrawal and drops the route, and a replay carries stored routes only, so nothing retracts what was sent | A stale route kept until the session ends (`test/plugin/rfc7606-relay-one-field.ci`, archived in `plan/known-failures/RESOLVED.md`) |
| 2 | The catch-up predicate and cut both rely on a single global high-water mark (`IngestedMsgID` vs `PeerState.ForwardFrom`). Reactor `MessageID`s (`reactor.nextMsgID`) are allocated concurrently across all peers' read loops, so UPDATEs arrive out of numeric order across sources (assumption A-5 is broken). Source B's higher MessageID advances `IngestedMsgID` past the cut while Source A's earlier announcement or withdrawal is still queued in `bgp-adj-rib-in`; worse, when `bgp-adj-rib-in` runs forked (`ingestTracked == false`), `IngestedMsgID` is omitted and the wait is skipped completely. Likewise, a single global `ForwardFrom` suppresses a late-arriving lower-numbered UPDATE from Source A on the live rail even though `bgp-rs` had not yet seen it at peer-up | A stale route, a missing route, or a duplicate UPDATE across concurrent sources and on forked `bgp-adj-rib-in` |
| 3 | Catch-up gives up after `replayCatchUpBudget` (5 s) without resetting the client session (leaving the peer's live-forward replay fence wedged), and an unrecoverable receive-store read failure from a loaded `bgp-adj-rib-in` calls `finishOptionalReplayRead`, which sends End-of-RIB and `signalSessionReady`, reporting the session complete with routes knowingly missing | A wedged live-forward fence after 5 s, or a peer reported complete with missing/stale routes |

The goal, incorporating the 2026-11-24 owner decision:

1. **Per-source causal cut and pre-walk catch-up (resolves Defects 1 and 2 / A-5):** Track each source actor's highest `MessageID` seen on `bgp-rs`'s event delivery loop (`PeerState.seenMsgID`). At `handleStateUp`, capture a per-source cut map (`PeerState.forwardFromBySource`) and the live source bounds (`PeerState.replaySources`) atomically under `rs.mu`. Before calling `ribevents.RelayReplay` (and before relaying any route), wait via the existing `ribevents.ReadReplaySourceProgress` projection (`request bgp adj-rib-in replay-routes <dest> 0 source-progress <json>`, already implemented in `adj_rib_in/replay_sources.go` on both internal and forked transports and already used by `bgp-rr`) until `bgp-adj-rib-in` has committed every captured live source's `MessageID`. Use the captured per-source cut on both the live rail (`selectForwardTargets`, `fastPathOwedTarget`) and the replay page / FlowSpec filter (`relayReplayRead`, `replayFlowSpecs`) so every UPDATE from every source is owned by exactly one rail.
2. **2-minute timeout and session-bound Cease reset (resolves Defect 3 / Owner decision 2026-11-24):** Remove the 5-second `replayCatchUpBudget` early give-up and wait up to the 2-minute replay timeout (`peerReplayTimeout = 2 * time.Minute`). If `bgp-adj-rib-in` is absent (`ErrReplayStoreAbsent` / `unknown command`), preserve the optional-dependency fallback (FlowSpec replay + End-of-RIB + `signalSessionReady`). If `bgp-adj-rib-in` is loaded and either still has not covered the captured sources at the 2-minute timeout or fails a source-progress/replay read unrecoverably (and the peer's replay generation is still current and the plugin is not shutting down), do **not** send End-of-RIB or `signalSessionReady`; record the failure under `doctor-bgp-rs-lifecycle` (`failReplay` + `warnLifecycle`) and reset the client's captured BGP session with a Cease NOTIFICATION (`request peer <addr> teardown 4 bgp-rs replay catch-up failed`, bound to `(DestinationOwner, DestinationSession)` so a stale replay goroutine never tears down a reconnected successor session).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/bgp-plugins.md`, `ai/doors/bgp-engine.md`, `ai/doors/plugin-infra.md`, `ai/doors/le-tooling.md`
  → Constraint: a plugin runs internal or forked, so `bgp-rs` reaches `bgp-adj-rib-in` and the reactor only through dispatched commands / SDK RPCs, never a Go call across packages, and every fix must work identically on both internal and forked transports
  → Constraint: "A barrier or a report is about timing, never about permission" (`ai/doors/bgp-plugins.md`)
  → Constraint: a forwarder keeps per-source FIFO order and never drops a cached UPDATE
- [ ] `docs/architecture/plugin/plugin-system.md`, "Peer-up barrier" and "Session-ready reports"
  → Constraint: the engine does not hold a peer's End-of-RIB for a session-ready report (owner ruling, 2026-09-18). The report lowers the replay fence and "does not assert successful route delivery"
  → Constraint: "Peer-up barrier" currently states that an UPDATE taken before `handleState` "belongs to the announce-only Adj-RIB-In replay, so its withdrawals never reach the peer"; AC-8 updates this paragraph to reflect pre-walk per-source catch-up
- [ ] `docs/architecture/api/architecture.md`, "Data Flow: Config to Delivery" (owner-relayed peer-up replay) and "API Sync Protocol"
  → Constraint: the replay owner relays each page under its own name, because the destination's `bgp:update` send grant is the owner's; the store answers pages and relays nothing
  → Constraint: a queued teardown that names an earlier session is dropped and never ends a successor session
- [ ] `docs/architecture/forward-congestion-pool.md`, "The replay fence"
  → Constraint: live forwards to a peer are held behind its peer-up replay and released by the session-ready report, so a live change never overtakes a replayed route
- [ ] `docs/architecture/core-design.md`, Section 9 "Forwarding Path"
  → Constraint: both route-server rails ask the forwarder only for targets "subject to the peer-up replay cut"
  → Constraint: a withdrawal "is written only where the destination's adj-out holds a record for that route"
- [ ] `docs/architecture/plugin/rib-storage-design.md`, "Received source-policy provenance", "Route-server optional replay read failure", and "Original source catch-up before cold membership"
  → Constraint: replayed routes carry their stored `source-policy-epoch`, `source-owner`, `source-connection-id`, and `source-receipt`
  → Constraint: "Original source catch-up before cold membership" already specifies `ribevents.ReadReplaySourceProgress` and `adj_rib_in.replaySourceProgressCommand` over `sourceLive.processedMsgID` on both typed and full-JSON transports; `bgp-rs` extends this existing mechanism rather than inventing a second signal
  → Decision (departure approved by Owner decision 2026-11-24): "Route-server optional replay read failure" currently says a failed optional receive-store page read still sends FlowSpec, End-of-RIB, and readiness; this spec departs for a loaded `bgp-adj-rib-in` (resetting with Cease instead) while preserving that fallback only when `bgp-adj-rib-in` is not loaded (`ErrReplayStoreAbsent`), and updates the page in the same change
- [ ] `docs/architecture/doctor-and-health-checks.md`, route-server runtime doctor section
  → Constraint: `bgp-rs` already declares runtime check `bgp-rs-lifecycle` under `doctor-bgp-rs-lifecycle` (`lifecycle_diagnostic.go`), which reads `actor.replayFailure` and raises bounded transient error events via `warnLifecycle`
- [ ] `docs/architecture/api/process-protocol.md`, "Plugin to Engine" (bound destination commands and stored-route replay) and `docs/architecture/api/commands.md` (`teardown`)
  → Constraint: destination-bound plugin commands carry `(DestinationOwner, DestinationSession)` via `DispatchDestinationCommand` and require `AcceptsDestinationSession: true` on the registered handler (`ze-bgp:peer-teardown`)

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4271.md` - Section 4.5 (NOTIFICATION), Section 6.7 (Cease), Section 8.2.2 (AutomaticStop Event 8), and Section 9.2 (Update-Send Process)
  → Constraint: Section 9.2 requires all newly installed routes and all newly unfeasible routes with no replacement to be advertised via UPDATE; retaining a withdrawn route at a late client violates it
  → Constraint: Section 6.7 permits Cease NOTIFICATION for non-fatal session termination, and Section 8.2.2 Event 8 (AutomaticStop) increments `ConnectRetryCounter` so a stuck store backs off instead of tight-looping
- [ ] `rfc/short/rfc7947.md` - Sections 2.1, 2.2, and 2.3 (route server forwarding and transparency)
  → Constraint: route-server replay must deliver the same policy-admitted, unmodified client routes as live forwarding without leaking withdrawn or import-filtered routes
- [ ] `rfc/short/rfc4724.md` - Section 2 (End-of-RIB marker)
  → Constraint: End-of-RIB indicates completion of the initial routing update; `bgp-rs` must not send its `sendEOR` / `signalSessionReady` when a loaded receive store failed to deliver the initial table

**Key insights:**
- A-5 is broken on HEAD: `reactor.nextMsgID()` is allocated concurrently across all peer read loops, so a single global high-water mark (`IngestedMsgID`) and a single global scalar `ForwardFrom` cannot prove per-source completeness across concurrent peers, and `IngestedMsgID` is absent when `bgp-adj-rib-in` runs forked.
- HEAD already landed the exact per-source causal mechanism in `ribevents/replay_sources.go` (`ReadReplaySourceProgress`) and `adj_rib_in/replay_sources.go` (`replaySourceProgressCommand` over `sourceLive.processedMsgID`, populated on both typed and JSON/forked paths) and already uses it in `bgp-rr`.
- HEAD also already landed `Peer.teardownOf(target *Session, notice teardownNotice, automatic bool)` (`reactor/peer.go`), `matchCapturedDestination` (`reactor/destination_constraint.go`), and `doctor-bgp-rs-lifecycle` (`rs/lifecycle_diagnostic.go`). Wiring `ze-bgp:peer-teardown` with `AcceptsDestinationSession: true` lets `bgp-rs` send a session-bound automatic Cease reset cleanly on both internal and forked transports without risking teardown of a reconnected successor session.

## Current Behavior (MANDATORY)

**Source files read (verified against HEAD on 2026-11-26):**
- [ ] `internal/component/bgp/plugins/rs/server_handlers.go` - `handleStateUp` sets `Up`, captures scalar `ForwardFrom = rs.seenMsgID`, and spawns `replayForPeer`. `replayForPeer` still calls `ribevents.RelayReplay` from index 0 *before* waiting, then polls `deltaReplay` against `!progress.coversCut(cut)` until `replayCatchUpBudget` (5 s). On 5 s timeout it logs a warning, calls `rs.failReplay`, and returns without sending EOR/ready and *without* resetting the peer session (leaving the replay fence held). On a non-output `RelayReplay`/`deltaReplay` error it calls `finishOptionalReplayRead`, which sends FlowSpec + EOR + `signalSessionReady`.
- [ ] `internal/component/bgp/plugins/rs/server.go` - defines `replayCatchUpBudget = 5 * time.Second`, `replayConvergenceDelay = 20 * time.Millisecond`, `replayConvergenceMax = 10`; updates only global `rs.seenMsgID` (not per-source `actor.seenMsgID`) in `dispatchStructured` and `dispatchText`; provides `peerDestinationAction` (`DispatchDestinationCommand` with `(DestinationOwner, DestinationSession)`).
- [ ] `internal/component/bgp/plugins/rs/peer.go` - `PeerState` holds scalar `ForwardFrom`, worker-updated `processedMsgID`, `replayFailure`, and `DestinationSession`; has no event-loop per-source `seenMsgID`, no per-source `forwardFromBySource`, and no `replaySources`.
- [ ] `internal/component/bgp/plugins/rs/server_forward.go` - `selectForwardTargets` and `fastPathOwedTarget` compare `msgID <= peer.ForwardFrom` (global scalar cut).
- [ ] `internal/component/bgp/plugins/rs/server_validation.go` - `replayFlowSpecs` compares `path.MsgID > cut` (global scalar cut).
- [ ] `internal/component/bgp/plugins/rs/replay_read.go` - `finishOptionalReplayRead` treats *any* non-`replayOutputError` read error (not only `unknown command`) as optional: warns, runs `replayFlowSpecs`, calls `finishReplayOutput` (`sendEOR` + `signalSessionReady`), and sets `actor.replayFailure`.
- [ ] `internal/component/bgp/plugins/rs/source_lifetime.go` - `failReplay`, `finishReplayOutput`, and `retryCurrentReplays` (re-invokes `replayForPeer` for live peers with `replayFailure != nil`).
- [ ] `internal/component/bgp/plugins/rs/lifecycle_diagnostic.go` - already registers and serves `doctor-bgp-rs-lifecycle` (`bgp-rs-lifecycle` runtime check reading `actor.replayFailure` and bounded transient event reporting via `warnLifecycle`).
- [ ] `internal/component/bgp/plugins/rs/retained_membership.go` - `currentStoredRouteLocked` and `checkColdRoutes` already filter replayed routes against live/GR-retained membership and per-source eligibility.
- [ ] `internal/core/bgp/ribevents/replay_sources.go` and `internal/component/bgp/plugins/adj_rib_in/replay_sources.go` + `source_state.go` - ALREADY LANDED in HEAD: `ReadReplaySourceProgress` queries `request bgp adj-rib-in replay-routes <dest> 0 source-progress <json>`, reading `sourceLive[owner].processedMsgID >= source.MessageID` (updated by `noteSourceProcessed` on BOTH structured and JSON/forked paths) without walking or relaying any route.
- [ ] `internal/component/bgp/plugins/rr/replay_sources.go` and `rr/rr.go` - ALREADY LANDED in HEAD: `bgp-rr` captures per-source bounds and runs `waitReplaySources` (`ReadReplaySourceProgress`) before `ribevents.RelayReplay`.
- [ ] `internal/component/bgp/plugins/cmd/peer/peer.go`, `internal/component/plugin/types_bgp.go`, `internal/component/plugin/coordinator.go`, `internal/component/bgp/reactor/reactor_api.go`, `internal/component/bgp/reactor/destination_constraint.go`, `internal/component/bgp/reactor/peer.go` - `ze-bgp:peer-teardown` (`handleTeardown`) calls `TeardownPeer(addr, NotifyCease, subcode, data)` without `AcceptsDestinationSession: true`; `matchCapturedDestination` and `Peer.teardownOf(session, notice, automatic)` already exist in the reactor.
- [ ] `internal/component/bgp/plugins/rs/replay_catchup_test.go` and `replay_read_test.go` - `replay_catchup_test.go` only tests the obsolete `coversCut` and `deltaReplay` helpers; `replay_read_test.go` `TestRSOptionalReadRefusalCompletesMandatoryReplayAndReconciles` currently asserts that a loaded-store read error still delivers FlowSpec + EOR + `signalSessionReady`.

**Behavior to preserve:**
- Disjoint partition between replay and live rail: no UPDATE reaches a peer on both rails, and no UPDATE is dropped between them
- Optional-dependency fallback when `bgp-adj-rib-in` is not loaded (`ErrReplayStoreAbsent` / `unknown command`): log once (`adjRibInMissingOnce`), replay authorized FlowSpecs from `bgp-rib`, send End-of-RIB, and call `signalSessionReady`
- Legacy-ingest routes with unknown `MessageID == 0` remain eligible for replay (never dropped)
- GR-retained routes from a source that went down before the new client came up remain eligible for replay
- The replay owner (`bgp-rs`) relays pages under its own name (`bgp:update` grant); `bgp-adj-rib-in` relays nothing
- End-of-RIB timing as ruled on 2026-09-18: no new hold of the engine's initial-sync marker
- Stale replay goroutine (peer reconnected with a newer `ReplayGen` or went down) sends no routes, no EOR, no readiness report, and no teardown against the newer session
- Unbound operator `request peer <sel> teardown <subcode> [message]` keeps its exact existing behavior (ManualStop, address/selector resolution, RFC 9003 Shutdown Communication)
- Published `ReplayPage` JSON shape (`IngestedMsgID` included) stays unchanged for third-party replay owners

**Behavior to change:**
- `bgp-rs` tracks per-source `seenMsgID` on the event loop and captures per-source cuts (`forwardFromBySource`) and live source bounds (`replaySources`) atomically at `handleStateUp`
- `replayForPeer` waits via `ribevents.ReadReplaySourceProgress` *before* the first `ribevents.RelayReplay` call, on both internal and forked transports, and deletes the old walk-first global `coversCut` / `deltaReplay` / 5 s `replayCatchUpBudget` code
- Both the live rail (`selectForwardTargets`, `fastPathOwedTarget`) and the replay filter (`relayReplayRead`, `replayFlowSpecs`) apply the captured per-source cut
- Catch-up waits up to the 2-minute `peerReplayTimeout`; if a loaded `bgp-adj-rib-in` fails to reach the cut by 2 minutes or fails a source-progress/replay read unrecoverably, `bgp-rs` withholds EOR and `signalSessionReady`, records the failure under `doctor-bgp-rs-lifecycle`, and resets the captured client session via session-bound automatic Cease teardown (`request peer <addr> teardown 4 bgp-rs replay catch-up failed`)

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Per-source receive-store progress projection | `internal/core/bgp/ribevents/replay_sources.go` (`ReplaySource`, `ReadReplaySourceProgress`, `ErrReplayStoreAbsent`); `internal/component/bgp/plugins/adj_rib_in/replay_sources.go` (`replaySourceProgressCommand`); `adj_rib_in/source_state.go` (`sourceLive.processedMsgID`, `noteSourceProcessed`) | Answers whether `bgp-adj-rib-in` has committed every requested physical source `(Address, PeerOwner, ConnectionID, MessageID)` on both internal and forked transports without walking or relaying any route | Extends (reuses directly): `bgp-rs` calls `ReadReplaySourceProgress` ahead of `RelayReplay`, matching `bgp-rr` (`rr/replay_sources.go`) and solving A-3 and A-5 with zero wire changes |
| Peer-up cut | `rs/server_handlers.go` (`handleStateUp`), `rs/peer.go` (`PeerState.ForwardFrom`), `rs/server_forward.go` (`selectForwardTargets`, `fastPathOwedTarget`), `rs/server_validation.go` (`replayFlowSpecs`), `adj_rib_in/replay_cut.go` (`replayCut`) | Splits the UPDATE stream by one global scalar `ForwardFrom` | Extends: adds per-source cut `PeerState.forwardFromBySource` captured from per-source `PeerState.seenMsgID` on the event loop; keeps `ForwardFrom` as the max bound sent to `adj_rib_in` while enforcing the exact per-source cut on both live forwarding and replay filtering |
| Global ingest-position catch-up loop | `rs/server_handlers.go` (`replayForPeer` phase 1/2, `replayProgress`, `coversCut`, `deltaReplay`); `rs/server.go` (`replayCatchUpBudget = 5s`) | Walks and relays first, then polls global `IngestedMsgID` for 5 s, then gives up without resetting the peer | Replaces: deletes the walk-first global `coversCut` / `deltaReplay` / 5 s budget loop (`ai/rules/no-layering.md`); replaces with pre-walk per-source `waitReplaySources` bounded by 2-minute `peerReplayTimeout` |
| Paged owner-relayed replay | `ribevents/replay.go` (`RelayReplay`), `adj_rib_in/replay_page.go` (`replayRoutesCommand`, `buildReplayPage`) | Reads stored routes page by page and relays them under `bgp-rs`'s own send grant | Extends: runs only *after* `waitReplaySources` confirms coverage, and filters each page by the captured per-source cut before `relayReplayRead`; `ReplayPage` wire format is preserved unchanged |
| Replay fence | `docs/architecture/forward-congestion-pool.md` ("The replay fence"); `rs/server_handlers.go` (`signalSessionReady`, `endReplay`) | Holds live forwards to the peer until `signalSessionReady` lowers the fence | Extends: keeps the fence up throughout the pre-walk wait and single consistent replay, and never lowers it on catch-up failure (the session is reset with Cease instead) |
| Destination-bound peer command & single-session teardown | `rs/server.go` (`peerDestinationAction`, `DispatchDestinationCommand`); `cmd/peer/peer.go` (`handleTeardown`, `ze-bgp:peer-teardown`); `reactor/destination_constraint.go` (`matchCapturedDestination`); `reactor/peer.go` (`Peer.teardownOf`) | `peerDestinationAction` sends destination-bound `request peer` commands; `matchCapturedDestination` resolves exact `(owner, session)`; `Peer.teardownOf(session, notice, true)` sends an automatic Cease NOTIFICATION to only that session and drops stale ones | Extends: enables `AcceptsDestinationSession: true` on `ze-bgp:peer-teardown` and routes bound teardown calls through `matchCapturedDestination` + `peer.teardownOf(session, notice, true)` so `bgp-rs` resets only the failed session |
| Route-server lifecycle doctor check & transient issue queue | `rs/lifecycle_diagnostic.go` (`lifecycleDoctorCode` = `doctor-bgp-rs-lifecycle`, `lifecycleDoctorCheck`, `warnLifecycle`); `rs/source_lifetime.go` (`failReplay`) | Reports `actor.replayFailure` on `ze show doctor` and raises rate-limited transient error events through the SDK report bus | Extends (reuses directly): catch-up timeout and unrecoverable store read failures record `failReplay` and emit `warnLifecycle` under `doctor-bgp-rs-lifecycle` |
| Planned doctor code `doctor-bgp-peer-up-replay-incomplete` | `plan/release/validation/spec-doctor-coverage-7-bgp-plugins.md`, AC-17 | Planned in sibling validation spec, not implemented | Does not apply: `bgp-rs` already landed `doctor-bgp-rs-lifecycle` in `lifecycle_diagnostic.go`; this spec reuses that live code rather than introducing a duplicate doctor code |
| Route reflector peer-up replay | `rr/replay_sources.go` (`captureReplaySources`, `waitReplaySources`), `rr/rr.go` (`replayForPeer`) | Already waits on `ReadReplaySourceProgress` before `RelayReplay` | Does not apply: `bgp-rr` already adopted pre-walk per-source catch-up on HEAD; this spec brings `bgp-rs` onto the same mechanism plus the 2-minute Cease reset |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | A plugin can run in another process | the door every change reads | "Every plugin, ours included, runs either internal (a goroutine in the daemon) or external (a forked subprocess that talks only over TLS and RPC). The SDK hides which. Code that only works internal is broken." | agrees | `bgp-rs` uses dispatched `ReadReplaySourceProgress` (backed by `noteSourceProcessed` on both typed and JSON/forked paths) and dispatched `request peer <addr> teardown` via `DispatchDestinationCommand`, working identically internal and forked |
| `ai/doors/bgp-plugins.md` | Boundaries | the door covering `internal/component/bgp/plugins/rs/*`, `internal/component/bgp/plugins/cmd/peer/peer.go` | "Reach the engine through the bus, DirectBridge or a text command, never a plain call into `reactor` or another component." | agrees | `bgp-rs` talks to `bgp-adj-rib-in` only via `ReplayRoutesCommand` (`source-progress` and page reads) and to the reactor only via SDK `DispatchDestinationCommand` (`request peer ... teardown`) and `RelayStoredRoute`; zero cross-plugin Go imports |
| `ai/doors/bgp-engine.md` | Boundaries | the door covering `internal/component/bgp/reactor/reactor_api.go`, `internal/component/bgp/reactor/peer_close_test.go` | "Never name a plugin package from the reactor." | agrees | The reactor change (`TeardownPeer` with optional bound `plugin.Sender`) uses existing generic `plugin.Sender.Destination()`, `matchCapturedDestination`, and `Peer.teardownOf` without naming `bgp-rs` or any plugin |
| `ai/doors/plugin-infra.md` | Boundaries | the door covering `internal/component/plugin/types_bgp.go`, `internal/component/plugin/coordinator.go` | "`DirectBridge` is an in-process shortcut of the same protocol, and a forked plugin always takes the RPC path." | agrees | `DispatchDestinationCommand` already crosses both `DirectBridge` and framed TLS RPC; adding `AcceptsDestinationSession: true` to `ze-bgp:peer-teardown` works identically on both transports |
| `ai/doors/le-tooling.md` | Boundaries | the door covering `test/plugin/*.ci`, `test/interop/scenarios/*` | "Run one package through `./le test unit pkg path <dir>/ [run <regex>] [vet]`, which takes the tags from the toolchain." | agrees | All Deliverables and verification commands use `./le test unit pkg path ...`, `./le test stress-repro`, and `./le test integration interop` rather than bare `go test` or whole-tree checks |
| `docs/architecture/core-design.md` | Forwarding Path | the `// Design:` page of `rs/peer.go`, `rs/server.go`, `rs/server_handlers.go`, `rs/replay_catchup_test.go`, `plugin/coordinator.go`, `reactor/reactor_api.go` | "Both route-server rails therefore ask the cached forwarder only for the skipped targets and sessions established after the UPDATE was numbered, subject to the peer-up replay cut." | agrees | Both `selectForwardTargets` and `fastPathOwedTarget` continue to partition live forwards by the peer-up replay cut, refined from one global scalar to a per-source causal cut |
| `docs/architecture/api/architecture.md` | Data Flow: Config to Delivery | anchored to `rs/server_handlers.go`, `reactor/reactor_api.go`; a page this spec modifies | "For their non-FlowSpec families, the receive store must first apply its queued UPDATE events." | agrees | Enforces this exact invariant in `bgp-rs` by running `ReadReplaySourceProgress` before the first `RelayReplay` page, and updates the page to document `bgp-rs`'s pre-walk source-progress wait and 2-minute Cease reset |
| `docs/architecture/api/process-protocol.md` | Plugin to Engine | the `// Design:` page of `plugin/types_bgp.go`; anchored to `rs/server.go`, `rs/server_handlers.go`, `reactor/reactor_api.go` | "An explicitly present null, zero-only or partial constraint refuses; it never becomes an unbound operator request." | agrees | Session-bound teardown reuses `DispatchDestinationCommand` (`ValidateDestination`) and `matchCapturedDestination`, refusing stale, partial, or mismatched `(DestinationOwner, DestinationSession)` tuples instead of falling back to address teardown |
| `docs/architecture/plugin/rib-storage-design.md` | Route-server optional replay read failure | the `// Design:` page of `rs/server_forward.go`, `rs/replay_read.go`, `rs/source_lifetime.go`, `rs/replay_read_test.go`; a page this spec modifies | "A failed optional receive-store page read does not suppress the selecting RIB's mandatory FlowSpec phase or the original destination's End-of-RIB/readiness report." | departs | Per Owner decision (2026-11-24), when `bgp-adj-rib-in` IS loaded and fails unrecoverably (or times out at 2 minutes), `bgp-rs` suppresses End-of-RIB and readiness and resets the client session with a Cease NOTIFICATION so it reconnects and receives a complete table; only an *absent* (`ErrReplayStoreAbsent`) store takes the FlowSpec-only fallback. This page is updated in the same change |
| `docs/architecture/plugin/plugin-system.md` | Peer-up barrier | anchored to `rs/server.go`; a page this spec modifies | "An UPDATE taken delivery of before that lands at or below the peer's cut and belongs to the announce-only Adj-RIB-In replay, so its withdrawals never reach the peer." | departs | Pre-walk per-source catch-up guarantees every UPDATE (announcements and withdrawals alike) at or below each source's cut is applied in `bgp-adj-rib-in` before `bgp-rs` walks and relays it, so withdrawn routes are already absent from the replay. AC-8 updates this paragraph in the same change |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | the `// Design:` page of `rs/server_validation.go`; anchored to `rs/server_forward.go`, `rs/server_validation.go` | "Both forwarding rails key body reuse on effective opaque-attribute treatment, not on a plugin or peer role." | agrees | `server_validation.go` only refines `replayFlowSpecs`'s cut comparison from scalar `cut` to per-source cut; opaque-attribute treatment, receive validation, and fast-path deferral are untouched |
| `docs/architecture/api/commands.md` | Peer Selectors | the `// Design:` page of `cmd/peer/peer.go`; anchored to `rs/server.go`, `cmd/peer/peer.go`, `reactor/reactor_api.go` | "A command whose handler resolves its selector through `ResolveSinglePeer` acts on one peer: the selector must resolve to exactly one, and `*`, an exclusion or a selector matching several peers is refused." | agrees | `handleTeardown` keeps `ResolveSinglePeer` and its exact operator CLI grammar unchanged, while honoring `ctx.Sender.Destination()` when invoked as a destination-bound plugin command |
| `docs/architecture/doctor-and-health-checks.md` | The two tests a new check carries | anchored to `rs/server.go` | "The route server declares `bgp-rs-lifecycle` under `doctor-bgp-rs-lifecycle`. Its runtime callback reads the existing original-source replay failures, unfinished source-lease releases, unknown captured outputs and failed settlement where that plugin runs." | agrees | Catch-up timeout and unrecoverable store read failure record `actor.replayFailure` and raise a transient event via `warnLifecycle` under the existing `doctor-bgp-rs-lifecycle` code |
| `docs/architecture/behavior/peer-lifecycle.md` | Entry points | the `// Design:` page of `reactor/peer_close_test.go`; anchored to `reactor/reactor_api.go` | "A reload's binding restart uses `teardownOf` with the session it decided on instead: it does nothing once the peer has left that session, and the drain drops it if still queued when a later session holds the peer." | agrees | Destination-bound `TeardownPeer` reuses `Peer.teardownOf(session, notice, true)` directly so a stale replay timeout never ends a replacement session |
| `docs/architecture/rib-transition.md` | Executive Summary | anchored to `rs/server_forward.go` | | silent | High-level transition history moving RIB ownership into plugins; route-server per-source replay catch-up keeps RIB storage in `bgp-adj-rib-in` and `bgp-rib` unchanged |
| `docs/architecture/behavior/fsm.md` | Optional events ze implements | anchored to `cmd/peer/peer.go`, `reactor/reactor_api.go` | "Event 2 (ManualStop) zeroes the counter; Event 8 increments it. The operator did not ask for this stop" | agrees | Destination-bound plugin teardown uses `Peer.teardownOf(session, notice, true)` (Event 8 `AutomaticStop`, incrementing `ConnectRetryCounter` so a stuck store backs off reconnects), while unbound operator teardown keeps Event 2 `ManualStop` |
| `docs/architecture/config/syntax.md` | Overview | anchored to `plugin/coordinator.go` | | silent | Configuration file syntax and keywords are untouched; no config option is added or changed |
| `docs/architecture/api/text-format.md` | Current Format | anchored to `reactor/reactor_api.go` | | silent | Text IPC event wire format is untouched; `bgp-rs` reads `msgID` from the existing text event header unchanged |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `reactor/reactor_api.go` | | silent | Config verify/apply/rollback transaction events are untouched; this spec changes runtime peer-up replay and teardown only |
| `docs/architecture/wire/nlri-bgpls.md` | Wire Format Overview | anchored to `reactor/reactor_api.go` | | silent | BGP-LS NLRI wire framing is untouched; `StoredRoute` payloads and NLRI splitters are unchanged |
| `docs/architecture/edge-cases/extended-message.md` | Capability and limits | anchored to `rs/server_handlers.go` | | silent | Extended message capability negotiation and 65,535-octet framing are unaffected; this spec only changes `bgp-rs` replay catch-up timing, per-source cut checks, and session-bound Cease reset |
| `docs/architecture/wire/nlri.md` | Overview | anchored to `rs/server_handlers.go` | | silent | NLRI wire encoding and family framing are unaffected; `StoredRoute` payloads and NLRI splitters do not change |
| `docs/architecture/api/text-parser.md` | Current Parser | anchored to `rs/server.go` | | silent | Text event syntax and `quickParseTextEvent` tokenization do not change; `dispatchText` only records `actor.seenMsgID` from the already-parsed `msgID` |
| `docs/architecture/buffer-architecture.md` | Executive Summary | anchored to `rs/server.go` | | silent | Pooled wire buffers and `WriteTo` encoding are unaffected; no hot-path buffer allocation or ownership contract changes |
| `docs/architecture/core-design.md` | Design decisions | `rs/peer.go`, `rs/server.go`, `rs/server_handlers.go`, `rs/replay_catchup_test.go`, `plugin/coordinator.go`, `reactor/reactor_api.go` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Unrelated to BGP route-server replay; this spec changes no data-plane backend or config validation |
| `docs/architecture/core-design.md` | Design decisions | same | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Unrelated to BGP route-server replay; this spec touches no startup or config-validation backend check |
| `docs/architecture/core-design.md` | Design decisions | same | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Unrelated to BGP route-server replay; this spec touches no firewall or YANG annotation |
| `docs/architecture/core-design.md` | Design decisions | same | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Unrelated to BGP route-server replay; this spec introduces or renames no data-plane selector |
| `docs/architecture/core-design.md` | Design decisions | same | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Replayed routes continue through `RelayStoredRoute` and the existing export chain unchanged, preserving route-server next-hop transparency |
| `docs/architecture/core-design.md` | Design decisions | same | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec does not touch `policychain` or export next-hop authoring |
| `docs/architecture/core-design.md` | Design decisions | same | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec does not touch export policy compilation or `FilterInsertion` |
| `docs/architecture/core-design.md` | Design decisions | same | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec does not alter export next-hop folding or `wireOverride` |
| `docs/architecture/core-design.md` | Design decisions | same | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec does not alter import/export next-hop validation |
| `docs/architecture/core-design.md` | Design decisions | same | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Replayed routes preserve their forwarded origin semantics through `RelayStoredRoute` unchanged |
| `docs/architecture/core-design.md` | Design decisions | same | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec adds or modifies no filter-chain insertion or `\| implied` behavior |
| `docs/architecture/core-design.md` | Design decisions | same | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec never stops `ze` from starting; a failed peer-up replay resets only that affected BGP client session with Cease while other sessions and services keep running |
| `docs/architecture/core-design.md` | Design decisions | same | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Unrelated to kernel/VPP startup validation; this spec keeps startup intact and reports replay failures under `doctor-bgp-rs-lifecycle` |
| `docs/architecture/api/architecture.md` | Design decisions | `rs/server_handlers.go`, `reactor/reactor_api.go`, modified page | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec does not change `\| implied` rendering or config display |
| `docs/architecture/api/architecture.md` | Design decisions | same | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | `bgp-rs` already registers `cli:request bgp adj-rib-in replay-routes` and `cli:request peer` in `rsBusSends()` (`rs/register.go`), and this spec uses only those registered bus messages |
| `docs/architecture/api/architecture.md` | Design decisions | same | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | This spec does not alter bus `send > to` resolution or participant declarations |
| `docs/architecture/api/architecture.md` | Design decisions | same | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | This spec does not alter bus `receive > from` filtering |
| `docs/architecture/api/architecture.md` | Design decisions | same | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Both `ReadReplaySourceProgress` (`replay-routes`) and session-bound `teardown` (`request peer`) pass through normal bus message enforcement |
| `docs/architecture/api/architecture.md` | Design decisions | same | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This spec does not alter bus filter compilation or commit validation |
| `docs/architecture/api/architecture.md` | Design decisions | same | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | `bgp-rs` keeps its existing `rs-client` scoping and `bgp-peer-up-replay` coordination without adding bus arbitration |
| `docs/architecture/api/architecture.md` | Design decisions | same | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | This spec does not alter bus display commands |
| `docs/architecture/api/process-protocol.md` | Design decisions | `plugin/types_bgp.go`, `rs/server.go`, `rs/server_handlers.go`, `reactor/reactor_api.go` | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | `bgp-rs` calls only existing engine operations (`DispatchCommandArgs` for `replay-routes` and `DispatchDestinationCommand` for `request peer ... teardown`), each already declared in `engineOps` |
| `docs/architecture/api/process-protocol.md` | Design decisions | same | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Per-source causal catch-up (`ReadReplaySourceProgress`) relies directly on lossless per-source event delivery to both `bgp-rs` and `bgp-adj-rib-in` |
| `docs/architecture/api/commands.md` | Design decisions | `cmd/peer/peer.go`, `rs/server.go` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `request peer <sel> teardown <subcode> [message]` syntax and pipe handling are unchanged |
| `docs/architecture/api/commands.md` | Design decisions | same | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec adds or modifies no pipe operator or command answer shape |
| `docs/architecture/api/commands.md` | Design decisions | same | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec adds or modifies no pipe behavior |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | `rs/server.go`, modified page | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | This spec does not change query-mode startup or offline config parsing |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A peer-up state event (`state up`, `RSClient == true`, `Reason != attached`, `!UpdateSendDenied`) delivered to `bgp-rs` for a route-server client while other RS clients have sent UPDATEs whose effects `bgp-adj-rib-in` may not yet have committed.

### Transformation Path
1. **Per-source event-loop watermark (`rs/server.go`):** On `bgp-rs`'s event delivery goroutine (`dispatchStructured` and `dispatchText`), for each admitted UPDATE from physical source `key = (address, peerOwner, connectionID)` with `msgID > 0`, advance both `rs.seenMsgID` and `actor.seenMsgID` under `rs.mu`.
2. **Atomic peer-up capture (`rs/server_handlers.go` `handleStateUp`):** Under one `rs.mu.Lock()` critical section:
   - Mark the destination peer `Up`, increment `ReplayGen`, shut `replayDone`, capture `ForwardFrom = rs.seenMsgID`.
   - Snapshot `forwardFromBySource[sourceKey] = source.seenMsgID` for every non-destination RS-client actor in `rs.peers` with `seenMsgID > 0` (including GR-retained actors that are already down, so their retained routes remain replayable).
   - Snapshot `replaySources` as the slice of `ribevents.ReplaySource{Address, PeerOwner, ConnectionID, MessageID: source.seenMsgID}` for every currently live (`Up || !StateSeen`) non-destination RS-client actor with `seenMsgID > 0`.
   - Spawn `replayForPeer` with the captured `target`, `gen`, `cut` (`ForwardFrom`), `forwardFromBySource`, `replaySources`, `initialReplay`, and `done`.
3. **Live-rail partition (`rs/server_forward.go`):** `selectForwardTargets` and `fastPathOwedTarget` suppress forwarding an UPDATE from `source` with `msgID != 0` to `peer` if and only if `msgID <= peer.forwardFromFor(source)` (where `forwardFromFor` reads `peer.forwardFromBySource[source]`, returning 0 when `source` was absent at peer-up). Every UPDATE above `source`'s captured cut goes on the live rail (held behind the peer's replay fence).
4. **Pre-walk per-source catch-up (`rs/server_handlers.go` `waitReplaySources`):** Before calling `ribevents.RelayReplay`, loop while any captured source in `replaySources` is still live (`Up || !StateSeen`):
   - Check stale generation (`peer == nil || peer.ReplayGen != gen || !peer.Up`); exit cleanly without sending anything if stale.
   - Check the 2-minute timeout against both `ctx.Err()` and `rs.now().After(deadline)` (`deadline := rs.now().Add(peerReplayTimeout)`).
   - Query `ribevents.ReadReplaySourceProgress(ctx, rs.dispatchCommand, peerAddr, pending)` (`request bgp adj-rib-in replay-routes <dest> 0 source-progress <json>`).
   - On `ErrReplayStoreAbsent` / `isDispatchUnknownCommand`: `bgp-adj-rib-in` is not loaded; run optional-store FlowSpec fallback (`replayFlowSpecs` + `finishReplayOutput`).
   - On any other error or 2-minute timeout: enter step 7 (failure + session-bound Cease reset).
   - If `covered == true`: clear `peer.replaySources` and proceed to step 5.
   - Otherwise wait `replayConvergenceDelay` (20 ms, cancellable by `ctx.Done()`, honoring `rs.clk` when injected for tests).
5. **Single consistent replay walk (`rs/server_handlers.go` + `ribevents.RelayReplay`):** Call `ribevents.RelayReplay` from index 0 bounded by `cutArg` (`maxCut = cut`). In the page relay callback, drop any `StoredRoute` whose `MsgID != 0 && MsgID > cutFor(route.SourcePeer, route.SourceOwner, route.SourceConnectionID)` so a cross-peer allocation inversion never delivers a route on both rails, then pass the surviving page to `rs.relayReplayRead`. If `RelayReplay` returns an error:
   - `isDispatchUnknownCommand`: optional-store FlowSpec fallback + `finishReplayOutput`.
   - Stale generation or plugin shutdown (`rs.sessionEnded(err)` / `parent.Err() != nil`): exit without teardown.
   - Any other store read or relay error: enter step 7 (failure + session-bound Cease reset).
6. **FlowSpec replay, EOR, and fence release (`rs/server_validation.go`, `rs/server_handlers.go`):** `replayFlowSpecs` filters authorized FlowSpec paths using the same per-source cut (`path.MsgID == 0 || path.MsgID <= cutFor(source)`). Then `finishReplayOutput` sends End-of-RIB (`sendEOR`) and calls `signalSessionReady`, lowering the replay fence and releasing held live forwards in order.
7. **Unrecoverable failure / 2-minute timeout path (`rs/server_handlers.go`, `cmd/peer/peer.go`, `reactor/reactor_api.go`):**
   - Do **not** call `sendEOR` and do **not** call `signalSessionReady`.
   - Call `rs.failReplay(key, gen, cause)` and `rs.warnLifecycle(...)` with `code = doctor-bgp-rs-lifecycle`, peer, cut, cause, and `fix = ze show bgp rs status`.
   - Dispatch `rs.peerDestinationAction(resetCtx, target, "teardown 4 bgp-rs replay catch-up failed")` (`DispatchDestinationCommand` with `(DestinationOwner, DestinationSession)`).
   - `handleTeardown` (`ze-bgp:peer-teardown`, now `AcceptsDestinationSession: true`) detects the bound `ctx.Sender.Destination()` tuple and calls the reactor's destination-bound teardown (`matchCapturedDestination` + `peer.teardownOf(session, ceaseNotice(4, "bgp-rs replay catch-up failed"), true)`), sending Cease / Administrative Reset (6/4) with RFC 9003 Shutdown Communication via AutomaticStop (Event 8) to only that captured session, and dropping the teardown harmlessly if the peer already reconnected.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `bgp-rs` ↔ `bgp-adj-rib-in` | `request bgp adj-rib-in replay-routes` (`source-progress` projection and page walk) through the dispatcher, on both internal and forked transports | Yes, read: `ribevents/replay_sources.go`, `ribevents/replay.go`, `adj_rib_in/replay_page.go`, `adj_rib_in/replay_sources.go` |
| `bgp-rs` ↔ engine (`relay` / `ready` / `teardown`) | `RelayStoredRoute`, `UpdateRouteRequest` (EOR), and `DispatchDestinationCommand` (`request peer <addr> plugin session ready session <token>`, `request peer <addr> teardown 4 ...`) carrying `(DestinationOwner, DestinationSession)` | Yes, read: `rs/server.go`, `plugin/server/dispatch_destination.go`, `cmd/peer/peer.go`, `reactor/destination_constraint.go`, `reactor/peer.go` |

### Changed-Wire Census

No wire format, RPC message schema, JSON field, or BGP protocol encoding changes in this spec. Both `request bgp adj-rib-in replay-routes <dest> 0 source-progress <json>` (`ReplaySource` / `ReplaySourceProgress`) and `DispatchDestinationCommand` (`destination-owner` / `destination-session` over `request peer <addr> teardown 4 <msg>`) already exist on the wire and are reused unchanged.

**Search evidence:** Searched `internal/` and `pkg/` for `RelayReplay(`, `ReplayRoutesCommand`, `ReplaySourceProgress`, `IngestedMsgID`, `ze-bgp:peer-teardown`, and `TeardownPeer(` on 2026-11-26. Positive controls: found `rs/server_handlers.go` `replayForPeer`, `rr/replay_sources.go` `waitReplaySources`, `adj_rib_in/replay_page.go` `replayRoutesCommand`, `cmd/peer/peer.go` `handleTeardown`, and `reactor/reactor_api.go` `TeardownPeer`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `request bgp adj-rib-in replay-routes` answer (`ReplayPage` JSON) | writer | `internal/component/bgp/plugins/adj_rib_in/replay_page.go` `replayRoutesCommand` | Builds `ReplayPage` and sets `IngestedMsgID` when tracked | Preserve unchanged: no field added or removed |
| `request bgp adj-rib-in replay-routes` answer (`ReplayPage` JSON) | reader | `internal/core/bgp/ribevents/replay.go` `RelayReplay` (called by `rs/server_handlers.go`, `rr/rr.go`, `reactor/policy_reload.go`, and external replay owners) | Decodes `ReplayPage` and relays routes | Preserve unchanged: `bgp-rs` runs it only after `ReadReplaySourceProgress` succeeds |
| `request bgp adj-rib-in replay-routes ... source-progress` request (`[]ReplaySource` JSON) | writer | `internal/core/bgp/ribevents/replay_sources.go` `ReadReplaySourceProgress` (called by `rr/replay_sources.go` and now `rs/server_handlers.go`) | Marshals `[]ReplaySource` in batches up to `ReplaySourceBatchMax` (1024) and dispatches `ReplayRoutesCommand` | Preserve unchanged: `bgp-rs` becomes a second caller beside `bgp-rr` |
| `request bgp adj-rib-in replay-routes ... source-progress` request (`[]ReplaySource` JSON) | reader | `internal/component/bgp/plugins/adj_rib_in/replay_sources.go` `replaySourceProgressCommand` | Unmarshals `[]ReplaySource` and checks `sourceLive[owner].processedMsgID >= source.MessageID` on both typed and JSON/forked paths | Preserve unchanged: reused directly by `bgp-rs` |
| `request bgp adj-rib-in replay-routes ... source-progress` answer (`ReplaySourceProgress` JSON) | writer | `internal/component/bgp/plugins/adj_rib_in/replay_sources.go` `replaySourceProgressCommand` | Answers `{"covered": bool}` with no route walk | Preserve unchanged: reused directly by `bgp-rs` |
| `request bgp adj-rib-in replay-routes ... source-progress` answer (`ReplaySourceProgress` JSON) | reader | `internal/core/bgp/ribevents/replay_sources.go` `ReadReplaySourceProgress` (called by `rr/replay_sources.go` and now `rs/server_handlers.go`) | Unmarshals `ReplaySourceProgress` and checks `*progress.Covered` | Preserve unchanged: `bgp-rs` becomes a second caller beside `bgp-rr` |
| `DispatchDestinationCommand` carrying `request peer <sel> teardown <subcode> [msg]` | writer | `internal/component/bgp/plugins/rs/server.go` `peerDestinationAction` (and unbound operator CLI/API/ExaBGP callers) | Sends `rpc.DispatchCommandInput` with `DestinationOwner` and `DestinationSession` | Preserve existing RPC schema; `ze-bgp:peer-teardown` opts into `AcceptsDestinationSession: true` |
| `DispatchDestinationCommand` carrying `request peer <sel> teardown <subcode> [msg]` | reader | `internal/component/plugin/server/dispatch_destination.go` `dispatchDestinationCommand` → `internal/component/bgp/plugins/cmd/peer/peer.go` `handleTeardown` → `reactor/reactor_api.go` `TeardownPeer` | Validates `(DestinationOwner, DestinationSession)` via `WithDestination` and `matchCapturedDestination`, calling `peer.teardownOf(session, notice, true)` when bound and `peer.TeardownNotification` when unbound | Preserve unbound operator behavior unchanged; add session-bound teardown dispatch when `ctx.Sender.Destination()` is nonzero |
| BGP NOTIFICATION Cease / Administrative Reset (code 6, subcode 4, RFC 9003 Shutdown Communication) | writer | `internal/component/bgp/reactor/peer.go` `Peer.teardownOf` → `session.teardownAutomatic` | Encodes standard RFC 4271/4486/9003 NOTIFICATION 6/4 with Shutdown Communication and closes the captured TCP session | Preserve standard BGP wire format unchanged |
| BGP NOTIFICATION Cease / Administrative Reset (code 6, subcode 4, RFC 9003 Shutdown Communication) | reader | Remote BGP route-server client (outside the repository) | Standard BGP speaker closes the session and reconnects to receive a fresh complete table | Preserve standard BGP wire format unchanged |

### Integration Points
- `ribevents.ReadReplaySourceProgress` (`internal/core/bgp/ribevents/replay_sources.go`) - called by `rs` `waitReplaySources` before `RelayReplay`.
- `rs.peerDestinationAction` (`internal/component/bgp/plugins/rs/server.go`) - dispatches session-bound `request peer <addr> teardown 4 bgp-rs replay catch-up failed` on catch-up timeout or unrecoverable read failure.
- `ze-bgp:peer-teardown` (`internal/component/bgp/plugins/cmd/peer/peer.go`) - sets `AcceptsDestinationSession: true` and forwards `ctx.Sender` into `ReactorLifecycle.TeardownPeer`.
- `reactorAPIAdapter.TeardownPeer` (`internal/component/bgp/reactor/reactor_api.go`) - uses existing `matchCapturedDestination` (`destination_constraint.go`) and `Peer.teardownOf(session, notice, true)` (`peer.go`) when `sender.Destination()` is nonzero.
- `rs.failReplay` and `rs.warnLifecycle` (`rs/source_lifetime.go`, `rs/lifecycle_diagnostic.go`) - report catch-up timeout and store read failures under `doctor-bgp-rs-lifecycle`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | `bgp-rs` reaches `bgp-adj-rib-in` only through `ReplayRoutesCommand` and resets the client only through `DispatchDestinationCommand` (`request peer ... teardown`) |
| No unintended coupling (components stay isolated) | Yes | No import between `rs` and `adj_rib_in`; `ribevents` stays the shared core leaf; `reactor` uses generic `plugin.Sender` and never names `bgp-rs` |
| No duplicated functionality (extends existing, does not recreate) | Yes | Reuses `ribevents.ReadReplaySourceProgress`, `adj_rib_in.replaySourceProgressCommand`, `rs.peerDestinationAction`, `matchCapturedDestination`, `Peer.teardownOf`, and `doctor-bgp-rs-lifecycle`; deletes obsolete `coversCut`/`deltaReplay` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Per-source `seenMsgID` is one `uint64` update under the existing `rs.mu` lock; `forwardFromFor` is one map lookup on the worker path; no wire buffer copy is added |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No new command, event, or plugin is added; `ze-bgp:peer-teardown` sets the existing generic `AcceptsDestinationSession: true` option on its own `RPCRegistration` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Introduces no new command, config leaf, doctor code, or plugin name. Searched `internal/component/plugin/server/command.go`, `internal/component/bgp/plugins/cmd/peer/peer.go`, and `internal/component/bgp/plugins/rs/register.go` — `AcceptsDestinationSession` and `doctor-bgp-rs-lifecycle` already derive dynamically from registration |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Defect 1's sequence is reachable on HEAD: `replayForPeer` relays routes from `bgp-adj-rib-in` before the store has ingested a withdrawal at or below the cut | Verified in `rs/server_handlers.go:645` (`RelayReplay` runs before the `coversCut` loop) and `adj_rib_in/replay_page.go:51` (`replayRoutesCommand` answers immediately without waiting) | The spec would fix an unreachable race | `TestReplayNeverRelaysBeforeStoreCoversCut`, failing on HEAD before the fix | confirmed by source; runtime test owed |
| A-2 | Once `bgp-adj-rib-in` has committed every live source's captured `seenMsgID` (via `ReadReplaySourceProgress`), a single `RelayReplay` walk filtered by `forwardFromBySource`, followed by FlowSpec replay, EOR, and `signalSessionReady` releasing the held live rail, yields an exact consistent table | Verified in `adj_rib_in/source_state.go:118` (`noteSourceProcessed` commits after withdrawals and installs on both typed and JSON/forked paths), `adj_rib_in/replay_sources.go:46` (`processedMsgID >= source.MessageID`), and `docs/architecture/forward-congestion-pool.md` ("The replay fence") | A second ordering defect would remain between replay pages and live forwards | Unit tests covering single-source withdrawal, cross-peer inversion (A-5), and GR-retained source, plus `rfc7606-relay-one-field.ci`, `forward-overflow-two-tier.ci`, and `llgr-readvertise-multipeer.ci` under stress | confirmed by source; runtime test owed |
| A-3 | `bgp-rs` can query `bgp-adj-rib-in`'s committed progress without walking or relaying routes, on both internal and forked transports | Verified in `ribevents/replay_sources.go:42` (`ReadReplaySourceProgress`) and `adj_rib_in/replay_sources.go:15` (`replaySourceProgressCommand`, wired into `replayRoutesCommand` at `replay_page.go:39`) | Would need a new RPC | Existing `adj_rib_in/replay_sources_test.go` (`TestReplaySourceProgressRequiresOriginalPhysicalIngest` on typed and full-json) and `rr/replay_store_lag_test.go` (`TestPeerUpReplayWaitsForOriginalReceiveStore` on internal and forked transports) | confirmed by source and existing tests |
| A-4 | Legacy-ingest routes with `MessageID == 0` and GR-retained routes from sources that went down before the new client came up remain replayable | Verified in `adj_rib_in/replay_cut.go:49` (`msgID != 0`), `rs/retained_membership.go:277` (GR-retained admission), and `forwardFromBySource` capturing `seenMsgID` for down GR-retained actors while `replaySources` waits only on live actors | Legacy or GR-retained routes would be skipped by the per-source replay filter | `TestReplayPreservesLegacyZeroAndGRRetainedRoutes` in `rs/replay_catchup_test.go` | confirmed by source; runtime test owed |
| A-5 | **BROKEN on HEAD (original assumption resolved):** Global `IngestedMsgID >= cut` does NOT prove `bgp-adj-rib-in` ingested every UPDATE at or below `cut`, because `reactor.nextMsgID()` is allocated concurrently across peer read loops (out-of-order arrival across peers) and `IngestedMsgID` is omitted when `bgp-adj-rib-in` runs forked; conversely, per-physical-ingress `(PeerOwner, ConnectionID)` read loops ARE serial, so per-source `sourceLive[owner].processedMsgID >= source.seenMsgID` IS sufficient on both transports | Verified in `reactor/reactor_notify.go:569`, `adj_rib_in/rib.go:438,603,914`, `adj_rib_in/source_state.go:17,118`, and `ribevents/replay_sources.go:15` | Global high-water wait lets Source B mask Source A's lag and skips waiting when forked; global `ForwardFrom` drops Source A's inverted UPDATE on the live rail | `TestReplayWaitsForLaggingSourceDespiteHigherSiblingMsgID` and `TestReplayPerSourceCutHandlesCrossPeerAllocationInversion` in `rs/replay_catchup_test.go` | broken (original global A-5) / confirmed (per-source replacement) |
| A-6 | A peer-up replay whose loaded `bgp-adj-rib-in` times out at 2 minutes or fails unrecoverably can reset ONLY its captured client session via `request peer <addr> teardown 4 ...` without tearing down a newer replacement session or same-address sibling | Verified in `plugin/server/dispatch_destination.go:25`, `reactor/destination_constraint.go:31` (`matchCapturedDestination`), and `reactor/peer.go:2380` (`teardownOf(target, notice, true)`) | Stale timeout could tear down a fresh reconnected session | `TestDestinationBoundTeardownDropsStaleSession` in `reactor/peer_close_test.go` and `TestPeerTeardownCarriesOriginalDestinationConstraint` in `cmd/peer/physical_control_test.go` | confirmed by source; runtime test owed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Waiting before the first walk adds peer-up latency for route-server clients | Peer-up convergence time in route-server functional/perf runs | Wait normally completes in zero extra polls (when `adj-rib-in` is already caught up, one fast projection call) or 1-2 20 ms polls; zero extra wait when `len(replaySources) == 0` |
| R-2 | Live rail stays held for up to 2 minutes if `bgp-adj-rib-in` is stuck | Replay fence held until 2-minute timeout, then Cease reset | Owner decision (2026-11-24): 2-minute bound prevents permanent wedge; automatic Cease (Event 8) increments `ConnectRetryCounter` so reconnects back off while `doctor-bgp-rs-lifecycle` alerts the operator |
| R-3 | Existing unit test `TestRSOptionalReadRefusalCompletesMandatoryReplayAndReconciles` (`rs/replay_read_test.go`) asserts that a loaded-store read error still delivers FlowSpec + EOR + `signalSessionReady`, which contradicts the 2026-11-24 owner ruling | Red on `TestRSOptionalReadRefusalCompletesMandatoryReplayAndReconciles` after Phase 3 | Update that test to separate (a) absent store (`ErrReplayStoreAbsent` / `unknown command`) completing FlowSpec + EOR + ready and (b) loaded-store read refusal triggering Cease teardown with no EOR/ready; route the test change through `ze-test-judge` (`ai/META-DECISIONS.md`, "Test changes") |
| R-4 | A source peer goes down while `waitReplaySources` is polling, so `adj_rib_in` deletes its `sourceLive[owner]` entry and `source-progress` would return `covered = false` forever if that source remained in `pending` | Timeout on peer-up when a source disconnects concurrently | Filter `pending` on every poll iteration to actors still live (`actor != nil && (actor.Up || !actor.StateSeen)`), matching `rr/replay_sources.go:55-60`; departed non-retained routes are already excluded by `rs.currentStoredRouteLocked` |
| R-5 | Existing `.ci` tests (`forward-overflow-two-tier.ci`, `rfc7606-relay-one-field.ci`, `llgr-readvertise-multipeer.ci`, `bgp-best-route-rs-lifetime.ci`) regress on replay ordering or absent-store fallback | Red on functional plugin tests | Keep absent-store fallback intact, test both internal and forked transports, and fix code rather than weakening tests |
| R-6 | A deterministic `.ci` cannot force `bgp-adj-rib-in` to lag on demand without adding a production lag seam | `.ci` passes even on buggy HEAD unless stressed | Unit tests in `rs/replay_catchup_test.go` carry the deterministic proof (including real `bgp-adj-rib-in` + `bgp-rs` over `pluginengine.Transports()` internal and forked); `.ci` tests run under `./le test stress-repro` as regression proof |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A route-server client's initial table (missing/stale/duplicate routes, or premature session reset on healthy peer-up), or operator `request peer teardown` if unbound callers are mishandled. Non-RS peers and peers already established are unaffected |
| How is it reverted? | Single commit revert; no config schema change, no persisted state change, no wire format change |
| Who else touches this path? | `plan/release/validation/spec-doctor-coverage-7-bgp-plugins.md` (AC-17 doctor check), `internal/component/bgp/plugins/rr/replay_sources.go` (sibling caller of `ReadReplaySourceProgress`, untouched) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Peer-up state event (`handleStateUp`) for an RS client while `bgp-adj-rib-in` trails a source withdrawal (including when a second source has a higher MessageID) | → | `handleStateUp` → `replayForPeer` → `waitReplaySources` (`ReadReplaySourceProgress`) → `RelayReplay` | `TestReplayNeverRelaysBeforeStoreCoversCut` and `TestReplayWaitsForLaggingSourceDespiteHigherSiblingMsgID` in `internal/component/bgp/plugins/rs/replay_catchup_test.go` |
| Peer-up state event where a loaded `bgp-adj-rib-in` stays behind for 2 minutes or fails a read unrecoverably | → | `replayForPeer` → `failReplay` + `warnLifecycle` + `resetFailedReplayPeer` → `DispatchDestinationCommand` (`ze-bgp:peer-teardown`) → `reactor.TeardownPeer` (`matchCapturedDestination` + `Peer.teardownOf`) | `TestReplayTimeoutResetsClientWithCease` and `TestReplayStoreErrorResetsClientWithCease` in `internal/component/bgp/plugins/rs/replay_catchup_test.go`; `TestPeerTeardownCarriesOriginalDestinationConstraint` in `internal/component/bgp/plugins/cmd/peer/physical_control_test.go`; `TestDestinationBoundTeardownDropsStaleSession` in `internal/component/bgp/reactor/peer_close_test.go` |
| Two route-server clients, one late, a route announced then withdrawn | → | daemon's `bgp-rs` and `bgp-adj-rib-in` | `test/plugin/rfc7606-relay-one-field.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An RS client comes up while Source A has announced prefix P (`msgID 10`) and withdrawn P (`msgID 12`), and `bgp-adj-rib-in` has committed only `msgID 10` (even if Source B has already committed `msgID 15`) | `bgp-rs` polls `ReadReplaySourceProgress` before calling `RelayReplay`, waits until `bgp-adj-rib-in` commits Source A `msgID 12`, then walks and relays the store's remaining routes (such as Source B's route) with P absent; zero routes are relayed before coverage is confirmed | Live routes still stored at or below each source's captured cut are relayed: they form the client's initial table |
| AC-2 | Cross-peer allocation inversion: Source B's `msgID 15` reaches `bgp-rs` before client C's `handleStateUp`, while Source A's `msgID 12` (allocated earlier) reaches `bgp-rs` after client C's `handleStateUp` | Client C's per-source cut records `{A: 10, B: 15}`; `selectForwardTargets` / `fastPathOwedTarget` forwards Source A's `msgID 12` on the live rail (held behind the replay fence), and the replay page filter skips Source A's `msgID 12` if `bgp-adj-rib-in` already ingested it, delivering `msgID 12` exactly once after replay | Routes from Source A with `msgID <= 10` and Source B with `msgID <= 15` stay on the replay rail because they were already seen before client C established |
| AC-3 | `bgp-adj-rib-in` is already caught up (or no live source has `seenMsgID > 0`), or `bgp-adj-rib-in` is not loaded (`ErrReplayStoreAbsent` / `unknown command`) | When caught up, `replayForPeer` relays stored routes immediately after one `source-progress` check (or zero checks when `replaySources` is empty); when not loaded, `bgp-rs` logs `adjRibInMissingOnce`, replays authorized FlowSpecs from `bgp-rib`, sends End-of-RIB, and signals session ready; legacy `MessageID == 0` routes and GR-retained routes from sources downed before peer-up remain replayed | One `source-progress` read when live sources exist (proves coverage); FlowSpec replay + EOR + ready when `bgp-adj-rib-in` is absent (`OptionalDependencies`); legacy/GR routes because neither is excluded by the live rail |
| AC-4 | A loaded `bgp-adj-rib-in` still has not covered all live captured sources when the 2-minute `peerReplayTimeout` expires (no early 5-second give-up) | `bgp-rs` keeps waiting past 5 seconds up to 2 minutes; at 2 minutes it sends neither End-of-RIB nor `signalSessionReady`, records `actor.replayFailure` and logs/raises `doctor-bgp-rs-lifecycle` naming the peer, cut, cause, and `ze show bgp rs status`, and dispatches destination-bound `request peer <addr> teardown 4 bgp-rs replay catch-up failed` so the reactor sends Cease / Administrative Reset (6/4) via AutomaticStop (Event 8) to only that captured session | Delay of up to 2 minutes while waiting for a slow store (slowness is permitted, silent route loss is not); unrelated peers and sessions remain untouched |
| AC-5 | A loaded `bgp-adj-rib-in` fails a `source-progress` read or `RelayReplay` page read unrecoverably (non-absent error while generation is current and plugin is not shutting down) | Same outcome as AC-4: no End-of-RIB, no `signalSessionReady`, `doctor-bgp-rs-lifecycle` failure recorded and logged naming the failed call and cause, and destination-bound Cease teardown sent for that captured session | Unrelated peers continue forwarding normally; if the plugin is shutting down (`stopping` / EOF / parent canceled), the replay exits cleanly without sending teardown |
| AC-6 | The client reconnects (`ReplayGen` advances) or goes down while its replay is waiting, or a stale destination-bound teardown arrives at the reactor after reconnect | The superseded replay goroutine exits sending no routes, no EOR, no readiness report, and no teardown; at the reactor, a destination-bound `TeardownPeer` whose `(DestinationOwner, DestinationSession)` no longer matches the current established session returns without closing the successor session or a same-address sibling, while unbound operator `request peer <sel> teardown` continues to tear down the named peer | The newer session's own replay owns the peer and delivers its complete table; operator teardown keeps its existing ManualStop contract |
| AC-7 | `rfc7606-relay-one-field.ci`, `forward-overflow-two-tier.ci`, `llgr-readvertise-multipeer.ci`, and `bgp-best-route-rs-lifetime.ci` under `./le test stress-repro`, plus `bgp-route-server-replacement-frr` interop | Every stress iteration and the FRR interop scenario pass: withdrawals precede later announcements, all prefixes arrive in order, no UPDATE is duplicated, and real internal/forked RS lifecycles converge cleanly | Existing test assertions as written; they verify end-to-end route-server convergence without weakening |
| AC-8 | Architecture docs `docs/architecture/plugin/plugin-system.md` ("Peer-up barrier"), `docs/architecture/plugin/rib-storage-design.md` ("Route-server optional replay read failure" and "Original source catch-up before cold membership"), and `docs/architecture/api/architecture.md` (owner-relayed peer-up replay) | Each page accurately states `bgp-rs`'s per-source cut, pre-walk `source-progress` wait (replacing "so its withdrawals never reach the peer"), and the 2-minute timeout / loaded-store failure Cease reset (preserving FlowSpec-only fallback only when `bgp-adj-rib-in` is absent), with valid source anchors passing `./le doc check all` | Surrounding sections on the peer-up barrier, session-ready reports (2026-09-18 ruling), and `bgp-rr` behavior remain unchanged |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator's route-server client reconnects while other clients are withdrawing and announcing routes concurrently (with `bgp-adj-rib-in` running internal or forked) | `handleStateUp` → capture per-source cuts & live sources → `waitReplaySources` (`ReadReplaySourceProgress`) → `RelayReplay` (per-source filtered) → `replayFlowSpecs` → `sendEOR` → `signalSessionReady` → held live rail | `TestReplayNeverRelaysBeforeStoreCoversCut`, `TestReplayWaitsForLaggingSourceDespiteHigherSiblingMsgID`, `TestReplayPerSourceCutHandlesCrossPeerAllocationInversion`, `rfc7606-relay-one-field.ci` |
| 2 | A client connects while `bgp-adj-rib-in` is wedged past 2 minutes or returns an unrecoverable store error | `handleStateUp` → wait up to 2 minutes (not 5 s) → `failReplay` + `warnLifecycle` (`doctor-bgp-rs-lifecycle`) → `DispatchDestinationCommand` (`request peer <addr> teardown 4 ...`) → `matchCapturedDestination` → `Peer.teardownOf(session, notice, true)` (Cease 6/4, Event 8) → client reconnects for a fresh complete table | `TestReplayTimeoutResetsClientWithCease`, `TestReplayStoreErrorResetsClientWithCease`, `TestDestinationBoundTeardownDropsStaleSession` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReplayNeverRelaysBeforeStoreCoversCut` | `internal/component/bgp/plugins/rs/replay_catchup_test.go` | AC-1: lagging store returns `covered: false` while P is still present, then commits withdrawal and returns `covered: true`; relay hook never sees P and receives remaining routes | |
| `TestReplayWaitsForLaggingSourceDespiteHigherSiblingMsgID` | `internal/component/bgp/plugins/rs/replay_catchup_test.go` | AC-1 / A-5: Source B has committed `msgID 15` while Source A is at `10` (cut `12`), on real `bgp-adj-rib-in` + `bgp-rs` over both internal and forked transports (`pluginengine.Transports()`); replay waits until Source A commits `12` | |
| `TestReplayPerSourceCutHandlesCrossPeerAllocationInversion` | `internal/component/bgp/plugins/rs/replay_catchup_test.go` | AC-2: Source B `15` precedes peer-up, Source A `12` arrives after peer-up; live rail forwards `12` while replay filters out `12` (unicast and FlowSpec), delivering `12` once | |
| `TestReplayPreservesLegacyZeroAndGRRetainedRoutes` | `internal/component/bgp/plugins/rs/replay_catchup_test.go` | AC-3 / A-4: `msgID == 0` routes and GR-retained routes from already-down sources are replayed; absent `bgp-adj-rib-in` (`ErrReplayStoreAbsent`) completes FlowSpec + EOR + ready; source disconnecting mid-wait unblocks `waitReplaySources` | |
| `TestReplayTimeoutResetsClientWithCease` | `internal/component/bgp/plugins/rs/replay_catchup_test.go` | AC-4: fake clock advances past 5 s (still waiting, no reset), then past 2 minutes (no EOR, no `signalSessionReady`, `doctor-bgp-rs-lifecycle` recorded, destination-bound `request peer <addr> teardown 4 ...` dispatched) | |
| `TestReplayStoreErrorResetsClientWithCease` | `internal/component/bgp/plugins/rs/replay_catchup_test.go` | AC-5: loaded store returns unrecoverable error during `source-progress` or `RelayReplay`; no EOR/ready, `doctor-bgp-rs-lifecycle` recorded, destination-bound Cease teardown dispatched; shutdown error sends no teardown | |
| `TestReplayStaleGenerationSendsNothing` | `internal/component/bgp/plugins/rs/replay_catchup_test.go` | AC-6: peer reconnects during wait; superseded replay exits sending no routes, no EOR, no ready, and no teardown | |
| `TestRSOptionalReadRefusalCompletesMandatoryReplayAndReconciles` (updated via `ze-test-judge`) | `internal/component/bgp/plugins/rs/replay_read_test.go` | AC-3 / AC-5: verifies absent store (`ErrReplayStoreAbsent` / `unknown command`) completes FlowSpec + EOR + ready while loaded-store error withholds EOR/ready and triggers destination-bound Cease reset | |
| `TestPeerTeardownCarriesOriginalDestinationConstraint` | `internal/component/bgp/plugins/cmd/peer/physical_control_test.go` | AC-6: `ze-bgp:peer-teardown` declares `AcceptsDestinationSession: true` and passes bound `ctx.Sender` to `ReactorLifecycle.TeardownPeer`, while unbound operator calls pass zero destination | |
| `TestDestinationBoundTeardownDropsStaleSession` | `internal/component/bgp/reactor/peer_close_test.go` | AC-6: bound `TeardownPeer` sends Cease (6/4) via AutomaticStop (`ConnectRetryCounter` incremented) to the matching `(owner, session)`, does not touch a reconnected successor session or same-address sibling, and keeps unbound operator teardown unchanged | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Source `processedMsgID` vs captured `seenMsgID` | 0 upward | `processedMsgID == seenMsgID` is covered | `processedMsgID == seenMsgID - 1` is not covered | N/A (`>` is covered) |
| Per-source cut (`forwardFromFor(source)`) | 0 upward | `0` is a real cut (every `msgID > 0` belongs to live rail; `msgID == 0` stays replayable) | N/A (unsigned) | N/A |
| Catch-up wait elapsed vs `peerReplayTimeout` (2 m) | 0 to 2 m | Elapsed `<= 2m` (including 5 s, 1m59s) keeps waiting | N/A | Elapsed `> 2m` triggers `failReplay` + Cease reset |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc7606-relay-one-field` | `test/plugin/rfc7606-relay-one-field.ci` | Late RS client receives withdrawal ahead of announcement without retaining stale route | existing; stress-repro regression |
| `forward-overflow-two-tier` | `test/plugin/forward-overflow-two-tier.ci` | Late RS client receives every route in order | existing; stress-repro regression |
| `llgr-readvertise-multipeer` | `test/plugin/llgr-readvertise-multipeer.ci` | No UPDATE reaches an RS client twice | existing; stress-repro regression |
| `bgp-best-route-rs-lifetime` | `test/plugin/bgp-best-route-rs-lifetime.ci` | Internal and forked RS lifecycles preserve exact routes and withdrawals across reconnects | existing; regression |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-route-server-replacement-frr` | `test/interop/scenarios/` | FRR | A route-server client run by FRR receives replacements and withdrawals cleanly after the per-source cut and pre-walk wait change | existing; run owed by main thread |

## Files to Modify
- `internal/component/bgp/plugins/rs/peer.go` - add `seenMsgID`, `forwardFromBySource`, `replaySources`, and `forwardFromFor` on `PeerState`
- `internal/component/bgp/plugins/rs/server.go` - replace `replayCatchUpBudget = 5 * time.Second` with `peerReplayTimeout = 2 * time.Minute`; update `actor.seenMsgID` under `rs.mu` in `dispatchStructured` and `dispatchText`
- `internal/component/bgp/plugins/rs/server_forward.go` - switch `selectForwardTargets` and `fastPathOwedTarget` from scalar `peer.ForwardFrom` to `peer.forwardFromFor(source)`
- `internal/component/bgp/plugins/rs/server_handlers.go` - capture `forwardFromBySource` and `replaySources` in `handleStateUp`; add `waitReplaySources`, per-source page filter, and `resetFailedReplayPeer`; remove old walk-first `coversCut` / `deltaReplay` / 5 s budget loop
- `internal/component/bgp/plugins/rs/server_validation.go` - apply per-source cut in `replayFlowSpecs`
- `internal/component/bgp/plugins/rs/replay_read.go` - restrict FlowSpec-only fallback in `finishOptionalReplayRead` to absent store (`ErrReplayStoreAbsent` / `unknown command`); route loaded-store failures to `failReplay` + `warnLifecycle` + Cease reset
- `internal/component/bgp/plugins/rs/source_lifetime.go` - pass captured per-source cut and `replaySources` in `retryCurrentReplays`
- `internal/component/bgp/plugins/cmd/peer/peer.go` - set `AcceptsDestinationSession: true` on `ze-bgp:peer-teardown` and pass `ctx.Sender` to `TeardownPeer`
- `internal/component/plugin/types_bgp.go` - carry `plugin.Sender` on `ReactorLifecycle.TeardownPeer` (updating mock implementations in `internal/component/plugin/coordinator.go`, `internal/component/plugin/coordinator_test.go`, `internal/component/plugin/server/mock_reactor_test.go`, `internal/component/bgp/plugins/cmd/peer/notify.go`, `internal/component/bgp/plugins/cmd/peer/mock_reactor_test.go`)
- `internal/component/plugin/coordinator.go` - forward `sender` through `Coordinator.TeardownPeer`
- `internal/component/bgp/reactor/reactor_api.go` - when `sender.Destination()` is nonzero in `TeardownPeer`, resolve via `matchCapturedDestination` and call `peer.teardownOf(session, notice, true)` (AutomaticStop, session-bound); preserve existing `peer.TeardownNotification` for unbound operator calls
- `internal/component/bgp/plugins/rs/replay_catchup_test.go` - add the unit tests covering AC-1..AC-6
- `internal/component/bgp/plugins/rs/replay_read_test.go` - update `TestRSOptionalReadRefusalCompletesMandatoryReplayAndReconciles` (with `ze-test-judge` approval) to match the 2026-11-24 owner ruling
- `internal/component/bgp/plugins/cmd/peer/physical_control_test.go` - add `TestPeerTeardownCarriesOriginalDestinationConstraint`
- `internal/component/bgp/reactor/peer_close_test.go` - add `TestDestinationBoundTeardownDropsStaleSession`
- `docs/architecture/plugin/plugin-system.md` - AC-8 ("Peer-up barrier")
- `docs/architecture/plugin/rib-storage-design.md` - AC-8 ("Route-server optional replay read failure" and "Original source catch-up before cold membership")
- `docs/architecture/api/architecture.md` - AC-8 (owner-relayed peer-up replay order and Cease reset)

## Files to Create
- None

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Reuses existing `request bgp adj-rib-in replay-routes` (`source-progress`) and `request peer <sel> teardown` |
| YANG validation constraints | N-A | No config or YANG change |
| YANG custom validators | N-A | No config or YANG change |
| CLI commands/flags | No | Existing `request peer <sel> teardown` operator syntax is unchanged |
| CLI grammar (keyword before value) | N-A | No CLI command added or changed |
| Editor autocomplete | N-A | No YANG change |
| Functional test for new RPC/API | N-A | No new RPC/API |
| Pipe completeness | N-A | No command output shape changed |
| Env var registration | No | `peerReplayTimeout` (2 minutes) stays a constant matching `bgp-rr` |
| Doctor check for runtime dependencies | No | No new runtime dependency |
| Doctor code per problem class | Yes | Reuses the already-registered `doctor-bgp-rs-lifecycle` (`internal/component/bgp/plugins/rs/lifecycle_diagnostic.go`, runtime check `bgp-rs-lifecycle` + transient SDK report queue) for catch-up timeout and loaded-store replay read failures |
| Prometheus counters/metrics | No | None added |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Defect fix |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | - |
| 4 | API/RPC added/changed? | No | Existing `replay-routes source-progress` and `request peer teardown` wire contracts stay unchanged |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | - |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | Reuses existing `DispatchDestinationCommand` and `ReplayRoutesCommand` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4271.md` Section 9.2 / Section 6.7 if a new unit test carries an RFC tag (and `./le rfc record` / `./le rfc check` if tagged) |
| 10 | Test infrastructure changed? | No | - |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/plugin-system.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/api/architecture.md` (AC-8) |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors spec plan/release/fixes/spec-bgp-rs-replay-reaches-the-cut.md`: `docs/architecture/plugin/plugin-system.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/api/architecture.md` updated (AC-8); `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/core-design.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/commands.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/rib-transition.md`, `docs/architecture/behavior/fsm.md`, `docs/architecture/config/syntax.md`, `docs/architecture/api/text-format.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/edge-cases/extended-message.md`, `docs/architecture/wire/nlri.md`, `docs/architecture/api/text-parser.md`, `docs/architecture/buffer-architecture.md` verified unaffected (see `## Design documents`) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | Existing examples remain valid |

## Design presentation

### What changes

Route-server (`bgp-rs`) peer-up replay now guarantees that a newly connected client never receives a stale route that was already withdrawn before the cut, never loses a route across concurrent source peers or when `bgp-adj-rib-in` runs forked, and never gets reported complete when a loaded `bgp-adj-rib-in` fails to catch up. Specifically, `bgp-rs` replaces its global scalar cut and walk-first 5-second global high-water-mark loop with per-source causal cuts (`PeerState.seenMsgID` → `forwardFromBySource`) and pre-walk per-source catch-up using the existing `ribevents.ReadReplaySourceProgress` projection (`adj_rib_in.replaySourceProgressCommand`, already shared with `bgp-rr` on both internal and forked transports). Per Thomas's 2026-11-24 decision, catch-up waits up to the 2-minute replay timeout (`peerReplayTimeout`); if a loaded `bgp-adj-rib-in` still has not reached the cut or fails unrecoverably, `bgp-rs` withholds End-of-RIB and readiness, records the failure under `doctor-bgp-rs-lifecycle`, and resets only that client's captured BGP session with a Cease / Administrative Reset (6/4) NOTIFICATION (AutomaticStop Event 8) via destination-bound `request peer <addr> teardown` so the client reconnects and receives a complete table. When `bgp-adj-rib-in` is not loaded (`ErrReplayStoreAbsent`), the existing FlowSpec-only optional-dependency fallback stays intact.

### Before and after

1. **Late client connects while Source A withdraws prefix P (`msgID 12`) and Source B announces Q (`msgID 15`), with `bgp-adj-rib-in` lagging on Source A**
   - **Before:** `replayForPeer` walks `bgp-adj-rib-in` immediately before waiting, relaying P; then `IngestedMsgID == 15 >= cut 15` (or omitted when forked) falsely satisfies `coversCut(15)`. Nothing retracts P, so the client keeps stale P until its next session.
   - **After:** `handleStateUp` captures per-source bounds `{A: 12, B: 15}`. `waitReplaySources` polls `ReadReplaySourceProgress` before walking; `bgp-adj-rib-in` answers `covered: false` until Source A's `msgID 12` commits (removing P). Only then does `RelayReplay` walk and relay Q without P, on both internal and forked transports.

2. **Cross-peer allocation inversion: Source B's `msgID 15` reaches `bgp-rs` before client C's peer-up, while Source A's `msgID 12` reaches `bgp-rs` after peer-up**
   - **Before:** Client C's global scalar `ForwardFrom = 15` causes `selectForwardTargets` to drop Source A's `msgID 12` on the live rail, while replay is not guaranteed to wait for `12` — causing silent route/withdrawal loss.
   - **After:** Client C's per-source cut is `{A: 10, B: 15}`. Source A's `msgID 12 > 10` is forwarded on the live rail (held behind the replay fence) and filtered out of replay pages/FlowSpecs if already stored, so client C receives `msgID 12` exactly once in order.

3. **Loaded `bgp-adj-rib-in` trails past 5 s / 2 m, or fails a replay read unrecoverably**
   - **Before:** At 5 s `replayForPeer` logs a warning, calls `failReplay`, and returns without resetting the peer — leaving the peer's live-forward replay fence wedged. On a store read error, `finishOptionalReplayRead` sends End-of-RIB and `signalSessionReady`, telling the peer its table is complete with routes missing.
   - **After:** `bgp-rs` keeps waiting past 5 s up to the 2-minute replay timeout. If `bgp-adj-rib-in` still has not caught up at 2 minutes, or returns an unrecoverable error while loaded, `bgp-rs` sends no EOR and no readiness report, records `doctor-bgp-rs-lifecycle`, and sends destination-bound `request peer <addr> teardown 4 bgp-rs replay catch-up failed` (Cease 6/4, Event 8 AutomaticStop) against only that captured session so the peer reconnects cleanly for a complete table.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep walk-first replay and retract withdrawn routes afterwards | Requires tracking every relayed key per peer and synthesizing withdrawals from `bgp-rs`, duplicating store state; waiting before the first walk reuses existing `ReadReplaySourceProgress` with zero extra state |
| Keep global `IngestedMsgID` high-water mark and only move the wait earlier | Fails A-5 across multiple concurrent sources (Source B's higher MessageID masks Source A's lag) and fails completely when `bgp-adj-rib-in` runs forked (`ingestTracked == false`) |
| Keep 5-second `replayCatchUpBudget` and report complete with doctor warning only | Violates the 2026-11-24 owner decision and `ai/rules/principles.md`: knowingly leaves the peer with stale or missing routes |
| Wait indefinitely past 2 minutes without resetting the session | Stalls the client's live-forward replay fence forever on a stuck receive store |
| Hold the engine's initial-sync End-of-RIB until replay completes | Ruled out by the owner ruling of 2026-09-18 in `docs/architecture/plugin/plugin-system.md` ("Session-ready reports") |
| Send an unbound address-only `request peer <addr> teardown` | A slow timeout could race with a rapid reconnect and tear down the client's newer replacement session or a same-address sibling; binding to `(DestinationOwner, DestinationSession)` via `AcceptsDestinationSession: true` + `matchCapturedDestination` + `Peer.teardownOf` eliminates that race |

### Documents departed from

- `docs/architecture/plugin/rib-storage-design.md`, section `Route-server optional replay read failure`: currently states *"A failed optional receive-store page read does not suppress the selecting RIB's mandatory FlowSpec phase or the original destination's End-of-RIB/readiness report."* Per the 2026-11-24 owner decision, after this work the page states that only an *absent* (`ErrReplayStoreAbsent` / `unknown command`) receive store takes the FlowSpec + EOR + readiness fallback, while a loaded receive store that times out at 2 minutes or fails a read unrecoverably withholds EOR/readiness, records `doctor-bgp-rs-lifecycle`, and resets the captured client session with a Cease NOTIFICATION.
- `docs/architecture/plugin/plugin-system.md`, section `Peer-up barrier`: currently states *"An UPDATE taken delivery of before that lands at or below the peer's cut and belongs to the announce-only Adj-RIB-In replay, so its withdrawals never reach the peer."* After this work, the paragraph states that `bgp-rs` captures per-source cuts and waits via `ReadReplaySourceProgress` before walking `bgp-adj-rib-in`, so every announcement and withdrawal at or below each source's cut is applied in the store before the replay begins, and an unrecoverable failure or 2-minute timeout resets the client session with Cease.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write failing wiring tests for pre-walk per-source catch-up, cross-peer A-5 lag/inversion, 2-minute Cease timeout, loaded-store read error Cease reset, and destination-bound teardown
   - Tests: `TestReplayNeverRelaysBeforeStoreCoversCut`, `TestReplayWaitsForLaggingSourceDespiteHigherSiblingMsgID`, `TestReplayPerSourceCutHandlesCrossPeerAllocationInversion`, `TestReplayTimeoutResetsClientWithCease`, `TestReplayStoreErrorResetsClientWithCease`, `TestPeerTeardownCarriesOriginalDestinationConstraint`, `TestDestinationBoundTeardownDropsStaleSession`
   - Files: `internal/component/bgp/plugins/rs/replay_catchup_test.go`, `internal/component/bgp/plugins/cmd/peer/physical_control_test.go`, `internal/component/bgp/reactor/peer_close_test.go`
   - Verify: new tests fail on HEAD for the exact defects each names (validating A-1, A-5, A-6)
2. **Phase: Destination-bound automatic Cease teardown** -- wire `AcceptsDestinationSession: true` on `ze-bgp:peer-teardown` and bound `matchCapturedDestination` + `Peer.teardownOf(session, notice, true)` in `reactor`
   - Tests: `TestPeerTeardownCarriesOriginalDestinationConstraint`, `TestDestinationBoundTeardownDropsStaleSession`, plus existing `cmd/peer`, `plugin`, and `reactor` teardown/notify tests
   - Files: `internal/component/plugin/types_bgp.go`, `internal/component/plugin/coordinator.go`, `internal/component/bgp/plugins/cmd/peer/peer.go`, `internal/component/bgp/plugins/cmd/peer/notify.go`, `internal/component/bgp/reactor/reactor_api.go` (plus mock signatures in `coordinator_test.go`, `server/mock_reactor_test.go`, `cmd/peer/mock_reactor_test.go`)
   - Verify: destination-bound teardown tests pass; unbound operator teardown and notify tests stay green
3. **Phase: Per-source cut and pre-walk `ReadReplaySourceProgress` wait in `bgp-rs` (Defects 1 and 2)** -- add `seenMsgID`, `forwardFromBySource`, `replaySources`, `waitReplaySources`, and per-source filtering on both live rail and replay/FlowSpec; delete obsolete walk-first `coversCut` / `deltaReplay` / 5 s budget code
   - Tests: `TestReplayNeverRelaysBeforeStoreCoversCut`, `TestReplayWaitsForLaggingSourceDespiteHigherSiblingMsgID`, `TestReplayPerSourceCutHandlesCrossPeerAllocationInversion`, `TestReplayPreservesLegacyZeroAndGRRetainedRoutes`, `TestReplayStaleGenerationSendsNothing`
   - Files: `internal/component/bgp/plugins/rs/peer.go`, `server.go`, `server_forward.go`, `server_handlers.go`, `server_validation.go`, `source_lifetime.go`
   - Verify: all catch-up, cross-peer A-5 (internal & forked), inversion, legacy/GR, and stale-generation unit tests pass
4. **Phase: 2-minute timeout and loaded-store failure Cease reset (Defect 3)** -- enforce 2-minute `peerReplayTimeout`, restrict optional fallback to `ErrReplayStoreAbsent` / `unknown command`, trigger `failReplay` + `warnLifecycle` (`doctor-bgp-rs-lifecycle`) + `resetFailedReplayPeer` on timeout or loaded-store read failure, and update `TestRSOptionalReadRefusalCompletesMandatoryReplayAndReconciles` with `ze-test-judge` approval
   - Tests: `TestReplayTimeoutResetsClientWithCease`, `TestReplayStoreErrorResetsClientWithCease`, `TestRSOptionalReadRefusalCompletesMandatoryReplayAndReconciles`
   - Files: `internal/component/bgp/plugins/rs/server_handlers.go`, `replay_read.go`, `replay_read_test.go`
   - Verify: all `internal/component/bgp/plugins/rs/` unit tests pass under `-race`
5. **Phase: Architecture docs and end-to-end regression** -- update `plugin-system.md`, `rib-storage-design.md`, and `architecture.md` (AC-8); run doc anchor checks, functional stress-repro (AC-7), and FRR interop scenario
   - Tests: `./le doc check all`, AC-7 stress-repro runs, `bgp-route-server-replacement-frr` interop
   - Files: `docs/architecture/plugin/plugin-system.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/api/architecture.md`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-8 has an implementation at file:line and a passing test |
| Feature completeness | Both user stories work end to end on both internal and forked transports |
| Correctness | Zero routes are relayed before `ReadReplaySourceProgress` confirms every live captured source; per-source cut is enforced identically on live forwarding (`selectForwardTargets`, `fastPathOwedTarget`), replay pages, and `replayFlowSpecs`; departed sources are dropped from `pending` during wait; GR-retained and `msgID == 0` routes remain replayable |
| Correctness | A loaded-store 2-minute timeout or unrecoverable read failure sends neither EOR nor `signalSessionReady`, resets only the captured `(DestinationOwner, DestinationSession)` via AutomaticStop Cease 6/4, and never resets a successor session or an unrelated peer; plugin shutdown sends no teardown |
| Data flow | `bgp-rs` reaches `bgp-adj-rib-in` only via `ReplayRoutesCommand` and resets peers only via `DispatchDestinationCommand`; no cross-plugin Go imports (`./le plugin boundary check`, `./le arch tier check`) |
| Operator errors and doctor codes | Timeout and read-failure warnings use `rs.warnLifecycle` with `doctor-bgp-rs-lifecycle`, naming the peer, cut, cause, and `ze show bgp rs status` |
| Rule: `ai/rules/no-layering.md` | The walk-first `coversCut` / `deltaReplay` / 5 s `replayCatchUpBudget` code is deleted, not retained as a fallback |
| Rule: `ai/rules/stale-comments.md` | Comments in `peer.go`, `server.go`, `server_handlers.go`, `replay_read.go`, and `ribevents/replay.go` referencing `coversCut` or the old walk-first order are updated |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Unit tests in `rs`, `cmd/peer`, `reactor`, `plugin` | Main thread runs `./le test unit pkg path ./internal/component/bgp/plugins/rs/ path ./internal/component/bgp/plugins/cmd/peer/ path ./internal/component/bgp/reactor/ path ./internal/component/plugin/` |
| Regression under stress (AC-7) | Main thread runs `./le test stress-repro run suite plugin test rfc7606-relay-one-field any-failure`, and the same for `forward-overflow-two-tier`, `llgr-readvertise-multipeer`, and `bgp-best-route-rs-lifetime` |
| FRR route-server interop (AC-7) | Main thread runs `./le test integration interop` selecting `bgp-route-server-replacement-frr` |
| Boundary, tier, and doc gates (AC-8) | Main thread runs `./le plugin boundary check`, `./le arch tier check`, `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Resource exhaustion | A peer flapping while `bgp-adj-rib-in` lags must not accumulate waiting replay goroutines (`ReplayGen` check ends superseded goroutines, and `replays.stop()` cancels all on shutdown); AutomaticStop (Event 8) increments `ConnectRetryCounter` so a stuck store backs off reconnects rather than tight-looping |
| Authorization that could fail open | Destination-bound teardown requires a valid nonzero `(DestinationOwner, DestinationSession)` validated by `DispatchDestinationCommand` and `matchCapturedDestination`; partial/zero/stale tuples are refused and never fall back to address-wide teardown |
| Silent route-table corruption | A loaded `bgp-adj-rib-in` failure or 2-minute timeout never lowers the replay fence or emits End-of-RIB with an incomplete table |

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

- Two fixes of 2026-07-26 closed the duplicate UPDATE that `llgr-readvertise-multipeer.ci` recorded: `a588ac5277` (a cut of 0 read as no cut) and `e4ee32b034` (the replay index read as inclusive, so the boundary route was relayed on every delta pass).
- `81dd09dfa8` (2026-07-27) added the catch-up wait and put it after the first walk, leaving Defect 1 open and relying on a global high-water mark (`IngestedMsgID`) that is unsound across concurrent sources and absent on forked `bgp-adj-rib-in`.
- By 2026-11-25, HEAD had already landed per-source committed progress (`ribevents.ReadReplaySourceProgress` + `adj_rib_in.replaySourceProgressCommand` over `sourceLive.processedMsgID` on both typed and JSON/forked paths) for `bgp-rr`, plus `Peer.teardownOf`, `matchCapturedDestination`, and `doctor-bgp-rs-lifecycle`. Extending those existing mechanisms to `bgp-rs` resolves A-3, A-5, and the 2026-11-24 Cease-reset decision with zero wire changes.
- Reference daemons (FRR `bgp_announce_table` and BIRD `rt_feed_channel`) are single-process and walk their in-memory route table synchronously on peer-up, so neither has an asynchronous external receive store to trail; Ze's multi-process plugin architecture makes pre-walk per-source synchronization and a bounded Cease reset necessary.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `suppression-decision-not-deterministic.md` | 2026-11-14 | bgp-rs `replayForPeer` walks the store before it covers the cut, so a withdrawn route is replayed |
| `transient-failure-treated-as-fatal.md` | 2026-11-14 | bgp-rs `replayForPeer` gives up after `replayCatchUpBudget` and reports the peer complete |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Pre-walk per-source catch-up via `ribevents.ReadReplaySourceProgress` and per-source cuts (`forwardFromBySource`) | Keep global `IngestedMsgID` + global `ForwardFrom`; or walk first and synthesize retractions afterwards | Global `IngestedMsgID` fails A-5 across concurrent peers and is absent when `bgp-adj-rib-in` runs forked; global `ForwardFrom` drops cross-peer inverted UPDATEs on the live rail. Per-source `ReadReplaySourceProgress` already exists in HEAD for `bgp-rr` on both transports, requires zero wire changes, and closes Defects 1 and 2 together |
| Wait up to 2-minute `peerReplayTimeout`, then reset the client session with destination-bound Cease / Administrative Reset (6/4) via AutomaticStop (Event 8) on timeout or loaded-store read failure (Owner decision 2026-11-24) | Give up at 5 s without reset (wedging replay fence); complete with EOR+ready and doctor warning only; wait indefinitely; or send unbound address-only teardown | Implements Thomas's 2026-11-24 ruling directly: avoids silent stale/missing routes and permanent fence wedges, backs off reconnects via Event 8 on a stuck store, and binds teardown to `(DestinationOwner, DestinationSession)` so a stale timeout never drops a successor session |
| Preserve FlowSpec-only + EOR + `signalSessionReady` fallback only when `bgp-adj-rib-in` is not loaded (`ErrReplayStoreAbsent` / `unknown command`) | Reset client even when `bgp-adj-rib-in` is intentionally omitted | `bgp-adj-rib-in` is in `OptionalDependencies`; when intentionally absent, FlowSpec-only RS setups must continue working cleanly |
| Reuse existing `doctor-bgp-rs-lifecycle` (`lifecycle_diagnostic.go`) for timeout and read-failure reporting | Register a second route-server replay doctor code | `doctor-bgp-rs-lifecycle` already reads `actor.replayFailure` and raises bounded transient error events on both transports |
| Keep the engine's initial-sync End-of-RIB unheld and leave `bgp-rr` unchanged | Hold End-of-RIB for `signalSessionReady`; modify `bgp-rr` | 2026-09-18 owner ruling governs End-of-RIB; `bgp-rr` already uses pre-walk `ReadReplaySourceProgress` on HEAD |

## Known Limitations

- A deterministic `.ci` test for `bgp-adj-rib-in` lag requires a production lag-injection seam, which this spec deliberately does not add; unit tests over both real `pluginengine.Transports()` (internal and forked) carry the deterministic proof while `.ci` stress-repro runs provide end-to-end regression coverage.
- `bgp-rr`'s own optional-store failure policy (currently logging and completing FlowSpec + EOR + ready when `ReadReplaySourceProgress` or `RelayReplay` fails, `rr/rr.go`) is separate from `bgp-rs` and is not changed by this spec.

## RFC Documentation (Scope: protocol)

Add `// RFC 4271 Section 9.2: "All newly installed routes and all newly unfeasible routes for which there is no replacement route SHALL be advertised to its peers by means of an UPDATE message."` above the pre-walk `waitReplaySources` guard in `internal/component/bgp/plugins/rs/server_handlers.go`, and `// RFC 4271 Section 6.7` / `// RFC 4486` / `// RFC 9003 Section 2` comments on `resetFailedReplayPeer` and the bound automatic Cease teardown path.

| RFC / section | Default configuration and required behavior | Explicit options / extensions to preserve | Tests of default behavior | Tests preserving options / extensions |
|---------------|---------------------------------------------|-------------------------------------------|---------------------------|---------------------------------------|
| RFC 4271 Section 9.2 | A late route-server client receives only feasible routes still valid at its per-source cut, followed by all later announcements and withdrawals in order; routes withdrawn before the cut are never relayed | Optional-dependency deployment without `bgp-adj-rib-in` (FlowSpec-only fallback) and legacy `MessageID == 0` / GR-retained routes remain preserved | `TestReplayNeverRelaysBeforeStoreCoversCut`, `TestReplayWaitsForLaggingSourceDespiteHigherSiblingMsgID`, `TestReplayPerSourceCutHandlesCrossPeerAllocationInversion`, `rfc7606-relay-one-field.ci` | `TestReplayPreservesLegacyZeroAndGRRetainedRoutes` |
| RFC 4271 Sections 6.7 & 8.2.2; RFC 4486; RFC 9003 Section 2 | When a loaded `bgp-adj-rib-in` fails to cover the cut within 2 minutes or fails a replay read unrecoverably, `bgp-rs` closes only that captured session with Cease / Administrative Reset (6/4) + Shutdown Communication via Event 8 AutomaticStop so the client reconnects cleanly | Operator unbound `request peer <sel> teardown <subcode> [message]` keeps its existing ManualStop (Event 2) contract | `TestReplayTimeoutResetsClientWithCease`, `TestReplayStoreErrorResetsClientWithCease`, `TestDestinationBoundTeardownDropsStaleSession` | `TestPeerTeardownCarriesOriginalDestinationConstraint`, existing `cmd/peer/notify_test.go` (`TestHandlerTeardownStaysCease`) |
| RFC 7947 Sections 2.1–2.3 | Route-server replay delivers unmodified client routes admitted by policy without leaking withdrawn or import-filtered routes | Existing RS transparency (`bgp-rs:transparency`), per-client export chains, and ADD-PATH send remain untouched | `bgp-route-server-replacement-frr`, `bgp-best-route-rs-lifetime.ci` | Existing `rs` transparency and policy tests |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions and Permitted Survivors and Why
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks every row Yes / No / N-A with reasons

### Goal Gates (MUST pass)
- [ ] AC-1..AC-8 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches; GitHub CI verifies the pushed commit
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
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-bgp-rs-replay-reaches-the-cut.md` only, in the same `./le commit create` script
