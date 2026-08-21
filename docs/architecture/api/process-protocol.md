# Plugin Process Protocol

**Purpose:** Document Ze's plugin communication protocol and process lifecycle.

---

## Overview

Ze plugins communicate with the engine via newline-framed YANG RPCs over a
single bidirectional connection. All messages use the wire format
`#<id> <verb> [<json>]\n`.
<!-- source: pkg/plugin/rpc/conn.go -- Conn doc comment -->

- **Events (engine to plugin):** BGP events delivered via `deliver-event` or `deliver-batch` RPCs
- **Commands (plugin to engine):** Route updates, command dispatch, event emission via engine RPCs
- **Callbacks (engine to plugin):** Config verification, command execution, OPEN validation

The protocol is the same for all invocation modes (internal goroutine, external subprocess).
Internal plugins get a performance optimization via `DirectBridge` after startup.
<!-- source: pkg/plugin/rpc/bridge.go -- DirectBridge -->

---

## Wire Format

Every message is a single newline-terminated line:
<!-- source: pkg/plugin/rpc/message.go -- FormatRequest, FormatResult, FormatError -->

| Message type | Format | Example |
|-------------|--------|---------|
| Request | `#<id> <method> [<json>]\n` | `#1 ze-plugin-engine:ready {"subscribe":{...}}` |
| Success | `#<id> ok [<json>]\n` | `#1 ok {"peers-affected":2}` |
| Error | `#<id> error [<json>]\n` | `#1 error {"code":"error","message":"..."}` |

A command answer has a second form, and a plugin that names `record-answers` at
Stage 3 both reads it and writes it. The engine answers that plugin's
`dispatch-command` and `dispatch-command-args` with a head, zero or more
records, and a terminator. The plugin answers the engine's `execute-command` the
same way. Each line is `#<id> ok <key=value tail>`. Every other request, and
every plugin that named nothing, keeps the single line above. The grammar is in
[ipc_protocol.md](ipc_protocol.md), "Answer Protocol".
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerHead, AppendAnswerTerminator -->
<!-- source: internal/component/plugin/server/dispatch_registry.go -- answerResult -->
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- Plugin.answerExecuteCommand -->

- `<id>` is a monotonically increasing `uint64` correlation ID
<!-- source: pkg/plugin/rpc/conn.go -- NextID -->
- Methods use YANG-style `<module>:<rpc-name>` naming
- JSON payloads are optional (omitted when empty or null)
- Error payloads carry `code` and `message` fields
<!-- source: pkg/plugin/rpc/message.go -- NewErrorPayload -->

**Framing:** `FrameReader` uses `bufio.Scanner` with newline splitting.
`FrameWriter` appends `\n` to each message. Maximum message size is 16 MB.
<!-- source: pkg/plugin/rpc/framing.go -- FrameReader, FrameWriter, MaxMessageSize -->

**Multiplexing:** `MuxConn` wraps a `Conn` to support concurrent RPCs on a single
connection. A background reader goroutine routes responses (verb `ok`/`error`) to
waiting `CallRPC` callers by `#<id>`, and pushes inbound requests (verb is a method
name) to the `Requests()` channel.
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn, readLoop -->

---

## 5-Stage Startup Protocol

Ze uses a synchronized 5-stage startup protocol with barriers between stages.
Within each dependency tier, all plugins must complete each stage before any
can proceed to the next. Tiers are sequenced by dependency order (see
Tier-Ordered Startup below).
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->

```
+---------------------------------------------------------------------------+
|                          STARTUP TIMELINE                                 |
+---------------------------------------------------------------------------+
|                                                                           |
|    Plugin A               Coordinator              Plugin B               |
|    --------               -----------              --------               |
|                                                                           |
|    STAGE 1: REGISTRATION                                                  |
|    #1 declare-registration  |                 #1 declare-registration     |
|    {...} ------------------>|                 {...} ------------------>    |
|    <-- #1 ok                |                 <-- #1 ok                   |
|                             |                                             |
|                  BARRIER (all plugins complete Stage 1)                   |
|                                                                           |
|    STAGE 2: CONFIG DELIVERY                                               |
|    <-- #1 configure {...}   |                 <-- #1 configure {...}      |
|    #1 ok ------------------>|                 #1 ok ------------------>   |
|                             |                                             |
|                  BARRIER (all plugins complete Stage 2)                   |
|                                                                           |
|    STAGE 3: CAPABILITY DECLARATION                                        |
|    #2 declare-capabilities  |                 #2 declare-capabilities     |
|    {...} ------------------>|                 {...} ------------------>    |
|    <-- #2 ok                |                 <-- #2 ok                   |
|                             |                                             |
|                  BARRIER (all plugins complete Stage 3)                   |
|                                                                           |
|    STAGE 4: REGISTRY SHARING                                              |
|    <-- #2 share-registry    |                 <-- #2 share-registry       |
|    {...} ------------------>|                 {...} ------------------>    |
|    #2 ok                    |                 #2 ok                       |
|                             |                                             |
|                  BARRIER (all plugins complete Stage 4)                   |
|                                                                           |
|    STAGE 5: READY                                                         |
|    #3 ready {...} --------->|                 #3 ready {...} --------->   |
|    <-- #3 ok                |                 <-- #3 ok                   |
|                             |                                             |
|                  BARRIER (all plugins ready)                              |
|                             |                                             |
|    [BGP peers start]        |                 [BGP peers start]           |
|                                                                           |
+---------------------------------------------------------------------------+
```

**Barrier Semantics:**
- Each plugin signals stage completion via `StageComplete(pluginID, stage)`
- Coordinator waits until ALL plugins complete the current stage
- Only then does coordinator advance to next stage
- All waiting plugins unblock simultaneously

**Stage RPCs:**
<!-- source: pkg/plugin/rpc/types.go -- all stage input types -->

| Stage | Direction | RPC Method | Input Type |
|-------|-----------|-----------|------------|
| 1. Registration | Plugin to Engine | `ze-plugin-engine:declare-registration` | `DeclareRegistrationInput` |
| 2. Config | Engine to Plugin | `ze-plugin-callback:configure` | `ConfigureInput` |
| 3. Capability | Plugin to Engine | `ze-plugin-engine:declare-capabilities` | `DeclareCapabilitiesInput` (see `protocol` below) |
| 4. Registry | Engine to Plugin | `ze-plugin-callback:share-registry` | `ShareRegistryInput` |
| 5. Ready | Plugin to Engine | `ze-plugin-engine:ready` | `ReadyInput` |
| Post | Engine to Plugin | `ze-plugin-callback:post-startup` | (empty) |

**Wire shape negotiation (Stage 3).** `DeclareCapabilitiesInput` carries an
optional `protocol` list. It names the wire shapes the peer speaks.
`record-answers` is the only name today, and it is symmetric. The plugin READS
the record answer form of `dispatch-command` and `dispatch-command-args`, and it
WRITES that form for `execute-command`. No engine-to-plugin message carries a
protocol list, so this one line is where both facts come from.

The Go SDK sends the name for every plugin built on it, and a plugin author sets
no field to get it. A peer that speaks the protocol by hand chooses for itself.
<!-- source: pkg/plugin/sdk/sdk.go -- Plugin.Run, Stage 3 declare-capabilities -->

The engine records the declaration before the capability injector runs. Every
answer after the Stage 3 barrier is therefore written in a shape the peer agreed
to. An absent list, an empty list, and an unknown name all read false.
<!-- source: pkg/plugin/rpc/types.go -- DeclareCapabilitiesInput.Understands, ProtocolRecordAnswers -->
<!-- source: internal/component/plugin/server/startup.go -- engineStartupSink -->
<!-- source: internal/component/plugin/process/process.go -- Process.SetRecordAnswers -->

**Timeout:** Each stage has a 5-second timeout (configurable via `stage-timeout` in plugin config).
If any plugin fails to complete a stage, startup aborts for all plugins.

**Post-Startup Callback.** Stages 1-5 run per-phase. Plugins load across up to
five phases (config-path auto-load, explicit, family, event-type, send-type) in
serial order, so the dispatcher command registry only contains every plugin's
commands after the last phase completes. The engine sends the `post-startup`
callback to every running plugin after `signalStartupComplete` has frozen both
the plugin registry and the dispatcher command registry; at that point cross-
plugin `DispatchCommand` is guaranteed to resolve. Plugins register a handler
via the SDK `OnAllPluginsReady` method. Delivery is best-effort (fire-and-
forget, bounded timeout, per-plugin goroutine) so a single slow or broken
plugin cannot delay notification to the rest.
<!-- source: internal/component/plugin/server/startup.go — sendPostStartupToAll -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go — OnAllPluginsReady -->

**Why Barriers:**
- Ensures all plugins register commands before any receive config
- Ensures all capabilities declared before registry shared
- Prevents race conditions in multi-plugin configurations
- Guarantees consistent state before BGP peers start

**Shared stage-driver.** Both startup callers drive these five stages through a
single implementation, `runStartupHandshake`, rather than each hard-coding the
wire sequence. The driver owns the wire choreography -- reading each
plugin-initiated request, validating its method string, responding, and sending
the two engine-initiated callbacks -- while the caller-specific effects between
stages are injected through a `startupSink`. The engine's `engineStartupSink`
performs the full registration set (registry, families, capabilities, commands,
subscriptions, bridge dispatch, reactor signaling) and synchronizes each tier
through the `StartupCoordinator` barrier; the hub's `hubStartupSink` harvests the
plugin's declared commands and schema, delivers nil config and nil registry, and
runs with no barrier. A protocol change (a new method string, a reordered stage,
an added validation) therefore touches one place.
<!-- source: internal/component/plugin/server/startup_driver.go -- runStartupHandshake, startupSink -->
<!-- source: internal/component/plugin/server/startup.go -- engineStartupSink -->
<!-- source: internal/component/plugin/server/subsystem.go -- hubStartupSink -->

**Filter Declaration (Stage 1):**

Plugins may include a `filters` list in their `declare-registration` to offer named
route filters. Each entry declares a filter name, direction (import/export/both),
requested attributes, NLRI/raw payload needs, failure mode, and optional overrides
of default filters.

| Field | Type | Description |
|-------|------|-------------|
| `filters[].name` | string | Filter name (referenced in config as `<plugin>:<name>`) |
| `filters[].direction` | enum | `import`, `export`, or `both` |
| `filters[].attributes` | list | Attribute names to receive (e.g., `as-path`, `community`) |
| `filters[].nlri` | bool | Include parsed NLRI text; defaults to true |
| `filters[].raw` | bool | Include raw UPDATE body hex for wire-format filters |
| `filters[].on-error` | enum | `reject` (fail-closed) or `accept` (fail-open) |
| `filters[].overrides` | list | Default filters this filter replaces (e.g., `rfc:no-self-as`) |

Config references filters as `<plugin>:<filter>` in `filter { import [...] export [...] }`.
The runtime uses the declaration to choose raw payload delivery, validate modify
deltas against declared attributes, and pick fail-open or fail-closed behavior on
filter RPC errors.

<!-- source: pkg/plugin/rpc/types.go -- FilterDecl -->
<!-- source: internal/component/plugin/server/startup.go -- registrationFromRPC filters -->
<!-- source: internal/component/plugin/server/server.go -- FilterInfo and FilterOnError -->

**Doctor Check Declaration (Stage 1):**

Plugins may include a `doctor-checks` list in their `declare-registration` to provide
runtime health checks. Each entry declares a check name, phase, ordering, and the
diagnostic codes it may emit. The engine stores declarations in `PluginRegistration`
and invokes them via the `doctor-check` callback when `show doctor` runs.

| Field | Type | Description |
|-------|------|-------------|
| `doctor-checks[].name` | string | Check name (kebab-case, 1-128 chars) |
| `doctor-checks[].phase` | enum | `pre-config`, `missing-config`, or `post-config` |
| `doctor-checks[].order` | int | Ordering within phase (0-9999, default 0) |
| `doctor-checks[].dependencies` | list | Other check names that must run first |
| `doctor-checks[].platforms` | list | Platform filter (empty defaults to `any`) |
| `doctor-checks[].codes` | list | Diagnostic codes (1-16, must start with `doctor-`) |

Offline `ze doctor` does not invoke plugin doctor checks (plugins are runtime
processes). Plugin checks cover runtime health; Go-registered checks cover
pre-start readiness.

<!-- source: pkg/plugin/rpc/types.go -- DoctorCheckDecl -->
<!-- source: internal/component/plugin/server/startup.go -- registrationFromRPC doctor checks, validateDoctorCheckDecls -->
<!-- source: internal/component/plugin/registration.go -- DoctorCheckRegistration -->

**Enricher Declaration (Stage 1):**

Plugins may include an `enrichers` list in their `declare-registration` to provide
show command enrichment. Each entry declares a command path and unique key. The
server registers a proxy enricher via `show.Register()` for each declaration. At
show time, the proxy serializes the base map, calls `enrich-show` with a 2s timeout,
and merges the response into the base map. Proxy enrichers are cleaned up via
`show.Unregister()` when the plugin process exits.

| Field | Type | Description |
|-------|------|-------------|
| `enrichers[].command` | string | Show command path (e.g., `show subscriber detail`) |
| `enrichers[].key` | string | Unique enricher key within command (kebab-case, 1-128 chars) |

<!-- source: pkg/plugin/rpc/types.go -- EnricherDecl -->
<!-- source: internal/component/plugin/server/enricher.go -- registerProxyEnrichers, validateEnricherDecls -->

### Tier-Ordered Startup

Plugins are grouped into dependency tiers before handshake begins. All processes
are started at once (single ProcessManager), but the 5-stage handshake is
sequenced tier by tier. Tier 0 completes its full handshake -- including command
registration -- before tier 1 begins.

```
Tier computation (Kahn's algorithm / BFS layering):
  Tier 0: plugins with no dependencies      (e.g., bgp-adj-rib-in)
  Tier 1: plugins depending only on tier 0   (e.g., bgp-rs depends on bgp-adj-rib-in)
  Tier N: plugins whose deps are all in tiers < N
```

**Per-tier coordinator:** Each tier gets its own `StartupCoordinator` with
tier-local indices. Processes in later tiers block naturally on `net.Pipe` write
until the engine reads their `declare-registration` during their tier's turn.

```
+-------------------------------------------------+
|            TIER-ORDERED STARTUP                  |
+-------------------------------------------------+
|                                                  |
|  ProcessManager starts ALL processes             |
|         |                                        |
|         v                                        |
|  TopologicalTiers(names) -> [[rib], [rs]]        |
|         |                                        |
|         v                                        |
|  TIER 0: [bgp-adj-rib-in]                       |
|    Coordinator(1 plugin)                         |
|    5-stage handshake -> commands registered       |
|    procWg.Wait()                                 |
|         |                                        |
|         v                                        |
|  TIER 1: [bgp-rs]                                |
|    Coordinator(1 plugin)                         |
|    5-stage handshake -> can dispatch to rib      |
|    procWg.Wait()                                 |
|         |                                        |
|         v                                        |
|  ALL TIERS DONE                                  |
|    coordinator = nil                             |
|    Start async handlers for ALL processes        |
|                                                  |
+-------------------------------------------------+
```

**Why tier ordering:**
- Prevents "unknown command" errors when dependent plugins dispatch to
  dependencies during or immediately after startup
- Dependencies are registered in `CommandRegistry` after stage 5, so they
  must fully complete before dependents attempt `dispatch-command` RPCs

**Plugins within the same tier** still use the original barrier model (diagram
above) -- they progress through all 5 stages together. Tier ordering only
serializes across tiers, not within them.

### Shutdown

Engine sends `ze-plugin-callback:bye` with an optional reason:
<!-- source: pkg/plugin/rpc/types.go -- ByeInput -->

```
#99 ze-plugin-callback:bye {"reason":"shutdown"}
#99 ok
```

For internal plugins, the engine then closes the connection (EOF signals exit).
For external plugins, the process is expected to exit cleanly after receiving bye.

#### Who stops the plugin server

**A component does not stop a server it borrowed. Whoever CONSTRUCTS the plugin
server stops it, and nobody else.**

The hub constructs the one plugin server of a normal daemon and stops it at
shutdown. A component that runs inside that server borrows the pointer: the BGP
reactor gets it through `registry.SetPluginServer` -> `registry.GetPluginServer`
-> `Reactor.SetPluginServerAny`, and its own engine runs as a plugin under the
same server. A borrowed server is read, never stopped.
<!-- source: cmd/ze/hub/main.go -- runYANGConfig, pluginserver.NewServer then apiServer.Stop -->
<!-- source: internal/component/bgp/reactor/reactor.go -- Reactor.cleanup, the !externalServer guard -->

Stopping a borrowed server costs twice, and both costs were measured:

| Occasion | What a component stopping its host does |
|----------|------------------------------------------|
| Daemon shutdown | `Server.Stop` calls `ProcessManager.Stop`, which waits for every plugin engine. The engine that made the call cannot return until the call returns, so the stop is bounded only by `pluginStopGrace` plus the group wait: 3.520s per stop, with `resources it installed may be left behind` logged twice, on a daemon that had released everything |
| A reload that removes the component | `runBGPEngine` returns when bgp is removed at reload, not only at shutdown, and its tail stops the reactor. An unguarded stop takes the hub's whole plugin server down, and every other plugin with it, while the daemon keeps running |

A component that CONSTRUCTS its own server still stops it: a standalone reactor
(`Config.Standalone`, the ze-chaos in-process runner) self-hosts, and its cleanup
is that server's only stop. So the rule is a test of ownership, never of the call
site.
<!-- source: internal/component/bgp/reactor/reactor.go -- startAPIServer, the self-host branch -->
<!-- source: internal/chaos/inprocess/runner.go -- the standalone reactor whose cleanup is its server's only stop -->

---

## Runtime RPCs

### Engine to Plugin (Callbacks)

<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- callback method constants, getCallback -->
<!-- source: pkg/plugin/rpc/types.go -- callback input and output types -->

| Method | Input | Response | Description |
|--------|-------|----------|-------------|
| `deliver-event` | `{"event":"<json>"}` | `ok` | Single event |
| `deliver-batch` | `{"events":[...]}` | `ok` | Batched events |
| `execute-command` | `ExecuteCommandInput` | `ExecuteCommandOutput`, or a record answer from a plugin that named `record-answers` | Command execution |
| `config-verify` | `ConfigVerifyInput` | `ConfigVerifyOutput` | Validate candidate config |
| `config-apply` | `ConfigApplyInput` | `ConfigApplyOutput` | Apply config changes |
| `config-rollback` | `{"transaction-id":"..."}` | `ok` | Undo changes for a config transaction |
| `config-operation-decompose` | `ConfigOperationDecomposeInput` | `ConfigOperationDecomposeOutput` | Decompose a config diff into operations |
| `config-operation-verify` | `ConfigOperationVerifyInput` | `ConfigOperationVerifyOutput` | Validate one config operation |
| `config-operation-apply` | `ConfigOperationApplyInput` | `ConfigOperationApplyOutput` | Apply one config operation |
| `config-operation-rollback` | `ConfigOperationRollbackInput` | `ConfigOperationRollbackOutput` | Undo applied config operations |
| `config-operation-commit` | `ConfigOperationCommitInput` | `ConfigOperationCommitOutput` | Commit a config operation transaction |
| `post-startup` | None | `ok` | Report that all plugin startup phases are complete |
| `validate-open` | `ValidateOpenInput` | `ValidateOpenOutput` | Validate an OPEN message |
| `encode-nlri` | `EncodeNLRIInput` | `{"hex":"..."}` | Encode NLRI |
| `decode-nlri` | `DecodeNLRIInput` | `{"json":<raw JSON>}` | Decode NLRI |
| `decode-capability` | `DecodeCapabilityInput` | `{"json":<raw JSON>}` | Decode a capability |
| `bye` | `ByeInput` | `ok` | Shutdown signal |
| `filter-update` | `FilterUpdateInput` | `FilterUpdateOutput` | Route filter request |
| `doctor-check` | `DoctorCheckInput` | `DoctorCheckOutput` | Doctor readiness check |
| `enrich-show` | `EnrichShowInput` | `EnrichShowOutput` | Show command enrichment |

All methods are prefixed with `ze-plugin-callback:`.

**doctor-check:** Engine invokes a plugin's declared doctor check by name.
Plugin runs the check and returns diagnostics (code, severity, message).
Declared during Stage 1 via `doctor-checks` field in `declare-registration`.
Only invoked at runtime via `show doctor`; offline `ze doctor` does not reach plugins.

<!-- source: pkg/plugin/rpc/types.go -- DoctorCheckDecl, DoctorCheckInput, DoctorCheckOutput -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnDoctorCheck -->
<!-- source: internal/component/plugin/server/server.go -- CallDoctorCheck, DoctorCheckPlugins -->
<!-- source: internal/component/doctor/cmd/show.go -- HandleShowDoctor plugin integration -->

**enrich-show:** Engine invokes a plugin's declared show enricher at show time.
Plugin receives the command, key, mode ("detail" or "brief"), and base data map as
JSON, returns enrichment data to merge into the base map. Declared during Stage 1
via `enrichers` field in `declare-registration`. Invoked at runtime when a show
handler calls `show.Enrich()` (mode "detail") or `show.EnrichBrief()` (mode "brief").
2s timeout prevents hung plugins from blocking show commands.

<!-- source: pkg/plugin/rpc/types.go -- EnricherDecl, EnrichShowInput, EnrichShowOutput -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnEnrichShow -->
<!-- source: internal/component/plugin/server/enricher.go -- registerProxyEnrichers, makeProxyCall -->

**filter-update:** Engine sends UPDATE attributes to a named filter.
Plugin responds accept, reject, or modify (delta-only changed attributes).
Includes filter name so the plugin can dispatch to the correct handler.

<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnFilterUpdate -->
<!-- source: internal/component/plugin/server/server.go -- CallFilterUpdate -->
<!-- source: internal/component/bgp/reactor/filter_chain.go -- filter-update caller -->

### Plugin to Engine

<!-- source: pkg/plugin/sdk/sdk_engine.go -- Plugin engine RPC methods -->
<!-- source: pkg/plugin/rpc/types.go -- Method* constants and engine RPC payload types -->

| Method | Input | Output | Description |
|--------|-------|--------|-------------|
| `update-route` | `UpdateRouteInput` | `UpdateRouteOutput` | Inject route to peers |
| `forward-cached` | `ForwardCachedInput` | - | Forward cached UPDATEs to destination peers |
| `release-cached` | `ReleaseCachedInput` | - | Release cached UPDATEs and do not forward them |
| `relay-stored-route` | `RelayStoredRouteInput` | - | Relay stored wire routes to one established peer |
| `route-install` | `RouteInstallInput` | `RouteInstallOutput` | Insert a batch of computed routes into the engine Loc-RIB (forked route-installing plugin) |
| `route-remove` | `RouteRemoveInput` | `RouteRemoveOutput` | Withdraw a batch of routes from the engine Loc-RIB (forked route-installing plugin) |
| `inject-wire-route` | `InjectWireRouteInput` | - | Inject a raw BGP UPDATE body into the RIB |
| `batch-validate` | `BatchValidateInput` | `BatchValidateResult` | Apply a batch of RPKI validation decisions |
| `dispatch-command` | `DispatchCommandInput` | `DispatchCommandOutput` | Inter-plugin command |
| `dispatch-command-args` | `DispatchCommandArgsInput` | `DispatchCommandOutput` | Exact inter-plugin command with pre-tokenized args |
| `emit-event` | `EmitEventInput` | `EmitEventOutput` | Push event to subscribers |
| `subscribe-events` | `SubscribeEventsInput` | - | Subscribe to events |
| `unsubscribe-events` | - | - | Unsubscribe from events |
| `decode-nlri` | `DecodeNLRIInput` | `DecodeNLRIOutput` | Decode NLRI via registry |
| `encode-nlri` | `EncodeNLRIInput` | `EncodeNLRIOutput` | Encode NLRI via registry |
| `decode-mp-reach` | `DecodeMPReachInput` | `DecodeMPReachOutput` | Decode MP_REACH_NLRI |
| `decode-mp-unreach` | `DecodeMPUnreachInput` | `DecodeMPUnreachOutput` | Decode MP_UNREACH_NLRI |
| `decode-update` | `DecodeUpdateInput` | `DecodeUpdateOutput` | Decode full UPDATE |

All methods are prefixed with `ze-plugin-engine:`.

#### Forked route install (`route-install` / `route-remove`)

Route-installing plugins (OSPF, IS-IS) do not program the FIB directly: their SPF
installers insert `locrib.Path` values into the process-wide Loc-RIB singleton
(`locrib.Default()`), which `sysrib` arbitrates and `fibkernel` programs. In a
FORKED (external) plugin subprocess, `locrib.Default()` returns nil (the singleton
lives in the engine's address space), so those installers instead hold a
`routeinstall.Sink` and ship each operation over `route-install` / `route-remove`.
The engine applies the batch to its real Loc-RIB, where `sysrib`'s `OnChange`
programs the kernel exactly as for an in-process installer. Each entry carries the
redistribute protocol **name** (not the numeric `ProtocolID`, which is per-process):
the engine re-resolves it to its own id via `redistevents.RegisterProtocol`.
<!-- source: internal/component/plugin/server/dispatch_route.go -- applyRouteInstall -->
<!-- source: internal/core/rib/routeinstall/sink.go -- Sink -->

---

## Event Delivery

A subscription states what a plugin CAN handle. The peer's configuration
decides what it GETS. A peer-scoped event is delivered when both halves name
it: the plugin subscribed to the type in that direction, and the peer's
`attach process <name>` block grants it. A peer that attaches no block for a
plugin feeds it nothing, whatever the plugin subscribed to.

**Write no plugin that assumes its subscription is enough.** Declare what the
program can act on, and tell the operator which `receive` list the program
needs. ze names each peer, process and event type the two halves disagree
about, at plugin ready and after every config apply, so an operator can see the
gap without reading the program.

An event that is not peer-scoped is untouched by this: the config filter is per
peer.
<!-- source: internal/component/plugin/server/delivery_graph.go -- DeliveryGraph, PeerScopedProcs -->
<!-- source: internal/component/plugin/server/delivery_reconcile.go -- deliveryDisagreements -->

Events that pass both halves are enqueued into a per-process channel.
The delivery goroutine drains all available events into a batch and sends them
in a single `deliver-batch` RPC, reducing syscalls and goroutine churn. Single
events are delivered as a batch of 1.

### Batched Delivery

<!-- source: pkg/plugin/rpc/batch.go -- WriteBatchFrame -->

```
#42 ze-plugin-callback:deliver-batch {"events":["<json-event-1>","<json-event-2>"]}
#42 ok
```

The SDK unpacks the batch and dispatches each event to the `OnEvent` handler individually.
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- callbackDeliverBatch -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- the callbackDeliverBatch handler -->

### Subscription Namespace

`SubscribeEventsInput` carries an optional `namespace` field. Empty (the default,
and the wire-compatible value for every pre-existing caller) resolves to the
namespace registered by the owning protocol component (`bgp` today). A non-empty
value subscribes to another namespace (e.g. `vpn-ipsec`) at startup; an
unregistered namespace is logged and the subscribe block is skipped rather than
registered as a silently-dead `NamespaceUnknown` subscription. The SDK exposes
this via `SetStartupSubscriptionsIn(namespace, events, peers, format)`;
`SetStartupSubscriptions` (namespace `""`) is unchanged.
<!-- source: internal/component/plugin/server/dispatch.go -- registerSubscriptions, resolveSubscriptionNamespace -->

A `"*"` event in a startup subscription expands at registration time into one
subscription per registered event type of the namespace (no wildcard branch on
the per-event match path).

### Enveloped Delivery

`SubscribeEventsInput.envelope` (opt-in, default false; SDK `SetEnvelope(true)`)
wraps each delivered event string in an `EventEnvelope`:

```
{"namespace":"vpn-ipsec","event":"sa-up","payload":<bare payload JSON>}
```

The envelope rides INSIDE the delivered event string, so it is transparent to
both `deliver-event` and `deliver-batch` (both still carry a JSON string). It
lets a plugin subscribed to several event types discriminate which one arrived
even when two events share a payload type (e.g. `sa-up` vs `sa-down`). Without
the opt-in, delivery is byte-identical to before: the bare payload. The engine
renders the envelope at most once per emit and only when a matching subscriber
opted in, preserving the lazy-marshal-once cost of the default path.
<!-- source: internal/component/plugin/server/dispatch.go -- deliverEvent, buildEventEnvelope; pkg/plugin/rpc/types.go -- EventEnvelope -->

### DirectBridge Optimization

For internal plugins with an active `DirectBridge`, `deliverBatch()` calls
`bridge.DeliverEvents(events)` directly instead of `conn.SendDeliverBatch()`,
bypassing RPC envelope construction, newline framing, and pipe I/O. The plugin's
`onEvent` handler is called synchronously in the delivery goroutine.
<!-- source: pkg/plugin/sdk/sdk.go -- Run, bridge activation -->

#### Structured Event Delivery

When a plugin registers `OnStructuredEvent`, the engine delivers `*rpc.StructuredEvent`
instead of formatted text strings. `StructuredEvent` carries pre-extracted peer metadata
(PeerAddress, PeerAS, LocalAS, etc.) and a `RawMessage` pointer for wire message events.
This eliminates the JSON round-trip: the engine skips text formatting, and the plugin
reads data directly from `AttrsWire` (lazy per-attribute parsing) and `WireUpdate`
(zero-copy section access) instead of calling `ParseEvent`.
<!-- source: pkg/plugin/rpc/bridge.go -- StructuredEvent -->

For UPDATE events, `RawMessage` carries `AttrsWire` and `WireUpdate` with lazy accessors.
For state events, `StructuredEvent.State` and `StructuredEvent.Reason` carry the data
directly. For other wire messages (OPEN, NOTIFICATION, REFRESH), `RawMessage.RawBytes`
contains the raw wire bytes.

`StructuredEvent` instances are pooled via `GetStructuredEvent`/`PutStructuredEvent`
to eliminate per-event heap allocations on the hot path.
<!-- source: internal/component/bgp/server/events.go -- getStructuredEvent -->

Plugins that register both `OnStructuredEvent` and `OnEvent` receive structured events
via the former and text events via the latter. The delivery pipeline (`deliverMixedBatch`)
routes each event to the appropriate handler based on whether `Event` or `Output` is set.
<!-- source: internal/component/plugin/process/delivery.go -- deliverMixedBatch -->

### Text Event Formatting: the scratch discipline

A plugin without a structured handler receives text, and the engine builds that
text on the delivery path. Four rules keep the path free of allocations, and the
comments in `internal/component/bgp/server/events.go` name this section rather
than restate them.

1. **No format strings.** No `fmt.Sprintf`, no `fmt.Fprintf`, no `fmt.Appendf`.
   Reflection defeats escape analysis even when the output size is trivial. Use
   `strconv.AppendUint`, `netip.Addr.AppendTo`, `hex.AppendEncode`, and
   `append(buf, "literal"...)`.
2. **No intermediate string lists.** Never build a `[]string` and `strings.Join`
   it. Write the element, write the separator byte, repeat. No `strings.Builder`,
   no `strings.ReplaceAll`, no `strings.Replacer`.
3. **One `string(scratch)` per named boundary.** Every other code path stays on
   `[]byte`. In the event path the boundary is the per-encoding format cache and
   the RPC payload; each is one conversion per distinct encoding, not one per
   subscriber.
4. **Scratch is stack-local to the outer caller.** `var scratchArr [N]byte`
   lives on the goroutine stack of the event function that owns the loop. It is
   never a struct field, never a `sync.Pool`, and never per-peer. Output larger
   than `N` spills to the heap through `append` growth for that one call, which
   is correct and costs one allocation in the pathological case: the array size
   is chosen to cover the realistic maximum, not every input.

The formatters this path calls take the `AppendXxx(buf []byte, ...) []byte`
shape, so they append into the caller's scratch and never allocate a string of
their own.
<!-- source: internal/component/bgp/server/events.go -- formatMessageForSubscription, onPeerStateChange, onEORReceived, onPeerCongestionChange -->

### Event Subscription

Plugins subscribe to events using either:

1. **Startup subscription** (recommended): included in the `ready` RPC so the engine
   registers atomically before `SignalAPIReady`, avoiding the race between the reactor
   sending routes and the plugin subscribing.
   <!-- source: pkg/plugin/rpc/types.go -- ReadyInput -->

2. **Runtime subscription**: via `subscribe-events` RPC in `OnStarted` callback.
   Safe but has a small window where events could be missed.
   <!-- source: pkg/plugin/sdk/sdk_engine.go -- SubscribeEvents -->

Neither plugin subscription form nor the operator's `request subscribe` can
widen what a peer grants. The operator command can add to the process's live
capability within that grant, and the next config apply discards the addition.
<!-- source: internal/component/plugin/server/delivery_graph.go -- (*Server).PeerScopedProcs, (*Server).DiscardRuntimeSubscriptions -->

A session that reaches Established before a plugin registers its subscription
raises its state event into an empty list, and nothing replays it. This is why
the startup form is recommended: it registers before `SignalAPIReady`, and ze
starts its peers after every plugin tier.

**Cross-Plugin DispatchCommand from Startup.** A plugin whose startup logic
must call `DispatchCommand` on a sibling plugin's command (e.g., `bgp-rpki`
enabling the `request bgp adj-rib-in enable-validation` gate) MUST register the call via
`OnAllPluginsReady`, not `OnStarted`. `OnStarted` fires after the plugin's own
5-stage handshake but potentially BEFORE other plugins in later startup phases
are loaded, so the dispatcher may not yet know about the target command.
`OnAllPluginsReady` fires via the event loop once the engine has frozen every
registry after every phase completes, so the dispatch is guaranteed to resolve.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go — OnStarted vs OnAllPluginsReady -->

### Non-CIDR Families in the Filter Text Protocol

The engine's filter text protocol (`FilterUpdateInput.Update`) inlines NLRI
prefixes only for address families whose wire encoding is a plain CIDR
prefix. These are the "CIDR-family" set: IPv4 and IPv6 for the SAFIs
`unicast`, `multicast`, and `mpls-label`. Every other family -- EVPN,
Flowspec, VPN, BGP-LS, MVPN, MUP, RTC, and any future family with a
specialised NLRI encoding -- is classified non-CIDR.

For non-CIDR families the engine emits a marker block of the form
`nlri <family> <op>` (for example `nlri l2vpn/evpn add`) with NO prefixes.
The marker tells a text-mode filter plugin that the family is present in
the update without forcing the engine to generate a family-specific text
format.
<!-- source: internal/component/bgp/reactor/filter_format.go — AppendUpdateForFilter, appendNLRIBlock -->
<!-- source: internal/component/bgp/reactor/filter_format.go — isCIDRFamily -->

A filter plugin that needs per-NLRI decisions on a non-CIDR family MUST
declare `raw=true` in its `FilterRegistration` and parse the wire payload
itself from `FilterUpdateInput.Raw`. A `raw=false` filter attached to a
session carrying non-CIDR families sees only the marker block and is
advisory for those families -- it cannot distinguish individual
destinations within the family.
<!-- source: internal/component/plugin/registration.go — FilterRegistration.Raw -->

| Family set | Filter text protocol emits | Filter requirement |
|------------|----------------------------|---------------------|
| CIDR (ipv4/ipv6 unicast / multicast / mpls-label) | `nlri <family> <op> <prefix>...` (prefixes inline) | `raw=false` works for text-mode per-prefix decisions |
| Non-CIDR (EVPN, Flowspec, VPN, BGP-LS, MVPN, MUP, RTC, ...) | `nlri <family> <op>` (marker only) | `raw=true` required for per-NLRI decisions; text-only is advisory |

**Subscription fields:**
<!-- source: pkg/plugin/rpc/types.go -- SubscribeEventsInput -->

| Field | Type | Description |
|-------|------|-------------|
| `events` | `[]string` | Event types (e.g., `["update","state"]`); `"*"` expands to all event types of the namespace |
| `peers` | `[]string` | Peer filter (e.g., `["*"]` for all) |
| `format` | `string` | Format preference (e.g., `"json"`) |
| `encoding` | `string` | `"json"` (default) or `"text"` |
| `namespace` | `string` | Event namespace; empty resolves to the protocol component's default (`bgp`), non-empty (e.g. `vpn-ipsec`) subscribes to another namespace (see Subscription Namespace) |
| `envelope` | `bool` | When true, deliveries are wrapped in an `EventEnvelope` (see Enveloped Delivery); default false = bare payload |

---

## Internal Plugin Invocation Modes

Ze plugins run as **long-lived processes** (goroutines for Go, subprocesses for external).
Each plugin registers the families it handles at startup, then processes requests in a loop.

### Architecture Overview

```
+--------------------------------------------------------------------------+
|                              ENGINE                                       |
|                                                                           |
|   +------------------------------------------------------------------+   |
|   |                        Family Registry                            |   |
|   |   ipv4/flowspec     -> flowspec plugin                            |   |
|   |   ipv6/flowspec     -> flowspec plugin                            |   |
|   |   ipv4/flowspec-vpn -> flowspec plugin                            |   |
|   |   ipv6/flowspec-vpn -> flowspec plugin                            |   |
|   +------------------------------------------------------------------+   |
|                                    |                                      |
|                        RPC (MuxConn / DirectBridge)                       |
|                                    |                                      |
|   +------------------------------------------------------------------+   |
|   |       FLOWSPEC PLUGIN (long-lived goroutine / process)            |   |
|   |                                                                    |   |
|   |  1. 5-stage startup (YANG RPCs)                                   |   |
|   |  2. Event loop (encode/decode callbacks)                          |   |
|   +------------------------------------------------------------------+   |
+--------------------------------------------------------------------------+
```

### Automatic OPEN Capability Injection

**Key design:** When a plugin declares `decode` for a family, the engine automatically
advertises that family in OPEN messages via Multiprotocol capability (Code 1).

**Rationale:**
- If a plugin can decode a family, peers should be able to send it
- No explicit capability declaration needed for Multiprotocol
- Reduces protocol overhead and prevents duplicate capability issues

**How it works:**

```
Plugin Stage 1: declare-registration with family ipv4/flow mode=decode
                     |
Registry: families["ipv4/flow"] = "flowspec"
                     |
Session.sendOpen(): GetDecodeFamilies() -> ["ipv4/flow", ...]
                     |
OPEN: Multiprotocol(AFI=1, SAFI=133)
```

**Override behavior:** Config families completely override plugin families:
- Config has `family {}` block: ONLY config families used, plugin families ignored
- Config has NO `family {}` block: plugin decode families used

This is intentional: explicit config = full control. Plugin families provide defaults
when config doesn't specify families.

**Auto-loading plugins:** When a family is configured but no plugin has claimed it,
the engine automatically loads the internal plugin for that family (if one exists).

**Five-phase plugin startup:**
1. **Phase 1:** Config-path plugins start first (for example, BGP, interface, and FIB infrastructure plugins)
2. **Phase 2:** Explicit plugins from `plugin { external ... }` start after config-path infrastructure is available
3. **Phase 3:** The engine checks which configured families are still unclaimed. Internal plugins are auto-loaded only for unclaimed families.
4. **Phase 4:** The engine checks which custom event types are referenced in peer `receive` config but not produced by any running plugin. Producing plugins and their transitive dependencies are auto-loaded. For example, `receive [ update-rpki ]` auto-loads `bgp-rpki-decorator` and its dependency `bgp-rpki`.
5. **Phase 5:** The engine checks which custom send types are referenced in peer `send` config but not enabled by any running plugin. Enabling plugins and their transitive dependencies are auto-loaded. For example, `send [ enhanced-refresh ]` auto-loads `bgp-route-refresh`.

**Family auto-loading** (Phase 3) is **prevented** when:
1. An explicit plugin declares `decode` for the family (family-based check)
2. `--plugin <name>` is passed on command line (prevents auto-load for that plugin)

The check is based on **family claims**, not plugin name. Plugin names are informational only.

| Config | Plugin | Result |
|--------|--------|--------|
| `family { ipv4/flow; }` | None | Auto-loads `bgp-nlri-flowspec` |
| `family { ipv4/flow; }` | `--plugin bgp-nlri-flowspec` | Uses explicit plugin (no auto-load) |
| `family { ipv4/flow; }` | `plugin { external my-traffic { declares ipv4/flow } }` | Uses config plugin (no auto-load, family claimed) |
| `family { ipv4/foo; }` | None | Startup fails (no plugin for family) |

**Event type auto-loading** (Phase 3) triggers when a peer process has `receive [ <custom-type> ]` and no running plugin produces that event type. The producing plugin is found via `registry.PluginForEventType()` which matches against `Registration.EventTypes`. Dependencies are resolved transitively.

| Config | Plugin | Result |
|--------|--------|--------|
| `receive [ update-rpki ]` | None | Auto-loads `bgp-rpki-decorator` + dependency `bgp-rpki` |
| `receive [ update-rpki ]` | `plugin { external rpki-decorator { ... } }` | Uses explicit plugin (no auto-load) |

**Send type auto-loading** (Phase 4) triggers when a peer process has `send [ <custom-type> ]` and no running plugin enables that send type. The enabling plugin is found via `registry.PluginForSendType()` which matches against `Registration.SendTypes`. Dependencies are resolved transitively.

| Config | Plugin | Result |
|--------|--------|--------|
| `send [ enhanced-refresh ]` | None | Auto-loads `bgp-route-refresh` |
| `send [ enhanced-refresh ]` | `plugin { external route-refresh { ... } }` | Uses explicit plugin (no auto-load) |

**Functional tests:**
- `test/plugin/flowspec-open-capability.ci` - auto-load for known family
- `test/plugin/family-no-plugin-failure.ci` - failure for unknown family
- `test/plugin/explicit-plugin-precedence.ci` - explicit `--plugin` prevents auto-load
- `test/plugin/explicit-plugin-config.ci` - config plugin prevents auto-load
- `test/plugin/rpki-decorator-autoload.ci` - auto-load for custom event type
- `test/parse/send-enhanced-refresh.ci` - dynamic send type accepted in config
- `test/parse/send-unknown-rejected.ci` - unregistered send type rejected

**Ordering:** Plugin families are sorted alphabetically for deterministic OPEN messages.

**What plugins should NOT do:**
- Send `declare-capabilities` with Multiprotocol (Code 1) for their families
- Assume plugin families will be used if config has a `family {}` block

**What plugins SHOULD do:**
- Declare `decode` for all families they can parse (provides defaults)
- Use `declare-capabilities` only for non-Multiprotocol capabilities (GR, hostname, etc.)

### NLRI Routing via Engine RPCs

NLRI encode/decode requests are routed via the engine's plugin registry:
<!-- source: pkg/plugin/sdk/sdk_engine.go -- EncodeNLRI, DecodeNLRI -->

| Direction | RPC Method | Input | Output |
|-----------|-----------|-------|--------|
| Plugin to Engine | `ze-plugin-engine:encode-nlri` | `{"family":"...","args":[...]}` | `{"hex":"..."}` |
| Plugin to Engine | `ze-plugin-engine:decode-nlri` | `{"family":"...","hex":"..."}` | `{"json":<raw JSON>}` |
| Engine to Plugin | `ze-plugin-callback:encode-nlri` | `{"family":"...","args":[...]}` | `{"hex":"..."}` |
| Engine to Plugin | `ze-plugin-callback:decode-nlri` | `{"family":"...","hex":"..."}` | `{"json":<raw JSON>}` |

**How it works:**
1. Plugin calls `EncodeNLRI`/`DecodeNLRI` via engine RPC
2. Engine looks up the family plugin via `registry.LookupFamily()`
3. Engine sends callback to the appropriate family plugin
4. Family plugin processes and returns result

For in-process plugins with `DirectBridge`, the RPC path is replaced by direct
function calls, bypassing JSON marshaling and pipe I/O entirely.

### Mode 1: In-Process (goroutine + net.Pipe + DirectBridge)

For Go plugins (`ze.pluginname`) -- runs in same process:
<!-- source: pkg/plugin/sdk/sdk.go -- NewWithConn, Run bridge activation -->

1. `startInternal()` creates a single `net.Pipe` for bidirectional YANG RPC
2. Creates a `DirectBridge` and wraps the plugin-side connection in `BridgedConn`
3. Runner goroutine receives `BridgedConn` (implements `net.Conn`) transparently
4. SDK discovers bridge via `Bridger` type assertion in `NewWithConn()`
5. 5-stage startup runs over sockets (cold path, 5 round-trips total)
6. After Stage 5: bridge activates for direct function calls (hot path)

**Bridge activation sequence:**

| Step | Side | Action |
|------|------|--------|
| 1 | Engine | `wireBridgeDispatch()` registers `DispatchRPC` handler on bridge |
| 2 | Engine | Sends Stage 5 OK response over pipe (last pipe message) |
| 3 | Engine | If `ReadyInput.Transport == "bridge"`: calls `conn.SetBridge(bridge)` |
| 4 | SDK | Receives OK, registers `DeliverEvents` handler on bridge |
| 5 | SDK | Calls `bridge.SetReady()` -- bridge now active |
| 6 | SDK | Closes pipe (`engineMux.Close()`), enters `bridgeEventLoop` |

The engine wires its handler (step 1) before sending OK (step 2), ensuring no race
between SDK bridge activation and engine readiness. After bridge activation, the pipe
is fully shut down -- the MuxConn readLoop exits, and all engine-to-plugin callbacks
flow through `bridge.CallbackCh()`.
<!-- source: pkg/plugin/sdk/sdk.go -- Run, bridge activation -->
<!-- source: internal/component/plugin/server/startup.go -- handleProcessStartupRPC -->
<!-- source: internal/component/plugin/ipc/rpc.go -- SetBridge -->

**Engine-side dispatch registry:** all three transports for a plugin-to-engine RPC
(the socket JSON path, the in-process Direct path, and the typed `DirectBridge`
fast-path slot) derive from a single method registry -- one `engineOp` entry per
operation carrying the `rpc.Method*` wire string, a `proc`-passed handler shared by
the JSON and Direct paths, and an optional typed-slot descriptor. `wireBridgeDispatch`
installs the typed slots by iterating the entries that declare a descriptor (not a
hand-written `Set*` list), and `dispatchPluginRPC` / `dispatchPluginRPCDirect` resolve
the method through the same table, so adding an operation touches one place and the
JSON / Direct / bridge paths cannot drift. The `rpc.Method*` constants are shared with
the SDK caller so the sent and dispatched method strings stay in lockstep.
<!-- source: internal/component/plugin/server/dispatch_registry.go -- engineOps, engineOp, lookupEngineOp -->
<!-- source: internal/component/plugin/server/dispatch.go -- dispatchPluginRPC, dispatchPluginRPCDirect, wireBridgeDispatch -->

**Runtime hot path (after bridge activates):**

| Direction | Socket path (before) | Direct path (after) |
|-----------|---------------------|---------------------|
| Engine to Plugin events (text) | RPC envelope -> newline frame -> `net.Pipe.Write` -> read -> unmarshal -> `onEvent` | `bridge.DeliverEvents(events)` -> `onEvent` directly |
| Engine to Plugin events (structured) | -- | `bridge.DeliverStructured([]any)` -> `onStructuredEvent` with `*StructuredEvent` (no text formatting, no JSON parsing) |
| Plugin to Engine RPCs (generic) | `json.Marshal` -> newline frame -> `net.Pipe.Write` -> read -> unmarshal -> `dispatcher.Dispatch` | `bridge.DispatchRPC(method, params)` -> `dispatcher.Dispatch` directly |
| Plugin to Engine dispatch-command | JSON marshal `DispatchCommandInput` -> RPC -> unmarshal -> dispatch | `bridge.DispatchCommand(command)` -> `dispatchCommand()` directly (struct passthrough, no serialization) |
| Plugin to Engine dispatch-command-args | JSON marshal `DispatchCommandArgsInput` -> RPC -> unmarshal -> exact command route | `bridge.DispatchCommandArgs(command, args, peer)` -> `dispatchCommandArgs()` directly (Go args slice, no tokenizer) |
| Plugin to Engine emit-event | JSON marshal `EmitEventInput` -> RPC -> unmarshal -> deliver | `bridge.EmitEvent(namespace, eventType, ...)` -> `deliverEvent()` directly (Go strings, no JSON) |
| Engine to Plugin callbacks | MuxConn RPC + 3-way select | `bridge.SendCallback()` -> callback channel -> `bridgeEventLoop` 2-way select |

**Callback dispatch:** Both event loops (pipe and bridge) dispatch through a generic
callback registry (`map[string]callbackHandler`). Each `On*` method registers a typed
wrapper in the map. Adding a new callback requires only one `On*` method -- zero changes
to the dispatch or event loop code. See `rules/plugin-design.md` "SDK Is Generic".
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- eventLoop, bridgeEventLoop, getCallback -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- initCallbackDefaults, On* methods -->

**Shutdown and callback failure:** `Process.Stop()` cancels the context and calls
`bridge.CloseCallbacks()` (guarded by `sync.Once`), closing callback channels. The
`bridgeEventLoop` exits on channel close. `SendCallback` recovers from send-on-closed-channel
panics and returns `ErrBridgeClosed`. If a DirectBridge callback panics, the SDK sends
an `ErrBridgeFailed`-wrapped error to the waiting caller, marks callbacks failed, closes
callback channels, and later `SendCallback` / `ExecuteCommand` calls fail fast.
<!-- source: internal/component/plugin/process/process.go -- Stop -->
<!-- source: pkg/plugin/rpc/bridge.go -- SendCallback, CloseCallbacks, FailCallbacks, ErrBridgeClosed, ErrBridgeFailed -->
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- bridgeEventLoop panic recovery -->

**Files:**

| File | Purpose |
|------|---------|
| `pkg/plugin/rpc/bridge.go` | `DirectBridge`, `BridgedConn`, `Bridger`, `BridgeCallback`, `SendCallback`, `CloseCallbacks` |
| `internal/component/plugin/server/dispatch_registry.go` | `engineOp`, `engineOps` (the unified plugin-to-engine method registry), `serveEngineOpJSON`, `serveEngineOpDirect` |
| `pkg/plugin/sdk/sdk_callbacks.go` | `initCallbackDefaults`, `On*` wrappers, `callbackHandler` registry |
| `pkg/plugin/sdk/sdk_dispatch.go` | `eventLoop`, `bridgeEventLoop`, `getCallback` -- generic dispatch |
| `internal/component/plugin/process/process.go` | Bridge creation in `startInternal()`, bridge check in `deliverBatch()`, `CloseCallbacks` in `Stop()` |
| `internal/component/plugin/ipc/rpc.go` | `PluginConn.SetBridge()`, `CallRPC` bridge routing |
| `internal/component/plugin/server/startup.go` | Bridge transport activation after Stage 5 OK |
| `pkg/plugin/sdk/sdk.go` | Bridge discovery, `callEngineRaw()` bridge path, `SetReady()`, pipe close |

### Mode 2: Subprocess (TLS connect-back)

For external plugins (Python, Rust, etc.) -- runs as separate process:
<!-- source: pkg/plugin/sdk/sdk.go -- NewFromTLSEnv -->

1. Engine starts TLS listener from `plugin { hub { server <name> { ip ...; port ...; secret ...; } } }` config
2. Engine forks child with env vars: `ZE_PLUGIN_HUB_HOST`, `ZE_PLUGIN_HUB_PORT`, `ZE_PLUGIN_HUB_TOKEN` (per-plugin unique token), `ZE_PLUGIN_CERT_FP` (server cert SHA-256 fingerprint), `ZE_PLUGIN_NAME`
3. Child verifies server cert fingerprint during TLS handshake, authenticates with `#0 auth {"token":"...","name":"..."}`
4. Engine validates token matches the per-plugin token generated for that name (name binding prevents impersonation)
5. Token is cleared from the child's OS environment after first read (`Secret: true` registration)
6. Single bidirectional connection using `MuxConn` (responses routed by `#id`, requests via `Requests()` channel)
7. No `DirectBridge` -- always uses newline-framed RPC over TLS
8. Same 5-stage handshake over the same connection
<!-- source: internal/component/plugin/process/process.go -- startExternal env var setup -->
<!-- source: internal/component/plugin/ipc/tls.go -- TokenForPlugin, CertFingerprint, combinedLookup -->

### Benefits of Long-Lived Design

| Benefit | Description |
|---------|-------------|
| No per-request overhead | Plugin starts once, handles many requests |
| Language agnostic | Same protocol for Go/Python/Rust |
| Hot-swappable | Restart plugin without engine restart |
| Testable | Plugin protocol can be tested independently |
| Internal optimization | In-process plugins bypass transport overhead via DirectBridge |

---

## Family Plugin NLRI System

Family plugins provide NLRI encoding/decoding for address families that require complex parsing
(FlowSpec, EVPN, BGP-LS, VPN). This section details the complete protocol.

### Family Registration (Stage 1)

Plugins declare which families they handle via the `families` field in
`DeclareRegistrationInput`. Each declaration carries both the canonical name
AND the RFC 4760 wire-format AFI/SAFI numbers, so the engine can register the
family in `internal/core/family/` (the cross-component family registry) at
runtime alongside families registered by internal plugins at init.
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput, FamilyDecl -->

```json
{
  "families": [
    {"name": "ipv4/flow",     "mode": "encode", "afi": 1, "safi": 133},
    {"name": "ipv4/flow",     "mode": "decode", "afi": 1, "safi": 133},
    {"name": "ipv6/flow",     "mode": "encode", "afi": 2, "safi": 133},
    {"name": "ipv6/flow",     "mode": "decode", "afi": 2, "safi": 133},
    {"name": "ipv4/flow-vpn", "mode": "both",   "afi": 1, "safi": 134},
    {"name": "ipv6/flow-vpn", "mode": "both",   "afi": 2, "safi": 134}
  ]
}
```

**FamilyDecl fields:**

| Field | Values | Description |
|-------|--------|-------------|
| `name` | `"ipv4/flow"`, `"l2vpn/evpn"`, etc. | Address family (`afi/safi` canonical form) |
| `mode` | `"encode"`, `"decode"`, `"both"` | Direction of conversion |
| `afi` | RFC 4760 AFI number (e.g., `1` = IPv4, `2` = IPv6, `25` = L2VPN, `16388` = BGP-LS) | Required for runtime registration |
| `safi` | RFC 4760 SAFI number (e.g., `1` = unicast, `133` = FlowSpec) | Required for runtime registration |

**Runtime family registration:**
<!-- source: internal/component/plugin/server/startup.go -- registerPluginFamilies -->

After the plugin completes Stage 1, the engine calls `registerPluginFamilies`,
which validates the whole `FamilyDecl` batch and commits it through
`family.RegisterFamilyBatch`. This is the runtime equivalent of internal plugins
calling `family.MustRegister` at init. The server records the families actually
added for that plugin and removes them if a later startup stage fails before the
plugin reaches ready. After committed startup, `Family.String()` and
`family.LookupFamily()` return the plugin's family. Re-registration with
identical values is a no-op; conflicting AFI or SAFI names abort plugin startup.

**Registry conflict detection:**
- Only ONE plugin can register for a family+mode combination
- Conflict results in startup error

**OPEN capability injection (decode mode):**
- Families declared with `decode` are automatically advertised in OPEN
- Engine adds Multiprotocol capability (Code 1) for each decode family
- No explicit `declare-capabilities` needed from plugins for Multiprotocol

### Encode/Decode Protocol

**Engine to Plugin (callback):**

| RPC | Input | Output |
|-----|-------|--------|
| `ze-plugin-callback:encode-nlri` | `{"family":"ipv4/flow","args":["destination","10.0.0.0/24"]}` | `{"hex":"0701180A0000"}` |
| `ze-plugin-callback:decode-nlri` | `{"family":"ipv4/flow","hex":"0701180A0000"}` | `{"json":{"destination":...}}` |

**Plugin to Engine (via registry):**

| RPC | Input | Output |
|-----|-------|--------|
| `ze-plugin-engine:encode-nlri` | `{"family":"ipv4/flow","args":["destination","10.0.0.0/24"]}` | `{"hex":"0701180A0000"}` |
| `ze-plugin-engine:decode-nlri` | `{"family":"ipv4/flow","hex":"0701180A0000"}` | `{"json":{"destination":...}}` |

### Additional Decode RPCs (Plugin to Engine)

These RPCs allow plugins to request full UPDATE or MP attribute decoding:
<!-- source: pkg/plugin/rpc/types.go -- DecodeMPReachInput, DecodeMPUnreachInput, DecodeUpdateInput -->

| RPC | Input | Output |
|-----|-------|--------|
| `ze-plugin-engine:decode-mp-reach` | `{"hex":"...","add-path":false}` | `{"family":"...","next-hop":"...","nlri":[...]}` |
| `ze-plugin-engine:decode-mp-unreach` | `{"hex":"...","add-path":false}` | `{"family":"...","nlri":[...]}` |
| `ze-plugin-engine:decode-update` | `{"hex":"...","add-path":false}` | `{"json":"..."}` |

### Error Handling

| Error Type | Response |
|------------|----------|
| Invalid family | `#<id> error {"code":"error","message":"unknown family: ipv4/unknown"}` |
| Parse error (encode) | `#<id> error {"code":"error","message":"invalid prefix: 10.0.0/24"}` |
| Cannot decode | `#<id> error {"code":"error","message":"..."}` |
| Handler not registered | `#<id> error {"code":"error","message":"encode-nlri not supported"}` |

### Files

| File | Purpose |
|------|---------|
| `internal/component/plugin/registration.go` | Family registry, conflict detection |
| `internal/component/plugin/server/events.go` | NLRI routing |
| `pkg/plugin/rpc/types.go` | RPC input/output types |
| `pkg/plugin/sdk/sdk_engine.go` | SDK encode/decode methods |
| `pkg/plugin/sdk/sdk_dispatch.go` | SDK encode/decode callback handlers |

---

## Command Execution

Plugins register commands in Stage 1 via the `commands` field of `DeclareRegistrationInput`.
At runtime, the engine dispatches commands to plugins via `execute-command`:
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandInput, ExecuteCommandOutput -->

**Engine to Plugin:**
```
#5 ze-plugin-callback:execute-command {"serial":"abc","command":"rib adjacent status","args":[],"peer":"*"}
```

**Plugin to Engine, a plugin that named nothing at Stage 3:**
```
#5 ok {"status":"done","data":{"running":true,"peers":1}}
#5 ok {"status":"error","data":"component not found"}
```

Command execution results are sent as `ok` responses with a `status` field.
They are not sent as `error` responses, because the RPC itself succeeded even
when the command failed.

**Plugin to Engine, a plugin that named `record-answers` at Stage 3:** the
answer is a head, its records and a terminator. Every plugin built on the Go SDK
is in this group. The frame follows that declaration and never the payload. A
handler that built one value therefore takes the same three lines as a handler
that walked a table.

A handler that built one value writes it as the one record of a `type=json`
answer. The `data` member above is what that record carries, byte for byte. The
handler's status moves to the head:

```
#5 ok status=done type=json
#5 ok item={"running":true,"peers":1}
#5 ok count=1
```

A handler that answered with a `plugin.Records` walk of more than 256 rows
writes one line for each row. A walk over a large table therefore never becomes
one 16 MB line. A shorter walk collapses to the `type=json` document above:

```
#5 ok status=done type=ndjson key=peers
#5 ok item={"address":"10.0.0.1","state":"established"}
#5 ok count=1
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- executeCommandAnswer -->
<!-- source: pkg/plugin/records.go -- Records, Records.WriteAnswer -->
<!-- source: internal/component/plugin/ipc/rpc.go -- PluginConn.SendExecuteCommandAnswer, ExecuteCommandValue -->
<!-- source: internal/component/plugin/server/command.go -- routeToProcess, pluginAnswerRows -->

### Inter-Plugin Communication

Plugins can dispatch commands to other plugins via the external-compatible string API:
<!-- source: pkg/plugin/sdk/sdk_engine.go -- DispatchCommand -->

```
#4 ze-plugin-engine:dispatch-command {"command":"rib adjacent inbound show"}
#4 ok {"status":"done","data":{...}}
```

Internal plugins that already know the exact registered command should use the
typed args API. It carries the command name, pre-tokenized arguments, and optional
peer selector separately, so runtime values are not split by the command tokenizer.
<!-- source: pkg/plugin/rpc/types.go -- DispatchCommandArgsInput -->
<!-- source: internal/component/plugin/server/dispatch.go -- dispatchCommandArgs -->

```
#4 ze-plugin-engine:dispatch-command-args {"command":"request bgp adj-rib-in replay","args":["peer key with spaces","0"],"peer":"*"}
#4 ok {"status":"done","data":{"last-index":7,"replayed":3}}
```

Both APIs return `DispatchCommandOutput` to a peer that negotiated nothing. The
`data` field carries raw JSON (single-decode). On error, the response uses a
separate `error` field: `{"status":"error","error":"message"}`.
<!-- source: pkg/plugin/rpc/types.go -- DispatchCommandOutput -->

A peer that named `record-answers` at Stage 3 receives the same two answers as a
head, records, and a terminator instead. The document a bounded answer carries in
its one `item=` equals the `data` the line above carries. See
[ipc_protocol.md](ipc_protocol.md), "Answer Protocol".
<!-- source: internal/component/plugin/server/dispatch_registry.go -- answerResult, recordAnswer -->
<!-- source: internal/component/plugin/dispatch.go -- WriteAnswer -->

The string API routes by normal dispatcher parsing and remains the compatibility
surface for CLI and external plugin callers. The typed args API routes by exact
registered plugin command and sends the existing `execute-command` callback to
the target plugin with `args []string`.
<!-- source: internal/component/plugin/server/command.go -- ForwardToPlugin -->
---

## Config Reload Protocol

Config reload uses a two-phase verify/apply pattern:
<!-- source: pkg/plugin/rpc/types.go -- ConfigVerifyInput, ConfigApplyInput -->

**Phase 1: Verify** -- engine sends candidate config to all plugins for validation:

```
#10 ze-plugin-callback:config-verify {"sections":[{"root":"bgp","data":"{...}"}]}
#10 ok {"status":"ok"}
```

If any plugin rejects, the reload is aborted.

**Phase 2: Apply** -- engine sends config diffs to all plugins:

```
#11 ze-plugin-callback:config-apply {"sections":[{"root":"bgp","added":"{...}","removed":"","changed":""}]}
#11 ok {"status":"ok"}
```

**ConfigDiffSection fields:**
<!-- source: pkg/plugin/rpc/types.go -- ConfigDiffSection -->

| Field | Type | Description |
|-------|------|-------------|
| `root` | `string` | Config root name |
| `added` | `string` | JSON-encoded added config |
| `removed` | `string` | JSON-encoded removed config |
| `changed` | `string` | JSON-encoded changed config |

---

## OPEN Validation

Plugins can validate incoming OPEN messages by registering `OnValidateOpen`.
The engine sends both local and remote OPENs for inspection:
<!-- source: pkg/plugin/rpc/types.go -- ValidateOpenInput, ValidateOpenOutput -->

```
#7 ze-plugin-callback:validate-open {"peer":"192.168.1.1","local":{"asn":65001,"router-id":"1.1.1.1","hold-time":90,"capabilities":[...]},"remote":{"asn":65002,...}}
#7 ok {"accept":true}
```

`peer` is the peer's configured name. `group` is its enclosing group, and the
field is omitted for a peer that stands alone:

```
#8 ze-plugin-callback:validate-open {"peer":"dyn-192.0.2.7","group":"ix","local":{...},"remote":{...}}
```

A peer created from a dynamic group's template has only that second identity. The
engine builds such a peer when a connection arrives inside the group's range, and
names it `dyn-<addr>`. Neither that name nor its address is in the config the
plugin read. A plugin that keys per-peer policy on the config resolves that peer
through `group` or resolves nothing for it.

To reject:
```
#7 ok {"accept":false,"notify-code":2,"notify-subcode":6,"reason":"unacceptable hold time"}
```

---

## Plugin Transport

### Internal Plugins (goroutine)

Transport: `net.Pipe()` for 5-stage startup, then `DirectBridge` for hot path.
No network, no TLS, no auth. Fastest path.

### External Plugins (TLS connect-back)

Transport: single TLS connection per plugin.
<!-- source: pkg/plugin/sdk/sdk.go -- NewFromTLSEnv -->

1. Engine reads `plugin { hub { server <name> { ip ...; port ...; secret ...; } } }` from config
2. Engine starts TLS listener(s) (one per `server` entry), creates `PluginAcceptor` with cert fingerprint
3. Engine generates per-plugin token, forks child with `ZE_PLUGIN_HUB_HOST`, `ZE_PLUGIN_HUB_PORT`, `ZE_PLUGIN_HUB_TOKEN` (unique per plugin), `ZE_PLUGIN_CERT_FP`, `ZE_PLUGIN_NAME` env vars
4. Child verifies server cert fingerprint, connects via TLS, sends `#0 auth {"token":"...","name":"..."}`
5. Engine authenticates: per-plugin token lookup by name (constant-time comparison), name binding enforced
6. Token cleared from child OS environment after first read
7. Single `MuxConn` handles bidirectional RPC (responses by `#id`, requests via `Requests()` channel)
8. Standard 5-stage handshake proceeds over the same connection
<!-- source: internal/component/plugin/ipc/tls.go -- combinedLookup, AuthenticateWithLookup -->

### Config

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `plugin.hub.server` | named list | -- | TLS listener entries (keyed by name) |
| `plugin.hub.server.<name>.ip` | string | `127.0.0.1` | Bind address |
| `plugin.hub.server.<name>.port` | uint16 | `12700` | Bind port |
| `plugin.hub.server.<name>.secret` | string | (required, min 32 chars) | Auth token |

### Files

| File | Purpose |
|------|---------|
| `internal/component/plugin/ipc/tls.go` | TLS listener, auth, cert gen, PluginAcceptor |
| `internal/component/plugin/ipc/rpc.go` | PluginConn with MuxConn |
| `internal/component/plugin/process/process.go` | startInternal, startExternal, InitConns |
| `pkg/plugin/sdk/sdk.go` | NewFromTLSEnv, NewWithConn |

---

## Plugin Examples

### Capability-Only Plugin (e.g., GR)

The GR plugin only participates in startup -- it injects GR capabilities into OPEN messages.
No event subscription needed because it doesn't need runtime events.

```
Ze Engine                              GR Plugin
----------                             ---------

STAGE 1: REGISTRATION
                       <--- #1 ze-plugin-engine:declare-registration
                            {"wants-config":["bgp"]}
#1 ok                  --->

STAGE 2: CONFIG DELIVERY
#1 ze-plugin-callback:configure
  {"sections":[{"root":"bgp","data":"{...}"}]}  --->
                       <--- #1 ok

STAGE 3: CAPABILITY DECLARATION
                       <--- #2 ze-plugin-engine:declare-capabilities
                            {"capabilities":[
                              {"code":64,"encoding":"hex","payload":"0078",
                               "peers":["192.168.1.1"]},
                              {"code":64,"encoding":"hex","payload":"005a",
                               "peers":["10.0.0.1"]}
                            ]}
#2 ok                  --->

STAGE 4: REGISTRY SHARING
#2 ze-plugin-callback:share-registry {"commands":[...]}  --->
                       <--- #2 ok

STAGE 5: READY
                       <--- #3 ze-plugin-engine:ready
#3 ok                  --->

=== BGP PEERS START - GR capability included in OPEN ===

RUNTIME: (waits for bye)
#99 ze-plugin-callback:bye {"reason":"shutdown"}  --->
                       <--- #99 ok
(plugin exits)
```

**Capability hex format:** Code 64 = Graceful Restart (RFC 4724).
`0078` = restart-time 120 (0x78 = 120). `005a` = restart-time 90.

### Event-Driven Plugin (e.g., RIB)

The RIB plugin tracks routes and replays them on peer reconnect.
Requires event subscription for runtime events.

```
Ze Engine                              RIB Plugin
----------                             ----------

STAGE 1: REGISTRATION
                       <--- #1 ze-plugin-engine:declare-registration
                            {"commands":[
                              {"name":"rib adjacent status"},
                              {"name":"rib adjacent inbound show"},
                              {"name":"rib adjacent outbound resend"}
                            ]}
#1 ok                  --->

STAGE 2: CONFIG DELIVERY
#1 ze-plugin-callback:configure {"sections":[]}  --->
                       <--- #1 ok

STAGE 3: CAPABILITY DECLARATION
                       <--- #2 ze-plugin-engine:declare-capabilities
                            {"capabilities":[]}
#2 ok                  --->

STAGE 4: REGISTRY SHARING
#2 ze-plugin-callback:share-registry {"commands":[...]}  --->
                       <--- #2 ok

STAGE 5: READY (with startup subscription)
                       <--- #3 ze-plugin-engine:ready
                            {"subscribe":{"events":["update","state","sent"],
                                          "peers":["*"],"format":"json"}}
#3 ok                  --->

=== BGP PEERS START ===

RUNTIME: Peer comes up
#42 ze-plugin-callback:deliver-batch
  {"events":["{\"type\":\"state\",\"peer\":\"192.168.1.1\",\"state\":\"up\"}"]}  --->
                       <--- #42 ok

RUNTIME: Route sent to peer
#43 ze-plugin-callback:deliver-batch
  {"events":["{\"type\":\"sent\",\"peer\":\"192.168.1.1\",...}"]}  --->
                       <--- #43 ok

RUNTIME: Command request
#44 ze-plugin-callback:execute-command
  {"serial":"abc","command":"rib adjacent status","args":[],"peer":"*"}  --->
                       <--- #44 ok {"status":"done","data":"{\"running\":true,\"peers\":1}"}

RUNTIME: Plugin sends route update to engine
                       <--- #4 ze-plugin-engine:update-route
                            {"peer-selector":"192.168.1.1",
                             "command":"update text nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.1.0/24"}
#4 ok {"peers-affected":1,"routes-sent":1}  --->

SHUTDOWN
#99 ze-plugin-callback:bye {"reason":"shutdown"}  --->
                       <--- #99 ok
(plugin exits)
```

---

## Capability Decode API

Plugins can provide capability decoding for `ze bgp decode --plugin <name>`.

This is a **standalone mode** separate from the 5-stage startup protocol.

### Usage

```bash
# Decode OPEN message with plugin-provided capability decoding
ze bgp decode --plugin bgp-hostname --open FFFF...
```

Without plugin, unknown capabilities show raw hex:
```json
{"code": 73, "name": "unknown", "raw": "0C6D792D686F73742D6E616D65..."}
```

With plugin, capabilities are decoded:
```json
{"name": "fqdn", "hostname": "my-host-name", "domain": "my-domain-name.com"}
```

### Protocol

Plugin is spawned with `--decode` flag and communicates via stdin/stdout.

#### Request Formats

| Request | Description |
|---------|-------------|
| `decode capability <code> <hex>` | JSON output (default) |
| `decode json capability <code> <hex>` | JSON output (explicit) |
| `decode text capability <code> <hex>` | Human-readable text output |
| `decode nlri <family> <hex>` | JSON output (default) |
| `decode json nlri <family> <hex>` | JSON output (explicit) |
| `decode text nlri <family> <hex>` | Human-readable text output |

#### Response Formats

| Response | Description |
|----------|-------------|
| `decoded json <json>` | JSON-formatted result |
| `decoded text <text>` | Human-readable single-line text |
| `decoded unknown` | Plugin cannot decode this input |

#### Examples

**Capability decode (JSON):**

| Direction | Message |
|-----------|---------|
| ze to plugin | `decode json capability 73 0C6D792D686F7374...` |
| plugin to ze | `decoded json {"name":"fqdn","hostname":"my-host","domain":"dom.com"}` |

**Capability decode (text):**

| Direction | Message |
|-----------|---------|
| ze to plugin | `decode text capability 73 0C6D792D686F7374...` |
| plugin to ze | `decoded text fqdn                 my-host.dom.com` |

**NLRI decode (text):**

| Direction | Message |
|-----------|---------|
| ze to plugin | `decode text nlri ipv4/flow 0501180a0000` |
| plugin to ze | `decoded text destination 10.0.0.0/24` |

If plugin cannot decode:

| Direction | Message |
|-----------|---------|
| plugin to ze | `decoded unknown` |

### Plugin Implementation

Plugin entry point with `--decode` flag:

```bash
ze plugin bgp-hostname --decode
```

Plugin reads decode requests from stdin, writes responses to stdout, exits on EOF.

### Files

| File | Purpose |
|------|---------|
| `internal/component/bgp/cli/decode_plugin.go` | Invokes plugin decode API |
| `internal/component/bgp/cli/cmd_plugin.go` | `ze plugin <name> --decode` entrypoint |
| `internal/component/bgp/plugins/hostname/hostname.go` | `RunDecodeMode()` - hostname capability |
| `internal/component/bgp/plugins/nlri/flowspec/plugin.go` | `RunFlowSpecDecode()` - FlowSpec NLRI |

---

**Last Updated:** 2026-03-22
