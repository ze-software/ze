# Spec: single-owner-lifecycle-5-session-end-locals

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Three shipped-daemon defects an operator meets in production: an LDP session that ended cleanly is retried as if it had failed and costs an OSPF link out, an L2TP subscriber that hung up stays in the subscriber table with an accounting record nobody stops, and a FlowSpec discard rule stays at every peer after the plugin that announced it is gone. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-single-owner-lifecycle.md` (parent: decision record and row routing; no code dependency). No dependency on children 1 to 4: this child edits no file they edit |
| Phase | 1/3 |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 5 of `plan/release/fixes/spec-single-owner-lifecycle.md`, which holds the
owner's decisions and the routing of every journal row. Design tier: strongest.
Implementation tier: development.

Three local fixes. In each one a session or a plugin ends, and the code that
reacts reads the wrong thing or acts in the wrong order. None needs a new owner
object or a generation counter: each closes one bypass at the place it happens.

| Part | Symptom today | Goal |
|------|---------------|------|
| LDP session end | Every session end is recorded as a failed setup, so a session that ran for days is redialled only after 15 seconds, then 30, up to 2 minutes, and the backoff never clears. A setup attempt that never became a session still publishes SessionDown, which costs an OSPF LDP-sync link out and re-arms its stuck alert | A session that reached operational clears the backoff and publishes SessionDown once. One that never reached it records a setup failure and publishes nothing |
| L2TP session events | A teardown on another goroutine can publish SessionDown before the SessionUp or SessionIPAssigned of the same session. The late event then re-creates the subscriber entry, starts a RADIUS accounting session nobody stops, and re-injects the subscriber route | The events and route notifications of one session reach their readers in the order the session's state changed |
| DDoS FlowSpec exit | The plugin withdraws on a config apply only. Every other end of the plugin leaves the FlowSpec rule at the peers, and a restarted plugin does not know the rule exists | Each end of the plugin that can still reach the BGP engine withdraws the rule. A rule the plugin could not withdraw is recorded, withdrawn when the plugin next starts, and reported by `ze doctor` until then |

The web leaf row (`late-write-lands-on-the-successor`, 2026-10-10) is not this
root cause and is not owned here: the parent re-homes it.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
  → Constraint: every plugin can run forked. A fix that reaches another unit does it through the EventBus, the SDK or a command, never a Go call into another package's state.
  → Constraint: a plugin persists runtime state through the SDK state calls, never a file write: a forked process has no store.
- [ ] `ai/doors/routing-protocols.md` - LDP and OSPF
  → Constraint: a wire-visible LDP change owes an interop scenario under `test/interop/scenarios/`, named for protocol, case and peer, proven red with the fix reverted and the image rebuilt.
  → Constraint: the RFC text in `rfc/full/` judges the work, never the summary.
- [ ] `ai/doors/access-security.md` - L2TP, PPP, subscriber plugins
  → Constraint: subscriber auth, pool and shaper reach a session through `l2tp/subscriber` handler registries and the session events. The reactor names no subscriber.
- [ ] `ai/doors/system-plugins.md` - `ddos`, `cos`
  → Constraint: a plugin owns its whole surface: the doctor check for the FlowSpec plugin lives in its own folder.
- [ ] `ai/doors/plugin-infra.md` - SDK, process protocol
  → Constraint: a new SDK callback use is one `On*` registration in the plugin. The event loops and the SDK do not change. State RPCs are refused until the handshake ends, so the first state read is in `OnStarted` or later.
- [ ] `ai/doors/core.md` - diagnostic codes, the state store
  → Constraint: a plugin uses the SDK state RPCs, and its owning package grants its key patterns with `statestore.RegisterPluginKeys`. A key name is a contract an operator sees in `ze data registered`.
- [ ] `ai/doors/le-tooling.md` - tests and fixtures
  → Constraint: a `.ci` that needs a Linux kernel feature carries `option=needs-linux`. A compiled fixture returns an error when an assertion fails.
- [ ] `docs/architecture/ldp/mpls-ldp.md`, "Decision: a failed session setup backs off on the Hello cadence" and the panic-guard table
  → Decision: the Hello is the retry tick, there is no timer goroutine. This spec keeps that.
  → Constraint: the page already says "A session that reaches operational clears the backoff, and so does the adjacency expiring, so a peer rediscovered later starts undelayed." Both halves are false in today's code and true after phase 1. The panic-guard row for `session` says SessionDown is published on every end: phase 1 corrects it to "when the session had reached OPERATIONAL".
- [ ] `ai/digests/mpls-signaling.md` - LDP flow
  → Constraint: label entries go through `(mpls-fib, entry)` events. The binding withdrawal in the session-end path (`reconcilePeerDown`) is unchanged and runs on every end.
- [ ] `docs/architecture/l2tp/subscriber-session-model.md`, `ai/digests/subscriber.md` - the session event bridge and its readers
  → Constraint: `subscriberBridge` re-publishes each L2TP session event as a subscriber event, and the pool reads only the subscriber SessionDown. Order on the L2TP events is therefore order on the subscriber events too.
  → Decision: SessionIPAssigned normally arrives before SessionUp, so "no entry yet" is the ordinary first-event case at every reader.
- [ ] `docs/architecture/api/process-protocol.md`, "Persistent runtime state", "Failure-Policy Declaration"
  → Constraint: `StateGet`, `StatePut` and `StateRemove` work on both transports after the handshake. Only a successful mutation returns `ok`.
- [ ] `docs/guide/ddos-mitigation.md`, the two "what a commit does to a live rule" tables and "A daemon stop makes no promise about the drop"
  → Decision: after a restart, protection comes from fresh detection (the ddos local precedent). The FlowSpec plugin follows it: a recorded rule is withdrawn at start, never re-adopted.
  → Constraint: the ddos local row "The parent `ddos` block deleted" says the commit tells the plugin nothing. That stopped being true on 2026-10-10 (`rootHasChanges` matches an ancestor, and the comment above `stopResponder` in `internal/plugins/ddos/local/register.go` says so). Phase 3 corrects it on the same page it edits.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5036.md` - rows RFC5036-2.5.3-3 and RFC5036-2.5.3-4 (the setup backoff), read against `rfc/full/rfc5036.txt` Section 2.5.3
  → Constraint: Section 2.5.3: "An LSR MUST throttle its session setup retry attempts with an exponential backoff in situations where Initialization messages are being NAK'd."
  → Constraint: Section 2.5.3: "The session establishment setup attempt following a NAK'd Initialization message MUST be delayed no less than 15 seconds, and subsequent delays MUST grow to a maximum delay of no less than 2 minutes."
  → Decision: Section 2.5.3: "After such a configuration action, there is no further need to throttle subsequent session establishment attempts (until their Initialization messages are NAK'd)." The RFC binds the backoff to NAK'd Initializations and releases it once they stop. A session that reached operational had its Initialization accepted, so clearing the backoff then is what the text describes. No requirement row covers the clear, so the new tests carry no `RFC requirement:` tag for it.
  → Constraint: Section 2.5.3: "When LSR1 has received both an acceptable Initialization message and a KeepAlive message, the session is operational from LSR1's point of view." This is the instant the on-operational callback marks.
- [ ] `rfc/short/rfc5443.md` - LDP-IGP synchronization, the reader of the LDP session events
  → Constraint: this spec changes no OSPF code. A claim about what RFC 5443 requires is read in `rfc/full/` before it is written anywhere.
- [ ] `rfc/short/rfc2661.md`, `rfc/short/rfc8955.md` - present. No L2TP or FlowSpec wire format changes.

**Key insights:**
- LDP: `runSession` defers `onDone`, then `sess.Stop`. Defers run last-in first-out, so `Stop` resets the state to non-existent before `onDone` reads it. `Stop` is also called by `expireAdjacencies` and by the accepted-Initialization callback when the KeepAlive send fails, so the state is unreadable at session end whatever the defer order. The fact is held elsewhere: the on-operational callback runs once, on `runSession`'s own goroutine.
- LDP: `expireAdjacencies` deletes the backoff entry and then stops the session. `onDone` runs afterwards and writes the entry back. The page's second claim needs that closed too.
- L2TP: state changes under `tunnelsMu`, every notification after the unlock. The reactor goroutine publishes SessionUp and SessionIPAssigned. Timeout, CLI, CoA and realm-relay goroutines publish SessionDown.
- L2TP: `handleLCPDown` publishes SessionDown for a session that stays alive and can be assigned an address again. A reader that remembered "this session ended" would drop a valid event.
- L2TP: the RADIUS accounting reader sends Accounting-Stop synchronously, bounded at 10 seconds, on the publishing goroutine. A lock held across a publication would therefore let one slow RADIUS server stop every state change in the reactor for 10 seconds, which is why the order is carried by a ticket and no lock is held while a reader runs.
- L2TP: a tunnel leaves the reactor's maps in one place, `discardTunnelLocked`, which drains its queued session-downs first. After that no handler can find it, so no new event is made for it. A tunnel id comes from a counter that wraps after 65534 allocations (`allocateLocalTID`). The tunnel object is therefore the narrowest object whose life covers every event of one session and of any later session that reuses its id.
- L2TP: a session id is random and unique inside its tunnel only (`allocateSessionID`), and a freed id can be drawn again at once. Two consequences are separate defects, recorded in the journal and owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md`, not here: a queued PPP event of an ended session can be applied to a successor with the same id, and the route observer and the CQM observer key their maps by session id alone, so two tunnels' sessions can share a record.
- LDP: OSPF keys its LDP-sync machine by interface name, and SessionDown carries the adjacency's interface. A failed setup attempt to one neighbor on a shared segment therefore costs the whole link out today while another neighbor's session is up. With no SessionDown for a session that never reached operational, that disappears. An operational session ending on a shared segment still costs the link out, which RFC 5443 Section 3 leaves to policy: "On broadcast links with more than one IGP/LDP peer, the cost-out procedure can only be applied to the link as a whole and not to an individual peer."
- DDoS: what is gone when the plugin ends is the connection to the engine, not only the context. `Process.Stop` refuses bridge dispatch, cancels, and closes the connections. The engine sends `bye` with reason `removed` and waits for the answer before it stops a removed plugin, and it never sends `bye` on daemon stop (`OnBye` comment, `pkg/plugin/sdk/sdk_callbacks.go`).
- DDoS: the 2026-10-10 fix already delivers a verify and an apply to a plugin whose root sits under a deleted ancestor. Deleting the whole `ddos` block therefore reaches `replaceResponder` with `configured` false, which withdraws. Proven at unit level for the reload, not for this plugin through a daemon.
- DDoS: the plugin runs in the daemon only. `loadBus` refuses a nil bus, and a forked plugin is handed none.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/ldp/register.go` - `runLDPEngine`, `startSessionForAdj` (the `onDone` closure), `runSession`, `recordSetupFailure`, `setupRetry`, `expireAdjacencies`, `emitSessionEvent`. `onDone` reads `sess.State()` after `Stop` reset it, so it always calls `recordSetupFailure` and never deletes the entry, then always publishes SessionDown. The comment above the backoff constants quotes two RFC sentences as one quotation and drops the sentence between them.
- [ ] `internal/plugins/ldp/session.go` - `Stop` (once, closes the connection, sets non-existent), `State`, `ReadLoop` (calls the on-operational callback once, inline, after the PDU that made the session operational; a `Stop` that lands between that PDU and the check means the callback never runs).
- [ ] `internal/plugins/ldp/events.go` - `SessionUp`, `SessionDown`, `SessionEvent`.
- [ ] `internal/plugins/ospf/ldp_sync.go` - `subscribe`, `handleSessionEvent`, `onSessionUp`, `onSessionDown`. SessionDown moves the interface to not-synchronized, cancels the hold-down timer, and arms the stuck timer when the interface had ever synchronized. It cannot tell a session that ended from a setup attempt that failed.
- [ ] `internal/test/fixture/routing_fixture.go` - `emitLDPSession`: a test fixture publishes `(ldp, session-up)` and `(ldp, session-down)` through the plugin RPC for `test/ospf/ospf-ldp-sync-restore.ci`.
- [ ] `internal/component/l2tp/reactor_kernel.go` - `handlePPPEvent`, `handleLCPDown`, `handleSessionIPAssigned`, `handleSessionUp`, `handleKernelError`, `notifyRouteObserverDown`. Each reads or changes the session under `tunnelsMu`, unlocks, then calls the route observer and publishes. `handleSessionIPAssigned` and `handleLCPDown` defer `ev.Acknowledge()`, which PPP waits on. `handleSessionUp` calls `startSessionTimeouts` after publishing, and that function takes `tunnelsMu`.
- [ ] `internal/component/l2tp/teardown.go` - `teardownTunnel`, `teardownSessionByID`, `teardownSessionOnTunnelWith`, `emitSessionDowns`. Same shape. The comment on `emitSessionDowns` says it is the only publisher of SessionDown (false: `handleLCPDown` publishes one directly) and that a reader "can call back into the reactor".
- [ ] `internal/component/l2tp/reactor.go` - `run` (one goroutine serves packets, ticks, kernel results and PPP events), the packet path and the tick path, which also call `emitSessionDowns` after the unlock.
- [ ] `internal/component/l2tp/route_observer.go` - `OnSessionIPUp` creates a record keyed by session id when none exists, `OnSessionDown` deletes it. An IP-up that arrives after the down leaves a record and an injected route nothing removes.
- [ ] `internal/component/l2tp/session.go` - `allocateSessionID` (random, unique in the tunnel, no quarantine of a freed id), `removeSession` (the one place a session leaves a tunnel; queues the session-down), `drainPendingTeardowns` (hands the queued downs to the caller under `tunnelsMu`), `sessionDown` (carries its tunnel id because one drain can hold several tunnels' sessions).
- [ ] `internal/component/l2tp/reactor.go` - `discardTunnelLocked` (the only place a tunnel leaves the maps; it drains first), `reapExpiredLocked` and `resolveTieBreakerLocked` (collect the downs of several discarded tunnels), `allocateLocalTID` (a wrapping counter).
- [ ] `internal/component/l2tp/subscriber_bridge.go` - `onSessionUp`, `onSessionIPAssigned` each add the registry entry when it is missing. `onSessionDown` removes it.
- [ ] `internal/component/l2tp/plugins/authradius/acct.go` - `onSessionIPAssigned` creates the accounting session and starts its task when the key is missing. `onSessionDown` removes it and sends Accounting-Stop synchronously.
- [ ] `internal/component/l2tp/plugins/shaper/shaper.go`, `internal/plugins/cos/handler.go`, `internal/component/l2tp/observer.go`, `internal/component/l2tp/plugins/pool/register.go` - the other readers (census in Data Flow).
- [ ] `internal/component/l2tp/events/events.go` - the typed handles. Its comments name the pool as the reader of the L2TP SessionDown, which the pool's own comment contradicts.
- [ ] `internal/component/plugin/server/engine_event.go`, `dispatch.go` (`deliverEvent`), `internal/component/plugin/process/delivery.go` (`Deliver`) - in-process readers run synchronously on the publishing goroutine. A forked reader's event is put on that process's own channel in call order, blocking when it is full.
- [ ] `internal/plugins/ddos/flowspec/register.go` - `runEngine`, `replaceResponder`, `sdkDispatcher`. The exit path stores nil in `activeResponder` and unsubscribes. The dispatcher sends every command under the signal context.
- [ ] `internal/plugins/ddos/flowspec/responder.go` - `withdraw`, `withdrawForConfig`, `retire`, `adoptAnnouncement`.
- [ ] `internal/plugins/ddos/local/register.go` - `stopResponder` and the deferred call in `runEngine`: the sibling's exit withdrawal. Its start-time half is `clearStaleDropRule`.
- [ ] `internal/component/plugin/server/reload.go` - `rootHasChanges` matches an ancestor key since 2026-10-10.
- [ ] `internal/component/plugin/server/startup_removal.go` - `notifyPluginRemoval` sends `bye` with reason `removed`, waits up to `pluginRemovalGrace` (500 ms) for the answer, then `stopRemovedProcess`.
- [ ] `internal/component/plugin/process/process.go` - `Stop`, `SendShutdown`.
- [ ] `internal/component/plugin/server/dispatch_route.go` - `withdrawPluginRoutes` covers Loc-RIB routes a plugin installed through route-install, and nothing a plugin announced through update-route.
- [ ] `internal/component/plugin/server/dispatch.go` `cleanupProcess` - at a process end it unregisters commands, cancels pending requests, clears subscriptions, unregisters the cache consumer, calls `withdrawPluginRoutes` and runs the cleanup hooks. Nothing there knows what the plugin announced through update-route.
- [ ] `internal/component/plugin/server/rpc_register.go` `RegisterProcessCleanup` - a hook list run at every process end. A hook receives the process name and nothing else.
- [ ] `internal/component/bgp/reactor/reactor_api_batch.go` `AnnounceNLRIBatch`, `WithdrawNLRIBatch` - the rail a plugin's text announcement takes. The sender is used for permission only. `PluginRoutes` in the peer settings are routes from the config (`internal/component/bgp/config/peers.go`), not a record of runtime announcements.
- [ ] `pkg/plugin/sdk/sdk_dispatch.go` `handleBridgeCallback` - the `bye` callback runs on the callback loop, and the loop ends after it returns without error. `pkg/plugin/rpc/bridge.go` `beginDispatch` admits a plugin-to-engine call until `StopDispatch`, which only `Process.Stop` calls, after the `bye` was answered.
- [ ] `pkg/plugin/sdk/sdk_state.go` - `StateGet`, `StatePut`, `StateRemove`, `StateList`, `StateIncrement`. Each is an engine RPC through `callEngineRaw`, so a forked plugin uses the daemon's store. `StatePut` returns an error for anything but an acknowledged write.
- [ ] `internal/component/plugin/registry/doctor.go` `validateDoctorCheckDef` - a check that reads a plugin's live state cannot be a registered function: the registry refuses `rpc.DoctorPhaseRuntime` there and says to declare the check in the `sdk.Registration` `DoctorChecks` and answer it with `OnDoctorCheck`. `internal/plugins/ddos/detect/register.go` does that (`rateFeedCheckDecl`, `checkRateFeed`). `internal/plugins/ldp/doctor.go` is the registered-function kind, for a check that needs no live state.
- [ ] `internal/core/diagnostic/codes.go` - the two `doctor-ddos-detect-*` codes are the only ddos codes.

**Behavior to preserve:**
- LDP: the backoff values (15 seconds doubling to 2 minutes), the Hello as retry tick, and the backoff for every setup attempt that ends before operational, a refused TCP connect included.
- LDP: the SessionUp payload and the instant it is published. The SessionDown payload. The label withdraw events and `reconcilePeerDown` on every end.
- LDP: tests `TestRFC5036SetupRetryBackoffGrows` and `TestRFC5036SetupRetryRefusedInsideDelay` are RFC-tagged and are not edited.
- L2TP: every payload, every event name, which path publishes which event, the route observer being told before the bus, PPP waiting on `Acknowledge` until publication finished, and outbound control messages sent after the unlock.
- L2TP: in-process readers run synchronously on the publishing goroutine.
- DDoS: `replaceResponder`'s carry and withdraw rules, the announce rate limit, the cap worker, `show ddos flowspec`.

**Behavior to change:**
- LDP: what `onDone` records and publishes depends on whether the session reached operational. An adjacency expiry leaves no backoff entry.
- L2TP: the order in which notifications of one session reach their readers.
- DDoS: the plugin withdraws on `bye` and on its way out, records a live announcement, withdraws a recorded one at start, and has a doctor check.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The on-operational callback | `internal/plugins/ldp/session.go` `ReadLoop`, and the fourth callback `runSession` passes it | Fires once when the session becomes operational. `runSession` publishes SessionUp from it | Extends: the same callback records that the session was operational, and `runSession` hands that to `onDone` |
| The setup backoff table | `internal/plugins/ldp/register.go` `setupRetry`, `recordSetupFailure`, the `retries` map under `sessionsMu` | One entry per adjacency key while attempts fail | Extends: `onDone` deletes the entry for a session that was operational. No new table |
| Adjacency expiry | `internal/plugins/ldp/register.go` `expireAdjacencies` | Deletes the backoff entry, stops the session | Extends: the session carries the fact that its adjacency expired, so `onDone` does not write the entry back |
| `tunnelsMu` | `internal/component/l2tp/reactor.go`, field of `l2tpReactor` | Orders every session state change | Extends: the order it gives is carried to the publication by a per-tunnel order ticket taken while it is held and waited for after it is released |
| The tunnel's queue of session-downs | `internal/component/l2tp/session.go` `removeSession`, `drainPendingTeardowns` | Each removal queues its down on the tunnel, and one drain hands the batch to the path that emits it | Extends: the drain also issues the tunnel's order ticket for that batch, so the ticket travels with the downs through `discardTunnelLocked` and the two collectors |
| Single SessionDown emitter | `internal/component/l2tp/teardown.go` `emitSessionDowns` | One function publishes what `removeSession` queued | Extends: it publishes inside the ordered section. Its two false comments are corrected |
| Synchronous in-process delivery, FIFO per forked process | `internal/component/plugin/server/engine_event.go` `dispatch`, `internal/component/plugin/process/delivery.go` `Deliver` | Readers see events in the order `Emit` is called | Extends by use: ordering the `Emit` calls orders every reader, forked ones included. No bus change |
| Config-driven withdrawal | `internal/plugins/ddos/flowspec/register.go` `replaceResponder` | Withdraws when the section is gone or response-level left enforce | Unchanged. Already reached when the parent block is deleted. This spec adds the daemon-level proof |
| The sibling's exit withdrawal | `internal/plugins/ddos/local/register.go` `stopResponder` | Withdraws and retires the responder on the way out | Extends: the FlowSpec plugin gets the same function shape |
| The sibling's start-time cleanup | `internal/plugins/ddos/local/register.go` `clearStaleDropRule` | Removes a rule a previous run left | Extends: the FlowSpec plugin withdraws a recorded announcement at start |
| `bye` on removal | `pkg/plugin/sdk/sdk_callbacks.go` `OnBye`, `internal/component/plugin/server/startup_removal.go` `notifyPluginRemoval` | The engine tells a removed plugin and waits for its answer while the connection is open | Extends by use: the plugin registers a handler. No SDK change |
| Plugin runtime state | `pkg/plugin/sdk/sdk_state.go` `StateGet`, `StatePut`; `process-protocol.md`, "Persistent runtime state" | Persists small values per plugin on both transports | Extends by use: one key holds the live announcement |
| Engine cleanup of a plugin's routes | `internal/component/plugin/server/dispatch_route.go` `withdrawPluginRoutes` | Removes Loc-RIB routes installed through route-install | Does not apply: `cleanupProcess` holds no record of update-route announcements, and the BGP rail (`AnnounceNLRIBatch`) uses the sender for permission only. Tracking them is engine work outside this child |
| Process cleanup hooks | `internal/component/plugin/server/rpc_register.go` `RegisterProcessCleanup` | Runs registered functions at every process end, with the process name | Does not apply: a hook gets no dispatch handle and no state, and an edge plugin does not import `plugin/server`. Child 1 reshapes this list |
| Runtime doctor check answered by the plugin | `pkg/plugin/sdk/sdk_callbacks.go` `OnDoctorCheck`, used by `internal/plugins/ddos/detect/register.go`; `internal/component/plugin/registry/doctor.go` `validateDoctorCheckDef` refuses a runtime check as a registered function | The plugin declares the check in its `sdk.Registration` and answers it from its own process, internal and forked alike | Extends by pattern: the FlowSpec plugin declares and answers one |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- LDP: a TCP session to a neighbor ends (peer close, KeepAlive timeout, Notification, adjacency expiry, engine stop, a contained panic).
- L2TP: a PPP event from the driver, a control message, a tick, an operator `clear`, a RADIUS CoA Disconnect, a session or idle timeout.
- DDoS: the engine's `bye`, the plugin's `Run` returning, a contained panic, the plugin's next start.

### Transformation Path
1. LDP: `ReadLoop` returns to `runSession`. Its deferred calls run: panic guard, `Stop`, `onDone`. `onDone` updates the session table and the backoff under `sessionsMu`, withdraws the peer's bindings, publishes SessionDown and the label withdraws. OSPF's `ldpSyncManager` reads SessionDown.
2. L2TP: a handler takes `tunnelsMu`, changes the session, collects what to publish, takes the tunnel's next order ticket, and releases `tunnelsMu`. Holding no lock, it waits until every earlier ticket of that tunnel has finished, tells the route observer, publishes on the bus, and marks its ticket finished. Readers run inside the publish step. `tunnelsMu` is never held across the wait or across a publication.
3. DDoS: the stop function takes the responder's lock, withdraws through the dispatcher under a bounded context of its own, retires the responder, removes the record. At start the plugin reads the record and withdraws what it names.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| LDP plugin to OSPF plugin | `(ldp, session-down)` on the EventBus, typed in process, JSON when forked | No |
| L2TP reactor to subscriber plugins | the three L2TP session events on the EventBus, and the re-published subscriber events | No |
| DDoS plugin to BGP engine | update-route RPC carrying the rendered `del` command | No |
| DDoS plugin to state store | SDK state RPC | No |
| BGP engine to peer | BGP UPDATE withdrawing the FlowSpec NLRI | No |

### Changed-Wire Census

Four existing wires are touched and one is new. No payload, field or encoding
changes on an existing wire: what changes is when a message is sent (LDP
SessionDown, FlowSpec withdrawal) or the order messages are sent in (L2TP).

**Search evidence:** scope `internal/`, `cmd/`, `pkg/`, `test/`, `docs/plugin-development/`, Go and non-Go (`.ci`, `.py`, `.sh`, `.et`, `.yang`, `.md`). L2TP terms: the typed handles `SessionUp`, `SessionDown`, `SessionIPAssigned` with `.Emit` and `.Subscribe`, `emitSessionDowns(`, the strings `session-ip-assigned` and `session-down`; control: the search finds the known writer `handleSessionIPAssigned` and the known reader `subscriberBridge.subscribe`, and the non-Go search for `session-ip-assigned` returns no file. LDP terms: `EventSessionDown`, `EventSessionUp`, `ldpSyncEventSession`, `SessionEvent`, and `ldp` beside `session-up` or `session-down` in non-Go files; control: the search finds the known reader `ldpSyncManager.subscribe`, and one writer outside the plugin, the test fixture behind `test/ospf/ospf-ldp-sync-restore.ci`. FlowSpec terms: `UpdateRoute` and `Dispatch(` under `internal/plugins/ddos/flowspec/`, `renderFlowspecCommand`; control: `responder.withdraw`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| LDP session events | writer | `internal/plugins/ldp/register.go`, the `onDone` closure in `startSessionForAdj` | The only `SessionDown` emit in the plugin | Change: SessionDown is published only for a session that was operational. Payload unchanged |
| LDP session events | writer | `internal/plugins/ldp/register.go` `runSession`, on-operational callback | The only `SessionUp` emit | Preserve |
| LDP session events | writer | `internal/test/fixture/routing_fixture.go` `emitLDPSession`, used by `test/ospf/ospf-ldp-sync-restore.ci` | Publishes both through the plugin emit RPC | Preserve: the `.ci` is re-run in phase 1 |
| LDP session events | reader | `internal/plugins/ospf/ldp_sync.go` `ldpSyncManager.subscribe`, `handleSessionEvent` | The only subscriber of the `ldp` namespace session events | Preserve, no edit. It now receives a down only for a session it was told was up |
| LDP session events | reader | a third-party plugin subscribed to the `ldp` namespace | Outside the repository | Preserve the payload. It stops receiving a down with no matching up |
| L2TP session events | writer | `internal/component/l2tp/teardown.go` `emitSessionDowns`, called from `teardownTunnel`, `teardownSessionByID`, `teardownSessionOnTunnelWith`, `handlePPPEvent`, `handleKernelError`, and the packet and tick paths in `reactor.go` | Nine call sites | Change: each call is inside the ordered section. Payload unchanged |
| L2TP session events | writer | `internal/component/l2tp/reactor_kernel.go` `handleLCPDown` | A direct SessionDown emit for a network lifetime that ended on a live session | Change: inside the ordered section |
| L2TP session events | writer | `internal/component/l2tp/reactor_kernel.go` `handleSessionIPAssigned` | The only SessionIPAssigned emit | Change: inside the ordered section |
| L2TP session events | writer | `internal/component/l2tp/reactor_kernel.go` `handleSessionUp` | The only SessionUp emit | Change: inside the ordered section |
| L2TP session events | reader | `internal/component/l2tp/subscriber_bridge.go` `onSessionUp`, `onSessionIPAssigned`, `onSessionDown` | Registry add and remove, then re-publish as subscriber events | Preserve, no edit |
| L2TP session events | reader | `internal/component/l2tp/plugins/authradius/acct.go` `onSessionIPAssigned`, `onSessionDown` | Accounting session start and stop | Preserve, no edit |
| L2TP session events | reader | `internal/component/l2tp/plugins/shaper/shaper.go` `onSessionUp`, `onSessionDown` | Traffic rules on the PPP interface | Preserve, no edit |
| L2TP session events | reader | `internal/plugins/cos/handler.go` `onSessionUp`, `onSessionDown` | QoS maps on the access interface | Preserve, no edit |
| L2TP session events | reader | `internal/component/l2tp/observer.go` `wireObserverSubscriptions` | Event ring and login state | Preserve, no edit |
| L2TP session events | reader | a forked or third-party plugin subscribed to the `l2tp` namespace | `Deliver` queues per process in call order | Preserve: it sees the same order |
| Subscriber session events | writer | `internal/component/l2tp/subscriber_bridge.go`, the three re-publishing handlers; `internal/component/l2tp/pppoe/subsystem.go` for PPPoE | Emitted from inside the L2TP reader, so in L2TP order | Preserve, no edit. PPPoE's own emits are untouched |
| Subscriber session events | reader | `internal/component/l2tp/plugins/pool/register.go` `onSessionDown` | Address release, safe against a repeat | Preserve, no edit |
| FlowSpec withdrawal | writer | `internal/plugins/ddos/flowspec/responder.go` `withdraw` through `sdkDispatcher.Dispatch` | The only withdraw path: the rendered `del` command over update-route | Change: also called on `bye`, on the way out, and at start for a recorded rule. Command text unchanged |
| FlowSpec withdrawal | reader | the BGP engine's update-route handler (`internal/component/plugin/server/dispatch.go`), which sends a BGP UPDATE to every peer the plugin's selector matches | Unchanged path | Preserve |
| FlowSpec withdrawal | reader | a BGP peer (FRR, BIRD, a vendor router) | Outside the repository | Preserve: the peer receives a withdrawal it previously did not receive, for an NLRI it holds |
| Live-announcement record | writer | `internal/plugins/ddos/flowspec/responder.go`, the announce and withdraw paths | New plugin runtime state key | New |
| Live-announcement record | reader | `internal/plugins/ddos/flowspec/register.go`, the start path, and the plugin's doctor check | New | New |

### Integration Points
- `runSession` and the `onDone` closure - the fact is passed as an argument, read when the deferred call runs.
- `L2TPTunnel` - the order ticket: a count of tickets issued and a count finished, with a way to wait for a turn. Issued under `tunnelsMu`, only by a path that has a session notification to deliver.
- `sdk.Plugin.OnBye`, `StateGet`, `StatePut`, `StateRemove` - used as they are.
- `registry.Registration.DoctorChecks` - one `DoctorCheckDef` declared by the FlowSpec plugin.

### The order ticket

| Property | Statement |
|----------|-----------|
| Scope | One sequence per tunnel object. It orders every notification of every session the tunnel ever holds, a reused session id included |
| Issue | While `tunnelsMu` is held, by the path that changed the state, when it has a route-observer call or a session event to deliver. For session-downs the drain issues it, so the ticket travels with its batch |
| Redeem | After `tunnelsMu` is released: wait for all earlier tickets of the tunnel, deliver, mark finished. The mark is a deferred call in the function that waits, so a reader that panics or an early return still finishes the ticket |
| Several tunnels in one path | Tunnel teardown, retention reaping and a tie-breaker loss deliver the downs of each tunnel under that tunnel's own ticket, one tunnel after another |
| What a slow reader delays | Later notifications of the same tunnel, and the goroutine that carries them while it waits. No state change waits, because no lock is held: other goroutines (operator, timeouts, CoA) and `show` reads proceed. When the waiting goroutine is the reactor's own, packet processing waits with it, exactly as it does today whenever the reactor goroutine itself publishes to a slow reader |
| PPP acknowledgement | Unchanged: the handler's deferred `Acknowledge` runs after its own publication returned |
| Reader contract | A reader runs on the publishing goroutine and does not cause a publication for the same tunnel from inside its handler: that publication would wait behind the ticket its own caller holds |

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The on-operational callback runs on `runSession`'s goroutine, so the flag it sets needs no lock | `ReadLoop` calls it inline (`session.go`) | A data race on the flag | Read at the producer 2026-11-11; `TestRunSessionReportsOperationalToOnDone` under the race detector | validated by read, test owed |
| A-2 | OSPF's `ldpSyncManager` is the only in-tree reader of the LDP session events | Census above | Another reader depends on a down with no up | Re-run the census search in phase 1 | validated by search |
| A-3 | No in-process reader of an L2TP session event calls back into the reactor from its handler | Each reader in the census read 2026-11-11: registry, sync maps, traffic backend, RADIUS client, metadata map. The pool's `lookupSessionUsername` takes it, but from its show handler | A reader that causes a publication for the same tunnel from inside its handler waits behind the ticket its caller holds, and that tunnel's events stop | `TestSessionEventReadersDoNotTakeTunnelsMu` (phase 2), and the contract written on the events package | validated by read, test owed |
| A-4 | A forked reader sees events in `Emit` call order | `Deliver` puts each event on the process's channel, blocking when full | Order is lost for forked readers and the fix is partial | Read at the producer 2026-11-11 | validated by read |
| A-5 | `startSessionTimeouts` can run after the emit-order lock is released with no change in behavior | It takes `tunnelsMu` and starts timers. It reads the session again and returns when it is gone | A session timeout armed for a dead session | `TestSessionUpStartsTimeoutsAfterPublication` | unvalidated |
| A-7 | Deleting the whole `ddos` block delivers an apply to the FlowSpec plugin with `configured` false | `rootHasChanges`, `TestRootHasChangesUnderAnAddedOrRemovedAncestor`, `TestBuildDiffSectionsFilesAnAncestorKeyUnderItsDeclaredRoot` | The parent-block removal still orphans the rule, and the `bye` handler is what withdraws it | `test/reload/ddos-flowspec-parent-removed-withdraws.ci` | unvalidated at daemon level |
| A-8 | The BGP engine keeps no per-plugin record of update-route announcements, so nothing withdraws them when a plugin's generation ends | `withdrawPluginRoutes` covers route-install only. A search of the plugin server and the BGP reactor for a per-plugin announcement table found `installedByPlugin` (route-install) and nothing else | The recorded-rule recovery duplicates an engine mechanism | Read 2026-11-11 at `cleanupProcess`, `AnnounceNLRIBatch` and the config origin of `PluginRoutes` | validated by read |
| A-9 | A withdrawal for an NLRI no peer holds is harmless | BGP withdraws are idempotent at the receiver | A start-time withdrawal after a daemon restart disturbs a peer | Read the peer's withdraw handler at the lab's version before asserting it in a scenario | unvalidated |
| A-10 | A live-peer LDP `.ci` can run in the `ldp` suite with a compiled fixture playing the neighbor on a veth pair | `test/plugin/ddos-announce-rate-limit.ci` builds a veth topology from a fixture. `test/ldp/ldp-convergence.ci` says no mock peer exists today | The `.ci` needs harness work first | Phase 1 writes the fixture in `test/draft/ldp/` | unvalidated |
| A-11 | The bridge still accepts an update-route call from inside the `bye` handler | `notifyPluginRemoval` sends `bye` before `stopRemovedProcess` | The handler's withdrawal fails and the record stays | Read 2026-11-11: `handleBridgeCallback` runs the handler before the loop ends, and `beginDispatch` admits calls until `StopDispatch`, which `Process.Stop` calls after the answer. `TestByeWithdrawsTheLiveAnnouncement` over a real `sdk.Plugin` pair | validated by read, test owed |
| A-12 | Two tunnel objects never carry the same tunnel id closely enough for their events to be confused | `allocateLocalTID` is a counter that wraps after 65534 allocations, and a discarded tunnel's last downs are emitted by the path that discarded it | A reader sees a new tunnel's session event before the old tunnel's down for the same ids | Read at the producer 2026-11-11. No test: the window needs 65534 tunnel creations during one publication | validated by read |
| A-13 | `ze doctor` reports a declared runtime check that its plugin cannot answer because the plugin's process ended | Not read: the daemon's collection of plugin doctor answers | A recorded rule is invisible while the plugin is dead and not restarted | Phase 3 reads the doctor collection path and `docs/architecture/doctor-and-health-checks.md`. If a dead plugin's check is silently skipped, the spec gains a registered post-config check that reads the record through `statestore` when the `ddos flowspec` section is absent, and says it cannot answer where no store is open | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The reactor goroutine publishes synchronously, so when it publishes to a slow reader, or waits its turn behind another goroutine's publication for the same tunnel, it processes no packet for any tunnel meanwhile (up to 10 seconds against a dead RADIUS server). This exists today for its own publications; the ticket adds the wait behind another goroutine, for the same tunnel only | Control messages of healthy tunnels are answered late during a RADIUS outage | Not made worse for state: no lock is held, so operator, timeout and CoA paths and `show` reads proceed (AC-18). The stall itself is a defect, not an accepted limit: its journal row is `plan/journal/slow-reader-stalls-a-shared-goroutine.md` (2026-11-11), owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md`, which decides where the Accounting-Stop wait moves |
| R-2 | A future in-process reader publishes for the same tunnel from inside its handler and waits behind itself | That tunnel's sessions stop coming up or going down, with one goroutine parked in the wait | The contract is written on the events package and on `emitSessionDowns`, and A-3's test runs every registered reader against a reactor whose entry points fail the test when called |
| R-3 | A path takes a ticket and returns without finishing it | Every later event of that tunnel waits for ever | The ticket is finished by a deferred call in the function that waits, and it is issued only by a path that goes on to deliver. `TestEveryIssuedTicketIsFinished` drives each of the twelve paths and asserts issued equals finished |
| R-4 | `onDone`'s flag is read when the defer is registered, not when it runs | The backoff never clears, as today | `TestRunSessionReportsOperationalToOnDone` drives a real session to operational and asserts the argument |
| R-5 | A session ended by adjacency expiry while operational publishes SessionDown after the expiry already cleared its state | None: this is correct, the session did end | Covered by AC-2 and AC-5 |
| R-6 | The `bye` handler's withdrawal outlasts `pluginRemovalGrace` and the removal is reported as failed | A reload that removes the plugin logs a removal callback error | The handler's context is shorter than the grace. A withdrawal that does not finish leaves the record, and the doctor check reports it |
| R-7 | The plugin logs an error on every daemon stop with a live rule, because the connection is closed | An `ERROR` line at each stop | A closed transport on the way out is one `INFO` line that says the BGP sessions end with the daemon |
| R-8 | The state write fails and a later crash leaves an unrecorded rule | A `WARN` line at announce time | The warning names the doctor code. The announcement still goes out: refusing to mitigate because the record failed would be worse |
| R-9 | The start-time withdrawal runs before any peer is established and reaches nobody, then the record is removed | None needed: sessions that start after it never held the rule | Stated in the guide |
| R-10 | The LDP interop check asserts an FRR command or output FRR does not have | The scenario cannot pass | Read FRR's `ldpd` source for the clear command and the neighbor output at the lab's version, and name the function in the scenario, before writing the check |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | LDP: a session is redialled too soon or too late, or OSPF never hears a session ended and keeps a link in service without labels. L2TP: a deadlock stops every tunnel on the box. DDoS: a discard rule is withdrawn while the attack runs, or stays after it ended |
| How is it reverted? | Each phase is one commit and reverts alone. The DDoS state key is left behind by a revert and is harmless |
| Who else touches this path? | `internal/plugins/ldp/register.go`: no other open spec in `plan/release/fixes/`. `internal/component/l2tp/`: `spec-rfc5176-remaining-defect-classes` found the row and edits the CoA path; check its Files before phase 2. `internal/plugins/ddos/flowspec/`: none. Child 1 edits `plugin/server` and `process`, which this child reads and does not edit |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| An LDP neighbor completes Initialization and KeepAlive, then closes the connection | → | `runSession` on-operational callback, `onDone` | `TestRunSessionReportsOperationalToOnDone` |
| An LDP neighbor through the daemon: session up, closed, redialled on the next Hello | → | `startSessionForAdj`, `onDone` | `test/ldp/ldp-session-end-clears-backoff.ci` |
| A PPP IP-assigned event handled while another goroutine tears the session down | → | the ordered section in `handleSessionIPAssigned` and `teardownSessionByID` | `TestSessionIPAssignedIsPublishedBeforeTheTeardownThatFollowsIt` |
| A real PPP session through the daemon, cleared by the operator | → | `teardownSessionByID`, `emitSessionDowns`, the readers | `test/l2tp/session-clear-leaves-no-subscriber.ci` |
| The engine's `bye` to the FlowSpec plugin | → | the `OnBye` handler, the stop function, `responder.withdraw` | `TestByeWithdrawsTheLiveAnnouncement` |
| A reload that deletes the whole `ddos` block with a rule announced | → | `rootHasChanges`, `replaceResponder` | `test/reload/ddos-flowspec-parent-removed-withdraws.ci` |
| The FlowSpec plugin starts with a recorded rule | → | the start path, `responder.withdraw` | `TestStartWithdrawsTheRecordedAnnouncement` |
| `ze doctor` with a recorded rule and no live responder | → | the FlowSpec doctor check | `test/plugin/ddos-flowspec-orphan-doctor.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An LDP session reached operational and then ends for any reason | The adjacency has no backoff entry afterwards, and the next Hello from the neighbor starts a new session at once | An entry created by a later failed attempt: that is a new failure |
| AC-2 | An LDP session reached operational and then ends | Exactly one `(ldp, session-down)` is published, carrying the neighbor's address, LDP identifier and interface | None: one up is answered by one down |
| AC-3 | A setup attempt ends before the session is operational (NAK'd Initialization, transport closed, KeepAlive send failure, TCP connect refused) | A setup failure is recorded: the next attempt waits 15 seconds, then double on each further failure, to 2 minutes. The log line `session setup failed, next attempt delayed` names the delay | The label withdraw events and `reconcilePeerDown` still run: they are no-ops for a session that learned nothing |
| AC-4 | A setup attempt ends before the session is operational | The bus carries the same session events as before the attempt: the attempt published no `(ldp, session-up)`, so nothing answers one. An OSPF LDP-sync interface keeps the state it had, and its stuck timer keeps its deadline | SessionDown for a session that had published SessionUp (AC-2) |
| AC-5 | An adjacency expires, whatever state its session is in | The adjacency has no backoff entry once its session has ended, so a neighbor rediscovered later is dialled on its first Hello | SessionDown when the session was operational (AC-2 holds here too) |
| AC-6 | The LDP page `docs/architecture/ldp/mpls-ldp.md` | Its backoff decision and its panic-guard row state what AC-1 to AC-5 state, with source anchors | None: a page statement the code contradicts is a defect |
| AC-7 | An L2TP session's state changes in the order A then B under `tunnelsMu`, on any two goroutines | Every reader, in process or forked, receives A's notifications before B's. The route observer does too | Notifications of different sessions interleave freely: no reader relates them |
| AC-8 | A session is torn down by the operator, a timeout or a CoA Disconnect while its IP-assigned or session-up event is being handled | After both complete, the subscriber registry holds no entry for the session, RADIUS accounting holds no session for it and has sent a Stop for every Start, the route observer holds no record, and no shaper or CoS state remains | A reader that never saw the session: an event for a session already gone from the tunnel is not published at all, as today |
| AC-9 | An L2TP session ends through any path with RADIUS accounting, pool, shaper and CoS loaded | Each reader receives exactly one SessionDown and releases what it held, as before this change | The second SessionDown of a session whose LCP dropped in the network phase and which was then torn down: two lifetimes ended |
| AC-10 | PPP reports an address and waits on its acknowledgement | The acknowledgement arrives after every reader has returned, as today | None: PPP enables forwarding on it |
| AC-11 | The FlowSpec plugin holds a live announcement and receives the engine's `bye` | The peer receives the withdrawal before the plugin's process is stopped, `show ddos flowspec` reports nothing announced, and the record is removed | A withdrawal already made by the apply of the same reload: the handler then finds nothing live and sends nothing |
| AC-12 | The whole `ddos` block is deleted with a rule announced | The peer receives the withdrawal during the reload, and the log carries `the ddos flowspec section was removed, withdrawing the announcement` | None: the plugin is stopped by the same reload and nothing else could withdraw |
| AC-13 | The FlowSpec plugin's `Run` returns, or a panic unwinds its engine function, while the engine still answers | The stop function withdraws under a context of its own, bounded and independent of the signal context, and removes the record | When the transport is already closed (daemon stop), no withdrawal is sent and one `INFO` line says the BGP sessions end with the daemon: the peers drop the rule with the session, or hold it as stale until their restart timer under graceful restart |
| AC-14 | The plugin announces a rule | The announcement is recorded in the plugin's runtime state before the function returns, and removed when the rule is withdrawn by any path | A record left by a daemon stop or a kill: AC-15 consumes it |
| AC-15 | The plugin starts and finds a recorded rule | It sends the withdrawal for that rule, logs it at `WARN` with the victim prefix, and removes the record. It adopts nothing: protection comes from fresh detection | A withdrawal that reaches no peer because none is established yet: sessions that come up later never held the rule |
| AC-16 | A recorded rule exists and the running plugin's responder does not hold it as its live announcement (a start-time withdrawal that failed) | `ze doctor` reports it through the plugin's runtime check, with the victim prefix, why it is there, and the exact command that withdraws it. With the plugin's process ended, `ze doctor` still reports the record (A-13 settles by which check) | None while the condition holds. The report clears when the record is removed |
| AC-17 | `docs/guide/ddos-mitigation.md` | The FlowSpec table has a row for the parent block, for `bye`, for a daemon stop and for a crash, and the ddos local parent-block row states what the code does since 2026-10-10 | None: a page statement the code contradicts is a defect |
| AC-18 | A reader of one tunnel's session event takes 10 seconds, and meanwhile the operator clears a session on another tunnel and runs `show l2tp session` | Both complete without waiting for the slow reader: the other tunnel's session is removed and its SessionDown reaches the readers, and the show answers. Later notifications of the slow reader's own tunnel are delivered after it returns, in order | Events carried by the reactor goroutine while it is itself inside the slow publication: it publishes synchronously, today and after (R-1) |
| AC-21 | Two LDP neighbors are discovered on one interface, one session is operational, and a setup attempt to the other fails | No `(ldp, session-down)` is published for the interface, and an OSPF LDP-sync machine on it stays in the state the operational session gave it | SessionDown when the operational session itself ends: RFC 5443 Section 3 leaves costing out a shared link for one peer to policy, and Ze costs the link out |

AC-19 and AC-20 moved to `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md`. The remaining ids are unchanged.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Restarts LDP on a neighbor of a Ze router | neighbor closes TCP -> `runSession` ends -> `onDone` clears backoff, publishes SessionDown -> next Hello -> `startSessionForAdj` dials | `ldp-backoff-clear-frr`, `test/ldp/ldp-session-end-clears-backoff.ci` |
| 2 | Runs OSPF LDP-sync beside a neighbor that rejects Ze's session parameters | NAK -> `onDone` records the failure, publishes nothing -> OSPF state unchanged | `TestSessionEndBeforeOperationalPublishesNoSessionDown`, `test/ldp/ldp-setup-failure-publishes-no-session-down.ci` |
| 3 | Clears a subscriber session while it is still negotiating its address | CLI -> `teardownSessionByID` and `handleSessionIPAssigned` -> ordered publication -> readers | `TestSessionIPAssignedIsPublishedBeforeTheTeardownThatFollowsIt`, `test/l2tp/session-clear-leaves-no-subscriber.ci` |
| 4 | Deletes the `ddos` block during an attack | reload -> apply with the section absent -> withdrawal -> peer | `test/reload/ddos-flowspec-parent-removed-withdraws.ci` |
| 5 | Runs `ze doctor` after the FlowSpec plugin died with a rule announced | state record -> doctor check -> report with the fix command | `test/plugin/ddos-flowspec-orphan-doctor.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRunSessionReportsOperationalToOnDone` | `internal/plugins/ldp/session_end_test.go` | A session driven to operational over an in-memory connection hands true to `onDone` | |
| `TestRunSessionReportsNotOperationalToOnDone` | `internal/plugins/ldp/session_end_test.go` | A session whose peer closes during Initialization hands false | |
| `TestSessionEndAfterOperationalClearsBackoff` | `internal/plugins/ldp/session_end_test.go` | AC-1: an existing entry is gone after the end | |
| `TestSessionEndAfterOperationalPublishesOneSessionDown` | `internal/plugins/ldp/session_end_test.go` | AC-2 | |
| `TestSessionEndBeforeOperationalRecordsSetupFailure` | `internal/plugins/ldp/session_end_test.go` | AC-3: the entry exists and its delay is 15 seconds | |
| `TestSessionEndBeforeOperationalPublishesNoSessionDown` | `internal/plugins/ldp/session_end_test.go` | AC-4 | |
| `TestAdjacencyExpiryLeavesNoBackoffEntry` | `internal/plugins/ldp/adjacency_expiry_test.go` | AC-5, for a session that never reached operational | |
| `TestSessionIPAssignedIsPublishedBeforeTheTeardownThatFollowsIt` | `internal/component/l2tp/session_event_order_test.go` | AC-7: a reader that blocks the first publication proves the teardown's SessionDown waits for it. Controlled seam: the interleaving cannot be forced through a daemon | |
| `TestSessionUpIsPublishedBeforeTheTeardownThatFollowsIt` | `internal/component/l2tp/session_event_order_test.go` | AC-7 for SessionUp | |
| `TestRouteObserverSeesIPUpBeforeTheDownThatFollowsIt` | `internal/component/l2tp/session_event_order_test.go` | AC-7 for the route observer: no record remains | |
| `TestTeardownDuringIPAssignedLeavesNoSubscriberEntry` | `internal/component/l2tp/session_event_order_test.go` | AC-8 at the bridge | |
| `TestTeardownDuringIPAssignedLeavesNoAccountingSession` | `internal/component/l2tp/plugins/authradius/acct_order_test.go` | AC-8 at RADIUS accounting, driven through a real bus | |
| `TestSessionEventReadersDoNotCallBackIntoTheReactor` | `internal/component/l2tp/session_event_order_test.go` | A-3: every registered reader handles each of the three events against a reactor whose entry points fail the test when called | |
| `TestSlowReaderDelaysNoStateChange` | `internal/component/l2tp/session_event_order_test.go` | AC-18: with a reader blocked on one tunnel, a teardown on another tunnel and a session lookup both return | |
| `TestTunnelTeardownDownsFollowEarlierEventsOfItsSessions` | `internal/component/l2tp/session_event_order_test.go` | AC-7 for a tunnel teardown: each session's down is delivered after that session's pending IP-assigned | |
| `TestEveryIssuedTicketIsFinished` | `internal/component/l2tp/session_event_order_test.go` | R-3: after each of the twelve paths, tickets issued equals tickets finished, a panicking reader included | |
| `TestFailedSetupBesideAnOperationalNeighborPublishesNoSessionDown` | `internal/plugins/ldp/session_end_test.go` | AC-21: two adjacencies on one interface, one operational, the other's attempt fails, the bus carries no SessionDown | |
| `TestSessionUpStartsTimeoutsAfterPublication` | `internal/component/l2tp/session_event_order_test.go` | A-5 | |
| `TestIPAssignedAcknowledgesAfterReadersReturn` | `internal/component/l2tp/session_event_order_test.go` | AC-10 | |
| `TestByeWithdrawsTheLiveAnnouncement` | `internal/plugins/ddos/flowspec/exit_test.go` | AC-11 | |
| `TestByeWithNothingAnnouncedSendsNothing` | `internal/plugins/ddos/flowspec/exit_test.go` | AC-11 survivor | |
| `TestStopResponderWithdrawsUnderItsOwnContext` | `internal/plugins/ddos/flowspec/exit_test.go` | AC-13: the dispatch succeeds with the signal context cancelled | |
| `TestStopResponderOnAClosedTransportLogsOnce` | `internal/plugins/ddos/flowspec/exit_test.go` | AC-13 survivor: no error line, the record stays | |
| `TestAnnounceRecordsTheRule` | `internal/plugins/ddos/flowspec/record_test.go` | AC-14, write half | |
| `TestWithdrawRemovesTheRecord` | `internal/plugins/ddos/flowspec/record_test.go` | AC-14, remove half | |
| `TestStartWithdrawsTheRecordedAnnouncement` | `internal/plugins/ddos/flowspec/record_test.go` | AC-15 | |
| `TestOrphanCheckReportsARecordWithNoLiveResponder` | `internal/plugins/ddos/flowspec/doctor_test.go` | AC-16, positive | |
| `TestOrphanCheckIsQuietForTheLiveAnnouncement` | `internal/plugins/ddos/flowspec/doctor_test.go` | AC-16, negative | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | This spec adds no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ldp-session-end-clears-backoff` | `test/ldp/ldp-session-end-clears-backoff.ci` | A fixture neighbor on a veth pair brings a session to operational, closes it, and observes Ze's next connection within one Hello interval. `needs-linux` | |
| `ldp-setup-failure-publishes-no-session-down` | `test/ldp/ldp-setup-failure-publishes-no-session-down.ci` | The fixture rejects Ze's Initialization. A subscribed observer plugin receives no `(ldp, session-down)`, and the daemon logs the 15-second delay. `needs-linux` | |
| `session-clear-leaves-no-subscriber` | `test/l2tp/session-clear-leaves-no-subscriber.ci` | A real PPP session reaches its address, the operator clears it, the RADIUS fixture has one Start and one Stop, and the session and subscriber lists are empty. Modelled on `test/l2tp/radius-acct-wire.ci` | |
| `ddos-flowspec-parent-removed-withdraws` | `test/reload/ddos-flowspec-parent-removed-withdraws.ci` | A rule is announced, the config is reloaded without the `ddos` block, the peer receives the withdrawal. Modelled on `test/plugin/ddos-announce-rate-limit.ci` | |
| `ddos-flowspec-orphan-doctor` | `test/plugin/ddos-flowspec-orphan-doctor.ci` | A record is present with no responder, `ze doctor` prints the code, the prefix and the fix command | |

Each `.ci` is drafted under `test/draft/<suite>/` and promoted (`/ze-test`).
The FlowSpec plugin runs in the daemon only, so the external-mode half of the
lifecycle rule in `ai/rules/testing.md` cannot be run for it: `loadBus` refuses.

Lifecycle scenarios of `ai/rules/testing.md` this change can reach: none of the
ten rows. It edits no start-up, supervision, failure-policy, reload or stop
code. It uses `bye` on removal and the reload's apply as they are.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ldp-backoff-clear-frr` | `test/interop/scenarios/` | FRR | An operational session cleared on FRR is re-established by Ze within one Hello interval, where the unfixed code waits 15 seconds or more. Red with the fix reverted and the image rebuilt | |

L2TP: none owed. Nothing a LAC or LNS can observe changes: the fix orders
events between Ze's own components.
DDoS FlowSpec: no new scenario. The withdrawal's bytes come from the existing
renderer and encoder. What changes is the occasion, which the `.ci` proves at a
BGP peer with a hex expectation.

## Files to Modify
- `internal/plugins/ldp/register.go` - `runSession` hands the operational fact to `onDone`. `onDone` branches on it. The backoff comment quotes each RFC sentence on its own
- `internal/plugins/ldp/session.go` - the session records that its adjacency expired before it is stopped
- `internal/component/l2tp/reactor.go` - the emit-order lock, and the packet and tick paths
- `internal/component/l2tp/reactor_kernel.go` - `handlePPPEvent`, `handleLCPDown`, `handleSessionIPAssigned`, `handleSessionUp`, `handleKernelError`
- `internal/component/l2tp/teardown.go` - the three teardown paths, and the comments on `emitSessionDowns`
- `internal/component/l2tp/events/events.go` - the reader contract and the stale reader names in its comments
- `internal/component/l2tp/session.go` - the tunnel's order ticket, issued by `drainPendingTeardowns`
- `internal/plugins/ddos/flowspec/register.go` - the stop function, the `OnBye` handler, the deferred call, the start-time read, the state key grant
- `internal/plugins/ddos/flowspec/responder.go` - record on announce, remove on withdraw, the exit dispatch context
- `internal/core/diagnostic/codes.go` - the doctor code
- `docs/architecture/ldp/mpls-ldp.md`, `ai/digests/mpls-signaling.md` - LDP session end
- `docs/architecture/l2tp/subscriber-session-model.md`, `ai/digests/subscriber.md` - event order and the reader contract
- `docs/guide/ddos-mitigation.md` - the FlowSpec rows and the ddos local parent-block row
- `plan/journal/state-read-after-teardown-reset-it.md`, `plan/journal/late-write-lands-on-the-successor.md`, `plan/journal/component-rebuilt-during-reload.md` - each row marked fixed in the commit that fixes it

## Files to Create
- `internal/plugins/ldp/session_end_test.go`
- `internal/component/l2tp/session_event_order_test.go`
- `internal/component/l2tp/plugins/authradius/acct_order_test.go`
- `internal/plugins/ddos/flowspec/exit_test.go`, `record_test.go`, `doctor.go`, `doctor_test.go`
- `test/ldp/ldp-session-end-clears-backoff.ci`, `test/ldp/ldp-setup-failure-publishes-no-session-down.ci`, and the fixture neighbor they share
- `test/l2tp/session-clear-leaves-no-subscriber.ci`
- `test/reload/ddos-flowspec-parent-removed-withdraws.ci`, `test/plugin/ddos-flowspec-orphan-doctor.ci`
- `test/interop/scenarios/ldp-backoff-clear-frr/`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No RPC and no config leaf is added |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command is added or changed |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC. The five `.ci` tests above prove behavior |
| Pipe completeness | N-A | No command output changes |
| Env var registration | N-A | None |
| Doctor check for runtime dependencies | Yes | The plugin runtime state key: a runtime check declared in the plugin's `sdk.Registration` `DoctorChecks` and answered from `OnDoctorCheck` (`internal/plugins/ddos/flowspec/doctor.go` holds the check, as `ddos/detect` does), code in `internal/core/diagnostic/codes.go` |
| Doctor code per problem class | Yes | `doctor-ddos-flowspec-orphan-announcement`, runtime phase: a recorded rule no responder owns (AC-16). The `WARN` at start (AC-15) and the `WARN` on a failed state write (R-8) name it. LDP and L2TP add no problem class: they remove two |
| Prometheus counters/metrics | No | No counter is added. The existing LDP session gauges and L2TP counters are unchanged |
| BGP family surface (new SAFI / capability / attribute) | N-A | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Three defect fixes. Phase 3 runs `./le feature report` for the ddos entry and records the answer |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | No plugin is added and no registration field changes |
| 6 | Has a user guide page? | Yes | `docs/guide/ddos-mitigation.md` (phase 3). `docs/guide/mpls.md` and `docs/guide/l2tp.md`: each phase greps them for the changed behavior and edits a statement it makes wrong |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | The SDK is used as documented |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No requirement row gains or loses proof. The tagged backoff tests are untouched |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: the `ldp` suite gains its first live-neighbor fixture (phase 1) |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | Yes | `docs/architecture/ldp/mpls-ldp.md` (phase 1), `docs/architecture/l2tp/subscriber-session-model.md` (phase 2) |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No event type is added. The doctor code inventory gains one entry, generated from `codes.go` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Each phase runs `./le spec citation anchors spec plan/release/fixes/spec-single-owner-lifecycle-5-session-end-locals.md` and names each declared page here. `internal/component/l2tp/events/events.go` declares `docs/guide/l2tp.md`. Three declared pages are named here as unaffected, each to be confirmed by a grep in its phase: `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` (the detector design; it states nothing about the responder's exit), `docs/research/l2tpv2-ze-integration.md` (research notes on the reactor; no statement about event order), `docs/research/l2tpv2-implementation-guide.md` (protocol notes; it says a session id is unique within the tunnel, and nothing about event order), `docs/features/ai-first.md` (the diagnostic code catalog page; it lists no code by hand) |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No example changes |

## Implementation Steps

Each phase is one commit that stands alone. They share no file and can land in
any order.

1. **Phase: LDP session end** -- wiring first: `TestRunSessionReportsOperationalToOnDone` fails because `onDone` takes no argument
   - Tests: the seven LDP unit tests, the two `ldp` `.ci` tests, `ldp-backoff-clear-frr`, and a re-run of `test/ospf/ospf-ldp-sync-restore.ci`
   - Files: `internal/plugins/ldp/register.go`, `session.go`, `session_end_test.go`, `adjacency_expiry_test.go`, the two pages, the journal row
   - Steps: (a) `runSession` keeps a flag its on-operational callback sets, and passes it to `onDone` from a deferred call that reads it when it runs. (b) `onDone` with the flag set deletes the backoff entry and publishes SessionDown. Without it, it records a setup failure and publishes no SessionDown. Both still remove the session, update the gauge, and run `reconcilePeerDown` with its label withdraw events. (c) `expireAdjacencies` marks the session as ended by expiry before stopping it, and `onDone` records no failure for such a session. (d) The backoff comment quotes each RFC 5036 Section 2.5.3 sentence separately, byte-exact. (e) The page and the digest
   - Verify: tests fail, implement, tests pass. The interop scenario is red with (b) reverted and the image rebuilt
2. **Phase: L2TP event order** -- wiring first: `TestSessionIPAssignedIsPublishedBeforeTheTeardownThatFollowsIt` fails
   - Tests: the eleven L2TP unit tests, `session-clear-leaves-no-subscriber.ci`
   - Files: `internal/component/l2tp/reactor.go`, `reactor_kernel.go`, `teardown.go`, `events/events.go`, the test files, the page, the digest, the journal row
   - Steps: (b) Give the tunnel its order ticket as "The order ticket" in Data Flow states it. (c) `drainPendingTeardowns` issues the ticket for the batch it returns when the batch holds a down, and the ticket travels with the batch through `discardTunnelLocked`, `reapExpiredLocked` and `resolveTieBreakerLocked`. (d) In each of the twelve publishing paths (nine `emitSessionDowns` callers, `handleLCPDown`, `handleSessionIPAssigned`, `handleSessionUp`): take the ticket while `tunnelsMu` is held, release `tunnelsMu`, wait for the turn, tell the route observer, publish, finish the ticket through a deferred call. A path with nothing to deliver takes none. `startSessionTimeouts`, `ClearSessionMetadata`, outbound sends and kernel events keep their place after the publication. (g) Correct both comments on `emitSessionDowns` and the comment on `drainPendingTeardowns`, and write the reader contract on the events package. (h) The page and the digest
   - Verify: tests fail, implement, tests pass, then the package under the race detector twenty times through `./le job run`, output pasted
3. **Phase: DDoS FlowSpec exit** -- wiring first: `TestByeWithdrawsTheLiveAnnouncement` fails because no handler is registered
   - Tests: the nine FlowSpec unit tests, the two `.ci` tests
   - Files: `internal/plugins/ddos/flowspec/register.go`, `responder.go`, `doctor.go`, the test files, `internal/core/diagnostic/codes.go`, the guide, the journal row
   - Steps: (a) Read the daemon's collection of plugin doctor answers and settle A-13. (b) One stop function, shaped as the sibling's: under the responder's lock, withdraw what is live through a dispatcher whose context is the function's own and bounded below `pluginRemovalGrace`, retire, remove the record. A closed transport is one `INFO` line and leaves the record. (c) Call it from an `OnBye` handler and from a deferred call in `runEngine` registered so it runs after the cap worker has stopped. The second call finds nothing live. (d) Record the rule when an announcement succeeds and remove the record on every withdrawal. Grant the key with `statestore.RegisterPluginKeys`. (e) In `OnStarted`, read the record: when present, withdraw what it names, log at `WARN`, remove it. (f) The doctor check, built as `internal/plugins/ddos/detect/register.go` builds its own: declared in the `sdk.Registration` `DoctorChecks`, answered from `OnDoctorCheck`, with its code in `codes.go` and the fix command on the last line. Remove the record with `StateRemove`. (g) The guide
   - Verify: tests fail, implement, tests pass

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol |
| Feature completeness | Every user story has a working path |
| Correctness | LDP: the flag is read when the deferred call runs. L2TP: all twelve paths, the ticket issued under `tunnelsMu`, `tunnelsMu` never held across the wait or a publication, the ticket finished by a deferred call. DDoS: the exit context is not derived from the signal context |
| Naming | The doctor code follows the `doctor-<area>-<problem>` form of its neighbors in `codes.go` |
| Data flow | The L2TP reactor names no reader. The FlowSpec plugin reaches the engine only through the SDK |
| Operator errors and doctor codes | The `WARN` lines of AC-15 and R-8 name the doctor code, and the check's `Fix` is the exact withdraw command |
| Rule: `ai/rules/stale-comments.md` | The `onDone` comment, both `emitSessionDowns` comments, the `runSession` comment and the `runEngine` tail describe the new behavior |
| Rule: `ai/rules/rfc-compliance.md` | The RFC 5036 quotes are byte-exact, one sentence per quotation, and no tagged test changed |
| Rule: `ai/rules/goroutine-lifecycle.md` | No goroutine is added in any phase |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| LDP unit tests | `./le test unit pkg path ./internal/plugins/ldp/` |
| L2TP unit tests under the race detector | `./le test unit pkg path ./internal/component/l2tp/` and the twenty-run race pass |
| FlowSpec unit tests | `./le test unit pkg path ./internal/plugins/ddos/flowspec/` |
| Functional tests | `./le test ldp`, `./le test functional l2tp`, `./le test functional reload`, `./le test functional plugin`, each for the named test |
| Interop | `./le test integration interop` with `INTEROP_SCENARIO=ldp-backoff-clear-frr` |
| Pages | `./le doc check all` |
| Journal rows | Each of the four owned rows reads `fixed` with its test names |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Remote stall | A LAC or a RADIUS server that stops answering delays only later events of the same tunnel, for no longer than the reader's own bound (10 seconds for Accounting-Stop), and never a state change. A peer cannot make a ticket wait for ever. The reactor goroutine's own stall while it publishes (R-1) is unchanged here and owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` |
| Peer-driven backoff bypass | A neighbor cannot clear its backoff without completing Initialization and KeepAlive: the flag is set only by the on-operational callback |
| State record content | The record holds a victim prefix and match fields, no credential, and its key is granted to this plugin only |
| Withdrawal on untrusted input | The start-time withdrawal renders from a record this plugin wrote. A corrupt record is reported by the state call and by the doctor check, never rendered |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-3 is broken | Stop phase 2 and report: the mechanism in Key Design Decisions has a named fallback, and it is a design choice |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The LDP row asked for a reordering of two defers. `Stop` has three callers, so the state is gone at session end whichever defer runs first. The fact has to come from the code that saw it happen.
- An absence at a reader is not a signal. At every L2TP reader "no entry for this session" is the ordinary first event, so no reader can tell a late event from an early one. Order has to be given by the writer.
- "The exit path withdraws" is two different problems. Where the engine still answers (`bye`, a panic unwinding, a returned `Run`), the plugin withdraws. Where it does not (daemon stop, a killed process), nothing the plugin does on the way out can reach a peer, so the rule is recorded while it is live and settled at the next start.
- A lock orders critical sections, and here the thing to order is what happens after the critical section. Holding a second lock across the publication orders it and makes every state change wait for the slowest reader. A ticket taken inside the critical section and waited for outside it orders the publication and holds nothing.
- One defect met on the way is owned here with its journal row (2026-11-11): the FlowSpec rule nothing remembers after a generation ends (AC-14 to AC-16). Four L2TP defects met on the way are recorded in the journal and owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` ("Rows considered and not owned" below).
- A runtime doctor check cannot be a registered function: the registry refuses it, because the function would read the daemon's copy of a forked plugin's state. The plugin answers it itself, so a dead plugin answers nothing (A-13).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/state-read-after-teardown-reset-it.md` | 2026-10-02 | LDP `startSessionForAdj` onDone reads the state `Stop` reset |
| `plan/journal/late-write-lands-on-the-successor.md` | 2026-10-14 | L2TP SessionDown can reach readers before SessionIPAssigned |
| `plan/journal/component-rebuilt-during-reload.md` | 2026-09-20 | ddos flowspec engine exit withdraws nothing |
| `plan/journal/component-rebuilt-during-reload.md` | 2026-11-11 | ddos flowspec generation end leaves a rule nothing remembers |

## Rows considered and not owned

| Class file | Date | Surface (short) | Why not this root cause |
|------------|------|-----------------|-------------------------|
| identifier-reused-after-its-owner-is-gone | 2026-11-11 | L2TP PPP event applied to a successor with a reused session id | Session identity, not event order: owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` |
| key-omits-a-fact-the-builder-uses | 2026-11-11 | L2TP route observer and CQM observer keyed by session id alone | Session identity: owned by the same spec |
| key-omits-a-fact-the-builder-uses | 2026-11-11 | L2TP `teardownSessionByID` ends the first session with an id on any tunnel | Session identity: owned by the same spec |
| slow-reader-stalls-a-shared-goroutine | 2026-11-11 | L2TP reactor goroutine waits on a synchronous Accounting-Stop | Where a reader's wait runs, not event order: owned by the same spec |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| LDP: `runSession` hands "reached operational" to `onDone` | Swap the two defers. Add a "was operational" field read through `State` | `Stop` is called from three places, so the swap fixes one of them. The callback already fires once on the right goroutine, so the fact needs no lock and no new session state |
| LDP: no SessionDown for a session that never published SessionUp | Keep publishing and make OSPF ignore a down with no up | The reader cannot tell the two apart from the payload, and a third-party reader would need the same guard. The writer knows |
| L2TP: a per-tunnel order ticket taken under `tunnelsMu` and waited for after it is released | (1) An emit-order lock taken before `tunnelsMu` is released and held across the publication. (2) A queue with one emitter goroutine. (3) Publish while holding `tunnelsMu`. (4) A guard at each reader. (5) One ticket sequence for the whole reactor | (1) is rejected: a path would wait for the lock while holding `tunnelsMu`, and the RADIUS accounting reader sends Accounting-Stop synchronously with a 10-second bound, so one slow RADIUS server would stop every state change in the reactor for 10 seconds. (2) is rejected: PPP waits on `Acknowledge` until publication finished, so the queue would have to carry the acknowledgement, and it is unbounded. (3) is rejected: readers do I/O. (4) is rejected: "unknown session" is the normal first event, `handleLCPDown` ends a lifetime of a session that continues, and there are five in-tree readers. (5) is rejected: it makes a slow reader delay every tunnel's events. The tunnel is enough because it leaves the maps in one place after draining, and a reused session id stays inside it |
| L2TP: the route observer is inside the ordered section | Leave it outside, as the journal row names only the bus | It has the same defect: an IP-up after the down leaves an injected route |
| DDoS: withdraw in the `bye` handler and on the way out, through one function | Only a deferred call after `Run` returns, on a fresh context (the design as approved) | After `Run` returns on a removal or a daemon stop the connection is closed or closing, so a fresh context alone reaches nobody. `bye` is the point where the engine waits for the plugin with the connection open. The deferred call stays for the ends where the engine still answers |
| DDoS: record the live rule and withdraw it at the next start | Engine-side tracking of update-route announcements per plugin. Re-adopt the rule at start | Engine tracking is the general fix and belongs to the plugin-process and BGP engine work, not to a local fix. Re-adopting needs the probe, the clock and the rate-limit window persisted, and the sibling's precedent is that protection after a restart comes from fresh detection |

## Known Limitations

- This spec does not cure the reactor stall of R-1: the reactor goroutine publishes synchronously, and a slow reader still stops packet processing for every tunnel while that goroutine is inside the publication or waiting its turn for the same tunnel. That is an open defect with a journal row (`plan/journal/slow-reader-stalls-a-shared-goroutine.md`, 2026-11-11), owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md`.
- On a shared segment, the end of one operational LDP session costs the link out while another neighbor's session is up. RFC 5443 Section 3 leaves that to policy, and this spec does not change it.
- `teardownSessionByID` ends the first session it finds with a given id on any tunnel. Session ids are unique per tunnel only, so an operator clearing by id alone can hit another tunnel's session. Not changed here: an open defect with a journal row (`plan/journal/key-omits-a-fact-the-builder-uses.md`, 2026-11-11), owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md`.
- Update-route announcements are not tracked per plugin by the engine. Any other plugin that announces through update-route and dies has the same exposure. That is engine work for the plugin-process child or a spec of its own, raised to the parent.
- At a daemon stop no withdrawal is sent. The BGP sessions end with the daemon.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

For this spec: the two RFC 5036 Section 2.5.3 backoff sentences above the
backoff constants, each as its own quotation; the "no further need to throttle"
sentence above the branch that clears the entry; the "session is operational"
sentence stays where it is, above the on-operational callback.

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
- [ ] AC-1..AC-18 and AC-21 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. GitHub CI verifies the pushed commit, and a red CI reports gets a fix commit
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
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
