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
| Record answer | `#<id> <kind> <positional fields>\n`: a head, one line for each record, and a terminator |

<!-- source: pkg/plugin/rpc/message.go -- AppendRequest, AppendResult, AppendError -->

The record answer applies to three methods. It replaces the single success line
with a head, zero or more records, and a terminator. Every other method keeps
the three forms above.

An answer line carries no verb and no key name. The field after the id is a
three-byte word saying what the line IS:

| Word | The line is |
|------|-------------|
| `top` | the head, which opens the answer |
| `row` | one record the command produced |
| `bad` | one record it rejected. The walk goes on |
| `end` | the terminator, which ends the answer |
| `nay` | the whole answer to a command text naming no command |

The head carries a second three-byte word saying how the records read:

| Word | The records are |
|------|-----------------|
| `doc` | one document. The whole answer is that one value |
| `map` | one map of names to values for each record |
| `tab` | one positional row for each record, read against the column names |

Every field after those is positional and takes one of two shapes. A NUMBER is
decimal digits closed by a space or by the end of the line. A TEXT is decimal
digits, a colon, then that many BYTES. **The count is a BYTE count, never a count
of characters.** A text of zero bytes is written `0:`, so a line's field count
never varies:

```
#43 top doc 0: 0:
|   |   |   |  |
|   |   |   |  +----- column names, 0 BYTES, so the records are not positional
|   |   |   +-------- envelope name, 0 BYTES, so the document carries its own
|   |   +------------ item type doc: the whole answer is one document
|   +---------------- kind top: the head, always the first line
+-------------------- correlation id 43, echoed from the request

#43 row 26:{"running":true,"peers":1}
|   |   |  |
|   |   |  +----- those 26 bytes, the handler's value byte for byte
|   |   +-------- 26, the payload's BYTE count, then its colon
|   +------------ kind row: one record the command produced
+---------------- correlation id 43

#43 end 1 0 0:
|   |   | | |
|   |   | | +----- message, 0 BYTES, so the command stated none
|   |   | +------- 0 rows rejected
|   |   +--------- 1 record produced
|   +------------- kind end: the terminator, always the last line
+----------------- correlation id 43
```

Neither the head nor the terminator states an outcome. The verdict is DERIVED
from what the terminator carries:

| The terminator | Verdict |
|----------------|---------|
| no rejected row and no message | `done` |
| records and rejected rows, no message | `partial` |
| a message over no record | `error` |
| a message over records | `aborted` |
| no terminator at all | `truncated` |

[ipc_protocol.md](../architecture/api/ipc_protocol.md), "Answer Protocol",
carries the same grammar with the buffering threshold and the failure cases
beside it.
<!-- source: pkg/plugin/rpc/message.go -- AppendAnswerHead, AppendAnswerTerminator -->

| Method | The plugin | The engine |
|--------|-----------|-----------|
| `dispatch-command` | reads the answer | writes it |
| `dispatch-command-args` | reads the answer | writes it |
| `execute-command` | writes the answer | reads it |

One encoding covers both columns, on every connection. Nothing is declared and
nothing is negotiated, so a plugin author sets no field and reaches no option.
<!-- source: pkg/plugin/rpc/types.go -- DeclareCapabilitiesInput -->
<!-- source: pkg/plugin/rpc/answer_write.go -- WriteRecordAnswer, WriteDocumentAnswer -->

A plugin READS an engine answer in one of two ways, and both read the same
frame. `Plugin.DispatchCommandAnswer` yields each row as it arrives, which is
what bounds the memory of a walk over a large table. `Plugin.DispatchCommand`
and `Plugin.DispatchCommandArgs` collapse the same answer into the one document
a caller that wants the whole payload reads.
<!-- source: pkg/plugin/sdk/sdk_engine.go -- Plugin.DispatchCommandAnswer, Plugin.DispatchCommand, dispatchCommandValue -->

A plugin WRITES its own answer to `execute-command`, and the frame is the same
whatever the payload is. A handler that returns a `plugin.Records` writes one
line for each row of the walk. A handler that returns a built value writes that
value as the one record of a `doc` answer, byte for byte.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- executeCommandAnswer -->
<!-- source: pkg/plugin/records.go -- Records, Records.WriteAnswer -->

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
| `pipes` | `[]PipeDecl` | CLI pipe aliases the plugin names for its own commands |
| `claims` | `[]string` | Exclusive runtime roles the plugin takes over |


Each `CommandDecl` has these fields:
<!-- source: pkg/plugin/rpc/types.go -- CommandDecl -->
<!-- source: internal/component/plugin/server/startup.go -- validateShapeDecls, validateHelpDecls, registerPluginShapes -->

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Command path the plugin serves |
| `description` | `string` | The one-line SUMMARY. Every surface that shows the command on one line reads it. One line, at most 256 bytes, no control character |
| `long-help` | `string` | The LONG explanation the command's own help page prints. At most 4096 bytes, newlines kept, every other control character refused |
| `args` | `[]string` | Expected argument names, for help and completion |
| `completable` | `bool` | Whether the command supports tab completion |
| `hidden` | `bool` | Whether the command is left out of help and completion |
| `deprecated-names` | `[]string` | Older spellings that still reach this command |
| `shape` | `string` | What the answer holds: `doc`, `map` or `tab` |
| `columns` | `[]string` | The answer's keys, in the order a person reads them. Needs a `shape` that has rows. Maximum 64 |
| `address-fields` | `[]string` | The keys whose value holds an IP address or a prefix. Needs a `shape`. Maximum 16 |

The last three are optional and additive. A plugin that sends none keeps the
behavior it had before they existed. A plugin that sends one the engine refuses
fails Stage 1 and does not start.

`description` and `long-help` are two texts and neither is derived from the
other. The key is `long-help` and not `help`, because `help` already names the
SUMMARY in a completion row on this same protocol. A plugin that sends
`description` and no `long-help` renders with its summary and an empty
explanation. That is what every plugin written before `long-help` existed
sends. An empty `long-help` NEVER renders as an empty summary.

`validateHelpDecls` reads both texts before any conversion. It refuses a text
past its bound, and a control character the text's shape does not allow. The
summary reaches the tab-separated shell-completion format and the one-line
terminal candidate. A newline, a tab or an ESC in it breaks the format for
every row that follows. The alias `description` below is held to the same
one-line rule.

Each `PipeDecl` has these fields:
<!-- source: pkg/plugin/rpc/types.go -- PipeDecl -->
<!-- source: internal/component/plugin/server/startup.go -- validatePipeDecls, registerPluginPipes -->

| Field | Type | Description |
|-------|------|-------------|
| `command` | `string` | Command path the alias sits on. MUST be one of this plugin's own declared commands |
| `name` | `string` | The word an operator types after the pipe character (kebab-case, 1-64 chars) |
| `description` | `string` | The one-line summary completion and `command help` show beside the name. At most 256 bytes, no control character |
| `expansion` | `string` | The operator chain the name stands for, as an operator would type it |

A pipe alias SELECTS and re-sequences the answer the command already returned.
It renames no key, sums no numbers and counts no rows, so the command MUST emit
the aggregate fields beside the detail rows. One bad entry refuses the whole
list and fails the plugin's startup. Read
`docs/architecture/api/commands.md` for the collision rules and the payload
obligation.

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
<!-- source: pkg/plugin/sdk/sdk.go -- Plugin.Run, Stage 3 declare-capabilities -->

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
| `ze-plugin-callback:execute-command` | `OnExecuteCommand` | `ExecuteCommandInput` | a record answer | Run a command |
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
   |-- #43 top doc 0: 0: ---------------------------->|  head: doc, no envelope,
   |                                                  |  no column names
   |-- #43 row 26:{"running":true,"peers":1} -------->|  26 BYTES of payload
   |-- #43 end 1 0 0: ------------------------------->|  1 produced, 0 rejected,
   |                                                  |  no message
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
