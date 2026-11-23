# Spec: explicit-plugin-and-attach-config

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Configuration correctness and identity integrity of shipped behavior: replaces the ExaBGP-derived `plugin { }` and per-peer `attach process` grammar with the protocol-agnostic `bus { endpoint, flow, hub }` model in which every plugin registers what it publishes, receives and sends, converts all repository configs/tests/docs cleanly with no hybrid layering, and fixes aliased plugin state-key and doctor snapshot keying |
| RFC gaps | - |
| Depends | - |
| Phase | Design reworked 2026-11-23 on rulings R1 to R7; awaits the design judge and owner approval |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-23 |

Bucket: `plan/release/fixes/`. Child 1 of 3 (split per R-8 on 2026-11-23):
- **Child 1 (this spec):** `bus { endpoint, flow, hub }` grammar cutover (pure bus, zero `flow` in `bgp {}`); the registration every plugin makes of what it publishes (events and their tag keys), what it receives by default, and what it sends; generic message tags on the receive side of a flow; the plugin-wide send allow-list for the BGP send types that are enforced today; a compiled-in plugin run as a separate process under its own identity; `ze exabgp migrate` and `create bgp peer` cutover; parsed-tree conversion of all repository configs/tests/docs; and the owned alias-keying journal row plus 5 sibling alias-keying defects.
- **Child 2:** `plan/release/fixes/spec-bus-2-implied-display-and-grant-refusals.md`: the `| implied` display of every registered default beside what the operator wrote, the feature requirements a plugin registers and the refusal of an operator allow-list narrower than a configured feature needs, the AIGP trigger, and the `origin` of each edge on `show event delivery`.
- **Child 3 (P0):** `plan/release/fixes/spec-bus-3-external-command-gating.md`: one check, for every engine operation and every plugin of either kind, that a plugin sends only what it declared and the operator's allow-list keeps, which closes the live `ReservedInternalPrefix` AAA bypass on `dispatch-command`.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Replace `plugin { internal ...; external ...; hub ...; }` and BGP's `attach process <name> { ... }` (including inline `run`/`use` and the dead `content` container) with one protocol-agnostic `bus { endpoint ...; flow ...; hub ...; }` configuration block. `bgp { }` stays pure BGP. Every plugin, compiled-in or a program Ze did not ship, registers the same three things through the same API: the events it publishes with the tag keys they carry, the events it receives by default, and the commands it sends to another component. With nothing written by the operator, those registrations are what runs. An operator flow narrows what an endpoint receives, and the one send allow-list on the endpoint narrows what it sends. Neither ever widens them. The work also fixes the aliased-plugin identity defects on the state and doctor surfaces.

### Owner rulings governing this spec

Thomas speaks through voice transcription. Each row gives the reading the main session confirmed with him. Rows R1 to R7 were given on 2026-11-23 after the first design of this spec, and they replace the parts of it they contradict.

| # | Ruling, as read | Status in Child 1 |
|---|-----------------|-------------------|
| D-4 | The config states an `endpoint` (who is on the bus) apart from a `flow` (what moves between two parties). `attach process` is replaced whole, its inline `run`/`use` and dead `content` block included | Landed here |
| D-5 (reversed 2026-11-23) | Pure protocol-agnostic bus and pure BGP: no flow inside `bgp { }`, no BGP keyword (`peer`, `group`) inside `bus { }` | Landed here |
| D-7 / Q3 | `plugin { }` becomes `bus { }`, and `hub` moves under it. The listener service `plugin-hub` becomes `bus-hub`. Repository configs are converted directly | Landed here |
| R1 | Every plugin registers the flows it wants to receive by default: a publisher and tags. With no operator flow those defaults apply. Publishers tag every message, and filtering is on generic tags only. A peer added later is covered with no config change | Landed here. The display of the defaults (`\| implied`) is Child 2 |
| R2 | A plugin registers the events it publishes. Every publication registration completes before any reception registration, so a receiver can learn what is on the bus. A reception that names something nobody publishes yet is valid and dormant | Landed here |
| R3 | A compiled-in plugin runs in the daemon by default. Running it as a separate process is an operator option on its endpoint, and it keeps the same identity, state grants and default flows | Landed here. Replaces the refusal of state to such a process in the first design |
| R4 | The API for a compiled-in plugin and for a program Ze did not ship is the same. No rule applies to one kind alone without a transport reason | Landed here. Reverses "built-in publishers only in `from`" and "built-in only on `ze start --plugin`" of the first design (A-Q2, A-Q3) |
| R5 | `ze exabgp migrate` emits a complete loadable config with its bus section, and a bridged script holds the whole ExaBGP API command set | Landed here (A-Q6: the migration writes no list) |
| R6 | The operator's bus configuration is the trust boundary. A plugin the operator configured is trusted whether compiled-in or external | Landed here: no rule below separates the two kinds on trust |
| R7 | A plugin may send everything it declares. The operator narrows it with an allow-list, never a deny list. The allow-list is per plugin, not per peer or tag. An allow-list that is present and empty means the plugin sends nothing. No allow-list means it sends everything it declared. Anything the plugin did not declare is refused | The declaration, the allow-list grammar and the BGP send types land here. The check on every other engine operation is Child 3 |
| D-6, D-10, Q4, Q5 | The `\| implied` display, narrow-list refusals, the AIGP trigger and edge origins | Homed in `plan/release/fixes/spec-bus-2-implied-display-and-grant-refusals.md` |
| D-8, Q6 | The check on every command a plugin sends | Homed in `plan/release/fixes/spec-bus-3-external-command-gating.md`, restated by R7 |
| Owned journal row (2026-11-14) | Key compiled-in state-key grants, doctor snapshots and live-owner guards on the compiled registry name, and fix the 5 sibling alias-keying defects | Landed here |

Thomas then answered the seven questions the first rework put to him, the same day. Each row is the reading the main session took.

| # | Answer, as read | Where it lands |
|---|-----------------|----------------|
| A-Q1 | An operator flow for a plugin and a publisher replaces the registered reception: only what the flow lists is delivered (his example: "only peer state"), and an empty list turns the reception off | Semantics of flows |
| A-Q2 | Offline validation starts the external program far enough to take its declaration and its YANG, does not enter its run loop, and validates against that. This reverses the recorded ruling that with no daemon Ze starts no external program, and Thomas confirmed the reversal is intended | "Offline validation asks the program" |
| A-Q3 | `ze start --plugin <external program> -` keeps working | Unchanged behavior, AC-8 |
| A-Q4 | A route server serves every peer connected to it; per-client differences are export policy. Nothing replaces the per-peer send grant | "Which peers a forwarding role serves" |
| A-Q5 | One send allow-list per plugin, covering every component it sends to | The list lives on the endpoint |
| A-Q6 | `ze exabgp migrate` writes no list, so the full declared set applies | AC-9 |
| A-Q7 | The new default is every peer. The converter writes no narrowing flow | "Receive census" |

Thomas also confirmed R7's default (a plugin sends what it declared with nothing written), and that A-Q7 changes behavior for existing configs with no exception, ExaBGP configs included. `ai/META-DECISIONS.md` ("CLI, config, and daemon surfaces") now records the rule this design rests on: "What a plugin registers is not such a choice: a configured plugin receives and sends what it registered, shown as implied, and the operator writes an allow-list only to narrow it."

### What a plugin registers

One registration carries three declarations. A compiled-in plugin makes it on its `registry.Registration` from `init()`. A program Ze did not ship makes it in its Stage 1 `declare-registration` message. The same code judges both.

| Declaration | What it states | Fields |
|-------------|----------------|--------|
| Publishes | The events this party puts on the bus | Namespace, event types, and for each the tag keys every message carries |
| Receives | The events this party wants by default | Publisher namespace, event types, and an optional tag match |
| Sends | The commands this party issues to another component | Target component, and the command tokens that component registered |

A component that is not a plugin (BGP, the interface component, the system RIB) registers its publications and the command tokens it accepts the same way, from its own `init()`.

| Rule | Meaning |
|------|---------|
| Publications first | Compiled-in publications exist at `init()`. A declared external program's publications are read when `ze` starts, before the full parse. So the publication set is complete before any reception is resolved |
| The catalog is delivered | The union of publications in the startup set reaches every plugin in its Stage 2 configure message, beside `Claims`, so a plugin can decide what it accepts |
| A dormant reception is valid | A registered reception whose publisher is absent (a build without BGP, a plugin configured later) resolves to nothing, is not an error, and is shown as dormant |
| An unknown name the operator wrote is refused | A flow the operator wrote that names an unknown publisher, event, tag or command is refused at commit with the accepted values. Ze never loads it to do nothing |

### Target `bus { }` configuration model

| Concept | What it states | Where it is written |
|---------|----------------|---------------------|
| Endpoint | One party on the bus under an operator-chosen name | `bus { endpoint <name> { ... } }` |
| Flow | What one endpoint receives from one publisher. Where the operator writes one, it replaces what the endpoint registered for that publisher | `bus { flow <name> { ... } }` |
| Send allow-list | What one endpoint may send, to every component, as one list | `bus { endpoint <name> { send { ... } } }` |
| Hub | The TLS listener and managed-config client blocks moved from `plugin { hub }` | `bus { hub { server ...; client ...; } }` |

Fields of `bus { endpoint <name> { ... } }`:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string (key), `ze:validate "external-plugin-name"` | Endpoint name that flows and diagnostics use |
| `internal` | string (`ze:validate "internal-plugin-name"`) | Registry name of a compiled-in plugin. Its identity is the compiled registration, whatever the endpoint is called. Mutually exclusive with `external` |
| `process` | enumeration `in-daemon` (default) or `separate`, valid with `internal` only | `separate` makes Ze fork its own binary for that plugin over the TLS connect-back. The operator writes no command line. Refused for a plugin whose registration sets `RunsInProcessOnly` |
| `external` | container (`run`, `respawn`, `timeout`) | A program Ze did not ship. Its identity is the endpoint name, and it takes nothing from a compiled registration of the same name |
| `encoding` | enumeration `json` or `text` (default `json`) | Wire encoding of events delivered to this endpoint (replaces `encoder`) |
| `send` | presence container holding leaf-list `allow` | The one send allow-list of this plugin. Absent: the plugin sends everything it declared. Present and empty: it sends nothing. Each entry is `<component>:<token>`, the form a filter reference already has (`<plugin>:<filter>`): `bgp:update`, `system-rib:route-install`, `"cli:show bgp peer"`. Each entry MUST be a send the plugin declared |

A token names its target in the entry itself, so one list covers every component with no ambiguity: the part before the first colon is a component that registered command tokens, and the rest is a token of that component. A component name holds no colon.

To narrow a plugin Ze loaded with no `endpoint` written, the operator writes its endpoint: `endpoint bgp-rib { internal bgp-rib; send { allow [ bgp:update ]; } }`.

Fields of `bus { flow <name> { ... } }`:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string (key) | Unique flow name |
| `from` | string | The publisher: a registered publisher namespace, or an endpoint name of either kind |
| `namespace` | string (optional) | Chooses the namespace when `from` owns several (`l2tp` and `subscriber`). Refused when `from` does not own it |
| `to` | string | The endpoint this flow narrows: a `bus { endpoint }` name or a compiled-in plugin's registry name |
| `event` | leaf-list of string | The event types `to` receives from `from`. Each MUST be one the publisher registered and one `to` registered a reception for. The flow itself is the presence: a flow with no `event` entry delivers nothing |
| `match` | list keyed by `tag` (`tag` string, `value` leaf-list of string) | A message matches when every listed tag carries one of its values. No `match` means every message |

Semantics of flows and of the send list:

| Case | Result |
|------|--------|
| No flow for a pair (publisher, endpoint) | The endpoint's registered receptions from that publisher apply, for every peer |
| One or more flows for the pair | They replace the registered reception for that pair: only what they list is delivered (A-Q1). A flow that lists `event [ state ]` delivers peer state and nothing else |
| A flow for the pair with no `event` entry | The reception from that publisher is off |
| Two flows for one pair with different `match` | Each delivers its own events to its own matching messages. A message that matches both is delivered once |
| A `match` tag the publisher did not register | Refused, listing the tag keys that publisher registered |
| A `match group` value that names no configured group | Refused, listing the configured groups. The publisher registers a validator for the values of a tag whose set the config holds |
| A `match peer` value | Checked for form only. A dynamic peer arrives later under an address no config names |
| No `send` container on the endpoint | The plugin sends everything it declared, to every peer |
| `send { allow [ ... ]; }` | The plugin sends the listed entries only |
| `send { }` | The plugin sends nothing |
| A `send` entry the plugin did not declare, or whose component or token is not registered | Refused, listing what the plugin declared |
| A `send` container on a plugin that declares a send toward a component whose commands are not enforced yet | Refused until Child 3 lands, naming those components. One list speaks for every component, so a list that is present promises that everything it omits is refused, and before Child 3 that holds for the BGP send types only. A plugin that declares sends toward `bgp` alone can be narrowed as soon as this spec lands |

### Which peers a forwarding role serves

The per-peer `send [ update ]` grant also decided, until now, which peers a route server or reflector serves. Thomas's answer (A-Q4) is that the RFC already says it, and nothing replaces the grant.

| Role | The RFC's words | What follows |
|------|-----------------|--------------|
| Route server | RFC 7947 Section 1: "Each multilateral interconnection participant (usually referred to as a "route server client") announces network reachability information to the route server using EBGP. The route server, in turn, forwards this information to each route server client connected to it, according to its configuration." | The route server plugin serves every peer connected to it. A difference between clients is export policy in the peer's filter list |
| Route reflector | RFC 4456 Section 5: "The internal peers of an RR are divided into two groups: 1) Client peers 2) Non-Client peers". Section 6: a route from a non-client is reflected "to all the Clients", and a route from a client "to all the Non-Client peers and also to the Client peers" | Client or non-client is BGP configuration of the peer. It changes the reflection rule the plugin applies. It never decides whether the plugin may talk to the peer |

A configuration that mixes route-server sessions with other sessions on one box, and used the per-peer grant to keep the route server away from the others, converts to export policy on those peers. The per-peer send census finds each such config.

### Offline validation asks the program

Ruled by A-Q2 and confirmed by Thomas as an intended reversal of the earlier ruling.

| Point | Statement |
|-------|-----------|
| Today | With no daemon running Ze starts no external program (owner ruling on `plugin-system.md`). `ze config validate` holds the compiled-in schema alone, asks a running Ze first, and refuses a section only an external module defines (`DeclaredSchemas.ParseRefusal`) |
| The mechanism that exists | Query mode (`process-protocol.md`, "Query Mode: Stage 1 and No Stage 2"): a program started with `ZE_PLUGIN_MODE=declare` writes its Stage 1 `declare-registration` line to stdout and exits 0. No connection, no Stage 2, no run loop. `sdk.RunOrDeclare` writes the line and never calls the plugin's start function. `show plugin declarations` and the conformance check already run a program this way |
| After | A command that parses a config with no daemon running starts each declared external program in query mode, takes its declaration (YANG modules, publications, receptions, sends), joins the modules through the same join the daemon uses (`HoldDeclaredPlugins` reads the same line at start), and validates the whole config against it. This is "start the init, do not start the loop" |
| What it extends | `config.SchemaWithDeclaredPlugins` and `PublishDeclaredSchemas`, which today publish nothing without a daemon. The route that asks a running Ze first stays, because a running daemon holds the declarations it started with |
| What validate prints when the program cannot be asked | The validation fails. The message names the endpoint and its `run` line, says which of four things happened (the program was not found, it exited non-zero, it wrote no declaration line, or its declaration was refused at Stage 1, with that refusal), says that the sections and flows that depend on it were not checked, and gives the command that shows the program's own answer: `ze show plugin conformance run "<run line>"` |
| What it never does | Accept a config it could not check. A program that adopts no query route and runs its live start is read as having written nothing, as today |
| Before and after for an operator | Before: `ze config validate site.conf` on a box with no daemon answers that the section of the external plugin `acme` is not available and names the running Ze as the fix. After: the same command runs `acme` in query mode and answers whether the section and the flows that name `acme` are valid |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform boundaries, process boundary, registration over enumeration
  -> Constraint: `internal/component/plugin/` never imports `internal/component/bgp/`; `bus` carries zero BGP-specific concepts
- [ ] `ai/doors/config-yang.md` - YANG schema loading, first-pass roots, `validatedSections`, `ze:validate`, config text manipulation rules
  -> Constraint: config text changes go through a parsed `*config.Tree`. The door's rule that with no daemon running Ze starts no external program is departed from by A-Q2 (offline validation runs the program in query mode), and the door is corrected with `plugin-system.md` ("Documents departed from", item 4b)
- [ ] `ai/doors/plugin-infra.md` - `plugin/registry`, `plugin/server`, `plugin/process`, `HoldDeclaredPlugins`, Stage 1 checks
  -> Constraint: "A compiled-in and an external plugin differ only in how they reach the bus. A rule for external plugins alone needs a transport reason or an arrival-order reason"
- [ ] `ai/doors/bgp-engine.md` - BGP reactor, `PeerSettings`, `DeliveryGraph` publication, `send_permission.go`
  -> Constraint: BGP stays isolated; it registers the `peer` and `group` tag keys and attaches them, and the bus learns no BGP word
- [ ] `ai/doors/core.md` - `internal/core/events`, `internal/core/statestore`
  -> Constraint: `internal/core/` never imports upward
- [ ] `ai/doors/hub-engine.md` - `cmd/ze/hub` startup, `FirstPass`, managed server/client, listener discovery
  -> Constraint: the first-pass roots switch from `plugin` to `bus` before the full parse
- [ ] `ai/doors/system-plugins.md` - `internal/exabgp/migration`, `internal/exabgp/bridge`, `internal/plugins/ddos/flowspec`
  -> Constraint: the ExaBGP migration and `bridge_neighbor.go` (`create bgp peer`) cut over in the same work
- [ ] `docs/architecture/plugin/plugin-system.md` - "One plugin, two ways to reach the bus"
  -> Constraint: a design states which transport or arrival-order reason, if any, makes an external plugin behave differently
- [ ] `docs/architecture/api/architecture.md` - "Attaching a Process to a Peer", "Owner ruling: implied and shown, or written", "Owner ruling: endpoints and flows on the bus", "What the Plugin Declares and What the Peer Grants"
  -> Decision: both owner rulings on that page are departed from by R1 and R7; the page is corrected in the same work
- [ ] `docs/architecture/config/yang-config-design.md` - "Where a validator actually runs", `--plugin`
  -> Constraint: the `--plugin` flag keeps starting the plugin it names
- [ ] `docs/architecture/api/process-protocol.md` - Stage 1, event delivery, persistent runtime state
  -> Constraint: Stage 1 gains the three declarations; Ze is pre-release, so the protocol changes directly and every plugin in the tree is rebuilt with it

**Key insights:**
- A plugin already declares what it receives, in its ready RPC (`sdk.Plugin.SetStartupSubscriptions`, `SetStartupSubscriptionsIn`, `AddStartupSubscriptionsIn`), and today the peer's `attach process` block decides what it gets. R1 makes that declaration the default instead of the upper bound. The declaration moves to the registration so an offline reader can show it.
- `internal/component/config/validate_sections.go` covers `plugin` and excludes `bgp`. `plugin.RegisterPluginEventTypes` and `RegisterPluginSendTypes` run lazily, under two `sync.Once`, from `PeersFromTree` and `NewServer`, after `ValidateCustomSections`. Registration at `init()` removes the ordering problem and both lazy registrars.
- `ProcessBinding.Encoding` and `ProcessBinding.Format` (`attach > content`) are read on no delivery path. Deleting `content` loses no behavior.
- The per-peer send grant is load-bearing beyond permission: `bindingsMaySend` decides which peers a route server or reflector owns for the peer-up replay, and `requireRIBDelivery` reads it for FlowSpec replay. R7 removes it, and RFC 7947 and RFC 4456 already say what replaces it: a forwarding role serves every peer connected to it ("Which peers a forwarding role serves").
- `test/plugin/external-yang-command-line.ci` and `TestFirstPassMergesCommandLinePlugins` prove that `ze --plugin "<program>"` starts an external program before the full parse. The first design removed that.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/yang/ze-plugin-conf.yang` - `container plugin` with `list internal` (`use`), `list external` (`run`, `use`, `encoder`, `respawn`, `timeout`), `container hub`
- [ ] `internal/component/plugin/types.go` - `PluginConfig.RunsExternalProgram`: an `external` block whose `run` is a command line, which takes nothing from a same-named registry row
- [ ] `internal/component/plugin/resolve.go` - `RegisterDefaultEventNamespace`, `RegisterPluginEventTypes`, `RegisterPluginSendTypes` and their two `sync.Once`; `ResolvePlugin` and `deriveName` (a `ze plugin X` command line is named `X`)
- [ ] `internal/component/plugin/registry/registry.go` - `Registration.EventTypes`, `SendTypes`, `Claims`, `Dissemination`, `RunsInProcessOnly`
- [ ] `pkg/plugin/rpc/types.go` - `DeclareRegistrationInput` carries no event, reception or send declaration
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` - `SetStartupSubscriptions`, `SetStartupSubscriptionsIn`, `AddStartupSubscriptionsIn`: the reception a plugin declares, carried by the ready RPC
- [ ] `internal/core/events/events.go` - `RegisterNamespace`, `RegisterEventType`, the flat `ValidSendTypes` and `RegisterSendType`
- [ ] `internal/component/config/first_pass.go`, `yang/join.go` (`firstPassRoots`, `FirstPassRoots`), `loader.go` (`ExtractPluginsFromTree`, `MergeCliPlugins`, `RegisterPluginExtractor`), `loader_extract.go` (`ExtractHubConfig`) - the first pass reads `plugin` and `environment`, and merges `--plugin`
- [ ] `internal/component/config/validate_sections.go` - `validatedSections` covers `plugin`, excludes `bgp`
- [ ] `internal/component/bgp/config/plugins.go`, `redistribute_binding.go`, `flowspec_binding.go`, `filter_state_binding.go` - `extractBGPInlinePlugins`, `wireRedistributeDelivery`, `requireSelectionFeeds`, `requireRIBDelivery`, `grantFilterOwnersState`
- [ ] `internal/component/bgp/reactor/config.go`, `send_permission.go` - `EnsureProcessBinding`, `GrantImplicitReceive`, `Peer.maySend`, `bindingsMaySend`, `filterPermittedPeers`, `sendPermissionDenied`, `recordSendRefused`
- [ ] `internal/component/plugin/server/delivery_graph.go`, `delivery_reconcile.go`, `dissemination_validate.go` - `DeliveryGraph` keyed by peer address, `ProcessEdges.Implicit`, `PeerScopedProcs`, `reconcileDelivery`, `ValidateDisseminationBindings`, `disseminationAttachLine`
- [ ] `internal/component/plugin/server/dispatch_state.go`, `internal/core/statestore/permissions.go` - state grants checked against `proc.Name()`, the operator's alias
- [ ] `internal/component/bgp/server/events.go` - `externalIdentity`, the delivery-order identity no registry name can take
- [ ] `internal/exabgp/migration/migrate.go`, `internal/exabgp/bridge/bridge_neighbor.go`, `internal/plugins/exabgp/bridgerun/script.go`, `internal/component/bgp/plugins/cmd/peer/create.go` - the migration writes `attach process`, the bridge relays each script command through `DispatchCommand`, and `create bgp peer` takes `attach`
- [ ] `internal/component/bgp/reactor/reactor_test.go` - the only callers of `GetPeerProcessBindings` are three tests

**Behavior to preserve:**
- Every plugin runs in the daemon or as a separate process over the five-stage protocol, and the SDK hides which.
- `ze --plugin ze.<name> -` and `ze --plugin "<program>" -` both keep starting the plugin they name, and a plugin named on the command line gives the same outcome as one declared in the config (`test/plugin/external-yang-command-line.ci`).
- Redistribution, FlowSpec authorization, AIGP selection and the filter owner's `state` reception keep working, and each keeps its refusal of a narrower operator list, with the text in bus syntax.
- A plugin never receives an event it registered no reception for, and never sends a command it did not declare.
- Offline `ze bgp decode --plugin` and `ze plugin test --plugin` keep accepting external decoders.
- Delivery to a peer-scoped event stays one index read with no allocation.

**Behavior to change:**
- Delete `container plugin` from `ze-plugin-conf.yang` and `container attach` from `ze-bgp-conf.yang`. Add `container bus { list endpoint; list flow; container hub; }`. No hybrid.
- Delete `extractBGPInlinePlugins`, `attach > content`, `ProcessBinding` and its derived-binding writers (`EnsureProcessBinding`, `GrantImplicitReceive`, `grantEveryPeer`), `GetPeerProcessBindings`, and the `attach` keyword of `create bgp peer`.
- Add the three declarations to `registry.Registration` and to Stage 1. Delete `Registration.EventTypes`, `Registration.SendTypes`, `RegisterPluginEventTypes`, `RegisterPluginSendTypes`, both `sync.Once`, and the default event namespace.
- Silence is no longer the default. A configured endpoint receives what it registered and sends what it declared until the operator narrows it. This reverses the 2026-11-18 ruling recorded on `architecture.md`.
- The send permission is plugin-wide: one allow-list on the endpoint. The per-peer grant has no successor, and a route server or reflector serves every peer connected to it.
- A program that was attached to some peers only now sees every peer. The converter writes no narrowing flow (A-Q7).
- A command that parses a config with no daemon running starts each declared external program in query mode and validates against its declaration (A-Q2).
- A compiled-in plugin run as a separate process keeps its registration's identity. The form `external <n> { run "ze plugin <x>" }` is converted to `internal <x>; process separate;`, and a `run` line always names a program Ze did not ship.
- Move `hub` under `bus` (`bus-hub` listener service).
- Fix the owned journal row and the 5 sibling alias-keying defects.
- Convert every repository `.conf`, `.ci`, `.et`, interop scenario, fixture, Go test, doc and website example through a parsed-tree converter in the same work.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Program and hub declaration | `internal/component/plugin/yang/ze-plugin-conf.yang` (`container plugin`); `internal/component/config/loader.go` (`ExtractPluginsFromTree`); `loader_extract.go` (`ExtractHubConfig`) | Declares `internal`, `external` and `hub` under `plugin { }` | Replaces: one `endpoint` list and `hub` under `bus` |
| Internal `use` and external `run` shapes | `ze-plugin-conf.yang` (`internal > use`, `external > run`, `external > use`); `internal/component/config/loader.go` (`MarkInternalPlugin`) | `use` names a registry row and runs it in the daemon. `run` forks a command line. `run "ze.<x>"` runs in the daemon. `run "ze plugin <x>"` forks and is named `<x>` | Replaces: `internal <x>` names the registration, `process` chooses the transport, `external { run }` is always a foreign program. The three overlapping spellings become two leaves that answer two questions |
| Foreign-program rule | `internal/component/plugin/types.go` (`PluginConfig.RunsExternalProgram`) and its ten callers | Answers both "is this registration the block's" and "does it run in another process" with one predicate | Replaces: the predicate splits into identity (foreign or compiled) and transport (in the daemon or separate). Each caller is assigned the question it asks. R3 needs a compiled identity on a separate process, which the single predicate cannot state |
| In-process-only declaration | `internal/component/plugin/registry/registry.go` (`Registration.RunsInProcessOnly`); `internal/component/plugin/doctor/check_external.go` | A plugin that works only in the daemon says so, and doctor reports a block that forks it | Extends: `process separate` on such a plugin is refused at commit with the same reason. This is a transport reason |
| Reception declared by the plugin | `pkg/plugin/sdk/sdk_callbacks.go` (`SetStartupSubscriptions`, `SetStartupSubscriptionsIn`, `AddStartupSubscriptionsIn`); ready RPC | The plugin states the events, peers and format it handles. Delivery is the overlap with the peer's grant | Replaces: the events and tag match move to the `Receives` declaration, which the SDK reads to subscribe. The format stays a ready-time parameter. No second declaration remains |
| Event types a plugin produces | `registry.Registration.EventTypes`; `internal/component/plugin/resolve.go` (`RegisterPluginEventTypes`, `registerEventTypesOnce`) | A list of type names registered lazily, once, into the default namespace | Deleted: replaced by the `Publishes` declaration, which names its namespace and tag keys and registers at `init()` |
| Send types a plugin enables | `registry.Registration.SendTypes`; `resolve.go` (`RegisterPluginSendTypes`, `registerSendTypesOnce`); `internal/core/events/events.go` (`ValidSendTypes`, `RegisterSendType`) | A flat global set of send type names, filled lazily, once | Deleted: replaced by command tokens registered under the component that executes them. `route_refresh` registers `enhanced-refresh` under `bgp` from its own `init()` |
| Default event namespace | `resolve.go` (`RegisterDefaultEventNamespace`, `DefaultEventNamespace`) | The namespace a namespace-less declaration or subscription resolves to (`bgp` today) | Deleted: every publication and reception names its namespace |
| Event namespace registry | `internal/core/events/events.go` (`RegisterNamespace`, `RegisterEventType`, `AllEventTypes`) | Holds namespaces and their event types with interned ids | Extends: a namespace's registration also holds the tag keys of each event type and the owner that publishes it |
| Claims delivered at Stage 2 | `registry.Registration.Claims`; `rpc.ConfigureInput.Claims` | The engine unions a declaration over the startup set and delivers it before any runtime event | Extends: the publication catalog is delivered the same way |
| External declarations read before the full parse | `pluginserver.HoldDeclaredPlugins`, `Manager.StartDeclared`; `config.SchemaWithDeclaredPlugins`, `DeclaredSchemas.ParseRefusal` | Each declared external program runs before the full parse and its Stage 1 declaration is kept. With no daemon, a section only its module defines is refused with a fix that reaches a running Ze | Extends: the same read supplies an external program's publications, receptions and sends. With no daemon, the commands that parse a config stop refusing and ask the program instead (next row) |
| Query mode | `docs/architecture/api/process-protocol.md` ("Query Mode: Stage 1 and No Stage 2"); `sdk.RunOrDeclare`; `rpc.WriteDeclaration`; `show plugin declarations`; the conformance check | A program started with `ZE_PLUGIN_MODE=declare` writes its Stage 1 line to stdout and exits 0, with no connection and no run loop. Only two commands use it | Extends: every command that parses a config with no daemon runs each declared external program this way and validates against the answer (A-Q2). `ParseRefusal` keeps its message for a program that cannot be asked |
| Per-peer attachment | `internal/component/bgp/yang/ze-bgp-conf.yang` (`container attach`); `internal/component/bgp/reactor/config.go` (`parseProcessBindingsFromTree`); `bgp/config/plugins.go` (`extractBGPInlinePlugins`) | Per-peer or per-group `attach process` with `receive`, `send`, inline `run`/`use` and dead `content` | Replaces: deleted from `bgp { }`. Registered receptions and `bus { flow }` take its place |
| Derived bindings | `bgp/config/redistribute_binding.go` (`wireRedistributeDelivery`, `grantEveryPeer`); `reactor/config.go` (`EnsureProcessBinding`) | Adds a Loc-RIB or orchestrator binding on every peer when a rule needs it | Deleted: a plugin that runs receives what it registered on every peer, so no binding is derived. Child 2 owns how the requirement is declared |
| Implicit state grant | `bgp/config/filter_state_binding.go` (`grantFilterOwnersState`); `reactor/config.go` (`GrantImplicitReceive`); `ProcessEdges.Implicit` | Grants a filter owner the peer's `state`, widening the operator's block, marked implicit | Replaces in part: the filter owner registers the reception of `state`. Where the operator's flow omits `state`, the delivery stays and stays marked implicit, as today. Child 2 owns the origin field |
| Narrow-grant refusals | `bgp/config/redistribute_binding.go` (`requireSelectionFeeds`); `flowspec_binding.go` (`requireRIBDelivery`) | Refuses a Loc-RIB binding narrower than FlowSpec or AIGP needs, naming `attach process` | Extends: the refusals stay. They judge what the Loc-RIB receives, so each names the `bus { flow }` line that restores `update-received` and `state` for it; a flow carries no send list. Where `requireRIBDelivery` reads a forwarder's `send [ update ]` today, it reads the plugin's declared `bgp:update` and the endpoint's one send allow-list. Their literals (`bgp-rs`, `bgp-rr`, `bgp-adj-rib-in`) are Child 2's |
| Dissemination role check | `internal/component/plugin/server/dissemination_validate.go` (`ValidateDisseminationBindings`, `disseminationAttachLine`); `rpc.DisseminationDecl` | Judges the operator's per-peer blocks against the grants a peer-output role declares | Extends: judged against the plugin-wide declaration and allow-list, with the fix line in bus syntax. A role serves every peer connected to it (RFC 7947 Section 1, RFC 4456 Section 6), so the per-peer part of the check is deleted |
| Delivery reconcile | `internal/component/plugin/server/delivery_reconcile.go` (`reconcileDelivery`, `deliveryDisagreements`); `doctor_snapshot.go` (`DeliveryDisagreement`) | Reports each peer, process and event the plugin's subscription and the peer's grant disagree about | Extends: reports a flow that lists an event the endpoint never subscribed to, and a dormant reception, with fixes in bus syntax |
| Delivery graph and send permission | `internal/component/plugin/server/delivery_graph.go` (`DeliveryGraph`, `Receivers`, `MaySend`, `PeerScopedProcs`); `internal/component/bgp/reactor/send_permission.go` (`Peer.maySend`, `filterPermittedPeers`) | One index keyed by peer address, built from each peer's bindings. The send check asks each peer | Extends: see "How delivery is indexed". This spec owns the index. Child 2 owns the origin field and its display |
| Send refusal reporting | `send_permission.go` (`sendPermissionDenied`, `recordSendRefused`, `errSendNotPermitted`) | A log line, a counter by process and send type, and an operator error with the `attach process` fix | Extends: the texts name the endpoint's send allow-list, and the fix is the exact line `set bus endpoint <name> send allow <component>:<token>`; a flow carries no send list. A send the plugin did not declare names no config fix. Child 3 generalizes the counter to every operation |
| Runtime subscribe | `delivery_graph.go` (`PeerScopedProcs`, `DiscardRuntimeSubscriptions`) | A `request subscribe` adds a type only inside the peer's grant, until the next apply | Extends: bounded by the endpoint's registration and the operator's flows instead |
| Process alias resolution | `internal/component/plugin/process/process.go` (`RegistrationName`); `internal/component/plugin/resolve.go` (`RegistryName`, `RegistryNames`) | Separates the configured alias from the compiled registry name | Extends: used at the state, doctor and lifecycle sites. `deriveName`'s `ze plugin X` branch is deleted with the spelling it served |
| Identity no registry name can take | `internal/component/bgp/server/events.go` (`externalIdentity`) | Keeps a program named `bgp-gr` off the node of the registered `bgp-gr` | Extends: the same construction keys state grants and doctor snapshots for a foreign program, so it never matches a compiled grant |
| Retired keyword hints | `internal/component/config/retired.go` (`retiredKeywords`, `RetiredKeywordHint`) | Maps a retired keyword to a migration hint without accepting it | Extends: `plugin`, `attach` and `process` point to `bus` |
| CLI `--plugin` flag | `internal/component/config/loader.go` (`MergeCliPlugins`); `internal/component/plugin/resolve.go` (`ResolvePlugin`) | Adds a compiled-in plugin or an external program to the declared set before the first pass | Extends: the merge produces endpoints. Nothing is restricted. The first design's built-in-only rule is withdrawn (R4, A-Q3) |
| First-pass roots | `internal/component/config/yang/join.go` (`firstPassRoots`, `FirstPassRoots`); `yang_schema.go` (`PluginOnlySchema`) | A literal list, `plugin` and `environment` | Replaces: the module that owns a root read before the full parse declares that on the root, and the list is derived. The literal names a specific root in config code, which the modular-engine rule forbids |
| Plugin extraction seam | `internal/component/config/loader.go` (`RegisterPluginExtractor`) | A section's owner registers a function that extracts plugin configs from the tree | Extends: the `bus` reader is owned by `internal/component/plugin`, beside the YANG module, and registered through this seam. No `ExtractBusFromTree` is added to the config package |
| ExaBGP migration and bridge | `internal/exabgp/migration/migrate.go`, `migrate_serialize.go`; `internal/exabgp/bridge/bridge_neighbor.go`; `internal/component/bgp/plugins/cmd/peer/create.go` | Emits `plugin { }`, `attach process`, and `create bgp peer ... attach` | Extends: emits the `bus` section, and `create bgp peer` loses `attach` because a new peer is covered by the registered receptions |
| `ze config graph` | `internal/component/config/graph.go` (`listenerSectionName`) | Draws listeners and the bindings the config holds | Extends: reads `bus` and `bus-hub` |
| Config tree parser and corpus validator | `internal/component/config/serialize.go`, `serialize_set.go`; `internal/component/config/cli/validate_corpus_test.go` | Parses and serializes `*config.Tree` and validates every tracked config | Extends: drives the one-shot converter |

### How delivery is indexed

| Question | Answer |
|----------|--------|
| Does `DeliveryGraph` index by tags or project onto peers? | It projects. A publisher hands the graph its subjects, each with a key and its stable tags. BGP hands one subject per peer: the address as key, and the tags `peer` and `group`. The graph resolves every reception and flow against each subject's tags when it is built, and stores the receivers per subject and event |
| When is it built? | Where it is built today: after every peer change (`AddPeer`, `createDynamicPeer`, `completeRemovals`, the end of a config apply). A peer added later is covered by that rebuild with no config change |
| What does one delivery cost? | One map read on the subject and one on the event key, as today, with no allocation. `match` adds nothing per message for a stable tag |
| What about a tag that differs per message? | A publisher registers such a tag as per-message (`source` on a route change). Only a flow or reception that matches on it pays: one comparison of interned ids per such entry. An event with no such entry pays nothing |
| What proves it? | A benchmark of `Receivers` and of the per-message path reports allocations per operation, and the test fails above zero. Allocation figures are valid on any host |
| Does the bus learn a BGP word? | No. Subjects, tag keys and value validators are registered by the publisher |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "A process Ze forks from `plugin { external <name> { run ...; } }`, or one built on `pkg/plugin/sdk`" | departs | Replaces `plugin { external <name> { run ...; } }` with `bus { endpoint <name> { external { run ...; } } }`; door updated in the same work |
| `ai/doors/plugin-infra.md` | Who depends on it from outside | covering door for `internal/component/plugin/` | "`plugin { external <name> { run ...; } }` (`internal/component/plugin/yang/ze-plugin-conf.yang`)" | departs | Replaces `plugin { external }` with `bus { endpoint <name> { external { run ...; } } }`; door updated in the same work |
| `ai/doors/bgp-engine.md` | Who depends on it from outside | covering door for `internal/component/bgp/` | "Which process sees which peer is the peer's `attach process` block" | departs | Removes `attach process` from `bgp { }`; a plugin's registered receptions and `bus { flow }` decide; door updated in the same work |
| `ai/doors/bgp-plugins.md` | Who depends on it from outside | covering door for `internal/component/bgp/plugins/cmd/peer/` | "and the alias form `internal rs { use bgp-rs }`" | departs | Replaces `internal rs { use bgp-rs }` with `bus { endpoint rs { internal bgp-rs; } }`; door updated in the same work |
| `ai/doors/core.md` | Boundaries | covering door for `internal/core/events/` | "Never import upward." | agrees | `internal/core/events` stays leaf-only; components register namespaces, tag keys and command tokens into core from `init()` |
| `ai/doors/config-yang.md` | Boundaries | covering door for `internal/component/config/` | "Config text changes through a parsed tree or `set` lines only." | agrees | The repository-wide conversion uses a parsed `*config.Tree` converter |
| `ai/doors/system-plugins.md` | Boundaries | covering door for `internal/exabgp/` and `internal/plugins/ddos/flowspec/` | "A plugin owns its whole surface." | agrees | The ExaBGP migration and bridge and the `ddos-flowspec` doctor stay inside their own packages |
| `ai/doors/hub-engine.md` | Boundaries | covering door for `cmd/ze/` and `internal/component/managed/` | "The hub and the Engine carry no BGP knowledge." | agrees | The bus, the hub and the engine stay free of BGP concepts |
| `ai/doors/le-tooling.md` | Boundaries | covering door for `test/` `.ci` files | "Never import `internal/le` from product code" | agrees | Product packages import nothing from `internal/le`; new tests live under `test/` |
| `docs/architecture/plugin/plugin-system.md` | One plugin, two ways to reach the bus | page the brief and the plugin door name | "A behaviour that differs between the two kinds with no reason of this kind is a defect." | agrees | Publications, receptions and sends are declared and judged alike for both kinds. The differences that remain are transport reasons and are stated: an external declaration is taken by running the program in query mode where a compiled-in one is read from the registry, and `RunsInProcessOnly` |
| `docs/architecture/plugin/plugin-system.md` | One plugin, two ways to reach the bus | same page | "With no daemon running, no command starts it, and a config section only its module defines is not available" | departs | Reversed by Thomas on 2026-11-23, confirmed as intended: with no daemon, a command that parses a config starts the program in query mode, takes its declaration and validates against it; page updated in the same work |
| `docs/architecture/api/architecture.md` | Owner ruling: endpoints and flows on the bus | `// Design:` page of `plugin/registry`, `bgp/config`, `bgp/reactor`, `plugin/server`; modified page | "A BGP group or peer takes part in a flow by listing its name, and the list merges through groups as every list does." | departs | Reversed by Thomas on 2026-11-23: no flow in `bgp { }`, no BGP concept in `bus { }`, a flow narrows the receptions a plugin registered and matches generic tags, and sends are narrowed by one allow-list on the endpoint; page updated in the same work |
| `docs/architecture/api/architecture.md` | Owner ruling: implied and shown, or written | same page | "Where the operator has a choice, which program watches a peer and what it may send, the operator writes it, and silence stays the default." | departs | Reversed by R1 and R7 on 2026-11-23: a configured plugin receives what it registered and sends what it declared, shown by `\| implied`, until the operator writes an allow-list; page updated in the same work |
| `docs/architecture/api/architecture.md` | What the Plugin Declares and What the Peer Grants | same page | "A plugin declares what it CAN handle, in its ready RPC." | departs | The declaration moves to the registration and becomes the default delivery, not the upper bound of a peer's grant; page updated in the same work |
| `docs/architecture/api/architecture.md` | Data Flow: Config to Delivery | same page | "A relay is judged on the grant of the process that sends it, and on nothing else: no process lends its grant to another." | departs | The grant becomes plugin-wide: a relay is judged on the sender's declaration and allow-list, with no per-peer part. A forwarder serves every peer connected to it, as RFC 7947 Section 1 states for a route server; page updated in the same work |
| `docs/architecture/config/syntax.md` | The first pass: plugin declarations before the full parse | `// Design:` page of `config/first_pass.go`, `yang_schema.go`, `retired.go`, `listener.go`, `graph.go`, `bgp/config/`; modified page | "`PluginOnlySchema` holds the roots `plugin` and `environment` and no other." | departs | The first-pass roots become `bus` and `environment`, derived from a declaration on the root; `RetiredKeywordHint` points `plugin`, `attach` and `process` to `bus`; page updated in the same work |
| `docs/architecture/config/yang-config-design.md` | Decided, not built: modules compiled at build time | `// Design:` page of `validate_sections.go`, `validators.go`; modified page | "The `--plugin` flag keeps starting the plugin it names" | agrees | `ze start --plugin` keeps starting a compiled-in plugin or an external program (R4). The page's examples change to `bus` syntax only |
| `docs/architecture/core-design.md` | 1. System Architecture | `// Design:` page of `bgp/reactor/`, `bgp/server/events.go`, `exabgp/migration/`; modified page | "Phase 2: explicit plugins (from config `plugin { }` block)." | departs | Phase 2 reads explicit endpoints from `bus { endpoint ... }`; page updated in the same work |
| `docs/architecture/core-design.md` | Design decisions | decision row on `docs/architecture/core-design.md` | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | The bus cutover does not alter the VPP and kernel commit checks |
| `docs/architecture/core-design.md` | Design decisions | decision row on `docs/architecture/core-design.md` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | The data-plane guard is untouched; the `bus` refusals follow the same rule of one verdict at start, commit, reload and validate |
| `docs/architecture/core-design.md` | Design decisions | decision row on `docs/architecture/core-design.md` | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "nft"` annotation." | agrees | The bus cutover touches no firewall verifier and no `ze:backend` annotation |
| `docs/architecture/firewall/firewall-domain-group.md` | What the functional tests drive | anchored to `pkg/plugin/sdk/sdk_callbacks.go`; modified page | "`plugin { external ... }`" | departs | The sentence names the old block; it becomes `bus { endpoint ... { external ... } }`; page updated in the same work |
| `docs/architecture/api/wire-format.md` | Transport | anchored to `pkg/plugin/rpc/types.go`, `ze-plugin-engine.yang` | - | silent | Covers answer framing and item types; holds no statement on plugin config syntax, subscriptions or send permission |
| `docs/architecture/firewall/firewall-irr.md` | Decisions | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers IRR-fed firewall sets; silent on `bus` config and plugin registration of events |
| `docs/architecture/forked-route-install.md` | The wire carries a name, the engine resolves it | anchored to `pkg/plugin/rpc/types.go` | - | silent | Covers the `route-install` wire for a forked plugin; silent on who may call it, which Child 3 owns |
| `docs/architecture/plugin-manager-wiring.md` | Problem | anchored to `pkg/plugin/rpc/types.go`, `internal/component/plugin/types.go` | - | silent | Covers the spawn and handshake phases; holds no statement on plugin config syntax, subscriptions or send permission |
| `docs/architecture/update-cache.md` | Purpose | anchored to `pkg/plugin/rpc/types.go`, `plugin/process/process.go` | - | silent | Covers the recent-UPDATE cache; silent on `bus` config syntax |
| `docs/architecture/cli/error-surface.md` | The rule | anchored to `internal/component/plugin/types.go` | - | silent | Covers where a fault appears on each surface; silent on `bus` config syntax |
| `docs/architecture/encoding-context.md` | TL;DR (Read This First) | anchored to `internal/component/plugin/types.go` | - | silent | Covers per-peer encoding contexts; silent on `bus` config syntax |
| `docs/architecture/plugin/show-enrichers.md` | The decisions | anchored to `plugin/process/process.go` | - | silent | Covers show enrichers; silent on `bus` config syntax |
| `docs/architecture/cli/tui-launcher.md` | Decisions | anchored to `cmd/ze/ze_core_dispatch.go` | - | silent | Covers the TUI launcher; silent on `bus` config syntax |
| `docs/architecture/api/process-protocol.md` | Event Delivery | `// Design:` page of `core/events/events.go`, `plugin/resolve.go`, `startup_autoload.go`, `startup.go`, `dispatch_state.go`; modified page | "the plugin subscribed to the type in that direction, and the peer's `attach process <name>` block grants it." | departs | Delivery needs the plugin's registered reception and, where the operator wrote one, the flow that keeps it. Stage 1 gains the three declarations; page updated in the same work |
| `docs/architecture/api/process-protocol.md` | Persistent runtime state | same page | "Requests use the daemon's configured process name as identity." | departs | A compiled-in plugin's requests use its registration name, under any endpoint name and on either transport. A foreign program's use an identity no registry name can take; page updated in the same work |
| `docs/architecture/api/commands.md` | Event Subscription Commands | `// Design:` page of `cmd/peer/create.go`, `plugin/server/startup.go`, `managed/cmd_fleet.go`; modified page | "the process subscribed to the event, and the peer's `attach process <name>` block grants that type in that direction." | departs | Replaces `attach process <name>` with the registration and `bus { flow }`; page updated in the same work |
| `docs/architecture/api/commands.md` | Design decisions | decision row on `docs/architecture/api/commands.md` | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Child 1 adds no command argument that belongs in a pipe (`create bgp peer` removes `attach`) |
| `docs/architecture/api/commands.md` | Design decisions | decision row on `docs/architecture/api/commands.md` | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Child 1 adds no pipe operation or override (`\| implied` is in Child 2) |
| `docs/architecture/fleet-config.md` | Unified Named Blocks | `// Design:` page of `check_managed_listener.go`, `managed/client.go`, `cmd_fleet.go`, `managed_owner.go`; modified page | "All hub configuration uses explicit `server` and `client` keywords under `plugin { hub { } }`:" | departs | Moves `hub { server; client; }` under `bus { hub { } }` (listener `"plugin-hub"` becomes `"bus-hub"`); page updated in the same work |
| `docs/architecture/hub-architecture.md` | Config File | `// Design:` page of `data_rpc.go`, `cmd/ze/hub/main.go`; modified page | "external acme {" | departs | Replaces the top-level `plugin { external acme { ... } }` layout with `bus { endpoint acme { external { ... } } }`; page updated in the same work |
| `docs/architecture/system-architecture.md` | Overview | `// Design:` page of `cmd/ze/ze_core_start.go`; modified page | "`plugin { external ... }` block" | departs | External programs are declared under `bus { endpoint <name> { external { ... } } }`; page updated in the same work |
| `docs/architecture/rib-transition.md` | Owner ruling: a learned route needs the recipient's export policy | anchored to `ze-bgp-conf.yang`; modified page | "The `bgp-rib` plugin is fed a peer's routes, and may send to a peer, only where that peer's `attach process` block grants it, as for any process" | departs | `bgp-rib` is fed every peer's routes by its registered reception and sends by its declaration, unless the operator narrows either; export policy still governs each advertisement; page updated in the same work |
| `docs/architecture/bgp/structural-forwarding.md` | Receive validation and cache changes | anchored to `plugin/registry/registry.go`; modified page | "the source's `attach process <route server> { receive [ update-received ] }`" | departs | Replaces `attach process` references with the route server's registered reception and `bus { flow }`; page updated in the same work |
| `docs/architecture/api/capability-contract.md` | Config Validation (✅ DONE) | anchored to `bgp/config/peers.go`; modified page | "configured: attach process logger, attach process monitor - none carry either word" | departs | Updates the `validatePeerProcessCaps` refusal text and examples to bus syntax; page updated in the same work |
| `docs/architecture/plugin/plugin-system.md` | Registration fields | anchored to `plugin/registry/registry.go`, `resolve.go`, `startup_autoload.go`, `startup.go`; modified page | "The fast path then relays a source only when the source's attach block feeds this plugin its UPDATEs (`receive [ update-received ]`), and reaches a destination only when the destination grants this plugin `send [ update ]`" | departs | The fast path reads the plugin's reception of the source and its plugin-wide send; the per-destination grant is gone and every connected peer is a destination; the `EventTypes` and `SendTypes` rows become the three declarations; page updated in the same work |
| `docs/architecture/doctor-and-health-checks.md` | Plugin registration and delivery checks | `// Design:` page of `doctor_snapshot.go`, `check_external.go`; modified page | "the `attach process` change (`DeliveryDisagreement.Fix`)" | departs | `DeliveryDisagreement.Fix` names `bus { flow }`, and `PluginDoctorState` carries the registration name and the transport; page updated in the same work |
| `docs/architecture/exabgp-bridge.md` | A neighbor command is one of three things | `// Design:` page of `bridge_neighbor.go`; modified page | "`attach exabgp-bridge`, once" | departs | Drops the `attach` keyword from `create bgp peer`: the bridge's registered reception covers a peer created later; page updated in the same work |
| `docs/architecture/update-building.md` | Shared ownership admission | anchored to `exabgp/migration/migrate.go`; modified page | "when no `attach process` block grants the owner process the owning source's" | departs | Updates the `attach process` reference to the owner's reception; page updated in the same work |
| `docs/architecture/update-building.md` | Design decisions | decision row on `docs/architecture/update-building.md` | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | The ExaBGP migration keeps writing `session { flowspec-redirect-next-hop true; }` while it emits the `bus` section |
| `docs/guide/ddos-mitigation.md` | FlowSpec mode sensor blindness | `// Design:` page of `ddos/flowspec/doctor.go`; modified page | "A block of another name (`plugin { internal <name> { use ddos-flowspec } }`)" | departs | Replaces the block with `bus { endpoint <name> { internal ddos-flowspec; } }` and resolves the alias by the registration name; page updated in the same work |
| `docs/config-reference.md` | Plugin Loading | modified page | "Plugins are declared in the `plugin { }` block:" | departs | Replaces the `plugin { }` and `attach process` sections with `bus { endpoint; flow; hub; }`; page updated in the same work |
| `docs/exabgp/exabgp-migration.md` | Key Differences | modified page | "`attach process <name> { }`" | departs | The migration emits the `bus` section and pure `bgp { }`; page updated in the same work |
| `docs/architecture/testing/ci-format.md` | Where the block goes | anchored to `cmd/ze/hub/main.go` | "The daemon `-` is recognized by POSITION, not by a list of verbs:" | agrees | `ze --plugin <name-or-program> -` invocations keep the same stdin routing |
| `docs/architecture/config/transaction-protocol.md` | Phase 1: Verify | anchored to `plugin/registry`, `loader_create.go`, `reactor_api.go`, `startup.go`; modified page | "`plugin { internal rpki { use bgp-rpki } }` names the process `rpki`, and its `pki` section still arrives" | departs | Replaces the examples with `bus { endpoint rpki { internal bgp-rpki; } }` and `bus { endpoint bgp-rpki { external { run "/opt/x"; } } }`, and states the identity and transport questions apart; page updated in the same work |
| `docs/architecture/plugin/component-boundaries.md` | The four components | anchored to `internal/core/events/events.go` | - | silent | Covers the four public `pkg/ze` interfaces; silent on config syntax |
| `docs/guide/flowspec-protected-router.md` | 2. Update the active stored config | `// Design:` page of `bgp/config/flowspec_binding.go`; modified page | "Execution mode comes from the effective plugin declaration: `use bgp-rib` runs in-process even inside an `external` stanza, while `run "ze plugin bgp-rib"` forks a subprocess." | departs | Execution mode comes from the endpoint's `process` leaf; `internal bgp-rib; process separate;` replaces `run "ze plugin bgp-rib"`, and FlowSpec authorization still refuses it because its lookups are process-local; page updated in the same work |
| `ai/rules/repo-maintenance.md` | Directives | `// Design:` page of `managed/doctor.go` | - | silent | Covers generated-file maintenance and doctor registration hygiene; silent on `bus` config syntax |
| `docs/architecture/api/update-syntax.md` | Raw Passthrough Commands | anchored to `ze-bgp-conf.yang`, `registry.go`; modified page | "The peer must attach the program with `send [ raw ]`; no other send type permits it." | departs | A program sends raw when it declared `raw` toward `bgp` and no allow-list removes it; page updated in the same work |
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
| `docs/architecture/plugin/rib-storage-design.md` | Overview | anchored to `redistribute_binding.go`; modified page | "the message names the peer and the forwarder, and tells the operator to attach the RIB to the peer with `receive [ update-received ]` or to take the peer down around the change." | departs | Replaces the per-peer attach wording and the `source_policy_reload.go` refusal text with the flow line that keeps `update-received`; page updated in the same work |
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

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config file, stdin, editor commit and SIGHUP reload (`bus { endpoint ...; flow ...; hub ...; }`, `bgp { ... }`), `ze start --plugin <name-or-program>`, `ze exabgp migrate`, the Stage 1 declaration of an external program, and the runtime plugin RPCs (`state-*`, the doctor snapshot, the BGP send commands).

### Transformation Path
1. **Registration at `init()`:** each component and compiled-in plugin registers its namespaces, event types and tag keys, the command tokens it accepts, and its three declarations. Nothing is registered lazily.
2. **First pass (`config.FirstPass`):** the roots read before any plugin runs are derived from the modules that declare them (`bus`, `environment`). The owner of `bus` extracts the endpoints through the `RegisterPluginExtractor` seam. `MergeCliPlugins` adds each `--plugin` as an endpoint of the kind it names.
3. **External declarations:** `HoldDeclaredPlugins` runs each declared external program and keeps its Stage 1 declaration, as today for its YANG modules. Its publications join the registry. The publication set is now complete.
4. **Full parse and validation (`config.LoadConfig`, `ze config validate`):** `ValidateCustomSections` judges `bus`: `from` and `to` are known parties, `namespace` is owned by `from`, each `event` is published by `from` and received by `to`, each `match` tag is registered for those events and its values pass the publisher's validator, and each `send` entry of an endpoint is a send that plugin declared. With no daemon running, the command takes each external program's declaration in query mode first, and fails with the endpoint and the cause when a program cannot be asked.
5. **Resolution:** the bus builds the effective edges: for each pair, the registered receptions, replaced by the operator's flows where the operator wrote any; for each plugin, the declared sends, cut down to its one allow-list where one is present. A reception with no publisher is recorded as dormant.
6. **Catalog delivery:** Stage 2 configure carries the publication catalog to each plugin, beside `Claims`. The ready RPC then subscribes from the plugin's declaration.
7. **Delivery index:** each publisher hands the graph its subjects and their stable tags. BGP republishes after every peer change. The graph projects the edges onto the subjects.
8. **Send check:** a BGP send command asks the graph once whether the sending plugin holds the send type, in place of asking every peer. Child 3 moves the same question to every engine operation.
9. **Process identity:** `process.Process` answers three things apart: the endpoint name, the registration name of a compiled-in plugin, and the transport. State, doctor and lifecycle sites key on the registration name for a compiled-in plugin on either transport, and on an identity no registry name can take for a foreign program.
10. **ExaBGP migration:** emits the bridge endpoint, no flow and no send list, and pure `bgp { }`. The bridge declares its receptions and the whole ExaBGP command set. Which neighbor feeds which script stays in the bridge's own `exabgp { bridge { process { feed } } }` blocks, which the migration already writes.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `internal/core/events` <-> `internal/component/config` | Registries filled at `init()` and read by the `bus` validators | No |
| `internal/component/plugin` <-> `internal/component/config` | The `bus` reader registered through `RegisterPluginExtractor`; roots declared on the module | No |
| `internal/component/bgp` <-> `internal/component/plugin/server` | BGP registers tag keys and a value validator, and publishes subjects with their tags | No |
| Engine <-> plugin process, either kind | Stage 1 carries `publishes`, `receives` and `sends`; Stage 2 carries the catalog | No |

### Changed-Wire Census

**Search evidence:** Searched `pkg/plugin/rpc/types.go` for the fields of `DeclareRegistrationInput` (positive control: `Claims`, `Dissemination`, `FilterInsertions` are present). It holds no event, reception or send declaration, so R1, R2 and R7 add three fields. Searched `pkg/plugin/sdk/sdk_callbacks.go` for the reception a plugin declares today (positive control: `SetStartupSubscriptions` and its two siblings, 19 call sites in the tree). Searched `internal/core/ipc/yang/` for the Stage 1 and Stage 2 schemas that must follow. The first design's claim that no plugin wire changes is withdrawn.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Ze config grammar | writer | `internal/exabgp/migration/migrate_serialize.go` (`SerializeTree`), `internal/component/config/serialize.go` (`Serialize`), about 1,752 tracked configs, tests and docs | Phase 1 census: 248 Go, 1,197 `.ci`, 2 `.et`, 148 interop, 74 other test, 55 docs, 9 website, 11 `ai/` and `etc/` | Changed in the same work by the parsed-tree converter; `retired.go` gives the hints |
| Ze config grammar | reader | `internal/component/config/first_pass.go` (`FirstPass`), `loader.go` (`ExtractPluginsFromTree`), `loader_extract.go` (`ExtractHubConfig`), `internal/component/bgp/reactor/config.go` (`parseProcessBindingsFromTree`), `internal/component/plugin/doctor/check_external.go` | Read at each producer | Changed in the same work; `plugin` and `attach` are deleted first |
| Stage 1 declare-registration | writer | `pkg/plugin/sdk` (every plugin built on it), every external fixture under `internal/test/fixture/` and `test/`, the ExaBGP bridge's SDK runner, a third-party plugin in any language | `DeclareRegistrationInput` in `pkg/plugin/rpc/types.go` | Changed directly, every plugin in the tree rebuilt (`ai/META-DECISIONS.md`, "Ze is pre-release"); no version gate |
| Stage 1 declare-registration | reader | `internal/component/plugin/server/stage1_checks.go` (the Stage 1 check table), `pluginserver.HoldDeclaredPlugins`, `ze show plugin declarations`, the conformance checker | Read at `engineStartupSink.site` | Changed in the same work; the check table gains one entry per declaration |
| Query-mode declaration line | writer | `pkg/plugin/sdk/sdk_query.go` (`RunOrDeclare`, for a plugin binary Ze does not carry: a third-party program and every external fixture built on the SDK) and `internal/component/plugin/cli/main.go` (for a plugin Ze carries), both through `pkg/plugin/rpc/declaration.go` (`WriteDeclaration`) | Read at `runOrDeclare` and `WriteDeclaration`: under `ze.plugin.mode=declare` (`ZE_PLUGIN_MODE` in the environment, `sdk.EnvPluginMode`, `sdk.ModeDeclare`) the program writes one newline-framed request line, id 1, method `declare-registration`, the compact JSON of `DeclareRegistrationInput`, never calls its activation function, and exits 0 | The env value and the framing are unchanged. The line carries the three declarations because it is the Stage 1 payload, changed with the Stage 1 rows above; no version gate |
| Query-mode declaration line | reader | `internal/component/plugin/declarations.go` (`QueryDeclaration`; `queryChildEnv` sets `ZE_PLUGIN_MODE=declare` from `sdk.EnvPluginMode`; `declarationFromStdout` decodes through `rpc.DecodeDeclaration`). Readers today: `show plugin declarations` (`queriedDeclarationRow`) and the conformance check (`internal/component/plugin/conformance/conformance.go`). Added here: every command that parses a config with no daemon, through `config.SchemaWithDeclaredPlugins` and `PublishDeclaredSchemas` | Read at `queryChildEnv` and `declarationFromStdout` | New consumer in the same work (A-Q2). It reads the line through the decoder the daemon's Stage 1 uses, so the offline and daemon readers cannot accept different declarations |
| Stage 2 configure | writer | `internal/component/plugin/server` (the configure stage, where `rpc.ConfigureInput.Claims` is filled) | `Claims` comment in `registry.go` | Additive field for the publication catalog, same work |
| Stage 2 configure | reader | `pkg/plugin/sdk` configure callback; a third-party plugin in any language | `Claims` comment in `pkg/plugin/rpc/types.go` | Additive field; a plugin that ignores it is unaffected |
| Ready RPC startup subscriptions | writer | `sdk.Plugin.SetStartupSubscriptions`, `SetStartupSubscriptionsIn`, `AddStartupSubscriptionsIn` (19 call sites) | `pkg/plugin/sdk/sdk_callbacks.go` | The event and tag part is replaced by the `Receives` declaration; the calls are deleted; the format stays |
| Ready RPC startup subscriptions | reader | `internal/component/plugin/server/dispatch.go` (`registerSubscriptions`) | Named in the `delivery_graph.go` comment on the wildcard expansion | Reads the declaration held from Stage 1; same work |
| create bgp peer command | writer | `internal/exabgp/bridge/bridge_neighbor.go` (`convertNeighborCreate`), operators, tests | `convertNeighborCreate` emits `attach exabgp-bridge` | Changed in the same work: the keyword is dropped |
| create bgp peer command | reader | `internal/component/bgp/plugins/cmd/peer/create.go` (`peerCreateKeywords`, `setAttach`), `ze-peer-cmd.yang` | `setAttach` writes `attach.process.<name>` | Changed in the same work: `attach` and `setAttach` are removed |
| Listener service name | writer | `internal/component/config/listener.go` (`DiscoverListenerServices`), `graph.go` (`listenerSectionName`) | Derives `<top>-<child>`: `plugin-hub` becomes `bus-hub` | Changed in the same work |
| Listener service name | reader | `internal/component/config/cli/` (`show listeners`), `internal/component/managed/doctor.go`, `internal/component/plugin/doctor/check_managed_listener.go` | Phase 1 audit | Changed in the same work |
| Send refusal text | writer | `internal/component/bgp/reactor/send_permission.go` (`sendPermissionDenied`, `errSendNotPermitted`) | Read at the producer | Text changes to bus syntax here; Child 3 owns the counter's generalization |
| Send refusal text | reader | The `.ci` files that match the text (34 files name the refusal), operators and their log matchers | Grep of `test/` for the refusal strings | Each matching `.ci` is converted with the text, through the test judge where it proves the per-peer refusal |

### Integration Points
- `internal/component/plugin/registry/registry.go` (`Registration`) - the three declarations replace `EventTypes` and `SendTypes`.
- `internal/core/events/events.go` - a namespace's registration holds tag keys and its publisher; command tokens are registered per accepting component.
- `internal/component/config/yang/join.go` (`FirstPassRoots`) - derived from a declaration on the root.
- `internal/component/config/validate_sections.go` (`validatedSections`) - `bus` replaces `plugin`.
- `internal/component/plugin/server/delivery_graph.go` (`DeliveryGraph`) - built from subjects, receptions and flows.
- `internal/component/plugin/server/dispatch_state.go`, `doctor_snapshot.go` - identity by registration name or by the foreign identity.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | One config path through `FirstPass` and `LoadConfig`; one resolution of edges feeds delivery, the send check and the reconcile |
| No unintended coupling (components stay isolated) | Yes | `bus` carries no BGP keyword; tag keys, subjects and validators are registered by the publisher; `internal/core/events` stays a leaf |
| No duplicated functionality (extends existing, does not recreate) | Yes | The reception a plugin declares, the claims delivery, the held external declarations, the alias resolution and the foreign identity are each extended. Every replaced declaration is deleted, listed under Existing Mechanisms |
| Zero-copy preserved where applicable (refs, not copies) | Yes | UPDATE buffers and `RawMessage` lifetime are untouched; the index stores receivers per subject, and the benchmark asserts no allocation |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Plugins and components register publications, tag keys, value validators, command tokens and declarations from `init()`; the bus validates from those registries |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | `firstPassRoots` (`config/yang/join.go`) is a literal list that would learn `bus`; this design derives it. `validatedSections`, `retiredKeywords`, `listenerSectionName` and `peerCreateKeywords` were searched. `requireRIBDelivery` keeps three plugin-name literals until Child 2. The row turns Yes when the derived list is built and Child 2 lands |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Dropping `attach > content` removes no working feature | `delivery_graph.go` (`addBinding`) drops both fields; `bgp/server/events.go` reads only the process's encoding | A consumer depends on per-attachment encoding | Repo-wide grep for `ProcessBinding` readers | validated |
| A-2 | No repository test or scenario depends on a program not seeing a peer, apart from the tests that prove that silence itself | Not measured. The converter writes no narrowing flow (A-Q7), so every such program sees every peer after conversion | A scenario's result changes because a program now receives a peer's events | The receive census under "Receive census" | unvalidated |
| A-3 | The events a compiled-in plugin subscribes to are a constant of the plugin, not a function of its config | 19 call sites of the three subscription calls; `internal/plugins/redistribute/register.go` subscribes to `bgp` `state` only when the namespace exists, which the dormant rule covers; the ExaBGP bridge varies only the format | A plugin whose reception depends on its config cannot declare it as a pure value | Read of all 19 call sites at implementation start | unvalidated |
| A-4 | No repository test or scenario depends on a plugin being refused a send toward one peer while it is allowed toward another, apart from the tests that prove that refusal itself | Grep: 34 `.ci` and config files and 6 Go test files name the refusal; 520 `.ci` files carry a `send [` list | A scenario's routing depends on the per-peer refusal | The census under "Per-peer send census" | unvalidated |
| A-5 | Every event a publisher emits can carry the tag keys it registers | BGP peer-scoped delivery already carries the peer address and name (`PeerScopedProcs`) | A publisher has no stable subject | Design read of each registering publisher | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A plugin starts receiving or sending on peers the operator kept it away from, because the per-peer block is gone | A converted scenario's peer receives routes it did not before | This is the ruled behavior change (A-Q7, R7), not a defect: the converter writes no narrowing flow. The two censuses name every config and test it reaches, and a route-server config that kept the server from some peers converts to export policy |
| R-2 | The converter corrupts `$PORT` placeholders, re-encodes test secrets, or mangles `refused=` blocks | Diff review or `TestRepositoryConfigsValidate` failures | Sentinels around parse and serialize, no `EncodeSecrets` in the converter, each `refused=` block triaged by hand |
| R-3 | `hub` validation is skipped or the `bus-hub` listener edge is dropped | Managed `.ci` or `TestConfigGraph` fails | Update the roots, the validate guard, `listenerSectionName` and the diagnostic strings together |
| R-4 | A foreign program named like a compiled-in plugin is taken for it | A unit test of a foreign `ddos-flowspec` reading the compiled plugin's state key | Identity comes from the `internal` leaf alone; a foreign program is keyed under an identity no registry name can take |
| R-5 | A forked compiled-in plugin declares something other than its registry row | The Stage 1 check table reports a difference | Stage 1 compares the declaration of an `internal ... process separate` endpoint with the registry row and refuses a difference |
| R-6 | The scale of the conversion collides with concurrent sessions in the shared checkout | Dirty files or compile conflicts | Land the engine, YANG and converter first, then convert in one focused pass, committing compiling chunks |
| R-7 | An offline `ze config validate` accepts a flow the daemon refuses | `test/plugin/config-validate-agrees-with-boot.ci` | Offline validation takes the program's declaration in query mode and fails when it cannot, so it never accepts what it did not check |
| R-8 | `ze config validate` now executes the programs a config names | A validate run on an untrusted file starts a program | The bus configuration is the trust boundary (R6), and query mode starts nothing: a declaration is a pure value. The message of a validate run names each program it asked |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Configs fail to parse, plugins receive nothing or everything, a plugin sends toward a peer it must not, managed hub listeners fail to bind, or aliased plugins fail state access |
| How is it reverted? | Git revert of the cutover commits; pre-release `main` has no deployed user |
| Who else touches this path? | Child 2, Child 3, and any concurrent spec that adds `.ci` tests with `plugin {` or `attach process` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` and `ze start` of a `bus { endpoint; }` config with pure `bgp { }` and no flow | -> | Registered receptions -> `DeliveryGraph` -> BGP event delivery and the plugin-wide send check | `test/plugin/bus-registered-default-delivery.ci` (new) |
| The same config with a flow that narrows events, a `match`, and a `send` allow-list on the endpoint | -> | `bus` validation -> edge resolution -> delivery and send check | `test/plugin/bus-flow-narrows.ci` (new) |
| `ze --plugin "<program>" -` | -> | `MergeCliPlugins` -> first pass -> held Stage 1 declaration | `test/plugin/external-yang-command-line.ci` (existing, converted, not weakened) |
| `bus { endpoint my-ddos { internal ddos-flowspec; process separate; } }`, then `show doctor` | -> | Process identity -> `dispatch_state.go` and `doctor_snapshot.go` | `test/plugin/bus-aliased-forked-state-doctor.ci` (new) |
| An ExaBGP script that sends `create neighbor`, then announces to it | -> | Bridge -> `create bgp peer` -> republished subjects -> delivery to the bridge | `test/exabgp/bridge-dynamic-peer-covered.ci` (new) |
| `ze config validate <file>` with no daemon running, on a config with an external endpoint and a flow that names it | -> | `config.SchemaWithDeclaredPlugins` -> `plugin.QueryDeclaration` (the program started with `ZE_PLUGIN_MODE=declare`) -> `rpc.DecodeDeclaration` -> module join -> `bus` validation | `test/parse/bus-offline-validate-query-mode.ci` (new) |
| `ze start` of `bus { endpoint rs { internal bgp-rs; } }` with three connected peers, no per-peer line and no send list | -> | Registered reception of `bgp-rs` -> `selectForwardTargets` -> plugin-wide send check -> the peer's export policy -> forward to each other connected peer | `test/plugin/bus-rs-serves-every-peer.ci` (new) |
| `ze start` of `bus { endpoint rr { internal bgp-rr; } }` with one client and one non-client, no per-peer line | -> | Registered reception of `bgp-rr` -> reflection by the peer's client setting (RFC 4456 Section 6) -> plugin-wide send check | `test/plugin/bus-rr-client-rule.ci` (new) |
| `ze start` of a config with a flow `event [ state ]` for one pair and a flow with no `event` entry for another | -> | Edge resolution (a flow replaces the registered reception of its pair) -> `DeliveryGraph` -> delivery | `test/plugin/bus-flow-replaces-reception.ci` (new) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `bus { endpoint collector { external { run ...; } } }`, pure `bgp { }`, no flow. The program registers the reception of `update-received` and `state` from `bgp` and declares no send | The config validates and boots. `collector` receives both event types for every peer, including a peer added by a later reload. A send of `update` is refused because it was not declared | An event the program registered no reception for is never delivered |
| AC-2 | The same endpoint with a flow that lists `event [ state ]` and `match group { value [ customers ]; }` | Only `state` of peers in group `customers` is delivered. A second flow for the same pair with another `match` delivers its own events to its own peers, and a message matching both is delivered once | No `match` means every message |
| AC-3 | A flow with an unknown `from` or `to`, a `namespace` that `from` does not own, an `event` the publisher did not register or the endpoint does not receive, a `match` tag the publisher did not register, a `match group` value that names no group, or a `send` entry the endpoint's plugin did not declare | Each is refused at `ze config validate`, editor commit and `ze start`, naming the flow or endpoint, the offending token and the accepted values. An external endpoint in `from` is accepted from its declaration | A `match peer` value is checked for form only |
| AC-4 | A plugin that declares the sends `update` and `refresh` toward `bgp`, under three configs: no `send` container on its endpoint, `send { allow [ bgp:update ]; }`, and an empty `send { }` | With none, both sends succeed toward every peer. With `allow [ bgp:update ]`, `update` succeeds and `refresh` is refused. With the empty container, both are refused. Each refusal names the endpoint, the token and the allow-list line that admits it, and counts once | The config text and `show config` keep the empty container distinct from an absent one |
| AC-5 | A reception registered for a publisher that is absent: a build without BGP, or an endpoint not configured | The config loads, nothing is delivered, and `show doctor` and the reconcile name the reception as dormant. Configuring the publisher later, by reload, starts the delivery | An operator-written flow naming an unknown publisher is still refused (AC-3) |
| AC-6 | Config moves `hub { server ...; client ...; }` under `bus { hub { ... } }` | The managed hub listener binds and validates, `show listeners` reports `bus-hub`, `config graph` links `bus` to `bus-hub`, and the managed doctor checks pass | None |
| AC-7 | Config uses the retired `plugin { }`, `attach process`, or `process` under `bgp` | Refused at parse with `RetiredKeywordHint` pointing to `bus { endpoint; flow; hub; }` | None |
| AC-8 | `ze --plugin ze.bgp-bmp -` and `ze --plugin "<program>" -` | Both start the plugin they name. The program's module is joined before the full parse and its declarations are honored as a config endpoint's are | Offline `ze bgp decode --plugin` and `ze plugin test --plugin` keep accepting external decoders |
| AC-9 | An ExaBGP config with `process`, `api { processes [...] }`, and a script that creates a neighbor at run time, through `ze exabgp migrate` and the bridge | The migration emits a complete config with its `bus` section (the bridge endpoint, no flow, no send list) and pure `bgp { }` that loads with no edit. Each script may send the whole ExaBGP API command set, because the bridge declares it and no list narrows it. A neighbor created at run time is fed to its script and takes its announcements with no `attach` | `exabgp { bridge { process { encoder; feed; } } }` stays, and is where ExaBGP's per-neighbor `api { processes }` scoping lives: a script with `feed` blocks is fed by those peers alone, and a command that names no neighbor reaches those peers alone |
| AC-10 | `bus { endpoint my-ddos { internal ddos-flowspec; } }`, the same with `process separate;`, and `bus { endpoint ddos-flowspec { external { run ...; } } }` | Both compiled endpoints read and write the plugin's state key, report healthy in `show doctor`, hold the same default flows, and block `ze data rm` while running. The foreign program named `ddos-flowspec` holds none of the compiled plugin's grants, and `show doctor` reports the compiled plugin as not running. The five sibling sites (`RequiresPeerUpBarrier`, `configRefusalIsFatal`, `isPluginLoaded`, `collectOrphanedDependencies`, `appendHeldNeedProviders`) resolve by registration name | `process separate` on a plugin whose registration sets `RunsInProcessOnly` is refused at commit with that reason |
| AC-11 | A config with a redistribution rule importing from BGP, a FlowSpec family, an iBGP peer, and a filter member owned by a plugin, first with no flow, then with a flow that narrows the Loc-RIB's or the filter owner's events below what the feature needs | With no flow, redistribution, FlowSpec authorization, AIGP selection and the filter owner's cleanup on peer removal all work. With the narrow flow, FlowSpec and AIGP are refused with the `bus { flow }` line that completes the list, and the filter owner still receives `state`, marked implicit | Redistribution with a narrow flow loads and moves nothing from that peer, with its log line, until Child 2 adds the refusal |
| AC-12 | Every tracked `.conf`, `.ci`, `.et`, interop scenario, fixture, doc and website example | None keeps `plugin {` or `attach process` outside `plan/` history. `TestRepositoryConfigsValidate` and the scoped functional suites pass. The converted interop scenarios pass against their peers | History under `plan/journal/`, `plan/learned/`, `plan/handover/` and `plan/records/` may quote the old syntax |
| AC-13 | `Receivers` and the per-message tag path under a benchmark | Zero allocations per operation on both | None |
| AC-14 | `ze config validate` with no daemon running, on a config with an external endpoint and a flow that names it: with a program that answers in query mode, with a flow naming an event that program does not publish, and with a `run` line that names no program | The first validates. The second is refused with the events the program declared. The third fails naming the endpoint, the `run` line, the cause, and `ze show plugin conformance run "<run line>"`. The program's run loop never starts in any of them | A running daemon is still asked first and answers from the declarations it started with |
| AC-15 | A route server with three connected peers and no send list, and a reflector with a client and a non-client | The route server forwards a peer's routes to every other connected peer, with no per-peer line written. The reflector applies RFC 4456 Section 6 by the peer's client setting. An export policy on one peer is what keeps a route from it | None: no peer is skipped for lack of a grant |
| AC-16 | A flow for a pair that lists `event [ state ]`, and a flow for a pair with no `event` entry, for a plugin that registered `update-received` and `state` | The first delivers `state` only. The second delivers nothing from that publisher | A pair with no flow keeps the registered reception |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Declares an external collector as an endpoint and writes nothing else | Stage 1 declaration -> registered receptions -> delivery for every peer | `test/plugin/bus-registered-default-delivery.ci` |
| 2 | Narrows that collector to one group and forbids it to send | `bus` validation -> edge resolution -> delivery and send check | `test/plugin/bus-flow-narrows.ci` |
| 3 | Runs a compiled-in plugin as a separate process under a name of their own and checks `show doctor` | Endpoint `process separate` -> fork of Ze's own binary -> identity by registration | `test/plugin/bus-aliased-forked-state-doctor.ci` |
| 4 | Migrates an ExaBGP config with processes and runs it | `ze exabgp migrate` -> `ze start` -> bridge | `test/exabgp/` suite and `test/exabgp/bridge-dynamic-peer-covered.ci` |
| 5 | Configures the managed hub under `bus { hub }` | First pass -> `ExtractHubConfig` -> `show listeners` | `test/managed/` suite |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBusFlowValidationRefusals` | `internal/component/plugin/bus_validate_test.go` | AC-3: one case per refusal, each asserting the accepted values in the message | |
| `TestBusEdgeResolution` | `internal/component/plugin/server/bus_edges_test.go` | AC-1, AC-2, AC-4, AC-5: default, narrowed, two flows with `match`, absent and empty allow-list, dormant | |
| `TestDeliveryGraphProjectsSubjects` | `internal/component/plugin/server/delivery_graph_test.go` | AC-2: a subject added later is covered; per-message tag matching | |
| `BenchmarkDeliveryReceivers`, `TestDeliveryReceiversAllocatesNothing` | `internal/component/plugin/server/delivery_graph_test.go` | AC-13 | |
| `TestPublicationsCompleteBeforeReceptions` | `internal/component/plugin/server/startup_bus_test.go` | R2: a reception resolved at startup sees a held external program's publication; the catalog reaches Stage 2 | |
| `TestRegistrationCarriesNoSecondDeclaration` | `internal/component/plugin/all/all_test.go` | No registration in the tree keeps a subscription call beside its `Receives` declaration | |
| `TestHubUnderBusListenerAndGraph` | `internal/component/config/listener_test.go` | AC-6: service `bus-hub` and the `config graph` edge | |
| `TestRetiredPluginAndAttachHints` | `internal/component/config/retired_test.go` | AC-7 | |
| `TestFirstPassMergesCommandLinePlugins` | `internal/component/config/first_pass_test.go` | AC-8: existing test, converted to endpoints, proving at least what it proves today | |
| `TestFirstPassRootsAreDeclared` | `internal/component/config/yang/join_test.go` | The first-pass roots are derived, and a module that declares none adds none | |
| `TestProcessIdentityByRegistration` | `internal/component/plugin/server/dispatch_state_test.go` | AC-10: aliased, aliased and forked, and foreign same-named | |
| `TestAliasedPluginLifecycleHelpers` | `internal/component/plugin/server/startup_alias_test.go` | AC-10: the five sibling sites | |
| `TestExaBGPMigrateEmitsBusSection` | `internal/exabgp/migration/migrate_bus_test.go` | AC-9: the emitted config loads through `LoadConfig` unchanged | |
| `TestBridgeDeclaresWholeCommandSet` | `internal/plugins/exabgp/bridgeplugin/declare_test.go` | AC-9: every command the translator can emit is in the bridge's `Sends`, derived from the translator's own table | |
| `TestNarrowFeatureRefusalsNameBusFlow` | `internal/component/bgp/config/redistribute_binding_test.go` | AC-11 | |
| `TestRepositoryConfigsValidate` | `internal/component/config/cli/validate_corpus_test.go` | AC-12 | |
| `TestOfflineValidateAsksProgramInQueryMode` | `internal/component/config/declared_schemas_test.go` | AC-14: with no daemon, `SchemaWithDeclaredPlugins` takes a fixture program's declaration in query mode and the config validates; a flow naming an event the program does not publish is refused with the events it declared; a `run` line that names no program fails naming the endpoint, the `run` line, the cause and `ze show plugin conformance run "<run line>"`. Negative in every case: the fixture's activation function never runs | |
| `TestOfflineQueryFailureNamesItsCause` | `internal/component/config/declared_schemas_test.go` | AC-14 and "What validate prints when the program cannot be asked": one case per cause (program not found, non-zero exit, no declaration line, declaration refused at Stage 1 with that refusal), each failing the validation and saying which sections and flows were not checked; none accepts the config | |
| `TestRouteServerTargetsEveryConnectedPeer` | `internal/component/bgp/plugins/rs/server_forward_test.go` | AC-15: `selectForwardTargets` names every connected peer except the source, with no per-peer grant written. Negative: the source and a peer that is not connected are not targets | |
| `TestForwardRailsServeEveryConnectedPeer` | `internal/component/bgp/reactor/forward_rs_permission_test.go` | AC-15: the route-server flush rail and the peer-up replay reach every connected peer under the plugin-wide check, where today a peer with no `send [ update ]` grant is dropped. Negative: a plugin whose endpoint's send list omits `bgp:update` reaches no peer | |
| `TestReflectorAppliesClientRuleWithNoGrant` | `internal/component/bgp/plugins/rr/rr_test.go` | AC-15: RFC 4456 Section 6 with no grant written: a route from a non-client reaches the clients, and a route from a client reaches the clients and the non-clients. Negative: a route from a non-client does not reach another non-client | |
| `TestFlowReplacesRegisteredReception` | `internal/component/plugin/server/bus_edges_test.go` | AC-16: for a plugin that registered `update-received` and `state` from `bgp`, a flow listing `event [ state ]` yields a `state` edge and no `update-received` edge; a flow with no `event` entry yields no edge from that publisher; a pair with no flow keeps both registered events | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `bus > endpoint > external > timeout` | Existing range kept from `plugin > external > timeout` | Unchanged | Unchanged | Unchanged |
| `bus > hub > server/client > port` | Existing `1..65535` kept from `plugin > hub` | 65535 | 0 | 65536 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bus-registered-default-delivery` | `test/plugin/` | AC-1, AC-5 | |
| `bus-flow-narrows` | `test/plugin/` | AC-2, AC-4 | |
| `bus-flow-validation-refusals` | `test/parse/` | AC-3, AC-7 | |
| `bus-hub-listener` | `test/managed/` | AC-6 | |
| `external-yang-command-line` (existing) | `test/plugin/` | AC-8 | |
| `bus-aliased-forked-state-doctor` | `test/plugin/` | AC-10, through `ze start` and `show doctor` | |
| `bridge-dynamic-peer-covered` | `test/exabgp/` | AC-9, the dynamic `create bgp peer` path through the bridge | |
| `bus-feature-requirements-survive` | `test/plugin/` | AC-11 | |
| `bus-offline-validate-query-mode` | `test/parse/` | AC-14 through `ze config validate` with no daemon: a fixture program built on `sdk.RunOrDeclare`, whose activation writes a marker file. The valid config passes; the flow naming an event the program does not publish is refused with the events it declared; the `run` line naming no program fails with the endpoint, the `run` line, the cause and the conformance command. Negative: the marker file is never written | |
| `bus-rs-serves-every-peer` | `test/plugin/` | AC-15, route server: three connected peers, no per-peer line, no send list; a route from one peer reaches the other two. Negative: with an export policy on one peer that rejects the route, that peer receives nothing and the other still does | |
| `bus-rr-client-rule` | `test/plugin/` | AC-15, reflector: one client and one non-client, no per-peer line; the non-client's route reaches the client and the client's route reaches the non-client, by the client setting alone | |
| `bus-flow-replaces-reception` | `test/plugin/` | AC-16: a plugin that registered `update-received` and `state` under a flow `event [ state ]` receives `state` and no `update-received`; under a flow with no `event` entry it receives nothing from `bgp`; with no flow it receives both. The two negatives are the events that never arrive | |

### Tests routed through the test judge

Each of these proves something the rework changes or removes. The author does not judge them (`ai/META-DECISIONS.md`, "Test changes").

| Test | What it proves today | What the rework does to it |
|------|----------------------|----------------------------|
| `test/plugin/attach-process-send-permission.ci` | A process is refused a send toward a peer that does not grant it | The per-peer refusal has no successor; the plugin-wide refusal replaces it |
| `test/plugin/bgp-rs-fastpath-relay-grants.ci` | The route server relays only toward peers that grant it `send [ update ]` | The route server relays toward every connected peer (RFC 7947 Section 1); what the test proves about a refused destination has no successor, and what it proves about relay correctness is kept |
| Every test the receive census finds | A program is not fed a peer that does not attach it | The program is fed every peer; the test's negative has no successor unless it is restated with a flow |
| `internal/component/bgp/reactor/send_permission_test.go`, `send_permission_rails_test.go`, `forward_rs_permission_test.go` | The ten send rails share one per-peer filter | The rails share one plugin-wide check |
| `test/plugin/external-yang-command-line.ci`, `TestFirstPassMergesCommandLinePlugins` | A program named with `--plugin` gives the same outcome as one in the config | Converted only; listed so the conversion is judged as not weakening |

### Per-peer send census

The 34 files that name the send refusal, and the 520 `.ci` files and 115 interop files that carry a `send [` list, are read before the converter is written. For each file the census records one of three answers: the same send list on every peer of the plugin (converts to nothing, or to one allow-list), different lists per peer with no test of the difference (converts to the union, and the row says what widened), or a test of the difference (goes to the test judge). A config that mixes route-server sessions with other sessions and used the grant to separate them is recorded with the export policy that replaces it. The census table is added to this spec before it is presented.

### Receive census

The converter writes no narrowing flow, so every program that was attached to some peers only sees every peer after conversion (A-Q7). Before the converter is written, every tracked config where a program is attached to fewer than all peers, or with different `receive` lists per peer, is read. For each the census records: no test depends on the difference (converts with no flow), or a test asserts that the program did not receive a peer's event (goes to the test judge, with the flow that would restate it). ExaBGP configs are a separate column: their per-neighbor `api { processes }` scoping is carried by the bridge's `feed` blocks and does not change. The census table is added to this spec before it is presented.

### Interop Tests (Scope: protocol)
No wire format changes. Which plugin sees and sends what does change, so the converted scenarios under `test/interop/` run against their peers and that run is a goal gate. The run names each scenario whose config the converter changed beyond syntax.

## Files to Modify
- `internal/component/plugin/yang/ze-plugin-conf.yang` - replace `container plugin` with `container bus` (`endpoint`, `flow`, `hub`)
- `internal/component/bgp/yang/ze-bgp-conf.yang` - remove `container attach`
- `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang`, `internal/component/bgp/plugins/cmd/peer/create.go` - remove the `attach` keyword and `setAttach`
- `internal/component/plugin/registry/registry.go` - the three declarations replace `EventTypes` and `SendTypes`
- `pkg/plugin/rpc/types.go`, `internal/core/ipc/yang/ze-plugin-engine.yang` - Stage 1 fields and the Stage 2 catalog
- `pkg/plugin/sdk/sdk_callbacks.go` - subscribe from the declaration; delete the three subscription calls
- `internal/core/events/events.go` - tag keys and publisher per namespace; command tokens per accepting component; delete `ValidSendTypes` and `RegisterSendType`
- `internal/component/plugin/resolve.go` - delete `RegisterPluginEventTypes`, `RegisterPluginSendTypes`, both `sync.Once`, the default event namespace and the `ze plugin X` branch of `deriveName`
- `internal/component/plugin/types.go` - identity and transport as two questions in place of `RunsExternalProgram`; its ten callers
- `internal/component/plugin/process/process.go` - fork of Ze's own binary for `process separate`
- `internal/component/config/first_pass.go`, `yang_schema.go`, `yang/join.go`, `yang/publish.go`, `loader.go`, `loader_extract.go` - derived first-pass roots, `bus` in place of `plugin`, `--plugin` merge into endpoints
- `internal/component/config/validate_sections.go`, `validate_semantic.go`, `validators.go`, `retired.go`, `listener.go`, `graph.go`, `cli/cmd_validate.go` - `bus` section, `bus-hub`, retired hints
- `internal/component/bgp/config/plugins.go`, `peers.go`, `resolve.go`, `redistribute_binding.go`, `flowspec_binding.go`, `filter_state_binding.go`, `dissemination.go`, `dissemination_config.go`, `loader_create.go`, `register.go` - delete inline plugin extraction, `attach` parsing and the derived bindings; keep the refusals in bus syntax; register BGP's tag keys, value validator and command tokens
- `internal/component/bgp/reactor/config.go`, `delivery_graph.go`, `send_permission.go`, `reactor_api.go`, `peer_settings.go`, `peer_settings_apply.go`, `peer_derived_bindings.go`, `source_policy_reload.go` - delete `ProcessBinding`, `EnsureProcessBinding`, `GrantImplicitReceive`, `GetPeerProcessBindings`; publish subjects with tags; plugin-wide send check
- `internal/component/bgp/server/events.go` - alias fix for `RequiresPeerUpBarrier`
- `internal/component/plugin/server/delivery_graph.go`, `delivery_reconcile.go`, `dissemination_validate.go`, `stage1_checks.go`, `startup_autoload.go`, `startup.go` - graph built from subjects, receptions and flows; Stage 1 checks for the declarations; alias fixes; refusal texts in bus syntax
- `internal/component/plugin/server/dispatch_state.go`, `doctor_snapshot.go` - identity by registration name or foreign identity
- `internal/plugins/ddos/flowspec/doctor.go`, `internal/component/config/storage/cli/data_rpc.go` - match the registration name and report the configured alias in fixes
- `internal/component/plugin/doctor/check_external.go`, `check_managed_listener.go`, `internal/component/managed/client.go`, `cmd_fleet.go`, `doctor.go`, `cmd/ze/hub/main.go`, `managed_owner.go`, `cmd/ze/ze_core_start.go`, `cmd/ze/ze_core_dispatch.go` - read `bus`; `bus hub` diagnostics
- Every package that calls one of the three subscription calls or sets `EventTypes` or `SendTypes` - declare instead
- `internal/exabgp/migration/migrate.go`, `migrate_serialize.go`, `internal/exabgp/bridge/bridge_neighbor.go`, `internal/plugins/exabgp/bridgerun/script.go` - the `bus` section, the bridge's declarations, no `attach`
- `docs/architecture/api/architecture.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/config/syntax.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/core-design.md`, `docs/architecture/fleet-config.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/system-architecture.md`, `docs/architecture/rib-transition.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/api/capability-contract.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/exabgp-bridge.md`, `docs/architecture/update-building.md`, `docs/guide/ddos-mitigation.md`, `docs/guide/flowspec-protected-router.md`, `docs/guide/plugins.md`, `docs/guide/configuration.md`, `docs/config-reference.md`, `docs/exabgp/exabgp-migration.md`, `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `website/` - departed pages, doors and examples, and the decision rows of R1 to R7
- Tracked `.conf`, `.ci`, `.et`, interop scenarios, fixtures and unit tests (about 1,752 files) - converted by the parsed-tree converter

## Files to Create
- `internal/component/plugin/bus_validate_test.go`, `internal/component/plugin/server/bus_edges_test.go`, `startup_bus_test.go`, `startup_alias_test.go` - unit tests
- `internal/exabgp/migration/migrate_bus_test.go`, `internal/component/bgp/plugins/rs/server_forward_test.go` - unit tests
- `test/plugin/bus-registered-default-delivery.ci`, `test/plugin/bus-flow-narrows.ci`, `test/plugin/bus-aliased-forked-state-doctor.ci`, `test/plugin/bus-feature-requirements-survive.ci`, `test/parse/bus-flow-validation-refusals.ci`, `test/managed/bus-hub-listener.ci`, `test/exabgp/bridge-dynamic-peer-covered.ci`, `test/parse/bus-offline-validate-query-mode.ci`, `test/plugin/bus-rs-serves-every-peer.ci`, `test/plugin/bus-rr-client-rule.ci`, `test/plugin/bus-flow-replaces-reception.ci` - functional tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-plugin-conf.yang` (`bus`), `ze-bgp-conf.yang` (remove `attach`), `ze-peer-cmd.yang` (remove `attach`), `ze-plugin-engine.yang` (Stage 1 and Stage 2 fields); regenerate glue with `./le yang glue update` |
| YANG validation constraints | Yes | `enumeration` on `encoding` and `process`, mutual exclusion of `internal` and `external`, `process` valid with `internal` only, presence on the endpoint's `send`, `range` on `timeout` and hub ports |
| YANG custom validators | Yes | `ze:validate` for the endpoint names, for the `send` entries, and for `from`, `namespace`, `to`, `event` and `match`, each reading a registry filled at `init()` or the held external declarations |
| CLI commands/flags | Yes | `cmd/peer/create.go` (drop `attach`); `--plugin` unchanged in what it accepts |
| CLI grammar (keyword before value) | Yes | `bus { endpoint <name> ...; flow <name> ...; }` and `create bgp peer` obey R1 to R9 (`./le cli grammar`) |
| Editor autocomplete | Yes | Automatic for enums and containers; completion for `internal`, `from`, `namespace`, `to`, `event`, `match` tags and values, and `send` entries from the plugin's declaration |
| Functional test for new RPC/API | Yes | The functional tests above |
| Pipe completeness | N-A | No new operational command here (`\| implied` is Child 2) |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | Yes | `check_external.go`, `check_managed_listener.go`, `ddos/flowspec/doctor.go`, `managed/doctor.go` read `bus` and the registration name; a dormant reception is reported |
| Doctor code per problem class | Yes | Existing codes kept (`doctor-plugin-delivery-disagreement`, managed listener, ddos-flowspec); one new code for a dormant reception; fix texts in bus syntax |
| Prometheus counters/metrics | N-A | The send refusal counter keeps its name and labels here; Child 3 generalizes it |
| BGP family surface (new SAFI / capability / attribute) | N-A | None added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/` pages that show `attach process` or `plugin {` (regenerate `docs/features.md`) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md`, `docs/config-reference.md` |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (`create bgp peer`) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/process-protocol.md` and `docs/plugin-development/protocol.md`: Stage 1 `publishes`, `receives`, `sends`; the Stage 2 catalog; the ready RPC without event subscriptions |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, `docs/plugin-development/` |
| 6 | Has a user guide page? | Yes | `docs/guide/plugins.md`, `docs/guide/route-reflection.md`, `docs/guide/fleet-config.md` |
| 7 | Wire format changed? | No | BGP and IGP wire formats unchanged |
| 8 | Plugin SDK/protocol changed? | Yes | `ai/rules/plugins.md`, `docs/plugin-development/handlers.md`: a plugin declares instead of calling a subscription setter |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Config and bus registration only |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, `docs/architecture/testing/ci-format.md` examples |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` holds no dependency on `attach process` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/architecture.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/core-design.md`, `docs/architecture/exabgp-bridge.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None here |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`: each plugin's publications, receptions and sends replace its event and send type lists |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived with `./le spec citation anchors` after the file lists above settle |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The 55 `docs/` files, 9 `website/` files and 7 `ai/` files of the Phase 1 census |

## Design presentation

### What changes

Today an operator names a program under `plugin { }` and then, on every BGP peer, writes an `attach process` block that says which events the program is fed and which messages it may send. A program nobody attaches receives nothing and sends nothing.

After this work a plugin states for itself what it publishes, what it wants to receive and what it sends. It states this in its registration, and a compiled-in plugin and a program Ze did not ship use the same registration. The operator writes `bus { endpoint }` to put a party on the bus. With nothing more written, that party receives what it registered and sends what it declared, on every peer, including a peer added later. The operator writes `bus { flow }` only to narrow what a party receives: a list of events and a match on generic tags that the publisher attaches to its messages. To narrow what it sends, the operator writes one send allow-list on its endpoint, `send { allow [ ... ]; }`, which applies to the whole plugin and to every component it sends to. An allow-list that is present and empty means the plugin sends nothing. `bgp { }` holds BGP settings and nothing about the bus.

A compiled-in plugin runs in the daemon unless its endpoint says `process separate`. It then runs as a process of its own and stays the same plugin: same identity, same state, same defaults. `hub` moves under `bus`. The aliased-plugin identity defects are fixed. Every config, test and page in the repository is converted in the same work.

### Before and after

1. **An external collector that watches BGP.**

Before:
```
plugin {
    external collector { run ./collector.py; encoder json; }
}
bgp {
    group transit {
        attach process collector { receive [ update-received state ]; }
        peer 198.51.100.1 { remote-as 65001; }
    }
}
```

After, with the collector registering that it receives `update-received` and `state` from `bgp`:
```
bus {
    endpoint collector { external { run ./collector.py; } encoding json; }
}
bgp {
    group transit {
        peer 198.51.100.1 { remote-as 65001; }
    }
}
```
The collector now receives those events for every peer, not only for group `transit`. To keep it to that group, the operator writes:
```
bus {
    endpoint collector { external { run ./collector.py; } encoding json; }
    flow watch {
        from bgp;
        to collector;
        match group { value [ transit ]; }
        event [ update-received state ];
    }
}
```

2. **A plugin that may announce routes, and one that must not.**

Before, a program announces only toward the peers that write `send [ update ]` for it:
```
peer 198.51.100.1 { attach process injector { send [ update ]; } }
peer 198.51.100.2 { }
```
The program reaches 198.51.100.1 and is refused toward 198.51.100.2.

After, a program that declares the send `update` reaches every peer with no line written. To forbid it, the operator writes an empty allow-list on the endpoint:
```
bus {
    endpoint injector {
        external { run ./injector.py; }
        send { }
    }
}
```
To keep `update` and remove `refresh`, the operator writes `send { allow [ bgp:update ]; }`. The one list covers every component the plugin sends to: `send { allow [ bgp:update "cli:show bgp peer" ]; }`. No form says "this peer only": the allow-list is per plugin. What an operator sees when a send is refused:
```
endpoint injector may not send bgp:update: its send allow-list does not list it
fix: set bus endpoint injector send allow bgp:update
```

2b. **A route server.** Before, each client wrote `attach process rs { receive [ update-received state ]; send [ update ]; }`, and a peer without it was neither heard nor served. After, `bus { endpoint rs { internal bgp-rs; } }` and nothing on the peers: the route server hears and serves every connected peer, as RFC 7947 Section 1 describes, and a client that must not get a route says so in its export policy.

2c. **Validating with no daemon.** Before: `ze config validate site.conf` refuses the section of an external plugin and names a running Ze as the fix. After: it runs the program in query mode and validates. When the program cannot be asked:
```
config site.conf: endpoint acme: its program wrote no declaration (run: /opt/acme --serve)
The acme section and the flows that name acme were not checked.
fix: ze show plugin conformance run "/opt/acme --serve"
```

3. **A compiled-in plugin as a separate process.**

Before: `plugin { external rib { run "ze plugin bgp-rib"; } }` forked the plugin, and Ze treated it as a program of its own that borrowed nothing from the compiled `bgp-rib`.

After: `bus { endpoint rib { internal bgp-rib; process separate; } }` forks Ze's own binary for `bgp-rib`. It holds the identity, state grants and defaults of `bgp-rib`. `bus { endpoint bgp-rib { external { run ./mine; } } }` is a different program that happens to share the name, and it holds none of them.

4. **`hub` under `bus`.** `plugin { hub { server { ... } } }` becomes `bus { hub { server { ... } } }`, and `show listeners` reports `bus-hub` where it reported `plugin-hub`.

5. **An aliased built-in plugin.** Before, `plugin { internal my-ddos { use ddos-flowspec; } }` failed every state read and write, and `show doctor` reported `ddos-flowspec` as not running. After, `bus { endpoint my-ddos { internal ddos-flowspec; } }` reads and writes its state, and doctor reports it running under its alias.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep `flow [ ... ]` inside `bgp { group; peer; }` (2026-11-17 D-5) | Puts bus wiring in `bgp { }`; reversed by Thomas on 2026-11-23 |
| BGP `group` and `peer` leaves on `bus { flow }` | Puts BGP concepts in the bus; replaced by generic tags the publisher registers |
| Silence by default, every flow written by the operator (the first design) | Reversed by R1 and R7: a plugin receives what it registered and sends what it declared |
| Built-in publishers only in `from` (the first design) | Reversed by R4 and R6: no rule separates the kinds on trust. The one transport difference, an external declaration unknown offline, is stated and handled by the existing offline route |
| Built-in plugins only on `ze start --plugin` (the first design) | Contradicts R4, and removes what `test/plugin/external-yang-command-line.ci` and `TestFirstPassMergesCommandLinePlugins` prove and `yang-config-design.md` records |
| Refuse state to a compiled-in plugin that runs as a separate process (the first design) | Reversed by R3 |
| Keep the reception in the ready RPC and add a declaration beside it | Two declarations of one fact; the offline display could not show a plugin's defaults without running it |
| A deny list for sends | Thomas ruled that narrowing is the presence of an allow-list |
| A send allow-list scoped by tags or peers | Thomas ruled that the allow-list is per plugin |
| A bare leaf-list for the allow-list | Cannot state "present and empty", which means "send nothing" |
| A send list per flow, one per target component (the first rework) | Thomas ruled one list per plugin; the list moves to the endpoint and each entry names its component |
| A role's own peer config deciding which peers a route server serves | RFC 7947 Section 1 already says it: every route server client connected to it |
| The converter writing a `match` flow wherever today's config attached fewer than all peers | Thomas ruled that the new default is every peer |
| Refusing offline what needs an external declaration | Thomas ruled that validation starts the program's init and uses its YANG; query mode already does exactly that |
| Match tags per message for every event | Costs a comparison per delivery on the peer path; stable tags are resolved when the index is built |
| An `ExtractBusFromTree` in the config package and `bus` added to the literal first-pass list | Config code would name a specific root; the owner registers through the existing seam and the list is derived |

### Documents departed from

1. `docs/architecture/api/architecture.md` ("Owner ruling: implied and shown, or written"): "Where the operator has a choice, which program watches a peer and what it may send, the operator writes it, and silence stays the default." Before: a program nobody attaches receives and sends nothing. After: a configured endpoint receives what it registered and sends what it declared, and `| implied` shows it. Example 1 and example 2 above. Ruled by R1 and R7.
2. `docs/architecture/api/architecture.md` ("Owner ruling: endpoints and flows on the bus"): "A BGP group or peer takes part in a flow by listing its name, and the list merges through groups as every list does." Replaced by flows that match generic tags. Example 1.
3. `docs/architecture/api/architecture.md` ("What the Plugin Declares and What the Peer Grants"): "A plugin declares what it CAN handle, in its ready RPC." The declaration moves to the registration and becomes the default.
4. `docs/architecture/api/architecture.md` ("Data Flow: Config to Delivery"): "A relay is judged on the grant of the process that sends it, and on nothing else: no process lends its grant to another." The grant becomes plugin-wide, and a route server or reflector serves every peer connected to it (RFC 7947 Section 1, RFC 4456 Section 6). Examples 2 and 2b.
4b. `docs/architecture/plugin/plugin-system.md` ("One plugin, two ways to reach the bus"): "With no daemon running, no command starts it, and a config section only its module defines is not available". Before: offline validation refuses that section and names a running Ze. After: it starts the program in query mode, takes its declaration and validates. Example 2c. Confirmed by Thomas as intended. `ai/doors/config-yang.md` and `ai/doors/plugin-infra.md` restate the old rule and are corrected with the page.
5. `docs/architecture/api/process-protocol.md` ("Persistent runtime state"): "Requests use the daemon's configured process name as identity." A compiled-in plugin's identity is its registration name. Examples 3 and 5.
6. `docs/architecture/api/process-protocol.md` ("Event Delivery"): "the plugin subscribed to the type in that direction, and the peer's `attach process <name>` block grants it." Replaced by the registration and the flow.
7. `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`: the `plugin { }` and `attach process` forms they quote, replaced by `bus`.
8. `docs/architecture/config/syntax.md` ("The first pass: plugin declarations before the full parse"): "`PluginOnlySchema` holds the roots `plugin` and `environment` and no other." The roots are `bus` and `environment`, derived.
9. `docs/architecture/core-design.md`, `docs/architecture/api/commands.md`, `docs/architecture/fleet-config.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/system-architecture.md`, `docs/architecture/rib-transition.md`, `docs/architecture/bgp/structural-forwarding.md`, `docs/architecture/api/capability-contract.md`, `docs/architecture/plugin/plugin-system.md`, `docs/architecture/doctor-and-health-checks.md`, `docs/architecture/exabgp-bridge.md`, `docs/architecture/update-building.md`, `docs/guide/ddos-mitigation.md`, `docs/config-reference.md`, `docs/exabgp/exabgp-migration.md`, `docs/architecture/config/transaction-protocol.md`, `docs/guide/flowspec-protected-router.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/plugin/rib-storage-design.md`, `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/firewall/firewall-domain-group.md`: each quotes `plugin { }` or `attach process`, as the Design documents table lists row by row, and each is updated to the registration and `bus` forms.
10. `ai/META-DECISIONS.md` ("CLI, config, and daemon surfaces"): "Where the configuration leaves the operator a choice, the operator writes it." R1 and R7 make the endpoint the operator's choice and everything the plugin registered a consequence of it. Thomas approved the sentence that now follows it there: "What a plugin registers is not such a choice: a configured plugin receives and sends what it registered, shown as implied, and the operator writes an allow-list only to narrow it."

### Choices for Thomas

Decided on 2026-11-23 and written into this design: C-1 the three-child split, C-2 the pure bus with generic tags, C-3 the optional `namespace` leaf, C-5 `hub` under `bus`, and R1 to R7 as read above. C-4 of the first design (built-in only on `--plugin` and in `from`) is withdrawn by R4 and R6 and returns as Q-3.

The seven questions the first rework put to Thomas are answered, and each answer is written into this design.

| # | Question | Today | His answer, as read | What it changed here |
|---|----------|-------|---------------------|----------------------|
| Q-1 | Does an operator flow replace the plugin's registered reception or add to it? | No default exists | Replace: only what the flow lists is delivered; an empty list turns it off | Semantics of flows, AC-16 |
| Q-2 | What does offline validation do for a flow or section an external program defines? | Refuses it, after asking a running Ze | Start the program's init, not its loop, and validate with its YANG | Offline validation runs query mode, AC-14 |
| Q-3 | Does `ze start --plugin "<program>"` keep working? | Yes, tested | It keeps working | AC-8. One detail stays to settle in implementation: the name derived from a command line can hold a dot, which the endpoint name rule refuses, so the derived name is made legal as it is today for `--plugin` |
| Q-4 | What decides which peers a route server or reflector serves? | The per-peer send grant | The RFC: every route server client connected to it; differences are export policy | "Which peers a forwarding role serves", AC-15 |
| Q-5 | One send list per plugin, or one per component? | Send types exist for BGP only | One list | The list moves to the endpoint; each entry names its component |
| Q-6 | What does `ze exabgp migrate` write? | `plugin { }` and `attach process` per neighbor | No list, so the full declared set applies | AC-9 |
| Q-7 | Does the default reception cover every peer for existing and migrated configs? | Each program is fed only the peers that attach it | The new default is every peer | The converter writes no narrowing flow; "Receive census" |

Still waiting for Thomas. Each row gives what happens today and what each answer changes.

| # | Item | Today | Options and what each changes | Assumed until answered |
|---|------|-------|-------------------------------|------------------------|
| P-3 | Before Child 3 lands, a `send` list on a plugin that also sends to a component not yet enforced is refused. Is that acceptable, or does Child 3 land together with this spec? | Only BGP send types are enforced | (a) Refuse until Child 3: the list is usable at once for plugins that send to `bgp` alone. (b) Land Child 3 with Child 1: no interim refusal, one larger change | (a) |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add the failing functional tests of the Wiring Test table and the failing unit tests `TestBusEdgeResolution`, `TestProcessIdentityByRegistration` and `TestBusFlowValidationRefusals`. Record the per-peer send census.
2. **Phase: Registration** -- add the three declarations to `registry.Registration`, Stage 1 and the event registry; deliver the catalog at Stage 2; convert every plugin and fixture in the tree; delete `EventTypes`, `SendTypes`, the two lazy registrars, the default event namespace and the three subscription calls.
3. **Phase: Identity** -- split identity from transport; add `process separate`; fix `dispatch_state.go`, `doctor_snapshot.go`, `ddos/flowspec/doctor.go`, `data_rpc.go` and the five sibling sites.
4. **Phase: `bus` schema, loader, first pass and validation** -- delete `container plugin` and `container attach`; add `container bus`; regenerate the YANG glue; derive the first-pass roots; register the `bus` reader; update `validatedSections`, the retired hints, the listener and graph names, and `create bgp peer`.
5. **Phase: Delivery and send** -- build the graph from subjects, receptions and flows; make the send check plugin-wide; delete `ProcessBinding` and its writers; keep the feature refusals in bus syntax; update the ExaBGP migration and bridge.
6. **Phase: Conversion and documentation** -- run the converter over the tracked files; update the pages, doors and decision rows; run `TestRepositoryConfigsValidate`, the scoped functional suites and the converted interop scenarios.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 to AC-16 has an implementation and a passing test |
| No layering | No acceptance of `plugin {` or `attach process` remains; no subscription call, `EventTypes`, `SendTypes` or lazy registrar remains beside the declarations |
| Pure bus isolation | `internal/component/plugin/` and `internal/component/config/` hold no BGP import and no `peer` or `group` word |
| One API for both kinds | Every rule that differs for an external program names its transport or arrival-order reason on `plugin-system.md` |
| Identity | A foreign program named like a compiled-in plugin holds none of its grants; a forked compiled-in plugin holds all of them |
| Validation order | `ValidateCustomSections` on `bus` passes inside `LoadConfig` before any reactor or plugin server exists |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line in bus syntax |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No active `plugin {` or `attach process` outside `plan/` history and `retired.go` | `git grep -E 'plugin \{|attach process' test/ docs/ etc/ examples/` |
| All tracked configs validate | `./le test unit pkg path ./internal/component/config/cli/ run TestRepositoryConfigsValidate` |
| Scoped unit and functional suites green | `./le test unit pkg` on changed packages, and `./le test functional parse`, `plugin`, `reload`, `encode`, `exabgp`, `editor`, `managed` |
| Converted interop scenarios pass | `./le test integration interop` for the scenarios the converter changed |
| YANG glue, tier, grammar, enumeration and doc gates green | `./le yang glue check`, `./le arch tier check`, `./le arch enumeration check`, `./le cli grammar`, `./le plugin declarations check`, `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Identity by name | No site takes a compiled plugin's identity from an endpoint's name or a `run` string; only the `internal` leaf gives it |
| Undeclared send | A send the plugin did not declare is refused on every BGP rail, with no allow-list written |
| Empty allow-list | A present and empty `send` refuses every send, and no code path reads it as absent |
| Offline validation runs programs | A program is started in query mode only, never in its run loop, and the validate output names each program it asked |
| Operator text off the wire | An external declaration's names are bounded and free of control characters before they reach a message or a registry (`ai/rules/plugins.md`) |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| The converter breaks `$PORT` or a `refused=` block | Fix the converter's sentinel handling or triage that test |
| A multi-peer test fails after `attach process` is removed | Decide from the census whether it needs a `match` flow, an allow-list, or the test judge |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- Removing `attach > content` also removes peer session bounces that `reflect.DeepEqual` caused over dead leaves.
- The reception a plugin declares already exists, in the ready RPC. R1 changes what it means, from an upper bound to a default, and moving it to the registration is what lets an offline reader show it.
- The per-peer send grant did two jobs: permission, and choosing which peers a forwarding role owns. R7 removes the first. A-Q4 answers the second from the RFCs: a route server or reflector serves every peer connected to it (RFC 7947 Section 1, RFC 4456 Section 6), so nothing replaces the grant ("Which peers a forwarding role serves").

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `guard-keyed-on-a-field-its-consumer-ignores.md` | 2026-11-14 | `statestore.PluginKeyReadable`/`Writable`/`PluginList` (`plugin/server/dispatch_state.go`), `PluginDoctorState.Name` (`doctor_snapshot.go`), `ddos/flowspec/doctor.go`, and `data_rpc.go` (`runningOwnerError`) key on configured process alias instead of compiled registry name |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Pure `bus { endpoint, flow, hub }`, no flow in `bgp { }`, generic tags (2026-11-23) | Flow lists in `bgp { }`; BGP leaves in `bus { }` | Keeps `bgp { }` pure and the bus protocol-agnostic |
| One registration of publications, receptions and sends for both kinds of plugin (R1, R2, R4, R7) | Operator-written flows only; a declaration for compiled-in plugins alone | Thomas's rulings; one API; an offline reader can show the defaults |
| A flow narrows the registered receptions and carries no send list; the send list is one per plugin, on its endpoint, and presence-bearing (R7, A-Q5) | Deny list; per-peer list; bare leaf-list | Thomas's rulings, and "present and empty" must be stateable |
| Identity and transport are two leaves (R3) | One predicate; a `run` line that names a compiled plugin | A compiled-in plugin keeps its identity on either transport, and a `run` line is never ambiguous |
| `--plugin` keeps starting an external program (R4) | Built-in only | Same API; keeps a tested feature |
| `hub` under `bus` (`bus-hub`) | Top-level `hub { }` | Follows D-7 |
| The graph projects edges onto publisher subjects | Tag matching per message | Keeps the peer path at one index read with no allocation |
| Three child specs | One spec | The cutover lands first |

## Known Limitations
- The `| implied` display, the declared feature requirements with their refusals, the AIGP trigger and the edge origin are owned by `plan/release/fixes/spec-bus-2-implied-display-and-grant-refusals.md`.
- The check on every engine operation other than the BGP send types, and the closure of the `ReservedInternalPrefix` AAA bypass, are owned by `plan/release/fixes/spec-bus-3-external-command-gating.md`. Until it lands, a `send` list on a plugin that declares a send toward a component other than BGP is refused (P-3).
- A send cannot be limited to some peers, and a route server serves every peer connected to it. Export policy is what limits an advertisement per peer.
- A program that was attached to some peers sees every peer after conversion, unless the operator writes a flow with a `match`.
- FlowSpec authorization refuses a Loc-RIB that runs as a separate process, because its lookups are process-local. That is a transport reason and is stated on `plugin-system.md`.

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
- [ ] The per-peer send census table is recorded in this spec

### Goal Gates (MUST pass)
- [ ] AC-1..AC-16 all demonstrated
- [ ] The receive census table is recorded in this spec
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: scoped package tests and `./le test functional parse`, `plugin`, `reload`, `encode`, `exabgp`, `editor`, `managed`
- [ ] The converted interop scenarios pass against their peers
- [ ] Every test in "Tests routed through the test judge" carries the judge's verdict
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
