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
| Updated | 2026-11-11 |

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
| 2 | One run of a plugin that installs routes when forked (`bgp-rib`, `ospf`, `isis`, `static`, `connected`) | The context its `Run` is given | The forked route-install Sink of all five, and the `bgp-rib` next-hop metric lookup, call the engine on a context nothing cancels |
| 3 | One peer-down, decided by `bgp-gr` and acted on by `bgp-rib` | The ordered, acknowledged peer-state delivery | A forked `bgp-gr` that misses the delivery timeout is skipped with a warning and `bgp-rib` releases the routes without its verdict; nothing proves the retained count on any path |

Goals:

1. A validation decision with msgID 0 applies only to a route present when it
   arrives. With none present it is not buffered, and its sender is told.
   A decision that carries a msgID still waits for its UPDATE, and every
   entry can carry one: `accept-routes` and `reject-routes` gain the optional
   msgID argument the batch form already has, so a validator that answers an
   UPDATE event ahead of the route passes the event's message id and loses
   nothing. The form without a msgID is for an operator at the CLI.
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
  → Constraint: it states "A zero explicitly selects the current route, or the next route when none is stored." Rewritten with the behaviour. It also states an empty or null result is an SDK error: preserved.
- [ ] `docs/architecture/plugin/plugin-relationships.md`
  → Constraint: `bgp-gr` depends on `bgp` and `bgp-rib`; `bgp-rr` and `bgp-rs` read the `bgp-gr/retention` verdict. The verdict event is queued in a consumer's own delivery channel ahead of its DOWN, which is why delivery order matters to three plugins, not one.
- [ ] `docs/guide/graceful-restart.md`
  → Constraint: the guide promises retention, on a peer-down that is not a NOTIFICATION close, of the families the peer's capability named, and that a forked `bgp-gr` is ordered as an internal one. It is silent on what happens when a forked `bgp-gr` does not answer a peer-down in time.
- [ ] `ai/digests/rib.md`
  → Constraint: on peer down, "unless retained for GR (`retainedPeers`), the peer's `PeerRIB` is released". `retainedPeers` is written only by the `request bgp rib retain-routes` command.
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
- The event a validator receives carries the id it must pass: `bgp.message.id` in the JSON event (`docs/architecture/api/json-format.md`; read by `Event.GetMsgID`, `internal/component/bgp/event.go`) and `StructuredEvent.MessageID` on the bridge (`pkg/plugin/rpc/bridge.go`). The same value becomes `RawRoute.MsgID` in `bgp-adj-rib-in`.
- `routeinstall.New` has five callers and all five pass a background context: `bgp-rib`, `ospf`, `isis`, `static`, `connected`.
- A dependency cycle among state-event recipients is reachable at runtime: the startup check (`startupSelectedPhases`) reads `selectedPluginGraph`, which gives an external program no edges, while the delivery sort adds the edges that program declared at Stage 1.
- `bgp/server` holds each recipient as a `*process.Process`, whose `Stop` ends the generation; `superviseProcess` then runs `applyFailurePolicy`, which raises the plugin-crash report through `ProcessManager.ReportCrash` and applies the declared policy.
- Peer-state delivery is already sequential and acknowledged (`deliverPeerState`). The hole is the error arm: a failed delivery is logged and the loop moves on.
- `setupRemoteRIB` runs before the plugin's context is created, so the fix moves the context up; it cannot be passed where it does not yet exist.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` - `applyDecision` applies a decision to the pending route, else to the installed route, else calls `storeEarlyDecision`. Every msgID comparison is guarded by "msgID is not zero", so a zero decision matches any generation. `applyEarlyDecision`, called for every newly arrived route while validation is on, applies a buffered decision whose msgID is zero to that arrival. `sweepExpiredEarlyDecisions` drops a buffered decision after one minute (`earlyDecisionTimeout`).
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - `acceptRoutesCommand` and `rejectRoutesCommand` build a `rpc.ValidationDecision` with no msgID and answer `early: true` when it was buffered. `batchValidateCommand` parses seven fields per decision, the seventh the msgID. `handleBatchValidateTyped` is the one typed entry; it answers `accepted`, `rejected`, `early`.
- [ ] `internal/component/bgp/plugins/adj_rib_in/rib.go` - the `RawRoute.MsgID` comment promises "Validation decisions with MsgID zero apply to the current route, or buffer until its arrival." `installStructuredNLRIs` and the JSON ingest both remove the installed route, build the pending route and call `applyEarlyDecision`.
- [ ] `internal/component/bgp/plugins/rpki/rpki.go`, `rpki_reload.go` - `buildDecisions` sets `MsgID` from the request, which `validateNLRIs` filled from the structured event's `MessageID` or the JSON event's message id. `dispatchBatch` and `revalidateConfigSnapshot` are the two callers of `sdk.Plugin.BatchValidate`, and both discard the result.
- [ ] `internal/component/bgp/reactor/received_update.go` - `nextMsgID` returns the counter after adding one: the first id is 1.
- [ ] `pkg/plugin/rpc/bridge.go`, `pkg/plugin/sdk/sdk_engine.go`, `internal/component/plugin/server/dispatch_registry.go` - `ValidationDecision` and `BatchValidateResult` types; `BatchValidate` takes the typed bridge slot internal and the JSON method `ze-plugin-engine:batch-validate` forked; `opBatchValidate` is the JSON reader.
- [ ] `internal/component/bgp/plugins/rib/rib_remote.go` - `setupRemoteRIB` builds the Sink with `routeinstall.New` on a background context; `igpDistance` calls `RouteMetrics` on a background context. `insertLocRIB` and `removeLocRIB` call `Sink.Flush` under the remote mutex.
- [ ] `internal/core/rib/routeinstall/sink.go` - `New` stores the context it is given and uses it for every `RouteInstall` and `RouteRemove`. Its comment says the forked OSPF and IS-IS wiring also passes a background context.
- [ ] `internal/component/bgp/plugins/rib/rib.go` - the run function calls `setupRemoteRIB` when no in-process Loc-RIB exists, and only later creates its context with `sdk.SignalContext`, cancelled by a deferred call when `Run` returns. `handleState` and `handleStructuredState` release the peer's Adj-RIB-In on the down edge unless `retainedPeers` holds the peer.
- [ ] `internal/component/bgp/plugins/gr/gr.go`, `register.go` - on a DOWN, `handleStructuredState` dispatches `purge-stale`, `retain-routes <peer> on-down <families>` and `mark-stale` synchronously, each through `dispatchCommand` on a background context, then returns; a deferred `emitRetention` puts the `bgp-gr/retention` verdict on the bus. `grDependencies` names `bgp-rib`.
- [ ] `internal/component/bgp/server/events.go` - `deliverPeerState` sorts recipients with `sortByReverseDependencyTier` (dependents first), then for each one enqueues the event and blocks on its result before the next. A refused enqueue (`Deliver` false: the process is stopping) and a result carrying an error both continue to the next recipient; the error is a `Warn`. When the tier computation fails, the sort falls back to ascending process name.
- [ ] `internal/component/plugin/process/delivery.go` - `sendBatch` calls the bridge directly for an internal plugin (no timeout) and `deliverViaConn` for a forked one, bounded by `ze.plugin.delivery.timeout`, default 5 s (`defaultDeliveryTimeout`). `deliverBatch` hands the batch error to every waiting result.
- [ ] `internal/component/bgp/plugins/bmp/bmp_events.go`, `bmp_replay.go` - `replayPeerLocked` is called from `primeSender`, `bounceMonitoredPeers` and `handleSenderState` (commit ee78b53a3d).
- [ ] `internal/plugins/ospf/spf_wiring.go`, `internal/plugins/isis/spf_wiring.go` (`initSPF`), `internal/plugins/static/register.go`, `internal/plugins/connected/connected.go` - each builds its forked Sink with `routeinstall.New` on a background context when `locrib.Default()` answers nil. How each of the four reaches its plugin's run context was not read.
- [ ] `internal/component/plugin/server/startup.go` - `startupSelectedPhases` refuses a cycle over `selectedPluginGraph`, whose comment says it "gives external programs no compiled-in edges"; Stage 1 declared edges are not in that graph.
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
- The seven-field text batch form and the existing answer keys `accepted`, `rejected`, `early`, `status`.
- Sequential, acknowledged peer-state delivery; the peer-up barrier accounting in `deliverPeerState`.
- The operator form `request bgp rib retain-routes <selector>` and the `on-down` family form `bgp-gr` sends.
- The in-process `bgp-rib` path (shared Loc-RIB, `igpcost.Lookup`): untouched.

**Behavior to change:**
- A msgID-0 decision that finds neither a pending nor an installed route is not buffered. Its sender is told, and it is counted.
- `accept-routes` and `reject-routes` take an optional trailing msgID, parsed by the one function the batch form uses (a decimal unsigned 64-bit value, as `batchValidateCommand` parses its seventh field today). The protocol changes directly: no compatibility path (pre-release rule).
- Every forked route-install Sink, and `bgp-rib`'s `igpDistance`, use the run context of the plugin that owns them.
- A recipient whose peer-state delivery failed or timed out, with the daemon running, is ended through its failure policy before delivery continues to the remaining recipients.
- A dependency cycle that a forked program's Stage 1 declaration closes is refused at Stage 1. The name-order fallback is removed; if the sort still fails, delivery says so loudly and never orders by name.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| UPDATE generation identity | `adj_rib_in/rib.go` `RawRoute.MsgID`, `ingestedMsgID`; `reactor/received_update.go` `nextMsgID` | Stamps every stored route with the reactor message id of its UPDATE; never zero for a received UPDATE | Extends: the identity becomes mandatory for anything that waits. A decision without it can only act on what is present |
| Early-decision buffer and its expiry | `adj_rib_in/rib_validation.go` `storeEarlyDecision`, `applyEarlyDecision`, `sweepExpiredEarlyDecisions`, `earlyDecisionTimeout` | Holds a decision that arrived before its route, one minute at most unless a known predecessor is waiting | Extends: `storeEarlyDecision`, the only writer of the buffer, admits only a decision with a msgID, so every reader loses its zero arm. Not replaced |
| Pending route and fail-open timeout | `adj_rib_in/rib_validation.go` `pendingRoute`, `sweepExpiredPending` | A route awaiting its verdict is installed as not-validated after the timeout | Does not apply as a change: it is what resolves a route whose unversioned verdict was dropped, and that consequence is stated in the pages |
| Decision answer counters | `pkg/plugin/rpc/bridge.go` `BatchValidateResult` | Reports `accepted`, `rejected`, `early` to the sender | Extends: one more counter, `dropped`, so a sender can tell a dropped decision from an applied one |
| Plugin run context | `rib/rib.go` run function, `sdk.SignalContext`; `OnStarted` hands a context to `runAIGPSelection` and `runRemoteMetrics` | Ends when the plugin's `Run` returns or a signal arrives | Extends: the Sink and `igpDistance` take it. No second context is created |
| Route-install Sink | `internal/core/rib/routeinstall/sink.go` `New`, `Flush`; `docs/architecture/forked-route-install.md` | Already takes a context for every RPC; all five callers hand it a background one, and its comment records that | Extends: the constructor is unchanged, its comment and all five callers change. One rule, no per-plugin variant |
| Batch msgID parsing | `adj_rib_in/rib_commands.go` `batchValidateCommand` | Parses the seventh field of a decision as the msgID | Extends: that parse becomes the one function the two individual commands also call for their optional last argument |
| Event message id | `bgp.message.id` (`docs/architecture/api/json-format.md`), `StructuredEvent.MessageID` | Gives every UPDATE event the id `bgp-adj-rib-in` stores with the route | Does not change: it is what a validator passes back, and the validator page says so |
| Startup cycle refusal | `plugin/server/startup.go` `startupSelectedPhases`, `selectedPluginGraph`; `registry.ErrCircularDependency` | Refuses a cycle among compiled-in edges before any plugin starts | Extends: the same refusal is applied to the edges a forked program declares at Stage 1, the only edges that check never sees |
| Ordered peer-state delivery | `bgp/server/events.go` `deliverPeerState`, `sortByReverseDependencyTier`; `docs/guide/graceful-restart.md` | Dependents first, one at a time, each acknowledged | Extends: the error arm ends the failed recipient before serving a recipient that depends on its answer. The order and the acknowledgement are unchanged |
| Delivery graph and start-order edges | `registry.StartOrderEdges`, `registry.TopologicalGraphTiers`, Stage 1 `Dependencies`, `OptionalDependencies`, `StartAfter` | Give the tiers the sort uses, for internal and forked processes alike | Extends: a failed tier computation is reported, not hidden behind a name sort |
| Plugin failure path | `process.Process.Stop`; `plugin/server/failure_policy.go` `superviseProcess`, `applyFailurePolicy`; `process.ProcessManager.ReportCrash`; `ai/doors/plugin-infra.md`, Traps | `Stop` ends a generation; the supervisor then reports the crash and applies the declared policy | Reused as is: `deliverPeerState` calls `Stop` on the failed recipient and the supervisor does the rest. Child 1 owns any change to the path itself |
| GR retention flag | `rib/rib.go` `retainedPeers`, `rib/rib_commands.go` `retainRoutes` | Set by `retain-routes`, read on the down edge, cleared on the next up | Does not change. The parent's earlier plan ("`bgp-rib` defers the release") is dropped: the order already guarantees the flag is set first when `bgp-gr` answers |
| `bgp-gr/retention` verdict | `gr/gr.go` `emitRetention`, `internal/component/bgp/retention` | Tells `bgp-rr` and `bgp-rs` which families a DOWN retains | Does not change; named because the same delivery-failure hole reaches its readers |
| BMP replay on collector connect | `bmp/bmp_replay.go` `replayPeerLocked` | Replays a monitored peer's snapshot | Does not apply: already fixed (see "Rows found already fixed") |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A validation decision: the `ze-plugin-engine:batch-validate` RPC (typed through DirectBridge, JSON when forked), or the text commands `request bgp adj-rib-in batch-validate`, `accept-routes`, `reject-routes`.
- A forked `bgp-rib` engine call: `route-install`, `route-remove`, `route-metrics` RPCs.
- A peer state change: the reactor's peer-state callback into `bgp/server`.

### Transformation Path
1. Decision: sender builds `rpc.ValidationDecision` values → `sdk.Plugin.BatchValidate` (bridge or JSON) or the command dispatcher → `handleBatchValidateTyped` or the individual command handlers → `applyDecision` → pending route, installed route, or `storeEarlyDecision`. The change is at the last arm and in the answer.
2. Route arrival: `installStructuredNLRIs` (or the JSON ingest) → `applyEarlyDecision` → pending or installed. The change removes the zero arm.
3. Forked RIB: best-path change → `insertLocRIB` / `removeLocRIB` → `Sink.Flush` → SDK RPC on the Sink's context. `extractCandidate` → `igpDistance` → `RouteMetrics`.
4. Peer down: reactor → `onPeerStateChange` → `deliverPeerState` → `bgp-gr` handler (`purge-stale`, `retain-routes`, `mark-stale` dispatched to `bgp-rib` as commands, then the retention verdict on the bus) → result → `bgp-rib` handler reads `retainedPeers`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Validator plugin → `bgp-adj-rib-in` | typed slot, JSON RPC, or text command; answer carries counters | No |
| Forked `bgp-rib` → engine | SDK RPC bounded by a context | No |
| Engine → `bgp-gr` → `bgp-rib` | acknowledged event delivery; `bgp-gr` → `bgp-rib` by text command | No |

### Changed-Wire Census

**Search evidence:** scope: the whole checkout except `vendor/`, `tmp/`, `.git/`, `plan/` and `website/`, every file type (no include filter), so shell, Python and `.ci` bodies are covered. Terms: `adj-rib-in accept-routes`, `adj-rib-in reject-routes`, `adj-rib-in batch-validate`, `BatchValidate`, `ValidationDecision`, `jsonKeyEarly`, the `Early` field. Positive control: the search returned the known producer `rpki.go` `dispatchBatch` and the known consumer `rib_commands.go` `handleBatchValidateTyped`. Findings: no shell, Python or other non-Go file writes or reads the wire; two `.ci` files name the commands only in comments (`test/plugin/rpki-validate-accept.ci`, `test/plugin/rpki-per-peer-action.ci`). Not searched: `website/` (excluded), and the YANG search for the method name returned nothing and had no control, so the implementer reruns it with `update-route` as the control before relying on it.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `batch-validate` RPC, decision list | writer | `rpki/rpki.go` `dispatchBatch`, `buildDecisions` | Sets `MsgID` from the event; reactor ids start at 1 | Preserve: never sends 0. Unit test pins that a built decision carries the request's msgID |
| `batch-validate` RPC, decision list | writer | `rpki/rpki_reload.go` `revalidateConfigSnapshot` | `MsgID` from the snapshot route's `msg-id`, which is `RawRoute.MsgID` | Preserve. A stored route with msgID 0 does not occur in a daemon (R-4) |
| `batch-validate` RPC, decision list | writer | `pkg/plugin/sdk/sdk_engine.go` `BatchValidate` | Passes the caller's decisions unchanged, bridge or JSON | Preserve the call; comment on the zero meaning rewritten |
| `batch-validate` RPC, decision list | writer | A third-party plugin, any language, outside the repository | Contract page `docs/architecture/api/process-protocol.md` | Change, on purpose: a decision sent ahead of its UPDATE must carry `msg-id`. Page and type comment rewritten in the same phase |
| `batch-validate` RPC, decision list | reader | `plugin/server/dispatch_registry.go` `opBatchValidate` (JSON) and the bridge setter beside it | Both hand the list to `rpc.GetBatchValidator` | Preserve |
| `batch-validate` RPC, decision list | reader | `adj_rib_in/rib_commands.go` `handleBatchValidateTyped` → `adj_rib_in/rib_validation.go` `applyDecision` | Registered by `rpc.RegisterBatchValidator` | Change: the unversioned no-route arm |
| `batch-validate` RPC, result | writer | `adj_rib_in/rib_commands.go` `handleBatchValidateTyped` | Builds `BatchValidateResult` | Change: adds `dropped` |
| `batch-validate` RPC, result | reader | `pkg/plugin/sdk/sdk_engine.go` `BatchValidate` | Unmarshals into `BatchValidateResult`; refuses a missing result | Preserve: an added key is read by the new field and ignored by an old reader |
| `batch-validate` RPC, result | reader | `rpki/rpki.go` `dispatchBatch`, `rpki/rpki_reload.go` `revalidateConfigSnapshot` | Both discard the result | Preserve |
| `batch-validate` RPC, result | reader | A third-party plugin | Contract page | Change, additive: new `dropped` key documented |
| Text `request bgp adj-rib-in batch-validate` | writer | A third-party plugin or an operator; in-tree only tests (`adj_rib_in/*_test.go`, `rib/draft_ietf_sidrops_aspa_verification_rib_aspa_retention_test.go`) | Seven fields per decision, the last the msgID | Preserve the form. A `0` in the seventh field now follows the unversioned rule |
| Text `request bgp adj-rib-in batch-validate` | reader | `adj_rib_in/rib_commands.go` `batchValidateCommand` | Parses into the same decision type | Change: answer map gains `dropped` |
| Text `accept-routes`, `reject-routes` | writer | A third-party plugin or an operator; in-tree only tests (`adj_rib_in/*_test.go`; `pkg/plugin/sdk/sdk_test.go` uses the unrelated name `bgp rib accept-routes`; `tacacs/authorizer_test.go` uses the command as an authorization sample) | No msgID argument exists today, so every such decision is unversioned | Change, on purpose: an optional trailing msgID is added. With it the decision follows the versioned rules, buffering included; without it the decision acts on a present route only and the answer carries `dropped: true` when there is none. Existing callers that pass no msgID keep parsing |
| Text `accept-routes`, `reject-routes` | reader | `adj_rib_in/rib_commands.go` `acceptRoutesCommand`, `rejectRoutesCommand` | Call `applyDecision` | Change: the optional argument and the answer. The two usage errors (`errAcceptRoutesRequiresPeerFamilyPrefix`, `errRejectRoutesRequiresPeerFamilyPrefix`) name the new argument |
| UPDATE event, `bgp.message.id` and `StructuredEvent.MessageID` | writer | the engine's event formatting and the structured event (`bgp/server`, `pkg/plugin/rpc/bridge.go`) | Carries the reactor message id; never 0 for an UPDATE | Preserve: unchanged, and named because the new contract tells a validator to pass it back |
| UPDATE event, `bgp.message.id` and `StructuredEvent.MessageID` | reader | `rpki/rpki.go` (both event paths), `adj_rib_in/rib.go` ingest, a third-party validator | Read through `Event.GetMsgID` or the structured field | Preserve |

No peer-facing BGP wire changes. The retention work changes when Ze withdraws a peer's routes in one failure case; it adds no message and no field.

### Integration Points
- `adj_rib_in` `applyDecision`, `storeEarlyDecision`, `applyEarlyDecision` - the unversioned rule lives in `storeEarlyDecision`, the buffer's only writer.
- `rib` run function - creates its context before `setupRemoteRIB` and hands it in.
- `bgp/server` `deliverPeerState` - the error arm; `sortByReverseDependencyTier` - the fallback arm.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | `bgp/server` names no plugin: the rule is "a recipient a later recipient depends on", read from the tiers |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | no hot-path allocation added: one integer counter per decision batch |
| Registration over hardcoding, outbound | No | no new command, family or handler |
| Registration over hardcoding, inbound | No | searched: the command switch in `handleCommand`, `commandDecls`, the engine-op table. None learns a new name |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No in-tree non-test producer sends a decision with msgID 0 | Read `rpki.go` `buildDecisions`, `validateNLRIs`, `rpki_reload.go`, `received_update.go` `nextMsgID`; census above | `bgp-rpki` would lose verdicts that race ahead of their UPDATE | Unit test that a decision built from an event carries its message id; census rerun in Phase 2 | validated at design by reading the producers |
| A-2 | A validator that answers an UPDATE event can always pass a msgID, so no validator needs an unversioned decision to be buffered | The event carries `bgp.message.id` and `StructuredEvent.MessageID` (read); the batch form already takes a msgID; the individual commands gain one here | A validator on some event path without an id would lose ahead-of-route verdicts to the fail-open timeout | `TestIndividualCommandWithMsgIDBuffersAheadOfRoute`; the validator page states the rule | validated at design by reading the event producers and readers |
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
| `ze-plugin-engine:batch-validate` over the JSON codec with msgID 0 and no route | → | `opBatchValidate` → `handleBatchValidateTyped` | `TestBatchValidateJSONReportsDroppedUnversionedDecision` |
| Forked `bgp-rib` run context ends while a route-install is unanswered | → | `setupRemoteRIB`, `Sink.Flush` | `TestRemoteSinkStopsWithPluginContext` |
| Peer with a GR capability loses TCP, `bgp-gr` and `bgp-rib` internal | → | `deliverPeerState`, `handleStructuredState` (gr and rib) | `test/plugin/gr-retained-count-on-tcp-loss.ci` |
| The same with `bgp-gr` forked | → | `deliverViaConn`, text `handleState` | `test/plugin/gr-retained-count-on-tcp-loss-forked.ci` |
| Peer-state delivery to a dependent fails | → | the error arm of `deliverPeerState` | `TestPeerDownDeliveryFailureEndsDependentBeforeDependency` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Validation on; a route is pending; an accept with msgID 0 arrives | The route is installed with the decision's state; answer counts it accepted, `dropped` 0 | Unchanged path: a present route is what an unversioned decision means |
| AC-2 | Validation on; a route is installed; a reject with msgID 0 arrives | The route is removed; `dropped` 0 | Unchanged path, same reason |
| AC-3 | Validation on; no pending and no installed route for the key; a decision with msgID 0 arrives by each of the four entries (typed, JSON, text batch, individual command) | Nothing is buffered; the answer reports the decision dropped (`dropped` 1 for a batch, `dropped: true` for an individual command); `show bgp adj-rib-in status` shows the running drop count raised by one | The decision is still counted in `accepted` or `rejected` as sent, so existing readers of those keys see what they saw |
| AC-4 | The journal row's sequence: route pending, withdrawn, an accept with msgID 0 arrives, the same key is announced again | The new announcement is pending and resolves by its own verdict or the fail-open timeout; it carries no validation state from the old verdict | The fail-open install after the timeout survives: it is the documented behaviour for a route no verdict reached |
| AC-5 | A decision with a msgID newer than anything stored arrives before its UPDATE | It is buffered (`early` 1), the previous path becomes ineligible, and the UPDATE with that msgID resolves at once on arrival | Buffering survives for versioned decisions: the msgID is the fence |
| AC-6 | Any state of the manager | Every entry in the early-decision buffer has a non-zero msgID; a route arrival is matched against the buffer by equality of msgID only | None: the zero arm in `applyEarlyDecision`, `storeEarlyDecision` and the sweep is removed, because a zero can no longer be stored |
| AC-7 | Forked `bgp-rib`; its run context ends while `Sink.Flush` waits for a `route-install` answer | `Flush` returns, the plugin's workers exit, and the test's cleanup completes; proven by `-race -count=20` on the package | A flush already answered completes normally; the in-process path does not use the Sink |
| AC-8 | Forked `bgp-rib`; its run context ends while `igpDistance` waits for `route-metrics` | The lookup returns an unresolved distance and logs the failure once at the existing call site | A cached distance is still served without an engine call |
| AC-9 | A peer that advertised GR for ipv4/unicast with the forwarding bit announces three routes, sends End-of-RIB, and its TCP session ends with no NOTIFICATION; `bgp-gr` internal | `show bgp rib status` reports three routes in for that peer and a GR state row; each route shows as stale | Routes of a family the capability did not name are released: RFC 4724 Section 4.2 retains only "the address families that were previously received in the Graceful Restart Capability" |
| AC-10 | The same with `bgp-gr` forked | The same three stale routes | Same |
| AC-11 | The same peer ends the session with a NOTIFICATION | `show bgp rib status` reports zero routes in for that peer and no GR state row | None retained: RFC 4724 Section 4 normal procedures |
| AC-12 | `bgp-gr` (any recipient ordered ahead of one that depends on it) answers a peer-down in time | The dependency receives the DOWN only after the dependent's result; proven by a recorded order in a seam test | A recipient with no dependency relation to another is served in tier and name order, as today |
| AC-13 | Any recipient's peer-state delivery fails with the daemon running (timeout, handler error or panic) | `deliverPeerState` calls that recipient's `Stop`, so the supervisor raises the plugin-crash report and applies its declared failure policy; it logs a `Warn` naming the plugin, the peer, what was lost and `ze.plugin.delivery.timeout`; delivery then continues to the remaining recipients. With `bgp-gr` as the failed recipient, `bgp-rib` receives the DOWN next and releases the peer's routes | A refused enqueue (the recipient is already stopping) is served past without a second stop; a failure while the daemon is stopping ends nothing, because every plugin is ending then |
| AC-14 | A forked program's Stage 1 declaration closes a dependency cycle with the selected plugins | Stage 1 refuses the declaration with an error naming the two plugins, the edge and the fix (remove the dependency), and that program does not start | A declaration whose edges name plugins that are not selected is accepted: such an edge is inert in `TopologicalGraphTiers` |
| AC-16 | The recipients of a state event still cannot be ordered | No order by name exists. Delivery logs at `Error` naming the recipients and that their order is undefined, and serves them in the order of the compiled-in registry edges alone (the graph the startup check proved acyclic), with the forked programs' declared edges left out because they are what cannot be honoured | The event is still delivered to every recipient: dropping a peer-down would leave every plugin with a peer it believes up. This arm is a named guard that AC-14 makes unreachable; its test builds the cycle through a seam |
| AC-17 | `accept-routes` or `reject-routes` carries a trailing msgID newer than anything stored and no route is present | The decision is buffered (`early: true`), and the UPDATE with that msgID resolves at once on arrival, exactly as the batch form | A msgID that does not parse as a decimal unsigned 64-bit value is refused with the usage error; nothing is applied |
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
| `TestIGPDistanceStopsWithPluginContext` | same file | AC-8 | |
| `TestSocketMetricFeedReselectsRetainedRoutes` (existing) | same file | AC-7 under `-race -count=20`, the row's own reproduction | |
| `TestPeerDownReachesDependencyAfterDependentAnswered` | `internal/component/bgp/server/retention_order_test.go` | AC-12 | |
| `TestPeerDownDeliveryFailureEndsDependentBeforeDependency` | same file | AC-13, timeout case | |
| `TestPeerDownDeliveryFailureDuringStopEndsNothing` | same file | AC-13 survivor | |
| `TestPeerStateOrderFailureNeverOrdersByName` | same file | AC-16: recipients named so that name order and registry order differ | |
| `TestStage1RefusesDependencyCycle` | `internal/component/plugin/server/`, beside the Stage 1 declaration tests | AC-14 | |
| `TestStage1AcceptsEdgeToUnselectedPlugin` | same file | AC-14 survivor | |
| `TestIndividualCommandWithMsgIDBuffersAheadOfRoute` | `adj_rib_in/rib_validation_unversioned_test.go` | AC-17, accept and reject as two subtests | |
| `TestIndividualCommandRefusesMalformedMsgID` | same file | AC-17 survivor | |
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
| decision msgID, batch seventh field and individual trailing argument | 0 to 2^64-1 | 2^64-1 (versioned) | N/A: 0 is valid and means unversioned, the same as an absent argument on the individual forms | 2^64 is refused by the shared parser on all three text forms |

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

Peer source read before the scenario is written (BIRD 3.2.1, `/home/thomas/Code/gitlab.nic.cz/labs/bird`): `bgp_prepare_capabilities` (`proto/bgp/packets.c`) for what the capability carries; the per-channel `gr_able` derivation in `proto/bgp/bgp.c` (A-7); `bgp_handle_graceful_restart` is BIRD's own Receiving Speaker and is the reference for per-family retention, not something this scenario exercises. How the lab ends a peer's process without a NOTIFICATION was not read: the implementer reads the checker of `bgp-graceful-restart-frr` and the lab engine first. The discrimination walk (`docs/architecture/testing/interop.md`, "Prove a scenario discriminates") breaks `retainRoutes` so the flag is never set and observes the scenario red; the result goes through `./le rfc record` because the functional carrier holds the tag.

## Files to Modify
- `internal/component/bgp/plugins/adj_rib_in/rib_validation.go` - unversioned rule in `storeEarlyDecision` and `applyDecision`; zero arms removed from `applyEarlyDecision` and the sweep; comments
- `internal/component/bgp/plugins/adj_rib_in/rib_commands.go` - `dropped` in the three answers and in `status`
- `internal/component/bgp/plugins/adj_rib_in/rib.go` - the `RawRoute.MsgID` comment; the drop counter field
- `pkg/plugin/rpc/bridge.go` - `BatchValidateResult` gains the dropped counter; the `ValidationDecision` comment
- `pkg/plugin/sdk/sdk_engine.go` - `BatchValidate` comment
- `internal/component/bgp/plugins/rib/rib.go`, `rib_remote.go` - context created before `setupRemoteRIB`, passed to the Sink, held for `igpDistance`
- `internal/plugins/ospf/spf_wiring.go`, `internal/plugins/isis/spf_wiring.go`, `internal/plugins/static/register.go`, `internal/plugins/connected/connected.go` - each Sink takes its plugin's run context
- `internal/core/rib/routeinstall/sink.go` - the `New` comment (it records the background context as the wiring's choice); `send` if R-10 needs it
- `internal/component/bgp/server/events.go` - the error arm of `deliverPeerState` calls `Stop`; the name-order arm of `sortByReverseDependencyTier` is removed
- `internal/component/plugin/server/` Stage 1 declaration handling - the cycle refusal for a forked program's declared edges (the file is named in Phase 4 after its search)
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
| CLI commands/flags | Yes | `accept-routes` and `reject-routes` gain an optional trailing msgID in `adj_rib_in/rib_commands.go`; three answers gain a key. Both declarations (`registry.Registration` and the Stage 1 `commandDecls`) keep the same names, so `./le plugin declarations check` is unaffected |
| CLI grammar (keyword before value) | Yes | The argument is positional, like the six that precede it and like the batch form's seventh field: the existing form of these commands is extended, no new shape is introduced. Read `ai/rules/cli.md` in Phase 2 and record there if it asks for more |
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
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The seven-field batch example in `rib-storage-design.md` stays valid; verify after the edit |

## Implementation Steps

Each phase is one commit that stands alone, with its pages and its journal row.

1. **Phase: Wiring and reproduction (MANDATORY FIRST)**
   - Write the failing unit tests of Phase 2 to 4 and the two retention `.ci` drafts.
   - Run the retention drafts on today's tree, internal and forked. Record in Design Insights what `show bgp rib status` answers after the down edge. If the internal run shows zero routes, trace it to its producer (candidates, none verified: the family list `bgp-gr` passes with `on-down` not matching the family the routes are in; the peer never marked up in `bgp-rib` so the down edge is not taken) and correct A-4 before Phase 5.
   - Verify: the unversioned tests are red for the stated reason; the retention outcome is recorded.
2. **Phase: Unversioned validation decision**
   - `storeEarlyDecision` stores only a decision with a msgID and reports whether it stored; `applyDecision` returns the dropped outcome to its three callers; the answers and `status` carry the count; zero arms and their comments removed; `RawRoute.MsgID`, `ValidationDecision` and SDK comments rewritten.
   - The msgID parse of `batchValidateCommand` becomes one function; `acceptRoutesCommand` and `rejectRoutesCommand` call it for their optional last argument; the two usage errors name the argument.
   - Judge the two old-contract tests before touching them.
   - Pages: `process-protocol.md`, `rib-storage-design.md`, `plugin-development/protocol.md`. Promote the `.ci`. Journal row 2026-10-23.
   - Tests: the `adj_rib_in`, `plugin/server`, `sdk` and `rpki` rows above.
3. **Phase: Forked engine calls end with the plugin**
   - Read where the SDK invokes the started callback (A-5) and each of the four other run functions (A-8). In `bgp-rib`, create the run context before `setupRemoteRIB`, pass it to the Sink, keep it for `igpDistance`. Give the `ospf`, `isis`, `static` and `connected` Sinks their plugin's run context. Rewrite the `routeinstall.New` comment; settle R-10 in `Sink.send`.
   - Page: `docs/architecture/forked-route-install.md`.
   - Tests: the six Sink rows and `TestIGPDistanceStopsWithPluginContext`, then the `rib` package under `-race -count=20` with the output pasted. Journal rows 2026-10-12 and 2026-11-11.
4. **Phase: Peer-state delivery keeps its order when a recipient fails**
   - Search the Stage 1 handling for an existing cycle refusal, with `startupSelectedPhases` as the control (A-6).
   - Error arm of `deliverPeerState`: when the daemon is not stopping, call the failed recipient's `Stop`, log, then continue.
   - Stage 1: refuse a forked program's declaration whose edges close a cycle with the selected plugins. Remove the name-order arm of `sortByReverseDependencyTier`; its replacement is the guard of AC-16.
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
| Data flow | `bgp-rib` still learns retention only from `retain-routes`; no new call from `bgp-gr` into `bgp-rib` |
| Operator errors and doctor codes | The new `Warn` and `Error` lines say what failed, why, and name `ze.plugin.delivery.timeout`; the doctor row's two answers are confirmed or corrected |
| Rule: process boundary | Every unit test of the decision path runs typed and JSON; every retention test runs internal and forked |
| Rule: stale comments | The comments that promise buffering for msgID zero (`RawRoute.MsgID`, `ValidationDecision`, `applyDecision`, `applyEarlyDecision`, `storeEarlyDecision`) are rewritten in Phase 2 |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Unversioned rule | `./le test unit pkg path ./internal/component/bgp/plugins/adj_rib_in/` |
| No zero arm left | a search of `adj_rib_in/rib_validation.go` for a msgID compared with zero shows only comparisons that still need it, each explained in review |
| Sink context | a search of `rib/rib_remote.go` for a background context returns nothing |
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
