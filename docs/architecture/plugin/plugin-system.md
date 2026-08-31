# Plugin System Reference

How a Ze plugin is registered, what the engine does with each declared field,
and how a plugin reaches the rest of the daemon. The obligations this page
describes are stated in `ai/rules/plugins.md`; the page holds the mechanism.

The wire protocol between the engine and a plugin process is
`docs/architecture/api/process-protocol.md`. Command ownership and the folder
tests are `docs/architecture/command-ownership.md`. The structural template for
a new plugin is `ai/patterns/plugin.md`.

## Where each layer lives

| Layer | Location | Purpose |
|-------|----------|---------|
| Registry | `internal/component/plugin/registry/` | Central registry. A leaf package with no plugin dependencies |
| Family registry | `internal/core/family/` | Cross-component address-family registration: the `Family`, `AFI` and `SAFI` types plus `family.MustRegister` |
| Public SDK | `pkg/plugin/sdk/` | Callback abstraction for external plugins |
| RPC types | `pkg/plugin/rpc/` | Shared YANG RPC types and `MuxConn` for concurrent RPCs |
| Internal plugin | `internal/component/bgp/plugins/<name>/`, `internal/plugins/<name>/` | Plugin implementation plus its `register.go` |
| Composition root | `internal/component/plugin/all/` | Blank imports that trigger every `init()`. Generated |
| CLI shared | `internal/component/plugin/cli/` | `PluginConfig` and `RunPlugin()` |

<!-- source: internal/component/plugin/registry/registry.go -- Registration -->
<!-- source: internal/core/family/registry.go -- MustRegister -->
<!-- source: internal/component/plugin/cli/cli.go -- BaseConfig, PluginConfig, RunPlugin -->

## Registration fields

Each plugin registers exactly one `registry.Registration` from its `init()`.
Four fields are required; the rest are optional.

| Field | Type | Purpose |
|-------|------|---------|
| `Name` | `string` | Plugin name, hyphenated (`bgp-gr`) |
| `Description` | `string` | Human-readable description for help text |
| `RunEngine` | `func(conn net.Conn) int` | Engine-mode entry point over a single connection |
| `CLIHandler` | `func(args []string) int` | Handler for `ze plugin <name>` |

Optional metadata:

| Field | Type | Purpose |
|-------|------|---------|
| `RFCs` | `[]string` | Related RFC numbers |
| `Families` | `[]string` | Address families handled, as `afi/safi` |
| `CapabilityCodes` | `[]uint8` | Capability codes this plugin decodes |
| `ConfigRoots` | `[]string` | Config roots the plugin wants |
| `Dependencies` | `[]string` | Plugin names that must also load. A missing name gives `ErrMissingDependency` |
| `OptionalDependencies` | `[]string` | Plugin names the owner uses when present. A missing name is skipped in silence |
| `EventTypes` | `[]string` | Event types this plugin produces. Registered at startup |
| `SendTypes` | `[]string` | Send types this plugin enables, such as `enhanced-refresh` |
| `Claims` | `[]string` | Exclusive runtime roles this plugin takes over from another plugin's default |
| `PeerUpBarrier` | `bool` | The plugin registers the peer on the peer-up event, so End-of-RIB waits for it |
| `YANG` | `string` | YANG schema content |
| `FilterTypes` | `[]string` | YANG filter list names this plugin owns, such as `prefix-list`. Names are globally unique; a duplicate aborts startup |
| `DoctorChecks` | `[]DoctorCheckDef` | Doctor readiness checks. `Component` is set from `Name` |
| `FatalOnConfigError` | `bool` | A configure-callback failure exits `ze` instead of running without the plugin |
| `Features` | `string` | Space-separated CLI feature flags, such as `nlri yang capa` |
| `SupportsNLRI` | `bool` | The plugin decodes NLRI from the CLI |
| `SupportsCapa` | `bool` | The plugin decodes capabilities from the CLI |

Optional handlers. Each one lets infrastructure code reach a plugin without
importing its package:

| Field | Type | Purpose |
|-------|------|---------|
| `ConfigureEngineLogger` | `func(loggerName string)` | Set the logger for in-process engine mode |
| `ConfigureMetrics` | `func(reg metrics.Registry)` | Called before `RunEngine` so the plugin registers its gauges and counters |
| `ConfigureEventBus` | `func(eventBus ze.EventBus)` | Called before `RunEngine` with the bus the plugin emits on |
| `ConfigurePluginServer` | `func(server PluginServerAccessor)` | Called before `RunEngine` with the plugin server, through a leaf interface |
| `RPCHandlers` | `map[string]func(json.RawMessage) (any, error)` | RPC method name to handler, collected by the plugin server |
| `InProcessDecoder` | `func(input, output *bytes.Buffer) int` | Decode function for the CLI fallback path |
| `InProcessNLRIDecoder` | `func(family, hex string) (any, error)` | NLRI decode without RPC |
| `InProcessNLRIEncoder` | `func(family string, args []string) (string, error)` | NLRI encode without RPC |
| `InProcessRouteEncoder` | `func(routeCmd, family string, localAS uint32, isIBGP, asn4, addPath bool) ([]byte, []byte, error)` | Builds a full UPDATE for `ze bgp encode` |
| `InProcessConfigVerifier` | `func([]rpc.ConfigSection) error` | Side-effect-free equivalent of `OnConfigVerify`, used by static, API and CLI validation |
| `InProcessConfigNLRIBuilder` | `func(matchCriteria map[string][]string, isIPv6, forVPN bool) []byte` | Builds NLRI bytes from config-format match criteria |
| `InProcessConfigRouteParser` | `func(req ConfigRouteRequest) (PluginRoute, error)` | Parses an update block's NLRI tokens and attribute block into a route |

<!-- source: internal/component/plugin/registry/registry.go -- Registration -->

## The setup record: a plugin's second declaration

`registry.Register` says what a plugin IS. `registry.RecordSetup(name, outcome,
reason)` says what its setup ACHIEVED. Both are called from the plugin's own
`init()`, both are keyed by the plugin name, and neither reads the other, so a
plugin author never has to know that Go initializes the files of one package in
filename order.

| Outcome | Meaning | Effect on the daemon |
|---------|---------|----------------------|
| `SetupSucceeded` | The setup completed | None |
| `SetupFailedSoft` | The feature is absent and the daemon runs correctly without it | The daemon starts |
| `SetupFailedHard` | The daemon cannot run without it | `hub.run` refuses to start, naming every failing module |
| `SetupUnknown` | Recorded nothing. A stored state, never a valid ARGUMENT to `RecordSetup` | None |

The record is optional at the call site, so no existing registration changes.
`show module list` derives its rows from the registry, so a plugin that
recorded nothing is listed as `unknown` rather than dropped: absence would read
as "not built into this binary", which is precisely the silence this record
exists to remove.

The reason reaches CLI output as data, so a recording site MUST NOT put a
secret in one.
<!-- source: internal/component/plugin/registry/setup.go -- RecordSetup, SetupOutcome, SetupResults -->
<!-- source: internal/plugins/memlock/memlock_linux.go -- init, the worked example -->

## Dependencies

| Field | Semantics |
|-------|-----------|
| `Dependencies` | Hard. `ResolveDependencies` returns `ErrMissingDependency` when the named plugin is not registered, and startup fails |
| `OptionalDependencies` | Soft. The resolver pulls the plugin in when it is registered and skips it in silence when it is not |

Validation at registration is the same for both fields: an empty string and a
self-dependency are rejected. Cycle detection and `TopologicalTiers` walk both
kinds of edge when both endpoints appear in the resolved name set, so startup
order holds whenever the optional dependency is present.

Graceful fallback belongs to the owner. `bgp-rs` is the worked example. It uses
`bgp-adj-rib-in` for replay on peer-up, dispatches the command normally, treats
the engine's `ErrUnknownCommand` string as the plugin-absent signal, logs one
`WARN` per process with `sync.Once`, skips the replay convergence loop, and
continues.

<!-- source: internal/component/plugin/registry/registry.go -- OptionalDependencies -->

## Address family registration

An NLRI plugin registers the families it handles with
`family.MustRegister(afi, safi, afiStr, safiStr)` at package init time. The four
RFC 4760 base families (`IPv4Unicast`, `IPv6Unicast`, `IPv4Multicast`,
`IPv6Multicast`) live in `internal/core/family/registry.go`. Every other family
is owned by its plugin's `types.go`.

| Property | Detail |
|----------|--------|
| One canonical name per family | No aliases. The `afiStr`/`safiStr` arguments form the canonical `<afi>/<safi>` string |
| Conflict is fatal | `family.MustRegister` panics when AFI or SAFI numbers collide with a different name. The same name with the same numbers is a no-op |
| The plugin owns the SAFI name | The `vpn` plugin chose `mpls-vpn`, the `flowspec` plugin chose `flow` |
| External plugins use the protocol | A forked plugin declares families in `declare-registration` at stage 1 with AFI and SAFI numbers. The engine forwards them to `family.RegisterFamily` |
| Tests use the helper | `family.RegisterTestFamilies()` registers a SAFI no internal plugin registers |

<!-- source: internal/core/family/registry.go -- MustRegister -->
<!-- source: internal/component/plugin/server/startup.go -- registerPluginFamilies -->

## Startup phases and callbacks

Stages 1 to 5 of the handshake run per phase. The engine loads plugins across up
to five phases in series: config-path auto-load, explicit, family, event-type,
send-type. A plugin's `OnStarted` therefore fires after its own handshake, and
before the plugins of a later phase are loaded.

| Callback | What belongs in it |
|----------|--------------------|
| `OnStarted(fn)` | Local setup: long-lived goroutines, subscriptions, per-plugin state |
| `OnAllPluginsReady(fn)` | Any `DispatchCommand` that targets another plugin's command at startup. The callback fires through the event loop once the dispatcher command registry is frozen, so cross-plugin dispatch resolves |

`bgp-rpki` is the reference. Its `adj-rib-in enable-validation` dispatch runs
from `OnAllPluginsReady`. From `OnStarted` it can run before `bgp-adj-rib-in`
loads and fail with "unknown command".

<!-- source: internal/component/bgp/plugins/rpki/rpki.go -- OnAllPluginsReady dispatch -->

## Exclusive role claims

When plugin A takes over a role that plugin B performs by default, B learns it
before it can receive its first runtime event.

| Step | What |
|------|------|
| A declares | `Claims: []string{"<role-token>"}` in its `registry.Registration` |
| The engine resolves | It unions the claims of the whole startup set and delivers them on every plugin's stage-2 configure callback (`rpc.ConfigureInput.Claims`) |
| B stands down | It reads `sdk.Plugin.ClaimActive("<role-token>")` from its `OnConfigure` handler |

Stage 2 is part of the sequential handshake, so it completes before stage-5
ready and therefore before `SignalPluginStartupComplete` reaches `StartPeers`.
The token is opaque to the engine: A and B agree on the spelling, and B spells
it itself rather than importing A's package, so deleting A leaves B building and
self-serving.

`OnAllPluginsReady` cannot resolve a role. `sendPostStartupToAll` fans that
callback out on detached goroutines immediately before peers start, so waiting
there deadlocks, and a role resolved from it races the first session by 1 to 2
milliseconds on an idle host.

An unclaimed or unresolvable role reads `false`, so B keeps doing the work. When
a claimant never reaches Running, the engine logs it through
`verifyAdvertisedClaims` and startup continues.

A claim is daemon-wide and delivery is per-peer, so the engine retracts a claim
per event. Each peer-scoped event carries the claimed roles that no process
being fed that event holds: `StructuredEvent.UnheldRoles` on the direct bridge,
and the `unheld-roles` member of the JSON event otherwise. An absent list means
every claim holds, which is the common case.

Reference: `bgp-rs` claims `bgp-peer-up-replay`, and `bgp-adj-rib-in` stands
down.

<!-- source: internal/component/bgp/plugins/rs/register.go -- Claims -->
<!-- source: internal/component/bgp/plugins/adj_rib_in/rib_claims.go -- ClaimActive -->

## Peer-up barrier

A plugin that decides, on the peer-up event, whether a peer can receive traffic
declares `PeerUpBarrier: true`. The engine then holds that peer's initial-sync
End-of-RIB until the plugin has taken delivery of the event, so "End-of-RIB
sent" implies "every barrier plugin has registered this peer".

| Step | What |
|------|------|
| The plugin declares | `PeerUpBarrier: true` in its `registry.Registration` |
| The engine counts | The barrier-declaring plugins among those the peer-up event is actually delivered to, before the first delivery |
| The engine acknowledges | Each successful delivery result signals the peer's barrier. The result is the plugin's acknowledgement that its handler ran and returned |
| The peer waits | `Peer.waitPeerUpBarrier` runs before the initial-sync End-of-RIB |

The count is taken over the delivery set rather than the registry. A plugin that
declares the barrier but subscribes to no state events never takes delivery, so
counting it would cost every peer the full timeout.

The barrier is separate from the API-sync wait. `apiSync` counts plugins that
send routes and carries a 500 ms IPC grace for external ones. Barrier plugins
only register. A peer whose only barrier plugin is in-process does not block on
the wait: state-event delivery is synchronous, so the acknowledgement has landed
on the FSM callback goroutine by the time `sendInitialRoutes` is spawned.

The wait is bounded. A plugin that never acknowledges delays the marker to
`peerUpBarrierTimeout` (2 s), which releases it with a WARN naming the peer and
the shortfall. Establishment is never blocked.

`bgp-rs` declares the barrier because `handleState` sets `Up` and captures
`ForwardFrom` in one critical section. An UPDATE taken delivery of before that
lands at or below the peer's cut and belongs to the announce-only Adj-RIB-In
replay, so its withdrawals never reach the peer.

<!-- source: internal/component/bgp/server/events.go -- countPeerUpBarrier -->
<!-- source: internal/component/bgp/reactor/peer_initial_sync.go -- waitPeerUpBarrier -->

## Cross-boundary value types

A payload that crosses a plugin or component boundary is a self-contained value
type. A shared type definition such as `family.Family` or `RouteChangeBatch` is a
compile-time contract and stays allowed. What is forbidden is one plugin holding
a pointer to data another plugin allocated.

| Surface | What crosses it |
|---------|-----------------|
| Event bus (`Emit`/`Subscribe`) | Value types only: numeric IDs, `family.Family`, `netip.Prefix`, `netip.Addr`, enum `uint8` |
| Cross-plugin identifiers | Registered numeric IDs, not pointers into a shared registry |
| Cross-component IPC | `*foopkg.Something` as a payload field is refused even when `foopkg` is shared core |
| Registry surfaces | Value types only: IDs, immutable string copies, bits, not pointers to producer-allocated handles |

## Communication patterns

DirectBridge (`pkg/plugin/rpc/bridge.go`) gives typed direct function calls
between the engine and an internal plugin, and bypasses JSON serialization and
socket I/O.

| Pattern | Mechanism | Use when |
|---------|-----------|----------|
| Async broadcast, one to many | EventBus (`pkg/ze/eventbus.go`) | A component notifies zero or more listeners about a state change and needs no return value. Example: `(l2tp, session-down)`, `(bgp-rib, best-change)` |
| Sync request and response, one to one | DirectBridge typed handler | The core calls a plugin function with typed arguments and waits for a typed result. Example: `ForwardCached`, `DispatchCommand`, `EmitEvent` |
| Structured event delivery | DirectBridge `DeliverStructured` | The engine delivers pre-parsed event data to internal plugins with no JSON. Example: `StructuredEvent` for BGP UPDATEs |
| Text command dispatch | `DispatchCommand`, over the bridge or the pipe | A plugin sends a text command to the engine's command registry. The slow path, for ad-hoc or external callers |

The bridge struct carries a `Set*`/`Has*`/call triplet for each fast-path
handler, so a new handler is a function type, an `atomic.Bool`, and those three
methods.

Structured event delivery, the event payload of each event type, and the
subscription namespace rules are in
`docs/architecture/api/process-protocol.md`.

## The process boundary

A plugin runs internal, as a goroutine sharing the daemon's process through
`startInternal`, or external, as a forked subprocess that talks only over
TLS/RPC through `startExternal`. Plugin code reaches the engine through the
SDK's RPC layer, which hides the difference.

A plugin that instead calls a plain exported Go function in another
`internal/component/*` package reaches straight into that package's
process-local state. The call works when the plugin runs internal, because the
memory is shared. It does nothing useful when the plugin runs external: it
mutates the subprocess's own disconnected copy of that state. There is no error,
no panic and no log line, so the feature quietly never works.

`./le plugin boundary check` scans for this class. It reads every package under
the generator's plugin search roots, derived at run time from
`pluginDirs` plus `nestedPluginDomains` rather than a second hardcoded list, and
it fails when a plugin package contains a call from the maintained
`dangerousCalls` list with no `.IsInternal()` or `warnIfExternal(` call anywhere
in that same package. `--print-roots` prints the derived set. The check is a
presence heuristic: it does not prove the guard covers the call at run time.
An `allowlist` entry covers a package's own legitimate calls to its own
function.

<!-- source: internal/component/plugin/process/process.go -- startInternal, startExternal -->
<!-- source: internal/le/plugin/boundary/pluginboundary.go -- loadScanRootsFrom, dangerousCalls -->

## A registered name lives in many loose strings

A plugin or subsystem name is not a single string. It appears in places that all
have to agree, and most of them are loose strings that no compiler checks.

| Consumer | Where to grep | Looks like |
|----------|--------------|-----------|
| Plugin registration | `internal/component/bgp/plugins/*/register.go`, `internal/component/plugin/all/all.go` | `Name: "bgp-gr"` |
| Subsystem logger | `internal/core/slogutil/`, `slogutil.Logger("...")` calls | `slogutil.Logger("bgp.gr")` |
| Env var registration | `env.MustRegister("ze.log.bgp.gr", ...)` | `ze.log.<name>` |
| YANG config keys | `internal/component/*/yang/*.yang`, `grouping` and `container` names | `container gr { ... }` |
| Config consumer | `internal/component/bgp/config/`, any string-keyed lookup in the parsed tree | `tree["bgp"]["gr"]` |
| Dispatch keys | `dispatchCommand("bgp gr ...")`, command prefix matching | `"bgp gr"` |
| Test fixtures | `test/**/*.ci`, `test/**/*.conf`, env vars in tests | `option=env:var=ze.log.bgp.gr` |
| Documentation | `docs/`, `<!-- source: -->` anchors | text references |
| Problem journal | `plan/journal/*.md` | text references |

The mechanical check before a rename:

```
old_name="bgp-gr"  # what you are renaming away from
new_name="bgp.gr"  # what you are renaming to
grep -rn "$old_name" internal/ pkg/ cmd/ test/ docs/ plan/ .claude/ 2>/dev/null
```

Every match is a deliberate keep (vendored code, history, a learned summary) or
a bug.

Subsystem and log keys use dots (`bgp.gr`). Plugin names registered with
`registry.Register()` use hyphens (`bgp-gr`). The two are not the same string,
and the hub canonicalizes hyphen to dot for in-process subsystem names.

## The SDK stays generic

`pkg/plugin/sdk/` carries no plugin-specific code. Adding or removing a callback
type needs one `On*` method in `sdk_callbacks.go` that registers a handler in
the callback map. The event loops, the dispatch logic and the transport layers
are callback-agnostic: they dispatch through `map[string]callbackHandler`
without knowing which callbacks exist.

| Property | Meaning |
|----------|---------|
| No switch on method names in an event loop | Dispatch is a map lookup, not an enumeration |
| No transport-specific handler method | One handler per callback, used by the pipe and the bridge |
| Bye is the only special case | It ends the loop, checked by method name rather than by handler signature |
| A new callback is one `On*` method | `sdk_dispatch.go` and the event-loop code do not change |

<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- On* registration -->

## Proximity

Related code belongs in one folder. The mechanical check is the delete-the-folder
test in `docs/architecture/command-ownership.md`.

| Property | Meaning |
|----------|---------|
| All the code for a concern sits in its folder | Commands, handlers, registration and logic, not scattered across packages |
| No external reference to internals | Infrastructure, the reactor and other units never import a specific plugin or command module |
| A blank import is the only coupling | One `_ "package"` triggers `init()`, and removing it disables the unit cleanly |
| The engine core works with no command module | The reactor, the FSM and the wire layer function without CLI command handlers |

A doctor check follows the same rule. The check, its registration and its unit
test live in the plugin, component, backend or command package that owns the
dependency. `internal/component/doctor` keeps the runner, the user-entry
functional tests, and the checks for dependencies with no narrower owner.

## Registration metadata feeds the website catalog

The website plugin catalog in `../gh-pages/docs/features/plugins/` is generated
from `registry.Registration` fields by `./le site build`, and
`internal/le/site/plugins.go` owns the producer. `Name`, `Description`, `ConfigRoots`,
`Dependencies`, `OptionalDependencies` and `YANG` are public catalog data.

Local prose or display metadata goes in a `PLUGIN.md` next to that plugin's
`register.go`, with front matter (`area`, `summary`, `tags`) and a Markdown
body. A new machine fact goes into the registry or the extractor, and the
website renders from that data. Catalog grouping is derived from `ConfigRoots`
and the source path layout, so a package move or a config-root change changes
the generated site.

<!-- source: internal/le/site/plugins.go -- renderPluginCatalog, pluginEntries -->
