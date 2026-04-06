# Plugin Protocol

Ze plugins communicate with the engine via JSON RPCs over a single bidirectional
connection. Internal plugins use `net.Pipe()`; external plugins connect back via TLS.
<!-- source: pkg/plugin/sdk/sdk.go -- NewWithConn, NewFromTLSEnv -->

All messages use newline-delimited framing with the wire format `#<id> <verb> [<json>]\n`.
<!-- source: pkg/plugin/rpc/conn.go -- Conn doc comment -->

## Wire Format

Every message is a single newline-terminated line:

| Message type | Format |
|-------------|--------|
| Request | `#<id> <method> [<json-params>]\n` |
| Success response | `#<id> ok [<json-result>]\n` |
| Error response | `#<id> error [<json-error>]\n` |

<!-- source: pkg/plugin/rpc/message.go -- FormatRequest, FormatResult, FormatError -->

- `<id>` is a monotonically increasing uint64 correlation ID
- `<method>` uses YANG-style `<module>:<rpc-name>` naming (e.g., `ze-plugin-engine:declare-registration`)
- JSON payloads are optional (omitted when empty or null)
- Responses use `ok` or `error` as the verb; requests use the method name

**Routing:** `MuxConn` multiplexes a single connection for concurrent RPCs. A background
reader goroutine routes incoming lines by verb: `ok`/`error` responses go to the waiting
`CallRPC` caller by `#<id>`, while method-name requests go to the `Requests()` channel.
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn, readLoop -->

**Examples:**

```
# Plugin sends declare-registration (Stage 1)
#1 ze-plugin-engine:declare-registration {"families":[{"name":"ipv4/flow","mode":"both"}]}

# Engine responds OK
#1 ok

# Engine sends configure to plugin (Stage 2)
#1 ze-plugin-callback:configure {"sections":[{"root":"bgp","data":"{...}"}]}

# Plugin responds OK
#1 ok

# Engine sends event at runtime
#42 ze-plugin-callback:deliver-event {"event":"{\"type\":\"state\",...}"}

# Plugin responds OK
#42 ok

# Error response with payload
#5 error {"code":"error","message":"unknown family: ipv4/unknown"}
```

## Protocol Stages

The SDK handles the 5-stage startup protocol automatically via `Plugin.Run()`.
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->

### Stage 1: Registration (Plugin to Engine)

Plugin sends `ze-plugin-engine:declare-registration` with a `DeclareRegistrationInput`:
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput -->

| Field | Type | Description |
|-------|------|-------------|
| `families` | `[]FamilyDecl` | Address families the plugin handles (name + mode) |
| `commands` | `[]CommandDecl` | Commands the plugin provides |
| `dependencies` | `[]string` | Plugin names that must also be loaded |
| `wants-config` | `[]string` | Config roots the plugin wants to receive |
| `schema` | `SchemaDecl` | YANG schema (module, namespace, yang-text, handlers) |
| `wants-validate-open` | `bool` | Whether plugin wants OPEN validation callbacks |
| `cache-consumer` | `bool` | Whether plugin consumes cached events |
| `cache-consumer-unordered` | `bool` | Whether unordered cache delivery is acceptable |
| `connection-handlers` | `[]ConnectionHandlerDecl` | Listen sockets to receive via fd passing |

**Wire example:**

```
#1 ze-plugin-engine:declare-registration {"families":[{"name":"ipv4/flow","mode":"both"}],"commands":[{"name":"flowspec status","description":"Show FlowSpec status"}],"wants-config":["bgp"]}
#1 ok
```

### Stage 2: Config (Engine to Plugin)

Engine sends `ze-plugin-callback:configure` with a `ConfigureInput`:
<!-- source: pkg/plugin/rpc/types.go -- ConfigureInput -->

| Field | Type | Description |
|-------|------|-------------|
| `sections` | `[]ConfigSection` | Config sections (root name + JSON data) |

Each `ConfigSection` has:
<!-- source: pkg/plugin/rpc/types.go -- ConfigSection -->

| Field | Type | Description |
|-------|------|-------------|
| `root` | `string` | Config root name (e.g., `"bgp"`) |
| `data` | `string` | JSON-encoded config data |

**Wire example:**

```
#1 ze-plugin-callback:configure {"sections":[{"root":"bgp","data":"{\"bgp\":{\"peer\":{...}}}"}]}
#1 ok
```

### Stage 3: Capabilities (Plugin to Engine)

Plugin sends `ze-plugin-engine:declare-capabilities` with a `DeclareCapabilitiesInput`:
<!-- source: pkg/plugin/rpc/types.go -- DeclareCapabilitiesInput -->

| Field | Type | Description |
|-------|------|-------------|
| `capabilities` | `[]CapabilityDecl` | BGP capabilities for OPEN injection |

Each `CapabilityDecl` has:
<!-- source: pkg/plugin/rpc/types.go -- CapabilityDecl -->

| Field | Type | Description |
|-------|------|-------------|
| `code` | `uint8` | Capability code (e.g., 64 for Graceful Restart) |
| `encoding` | `string` | `"hex"`, `"b64"`, or `"text"` |
| `payload` | `string` | Encoded capability value |
| `peers` | `[]string` | Peer addresses to inject into (empty = all peers) |

**Wire example:**

```
#2 ze-plugin-engine:declare-capabilities {"capabilities":[{"code":64,"encoding":"hex","payload":"0078","peers":["192.168.1.1"]}]}
#2 ok
```

### Stage 4: Registry (Engine to Plugin)

Engine sends `ze-plugin-callback:share-registry` with a `ShareRegistryInput`:
<!-- source: pkg/plugin/rpc/types.go -- ShareRegistryInput -->

| Field | Type | Description |
|-------|------|-------------|
| `commands` | `[]RegistryCommand` | Registered commands from all plugins |

Each `RegistryCommand` has:
<!-- source: pkg/plugin/rpc/types.go -- RegistryCommand -->

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Command name |
| `plugin` | `string` | Plugin that registered it |
| `encoding` | `string` | Encoding format |

**Wire example:**

```
#2 ze-plugin-callback:share-registry {"commands":[{"name":"rib adjacent status","plugin":"bgp-adj-rib-in"},{"name":"peer","plugin":"bgp"}]}
#2 ok
```

### Stage 5: Ready (Plugin to Engine)

Plugin sends `ze-plugin-engine:ready` with an optional `ReadyInput`:
<!-- source: pkg/plugin/rpc/types.go -- ReadyInput -->

| Field | Type | Description |
|-------|------|-------------|
| `subscribe` | `SubscribeEventsInput` | Optional startup event subscription |
| `transport` | `string` | `"bridge"` for internal plugins; pipe closed after ack |

The `subscribe` field allows plugins to register event subscriptions atomically with
startup completion. This avoids a race where `SignalAPIReady` triggers route sends before
a separate `subscribe-events` RPC could be processed.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- SetStartupSubscriptions -->

When `transport` is `"bridge"`, the engine activates bridge callbacks on the `PluginConn`
and the SDK closes the pipe after receiving the OK response. All subsequent engine-to-plugin
callbacks flow through `bridge.CallbackCh()` instead of the MuxConn.
<!-- source: pkg/plugin/rpc/types.go -- ReadyInput.Transport -->

**Wire example:**

```
#3 ze-plugin-engine:ready {"subscribe":{"events":["update","state"],"peers":["*"],"format":"json"},"transport":"bridge"}
#3 ok
```

After Stage 5, the SDK activates the DirectBridge (for internal plugins) and enters
the event loop.

## Runtime Callbacks (Engine to Plugin)

After startup, the engine sends runtime RPCs to the plugin. The SDK dispatches
these through a generic callback registry (`map[string]callbackHandler`) -- both
the pipe and bridge event loops use the same map-based lookup, with no
transport-specific handler code.
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- eventLoop, bridgeEventLoop, getCallback -->

| Method | Input | Description |
|--------|-------|-------------|
| `ze-plugin-callback:deliver-event` | `{"event":"<json>"}` | Single event delivery |
| `ze-plugin-callback:deliver-batch` | `{"events":[...]}` | Batched event delivery |
| `ze-plugin-callback:execute-command` | `ExecuteCommandInput` | Command execution request |
| `ze-plugin-callback:config-verify` | `ConfigVerifyInput` | Config verification (reload) |
| `ze-plugin-callback:config-apply` | `ConfigApplyInput` | Config apply (reload) |
| `ze-plugin-callback:validate-open` | `ValidateOpenInput` | OPEN message validation |
| `ze-plugin-callback:encode-nlri` | `EncodeNLRIInput` | NLRI encoding request |
| `ze-plugin-callback:decode-nlri` | `DecodeNLRIInput` | NLRI decoding request |
| `ze-plugin-callback:decode-capability` | `DecodeCapabilityInput` | Capability decoding request |
| `ze-plugin-callback:bye` | `ByeInput` | Shutdown notification |

## Runtime RPCs (Plugin to Engine)

Plugins can call the engine during runtime via these RPCs:
<!-- source: pkg/plugin/sdk/sdk_engine.go -- all methods -->

| Method | Input | Output | Description |
|--------|-------|--------|-------------|
| `ze-plugin-engine:update-route` | `UpdateRouteInput` | `UpdateRouteOutput` | Inject route to peers |
| `ze-plugin-engine:dispatch-command` | `DispatchCommandInput` | `DispatchCommandOutput` | Inter-plugin command dispatch |
| `ze-plugin-engine:emit-event` | `EmitEventInput` | `EmitEventOutput` | Push event to subscribers |
| `ze-plugin-engine:subscribe-events` | `SubscribeEventsInput` | - | Subscribe to events |
| `ze-plugin-engine:unsubscribe-events` | - | - | Unsubscribe from events |
| `ze-plugin-engine:decode-nlri` | `DecodeNLRIInput` | `DecodeNLRIOutput` | Decode NLRI via registry |
| `ze-plugin-engine:encode-nlri` | `EncodeNLRIInput` | `EncodeNLRIOutput` | Encode NLRI via registry |
| `ze-plugin-engine:decode-mp-reach` | `DecodeMPReachInput` | `DecodeMPReachOutput` | Decode MP_REACH_NLRI |
| `ze-plugin-engine:decode-mp-unreach` | `DecodeMPUnreachInput` | `DecodeMPUnreachOutput` | Decode MP_UNREACH_NLRI |
| `ze-plugin-engine:decode-update` | `DecodeUpdateInput` | `DecodeUpdateOutput` | Decode full UPDATE message |

## Message Flow Example

```
Plugin                                             Engine
   |                                                  |
   |  STAGE 1: declare-registration                   |
   |-- #1 ze-plugin-engine:declare-registration {...}->|
   |<- #1 ok ---------------------------------------- |
   |                                                  |
   |  STAGE 2: configure                              |
   |<- #1 ze-plugin-callback:configure {...} ---------|
   |-- #1 ok ---------------------------------------->|
   |                                                  |
   |  STAGE 3: declare-capabilities                   |
   |-- #2 ze-plugin-engine:declare-capabilities {...}->|
   |<- #2 ok ---------------------------------------- |
   |                                                  |
   |  STAGE 4: share-registry                         |
   |<- #2 ze-plugin-callback:share-registry {...} ----|
   |-- #2 ok ---------------------------------------->|
   |                                                  |
   |  STAGE 5: ready                                  |
   |-- #3 ze-plugin-engine:ready {...} -------------->|
   |<- #3 ok ---------------------------------------- |
   |                                                  |
   |  RUNTIME: event delivery                         |
   |<- #42 ze-plugin-callback:deliver-batch {...} ----|
   |-- #42 ok ---------------------------------------->|
   |                                                  |
   |  RUNTIME: plugin sends route update              |
   |-- #4 ze-plugin-engine:update-route {...} -------->|
   |<- #4 ok {"peers-affected":2,"routes-sent":2} --- |
   |                                                  |
   |  RUNTIME: command execution                      |
   |<- #43 ze-plugin-callback:execute-command {...} ---|
   |-- #43 ok {"status":"done","data":"..."} -------->|
   |                                                  |
   |  SHUTDOWN: bye                                   |
   |<- #99 ze-plugin-callback:bye {"reason":"..."} ---|
   |-- #99 ok ---------------------------------------->|
   |  (plugin exits)                                  |
```

## Error Handling

**Stage errors:** If any stage RPC fails (error response or timeout), the SDK returns
an error from `Run()` with context like `"stage 1 (declare-registration): ..."`.
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->

**Runtime errors:** Callback handlers return errors via `#<id> error {"code":"...","message":"..."}`.
Unknown methods are rejected with `"unknown method: <method>"`.
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- eventLoop, bridgeEventLoop -->

**Connection errors:** EOF or closed connection during the event loop is treated as
clean shutdown (engine closes socket to signal exit).
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- eventLoop, isConnectionClosed -->

**Config reload errors:** `config-verify` and `config-apply` return structured results
with `{"status":"ok"}` or `{"status":"error","error":"..."}`. If no handler is registered,
the response is `{"status":"ok"}` (graceful no-op).
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigVerify, OnConfigApply, initCallbackDefaults -->

## Batched Event Delivery

Events are batched for efficiency. The engine collects pending events from a per-process
channel, JSON-quotes each one, and sends them in a single `deliver-batch` RPC.
<!-- source: pkg/plugin/rpc/batch.go -- WriteBatchFrame -->

```
#42 ze-plugin-callback:deliver-batch {"events":["<json-event-1>","<json-event-2>",...]}
#42 ok
```

The SDK unpacks the batch and dispatches each event to the `OnEvent` handler individually.
Both `deliver-event` and `deliver-batch` handlers are registered in the callback map
when `OnEvent` is called.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnEvent -->

For internal plugins with an active `DirectBridge`, event delivery bypasses the callback
channel entirely: `bridge.DeliverEvents(events)` calls the `onEvent` handler directly
(hot path). The callback channel is only used for non-event callbacks (execute-command,
config-verify, etc.) and bye.
<!-- source: pkg/plugin/sdk/sdk.go -- Run, bridge activation -->
