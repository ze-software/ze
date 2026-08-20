# Plugin Protocol

Ze plugins communicate with the engine via newline-framed YANG RPCs over a single
bidirectional connection. Internal plugins use `net.Pipe()` for startup; external
plugins connect back via TLS.
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
| Record answer | `#<id> ok <key=value tail>\n`, one line per record |

<!-- source: pkg/plugin/rpc/message.go -- FormatRequest, FormatResult, FormatError -->

The record answer applies to `dispatch-command` and `dispatch-command-args`
alone, and only for a plugin that asked for it at Stage 3. It replaces the single
success line with a head, zero or more records, and a terminator. A plugin that
asks for nothing keeps the three forms above. The grammar is in
[ipc_protocol.md](../architecture/api/ipc_protocol.md), "Answer Protocol".
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerHead, AppendAnswerTerminator -->

The Go SDK cannot ask for it, and must not be made to until it can read it.
`dispatchCommandRPC` unmarshals the response payload into a
`DispatchCommandOutput`, and a head such as `status=done type=json` is not JSON.
Declare the shape only from a client that reads the sequence, such as the Python
helper `test/scripts/ze_api.py`.
<!-- source: pkg/plugin/sdk/sdk_engine.go -- dispatchCommandRPC, Plugin.DispatchCommand -->

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
#1 ze-plugin-engine:declare-registration {"families":[{"name":"ipv4/flow","mode":"both","afi":1,"safi":133}]}

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
| `families` | `[]FamilyDecl` | Address families the plugin handles (name, mode, AFI, and SAFI) |
| `commands` | `[]CommandDecl` | Commands the plugin provides |
| `dependencies` | `[]string` | Plugin names that must also be loaded |
| `wants-config` | `[]string` | Config roots the plugin wants to receive |
| `config-operations` | `[]ConfigOperationDecl` | Config operation callbacks the plugin supports |
| `verify-budget` | `int` | Estimated verify time in seconds (`0` means trivial) |
| `apply-budget` | `int` | Estimated apply time in seconds (`0` means trivial) |
| `schema` | `*SchemaDecl` | YANG schema (module, namespace, yang-text, handlers) |
| `wants-validate-open` | `bool` | Whether plugin wants OPEN validation callbacks |
| `cache-consumer` | `bool` | Whether plugin consumes cached events |
| `cache-consumer-unordered` | `bool` | Whether unordered cache delivery is acceptable |
| `filters` | `[]FilterDecl` | Named route filters the plugin provides |
| `doctor-checks` | `[]DoctorCheckDecl` | Doctor checks the plugin provides |
| `enrichers` | `[]EnricherDecl` | Show enrichers the plugin provides |
| `claims` | `[]string` | Exclusive runtime roles the plugin takes over |


Each `FamilyDecl` has these fields:
<!-- source: pkg/plugin/rpc/types.go -- FamilyDecl -->
<!-- source: internal/component/plugin/server/startup.go -- registerPluginFamilies -->

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Canonical `afi/safi` name |
| `mode` | `string` | `encode`, `decode`, or `both` |
| `afi` | `uint16` | RFC 4760 Address Family Identifier |
| `safi` | `uint8` | RFC 4760 Subsequent Address Family Identifier |

Set `afi` and `safi` for a custom family. A built-in family can omit both numeric fields.

**Wire example:**

```
#1 ze-plugin-engine:declare-registration {"families":[{"name":"ipv4/flow","mode":"both","afi":1,"safi":133}],"commands":[{"name":"flowspec status","description":"Show FlowSpec status"}],"wants-config":["bgp"]}
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
| `protocol` | `[]string` | Wire shapes this plugin understands. `record-answers` is the only name. Omit it to keep the single-line answer |
<!-- source: pkg/plugin/rpc/types.go -- DeclareCapabilitiesInput.Understands, ProtocolRecordAnswers -->

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
each wire method to its registered handler.
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- callbackBye through callbackPostStartup -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnEvent through OnAllPluginsReady -->
<!-- source: pkg/plugin/rpc/types.go -- callback input and output types -->

| Method | SDK handler | Input | Output | Purpose |
|--------|-------------|-------|--------|---------|
| `ze-plugin-callback:deliver-event` | `OnEvent` | `DeliverEventInput` | None | Deliver one event |
| `ze-plugin-callback:deliver-batch` | `OnEvent` | `{"events":[]}` | None | Deliver an event batch |
| `ze-plugin-callback:execute-command` | `OnExecuteCommand` | `ExecuteCommandInput` | `ExecuteCommandOutput` | Run a command |
| `ze-plugin-callback:encode-nlri` | `OnEncodeNLRI` | `EncodeNLRIInput` | `EncodeNLRIOutput` | Encode NLRI |
| `ze-plugin-callback:decode-nlri` | `OnDecodeNLRI` | `DecodeNLRIInput` | `DecodeNLRIOutput` | Decode NLRI |
| `ze-plugin-callback:decode-capability` | `OnDecodeCapability` | `DecodeCapabilityInput` | `{"json":...}` | Decode a capability |
| `ze-plugin-callback:config-verify` | `OnConfigVerify` | `ConfigVerifyInput` | `ConfigVerifyOutput` | Verify candidate config |
| `ze-plugin-callback:config-apply` | `OnConfigApply` | `ConfigApplyInput` | `ConfigApplyOutput` | Apply a config diff |
| `ze-plugin-callback:config-rollback` | `OnConfigRollback` | `{"transaction-id":"..."}` | None | Roll back a config transaction |
| `ze-plugin-callback:config-operation-decompose` | `OnConfigOperationDecompose` | `ConfigOperationDecomposeInput` | `ConfigOperationDecomposeOutput` | Decompose a config transaction |
| `ze-plugin-callback:config-operation-verify` | `OnConfigOperationVerify` | `ConfigOperationVerifyInput` | `ConfigOperationVerifyOutput` | Verify a config operation |
| `ze-plugin-callback:config-operation-apply` | `OnConfigOperationApply` | `ConfigOperationApplyInput` | `ConfigOperationApplyOutput` | Apply a config operation |
| `ze-plugin-callback:config-operation-rollback` | `OnConfigOperationRollback` | `ConfigOperationRollbackInput` | `ConfigOperationRollbackOutput` | Roll back config operations |
| `ze-plugin-callback:config-operation-commit` | `OnConfigOperationCommit` | `ConfigOperationCommitInput` | `ConfigOperationCommitOutput` | Commit operation journals |
| `ze-plugin-callback:validate-open` | `OnValidateOpen` | `ValidateOpenInput` | `ValidateOpenOutput` | Validate an OPEN message |
| `ze-plugin-callback:filter-update` | `OnFilterUpdate` | `FilterUpdateInput` | `FilterUpdateOutput` | Filter a route update |
| `ze-plugin-callback:doctor-check` | `OnDoctorCheck` | `DoctorCheckInput` | `DoctorCheckOutput` | Run a doctor check |
| `ze-plugin-callback:enrich-show` | `OnEnrichShow` | `EnrichShowInput` | `EnrichShowOutput` | Add data to show output |
| `ze-plugin-callback:post-startup` | `OnAllPluginsReady` | None | None | Signal that all plugins are ready |
| No wire method (DirectBridge only) | `OnStructuredEvent` | `[]any` of `*rpc.StructuredEvent` | None | Deliver structured events without JSON |
| `ze-plugin-callback:bye` | `OnBye` | `ByeInput` | None | Notify the plugin of shutdown |

## Runtime RPCs (Plugin to Engine)

Plugins call the engine during runtime through these SDK methods:
<!-- source: pkg/plugin/sdk/sdk_engine.go -- UpdateRoute through DecodeUpdate -->
<!-- source: pkg/plugin/rpc/types.go -- plugin-to-engine runtime RPC input and output types -->
<!-- source: pkg/plugin/rpc/bridge.go -- BatchValidateResult -->

| Method | SDK method | Input | Output | Purpose |
|--------|------------|-------|--------|---------|
| `ze-plugin-engine:update-route` | `UpdateRoute`, `UpdateRouteWithMeta`, `UpdateRouteSel`, `UpdateRouteSelWithMeta` | `UpdateRouteInput` | `UpdateRouteOutput` | Send a route update |
| `ze-plugin-engine:forward-cached` | `ForwardCached` | `ForwardCachedInput` | None | Forward cached UPDATEs |
| `ze-plugin-engine:release-cached` | `ReleaseCached` | `ReleaseCachedInput` | None | Release cached UPDATEs |
| `ze-plugin-engine:relay-stored-route` | `RelayStoredRoute` | `RelayStoredRouteInput` | None | Relay stored routes |
| `ze-plugin-engine:route-install` | `RouteInstall` | `RouteInstallInput` | `RouteInstallOutput` | Install routes in the Loc-RIB |
| `ze-plugin-engine:route-remove` | `RouteRemove` | `RouteRemoveInput` | `RouteRemoveOutput` | Remove routes from the Loc-RIB |
| `ze-plugin-engine:inject-wire-route` | `InjectWireRoute` | `InjectWireRouteInput` | None | Inject a raw BGP UPDATE |
| `ze-plugin-engine:batch-validate` | `BatchValidate` | `BatchValidateInput` | `BatchValidateResult` | Submit route validation decisions |
| `ze-plugin-engine:dispatch-command` | `DispatchCommand` | `DispatchCommandInput` | `DispatchCommandOutput` | Dispatch a command string |
| `ze-plugin-engine:dispatch-command-args` | `DispatchCommandArgs` | `DispatchCommandArgsInput` | `DispatchCommandOutput` | Dispatch a command and pre-tokenized arguments |
| `ze-plugin-engine:emit-event` | `EmitEvent` | `EmitEventInput` | `EmitEventOutput` | Emit an event |
| `ze-plugin-engine:subscribe-events` | `SubscribeEvents` | `SubscribeEventsInput` | None | Subscribe to events |
| `ze-plugin-engine:unsubscribe-events` | `UnsubscribeEvents` | None | None | Unsubscribe from events |
| `ze-plugin-engine:decode-nlri` | `DecodeNLRI` | `DecodeNLRIInput` | `DecodeNLRIOutput` | Decode NLRI through the registry |
| `ze-plugin-engine:encode-nlri` | `EncodeNLRI` | `EncodeNLRIInput` | `EncodeNLRIOutput` | Encode NLRI through the registry |
| `ze-plugin-engine:decode-mp-reach` | `DecodeMPReach` | `DecodeMPReachInput` | `DecodeMPReachOutput` | Decode MP_REACH_NLRI |
| `ze-plugin-engine:decode-mp-unreach` | `DecodeMPUnreach` | `DecodeMPUnreachInput` | `DecodeMPUnreachOutput` | Decode MP_UNREACH_NLRI |
| `ze-plugin-engine:decode-update` | `DecodeUpdate` | `DecodeUpdateInput` | `DecodeUpdateOutput` | Decode a BGP UPDATE |

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
