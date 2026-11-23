# Spec: explicit-plugin-and-attach-config

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P0 |
| Release reason | Security and configuration correctness of shipped behavior: closes the authorization bypass every plugin identity holds on `dispatch-command` (`aaa.ReservedInternalPrefix`), replaces the ExaBGP-derived `plugin { }` and per-peer `attach process` grammar with one message bus on which every participant registers what it sends and receives and the operator overrides it with `to`, `from` and allow-lists, shows every registered default beside what the operator wrote, converts all repository configs, tests and docs with no hybrid layering, and fixes aliased plugin state-key and doctor snapshot keying |
| RFC gaps | - |
| Depends | - |
| Phase | Design rewritten 2026-11-23 as ONE spec on the message model (participants, `to`, `from`, allow-lists), absorbing `spec-bus-2-implied-display-and-grant-refusals.md` and `spec-bus-3-external-command-gating.md`. NOT ready for the judge: the ExaBGP section records owner decision E-1 (the bridge section and the internal bridge mode are removed) whose design investigation is still owed, and the census of the configs that use the bridge section is not taken. Then `./le spec design check`, the design judge and owner approval. Open for Thomas: "Questions for Thomas" under "Choices for Thomas" |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Bucket: `plan/release/fixes/`. One spec carries the whole work (Thomas, transcribed, 2026-11-23: "I think we should have one spec because all the work is very interdependent and while it's harder to do, it can ensure that everything works before it's committed."). The two former child specs, `spec-bus-2-implied-display-and-grant-refusals.md` and `spec-bus-3-external-command-gating.md`, are absorbed here whole (goals, acceptance criteria, censuses, tests and the owned journal row) and are removed by the main session through the commit tool. The work is cut into phases inside this spec ("Implementation Steps"), never into child specs.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Replace `plugin { internal ...; external ...; hub ...; }` and BGP's `attach process <name> { ... }` (including inline `run`/`use` and the dead `content` container) with one message bus. Everything that takes part in the bus is a participant: a BGP peer, a BGP group, a plugin, a program Ze did not ship, an ExaBGP script. A participant registers a name based on its configuration path and says which messages it sends and which it receives. Events and commands are one thing: messages, each with a name, content and tags, the sender being a mandatory tag. With nothing written by the operator, what the participants registered is what runs. The operator overrides it near the emitter, in a `bus` block on the participant's own configuration: `to` says where this participant's messages go, `from` says who may address it, and a send and a receive allow-list say which message names it may send and receive. The bus compiles what is written on the participant and what is written in the `bus` section into one filter per participant and enforces it when it receives a message. One display shows the registered defaults beside what the operator wrote. The same filter is applied to every engine operation a plugin calls, which closes the authorization bypass plugin identities hold today. The work also fixes the aliased-plugin identity defects on the state and doctor surfaces, and converts every config, test and page in the repository.

### Owner decisions governing this spec

Thomas speaks through voice transcription. Short quotes are his transcribed words; each row gives the reading taken. The M and E rows were given on 2026-11-23 after the path-flow design of this spec, and they replace it. The S rows are earlier rulings still in force.

| # | Decision, as read | His words (transcribed) where short |
|---|-------------------|--------------------------------------|
| M-1 | Two things exist: what is said and who it is said to. A message has a name, content and tags. Tags filter and limit spread. A destination optionally limits a message to one part of the bus instead of broadcasting | "We have two things: what we say and who we say to." |
| M-2 | Everything that registers on the bus gives its name, based on its configuration path, and says what it is willing to receive and send. The sender is a mandatory tag. Events and commands are one thing | "Everything registering to the bus for message, like a peer, should give its name, which will be based on the path" |
| M-3 | A path is written in the short form. A peer may be named by its name or by its address | "Accept the short form."; "allow both" |
| M-4 | Default wiring comes from the code: plugins register under predictable names what they send and receive. Publications are registered before receptions, and a reception with no sender yet is valid and dormant. With nothing written, the registration is accepted as is | "By default we should otherwise accept what the plugin sends us as requested events" |
| M-5 | Configuration is an override written near the emitter: a `bus` block on the participant's own configuration. One YANG structure, defined once by the bus, is attached by augmentation to every node registered as a participant, and is inherited from group to peer. A participant's block goes where its configuration already is: the route server's under `bgp { route-server { } }` (`ze-rs-conf.yang` augments `/bgp:bgp`, verified by the main session); a plugin with no configuration of its own, such as the route reflector, uses its bus endpoint | "The to should be on the route server and route reflector plugins yang config" |
| M-6 | `to` says where this participant's messages go. A written list replaces the registered default entirely. There is no add form. Present and empty means nobody | "Yes, it replaces." |
| M-7 | `from` says who may address this participant. Absent means everybody. A member's list replaces its group's. Present and empty means nobody | "If not set, the from should be everybody. But it must exist to allow you to limit who you allow." |
| M-8 | Both `to` and `from` exist as a block-level list covering every message and as a per-message list that wins for that message | His "Option A" |
| M-9 | A send allow-list and a receive allow-list of message names also exist: absent means everything declared, empty means nothing, and there is no deny form. They work with `to` and `from`. The bus is the filter: enforcement is at the bus when it receives the message from a participant | "The bus should be a filter of what is allowed to be sent on the bus. Instead of limiting the message being generated, we can be limiting the message being received by the bus" |
| M-10 | There are two places to write: on the participant, and centrally in the `bus` section (his example: a third-party plugin Ze did not write, limited from the bus side). Both are compiled into one filter per participant. A disagreement between the two places is refused at commit, naming both lines. A destination written for a message the same participant's allow-list excludes is the same refusal (proposed by the main session; he did not object) | "The configuration should generate the filter as one structure and make sure there is no conflict between what is set up on the bus, the bus side, and the plugin side" |
| M-11 | The bus does not arbitrate between participants. No exclusive-role rule, no commit refusal and no silent winner in the bus. Two plugins answering the same message is acceptable. That a route server and a reflector do not both replay the same routes to one peer is the business of those plugins | "By default, both plugins should have an imply rule of to /bgp. And by default, the BGP should send to both because they will register on the broadcast."; "So I see no issue with the design." |
| M-12 | Converting today's `attach process` one to one is wrong: attach adds from silence, and `to` is exclusive. The converter writes nothing by default. Only configs and tests that prove a difference are restated, through the test judge | - |
| M-13 | Enforcement of the allow-lists covers every engine operation from the start, and the authorization-profile bypass by plugin identity is closed in this spec | - |
| E-1 | ExaBGP compatibility is a normal plugin, run as a normal program: a piece of code that transforms the ExaBGP script format to the native one. The `exabgp { bridge { } }` config section and the compiled-in internal bridge mode are removed. Each ExaBGP script is an ordinary bus endpoint whose `run` line is the existing external wrapper, and its scoping is the common `bus` block on that endpoint. This supersedes the earlier "the bridge keeps its `feed` blocks" and, where ExaBGP scoped a process per neighbor, "the migration writes no list" | "I thought ExaBGP will be a normal plugin, run as a normal application, and will not need a separate section on the configuration."; "Yes, that will remove the ExaBGP bridge. That's why I decided to have a bus design." |
| S-1 | The operator's bus configuration is the trust boundary. The API is the same for a compiled-in plugin and a program Ze did not ship, and no rule applies to one kind alone without a transport reason | - |
| S-2 | A compiled-in plugin runs in the daemon by default. Running it as a separate process is an operator option, and it keeps the same identity, state grants and defaults | - |
| S-3 | Offline validation starts an external program through its declaration (query mode), never its run loop. This reverses the earlier ruling that with no daemon Ze starts no external program; he confirmed the reversal is intended | - |
| S-4 | `ze start --plugin <program>` keeps working | - |
| S-5 | A route server serves every route server client connected to it (RFC 7947 Section 1); a reflector follows RFC 4456 | - |
| S-6 | ExaBGP: a script is passed only the neighbors that named it, `ze exabgp migrate` emits a full loadable config, and a script holds the whole ExaBGP command set. How it is expressed is E-1 | - |
| S-7 | Nothing implied is hidden: one display shows the registered defaults beside what the operator wrote. It shows the defaults a written list replaces, and which line produced each effective entry | - |
| S-8 | A feature's required relationship that an override removes is refused at commit with the line that completes it. AIGP being on by default implies nothing | - |
| S-9 | `ai/META-DECISIONS.md`, "CLI, config, and daemon surfaces": "What a plugin registers is not such a choice: a configured plugin receives and sends what it registered, shown as implied, and the operator writes an allow-list only to narrow it." | - |
| S-10 | `plugin { }` becomes `bus { }` and `hub` moves under it; the listener service `plugin-hub` becomes `bus-hub`. Repository configs are converted directly | - |
| Owned journal rows | The alias-keying row of 2026-11-14 and its five sibling defects, and the plugin identity that bypasses authorization profiles (2026-11-23) | "Owned journal rows" |

### The message model

| Term | Meaning | Who declares it |
|------|---------|-----------------|
| Participant | One party on the bus. It has one name, sends messages and receives messages | The owner of the configuration node: BGP for its peers and groups, the bus component for an endpoint, a plugin for its own config section |
| Name | The configuration path of the participant, in the short form: `/bgp/peer/edge1`, `/bgp/group/transit`, `/bgp/group/transit/peer/edge1`, `/bgp/route-server`, `/bus/endpoint/collector`. A peer is also named by its address, `/bgp/peer/192.0.2.1`: both spellings name the same participant (M-3) | Derived from the schema node the owner registered and the entry's key |
| Part of the bus | A shorter path. `/bgp/group/transit` covers the group and every peer under it; `/bgp` covers every BGP participant | Nothing to declare: a prefix of names |
| Message | A name in a namespace (`bgp:state`, `bgp:update-received`, `bgp:update`, `system-rib:route-install`, `"cli:show bgp peer"`), content, and tags. An event and a command are both messages | The participant kind that sends it; a command path is registered by the command component under `cli` |
| Tag | A property of one message that filters it and limits its spread. `sender` is mandatory and is set by the bus from the authenticated registration, never from content. The direction of a BGP event stays a tag in the index key, as today (`deliveryKey`) | The sender's kind declares which tags a message carries |
| Destination | Optional. Absent, the message is broadcast to every participant registered to receive it. Present, it reaches one participant or one part of the bus | The sender, per message; a command's destination is the participant that owns the command |

What a participant kind registers, through one API for a compiled-in plugin (`registry.Registration`, from `init()`) and a program Ze did not ship (the Stage 1 `declare-registration` message):

| Declaration | What it states |
|-------------|----------------|
| Participant node | The schema node whose configured entries are participants of this kind (a list or a container of the registrant's own module). A plugin that registers none is the participant `/bus/endpoint/<its endpoint name>`, and a plugin loaded with no endpoint written is `/bus/endpoint/<registry name>` |
| Sends | The message names an instance may send, with the tags each carries, and for an event its default destinations (broadcast unless stated) |
| Receives | The message names an instance is willing to receive, and from which part of the bus by default (everybody unless stated) |

| Rule | Meaning |
|------|---------|
| Sends before receptions | Compiled-in sends exist at `init()`. A declared external program's sends are read when `ze` starts, before the full parse. So the set of messages on the bus is complete before any reception is resolved, and the catalog reaches every plugin in its Stage 2 configure message, beside `Claims` |
| A dormant reception is valid | A registered reception whose sender is absent (a build without BGP, a plugin configured later) resolves to nothing, is not an error, and is shown as dormant |
| A process speaks only as itself | The bus stamps `sender` from the registration the connection authenticated as. A process names as sender only a participant it registered, as BGP does for its peers |
| Run-time participants | An in-daemon owner adds participants while it runs on the publication it already makes (BGP's `AddPeer`, `createDynamicPeer`, `completeRemovals`). A peer added later is covered by the registered wiring with no config change |
| No arbitration | The bus delivers to every participant the filters admit. It holds no exclusive-role rule and picks no winner (M-11) |

When a message passes from sender S to receiver D. All six are resolved when the index is built or the config is applied, never per message:

| # | Condition | Default with nothing written |
|---|-----------|------------------------------|
| 1 | S's kind declared sending the message | The registration |
| 2 | S's send allow-list admits the name | Everything declared |
| 3 | S's `to` for that message covers D | The registered destinations: every registered receiver for a broadcast, the owner for a command |
| 4 | D's kind declared receiving the message | The registration |
| 5 | D's receive allow-list admits the name | Everything declared |
| 6 | D's `from` for that message covers S | Everybody |

### The `bus` block on a participant

One YANG grouping, defined once by the bus module, is attached by augmentation as a container named `bus` to every schema node registered as a participant node. The set of nodes derives from the registrations: compiled-in ones before the schema is built, an external program's with its YANG at Stage 1 (joined at `ze start` before the full parse, and taken in query mode with no daemon). BGP's own YANG module names nothing of the bus, and no engine or config code names a feature.

| Field | Type | Meaning |
|-------|------|---------|
| `to` | list of paths, short form | Where every message this participant sends goes. Absent: the registered destinations. Present: the complete destination set, replacing the registered one; nothing is added to it (M-6). Present and empty: nobody |
| `from` | list of paths, short form | Who may address this participant, for every message. Absent: everybody. Present: those senders only. Present and empty: nobody (M-7) |
| `message <name>` | list keyed by message name, holding `to` and `from` | The same two lists for one message name. A per-message list wins over the block-level list for that message (M-8) |
| `send` | presence container holding leaf-list `allow` of message names | The send allow-list. Absent: everything the kind declared. Present: the listed names only. Present and empty: nothing. No deny form (M-9) |
| `receive` | presence container holding leaf-list `allow` of message names | The receive allow-list, same three states |

| Rule | Statement |
|------|-----------|
| Inheritance | A group's block applies to each peer under it. A list written on the member replaces the group's list of the same field and the same message whole; it is never merged with it. Whether a member's block replaces its group's block whole, or only the items the member writes, is open with Thomas (Q-8); this spec assumes only the items written. Either way the grouping needs a schema-level "replaces whole" marker, because `deepMergeAt` merges containers key by key and the only replace-whole mechanism today is the literal BGP map `replacePaths` |
| Precedence | For one message: the member's per-message list, then the member's block-level list, then the group's per-message list, then the group's block-level list, then the registered default (under the Q-8 assumption) |
| A path covers | The participant it names and every participant under it. A peer written by name and by address is one participant |
| Allow-list and lists together | A message must pass the allow-list and the `to`/`from` lists. A `to` or `from` written for a message name the same participant's allow-list excludes is refused at commit (M-10) |
| Commands | A command's destination is its owner, so `to` cannot widen it. A `to` that does not cover the owner means that command is not sent; the display says so |

### Two places to write, one filter

| Place | Written as | Use |
|-------|------------|-----|
| On the participant | `bus { ... }` inside its own configuration: a BGP group or peer, the route server's `bgp { route-server { } }` section | The override sits beside what it concerns |
| On the bus | The same fields directly inside `bus { endpoint <name> { ... } }` | The place for a plugin with no configuration of its own (the route reflector, a collector), and the bus side of any plugin: his example, a third-party program Ze did not write, kept from sending anything or limited in what it receives |

| Rule | Statement |
|------|-----------|
| One filter | For each participant the bus compiles the registration, the bus-side fields and the participant-side block into one filter. Delivery, the send check, the display and the doctor reconcile all read that one structure |
| Conflict | Where a plugin has both an endpoint entry and a config section of its own, the same field for the same message written in both places with different content is refused at commit. The message names both lines and says to keep one |
| One verdict | Every refusal below is the same at `ze config validate`, editor commit, reload and `ze start` |

Refusals at commit. Each names the line, the offending value and what is valid:

| # | Refused | The message lists |
|---|---------|-------------------|
| 1 | A `to` or `from` entry that names no participant and no part of the bus | The participant nodes that are registered and the configured entries of the list the path reaches |
| 2 | A `to` entry under which nobody registered to receive that message | Who registered to receive it |
| 3 | A `from` entry under which nobody declared sending that message | Who declared sending it |
| 4 | A `message` name the participant's kind did not declare, or an allow-list entry it did not declare | What the kind declared |
| 5 | A `to` or `from` written for a message the participant's own allow-list excludes | Both lines |
| 6 | The same field written in both places with different content | Both lines |
| 7 | An override that removes a relationship a configured feature requires | The feature, the participant and the completing line ("Feature requirements") |
| 8 | A commit that deletes an entry a `to` or `from` names | The line that names it, so the operator changes it first |

### The endpoint, the hub and process identity

`bus { endpoint <name> { ... } }` states one program on the bus. It replaces `plugin { internal }` and `plugin { external }`.

| Field | Type | Description |
|-------|------|-------------|
| `name` | string (key), `ze:validate "external-plugin-name"` | Endpoint name; the participant is `/bus/endpoint/<name>` |
| `internal` | string (`ze:validate "internal-plugin-name"`) | Registry name of a compiled-in plugin. Its identity is the compiled registration, whatever the endpoint is called. Mutually exclusive with `external` |
| `process` | enumeration `in-daemon` (default) or `separate`, valid with `internal` only | `separate` makes Ze fork its own binary for that plugin over the TLS connect-back. The operator writes no command line. Refused for a plugin whose registration sets `RunsInProcessOnly` |
| `external` | container (`run`, `respawn`, `timeout`) | A program Ze did not ship. Its identity is the endpoint name, and it takes nothing from a compiled registration of the same name |
| `encoding` | enumeration `json` or `text` (default `json`) | Wire encoding of events delivered to this endpoint (replaces `encoder`) |
| `to`, `from`, `message`, `send`, `receive` | The bus grouping | The bus-side filter of this plugin |

`bus { hub { server ...; client ...; } }` holds the TLS listener and managed-config client blocks moved from `plugin { hub }`.

### Enforcement at the bus: every engine operation

The filter is applied where the bus receives a message from a participant. For events that is the delivery index. For everything a plugin calls on the engine it is the shared dispatch path every operation already passes through, so the socket rail and the DirectBridge rail cannot differ.

| Rule | Design |
|------|--------|
| A field on the entry | Each engine operation states the message it carries, or `none` with a written reason. This covers the entries of `engineOps` (`internal/component/plugin/server/dispatch_registry.go`) and the operations their owners register through `RegisterEngineOp` (`register_engine_op.go`), which has no such field today |
| `none` is a statement | `cancel-request` cancels the caller's own request. The `state-*` operations are governed by state key grants. `report-issue` and `metrics-publish` act on the caller's own records. Each reason is on the entry |
| An undeclared entry is refused | A test walks both registries and fails on an entry with neither a message nor a reason. A new operation cannot ship ungated by omission |
| Commands by path | `dispatch-command` and `dispatch-command-args` carry the message `cli:<command path>`. The path, not the wire method, is the key. `cli` is the namespace the command component registers, with the prefix matcher `authz` uses for profiles; it is no special case in the bus |
| Receiving is not sending | `subscribe-events` and `unsubscribe-events` are bounded by the caller's registered receptions and its receive filter |
| Emitting | `emit-event` is refused for a message the caller did not register as a send |

Every operation and the check it makes today on its caller, read at the handler on 2026-11-23 by the agent that took the census. The last column is this design's.

| Operation | Handler (file) | Check on the caller today | Message it carries, or `none` and why |
|-----------|----------------|---------------------------|----------------------------------------|
| `state-get`, `state-put`, `state-remove`, `state-list`, `state-increment` | `opState*` (`plugin/server/dispatch_state.go`) | State key grants (`statestore.PluginKeyReadable`, `PluginKeyWritable`, `PluginList`), keyed on `proc.Name()`, the operator's alias | `none`: governed by state key grants, keyed on the registration identity after this work |
| `metrics-publish` | `opMetricsPublish` (`dispatch_metrics.go`) | None; acts on the metric set owned by `proc.Name()` | `none`: the caller's own records |
| `report-issue` | `opReportIssue` (`dispatch_report.go`) | None; reports under `proc.Name()`, rate-limited per owner | `none`: the caller's own records |
| `update-route` | `opUpdateRoute` (`dispatch_registry.go`), `handleUpdateRouteSelResultDirect` (`dispatch.go`) | `RequirePeerDeclarations`, then the command runs under `internalPluginIdentity(proc.Name())`, which `Store.authorize` allows by prefix; the BGP rails then apply the per-peer send grant (`filterPermittedPeers`) | `bgp:update` (Q-T4: also the `cli` path of its command text) |
| `dispatch-command` | `opDispatchCommand`, `dispatchCommandPeerResponse` (`dispatch.go`) | The prefix bypass: `internalPluginIdentity(proc.Name())`, allowed before any profile | `cli:<path>` |
| `dispatch-command-args` | `opDispatchCommandArgs`, `dispatchCommandArgsResponse` (`dispatch.go`) | `RequirePeerDeclarations`, then the prefix bypass | `cli:<path>` |
| `subscribe-events`, `unsubscribe-events` | `opSubscribeEvents`, `opUnsubscribeEvents` (`dispatch_registry.go`) | None at the operation; delivery stays bounded by the peer's grant (`PeerScopedProcs`) | The named message, judged as a reception |
| `emit-event` | `opEmitEvent` (`dispatch_registry.go`) | None; no check that the emitter owns the namespace | The emitted message, judged as a send |
| `forward-cached` | `opForwardCached` (`dispatch_cached.go`) | `RequirePeerDeclarations`, then the per-peer send grant on the forward rails | `bgp:update` |
| `release-cached` | `opReleaseCached` (`dispatch_cached.go`) | None; releases cache entries under `proc.Name()` | `none`: the caller's own cache holds |
| `relay-stored-route` | `opRelayStoredRoute` (`dispatch_cached.go`) | `RequirePeerDeclarations`, then the per-peer send grant on the relay rail | `bgp:update` |
| `select-routes` | `opSelectRoutes` (`dispatch_selected.go`) | The caller's `SelectedOwner`, the per-peer send grant, and for cancellation the destination's `update` grant (`selectedCancellationScope`) | `bgp:update` |
| `cancel-request` | `opCancelRequest` (`request_cancel.go`) | Cancels the caller's own request | `none`: the caller's own request |
| `select-refresh` | `opSelectRefresh` (`dispatch_selected_refresh.go`) | The caller's `SelectedOwner`, and `DeliveryGraph.MaySend(destination, proc.Name(), "update")` | `bgp:update` |
| `route-install`, `route-remove` | `opRouteInstall`, `opRouteRemove` (`dispatch_route.go`) | None | `system-rib:route-install` |
| `route-metrics` | `opRouteMetrics` (`dispatch_route_metrics.go`) | None | `system-rib:route-metrics` |
| `inject-wire-route` | `opInjectWireRoute` (`dispatch_registry.go`) | `RequirePeerDeclarations` only | `bgp:raw` |
| `resolve-dns` | `opResolveDNS` (`dispatch_resolve.go`) | None | A message the resolver registers (name owed at implementation) |
| `batch-validate` | `opBatchValidate`, `dispatchBatchValidation` (`dispatch_batch_validate.go`) | Dispatches `rpc.BatchValidateCommand` through `dispatchCommandArgs`, so the prefix bypass | `cli:<path of that command>` (Q-T3) |
| `ze-bgp:source-lease` (owner-registered) | `sourceLeaseJSON` (`bgp/reactor/register_source_lease.go`) | Bound to the caller's `SelectedOwner` | Owed at implementation (Q-T1) |
| `ze-firewall:tables-replace` (owner-registered) | `tablesReplaceHandler` (`firewall/register_contribution.go`) | A table reservation keyed on `proc.Name()` | Owed at implementation (Q-T1, Q-T2) |
| `ze-sysrib:route-view-open`, `route-view-read`, `route-view-close`, `route-view-candidates`, `route-view-distances`, `route-view-publish` (owner-registered) | `sysrib/register_route_view*.go` | `routeViewAuthorize` on open: the caller's `RegistrationName()` must be the system RIB; the others are keyed on the caller process and were not read further | Owed at implementation (Q-T1) |
| `ze-iface:owned-bridges-replace`, `owned-link-create`, `owned-link-remove` (owner-registered) | `iface/cmd/owned_devices.go` | `iface.RefuseUngrantedDeviceOwner(proc.Name())`, keyed on the operator's alias | Owed at implementation (Q-T1, Q-T2) |

How the authorization bypass closes. Today `dispatchCommandPeerResponse` and `dispatchCommandArgsResponse` set the command's username to `internalPluginIdentity(proc.Name())`, which carries `aaa.ReservedInternalPrefix`, and `Store.authorize` (`internal/component/authz/authz.go`) returns `Allow` for that prefix before it looks at any profile.

| Option | What it does | Verdict |
|--------|--------------|---------|
| (a) A profile for the plugin identity | The participant's filter for `cli` messages (declared, cut by its send allow-list and its `to`) is compiled into an ordinary authorization profile bound to the plugin identity. `Store.authorize` stops allowing the `plugin:` identities by prefix and evaluates that profile with the matcher it uses for every user. Accounting and audit keep the identity | Chosen: one authorizer, one matcher, one audit trail |
| (b) A second gate in `plugin/server` before the dispatcher | The plugin server checks the path, then dispatches under the reserved identity as today | Set aside: two authorizers for one command, and the prefix bypass stays in `authz` for any path that forgets the first gate |
| (c) Keep the bypass for compiled-in plugins | Only programs Ze did not ship are checked | Set aside: S-1 rules out a trust difference |

`internalRPCIdentity`, the reserved identity of an in-process RPC call that carries no plugin, stays allowed, with its reason restated on the constant: it names no participant, so no filter exists to check it against. Each of its callers is listed at implementation.

One refusal, one counter, one doctor code:

| Existing mechanism | Where | Here |
|--------------------|-------|------|
| Request refusal event | `plugin/server/dispatch_report.go` (`refusePluginRequest`, `diagnostic.CodeDoctorPluginRequestRefused`) | Extends: a message refused by the filter raises it, with the participant, the message and the fix |
| Send refusal counter | `bgp/reactor/send_permission.go` (`recordSendRefused`, metric `ze_bgp_send_refused_total`) | Replaces: one counter in the plugin server for every operation, labels participant and message. The BGP counter is deleted, its page updated |
| Send refusal text | `send_permission.go` (`sendPermissionDenied`, `errSendNotPermitted`) | Extends: one text shape for every operation, saying which reason applies |

| Reason a message is refused | Message the operator reads |
|-----------------------------|----------------------------|
| The participant's kind did not declare it | Names the participant and the message, says its registration does not declare it, and gives the command that shows what it does declare. No config line fixes this |
| An allow-list removed it | Names the participant and the list, and gives the exact line that adds the entry |
| A `to` or `from` list does not cover the other end | Names both participants and the line that holds the list |

### The display: nothing implied is hidden

One pipe operation, `| implied`, shows every registered default beside what the operator wrote, with what produced it. Every implied kind comes from a registration; none is computed from a name in engine or config code. The view reads the one compiled filter per participant, so it cannot differ from what runs.

| Shown item | Produced by | What the view prints |
|------------|-------------|----------------------|
| An endpoint Ze loads with no `endpoint` written | The plugin's `ConfigRoots`, `Families`, `Dependencies`, `ConfigNeeds`, `DataPlane` | The endpoint, with the config path, family or dependent plugin that implies it |
| A participant's effective `to` and `from`, per message | The registration, a group's block, the participant's block, or the bus side | Each effective entry with the line that produced it: written here, inherited (naming the group), registered, required, or dormant |
| The defaults a written list replaces | The registration | Beside a written `to`, the registered destinations it replaced, so the operator sees what the list removed |
| The effective send and receive allow-lists | The registration and the two places | The declared names, and which a written list keeps. A present and empty list is shown as written, distinct from an absent one |
| A filter member a plugin inserted in a peer's filter list | The plugin's `FilterInsertions` | The config path that meets the insertion's condition |
| A delivery a feature requires | The feature's declared requirement | Marked required, with the feature |

| Surface | Form |
|---------|------|
| Pipe, bare, no argument | `show config running \| implied`, `show config source <file> \| implied`, `show config dump <file> \| implied`, editor `show \| implied` and `show \| implied \| format config`. Written and implied items together; standard pipes such as `\| match` filter them |
| Text | A comment `# implied by: <reason>` above each implied item: a single hash followed by a space, never `##` at column 0, which `config.DetectFormat` (`internal/component/config/serialize_set.go`) reads as a set-meta line. An implied `to` is printed as the `bus` block that would state it, in the short path form, so an operator can copy it and change it |
| JSON and YAML | An envelope with the written config under `config` and the implied items under `implied`, each with `implied-by`. The written config stays byte-pure |
| Plain-display notice | When implied items exist, a human display states the count and names `\| implied`: the editor status bar, the `\| text` footer, `stderr` of `ze config show`. Never in `\| json`, `\| yaml`, REST, gRPC or `ze config show` stdout |
| Web | The web CLI and the graphical inspection offer `show \| implied`. Workbench tree pages show implied rows read-only, with a badge |
| gNMI | `Get` returns the written config only, so `Get` then `Set` round-trips |
| The whole bus | `show event delivery` lists every namespace, each edge once with its origin (`written`, `inherited`, `registered`, `required`, `dormant`), naming both participants by path; `ze config graph` draws the same edges. `required` replaces `ProcessEdges.Implicit` |
| No daemon, external endpoint | The view takes the program's declaration in query mode. When the program cannot be asked, the view refuses, names the endpoint, its `run` line and the cause, and gives `ze show plugin conformance run "<run line>"`. It never shows a partial set as complete |

### Feature requirements

Today BGP config code names plugins to decide what a feature needs: `requireRIBDelivery` (`internal/component/bgp/config/flowspec_binding.go`) holds the literals `bgp-rs`, `bgp-rr` and `bgp-adj-rib-in`, and `wireRedistributeDelivery` and `requireSelectionFeeds` (`redistribute_binding.go`) reach the Loc-RIB and the orchestrator through named constants.

| Step | Design |
|------|--------|
| The declaration | A plugin declares, on its registration, each requirement a feature's config places on it: the condition (a config path or a session fact, in the form `FilterInsertions` conditions already use) and the messages that condition needs to pass between which participants. It generalizes `rpc.DisseminationDecl` |
| The check | One function compares every active requirement with the compiled filters, at validate, commit, reload and start, with one verdict |
| The refusal | Where a `to`, a `from` or an allow-list removes something an active requirement needs, the commit is refused. The message names the feature, the participant, the line that removes it and the line that completes it, as a typed `FixCommand`. Ze never widens what was written and never loads it to do nothing |
| What it replaces | The literals in `requireRIBDelivery`, the constants behind `wireRedistributeDelivery` and `requireSelectionFeeds`, the derived bindings (`grantEveryPeer`, `EnsureProcessBinding`), and the per-role check of `ValidateDisseminationBindings` with its `disseminationAttachLine` |
| Redistribution | Gains the refusal it lacks today: a rule whose Loc-RIB or orchestrator is cut below what the rule needs is refused, where today it loads and moves no route with one log line |
| The filter owner's `state` | The owner of a filter member requires the peer's `state`. The requirement declaration states it, in place of `grantFilterOwnersState` naming it in BGP config code. An override that removes it is refused like any other requirement, where today the grant silently widens the operator's block (`GrantImplicitReceive`) |
| AIGP | `PeerSettings.AIGPEnabled` (`bgp/reactor/config_aigp.go`) answers true for every iBGP peer with no `aigp` configured (RFC 7311 Section 3.3). A capability that is merely on by default implies nothing (S-8): only a peer with `aigp` written activates the requirement. The session default, and what Ze sends, accepts and compares, are unchanged. A test on the default configuration proves a route carrying AIGP is still compared by AIGP |
| Dynamic groups | The collectors in `bgp/config/loader_create.go` that skip dynamic groups for family, event and send auto-load are fixed, so an implied endpoint a dynamic group needs is loaded and shown |

### No arbitration: route server and reflector

Under M-11 the bus picks no winner, so two plugins that both forward must not duplicate each other by default. Read at the producers on 2026-11-23:

| Producer | What it does today | Under the default wiring |
|----------|--------------------|--------------------------|
| `peerStateUnheldRoles` (`bgp/server/events.go`) | The role `bgp-peer-up-replay` is held for a peer when a fed claimant may send `update` to it (`graph.MaySend`) | Same question, answered by the compiled filter. Held by both claimants when both are loaded |
| `rsRelaySendProcessName` (`bgp/reactor/peer_forward_facts.go`) | The first process in the peer's bindings that may send `update` and whose plugin declares `registry.RelaysOnRSFastPath` | Reads the filter. Only `bgp-rs` declares the fast path |
| `Peer.policyReplayOwner` (`bgp/reactor/policy_reload.go`) | The first binding that may send `update`, receives the peer's state and claims `bgp-peer-up-replay`; else the store that owns the replay command | Reads the filter. With two claimants on one peer it returns the first in order: a silent winner |
| `Peer.initialUpdateReporters` (`bgp/reactor/peer_run.go`) | Every process with a route-push rail on the peer, the peer's state and `SignalsSessionReady` | Every such plugin, on every peer it serves |
| `routeServer.selectForwardTargets` (`bgp/plugins/rs/server_forward.go`), `PeerState.updateTarget` (`rs/peer.go`) | Targets every peer the route server knows that is up and not send-denied (`Up && !updateSendDenied`). No session-type condition in the selection | Every peer it is fed |
| `bgp-rs` and `bgp-rr` registrations (`rs/register.go`, `rr/register.go`) | Both claim `bgp-peer-up-replay` and both set `SignalsSessionReady` and `FencesLiveForwards` | Both are fed every peer and may send to every peer |

Finding: with `bgp-rs` and `bgp-rr` both loaded and nothing written, both are fed every peer and both hold `update` toward every peer, so both would forward every received route to every other peer and both would replay at peer-up, and one peer would receive the same routes twice. Today the per-peer `attach process` blocks keep them apart. This is a defect of those two plugins under the new default, fixed in this work: each plugin limits itself by the session it is for. The route server serves the peers of its client set (`ze-rs-conf.yang`: "A peer joins the client set with the rs-client leaf of its session", verified by the main session; RFC 7947 Section 1), and the reflector serves internal peers (RFC 4456 Section 5). `policyReplayOwner` and the role retraction then find one claimant per peer because only one serves it, with no arbitration in the bus. Not read: whether a peer outside the client set ever enters the route server's peer table, and the reflector's own target selection (assumption A-10). The discriminator is put to Thomas as Q-5.

| Role | The RFC's words | What follows |
|------|-----------------|--------------|
| Route server | RFC 7947 Section 1: "Each multilateral interconnection participant (usually referred to as a "route server client") announces network reachability information to the route server using EBGP. The route server, in turn, forwards this information to each route server client connected to it, according to its configuration." | The route server serves every route server client connected to it. A difference between clients is export policy in the peer's filter list |
| Route reflector | RFC 4456 Section 5: "The internal peers of an RR are divided into two groups: 1) Client peers 2) Non-Client peers". Section 6: a route from a non-client is reflected "to all the Clients", and a route from a client "to all the Non-Client peers and also to the Client peers" | Client or non-client is BGP configuration of the peer. It changes the reflection rule, never whether the plugin may talk to the peer |

An operator who wants a forwarder kept to some peers writes a `to` on it (`bgp { route-server { bus { to [ /bgp/group/ix ]; } } }`), or a `from` on the peers.

### ExaBGP

Owner decision E-1 arrived after the rest of this design was written. This section records it and what is still owed; it is NOT settled design, and the spec does not go to the judge before the owed rows are answered.

| Point | Today | Under E-1 |
|-------|-------|-----------|
| How a script runs | Two runners: the compiled-in internal bridge (`bridgeplugin`, `bridgerun`, config root `exabgp { bridge { process ... } }`), and the external wrapper `ze exabgp plugin <cmd>`, an SDK process over TLS connect-back | One: each ExaBGP script is `bus { endpoint <name> { external { run "ze exabgp plugin <cmd>"; } } }`, one script per process. The internal bridge mode and its config section (`ze-exabgp-bridge-conf.yang`) are deleted, with no layering |
| Which neighbors feed a script | `process <name> { feed <address> { event [ ... ]; } }`, read by `parseFeeds` into `grantSet` | `from` on the script's endpoint: the neighbors that named it. Its message kinds are the endpoint's receive allow-list, or per-message `from` lists where they differ per neighbor |
| Where a command with no neighbor selector goes | `feedPeers` in the translator: the peers that feed the script, even one that feeds it no event (ExaBGP's `Reactor.peers(service)` reads `api['processes']` alone) | `to` on the script's endpoint: the neighbors that named it |
| A neighbor that names a script and no message kind | A `feed` block with an empty `event` list | The neighbor is in `to` and in no `from` |
| `encoder`, and the form words `parsed`, `packets`, `consolidate` per neighbor | The process's `encoder` leaf and entries of the `feed` block's `event` list (`bridge.FormsFor`) | Options of the wrapper. Their form, per-neighbor choices included, is owed design |
| The migration | Writes the bridge root, its `process` and `feed` blocks, and `attach process` | Writes one endpoint per ExaBGP process with its `from` and `to` lines. A process no neighbor scoped gets no block |
| `create bgp peer` | The bridge appends `attach exabgp-bridge` (`convertNeighborCreate`) | Loses the `attach` keyword: a peer created at run time is covered by the registered wiring |
| The command set | The bridge relays each script line through `DispatchCommand` | The wrapper declares every command path its translator can emit, derived from the translator's own table, so a script holds the whole ExaBGP command set and an allow-list on its endpoint narrows that script alone |

Owed before this section is settled. Each row is answered as "carried by the wrapper" or becomes a question for Thomas, after reading `docs/architecture/exabgp-bridge.md` and the `bridgeplugin` and `bridgerun` code, which this pass did not read beyond `parseFeeds`, `feedPeers`, `script.forms`, `Fleet.New` and the registration:

| # | What the internal mode may provide that one wrapper per script cannot | Seen so far |
|---|-----------------------------------------------------------------------|-------------|
| X-1 | State shared across scripts | `bridgerun.New` builds one `bridge.NewOwnerLedger()` and one `apiSetting` for the whole fleet and hands them to every script |
| X-2 | Withdrawal of a script's routes when it exits | Not read |
| X-3 | Holding the ExaBGP API version a script speaks | `apiSetting` is fleet-wide; the `encoder` description speaks of a script "that has written neither yet". Not read further |
| X-4 | The fork rate limit | `script.start` counts each fork against `s.limiter`; the endpoint's `respawn` leaf is the candidate successor |
| X-5 | The cost of one process and one TLS connection per script, with many scripts | Not measured |
| X-6 | The census of configs, tests and docs that use the `exabgp { bridge }` section | Not taken |
| X-7 | Whether `ze exabgp plugin <cmd>` already carries everything the internal runner does, per `exabgp-bridge.md` "The internal runner cannot read the `run` line" | Not read |

### Offline validation asks the program

| Point | Statement |
|-------|-----------|
| Today | With no daemon running Ze starts no external program (owner ruling on `plugin-system.md`). `ze config validate` holds the compiled-in schema alone, asks a running Ze first, and refuses a section only an external module defines (`DeclaredSchemas.ParseRefusal`; the comment in `runValidationUnanswered`, `config/cli/cmd_validate.go`) |
| The mechanism that exists | Query mode (`process-protocol.md`, "Query Mode: Stage 1 and No Stage 2"): a program started with `ZE_PLUGIN_MODE=declare` writes its Stage 1 `declare-registration` line to stdout and exits 0. No connection, no Stage 2, no run loop. `sdk.RunOrDeclare` writes the line and never calls the plugin's start function. `show plugin declarations` and the conformance check already run a program this way |
| After | A command that parses a config with no daemon running starts each declared external program in query mode, takes its declaration (YANG modules, participant node, sends, receptions), joins the modules through the same join the daemon uses, and validates the whole config against it |
| What it extends | `config.SchemaWithDeclaredPlugins` and `PublishDeclaredSchemas`. The route that asks a running Ze first stays |
| When the program cannot be asked | The validation fails. The message names the endpoint and its `run` line, says which of four things happened (not found, exited non-zero, wrote no declaration line, declaration refused at Stage 1 with that refusal), says which sections and lists were not checked, and gives `ze show plugin conformance run "<run line>"` |
| What it never does | Accept a config it could not check, or start a run loop |

### How delivery is indexed

| Question | Answer |
|----------|--------|
| When is the filter applied? | When the index is built: after every participant change (`AddPeer`, `createDynamicPeer`, `completeRemovals`) and at the end of a config apply, where it is built today |
| What does one broadcast cost? | One map read on the sender's key and one on the message key (`deliveryKey`), yielding a stored receiver slice, with no allocation, as today. Every `to`, `from` and allow-list is resolved into those slices at build, by prefix match on parsed path steps, never per message |
| What does an addressed message cost? | One read in a path table (names, second spellings such as a peer's address, and every prefix with a participant under it). The typed SDK keeps a handle |
| What does the send check cost? | One read of table[sender][message], answering all or a participant set, tested into caller scratch as `filterPermittedPeers` does today |
| What about a tag that differs per message? | Small closed sets go in the key, as direction does. Other tags are compared as interned ids, only for a reception that names them |
| What proves it? | Benchmarks of a broadcast, an addressed message and the send check report allocations per operation, and the test fails above zero |
| Does the bus learn a BGP word? | No. Participant nodes and messages are registered by their owners, and a path is resolved against the schema |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform boundaries, process boundary, registration over enumeration
  -> Constraint: `internal/component/plugin/` never imports `internal/component/bgp/`; the bus learns no BGP word, and the `bus` grouping is attached to BGP's nodes by augmentation from the bus module
- [ ] `ai/doors/config-yang.md` - YANG schema loading, first-pass roots, `validatedSections`, `ze:validate`, config text manipulation rules
  -> Constraint: "Extend a container with `augment`, never with a second definition." Config text changes go through a parsed `*config.Tree`. The door's rule that with no daemon Ze starts no external program is departed from by S-3 and corrected with `plugin-system.md`
- [ ] `ai/doors/plugin-infra.md` - `plugin/registry`, `plugin/server`, `plugin/process`, `HoldDeclaredPlugins`, Stage 1 checks, `engineOps`
  -> Constraint: "A compiled-in and an external plugin differ only in how they reach the bus." and "One engine-op entry per RPC."
- [ ] `ai/doors/commands-cli.md` - the command registry, `command.AnswerSurface`, the dispatcher
  -> Constraint: a command is declared once and reaches every interface; `| implied` is one pipe operation, registered once, offered by every surface
- [ ] `ai/doors/access-security.md` - `authz` profiles, the audit log
  -> Constraint: command authorization reads its policy from one accepted generation; the plugin identity gets a profile there, not a second authorizer
- [ ] `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md` - reactor, `PeerSettings`, `DeliveryGraph`, the forwarding plugins
  -> Constraint: BGP registers its peer and group lists as participant nodes; "A role one plugin takes from another is a claim, not an import"
- [ ] `ai/doors/core.md` - `internal/core/events`, `internal/core/statestore`
  -> Constraint: `internal/core/` never imports upward
- [ ] `ai/doors/hub-engine.md` - `cmd/ze/hub` startup, `FirstPass`, managed server and client
  -> Constraint: the first-pass roots switch from `plugin` to `bus` before the full parse
- [ ] `ai/doors/system-plugins.md` - `internal/exabgp/migration`, `internal/exabgp/bridge`, `internal/plugins/ddos/flowspec`
  -> Constraint: ExaBGP 6.0, 5.0 and 4.2 compatibility is agreed: per neighbor and per process, the events, the form words and the command scope all keep their effect, whatever carries them after E-1
- [ ] `docs/architecture/api/architecture.md` - "Attaching a Process to a Peer", both owner rulings, "What the Plugin Declares and What the Peer Grants"
  -> Decision: both owner rulings on that page are departed from by M-4 to M-10; the page is corrected in the same work
- [ ] `docs/architecture/api/process-protocol.md` - Stage 1, query mode, event delivery, persistent runtime state
  -> Constraint: Stage 1 gains the participant declarations; Ze is pre-release, so the protocol changes directly and every plugin in the tree is rebuilt with it
- [ ] `internal/component/bgp/config/resolve.go` (`ResolveBGPTree`, `deepMergeAt`, `replacePaths`), read by the agent that wrote the design draft
  -> Constraint: group to peer inheritance merges containers key by key; a member's list replaces its group's only through a replace-whole marker, which must come from the schema, not a BGP literal

**Key insights:**
- A plugin already declares what it receives, in its ready RPC (`sdk.Plugin.SetStartupSubscriptions` and its two siblings), and today the peer's `attach process` block decides what it gets. M-4 makes that declaration the default instead of the upper bound, and moving it to the registration lets an offline reader show it.
- `ProcessBinding.Encoding` and `ProcessBinding.Format` (`attach > content`) are read on no delivery path. Deleting `content` loses no behavior.
- The per-peer send grant chose which plugin does a job (`policyReplayOwner`, `rsRelaySendProcessName`, `initialUpdateReporters`, the replay role). Under M-11 the bus does not choose; each forwarding plugin limits itself by session type.
- `Store.authorize` allows `aaa.ReservedInternalPrefix` before any profile lookup (`authz/authz.go`), and `internalPluginIdentity` (`plugin/server/dispatch.go`) stamps it on every plugin process, a forked program included.
- `test/plugin/external-yang-command-line.ci` and `TestFirstPassMergesCommandLinePlugins` prove that `ze --plugin "<program>"` starts an external program before the full parse.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/yang/ze-plugin-conf.yang` - `container plugin` with `list internal` (`use`), `list external` (`run`, `use`, `encoder`, `respawn`, `timeout`), `container hub`
- [ ] `internal/component/plugin/types.go` - `PluginConfig.RunsExternalProgram`: an `external` block whose `run` is a command line, which takes nothing from a same-named registry row
- [ ] `internal/component/plugin/resolve.go` - `RegisterDefaultEventNamespace`, `RegisterPluginEventTypes`, `RegisterPluginSendTypes` and their two `sync.Once`; `deriveName`
- [ ] `internal/component/plugin/registry/registry.go` - `Registration.EventTypes`, `SendTypes`, `Claims`, `Dissemination`, `RunsInProcessOnly`
- [ ] `pkg/plugin/rpc/types.go` - `DeclareRegistrationInput` carries no event, reception or send declaration
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `SetStartupSubscriptions`, `SetStartupSubscriptionsIn`, `AddStartupSubscriptionsIn`: the reception a plugin declares, carried by the ready RPC
- [ ] `internal/core/events/events.go` - `RegisterNamespace`, `RegisterEventType`, the flat `ValidSendTypes` and `RegisterSendType`
- [ ] `internal/component/config/first_pass.go`, `yang/join.go` (`firstPassRoots`, `FirstPassRoots`) - the first pass reads `plugin` and `environment`, and merges `--plugin`
- [ ] `internal/component/config/validate_sections.go` - `validatedSections` covers `plugin`, excludes `bgp`
- [ ] `internal/component/bgp/config/plugins.go` - `extractBGPInlinePlugins`
- [ ] `internal/component/bgp/reactor/config.go` - `parseProcessBindingsFromTree`, `EnsureProcessBinding`, `GrantImplicitReceive`
- [ ] `internal/component/plugin/server/delivery_graph.go` - `DeliveryGraph` keyed by peer address, `deliveryKey{ns, et, dir}`, `MaySend`
- [ ] `internal/component/plugin/server/dispatch_state.go`, `internal/core/statestore/permissions.go` - state grants checked against `proc.Name()`, the operator's alias
- [ ] `internal/component/bgp/server/events.go` - `peerStateUnheldRoles`, `externalIdentity`
- [ ] `internal/component/bgp/reactor/reactor_api_forward.go` - `filterPermittedPeers` drops each matching peer the sender holds no grant on
- [ ] `internal/component/bgp/plugins/rs/peer.go` (`updateTarget`), `rs/register.go`, `internal/component/bgp/plugins/rr/register.go` - both plugins claim `bgp-peer-up-replay`; the route server's target test is `Up && !updateSendDenied`
- [ ] `internal/exabgp/migration/migrate.go`, `internal/exabgp/bridge/bridge_neighbor.go`, `internal/plugins/exabgp/bridgerun/script.go`, `bridgerun/fleet.go`, `bridgeplugin/config.go`, `bridgeplugin/register.go`, `bridgeplugin/yang/ze-exabgp-bridge-conf.yang` - the migration writes `attach process` and `feed` blocks, the bridge scopes a script by `feedPeers`, and `create neighbor` emits `attach exabgp-bridge`
- [ ] `internal/component/bgp/plugins/cmd/peer/create.go`, `yang/ze-peer-cmd.yang`, `internal/component/bgp/yang/ze-bgp-conf.yang` - `setAttach`, `container attach`
- [ ] `internal/component/config/yang/validator_reference.go`, `modules/ze-types.yang`, `yang/publish.go`, `internal/component/config/cli/cmd_validate.go` - the path parser, `node-name`, `PublishExternalModules`, `SchemaWithDeclaredPlugins` in `runValidationUnanswered`
- [ ] `internal/component/plugin/process/process.go` (`RegistrationName`), `internal/component/plugin/server/request_cancel.go` (`selectedCancellationScope`), `internal/component/plugin/doctor/check_external.go`, `internal/plugins/ddos/flowspec/doctor.go`, `internal/component/config/storage/cli/data_rpc.go`, `internal/component/managed/client.go`, `internal/component/managed/doctor.go`, `cmd/ze/hub/main.go` (`ExtractHubConfig`), `cmd/ze/ze_core_start.go`, `cmd/ze/ze_core_dispatch.go`, `internal/core/ipc/yang/ze-plugin-engine.yang`

**Behavior to preserve:**
- Every plugin runs in the daemon or as a separate process over the five-stage protocol, and the SDK hides which.
- `ze --plugin ze.<name> -` and `ze --plugin "<program>" -` both keep starting the plugin they name (`test/plugin/external-yang-command-line.ci`).
- Redistribution, FlowSpec authorization, AIGP selection and the filter owner's `state` reception keep working with nothing written.
- A participant never receives a message it registered no reception for, and never sends a message it did not declare.
- An ExaBGP script is written only the neighbors that named it, and its commands reach those neighbors alone.
- Offline `ze bgp decode --plugin` and `ze plugin test --plugin` keep accepting external decoders.
- Delivery of a peer-scoped event stays one index read with no allocation.
- Operators on every command surface are authorized exactly as today.

**Behavior to change:**
- Delete `container plugin` from `ze-plugin-conf.yang` and `container attach` from `ze-bgp-conf.yang`. Add `container bus { list endpoint; container hub; }` and the `bus` grouping augmented onto every participant node. No hybrid.
- Delete `extractBGPInlinePlugins`, `attach > content`, `ProcessBinding` and its derived-binding writers (`EnsureProcessBinding`, `GrantImplicitReceive`, `grantEveryPeer`), `GetPeerProcessBindings`, and the `attach` keyword of `create bgp peer`.
- Add the participant declarations to `registry.Registration` and to Stage 1. Delete `Registration.EventTypes`, `Registration.SendTypes`, `RegisterPluginEventTypes`, `RegisterPluginSendTypes`, both `sync.Once`, the default event namespace and the three subscription calls.
- Silence is no longer the default: a configured participant receives and sends what it registered until the operator overrides it.
- The filter is enforced on every engine operation, and a plugin identity is authorized by a profile, never by prefix.
- A route server serves route server clients and a reflector serves internal peers, so both can be loaded with nothing written.
- A command that parses a config with no daemon running starts each declared external program in query mode.
- A compiled-in plugin run as a separate process keeps its registration's identity. `external <n> { run "ze plugin <x>" }` is converted to `internal <x>; process separate;`.
- Delete the `exabgp { bridge }` config section and the internal bridge mode; each ExaBGP script is an endpoint that runs the wrapper (E-1, design owed).
- Move `hub` under `bus` (`bus-hub` listener service).
- Add the `| implied` display and the edge origins on `show event delivery`.
- Fix the owned journal rows and the five sibling alias-keying defects.
- Convert every repository `.conf`, `.ci`, `.et`, interop scenario, fixture, Go test, doc and website example through a parsed-tree converter in the same work.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Program and hub declaration | `internal/component/plugin/yang/ze-plugin-conf.yang` (`container plugin`); `internal/component/config/loader.go` (`ExtractPluginsFromTree`); `loader_extract.go` (`ExtractHubConfig`) | Declares `internal`, `external` and `hub` under `plugin { }` | Replaces: one `endpoint` list and `hub` under `bus` |
| Internal `use` and external `run` shapes | `ze-plugin-conf.yang` (`internal > use`, `external > run`, `external > use`) | `use` names a registry row and runs it in the daemon. `run` forks a command line. `run "ze plugin <x>"` forks and is named `<x>` | Replaces: `internal <x>` names the registration, `process` chooses the transport, `external { run }` is always a foreign program |
| Foreign-program rule | `internal/component/plugin/types.go` (`PluginConfig.RunsExternalProgram`) and its callers | Answers both "is this registration the block's" and "does it run in another process" with one predicate | Replaces: the predicate splits into identity and transport. S-2 needs a compiled identity on a separate process |
| In-process-only declaration | `registry.Registration.RunsInProcessOnly`; `internal/component/plugin/doctor/check_external.go` | A plugin that works only in the daemon says so, and doctor reports a block that forks it | Extends: `process separate` on such a plugin is refused at commit with the same reason. A transport reason |
| Reception declared by the plugin | `pkg/plugin/sdk/sdk_callbacks.go` (`SetStartupSubscriptions`, `SetStartupSubscriptionsIn`, `AddStartupSubscriptionsIn`); ready RPC | The plugin states the events, peers and format it handles. Delivery is the overlap with the peer's grant | Replaces: the events move to the `Receives` declaration, which the SDK reads to subscribe. The format, and a narrower per-instance reception, stay ready-time parameters |
| Event and send types a plugin produces | `registry.Registration.EventTypes`, `SendTypes`; `internal/component/plugin/resolve.go` (`RegisterPluginEventTypes`, `RegisterPluginSendTypes`); `internal/core/events/events.go` (`ValidSendTypes`, `RegisterSendType`) | Type names registered lazily, once, into the default namespace and a flat global send set | Deleted: replaced by the `Sends` declaration, which names its namespace and registers at `init()` |
| Default event namespace | `resolve.go` (`RegisterDefaultEventNamespace`, `DefaultEventNamespace`) | The namespace a namespace-less declaration resolves to (`bgp` today) | Deleted: every message names its namespace |
| Event namespace registry | `internal/core/events/events.go` (`RegisterNamespace`, `RegisterEventType`) | Holds namespaces and their event types with interned ids | Extends: it is the message registry; a name also holds who sends it and the tags it carries |
| Subscription filters | `internal/component/plugin/server/subscribe.go` (`Subscription.Direction`, `PeerFilter`, `PluginFilter`); `deliveryKey{ns, et, dir}` | A subscription narrows by direction, peer selector and plugin | Extends: these are the tags and the `from` of the model under other names. `PeerFilter.Selector` already accepts a peer's name or address |
| Group to peer inheritance | `internal/component/bgp/config/resolve.go` (`ResolveBGPTree`, `deepMergeAt`, `replacePaths`) | Merges a group's containers into each peer key by key; `replacePaths` is a literal map of containers replaced whole | Extends: the `bus` block inherits through it. Replace-whole for the bus lists comes from a marker on the grouping the resolver reads from the schema, in place of a second literal entry |
| Augmentation by a plugin's module | `ze-rs-conf.yang` and the other plugin modules that augment `/bgp:bgp` | A plugin adds its nodes under BGP's from its own module | Extends: the bus module augments its grouping onto every registered participant node the same way |
| A registration that names a schema path | `pkg/plugin/rpc/types.go` (`FilterInsertion.Config`); `registry.CheckFilterInsertions`, `config.CheckFilterInsertionPaths` | A plugin names configuration by path words in its registration, and Stage 1 refuses a path that is not a compiled schema node | Extends: a participant node is declared and judged the same way |
| Path values | `internal/component/config/yang/validator_reference.go` (`parseInstanceIdentifier`); the edit path of `set` and `edit` | Parses the RFC 7950 bracket form; an operator already types the slash form in `set bgp group transit` | Extends: `to` and `from` take the short slash form (M-3), typed, stored and shown. The bracket form is not used: an address and a prefix of names have no instance-identifier form |
| Reference walk | `internal/component/config/yang/validator_instance.go` (`ValidateReferences`) | At commit, refuses a reference whose instance is absent, naming the referring leaf | Extends: it is what refuses a `to` or `from` naming an absent entry, and a commit that deletes one |
| Exclusive role claims | `registry.Registration.Claims`; `rpc.ConfigureInput.Claims`; `sdk.Plugin.ClaimActive`; `pluginserver.Server.UnheldRoles`; `bgp/server/events.go` (`peerStateUnheldRoles`) | A plugin claims a role another performs by default. The claim is retracted per peer when no fed claimant can send `update` to it | Extends: the retraction reads the compiled filter. Claims are not extended into an arbitration rule (M-11) |
| Feature requirement checks | `bgp/config/redistribute_binding.go` (`wireRedistributeDelivery`, `requireSelectionFeeds`, `grantEveryPeer`); `flowspec_binding.go` (`requireRIBDelivery`); `filter_state_binding.go` (`grantFilterOwnersState`); `plugin/server/dissemination_validate.go` (`ValidateDisseminationBindings`); `rpc.DisseminationDecl` | Derives or demands bindings by plugin-name literals in BGP config code; judges a role's per-peer grants | Replaces: one declared requirement per feature, one check against the compiled filters, one message shape ("Feature requirements") |
| Delivery graph and send permission | `internal/component/plugin/server/delivery_graph.go` (`DeliveryGraph`, `Receivers`, `MaySend`, `PeerScopedProcs`); `internal/component/bgp/reactor/send_permission.go` (`Peer.maySend`, `filterPermittedPeers`) | One index keyed by peer address, built from each peer's bindings. The send check asks each peer | Extends: built from the compiled filters, same rebuild points, same per-delivery cost ("How delivery is indexed") |
| Delivery reconcile | `plugin/server/delivery_reconcile.go` (`reconcileDelivery`); `doctor_snapshot.go` (`DeliveryDisagreement`) | Reports each peer, process and event the subscription and the grant disagree about | Extends: reports a dormant reception and a written list no message passes through, with fixes in bus syntax |
| Engine operation registry | `plugin/server/dispatch_registry.go` (`engineOps`); `register_engine_op.go` (`RegisterEngineOp`, `EngineOperation`) | One entry per RPC; the JSON path, the direct path and the bridge setter derive from it | Extends: each entry states the message it carries or `none` with a reason, and the shared dispatch path applies the filter |
| Plugin identity in authorization | `plugin/server/dispatch.go` (`internalPluginIdentity`, `internalRPCIdentity`); `authz/authz.go` (`Store.authorize`); `aaa/reserved.go` (`ReservedInternalPrefix`) | Every plugin command runs under a reserved identity that is allowed before any profile | Replaces for plugin identities: a profile compiled from the participant's filter. Kept for the plugin-less RPC identity |
| Authorization profiles and matcher | `internal/component/authz` | Profiles of command path prefixes, one matcher for every user | Extends: the same matcher validates `cli:` message names and evaluates the plugin identity's profile |
| Request refusal reporting | `plugin/server/dispatch_report.go` (`refusePluginRequest`); `bgp/reactor/send_permission.go` (`sendPermissionDenied`, `recordSendRefused`) | A doctor event per plugin, and a BGP-only counter, log line and operator error | Extends and replaces: one refusal shape, one counter, one doctor code for every operation |
| Runtime subscribe | `delivery_graph.go` (`PeerScopedProcs`, `DiscardRuntimeSubscriptions`) | A `request subscribe` adds a type only inside the peer's grant, until the next apply | Extends: bounded by the participant's registration and its receive filter |
| Process alias resolution | `internal/component/plugin/process/process.go` (`RegistrationName`); `internal/component/plugin/resolve.go` (`RegistryName`, `RegistryNames`) | Separates the configured alias from the compiled registry name | Extends: used at the state, doctor and lifecycle sites. `deriveName`'s `ze plugin X` branch is deleted with the spelling it served |
| Identity no registry name can take | `internal/component/bgp/server/events.go` (`externalIdentity`) | Keeps a program named `bgp-gr` off the node of the registered `bgp-gr` | Extends: the same construction keys state grants and doctor snapshots for a foreign program |
| ExaBGP internal bridge and its feeds | `ze-exabgp-bridge-conf.yang` (`process > feed`); `bridgeplugin/config.go` (`parseFeeds`); `bridgeplugin/register.go` (`runInternalBridge`); `bridgerun/script.go` (`grantSet`, `receives`, `forms`, `feedPeers`); `bridgerun/fleet.go` | A compiled-in plugin that runs every script as a child and scopes each by its `feed` blocks | Replaces (E-1): deleted. The scoping becomes the script endpoint's `from`, `to` and receive allow-list. What the internal mode provides beyond that is owed ("ExaBGP", X-1 to X-7) |
| ExaBGP external wrapper | `ze exabgp plugin <cmd>` (`docs/architecture/exabgp-bridge.md`, "The internal runner cannot read the `run` line") | Runs one script as an SDK process over TLS connect-back | Extends (E-1): the only runner; gains the form words and the encoder as its options |
| Claims delivered at Stage 2 | `registry.Registration.Claims`; `rpc.ConfigureInput.Claims` | The engine unions a declaration over the startup set and delivers it before any runtime event | Extends: the message catalog is delivered the same way |
| External declarations read before the full parse | `pluginserver.HoldDeclaredPlugins`, `Manager.StartDeclared`; `config.SchemaWithDeclaredPlugins`, `DeclaredSchemas.ParseRefusal` | Each declared external program runs before the full parse and its Stage 1 declaration is kept | Extends: the same read supplies an external program's participant declarations |
| Query mode | `docs/architecture/api/process-protocol.md` ("Query Mode"); `sdk.RunOrDeclare`; `rpc.WriteDeclaration`; `plugin.QueryDeclaration` | A program started with `ZE_PLUGIN_MODE=declare` writes its Stage 1 line and exits 0. Two commands use it | Extends: every command that parses a config with no daemon runs each declared external program this way (S-3) |
| Per-peer attachment | `ze-bgp-conf.yang` (`container attach`); `bgp/reactor/config.go` (`parseProcessBindingsFromTree`); `bgp/config/plugins.go` (`extractBGPInlinePlugins`) | Per-peer or per-group `attach process` with `receive`, `send`, inline `run`/`use` and dead `content` | Replaces: deleted. The registered wiring and the `bus` block take its place. The block reuses attach's place in the tree, its group to peer inheritance and its rebuild points |
| Config view rendering and pipes | `internal/component/command` (`pipe_catalog.go`, `ApplyPipes`); `config.DetectFormat` (`serialize_set.go`) | Pipe operations registered once and offered by every surface; the text format detector reads `##` at column 0 as set-meta | Extends: `\| implied` is one registered pipe operation; its text marker avoids `##` |
| Retired keyword hints | `internal/component/config/retired.go` (`retiredKeywords`, `RetiredKeywordHint`) | Maps a retired keyword to a migration hint without accepting it | Extends: `plugin`, `attach`, `process` and `exabgp` point to `bus` |
| CLI `--plugin` flag | `internal/component/config/loader.go` (`MergeCliPlugins`); `internal/component/plugin/resolve.go` (`ResolvePlugin`) | Adds a compiled-in plugin or an external program to the declared set before the first pass | Extends: the merge produces endpoints. Nothing is restricted |
| First-pass roots | `internal/component/config/yang/join.go` (`firstPassRoots`, `FirstPassRoots`); `yang_schema.go` (`PluginOnlySchema`) | A literal list, `plugin` and `environment` | Replaces: the module that owns a root read before the full parse declares that on the root, and the list is derived |
| Plugin extraction seam | `internal/component/config/loader.go` (`RegisterPluginExtractor`) | A section's owner registers a function that extracts plugin configs from the tree | Extends: the `bus` reader is owned by `internal/component/plugin` and registered through this seam |
| `ze config graph` | `internal/component/config/graph.go` (`listenerSectionName`) | Draws listeners and the bindings the config holds | Extends: reads `bus` and `bus-hub`, and draws the compiled edges with their origins |
| Config tree parser and corpus validator | `internal/component/config/serialize.go`, `serialize_set.go`; `internal/component/config/cli/validate_corpus_test.go` | Parses and serializes `*config.Tree` and validates every tracked config | Extends: drives the one-shot converter |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "A process Ze forks from `plugin { external <name> { run ...; } }`, or one built on `pkg/plugin/sdk`" | departs | Replaces `plugin { external <name> { run ...; } }` with `bus { endpoint <name> { external { run ...; } } }`; door updated in the same work |
| `ai/doors/plugin-infra.md` | Who depends on it from outside | covering door for `internal/component/plugin/` | "`plugin { external <name> { run ...; } }` (`internal/component/plugin/yang/ze-plugin-conf.yang`)" | departs | Replaces `plugin { external }` with `bus { endpoint <name> { external { run ...; } } }`; the door's paragraph on starting no external program with no daemon is corrected with it; door updated in the same work |
| `ai/doors/bgp-engine.md` | Who depends on it from outside | covering door for `internal/component/bgp/` | "Which process sees which peer is the peer's `attach process` block" | departs | Removes `attach process`; the registered wiring decides, and the peer's `bus` block overrides it; door updated in the same work |
| `ai/doors/bgp-plugins.md` | Who depends on it from outside | covering door for `internal/component/bgp/plugins/` | "and the alias form `internal rs { use bgp-rs }`" | departs | Replaces `internal rs { use bgp-rs }` with `bus { endpoint rs { internal bgp-rs; } }`; door updated in the same work |
| `ai/doors/core.md` | Boundaries | covering door for `internal/core/events/` | "Never import upward." | agrees | `internal/core/events` stays leaf-only; components register namespaces and messages into core from `init()` |
| `ai/doors/config-yang.md` | Boundaries | covering door for `internal/component/config/` | "Extend a container with `augment`, never with a second definition." | agrees | The `bus` grouping reaches every participant node by augmentation from the bus module; the repository-wide conversion uses a parsed `*config.Tree` converter |
| `ai/doors/config-yang.md` | Who depends on it from outside | same door | "With no daemon running Ze starts no external plugin (owner ruling)" | departs | Reversed by S-3: a command that parses a config with no daemon starts the program in query mode; door updated in the same work |
| `ai/doors/system-plugins.md` | Boundaries | covering door for `internal/exabgp/` and `internal/plugins/exabgp/` | "`ze exabgp plugin` runs as an SDK process over TLS connect-back, beside the internal runner" | departs | E-1 removes the internal runner and the `exabgp { bridge }` root; the wrapper is the only runner; door updated in the same work |
| `ai/doors/hub-engine.md` | Boundaries | covering door for `cmd/ze/` and `internal/component/managed/` | "The hub and the Engine carry no BGP knowledge." | agrees | The bus, the hub and the engine stay free of BGP concepts |
| `ai/doors/commands-cli.md` | Boundaries | covering door for the command model | "Help, usage and completion are derived" | agrees | `\| implied` is one registered pipe operation, and the `cli` message names are validated by the command component's own matcher |
| `ai/doors/access-security.md` | Who depends on it from outside | covering door for `internal/component/authz/` | "command authorization reads its policy from the same" | agrees | The plugin identity is authorized by a profile in the same authorizer; no second gate is built |
| `ai/doors/le-tooling.md` | Boundaries | covering door for `test/` `.ci` files | "Never import `internal/le` from product code" | agrees | Product packages import nothing from `internal/le`; new tests live under `test/` |
| `docs/architecture/plugin/plugin-system.md` | One plugin, two ways to reach the bus | page the plugin door names | "A behaviour that differs between the two kinds with no reason of this kind is a defect." | agrees | Declarations, filters and the engine-operation check are the same for both kinds. The differences that remain are transport reasons and are stated: an external declaration is taken by running the program in query mode, and `RunsInProcessOnly` |
| `docs/architecture/plugin/plugin-system.md` | One plugin, two ways to reach the bus | same page | "With no daemon running, no command starts it, and a config section only its module defines is not available" | departs | Reversed by Thomas on 2026-11-23, confirmed as intended (S-3); page updated in the same work |
| `docs/architecture/api/architecture.md` | Owner ruling: endpoints and flows on the bus | `// Design:` page of `plugin/registry`, `bgp/config`, `bgp/reactor`, `plugin/server`; modified page | "A BGP group or peer takes part in a flow by listing its name, and the list merges through groups as every list does." | departs | Reversed by M-1 to M-10: no flow object exists; a BGP group or peer carries a `bus` block whose lists replace, never merge; page updated in the same work |
| `docs/architecture/api/architecture.md` | Owner ruling: implied and shown, or written | same page | "Where the operator has a choice, which program watches a peer and what it may send, the operator writes it, and silence stays the default." | departs | Reversed by M-4 and M-9: a participant receives and sends what it registered, shown by `\| implied`, until the operator overrides it; page updated in the same work |
| `docs/architecture/api/architecture.md` | What the Plugin Declares and What the Peer Grants | same page | "A plugin declares what it CAN handle, in its ready RPC." | departs | The declaration moves to the registration and becomes the default delivery, not the upper bound of a peer's grant; page updated in the same work |
| `docs/architecture/api/architecture.md` | Data Flow: Config to Delivery | same page | "A relay is judged on the grant of the process that sends it, and on nothing else: no process lends its grant to another." | departs | A relay is judged on the sender's compiled filter; no per-peer grant is written by default, and a forwarder serves the peers its session type names; page updated in the same work |
| `docs/architecture/config/syntax.md` | The first pass: plugin declarations before the full parse | `// Design:` page of `config/first_pass.go`, `yang_schema.go`, `retired.go`, `listener.go`, `graph.go`, `bgp/config/`; modified page | "`PluginOnlySchema` holds the roots `plugin` and `environment` and no other." | departs | The first-pass roots become `bus` and `environment`, derived from a declaration on the root; page updated in the same work |
| `docs/architecture/config/yang-config-design.md` | Decided, not built: modules compiled at build time | `// Design:` page of `validate_sections.go`, `validators.go`; modified page | "The `--plugin` flag keeps starting the plugin it names" | agrees | `ze start --plugin` keeps starting a compiled-in plugin or an external program (S-4). The page's examples change to `bus` syntax only |
| `docs/architecture/core-design.md` | 1. System Architecture | `// Design:` page of `bgp/reactor/`, `bgp/server/events.go`, `exabgp/migration/`; modified page | "Phase 2: explicit plugins (from config `plugin { }` block)." | departs | Phase 2 reads explicit endpoints from `bus { endpoint ... }`; page updated in the same work |
| `docs/architecture/buffer-architecture.md` | 1. Bytes Are the Source of Truth | `// Design:` page of `bgp/reactor/reactor_api_forward.go` | "Never duplicate data in different representations" | agrees | The read site changes how the forward targets are permitted, by the compiled filter in place of the per-peer binding. The forwarded buffer and its zero-copy path are untouched |
| `docs/architecture/core-design.md` | Design decisions | decision row on `docs/architecture/core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | The bus cutover does not alter the VPP and kernel commit checks |
| `docs/architecture/core-design.md` | Design decisions | decision row on `docs/architecture/core-design.md` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | The data-plane guard is untouched; the `bus` refusals follow the same rule of one verdict at start, commit, reload and validate |
| `docs/architecture/core-design.md` | Design decisions | decision row on `docs/architecture/core-design.md` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "nft"` annotation." | agrees | The bus cutover touches no firewall verifier and no `ze:backend` annotation |
| `docs/architecture/firewall/firewall-domain-group.md` | What the functional tests drive | anchored to `pkg/plugin/sdk/sdk_callbacks.go`; modified page | "`plugin { external ... }`" | departs | The sentence names the old block; it becomes `bus { endpoint ... { external ... } }`; page updated in the same work |
| `docs/architecture/api/wire-format.md` | Transport | anchored to `pkg/plugin/rpc/types.go`, `ze-plugin-engine.yang` | - | silent | Covers answer framing and item types; holds no statement on plugin config syntax, subscriptions or send permission |
| `docs/architecture/firewall/firewall-irr.md` | Decisions | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers IRR-fed firewall sets; silent on `bus` config and plugin registration of messages |
| `docs/architecture/forked-route-install.md` | The wire carries a name, the engine resolves it | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers the `route-install` wire for a forked plugin; silent on who may call it. This design adds the check on the caller and leaves the wire unchanged |
| `docs/architecture/plugin-manager-wiring.md` | Problem | anchored to `pkg/plugin/rpc/types.go`, `internal/component/plugin/types.go` | - | silent | Covers the spawn and handshake phases; holds no statement on plugin config syntax, subscriptions or send permission |
| `docs/architecture/update-cache.md` | Purpose | anchored to `pkg/plugin/rpc/types.go`, `plugin/process/process.go` | - | silent | Covers the recent-UPDATE cache; silent on `bus` config syntax |
| `docs/architecture/cli/error-surface.md` | The rule | anchored to `internal/component/plugin/types.go` | - | silent | Covers where a fault appears on each surface; silent on `bus` config syntax |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to `internal/component/plugin/types.go` | - | silent | Covers per-peer encoding contexts; silent on `bus` config syntax |
| `docs/architecture/plugin/show-enrichers.md` | The decisions | anchored to `plugin/process/process.go` | - | silent | Covers show enrichers; silent on `bus` config syntax |
| `docs/architecture/cli/tui-launcher.md` | Decisions | anchored to `cmd/ze/ze_core_dispatch.go` | - | silent | Covers the TUI launcher; silent on `bus` config syntax |
| `docs/architecture/api/process-protocol.md` | Event Delivery | `// Design:` page of `core/events/events.go`, `plugin/resolve.go`, `startup_autoload.go`, `startup.go`, `dispatch_state.go`; modified page | "the plugin subscribed to the type in that direction, and the peer's `attach process <name>` block grants it." | departs | Delivery needs the six conditions of "The message model". Stage 1 gains the participant declarations; page updated in the same work |
| `docs/architecture/api/process-protocol.md` | Persistent runtime state | same page | "Requests use the daemon's configured process name as identity." | departs | A compiled-in plugin's requests use its registration name, under any endpoint name and on either transport. A foreign program's use an identity no registry name can take; page updated in the same work |
| `docs/architecture/api/commands.md` | Event Subscription Commands | `// Design:` page of `cmd/peer/create.go`, `plugin/server/startup.go`, `managed/cmd_fleet.go`; modified page | "the process subscribed to the event, and the peer's `attach process <name>` block grants that type in that direction." | departs | Replaces `attach process <name>` with the registration and the `bus` block; page updated in the same work |
| `docs/architecture/api/commands.md` | Design decisions | decision row on `docs/architecture/api/commands.md` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | `\| implied` is a pipe operation after the command path, and `create bgp peer` loses an argument (`attach`) |
| `docs/architecture/api/commands.md` | Design decisions | decision row on `docs/architecture/api/commands.md` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | `\| implied` is a new operation of the config commands, not an override of a generic one, and it keeps the format operations generic |
| `docs/architecture/fleet-config.md` | Unified Named Blocks | `// Design:` page of `check_managed_listener.go`, `managed/client.go`, `cmd_fleet.go`, `managed_owner.go`; modified page | "All hub configuration uses explicit `server` and `client` keywords under `plugin { hub { } }`:" | departs | Moves `hub { server; client; }` under `bus { hub { } }` (listener `"plugin-hub"` becomes `"bus-hub"`); page updated in the same work |
| `docs/architecture/fleet-config.md` | Design decisions | decision row on `docs/architecture/fleet-config.md` | "A reload that changes the certificate a serving hub's managed listener presents installs it on the running listener at that reload: the next handshake presents the certificate the configuration names, the listener keeps its socket and no session is closed, as the web and looking glass listeners do. The certificate stays in the configuration's `pki` block. In Thomas's words (voice-transcribed): "Keep the certificate in the configuration." and "In that case on commit we can perform the certificate change on the listener."" | agrees | Only the block's parent changes, from `plugin` to `bus`; the listener's certificate handling on reload is untouched |
| `docs/architecture/hub-architecture.md` | Config File | `// Design:` page of `data_rpc.go`, `cmd/ze/hub/main.go`; modified page | "external acme {" | departs | Replaces the top-level `plugin { external acme { ... } }` layout with `bus { endpoint acme { external { ... } } }`; page updated in the same work |
| `docs/architecture/hub-api-commands.md` | Cross-subsystem references | anchored to `config/yang/validator_instance.go`; quote taken by the agent that wrote the design draft, not re-read in this pass | "a leafref value against the referred leaf's instances in the whole config" | agrees | Leafref checking is unchanged; the same walk also refuses a commit that deletes an entry a `to` or `from` names |
| `docs/architecture/system-architecture.md` | Overview | `// Design:` page of `cmd/ze/ze_core_start.go`; modified page | "`plugin { external ... }` block" | departs | External programs are declared under `bus { endpoint <name> { external { ... } } }`; page updated in the same work |
| `docs/architecture/rib-transition.md` | Owner ruling: a learned route needs the recipient's export policy | anchored to `ze-bgp-conf.yang`; modified page | "The `bgp-rib` plugin is fed a peer's routes, and may send to a peer, only where that peer's `attach process` block grants it, as for any process" | departs | `bgp-rib` is fed every peer's routes by its registered reception and sends by its declaration, unless the operator overrides either; export policy still governs each advertisement; page updated in the same work |
| `docs/architecture/bgp/structural-forwarding.md` | Receive validation and cache changes | anchored to `plugin/registry/registry.go`; modified page | "the source's `attach process <route server> { receive [ update-received ] }`" | departs | Replaces `attach process` references with the route server's registered reception and the `bus` block; page updated in the same work |
| `docs/architecture/api/capability-contract.md` | Config Validation (✅ DONE) | anchored to `bgp/config/peers.go`; modified page | "configured: attach process logger, attach process monitor - none carry either word" | departs | Updates the `validatePeerProcessCaps` refusal text and examples to bus syntax; page updated in the same work |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `plugin/registry/registry.go`, `resolve.go`, `startup_autoload.go`, `startup.go`; modified page | "The fast path then relays a source only when the source's attach block feeds this plugin its UPDATEs (`receive [ update-received ]`), and reaches a destination only when the destination grants this plugin `send [ update ]`" | departs | The fast path reads the compiled filter for the source and the destination; the `EventTypes` and `SendTypes` rows become the participant declarations; page updated in the same work |
| `docs/architecture/doctor-and-health-checks.md` | Plugin registration and delivery checks | `// Design:` page of `doctor_snapshot.go`, `check_external.go`; modified page | "the `attach process` change (`DeliveryDisagreement.Fix`)" | departs | `DeliveryDisagreement.Fix` names the `bus` block line, and `PluginDoctorState` carries the registration name and the transport; page updated in the same work |
| `docs/architecture/exabgp-bridge.md` | A neighbor command is one of three things | `// Design:` page of `bridge_neighbor.go`; modified page | "`attach exabgp-bridge`, once" | departs | Drops the `attach` keyword from `create bgp peer`. Under E-1 the page's internal runner and `exabgp { bridge }` root are removed whole; the page is rewritten in the same work once the owed ExaBGP rows are answered |
| `docs/architecture/update-building.md` | Shared ownership admission | anchored to `exabgp/migration/migrate.go`; modified page | "when no `attach process` block grants the owner process the owning source's" | departs | Updates the `attach process` reference to the owner's reception; page updated in the same work |
| `docs/architecture/update-building.md` | Design decisions | decision row on `docs/architecture/update-building.md` | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | The ExaBGP migration keeps writing `session { flowspec-redirect-next-hop true; }` while it emits the `bus` section |
| `docs/guide/ddos-mitigation.md` | FlowSpec mode sensor blindness | `// Design:` page of `ddos/flowspec/doctor.go`; modified page | "A block of another name (`plugin { internal <name> { use ddos-flowspec } }`)" | departs | Replaces the block with `bus { endpoint <name> { internal ddos-flowspec; } }` and resolves the alias by the registration name; page updated in the same work |
| `docs/config-reference.md` | Plugin Loading | modified page | "Plugins are declared in the `plugin { }` block:" | departs | Replaces the `plugin { }` and `attach process` sections with `bus { endpoint; hub; }` and the `bus` block; page updated in the same work |
| `docs/exabgp/exabgp-migration.md` | Key Differences | modified page | "`attach process <name> { }`" | departs | The migration emits one endpoint per ExaBGP process with its `from` and `to` lines; page updated in the same work |
| `docs/architecture/testing/ci-format.md` | Where the block goes | anchored to `cmd/ze/hub/main.go` | "The daemon `-` is recognized by POSITION, not by a list of verbs:" | agrees | `ze --plugin <name-or-program> -` invocations keep the same stdin routing |
| `docs/architecture/config/transaction-protocol.md` | Phase 1: Verify | anchored to `plugin/registry`, `loader_create.go`, `reactor_api.go`, `startup.go`; modified page | "`plugin { internal rpki { use bgp-rpki } }` names the process `rpki`, and its `pki` section still arrives" | departs | Replaces the examples with `bus { endpoint rpki { internal bgp-rpki; } }` and `bus { endpoint bgp-rpki { external { run "/opt/x"; } } }`, and states the identity and transport questions apart; page updated in the same work |
| `docs/architecture/plugin/component-boundaries.md` | The four components | anchored to `internal/core/events/events.go` | - | silent | Covers the four public `pkg/ze` interfaces; silent on config syntax |
| `docs/guide/flowspec-protected-router.md` | 2. Update the active stored config | `// Design:` page of `bgp/config/flowspec_binding.go`; modified page | "Execution mode comes from the effective plugin declaration: `use bgp-rib` runs in-process even inside an `external` stanza, while `run "ze plugin bgp-rib"` forks a subprocess." | departs | Execution mode comes from the endpoint's `process` leaf; `internal bgp-rib; process separate;` replaces `run "ze plugin bgp-rib"`, and FlowSpec authorization still refuses it because its lookups are process-local; page updated in the same work |
| `ai/rules/repo-maintenance.md` | Directives | `// Design:` page of `managed/doctor.go` | - | silent | Covers generated-file maintenance and doctor registration hygiene; silent on `bus` config syntax |
| `docs/architecture/api/update-syntax.md` | Raw Passthrough Commands | anchored to `ze-bgp-conf.yang`, `registry.go`; modified page | "The peer must attach the program with `send [ raw ]`; no other send type permits it." | departs | A program sends raw when it declared `bgp:raw` and no filter removes it; page updated in the same work |
| `docs/architecture/bgp/filter-path-asn.md` | The config | anchored to `ze-bgp-conf.yang` | - | silent | Covers `reject-asn` BGP filter config; silent on `plugin { }`, `attach process` and `bus { }` |
| `docs/architecture/config/environment-block.md` | TL;DR | anchored to `ze-bgp-conf.yang` | - | silent | Covers the `environment { }` block; silent on `plugin { }`, `attach process` and `bus { }` |
| `docs/architecture/config/environment.md` | Overview | anchored to `ze-bgp-conf.yang` | - | silent | Covers env var precedence; silent on `plugin { }`, `attach process` and `bus { }` |
| `docs/architecture/cli/command-verbs.md` | The axis: does the command change the system | anchored to `ze-peer-cmd.yang` | - | silent | Covers CLI verb classification; silent on `attach` or `bus` |
| `docs/architecture/decisions/001-pull-model-metrics.md` | The Question | anchored to `registry.go` | - | silent | Covers Prometheus pull-model metrics; silent on `bus` config or plugin aliasing |
| `docs/architecture/dns/geodns.md` | Config model | anchored to `registry.go` | - | silent | Covers GeoDNS configuration; silent on `bus` config or plugin aliasing |
| `docs/architecture/ike/ipsec-7-ikev2-engine.md` | Decisions | anchored to `registry.go`, `startup_autoload.go`, `startup.go` | - | silent | Covers IKEv2 engine decisions; silent on `bus` config or plugin aliasing |
| `docs/architecture/wire/nlri.md` | TL;DR (Read This First) | anchored to `registry.go` | - | silent | Covers BGP NLRI wire encoding; silent on `bus` config or plugin aliasing |
| `docs/architecture/config/system-update.md` | The backend | anchored to `validators.go` | - | silent | Covers the appliance system update backend; silent on `bus` config |
| `docs/architecture/isis/isis-4-component-config.md` | Decision: registration, never a switch | anchored to `validators.go` | - | silent | Covers IS-IS component config registration; silent on `bus` config |
| `docs/architecture/ospf/ospf-4-component-config.md` | Decisions | anchored to `validators.go` | - | silent | Covers OSPF component config; silent on `bus` config |
| `docs/architecture/resolve.md` | Structure | anchored to `validators.go` | - | silent | Covers the DNS resolution helper; silent on `bus` config |
| `docs/architecture/dns/as112.md` | The response code says where the query name sits | anchored to `listener.go` | - | silent | Covers AS112 DNS responses; silent on `bus` config or the `bus-hub` listener |
| `docs/architecture/config/apply-ordering.md` | The requirement | anchored to `peers.go`, `loader_create.go`, `peer_settings_apply.go` | - | silent | Covers deterministic config apply order; silent on `bus` syntax |
| `docs/architecture/wire/attributes.md` | TL;DR (Read This First) | anchored to `peers.go` | - | silent | Covers BGP path attribute wire encoding; silent on `bus` syntax |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `resolve.go`, `filter_state_binding.go`, `reactor/config.go`, `events.go`, `delivery_graph.go` | - | silent | Covers RFC 9234 route metadata keys; silent on `bus` config syntax |
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `redistribute_binding.go`; modified page | "the message names the peer and the forwarder, and tells the operator to attach the RIB to the peer with `receive [ update-received ]` or to take the peer down around the change." | departs | Replaces the per-peer attach wording and the `source_policy_reload.go` refusal text with the `bus` line that restores `bgp:update-received`; page updated in the same work |
| `docs/architecture/route-selection.md` | Overview | anchored to `redistribute_binding.go` | - | silent | Covers BGP best-path selection; silent on `bus` config syntax |
| `docs/architecture/rfc-may-decisions.md` | RFC 4760 Section 7 - Non-Negotiated AFI/SAFI Handling | anchored to `reactor/config.go` | - | silent | Covers BGP RFC MAY decisions; silent on `bus` config syntax |
| `docs/architecture/wire/capabilities.md` | TL;DR (Read This First) | anchored to `reactor/config.go` | - | silent | Covers BGP OPEN capability wire encoding; silent on `bus` config syntax |
| `docs/architecture/api/text-format.md` | Current Format | anchored to `reactor_api.go` | - | silent | Covers the BGP text event payload format; silent on `bus` config syntax |
| `docs/architecture/behavior/fsm.md` | TL;DR (Read This First) | anchored to `reactor_api.go`, `peer_derived_bindings.go` | - | silent | Covers BGP FSM state transitions; silent on `bus` config syntax |
| `docs/architecture/behavior/peer-lifecycle.md` | TL;DR | anchored to `reactor_api.go`, `peer_settings_apply.go`, `peer_derived_bindings.go`, `events.go` | - | silent | Covers the BGP peer TCP and session lifecycle; silent on `bus` config syntax |
| `docs/architecture/wire/nlri-bgpls.md` | Consumer Decoding and Origination | anchored to `reactor_api.go`; modified page | "Collector peers negotiate the `bgp-ls` family and attach the plugin with `state` and `refresh` event delivery and UPDATE-send permission." | departs | `bgp-ls-export` and `bgp-epe` register those receptions and that send; page updated in the same work |
| `docs/architecture/api/json-format.md` | Overview | anchored to `peer_derived_bindings.go`, `events.go` | - | silent | Covers the BGP JSON event envelope format; silent on `bus` config syntax |
| `docs/architecture/firewall/table-ownership-and-shutdown-flush.md` | Rule 1: the backend loads on demand | anchored to `startup_autoload.go`, `startup.go` | - | silent | Covers firewall nft table ownership; silent on `bus` config syntax |
| `docs/architecture/ospf/bfd-client.md` | Decisions | anchored to `startup_autoload.go` | - | silent | Covers the OSPF BFD client integration; silent on `bus` config syntax |
| `docs/architecture/plugin/feature-gates.md` | The inventory is `feature-gates.txt` | anchored to `startup_autoload.go` | - | silent | Covers `ze_<feature>` compile-out tags; silent on `bus` config syntax |
| `docs/architecture/static-routes.md` | A main-table route reaches the FIB through the Loc-RIB | anchored to `startup_autoload.go` | - | silent | Covers static route installation into the Loc-RIB; silent on `bus` config syntax |
| `docs/architecture/api/ipc_protocol.md` | Overview | anchored to `startup.go` | - | silent | Covers IPC wire framing; silent on `bus` config syntax |
| `docs/architecture/cli/command-completion.md` | The producers | anchored to `startup.go`, `cmd/ze/hub/main.go` | - | silent | Covers CLI completion producers; silent on `bus` config syntax |
| `docs/architecture/l2tp/cpe-1-pppoe-client.md` | RFC obligations carried by this code | anchored to `startup.go` | - | silent | Covers PPPoE client RFC obligations; silent on `bus` config syntax |
| `docs/architecture/storage-backends.md` | The memory encoding | anchored to `data_rpc.go`, `main.go`, `ze_core_start.go` | - | silent | Covers ZeFS and memory storage backend mechanics; silent on `bus` config syntax |
| `docs/architecture/web-interface.md` | Source Files | anchored to `data_rpc.go`, `main.go` | - | silent | Covers Web UI routes and HTMX templates; silent on `bus` config syntax |
| `docs/architecture/zefs-format.md` | Netcapstring | anchored to `data_rpc.go` | - | silent | Covers the ZeFS binary format; silent on `bus` config syntax |
| `docs/architecture/appliance/remote-operations.md` | Decisions | anchored to `cmd/ze/hub/main.go` | - | silent | Covers appliance remote operations; silent on `bus` config syntax |
| `docs/architecture/behavior/signals.md` | Supported Signals | anchored to `cmd/ze/hub/main.go` | - | silent | Covers SIGHUP and SIGTERM handling; silent on `bus` config syntax |
| `docs/architecture/diagnostics/crash-capture.md` | The decisions | anchored to `cmd/ze/hub/main.go` | - | silent | Covers crash capture dumps; silent on `bus` config syntax |
| `docs/architecture/pki/pki-store.md` | Decisions | anchored to `cmd/ze/hub/main.go` | - | silent | Covers the PKI certificate store; silent on `bus` config syntax |
| `docs/architecture/pki/tls-listeners.md` | Decision: the leaf comes first, then the intermediates | anchored to `cmd/ze/hub/main.go` | - | silent | Covers TLS certificate chain ordering; silent on `bus` config syntax |
| `docs/architecture/appliance/device-config.md` | Legacy boot inbox | anchored to `cmd/ze/ze_core_start.go` | - | silent | Covers the appliance legacy boot inbox; silent on `bus` config syntax |
| `docs/architecture/appliance/kernel-profiles.md` | Go owns the guarantee | anchored to `cmd/ze/ze_core_start.go` | - | silent | Covers appliance kernel profiles; silent on `bus` config syntax |
| `docs/architecture/fleet-interface.md` | The package | anchored to `cmd/ze/ze_core_start.go` | - | silent | Covers `pkg/fleet` wire envelopes; silent on `bus` config syntax |
| `docs/architecture/config/exabgp-syntax.md` | Static Routes | anchored to `exabgp/migration/migrate.go` | - | silent | Covers ExaBGP static route syntax; silent on `bus` config syntax |
| `docs/architecture/bgp/egress-attribute-rules.md` | The two rules | anchored to `bgp/reactor/reactor_api_forward.go` | - | silent | Covers attribute provenance and final wire admission on egress; holds no statement on which plugin is fed a peer or may send to it |
| `docs/architecture/bgp/fanout-dedup.md` | The decisions | anchored to `bgp/reactor/reactor_api_forward.go` | - | silent | Covers sharing the rebuild plan across forward destinations; the page states nothing about how that set is chosen |
| `docs/architecture/forward-congestion-pool.md` | Invariant | anchored to `bgp/reactor/reactor_api_forward.go`, `reactor_api_relay.go` | - | silent | Covers forward buffer pools and congestion response; silent on plugin delivery and send permission |
| `docs/architecture/memory/lifetime-contracts.md` | The four contracts | anchored to `bgp/reactor/peer_run.go`, `reactor_api_forward.go` | "The source lease RPC checks the exact original Peer's explicit receive grant for the authenticated configured process." | departs | No per-peer grant is written by default. The lease check reads whether the compiled filter feeds that peer to the process, under the registration identity for a compiled-in plugin; page updated in the same work |
| `docs/architecture/behavior/fsm-idle.md` | TL;DR | anchored to `bgp/reactor/peer_run.go` | - | silent | Covers the Idle state of the BGP FSM; silent on plugin delivery and `bus` config |
| `docs/architecture/behavior/fsm-connect.md` | TL;DR | anchored to `bgp/reactor/peer_run.go` | - | silent | Covers the Connect state; silent on plugin delivery and `bus` config |
| `docs/architecture/behavior/fsm-active.md` | TL;DR | anchored to `bgp/reactor/peer_run.go` | - | silent | Covers the Active state; silent on plugin delivery and `bus` config |
| `docs/architecture/behavior/fsm-open-sent.md` | TL;DR | anchored to `bgp/reactor/peer_run.go` | - | silent | Covers the OpenSent state; its mention of the report bus concerns connection diagnostics, not event delivery to plugins |
| `docs/architecture/behavior/fsm-open-confirm.md` | TL;DR | named by the design check beside its sibling state pages | - | silent | Covers the OpenConfirm state; silent on plugin delivery and `bus` config |
| `docs/architecture/behavior/fsm-established.md` | TL;DR | anchored to `bgp/reactor/peer_run.go` | - | silent | Covers the Established state. Its compatibility note says per-peer KEEPALIVE receive events are not published on the plugin bus; this design publishes no new event |
| `docs/architecture/bgp/protocol-event-capture.md` | Enabling it | anchored to `bgp/reactor/peer_run.go` | - | silent | The capture is enabled by a `capture` block on the peer and tees the session; it is not an event delivery and names no plugin |
| `docs/architecture/diagnostics/event-history-rings.md` | The decisions | anchored to `bgp/reactor/peer_run.go`, `plugin/server/dispatch.go` | "The global ring taps `deliverEvent()`, the single dispatch point for every event." | agrees | The compiled filter changes which receivers an event has, not where it is dispatched: one dispatch point stays, and the ring still appends there once per event |
| `docs/architecture/edge-cases/extended-message.md` | TL;DR | anchored to `bgp/reactor/peer_run.go` | - | silent | Covers RFC 8654 message sizes; silent on plugin delivery and `bus` config |
| `docs/architecture/mrt.md` | Daemon Component (`internal/plugins/mrt`) | anchored to `bgp/reactor/peer_run.go` | "Registers as a Ze component. Subscribes to:" | agrees | The MRT plugin keeps receiving BGP update and state events for every peer; it states them as `Receives` in its registration in place of a subscription call, and the page's wording follows |

Rows owed and not yet written: the pages `./le spec design documents` derives from the sources this rewrite added (`internal/component/authz/authz.go`, `internal/component/aaa/reserved.go`, `plugin/server/dispatch.go`, `dispatch_registry.go`, `register_engine_op.go`, `internal/component/command/pipe_catalog.go`, the route server and reflector packages, `internal/plugins/exabgp/`), and the rows the two absorbed specs held in their own Design documents tables, which this pass did not read. The main session runs the command and a continuation adds each row it prints.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config file, stdin, editor commit and SIGHUP reload (`bus { endpoint ...; hub ...; }`, a `bus` block on any participant), `ze start --plugin <name-or-program>`, `ze exabgp migrate`, the Stage 1 declaration of an external program, every engine operation a plugin calls, and `show ... | implied`.

### Transformation Path
1. **Registration at `init()`:** each component and compiled-in plugin registers its namespaces and messages, its participant node, and what an instance sends and receives. Nothing is registered lazily.
2. **Schema build:** the bus module augments its grouping onto every registered participant node. An external program's node is added when its declaration is read.
3. **First pass (`config.FirstPass`):** the roots read before any plugin runs are derived from the modules that declare them (`bus`, `environment`). The owner of `bus` extracts the endpoints through the `RegisterPluginExtractor` seam. `MergeCliPlugins` adds each `--plugin` as an endpoint.
4. **External declarations:** `HoldDeclaredPlugins` runs each declared external program and keeps its Stage 1 declaration. Its sends join the registry, so the message set is complete. With no daemon, the same declaration is taken in query mode.
5. **Full parse and validation (`config.LoadConfig`, `ze config validate`):** group to peer inheritance resolves each participant's block; the eight commit refusals of "Two places to write, one filter" are judged, with one verdict on every path.
6. **Compilation:** for each participant the bus compiles the registration, the bus-side fields and the participant-side block into one filter. Feature requirements are checked against the filters.
7. **Catalog delivery:** Stage 2 configure carries the message catalog to each plugin, beside `Claims`. The ready RPC then subscribes from the plugin's declaration.
8. **Delivery index:** built from the filters after every participant change and at the end of a config apply. A broadcast reads the sender's key and the message key and gets a stored receiver slice.
9. **Engine operations:** the shared dispatch path reads the message the operation's entry names and asks the caller's filter. A `cli` message is authorized by the profile compiled from the filter, in `authz`.
10. **Process identity:** `process.Process` answers three things apart: the endpoint name, the registration name of a compiled-in plugin, and the transport.
11. **Display:** `| implied` renders the compiled filters with the origin of each effective entry.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `internal/core/events` <-> `internal/component/config` | Registries filled at `init()` and read by the `bus` validators | No |
| `internal/component/plugin` <-> `internal/component/config` | The `bus` reader registered through `RegisterPluginExtractor`; the grouping augmented by the bus module; roots declared on the module | No |
| `internal/component/bgp` <-> `internal/component/plugin/server` | BGP registers its peer and group lists as participant nodes and publishes each participant with its key and its path | No |
| `internal/component/plugin/server` <-> `internal/component/authz` | The profile of a plugin identity, compiled from its filter | No |
| `internal/component/plugin/server` <-> `internal/component/command` | The `cli` namespace and its validator; the `\| implied` pipe operation | No |
| Engine <-> plugin process, either kind | Stage 1 carries the participant node, `sends` and `receives`; Stage 2 carries the catalog | No |

### Changed-Wire Census

**Search evidence:** Searched `pkg/plugin/rpc/types.go` for the fields of `DeclareRegistrationInput` (positive control: `Dependencies`, `StartAfter`, `WantsConfig` are present). It holds no event, reception or send declaration. Searched `pkg/plugin/sdk/sdk_callbacks.go` for the reception a plugin declares today (positive control: `SetStartupSubscriptions` and its two siblings, 19 call sites in the tree per the earlier census). The carriers below were derived by the earlier passes of this spec and the two absorbed specs; the rows the absorbed specs held in their own census tables were not read in this pass and are owed to a continuation.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Ze config grammar | writer | `internal/exabgp/migration/migrate_serialize.go` (`SerializeTree`), `internal/component/config/serialize.go` (`Serialize`), 1,750 tracked configs, tests and docs outside `plan/` (2026-11-23 count) | Census: 248 Go, 1,197 `.ci`, 2 `.et`, 148 interop, 74 other test, 55 docs, 9 website, 11 `ai/` and `etc/` | Changed in the same work by the parsed-tree converter; `retired.go` gives the hints |
| Ze config grammar | reader | `internal/component/config/first_pass.go` (`FirstPass`), `loader.go` (`ExtractPluginsFromTree`), `loader_extract.go` (`ExtractHubConfig`), `internal/component/bgp/reactor/config.go` (`parseProcessBindingsFromTree`), `internal/component/plugin/doctor/check_external.go`, `internal/plugins/exabgp/bridgeplugin/config.go` (`parseFeeds`) | Read at each producer | Changed in the same work; `plugin`, `attach` and the `exabgp` root are deleted first |
| Stage 1 declare-registration | writer | `pkg/plugin/sdk` (every plugin built on it), every external fixture under `internal/test/fixture/` and `test/`, the ExaBGP wrapper, a third-party plugin in any language | `DeclareRegistrationInput` in `pkg/plugin/rpc/types.go` | Changed directly, every plugin in the tree rebuilt (`ai/META-DECISIONS.md`, "Ze is pre-release"); no version gate |
| Stage 1 declare-registration | reader | `internal/component/plugin/server/stage1_checks.go`, `pluginserver.HoldDeclaredPlugins`, `ze show plugin declarations`, the conformance checker | Earlier pass | Changed in the same work; the check table gains one entry per declaration |
| Query-mode declaration line | writer | `pkg/plugin/sdk/sdk_query.go` (`RunOrDeclare`) and `internal/component/plugin/cli/main.go`, both through `pkg/plugin/rpc/declaration.go` (`WriteDeclaration`) | Earlier pass: under `ze.plugin.mode=declare` the program writes one request line, method `declare-registration`, and exits 0 | The env value and the framing are unchanged. The line carries the new declarations because it is the Stage 1 payload |
| Query-mode declaration line | reader | `internal/component/plugin/declarations.go` (`QueryDeclaration`); `show plugin declarations`; the conformance check. Added here: every command that parses a config with no daemon, through `config.SchemaWithDeclaredPlugins` | Earlier pass | New consumer in the same work (S-3), through the decoder the daemon's Stage 1 uses |
| Stage 2 configure | writer | `internal/component/plugin/server` (the configure stage, where `rpc.ConfigureInput.Claims` is filled) | `Claims` comment in `registry.go` | Additive field for the message catalog |
| Stage 2 configure | reader | `pkg/plugin/sdk` configure callback; a third-party plugin in any language | `Claims` comment in `pkg/plugin/rpc/types.go` | Additive field; a plugin that ignores it is unaffected |
| Ready RPC startup subscriptions | writer | `sdk.Plugin.SetStartupSubscriptions`, `SetStartupSubscriptionsIn`, `AddStartupSubscriptionsIn` | `pkg/plugin/sdk/sdk_callbacks.go` | The event part is replaced by the `Receives` declaration; the calls are deleted; the format stays |
| Ready RPC startup subscriptions | reader | `internal/component/plugin/server/dispatch.go` (`registerSubscriptions`) | Earlier pass | Reads the declaration held from Stage 1 |
| create bgp peer command | writer | `internal/exabgp/bridge/bridge_neighbor.go` (`convertNeighborCreate`), operators, tests | `convertNeighborCreate` emits `attach exabgp-bridge` | The keyword is dropped |
| create bgp peer command | reader | `internal/component/bgp/plugins/cmd/peer/create.go` (`setAttach`), `ze-peer-cmd.yang` | `setAttach` writes `attach.process.<name>` | `attach` and `setAttach` are removed |
| Listener service name | writer | `internal/component/config/listener.go` (`DiscoverListenerServices`), `graph.go` (`listenerSectionName`) | Derives `<top>-<child>`: `plugin-hub` becomes `bus-hub` | Changed in the same work |
| Listener service name | reader | `internal/component/config/cli/` (`show listeners`), `internal/component/managed/doctor.go`, `internal/component/plugin/doctor/check_managed_listener.go` | Earlier pass | Changed in the same work |
| Request refusal text and counter | writer | `internal/component/bgp/reactor/send_permission.go` (`sendPermissionDenied`, `recordSendRefused`); `plugin/server/dispatch_report.go` (`refusePluginRequest`) | Read at `request_cancel.go` for the text shape; the others by the earlier pass | One shape and one counter in the plugin server; `ze_bgp_send_refused_total` is deleted |
| Request refusal text and counter | reader | The `.ci` files that match the text (34 files name the refusal), operators' log matchers and dashboards | Earlier grep of `test/` | Each matching `.ci` is converted with the text, through the test judge where it proves the per-peer refusal |
| Authorization identity of a plugin command | writer | `plugin/server/dispatch.go` (`internalPluginIdentity`, set in `dispatchCommandPeerResponse`, `dispatchCommandArgsResponse`, `opUpdateRoute`) | `dispatch.go` lines found by grep in this pass | Unchanged on the wire of accounting and audit; what it authorizes changes |
| Authorization identity of a plugin command | reader | `authz/authz.go` (`Store.authorize`), `tacacs/text.go`, the audit log | grep of `ReservedInternalPrefix` in this pass | `Store.authorize` evaluates a profile for a `plugin:` identity; the TACACS+ and audit readers keep the identity |
| Config view output | writer | `internal/component/config/cli` (the new renderer behind `show config ... \| implied`) | Absorbed spec | New: the JSON and YAML envelope; without the pipe the output is byte-identical to today's |
| Config view output | reader | The CLI, the editor, the web inspection and workbench, REST, gRPC, MCP | Absorbed spec | New consumers; gNMI `Get` is unchanged |

### Integration Points
- `internal/component/plugin/registry/registry.go` (`Registration`) - the participant declarations replace `EventTypes` and `SendTypes`; a declared feature requirement generalizes `Dissemination`.
- `internal/core/events/events.go` - the message registry.
- `internal/component/bgp/config/resolve.go` (`ResolveBGPTree`) - the replace-whole marker for the bus lists.
- `internal/component/config/yang/join.go` (`FirstPassRoots`) - derived from a declaration on the root.
- `internal/component/plugin/server/delivery_graph.go` (`DeliveryGraph`) - built from the compiled filters.
- `internal/component/plugin/server/dispatch_registry.go`, `register_engine_op.go` - the message field and the one check.
- `internal/component/authz/authz.go` (`Store.authorize`) - the profile of a plugin identity.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | One compiled filter per participant feeds delivery, the engine-operation check, the authorization profile, the display and the reconcile |
| No unintended coupling (components stay isolated) | Yes | The bus carries no BGP word; participant nodes and messages are registered by their owners; `internal/core/events` stays a leaf |
| No duplicated functionality (extends existing, does not recreate) | Yes | Every row of Existing Mechanisms is extended or deleted; no second authorizer, no second inheritance, no second dispatch path |
| Zero-copy preserved where applicable (refs, not copies) | Yes | UPDATE buffers and `RawMessage` lifetime are untouched; the index stores receivers per sender and message, and the benchmarks assert no allocation |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Participants, messages and feature requirements register from `init()`; the bus validates from those registries |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes, once built | `firstPassRoots` (`config/yang/join.go`) is a literal list that would learn `bus`; this design derives it. `replacePaths` would learn the bus lists; this design reads a schema marker. The three plugin-name literals of `requireRIBDelivery` are replaced by the declared requirement. `validatedSections`, `retiredKeywords` and `listenerSectionName` were searched |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Dropping `attach > content` removes no working feature | `delivery_graph.go` (`addBinding`) drops both fields; `bgp/server/events.go` reads only the process's encoding | A consumer depends on per-attachment encoding | Repo-wide grep for `ProcessBinding` readers | validated |
| A-2 | No repository test depends on a program not seeing a peer, apart from the tests that prove that silence itself | The receive census (2026-11-23) | A scenario's result changes because a program now receives a peer's events | The receive census | measured: holds only with the listed tests routed to the judge |
| A-3 | The messages a compiled-in plugin receives are a constant of the plugin, not a function of its config | 19 call sites of the three subscription calls; a narrower per-instance reception rides the ready RPC | A plugin whose reception depends on its config cannot declare it as a pure value | Read of all 19 call sites at implementation start | unvalidated |
| A-4 | No repository test depends on a plugin being refused a send toward one peer while allowed toward another, apart from the tests that prove it | The per-peer send census (2026-11-23) | A scenario's routing depends on the per-peer refusal | The census | invalidated: the dependent tests are listed under "Per-peer send census"; each is restated with a `to` or `from` list through the judge |
| A-5 | Message names for events and for sends do not collide in one registry (`bgp:update` the send, `bgp:update-received` the event) | The existing receive and send token sets | A name means two things | A test over the registry at implementation start | unvalidated |
| A-6 | The schema loader can apply an augment whose targets come from registrations | Plugin modules already augment `/bgp:bgp`; a registration-driven target set was not found | The grouping must be `uses`d by each owner's module, which makes BGP's YANG name the bus | Read of the YANG loader at implementation start | unvalidated |
| A-7 | The config syntax keeps a present and empty list distinct from an absent one | Not read | `to` and `from` become presence containers holding the list (Q-1) | Read of the leaf-list parser and serializer | unvalidated |
| A-8 | The resolver can read a replace-whole marker from the schema | `replacePaths` is a literal map today | A second literal entry in BGP config code, which the modular-engine rule forbids | Read of `ResolveBGPTree` at implementation start | unvalidated |
| A-9 | A peer created at run time by `create bgp peer` can be named `/bgp/peer/<address>` | `convertNeighborCreate`'s comment: the peer lives in the reactor alone and is not written to the configuration | Such a peer is covered only by `/bgp` | Read of `AddDynamicPeer` | unvalidated |
| A-10 | Limiting the route server to its client set and the reflector to internal peers removes the double replay, and neither plugin already limits itself | `selectForwardTargets` and `updateTarget` hold no session-type condition; a grep of the two packages for an iBGP or client test in target selection found none; how `rs.peers` is filled and the reflector's selection were not read | The defect is narrower or already absent, or the limit breaks a config that runs the route server over non-client peers | Read of the route server's and the reflector's peer-up handling, and a census of route-server tests without `rs-client` | unvalidated |
| A-11 | Everything the internal ExaBGP bridge does can be carried by one wrapper process per script | E-1; rows X-1 to X-7 | Something stays that needs shared state, and E-1 goes back to Thomas with the evidence | The owed read of `exabgp-bridge.md`, `bridgeplugin` and `bridgerun` | unvalidated |
| A-12 | The command component's matcher can validate a `cli:` prefix at commit, and one profile per plugin identity is cheap to compile | `authz` evaluates prefixes for every user today | Commit-time validation of `cli:` entries is weaker than stated | Read of `authz` at implementation start | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A plugin starts receiving or sending on peers the operator kept it away from, because the per-peer block is gone | A converted scenario's peer receives routes it did not before | The ruled behavior change (M-4, M-12). The two censuses name every config and test it reaches; each test of the difference is restated with a list through the judge |
| R-2 | The converter corrupts `$PORT` placeholders, re-encodes test secrets, or mangles `refused=` blocks | Diff review or `TestRepositoryConfigsValidate` failures | Sentinels around parse and serialize, no `EncodeSecrets` in the converter, each `refused=` block triaged by hand |
| R-3 | `hub` validation is skipped or the `bus-hub` listener edge is dropped | Managed `.ci` or `TestConfigGraph` fails | Update the roots, the validate guard, `listenerSectionName` and the diagnostic strings together |
| R-4 | A foreign program named like a compiled-in plugin is taken for it | A unit test of a foreign `ddos-flowspec` reading the compiled plugin's state key | Identity comes from the `internal` leaf alone |
| R-5 | A forked compiled-in plugin declares something other than its registry row | The Stage 1 check table reports a difference | Stage 1 compares the declaration of an `internal ... process separate` endpoint with the registry row and refuses a difference |
| R-6 | The scale of the conversion collides with concurrent sessions in the shared checkout | Dirty files or compile conflicts | Land the engine, YANG and converter first, then convert in one focused pass, committing compiling chunks |
| R-7 | An offline `ze config validate` accepts what the daemon refuses | `test/plugin/config-validate-agrees-with-boot.ci` | Offline validation takes the program's declaration in query mode and fails when it cannot |
| R-8 | `ze config validate` now executes the programs a config names | A validate run on an untrusted file starts a program | The bus configuration is the trust boundary (S-1), query mode starts nothing, and the validate output names each program it asked |
| R-9 | A compiled-in plugin loses a command it uses because its declaration is incomplete | A functional suite fails with the "did not declare" refusal | The fixture command census and a runtime capture write each declaration once; `TestCompiledInDeclarationsCoverWhatTheySend` |
| R-10 | A filter leaks onto the delivery path | A benchmark reports an allocation, or a profile shows path text compared per message | Lists are parsed at commit and resolved when the index is built |
| R-11 | The view and the runtime differ | `TestImpliedViewMatchesCompiledFilters` | Both read the one compiled filter; no second computation exists |
| R-12 | The implied text does not round-trip | `TestImpliedTextRoundTrips` | The comment marker is `# `, never `##` at column 0 |
| R-13 | An ExaBGP script loses or gains a neighbor in the move from `feed` to `from` and `to` | A case of `./le test functional exabgp` changes its output | The migration writes both lists from the `api` relation; the per-neighbor cases keep their expectations |
| R-14 | Restricting the route server to its client set breaks tests that run it over plain peers | Route-server `.ci` files fail with no relayed UPDATE | The census of A-10 is taken before the change; Q-5 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Configs fail to parse, plugins receive nothing or everything, a plugin sends what it must not or is refused what it needs, managed hub listeners fail to bind, aliased plugins fail state access, or a plugin command is authorized wrongly |
| How is it reverted? | Git revert of the cutover commits; pre-release `main` has no deployed user |
| Who else touches this path? | Any concurrent spec that adds `.ci` tests with `plugin {`, `attach process` or `exabgp { bridge`, and any that adds an engine operation |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze start` of `bus { endpoint collector { external { run ...; } } }` with pure `bgp { }` | -> | Registration -> compiled filter -> delivery index -> BGP event delivery | `test/plugin/bus-default-wiring.ci` (new) |
| `ze start` with a `bus { to [ ... ]; }` block on a peer and on a group | -> | Inheritance -> compilation -> index | `test/plugin/bus-to-replaces-default.ci`, `test/plugin/bus-group-inheritance.ci` (new) |
| `ze start` with `from`, per-message lists and both allow-lists | -> | Compilation -> index and engine-operation check | `test/plugin/bus-from-limits.ci`, `test/plugin/bus-allow-lists.ci` (new) |
| `ze config validate` and an editor commit of each refused config | -> | The eight commit refusals | `test/parse/bus-commit-refusals.ci`, `test/editor/bus-block-commit.et` (new) |
| `bgp { route-server { bus { ... } } }` and `bus { endpoint bgp-rs { ... } }` together | -> | Two places -> one filter -> `\| implied` | `test/plugin/bus-two-places.ci` (new) |
| `ze --plugin "<program>" -` | -> | `MergeCliPlugins` -> first pass -> held Stage 1 declaration | `test/plugin/external-yang-command-line.ci` (existing, converted, not weakened) |
| `bus { endpoint my-ddos { internal ddos-flowspec; process separate; } }`, then `show doctor` | -> | Process identity -> `dispatch_state.go` and `doctor_snapshot.go` | `test/plugin/bus-aliased-forked-state-doctor.ci` (new) |
| `ze config validate <file>` with no daemon, on a config with an external endpoint | -> | `config.SchemaWithDeclaredPlugins` -> `plugin.QueryDeclaration` -> module join -> `bus` validation | `test/parse/bus-offline-validate-query-mode.ci` (new) |
| `ze start` with `bgp-rs` and `bgp-rr` both loaded, client and internal peers, nothing written | -> | Each plugin's own session-type limit -> one replay per peer | `test/plugin/bus-rs-rr-one-box.ci` (new) |
| An external program dispatching a command it did not declare | -> | Shared dispatch path -> filter -> `authz` profile -> refusal, counter, doctor event | `test/plugin/bus-send-undeclared-refused.ci` (new) |
| `show config running \| implied` on the CLI, the editor and the web | -> | The registered pipe operation -> the compiled filters | `test/plugin/bus-implied-view.ci`, `test/editor/bus-implied-editor.et`, `test/web/bus-implied-workbench.ci` (new) |
| `ze exabgp migrate` of a config with two processes and per-neighbor `api` blocks, then `ze start` | -> | Migration writes endpoints with `from` and `to` -> the wrapper -> delivery and command scope | `test/exabgp/exabgp-script-endpoints.ci` (new; pending the ExaBGP design) |
| SIGHUP after changing a list on a running participant | -> | Reload -> recompilation -> index rebuild | `test/reload/bus-list-reload.ci` (new) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An endpoint whose program registers the reception of `bgp:update-received` and `bgp:state` and declares no send; pure `bgp { }`; nothing else written | The config validates and boots. The program receives both messages for every peer, including a peer added by a later reload. A send of `bgp:update` is refused because it was not declared | A message the program registered no reception for is never delivered |
| AC-2 | `to` written on a peer, on a group, and present and empty | The written list is the complete destination set of that participant's messages: the listed participants receive them and registered receivers outside it receive nothing. A group's list applies to its peers; a member's list replaces it. The empty list delivers to nobody | Participants with no list keep the registered wiring |
| AC-3 | `from` absent, written, and present and empty, on a receiver | Absent: the receiver gets the message from every sender. Written: from the senders the list covers only. Empty: from nobody | The receiver's own sends are unaffected |
| AC-4 | A block-level `to` and a `message <name> { to ... }` on one participant; the same for `from` | The named message follows its own list; every other message follows the block-level list | None: one list decides each message |
| AC-5 | A send allow-list and a receive allow-list, each absent, partial and empty | Absent: everything declared passes. Partial: the listed names pass and the others are refused. Empty: nothing passes. Each refusal names the participant, the message and the line that admits it, and counts once | `show config` keeps an empty list distinct from an absent one |
| AC-6 | Each of the eight commit refusals of "Two places to write, one filter" | Refused at `ze config validate`, editor commit, reload and `ze start` with one verdict, naming the line, the value and what is valid; refusals 5 and 6 name both lines | The corrected config of each case loads |
| AC-7 | A plugin with an endpoint entry and a config section of its own, with a block in one place, in the other, in both agreeing, and in both disagreeing | The first three load and yield the same single filter, which the display shows once. The fourth is refused naming both lines | Identical content in both places is permitted because it states one filter |
| AC-8 | A registered reception whose sender is absent, then configured by reload | The config loads, and `show doctor`, the reconcile and the display name the reception as dormant. After the reload the delivery starts | A written list naming an unknown path is still refused (AC-6) |
| AC-9 | `hub { server ...; client ...; }` under `bus { hub { ... } }` | The managed hub listener binds and validates, `show listeners` reports `bus-hub`, `config graph` links `bus` to `bus-hub`, and the managed doctor checks pass | None: the block only moves |
| AC-10 | A config that uses the retired `plugin { }`, `attach process`, `process` under `bgp`, or `exabgp { bridge }` | Refused at parse with `RetiredKeywordHint` pointing to `bus` | None: no hybrid is accepted |
| AC-11 | `ze --plugin ze.bgp-bmp -` and `ze --plugin "<program>" -` | Both start the plugin they name. The program's module is joined before the full parse and its declarations are honored as a config endpoint's are | Offline `ze bgp decode --plugin` and `ze plugin test --plugin` keep accepting external decoders |
| AC-12 | An ExaBGP config with `process`, `api { processes [...] }`, and a script that creates a neighbor at run time, through `ze exabgp migrate` and `ze start` | The migration emits a config that loads with no edit: one endpoint per process running the wrapper, with `from` and `to` naming the neighbors that named it, and no block for an unscoped process. A script is written the events of those neighbors; a command with no neighbor selector reaches those neighbors; a neighbor that names a script and no message kind feeds it no event and still takes its routes; the `parsed`, `packets` and `consolidate` words keep their per-neighbor effect; a script holds the whole ExaBGP command set. Pending the owed ExaBGP design | Every case of `./le test functional exabgp` that covers per-neighbor scoping keeps its expectations |
| AC-13 | `bus { endpoint my-ddos { internal ddos-flowspec; } }`, the same with `process separate;`, and `bus { endpoint ddos-flowspec { external { run ...; } } }` | Both compiled endpoints read and write the plugin's state key, report healthy in `show doctor`, hold the same registered wiring, and block `ze data rm` while running. The foreign program holds none of the compiled plugin's grants. The five sibling sites (`RequiresPeerUpBarrier`, `configRefusalIsFatal`, `isPluginLoaded`, `collectOrphanedDependencies`, `appendHeldNeedProviders`) resolve by registration name | `process separate` on a plugin whose registration sets `RunsInProcessOnly` is refused at commit with that reason |
| AC-14 | A redistribution rule importing from and exporting to BGP, a FlowSpec family, `aigp` written on a peer, and a filter member owned by a plugin: first with nothing written, then with an override that removes what the feature needs | With nothing written all four work. With the override, validate, commit, reload and start refuse with one message naming the feature, the participant, the removing line and the completing line as a typed fix; completing it loads | An unrelated participant with an empty allow-list is not refused |
| AC-15 | iBGP peers with no `aigp` written and the Loc-RIB's reception cut by an override | The config loads. On the default configuration with nothing written, a route carrying AIGP is compared by AIGP | Only written `aigp` activates the requirement |
| AC-16 | Every tracked `.conf`, `.ci`, `.et`, interop scenario, fixture, doc and website example | `TestRepositoryConfigsValidate` and the scoped functional suites pass, and the converted interop scenarios pass against their peers. None keeps `plugin {`, `attach process` or `exabgp { bridge` outside `plan/` history, and the converter wrote no `bus` block by default | History under `plan/journal/`, `plan/learned/`, `plan/handover/` and `plan/records/` may quote the old syntax |
| AC-17 | A broadcast, an addressed message and the send check, under a benchmark, with `to` and `from` lists naming a peer, a group and `/bgp` | Each delivers to the receivers the lists select, at zero allocations per operation. No path text is parsed or compared per message | None: the lists are resolved at build |
| AC-18 | `ze config validate` with no daemon, on a config with an external endpoint: with a program that answers in query mode, with a list naming a message that program did not declare, and with a `run` line that names no program | The first validates. The second is refused with what the program declared. The third fails naming the endpoint, the `run` line, the cause and `ze show plugin conformance run "<run line>"`. The program's run loop never starts | A running daemon is still asked first |
| AC-19 | A route server with three connected clients; a reflector with a client and a non-client; both plugins loaded on one box with client and internal peers; nothing written | The route server forwards a client's routes to every other client. The reflector applies RFC 4456 Section 6. With both loaded, each peer receives each route once and one replay at peer-up | An export policy on one peer still keeps a route from it |
| AC-20 | A commit that deletes a group, a peer or an endpoint that a `to` or `from` names, in the editor, by reload and by validate of the candidate | Refused, naming the line and the deleted entry. Changing the line in the same commit makes it pass | Deleting an entry no list names is accepted |
| AC-21 | A dynamic group, a receiver whose `from` names that group's path, and a peer that connects from the group's range after start | The dynamic peer's `bgp:state` and updates are delivered to that receiver with no config change. A dynamic peer of another group delivers nothing to it | A list naming one range-admitted peer follows Q-6 |
| AC-22 | A list naming a peer by name, the same peer by address, a group, and `/bgp` | Name and address give the same filter. The group covers its peers; `/bgp` covers every peer. `show config` prints the path as written | None: both spellings are accepted |
| AC-23 | Implied endpoints, registered wiring, a written `to`, an inherited list, a required delivery and a dormant reception, viewed with `\| implied` on the CLI (three config commands), the editor, the web inspection, the workbench, REST, gRPC and MCP | Every item is shown with its origin and matches what runs. Beside a written `to` the replaced registered destinations are shown. A human display carries the notice; machine outputs do not. With `\| json` the envelope holds `config` and `implied`; without `\| implied` the output is byte-identical to today's | A written endpoint appears once, as written. gNMI `Get` stays written-only |
| AC-24 | `show config source <file> \| implied` with no daemon, for a file with an external endpoint: with a program that answers in query mode, and with a `run` line that names no program | The first shows the program's registered wiring as implied, and its run loop never starts. The second is refused, naming the endpoint, the `run` line, the cause and the conformance command | None: a partial set is never shown |
| AC-25 | A running daemon with edges of every origin | `show event delivery` lists each edge once with its origin and both participant paths, across every namespace, and `ze config graph` draws the same set | None: every edge carries an origin |
| AC-26 | A program Ze did not ship dispatches `clear bgp peer`, which it did not declare; nothing written | Refused with the "did not declare" message. The counter increments once and doctor reports `CodeDoctorPluginRequestRefused` for that participant. Against today's code this test is red: the command runs | Operators on every surface are unaffected |
| AC-27 | The program declares `cli:show bgp peer` and `cli:clear bgp peer`; the operator writes a send allow-list with the first, then an empty one | With the first list `show` passes and `clear` is refused with the allow-list message and its fix line. With the empty list both are refused. With none both pass, and `show interface` is refused as undeclared | Only declared and admitted paths pass |
| AC-28 | A program calls `route-install`, `route-remove` or `inject-wire-route` without declaring the message, then with it | Refused without; succeeds with | None: every such operation is gated |
| AC-29 | A compiled-in plugin, in the daemon and under `./le mock plugin-external`, runs its normal work; then it is given an allow-list that removes a command it uses | Every command succeeds in both transports with nothing written. With the list the command is refused in the same message shape as AC-27 | None: a compiled-in plugin refused a command it uses with nothing written is a defect of its declaration |
| AC-30 | A plugin calls `emit-event` for a message it did not register as a send | Refused. With the send registered, the message is delivered to its registered receivers | None: an emitter owns what it registered |
| AC-31 | A reload adds, changes and removes a list or an allow-list entry while the participant runs | The next message follows the new filter, and the participant and every BGP session keep running across the reload | None: a filter change restarts nothing |
| AC-32 | Every entry of `engineOps` and every operation registered through `RegisterEngineOp` | The walk test passes over both registries and reports, for each operation, the message it carries or its written reason. A test entry with neither makes the walk fail, naming that entry | An entry with `none` and a reason is permitted because the reason is reviewed with the entry |
| AC-33 | An ExaBGP script sends a line the wrapper turns into a declared command; then the operator narrows that script's send allow-list | The first succeeds with nothing written. After the narrowing the line is refused and the script receives the error ExaBGP's API contract gives for a failed command | A line addressed to a neighbor outside the script's `to` reaches no peer, by the filter |
| AC-34 | The plugin-less in-process RPC identity | Its commands are authorized as today, and every caller of it is listed with its reason | Permitted because it names no participant, so no filter exists to judge it |
| AC-35 | A plugin process sends a message naming another participant as sender | The bus stamps the sender from the authenticated registration, and the message is filtered and delivered as the real sender's | A process that registered participants (BGP for its peers) names them |
| AC-36 | A dynamic group whose members need an implied endpoint | The endpoint is loaded and shown by `\| implied` | None: a dynamic group implies what a static one does |

## Worked scenarios

Twenty-five scenarios. Each becomes the named test; the implementer writes the test from the row. Common setup unless a row says otherwise: peers `edge1` (192.0.2.1) and `lab` (192.0.2.9) standalone, group `transit` with peers `t1` and `t2`; endpoint `collector`, a fixture program that registers the reception of `bgp:state` and `bgp:update-received` and writes each message it receives to a file the test reads; the Loc-RIB loaded with nothing written, named here `/bus/endpoint/bgp-rib`. The present and empty list is written `to [ ];` pending Q-1. A refusal's required content is listed; its literal text follows `ai/rules/user-facing-errors.md` and is fixed in the test when written.

| # | Configuration | Stimulus | Expected observation | Negative that discriminates | AC and test |
|---|---------------|----------|----------------------|-----------------------------|-------------|
| S1 | `bus { endpoint collector { external { run ./collector; } } }` and pure `bgp { }` | `edge1` establishes and sends one UPDATE; then a reload adds peer `new1`, which establishes | `collector` records `bgp:state` and `bgp:update-received` for `edge1`, then `bgp:state` for `new1`. `show config running \| implied` shows the collector's two receptions as registered | `collector` sends `bgp:update` (undeclared): refused, message names `collector` and `bgp:update`, counter 1 | AC-1; `test/plugin/bus-default-wiring.ci` |
| S2 | As S1, plus `bgp { peer lab { bus { to [ /bus/endpoint/collector ]; } } }` | `lab` and `edge1` each send one UPDATE | `collector` records both. The Loc-RIB holds `edge1`'s route and not `lab`'s. `\| implied` shows `lab`'s `to` as written and lists the Loc-RIB among the registered destinations it replaced | Without the line, the Loc-RIB holds `lab`'s route | AC-2, AC-23; `test/plugin/bus-to-replaces-default.ci` |
| S3 | As S1, plus `bgp { peer lab { bus { to [ ]; } } }` | `lab` establishes and sends one UPDATE | No participant records anything for `lab`: `collector` holds no `lab` message, the Loc-RIB no `lab` route. `show config` prints the empty list | `edge1`'s messages still arrive | AC-2; `test/plugin/bus-to-replaces-default.ci` (second case) |
| S4 | Three configs on `collector`: no `from`; `from [ /bgp/peer/edge1 ];`; `from [ ];` | `edge1` and `lab` each establish | First: both recorded. Second: `edge1` only. Third: none | In the second, the Loc-RIB still holds `lab`'s route: `from` on one receiver removes nothing from another | AC-3; `test/plugin/bus-from-limits.ci` |
| S5 | `bgp { peer lab { bus { to [ /bus/endpoint/bgp-rib ]; message bgp:state { to [ /bus/endpoint/collector ]; } } } }` | `lab` establishes and sends one UPDATE | `collector` records `bgp:state` for `lab` and no `bgp:update-received`. The Loc-RIB holds `lab`'s route | Removing the `message` block: `collector` records nothing for `lab` | AC-4; `test/plugin/bus-from-limits.ci` (per-message case) |
| S6 | `bgp { group transit { bus { to [ /bus/endpoint/bgp-rib ]; from [ /bus/endpoint/bgp-rib ]; } peer t1 { bus { to [ /bus/endpoint/collector ]; } } } }` | `t1` and `t2` each send one UPDATE | `t2`: Loc-RIB only. `t1`: `collector` only (the member's `to` replaces the group's). PENDING Q-8, both answers written: (a) per item: `t1` keeps the group's `from`, and `\| implied` shows it as inherited from `transit`; (b) whole block: `t1` has no `from`, so everybody may address it | Under (a) a third sender addressing `t1` is refused; under (b) it is delivered. The test asserts the answer Thomas gives | AC-2, AC-23; `test/plugin/bus-group-inheritance.ci` |
| S7 | A fixture endpoint `sender` that declares `bgp:update` and `bgp:refresh`: no `send`; `send { allow [ bgp:update ]; }`; `send { }`; `send { allow [ bgp:raw ]; }` | `sender` sends an update and a refresh to `edge1` | None: both reach `edge1`. Partial: the update arrives, the refresh is refused naming `sender`, `bgp:refresh` and the line `set bus endpoint sender send allow bgp:refresh`. Empty: both refused. The fourth config is refused at commit, listing `bgp:update` and `bgp:refresh` as what `sender` declared | The counter is 0, 1, 2 across the first three | AC-5, AC-6; `test/plugin/bus-allow-lists.ci`, `test/parse/bus-commit-refusals.ci` |
| S8 | On `collector`: no `receive`; `receive { allow [ bgp:state ]; }`; `receive { }` | `edge1` establishes and sends one UPDATE | None: both messages recorded. Partial: `bgp:state` only. Empty: nothing | The Loc-RIB still holds the route in all three | AC-5; `test/plugin/bus-allow-lists.ci` (receive case) |
| S9 | On `sender`: `send { allow [ bgp:update ]; } to [ /bgp/group/transit ];`; then the same with `message bgp:refresh { to [ /bgp/peer/edge1 ]; }` added | `sender` sends an update with no selector | First: `t1` and `t2` receive it, `edge1` and `lab` do not. Second: refused at commit, naming the `send allow` line and the `message bgp:refresh` line | In the first, a refresh is refused by the allow-list, not by `to` | AC-5, AC-6; `test/plugin/bus-allow-lists.ci`, `test/parse/bus-commit-refusals.ci` |
| S10 | The route server, four configs: (a) `bus { endpoint bgp-rs { internal bgp-rs; to [ /bgp/group/transit ]; } }`; (b) `bgp { route-server { bus { to [ /bgp/group/transit ]; } } }`; (c) both, same list; (d) both, the endpoint's list `/bgp/group/transit`, the section's `/bgp/peer/edge1` | Validate each; for (a) to (c), `\| implied \| json` | (a), (b), (c) load and the display shows one identical effective `to` for the route server, with origin naming the written line (both lines for (c)). (d) is refused at validate, commit and start, naming both lines and saying to keep one | The JSON of (a), (b) and (c) is equal under `implied` except for the origin field | AC-7, AC-6, AC-23; `test/plugin/bus-two-places.ci` |
| S11 | `bus { endpoint acme { external { run ./acme; } send { } receive { allow [ bgp:state ]; } } }`, where `acme` declares `bgp:update` and receptions of `bgp:state` and `bgp:update-received` | `edge1` establishes and sends one UPDATE; `acme` sends an update | `acme` records `bgp:state` only. Its update is refused with the allow-list message. `\| implied` shows both lists as written and what they removed | With the two lines deleted, `acme` records both messages and its update reaches `edge1` | AC-5, AC-23; `test/plugin/bus-central-limit.ci` |
| S12 | On `collector`: `from [ /bgp/group/transit ];`, then `from [ /bgp/peer/192.0.2.1 ];`, then `from [ /bgp/peer/edge1 ];`, then `from [ /bgp ];` | All four peers establish | Group: `t1` and `t2`. Address and name: `edge1` only, with equal compiled filters. `/bgp`: all four. `show config` prints each path as written | `from [ /bgp/peer/nosuch ];` is refused at commit, listing the configured peers | AC-22, AC-6; `test/plugin/bus-path-forms.ci` |
| S13 | Two fixture endpoints `det1` and `det2`, both registered to receive `bgp:update-received` and both answering each route with a `cli:show bgp peer` command; and separately `bgp-rs` and `bgp-rr` loaded with client peers `c1`, `c2` (`session rs-client`) and internal peers `i1`, `i2`, nothing written | `edge1` sends one UPDATE; then `c1` and `i1` each send one route | Both detectors record the route and both commands succeed: no arbitration, no refusal, no commit error. `c2` receives `c1`'s route once and `i2` receives `i1`'s route once; each peer gets one replay at peer-up | `c2` does not receive `i1`'s route from the route server, and no peer receives any route twice (the peer in check mode fails on a second copy) | AC-19; `test/plugin/bus-no-arbitration.ci`, `test/plugin/bus-rs-rr-one-box.ci` |
| S14 | `bgp { route-server { bus { to [ /bgp/group/ix ]; } } }` with client groups `ix` and `ix2`; and `bus { endpoint bgp-rr { internal bgp-rr; to [ /bgp/group/core ]; } }` with internal groups `core` and `edge` | A peer of each group sends one route | The route server forwards to `ix` peers only; the reflector reflects to `core` peers only. `\| implied` shows each `to` as written at its own place | Without each line, `ix2` and `edge` peers receive the routes | AC-2, AC-7; `test/plugin/bus-plugin-own-section.ci` |
| S15 | `bus { endpoint my-ddos { internal ddos-flowspec; process separate; } }` | Start; the plugin writes its state key; `show doctor`; `ze data rm` of its key | The state write succeeds, doctor reports the plugin healthy under its registration name with alias `my-ddos`, `ze data rm` is refused while it runs, and `\| implied` shows the same registered wiring as the in-daemon run | `bus { endpoint ddos-flowspec { external { run ./fake; } } }`: the state write is refused and doctor reports the compiled plugin as not running | AC-13; `test/plugin/bus-aliased-forked-state-doctor.ci` |
| S16 | `bus { endpoint acme { external { run ./acme; } from [ /bgp/peer/edge1 ]; } }`, no daemon; `acme` built on `sdk.RunOrDeclare`, its activation writes a marker file | `ze config validate site.conf`; the same with `receive { allow [ bgp:nosuch ]; }`; the same with `run ./missing`; then `ze --plugin "./acme" -` | First: valid. Second: refused, listing what `acme` declared. Third: fails naming `acme`, `./missing`, "not found" and `ze show plugin conformance run "./missing"`. Fourth: the daemon starts `acme` and it receives `edge1`'s state | The marker file is never written by the three validate runs | AC-18, AC-11, AC-24; `test/parse/bus-offline-validate-query-mode.ci`, `test/plugin/external-yang-command-line.ci` |
| S17 | An ExaBGP config with processes `p1` and `p2`, neighbor A naming `p1`, neighbor B naming `p2`, through `ze exabgp migrate` | Load the output unedited; A and B establish; `p1` announces a route with no neighbor selector | The output holds `bus { endpoint p1 { external { run "ze exabgp plugin ..."; } from [ /bgp/peer/<A> ]; to [ /bgp/peer/<A> ]; } }` and the same for `p2` with B, and no `exabgp { }` section. `p1` is written A's events only. The route reaches A | B receives no UPDATE, and `p2` is written nothing about A. Pending the owed ExaBGP design | AC-12, AC-33; `test/exabgp/exabgp-script-endpoints.ci` |
| S18 | A redistribution rule importing from BGP, plus `bgp { peer edge1 { bus { to [ /bus/endpoint/collector ]; } } }` | `ze config validate`, editor commit, reload, `ze start` | Each refuses with one message naming redistribution, `edge1`, the `to` line, and the completing line that adds the Loc-RIB to it, as a typed fix. With that line the config loads and `edge1`'s route is redistributed | The same `to` with no redistribution rule loads | AC-14; `test/parse/bus-requirement-refusals.ci` |
| S19 | Running daemon from S1. Reload 1 adds `from [ /bgp/peer/edge1 ];` on `collector`; reload 2 changes it to `lab`; reload 3 removes it; reload 4 deletes peer `lab` while a list names it | After each reload both peers send one UPDATE | After 1: `edge1` only. After 2: `lab` only. After 3: both. No session resets across 1 to 3 (the peers in check mode see no NOTIFICATION or reconnect). Reload 4 is refused, naming the `from` line and `lab` | Reload 4 with the line changed in the same commit passes | AC-31, AC-20; `test/reload/bus-list-reload.ci` |
| S20 | A fixture program that declares only `cli:show bgp peer`, nothing written | It dispatches `show bgp peer`, then `clear bgp peer`, then calls `route-install`, then `emit-event` of `bgp:state` | The first succeeds. The other three are refused at the bus with the "did not declare" message; `show doctor` reports `CodeDoctorPluginRequestRefused` for the participant and the refusal counter reads 3. Against today's code `clear bgp peer` runs: the test is recorded red first | An operator running `clear bgp peer` over SSH is authorized as before | AC-26, AC-28, AC-30, AC-32, AC-34; `test/plugin/bus-send-undeclared-refused.ci`, `TestEveryEngineOpDeclaresItsMessage` |
| S21 | An endpoint whose program registers a reception of `acme:alert`, with no sender of it configured; reload adds the endpoint that sends it | Start; `show doctor`; `\| implied`; reload; the sender emits one `acme:alert` | Before: the config loads, doctor and the display name the reception as dormant with the missing sender. After: the alert is recorded and `show event delivery` shows the edge as registered | A written `from [ /bus/endpoint/nosuch ];` is refused, not dormant | AC-8, AC-25; `test/plugin/bus-dormant-reception.ci` |
| S22 | Dynamic groups `dyn1` and `dyn2` with ranges; `collector` with `from [ /bgp/group/dyn1 ];`; a feature block under `dyn1` that implies a plugin | A peer connects from each range after start | The `dyn1` peer is recorded with no config change; the `dyn2` peer is not. The implied endpoint the `dyn1` feature needs is loaded and shown by `\| implied` | Without the fix to the dynamic-group collectors the implied endpoint is absent | AC-21, AC-36; `test/plugin/bus-dynamic-peer.ci` |
| S23 | `bus { hub { server ...; } }`; and four configs using `plugin { }`, `attach process`, `process` under `bgp`, `exabgp { bridge }` | Start the first, run `show listeners` and `config graph`; validate the four | `show listeners` reports `bus-hub` and the graph links `bus` to it. Each of the four is refused at parse with a hint naming `bus` | `plugin-hub` appears nowhere in the output | AC-9, AC-10; `test/managed/bus-hub-listener.ci`, `test/parse/bus-commit-refusals.ci` |
| S24 | The tracked corpus; and an index with lists naming a peer, a group and `/bgp` | `TestRepositoryConfigsValidate`; the three benchmarks | Every tracked config validates and none holds the old syntax or a converter-written `bus` block. Each benchmark reports zero allocations per operation | A benchmark variant that parses a path per message fails the allocation assertion | AC-16, AC-17; `TestRepositoryConfigsValidate`, `TestBusDeliveryAllocatesNothing` |
| S25 | Two fixture endpoints `a` and `b`; `collector` with `from [ /bus/endpoint/a ];` | `b` sends a message whose content names `a` as sender | `collector` records nothing: the bus stamped `b`. A message from `a` is recorded with sender `a` | With the stamp taken from content the message would be recorded | AC-35; `TestSenderIsStampedByTheBus`, `test/plugin/bus-sender-stamp.ci` |

Coverage: AC-1 S1; AC-2 S2, S3, S6, S14; AC-3 S4; AC-4 S5; AC-5 S7, S8, S9, S11; AC-6 S7, S9, S10, S12; AC-7 S10, S14; AC-8 S21; AC-9 S23; AC-10 S23; AC-11 S16; AC-12 S17; AC-13 S15; AC-14 S18; AC-16 S24; AC-17 S24; AC-18 S16; AC-19 S13; AC-20 S19; AC-21 S22; AC-22 S12; AC-23 S2, S6, S10, S11; AC-24 S16; AC-25 S21; AC-26 S20; AC-28 S20; AC-30 S20; AC-31 S19; AC-32 S20; AC-33 S17; AC-34 S20; AC-35 S25; AC-36 S22. Three criteria have no scenario row yet and owe one each, which a continuation writes: AC-15 (AIGP at its default), AC-27 (a `cli` allow-list, partial and empty) and AC-29 (a compiled-in plugin on both transports). Until then they are covered by `TestAIGPDefaultActivatesNoRequirement` with `TestAIGPDefaultPathStillSelectsByAIGP`, `test/reload/bus-send-allow-list-reload.ci`, and `test/plugin/bus-send-compiled-in-keeps-commands.ci`.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Declares an external collector as an endpoint and writes nothing else | Stage 1 declaration -> registered wiring -> delivery for every peer | `test/plugin/bus-default-wiring.ci` |
| 2 | Keeps a lab peer's routes out of the RIB with one `to` line on the peer | Inheritance -> compiled filter -> index | `test/plugin/bus-to-replaces-default.ci` |
| 3 | Limits a third-party program from the bus side | Endpoint fields -> compiled filter -> engine-operation check | `test/plugin/bus-central-limit.ci` |
| 4 | Asks what Ze does on their behalf | `show config running \| implied` -> compiled filters | `test/plugin/bus-implied-view.ci` |
| 5 | Runs a compiled-in plugin as a separate process under a name of their own and checks `show doctor` | Endpoint `process separate` -> fork of Ze's own binary -> identity by registration | `test/plugin/bus-aliased-forked-state-doctor.ci` |
| 6 | Migrates an ExaBGP config with processes and runs it | `ze exabgp migrate` -> `ze start` -> the wrapper | `test/exabgp/exabgp-script-endpoints.ci` |
| 7 | Configures the managed hub under `bus { hub }` | First pass -> `ExtractHubConfig` -> `show listeners` | `test/managed/bus-hub-listener.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBusFilterCompilation` | `internal/component/plugin/server/bus_filter_test.go` | AC-1 to AC-5, AC-7: the six conditions; registration, bus side and participant side into one filter; block-level against per-message; absent, partial and empty lists | |
| `TestBusCommitRefusals` | `internal/component/plugin/bus_validate_test.go` | AC-6: one case per refusal, each asserting the lines and the valid set in the message | |
| `TestBusBlockInheritance` | `internal/component/bgp/config/resolve_bus_test.go` | AC-2, S6: a member's list replaces its group's whole; the replace-whole marker is read from the schema. Negative: an unrelated container still merges key by key | |
| `TestBusPathForms` | `internal/component/plugin/bus_path_test.go` | AC-22: name and address give one participant; a shorter path covers what is under it. Negative: a path to no participant is refused with the configured entries | |
| `TestDeliveryIndexFromFilters` | `internal/component/plugin/server/delivery_graph_test.go` | AC-1, AC-21: a participant added later is covered; a dynamic peer is covered through its group's path. Negative: a peer of another group is not | |
| `BenchmarkBusDelivery`, `TestBusDeliveryAllocatesNothing` | `internal/component/plugin/server/delivery_graph_test.go` | AC-17: broadcast, addressed message, send check | |
| `TestSendsCompleteBeforeReceptions` | `internal/component/plugin/server/startup_bus_test.go` | M-4: a reception resolved at startup sees a held external program's send; the catalog reaches Stage 2 | |
| `TestDormantReception` | `internal/component/plugin/server/bus_filter_test.go` | AC-8 | |
| `TestRegistrationCarriesNoSecondDeclaration` | `internal/component/plugin/all/all_test.go` | No registration in the tree keeps a subscription call beside its `Receives` declaration | |
| `TestSenderIsStampedByTheBus` | `internal/component/plugin/server/bus_sender_test.go` | AC-35 | |
| `TestHubUnderBusListenerAndGraph` | `internal/component/config/listener_test.go` | AC-9 | |
| `TestRetiredKeywordHintsNameBus` | `internal/component/config/retired_test.go` | AC-10 | |
| `TestFirstPassMergesCommandLinePlugins` | `internal/component/config/first_pass_test.go` | AC-11: existing test, converted to endpoints, proving at least what it proves today | |
| `TestFirstPassRootsAreDeclared` | `internal/component/config/yang/join_test.go` | The first-pass roots are derived, and a module that declares none adds none | |
| `TestProcessIdentityByRegistration` | `internal/component/plugin/server/dispatch_state_test.go` | AC-13: aliased, aliased and forked, and foreign same-named | |
| `TestAliasedPluginLifecycleHelpers` | `internal/component/plugin/server/startup_alias_test.go` | AC-13: the five sibling sites | |
| `TestRequirementCheckRefusals` | `internal/component/plugin/server/bus_requirement_test.go` | AC-14: one case per feature, each asserting the fix line | |
| `TestAIGPDefaultActivatesNoRequirement` | `internal/component/bgp/config/aigp_binding_test.go` | AC-15 | |
| `TestAIGPDefaultPathStillSelectsByAIGP` | the Loc-RIB plugin's package | AC-15, the companion assertion on the default configuration | |
| `TestNoPluginNameLiteralInBGPConfig` | `internal/component/bgp/config/` | The three literals of `requireRIBDelivery` are gone; `./le arch enumeration check` judges the same | |
| `TestRepositoryConfigsValidate` | `internal/component/config/cli/validate_corpus_test.go` | AC-16 | |
| `TestOfflineValidateAsksProgramInQueryMode`, `TestOfflineQueryFailureNamesItsCause` | `internal/component/config/declared_schemas_test.go` | AC-18: the three cases, and one case per cause; the fixture's activation function never runs | |
| `TestRouteServerServesItsClientSet` | `internal/component/bgp/plugins/rs/server_forward_test.go` | AC-19: every connected client except the source is a target. Negative: a peer outside the client set is not | |
| `TestReflectorServesInternalPeers` | `internal/component/bgp/plugins/rr/rr_test.go` | AC-19: RFC 4456 Section 6 over internal peers. Negative: an external peer is not a target, and a non-client's route does not reach another non-client | |
| `TestBusReferenceWalk` | `internal/component/config/yang/validator_instance_test.go` | AC-20: deleting an entry a list names is refused with the referring line | |
| `TestImpliedViewMatchesCompiledFilters` | `internal/component/plugin/server/` | AC-23, R-11: the view and the runtime read one set | |
| `TestImpliedShowsReplacedDefaults` | `internal/component/config/cli/` | AC-23: beside a written `to`, the registered destinations it replaced; each effective entry names its producing line | |
| `TestImpliedEnvelopeKeepsConfigPure`, `TestImpliedTextRoundTrips` | `internal/component/config/cli/`, `internal/component/config/` | AC-23, R-12 | |
| `TestOfflineImpliedRefusesUnknownDeclaration` | `internal/component/config/cli/` | AC-24 | |
| `TestDeliveryEdgeOrigins` | `internal/component/plugin/server/` | AC-25 | |
| `TestPluginIdentityIsAuthorizedByProfile` | `internal/component/authz/` | AC-26, AC-27: no prefix allowance for a plugin identity; absent, partial and empty lists | |
| `TestInternalRPCIdentityStaysAllowed` | `internal/component/authz/` | AC-34 | |
| `TestEveryEngineOpDeclaresItsMessage` | `internal/component/plugin/server/` | AC-32: the walk over both registries, and a fixture entry with neither that fails it | |
| `TestRefusalNamesItsReason` | `internal/component/plugin/server/` | The three messages, the counter and the doctor event | |
| `TestCompiledInDeclarationsCoverWhatTheySend` | `internal/component/plugin/all/` | AC-29: each registration's `Sends` covers the command strings its package sends | |
| `TestEmitNeedsARegisteredSend` | `internal/component/plugin/server/` | AC-30 | |
| `TestDynamicGroupImpliedEndpoints` | `internal/component/bgp/config/` | AC-36 | |
| `TestMigrateWritesScriptEndpoints`, `TestWrapperDeclaresWhatItsTranslatorEmits` | `internal/exabgp/migration/migrate_bus_test.go`, the wrapper's package | AC-12, AC-33; pending the owed ExaBGP design | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `bus > endpoint > external > timeout` | Existing range kept from `plugin > external > timeout` | Unchanged | Unchanged | Unchanged |
| `bus > hub > server/client > port` | Existing `1..65535` kept from `plugin > hub` | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bus-default-wiring` | `test/plugin/` | S1 | |
| `bus-to-replaces-default` | `test/plugin/` | S2, S3 | |
| `bus-from-limits` | `test/plugin/` | S4, S5 | |
| `bus-group-inheritance` | `test/plugin/` | S6 | |
| `bus-allow-lists` | `test/plugin/` | S7, S8, S9 | |
| `bus-commit-refusals` | `test/parse/` | S7, S9, S23 and every refusal of AC-6, each with its corrected config as the negative control | |
| `bus-block-commit` | `test/editor/` | AC-6, AC-20 through the editor | |
| `bus-two-places` | `test/plugin/` | S10 | |
| `bus-central-limit` | `test/plugin/` | S11 | |
| `bus-path-forms` | `test/plugin/` | S12 | |
| `bus-no-arbitration`, `bus-rs-rr-one-box` | `test/plugin/` | S13 | |
| `bus-plugin-own-section` | `test/plugin/` | S14 | |
| `bus-aliased-forked-state-doctor` | `test/plugin/` | S15 | |
| `bus-offline-validate-query-mode` | `test/parse/` | S16 | |
| `external-yang-command-line` (existing) | `test/plugin/` | S16, AC-11 | |
| `exabgp-script-endpoints` | `test/exabgp/` | S17 | |
| `bus-requirement-refusals` | `test/parse/` | S18, AC-15 | |
| `bus-list-reload`, `bus-send-allow-list-reload` | `test/reload/` | S19, AC-27, AC-31 | |
| `bus-send-undeclared-refused` | `test/plugin/` | S20, the security negative, recorded red first | |
| `bus-send-compiled-in-keeps-commands` | `test/plugin/` | AC-29, in the daemon and forked | |
| `bus-send-refusal-doctor` | `test/plugin/` | The doctor code and the counter through `show doctor` and the metrics endpoint | |
| `bus-dormant-reception` | `test/plugin/` | S21 | |
| `bus-dynamic-peer` | `test/plugin/` | S22 | |
| `bus-hub-listener` | `test/managed/` | S23 | |
| `bus-sender-stamp` | `test/plugin/` | S25 | |
| `bus-implied-view`, `bus-delivery-origins` | `test/plugin/` | AC-23, AC-25 through the running daemon | |
| `bus-implied-offline` | `test/parse/` | AC-24 | |
| `bus-implied-editor` | `test/editor/` | AC-23: editor `show \| implied` and the status-bar notice | |
| `bus-implied-workbench` | `test/web/` | AC-23: the inspection view and the read-only workbench rows | |

### Tests routed through the test judge

Each of these proves something the rework changes or removes. The author does not judge them (`ai/META-DECISIONS.md`, "Test changes"). Under M-12 the converter writes nothing, so a test of a difference is restated with a `to` or `from` list, which makes the per-peer cases expressible again.

| Test | What it proves today | What the rework does to it |
|------|----------------------|----------------------------|
| `test/plugin/attach-process-send-permission.ci` | A process is refused a send toward a peer that does not grant it, and an unattached process is silent | Restated: the same process with a `to` that omits that peer, and an endpoint with `from [ ];` |
| `test/plugin/bgp-rs-fastpath-relay-grants.ci` | The route server relays only toward peers that grant it `send [ update ]`: destination E is refused, peer N is receive-only | Restated with a `to` on the route server that omits E, and N outside its `from` |
| The ten detector tests: `test/plugin/nexthop-auto-rs-client-unchanged.ci`, `nexthop-auto-ebgp-rewrite.ci`, `bgp-rs-rpki-revalidate-relay.ci`, `bgp-rs-rpki-revalidate-relay-forked.ci`, `bgp-rs-rpki-pending-withdraw-relay.ci`, `bgp-rs-replay-late-destination.ci`, `bgp-rr-rpki-revalidate-relay.ci`, `bgp-rr-rpki-pending-withdraw-relay.ci`, `bgp-rr-replay-late-destination.ci`, `adj-rib-in-filtered-not-replayed.ci` | Each carries `reject=stderr:pattern=send refused`, so a relay sent under the name of a plugin that holds no grant on that peer turns the test red | Each keeps its detector by a `to` on the plugin that must not relay to that peer, or by that plugin's session-type limit; the refusal text changes with the one message shape. `bgp-adj-rib-in` declares `bgp:update`, because it replays today |
| About 40 route-server and reflector `.ci` files with a receive-only source peer in check mode | The source peer receives no relayed UPDATE, because it granted the forwarder no `send [ update ]` | Each is restated with a `to` on the forwarder or keeps its expectation through the forwarder's session-type limit; each is listed before the converter runs |
| `test/exabgp-compat/native/api-multiple-api.conf` | Two plain external scripts with per-neighbor `api { processes }` scope | Each script is an endpoint with `from` and `to` (E-1) |
| `internal/component/bgp/reactor/send_permission_test.go` (6 tests), `send_permission_rails_test.go` (6), `forward_rs_permission_test.go` (4), `source_receipt_bound_grant_test.go` | The ten send rails share one per-peer filter, and a source receipt is bound to the per-peer grant | The rails share the compiled filter; each per-peer case is rebuilt with a `to` list |
| `internal/component/bgp/config/config_test.go`, `redistribute_binding_test.go`, `aigp_binding_test.go`, `flowspec_binding_test.go`, `delivery_graph_test.go`; `internal/component/bgp/reactor/delivery_graph_test.go`; `internal/component/plugin/server/delivery_graph_test.go`, `dissemination_test.go`, `dissemination_config_test.go`, `dissemination_validate_test.go`; `internal/component/bgp/plugins/rib/rfc8212_write_refused_test.go`; `internal/component/bgp/plugins/filter_irr/readmit_test.go` | Each builds or asserts per-peer `send` grants | Rewritten on the compiled filter; each judged for proving no less. A changed RFC-tagged test re-takes its record (`ai/rules/rfc-compliance.md`) |
| The receive census tests (listed under "Receive census") | A program is not fed a peer that does not attach it | Restated with a `from` on the program or a `to` on the peer |
| `test/plugin/external-yang-command-line.ci`, `TestFirstPassMergesCommandLinePlugins` | A program named with `--plugin` gives the same outcome as one in the config | Converted only; listed so the conversion is judged as not weakening |
| The AIGP tests that assert a refusal with `aigp` at its default | A narrowed Loc-RIB is refused whenever an iBGP peer exists | The refusal applies only with `aigp` written (S-8); the old expectation is shown wrong against the owner decision |

### Per-peer send census

Taken 2026-11-23 over every tracked config, `.ci`, interop scenario and doc that carries `attach process`. A file is asymmetric when a process holds a `send` list on one peer and a different list, or none, on another.

| Where | Files with a per-peer send asymmetry |
|-------|--------------------------------------|
| `test/plugin/` | 124 |
| `test/interop/scenarios/` | 28 |
| `test/draft/plugin/` | 17 |
| `test/reload/` | 14 |
| `docs/guide/` (`bgp-peering.md`, `configuration.md`, `plugins.md`) | 3 |
| `test/exabgp-compat/native/` (`api-blocklist.conf`, `api-multiple-api.conf`) | 2 |
| `test/draft/reload/` | 2 |
| `test/ui/cli-completion-full-config.ci`, `test/perf/configs/ze.conf`, `docs/config-reference.md` | 1 each |
| Total | 193 |

The 28 interop scenarios: `as-path-mixed-width-relay-frr`, `bgp-addpath-readvertise-collision-frr`, `bgp-aggregator-as4-downgrade-bird`, `bgp-atomic-policy-admission-frr`, `bgp-attr-set-included-as-set-frr-bird`, `bgp-best-route-transit-bird`, `bgp-best-route-transit-frr`, `bgp-flowspec-gobgp`, `bgp-graceful-restart-frr`, `bgp-large-community-no-repeat-gobgp`, `bgp-med-across-as-gobgp`, `bgp-med-ibgp-ordinary-post-selection-removal-gobgp`, `bgp-med-ibgp-post-selection-removal-gobgp`, `bgp-med-remove-configured-gobgp`, `bgp-nexthop-auto-ebgp-frr`, `bgp-nexthop-self-local-auto-frr`, `bgp-paths-limit-best-path-frr`, `bgp-prefix-sid-duplicate-tlv-frr`, `bgp-prefix-sid-malformed-frr`, `bgp-rfc7606-relay-shape-frr`, `bgp-rfc7999-blackhole-readvertise-frr`, `bgp-rfc9774-as-set-frr`, `bgp-role-export-roleless-destination-frr`, `bgp-role-rs-client-learned-not-upward-frr`, `bgp-route-server-replacement-frr`, `bgp-srv6-next-hop-identity-frr`, `bgp-srv6-next-hop-identity-unchanged-frr`, `bgp-vpn-frr`. Each is a scenario whose config the converter changes beyond syntax, so each is named in the interop goal gate.

| Answer | Files | Conversion |
|--------|-------|------------|
| A test of the difference | The two refusal tests, the ten detector tests, `api-multiple-api.conf` and about 40 route-server and reflector tests in check mode | Restated with a list, through the test judge |
| Different lists per peer and no test of the difference | The remaining asymmetric files | Nothing is written: the plugin sends what it declared (M-12) |
| The same list on every peer | Every other file | Converts to nothing, or to one send allow-list where the list is narrower than the declaration |

Per-peer grant read sites, each with its successor:

| Read site | File (symbol) | What it decides today | Per the design |
|-----------|---------------|-----------------------|----------------|
| `filterPermittedPeers` | `internal/component/bgp/reactor/reactor_api.go` (two calls), `reactor_api_forward.go`, `reactor_api_forward_batch.go`, `reactor_api_relay.go`, `reactor_api_selected.go`, `aigp_readvertise.go`, `origin_receipt.go`, `destination_marker.go`, `destination_constraint.go` | Drops each target peer that does not grant the sender the send type | Reads the sender's compiled filter: all peers, or the set its `to` and the peers' `from` leave |
| `bindingsMaySend` | `reactor/origin_runtime.go` (`originWithdrawal`), `reactor/origin_grants.go` (`revokeRemovedOriginGrants`) | Whether an origin withdrawal is allowed on a peer; revocation after a reload removed the grant | Reads the filter; revocation follows a reload that removes the peer from the sender's filter |
| `DeliveryGraph.MaySend` | `internal/component/bgp/server/events.go` (`peerStateUnheldRoles`) | Whether a fed claimant holds the replay role for a peer | Reads the filter |
| `DeliveryGraph.MaySend` | `internal/component/plugin/server/request_cancel.go` (`selectedCancellationScope`), `dispatch_selected_refresh.go` | A `select-routes` or `select-refresh` destination needs the caller's `update` grant; the refusal names the `attach process` line | Reads the filter; the refusal takes the one message shape |
| `DeliveryGraph.MaySend` | `internal/component/plugin/server/dissemination.go`, `dissemination_validate.go` | A peer-output role's grants, judged per peer | Replaced by the declared feature requirement and its one check |
| `requireRIBDelivery` | `internal/component/bgp/config/flowspec_binding.go` | Finds a forwarder that receives `state` and holds `send [ update ]` on the peers | Replaced by the declared feature requirement |
| `ProcessBinding.MayPushRoutes` | `reactor/peer_settings.go`, read by `Peer.initialUpdateReporters` (`reactor/peer_run.go`) | Which processes count as reporters of a session's initial update | Reads the filter: a plugin that may push routes to the peer, is fed its state and signals session-ready. The forwarders' session-type limit keeps that set to the plugins that serve the peer |
| `ProcessBinding.MaySend(update)` | `reactor/peer_bound_info.go` (`boundPeerInfo`), `reactor/peer_forward_facts.go` (`rsRelaySendProcessName`), `reactor/policy_reload.go` (`Peer.policyReplayOwner`) | Each picks the one process that holds `send [ update ]` on the peer | Reads the filter. The pick stays "first in order"; it is single-valued because only one forwarder serves a peer after the session-type limit. Two forwarders an operator deliberately points at one peer both serve it, which M-11 accepts (Q-9) |

### Receive census

Taken 2026-11-23 with the per-peer send census, over the same files.

| Finding | Files |
|---------|-------|
| A program attached to fewer than all peers, or with different `receive` lists per peer | 186 |
| of which a program is attached to some peers only | 125 |
| of which the `receive` lists differ per peer | 109 (a file can be in both) |
| A declared external plugin that no peer attaches | 673: the upper bound of the programs that start receiving every peer |

The tests that assert a program did not receive a peer's event go to the test judge:

| Test | What it proves today | What restates it |
|------|----------------------|------------------|
| `test/plugin/attach-process-unattached-is-silent.ci` | An unattached program receives nothing | `from [ ];` on its endpoint (S4) |
| `test/plugin/attach-process-receive-filter.ci`, `redistribute-explicit-attach-kept.ci`, `bgp-best-route-attach-isolation.ci`, `bgp-best-route-start-destination-send-only.ci` | A program is fed only the events or peers its attachment names | A receive allow-list and a `from` list on the program (S4, S8) |
| `test/plugin/attach-process-dynamic-group.ci` | A dynamic peer is fed to a program through its group's attachment | A `from` naming the group's path (S22) |
| `test/plugin/attach-process-delivery-graph.ci` | The graph holds one edge per attached peer | `TestDeliveryIndexFromFilters` |
| `test/plugin/attach-process-runtime-subscribe.ci` | `request subscribe` adds a type only inside the peer's grant | Bounded by the registration and the receive filter |
| `test/plugin/adj-rib-in-replay-unowned-peer.ci` | `bgp-adj-rib-in` replays to a peer no forwarder owns | A peer outside the forwarder's session type, or outside its `to`: the forwarder's claim is retracted for it and `bgp-adj-rib-in` replays |
| `test/reload/bgp-attach-receive-only-live.ci`, `bgp-attach-dynamic-receive-only-live.ci`, `bgp-attach-grant-change-live.ci`, `bgp-attach-no-route-refresh-restarts.ci`, `bgp-attach-sent-direction-restarts.ci`, `bgp-rpki-attach-live.ci` | A reload that changes a peer's attachment changes delivery live, or restarts the session where it must | A reload that changes a list (S19) |
| `test/interop/scenarios/bgp-attach-receive-only-live-frr` | The same against FRR | A reload that changes a list, rerun against FRR |
| `test/reload/reload-exabgp-bridge-capability-accepted.ci`, `test/plugin/rfc8212-ebgp-no-export-policy.ci` | The bridge attached to some peers only | The script endpoint's `from` (E-1) |

### Fixture command census

Every external test fixture and every in-tree plugin needs a declaration that covers what it sends, or its tests fail closed. Taken 2026-11-23, static:

| Measure | Result |
|---------|--------|
| Fixtures the functional tests start | 827 fixture names, used by 1,093 `.ci` files |
| Where they are registered | 167 source files under `internal/test/fixture/`; 796 names resolved to their source by name, 31 not resolved statically |
| SDK calls in the fixture sources | `UpdateRoute` 51, `DispatchCommand` 48, `RouteMetrics` 7, `RouteInstall` 6, `ReleaseCached` 4, `EmitEvent` 4, `ForwardCached` 3, `RouteRemove` 3, the `State*` calls 5, `RaiseIssue` and `ClearIssue` 3 |
| Literal command prefixes they dispatch | Led by `show bgp` (67), `request shutdown` (22) and `request quiesce` (21) |
| Why the static census is not the per-fixture set | Most fixtures dispatch through about 25 shared wrapper helpers, so a fixture's source does not show which commands it sends |

The complete per-fixture set comes from a runtime capture through the dispatcher's existing plugin accounting (`Username` from `internalPluginIdentity(proc.Name())` and `Sender` from `plugin.ProcessSender(proc.Name())`). A capture over the functional suites records, for each plugin name, every command path and engine operation it used, and the declarations are written from it once.

### ExaBGP bridge section census

Not taken (row X-6 of "ExaBGP"): the configs, tests and docs that use `exabgp { bridge }` or the internal bridge mode.

### Interop Tests (Scope: protocol)
No wire format changes. Which plugin sees and sends what does change, so the converted scenarios under `test/interop/` run against their peers and that run is a goal gate. The run names each scenario whose config the converter changed beyond syntax, the 28 above included. The route server and reflector session-type limit is proven against FRR by `bgp-route-server-replacement-frr` and one reflector scenario named at implementation.

## Files to Modify
- `internal/component/plugin/yang/ze-plugin-conf.yang` - replace `container plugin` with `container bus` (`endpoint`, `hub`); define the bus grouping and its augmentation onto registered participant nodes
- `internal/component/bgp/yang/ze-bgp-conf.yang` - remove `container attach`
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang`, `internal/component/bgp/plugins/cmd/peer/create.go` - remove the `attach` keyword and `setAttach`
- `internal/component/plugin/registry/registry.go` - the participant declarations replace `EventTypes` and `SendTypes`; the declared feature requirement generalizes `Dissemination`
- `pkg/plugin/rpc/types.go`, `internal/core/ipc/yang/ze-plugin-engine.yang` - Stage 1 fields and the Stage 2 catalog
- `pkg/plugin/sdk/sdk_callbacks.go` - subscribe from the declaration; delete the three subscription calls
- `internal/core/events/events.go` - the message registry; delete `ValidSendTypes` and `RegisterSendType`
- `internal/component/config/yang/validator_reference.go`, `validator_instance.go`, `modules/ze-types.yang` - the short path type and the reference walk over `to` and `from`
- `internal/component/plugin/resolve.go` - delete `RegisterPluginEventTypes`, `RegisterPluginSendTypes`, both `sync.Once`, the default event namespace and the `ze plugin X` branch of `deriveName`
- `internal/component/plugin/types.go` - identity and transport as two questions in place of `RunsExternalProgram`
- `internal/component/plugin/process/process.go` - fork of Ze's own binary for `process separate`
- `internal/component/config/first_pass.go`, `yang_schema.go`, `yang/join.go`, `yang/publish.go`, `loader.go`, `loader_extract.go` - derived first-pass roots, `bus` in place of `plugin`, `--plugin` merge into endpoints
- `internal/component/config/validate_sections.go`, `validate_semantic.go`, `validators.go`, `retired.go`, `listener.go`, `graph.go`, `serialize_set.go`, `cli/cmd_validate.go` and the config view commands under `cli/` - `bus` section, `bus-hub`, retired hints, the `| implied` renderer and envelope
- `internal/component/command/pipe_catalog.go` - the `implied` pipe operation
- `internal/component/bgp/config/plugins.go`, `peers.go`, `resolve.go`, `redistribute_binding.go`, `flowspec_binding.go`, `filter_state_binding.go`, `dissemination.go`, `dissemination_config.go`, `loader_create.go`, `register.go` - delete inline plugin extraction, `attach` parsing and the derived bindings; the replace-whole marker; BGP's participant nodes and messages; the dynamic-group collectors
- `internal/component/bgp/reactor/config.go`, `config_aigp.go`, `delivery_graph.go`, `send_permission.go`, `reactor_api.go`, `peer_settings.go`, `peer_settings_apply.go`, `peer_derived_bindings.go`, `source_policy_reload.go` - delete `ProcessBinding`, `EnsureProcessBinding`, `GrantImplicitReceive`, `GetPeerProcessBindings`; publish participants; read the compiled filter
- `internal/component/bgp/reactor/reactor_api_forward.go`, `reactor_api_forward_batch.go`, `reactor_api_relay.go`, `reactor_api_selected.go`, `aigp_readvertise.go`, `origin_receipt.go`, `origin_runtime.go`, `origin_grants.go`, `destination_marker.go`, `destination_constraint.go`, `peer_run.go`, `peer_bound_info.go`, `peer_forward_facts.go`, `policy_reload.go`, `register_source_lease.go` - the per-peer grant read sites
- `internal/component/bgp/plugins/rs/` (`server_forward.go`, `peer.go`, `server_handlers.go`, `register.go`), `internal/component/bgp/plugins/rr/` (`rr.go`, `register.go`), `internal/component/bgp/plugins/adj_rib_in/register.go` - the session-type limit of each forwarder; `bgp-adj-rib-in` declares `bgp:update`
- `internal/component/bgp/server/events.go` - the role retraction on the filter; alias fix for `RequiresPeerUpBarrier`
- `internal/component/plugin/server/delivery_graph.go`, `delivery_reconcile.go`, `dissemination.go`, `dissemination_validate.go`, `stage1_checks.go`, `startup_autoload.go`, `startup.go`, `subscribe.go` - the compiled filter, the index, Stage 1 checks, alias fixes
- `internal/component/plugin/server/dispatch.go`, `dispatch_registry.go`, `register_engine_op.go`, `dispatch_report.go`, `dispatch_route.go`, `dispatch_cached.go`, `dispatch_selected.go`, `dispatch_selected_refresh.go`, `dispatch_batch_validate.go`, `request_cancel.go` - the message field on every operation, the one check, the one refusal shape and counter
- `internal/component/authz/authz.go`, `internal/component/aaa/reserved.go` - a profile for the plugin identity; the prefix allowance kept for the plugin-less RPC identity alone
- `internal/component/firewall/register_contribution.go`, `internal/component/sysrib/register_route_view*.go`, `internal/component/iface/cmd/owned_devices.go` - the owner-registered operations state their message; identity by registration name (Q-T1, Q-T2)
- `internal/component/plugin/server/dispatch_state.go`, `doctor_snapshot.go` - identity by registration name or foreign identity; edge origins
- `internal/plugins/ddos/flowspec/doctor.go`, `internal/component/config/storage/cli/data_rpc.go` - match the registration name and report the configured alias in fixes
- `internal/component/plugin/doctor/check_external.go`, `check_managed_listener.go`, `internal/component/managed/client.go`, `cmd_fleet.go`, `doctor.go`, `cmd/ze/hub/main.go`, `managed_owner.go`, `cmd/ze/ze_core_start.go`, `cmd/ze/ze_core_dispatch.go` - read `bus`; `bus hub` diagnostics
- Every package that calls one of the three subscription calls or sets `EventTypes` or `SendTypes`, and every fixture under `internal/test/fixture/` - declare instead
- `internal/exabgp/migration/migrate.go`, `migrate_serialize.go`, `internal/exabgp/bridge/bridge_neighbor.go`, the `ze exabgp plugin` wrapper; delete `internal/plugins/exabgp/bridgeplugin/` and the internal mode of `bridgerun/` (E-1, pending the owed design)
- The web inspection and workbench pages that render the config tree - read-only implied rows
- `rfc/short/rfc7311.md` - read before the design is presented, to confirm no requirement binds the config-time refusal S-8 removes
- `docs/architecture/api/architecture.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/config/syntax.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/core-design.md`, `docs/architecture/fleet-config.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/system-architecture.md`, `docs/architecture/rib-transition.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/api/capability-contract.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/exabgp-bridge.md`, `docs/architecture/update-building.md`, `docs/architecture/memory/lifetime-contracts.md`, `docs/architecture/firewall/firewall-domain-group.md`, `docs/guide/ddos-mitigation.md`, `docs/guide/flowspec-protected-router.md`, `docs/guide/plugins.md`, `docs/guide/configuration.md`, `docs/guide/authorization.md`, `docs/config-reference.md`, `docs/exabgp/exabgp-migration.md`, `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/config-yang.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/doors/system-plugins.md`, `website/` - departed pages, doors and examples, and the decision rows of "Decision rows to record"
- Tracked `.conf`, `.ci`, `.et`, interop scenarios, fixtures, docs and unit tests: 1,750 files outside `plan/` carry `plugin {` or `attach process`, 905 of them `attach process` (counted 2026-11-23 with `git grep -lE 'plugin[[:space:]]*\{|attach[[:space:]]+process' -- ':!plan'`) - converted by the parsed-tree converter

## Files to Create
- `internal/component/plugin/bus_validate_test.go`, `bus_path_test.go`, `internal/component/plugin/server/bus_filter_test.go`, `bus_requirement_test.go`, `bus_sender_test.go`, `startup_bus_test.go`, `startup_alias_test.go`, `internal/component/bgp/config/resolve_bus_test.go`, `internal/exabgp/migration/migrate_bus_test.go`, `internal/component/bgp/plugins/rs/server_forward_test.go` - unit tests
- The `.ci` and `.et` files of the Functional Tests table - functional tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-plugin-conf.yang` (`bus`, the grouping), `ze-bgp-conf.yang` (remove `attach`), `ze-peer-cmd.yang` (remove `attach`), `ze-plugin-engine.yang` (Stage 1 and Stage 2 fields); delete `ze-exabgp-bridge-conf.yang`; regenerate glue with `./le yang glue update` |
| YANG validation constraints | Yes | `enumeration` on `encoding` and `process`, mutual exclusion of `internal` and `external`, `process` valid with `internal` only, presence on `send` and `receive`, `range` on `timeout` and hub ports |
| YANG custom validators | Yes | `ze:validate` for the endpoint names, the message names of `message`, `send` and `receive`, and the paths of `to` and `from`, each reading a registry filled at `init()` or the held external declarations |
| CLI commands/flags | Yes | `cmd/peer/create.go` (drop `attach`); the `implied` pipe operation; `--plugin` unchanged in what it accepts |
| CLI grammar (keyword before value) | Yes | The `bus` block, `bus { endpoint <name> ... }` and `create bgp peer` obey R1 to R9 (`./le cli grammar`) |
| Editor autocomplete | Yes | Automatic for enums and containers; completion for `internal`, for message names from the participant kind's declaration, and for `to` and `from` the registered participant nodes and their configured entries, each step of the path in turn |
| Functional test for new RPC/API | Yes | The functional tests above |
| Pipe completeness | Yes | `\| implied` composes with `\| match`, `\| json`, `\| yaml`, `\| text` and `\| format config` |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | Yes | `check_external.go`, `check_managed_listener.go`, `ddos/flowspec/doctor.go`, `managed/doctor.go` read `bus` and the registration name; a dormant reception and a refused request are reported |
| Doctor code per problem class | Yes | Existing codes kept (`doctor-plugin-delivery-disagreement`, `CodeDoctorPluginRequestRefused`, managed listener, ddos-flowspec); one new code for a dormant reception; fix texts in bus syntax |
| Prometheus counters/metrics | Yes | One refusal counter in the plugin server, labels participant and message; `ze_bgp_send_refused_total` deleted |
| BGP family surface (new SAFI / capability / attribute) | N-A | None added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/` pages that show `attach process` or `plugin {` (regenerate `docs/features.md`) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md`, `docs/config-reference.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`create bgp peer`, `\| implied`, `show event delivery`) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md` and `docs/plugin-development/protocol.md`: Stage 1 participant declarations; the Stage 2 catalog; the ready RPC without event subscriptions; what each engine operation needs |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/plugin-development/` |
| 6 | Has a user guide page? | Yes | `docs/guide/plugins.md`, `docs/guide/route-reflection.md`, `docs/guide/fleet-config.md`, `docs/guide/authorization.md` |
| 7 | Wire format changed? | No | BGP and IGP wire formats unchanged |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `docs/plugin-development/handlers.md`, `commands.md`: a plugin declares what it sends and receives |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc7947.md` and `rfc/short/rfc4456.md`: the tests that prove a route server serves its client set and a reflector its internal peers; `rfc/short/rfc7311.md`: the default path stays compliant |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` examples |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` holds no dependency on `attach process` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/core-design.md`, `docs/architecture/exabgp-bridge.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | The metrics page that lists `ze_bgp_send_refused_total` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`: each plugin's sends and receptions replace its event and send type lists; `exabgp-bridge` leaves the inventory |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived with `./le spec citation anchors` after the file lists above settle |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The 55 `docs/` files, 9 `website/` files and 7 `ai/` files of the earlier census |

## Design presentation

### What changes

Today an operator names a program under `plugin { }` and then, on every BGP peer, writes an `attach process` block that says which events the program is fed and which messages it may send. A program nobody attaches receives nothing and sends nothing, and yet any command a plugin dispatches is authorized without a look at any profile.

After this work everything on the bus is a participant with a name based on its configuration path. A participant says for itself which messages it sends and which it receives, and a compiled-in plugin and a program Ze did not ship say it the same way. With nothing written, that is what runs, on every peer, including a peer added later. The operator overrides it where the participant is configured: `to` replaces where its messages go, `from` limits who may address it, and two allow-lists limit which messages it may send and receive. A third-party program can be limited the same way from the `bus` section. Ze compiles both places into one filter per participant, refuses a disagreement naming both lines, and enforces the filter on events and on every operation a plugin calls, so a plugin can no longer run a command it did not declare. `show ... | implied` shows what was registered beside what was written, including what a written list replaced.

A compiled-in plugin runs in the daemon unless its endpoint says `process separate`, and stays the same plugin either way. `hub` moves under `bus`. An ExaBGP script becomes an ordinary endpoint that runs the wrapper. Every config, test and page in the repository is converted in the same work, and the converter writes no override.

### Before and after

1. **Default wiring: an external collector.**

Before:
```
plugin { external collector { run ./collector.py; encoder json; } }
bgp {
    peer edge1 { attach process collector { receive [ state update-received ]; } }
    peer edge2 { attach process collector { receive [ state update-received ]; } }
}
```
After:
```
bus { endpoint collector { external { run ./collector.py; } } }
bgp { peer edge1 { } peer edge2 { } }
```
The collector receives what it registered from every peer, and a peer added later needs no line.

2. **`to`: a lab peer whose routes stay out of the RIB.**

Before: `peer lab { attach process collector { receive [ update-received ]; } }` and no attachment of the RIB on `lab`.
After:
```
bgp { peer lab { bus { to [ /bus/endpoint/collector ]; } } }
```
The list is the whole destination set; `| implied` shows the RIB among the defaults it replaced.

3. **`from`: a collector that hears one group.**

Before: an `attach process collector` block on each peer of the group and on no other peer.
After:
```
bus { endpoint collector { external { run ./collector.py; } from [ /bgp/group/transit ]; } }
```

4. **Per-message list.**

After only (no equivalent before):
```
bgp { peer lab { bus { to [ /bus/endpoint/bgp-rib ]; message bgp:state { to [ /bus/endpoint/collector ]; } } } }
```

5. **Allow-lists: a program that may only watch state and send nothing.**

Before: `attach process acme { receive [ state ]; }` on every peer, and no `send` list anywhere.
After:
```
bus { endpoint acme { external { run ./acme; } send { } receive { allow [ bgp:state ]; } } }
```

6. **Two places, one filter.**

After:
```
bgp { route-server { bus { to [ /bgp/group/ix ]; } } }
bus { endpoint bgp-rs { internal bgp-rs; to [ /bgp/peer/edge1 ]; } }
```
is refused at commit, naming both `to` lines. Either line alone, or both with the same list, loads.

7. **Commands a plugin may run.**

Before: any plugin, a forked third-party program included, runs `clear bgp peer` with no check.
After: the command is refused unless the program declared `cli:clear bgp peer`, and the operator can cut it further: `send { allow [ "cli:show bgp peer" ]; }`.

8. **Seeing what is implied.**

Before: `show config running` shows only what was written; derived bindings are invisible.
After: `show config running | implied` prints each registered default under a `# implied by:` comment, and beside each written list what it replaced.

9. **Identity of a renamed plugin.**

Before: `plugin { internal my-ddos { use ddos-flowspec; } }` fails to read the plugin's state key, because the grant is keyed on the alias.
After: `bus { endpoint my-ddos { internal ddos-flowspec; process separate; } }` reads it, in the daemon or forked.

10. **Offline validation of an external program's config.**

Before: `ze config validate site.conf` with no daemon answers that the section of the external plugin `acme` is not available.
After: it runs `acme` in query mode and answers whether the config is valid.

11. **ExaBGP (pending the owed design).**

Before:
```
exabgp { bridge { process p1 { run ./p1.py; feed 192.0.2.1 { event [ receive-update ]; } } } }
bgp { peer a { attach process exabgp-bridge { receive [ * ]; send [ * ]; } } }
```
After:
```
bus { endpoint p1 { external { run "ze exabgp plugin ./p1.py"; } from [ /bgp/peer/a ]; to [ /bgp/peer/a ]; } }
```

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Flows as named objects in `bus { flow }`, naming receiver and subjects by instance-identifier path (the previous design of this spec) | Superseded by M-1 to M-10: the override sits on the participant, a list replaces rather than narrows, and an address or a prefix of names has no instance-identifier form |
| Allow-lists only, on the participant, with no `to` or `from` | Says what may be said or heard, never to whom: "this peer's routes go to the collector only" is not expressible |
| `to` and `from` only, with no allow-list | A command's destination is fixed, so a destination list cannot say "these commands only" |
| An add form beside the replacing list | Thomas ruled replacement; an add form and a remove form are a deny list by another name |
| The bus arbitrates exclusive roles (extend `Claims`) | Ruled out by M-11; each forwarding plugin limits itself |
| A second gate before the dispatcher for plugin commands | Two authorizers for one command, and the prefix allowance stays in `authz` |
| Converting each `attach process` block to a `to` list | Wrong by M-12: attach adds from silence, `to` is exclusive, so a literal conversion would cut the RIB off |
| Three child specs | Thomas ruled one spec; the phases are inside it |

### Documents departed from

1. `docs/architecture/api/architecture.md` ("Owner ruling: implied and shown, or written"): "Where the operator has a choice, which program watches a peer and what it may send, the operator writes it, and silence stays the default." Before: a program nobody attaches receives and sends nothing. After: a participant receives and sends what it registered. Example 1.
2. `docs/architecture/api/architecture.md` ("Owner ruling: endpoints and flows on the bus"): "A BGP group or peer takes part in a flow by listing its name, and the list merges through groups as every list does." Before (as designed there, never built): named flows. After: no flow object; `bgp { }` carries a `bus` block whose lists replace. Examples 2 to 4.
3. The earlier ruling of this spec that `bgp { }` holds no bus wiring ("pure BGP", D-5). Before: nothing of the bus under `bgp { }`. After: a BGP group or peer carries a `bus` block. What still holds: BGP's own YANG module names nothing of the bus, the bus module attaches the grouping, and no engine or config code names a feature. Example 2.
4. The earlier ruling that the send allow-list is per plugin, not per peer (R7, A-Q5). Before: one list on the endpoint. After: one filter per participant, so a peer, a group or a script can carry its own. Example 5.
5. `docs/architecture/api/architecture.md` ("What the Plugin Declares and What the Peer Grants"): "A plugin declares what it CAN handle, in its ready RPC." The declaration moves to the registration and becomes the default.
6. `docs/architecture/api/architecture.md` ("Data Flow: Config to Delivery"): "A relay is judged on the grant of the process that sends it, and on nothing else: no process lends its grant to another." A relay is judged on the sender's compiled filter.
7. `docs/architecture/plugin/plugin-system.md` ("One plugin, two ways to reach the bus"): "With no daemon running, no command starts it, and a config section only its module defines is not available". Example 10. `ai/doors/config-yang.md` and `ai/doors/plugin-infra.md` restate the old rule and are corrected with the page.
8. `docs/architecture/api/process-protocol.md` ("Persistent runtime state"): "Requests use the daemon's configured process name as identity." Example 9. And ("Event Delivery"): "the plugin subscribed to the type in that direction, and the peer's `attach process <name>` block grants it."
9. The implicit `state` grant to a filter owner, which today widens what the operator wrote (`GrantImplicitReceive`). After: the filter owner declares the requirement, and an override that removes it is refused with the completing line; nothing is widened in silence.
10. `docs/architecture/memory/lifetime-contracts.md` ("The four contracts"): "The source lease RPC checks the exact original Peer's explicit receive grant for the authenticated configured process." The lease check reads the compiled filter.
11. `docs/architecture/exabgp-bridge.md` and `ai/doors/system-plugins.md`: the internal runner and the `exabgp { bridge }` root, removed by E-1. Example 11.
12. `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `docs/architecture/config/syntax.md` and the pages of the Design documents table marked `departs` for syntax alone: the `plugin { }`, `attach process` and first-pass root forms they quote.

### Decision rows to record

Proposed wording for the `## Design decisions` section of each page, written there once Thomas approves it.

| Page | Decision | It forbids |
|------|----------|------------|
| `docs/architecture/api/architecture.md` | Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs | A default of silence; a second vocabulary for commands; a participant with no path name |
| `docs/architecture/api/architecture.md` | A `to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody | An add or remove form; merging a member's list into its group's |
| `docs/architecture/api/architecture.md` | A `from` list limits who may address a participant. Absent means everybody; present and empty means nobody | A default of nobody; a deny form |
| `docs/architecture/api/architecture.md` | A send and a receive allow-list of message names exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message | A deny list; enforcement left to the sender |
| `docs/architecture/api/architecture.md` | What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines | Two filters for one participant; a silent precedence between the two places |
| `docs/architecture/api/architecture.md` | The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself | A role registry in the bus that picks one plugin |
| `docs/architecture/api/architecture.md` | One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry | An implied relationship that no display shows |
| `docs/architecture/plugin/plugin-system.md` | With no daemon running, a command that parses a config starts each declared external program in query mode, never its run loop, and validates against its declaration | Refusing a section because no daemon runs; accepting a config that could not be checked |
| `docs/architecture/api/process-protocol.md` | Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter | An operation with no declaration; authorizing a plugin identity by prefix |
| `docs/architecture/exabgp-bridge.md` | An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist | A second config root or a second runner for ExaBGP scripts |

### Choices for Thomas

Answered and written into this design: the M, E and S rows under "Owner decisions governing this spec".

Questions for Thomas. Each row gives what happens today, what each answer changes, and what this spec assumes until he answers.

| # | Question | Today | Options and what each changes | Assumed until answered |
|---|----------|-------|-------------------------------|------------------------|
| Q-1 | How is "present and empty" written for `to` and `from`? | No such list exists | (a) `to [ ];`, if the config syntax keeps an empty list distinct from an absent one (A-7). (b) A presence container, `to { }`, holding the list | (a) |
| Q-3 | In the `bus` section, can the operator write a block for any participant by its path, or only on a plugin's endpoint entry? | No central place exists | (a) Only `bus { endpoint <name> { ... } }`: the bus side of a plugin, his example. (b) Also `bus { participant <path> { ... } }` for any participant, a BGP group included, which gives every participant two places and the conflict rule more to judge | (a) |
| Q-4 | The same field written in both places with identical content: accepted or refused? | - | (a) Accepted, shown once with both lines as origin. (b) Refused as redundant | (a) |
| Q-5 | What makes a peer one the route server serves, and one the reflector serves? | The route server targets every peer it is fed that is up (`updateTarget`); the per-peer `attach process` blocks decide which those are | (a) The route server serves the peers of its client set (`session rs-client`), the reflector internal peers. (b) The route server serves every external peer, the reflector every internal one. (c) Neither limits itself, and an operator who loads both writes a `to` on each | (a) |
| Q-6 | Can a list name one peer admitted by a dynamic group's range, by its address? | A group's `attach` covers its dynamic peers; one cannot be singled out | (a) No: the commit refuses it and names the group to write. (b) Yes: `/bgp/group/<g>/peer/<address>` is accepted when the group's range admits the address | (a) |
| Q-7 | Is a tag other than the sender writable in a `bus` block in this work (for example the source protocol of a route change)? | A subscription can carry a direction and a peer selector | (a) No config form now: tags are declared and used by registrations, and a config form waits for a need. (b) A `tag <name> { value [ ... ]; }` filter in the block | (a) |
| Q-8 | Does a member's `bus` block replace its group's block whole, or only the items the member writes? | `attach` merges through groups | (a) Only the items written: a member that writes `to` keeps the group's `from` and allow-lists. (b) The whole block: a member that writes anything inherits nothing of the group's block. S6 holds both expectations | (a) |
| Q-9 | When an operator deliberately points two forwarders at one peer, `policyReplayOwner` returns the first in order. Is that acceptable under "no silent winner"? | One forwarder per peer by the attach blocks | (a) Accept: it is the operator's written choice, and doctor reports the peer as served by two forwarders. (b) Each claimant replays its own routes and the engine picks none | (a) |
| Q-T1 | Do the operations their owners register through `RegisterEngineOp` state a message and join the walk? | `EngineOperation` has no such field | (a) Yes, the same field and the same walk. (b) They stay judged by their owners' own checks | (a) |
| Q-T2 | Which identity do the checks keyed on `proc.Name()` move to (firewall table reservation, interface device-owner grant, metric and issue owners, cache release)? | The operator's alias | (a) The registration name for a compiled-in plugin, as for state keys. (b) The alias stays for these | (a) |
| Q-T3 | Is `batch-validate` judged as the `cli` path it dispatches, or by a message of its own? | The prefix bypass | (a) The `cli` path. (b) Its own message | (a) |
| Q-T4 | Is `update-route` judged as `bgp:update`, as the `cli` path of its command text, or both? | The prefix bypass, then the per-peer send grant | (a) Both. (b) `bgp:update` alone | (a) |
| Q-E | The ExaBGP rows X-1 to X-7: each is either carried by the wrapper or returns here as a question with its evidence | The internal bridge holds a fleet-wide owner ledger and API setting | Owed investigation | E-1 as recorded, unsettled |

## Implementation Steps

One spec, eight phases. A phase lands as compiling commits; the spec closes when all are in.

1. **Phase: Wiring (MANDATORY FIRST)** -- add the failing functional tests of the Wiring Test table and the failing unit tests `TestBusFilterCompilation`, `TestProcessIdentityByRegistration`, `TestBusCommitRefusals` and `TestEveryEngineOpDeclaresItsMessage`. Record `bus-send-undeclared-refused` red against today's code. Take the runtime fixture capture, the ExaBGP bridge section census and the route-server census of A-10.
2. **Phase: Registration** -- add the participant declarations to `registry.Registration`, Stage 1 and the message registry; deliver the catalog at Stage 2; convert every plugin and fixture in the tree; delete `EventTypes`, `SendTypes`, the two lazy registrars, the default event namespace and the three subscription calls.
3. **Phase: Identity** -- split identity from transport; add `process separate`; fix `dispatch_state.go`, `doctor_snapshot.go`, `ddos/flowspec/doctor.go`, `data_rpc.go` and the five sibling sites.
4. **Phase: Schema, loader, first pass and validation** -- delete `container plugin` and `container attach`; add `container bus` and the augmented grouping; regenerate the YANG glue; derive the first-pass roots; the replace-whole marker; the eight commit refusals; offline validation in query mode; retired hints, listener and graph names, `create bgp peer`.
5. **Phase: Filter, delivery and forwarders** -- compile one filter per participant; build the index from it; delete `ProcessBinding` and its writers; move every per-peer grant read site; limit the route server and the reflector by session type; the declared feature requirement and its check; the AIGP trigger.
6. **Phase: Engine operations and authorization** -- the message field on every operation and the walk; the one check in the shared dispatch path; the profile for the plugin identity; the one refusal shape, counter and doctor code.
7. **Phase: Display** -- the `implied` pipe operation, its text and envelope renderers, the notice, the web rows, the edge origins on `show event delivery` and `ze config graph`.
8. **Phase: ExaBGP, conversion and documentation** -- the wrapper as the only runner and the migration output (after the owed design is settled); run the converter over the tracked files; restate the judged tests; update the pages, doors and decision rows; run `TestRepositoryConfigsValidate`, the scoped functional suites and the converted interop scenarios.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 to AC-36 has an implementation and a passing test, and every scenario S1 to S25 its named test |
| Filters off the hot path | No path text is parsed, built or compared on a delivery; every list is resolved at commit and at index build |
| No layering | No acceptance of `plugin {`, `attach process` or `exabgp { bridge` remains; no subscription call, `EventTypes`, `SendTypes`, lazy registrar, `ProcessBinding` or prefix allowance for a plugin identity remains |
| Bus isolation | `internal/component/plugin/` and `internal/component/config/` hold no BGP import and no `peer` or `group` word; BGP's YANG names nothing of the bus |
| One API for both kinds | Every rule that differs for an external program names its transport or arrival-order reason on `plugin-system.md` |
| Identity | A foreign program named like a compiled-in plugin holds none of its grants; a forked compiled-in plugin holds all of them |
| One filter | Delivery, the engine-operation check, the authorization profile and the display read the same compiled structure |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line in bus syntax |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No active `plugin {`, `attach process` or `exabgp { bridge` outside `plan/` history and `retired.go` | `git grep -E 'plugin \{|attach process|exabgp \{' test/ docs/ etc/ examples/` |
| All tracked configs validate | `./le test unit pkg path ./internal/component/config/cli/ run TestRepositoryConfigsValidate` |
| Scoped unit and functional suites green | `./le test unit pkg` on changed packages, and `./le test functional parse`, `plugin`, `reload`, `encode`, `exabgp`, `editor`, `managed`, `web` |
| Converted interop scenarios pass | `./le test integration interop` for the scenarios the converter changed |
| YANG glue, tier, grammar, enumeration and doc gates green | `./le yang glue check`, `./le arch tier check`, `./le arch enumeration check`, `./le cli grammar`, `./le plugin declarations check`, `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Identity by name | No site takes a compiled plugin's identity from an endpoint's name or a `run` string; only the `internal` leaf gives it |
| Undeclared message | A message the participant did not declare is refused on every engine operation, with nothing written |
| Prefix allowance | `Store.authorize` allows no `plugin:` identity by prefix; the plugin-less RPC identity's callers are listed |
| Sender | No code path takes the sender from message content |
| Empty lists | A present and empty list refuses everything, and no code path reads it as absent |
| Offline validation runs programs | A program is started in query mode only, and the validate output names each program it asked |
| Operator text off the wire | An external declaration's names are bounded and free of control characters before they reach a message or a registry (`ai/rules/plugins.md`) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| The converter breaks `$PORT` or a `refused=` block | Fix the converter's sentinel handling or triage that test |
| A multi-peer test fails after `attach process` is removed | Decide from the censuses whether it needs a `to`, a `from`, an allow-list, or the test judge |
| A compiled-in plugin is refused a command it uses | Its declaration is incomplete: fix the declaration from the runtime capture |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- Removing `attach > content` also removes peer session bounces that `reflect.DeepEqual` caused over dead leaves.
- The reception a plugin declares already exists, in the ready RPC. Moving it to the registration is what lets an offline reader show it.
- `attach process` already had the right place in the tree and the right inheritance. What was wrong was its default (silence), its direction (it named the receiver from the sender's side only) and its scope (BGP alone).
- The per-peer send grant did two jobs: permission, and choosing which forwarder owns a peer. The filter keeps the first. The second belongs to the forwarders, by session type.
- One compiled filter per participant is what makes the display trustworthy: there is no second computation to differ from what runs.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-keyed-on-a-field-its-consumer-ignores.md` | 2026-11-14 | `statestore.PluginKeyReadable`/`Writable`/`PluginList` (`plugin/server/dispatch_state.go`), `PluginDoctorState.Name` (`doctor_snapshot.go`), `ddos/flowspec/doctor.go`, and `data_rpc.go` (`runningOwnerError`) key on configured process alias instead of compiled registry name |
| `escape-hatch-scoped-wider-than-its-justification.md` | 2026-11-23 | `internalPluginIdentity` (`plugin/server/dispatch.go`) stamps `aaa.ReservedInternalPrefix` on every plugin process, a forked program included, and `Store.authorize` (`authz/authz.go`) allows the prefix before any profile lookup; `opRouteInstall` and `opRouteRemove` (`dispatch_route.go`) check no caller permission |

The second row was owned by `spec-bus-3-external-command-gating.md` and moves here with that spec's work. Its journal file names that spec as owner and is updated by the main session when the child files are removed. The unchecked `route-install` and `route-remove` named in the surface are closed by AC-28.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One message model: participants named by path, messages for events and commands, the sender a mandatory tag (M-1, M-2) | Endpoints and flows; a separate vocabulary for commands | Thomas's words; one filter then covers delivery and command gating |
| The override sits on the participant, in one grouping the bus augments (M-5) | A central flow list; a block per component written by hand | Written near the emitter; defined once; BGP's YANG names nothing of the bus |
| `to` replaces, `from` limits, both block-level and per message (M-6, M-7, M-8) | Narrowing flows; add and remove forms | Thomas's rulings |
| Allow-lists beside the destination lists, enforced at the bus (M-9) | Destination lists alone | A command's destination is fixed; names need their own list |
| Two places compile to one filter, and a disagreement is refused naming both lines (M-10) | A precedence rule between the places | No hidden winner; the operator fixes the line |
| No arbitration in the bus; forwarders limit themselves by session type (M-11) | Extend `Claims` into exclusive roles | Thomas's ruling; RFC 7947 and RFC 4456 already say whom each serves |
| The converter writes nothing; tests of a difference are restated through the judge (M-12) | One-to-one conversion of attach | Attach adds from silence, `to` is exclusive |
| A profile for the plugin identity in the authorizer (M-13) | A second gate; keep the prefix allowance | One authorizer and one matcher; the bypass is deleted where it lives |
| A message field on every engine operation, with a walk test | A hand list of gated operations | A new operation cannot ship ungated by omission |
| Identity and transport are two leaves (S-2) | One predicate; a `run` line that names a compiled plugin | A compiled-in plugin keeps its identity on either transport |
| Bare `\| implied` pipe, a `# implied by:` comment, a JSON and YAML envelope, read-only workbench rows, gNMI written-only | `\| display implied`; `##` marker; implied keys merged into the config | Composes with `\| match`; `##` at column 0 breaks format detection; the written config stays pure and `Get` then `Set` round-trips |
| One declared requirement for features and roles | Plugin-name literals in BGP config code; keep `DisseminationDecl` and add a second declaration | One check and one message shape; config code names no plugin |
| Short slash paths, a peer by name or by address (M-3) | RFC 7950 instance-identifier text | Thomas's ruling; an address and a prefix of names have no instance-identifier form |
| One spec, phased | Three child specs | Thomas's ruling |

## Known Limitations
- A list cannot name one peer admitted by a dynamic group's range unless Q-6 is answered (b); it names the group.
- gNMI `Get` returns the written configuration only.
- The offline view and offline validation refuse a config whose external program cannot be asked for its declaration.
- A plugin that builds a command path from run-time data must declare the prefix it builds under.
- A tag other than the sender has no configuration form in this work (Q-7).
- FlowSpec authorization refuses a Loc-RIB that runs as a separate process, because its lookups are process-local. That is a transport reason and is stated on `plugin-system.md`.
- The ExaBGP section is not settled design (E-1, rows X-1 to X-7).

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] The ExaBGP rows X-1 to X-7 are answered and the bridge section census is recorded
- [ ] Every row `./le spec design documents` prints has a verdict, the rows of the two absorbed specs included
- [ ] AC-15, AC-27 and AC-29 each have a worked scenario

### Goal Gates (MUST pass)
- [ ] AC-1..AC-36 all demonstrated
- [ ] Every worked scenario S1 to S25 has its named test passing
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: scoped package tests and `./le test functional parse`, `plugin`, `reload`, `encode`, `exabgp`, `editor`, `managed`, `web`
- [ ] The converted interop scenarios pass against their peers
- [ ] Every test in "Tests routed through the test judge" carries the judge's verdict
- [ ] `bus-send-undeclared-refused` was recorded red before the fix
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-user behavior
- [ ] Interop tests: converted scenarios run against their peers

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-explicit-plugin-and-attach-config.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
