# Plugins

**When:** creating or changing a plugin: its registration, placement, transport, command surface, process boundary, dispatch table, or a feature gate
**Severity:** blocking
**Related:** repo-maintenance, cli, evidence

## Directives

- **Every plugin MUST follow the directives in this rule.** The mechanism behind them is documented, and the page is the place to read it: `docs/architecture/plugin/plugin-system.md` for registration and the engine boundary, `docs/architecture/plugin/feature-gates.md` for compile-out, `docs/architecture/command-ownership.md` for command placement, `docs/architecture/api/process-protocol.md` for the wire protocol, and `ai/patterns/plugin.md` for the file template and the new-plugin checklist.

- **A plugin MUST own its ENTIRE feature surface. Removing the plugin MUST make every one of its features disappear; every OTHER plugin and the core MUST keep working.** Section "Plugin Self-Containment".
- **A plugin that calls a same-process-effect function in another package directly MUST check `sdk.Plugin.IsInternal()` and then refuse to start, or warn, when it runs external.** Section "Plugin Process Boundary".
- **MUST NOT use switch/case to dispatch subcommands: register each handler into a dispatcher, then call `Dispatch(args)`.** Section "Registration-Based Dispatch".
- **A compile-out-able feature MUST be declared ONCE in `feature-gates.txt`, and an always-on (untagged, non-test) package MUST NOT import it.** Section "Feature-Gate Registration".

## Cross-Boundary Value Types (BLOCKING)

- **A payload that crosses a plugin or component boundary MUST be a self-contained value type.** It carries no pointer field into data another plugin or component owns, and a shared core package is no exception. The surface-by-surface list is `docs/architecture/plugin/plugin-system.md`, "Cross-boundary value types".

- **A shared type definition such as `family.Family` or `RouteChangeBatch` MAY cross a boundary, because it is a compile-time contract rather than data.** The ban is on one plugin holding a pointer to data another plugin allocated.

## Component vs Plugin Placement (BLOCKING)

- **Copying a plugin folder in and running `./le repository generate` MUST make its commands live, and deleting the folder and running it again MUST make them vanish.** No manual wiring. This is the "delete the folder" proximity test applied to the whole user-facing surface. The directory layout, the codegen that discovers it, and the two folder tests are `docs/architecture/command-ownership.md`.

## SDK Is Generic (BLOCKING)

- **The SDK (`pkg/plugin/sdk/`) MUST NOT contain plugin-specific code.** Adding or removing a callback type is one `On*` method in `sdk_callbacks.go` and nothing else: the event loops, the dispatch logic and the transport layers dispatch through `map[string]callbackHandler` without knowing which callbacks exist. What that property requires is `docs/architecture/plugin/plugin-system.md`, "The SDK stays generic".

## Proximity Principle (BLOCKING)

- **Related code MUST belong together in one folder.** The "delete the folder" test is the mechanical check, and what proximity requires of each surface is `docs/architecture/plugin/plugin-system.md`, "Proximity".

- **A runtime dependency check, its registration and its unit test MUST live in the plugin, component, backend or command package that owns the dependency.** `internal/component/doctor` keeps only the runner, the user-entry functional tests, and the checks for a dependency with no narrower owner.

## YANG Is Required (BLOCKING)

- **Every RPC MUST carry a YANG registration for the CLI, whether it is registered through `registry.Register()` or through `pluginserver.RegisterRPCs()`.** A command handler with no YANG schema is a structural defect to fix, not a different category. There is no "command module": everything with RPCs is a plugin and lives under `plugins/<name>/`.

**Anti-pattern:** command handlers MUST NOT be placed in reactor/ (couples engine core to command surface), in a separate handler/ package (middleman), or in a `command/` folder (formalizes missing YANG as acceptable). Commands MUST belong in `plugins/` with YANG schemas.

## Import Rules (BLOCKING)

- **Infrastructure MUST NOT import a plugin implementation directly; it MUST use a registry lookup.**
- **A plugin MUST NOT import a sibling plugin package; it MUST send a text command through `DispatchCommand`.**

- `internal/component/plugin/`, `internal/component/bgp/`, `internal/component/config/`, `cmd/ze/` -> registry
- NLRI decoding: `registry.NLRIDecoder(family)` -> `func(hex) (json, error)`
- NLRI encoding: `registry.NLRIEncoder(family)` -> `func(args) (hex, error)`
- Plugin `register.go` and `internal/component/plugin/all/all.go` blank imports MAY be used
- Schema imports (`<plugin>/yang/`) MAY be used (data, not logic)
- Test imports MAY be used

## Plugin Boundary Naming (BLOCKING)

- **A helper that sends a command through `DispatchCommand` MUST be named for what it does, never for where it sends it.** The engine routes by prefix, so a function, variable or type name MUST NOT encode the destination: `dispatchCommand`, never `dispatchRIBCommand`.

## Command Answer Framing (BLOCKING)

- **A plugin MUST NOT be written to assume a single-line command frame.** A command answer is a head, one line for each record, and a terminator. It is that in both directions, on every connection, and Stage 3 carries no wire shape to ask for another.

- **The frame never follows the payload.** A plugin answers every `execute-command` with a head, its records and a terminator, a built value included. The VALUE is unchanged, byte for byte; the frame around it is not. A test peer written by hand MUST write and read that frame, or the engine takes a head line's tail for its result. The line tags are in `docs/architecture/api/process-protocol.md`, "Command Execution".

- **A command handler MAY answer with a `plugin.Records` rather than a built value, and `Records.Rows` MUST NOT be stored.** It is walked once, before the handler's call returns.
<!-- source: pkg/plugin/records.go -- Records, Records.WriteAnswer -->

## OnStarted vs OnAllPluginsReady (BLOCKING)

- **A `DispatchCommand` that targets another plugin's command at startup MUST be issued from `OnAllPluginsReady`, and MUST NOT be issued from `OnStarted`.** The engine loads plugins across up to five phases in series, so `OnStarted` fires after this plugin's own handshake and before a later phase's plugins load, and the dispatch fails with "unknown command". `OnAllPluginsReady` fires once the dispatcher command registry is frozen.
- **`OnStarted` MUST carry local setup only:** long-lived goroutines, subscriptions, per-plugin state. The phase order is `docs/architecture/plugin/plugin-system.md`, "Startup phases and callbacks".

## Exclusive Role Claims (BLOCKING for cross-plugin default overrides)

- **When plugin A takes over a role plugin B performs by default, A MUST declare it as `Claims` in its `registry.Registration`, and MUST NOT announce it by dispatching a command at runtime.** B has to learn the claim before its first runtime event, and the engine delivers the union of the startup set's claims on every plugin's stage-2 configure callback. B reads `sdk.Plugin.ClaimActive("<role-token>")` from `OnConfigure`. The resolution mechanism is `docs/architecture/plugin/plugin-system.md`, "Exclusive role claims".

- **A role MUST NOT be resolved from `OnAllPluginsReady`.** `sendPostStartupToAll` fans that callback out on detached goroutines immediately before peers start, so waiting there deadlocks, and a role resolved from it races the first session by 1 to 2 milliseconds on an idle host, which inverts under load.

**Fail closed:** an unclaimed or unresolvable role reads `false`, so B keeps doing
the work. That MUST NOT be inverted: standing down for an owner that never runs
is worse than both running. If a claimant never reaches Running, the engine logs it
(`verifyAdvertisedClaims`) but does not fail startup.

- **A plugin that stood a role down MUST run its own default behaviour for an event whose unheld-roles list names that role.** A claim is daemon-wide and delivery is per-peer, so the engine retracts it per event: for that peer, nothing else will do the work. An absent list means every claim holds, which is the common case.

## Peer-Up Barrier (BLOCKING for plugins that register peers)

- **A plugin that decides, ON the peer-up event, whether a peer is eligible to receive traffic MUST declare `PeerUpBarrier: true` in its `registry.Registration`.** The engine then holds that peer's initial-sync End-of-RIB until the plugin has taken delivery of the event, so "End-of-RIB sent" implies "every barrier plugin has registered this peer", the property a peer or a test needs to treat the marker as the go-ahead to send. How the barrier is counted, acknowledged and bounded is `docs/architecture/plugin/plugin-system.md`, "Peer-up barrier".

**The barrier MUST be counted over the delivery set, never the registry.** A
plugin that declares the barrier but is not subscribed to state events never
takes delivery, so counting it would cost every peer the full barrier timeout.

- **The peer-up barrier and the API-sync wait MUST NOT be merged, and a barrier plugin MUST NOT drag in `apiSync`'s 500 ms IPC grace.** `apiSync` counts plugins that SEND routes; a barrier plugin only registers. A route sender's signal satisfying a registrar's obligation is a fail-open (`ai/rules/evidence.md`).

- **Establishment MUST NOT be blocked by the barrier.** A plugin that never acknowledges delays the marker to `peerUpBarrierTimeout` (2 s), which releases it with a WARN naming the peer and the shortfall.

## Registration Metadata Feeds Generated Docs

- **`Name`, `Description`, `ConfigRoots`, `Dependencies`, `OptionalDependencies` and `YANG` MUST be treated as public catalog data.** `./le site build` generates the website plugin catalog from them, so a package move or a config-root change changes what is published. The generation path is `docs/architecture/plugin/plugin-system.md`, "Registration metadata feeds the website catalog".

- **A second hand-written plugin list MUST NOT be created in docs or website content.** Local prose or display metadata goes in a `PLUGIN.md` beside that plugin's `register.go`; a new machine fact goes into the registry or the extractor, and the website renders from that data.

## Optional Dependencies

- **A plugin that USES another plugin when it is present but runs without it MUST declare the relationship as `OptionalDependencies`, never as `Dependencies`.** `Dependencies` is hard: a missing name gives `ErrMissingDependency` and startup fails. The resolver, validation and ordering semantics are `docs/architecture/plugin/plugin-system.md`, "Dependencies".

- **The owner of an optional dependency MUST detect its absence at run time and fall back cleanly.** It MUST treat the engine's `ErrUnknownCommand`, propagated as a string across the plugin IPC boundary, as the plugin-absent signal, and it MUST use `sync.Once` to log one `WARN` per process lifetime before skipping the feature and continuing. `bgp-rs` disabling replay when `bgp-adj-rib-in` is absent is the worked example.

## Family Registration (BLOCKING)

- **An NLRI plugin MUST register every address family it handles with `family.MustRegister(afi, safi, afiStr, safiStr)` at package init time.** Each family gets ONE canonical name and no alias, the plugin owns its SAFI spelling, and a number collision under a different name panics. The four RFC 4760 base families live in `internal/core/family/registry.go`; everything else is owned by its plugin. What registration guarantees, and how a forked plugin declares families over the wire, is `docs/architecture/plugin/plugin-system.md`, "Address family registration".

## Non-CIDR Filter Declaration (BLOCKING for filter plugin authors)

- **A filter plugin that needs per-NLRI decisions on a non-CIDR family MUST declare `raw=true` and MUST parse `FilterUpdateInput.Raw` itself.** For EVPN, Flowspec, VPN, BGP-LS, MVPN, MUP, RTC and every future non-CIDR family the text protocol emits `nlri <family> <op>` as a marker with no prefixes. A CIDR family inlines its prefixes, so `raw=false` is sufficient there. The contract is `docs/architecture/api/process-protocol.md`, "Non-CIDR Families in the Filter Text Protocol".

## Answer Shape Declaration (stage 1 wire protocol)

- **A plugin's declaration MUST NOT be able to panic the daemon.** The shape, column and address-field registries keep their panic on `declare`, which only in-tree Go reaches. A plugin's declaration goes through `declareFor` instead: the same cases with the panic replaced by an error, so a conflicting plugin is refused and the daemon keeps running. `RegisterPluginShapes` is the only caller-facing write, and `UnregisterPluginShapes` takes the whole declaration back when the plugin stops.
<!-- source: internal/component/command/column_order.go -- declare, declareFor, withdraw -->

- **A command MUST answer one shape whatever its argument.** One command path carries one declaration, so a command that answers a row set with no argument and a bare object with an argument declares neither branch truthfully. `show bgp healthcheck <name>` was corrected to answer a one-element row set; the other route declares the shape of one branch and refuses the operators of the other.
<!-- source: internal/component/bgp/plugins/healthcheck/healthcheck.go -- handleShow -->

- **A declared column name MUST be a key the plugin's handler writes, in the same spelling, and a functional test over the rendered answer MUST be what checks it.** The engine never sees the payload: `validateShapeDecls` checks the shape spelling, the presence of a command path and the two bounds, and it cannot check that a name names anything.
<!-- source: internal/component/plugin/server/startup.go -- validateShapeDecls -->

## Runtime Pipe Alias Declaration (stage 1 wire protocol)

- **A pipe alias SELECTS and re-sequences an answer: it renames no key, adds no number and counts no row. A command that wants an alias MUST therefore emit the aggregate fields beside the detail rows, as siblings at one level.** `show bgp rpki` is the worked example: `overviewCommand` writes both halves into one record and `| summary` selects the first half. A view whose data has to be computed stays a subcommand, and so does one that takes a value.
<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- overviewCommand, appendSummaryFields -->

- **A plugin MUST NOT declare an alias name that a built-in pipe operator, an overlapping pipe filter or a same-path alias already carries, and MUST NOT name a command path it did not declare in the same message.** One refusal fails the whole stage 1 registration, and the daemon log is where an operator reads it. Which holder wins, and why the populations differ, is `docs/architecture/api/process-protocol.md`, "Pipe Alias Declaration".

## Modification-Accumulator Buffer Arity (BLOCKING for filter plugin authors)

- **An egress filter records attribute modifications with `filterapi.ModAccumulator.Op(code, action, buf)`. For an attribute whose value is a LIST of fixed-width wire values, `buf` MUST hold a whole number of those values, concatenated.** Several values in ONE operation is allowed and means "every one of them"; splitting them across operations is allowed too. A buffer that is not a whole number of values is what is forbidden.
- **A handler MUST NOT assume one value.** `wireu.StripControlCommunities` returns every matching route-server control community as one buffer. The per-attribute widths, who checks the arity, and what a violation costs are in `docs/architecture/core-design.md`, "Buffer arity, list-valued attributes".

## Renaming a Registered Name (BLOCKING)

- **Before changing any registered name (plugin name, subsystem name, log subsystem, dispatch key, command prefix, family canonical name), EVERY consumer of that name MUST be grepped.** A registered name is not a single string, and most of its consumers are loose strings that no compiler catches. The consumer list and the grep to run are `docs/architecture/plugin/plugin-system.md`, "A registered name lives in many loose strings".

- **Every match of the old name MUST be resolved before the rename is committed.** Each one is a deliberate keep (vendored code, history, a learned summary) or a bug, and a diff that updates one consumer is incomplete by definition.

- **A subsystem or log key MUST use dots (`bgp.gr`), and a plugin name registered with `registry.Register()` MUST use hyphens (`bgp-gr`).** The two are NOT the same string. The hub canonicalizes hyphen to dot for in-process subsystem names, so a new plugin MUST be registered in the hyphen form while every config, log and env consumer uses the dot form, or the canonicalized form, depending on which side of the hub it sits on.

## DirectBridge: Choosing the Right Communication Pattern (BLOCKING)

- **Before designing any new core-to-plugin communication, `pkg/plugin/rpc/bridge.go` MUST be read to check whether DirectBridge already covers the case.** It gives typed direct function calls between the engine and an internal plugin, bypassing JSON serialization and socket I/O. Which pattern fits which problem is `docs/architecture/plugin/plugin-system.md`, "Communication patterns".

- **A new direct-call mechanism MUST NOT be proposed where DirectBridge already provides typed handler slots.** The bridge struct carries a `Set*`/`Has*`/call triplet for each fast-path handler, and a new one MUST follow that same pattern: a function type, an `atomic.Bool`, and the `Set`/`Has`/call methods.

- **EventBus MUST NOT be used for request and response.** It is pub/sub with no return channel, so emitting a request event and subscribing for a response event adds correlation IDs, timeouts and two event registrations that a direct function call avoids entirely.

- **A plugin MUST NOT call a plain exported function in another `internal/component/*` package directly to register a callback or reach shared engine state; it MUST go through DirectBridge or `DispatchCommand`.** The direct call compiles and works when the plugin runs internal, then silently no-ops when it runs external, because it mutates the subprocess's own disconnected copy of that package's state. Section "Plugin Process Boundary" carries the guard, and `./le plugin boundary check` enforces it.

## EventBus Typed Payloads (BLOCKING)

- **A new event MUST be declared with `events.Register[T](namespace, eventType)`, or with `events.RegisterSignal(namespace, eventType)` when it carries no payload.** `pkg/ze/eventbus.go` carries `payload any`, so the registry is the single source of truth for the payload type. Producers call `Handle.Emit(bus, payload)` and consumers call `Handle.Subscribe(bus, func(T))`.

- **Engine (in-process) subscribers deliver synchronously within `Emit`, and plugin-process subscribers deliver asynchronously, so a request and re-emit correlation MUST NOT assume every subscriber answered by the time `Emit` returns unless every subscriber is in-process.** `Emit` returns the plugin-process delivery count. The redistribute late-join replay holds its `ReplayID -> peer` map for a TTL rather than dropping it right after `Emit`, precisely because an out-of-process producer's re-emit arrives later.
<!-- source: internal/core/events/typed.go -- Emit returns plugin-process delivery count -->
<!-- source: internal/component/bgp/plugins/redistribute_egress/replay.go -- ReplayID token + TTL map -->

- **`SetStartupSubscriptions(events, peers, format)` subscribes in the protocol component's default namespace (`bgp`). An out-of-tree plugin that observes another namespace MUST call `SetStartupSubscriptionsIn(namespace, events, peers, format)`, and one that discriminates several event types sharing a payload shape MUST call `SetEnvelope(true)` and parse each delivery with `rpc.ParseEventEnvelope`.** An unregistered namespace is warned and skipped, never registered dead. An in-process plugin needs neither: it subscribes to any namespace directly on the `EventBus`.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- SetStartupSubscriptionsIn, SetEnvelope -->

- **A payload-carrying request event (`events.Register[T]`) SHOULD be preferred over a payload-less signal (`RegisterSignal`) when a returning batch has to be correlated back to a specific requestor.** The token rides the request and the producer echoes it, which keeps the returning batch peer-agnostic.

- **Every test file that defines a private mock of `ze.EventBus` MUST carry a compile-time check in that same file: `var _ ze.EventBus = (*<stubName>)(nil)`.** Without it, an interface change compiles the stub against an outdated signature and fails only when a test actually constructs the stub.

- **A subscriber MUST type-assert through the typed handle (`Event[T].Subscribe`) rather than calling `bus.Subscribe` directly.** The handle's wrapper logs a warn on a type mismatch; a raw `bus.Subscribe` caller swallows the mismatch in silence. The legacy `events.AsString` shim covers events not yet migrated to a typed handle, and MUST NOT be used in new code.

## Plugin Self-Containment

**A plugin MUST own its ENTIRE feature surface. Removing the plugin MUST make every one of its features disappear; every OTHER plugin and the core MUST keep working.**

- **Deleting a plugin's package directory plus its blank import in `internal/component/plugin/all/all.go` MUST remove every user-visible feature of that plugin and nothing else:** CLI commands, `show`/`set`/`clear`/`delete`/`update` subtrees, RPC registration, offline command registration, YANG command schema, help and usage text, completion entries, web and looking-glass routes, doctor checks, metrics.
- **The build MUST stay green after that deletion:** no dangling reference, no orphaned command spelling, no half-registered command anywhere.
- **Every other plugin and the core MUST keep fully working after it.** Removing BGP MUST NOT break iface, firewall, l2tp or the generic command plumbing.

- **A surface whose removal would break a different plugin, break the core, or leave a broken or empty command MUST be moved to its owner.** It is in the wrong package. The anti-patterns that fail this test are listed in `docs/architecture/command-ownership.md`, "What the Removal Test Forbids".

- **Generic command plumbing MUST NOT carry a plugin's command spelling; it carries selector scope only.** The dispatcher MAY extract a typed selector value because a YANG `ArgDef` declares it, and it contains no plugin grammar: not `peer`, not `bgp`, not `bfd`. The classification rule is ownership before grammar.

**The `ze-<ns>:` prefix on a WireMethod is a label, not an ownership claim, and is often a legacy misnomer. The owner MUST be determined by what the handler actually calls; the handler, schema, and registration MUST then be placed in that package.**

- **A command MUST stay in a central verb package only when it has no single removable owner:** it aggregates a cross-plugin registry, it reads a generic core system, or it is process-global (`show warnings`, `show health`, `subscribe`). Everything that reads one plugin's or one component's state MUST be placed with that owner, whatever the `ze-<ns>:` label on its WireMethod says. Worked examples are in `docs/architecture/command-ownership.md`, "Finding the Owner".

- **A carve MUST put the handler and its `pluginserver.RegisterRPCs` in the owner package, and the command YANG in `<owner>/yang/` as a standalone module that re-declares the path from the root.** The command YANG MUST NOT be nested under `<owner>/cmd/yang/`. The step-by-step, including the generator refresh a new `yang/` package needs, is `docs/architecture/command-ownership.md`, "Carving a Command Into Its Owner".

- **A verb whose subcommands belong to several owners MUST NOT declare its root container inside any one plugin.** Deleting that plugin would delete the whole verb. The root MUST live in a central, plugin-free `internal/component/cmd/<verb>` that holds NO handler, and each owner container-merges only its own subtree onto it.

- **The central verb anchor MUST NOT be deleted once every subcommand has carved out to an owner.** It is then a bare `container <verb>` owning no command, and it stays REQUIRED rather than optional, because an augmenting owner names it as its target module. `internal/component/cmd/clear` is the precedent.

- **A new carve SHOULD attach to a verb anchor by container merge rather than by `augment`.** The YANG loader unions same-named top-level containers, so container merge creates no base-module coupling. An `augment` names its target module, so deleting the anchor breaks every augmenting owner's build.

- **A feature that spreads across several verbs MUST get its own module `internal/component/<feature>` that owns every one of those commands, rather than being scattered across the verb packages.** Create the module when none exists. When two such modules would share low-level primitives, those primitives MUST be extracted to an `internal/core/<x>` package, so neither feature module depends on the other or on a central verb package.

- **A new feature MUST NOT require editing a `switch`, a `case`, a field list or a factory in a core or shared package: it registers and is discovered.** This holds for the CLI client model as much as for the daemon's command and schema tree. `plan/TEMPLATE.md` carries it as a review item, and the client-side view registry is described in `docs/architecture/command-ownership.md`, "Registration Over Hardcoding".

- **A removal-compliance test MUST exist and MUST run in verification:** build, or analyse the command and schema registries, with a plugin's provider import removed, and assert that no command, schema node, help string or handler reference to that plugin survives in any generic or central package. Adding this invariant means adding the gate that enforces it (`ai/rules/repo-maintenance.md`).

- **A carve MUST add BOTH halves of the guard: the banned token in the central verb schema's self-containment test, and the presence assertion in the owner's `yang/` package.** One half alone lets the command drift back into the central schema, or lets the surface vanish rather than move. The guards already in the tree, and the test names to extend, are in `docs/architecture/command-ownership.md`, "The Removal-Compliance Guards In The Tree".

## Plugin Process Boundary

- **A plugin that calls a same-process-effect function in another `internal/component/*` package directly MUST check `sdk.Plugin.IsInternal()` right after `sdk.NewWithConn(...)`.** Such a call works when the plugin runs internal and silently no-ops when it runs external, with no error, no panic and no log line. Why it is silent is `docs/architecture/plugin/plugin-system.md`, "The process boundary".

- **The call is the plugin's core purpose** (nothing useful happens without it) -> MUST hard refuse: log an error naming the specific call and why, `return 1` before doing anything else. See `internal/plugins/as112/register.go`, `internal/plugins/trafficusage/register.go`, `internal/plugins/flowexport/register.go`.
- **The plugin still provides real value external** (only one feature degrades) -> MUST warn: a `warnIfExternal(isInternal bool)` helper, called once after `sdk.NewWithConn`, logging what breaks and what still works. See `internal/plugins/cos/register.go`, `internal/plugins/ddos/detect/register.go`.

- **The severity choice MUST NOT be copy-pasted between plugins.** Judge each one on how much of its value actually survives running external.

- **A new instance of this class MUST be added to `dangerousCalls` in `internal/le/plugin/boundary/pluginboundary.go` when it is found and fixed, so `./le plugin boundary check` stays current.** An `allowlist` entry MUST cover only a package's own legitimate calls to its own function.

## Registration-Based Dispatch

**MUST NOT use switch/case to dispatch subcommands.** All command dispatch MUST use the registration pattern: register handlers into a dispatcher (or sub-dispatcher), then call `Dispatch(args)`. This applies at every level of nesting.

- **A command group that has sub-actions MUST be built with `subdispatch.New(name, summary)`, registering each sub-action with its handler and description.** The dispatcher then owns help, the unknown-command error and the suggestion. The template is `ai/patterns/plugin.md`, "Sub-Dispatcher Registration".

The following dispatch patterns MUST NOT be used:
- `switch args[0] { case "x": ... }` for command dispatch
- Manual "unknown command" error messages (the dispatcher handles this)
- Hand-written help listing subcommands (derive from registration)

## Feature-Gate Registration

- **An always-on (untagged, non-test) package MUST NOT import a gated feature package, for ANY reason: lifecycle or a borrowed helper.** One direct import pins the package into every binary and defeats the compile-out, and `./le tier check` fails on it. Always-on code reaches a feature only through build-tag-gated registration.

- **A non-lifecycle helper that always-on code needs MUST be extracted to an always-on `internal/core/*` leaf BEFORE the feature is gated.** Extract-then-gate is the order, and the registry work is the easy half.

- **Every gated package MUST be declared in the repo-root `feature-gates.txt`, and that file is the ONLY declaration point.** Every other consumer derives from it, so there is nothing to hand-sync. The line format, the consumer list and the current tag inventory are `docs/architecture/plugin/feature-gates.md` and the manifest itself.

- **The static files derived from the manifest MUST NOT have their tag lists hand-edited.** Add the gate to `feature-gates.txt`, run `./le feature-tags write`, then `./le feature-tags check`.

- **A new gate MUST be finished with `./le repository generate`, which emits `all_ze_<x>.go` and regenerates every derived tag list, and then `./le verify worktree`.** A feature-only helper MUST live INSIDE a gated file, or a no-feature build flags it U1000-unused. The six-step procedure is `docs/architecture/plugin/feature-gates.md`, "Procedure: add a feature gate".

- **A seam SHOULD be used ONLY when the listener construction registry genuinely cannot express the construction shape; the registry SHOULD be preferred.** ssh and gNMI are the two that qualify today. Both shapes, and what crosses the boundary in each, are in `docs/architecture/plugin/feature-gates.md`, "The registration shapes".

- **When start sites in DIFFERENT components reach one gated feature, the seam var MUST live in the always-on leaf both sites already import, never in the hub.** Only the part nothing always-on imports MUST be gated: the telemetry HTTP exporter is gated while the metric collection API stays in `internal/core/metrics`, so dependents keep working. A core leaf holding a nil-able hook var that a gated `init()` sets keeps `./le tier check` green, because it is a value rather than an import.

- **When the feature is already a self-registering plugin the generator discovers, gating is blank-import partitioning: there is no new `register_<x>.go` and no seam, and every owned directory MUST be its own `feature-gates.txt` line under the shared tag.** A protocol spans several discovered directories: engine, `transport`, `cli`, and the `*-cmd` command schema.

- **BOTH composition roots MUST be minded when gating a plugin: the generated `all.go` AND the hand-written `cmd/ze/ze_core_dispatch.go`.** A protocol with a programmatic `cli` package MUST move its dispatch-root blank import into a per-protocol gated `cmd/ze/dispatch_<proto>.go`; miss that root and the package stays linked. A plugin that registers its CLI through the registry's `CLIHandler` has only the ONE root and needs no dispatch companion.

- **An always-on importer that blocks a gate MUST be cleared by transitive package drop first, then core-leaf move, then inversion-of-control seam, and implementers SHOULD aim for the FEWEST source-tagged files rather than the fewest edits.** A core-leaf move MUST move the LEAF rather than the package, or it becomes a core-tier violation. A nil seam MUST have a CORRECT no-feature behaviour, not just a nil check. What each technique looks like at subsystem scale is `docs/architecture/plugin/feature-gates.md`.

- **A feature-gated file is still an always-on pin for a DIFFERENT gate, so gated files MUST be cleared too, not only untagged ones.** `dep_audit.file_requires_tag` is per-tag: a `ze_ssh`-on, `ze_bgp`-off build genuinely fails to compile when a `ze_ssh` file imports `bgp/config`.
- **After deleting an always-on import, what that package's `init()` was providing MUST be checked.** Removing the import can unlink an `init()` nobody else pulls in. A `package main` root can never be imported back, so linking it from `cmd/ze/dispatch_<x>.go` is always a safe edge.

- **A gated package that lives INSIDE another gate's package tree and imports it is a DEPENDENT piece, and its present build-tag test MUST carry the same compound constraint as the generated group file.** Otherwise the test runs in a build combination that cannot exist. The generator derives the constraint from the package path, so the manifest line stays the ordinary `<tag> <pkg>` with no new column. `ze_bmp` inside `ze_bgp` is the worked example, in `docs/architecture/plugin/feature-gates.md`.

- **A contract package other features consume, a nil-able seam or the value types a gated feature exposes, MUST stay OFF the manifest and always-on; only the machinery gates.** Every consumer of such a seam MUST already handle the absent case, so verify each call site before choosing this shape, and make the absent-build `nm` needles NAME the gated sub-packages instead of using the subtree prefix.

- **When a feature of plugin A exists only because feature B exists, A's files MUST carry B's tag, and each one MUST get a counterpart stub.** A stub MUST ANSWER HONESTLY, naming the missing feature, and MUST NOT silently no-op a user-visible request.

- **When the hub CONSTRUCTS a feature, parsing params and calling `eng.RegisterSubsystem`, rather than blank-importing it, a hub-local nil-able hook MUST carry the construction.** It is the ssh and gNMI seam shape, carrying only generic values across the boundary: config trees, the engine handle, portal entries.

- **A hand-maintained second list of gate tags or gated packages MUST NOT exist anywhere.**
- **A feature type MUST NOT appear in an always-on signature.** Widen the always-on handle to `Reconfigurable` or another always-on interface.
- **A gate MUST NOT be added without present and absent build-tag tests plus an `nm` symbol check.** The absent test asserts that zero feature symbols are linked.

## Related Rules

Command registration MUST also comply with these rules:
- `ai/patterns/cli-command.md`: owner-owned command registration.
- `ai/rules/cli.md`: typed selectors, command grammar ownership, ownership before grammar.
