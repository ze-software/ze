# Spec: bus-3-external-command-gating

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Priority | P0 |
| Release reason | Security fail-open boundary: every plugin process, a program Ze did not ship included, is stamped with `aaa.ReservedInternalPrefix` on `dispatch-command` and `dispatch-command-args`, which the authorizer allows before any profile lookup, so any configured plugin runs any daemon command; and `route-install`, `route-remove` and `inject-wire-route` check no caller permission at all |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-explicit-plugin-and-attach-config.md` |
| Phase | Design drafted 2026-11-23 on rulings R6 and R7. The check each operation makes today and the static fixture census are recorded (2026-11-23). Owed before the status moves to design: the capability of each operation, the design questions Q-T1 to Q-T4, the runtime capture of the per-fixture command set, the full Design documents verdicts and the changed-wire census |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Child 3 of 3, split from `plan/release/fixes/spec-explicit-plugin-and-attach-config.md` (Child 1). The file keeps its name; the check it designs is no longer for external programs alone.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Make one rule hold for every command a plugin sends, on every engine operation, for a compiled-in plugin and a program Ze did not ship alike: **a plugin sends only what it declared, minus what the operator's allow-list removes.** That rule, applied where the plugin identity is authorized, is what closes the AAA bypass.

### Owner rulings this spec stands on

Thomas speaks through voice transcription. Each row is the reading the main session confirmed.

| # | Ruling, as read | What it does here |
|---|-----------------|-------------------|
| D-8 (2026-11-17) | A flow's command list is the permission for every command a program sends, not only for a message toward a peer | Kept: one permission covers every operation. The list no longer sits on a flow: it is the one send allow-list on the plugin's endpoint (Thomas, 2026-11-23), and flows are receive-only |
| R6 (2026-11-23) | The operator's bus configuration is the trust boundary. A configured plugin is trusted whether compiled-in or external | The first draft's premise, "gate external programs, trust compiled-in ones", is withdrawn. The check is uniform |
| R7 (2026-11-23) | A plugin may send everything it declares. The operator narrows with an allow-list, per plugin. Present and empty means nothing; absent means everything declared. Anything not declared is refused | The check is "declared and not removed", not "granted by the operator" |
| Q6 Option A (2026-11-23) | Short capability tokens toward a subsystem, and command path prefixes toward the CLI | Kept as the vocabulary of the `Sends` declaration and of the allow-list |

### 1. What a send is, and who names it

Child 1 defines the `Sends` declaration and the one send allow-list of a plugin, `bus { endpoint <name> { send { allow [ ... ]; } } }` (Thomas, 2026-11-23: one list per plugin, covering every component it sends to). Each entry is `<component>:<token>`. This spec settles the vocabulary of the part after the colon, which Child 1's YANG validates against.

| Target component | Token form | Registered by | Validated by |
|------------------|-----------|---------------|--------------|
| A subsystem that accepts typed operations (`bgp`, `system-rib`, the event bus) | A short capability token: `update`, `refresh`, `raw`, `route-install`, `emit` | The subsystem, from its own `init()`, with the engine operations each token covers | Membership in that subsystem's registered set |
| `cli` | A command path prefix: `show bgp peer` | The command component. It registers `cli` as a target with a validator of its own, as any other component registers its tokens | The command component's validator: the prefix MUST match at least one registered command path, and the matcher is the one `authz` uses for profiles |

`cli` is therefore not a special case in the bus. It is a registered target like `bgp`: an entry reads `"cli:show bgp peer"` beside `bgp:update`, and the bus asks each target's registrant whether the token is valid.

Until this spec lands, Child 1 refuses a `send` list on a plugin that declares a send toward a component other than `bgp`, because one list speaks for every component and only the BGP send types are enforced. This spec removes that refusal: once every operation is checked, a list that is present means what it says for every component.

### 2. Every engine operation declares what it needs

The first draft listed nine operations by hand. `engineOps` (`internal/component/plugin/server/dispatch_registry.go`) holds more: the five `state-*` operations, `metrics-publish`, `report-issue`, `update-route`, `dispatch-command`, `dispatch-command-args`, `subscribe-events`, `unsubscribe-events`, `emit-event`, `forward-cached`, `release-cached`, `relay-stored-route`, `select-routes`, `cancel-request`, `select-refresh`, `route-install`, `route-remove`, `route-metrics`, `inject-wire-route`, `resolve-dns`, `batch-validate`, and any entry added later. Eleven more operations are registered by their owners through `RegisterEngineOp` (`internal/component/plugin/server/register_engine_op.go`) and live in the same lookup: the census report counted 12, and the six registration files hold 11.

| Rule | Design |
|------|--------|
| A field on the entry | Each `engineOps` entry states the capability it needs: a target and a token, or `none` with a written reason |
| `none` is a statement | `cancel-request` cancels the caller's own request. The `state-*` operations are governed by state key grants. `report-issue` and `metrics-publish` act on the caller's own records. Each reason is on the entry |
| An undeclared entry is refused | A test walks `engineOps` and fails on an entry with neither a capability nor a reason. A new operation therefore cannot ship ungated by omission |
| One place | The check runs in the shared dispatch path every entry already passes through, so the socket rail and the DirectBridge rail cannot differ |
| Commands by path | `dispatch-command` and `dispatch-command-args` need target `cli` and the command's path. The path, not the wire method, is the key |
| Receiving is not sending | `subscribe-events` and `unsubscribe-events` are bounded by the plugin's `Receives` declaration and the operator's flows, which Child 1 owns. Their entry names that |

Every operation and the check it makes today on its caller, read at the handler on 2026-11-23. The capability column is the design's and is still owed: this table records what exists.

| Operation | Handler (file) | Check on the caller today | Capability or reason |
|-----------|----------------|---------------------------|----------------------|
| `state-get`, `state-put`, `state-remove`, `state-list`, `state-increment` | `opState*` (`plugin/server/dispatch_state.go`) | State key grants: `statestore.PluginKeyReadable`, `PluginKeyWritable`, `PluginList`, keyed on `proc.Name()`, the operator's alias (Child 1 fixes the keying) | owed |
| `metrics-publish` | `opMetricsPublish` (`dispatch_metrics.go`) | None on the caller. It acts on the metric set owned by `proc.Name()` | owed |
| `report-issue` | `opReportIssue` (`dispatch_report.go`) | None on the caller. It reports under `proc.Name()`, rate-limited per owner (`pluginReportRate`) | owed |
| `update-route` | `opUpdateRoute` (`dispatch_registry.go`), `handleUpdateRouteSelResultDirect` (`dispatch.go`) | `RequirePeerDeclarations` (the reconcile gate), then the command runs under `internalPluginIdentity(proc.Name())`, which `Store.authorize` allows by prefix. The BGP rails then apply the per-peer send grant (`filterPermittedPeers`) | owed |
| `dispatch-command` | `opDispatchCommand`, `dispatchCommandPeerResponse` (`dispatch.go`) | The AAA prefix bypass: `internalPluginIdentity(proc.Name())`, allowed before any profile | owed |
| `dispatch-command-args` | `opDispatchCommandArgs`, `dispatchCommandArgsResponse` (`dispatch.go`) | `RequirePeerDeclarations`, then the AAA prefix bypass, as above | owed |
| `subscribe-events`, `unsubscribe-events` | `opSubscribeEvents`, `opUnsubscribeEvents` (`dispatch_registry.go`) | None at the operation. Delivery stays bounded by the peer's grant (`PeerScopedProcs`) until the next apply | owed (receiving, Child 1) |
| `emit-event` | `opEmitEvent` (`dispatch_registry.go`) | None. No check that the emitter owns the namespace it emits into | owed (AC-7) |
| `forward-cached` | `opForwardCached` (`dispatch_cached.go`) | `RequirePeerDeclarations`, then the per-peer send grant on the forward rails (`reactor_api_forward.go`, `reactor_api_forward_batch.go`) | owed |
| `release-cached` | `opReleaseCached` (`dispatch_cached.go`) | None. It releases cache entries under `proc.Name()` | owed |
| `relay-stored-route` | `opRelayStoredRoute` (`dispatch_cached.go`) | `RequirePeerDeclarations`, then the per-peer send grant on the relay rail (`reactor_api_relay.go`) | owed |
| `select-routes` | `opSelectRoutes` (`dispatch_selected.go`) | The caller's `SelectedOwner`, the per-peer send grant (`reactor_api_selected.go`), and for cancellation the destination's `update` grant (`selectedCancellationScope`, `request_cancel.go`) | owed |
| `cancel-request` | `opCancelRequest` (`request_cancel.go`) | It cancels the caller's own request | owed (`none` proposed in the draft) |
| `select-refresh` | `opSelectRefresh` (`dispatch_selected_refresh.go`) | The caller's `SelectedOwner`, and `DeliveryGraph.MaySend(destination, proc.Name(), "update")` | owed |
| `route-install`, `route-remove` | `opRouteInstall`, `opRouteRemove` (`dispatch_route.go`) | None. They decode, apply and count under `proc.Name()` | owed (AC-4) |
| `route-metrics` | `opRouteMetrics` (`dispatch_route_metrics.go`) | None | owed |
| `inject-wire-route` | `opInjectWireRoute` (`dispatch_registry.go`) | `RequirePeerDeclarations` only, on both rails | owed (AC-4) |
| `resolve-dns` | `opResolveDNS` (`dispatch_resolve.go`) | None | owed |
| `batch-validate` | `opBatchValidate`, `dispatchBatchValidation` (`dispatch_batch_validate.go`) | Dispatches `rpc.BatchValidateCommand` through `dispatchCommandArgs`, so the AAA prefix bypass | owed |
| `ze-bgp:source-lease` (owner-registered) | `sourceLeaseJSON` (`bgp/reactor/register_source_lease.go`) | Bound to the caller's `SelectedOwner` | owed |
| `ze-firewall:tables-replace` (owner-registered) | `tablesReplaceHandler` (`firewall/register_contribution.go`, `contribution.go`) | A table reservation keyed on `proc.Name()`, the operator's alias (`refuseOwnerWithoutReservation`) | owed |
| `ze-sysrib:route-view-open` (owner-registered) | `routeViewOpenHandler` (`sysrib/register_route_view.go`) | `routeViewAuthorize`: the caller's `RegistrationName()` must be the system RIB | owed |
| `ze-sysrib:route-view-read`, `route-view-close`, `route-view-candidates`, `route-view-distances`, `route-view-publish` (owner-registered) | `sysrib/register_route_view.go`, `register_route_view_policy.go`, `register_route_view_output.go` | Keyed on the caller process; `route-view-distances` acts on a session the caller opened (`withSession`). The others were not read further | owed |
| `ze-iface:owned-bridges-replace`, `owned-link-create`, `owned-link-remove` (owner-registered) | `iface/cmd/owned_devices.go` | `iface.RefuseUngrantedDeviceOwner(proc.Name())`, keyed on the operator's alias | owed |

Design questions the table forces. None is answered here.

| # | Question | The data that forces it |
|---|----------|-------------------------|
| Q-T1 | Does the capability rule, and the walk of AC-9, cover the owner-registered operations? | They are declared through `EngineOperation` (`register_engine_op.go`), which has no capability field, not through an `engineOps` entry |
| Q-T2 | Which identity do the checks keyed on `proc.Name()` move to? | State grants, the firewall table reservation, the interface device-owner grant, the metric and issue owners, and the cache release all key on the operator's alias. Child 1's identity fix names the state and doctor sites and five siblings; the firewall and interface grants are not among them |
| Q-T3 | Is `batch-validate` judged as the `cli` path it dispatches, or by a token of its own? | It reaches the dispatcher through `dispatchCommandArgs` with a fixed command |
| Q-T4 | Is `update-route` judged as `bgp:update`, as the `cli` path of its command text, or both? | It runs its command text under the plugin identity through the dispatcher, then the BGP send rails check the send type |

### 3. How the AAA bypass closes

Today `dispatchCommandPeerResponse` and `dispatchCommandArgsResponse` (`internal/component/plugin/server/dispatch.go`) set the command's username to `internalPluginIdentity(proc.Name())`, which carries `aaa.ReservedInternalPrefix`. `Store.authorize` (`internal/component/authz/authz.go`) returns `Allow` for that prefix before it looks at any profile. The comment on `internalPluginIdentity` says the plugin name "never affects the authorization decision".

| Option | What it does | Verdict |
|--------|--------------|---------|
| (a) A profile for the plugin identity | The plugin's declared `cli` sends, cut down by the operator's allow-list, are compiled into an ordinary authorization profile bound to the plugin identity. `Store.authorize` stops allowing the `plugin:` identities by prefix and evaluates that profile with the matcher it uses for every user. Accounting and audit keep the identity | Chosen: one authorizer, one matcher, one audit trail; `show` of the profile is the existing surface |
| (b) A second gate in `plugin/server` before the dispatcher | The plugin server checks the path against the allow-list, then dispatches under the reserved identity as today | Set aside: two authorizers for one command, and the prefix bypass stays in `authz` for any path that forgets the first gate |
| (c) Keep the bypass for compiled-in plugins | Only programs Ze did not ship are checked | Set aside: R6 rules out a trust difference, and a compiled-in plugin forked by the operator would lose or keep commands by how it runs |

`internalRPCIdentity`, the reserved identity of an in-process RPC call that carries no plugin, stays allowed, with its reason restated on the constant: it names no plugin, so no declaration exists to check it against. Each of its callers is listed in the design.

### 4. One refusal, one counter, one doctor code

| Existing mechanism | Where | Today | Here |
|--------------------|-------|-------|------|
| Request refusal event | `internal/component/plugin/server/dispatch_report.go` (`refusePluginRequest`) | Raises `diagnostic.CodeDoctorPluginRequestRefused`, at most once per plugin per window | Extends: a send refused by the check raises it, with the plugin, the token and the fix |
| Send refusal counter | `internal/component/bgp/reactor/send_permission.go` (`recordSendRefused`, metric `ze_bgp_send_refused_total`, labels process and type) | Counts BGP send refusals | Replaces: one counter in the plugin server for every operation, labels plugin, target and token. The BGP counter is deleted, its dashboards and page updated |
| Send refusal text | `send_permission.go` (`sendPermissionDenied`, `errSendNotPermitted`) | An operator error with its fix | Extends: one text shape for every operation, saying which of the two reasons applies |

The two reasons a send is refused, and what the operator reads:

| Reason | Message |
|--------|---------|
| The plugin did not declare it | Names the plugin and the token, says the plugin's registration does not declare it, and gives the command that shows what it does declare. No config line fixes this: the plugin's author does |
| The operator's allow-list removed it | Names the plugin and the entry, says the endpoint's send allow-list does not list it, and gives the exact line that adds the entry |

### 5. The ExaBGP bridge

The bridge relays each line a script writes through `DispatchCommand` (`internal/plugins/exabgp/bridgerun/script.go`), so every script command arrives as a `cli` send of the bridge plugin. Under R5 the bridge declares the whole ExaBGP API command set: every command path its translator can emit, derived from the translator's own table so the two cannot drift. The check then applies to the bridge as to any plugin. A script cannot make the bridge send a command the bridge did not declare, and an operator's allow-list on the bridge narrows every script at once.

### 6. The fixture command census

Every external test fixture and every in-tree plugin now needs a `Sends` declaration that covers what it sends, or its tests fail closed. The census is taken once, before any fixture is edited, so each declaration and each test config is written once.

| Column | Content |
|--------|---------|
| Fixture or plugin | Its package or script path |
| Operations it calls | From its source: each engine operation and each command path it dispatches |
| Declaration it needs | The `Sends` entries |
| Tests that run it | The `.ci` files and Go tests |

The static census, taken 2026-11-23:

| Measure | Result |
|---------|--------|
| Fixtures the functional tests start | 827 fixture names, used by 1,093 `.ci` files |
| Where they are registered | 167 source files under `internal/test/fixture/`; 796 names resolved to their source by name, 31 not resolved statically |
| SDK calls in the fixture sources | `UpdateRoute` 51, `DispatchCommand` 48, `RouteMetrics` 7, `RouteInstall` 6, `ReleaseCached` 4, `EmitEvent` 4, `ForwardCached` 3, `RouteRemove` 3, the `State*` calls 5, `RaiseIssue` and `ClearIssue` 3 |
| Literal command prefixes they dispatch | Led by `show bgp` (67), `request shutdown` (22) and `request quiesce` (21) |
| Why the static census is not the per-fixture set | Most fixtures dispatch through about 25 shared wrapper helpers, so a fixture's source does not show which commands it sends |

The complete per-fixture set comes from a runtime capture through the dispatcher's existing plugin accounting: every command a plugin dispatches already carries the sending plugin (`Username` from `internalPluginIdentity(proc.Name())` and `Sender` from `plugin.ProcessSender(proc.Name())`, set in `dispatchCommandArgsResponse`, `dispatchCommandPeerResponse` and `opUpdateRoute`). A capture over the functional suites that reach the fixtures records, for each plugin name, every command path and engine operation it used. That capture is the "Operations it calls" column, and the declarations are written from it once.

The per-operation table of section 2 holds every operation, including those no fixture calls.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/process-protocol.md` - the plugin RPCs and their descriptions
  -> Constraint: `internal/core/ipc/yang/ze-plugin-engine.yang` describes the `command` leaf of `dispatch-command` as "The engine gives the text to the same dispatcher the CLI uses, under the calling plugin's internal identity. It is not prefixed, so the plugin writes the whole command." It says nothing about trust or authorization. The word "trusted" is in the code comments: `aaa.ReservedInternalPrefix` (`internal/component/aaa/reserved.go`), `Store.authorize` (`internal/component/authz/authz.go`) and `opUpdateRoute` (`internal/component/plugin/server/dispatch_registry.go`). The YANG description and those comments change with the behavior
- [ ] `docs/architecture/api/architecture.md` - "Owner ruling: endpoints and flows on the bus", "Other command transports", "Data Flow: Config to Delivery"
  -> Decision: the endpoint's one send allow-list (`bus { endpoint <name> { send { allow [ ... ]; } } }`) is the permission for every command a plugin sends; a flow carries no send list
  -> Constraint: identity is injected by the transport and never accepted from the payload; the table there calls an internal plugin "engine-trusted", which R7 replaces
- [ ] `docs/architecture/plugin/plugin-system.md` - "One plugin, two ways to reach the bus"
  -> Constraint: a rule for external plugins alone needs a transport or arrival-order reason; this check has none and is uniform
- [ ] `ai/doors/access-security.md` - the authorizer and its profiles
  -> Constraint: command authorization reads one accepted generation of policy; a plugin profile joins that generation and is swapped with it on reload

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/dispatch.go` - `internalPluginIdentity`, `internalRPCIdentity`, and `dispatchCommandArgsResponse` setting `Username` and `Sender` for every plugin process
- [ ] `internal/component/plugin/server/dispatch_registry.go` - `engineOps`, the one table the JSON, Direct and typed paths derive from; an entry has no capability field
- [ ] `internal/component/plugin/server/dispatch_route.go` - `opRouteInstall` and `opRouteRemove` decode, apply and count, with no caller permission check
- [ ] `internal/component/plugin/server/dispatch_report.go` - `refusePluginRequest`
- [ ] `internal/component/authz/authz.go` - `Store.authorize` allows any username under `aaa.ReservedInternalPrefix` first
- [ ] `internal/component/aaa/reserved.go` - `ReservedInternalPrefix`: the descriptor after it "never affects the decision"
- [ ] `internal/component/bgp/reactor/send_permission.go` - `Peer.maySend`, `bindingsMaySend`, `filterPermittedPeers`, `recordSendRefused`: the one fail-closed send check today, for both kinds of plugin, on the ten BGP send rails
- [ ] `internal/plugins/exabgp/bridgerun/script.go` - `scriptRunner.Dispatch` relays each script command through `DispatchCommand`

**Behavior to preserve:**
- Operators at the CLI, SSH, web, REST, gRPC and MCP stay governed by AAA and are never limited by bus flows.
- A compiled-in plugin keeps every command it uses today, because it declares them.
- The socket rail and the DirectBridge rail give the same answer.
- Accounting and audit keep naming the plugin.

**Behavior to change:**
- No plugin identity is allowed by prefix. Every plugin's commands are judged against its declaration and the operator's allow-list.
- Every engine operation states the capability it needs or why it needs none.
- A refusal says which of the two reasons applies, counts once, and raises the doctor code.
- The BGP send counter gives way to one counter for every operation.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| BGP send permission | `internal/component/bgp/reactor/send_permission.go` (`Peer.maySend`, `filterPermittedPeers`) | Fails closed on the ten BGP send rails for a process, exempts the operator | Extends: Child 1 makes it plugin-wide; this spec moves the same question to the shared dispatch path for every operation and deletes the rail-level copy where the shared check covers it |
| Authorization profiles and prefix matcher | `internal/component/authz/authz.go` (`Store.authorize`, `Entry.matches`) | Matches a command against a user's profiles; allows the reserved internal prefix first | Extends: a plugin identity resolves to a profile compiled from its declared `cli` sends and the allow-list. The prefix allowance for plugin identities is deleted |
| Reserved identities | `internal/component/aaa/reserved.go` (`ReservedInternalPrefix`); `internal/component/plugin/server/dispatch.go` (`internalPluginIdentity`, `internalRPCIdentity`) | Injects an un-typeable username the authorizer always allows | Replaces for plugins: the identity stays un-typeable and stays in accounting, and no longer decides. Kept for the plugin-less RPC identity, with its reason |
| Engine operation table | `internal/component/plugin/server/dispatch_registry.go` (`engineOps`) | One entry per plugin-to-engine RPC, from which every rail derives | Extends: a capability field per entry, and a test that refuses an entry without one |
| Request refusal event | `internal/component/plugin/server/dispatch_report.go` (`refusePluginRequest`) | Raises `CodeDoctorPluginRequestRefused`, bounded per plugin | Extends: raised for a refused send |
| Send refusal counter and text | `send_permission.go` (`recordSendRefused`, `sendPermissionDenied`) | A BGP-only counter and a log line with a fix | Replaces: one counter and one text shape for every operation |
| Declaration check at Stage 1 | `internal/component/plugin/server/stage1_checks.go` (the Stage 1 check table) | Judges each field of a declaration for both kinds of plugin | Extends: a `Sends` token must be one its target registered |
| Mock external runner | `./le mock plugin-external` | Runs a registered engine plugin as an external process | Extends: used to prove a forked compiled-in plugin keeps its commands |

## Design documents

The full verdict table is owed before the status moves to design. The rows already judged:

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `docs/architecture/plugin/plugin-system.md` | One plugin, two ways to reach the bus | page the plugin door names | "A behaviour that differs between the two kinds with no reason of this kind is a defect." | agrees | The check is the same for both kinds |
| `docs/architecture/api/architecture.md` | Other command transports | page of `plugin/server` | "User identity for authorization is injected by the transport layer and is never accepted from the client payload:" | agrees | The plugin identity is still injected by the engine; what changes is that it is authorized, not waved through |
| `docs/architecture/api/architecture.md` | Owner ruling: endpoints and flows on the bus | same page | "The command list is the permission for every command an external program sends" | departs | R6 and R7: the list is an allow-list that narrows what any plugin declared, compiled-in or external, and it is one list on the plugin's endpoint, not a list on a flow; page updated in the same work |
| `ai/doors/plugin-infra.md` | Boundaries | covering door | "One engine-op entry per RPC." | agrees | The capability is a field of that entry; no second path |
| `ai/doors/access-security.md` | Who depends on it from outside | covering door | "Login is not an SSH feature." | agrees | Plugin profiles join the one policy generation every surface reads |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Any plugin-to-engine RPC, over the TLS socket or the DirectBridge, from a compiled-in plugin or a program Ze did not ship.

### Transformation Path
1. Child 1 resolves, for each plugin, the sends it declared and the allow-list that narrows them.
2. The `cli` part is compiled into an authorization profile for the plugin identity, in the policy generation a reload swaps.
3. The shared dispatch path reads the operation's capability from its `engineOps` entry and asks whether the plugin holds it. `none` passes with its reason.
4. A command dispatch goes on to the authorizer under the plugin identity, which evaluates that profile.
5. A refusal builds the message for its reason, counts once, raises the doctor event, and answers the plugin with a standard error.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin process <-> engine RPC | The authenticated process identity, checked against its own declaration | No |
| Plugin server <-> authorizer | A profile per plugin identity, no reserved-prefix allowance | No |

### Changed-Wire Census

**Search evidence:** owed at design. No RPC shape changes. The description of `dispatch-command` in `internal/core/ipc/yang/ze-plugin-engine.yang` changes, the refusal text changes, and the metric `ze_bgp_send_refused_total` is replaced.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| owed at design | | | | |

### Integration Points
- The edge resolution of Child 1 (declared sends and the allow-list).
- The policy generation of the authorizer.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every command a compiled-in plugin dispatches is known from its source, so its declaration can be complete | Plugins dispatch command strings they build; `./le cli dispatch check` already finds the strings the tree sends | A plugin builds a path from run-time data and fails closed in production | The fixture command census, and `./le cli dispatch check` extended to compare each sent string with the sender's declaration | unvalidated |
| A-2 | The check costs nothing measurable on the route path | `update-route`, `forward-cached` and `relay-stored-route` already ask the send permission once per command | A per-route check appears on a typed bridge slot | A benchmark of the typed slots reporting allocations per operation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A compiled-in plugin loses a command it needs, because its declaration is incomplete | A functional suite fails with the "did not declare" refusal | The census; the dispatch check compares sent strings with declarations at build time |
| R-2 | A forked compiled-in plugin differs from the same plugin in the daemon | A test under `./le mock plugin-external` fails where the internal run passes | Identity is the registration's on either transport (Child 1), so the declaration and the profile are the same |
| R-3 | The security test is green for a reason other than the check | The negative passes before the fix is written | The negative is recorded red first, against today's code, and its red output is kept |
| R-4 | A reload that removes a grant leaves the old profile in force | A command still succeeds after the reload | The profile lives in the policy generation the reload swaps |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A plugin keeps a live path to every daemon command, or a plugin that worked is refused a command it needs |
| How is it reverted? | Revert of this spec's commits |
| Who else touches this path? | Child 1 defines the declaration and the allow-list |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| A program that dispatches a command it did not declare | -> | The profile for the plugin identity in the authorizer | `test/plugin/bus-send-undeclared-refused.ci` (planned), recorded red first |
| A program that calls `route-install` without declaring it | -> | The capability on the `engineOps` entry | `test/plugin/bus-send-operation-capability.ci` (planned) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A program Ze did not ship dispatches `clear bgp peer`, which it did not declare. No allow-list is written | Refused with the "did not declare" message. The counter increments once and doctor reports `CodeDoctorPluginRequestRefused` for that plugin. Against today's code this test is red: the command runs | Operators on every surface are unaffected |
| AC-2 | The same program declares `show bgp peer` and dispatches it, then `show interface` | The first succeeds with no allow-list written. The second is refused | Only declared paths pass |
| AC-3 | The program declares `show bgp peer` and `clear bgp peer`; the operator writes `send { allow [ "cli:show bgp peer" ]; }` on its endpoint, then an empty `send { }` | With the first list, `show` passes and `clear` is refused with the allow-list message and its fix line. With the empty list, both are refused | An absent list passes both |
| AC-4 | A program calls `route-install`, `route-remove` or `inject-wire-route` without declaring the capability, then with it | Refused without; succeeds with | None |
| AC-5 | A compiled-in plugin, in the daemon and under `./le mock plugin-external`, runs its normal work | Every command it dispatches succeeds in both, with no allow-list written, because it declares them | None: a compiled-in plugin that is refused a command it uses is a defect of its declaration |
| AC-6 | A compiled-in plugin is given an allow-list that removes a command it uses | The command is refused in the same message shape as AC-3. A compiled-in plugin is narrowed exactly as a program is | None |
| AC-7 | A plugin calls `emit-event` for an event it did not register as a publication | Refused. With the publication registered, delivered | None |
| AC-8 | A reload removes a token from an allow-list while the plugin runs | The next send of that token is refused. A reload that adds it back makes the next send succeed | None |
| AC-9 | Every entry of `engineOps` | Each carries a capability or `none` with a reason. A test entry with neither fails the walk | None |
| AC-10 | An ExaBGP script, through the bridge, sends a line the translator turns into a command the bridge declares, then the operator narrows the bridge's allow-list | The first succeeds with no list written. After the narrowing the script's line is refused and the script receives the error ExaBGP's API contract gives for a failed command | None |
| AC-11 | The plugin-less in-process RPC identity | Still allowed, and every caller of it is listed with its reason | None |
| AC-12 | A plugin that declares sends toward `bgp` and toward `cli`, with one list `send { allow [ bgp:update ]; }` | The commit loads (the interim refusal of Child 1 is gone), `bgp:update` passes, and every `cli` command the plugin declared is refused with the allow-list message | None: one list speaks for every component |
| AC-13 | `ze config validate` with no daemon running, on a config whose endpoint lists a `send` entry an external program did not declare | Refused, with what the program declared, taken from the program in query mode | None |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a monitoring script that declares two `show` commands, then narrows it to one | Stage 1 declaration -> profile -> authorizer; reload -> new profile | `test/plugin/bus-send-undeclared-refused.ci`, `test/reload/bus-send-allow-list-reload.ci` (planned) |
| 2 | Finds out why a plugin's command fails | `show doctor` -> the refusal with its reason and fix | `test/plugin/bus-send-refusal-doctor.ci` (planned) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryEngineOpDeclaresItsCapability` | `internal/component/plugin/server` | AC-9: the walk, and a fixture entry with neither that fails it | |
| `TestPluginIdentityIsAuthorizedByProfile` | `internal/component/authz` | AC-1, AC-2, AC-3: no prefix allowance for a plugin identity; absent, partial and empty lists | |
| `TestInternalRPCIdentityStaysAllowed` | `internal/component/authz` | AC-11 | |
| `TestSendRefusalNamesItsReason` | `internal/component/plugin/server` | The two messages, the counter and the doctor event | |
| `TestCompiledInDeclarationsCoverWhatTheySend` | `internal/component/plugin/all` | AC-5: each registration's `Sends` covers the command strings its package sends | |
| `TestBridgeDeclaresWhatItsTranslatorEmits` | the bridge plugin's package | AC-10 | |
| `TestEmitNeedsARegisteredPublication` | `internal/component/plugin/server` | AC-7 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric inputs introduced | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bus-send-undeclared-refused` (planned) | `test/plugin/` | AC-1, AC-2: the security negative, recorded red first | |
| `bus-send-operation-capability` (planned) | `test/plugin/` | AC-4, AC-7 | |
| `bus-send-compiled-in-keeps-commands` (planned) | `test/plugin/` | AC-5, internal and forked | |
| `bus-send-compiled-in-narrowed` (planned) | `test/plugin/` | AC-6 | |
| `bus-send-allow-list-reload` (planned) | `test/reload/` | AC-3, AC-8 | |
| `bus-send-refusal-doctor` (planned) | `test/plugin/` | The doctor code and the counter through `show doctor` and the metrics endpoint | |
| `bridge-send-narrowed` (planned) | `test/exabgp/` | AC-10 | |

## Files to Modify
- `internal/component/plugin/server/dispatch.go`, `dispatch_registry.go`, `dispatch_route.go`, `dispatch_report.go` - the capability field and the shared check; the plugin identity no longer waved through; the refusal
- `internal/component/authz/authz.go`, `internal/component/aaa/reserved.go` - the profile for a plugin identity; the narrowed meaning of the reserved prefix
- `internal/component/bgp/reactor/send_permission.go` - the BGP counter and the rail-level check give way to the shared one
- `internal/core/ipc/yang/ze-plugin-engine.yang` - the description of `dispatch-command`
- `internal/plugins/exabgp/bridgerun/script.go` and the bridge's registration - the declared command set
- Every plugin registration and external fixture the census lists - its `Sends` declaration: every in-tree plugin registration, and the 167 fixture source files under `internal/test/fixture/` that register the 827 fixtures the 1,093 `.ci` files start
- The six files that register owner operations (`bgp/reactor/register_source_lease.go`, `firewall/register_contribution.go`, `sysrib/register_route_view.go`, `register_route_view_policy.go`, `register_route_view_output.go`, `iface/cmd/owned_devices.go`) - their capability, once Q-T1 is answered
- `docs/architecture/api/process-protocol.md`, `docs/architecture/api/architecture.md`, `docs/plugin-development/`, `docs/plugin-development/metrics.md`, `docs/guide/authorization.md` - the check, the counter, the plugin profile

## Files to Create
- `test/plugin/bus-send-undeclared-refused.ci`, `test/plugin/bus-send-operation-capability.ci`, `test/plugin/bus-send-compiled-in-keeps-commands.ci`, `test/plugin/bus-send-compiled-in-narrowed.ci`, `test/plugin/bus-send-refusal-doctor.ci`, `test/reload/bus-send-allow-list-reload.ci`, `test/exabgp/bridge-send-narrowed.ci`

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- write `bus-send-undeclared-refused.ci` and run it against today's code. Keep its red output in this spec. Take the fixture command census.
2. **Phase: Declarations** -- every plugin and fixture declares its sends; the Stage 1 check for tokens.
3. **Phase: Capability per operation** -- the field, the walk test, the shared check.
4. **Phase: Authorizer** -- the plugin profile; delete the prefix allowance for plugin identities.
5. **Phase: Refusal** -- the two messages, the one counter, the doctor event; delete the BGP counter.
6. **Phase: Bridge and docs.**

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `escape-hatch-scoped-wider-than-its-justification.md` | 2026-11-23 | `internalPluginIdentity` (`plugin/server/dispatch.go`) stamps `aaa.ReservedInternalPrefix` on every plugin process, a forked program included, and `Store.authorize` (`authz/authz.go`) allows the prefix before any profile lookup; `opRouteInstall` and `opRouteRemove` (`dispatch_route.go`) check no caller permission |

The row was written on 2026-11-23, after a search of `plan/journal/` for `ReservedInternalPrefix`, `internalPluginIdentity` and "plugin identity" found none. It records the authorization bypass, verified at `internalPluginIdentity`, `dispatchCommandPeerResponse` and `dispatchCommandArgsResponse` (`plugin/server/dispatch.go`) and at `Store.authorize` (`authz/authz.go`). The unchecked `route-install` and `route-remove` named in the surface above are not in that row.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One check for both kinds of plugin: declared, minus the allow-list (R6, R7, 2026-11-23) | Gate external programs only (the first draft, D-8 as first read) | Thomas ruled that a configured plugin is trusted alike, and that a plugin sends what it declares |
| A profile for the plugin identity in the authorizer | A second gate before the dispatcher; keep the prefix allowance | One authorizer and one matcher; the bypass is deleted where it lives |
| A capability field on each `engineOps` entry, with a walk test | A hand list of gated operations | A hand list was already incomplete; a new operation cannot ship ungated by omission |
| `cli` is a target the command component registers | A `from cli` case in the bus | The bus names no component |
| Capability tokens toward a subsystem, path prefixes toward `cli` (Q6 Option A) | Tokens for every command; prefixes everywhere | Short lists for typed operations; no token invented per command |

## Known Limitations
- A send cannot be limited to some peers or tags: the allow-list is one list per plugin (R7). A route server serves every peer connected to it (RFC 7947 Section 1), and export policy is what limits an advertisement per peer.
- A plugin that builds a command path from run-time data must declare the prefix it builds under.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] The security negative was recorded red against the code before the fix, and its output is in this spec
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: scoped package tests with `./le test unit pkg`, and `./le test functional plugin`, `reload`, `exabgp`
- [ ] The fixture command census and the per-operation capability table are recorded in this spec
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-user behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + the journal row
- [ ] **Commit B:** `remove plan/release/fixes/spec-bus-3-external-command-gating.md` only
