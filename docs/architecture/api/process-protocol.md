# Plugin Process Protocol

**Purpose:** Document Ze's plugin communication protocol and process lifecycle.

---

## Overview

Ze plugins communicate with the engine via JSON RPCs over a single bidirectional
connection. All messages use newline-delimited framing with the wire format
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
| 3. Capability | Plugin to Engine | `ze-plugin-engine:declare-capabilities` | `DeclareCapabilitiesInput` |
| 4. Registry | Engine to Plugin | `ze-plugin-callback:share-registry` | `ShareRegistryInput` |
| 5. Ready | Plugin to Engine | `ze-plugin-engine:ready` | `ReadyInput` |

**Timeout:** Each stage has a 5-second timeout (configurable via `stage-timeout` in plugin config).
If any plugin fails to complete a stage, startup aborts for all plugins.

**Why Barriers:**
- Ensures all plugins register commands before any receive config
- Ensures all capabilities declared before registry shared
- Prevents race conditions in multi-plugin configurations
- Guarantees consistent state before BGP peers start

**Filter Declaration (Stage 1, planned):**

Plugins may include a `filters` list in their `declare-registration` to offer named
route filters. Each entry declares a filter name, direction (import/export/both),
requested attributes, failure mode, and optional overrides of default filters.

| Field | Type | Description |
|-------|------|-------------|
| `filters[].name` | string | Filter name (referenced in config as `<plugin>:<name>`) |
| `filters[].direction` | enum | `import`, `export`, or `both` |
| `filters[].attributes` | list | Attribute names to receive (e.g., `as-path`, `community`) |
| `filters[].on-error` | enum | `reject` (fail-closed) or `accept` (fail-open) |
| `filters[].overrides` | list | Default filters this filter replaces (e.g., `rfc:no-self-as`) |

Config references filters as `<plugin>:<filter>` in `redistribution { import [...] export [...] }`.
The engine validates plugin and filter names after stage 1 completes.

<!-- source: plan/spec-redistribution-filter.md -- filter declaration design -->

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

---

## Runtime RPCs

### Engine to Plugin (Callbacks)

<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- dispatchCallback -->

| Method | Input | Response | Description |
|--------|-------|----------|-------------|
| `deliver-event` | `{"event":"<json>"}` | `ok` | Single event |
| `deliver-batch` | `{"events":[...]}` | `ok` | Batched events |
| `execute-command` | `ExecuteCommandInput` | `ExecuteCommandOutput` | Command execution |
| `config-verify` | `ConfigVerifyInput` | `ConfigVerifyOutput` | Validate candidate config |
| `config-apply` | `ConfigApplyInput` | `ConfigApplyOutput` | Apply config changes |
| `validate-open` | `ValidateOpenInput` | `ValidateOpenOutput` | Validate OPEN message |
| `encode-nlri` | `EncodeNLRIInput` | `{"hex":"..."}` | Encode NLRI |
| `decode-nlri` | `DecodeNLRIInput` | `{"json":"..."}` | Decode NLRI |
| `decode-capability` | `DecodeCapabilityInput` | `{"json":"..."}` | Decode capability |
| `bye` | `ByeInput` | `ok` | Shutdown signal |
| `filter-update` | `FilterUpdateInput` | `FilterUpdateOutput` | Route filter request (planned) |

All methods are prefixed with `ze-plugin-callback:`.

**filter-update (planned):** Engine sends UPDATE attributes to a named filter.
Plugin responds accept, reject, or modify (delta-only changed attributes).
Includes filter name so the plugin can dispatch to the correct handler.

<!-- source: plan/spec-redistribution-filter.md -- filter-update RPC design -->

### Plugin to Engine

<!-- source: pkg/plugin/sdk/sdk_engine.go -- all methods -->

| Method | Input | Output | Description |
|--------|-------|--------|-------------|
| `update-route` | `UpdateRouteInput` | `UpdateRouteOutput` | Inject route to peers |
| `dispatch-command` | `DispatchCommandInput` | `DispatchCommandOutput` | Inter-plugin command |
| `emit-event` | `EmitEventInput` | `EmitEventOutput` | Push event to subscribers |
| `subscribe-events` | `SubscribeEventsInput` | - | Subscribe to events |
| `unsubscribe-events` | - | - | Unsubscribe from events |
| `decode-nlri` | `DecodeNLRIInput` | `DecodeNLRIOutput` | Decode NLRI via registry |
| `encode-nlri` | `EncodeNLRIInput` | `EncodeNLRIOutput` | Encode NLRI via registry |
| `decode-mp-reach` | `DecodeMPReachInput` | `DecodeMPReachOutput` | Decode MP_REACH_NLRI |
| `decode-mp-unreach` | `DecodeMPUnreachInput` | `DecodeMPUnreachOutput` | Decode MP_UNREACH_NLRI |
| `decode-update` | `DecodeUpdateInput` | `DecodeUpdateOutput` | Decode full UPDATE |

All methods are prefixed with `ze-plugin-engine:`.

---

## Event Delivery

Events are delivered to plugins that have subscribed (via `subscribe-events` RPC
or `SetStartupSubscriptions`). Events are enqueued into a per-process channel.
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
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- handleDeliverBatch -->

### DirectBridge Optimization

For internal plugins with an active `DirectBridge`, `deliverBatch()` calls
`bridge.DeliverEvents(events)` directly instead of `conn.SendDeliverBatch()`,
bypassing JSON-RPC envelope construction, newline framing, and pipe I/O. The plugin's
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

### Event Subscription

Plugins subscribe to events using either:

1. **Startup subscription** (recommended): included in the `ready` RPC so the engine
   registers atomically before `SignalAPIReady`, avoiding the race between the reactor
   sending routes and the plugin subscribing.
   <!-- source: pkg/plugin/rpc/types.go -- ReadyInput -->

2. **Runtime subscription**: via `subscribe-events` RPC in `OnStarted` callback.
   Safe but has a small window where events could be missed.
   <!-- source: pkg/plugin/sdk/sdk_engine.go -- SubscribeEvents -->

**Subscription fields:**
<!-- source: pkg/plugin/rpc/types.go -- SubscribeEventsInput -->

| Field | Type | Description |
|-------|------|-------------|
| `events` | `[]string` | Event types (e.g., `["update","state"]`) |
| `peers` | `[]string` | Peer filter (e.g., `["*"]` for all) |
| `format` | `string` | Format preference (e.g., `"json"`) |
| `encoding` | `string` | `"json"` (default) or `"text"` |

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
|   |  1. 5-stage startup (JSON RPCs)                                   |   |
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

**Four-phase plugin startup:**
1. **Phase 1:** Explicit plugins start first and register their families
2. **Phase 2:** After Phase 1 completes, engine checks which configured families are still unclaimed. Internal plugins are auto-loaded ONLY for unclaimed families.
3. **Phase 3:** After Phase 2 completes, engine checks which custom event types are referenced in peer `receive` config but not produced by any running plugin. Producing plugins (and their transitive dependencies) are auto-loaded. For example, `receive [ update-rpki ]` auto-loads `bgp-rpki-decorator` and its dependency `bgp-rpki`.
4. **Phase 4:** After Phase 3 completes, engine checks which custom send types are referenced in peer `send` config but not enabled by any running plugin. Enabling plugins (and their transitive dependencies) are auto-loaded. For example, `send [ enhanced-refresh ]` auto-loads `bgp-route-refresh`.

**Family auto-loading** (Phase 2) is **prevented** when:
1. An explicit plugin declares `decode` for the family (family-based check)
2. `--plugin ze.<name>` is passed on command line (prevents auto-load for that plugin)

The check is based on **family claims**, not plugin name. Plugin names are informational only.

| Config | Plugin | Result |
|--------|--------|--------|
| `family { ipv4/flow; }` | None | Auto-loads `ze.flowspec` |
| `family { ipv4/flow; }` | `--plugin ze.flowspec` | Uses explicit plugin (no auto-load) |
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
| Plugin to Engine | `ze-plugin-engine:decode-nlri` | `{"family":"...","hex":"..."}` | `{"json":"..."}` |
| Engine to Plugin | `ze-plugin-callback:encode-nlri` | `{"family":"...","args":[...]}` | `{"hex":"..."}` |
| Engine to Plugin | `ze-plugin-callback:decode-nlri` | `{"family":"...","hex":"..."}` | `{"json":"..."}` |

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
<!-- source: internal/component/plugin/server/startup.go -- handleProcessStartupRPC, SetBridge -->

**Runtime hot path (after bridge activates):**

| Direction | Socket path (before) | Direct path (after) |
|-----------|---------------------|---------------------|
| Engine to Plugin events (text) | JSON-RPC envelope -> newline frame -> `net.Pipe.Write` -> read -> unmarshal -> `onEvent` | `bridge.DeliverEvents(events)` -> `onEvent` directly |
| Engine to Plugin events (structured) | -- | `bridge.DeliverStructured([]any)` -> `onStructuredEvent` with `*StructuredEvent` (no text formatting, no JSON parsing) |
| Plugin to Engine RPCs | `json.Marshal` -> newline frame -> `net.Pipe.Write` -> read -> unmarshal -> `dispatcher.Dispatch` | `bridge.DispatchRPC(method, params)` -> `dispatcher.Dispatch` directly |
| Engine to Plugin callbacks | JSON-RPC via MuxConn + 3-way select | `bridge.SendCallback()` -> callback channel -> `bridgeEventLoop` 2-way select |

**Callback dispatch:** Both event loops (pipe and bridge) dispatch through a generic
callback registry (`map[string]callbackHandler`). Each `On*` method registers a typed
wrapper in the map. Adding a new callback requires only one `On*` method -- zero changes
to the dispatch or event loop code. See `rules/plugin-design.md` "SDK Is Generic".
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- eventLoop, bridgeEventLoop, getCallback -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- initCallbackDefaults, On* methods -->

**Shutdown:** `Process.Stop()` cancels the context and calls `bridge.CloseCallbacks()`
(guarded by `sync.Once`), closing the callback channel. The `bridgeEventLoop` exits
on channel close. `SendCallback` recovers from send-on-closed-channel panics and
returns `ErrBridgeClosed`.
<!-- source: internal/component/plugin/process/process.go -- Stop -->
<!-- source: pkg/plugin/rpc/bridge.go -- SendCallback, CloseCallbacks, ErrBridgeClosed -->

**Files:**

| File | Purpose |
|------|---------|
| `pkg/plugin/rpc/bridge.go` | `DirectBridge`, `BridgedConn`, `Bridger`, `BridgeCallback`, `SendCallback`, `CloseCallbacks` |
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
7. No `DirectBridge` -- always uses JSON-RPC over TLS
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
`DeclareRegistrationInput`:
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput, FamilyDecl -->

```json
{
  "families": [
    {"name": "ipv4/flow", "mode": "encode"},
    {"name": "ipv4/flow", "mode": "decode"},
    {"name": "ipv6/flow", "mode": "encode"},
    {"name": "ipv6/flow", "mode": "decode"},
    {"name": "ipv4/flow-vpn", "mode": "both"},
    {"name": "ipv6/flow-vpn", "mode": "both"}
  ]
}
```

**FamilyDecl fields:**

| Field | Values | Description |
|-------|--------|-------------|
| `name` | `"ipv4/flow"`, `"l2vpn/evpn"`, etc. | Address family (`afi/safi` format) |
| `mode` | `"encode"`, `"decode"`, `"both"` | Direction of conversion |

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
| `ze-plugin-callback:decode-nlri` | `{"family":"ipv4/flow","hex":"0701180A0000"}` | `{"json":"{\"destination\":...}"}` |

**Plugin to Engine (via registry):**

| RPC | Input | Output |
|-----|-------|--------|
| `ze-plugin-engine:encode-nlri` | `{"family":"ipv4/flow","args":["destination","10.0.0.0/24"]}` | `{"hex":"0701180A0000"}` |
| `ze-plugin-engine:decode-nlri` | `{"family":"ipv4/flow","hex":"0701180A0000"}` | `{"json":"{\"destination\":...}"}` |

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
| `internal/component/plugin/server.go` | NLRI routing |
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

**Plugin to Engine (success):**
```
#5 ok {"status":"done","data":"{\"running\":true,\"peers\":1}"}
```

**Plugin to Engine (error):**
```
#5 ok {"status":"error","data":"component not found"}
```

Note: command execution results are sent as `ok` responses with a `status` field
(not as `error` responses), because the RPC itself succeeded even if the command failed.

### Inter-Plugin Communication

Plugins can dispatch commands to other plugins via the engine:
<!-- source: pkg/plugin/sdk/sdk_engine.go -- DispatchCommand -->

```
#4 ze-plugin-engine:dispatch-command {"command":"rib adjacent inbound show"}
#4 ok {"status":"done","data":"{...}"}
```

The engine routes the command to the target plugin via longest-match registry lookup.

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
ze bgp decode --plugin ze.hostname --open FFFF...
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
ze plugin hostname --decode
```

Plugin reads decode requests from stdin, writes responses to stdout, exits on EOF.

### Files

| File | Purpose |
|------|---------|
| `cmd/ze/bgp/decode.go` | Invokes plugin decode API |
| `cmd/ze/bgp/plugin_hostname.go` | `--decode` flag handling (hostname) |
| `cmd/ze/bgp/plugin_flowspec.go` | `--decode` flag handling (flowspec) |
| `internal/component/plugin/hostname/hostname.go` | `RunDecodeMode()` - hostname capability |
| `internal/component/plugin/flowspec/plugin.go` | `RunFlowSpecDecode()` - FlowSpec NLRI |

---

**Last Updated:** 2026-03-22
