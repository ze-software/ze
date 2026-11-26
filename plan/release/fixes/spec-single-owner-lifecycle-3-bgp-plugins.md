# Spec: single-owner-lifecycle-3-bgp-plugins

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Priority | P1 |
| Release reason | Three shipped-daemon defects in the BGP plugins: an unversioned validation verdict installs a later announcement without validating it, a forked `bgp-rib` cannot be stopped while it waits on the engine, and RFC 4724 route retention has no proof and one ordering hole when `bgp-gr` runs forked. Each is wrong behaviour of a capability Ze already ships. |
| RFC gaps | RFC4724-4.2-3 |
| Depends | `plan/release/fixes/spec-single-owner-lifecycle.md` (parent: the owner's decisions and the row routing; no code dependency). Coordinate, same files: `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` and `plan/release/fixes/spec-daemon-drain-before-stop.md` both edit `internal/component/plugin/process` |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 3 of `plan/release/fixes/spec-single-owner-lifecycle.md`, which holds the
owner's decisions (D1: no annotation gate, each fix closes its bypass at a
choke point) and routes every journal row to a child. This child owns the rows
whose producer is a BGP plugin.

The parent's principle: every lifecycle here already has an identity, and each
defect is a path that bypasses it. This child closes three bypasses.

| # | Lifecycle | Identity that exists | The bypass |
|---|-----------|----------------------|------------|
| 1 | One received UPDATE generation of a route in `bgp-adj-rib-in` | `RawRoute.MsgID`, the reactor's message id, never zero for a received UPDATE | A validation decision carrying msgID 0 names no generation, passes the only fence, and when no route is present it is buffered and applied to whatever route next arrives for that key |
| 2 | One run of a plugin that installs routes when forked (`bgp-rib`, `ospf`, `isis`, `static`, `connected`) | The context its `Run` is given | The forked route-install Sink of all five, and the `bgp-rib` next-hop metric lookups (`RIBManager.igpDistance` and `remoteRIB.prospectiveDistance`), call the engine on a context nothing cancels |
| 3 | One peer-down, decided by `bgp-gr` and acted on by `bgp-rib` | The ordered, acknowledged peer-state delivery | A forked `bgp-gr` that misses the delivery timeout is skipped with a warning and `bgp-rib` releases the routes without its verdict; nothing proves the retained count on any path |

Goals:

1. A validation decision with msgID 0 applies only to a route present when it
   arrives. With none present it is not buffered, and its sender is told.
   A decision that carries a msgID still waits for its UPDATE, and every
   entry can carry one while preserving the source-provenance contract:
   `accept-routes` and `reject-routes` gain the optional trailing
   `[<msgID> [<peerOwner> <connectionID>]]` arguments the nine-field batch
   form carries, so a validator answering an owned UPDATE event ahead of route
   ingestion passes its exact `(msgID, peerOwner, connectionID)` tuple and
   loses nothing (an address-only `<msgID>` without owner and connection stays
   observational and never matches an owned peer session). The form without a
   msgID is for an operator acting on a present route at the CLI.
2. A forked plugin that installs routes stops when its run context ends,
   whatever engine call it is waiting in. One rule for every caller of
   `routeinstall.New`: the Sink takes the context of the plugin run that owns it.
3. After a peer that advertised Graceful Restart loses its TCP session, Ze
   holds that peer's routes, marked stale, on both transports of `bgp-gr`.
   A functional test asserts the retained count, an interop scenario asserts it
   against BIRD 3, and `bgp-rib` never receives the peer-down before `bgp-gr`
   answered it or was ended as failed.

The earlier measurement behind row 3 (`routes-in: 0` beside a GR state row,
2026-09-08) is not explained by today's code: peer-state delivery is ordered
and acknowledged. Reproducing or refuting it is the first step of the work,
before any fix is written against it.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/bgp-plugins.md`, `ai/doors/bgp-engine.md`, `ai/doors/plugin-infra.md`, `ai/doors/core.md`, `ai/doors/le-tooling.md`, `ai/doors/routing-protocols.md`, `ai/doors/system-plugins.md`
  → Constraint: `routing-protocols.md`, Boundaries: a forked protocol's installer needs its `RouteSink`, which ships each batch over the `route-install` RPC (`docs/architecture/forked-route-install.md`). The Sink's context is part of that path.
  → Constraint: every plugin runs internal or forked; the in-tree plugins hold no privilege over a third-party one on the protocol, so the validation decision RPC and its text commands are a published contract.
  → Constraint: a change under `pkg/` (here `rpc.ValidationDecision`, `rpc.BatchValidateResult`) is a contract change made on purpose, with its page.
  → Constraint: `bgp-plugins.md` lists "the dependency order on state events (`bgp-gr` before `bgp-rib`)" as a contract another plugin relies on.
  → Constraint: `plugin-infra.md`, Traps: every server error path that fails a plugin calls the failure path and stops the process. This spec reuses that path, it adds none.
- [ ] `docs/architecture/plugin/rib-storage-design.md`, "Validation eligibility"
  → Constraint: the page states "Zero explicitly means an unversioned decision for the current route, or the next arrival if no route is stored." The second half is the defect and is rewritten in the phase that changes it.
  → Decision: a decision for a newer UPDATE "waits for its route and removes the previous path from selection immediately". Preserved.
- [ ] `docs/architecture/api/process-protocol.md`, the `batch-validate` paragraph under the runtime RPCs
  → Constraint: it states "A zero message ID selects the current or next generation within that exact source, never a same-address sibling." Rewritten with the behaviour. It also states `peer-owner` and `connection-id` must be supplied together (both zero identify observations only) and an empty or null result is an SDK error: preserved.
- [ ] `docs/architecture/plugin/plugin-relationships.md`
  → Constraint: `bgp-gr` depends on `bgp` and `bgp-rib`; `bgp-rr` and `bgp-rs` read the `bgp-gr/retention` verdict. The verdict event is queued in a consumer's own delivery channel ahead of its DOWN, which is why delivery order matters to three plugins, not one.
- [ ] `docs/guide/graceful-restart.md`
  → Constraint: the guide promises retention, on a peer-down that is not a NOTIFICATION close, of the families the peer's capability named, and that a forked `bgp-gr` is ordered as an internal one. It is silent on what happens when a forked `bgp-gr` does not answer a peer-down in time.
- [ ] `ai/digests/rib.md`
  → Constraint: on peer down, "unless retained for GR (`retainedPeers`), the peer's `PeerRIB` is released". `retainedPeers` is written by `request bgp rib source retain-routes` (`rib_source_commands.go` `retainPhysicalSourceCommand`, dispatched by `bgp-gr`) and the operator form `request bgp rib retain-routes` (`rib_commands.go` `retainRoutes`).
- [ ] `docs/architecture/testing/interop.md`, "Scenario Structure", "Prove a scenario discriminates", "Which layer proves what", "Recording and replaying a checker"
  → Constraint: a scenario directory is named, never numbered; a checker change with a committed recording owes a replay test.
- [ ] `ai/rules/testing.md`, "Lifecycle Regressions"
  → Constraint: a change to a plugin's failure handling selects scenarios from that table and proves each in both modes. The selection is in the Test Plan below.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc4724.md`, row `RFC4724-4.2-3`, and `rfc/full/rfc4724.txt` Section 4.2
  → Constraint: RFC 4724 Section 4.2: "When the Receiving Speaker detects termination of the TCP session for a BGP session with a peer that has advertised the Graceful Restart Capability, it MUST retain the routes received from the peer for all the address families that were previously received in the Graceful Restart Capability and MUST mark them as stale routing information."
  → Constraint: RFC 4724 Section 4.2: "To deal with possible consecutive restarts, a route (from the peer) previously marked as stale MUST be deleted."
  → Constraint: `RFC4724-4-2` (Section 4): "It is noted that the normal BGP procedures MUST be followed when the TCP session terminates due to the sending or receiving of a BGP NOTIFICATION message." This is the negative twin of the retention test.
- [ ] `rfc/short/rfc6811.md`
  → Constraint: the validation-state tests in `adj_rib_in` carry `RFC6811-2-1` tags. A tagged test is not edited to match code (`ai/rules/testing.md`); the two old-contract tests named below are judged before any edit.

**Key insights:**
- `nextMsgID` in the reactor starts at 1, so a received UPDATE never has msgID 0. `bgp-rpki` copies the event's message id into every decision. The only in-tree source of msgID 0 is the two individual text commands, which have no msgID argument at all, and only tests send them.
- The event a validator receives carries the identity it must pass: `bgp.message.id`, `peer-owner`, and `connection-id` in the JSON event (`docs/architecture/api/json-format.md`; read by `Event.GetMsgID`, `internal/component/bgp/event.go`) and `StructuredEvent.MessageID`, `PeerOwner`, `ConnectionID` on the bridge (`pkg/plugin/rpc/bridge.go`). The same values become `RawRoute.MsgID`, `SourceOwner`, and `ConnectionID` in `bgp-adj-rib-in`.
- `routeinstall.New` has five callers and all five pass a background context: `bgp-rib`, `ospf`, `isis`, `static`, `connected`.
- A dependency cycle among state-event recipients is reachable at runtime: the startup check (`startupSelectedPhases`) reads `selectedPluginGraph`, which gives an external program no edges, while the delivery sort adds the edges that program declared at Stage 1.
- `bgp/server` holds each recipient as a `*process.Process`, whose `Stop` ends the generation; `superviseProcess` then runs `applyFailurePolicy`, which raises the plugin-crash report through `ProcessManager.ReportCrash` and applies the declared policy.
- Peer-state delivery is already sequential and acknowledged (`deliverPeerState`). The hole is the error arm: a failed delivery is logged and the loop moves on.
- `setupRemoteRIB` runs before the plugin's context is created, so the fix moves the context up; it cannot be passed where it does not yet exist.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` - `applyDecision` applies a decision to the pending route, else to the installed route, else calls `storeEarlyDecision`. Every msgID comparison is guarded by "msgID is not zero", so a zero decision matches any generation. `applyEarlyDecision`, called for every newly arrived route while validation is on, applies a buffered decision whose msgID is zero to that arrival. `sweepExpiredEarlyDecisions` drops a buffered decision after one minute (`earlyDecisionTimeout`).
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - `acceptRoutesCommand` and `rejectRoutesCommand` build a `rpc.ValidationDecision` with no msgID, owner, or connection, call `applyOperatorDecisionLocked`, and answer `early: true` when buffered. `batchValidateCommand` parses nine fields per decision (`action peer family prefix pathID state msgID peerOwner connectionID`, `batchValidateStride = 9`, with `msgID` as the seventh field) and delegates to `handleBatchValidateTyped`, which answers `accepted`, `rejected`, `early`.
- [ ] `internal/component/bgp/plugins/adj_rib_in/received_operator.go` - `applyOperatorDecisionLocked` selects every matching current owner for `address` in `ribIn` and `pending`, stamps each matching route's captured `PeerOwner` and `ConnectionID`, and calls `applyDecision`. When no route is present, it calls `applyDecision` with observational `receivedPeer{address: address}` (`owner: 0`, `ConnectionID: 0`), which never grants authority over an owned peer whose route has not arrived.
- [ ] `internal/component/bgp/plugins/adj_rib_in/received_peer.go` - `receivedBefore` orders original `connection` before original `message` and returns false whenever either `message == 0`; `receivedPeerOf` keeps missing provenance (`owner == 0` or `connection == 0`) observational.
- [ ] `internal/component/bgp/plugins/adj_rib_in/source_state.go` - `sourceDecisionAdmittedLocked` permits an owned source's pre-UPDATE decision only when `(owner, address, connectionID)` matches a live source in `sourceLive` or a stored route with matching `ConnectionID` and (`MsgID == 0` or equal `MsgID`); observational `owner == 0` stays separate.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - the `RawRoute.MsgID` comment promises "Validation decisions with MsgID zero apply to the current route, or buffer until its arrival." `installStructuredNLRIs` and the JSON ingest both remove the installed route, build the pending route and call `applyEarlyDecision`.
- [ ] `internal/component/bgp/plugins/rpki/rpki.go`, `rpki_reload.go` - `buildDecisions` sets `MsgID`, `PeerOwner`, and `ConnectionID` from the request, which `validateNLRIs` filled from the structured event or JSON event. `dispatchBatch` and `revalidateConfigSnapshot` are the two callers of `sdk.Plugin.BatchValidate`, and both discard the result.
- [ ] `internal/component/bgp/reactor/received_update.go` - `nextMsgID` returns the counter after adding one: the first id is 1.
- [ ] `pkg/plugin/rpc/bridge.go`, `pkg/plugin/sdk/sdk_engine.go`, `internal/component/plugin/server/dispatch_registry.go`, `internal/component/plugin/server/dispatch_batch_validate.go` - `ValidationDecision` and `BatchValidateResult` types; `BatchValidate` routes over `ze-plugin-engine:batch-validate` on both DirectBridge and JSON socket rails (no process-local `rpc.GetBatchValidator` slot) into `opBatchValidate` → `dispatchBatchValidation`, which serializes each decision into the nine-field `request bgp adj-rib-in batch-validate` command, dispatches to the registered receive-store owner, and unmarshals `BatchValidateResult`.
- [ ] `internal/component/bgp/plugins/rib/rib_remote.go`, `internal/component/bgp/plugins/rib/rib_next_hop.go` - `setupRemoteRIB` builds the Sink with `routeinstall.New` on a background context; both `RIBManager.igpDistance` (`rib_remote.go:66`, calling `RouteMetrics`) and `remoteRIB.prospectiveDistance` (`rib_next_hop.go:173`, calling `RouteMetricsProspective` for CIDR routes) derive their five-second timeout from `context.Background()`. `insertLocRIB` and `removeLocRIB` call `Sink.Flush` under the remote mutex.
- [ ] `internal/core/rib/routeinstall/sink.go` - `New` stores the context it is given and uses it for every `RouteInstall` and `RouteRemove`. Its comment says the forked OSPF and IS-IS wiring also passes a background context.
- [ ] `internal/component/bgp/plugins/rib/rib.go`, `internal/component/bgp/plugins/rib/rib_source_commands.go` - the run function calls `setupRemoteRIB` when no in-process Loc-RIB exists, and only later creates its context with `sdk.SignalContext`, cancelled by a deferred call when `Run` returns. `handleState` and `handleStructuredState` release the peer's Adj-RIB-In on the down edge unless `retainedPeers` holds the peer. `rib_source_commands.go` implements the physical-source GR commands `request bgp rib source retain-routes` (`retainPhysicalSourceCommand`), `mark-stale` (`markPhysicalSourceCommand`), and `purge-stale` (`purgePhysicalSourceCommand`).
- [ ] `internal/component/bgp/plugins/gr/gr.go`, `register.go` - on a DOWN, `handleStructuredState` dispatches `request bgp rib source purge-stale`, `request bgp rib source retain-routes` (`owner <id> connection <id> on-down families <families>`, `gr.go:490`), and `request bgp rib source mark-stale` synchronously, then returns; a deferred `emitRetention` puts the `bgp-gr/retention` verdict on the bus. `grDependencies` names `bgp-rib`.
- [ ] `internal/component/bgp/server/events.go` - `deliverPeerState` sorts recipients with `sortByReverseDependencyTier` (dependents first), then for each one enqueues the event and blocks on its result before the next. A refused enqueue (`Deliver` false: the process is stopping) and a result carrying an error both continue to the next recipient; the error is a `Warn`. When the tier computation fails, the sort falls back to ascending process name.
- [ ] `internal/component/plugin/process/delivery.go` - `sendBatch` calls the bridge directly for an internal plugin (no timeout) and `deliverViaConn` for a forked one, bounded by `ze.plugin.delivery.timeout`, default 5 s (`defaultDeliveryTimeout`). `deliverBatch` hands the batch error to every waiting result.
- [ ] `internal/component/bgp/plugins/bmp/bmp_events.go`, `bmp_replay.go` - `replayPeerLocked` is called from `primeSender`, `bounceMonitoredPeers` and `handleSenderState` (commit ee78b53a3d).
- [ ] `internal/plugins/ospf/spf_wiring.go`, `internal/plugins/isis/spf_wiring.go` (`initSPF`), `internal/plugins/static/register.go`, `internal/plugins/connected/connected.go` - each builds its forked Sink with `routeinstall.New` on a background context when `locrib.Default()` answers nil. How each of the four reaches its plugin's run context was not read.
- [ ] `internal/component/plugin/server/stage1_checks.go`, `internal/component/plugin/server/startup.go` - `startupSelectedPhases` refuses a cycle over `selectedPluginGraph`, whose comment says it "gives external programs no compiled-in edges"; `stage1Checks` (`stage1_checks.go`) and `engineStartupSink.onRegistration` (`startup.go`) validate Stage 1 declarations (including the inline `dependencies` check) without checking whether declared edges close a cycle with the selected plugins.
- [ ] `internal/component/plugin/registry/registry.go` - `TopologicalGraphTiers` fails for one reason only, a cycle (`ErrCircularDependency`); an edge to a node outside the map is inert.
- [ ] `internal/component/plugin/server/failure_policy.go`, `internal/component/plugin/process/manager.go` - `superviseProcess` calls `applyFailurePolicy` when a generation's command loop returns; it returns early when the daemon is stopping, a removal is pending, or the manager no longer holds that process; otherwise `ReportCrash` raises the plugin-crash report and the declared policy (restart, fatal, ignore) applies.
- [ ] `internal/le/interoplab/bgp/names.go` - scenario names are declared constants; `bgp-graceful-restart-frr` is the existing GR scenario.
- [ ] `/home/thomas/Code/gitlab.nic.cz/labs/bird/proto/bgp/bgp.c` (BIRD 3.2.1) - `bgp_handle_graceful_restart` decides per channel: a channel whose `gr_ready` is set starts a refresh cycle and keeps its routes; any other is flushed at once; the timer is the peer's restart time clamped by configuration. `bgp_prepare_capabilities` (`packets.c`) lists a channel in the capability only when the channel is `gr_able`, and sets the Restart flag only when the process is in `gr_recovery`.

**What the code shows about the peer-down order (item 3a).** In the code the
dependent is `bgp-gr` (it declares `bgp-rib` as a dependency) and it is served
first; the dependency, `bgp-rib`, is served after it.

| Case | Can `bgp-rib` receive the DOWN before `bgp-gr` answered? | Evidence |
|------|-----------------------------------------------------------|----------|
| `bgp-gr` internal | No. The bridge call is synchronous and unbounded; the result is sent only after the handler returned, and the handler returns only after its three dispatches returned | `sendBatch`, `deliverMixedBatch`, `handleStructuredState` |
| `bgp-gr` forked, answers within the delivery timeout | No | `deliverViaConn` |
| `bgp-gr` forked, does not answer within the timeout | **Yes.** The result carries the timeout error, `deliverPeerState` logs "OnPeerStateChange write failed" and delivers to `bgp-rib`, which releases the routes. `bgp-gr` is not ended and not declared failed; its late `retain-routes` finds nothing | `deliverViaConn`, the error arm of `deliverPeerState` |
| `bgp-gr` handler panics or returns an error | **Yes**, the same error arm | `safeDeliverBatch`, `safeBridgeCall` |
| `bgp-gr` is stopping (`Deliver` refused) | Yes, and that is correct: its generation is ending | `Process.Deliver` |
| Tier computation fails | Order becomes ascending process name. An operator who names the processes `gr` and `bgp-rib` (as `test/plugin/graceful-restart-rib.ci` does) gets `bgp-rib` first. Reachable: a forked program whose Stage 1 declaration names a dependency that, through the selected implementations, depends back on the implementation that program runs; the startup cycle check never saw those edges | the fallback arm of `sortByReverseDependencyTier`, `selectedPluginGraph`, `TopologicalGraphTiers` |

**Behavior to preserve:**
- A decision carrying a msgID: stale ones ignored, a newer one buffered and the previous path made ineligible at once, expiry rules of `sweepExpiredEarlyDecisions`.
- A msgID-0 decision applied to a pending or an installed route, exactly as today.
- The 30 s fail-open of a pending route (`defaultValidationTimeout`) and the `enable-validation` timeout argument.
- The nine-field text batch form (`action peer family prefix pathID state msgID peerOwner connectionID`) and the existing answer keys `accepted`, `rejected`, `early`, `status`.
- The source-provenance contract (`received_operator.go`, `received_peer.go`, `source_state.go`): `peer-owner` and `connection-id` must be supplied together (both zero identify observations only), and an address-only operator command never creates early-decision authority for an owned peer whose route has not arrived.
- Sequential, acknowledged peer-state delivery; the peer-up barrier accounting in `deliverPeerState`.
- The operator form `request bgp rib retain-routes <selector>` and the physical-source form `request bgp rib source retain-routes owner <id> connection <id> on-down families <families>` `bgp-gr` sends.
- The in-process `bgp-rib` path (shared Loc-RIB, `igpcost.Lookup`, `igpcost.ResolveProspective`): untouched.

**Behavior to change:**
- A msgID-0 decision that finds neither a pending nor an installed route is not buffered. Its sender is told, and it is counted.
- `accept-routes` and `reject-routes` take optional trailing `[<msgID> [<peerOwner> <connectionID>]]`, parsed by the same uint64 parser and provenance-pairing check as nine-field `batchValidateCommand`. Supplying both nonzero `peerOwner` and `connectionID` routes through `applyDecision` with exact physical provenance (subject to `sourceDecisionAdmittedLocked`); omitting them keeps `applyOperatorDecisionLocked`'s address-aggregate behavior for present routes and observational-only scope when no route is present. The protocol changes directly: no compatibility path (pre-release rule).
- Every forked route-install Sink, and both `bgp-rib` forked next-hop metric lookups (`RIBManager.igpDistance` and `remoteRIB.prospectiveDistance`), use the run context of the plugin that owns them.
- A recipient whose peer-state delivery failed or timed out, with the daemon running, is ended through its failure policy before delivery continues to the remaining recipients.
- A dependency cycle that a forked program's Stage 1 declaration closes is refused at Stage 1 (`stage1Checks` / `engineStartupSink.onRegistration`). The name-order fallback is removed; if the sort still fails, delivery says so loudly and never orders by name.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| UPDATE generation identity | `adj_rib_in/rib.go` `RawRoute.MsgID`, `ingestedMsgID`; `reactor/received_update.go` `nextMsgID` | Stamps every stored route with the reactor message id of its UPDATE; never zero for a received UPDATE | Extends: the identity becomes mandatory for anything that waits. A decision without it can only act on what is present |
| Early-decision buffer and its expiry | `adj_rib_in/rib_validation.go` `storeEarlyDecision`, `applyEarlyDecision`, `sweepExpiredEarlyDecisions`, `earlyDecisionTimeout` | Holds a decision that arrived before its route, one minute at most unless a known predecessor is waiting | Extends: `storeEarlyDecision`, the only writer of the buffer, admits only a decision with a msgID, so every reader loses its zero arm. Not replaced |
| Explicit operator address-selected validation | `adj_rib_in/received_operator.go` `applyOperatorDecisionLocked` | Selects every matching current owner for an address and stamps each matching route's captured `PeerOwner` and `ConnectionID`; when none is present, calls `applyDecision` with observational `owner: 0`, `ConnectionID: 0` | Extends: propagates the dropped outcome when `len(sources) == 0` and msgID is 0, while preserving observational-only isolation when `<msgID>` is passed without `peerOwner`/`connectionID` |
| Original connection and ingress ordering | `adj_rib_in/received_peer.go` `receivedBefore`, `receivedPeerOf` | Orders `connection` before `message`, returning false when either `message == 0`, and keeps missing provenance observational | Does not change: `receivedBefore` continues to order physical connections ahead of ingress IDs; with `earlyDecisions` holding only non-zero msgIDs, its zero guard serves only incoming unversioned decisions and R-4 |
| Source decision admission | `adj_rib_in/source_state.go` `sourceDecisionAdmittedLocked` | Admits an owned source's pre-UPDATE decision only for a live established `(owner, address, connectionID)` in `sourceLive` or a stored route with matching `ConnectionID` and (`MsgID == 0` or matching `MsgID`) | Does not change: governs owned `<msgID> <peerOwner> <connectionID>` decisions on `accept-routes`/`reject-routes` exactly as on `batch-validate`, ensuring an unversioned decision on a GR-retained route still applies while missing provenance never gains owned authority |
| Batch validation engine routing | `plugin/server/dispatch_batch_validate.go` `dispatchBatchValidation`; `plugin/server/dispatch_registry.go` `opBatchValidate` | Routes both DirectBridge and JSON `ze-plugin-engine:batch-validate` calls to the registered receive-store owner using nine-field `request bgp adj-rib-in batch-validate`, then unmarshals `BatchValidateResult` | Does not change structurally: automatically carries the additive `dropped` JSON key back in `BatchValidateResult` on both transports |
| Pending route and fail-open timeout | `adj_rib_in/rib_validation.go` `pendingRoute`, `sweepExpiredPending` | A route awaiting its verdict is installed as not-validated after the timeout | Does not apply as a change: it is what resolves a route whose unversioned verdict was dropped, and that consequence is stated in the pages |
| Decision answer counters | `pkg/plugin/rpc/bridge.go` `BatchValidateResult` | Reports `accepted`, `rejected`, `early` to the sender | Extends: one more counter, `dropped`, so a sender can tell a dropped decision from an applied one |
| Plugin run context | `rib/rib.go` run function, `sdk.SignalContext`; `OnStarted` hands a context to `runAIGPSelection` and `runRemoteMetrics` | Ends when the plugin's `Run` returns or a signal arrives | Extends: the Sink, `RIBManager.igpDistance` (`rib_remote.go`), and `remoteRIB.prospectiveDistance` (`rib_next_hop.go`) take it. No second context is created |
| Route-install Sink | `internal/core/rib/routeinstall/sink.go` `New`, `Flush`; `docs/architecture/forked-route-install.md` | Already takes a context for every RPC; all five callers hand it a background one, and its comment records that | Extends: the constructor is unchanged, its comment and all five callers change. One rule, no per-plugin variant |
| Batch msgID and provenance parsing | `adj_rib_in/rib_commands.go` `batchValidateCommand` | Parses the nine-field decision (`action peer family prefix pathID state msgID peerOwner connectionID`), with `msgID` as the seventh field and paired `(peerOwner, connectionID)` provenance | Extends: the uint64 parser and provenance-pairing check are shared by `accept-routes` and `reject-routes` for their optional trailing `[<msgID> [<peerOwner> <connectionID>]]` |
| Event message id | `bgp.message.id` (`docs/architecture/api/json-format.md`), `StructuredEvent.MessageID` | Gives every UPDATE event the id `bgp-adj-rib-in` stores with the route | Does not change: it is what a validator passes back, and the validator page says so |
| Startup cycle refusal | `plugin/server/startup.go` `startupSelectedPhases`, `selectedPluginGraph`, `engineStartupSink.onRegistration`; `plugin/server/stage1_checks.go` `stage1Checks`; `registry.ErrCircularDependency` | Refuses a cycle among compiled-in edges before any plugin starts; `stage1Checks` and `engineStartupSink.onRegistration` validate Stage 1 declarations without checking for cycles added by an external program's edges | Extends: adds the cycle refusal to Stage 1 (`stage1Checks` / `engineStartupSink.onRegistration`) for the edges a forked program declares |
| Ordered peer-state delivery | `bgp/server/events.go` `deliverPeerState`, `sortByReverseDependencyTier`; `docs/guide/graceful-restart.md` | Dependents first, one at a time, each acknowledged | Extends: the error arm ends the failed recipient before serving a recipient that depends on its answer. The order and the acknowledgement are unchanged |
| Delivery graph and start-order edges | `registry.StartOrderEdges`, `registry.TopologicalGraphTiers`, Stage 1 `Dependencies`, `OptionalDependencies`, `StartAfter` | Give the tiers the sort uses, for internal and forked processes alike | Extends: a failed tier computation is reported, not hidden behind a name sort |
| Plugin failure path | `process.Process.Stop`; `plugin/server/failure_policy.go` `superviseProcess`, `applyFailurePolicy`; `process.ProcessManager.ReportCrash`; `ai/doors/plugin-infra.md`, Traps | `Stop` ends a generation; the supervisor then reports the crash and applies the declared policy | Reused as is: `deliverPeerState` calls `Stop` on the failed recipient and the supervisor does the rest. Child 1 owns any change to the path itself |
| GR retention flag | `rib/rib.go` `retainedPeers`, `rib/rib_source_commands.go` `retainPhysicalSourceCommand` (`request bgp rib source retain-routes`, called by `gr/gr.go:490`), `rib/rib_commands.go` `retainRoutes` (operator form) | Set by `request bgp rib source retain-routes` (for GR peer-down) and `retain-routes` (operator), read on the down edge, cleared on the next up | Does not change. The parent's earlier plan ("`bgp-rib` defers the release") is dropped: the order already guarantees the flag is set first when `bgp-gr` answers |
| `bgp-gr/retention` verdict | `gr/gr.go` `emitRetention`, `internal/component/bgp/retention` | Tells `bgp-rr` and `bgp-rs` which families a DOWN retains | Does not change; named because the same delivery-failure hole reaches its readers |
| BMP replay on collector connect | `bmp/bmp_replay.go` `replayPeerLocked` | Replays a monitored peer's snapshot | Does not apply: already fixed (see "Rows found already fixed") |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "A change under `pkg/` is a change to a published contract. A type, a field, a callback name and the bytes on the wire all count." | agrees | Adding `Dropped` to `rpc.BatchValidateResult` and narrowing unversioned `ValidationDecision.MsgID` 0 semantics are deliberate contract changes made alongside their contract pages (`process-protocol.md`, `docs/plugin-development/protocol.md`). |
| `docs/architecture/plugin/rib-storage-design.md` | Validation eligibility | `// Design:` of `adj_rib_in/rib_validation.go`, `rib_commands.go`, `rib.go`, `received_operator.go`, `rib/rib.go`; modified | "Zero explicitly means an unversioned decision for the current route, or the next arrival if no route is stored." | departs | This spec removes buffering for unversioned (msgID 0) decisions when no route is stored: a msgID-0 decision applies only to a pending or installed route present on arrival and is otherwise dropped and counted in `dropped`, while versioned (non-zero msgID) decisions keep buffering under the existing nine-field `batch-validate` and `(peer-owner, connection-id)` provenance contract. Phase 2 rewrites this sentence. |
| `docs/architecture/bgp/structural-forwarding.md` | Receive validation and cache changes | anchored to `adj_rib_in/rib_validation.go` | "A route withdrawn while still pending is cancelled at its pending record: a verdict that arrives later for that generation installs nothing, and the relay answers the explicit withdrawal." | agrees | Dropping an unversioned decision when no route is stored closes the remaining bypass where a msgID-0 verdict arriving after withdrawal could buffer and install a later re-announcement without validating it. |
| `ai/doors/bgp-plugins.md` | Who depends on it from outside | door covering `adj_rib_in/*`, `rib/*` | "Route metadata keys a plugin sets or reads, and the dependency order on state events (`bgp-gr` before `bgp-rib`)" | agrees | Phase 4 enforces reverse dependency order on peer-down failures (stopping a failed dependent before continuing, and refusing Stage 1 cycles that could invert the sort) on both internal and forked transports. |
| `docs/architecture/api/process-protocol.md` | Forked route install (`route-install` / `route-remove`) | `// Design:` of `pkg/plugin/rpc/bridge.go`, `pkg/plugin/sdk/sdk_engine.go`, `plugin/server/startup.go`; anchored to `adj_rib_in`, `rib`, `sink.go`, `bgp/server/events.go`; modified | "A zero message ID selects the current or next generation within that exact source, never a same-address sibling." | departs | This spec narrows msgID 0 so it selects only a route currently pending or installed within that exact source (never the next arrival), drops otherwise, and reports `dropped` in `BatchValidateResult`, while keeping nine-field `batch-validate` routing through `dispatchBatchValidation` and the `(peer-owner, connection-id)` provenance contract intact. Phase 2 rewrites this sentence and Phase 4 documents Stage 1 dependency-cycle refusal. |
| `docs/architecture/api/commands.md` | Command Dispatch | anchored to `adj_rib_in/rib.go`, `pkg/plugin/rpc/bridge.go`, `pkg/plugin/sdk/sdk_engine.go` | - | silent | Covers CLI/API command dispatch and pipe routing; silent on `adj-rib-in` unversioned validation buffering, forked `routeinstall.Sink` run-context cancellation, and peer-down failure handling in `deliverPeerState`. |
| `docs/architecture/api/architecture.md` | Data Flow: Config to Delivery | anchored to `pkg/plugin/rpc/bridge.go`, `bgp/server/events.go` | "It then releases the lock, sends the Cease and delivers `down` with reason `removed` to that captured set, in reverse dependency order." | agrees | Reverse dependency order for peer-state delivery is preserved and strengthened by ending a failed recipient and refusing Stage 1 dependency cycles rather than falling back to name order. |
| `docs/architecture/api/ipc_protocol.md` | Plugin Transports | anchored to `pkg/plugin/rpc/bridge.go`, `pkg/plugin/sdk/sdk_engine.go` | - | silent | Covers `rpc.Conn` framing, `PluginConn.Err`, and `event-settlement` transport mechanics; silent on unversioned validation buffering, forked `routeinstall.Sink` contexts, and peer-down failure handling in `deliverPeerState`. |
| `docs/architecture/api/json-format.md` | Message Structure | anchored to `pkg/plugin/rpc/bridge.go`, `bgp/server/events.go` | "This structure keeps message metadata (type, id, direction) together in the `message` object." | agrees | This spec relies on the engine-assigned `bgp.message.id` (never zero for a received UPDATE) as the generation fence that validators return in versioned validation decisions, and adds lowercase kebab-case JSON keys (`dropped`). |
| `docs/architecture/core-design.md` | JSON Events (Engine → Plugin) | `// Design:` of `connected.go`, `bgp/server/events.go`; anchored to `bridge.go`, `sdk_engine.go`, `ospf/spf_wiring.go` | "**Message ID**: Plugins use the `message.id` for cache-control operations such as forwarding or releasing cached UPDATEs." | agrees | Validators use the same non-zero `message.id` as the UPDATE generation fence for buffered validation decisions, and the engine remains stateless for routes while keeping ordered peer-state delivery. |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Both ESP forms on one inbound state | anchored to `pkg/plugin/rpc/bridge.go` | - | silent | Covers IKEv2 Child SA XFRM dataplane and ESP form receiver; silent on BGP validation decisions, forked `routeinstall.Sink` contexts, and BGP peer-state delivery ordering. |
| `docs/architecture/plugin/show-enrichers.md` | Constraints | anchored to `pkg/plugin/rpc/bridge.go` | - | silent | Covers proxy show enrichers and restart teardown of enricher callbacks; silent on BGP validation decisions, forked `routeinstall.Sink` contexts, and BGP peer-state delivery ordering. |
| `ai/doors/plugin-infra.md` | Traps | door covering `pkg/plugin/rpc/bridge.go`, `pkg/plugin/sdk/sdk_engine.go` | "**One plugin failure stops the startup tier, by owner ruling.** Do not propose making it survivable, and make every server error path call `PluginFailed` and `proc.Stop()`." | agrees | Phase 4 calls `proc.Stop()` on a recipient whose peer-state delivery fails so the supervisor raises the crash report and applies its declared failure policy, and Stage 1 cycle refusal stops the startup tier. |
| `docs/architecture/behavior/fsm-established.md` | Compatibility notes | anchored to `rib/rib.go` | - | silent | Covers Established-state BoRR/EoRR and double-KEEPALIVE end-of-sync heuristics; silent on validation msgID-0 buffering, forked `routeinstall.Sink` contexts, and peer-down delivery failure handling. |
| `docs/architecture/decisions/001-pull-model-metrics.md` | Decision | anchored to `rib/rib.go` | - | silent | Decision record deferring pull-model Prometheus metrics; silent on validation decisions, forked `routeinstall.Sink` contexts, and BGP peer-state delivery ordering. |
| `docs/architecture/doctor-and-health-checks.md` | The runtime phase: checks only the daemon runs | anchored to `rib/rib.go` | "reads live `remoteRIB.metricFailureActive` state without issuing an engine RPC so `show doctor` reports an active forked `route-metrics` lookup failure directly alongside the SDK report bus" | agrees | Phase 3 derives both `RIBManager.igpDistance` and `remoteRIB.prospectiveDistance` timeouts from the plugin run context instead of `context.Background()`, preserving `remoteRIB.metricFailureActive` reporting and its runtime doctor check unchanged. |
| `docs/architecture/rib-transition.md` | Current State vs. Target State | anchored to `rib/rib.go` | - | silent | Covers high-level RIB extraction and origin admission preferences; silent on validation msgID-0 buffering, forked `routeinstall.Sink` run-context cancellation, and peer-down delivery failure handling. |
| `docs/architecture/rib/forward-handle.md` | The fast-path tracker | anchored to `rib/rib.go` | "SDK `Run` fences and drains admitted events before `runRIBPlugin` detaches the mirror with `SetLocRIB(nil)`." | agrees | Phase 3 creates the plugin run context before `setupRemoteRIB` while keeping `runRIBPlugin`'s drain and defer ordering intact before `SetLocRIB(nil)`. |
| `docs/architecture/route-selection.md` | Phase 2: Best-Path Selection (RFC 4271 Section 9.1.2) | `// Design:` of `rib/rib_remote.go`, `rib/rib_next_hop.go`; anchored to `rib/rib.go`, `rib_remote.go`, `rib_next_hop.go` | "Each engine metric request has a five-second response bound. A failed lookup leaves its candidate ineligible (`Resolved: false`) and is not cached." | agrees | Phase 3 derives the five-second timeout in both `RIBManager.igpDistance` (`rib_remote.go`) and `remoteRIB.prospectiveDistance` (`rib_next_hop.go`) from the plugin run context instead of `context.Background()`, so stop cancels both lookups promptly while keeping the five-second bound and fail-closed uncached behavior. |
| `docs/architecture/forked-route-install.md` | A sink per installer, not a remote Loc-RIB | `// Design:` of `sink.go`; anchored to `rib_remote.go`, `static/register.go`, `sink.go`; modified | "A persistent install or remove failure closes the SDK connection. A live connection with a rejected RPC is not sufficient for recovery: disconnect is what triggers the engine's withdrawal of that producer's routes." | agrees | Phase 3 keeps the persistent-failure close rule for live RPC failures, passes each plugin's run context to `routeinstall.New` across all five callers, keeps orderly shutdown quiet on context cancellation, and updates this page to document the run-context rule. |
| `docs/architecture/ospf/ospf-8-spf-rib.md` | Decisions | `// Design:` of `ospf/spf_wiring.go`; modified | "**Route install is a Loc-RIB insertion only.** Redistribution stays in the redistribution events path and never touches the kernel." | agrees | Phase 3 keeps OSPF's forked Loc-RIB insertion via `routeinstall.Sink`, binding its context to OSPF's plugin run context, and checks/updates this page where it describes the forked Sink. |
| `ai/doors/routing-protocols.md` | Boundaries | door covering `ospf/spf_wiring.go`, `isis/spf_wiring.go` | "In a subprocess `locrib.Default()` is nil, so the installer needs its `RouteSink`, which ships each batch over the `route-install` RPC (`docs/architecture/forked-route-install.md`)." | agrees | Phase 3 keeps the `RouteSink` and `route-install` RPC boundary for forked OSPF and IS-IS, binding the Sink's context to each plugin's run context so stop unblocks pending RPCs. |
| `docs/architecture/isis/isis-9-spf-rib.md` | Decision: install is Loc-RIB insertion, not a redistribute event | `// Design:` of `isis/spf_wiring.go`; modified | "After the inserted path, the Loc-RIB best-path to sysrib to FIB chain is the same machinery that already installs static, connected and BGP routes." | agrees | Phase 3 preserves IS-IS Loc-RIB insertion through `routeinstall.Sink`, binds its context to the IS-IS plugin run context, and checks/updates this page where it describes the forked Sink. |
| `docs/architecture/static-routes.md` | Startup ordering and emit rules | `// Design:` of `static/register.go`; modified | "Main-table route-install RPCs are runtime operations, and redistribution subscribers in later startup phases must exist before static emits its initial routes. This ordering applies to both internal and external static, independently of system RIB placement." | agrees | Phase 3 preserves static's startup ordering and forked `route-install` RPC path while passing static's plugin run context to `routeinstall.New`, and updates this page if needed. |
| `ai/doors/system-plugins.md` | Boundaries | door covering `static/register.go`, `connected/connected.go` | "Reach the engine through the SDK, the bus, or a command, never through a Go call into a component's state (`docs/architecture/plugin/plugin-system.md`, "The process boundary")." | agrees | Both `static` and `connected` continue to reach the engine's Loc-RIB over `routeinstall.Sink` SDK RPCs when forked, now bounded by each plugin's run context. |
| `ai/doors/core.md` | Where it sits | door covering `internal/core/rib/routeinstall/sink.go` | "A core package is a library with no config-driven lifecycle and no registry side effect (`docs/architecture/module-tiers.md`, "The three tiers"). It imports neither `internal/component/` nor `internal/plugins/`." | agrees | `routeinstall.Sink` remains a pure core library with unchanged constructor `New(ctx, client)` and no upward imports; callers pass their plugin run context and `Sink.send` handles run-context cancellation quietly. |
| `docs/architecture/behavior/peer-lifecycle.md` | FSM state callback | anchored to `bgp/server/events.go` | "Step 8 delivers the peer-up event to each barrier plugin and waits for its answer on the FSM callback goroutine (`deliverPeerState`): an acknowledgement opens the barrier, and a refusal lowers the expected count to what was acknowledged, which opens it too." | agrees | Phase 4 preserves sequential, acknowledged delivery and peer-up barrier accounting in `deliverPeerState`, adding `proc.Stop()` on failed deliveries when the daemon is not stopping. |
| `docs/architecture/exabgp-bridge.md` | The bridge translates in two directions | anchored to `bgp/server/events.go` | - | silent | Covers ExaBGP wrapper endpoints, wire translation, and raw base64 UPDATE delivery; silent on `adj-rib-in` unversioned validation buffering, forked `routeinstall.Sink` contexts, and peer-down failure handling in `deliverPeerState`. |
| `docs/architecture/meta/role.md` | Learned role lifecycle | anchored to `bgp/server/events.go` | - | silent | Covers RFC 9234 BGP Roles, OTC validation, and `bgp-role` learned-role pruning on removal; silent on validation msgID-0 buffering, forked `routeinstall.Sink` contexts, and `deliverPeerState` failure handling. |
| `docs/architecture/plugin/plugin-system.md` | Dependencies | anchored to `bgp/server/events.go` | "A dependency a plugin declares at Stage 1 is checked against the plugins the daemon runs (`process-protocol.md`, "Ordering Declaration")," | agrees | Phase 4 extends Stage 1 dependency checking to refuse a forked program whose declared edges close a dependency cycle with the selected plugins, matching `startupSelectedPhases` for compiled-in plugins. |
| `ai/doors/bgp-engine.md` | Boundaries | door covering `bgp/server/events.go` | "Never name a plugin package from the reactor." | agrees | `bgp/server/events.go` names no plugin; `deliverPeerState` applies its stop-on-failure rule uniformly to any failed state-event recipient and reads ordering solely from registered and Stage 1 declared edges. |
| `docs/architecture/testing/interop.md` | Prove a scenario discriminates | `// Design:` of `internal/le/interoplab/bgp/names.go`; anchored to `names.go` | "An interop scenario is evidence only if it goes RED when the behaviour it tests is broken. Before you rely on a new scenario, revert the fix, run the scenario, and confirm that it fails." | agrees | Phase 6 adds the named BIRD 3 scenario (`bgp-gr-retention-bird`), runs the discrimination walk by breaking `retainPhysicalSourceCommand` (`internal/component/bgp/plugins/rib/rib_source_commands.go`, `request bgp rib source retain-routes`, called by `gr/gr.go:490`) to confirm red, and commits a checker recording and replay test where applicable. |
| `ai/doors/le-tooling.md` | Boundaries | door covering `names.go`, `test/plugin/*.ci` | "**A compiled Go fixture returns an error when an assertion fails** (`ai/patterns/functional-test.md`, `ai/rules/testing.md`)." | agrees | All four new `.ci` functional tests are drafted under `test/draft/plugin/` first and use compiled Go observers under `internal/test/fixture/` that wait on state conditions and return an error on failed assertions. |
| `docs/plugin-development/protocol.md` | Runtime RPCs (Plugin to Engine) | modified | "Plugins call the engine during runtime through these SDK methods:" | agrees | Phase 2 documents the `batch-validate` msgID-0 rule (validators answering an UPDATE event pass the event's message ID) and the additive `dropped` counter in `BatchValidateResult` on this page. |
| `docs/architecture/plugin/plugin-relationships.md` | Cross-Plugin Bus Dependencies | modified | "gr emits before answering DOWN and receives state events before rr/rs (reverse dependency order), so FIFO delivery puts the verdict ahead of its owning DOWN on both transports" | agrees | Phase 4 preserves reverse dependency order for `bgp-gr` ahead of `bgp-rib`, `bgp-rr`, and `bgp-rs`, stops a failed recipient before continuing rather than warning past it, and updates this page to state what a failed peer-down delivery does. |
| `docs/architecture/config/yang-config-design.md` | 3. Module Loading | anchored to `internal/component/plugin/server/stage1_checks.go` | - | silent | Covers YANG module loading, schema publication, and `schema-unchanged` checks on restart; silent on Stage 1 dependency-cycle refusal, validation msgID-0 buffering, forked Sink contexts, and peer-down delivery failure handling. |
| `docs/architecture/cli/command-completion.md` | Why web sources at request time | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers CLI/web tab completion and `WaitForStartupComplete`; silent on Stage 1 dependency-cycle refusal, validation msgID-0 buffering, forked Sink contexts, and peer-down delivery failure handling. |
| `docs/architecture/config/transaction-protocol.md` | 13. Recovery | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers config transaction verify/apply/rollback phases and recovery; silent on Stage 1 dependency-cycle refusal, validation msgID-0 buffering, forked Sink contexts, and peer-down delivery failure handling. |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 4: an attack-response table is cleared at the next start | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers firewall nft/vpp table ownership and attack-response table startup flush; silent on Stage 1 dependency-cycle refusal, validation msgID-0 buffering, forked Sink contexts, and peer-down delivery failure handling. |
| `docs/architecture/hub-architecture.md` | Stage 1: Declaration Messages | anchored to `internal/component/plugin/server/startup.go` | "A plugin that declares `fatal` and fails a startup stage after declaring stops ze, and so does a `respawn true` leaf against a plugin that declares it must not be restarted." | agrees | Phase 4's Stage 1 dependency-cycle refusal runs inside `stage1Checks` / `engineStartupSink.onRegistration` and preserves the existing Stage 1 failure-policy and startup-lifetime handling unchanged. |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers IKEv2 engine startup and Child SA lifecycle; silent on Stage 1 dependency-cycle refusal, BGP validation decisions, forked Sink contexts, and BGP peer-down delivery failure handling. |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | Scripted kernel tests | anchored to `internal/component/plugin/server/startup.go` | - | silent | Covers PPPoE client config delivery and scripted kernel tests; silent on Stage 1 dependency-cycle refusal, BGP validation decisions, forked Sink contexts, and BGP peer-down delivery failure handling. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | This spec adds no engine operation and changes no authorization profile; `batch-validate`, `route-install`, `route-remove`, and `route-metrics` retain their existing declarations and profiles. |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | This spec adds no drop-on-full or lossy queue overflow policy on engine-to-plugin event delivery; Phase 4 ends a recipient whose peer-state delivery already failed or timed out rather than silently dropping events. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | This spec adds no command path, scope subcommand, or presentation argument; the optional trailing msgID on `accept-routes` and `reject-routes` identifies the route's UPDATE generation, matching the existing positional argument structure. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | This spec declares no pipe operations or overrides; existing `adj-rib-in` commands retain their generic pipe behavior. |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | This spec adds or modifies no pipe operation; all validation and RIB command changes act directly on command inputs and answers. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | This spec makes no change to `\| implied` rendering or filter-list display. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | This spec preserves participant registrations and registered defaults for `bgp-gr`, `bgp-rib`, `bgp-adj-rib-in`, and `bgp-rpki`, adding no second vocabulary. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | This spec makes no change to `send > to` bus filter semantics. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | This spec makes no change to `receive > from` bus filter semantics. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | This spec makes no change to `send > allow` or `receive > allow` enforcement. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | This spec makes no change to bus filter compilation or commit-time conflict refusal. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | This spec adds no bus arbitration rule; Phase 4 refuses only cyclic plugin startup/delivery dependencies declared at Stage 1. |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | This spec makes no change to bus default/override display. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | This spec makes no change to kernel-vs-VPP data-plane commit validation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | This spec makes no change to kernel-vs-VPP mismatch checks (superseded for start by the 2026-11-26 decision below). |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | This spec does not touch firewall verification or `ze:backend` annotations. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | This spec introduces no data-plane selector or backend name. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | This spec does not modify BGP egress next-hop rewriting or export filter chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | This spec does not touch `policychain.Sets` or export chain next-hop authoring forms. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | This spec does not alter built-in export chain steps or `FilterInsertion`. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | This spec does not alter export-chain next-hop folding or `wireOverride` provenance. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | This spec does not alter `policy { modify }` next-hop validation on import or export chains. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | This spec does not change `next-hop:auto` or locally originated route next-hop preservation. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | This spec does not add automatic filter-chain members or alter the implied-filter seam. |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | This spec does not refuse daemon start over a service configuration; Phase 4's Stage 1 refusal applies only to a cyclic plugin dependency declaration during the plugin handshake (extending `startupSelectedPhases`, under the standing plugin-tier rule in `ai/doors/plugin-infra.md`). |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | This spec does not touch kernel-on-VPP startup, commit, reload, or validation behavior. |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | This spec makes no change to ExaBGP endpoint configuration or runner architecture. |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-26 | "ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`, zero key names, `0:` when absent) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases" | agrees | This spec makes no change to `FormatBase64` UPDATE framing or `ze exabgp plugin` rendering. |
| `docs/architecture/plugin/plugin-system.md` | Design decisions | decision of 2026-11-23 | "With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration" | agrees | This spec makes no change to query-mode declaration probing; Stage 1 dependency-cycle refusal checks the same declared edges at daemon startup. |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | The new pure-BGP interop scenario `bgp-gr-retention-bird` requires no extra kernel capabilities beyond the BGP suite's own and adds no unrelated capability requirement. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A validation decision: the `ze-plugin-engine:batch-validate` RPC (routed over DirectBridge or JSON socket through `opBatchValidate` → `dispatchBatchValidation` to nine-field `request bgp adj-rib-in batch-validate`), or the text commands `request bgp adj-rib-in batch-validate`, `accept-routes`, `reject-routes`.
- A forked `bgp-rib` engine call: `route-install`, `route-remove`, `route-metrics` (both `RouteMetrics` and `RouteMetricsProspective`) RPCs.
- A peer state change: the reactor's peer-state callback into `bgp/server`.

### Transformation Path
1. Decision: sender builds `rpc.ValidationDecision` values → `sdk.Plugin.BatchValidate` (DirectBridge or JSON) → `opBatchValidate` → `dispatchBatchValidation` → nine-field `request bgp adj-rib-in batch-validate` → `batchValidateCommand` → `handleBatchValidateTyped` → `applyDecision`; or individual command handlers `acceptRoutesCommand` / `rejectRoutesCommand` → `applyOperatorDecisionLocked` (address-selected or observational) or `applyDecision` (when `<msgID> <peerOwner> <connectionID>` provenance is given) → `sourceDecisionAdmittedLocked` → pending route, installed route, or `storeEarlyDecision`. The change is at the last arm, in the optional trailing args, and in the answer.
2. Route arrival: `installStructuredNLRIs` (or the JSON ingest) → `applyEarlyDecision` → pending or installed. The change removes the zero arm.
3. Forked RIB: best-path change → `insertLocRIB` / `removeLocRIB` → `Sink.Flush` → SDK RPC on the Sink's context. `extractCandidate` / `nextHopResolved` → `RIBManager.igpDistance` (`RouteMetrics`) and `remoteRIB.prospectiveDistance` (`RouteMetricsProspective`), both deriving their timeout from the plugin run context.
4. Peer down: reactor → `onPeerStateChange` → `deliverPeerState` → `bgp-gr` handler (`request bgp rib source purge-stale`, `request bgp rib source retain-routes`, `request bgp rib source mark-stale` dispatched to `bgp-rib` as commands, then the retention verdict on the bus) → result → `bgp-rib` handler reads `retainedPeers`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Validator plugin → `bgp-adj-rib-in` | `ze-plugin-engine:batch-validate` RPC (DirectBridge or JSON via `dispatchBatchValidation`) or text command; answer carries counters | No |
| Forked `bgp-rib` → engine | SDK RPC bounded by the plugin run context | No |
| Engine → `bgp-gr` → `bgp-rib` | acknowledged event delivery; `bgp-gr` → `bgp-rib` by `request bgp rib source ...` commands | No |

### Changed-Wire Census

**Search evidence:** scope: the whole checkout except `vendor/`, `tmp/`, `.git/`, `plan/` and `website/`, every file type (no include filter), so shell, Python and `.ci` bodies are covered. Terms: `adj-rib-in accept-routes`, `adj-rib-in reject-routes`, `adj-rib-in batch-validate`, `BatchValidate`, `ValidationDecision`, `jsonKeyEarly`, the `Early` field. Positive control: the search returned the known producer `rpki.go` `dispatchBatch`, the engine router `dispatch_batch_validate.go` `dispatchBatchValidation`, and the known consumer `rib_commands.go` `batchValidateCommand` / `handleBatchValidateTyped`. Findings: no shell, Python or other non-Go file writes or reads the wire; two `.ci` files name the commands only in comments (`test/plugin/rpki-validate-accept.ci`, `test/plugin/rpki-per-peer-action.ci`). Not searched: `website/` (excluded), and the YANG search for the method name returned nothing and had no control, so the implementer reruns it with `update-route` as the control before relying on it.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `batch-validate` RPC, decision list | writer | `rpki/rpki.go` `dispatchBatch`, `buildDecisions` | Sets `MsgID`, `PeerOwner`, `ConnectionID` from the event; reactor ids start at 1 | Preserve: never sends 0. Unit test pins that a built decision carries the request's msgID |
| `batch-validate` RPC, decision list | writer | `rpki/rpki_reload.go` `revalidateConfigSnapshot` | `MsgID`, `PeerOwner`, `ConnectionID` from the snapshot route (`RawRoute`) | Preserve. A stored route with msgID 0 does not occur in a daemon (R-4) |
| `batch-validate` RPC, decision list | writer | `pkg/plugin/sdk/sdk_engine.go` `BatchValidate` | Passes the caller's decisions unchanged, DirectBridge or JSON | Preserve the call; comment on the zero meaning rewritten |
| `batch-validate` RPC, decision list | writer | A third-party plugin, any language, outside the repository | Contract page `docs/architecture/api/process-protocol.md` | Change, on purpose: a decision sent ahead of its UPDATE must carry `msg-id` (alongside paired `peer-owner` and `connection-id` for an owned source). Page and type comment rewritten in the same phase |
| `batch-validate` RPC, decision list | reader | `plugin/server/dispatch_registry.go` `opBatchValidate` → `plugin/server/dispatch_batch_validate.go` `dispatchBatchValidation` | Both DirectBridge and JSON rails serialize the list into nine-field `request bgp adj-rib-in batch-validate` and dispatch to the registered receive-store owner | Preserve |
| `batch-validate` RPC, decision list | reader | `adj_rib_in/rib_commands.go` `batchValidateCommand` → `handleBatchValidateTyped` → `adj_rib_in/rib_validation.go` `applyDecision` | Parses nine fields and applies each decision through `sourceDecisionAdmittedLocked` and `applyDecision` | Change: the unversioned no-route arm drops instead of buffering |
| `batch-validate` RPC, result | writer | `adj_rib_in/rib_commands.go` `handleBatchValidateTyped` → `batchValidateCommand` → `plugin/server/dispatch_batch_validate.go` `dispatchBatchValidation` | Builds `BatchValidateResult` JSON (`accepted`, `rejected`, `early`, `dropped`), unmarshaled by `dispatchBatchValidation` | Change: adds `dropped` |
| `batch-validate` RPC, result | reader | `pkg/plugin/sdk/sdk_engine.go` `BatchValidate` | Unmarshals into `BatchValidateResult`; refuses a missing result | Preserve: an added key is read by the new field and ignored by an old reader |
| `batch-validate` RPC, result | reader | `rpki/rpki.go` `dispatchBatch`, `rpki/rpki_reload.go` `revalidateConfigSnapshot` | Both discard the result | Preserve |
| `batch-validate` RPC, result | reader | A third-party plugin | Contract page | Change, additive: new `dropped` key documented |
| Text `request bgp adj-rib-in batch-validate` | writer | `plugin/server/dispatch_batch_validate.go` `dispatchBatchValidation`, a third-party plugin, an operator, or tests (`adj_rib_in/*_test.go`, `rib/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go`) | Nine fields per decision: `action peer family prefix pathID state msgID peerOwner connectionID` (`msgID` is field 7) | Preserve the nine-field form. A `0` in the seventh field now follows the unversioned rule |
| Text `request bgp adj-rib-in batch-validate` | reader | `adj_rib_in/rib_commands.go` `batchValidateCommand` | Parses nine fields (`batchValidateStride = 9`) into `rpc.ValidationDecision` | Change: answer map gains `dropped` |
| Text `accept-routes`, `reject-routes` | writer | A third-party plugin or an operator; in-tree only tests (`adj_rib_in/*_test.go`; `pkg/plugin/sdk/sdk_test.go` uses the unrelated name `bgp rib accept-routes`; `tacacs/authorizer_test.go` uses the command as an authorization sample) | No msgID or provenance arguments exist today, so every such decision is unversioned and address-selected | Change, on purpose: optional trailing `[<msgID> [<peerOwner> <connectionID>]]` is added. With nonzero `<msgID> <peerOwner> <connectionID>` it buffers ahead of an owned UPDATE (subject to `sourceDecisionAdmittedLocked`); with `<msgID>` alone it stays observational when no route is present; without `<msgID>` (or with `0`) it acts on present routes only via `applyOperatorDecisionLocked` and answers `dropped: true` when none is present. Existing callers without trailing args keep parsing |
| Text `accept-routes`, `reject-routes` | reader | `adj_rib_in/rib_commands.go` `acceptRoutesCommand`, `rejectRoutesCommand` → `adj_rib_in/received_operator.go` `applyOperatorDecisionLocked` / `adj_rib_in/rib_validation.go` `applyDecision` | Parses optional trailing args via the shared uint64/provenance parser; calls `applyDecision` when owned provenance is supplied and `applyOperatorDecisionLocked` otherwise | Change: the optional arguments, provenance pairing validation, and `dropped: true` answer. Usage errors (`errAcceptRoutesRequiresPeerFamilyPrefix`, `errRejectRoutesRequiresPeerFamilyPrefix`) name the new arguments |
| UPDATE event, `bgp.message.id` and `StructuredEvent.MessageID` | writer | the engine's event formatting and the structured event (`bgp/server`, `pkg/plugin/rpc/bridge.go`) | Carries the reactor message id (beside `peer-owner` and `connection-id`); never 0 for an UPDATE | Preserve: unchanged, and named because the new contract tells a validator to pass it back |
| UPDATE event, `bgp.message.id` and `StructuredEvent.MessageID` | reader | `rpki/rpki.go` (both event paths), `adj_rib_in/rib.go` ingest, a third-party validator | Read through `Event.GetMsgID` or the structured field | Preserve |

No peer-facing BGP wire changes. The retention work changes when Ze withdraws a peer's routes in one failure case; it adds no message and no field.

### Integration Points
- `adj_rib_in` `applyDecision`, `storeEarlyDecision`, `applyEarlyDecision`, `applyOperatorDecisionLocked` - the unversioned rule lives in `storeEarlyDecision`, the buffer's only writer, and `applyOperatorDecisionLocked` preserves observational vs. owned provenance.
- `rib` run function - creates its context before `setupRemoteRIB`, hands it to `routeinstall.New`, and holds it on `remoteRIB` for `igpDistance` and `prospectiveDistance`.
- `bgp/server` `deliverPeerState` - the error arm; `sortByReverseDependencyTier` - the fallback arm; `plugin/server` `stage1Checks` / `engineStartupSink.onRegistration` - Stage 1 dependency-cycle refusal.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Validation decisions continue through `ze-plugin-engine:batch-validate` (`opBatchValidate` → `dispatchBatchValidation` → nine-field `batchValidateCommand`) and `adj-rib-in` commands into `applyOperatorDecisionLocked` / `applyDecision` and `storeEarlyDecision`; forked route installs and metric lookups continue through `routeinstall.Sink`, `RIBManager.igpDistance`, and `remoteRIB.prospectiveDistance`; GR retention stays on `deliverPeerState` → `bgp-gr` → `request bgp rib source retain-routes` (`retainPhysicalSourceCommand`) → `bgp-rib` |
| No unintended coupling (components stay isolated) | Yes | `bgp/server` names no plugin: `deliverPeerState` applies its failure handling uniformly to any failed state-event recipient and reads ordering only from `registry.StartOrderEdges` and Stage 1 declarations; `bgp-gr` and `bgp-rib` still communicate only through commands and the bus |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends `storeEarlyDecision` (the early-decision buffer's single writer), shares `batchValidateCommand`'s msgID parser with `accept-routes` and `reject-routes`, passes each plugin's existing run context to `routeinstall.New`, and reuses `process.Process.Stop`, `superviseProcess`, and `applyFailurePolicy` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | No hot-path allocation or wire-byte copy is added: one integer `dropped` counter per decision batch, one context reference on `Sink` and `remoteRIB`, and no copy on the UPDATE receive or forward paths |
| Registration over hardcoding, outbound | Yes | No new command, family, capability, or handler is added; `accept-routes` and `reject-routes` extend their existing registered handlers and `batch-validate` extends `BatchValidateResult` additively |
| Registration over hardcoding, inbound | Yes | Searched `handleCommand` in `adj_rib_in/rib_commands.go`, `commandDecls` in `adj_rib_in/rib.go`, and `engineOps` in `plugin/server/dispatch_registry.go`: none learns a new name, and Stage 1 cycle checking derives from declared `Dependencies`, `OptionalDependencies`, and `StartAfter` edges rather than hardcoded plugin lists |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No in-tree non-test producer sends a decision with msgID 0 | Read `rpki.go` `buildDecisions`, `validateNLRIs`, `rpki_reload.go`, `received_update.go` `nextMsgID`; census above | `bgp-rpki` would lose verdicts that race ahead of their UPDATE | Unit test that a decision built from an event carries its message id; census rerun in Phase 2 | validated at design by reading the producers |
| A-2 | A validator that answers an UPDATE event can always pass a msgID (alongside `peer-owner` and `connection-id` for an owned peer), so no validator needs an unversioned decision to be buffered | The event carries `bgp.message.id`, `peer-owner`, and `connection-id` (and `StructuredEvent.MessageID`, `PeerOwner`, `ConnectionID`); the nine-field batch form already takes all three, and the individual commands gain optional `[<msgID> [<peerOwner> <connectionID>]]` here | A validator on some event path without an id would lose ahead-of-route verdicts to the fail-open timeout | `TestIndividualCommandWithMsgIDBuffersAheadOfRoute`; the validator page states the rule | validated at design by reading the event producers and readers |
| A-3 | A recipient that failed or timed out a peer-state delivery is ended through its failure policy | Design decision (main thread, 2026-11-11): such a plugin holds state that disagrees with the engine's, so it is ended like any other server error path (`ai/doors/plugin-infra.md`, Traps); delivery then continues to the remaining recipients. For `bgp-gr` the routes are released for that event, the safe direction: with no live `bgp-gr` nobody would purge routes left stale | A slow but healthy forked plugin is ended and, by policy, restarted | The seam tests of AC-13; the lifecycle scenarios in the Test Plan | accepted as design |
| A-4 | The 2026-09-08 `routes-in: 0` measurement does not reproduce on today's tree with `bgp-gr` internal | `deliverPeerState` is sequential and acknowledged | A second defect exists on the internal path; Phase 1 finds it and the fix is traced to its producer before Phase 5 | Phase 1 reproduction, both transports | unvalidated |
| A-5 | The context `OnStarted` hands the plugin and the one `Run` is given end together | Not read at the producer (`pkg/plugin/sdk/sdk.go`, the started callback's invocation) | The Sink must take whichever context ends when `Run` returns | Read that producer in Phase 3, before the edit | unvalidated |
| A-6 | The tier computation can fail at runtime, and only through a cycle closed by a forked program's Stage 1 edges | Read: `TopologicalGraphTiers` fails only on a cycle; `startupSelectedPhases` checks `selectedPluginGraph`, which carries no edge of an external program; `sortByReverseDependencyTier` adds those edges | If some Stage 1 check already refuses such a cycle, the Stage 1 work of Phase 4 is already done and only the fallback removal remains | Phase 4 searches the Stage 1 handling for a use of `ErrCircularDependency` with `startupSelectedPhases` as the control (the design-time search found none and had no control); `TestStage1RefusesDependencyCycle` | verified reachable by reading; the absence of a Stage 1 check is unvalidated |
| A-8 | Each of `ospf`, `isis`, `static` and `connected` can reach its plugin's run context at the point it builds its Sink | The constructor calls were read; the path from each to its `Run` context was not | A caller builds the Sink before its context exists, as `bgp-rib` does, and its wiring moves | Read each run function in Phase 3 before the edit | unvalidated |
| A-7 | BIRD 3 lists a channel in its GR capability only with `graceful restart on`, and sets the Restart flag only when started in recovery | `bgp_prepare_capabilities` read for the two conditions; the config default (`BGP_GR_AWARE`) read in `config.Y`; how `gr_able` is derived per channel not read | The scenario's `bird.conf` advertises no family and Ze retains nothing: a red that reads as a Ze defect | Read the channel `gr_able` derivation in `bgp.c` before writing the scenario | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A validator keeps sending decisions with no msgID ahead of the route; each is dropped and the route installs fail-open after 30 s | That validator's answers carry `dropped`; the count rises in `show bgp adj-rib-in status` | The answer says so; the validator page says a validator answering an event passes the event's message id, on every entry |
| R-2 | The two old-contract tests are edited to match the new code without judgement | A diff to `TestAcceptNonExistentRoute` or the "unknown prefix still buffers" subtest with no approval row | Both go to an independent `ze-test-judge` agent before the edit; the file carries `RFC6811-2-1` tags |
| R-3 | An unversioned decision is applied to a pending route whose replacement is already announced by a newer buffered decision | A unit test with both present | Unchanged from today and recorded here; the spec does not widen it |
| R-4 | A stored route with msgID 0 meets a versioned decision and waits forever (both sweeps skip it) | Unreachable while the reactor stamps every UPDATE | Not changed here; stated so nobody removes the guard believing it dead |
| R-5 | Moving the context above `setupRemoteRIB` reorders the run function's deferred releases | `-race` run of the package | The context is created first and cancelled last; the existing drain order comments are kept true |
| R-6 | When `bgp-gr` itself fails or times out a peer-down, RFC 4724 Section 4.2 retention is lost for that peer-down: `bgp-rib` releases the routes, and until `bgp-gr` is running again no peer-down is retained. Ze does not meet the Section 4.2 MUST for an event its GR plugin failed on | The plugin-crash report raised by `ProcessManager.ReportCrash` (shown by `ze doctor`), the `Warn` from `deliverPeerState` naming the plugin, the peer and `ze.plugin.delivery.timeout`, and the restart or stay-down line of `applyFailurePolicy` | Accepted (A-3): releasing is the safe direction, because routes left stale with no live `bgp-gr` would never be purged. `bgp-gr`'s declared failure policy decides whether it comes back; the guide states the consequence |
| R-9 | A forked program's Stage 1 refusal for a cycle stops its startup tier (one plugin failure stops the tier, by owner ruling) | Startup fails naming the cycle | That is the intended refusal; the message names both plugins and the edge to remove |
| R-10 | A Sink given a run context fails a flush during an orderly stop, and the Sink's persistent-failure rule closes the client | Error lines from `Sink.Flush` at every stop | A flush that fails because its own context ended is not a persistent failure: Phase 3 reads `Sink.send` and keeps the stop quiet |
| R-7 | The interop scenario kills BIRD in a way that sends a NOTIFICATION, so Ze correctly retains nothing | Ze logs a NOTIFICATION close | The scenario asserts the session ended without one before it asserts the count |
| R-8 | Child 1 and `spec-daemon-drain-before-stop` edit `plugin/process` at the same time | A conflict in `delivery.go` or the failure path | This spec edits `bgp/server/events.go` and calls the existing stop; it changes no file under `plugin/process` unless Phase 4's reading shows the stop cannot be reached from `bgp/server` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Validation: a verdict is lost and a route installs fail-open, or a versioned verdict stops waiting. Retention: a peer's routes are withdrawn at a restart, or a healthy forked plugin is ended. RIB: a forked `bgp-rib` fails engine calls it should have completed |
| How is it reverted? | Each phase is one commit and reverts alone. The `dropped` answer key is additive |
| Who else touches this path? | `spec-rpki-invalid-accepted-and-state-policy` (validation decisions), child 1 and `spec-daemon-drain-before-stop` (`plugin/process`, `plugin/server`), `bgp-rr` and `bgp-rs` (read the retention verdict) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `request bgp adj-rib-in accept-routes` through the daemon, after the route was withdrawn | → | `applyDecision`, `storeEarlyDecision` | `test/plugin/rpki-unversioned-decision-after-withdraw.ci` |
| `ze-plugin-engine:batch-validate` over the JSON codec with msgID 0 and no route | → | `opBatchValidate` → `dispatchBatchValidation` → `batchValidateCommand` → `handleBatchValidateTyped` | `TestBatchValidateJSONReportsDroppedUnversionedDecision` |
| Forked `bgp-rib` run context ends while a route-install is unanswered | → | `setupRemoteRIB`, `Sink.Flush` | `TestRemoteSinkStopsWithPluginContext` |
| Peer with a GR capability loses TCP, `bgp-gr` and `bgp-rib` internal | → | `deliverPeerState`, `handleStructuredState` (gr and rib) | `test/plugin/gr-retained-count-on-tcp-loss.ci` |
| The same with `bgp-gr` forked | → | `deliverViaConn`, text `handleState` | `test/plugin/gr-retained-count-on-tcp-loss-forked.ci` |
| Peer-state delivery to a dependent fails | → | the error arm of `deliverPeerState` | `TestPeerDownDeliveryFailureEndsDependentBeforeDependency` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Validation on; a route is pending; an accept with msgID 0 arrives | The route is installed with the decision's state; answer counts it accepted, `dropped` 0 | Unchanged path: a present route is what an unversioned decision means |
| AC-2 | Validation on; a route is installed; a reject with msgID 0 arrives | The route is removed; `dropped` 0 | Unchanged path, same reason |
| AC-3 | Validation on; no pending and no installed route for the key; a decision with msgID 0 arrives by each entry (`ze-plugin-engine:batch-validate` via DirectBridge and JSON through `dispatchBatchValidation`, nine-field text batch, and individual command) | Nothing is buffered; the answer reports the decision dropped (`dropped` 1 for a batch, `dropped: true` for an individual command); `show bgp adj-rib-in status` shows the running drop count raised by one | The decision is still counted in `accepted` or `rejected` as sent, so existing readers of those keys see what they saw |
| AC-4 | The journal row's sequence: route pending, withdrawn, an accept with msgID 0 arrives, the same key is announced again | The new announcement is pending and resolves by its own verdict or the fail-open timeout; it carries no validation state from the old verdict | The fail-open install after the timeout survives: it is the documented behaviour for a route no verdict reached |
| AC-5 | A decision with a msgID newer than anything stored arrives before its UPDATE | It is buffered (`early` 1), the previous path becomes ineligible, and the UPDATE with that msgID resolves at once on arrival | Buffering survives for versioned decisions: the msgID is the fence |
| AC-6 | Any state of the manager | Every entry in the early-decision buffer has a non-zero msgID; a route arrival is matched against the buffer by equality of msgID only | None: the zero arm in `applyEarlyDecision`, `storeEarlyDecision` and the sweep is removed, because a zero can no longer be stored |
| AC-7 | Forked `bgp-rib`; its run context ends while `Sink.Flush` waits for a `route-install` answer | `Flush` returns, the plugin's workers exit, and the test's cleanup completes; proven by `-race -count=20` on the package | A flush already answered completes normally; the in-process path does not use the Sink |
| AC-8 | Forked `bgp-rib`; its run context ends while `RIBManager.igpDistance` waits for `route-metrics` or `remoteRIB.prospectiveDistance` (`rib_next_hop.go`) waits for `route-metrics` prospective preview | Each lookup returns an unresolved distance promptly and logs the failure once at its existing call site | A cached distance is still served without an engine call |
| AC-9 | A peer that advertised GR for ipv4/unicast with the forwarding bit announces three routes, sends End-of-RIB, and its TCP session ends with no NOTIFICATION; `bgp-gr` internal | `show bgp rib status` reports three routes in for that peer and a GR state row; each route shows as stale | Routes of a family the capability did not name are released: RFC 4724 Section 4.2 retains only "the address families that were previously received in the Graceful Restart Capability" |
| AC-10 | The same with `bgp-gr` forked | The same three stale routes | Same |
| AC-11 | The same peer ends the session with a NOTIFICATION | `show bgp rib status` reports zero routes in for that peer and no GR state row | None retained: RFC 4724 Section 4 normal procedures |
| AC-12 | `bgp-gr` (any recipient ordered ahead of one that depends on it) answers a peer-down in time | The dependency receives the DOWN only after the dependent's result; proven by a recorded order in a seam test | A recipient with no dependency relation to another is served in tier and name order, as today |
| AC-13 | Any recipient's peer-state delivery fails with the daemon running (timeout, handler error or panic) | `deliverPeerState` calls that recipient's `Stop`, so the supervisor raises the plugin-crash report and applies its declared failure policy; it logs a `Warn` naming the plugin, the peer, what was lost and `ze.plugin.delivery.timeout`; delivery then continues to the remaining recipients. With `bgp-gr` as the failed recipient, `bgp-rib` receives the DOWN next and releases the peer's routes | A refused enqueue (the recipient is already stopping) is served past without a second stop; a failure while the daemon is stopping ends nothing, because every plugin is ending then |
| AC-14 | A forked program's Stage 1 declaration closes a dependency cycle with the selected plugins | Stage 1 refuses the declaration with an error naming the two plugins, the edge and the fix (remove the dependency), and that program does not start | A declaration whose edges name plugins that are not selected is accepted: such an edge is inert in `TopologicalGraphTiers` |
| AC-16 | The recipients of a state event still cannot be ordered | No order by name exists. Delivery logs at `Error` naming the recipients and that their order is undefined, and serves them in the order of the compiled-in registry edges alone (the graph the startup check proved acyclic), with the forked programs' declared edges left out because they are what cannot be honoured | The event is still delivered to every recipient: dropping a peer-down would leave every plugin with a peer it believes up. This arm is a named guard that AC-14 makes unreachable; its test builds the cycle through a seam |
| AC-17 | `accept-routes` or `reject-routes` carries trailing `<msgID> <peerOwner> <connectionID>` (all nonzero, matching a live source admitted by `sourceDecisionAdmittedLocked`) with no route present, or trailing `<msgID>` alone (nonzero, observational) with no route present | With `<msgID> <peerOwner> <connectionID>`, the decision is buffered (`early: true`) under that exact `(peerOwner, connectionID)` and the owned UPDATE with that `(peerOwner, connectionID, msgID)` resolves at once on arrival, exactly as nine-field `batch-validate`. With `<msgID>` alone, the decision is buffered only as an observational (`owner: 0, connectionID: 0`) early decision and never matches an owned peer UPDATE (`owner != 0, connectionID != 0`), preserving `received_operator.go` and `source_state.go` | A malformed uint64, an unpaired `peerOwner`/`connectionID` (only one supplied, or one zero and one nonzero), or extra trailing arguments is refused with the usage/provenance error and applies nothing; an owned `(peerOwner, connectionID)` refused by `sourceDecisionAdmittedLocked` buffers nothing |
| AC-18 | A forked `ospf`, `isis`, `static` or `connected` plugin's run context ends while its Sink waits for a `route-install` or `route-remove` answer | `Flush` returns and the plugin's run ends; no caller of `routeinstall.New` passes a background context | The in-process path of each (a shared Loc-RIB) does not build a Sink and is untouched |
| AC-15 | Ze peers with BIRD 3 configured `graceful restart on`; BIRD announces N routes and its process is killed with no NOTIFICATION | Ze holds N stale routes for the BIRD peer during the restart window; after BIRD restarts and sends N-1 routes and End-of-RIB, Ze holds N-1 routes, none stale | The one route BIRD did not re-announce is deleted at End-of-RIB: RFC 4724 Section 4.2 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | A validator plugin without message ids accepts a route that is waiting | command → `acceptRoutesCommand` → `applyDecision` → installed | `TestAcceptPendingRoute` (existing, unchanged) |
| 2 | The same validator answers late, after the peer withdrew the route, and the peer announces it again | command → dropped; UPDATE → pending → own verdict | `test/plugin/rpki-unversioned-decision-after-withdraw.ci` |
| 3 | An operator stops a daemon running a forked `bgp-rib` | stop → run context ends → Sink and metric calls return | `TestRemoteSinkStopsWithPluginContext` |
| 4 | A GR-capable neighbour restarts | TCP loss → reactor → `bgp-gr` → `retain-routes`, `mark-stale` → `bgp-rib` keeps stale routes | `test/plugin/gr-retained-count-on-tcp-loss.ci`, `bgp-gr-retention-bird` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestUnversionedDecisionWithNoRouteIsDropped` | `internal/component/bgp/plugins/adj_rib_in/rib_validation_unversioned_test.go` | AC-3 typed entry: buffer stays empty, the dropped count is 1 | |
| `TestUnversionedDecisionAfterWithdrawLeavesReannouncementPending` | same file | AC-4, the row's reproduction; red on today's tree | |
| `TestUnversionedTextBatchReportsDropped` | same file | AC-3 text batch answer key | |
| `TestUnversionedIndividualCommandReportsDropped` | same file | AC-3 individual commands, accept and reject as two subtests of one question | |
| `TestVersionedDecisionAheadOfRouteStillBuffers` | same file | AC-5 | |
| `TestEarlyDecisionBufferHoldsNoUnversionedEntry` | same file | AC-6: `storeEarlyDecision` given msgID 0 stores nothing | |
| `TestAdjRIBInStatusCountsDroppedDecisions` | same file | AC-3 status counter | |
| `TestBatchValidateJSONReportsDroppedUnversionedDecision` | `internal/component/plugin/server/dispatch_registry_test.go` | AC-3 JSON entry carries `dropped` | |
| `TestBatchValidateResultDecodesDropped` | `pkg/plugin/sdk/sdk_engine_result_test.go` | the SDK reads the new key; a result without it decodes to 0 | |
| `TestRPKIDecisionCarriesEventMessageID` | a new untagged file beside `internal/component/bgp/plugins/rpki/rfc6811_rpki_batch_test.go` | A-1 | |
| `TestRemoteSinkStopsWithPluginContext` | `internal/component/bgp/plugins/rib/rib_remote_test.go` | AC-7 | |
| `TestIGPDistanceStopsWithPluginContext` | same file | AC-8 (`RIBManager.igpDistance`) | |
| `TestProspectiveDistanceStopsWithPluginContext` | same file | AC-8 (`remoteRIB.prospectiveDistance` for CIDR routes) | |
| `TestSocketMetricFeedReselectsRetainedRoutes` (existing) | same file | AC-7 under `-race -count=20`, the row's own reproduction | |
| `TestPeerDownReachesDependencyAfterDependentAnswered` | `internal/component/bgp/server/retention_order_test.go` | AC-12 | |
| `TestPeerDownDeliveryFailureEndsDependentBeforeDependency` | same file | AC-13, timeout case | |
| `TestPeerDownDeliveryFailureDuringStopEndsNothing` | same file | AC-13 survivor | |
| `TestPeerStateOrderFailureNeverOrdersByName` | same file | AC-16: recipients named so that name order and registry order differ | |
| `TestStage1RefusesDependencyCycle` | `internal/component/plugin/server/`, beside the Stage 1 declaration tests | AC-14 | |
| `TestStage1AcceptsEdgeToUnselectedPlugin` | same file | AC-14 survivor | |
| `TestIndividualCommandWithMsgIDBuffersAheadOfRoute` | `adj_rib_in/rib_validation_unversioned_test.go` | AC-17: owned `<msgID> <peerOwner> <connectionID>` buffers for owned UPDATE (accept and reject), and observational `<msgID>`-only never matches an owned UPDATE | |
| `TestIndividualCommandRefusesMalformedMsgID` | same file | AC-17 survivor: malformed uint64 and unpaired `peerOwner`/`connectionID` | |
| `TestIndividualAndBatchParseMsgIDAlike` | same file | one parser: the same token gives the same decision on both forms | |
| `TestOSPFRemoteSinkStopsWithPluginContext` | `internal/plugins/ospf/`, beside `spf_wiring.go` | AC-18 | |
| `TestISISRemoteSinkStopsWithPluginContext` | `internal/plugins/isis/`, beside `spf_wiring.go` | AC-18 | |
| `TestStaticRemoteSinkStopsWithPluginContext` | `internal/plugins/static/` | AC-18 | |
| `TestConnectedRemoteSinkStopsWithPluginContext` | `internal/plugins/connected/` | AC-18 | |
| `TestSinkFlushEndedByItsContextIsNotAPersistentFailure` | `internal/core/rib/routeinstall/sink_test.go` | R-10 | |

Old-contract tests that go red with Phase 2 and are judged, never edited by the implementer alone: `TestAcceptNonExistentRoute` and the subtest "unknown prefix still buffers", both in `adj_rib_in/rfc6811_rib_validation_test.go`. Their expectation (an unversioned decision is buffered) is the behaviour the owner changed; the judge receives this spec's Task and A-2 as the source.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| decision `msgID`, nine-field batch seventh field and individual trailing argument | 0 to 2^64-1 | 2^64-1 (versioned) | N/A: 0 is valid and means unversioned, the same as an absent argument on the individual forms | 2^64 is refused by the shared parser on all three text forms |
| decision `peerOwner` and `connectionID`, nine-field batch fields 8-9 and individual trailing provenance pair | 0 to 2^64-1 (must be paired: both 0 or both nonzero) | 2^64-1 | N/A: 0 paired with 0 means observational; 0 paired with nonzero is refused | 2^64 is refused by the shared parser on all three text forms |

### Functional Tests
Each is drafted under `test/draft/plugin/` and promoted (`docs/functional-tests.md`, "Writing a Test: Draft First"). Observers are compiled fixtures that return an error on a failed assertion and wait on a condition, never a duration.

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rpki-unversioned-decision-after-withdraw` | `test/plugin/rpki-unversioned-decision-after-withdraw.ci` | Validation on; peer announces, withdraws; observer sends `accept-routes`, reads `dropped: true`; peer announces again; the route is pending, not installed as Valid | |
| `gr-retained-count-on-tcp-loss` | `test/plugin/gr-retained-count-on-tcp-loss.ci` | AC-9. Carries the `RFC requirement: RFC4724-4.2-3` positive tag | |
| `gr-retained-count-on-tcp-loss-forked` | `test/plugin/gr-retained-count-on-tcp-loss-forked.ci` | AC-10 | |
| `gr-notification-retains-no-route` | `test/plugin/gr-notification-retains-no-route.ci` | AC-11, the negative twin; carries the `RFC4724-4.2-3` negative tag | |

Lifecycle scenarios (`ai/rules/testing.md`, "Lifecycle Regressions"):

| Scenario | Reached? | Proof or reason |
|----------|----------|-----------------|
| A plugin's process ends after it declared, per `failure-policy` value | Yes: AC-13 ends a process by a new trigger | Seam test `TestPeerDownDeliveryFailureEndsDependentBeforeDependency` asserts the generation ended and the policy path ran. A `.ci` cannot make a forked `bgp-gr` miss a delivery timeout on demand, which is why this takes the seam. Run for a forked recipient (timeout) and an internal one (handler error) |
| A plugin exits during the stop drain or teardown | Yes: the survivor of AC-13 | `TestPeerDownDeliveryFailureDuringStopEndsNothing` |
| A plugin fails a startup stage; restart bound; `respawn` leaf; signal during stop; reload after shutdown; verify or apply crash; reload context ends in handshake; cross-plugin dispatch at startup | No | This spec changes no startup, reload, signal or restart-bound code; it calls the existing stop |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-gr-retention-bird` | `test/interop/scenarios/bgp-gr-retention-bird/` | BIRD 3 | AC-15: Ze as Receiving Speaker retains and marks stale the routes of a BIRD peer that restarts, then replaces them and deletes what was not re-announced | |

Peer source read before the scenario is written (BIRD 3.2.1, local BIRD checkout): `bgp_prepare_capabilities` (`proto/bgp/packets.c`) for what the capability carries; the per-channel `gr_able` derivation in `proto/bgp/bgp.c` (A-7); `bgp_handle_graceful_restart` is BIRD's own Receiving Speaker and is the reference for per-family retention, not something this scenario exercises. How the lab ends a peer's process without a NOTIFICATION was not read: the implementer reads the checker of `bgp-graceful-restart-frr` and the lab engine first. The discrimination walk (`docs/architecture/testing/interop.md`, "Prove a scenario discriminates") breaks `retainPhysicalSourceCommand` (`internal/component/bgp/plugins/rib/rib_source_commands.go`, `request bgp rib source retain-routes`, called by `gr/gr.go:490`) so the flag is never set and observes the scenario red; the result goes through `./le rfc record` because the functional carrier holds the tag.

## Files to Modify
- `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` - unversioned rule in `storeEarlyDecision` and `applyDecision`; zero arms removed from `applyEarlyDecision` and the sweep; comments
- `internal/component/bgp/plugins/adj_rib_in/rib_commands.go`, `received_operator.go` - `dropped` in the three answers and in `status`; optional trailing `[<msgID> [<peerOwner> <connectionID>]]` on `accept-routes` and `reject-routes` sharing nine-field `batchValidateCommand`'s uint64 and provenance-pairing validation; `applyOperatorDecisionLocked` propagates the dropped outcome while preserving observational-only scope when provenance is omitted
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - the `RawRoute.MsgID` comment; the drop counter field
- `pkg/plugin/rpc/bridge.go` - `BatchValidateResult` gains the dropped counter; the `ValidationDecision` comment
- `pkg/plugin/sdk/sdk_engine.go` - `BatchValidate` comment
- `internal/component/bgp/plugins/rib/rib.go`, `rib_remote.go`, `rib_next_hop.go` - context created before `setupRemoteRIB`, passed to the Sink, held on `remoteRIB` for both `RIBManager.igpDistance` (`rib_remote.go`) and `remoteRIB.prospectiveDistance` (`rib_next_hop.go`)
- `internal/plugins/ospf/spf_wiring.go`, `internal/plugins/isis/spf_wiring.go`, `internal/plugins/static/register.go`, `internal/plugins/connected/connected.go` - each Sink takes its plugin's run context
- `internal/core/rib/routeinstall/sink.go` - the `New` comment (it records the background context as the wiring's choice); `send` if R-10 needs it
- `internal/component/bgp/server/events.go` - the error arm of `deliverPeerState` calls `Stop`; the name-order arm of `sortByReverseDependencyTier` is removed
- `internal/component/plugin/server/stage1_checks.go`, `startup.go` - Stage 1 cycle refusal for a forked program's declared edges in `stage1Checks` / `engineStartupSink.onRegistration`
- `docs/architecture/api/process-protocol.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/plugin-development/protocol.md`, `docs/guide/rpki.md` - the decision contract, and the rule that a validator answering an event passes the event's message id
- `docs/architecture/forked-route-install.md` - the Sink's context
- `docs/architecture/ospf/ospf-8-spf-rib.md`, `docs/architecture/isis/isis-9-spf-rib.md`, `docs/architecture/static-routes.md` - declared by the changed wiring files' `// Design:` headers and not read for this spec: Phase 3 reads each and edits it where it describes the forked Sink, else records it unaffected with the reason
- `docs/guide/graceful-restart.md`, `docs/architecture/plugin/plugin-relationships.md` - what a failed peer-down delivery costs and does
- `rfc/short/rfc4724.md` - `RFC4724-4.2-3` proof state (through the `ze-rfc` skill)
- `plan/journal/late-write-lands-on-the-successor.md`, `plan/journal/stop-leaves-the-work-running.md`, `plan/journal/unwired-feature.md` - each owned row's status, in the commit that fixes it
- `internal/le/interoplab/bgp/names.go` and the scenario's register file - the new scenario's name and checker

## Files to Create
- `internal/component/bgp/plugins/adj_rib_in/rib_validation_unversioned_test.go`
- `test/plugin/rpki-unversioned-decision-after-withdraw.ci`, `test/plugin/gr-retained-count-on-tcp-loss.ci`, `test/plugin/gr-retained-count-on-tcp-loss-forked.ci`, `test/plugin/gr-notification-retains-no-route.ci`, and their compiled observers under `internal/test/fixture/`
- `test/interop/scenarios/bgp-gr-retention-bird/` (`ze.conf`, `bird.conf`) and its checker under `internal/le/interoplab/bgp/`
- `rfc/discrimination/` record for the tagged carrier, written by `./le rfc record`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No RPC or leaf is added; one result key is added to an existing RPC. Phase 2 reruns the YANG search with a control and updates the method's output node if one exists |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | Yes | `accept-routes` and `reject-routes` gain optional trailing `[<msgID> [<peerOwner> <connectionID>]]` in `adj_rib_in/rib_commands.go`; three answers gain `dropped`. Both declarations (`registry.Registration` and the Stage 1 `commandDecls`) keep the same names, so `./le plugin declarations check` is unaffected |
| CLI grammar (keyword before value) | Yes | The arguments are positional, matching the preceding fields and nine-field `batch-validate`'s trailing `msgID peerOwner connectionID`: the existing form of these commands is extended, no new shape is introduced. Read `ai/rules/cli.md` in Phase 2 and record there if it asks for more |
| Editor autocomplete | N-A | No config change |
| Functional test for new RPC/API | Yes | `test/plugin/rpki-unversioned-decision-after-withdraw.ci` for the changed answer |
| Pipe completeness | N-A | No new output path |
| Env var registration | No | `ze.plugin.delivery.timeout` exists; the new message names it |
| Doctor check for runtime dependencies | N-A | No new runtime dependency |
| Doctor code per problem class | Yes | Three classes. (1) A recipient ended after a failed peer-state delivery: the plugin-crash report `ProcessManager.ReportCrash` raises (`reportCodePluginCrash`), which `ze doctor` shows; no new code. (3) A Stage 1 dependency cycle: a startup refusal, reported as every Stage 1 refusal is. (2) Unversioned decisions dropped: a counter in `show bgp adj-rib-in status`; it is a sender's contract error told to the sender, so no doctor check is proposed, and that judgement is flagged for review |
| Prometheus counters/metrics | No | `bgp-adj-rib-in` registers no metrics today (searched the package for the metrics registry: none); the count is in the status answer |
| BGP family surface | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The GR feature page under `features/`: its evidence gains the retention test and the interop scenario; read its level with `./le feature report` |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` if it shows the `accept-routes`, `reject-routes`, `batch-validate` or `show bgp adj-rib-in status` answers |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md` (`batch-validate` paragraph); `docs/architecture/api/commands.md` if it lists the adj-rib-in validation commands |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/graceful-restart.md`; `docs/guide/rpki.md` if it describes third-party validators |
| 7 | Wire format changed? | No | No BGP bytes change |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md`, `docs/plugin-development/protocol.md` |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc4724.md` row `RFC4724-4.2-3`, with the discrimination record |
| 10 | Test infrastructure changed? | No | New tests use existing runner features |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/plugin/rib-storage-design.md` "Validation eligibility"; `docs/architecture/plugin/plugin-relationships.md`. `docs/architecture/core-design.md` is declared by a changed file's `// Design:` header and was not read for this spec: the phase that edits that file reads the section the header names and edits it if the peer-state delivery or validation text there becomes wrong, else records it unaffected with the reason |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/release/fixes/spec-single-owner-lifecycle-3-bgp-plugins.md`; `ai/digests/rib.md` and `ai/digests/plugin-transport.md` are named here as checked for the peer-down and batch-validate lines |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The nine-field batch example in `rib-storage-design.md` stays valid; verify after the edit |

## Design presentation

### What changes

This spec closes three single-owner lifecycle bypasses in the BGP plugins and the shared forked route-install path. First, `bgp-adj-rib-in` stops buffering unversioned (msgID 0) validation decisions when no route is present: an unversioned decision applies only to a pending or installed route currently stored, and otherwise drops without buffering, reporting `dropped` in `BatchValidateResult` / command answers (across nine-field `batch-validate` routed through `dispatchBatchValidation` on both DirectBridge and JSON rails, and the individual commands) and incrementing a counter in `show bgp adj-rib-in status`; `accept-routes` and `reject-routes` gain optional trailing `[<msgID> [<peerOwner> <connectionID>]]` arguments (parsed by the same uint64 and provenance-pairing check as nine-field `batch-validate`) so validators answering an owned UPDATE ahead of ingestion can buffer with exact physical provenance (subject to `sourceDecisionAdmittedLocked`), while an address-only `<msgID>` without owner/connection remains observational and never matches an owned peer session. Second, all five callers of `routeinstall.New` (`bgp-rib`, `ospf`, `isis`, `static`, `connected`) plus both `bgp-rib` forked next-hop metric lookups (`RIBManager.igpDistance` in `rib_remote.go` and `remoteRIB.prospectiveDistance` in `rib_next_hop.go`) derive from the plugin's own run context instead of `context.Background()`, so stopping a forked plugin unblocks any in-flight engine RPC cleanly without triggering a spurious persistent-failure close. Third, `bgp/server`'s `deliverPeerState` stops a recipient whose peer-state delivery failed or timed out (when the daemon is not stopping) via `proc.Stop()` before continuing, Stage 1 (`stage1Checks` / `engineStartupSink.onRegistration`) refuses any forked plugin declaration that closes a dependency cycle with the selected plugins, the name-order fallback in `sortByReverseDependencyTier` is replaced by a registry-order guard with an `Error` log, and RFC 4724 Section 4.2 route retention (driven by `bgp-gr` calling `request bgp rib source retain-routes` in `rib_source_commands.go`) is proven through three functional `.ci` tests (internal, forked `bgp-gr`, and NOTIFICATION negative twin) plus a BIRD 3 interop scenario (`bgp-gr-retention-bird`).

### Before and after

1. **Unversioned validation decision after a route is withdrawn:**
   - Before: `request bgp adj-rib-in accept-routes 192.0.2.1 ipv4/unicast 10.0.0.0/24 0 1` with no route stored returns `status: ok, early: true` and buffers an observational msgID-0 decision (and a nine-field `batch-validate` decision with msgID 0 buffers on an owned source), silently accepting and installing a later announcement of `10.0.0.0/24` without validating it.
   - After: the same command with no route stored buffers nothing, returns `status: ok, dropped: true`, increments the dropped counter in `show bgp adj-rib-in status`, and leaves a later re-announcement pending for its own verdict; passing the event's provenance tuple (`accept-routes 192.0.2.1 ipv4/unicast 10.0.0.0/24 0 1 42 7 11`) buffers ahead of owned UPDATE `(owner 7, conn 11, msg 42)` and returns `early: true`, while passing `<msgID>` alone stays observational. Nine-field `batch-validate` answers (returned through `dispatchBatchValidation` on both transports) include `dropped` alongside `accepted`, `rejected`, and `early`.

2. **Stopping a forked route-installing plugin while an engine RPC is unanswered:**
   - Before: `bgp-rib`, `ospf`, `isis`, `static`, and `connected` create `routeinstall.Sink` with `context.Background()`, and both `RIBManager.igpDistance` (`rib_remote.go`) and `remoteRIB.prospectiveDistance` (`rib_next_hop.go`) derive their five-second timeout from `context.Background()`, so ending the plugin's run context leaves in-flight `route-install`, `route-remove`, and `route-metrics` calls hanging until transport teardown or timeout.
   - After: every `routeinstall.New` caller, `RIBManager.igpDistance`, and `remoteRIB.prospectiveDistance` use the plugin's run context, so ending `Run` cancels pending engine RPCs immediately and `Sink.send` exits quietly on run-context cancellation without treating orderly shutdown as a persistent RPC failure.

3. **Peer-down delivery failure and cyclic Stage 1 dependencies:**
   - Before: if a forked `bgp-gr` misses `ze.plugin.delivery.timeout` or errors on peer-down, `deliverPeerState` logs a `Warn` and continues directly to `bgp-rib` (which releases the routes) while leaving the failed `bgp-gr` alive with stale state; if a forked plugin's Stage 1 edges close a dependency cycle, `sortByReverseDependencyTier` silently falls back to ascending process name order (which can serve `bgp-rib` before `gr`).
   - After: `deliverPeerState` calls `proc.Stop()` on the failed recipient when the daemon is running (so the supervisor raises the plugin-crash report and applies its declared failure policy) and logs a `Warn` naming the plugin, peer, lost event, and `ze.plugin.delivery.timeout` before continuing; Stage 1 (`stage1Checks` / `engineStartupSink.onRegistration`) refuses a forked declaration that closes a dependency cycle naming both plugins and the fix; `sortByReverseDependencyTier` never falls back to name order; and RFC 4724 Section 4.2 retention is proven by `gr-retained-count-on-tcp-loss.ci`, `gr-retained-count-on-tcp-loss-forked.ci`, `gr-notification-retains-no-route.ci`, and `bgp-gr-retention-bird`.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Bind an unversioned buffered decision to "the generation pending when it was stored" | With no route present there is no generation to bind to, and a late verdict for a withdrawn route is indistinguishable from an early verdict for a not-yet-arrived route; with a route present the decision already applies immediately without buffering. |
| Log unversioned drops only, or fail the whole `batch-validate` call with an error | Silent drops hide a sender's contract error, while returning an error would abort an entire batch over one benign withdrawal race; an additive `dropped` counter informs the sender without breaking other entries in the batch. |
| Guard every reader of `earlyDecisions` against msgID 0 | Enforcing non-zero msgID at `storeEarlyDecision` (the buffer's single writer) makes zero entries unrepresentable and removes the zero arm from every reader and sweep. |
| Wait without bound on a stuck peer-state recipient, or end it only when another plugin depends on it | An unbounded wait hangs the reactor's peer-state path on one stuck plugin; any plugin that missed a state event holds state inconsistent with the engine regardless of whether another plugin depends on it. |
| Defer route release in `bgp-rib` until a GR verdict arrives | `deliverPeerState` already delivers to `bgp-gr` before `bgp-rib` sequentially and synchronously; adding a deferral timer in `bgp-rib` would create a second owner of the retention decision. |
| Fix only `bgp-rib`'s `routeinstall.New` call and leave the other four plugins on `context.Background()` | All five callers share the same constructor and the same hang-on-stop defect; fixing all five establishes one rule across the codebase. |
| Keep ascending name order as the fallback in `sortByReverseDependencyTier` | Name order silently inverts `bgp-gr` and `bgp-rib` under operator-chosen aliases; refusing cycles at Stage 1 prevents the cycle, and the defensive fallback uses the startup-verified compiled registry order rather than names. |

### Documents departed from

1. **`docs/architecture/plugin/rib-storage-design.md`**, section `Validation eligibility`:
   - Current statement: `"Zero explicitly means an unversioned decision for the current route, or the next arrival if no route is stored."`
   - After this work (Phase 2): states that msgID 0 applies only to a pending or installed route currently stored; when no route is stored, an unversioned decision is not buffered, is dropped, and is counted in `dropped` (and in `show bgp adj-rib-in status`). Versioned decisions (non-zero msgID) keep buffering ahead of their UPDATE.

2. **`docs/architecture/api/process-protocol.md`**, section `Forked route install (\`route-install\` / \`route-remove\`)`:
   - Current statement: `"A zero message ID selects the current or next generation within that exact source, never a same-address sibling."`
   - After this work (Phase 2): states that a zero message ID selects only a route currently pending or installed within that exact source (never the next arrival), drops without buffering when none is stored, and reports the count in `BatchValidateResult.Dropped` (`dropped`); validators answering an UPDATE event ahead of route ingestion pass the event's non-zero message ID.

### Choices for Thomas

none

## Implementation Steps

Each phase is one commit that stands alone, with its pages and its journal row.

1. **Phase: Wiring and reproduction (MANDATORY FIRST)**
   - Write the failing unit tests of Phase 2 to 4 and the two retention `.ci` drafts.
   - Run the retention drafts on today's tree, internal and forked. Record in Design Insights what `show bgp rib status` answers after the down edge. If the internal run shows zero routes, trace it to its producer (candidates, none verified: the family list `bgp-gr` passes with `on-down` not matching the family the routes are in; the peer never marked up in `bgp-rib` so the down edge is not taken) and correct A-4 before Phase 5.
   - Verify: the unversioned tests are red for the stated reason; the retention outcome is recorded.
2. **Phase: Unversioned validation decision**
   - `storeEarlyDecision` stores only a decision with a msgID and reports whether it stored; `applyDecision` returns the dropped outcome to its three callers (`handleBatchValidateTyped`, `applyOperatorDecisionLocked`, and direct owned `acceptRoutesCommand`/`rejectRoutesCommand`); the answers and `status` carry the count; zero arms and their comments removed; `RawRoute.MsgID`, `ValidationDecision` and SDK comments rewritten.
   - The msgID and provenance-pairing parse of nine-field `batchValidateCommand` becomes shared; `acceptRoutesCommand` and `rejectRoutesCommand` call it for their optional trailing `[<msgID> [<peerOwner> <connectionID>]]`, preserving `applyOperatorDecisionLocked` when provenance is omitted and calling `applyDecision` directly when paired nonzero `(peerOwner, connectionID)` is given; the two usage errors name the arguments.
   - Send the two old-contract RFC6811-tagged test updates (`TestAcceptNonExistentRoute` and the `"unknown prefix still buffers"` subtest in `internal/component/bgp/plugins/adj_rib_in/rfc6811_rib_validation_test.go`) through an independent `ze-test-judge` agent (recording approval via `./le test weakened approve`) before editing either test.
   - Pages: `process-protocol.md`, `rib-storage-design.md`, `plugin-development/protocol.md`. Promote the `.ci`. Journal row 2026-10-23.
   - Tests: the `adj_rib_in`, `plugin/server`, `sdk` and `rpki` rows above.
3. **Phase: Forked engine calls end with the plugin**
   - Read where the SDK invokes the started callback (A-5) and each of the four other run functions (A-8). In `bgp-rib`, create the run context before `setupRemoteRIB`, pass it to the Sink, and keep it on `remoteRIB` for both `RIBManager.igpDistance` (`rib_remote.go`) and `remoteRIB.prospectiveDistance` (`rib_next_hop.go`). Give the `ospf`, `isis`, `static` and `connected` Sinks their plugin's run context. Rewrite the `routeinstall.New` comment; settle R-10 in `Sink.send`.
   - Page: `docs/architecture/forked-route-install.md`.
   - Tests: the six Sink rows, `TestIGPDistanceStopsWithPluginContext`, and `TestProspectiveDistanceStopsWithPluginContext`, then the `rib` package under `-race -count=20` with the output pasted. Journal rows 2026-10-12 and 2026-11-11.
4. **Phase: Peer-state delivery keeps its order when a recipient fails**
   - Search `stage1_checks.go` / `startup.go` for an existing cycle refusal, with `startupSelectedPhases` as the control (A-6).
   - Error arm of `deliverPeerState`: when the daemon is not stopping, call the failed recipient's `Stop`, log, then continue.
   - Stage 1 (`stage1Checks` / `engineStartupSink.onRegistration`): refuse a forked program's declaration whose edges close a cycle with the selected plugins. Remove the name-order arm of `sortByReverseDependencyTier`; its replacement is the guard of AC-16.
   - Pages: `graceful-restart.md`, `plugin-relationships.md`, and the Stage 1 section of `process-protocol.md` for the refusal. Tests: the `retention_order_test.go` rows and the two Stage 1 rows.
5. **Phase: Retention proof through the daemon**
   - Promote the three GR `.ci` tests; add the `RFC4724-4.2-3` tags; run `./le rfc discriminate` and `./le rfc record` for the carrier; update the summary row. Journal row 2026-09-08.
6. **Phase: Interop scenario `bgp-gr-retention-bird`**
   - Read the BIRD source named above; write the scenario and its checker; run the discrimination walk; commit a recording and its replay test if the checker reads answers a recording can hold.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | The four user stories each run end to end |
| Correctness | No reader of the early-decision buffer still tests for msgID zero; `storeEarlyDecision` is the buffer's only writer; the retention tests assert a count and the stale mark, never only "daemon still up" |
| Naming | Answer key is `dropped`, kebab-case status key; no plugin name appears in `bgp/server` |
| Data flow | `bgp-rib` still learns GR retention only from `request bgp rib source retain-routes` (`retainPhysicalSourceCommand`); no new call from `bgp-gr` into `bgp-rib` |
| Operator errors and doctor codes | The new `Warn` and `Error` lines say what failed, why, and name `ze.plugin.delivery.timeout`; the doctor row's two answers are confirmed or corrected |
| Rule: process boundary | Every unit test of the decision path runs typed and JSON; every retention test runs internal and forked |
| Rule: stale comments | The comments that promise buffering for msgID zero (`RawRoute.MsgID`, `ValidationDecision`, `applyDecision`, `applyEarlyDecision`, `storeEarlyDecision`) are rewritten in Phase 2 |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Unversioned rule | `./le test unit pkg path ./internal/component/bgp/plugins/adj_rib_in/` |
| No zero arm left | a search of `adj_rib_in/rib_validation.go` for a msgID compared with zero shows only comparisons that still need it, each explained in review |
| Sink and metric context | a search of `rib/rib_remote.go` and `rib/rib_next_hop.go` for a background context returns nothing |
| Delivery order | `./le test unit pkg path ./internal/component/bgp/server/` with a run filter for the four new tests |
| Retention through the daemon | `./le test functional plugin` selecting the three GR tests and the RPKI test by name |
| Interop | `./le test integration interop` selecting `bgp-gr-retention-bird` (run by the main thread) |
| RFC proof | `./le rfc check` |
| Pages | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A decision is still pre-validated for the whole batch before any state changes; the dropped count stays within the batch bound (256) |
| Fail-open | A dropped reject must not shorten the pending timeout or install the route early |
| Resource exhaustion | Dropped decisions allocate nothing and store nothing; the status counter is one integer |
| Availability | Ending a recipient on a delivery failure cannot be triggered by a peer: the trigger is a local plugin's own failure to answer |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Phase 1 shows retention fails on the internal path | Trace to the producer, add the cause to Current Behavior and an AC, then continue |
| An assumption A-3, A-5, A-6 or A-7 breaks | Record it, correct the affected AC, report to the main thread before coding past it |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- A validator and `bgp-adj-rib-in` both receive the UPDATE event, so a verdict can legitimately arrive before the route is ingested. The buffer exists for that race and stays for it; what leaves is the buffer's acceptance of a decision that names no UPDATE. Every entry can therefore name one: the individual commands gain the msgID the batch form has.
- The parent's first plan for retention (`bgp-rib` defers its release) solved a race the code no longer has. The remaining hole is in the engine's error arm, which is why this child edits `bgp/server` and not `bgp-rib`.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `late-write-lands-on-the-successor.md` | 2026-10-23 | `applyEarlyDecision`, msgID 0 |
| `stop-leaves-the-work-running.md` | 2026-10-12 | `setupRemoteRIB` Sink context |
| `stop-leaves-the-work-running.md` | 2026-11-11 | `ospf`, `isis`, `static`, `connected` Sinks on a background context |
| `unwired-feature.md` | 2026-09-08 | `RIBManager.handleState`, GR retention on peer-down |

## Rows found already fixed
| Class file | Date | Surface | Fixed by | Evidence read |
|------------|------|---------|----------|---------------|
| `announced-state-never-replayed.md` | 2026-08-31 | BMP: a collector told a peer is up is never given its routes | ee78b53a3d "BMP: complete RFC 7854 peer identity and replay" (2026-09-24) | `replayPeerLocked` (`bmp_replay.go`) is called from `primeSender`, `bounceMonitoredPeers` and `handleSenderState` (`bmp_events.go`). The journal row still reads open and is made true by the parent's row pass |

## Rows considered and not owned
| Row or finding | Owner | Why |
|----------------|-------|-----|
| BMP: a peer established before BMP observed it has no snapshot (RFC 7854 Section 3.1) | `plan/release/fixes/spec-bmp-late-attach-and-forked.md` | A coverage gap of the replay, not a lifecycle bypass; recorded there by another agent |
| BMP: not correct when forked (structured events only) | the same spec | Same |
| `unwired-feature.md` 2026-09-08, `enforcePathsLimit` and `parseGRCapValue` | not this child | Different producers and mechanisms; they share only the date and the finding session |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Drop an unversioned decision that finds no route | Bind it to "the generation pending when it was stored" (the journal row's own suggestion) | With no route present there is no generation to bind to, and a verdict ahead of its route cannot be told from a verdict behind a withdrawn one. With a route present the decision is applied at once and never stored |
| Tell the sender with a `dropped` counter in the answer | Log only; answer an error | A silent drop is a value that is silently wrong; an error would fail a batch for one benign race (the route was withdrawn) |
| Put the rule in `storeEarlyDecision` | Guard each reader | It is the buffer's only writer, so the readers lose their zero arm instead of each keeping one |
| End any recipient whose peer-state delivery failed or timed out | Wait without bound; keep warning and moving on; end it only when another recipient depends on it | An unbounded wait hangs the reactor's peer-state path on one plugin; moving on is the hole. A plugin that missed a state event holds state that disagrees with the engine's whoever depends on it, so the rule has no tier condition |
| Give the individual commands the batch form's msgID | Leave them unversioned-only | A validator can answer ahead of the route on any entry; without the argument that entry would lose verdicts to the fail-open timeout |
| Refuse a cycle at Stage 1 and remove the name-order arm | Keep ordering by name; order the cycle members by some other tiebreak | A name order is a plausible default that inverts `bgp-gr` and `bgp-rib` under operator-chosen names. No order of a cycle is right, so the cycle is refused where it is declared |
| One context rule for all five Sink callers | Fix `bgp-rib` alone | Same constructor, same defect; a rule held by one caller is rewritten wrong by the next |
| No change to `bgp-rib`'s release logic | Defer the release until a GR verdict arrives | The order already delivers the verdict first; a second owner of the decision is what the parent's principle forbids |

## Known Limitations
- A decision sent with no msgID before `bgp-adj-rib-in` has taken the UPDATE is dropped and reported; the route resolves by the fail-open timeout. That form is for an operator acting on a route that is present; a validator passes the event's message id.
- When `bgp-gr` itself fails on a peer-down, that peer-down retains nothing (R-6).
- The two BMP residuals: `plan/release/fixes/spec-bmp-late-attach-and-forked.md`.

## RFC Documentation (Scope: protocol)

The retention tests cite RFC 4724 Section 4.2 and Section 4 with the sentences quoted in Required Reading, byte-exact. Any function added in `bgp/server` that exists to keep the retention order carries `// RFC 4724 Section 4.2` at the call, per `ai/rules/rfc-compliance.md`.

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
- [ ] AC-1..AC-18 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. GitHub CI verifies the pushed commit; a red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `pkg/*`), not library-only
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
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
