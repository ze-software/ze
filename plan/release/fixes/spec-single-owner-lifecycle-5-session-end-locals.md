# Spec: single-owner-lifecycle-5-session-end-locals

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Priority | P1 |
| Release reason | Three shipped-daemon defects an operator meets in production: an LDP session that ended cleanly is retried as if it had failed and costs an OSPF link out, an L2TP subscriber that hung up stays in the subscriber table with an accounting record nobody stops, and a FlowSpec discard rule stays at every peer after the plugin that announced it is gone. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-single-owner-lifecycle.md` (parent: decision record and row routing; no code dependency). No dependency on children 1 to 4: this child edits no file they edit |
| Phase | closure: all three code phases committed (`4f1e24bf66`, `bae3903dbb`, `bec9096156`), the five daemon-level proofs and the interop `ldp-backoff-clear-frr` (`e299b1e031`) committed and proven, review rounds one to seven fixed (`3a19b67b81`, `c60a2b6edc`, `15d5cdd19b`, `e7e963b4e1`, `bd27660e0d`, `b6d1739194` with `944149988c`, `fd6fedfc62`), the stale RFC 5036 and RFC 2661 records re-taken (`8f45f32b93`, `4e4709af9d`), review round eight running over `fd6fedfc62` |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-13 |

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
| No bypassed layers (data flows through the intended path) | Yes | LDP publishes through `emitSessionEvent` on the EventBus (`ldp/register.go`, the session-end callback in `startSessionForAdj`). L2TP publishes through the typed handles in `l2tp/events` from `ticket.deliver` (`reactor_kernel.go` `handleSessionIPAssigned`, `teardown.go` `notifySessionDowns`). The FlowSpec plugin reaches the engine through the SDK dispatcher (update-route) and the SDK state RPCs only (`flowspec/register.go` `stopResponder`, `responder.go` `withdrawThrough`) |
| No unintended coupling (components stay isolated) | Yes | No new import between components: `flowspec/register.go` imports `plugin/registry`, core leaves and `pkg/`. The L2TP reactor names no reader (the contract is on the `l2tp/events` package doc). OSPF is unchanged. `./le arch tier check` and `./le plugin boundary check` are owed commands |
| No duplicated functionality (extends existing, does not recreate) | Yes | Every row of Existing Mechanisms marked "Extends" was extended in place: the on-operational callback, the `retries` map, the tunnel's queue of downs (`session.go` `drainPendingTeardowns` issues the ticket), the sibling's `stopResponder` shape, `OnBye`, the SDK state calls, `OnDoctorCheck`. No new table, lock or goroutine |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire encoding changes. The withdrawal is the existing rendered `del` command |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | The state key is granted from the plugin's own `init()` (`flowspec/register.go` `statestore.RegisterPluginKeys(Name, announcementKey)`), the post-config check registers there too (`diagnostic.RegisterDoctorCheck(orphanRecordCheck)`), and the runtime check is a declaration in the plugin's `sdk.Registration` (`DoctorChecks: orphanRuntimeCheckDecl`) answered by `OnDoctorCheck`. The one shared-file edit is the code constant in `internal/core/diagnostic/codes.go`, the declared catalog every doctor code lives in |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Names introduced: the check names `ddos-flowspec-orphan-announcement` and `ddos-flowspec-orphan-record`, the code `doctor-ddos-flowspec-orphan-announcement`, the key `meta/ddos-flowspec/announcement`. Searched: the doctor collection (`collectDoctorChecksFrom` asks each process for its declared checks), the doctor registry (`diagnostic.RegisterDoctorCheck`), the state key grants (`statestore.RegisterPluginKeys`, read by `ze data registered`). Each derives from the registration; no list outside the plugin names them |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The on-operational callback runs on `runSession`'s goroutine, so the flag it sets needs no lock | `ReadLoop` calls it inline (`session.go`) | A data race on the flag | Read at the producer 2026-11-11; `TestRunSessionReportsOperationalToOnDone` under the race detector | confirmed: `session.go` `ReadLoop` calls `onOperational` inline, after the PDU that made the session operational. `register.go` `runSession` sets its local `operational` in that callback and hands it to `onDone` from a deferred call that reads it when it runs (R-4). Test `TestRunSessionReportsOperationalToOnDone` (`session_end_test.go`, `4f1e24bf66`); the ldp package ran green 2026-11-12 |
| A-2 | OSPF's `ldpSyncManager` is the only in-tree reader of the LDP session events | Census above | Another reader depends on a down with no up | Re-run the census search in phase 1 | confirmed: the census was re-run in phase 1 (2026-11-12) and again on 2026-11-13. The `ldp` session events are named only in `ldp/register.go`, `ldp/events.go` and `ospf/ldp_sync.go`. The `isis` and `l2tp` hits are events of the same name in their own namespaces |
| A-3 | No in-process reader of an L2TP session event calls back into the reactor from its handler | Each reader in the census read 2026-11-11: registry, sync maps, traffic backend, RADIUS client, metadata map. The pool's `lookupSessionUsername` takes it, but from its show handler | A reader that causes a publication for the same tunnel from inside its handler waits behind the ticket its caller holds, and that tunnel's events stop | `TestSessionEventReadersDoNotCallBackIntoTheReactor` (`l2tp/session_event_order_test.go`) and `TestSessionEventReaderDoesNotCallBackIntoTheReactor` in `authradius`, `shaper`, `cos` and `pool` (phase 2), and the contract written on the events package | confirmed: every reader was read again on 2026-11-12 (bridge, observer, `authradius` accounting and CoA health, shaper, CoS, pool). None calls `l2tp.LookupService` or a reactor method, or emits an L2TP session event, from its handler. Tests: `TestSessionEventReadersDoNotCallBackIntoTheReactor` (`l2tp/session_event_order_test.go`, the bridge and the CQM observer) and `TestSessionEventReaderDoesNotCallBackIntoTheReactor` in `authradius`, `shaper`, `cos` and `pool`, each over a reactor tripwire installed through `l2tp.PublishService`. The contract is on the `l2tp/events` package doc and on `deliverSessionDowns` (`bae3903dbb`) |
| A-4 | A forked reader sees events in `Emit` call order | `Deliver` puts each event on the process's channel, blocking when full | Order is lost for forked readers and the fix is partial | Read at the producer 2026-11-11 | validated by read |
| A-5 | `startSessionTimeouts` can run after the emit-order lock is released with no change in behavior | It takes `tunnelsMu` and starts timers. It reads the session again and returns when it is gone | A session timeout armed for a dead session | `TestSessionUpStartsTimeoutsAfterPublication` | confirmed: `session_timeout.go` `startSessionTimeouts` takes `tunnelsMu`, looks the tunnel and the session up again, and returns when either is gone. `handleSessionUp` calls it after `ticket.deliver` returned. `TestSessionUpStartsTimeoutsAfterPublication` (`l2tp/session_event_order_test.go`) green before and after the fix |
| A-7 | Deleting the whole `ddos` block delivers an apply to the FlowSpec plugin with `configured` false | `rootHasChanges`, `TestRootHasChangesUnderAnAddedOrRemovedAncestor`, `TestBuildDiffSectionsFilesAnAncestorKeyUnderItsDeclaredRoot` | The parent-block removal still orphans the rule, and the `bye` handler is what withdraws it | `test/reload/ddos-flowspec-parent-removed-withdraws.ci` | confirmed at daemon level (2026-11-13, `af1d0bc452`): with the `ddos` block removed by a reload, the peer receives the withdrawal on connection 1 and the log carries "the ddos flowspec section was removed, withdrawing the announcement", which only the apply path writes. Its first run found the reload refused at verify (fixed in `3e347a86e6`); with the fix kept and `withdrawForConfig`'s active check inverted, the line is missing and the test is red |
| A-8 | The BGP engine keeps no per-plugin record of update-route announcements, so nothing withdraws them when a plugin's generation ends | `withdrawPluginRoutes` covers route-install only. A search of the plugin server and the BGP reactor for a per-plugin announcement table found `installedByPlugin` (route-install) and nothing else | The recorded-rule recovery duplicates an engine mechanism | Read 2026-11-11 at `cleanupProcess`, `AnnounceNLRIBatch` and the config origin of `PluginRoutes` | validated by read |
| A-9 | A withdrawal for an NLRI no peer holds is harmless | BGP withdraws are idempotent at the receiver | A start-time withdrawal after a daemon restart disturbs a peer | Read the peer's withdraw handler at the lab's version before asserting it in a scenario | confirmed for what the proofs reach: the `.ci` peer (`ze-peer`) keeps no RIB, and in `ddos-flowspec-orphan-doctor.ci` the session that came up after the kill never held the rule. The engine withholds the start-time withdrawal from it (`reactor_api_batch.go` `splitOnAdvertised`, `logWithdrawWithheld`), and the test asserts that withheld line, so no peer receives a withdrawal for an NLRI it does not hold. No scenario asserts a third-party peer's handling of such a withdrawal |
| A-10 | A live-peer LDP `.ci` can run in the `ldp` suite with a compiled fixture playing the neighbor on a veth pair | `test/plugin/ddos-announce-rate-limit.ci` builds a veth topology from a fixture. `test/ldp/ldp-convergence.ci` says no mock peer exists today | The `.ci` needs harness work first | Phase 1 writes the fixture in `test/draft/ldp/` | confirmed (2026-11-13, `0617ddff57`): the fixture `internal/test/fixture/ldp_neighbor_linux.go` builds two namespaces joined by one veth, so the Hello is not dropped as martian and the suite's sibling daemons never hear it. Both `test/ldp/` proofs ran green in Ze's QEMU guest and 80 of 80 under load. No harness work was needed |
| A-11 | The bridge still accepts an update-route call from inside the `bye` handler | `notifyPluginRemoval` sends `bye` before `stopRemovedProcess` | The handler's withdrawal fails and the record stays | Read 2026-11-11: `handleBridgeCallback` runs the handler before the loop ends, and `beginDispatch` admits calls until `StopDispatch`, which `Process.Stop` calls after the answer. `TestByeWithdrawsTheLiveAnnouncement` over a real `sdk.Plugin` pair | confirmed, re-read 2026-11-12 and 2026-11-13: `sdk_dispatch.go` `handleBridgeCallback` runs the `bye` handler, sends its answer, and ends the loop only for a `bye` that returned no error. `bridge.go` `beginDispatch` admits a call until `dispatchClosed`, which only `StopDispatch` sets, and `process.go` `Process.Stop` calls it. `notifyPluginRemoval` waits for the answer before `stopRemovedProcess`. `TestByeWithdrawsTheLiveAnnouncement` (`ddos/flowspec/exit_test.go`, `bec9096156`) proves it on the RPC path only. The daemon-level proof over the real transport is `ddos-flowspec-parent-removed-withdraws.ci` |
| A-12 | Two tunnel objects never carry the same tunnel id closely enough for their events to be confused | `allocateLocalTID` is a counter that wraps after 65534 allocations, and a discarded tunnel's last downs are emitted by the path that discarded it | A reader sees a new tunnel's session event before the old tunnel's down for the same ids | Read at the producer 2026-11-11. No test: the window needs 65534 tunnel creations during one publication | validated by read |
| A-13 | `ze doctor` reports a declared runtime check that its plugin cannot answer because the plugin's process ended | Not read: the daemon's collection of plugin doctor answers | A recorded rule is invisible while the plugin is dead and not restarted | Phase 3 reads the doctor collection path and `docs/architecture/doctor-and-health-checks.md`. If a dead plugin's check is silently skipped, the spec gains a registered post-config check that reads the record through `statestore` when the `ddos flowspec` section is absent, and says it cannot answer where no store is open | broken (read 2026-11-12, re-read 2026-11-13): `doctor/cmd/show.go` `collectDoctorChecksFrom` asks only the plugins `Server.DoctorCheckPlugins` (`plugin/server/server.go`) lists, and that walks `ProcessManager.AllProcesses`. `startup_removal.go` `takeAndStopProcess` calls `RemoveProcess`, so a removed plugin's declared check vanishes with no line. The consequence the row names was built in `bec9096156`: `ddos/flowspec/doctor.go` `orphanRecordCheck`, a registered post-config check (`diagnostic.RegisterDoctorCheck` in `register.go` `init`) that reads the record through the store when the `ddos flowspec` section is absent, beside the plugin-answered runtime check. Test `TestOrphanRecordCheckReportsARecordWithTheSectionAbsent` |
| A-14 | The `authradius` accounting test of AC-8 can drive the reactor, so it goes red against the unfixed order | The TDD plan row for `TestTeardownDuringIPAssignedLeavesNoAccountingSession` ("driven through a real bus") | The test cannot discriminate the reactor fix | Read 2026-11-12 | broken: the reactor is unexported and offers `authradius` no seam, and the l2tp tests cannot reach `radiusAcct`. No production seam was added for it. The test is a reader-level proof: over a real bus, an IPAssigned then a SessionDown in that order leave no accounting session and one Stop per Start. The reactor's order tests in `l2tp/session_event_order_test.go` and `session-clear-leaves-no-subscriber.ci` carry the red |
| A-15 | The deferred exit in `runEngine` can reach the engine when the plugin runs in process and its `Run` returns or a panic unwinds while the daemon runs | Implicit in AC-13 and in Implementation Steps phase 3 (c); never written as an assumption | The deferred withdrawal reaches nobody in process: the rule stays at the peers and in the record until a start of the plugin withdraws it | Added by review round three (2026-11-13), read at `pkg/plugin/sdk/sdk.go` `(*Plugin).Run` | broken: `Run` defers `p.bridge.CloseCallbacks()`, which stops dispatch before `runEngine`'s deferred `stopResponder` runs, so `DispatchRPC` answers `ErrBridgeClosed` on every in-process end. Only a plugin running as its own process withdraws there (`TestRunEndingWithoutByeWithdrawsTheLiveAnnouncement`). The plugin declares no failure policy, so the daemon does not start it again by itself: the start that withdraws the recorded rule is the operator's `ze request plugin restart ddos-flowspec`. While the section stays configured, the daemon's `show doctor` reports the recorded rule itself, through the runtime-phase check `stoppedRecordCheck` (`c633ddf84d`), with the victim, why nothing withdraws it, and that command as its fix; while a cleanup of the stopped process is owed it also says ze refuses the restart until that cleanup ends. Once the saved configuration lacks the section, the post-config check `orphanRecordCheck` reports the recorded rule. The prompt in-process withdrawal is the journal row `plan/journal/exit-cleanup-runs-after-its-transport-closed.md` (2026-11-13, `ada8f1c7a6`), owned by `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` |

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
| R-10 | The LDP interop check asserts an FRR command or output FRR does not have | The scenario cannot pass | Read FRR's `ldpd` source for the clear command and the neighbor output at the lab's version, and name the function in the scenario, before writing the check. Done: FRR 10.7.1 `ldp_vty_clear_nbr`, `nbr_clear_ctl` and `show_nbr_msg_json` are named in `check_ldp_backoff_clear.go`, and the scenario passed green, red, green (`e299b1e031`) |

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
| An LDP neighbor through the daemon: session up, closed, redialled on the next Hello | → | `startSessionForAdj`, `onDone` | `test/ldp/ldp-session-end-clears-backoff.ci` (green, red observed, `0617ddff57`) |
| A PPP IP-assigned event handled while another goroutine tears the session down | → | the ordered section in `handleSessionIPAssigned` and `teardownSessionByID` | `TestSessionIPAssignedIsPublishedBeforeTheTeardownThatFollowsIt` |
| A real PPP session through the daemon, cleared by the operator | → | `teardownSessionByID`, `deliverSessionDowns` (it replaced `emitSessionDowns`), the readers | `test/l2tp/session-clear-leaves-no-subscriber.ci` (green, red observed, `525577107c`) |
| The engine's `bye` to the FlowSpec plugin | → | the `OnBye` handler, the stop function, `responder.withdraw` | `TestByeWithdrawsTheLiveAnnouncement` |
| A reload that deletes the whole `ddos` block with a rule announced | → | `rootHasChanges`, `replaceResponder` | `test/reload/ddos-flowspec-parent-removed-withdraws.ci` (green, red observed, `af1d0bc452`) |
| The FlowSpec plugin starts with a recorded rule | → | the start path, `responder.withdraw` | `TestStartWithdrawsTheRecordedAnnouncement` |
| `ze doctor` with a recorded rule and no live responder | → | the FlowSpec runtime doctor check (`orphanRuntimeCheckDecl`) | `test/plugin/ddos-flowspec-orphan-doctor.ci` (green, red observed, `af1d0bc452`; green at its promoted path on 2026-11-13 against the `c60a2b6edc` sources) |
| `show doctor` through the daemon with a recorded rule, `ddos flowspec` configured and the plugin not running | → | the runtime-phase check `stoppedRecordCheck` (`checkStoppedRecord`, `judgeStoppedRecord`) | `test/plugin/ddos-flowspec-stopped-orphan-doctor.ci` (`c60b3fbbbb`, reworked in `c633ddf84d`) |

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

AC-16, who clears an orphan record:
→ Decision: the report's first fix withdrew the rule at the peers and nothing then removed the record, so the warning stood after the operator did what it said. Thomas decided "a + c" on 2026-11-12.
→ Decision: half (a), the section is configured. The fix the report prints is `ze request plugin restart ddos-flowspec`: the start path withdraws the recorded rule from its own record and removes it. Built in `85507073e8` (`doctor.go` `unownedOrphanDiagnostic`, `responder.go` `withdrawRecordedAnnouncement`). Unit proof: `TestRestartWithdrawsTheOrphanAndClearsTheReport`. Daemon-level proof: `ddos-flowspec-orphan-doctor.ci`. The same command is the fix of the report a configured plugin that is not running gets in the daemon (`stoppedRecordCheck`, `c633ddf84d`); while a stopped process still owes its cleanup, ze refuses that restart, and the report says so.
→ Decision: half (c), the section is removed. The operator withdraws with the command the report prints, then removes the record through the running daemon. It moved to `plan/release/fixes/spec-data-store-maintenance-through-the-daemon.md` (Work Not Done).
→ Constraint: building (a) showed that one key holds one record, so a new announcement overwrote the record of an earlier rule that was never withdrawn. Since `85507073e8` an announcement withdraws such a rule first, and announces nothing if that fails. Two failed state writes in a row can still leave a live rule with no record (R-8 class, stated in the guide).

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Restarts LDP on a neighbor of a Ze router | neighbor closes TCP -> `runSession` ends -> `onDone` clears backoff, publishes SessionDown -> next Hello -> `startSessionForAdj` dials | `ldp-backoff-clear-frr` (`e299b1e031`, redial 110 ms after FRR's clear), `test/ldp/ldp-session-end-clears-backoff.ci` (`0617ddff57`) |
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
| `TestTeardownDuringIPAssignedLeavesNoAccountingSession` | `internal/component/l2tp/plugins/authradius/acct_order_test.go` | AC-8 at RADIUS accounting, a reader-level proof over a real bus: it cannot drive the reactor and does not go red against the unfixed order (A-14). The red for AC-8 is carried by `TestTeardownDuringIPAssignedLeavesNoSubscriberEntry` and the other order tests | |
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
| The `TestStoppedRecordCheck*`, `TestOrphanRecordCheckStandsDownWhileTheSectionIsConfigured`, `TestDaemonRecordReadingReadsThePublishedStore`, `TestRecordChecksRegisterAtTheirPhases` and `TestPluginRunFromSnapshotNamesTheStatesTheStoppedCheckActsOn` units | `internal/plugins/ddos/flowspec/doctor_stopped_record_test.go` | AC-16, the record in the daemon while no running process holds it: one row per record for every plugin state and saved configuration | Green; each MUTATION note observed red (`c633ddf84d`, `0c9fecde5c`) |
| `TestPluginRunFromSnapshotNamesEachState` | `internal/plugins/ddos/flowspec/doctor_stopped_test.go` | AC-16, the snapshot read | Green; its MUTATION observed red |
| `TestRuntimeChecksReceiveTheConfigurationDoctorRead` | `internal/component/doctor/doctor_test.go` | AC-16, the runtime phase is handed the saved tree the post-config phase judged | Green; its MUTATION observed red (`c633ddf84d`) |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | This spec adds no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `ldp-session-end-clears-backoff` | `test/ldp/ldp-session-end-clears-backoff.ci` | A fixture neighbor on a veth pair brings a session to operational, closes it, and observes Ze's next connection within one Hello interval. `needs-linux` | Committed `0617ddff57`. AC-1, AC-2. Green; red with the operational arm of the session-end callback made unreachable (build overlay): Ze logged `ldp: session setup failed, next attempt delayed ... delay=15s` and the observer failed "no session from Ze within 10s of the first Hello after the operational session ended"; 80 of 80 under load (four workers), twice; green at its promoted path |
| `ldp-setup-failure-publishes-no-session-down` | `test/ldp/ldp-setup-failure-publishes-no-session-down.ci` | The fixture rejects Ze's Initialization. A subscribed observer plugin receives no `(ldp, session-down)`, and the daemon logs the 15-second delay. `needs-linux` | Committed `0617ddff57`. AC-3, AC-4. Green; red with session-down published on every end: "the ldp session events for 10.99.0.1:0 are [session-down session-up session-down], want [session-up session-down]"; 80 of 80 under load (four workers); green at its promoted path. A first stress attempt lost invocation 49 to a Go runtime fatal at process start, because another session rebuilt the shared guest `le` (journal `concurrent-session-corruption.md`, `d546500e1d`); with the guest path exclusive it passed 80 of 80 |
| `session-clear-leaves-no-subscriber` | `test/l2tp/session-clear-leaves-no-subscriber.ci` | A real PPP session reaches its address, the operator clears it, the RADIUS fixture has one Start and one Stop, and the session and subscriber lists are empty. Modelled on `test/l2tp/radius-acct-wire.ci` | Committed `525577107c`. AC-8 and AC-9 for accounting and the subscriber bridge; AC-7 as a positive outcome only (pool, shaper and CoS not observed). Green; red with the session-down publication in `teardown.go` `notifySessionDowns` disabled: "the RADIUS server holds 0 Accounting-Stop records ... once the clear answered, want 1"; 80 of 80 under load (four workers); green at its promoted path |
| `ddos-flowspec-parent-removed-withdraws` | `test/reload/ddos-flowspec-parent-removed-withdraws.ci` | A rule is announced, the config is reloaded without the `ddos` block, the peer receives the withdrawal. Modelled on `test/plugin/ddos-announce-rate-limit.ci` | Committed `af1d0bc452`. AC-12, A-7. The first run was red on unmodified code and found the verify defect fixed in `3e347a86e6`. Green on the fix: `withdrawn=060120CB007109` on connection 1. Red with the fix kept and `withdrawForConfig`'s active check inverted: the section-removed line is missing; 80 of 80 under load (one worker, exclusive group); green at its promoted path |
| `ddos-flowspec-orphan-doctor` | `test/plugin/ddos-flowspec-orphan-doctor.ci` | A record is present with no responder, `ze doctor` prints the code, the prefix and the fix command | Committed `af1d0bc452`. AC-16 half (a), user story 5. Green; red with `withdrawRecordedRule`'s `del` made `add`: the `withdrawal withheld` line is missing and the peer rejects the announcement on connection 3; 80 of 80 under load (one worker). The red ran at the promoted path. The first green run at the promoted path was lost when the test host's checkout was reset during it; it was re-run on 2026-11-13 against sources equivalent to `c60a2b6edc`: PASS in 11.1 s, beside `ddos-flowspec-parent-removed-withdraws` PASS in 6.2 s, then 20 of 20 invocations of each under load |
| `ddos-flowspec-stopped-orphan-doctor` | `test/plugin/ddos-flowspec-stopped-orphan-doctor.ci` | With `ddos flowspec` configured, a record stored and the plugin recorded stopped by the daemon, `show doctor` reports the record with the victim and `ze request plugin restart ddos-flowspec` | Committed `c60b3fbbbb`, reworked in `c633ddf84d`. AC-16, the configured section with the plugin not running. Green as a draft, at its promoted path, and after each rework; red at the record's own row with the stopped report silenced; no failure in 80 stress runs under load, run on the first version before the rework changed which check answers and not repeated after; runs on any host. Limits, as its header states them: it drives a startup handshake that failed with no process held, while a plugin that ran, ended and is still held is covered by the unit table only; it does not prove the restart clears the report, which `ddos-flowspec-orphan-doctor` proves with a running plugin |

All runs 2026-11-13 in Ze's QEMU guest (kernel 7.3.0-ze) on the Linux test host; stress is `le test qemu all test <ci> stress 80` beside sixteen CPU burners. `test/ospf/ospf-ldp-sync-restore.ci` was re-run in the same guest and passes. Both FlowSpec proofs set `absolute-floor 100`, `confirm-duration 2` and a steady 16,000 pps flood, so the detector's one-time grade is critical under load. That the detector grades once and never revises is a recorded defect owned elsewhere (journal `value-computed-once-never-refreshed.md`, `83c5971c24`).

Each `.ci` is drafted under `test/draft/<suite>/` and promoted (`/ze-test`).
The FlowSpec plugin runs in the daemon only, so the external-mode half of the
lifecycle rule in `ai/rules/testing.md` cannot be run for it: `loadBus` refuses.

Lifecycle scenarios of `ai/rules/testing.md` this change can reach: none of the
ten rows. It edits no start-up, supervision, failure-policy, reload or stop
code. It uses `bye` on removal and the reload's apply as they are.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `ldp-backoff-clear-frr` | `test/interop/scenarios/` | FRR | An operational session cleared on FRR is re-established by Ze within one Hello interval, where the unfixed code waits 15 seconds or more. Red with the fix reverted and the image rebuilt | Committed `e299b1e031`. Green, red, green on 2026-11-13 (paragraph below). AC-1 interop half, user story 1, R-10 |

L2TP: none owed. Nothing a LAC or LNS can observe changes: the fix orders
events between Ze's own components.
DDoS FlowSpec: no new scenario. The withdrawal's bytes come from the existing
renderer and encoder. What changes is the occasion.
`ddos-flowspec-parent-removed-withdraws.ci` shows the withdrawal at a BGP peer
in hex, on connection 1. `ddos-flowspec-orphan-doctor.ci` cannot show one on the
wire: after the kill the new session never held the rule, so the engine
withholds the withdrawal, and the test asserts the withheld line instead.

LDP interop `ldp-backoff-clear-frr` (AC-1 interop half, user story 1) is proven,
committed in `e299b1e031`: the scenario, the checker
`internal/le/interoplab/bgp/check_ldp_backoff_clear.go` with its tests,
`register_ldp_backoff_clear.go`, `prepare.go` (`scenarioHostAddresses`,
`addHostAddresses`), the recording under `testdata/ldp-backoff-clear-frr/` with
`TestLDPBackoffClearReplaysTheLabRecording`, and the inventory row in
`docs/architecture/testing/interop.md`. The runs, on 2026-11-13 on a Docker host
running Ze's kernel (7.2.9-ze), FRR in the lab image:

| Run | Image | Result |
|-----|-------|--------|
| Green | `sha256:a218f6cd97d5` (2m46s) | The first session was operational 5 s after ze started. FRR's clear ended it at 19:04:57.959, and ze logged a new `ldp: session operational` at 19:04:58.069, 110 ms later, with no `session setup failed` line |
| Red | `sha256:3c4dee38ac2e` | The operational arm of `startSessionForAdj`'s session-end callback changed to `recordSetupFailure`, one line in the working tree for the run, because the ze image builds from the tree; restored and checksum-verified in the same command. Failure: `ze logged "ldp: session setup failed, next attempt delayed" 1 time(s) after FRR cleared a session that had reached OPERATIONAL: its end recorded a setup backoff, which delays the redial by 15 s or more (the probes ended 15.454s after the clear)` |
| Green again | `sha256:c58b3f2734ff` (2m31s) | Passes after the restore |

R-10: the checker drives FRR 10.7.1's `clear mpls ldp neighbor <address>`
(`ldp_vty_clear_nbr`, then `nbr_clear_ctl`, which matches the transport address)
and reads `show mpls ldp neighbor json` (`show_nbr_msg_json`), both named in the
checker's comments.

The first failed run was a startup race in the scenario, now removed. Ze dials
from its transport address on FRR's first Hello, and the checker added that
address only when it started, so a dial could fail with
`bind: cannot assign requested address` and cost a backoff step (seen in a later
run: a failed dial at 18:53:24.679 with `delay=15s`, operational at
18:53:39.691). The lab now adds the address in ze's prestart
(`scenarioHostAddresses`). An earlier red attempt through a host build overlay
did not count: the overlay never reached the image, the run passed, and the
missing `session setup failed` line showed the mutation was absent. The sibling
scenarios `ldp-keepalive-expiry-frr` and `ldp-init-unknown-tlv-frr` still add the
address from their checkers and carry the same race: a journal row owned by
another spec, not this one.

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
- `internal/plugins/ddos/flowspec/exit_test.go`, `record_test.go`, `doctor.go`, `doctor_test.go`, `doctor_stopped_test.go` (`40e7226fca`), `doctor_stopped_record_test.go` (`c633ddf84d`)
- `test/plugin/ddos-flowspec-stopped-orphan-doctor.ci` (`c60b3fbbbb`)
- `internal/test/fixture/ddos_flowspec_orphan_cleared.go` and `register_ddos_flowspec_orphan_cleared.go` (`c60b3fbbbb`): a pure move of the clearing barrier out of the Linux-only `ddos_flowspec_exit_linux.go`, so it registers on every host; `ddos-flowspec-stopped-orphan-doctor.ci` does not use it
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
| Doctor check for runtime dependencies | Yes | The plugin runtime state key is judged by three checks in `internal/plugins/ddos/flowspec/doctor.go`, one row per record. (1) `ddos-flowspec-orphan-announcement` (`orphanRuntimeCheckDecl`), runtime phase, declared in the plugin's `sdk.Registration` `DoctorChecks` and answered from `OnDoctorCheck` (`answerOrphanRuntimeCheck`) while the plugin runs. (2) `ddos-flowspec-orphan-record` (`orphanRecordCheck`), post-config phase, registered with `diagnostic.RegisterDoctorCheck` in `register.go` `init`; it reports the record, with the withdrawal as fix, when the saved configuration doctor read does not name `ddos flowspec`, and stands down when it does (A-13). (3) `ddos-flowspec-stopped-record` (`stoppedRecordCheck`, `c633ddf84d`), runtime phase, registered the same way, so only the daemon route runs it: it reads the daemon's plugin snapshot, the store the daemon publishes and the saved tree the runtime phase is handed, and reports the record while the section is configured and no running ddos-flowspec process holds it. Codes in `internal/core/diagnostic/codes.go` |
| Doctor code per problem class | Yes | `doctor-ddos-flowspec-orphan-announcement`, from all three checks: a recorded rule no responder owns (AC-16). The post-config check also reports `doctor-storage-unavailable`, an existing code, when no store is open, and the runtime-phase check reports it when the daemon publishes no store while the plugin is configured or its state is not known yet. The `WARN` at start (AC-15) and the `WARN` on a failed state write (R-8) name it. LDP and L2TP add no problem class: they remove two |
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
- "The exit path withdraws" is two different problems. Where the engine still answers (`bye`, and, for a plugin running as its own process, a panic unwinding or a returned `Run`), the plugin withdraws. In process, `(*Plugin).Run` stops dispatch before the plugin's deferred exit, so only `bye` reaches the engine (A-15). Where it does not (daemon stop, a killed process), nothing the plugin does on the way out can reach a peer, so the rule is recorded while it is live and settled at the next start.
- A lock orders critical sections, and here the thing to order is what happens after the critical section. Holding a second lock across the publication orders it and makes every state change wait for the slowest reader. A ticket taken inside the critical section and waited for outside it orders the publication and holds nothing.
- One defect met on the way is owned here with its journal row (2026-11-11): the FlowSpec rule nothing remembers after a generation ends (AC-14 to AC-16). Four L2TP defects met on the way are recorded in the journal and owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` ("Rows considered and not owned" below).
- A runtime doctor check cannot be a registered function: the registry refuses it, because the function would read the daemon's copy of a forked plugin's state. The plugin answers it itself, so a dead plugin answers nothing (A-13).

### Findings of 2026-11-13 (defects found, behavior changed)

- A reload that removed the `ddos flowspec` section or its parent `ddos` block was refused at the plugin's config verify: the defaults for an absent section carry no action, and validation demanded one. The section-removed withdrawal of AC-12 was therefore unreachable through a real reload. Found by the first daemon-level run of `ddos-flowspec-parent-removed-withdraws.ci`, fixed in `3e347a86e6` (the plugin validates only a section the config holds), unit test `TestReloadWithoutTheSectionWithdrawsTheAnnouncement` (`ddos/flowspec/section_removal_test.go`).
- The orphan proof cannot assert a withdrawal on the wire after the daemon was killed. The restarted plugin's withdrawal reaches the engine, and the engine withholds it from a session that never advertised the rule (`reactor_api_batch.go` `splitOnAdvertised` and `logWithdrawWithheld`, RFC 4271 Section 4.3). `ddos-flowspec-orphan-doctor.ci` therefore asserts the withheld line, no FlowSpec UPDATE for the victim on the new session, and the cleared report. The wire half of the restart fix is carried by the unit test `TestRestartWithdrawsTheOrphanAndClearsTheReport`, and the wire half of removal by the section-removed `.ci`.
- `ldp: session operational` is logged at INFO from 2026-11-13 (`ldp/session.go` `ReadLoop`), in `95f99add59`, with `TestSessionOperationalLoggedAtInfo` (`ldp/session_log_test.go`).
- Lint: the scoped reactor lint was re-run on 2026-11-13. It reports no finding in `peer_run_backoff_test.go` or `reactor_listener_start_window_test.go`, the two files `2219d0d7b5` fixed. Two findings remain, in files this work did not touch: `peer_queue_gate_test.go` (goimports) and `default_next_hop_controls_test.go` (unparam).
- Journal rows written on 2026-11-12 and 2026-11-13 for defects walked into, each owned by another spec: "Rows considered and not owned" below.

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
| green-that-could-not-have-been-red | 2026-11-13 | `TestPrefixBackoffReset` asserts a reset it performs itself (`a0710bb9a9`) | A BGP reactor test, not a session end here: owned by `plan/release/fixes/spec-single-owner-lifecycle-2-bgp-session.md` |
| hook-existing-patterns-false-positive | 2026-11-12 | `bashTestDeletion` reads the word `rm` in a commit message as a delete target (`a0710bb9a9`) | A hook defect: owned by `plan/post-release/engineering/spec-hook-le-data-arguments.md` |
| retry-forgets-consumed-input | 2026-11-13 | L2TP `handleTick` drops the kernel setup events it collects (`a0710bb9a9`) | Kernel setup retry, not event order: owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` |
| concurrent-session-corruption | 2026-11-13 | guest `le` and daemons under `tmp/qemu/linux-<arch>/` shared between sessions (`d546500e1d`) | Test tooling: owned by `plan/post-release/engineering/spec-test-sandbox.md` |
| hook-existing-patterns-false-positive | 2026-11-13 | `goBuildText` and `textLossyPipe` refuse a read-only `ssh` with a quoted pattern (`827212f001`) | A hook defect: owned by `plan/post-release/engineering/spec-hook-le-data-arguments.md` |
| subscriber-waits-on-an-event-another-feature-produces | 2026-11-13 | ddos-flowspec never announces a non-critical detection with characterization off (`827212f001`) | The detection-to-announce path, not the plugin's exit: owned by `plan/release/validation/spec-doctor-coverage-14-system-plugins.md` |
| value-computed-once-never-refreshed | 2026-11-13 | ddos-detect grades severity once at trigger, so a growing attack never reaches the next response (`83c5971c24`) | Detection, not the plugin's exit: owned by `plan/release/fixes/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md` |
| session-up-before-its-subscriber | 2026-11-13 | the interop checkers of `ldp-keepalive-expiry-frr` and `ldp-init-unknown-tlv-frr` add ze's transport address after ze may already dial (`4ed8cc72b4`) | The sibling scenarios' setup race, which this spec removed from its own scenario only: owned by `plan/release/validation/spec-test-synchronisation-is-a-fence-never-a-clock.md` |
| lock-held-across-a-join | 2026-11-13 | LDP `startSessionForAdj` holds `sessionsMu` across the bounded TCP dial (`0b2afedc77`, a review note) | The session start, not its end: owned by `plan/release/validation/spec-doctor-coverage-13-routing-protocols.md` |
| exit-cleanup-runs-after-its-transport-closed | 2026-11-13 | An in-process ddos-flowspec that stops on its own while the daemon runs cannot withdraw on its way out: `(*Plugin).Run` stops dispatch before the plugin's deferred exit (review round three, A-15; `ada8f1c7a6`) | The fix is the engine withdrawing a plugin's announcements when its generation ends, or the SDK running exit cleanup before dispatch stops, for every plugin: owned by `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md`. Its generation-end cleanup today withdraws only routes a plugin installed (`withdrawPluginRoutes`, A-8), so its design must widen to update-route announcements |
| guard-added-to-one-half-of-a-pair | 2026-11-13 | `pkg/plugin/rpc/conn.go` `writeBatchWithDeadline` writes a batch whose context has ended, unlike `writeFrame`, so delivered events can be reported as failed (review round eight, note 2; `b8aa7e7739`) | Predates this work's SDK change, and is the plugin's delivery path, not its exit: owned by `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` |
| identity-default-hides-a-mapping | 2026-11-13 | The daemon's `show doctor` judges the store's configuration under the name `resolve.DefaultConfig` gives (`internal/component/doctor/doctor.go` `loadDoctorConfig`), never the file `ze start <file>` loaded, met writing `ddos-flowspec-stopped-orphan-doctor.ci` | Doctor's configuration source, not a session end: owned by `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` |
| wait-outlives-the-process-it-waits-on | 2026-11-13 | `(*Process).startExternal` waits up to 30 s for the connect-back of a forked plugin that has already exited, met writing `ddos-flowspec-stopped-orphan-doctor.ci` | The plugin's start, not its exit: owned by `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` |
| identity-default-hides-a-mapping | 2026-11-14 | The effects of the 2026-11-13 row: beside a daemon running the plugin with a saved file that lacks the section, `unconfiguredOrphanDiagnostic` says no running plugin can withdraw the rule, and its withdrawal ends a live mitigation (review round eleven, note 1) | Doctor's configuration source: owned by `plan/release/validation/spec-doctor-coverage-11-hub-engine.md` |
| guard-keyed-on-a-field-its-consumer-ignores | 2026-11-14 | A block running ddos-flowspec under another name cannot reach its record, and doctor cannot tell it from not running: the state-key grant and the doctor snapshot key on the process name (review round nine) | Plugin identity, not the plugin's exit: owned by `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md` |
| hook-existing-patterns-false-positive | 2026-11-13 | the test-weakening check counts an assertion moved into a same-file helper the unit calls as removed (`weakenedUnits`, `ada8f1c7a6`) | A gate defect met while committing `3a19b67b81`: owned by `plan/post-release/engineering/spec-weakening-check-judges-units-head-never-held.md` |
| callback-reenters-held-lock | 2026-11-13 | L2TP tunnel events emitted while a session-order ticket of the same tunnel is held (`0b2afedc77`, a review note) | Latent, the tunnel events, not the session events this spec orders: owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| LDP: `runSession` hands "reached operational" to `onDone` | Swap the two defers. Add a "was operational" field read through `State` | `Stop` is called from three places, so the swap fixes one of them. The callback already fires once on the right goroutine, so the fact needs no lock and no new session state |
| LDP: no SessionDown for a session that never published SessionUp | Keep publishing and make OSPF ignore a down with no up | The reader cannot tell the two apart from the payload, and a third-party reader would need the same guard. The writer knows |
| L2TP: a per-tunnel order ticket taken under `tunnelsMu` and waited for after it is released | (1) An emit-order lock taken before `tunnelsMu` is released and held across the publication. (2) A queue with one emitter goroutine. (3) Publish while holding `tunnelsMu`. (4) A guard at each reader. (5) One ticket sequence for the whole reactor | (1) is rejected: a path would wait for the lock while holding `tunnelsMu`, and the RADIUS accounting reader sends Accounting-Stop synchronously with a 10-second bound, so one slow RADIUS server would stop every state change in the reactor for 10 seconds. (2) is rejected: PPP waits on `Acknowledge` until publication finished, so the queue would have to carry the acknowledgement, and it is unbounded. (3) is rejected: readers do I/O. (4) is rejected: "unknown session" is the normal first event, `handleLCPDown` ends a lifetime of a session that continues, and there are five in-tree readers. (5) is rejected: it makes a slow reader delay every tunnel's events. The tunnel is enough because it leaves the maps in one place after draining, and a reused session id stays inside it |
| L2TP: the route observer is inside the ordered section | Leave it outside, as the journal row names only the bus | It has the same defect: an IP-up after the down leaves an injected route |
| DDoS: withdraw in the `bye` handler and on the way out, through one function | Only a deferred call after `Run` returns, on a fresh context (the design as approved) | After `Run` returns on a removal or a daemon stop the connection is closed or closing, so a fresh context alone reaches nobody. `bye` is the point where the engine waits for the plugin with the connection open. The deferred call stays for the ends where the engine still answers |
| LDP: an adjacency expiry drops the setup backoff, so a neighbor that NAKs and is rediscovered inside 15 seconds is dialed at once (AC-5). Settled without asking, on its evidence; the owner may overturn it | Keep the backoff across the loss of the adjacency | RFC 5036 Section 2.5.3 does not say what becomes of the backoff when the adjacency is lost. FRR 10.7.1 forgets it: `ldpd/adjacency.c` `adj_del` calls `nbr_del` once the neighbor has no adjacency left, and `nbr_new` allocates a fresh neighbor with `calloc`. Before this work the backoff survived only because the buggy session end wrote it back |
| L2TP: the route observer hears a withdrawal only for a removed session that had reached established, on every path (`teardown.go` `notifySessionDowns` reads `sessionDown.established`). Settled without asking; the owner may overturn it | Tell it of every removed session, as the operator and PPP paths did; tell it nothing on the reap and tie-breaker paths, as before | Only an established session ran PPP and can hold a route. The observer keys its records by session id alone, so a withdrawal for a session that never came up could delete another tunnel's record, and an established session of a reaped or tie-breaker-discarded tunnel left its route injected. Tests: `TestRetentionReapWithdrawsItsEstablishedSessionRoute`, `TestTieBreakerLossWithdrawsItsEstablishedSessionRoute`, `TestSessionRemovedBeforeEstablishedWithdrawsNoRoute` (`l2tp/session_route_withdrawal_test.go`, `bae3903dbb`) |
| L2TP: no SessionUp is published for a session already gone from its tunnel (`reactor_kernel.go` `handleSessionUp` looks the session up under `tunnelsMu` and returns when it is gone). Settled without asking; the owner may overturn it | Publish it from the PPP event alone, as before | A SessionUp for a session that already left its tunnel would reach readers after its SessionDown and re-create what the down removed: the defect this spec fixes |
| DDoS: record the live rule and withdraw it at the next start | Engine-side tracking of update-route announcements per plugin. Re-adopt the rule at start | Engine tracking is the general fix and belongs to the plugin-process and BGP engine work, not to a local fix. Re-adopting needs the probe, the clock and the rate-limit window persisted, and the sibling's precedent is that protection after a restart comes from fresh detection |

## Known Limitations

- This spec does not cure the reactor stall of R-1: the reactor goroutine publishes synchronously, and a slow reader still stops packet processing for every tunnel while that goroutine is inside the publication or waiting its turn for the same tunnel. That is an open defect with a journal row (`plan/journal/slow-reader-stalls-a-shared-goroutine.md`, 2026-11-11), owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md`.
- On a shared segment, the end of one operational LDP session costs the link out while another neighbor's session is up. RFC 5443 Section 3 leaves that to policy, and this spec does not change it.
- `teardownSessionByID` ends the first session it finds with a given id on any tunnel. Session ids are unique per tunnel only, so an operator clearing by id alone can hit another tunnel's session. Not changed here: an open defect with a journal row (`plan/journal/key-omits-a-fact-the-builder-uses.md`, 2026-11-11), owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md`.
- Update-route announcements are not tracked per plugin by the engine. Any other plugin that announces through update-route and dies has the same exposure. That is engine work for the plugin-process child or a spec of its own, raised to the parent.
- At a daemon stop no withdrawal is sent. The BGP sessions end with the daemon.
- An in-process ddos-flowspec that ends on its own (a panic, a failed run) while the daemon runs cannot withdraw on its way out (A-15). The rule stays at the peers until the operator's `ze request plugin restart ddos-flowspec` starts the plugin, which withdraws it from its record; meanwhile the daemon's `show doctor` reports the recorded rule with that command (`stoppedRecordCheck`). An open defect with a journal row (`plan/journal/exit-cleanup-runs-after-its-transport-closed.md`, 2026-11-13), owned by `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md`.
- AC-16's report of the record with the plugin's process ended holds in the daemon for a configured plugin that is not running, and on both routes for a section the saved configuration lacks. Two limits remain. (a) With no daemon answering and the section configured, `ze doctor` does not report the record: nothing there can tell a live mitigation from a stopped plugin, so the post-config check stands down and the run says through `doctor-runtime-not-checked` that the daemon's checks did not run. (b) Doctor judges the saved file, not the configuration the daemon runs, so a saved file without the section beside a daemon running a live mitigation gets the not-configured report and its withdrawal. Journal rows in `plan/journal/identity-default-hides-a-mapping.md` (2026-11-13, 2026-11-14), owned by `plan/release/validation/spec-doctor-coverage-11-hub-engine.md`.
- A plugin block that runs ddos-flowspec under another name cannot reach the record, and doctor cannot tell it from a plugin not running: the report then names the withdrawal in its message only and offers `ze show config running` as its fix. Journal row `plan/journal/guard-keyed-on-a-field-its-consumer-ignores.md` (2026-11-14), owned by `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md`.

## Work Not Done

<!-- Written before closure, so /ze-close extends this table instead of
     appending a second one from plan/templates/TEMPLATE-CLOSURE.md. -->
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The removed-section half of AC-16 (with `ddos flowspec` removed, the operator withdraws the rule with the command the report prints, then removes the record through the running daemon, and the report clears) moved to `plan/release/fixes/spec-data-store-maintenance-through-the-daemon.md`, which builds the daemon route for `ze data rm` and rewrites the report's removed-section text. | The store's write verbs (`rm`, `write`, `import`, `repair`) are refused while a daemon owns the store, so the record cannot be removed through the running daemon today. Owner decision "a + c", 2026-11-12 | `plan/release/fixes/spec-data-store-maintenance-through-the-daemon.md` |

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

---

## Implementation Summary

### What Was Implemented
- LDP session end (`4f1e24bf66`): `runSession` hands `onDone` whether its on-operational callback ran, read when the deferred call runs. The session-end callback in `startSessionForAdj` deletes the backoff entry and publishes one SessionDown for an operational session, records a setup failure and publishes nothing for an attempt that never became operational, and records nothing for a session its adjacency expiry stopped (`Session.endedByAdjacencyExpiry`). The RFC 5036 Section 2.5.3 sentences are quoted one per quotation. `95f99add59` logs `ldp: session operational` at INFO.
- L2TP event order (`bae3903dbb`): a per-tunnel order ticket (`L2TPTunnel.issueNoticeTicket`), issued under `tunnelsMu` and delivered after it is released, orders every route-observer call and session event of one tunnel. `drainPendingTeardowns` issues it for each batch of downs; `deliverSessionDowns` delivers batches in ticket order; `handleSessionIPAssigned`, `handleSessionUp` and `handleLCPDown` take one each. No lock is held across a publication. The reader contract is on the `l2tp/events` package doc.
- DDoS FlowSpec exit (`bec9096156`, `85507073e8`, `3e347a86e6`, `3a19b67b81`, `c60a2b6edc`): one `stopResponder`, called from the `OnBye` handler and from the deferred exit of `runEngine`, withdraws under its own bounded context (`exitWithdrawBudget`). The live announcement is recorded under `meta/ddos-flowspec/announcement` and removed on every withdrawal that reached the engine, before the live flag is cleared. `OnStarted` withdraws a recorded rule before the plugin subscribes to detections. Two doctor checks report a record no responder owns: the runtime check the plugin answers and the post-config check that reads the store while the section is absent. A reload that removes the section now passes verify.
  The deferred exit reaches the engine only in a plugin running as its own process (A-15).
  Review rounds three to seven (`15d5cdd19b`, `e7e963b4e1`, `bd27660e0d`, `944149988c`): the plugin knows whether its live rule is recorded, so an exit line never claims a record that may not exist, and an unrecorded rule it cannot withdraw is a WARN carrying the withdrawal to send by hand. The way the plugin ended is one of removed, stopped or failed (`exitKind` in `register.go`), and an unset kind is refused at `stopResponder`'s entry on every path. A plugin running as its own process recognises the engine hanging up at a daemon stop. One builder renders the manual withdrawal command.
- Plugin SDK (`b6d1739194`, `fd6fedfc62`, `pkg/plugin/rpc/conn.go`): a failed write under a deadline was reported as a timeout whatever failed. It now keeps its cause, and one helper makes that decision for every write path, the batch write that carries every event delivery to a plugin included.
- RFC proof records (`8f45f32b93`, `4e4709af9d`): the 27 records and 2 audit verdicts that `4f1e24bf66`, `95f99add59` and `bae3903dbb` staled were re-taken (Mistake Log).
- Proofs: five daemon-level `.ci` tests (`0617ddff57`, `525577107c`, `af1d0bc452`) and the FRR interop scenario `ldp-backoff-clear-frr` with its recording and replay test (`e299b1e031`).
- Kept, not deleted: `plan/handover/38-single-owner-lifecycle-unrun-proofs.patch`. The handover asked for its removal in the commit that lands the last proof, but it still holds the only copy of child 2's parked test `internal/component/bgp/reactor/peer_initial_sync_join_test.go.pending`, which is uncommitted. It belongs to child 2 (`plan/release/fixes/spec-single-owner-lifecycle-2-bgp-session.md`) from this closure on.

### Bugs Found/Fixed
- A reload removing `ddos flowspec` or its `ddos` parent was refused at verify, so AC-12's withdrawal was unreachable through a real reload. Fixed in `3e347a86e6`; `TestReloadWithoutTheSectionWithdrawsTheAnnouncement` and `ddos-flowspec-parent-removed-withdraws.ci`.
- The FlowSpec plugin acted on a detection before its start-time withdrawal had run, while state calls were still refused. Fixed in `3a19b67b81`; `TestDetectionDuringTheHandshakeIsNotActedOn`.
- The runtime doctor check could see the record with no live owner for the instant between clearing the flag and removing the record. Fixed in `3a19b67b81`; `TestWithdrawForgetsTheRecordBeforeClearingTheLiveFlag`.
- A new announcement overwrote the record of an earlier rule never withdrawn. Fixed in `85507073e8`; `TestAnnouncementWithdrawsAnEarlierRecordedRuleFirst`, `TestAnnouncementWaitsWhileAnEarlierRecordedRuleCannotBeWithdrawn`.
- The orphan report and the guide misstated what happened (round one: "did not reach the BGP engine"; round two: a daemon stop described as a race, and a report naming one cause of two). Fixed in `3a19b67b81` and `c60a2b6edc`; `TestRuntimeOrphanReportNamesBothCausesOfAFailedWithdrawal`.
- The exit lines claimed a recorded rule when the record write may have failed (round four). Fixed in `e7e963b4e1`, with tests that drive the run itself (`exit_record_test.go`, `exit_run_test.go`); a failed record write over a record of the same rule is an INFO line (`bd27660e0d`, `announce_record_level_test.go`), and the record-write WARN carries the withdrawal by hand (`944149988c`).
- A ddos-flowspec running as its own process did not recognise the engine hanging up at a daemon stop and logged a failed run. Found during round four's fixes, fixed in `e7e963b4e1`; the closed-connection match has a table test (`transport_closed_test.go`, `bd27660e0d`, its rows in the shapes the code produces since `944149988c`).
- An unset exit kind was accepted on some paths (round five). Refused at the entry of `stopResponder` on every path in `bd27660e0d` (`exit_kind_test.go`). The zero value stays `exitUnspecified`, as `docs/contributing/ze-go-style.md` asks, where the reviewer had suggested a valid zero.
- Plugin SDK: a write that failed under a deadline was reported as a timeout whatever failed, so the plugin's exit line could not tell a closed connection from a slow one (round six). Fixed in `b6d1739194` (`conn_write_error_test.go`); the batch write path made the same decision the old way (round seven), fixed in `fd6fedfc62` by one helper for every write path.
- The L2TP route observer was told of withdrawals inconsistently across the teardown paths. Fixed in `bae3903dbb` (Key Design Decisions); `TestRetentionReapWithdrawsItsEstablishedSessionRoute`, `TestTieBreakerLossWithdrawsItsEstablishedSessionRoute`, `TestSessionRemovedBeforeEstablishedWithdrawsNoRoute`.

### Documentation Updates
- `docs/architecture/ldp/mpls-ldp.md` and `ai/digests/mpls-signaling.md` (`4f1e24bf66`, `e299b1e031`): the backoff decision and the panic-guard row.
- `docs/guide/mpls.md` (`95f99add59`): the INFO line on session up.
- `docs/architecture/l2tp/subscriber-session-model.md`, `ai/digests/subscriber.md`, `docs/architecture/l2tp/bng-1-radius-attributes.md`, `docs/research/l2tpv2-ze-integration.md` (`bae3903dbb`): event order and the reader contract.
- `docs/guide/ddos-mitigation.md` (`bec9096156`, `85507073e8`, `3a19b67b81`, `c60a2b6edc`, `15d5cdd19b`, `e7e963b4e1`, `bd27660e0d`, `944149988c`, `40e7226fca`, `c633ddf84d`, `0c9fecde5c`): the FlowSpec exit rows, the ddos local parent-block row, the orphan report. The "plugin stops on its own" row states that the plugin declares no failure policy, that `ze show doctor` reports it as stopped (`doctor-plugin-stopped`) and reports the recorded rule with its victim under `doctor-ddos-flowspec-orphan-announcement`, that the fix of both is `ze request plugin restart ddos-flowspec`, and that this start withdraws the recorded rule. The guide's table of doctor answers per state names which check judges the record in each case, the local run with no daemon included.
- `docs/architecture/doctor-and-health-checks.md` (`c633ddf84d`): the runtime phase is handed the saved configuration the post-config phase judged (Deviations, approved departure).
- `internal/core/diagnostic/codes.go` (`15d5cdd19b`): the orphan code's description names every cause.
- `docs/functional-tests.md` (`0617ddff57`): the `ldp` suite's live-neighbor fixture. `docs/architecture/testing/interop.md` (`e299b1e031`): the scenario's inventory row.
- `./le doc check all`: owed command, run by the main thread before commit A.

### Deviations from Plan
- AC-8's RADIUS accounting test is a reader-level proof, not a reactor-driven one (A-14).
- A-13 broken: a second, registered post-config check covers the record while the plugin's process is gone.
- AC-16's removed-section half moved to `plan/release/fixes/spec-data-store-maintenance-through-the-daemon.md` (Work Not Done, owner decision).
- AC-19 and AC-20 moved to `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` before implementation.
- A-15 broken (review round three): the deferred exit withdraws only in a plugin running as its own process. In process the rule stays recorded until the operator's `ze request plugin restart ddos-flowspec` starts the plugin: it declares no failure policy, so the daemon does not restart it. While the section is configured, the daemon's `show doctor` reports the recorded rule with that command through the runtime-phase `stoppedRecordCheck` (`c633ddf84d`), beside `doctor-plugin-stopped`; once the saved configuration lacks the section, `orphanRecordCheck` reports it. A prompt in-process withdrawal is the journal row `exit-cleanup-runs-after-its-transport-closed.md`, owned by `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md`, not built here.
- AC-13's survivor names one INFO line saying the BGP sessions end with the daemon. The code logs, at a closed transport with the rule recorded, an INFO line saying the rule stays recorded because the plugin's connection to the engine is closed, with the plugin restart under `fix`; with no confirmed record it logs a WARN carrying the withdrawal by hand (`e7e963b4e1`, the guide's daemon-stop row). The guide states that the BGP sessions end with the daemon.
- The plugin SDK was changed (`pkg/plugin/rpc/conn.go`, `b6d1739194`, `fd6fedfc62`), which Documentation Update row 8 had answered No: the change keeps the cause of a failed write and adds no callback, field or wire byte, so no SDK page changes.
- The RFC 5036 and RFC 2661 proof records that the code phases staled were re-taken at closure (`8f45f32b93`, `4e4709af9d`) instead of by the phase that staled them (Mistake Log).
- Approved departure from the design documentation: the doctor runtime phase is now handed the saved configuration the post-config phase judged, so `stoppedRecordCheck` agrees with `orphanRecordCheck` on whether the section is configured (`c633ddf84d`). Thomas approved it ("A, amend the page"), and `docs/architecture/doctor-and-health-checks.md` states the rule and is its record.
- The `bye` withdrawal was added beside the approved deferred withdrawal (Key Design Decisions), and the FlowSpec plugin subscribes to detections only after its start cleanup (`3a19b67b81`, stated in the guide).

## Risks & observations

| ID | Surviving risk | Where it stands |
|----|----------------|-----------------|
| R-1 | The reactor goroutine still publishes synchronously, so a slow reader of its own publication stalls packet processing for every tunnel | Open defect, journal `slow-reader-stalls-a-shared-goroutine.md` (2026-11-11), owned by `plan/release/fixes/spec-l2tp-session-identity-and-publish-stall.md` |
| R-2 | A future in-process reader that publishes for its own tunnel from inside its handler waits behind itself | The contract is on the `l2tp/events` package doc and on `deliverSessionDowns`; `TestSessionEventReadersDoNotCallBackIntoTheReactor` and the four `TestSessionEventReaderDoesNotCallBackIntoTheReactor` cover today's readers only |
| R-6 | The `bye` withdrawal outlasts `pluginRemovalGrace` | The exit context is `exitWithdrawBudget` (400 ms), below the 500 ms grace; a withdrawal that does not finish leaves the record, which `orphanRecordCheck` reports once the section is removed, and which a later start of the plugin withdraws |
| A-15 | An in-process plugin that ends on its own while the daemon runs leaves its rule at the peers | Open defect, journal `exit-cleanup-runs-after-its-transport-closed.md` (2026-11-13), owned by `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md`; until it is fixed the operator's plugin restart withdraws the recorded rule (Known Limitations) |
| R-8 | A failed state write leaves a live rule unrecorded | The announcement still goes out and the WARN names the doctor code. Two failed writes in a row can leave a live rule with no record, stated in the guide |
| R-9 | The start-time withdrawal reaches no peer when none is established | Harmless: the engine withholds it from a session that never advertised the rule (`splitOnAdvertised`), as `ddos-flowspec-orphan-doctor.ci` asserts |

R-3, R-4, R-5, R-7 and R-10 are closed by the tests and runs named in their rows above.

Review notes left as they are, with the reason (round seven): a broken pipe or a reset is not read as a closed connection by the plugin's exit line, and the generic warning it gets instead is true in that case; a narrow race in the SDK watchdog signal; one row of the closed-connection table that only tests can reach. Round eight's two notes are in the Review Gate.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-13: a declared runtime doctor check of a plugin whose process ended is reported by `ze doctor` | `collectDoctorChecksFrom` asks only the processes `Server.DoctorCheckPlugins` lists, and a removed plugin's process is removed first (`takeAndStopProcess` calls `RemoveProcess`), so its check vanishes with no line | Read in phase 3 (2026-11-12) | A registered post-config check, `orphanRecordCheck`, reads the record through the store while the section is absent (`bec9096156`); today's orphan proof runs the runtime half |
| assumption | A-14: the `authradius` accounting test of AC-8 can drive the reactor and go red against the unfixed order | The reactor is unexported and offers `authradius` no seam; no production seam was added for a test | Read in phase 2 (2026-11-12) | The test stays reader-level; the reactor's order tests and `session-clear-leaves-no-subscriber.ci` carry AC-8's red. Today's `.ci` proof (`525577107c`) observed the accounting outcome through the daemon: one Start, one Stop |
| approach | The section-removed withdrawal was proven by a unit test that called `replaceResponder` directly | The reload's config verify refused the removal before apply could run, so the path was unreachable through a real reload | The first daemon-level run of `ddos-flowspec-parent-removed-withdraws.ci`, red on unmodified code | Fixed in `3e347a86e6`, with a unit test that drives verify and apply through the SDK harness |
| approach | The first interop red was attempted through a host build overlay | The ze image builds from the tree, so the overlay never reached it; the run passed and the absence of the `session setup failed` line showed the mutation was not there | Reading the run's log against the expected red | The red was taken with the mutation in the working tree for the run, restored and checksum-verified in the same command (`e299b1e031`) |
| approach | The interop scenario's checker added ze's transport address when it started | ze dials on FRR's first Hello, so a dial could fail with `bind: cannot assign requested address` and cost a backoff step | A failed run, then a later run showing the failed dial at 18:53:24.679 | The lab adds the address in ze's prestart (`scenarioHostAddresses`); the siblings' row is owned elsewhere |
| assumption | A-15: the deferred exit can reach the engine when an in-process plugin's `Run` returns or panics while the daemon runs | `(*Plugin).Run` defers `p.bridge.CloseCallbacks()`, which stops dispatch first, so the in-process withdrawal is answered `ErrBridgeClosed` on every end | Review round three | Texts corrected (guide, `stopResponder` comment and log line, `codes.go`); AC-13 recorded as proven for a plugin running as its own process and vacuous in process; the prompt in-process withdrawal is the journal row `exit-cleanup-runs-after-its-transport-closed.md` (`ada8f1c7a6`), owned by `spec-single-owner-lifecycle-1-plugin-process` |
| assumption | The closure text first written for A-15 (AC-13 audit, Goal Validation, Deviations) said the daemon starts the plugin again and that `ze doctor` reports the record meanwhile | ddos-flowspec declares no failure policy, so the daemon does not restart it; the next start is the operator's `ze request plugin restart ddos-flowspec`. At the time, while the section was configured, doctor reported `doctor-plugin-stopped` with that command and not the record; the post-config `orphanRecordCheck` reported the record only once the section was removed | Reading the guide's corrected row and `doctor.go` `checkOrphanRecord` (it deferred to the runtime check while the section was configured) | Every sentence corrected in this spec then. That gap was itself AC-16 unmet, and it is closed since `c633ddf84d`: the daemon reports the record while the section is configured and the plugin is not running (the AC-16 entries below) |
| approach | The first version of the SDK write test (`conn_write_error_test.go`, `b6d1739194`) was reported red against the unfixed code by the agent that wrote it | It passed against the unfixed code: on a pipe the failure comes when the write deadline is set, before the write, so the test never reached the misreported path | The main thread ran it against the unfixed code itself and saw it pass | Rewritten with a writer that accepts the deadline and fails the write; its red was then observed by the main thread before the fix landed |
| approach | Round six's fix of the write-error cause changed the single-message write path only | The batch write path, which carries every event delivery to a plugin, made the same decision the old way | Review round seven | `fd6fedfc62`: one helper decides for every write path |
| approach | Messages and comments corrected in one round were not re-read as new text | Rounds three to six each found a fresh misstatement in lines the previous round had rewritten (the exit lines, the orphan report, the guide's rows) | Review rounds four, five and six | Each corrected line was checked against the code that produces its condition (`e7e963b4e1`, `bd27660e0d`, `944149988c`) |
| approach | `4f1e24bf66`, `95f99add59` and `bae3903dbb` changed functions that 27 RFC proof records and 2 audit verdicts are tied to, and did not re-take them: a record stale at closure is a defect of the implementation phase (`ai/rules/planning.md`) | `./le rfc check` listed 29 stale items for RFC 5036 and RFC 2661, all from those commits | The main thread's `./le rfc check` before the closure commits | Re-taken 2026-11-13: 13 RFC 5036 and 4 RFC 2661 records on the host and both verdicts re-judged by an independent auditor, still enforced (`8f45f32b93`); the 10 Linux-only RFC 2661 records in Ze's QEMU guest (`4e4709af9d`). `./le rfc check` names neither document now |
| approach | The exit withdrawal for a run that ends without `bye` was implemented with no test that reached it | Removing it reddened nothing | Review round one | `TestRunEndingWithoutByeWithdrawsTheLiveAnnouncement` (`3a19b67b81`), red with the deferred withdrawal removed |
| approach | The plugin subscribed to detections at configure time | A detection before the start-time withdrawal was acted on while state calls were refused | Review round one | Subscription moved after the start cleanup (`3a19b67b81`) |
| approach | The guide and the exit comment called the daemon-stop withdrawal a race | An in-process plugin never withdraws at a daemon stop: the daemon refuses its calls before its run ends | Review round two | Corrected in `c60a2b6edc` |
| approach | AC-16 was first recorded as done on the case with the `ddos flowspec` section removed alone | The case with the section configured and the plugin not running was unmet: doctor did not report the record there | Closure | The configured-and-stopped case was built (`40e7226fca`), reworked (`c633ddf84d`, `0c9fecde5c`) and given its own unit and daemon tests (Implementation Audit, AC-16) |
| approach | The first fix of the configured-and-stopped case (`40e7226fca`) read live daemon state, the plugin snapshot, from the post-config phase, which also runs with no daemon | Live daemon state exists only in the daemon, so a check over it takes the runtime phase (`docs/architecture/doctor-and-health-checks.md`) | Review round nine | Reworked in `c633ddf84d`: the runtime-phase `stoppedRecordCheck`, with the runtime phase handed the saved configuration (approved departure, Deviations); `0c9fecde5c` then made it one row per record |
| assumption | The first draft of the daemon-level test for AC-16 assumed ddos-flowspec can run as its own process, and that after `ze init` the daemon's doctor judges the file the daemon was started with | Both false: the plugin cannot run as its own process, and the daemon's doctor judges the store's configuration under its default name, never the started file | Running the draft | The draft was corrected before it landed (`c60b3fbbbb`). The doctor's choice of configuration is the journal row `identity-default-hides-a-mapping.md` (2026-11-13), and the wait for the connect-back of an exited plugin process met on the way is `wait-outlives-the-process-it-waits-on.md` (2026-11-13) ("Rows considered and not owned") |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| LDP: an operational session clears the backoff and publishes SessionDown once; one that never reached it records a setup failure and publishes nothing | Done | `internal/plugins/ldp/register.go` `startSessionForAdj` session-end callback, `runSession`; `session.go` `endedByAdjacencyExpiry` | `4f1e24bf66` |
| L2TP: one session's events and route notifications reach readers in state-change order | Done | `internal/component/l2tp/session.go` `drainPendingTeardowns`, `issueNoticeTicket`; `teardown.go` `deliverSessionDowns`, `notifySessionDowns`; `reactor_kernel.go` `handleSessionIPAssigned`, `handleSessionUp`, `handleLCPDown` | `bae3903dbb` |
| DDoS: each end that can reach the engine withdraws; a rule not withdrawn is recorded, withdrawn at next start, reported by doctor | Done | `internal/plugins/ddos/flowspec/register.go` `stopResponder`, `exitKind`, `runEngine` (`OnBye`, `OnStarted`, the deferred exit); `responder.go` `withdrawThrough`; `doctor.go`; `pkg/plugin/rpc/conn.go` (the write-error cause the exit line reads) | `bec9096156`, `85507073e8`, `3e347a86e6`, `3a19b67b81`, `c60a2b6edc`, `15d5cdd19b`, `e7e963b4e1`, `bd27660e0d`, `b6d1739194`, `944149988c`, `fd6fedfc62`. In process, a run that ends on its own reaches no engine (A-15). AC-16's removed-section half is in Work Not Done |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSessionEndAfterOperationalClearsBackoff` (`4f1e24bf66`); `ldp-session-end-clears-backoff.ci` (`0617ddff57`); interop `ldp-backoff-clear-frr` (`e299b1e031`, redial 110 ms after FRR's clear) | Red observed at daemon level and in the interop lab |
| AC-2 | Done | `TestSessionEndAfterOperationalPublishesOneSessionDown` (`4f1e24bf66`); `ldp-setup-failure-publishes-no-session-down.ci` (`0617ddff57`), whose observer history for the operational session is `[session-up session-down]` | |
| AC-3 | Done | `TestSessionEndBeforeOperationalRecordsSetupFailure`, `TestRunSessionReportsNotOperationalToOnDone` (`4f1e24bf66`); `ldp-setup-failure-publishes-no-session-down.ci` (NAK'd Initialization, the daemon logs the 15 s delay) | A refused TCP connect keeps its `recordSetupFailure` call on the dial error and the RFC-tagged `TestRFC5036SetupRetryRefusedInsideDelay`, untouched. A KeepAlive send failure stops the session before the on-operational callback runs, so it takes the same failure arm: shown by reading `startSessionForAdj`, no test of its own |
| AC-4 | Done (bus) | `TestSessionEndBeforeOperationalPublishesNoSessionDown` (`4f1e24bf66`); `ldp-setup-failure-publishes-no-session-down.ci`: a subscribed observer plugin receives no `(ldp, session-down)` for the rejected attempt | Proven: the bus carries no SessionDown for an attempt that published no SessionUp. Not proven by any test: that an OSPF LDP-sync interface keeps its state and its stuck-timer deadline. That half follows by reading, because `ospf/ldp_sync.go` `handleSessionEvent` changes state only on an `(ldp, session-*)` event and none is published. `ospf-ldp-sync-restore.ci` was re-run green, which proves OSPF's reader unchanged, not this AC |
| AC-5 | Done | `TestAdjacencyExpiryLeavesNoBackoffEntry` (`adjacency_expiry_test.go`, `4f1e24bf66`), for a session that never reached operational | An operational session ended by expiry takes the `operational` arm, which deletes the entry: AC-1's test |
| AC-6 | Done | `docs/architecture/ldp/mpls-ldp.md` (`4f1e24bf66`, `e299b1e031`) | `./le doc check all` owed |
| AC-7 | Done | `TestSessionIPAssignedIsPublishedBeforeTheTeardownThatFollowsIt`, `TestSessionUpIsPublishedBeforeTheTeardownThatFollowsIt`, `TestRouteObserverSeesIPUpBeforeTheDownThatFollowsIt`, `TestTunnelTeardownDownsFollowEarlierEventsOfItsSessions` (`bae3903dbb`) | In-process order is proven by these tests. Forked readers get the same order because `process.Deliver` queues per process in call order (A-4, by reading). `session-clear-leaves-no-subscriber.ci` shows a positive outcome only |
| AC-8 | Done (bridge, accounting, route observer) | `TestTeardownDuringIPAssignedLeavesNoSubscriberEntry`, `TestRouteObserverSeesIPUpBeforeTheDownThatFollowsIt` (`bae3903dbb`); `TestTeardownDuringIPAssignedLeavesNoAccountingSession` (reader-level, A-14); `session-clear-leaves-no-subscriber.ci` (`525577107c`): one Start, one Stop, empty session and subscriber lists | Not observed by any test after a raced teardown: shaper and CoS state. Their `onSessionDown` remove their entry (`shaper.go`, `cos/handler.go`, by reading), and the order tests prove the down reaches every reader after the up |
| AC-9 | Done (accounting, subscriber bridge) | `session-clear-leaves-no-subscriber.ci` (`525577107c`): one Accounting-Stop for one Start, subscriber list empty; red with the session-down publication disabled | Not observed: pool, shaper and CoS releasing what they held. One SessionDown per removed session follows by reading (`removeSession` queues one down, `notifySessionDowns` is the single emitter of queued downs). `TestSessionEventReaderDoesNotCallBackIntoTheReactor` in `authradius`, `shaper`, `cos` and `pool` proves each reader runs without calling back, not what it releases |
| AC-10 | Done | `TestIPAssignedAcknowledgesAfterReadersReturn` (`bae3903dbb`) | |
| AC-11 | Done | `TestByeWithdrawsTheLiveAnnouncement`, `TestByeWithNothingAnnouncedSendsNothing` (`bec9096156`, strengthened in `3a19b67b81`: the record is removed and the status reports nothing announced) | Proven on the RPC path over a real `sdk.Plugin` pair. In `ddos-flowspec-parent-removed-withdraws.ci` the apply withdraws first, the AC-11 survivor, so no daemon-level test shows the `bye` handler's own withdrawal reaching a peer |
| AC-12 | Done | `ddos-flowspec-parent-removed-withdraws.ci` (`af1d0bc452`, after the `3e347a86e6` fix): `withdrawn=060120CB007109` on connection 1 and the section-removed log line; `TestReloadWithoutTheSectionWithdrawsTheAnnouncement` | Red observed with `withdrawForConfig`'s active check inverted |
| AC-13 | Done (own process); vacuous in process | `TestStopResponderWithdrawsUnderItsOwnContext`, `TestStopResponderOnAClosedTransportLogsOnce` (`bec9096156`); `TestRunEndingWithoutByeWithdrawsTheLiveAnnouncement` (`exit_signal_test.go`, `3a19b67b81`), for a plugin running as its own process | In the default in-process placement the AC's condition ("while the engine still answers") never holds at the deferred exit: `pkg/plugin/sdk/sdk.go` `(*Plugin).Run` defers `p.bridge.CloseCallbacks()`, which stops dispatch before `runEngine`'s deferred `stopResponder` runs, so the withdrawal is answered `ErrBridgeClosed` on every end (review round three, A-15). There the rule stays recorded, and a start of the plugin withdraws it (`withdrawRecordedAnnouncement`, AC-15). The plugin declares no failure policy, so the daemon does not start it again: the operator runs `ze request plugin restart ddos-flowspec`, which `ze doctor` prints under `doctor-plugin-stopped` while the section is configured. Doctor does not report the record itself then; it reports it through `orphanRecordCheck` once the section is removed. The exit lines name whether the rule is recorded (`e7e963b4e1`, `exit_record_test.go`, `exit_run_test.go`), the end is one of removed, stopped or failed with an unset kind refused (`bd27660e0d`, `exit_kind_test.go`), and a closed connection is recognised from the cause the SDK now keeps (`transport_closed_test.go`, `b6d1739194`, `fd6fedfc62`). A prompt withdrawal when an in-process plugin stops on its own while the daemon runs is not built here: the journal row `exit-cleanup-runs-after-its-transport-closed.md`, owned by `plan/release/fixes/spec-single-owner-lifecycle-1-plugin-process.md`. The panic half follows by reading for the own-process case; no test drives a panic |
| AC-14 | Done | `TestAnnounceRecordsTheRule`, `TestWithdrawRemovesTheRecord` (`bec9096156`); `TestWithdrawForgetsTheRecordBeforeClearingTheLiveFlag` (`3a19b67b81`); `TestAnnouncementWithdrawsAnEarlierRecordedRuleFirst` (`85507073e8`) | |
| AC-15 | Done | `TestStartWithdrawsTheRecordedAnnouncement` (`bec9096156`); `TestDetectionDuringTheHandshakeIsNotActedOn` (`3a19b67b81`); `ddos-flowspec-orphan-doctor.ci` (`af1d0bc452`): the withheld withdrawal and the cleared report | |
| AC-16 | Done, with two limits | `TestOrphanCheckReportsARecordWithNoLiveResponder`, `TestOrphanCheckIsQuietForTheLiveAnnouncement`, `TestOrphanRecordCheckReportsARecordWithTheSectionAbsent` (`bec9096156`); `TestRestartWithdrawsTheOrphanAndClearsTheReport` (`85507073e8`); `ddos-flowspec-orphan-doctor.ci` (`af1d0bc452`, green at its promoted path 2026-11-13); the units of `doctor_stopped_record_test.go` and `doctor_stopped_test.go` and `TestRuntimeChecksReceiveTheConfigurationDoctorRead` (`doctor/doctor_test.go`), all green, each MUTATION note observed red (`c633ddf84d`, `0c9fecde5c`); `ddos-flowspec-stopped-orphan-doctor.ci` (`c60b3fbbbb`, reworked `c633ddf84d`; its row in Functional Tests) | Three checks judge the record, one row per record: the running plugin's runtime answer; the post-config `orphanRecordCheck`, which reports with the withdrawal as fix when the saved configuration does not name `ddos flowspec` and stands down when it does; the daemon-only runtime-phase `stoppedRecordCheck`, which reports a configured plugin that is not running with the victim, why, and `ze request plugin restart ddos-flowspec` (and that ze refuses the restart while a cleanup is owed), and says what it cannot tell when the daemon holds no ddos-flowspec process (fix `ze show config running`, the withdrawal named in the message only), when no saved tree was read, or before plugin startup (fix `ze show doctor`). The AC's sentence "With the plugin's process ended, `ze doctor` still reports the record" is met in the daemon for a configured plugin that is not running, and on both routes for an unconfigured section. Limits (Known Limitations): (a) with no daemon answering and the section configured, doctor does not report the record; (b) doctor judges the saved file, so a saved file without the section beside a daemon running a live mitigation gets the not-configured report and its withdrawal (`identity-default-hides-a-mapping.md`, owned by doctor-coverage-11). The removed-section half is in Work Not Done, owned by `plan/release/fixes/spec-data-store-maintenance-through-the-daemon.md` |
| AC-17 | Done | `docs/guide/ddos-mitigation.md` (`bec9096156`, `85507073e8`, `3a19b67b81`, `c60a2b6edc`) | `./le doc check all` owed |
| AC-18 | Done | `TestSlowReaderDelaysNoStateChange` (`bae3903dbb`) | |
| AC-21 | Done (bus) | `TestFailedSetupBesideAnOperationalNeighborPublishesNoSessionDown` (`4f1e24bf66`) | Proven: no `(ldp, session-down)` is published for the interface. Not proven by any test: the OSPF LDP-sync machine's state on that interface; it follows by reading `handleSessionEvent`, as for AC-4 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The seven LDP unit tests | Done | `internal/plugins/ldp/session_end_test.go`, `adjacency_expiry_test.go` | `4f1e24bf66` |
| The eleven L2TP unit tests | Done | `internal/component/l2tp/session_event_order_test.go`, `plugins/authradius/acct_order_test.go` | `bae3903dbb`, with `session_ticket_panic_test.go`, `session_route_withdrawal_test.go` and the four reader-contract tests |
| The nine FlowSpec unit tests | Done | `internal/plugins/ddos/flowspec/exit_test.go`, `record_test.go`, `doctor_test.go` | `bec9096156`, `85507073e8`; added later: `section_removal_test.go` (`3e347a86e6`), `exit_signal_test.go`, `start_order_test.go`, `withdraw_order_test.go` (`3a19b67b81`), `exit_record_test.go`, `exit_run_test.go` (`e7e963b4e1`), `exit_kind_test.go`, `announce_record_level_test.go`, `transport_closed_test.go` (`bd27660e0d`) |
| SDK write-error tests (not in the plan) | Done | `pkg/plugin/rpc/conn_write_error_test.go` | `b6d1739194`, extended to the batch path in `fd6fedfc62` |
| The five functional `.ci` tests | Done | `test/ldp/`, `test/l2tp/`, `test/reload/`, `test/plugin/` | `0617ddff57`, `525577107c`, `af1d0bc452` |
| The AC-16 stopped-record tests (not in the plan) | Done | `internal/plugins/ddos/flowspec/doctor_stopped_test.go`, `doctor_stopped_record_test.go`, `internal/component/doctor/doctor_test.go` (`TestRuntimeChecksReceiveTheConfigurationDoctorRead`), `test/plugin/ddos-flowspec-stopped-orphan-doctor.ci` | `40e7226fca`, `c60b3fbbbb`, `c633ddf84d`, `0c9fecde5c` |
| `ldp-backoff-clear-frr` | Done | `test/interop/scenarios/ldp-backoff-clear-frr/` | `e299b1e031` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Files to Modify, LDP | Done | `4f1e24bf66`, `95f99add59` |
| Files to Modify, L2TP | Done | `bae3903dbb`; `reactor.go`'s change is in the drain and the collectors, not an emit-order lock (the ticket replaced it at design) |
| Files to Modify, DDoS and `codes.go` | Done | `bec9096156` and later; `config.go` added in `3e347a86e6` |
| `pkg/plugin/rpc/conn.go` (not in the plan) | Done | `b6d1739194`, `fd6fedfc62`: found by review round six (Deviations) |
| Pages and digests | Done | Documentation Updates above |
| Owned journal rows | Done | Marked in `4f1e24bf66`, `bae3903dbb`, `bec9096156` |
| Files to Create | Done | Files Exist below |

### Audit Summary
- **Total items:** 3 requirements, 19 acceptance criteria, 5 test groups, 6 file groups
- **Done:** all. AC-4, AC-8, AC-9 and AC-21 are done with the halves no test observes named in their rows. AC-13 is done for a plugin running as its own process and vacuous in process (A-15). AC-16's doctor report of the record with the plugin's process ended holds in the daemon for a configured plugin that is not running, and on both routes for an unconfigured section, with the two limits its row names
- **Partial:** none
- **Skipped:** none
- **Changed:** A-13, A-14, A-15, AC-16's removed-section half (Work Not Done), the SDK write-error fix, the RFC records re-taken at closure, recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| LDP: a session that reached operational clears the backoff and publishes SessionDown once; one that never reached it records a setup failure and publishes nothing | interop + functional | `ldp-backoff-clear-frr` against FRR 10.7.1: green, red (`ze logged "ldp: session setup failed, next attempt delayed" 1 time(s) after FRR cleared a session that had reached OPERATIONAL`), green, `e299b1e031`. `ldp-session-end-clears-backoff.ci` and `ldp-setup-failure-publishes-no-session-down.ci`, each green, red observed, 80 of 80 under load (`0617ddff57`) |
| L2TP: one session's events and route notifications reach their readers in state-change order | unit (controlled interleaving) + functional | The order tests of `session_event_order_test.go` (`bae3903dbb`), each forcing the interleaving the daemon cannot. `session-clear-leaves-no-subscriber.ci` through a real PPP session: one Accounting-Stop for one Start, empty session and subscriber lists, red with the session-down publication disabled, 80 of 80 under load (`525577107c`) |
| DDoS: each end that can reach the engine withdraws; an unwithdrawn rule is recorded, withdrawn at next start and reported by doctor | functional + unit | `ddos-flowspec-parent-removed-withdraws.ci`: `withdrawn=060120CB007109` at the peer on connection 1, red observed. `ddos-flowspec-orphan-doctor.ci`: doctor prints the code, prefix and fix, the restart withdraws and the report clears, red observed. Both green at their promoted paths on 2026-11-13 against the `c60a2b6edc` sources (11.1 s and 6.2 s) and 20 of 20 under load. `TestByeWithdrawsTheLiveAnnouncement` for `bye`; `TestRunEndingWithoutByeWithdrawsTheLiveAnnouncement` for a run that ends without `bye` in a plugin running as its own process. In process, a run that ends without `bye` cannot reach the engine (A-15): the rule stays recorded, the daemon's `show doctor` reports the stopped plugin (`doctor-plugin-stopped`) and the recorded rule with its victim, both with `ze request plugin restart ddos-flowspec` as fix, and the start that command makes withdraws the recorded rule, the start-time withdrawal the orphan proof shows. `ddos-flowspec-stopped-orphan-doctor.ci` proves the report of the recorded rule for a configured plugin that is not running (its startup handshake failed, no process held): green after each rework, red at the record's own row with the stopped report silenced (`c60b3fbbbb`, `c633ddf84d`); the unit table of `doctor_stopped_record_test.go` covers a plugin that ran, ended and is still held. Limits: AC-16's row. The two `.ci` proofs ran against the round-two sources; rounds three to seven changed log lines and the exit paths, not the reload, start or doctor paths those tests drive, and the `.ci` were not re-run in a guest after them: the exit lines and exit kinds are proven by the unit tests named in AC-13, not at daemon level |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | recorded by the main thread before commit A: `./le spec review record` with verdict CLEAN, rounds 11, and `rounds-reason` and `owner-authorised` (more than five rounds), over the files commit A carries |
| `./le spec review check` | run by the main thread after the record |
| Rounds | 11, closed: round eight clean at `fd6fedfc62` (0 BLOCKER, 0 ISSUE, 2 NOTE), then three narrow rounds over the AC-16 work, round eleven clean (0 BLOCKER, 0 ISSUE, 3 NOTE). Verdict CLEAN |
| Reviewer lenses used | Each round by a fresh independent reader that wrote none of the code. Round one over the whole change set (`4f1e24bf66` to `e299b1e031`); each later round over the previous round's fix commits. Rounds nine to eleven by one independent reviewer over the AC-16 commits only |

| Round | Scope | Result | Fixed in |
|-------|-------|--------|----------|
| 1 | The whole change set | 1 BLOCKER, 4 ISSUE, 6 NOTE | `3a19b67b81`; two NOTEs became journal rows owned elsewhere (`0b2afedc77`) |
| 2 | `3a19b67b81` plus the fixture change | 0 BLOCKER, 2 ISSUE, 1 NOTE | `c60a2b6edc` |
| 3 | `c60a2b6edc` | 0 BLOCKER, 2 ISSUE, 3 NOTE | `15d5cdd19b` |
| 4 | `15d5cdd19b` | 0 BLOCKER, 2 ISSUE, 3 NOTE | `e7e963b4e1` |
| 5 | `e7e963b4e1` | 0 BLOCKER, 3 ISSUE, 2 NOTE | `bd27660e0d` |
| 6 | `bd27660e0d` | 0 BLOCKER, 3 ISSUE, 2 NOTE | `b6d1739194`, `944149988c` |
| 7 | `b6d1739194`, `944149988c` | 0 BLOCKER, 1 ISSUE, 4 NOTE | `fd6fedfc62`; three NOTEs left as they are (Risks & observations) |
| 8 | `fd6fedfc62` | 0 BLOCKER, 0 ISSUE, 2 NOTE: clean | nothing to change |
| 9 | `40e7226fca`, `c60b3fbbbb` (AC-16) | 0 BLOCKER, 4 ISSUE, 4 NOTE | `c633ddf84d` |
| 10 | `c633ddf84d` | 0 BLOCKER, 3 ISSUE, 4 NOTE | `0c9fecde5c` |
| 11 | `0c9fecde5c` | 0 BLOCKER, 0 ISSUE, 3 NOTE: clean | note 1 recorded as a journal row; notes 2 and 3 left as recorded notes |

Rounds past five, the product defects later rounds found (for `rounds-reason`): round three, an in-process plugin that cannot withdraw on any end and a guide promising it would; round four, exit lines claiming a record that may not exist, and a plugin running as its own process not recognising the engine hanging up; round five, an unset exit kind accepted on some paths; round six, the plugin SDK reporting every failed write under a deadline as a timeout; round seven, the batch write path left on the old decision; round nine, a doctor check over live daemon state in the post-config phase, a restart offered while ze refuses it, and the plugin matched by process name only; round ten, two rows for one record and a storage row with no ground.

Round eleven's notes: (1) the not-configured report's effects beside a daemon that runs the plugin with a live mitigation (its "no running plugin can withdraw it" is false there, and its withdrawal ends the mitigation) are not recorded where the owning spec will fix them: handled by the journal row `plan/journal/identity-default-hides-a-mapping.md` (2026-11-14), owned by `plan/release/validation/spec-doctor-coverage-11-hub-engine.md`. (2) and (3) are comment-accuracy remarks, recorded and not changed.

Round eight's notes, recorded and not changed: (1) the rewrite of `writeLineWithContext` (test callers only) also changed what an oversized line returns under a deadline: it is now refused as "message exceeds maximum size", where the old code turned that refusal into a timeout; a correction, not a defect. (2) `writeBatchWithDeadline` does not check that the context has ended before it writes, unlike `writeFrame`; this predates `fd6fedfc62`. When a context with no deadline is cancelled on a transport with no write deadline, the batch is written and the call then fails at the read, so delivered events are reported as failed. Being recorded as a journal row owned by another spec (Rows considered and not owned).

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1.1 | round 1 (one of 1 BLOCKER, 4 ISSUE) | AC-13: the exit withdrawal when `Run` ends without `bye` had no test that reached it; removing it reddened nothing | `ddos/flowspec/register.go` `runEngine` deferred exit | `3a19b67b81`, `TestRunEndingWithoutByeWithdrawsTheLiveAnnouncement` |
| 1.2 | round 1 | The plugin subscribed to detections before its start-time withdrawal, so a detection was acted on while state calls were refused | `register.go` `OnStarted`, `OnConfigure` | `3a19b67b81`, `TestDetectionDuringTheHandshakeIsNotActedOn` |
| 1.3 | round 1 | AC-11's test did not assert the record removed or the status empty | `exit_test.go` | `3a19b67b81` |
| 1.4 | round 1 | A withdrawal cleared the live flag before it removed the record, so doctor could report an orphan for an instant | `responder.go` `withdrawThrough` | `3a19b67b81`, `TestWithdrawForgetsTheRecordBeforeClearingTheLiveFlag` |
| 1.5 | round 1 | The orphan report said the withdrawal did not reach the engine, when it can reach it and be refused | `doctor.go`, `codes.go`, the guide | `3a19b67b81` |
| 2.1 | ISSUE | The guide and the exit comment called the daemon-stop withdrawal a race; an in-process plugin never withdraws there | `register.go` `stopResponder` comment, the guide, the orphan proof's header | `c60a2b6edc` |
| 2.2 | ISSUE | The orphan report named one cause where it also covers a withdrawal whose record removal failed | `doctor.go`, `codes.go` | `c60a2b6edc`, `TestRuntimeOrphanReportNamesBothCausesOfAFailedWithdrawal` |

| 3.1 | ISSUE | An in-process ddos-flowspec never withdraws through its deferred exit on any end: `pkg/plugin/sdk/sdk.go` `(*Plugin).Run` defers `p.bridge.CloseCallbacks()`, which stops dispatch before `runEngine`'s deferred `stopResponder` runs, and `DispatchRPC` answers `ErrBridgeClosed`. The guide's "A crash" row, the `stopResponder` comment and the INFO line on a closed transport said more | the guide, `register.go` `stopResponder` comment and log line, one test's wording | `15d5cdd19b` |
| 3.2 | ISSUE | The orphan report and the code's Description named two causes while listing cases where no withdrawal was ever sent | `internal/core/diagnostic/codes.go`, `doctor.go` | `15d5cdd19b`: every cause named |
| 4.1 | ISSUE | The exit lines claimed a recorded rule when the record may not exist | `register.go`, `responder.go` | `e7e963b4e1`: the plugin knows whether its live rule is recorded; an unrecorded rule it cannot withdraw warns with the withdrawal by hand (`exit_record_test.go`) |
| 4.2 | ISSUE | The way the plugin ended was not one closed value, and no test drove the run itself | `register.go` `runEngine`, `stopResponder` | `e7e963b4e1`: removed, stopped or failed (`exitKind`), tests in `exit_run_test.go`. Found on the way: a plugin running as its own process did not recognise the engine hanging up at a daemon stop |
| 5.1 | ISSUE | An unset exit kind was accepted on some paths | `register.go` `stopResponder` | `bd27660e0d`: refused at the entry on every path (`exit_kind_test.go`); the zero value stays "unspecified", per `docs/contributing/ze-go-style.md` |
| 5.2 | ISSUE | The closed-connection match had no table test | `register.go` | `bd27660e0d` (`transport_closed_test.go`) |
| 5.3 | ISSUE | A failed record write over a record of the same rule was a WARN, and the lines said "not recorded" where the plugin only could not confirm a record | `responder.go`, `register.go`, the guide | `bd27660e0d`: an INFO line, "could not confirm a record", one builder for the withdrawal command (`announce_record_level_test.go`) |
| 6.1 | ISSUE | The SDK reported a failed write under a deadline as a timeout whatever failed, so the exit line could not read the cause | `pkg/plugin/rpc/conn.go` | `b6d1739194` (`conn_write_error_test.go`) |
| 6.2 | ISSUE | The table test's rows were not in the shapes the code produces | `transport_closed_test.go` | `944149988c` |
| 6.3 | ISSUE | The record-write warning did not carry the withdrawal by hand, and the guide's rows were not qualified | `responder.go`, the guide | `944149988c` |
| 7.1 | ISSUE | The batch write path, which carries every event delivery to a plugin, still made the write-error decision the old way | `pkg/plugin/rpc/conn.go` | `fd6fedfc62`: one helper for every write path |
| 9.1 | ISSUE | The stopped-plugin case was judged in the post-config phase from the daemon's plugin snapshot, live daemon state that phase cannot hold | `ddos/flowspec/doctor.go` | `c633ddf84d`: the runtime-phase `stoppedRecordCheck`, the runtime phase handed the saved configuration (Deviations) |
| 9.2 | ISSUE | The report offered the plugin restart while a cleanup is owed, when ze refuses it | `doctor.go` | `c633ddf84d`: the report says the restart is refused until the cleanup ends |
| 9.3 | ISSUE | The plugin was matched by process name only | `doctor.go` `pluginRunFromSnapshot` | `c633ddf84d`: a daemon with no process named ddos-flowspec gets a cannot-tell report, fix `ze show config running`, the withdrawal in the message only; the name keying is the journal row `guard-keyed-on-a-field-its-consumer-ignores.md` (2026-11-14) |
| 9.4 | ISSUE | The `.ci` fix assertion could be met by another row | `test/plugin/ddos-flowspec-stopped-orphan-doctor.ci` | `c633ddf84d`: asserted at the record's own row |
| 10.1 | ISSUE | One record got two rows when the saved file lacks the section | `doctor.go` `judgeStoppedRecord` | `0c9fecde5c`: the saved file decides first, one row per record |
| 10.2 | ISSUE | A storage row with no ground when the daemon holds no ddos-flowspec process | `doctor.go` `unavailableStoreFindings` | `0c9fecde5c`: silent there |
| 10.3 | ISSUE | The withdrawal instruction in the cannot-tell message was untested | `doctor_stopped_record_test.go` | `0c9fecde5c`: `TestStoppedRecordCheckUnrunNamesTheWithdrawalOnlyInTheMessage` |

Round one's severities per finding are in the reviewer's report: the round counted 1 BLOCKER and 4 ISSUE over rows 1.1 to 1.5. Round two's NOTE (the orphan helper called the record gone on any failed read) was taken in `c60a2b6edc` as well.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/ldp/session_end_test.go`, `adjacency_expiry_test.go` | Yes | `grep '^func Test'` lists `TestRunSessionReportsOperationalToOnDone` to `TestFailedSetupBesideAnOperationalNeighborPublishesNoSessionDown`, and `TestAdjacencyExpiryLeavesNoBackoffEntry` (2026-11-13) |
| `internal/component/l2tp/session_event_order_test.go`, `plugins/authradius/acct_order_test.go` | Yes | Same listing: the eleven order tests, `TestTeardownDuringIPAssignedLeavesNoAccountingSession` |
| `internal/plugins/ddos/flowspec/exit_test.go`, `record_test.go`, `doctor.go`, `doctor_test.go` | Yes | Same listing, and `doctor.go` read at `orphanRuntimeCheckDecl`, `orphanRecordCheck`, `stoppedRecordCheck`, `judgeStoppedRecord` (HEAD, 2026-11-14) |
| `internal/plugins/ddos/flowspec/doctor_stopped_test.go`, `doctor_stopped_record_test.go` | Yes | `git show --stat` of `40e7226fca`, `c633ddf84d`, `0c9fecde5c`; `grep '^func Test'` lists `TestPluginRunFromSnapshotNamesEachState` and the eighteen tests of `doctor_stopped_record_test.go` (2026-11-14) |
| `test/plugin/ddos-flowspec-stopped-orphan-doctor.ci`, `internal/test/fixture/ddos_flowspec_orphan_cleared.go`, `register_ddos_flowspec_orphan_cleared.go` | Yes | `git show --stat c60b3fbbbb`; the fixture files are a pure move the `.ci` does not use |
| The five `.ci` files and the fixture neighbor | Yes | `git show --stat` of `0617ddff57`, `525577107c`, `af1d0bc452`; the VALIDATES headers of each `.ci` name this spec |
| `test/interop/scenarios/ldp-backoff-clear-frr/` | Yes | `git show --stat e299b1e031`: `daemons`, `frr.conf`, `ze.conf` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-3, AC-5, AC-21 | The session-end callback branches on `operational`, then on `endedByAdjacencyExpiry`, else records a failure; SessionDown only when operational | Read 2026-11-13 at `ldp/register.go` `startSessionForAdj`, lines 630 to 674. Scoped `./le test unit pkg path ./internal/plugins/ldp/` green 2026-11-12 (spec A-1 row) |
| AC-7 to AC-10, AC-18 | Every publication runs inside a ticket delivered after `tunnelsMu` is released | Read 2026-11-13 at `reactor_kernel.go` `handleSessionIPAssigned` (`issueNoticeTicket` then `Unlock` then `ticket.deliver`), `teardown.go` `deliverSessionDowns`, `session.go` `drainPendingTeardowns` |
| AC-11 to AC-16 | `stopResponder` withdraws under `context.WithTimeout(context.Background(), exitWithdrawBudget)`, called from `OnBye` and the deferred exit | Read 2026-11-13 at `ddos/flowspec/register.go` lines 269 to 294, 416 to 419, 553 to 554. Both FlowSpec `.ci` green at their promoted paths 2026-11-13 |
| AC-16 | Three checks judge the record, one row per record; the runtime-phase `stoppedRecordCheck` reports a configured plugin that is not running in the daemon | Read 2026-11-14 at HEAD, `ddos/flowspec/doctor.go` `checkOrphanRecord`, `judgeStoppedRecord`, `pluginRunFromSnapshot`, `stoppedRunReport`, `unavailableStoreFindings`, and `register.go` `init` (both checks registered). Main thread: the units of `doctor_stopped_record_test.go`, `doctor_stopped_test.go` and `TestRuntimeChecksReceiveTheConfigurationDoctorRead` green, each MUTATION note observed red; `ddos-flowspec-stopped-orphan-doctor.ci` green after each rework; lint clean on host and Linux flavours for the three scopes touched; the weakening approvals of an independent judge carried as `Weakening-approved:` trailers (five on `c633ddf84d`, one on `0c9fecde5c`) |
| AC-6, AC-17 | The pages state the new behavior | `./le doc check all`: owed command |
| AC-11 to AC-16, the exit stretch (rounds three to seven) | The plugin package and the SDK packages pass | 2026-11-13, main thread: `./le test unit pkg path ./internal/plugins/ddos/flowspec/` passes; the exit, `bye` and ordering tests pass twenty times under the race detector; `./le test unit pkg path` over `./pkg/plugin/rpc/`, `./pkg/plugin/sdk/`, `./internal/component/plugin/ipc/` and `./internal/component/plugin/server/` passes; the SDK write tests passed fifty times under the race detector after `b6d1739194`, and `TestConnBatchWrite|TestConnWrite|TestConn_WriteLineWithContext` twenty times under the race detector after `fd6fedfc62` |
| The changed code is lint-clean | Scoped lint | `./le go lint run scope` for `./internal/plugins/ddos/flowspec/...`, `./pkg/plugin/rpc/...` and `./internal/core/diagnostic/...`: 0 issues on both platforms. `./le arch doctor-coverage check` and `./le plugin boundary check` pass |
| Daemon-level FlowSpec proofs | What is and is not re-proven | Both FlowSpec `.ci` passed in the guest at their promoted paths against the round-two sources, and 20 of 20 under load. They were not re-run in a guest after rounds three to seven, which changed log lines and the exit paths, not the reload, start and doctor paths those tests drive. The new exit lines and exit kinds are proven by unit tests only |
| RFC proof records | Fresh for every RFC this work touched | `./le rfc check` listed 29 stale items for RFC 5036 and RFC 2661, all from `4f1e24bf66`, `95f99add59` and `bae3903dbb`. Re-taken 2026-11-13 with `./le rfc record`: 13 RFC 5036 and 4 RFC 2661 records on the host and the 2 audit verdicts re-judged by an independent auditor, still enforced (`8f45f32b93`); the 10 Linux-only RFC 2661 records in Ze's QEMU guest (`4e4709af9d`). `./le rfc check` then named neither document; its other 158 findings belong to other work |
| Reds this work did not cause | Named, not fixed here | `./le arch tier check` fails on `internal/le/interoplab/bgp/ze_ready.go` (`fe96a2c67d`, 2026-11-11). `./le arch refuse-by-default check` names two sites in `internal/le/hookruntime/subagent_heavy.go`, another session's file. `TestRecoveryQueuedDeliveryTransport` and `TestRSFastReceiptRecipientExactlyOnce` (`internal/component/plugin/process`) fail identically with and without this work's SDK change. The whole-tree documentation check reports drift in the generated website and other pages, none naming this work's pages. `./le rfc check`'s 158 remaining findings |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| An LDP neighbor through the daemon: up, closed, redialled | `test/ldp/ldp-session-end-clears-backoff.ci` | Green, red observed, 80 of 80 under load, green at its promoted path (Functional Tests row) |
| A real PPP session cleared by the operator | `test/l2tp/session-clear-leaves-no-subscriber.ci` | Same |
| A reload deleting the whole `ddos` block | `test/reload/ddos-flowspec-parent-removed-withdraws.ci` | Same; 20 of 20 under load again on 2026-11-13 against `c60a2b6edc` |
| `ze doctor` with a recorded rule and no live responder | `test/plugin/ddos-flowspec-orphan-doctor.ci` | Green at its promoted path on 2026-11-13 against `c60a2b6edc` (11.1 s), 20 of 20 under load. Neither FlowSpec `.ci` was re-run after rounds three to seven (AC Verified) |
| `show doctor` through the daemon with `ddos flowspec` configured and the plugin not running | `test/plugin/ddos-flowspec-stopped-orphan-doctor.ci` | Green as a draft, at its promoted path and after each rework (`c633ddf84d`, `0c9fecde5c`); red at the record's own row with the stopped report silenced; no failure in 80 stress runs under load, on the first version only; runs on any host. It does not prove the restart clears the report (`ddos-flowspec-orphan-doctor.ci` does, with a running plugin) |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestRunSessionReportsOperationalToOnDone` (`4f1e24bf66`) |
| A-2 | confirmed | Census re-run 2026-11-12 and 2026-11-13 |
| A-3 | confirmed | `TestSessionEventReadersDoNotCallBackIntoTheReactor` and the four reader-contract tests (`bae3903dbb`) |
| A-4 | confirmed by read | `process/delivery.go` `Deliver` queues on the process's own channel, re-read 2026-11-13 |
| A-5 | confirmed | `TestSessionUpStartsTimeoutsAfterPublication` |
| A-7 | confirmed | `ddos-flowspec-parent-removed-withdraws.ci` (`af1d0bc452`) after `3e347a86e6` |
| A-8 | confirmed by read | `dispatch_route.go` `withdrawPluginRoutes` reads `installedByPlugin` only, re-read 2026-11-13 |
| A-9 | confirmed for the proofs | `reactor_api_batch.go` `splitOnAdvertised`; the orphan `.ci` asserts the withheld line |
| A-10 | confirmed | `internal/test/fixture/ldp_neighbor_linux.go` (`0617ddff57`) |
| A-11 | confirmed | `sdk_dispatch.go` `handleBridgeCallback`, `bridge.go` `beginDispatch`, re-read 2026-11-13; `TestByeWithdrawsTheLiveAnnouncement` |
| A-12 | confirmed by read | `reactor.go` `allocateLocalTID`, `discardTunnelLocked` |
| A-13 | broken | Mistake Log; `orphanRecordCheck` (`bec9096156`) |
| A-14 | broken | Mistake Log; the reactor's order tests carry AC-8's red |
| A-15 | broken | Mistake Log; review round three, read at `pkg/plugin/sdk/sdk.go` `(*Plugin).Run` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 6, user guide: `docs/guide/ddos-mitigation.md`, `docs/guide/mpls.md` | `stopResponder` comment and `OnBye` (`register.go`); `ldp: session operational` (`docs/guide/mpls.md` line 146) | Read 2026-11-13; `./le doc check all` owed |
| 6, user guide: `docs/guide/l2tp.md` | No statement about session event order: `grep -n -i order docs/guide/l2tp.md` returns only the PPP authentication option order | No edit needed |
| 10, test infrastructure: `docs/functional-tests.md` | `0617ddff57` adds the `ldp` suite's live-neighbor fixture section | Yes |
| 12, architecture: `docs/architecture/ldp/mpls-ldp.md`, `docs/architecture/l2tp/subscriber-session-model.md` | `4f1e24bf66`, `e299b1e031`, `bae3903dbb` | Yes; `./le doc check all` owed |
| 1, feature list | No `features/<id>.md` edited: three defect fixes | `./le feature report` for the ddos entry is owed |
| 9, RFC | No requirement row gains or loses proof. The records the code phases staled were re-taken (`8f45f32b93`, `4e4709af9d`) | `./le rfc check` names neither RFC 5036 nor RFC 2661 (2026-11-13) |
| 8, plugin SDK: `pkg/plugin/rpc/conn.go` | The write path keeps the cause of a failed write; no callback, field or wire byte changes | A grep of `docs/plugin-development/` and `docs/architecture/api/` finds no statement of how a write error is classified, so none was edited. Five pages anchor on `conn.go` (`Conn`, `NextID`, `writeFrame`; `process-protocol.md`, `wire-format.md`, `ipc_protocol.md`, `ike/ipsec-8-ikev2-child-xfrm.md`), and their claims (framing, serialised writes) are unchanged; `./le doc check all` owed || Security Review Checklist | Remote stall: `TestSlowReaderDelaysNoStateChange` and the 10 s Accounting-Stop bound; R-1 owned elsewhere. Backoff bypass: only the on-operational callback sets `operational` (`runSession`). Record content: a victim prefix and match fields under a key granted to this plugin only (`RegisterPluginKeys(Name, announcementKey)`). Untrusted input: a corrupt record is reported and never rendered (`answerOrphanRuntimeCheck` `recordCorrupt`, `TestCorruptRecordReportNamesTheReadCommand`). Generic: no new input parser, no unbounded allocation, no goroutine added | Read 2026-11-13 |
