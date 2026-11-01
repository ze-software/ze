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
| `wrn` | the terminator of a command that completed and states a warning |
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
| `wrn`, no rejected row and no message | `warning` |
| records and rejected rows, no message | `partial` |
| a message over no record | `error` |
| a message over records | `aborted` |
| no terminator at all | `truncated` |

When the engine cannot build the answer to a `dispatch-command`, for example
because the command's payload is not a JSON object or array, it still writes a
head and a terminator whose message names the reason. Your plugin reads
`error` with that message rather than waiting for its timeout.

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
#1 ze-plugin-engine:declare-registration {"protocol-version":1,"families":[{"name":"ipv4/flow","mode":"both","afi":1,"safi":133}]}

# Engine answers the protocol version both sides speak
#1 ok {"protocol-version":1}

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
| `protocol-version` | `int` | REQUIRED. The highest plugin protocol version the plugin speaks (this engine speaks `1`) |
| `families` | `[]FamilyDecl` | Address families the plugin handles (name, mode, AFI, and SAFI) |
| `commands` | `[]CommandDecl` | Commands the plugin provides |
| `dependencies` | `[]string` | Plugin names that must also be loaded |
| `wants-config` | `[]string` | Config roots the plugin wants to receive |
| `config-operations` | `[]ConfigOperationDecl` | Config operation callbacks the plugin supports, and the operation labels it emits for each root. The labels are the plugin's own: the engine keeps no list of them |
| `verify-budget` | `int` | Estimated verify time in seconds (`0` means trivial) |
| `apply-budget` | `int` | Estimated apply time in seconds (`0` means trivial) |
| `schema` | `*SchemaDecl` | YANG schema (module, namespace, yang-text, handlers) |
| `wants-validate-open` | `bool` | Whether plugin wants OPEN validation callbacks |
| `cache-consumer` | `bool` | Whether plugin consumes cached events |
| `cache-consumer-unordered` | `bool` | Whether unordered cache delivery is acceptable |
| `filters` | `[]FilterDecl` | Named route filters the plugin provides: each a `name` or a `name-pattern`, and the route fields it `reads` (`raw` for the UPDATE body) and `sets`; Stage 1 and config load judge them (`docs/architecture/api/process-protocol.md`, "Filter Declaration") |
| `doctor-checks` | `[]DoctorCheckDecl` | Doctor checks the plugin provides |
| `resolver-caches` | `[]string` | Resolver caches the plugin holds clients for: `peeringdb`, `irr`, `cymru`. Set by `OnResolverCacheClear`; any other name is refused |
| `enrichers` | `[]EnricherDecl` | Show enrichers the plugin provides |
| `pipes` | `[]PipeDecl` | CLI pipe aliases the plugin names for its own commands |
| `claims` | `[]string` | Exclusive runtime roles the plugin takes over |
| `failure-policy` | `string` | What ze does when this plugin fails: `restart`, `ignore` or `fatal`. Omitting it means `ignore` |

**The failure policy is what your plugin asks ze to do when your plugin fails.**
It is voluntary, and a plugin that omits it is carried on without. A value that
is none of the three fails the whole registration, so ze never guesses.

| You declare | ze does | May ze start your plugin again? |
|-------------|---------|---------------------------------|
| `restart` | starts the plugin again, bounded at 5 in 60 seconds and 20 in the life of the daemon | Yes |
| `ignore` | logs it and carries on without the plugin | No |
| `fatal` | stops ze | No |

`fatal` is open to any plugin, whether ze ships it or you wrote it (owner
directive, 2026-09-06). An operator who configures your plugin accepts its
terms.

Declaring anything other than `restart` also says your plugin must NOT be
started again. An operator whose `plugin { external <name> }` block writes
`respawn true` for such a plugin gets a daemon that refuses to start, with an
error naming your plugin and the policy it declared. Declare `restart` when
your plugin can come back from a crash, which usually means its state lives
outside the process.



Each `CommandDecl` has these fields:
<!-- source: pkg/plugin/rpc/types.go -- CommandDecl -->
<!-- source: internal/component/plugin/server/startup.go -- validateShapeDecls, validateHelpDecls, registerPluginShapes -->

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Command path the plugin serves |
| `description` | `string` | The one-line SUMMARY. Every surface that shows the command on one line reads it. One line, at most 256 bytes, no control character |
| `description` | `string` | The LONG explanation the command's own help page prints. At most 4096 bytes, newlines kept, every other control character refused |
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

`short-help` and `description` are two texts and neither is derived from the
other. The key is `description` and not `help`, because `help` already names the
SUMMARY in a completion row on this same protocol. A plugin that sends
`description` and no `description` renders with its summary and an empty
explanation. That is what every plugin written before `description` existed
sends. An empty `description` NEVER renders as an empty summary.

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
#1 ze-plugin-engine:declare-registration {"protocol-version":1,"families":[{"name":"ipv4/flow","mode":"both","afi":1,"safi":133}],"commands":[{"name":"flowspec status","short-help":"Show FlowSpec status"}],"wants-config":["bgp"]}
#1 ok {"protocol-version":1}
```

**The protocol version.** The engine answers Stage 1 with the version both
sides speak from then on: the lower of the plugin's `protocol-version` and its
own. A plugin declaring a version above the engine's speaks the answered
version. A declaration with no `protocol-version` is from the text era, before
filters declared the route fields they read, and the engine refuses it with
one error that names the plugin and the version to declare; the startup tier
then stops, as for any plugin failure. Version `1` is the first versioned
contract. The Go SDK declares it and refuses an engine that answers any other.
<!-- source: pkg/plugin/rpc/declaration.go -- NegotiateProtocolVersion -->
<!-- source: internal/component/plugin/server/startup_driver.go -- runStartupHandshake -->
<!-- source: pkg/plugin/sdk/sdk_version.go -- acceptProtocolVersion -->

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

**Refusing the configuration.** Answer with `#1 error` and the reason when you
cannot take what you were sent. That answer is a refusal: ze stops your plugin,
and ze itself stops when your registration carries `FatalOnConfigError`, so an
operator who mistyped a value is told rather than left with a router silently
missing your feature. Return an error from your `OnConfigure` handler and the
Go SDK writes that response for you.

```
#1 ze-plugin-callback:configure {"sections":[...]}
#1 error {"code":"error","message":"address family \"ipv6-unicat\" is not a family"}
```

Never answer `ok` for a configuration you rejected, and never just close the
connection: a lost connection is a transport failure, which says nothing about
the configuration and so never stops ze.

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

## Query Mode: Answering Without Starting

Ze can start your plugin to ask what it declares, with no daemon running and no
hub to connect to. It sets `ZE_PLUGIN_MODE=declare` in your process's
environment. Your plugin writes its Stage 1 declaration to stdout, exits 0, and
does none of a live start's work: no data initialization, no connection, no
bind, no listener, no timer.

### What you call

`sdk.RunOrDeclare` takes your declaration and your start function as two
arguments and returns the exit code:

```go
// declaration is what this plugin provides. It is a pure value: no file, no
// socket, no goroutine.
func declaration() *sdk.Registration {
    return &sdk.Registration{
        Commands: []sdk.CommandDecl{{Name: "acme-monitor status", ShortHelp: "Show monitor status"}},
    }
}

func main() {
    os.Exit(sdk.RunOrDeclare(declaration(), start))
}

// start does everything a live start does, and query mode never calls it.
func start() int {
    p, err := sdk.NewFromEnv("acme-monitor")
    if err != nil {
        return 1
    }
    defer p.Close()
    // ... register callbacks, open files, dial, start timers ...
    if err := p.Run(context.Background(), declaration()); err != nil {
        return 1
    }
    return 0
}
```

The declaration is the same `*sdk.Registration` you pass to `Plugin.Run`,
so query mode reads nothing new and adds no second list to keep in step.
<!-- source: pkg/plugin/sdk/sdk_query.go -- RunOrDeclare -->

### What the SDK guarantees

- Your start function is NOT called under the mode. Side-effecting code inside
  it is unreachable during a query, not merely discouraged.
- The line written is the Stage 1 `declare-registration` message in the
  protocol's own framing, so the reader parses one format.
- A declaration naming no command and no pipe still writes the line. "Declared
  nothing" and "sent nothing" are different answers.

### What it does not guarantee

- Work your `main` does BEFORE the call runs under a query too. The SDK cannot
  reach it. Put every side effect inside the start function.
- `Plugin.Run` derives `wants-validate-open` from the callbacks you registered.
  A query has no `Plugin`, so state that field in the declaration you pass here
  when you want it in the answer.
- A plugin that adopts neither this entry point nor the `ze plugin` route is
  reported as having sent nothing. Ze cannot make a binary it does not compile
  inert, so for third-party code this is enforcement for adopters and convention
  for everyone else.
- `QueryModeRequested()` reports the mode for a plugin that has its own reason
  to branch. It is a reader, not a guarantee: the branch is yours to place, and
  anything above it has already run.
<!-- source: pkg/plugin/sdk/sdk.go -- QueryModeRequested, EnvPluginMode, ModeDeclare -->

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
| `ze-plugin-callback:config-rollback` | `OnConfigRollback` | `{"transaction-id":"..."}` | None | Roll back a config transaction. Reaches the handler only while an apply this plugin accepted is neither committed nor rolled back |
| `ze-plugin-callback:config-committed` | `OnConfigCommitted` | `{"transaction-id":"..."}` | None | The transaction committed; the SDK stops owing a rollback for its apply. Reaches the handler only while an apply this plugin accepted is neither committed nor rolled back |
| `ze-plugin-callback:config-operation-decompose` | `OnConfigOperationDecompose` | `ConfigOperationDecomposeInput` | `ConfigOperationDecomposeOutput` | Decompose a config transaction |
| `ze-plugin-callback:config-operation-verify` | `OnConfigOperationVerify` | `ConfigOperationVerifyInput` | `ConfigOperationVerifyOutput` | Verify a config operation |
| `ze-plugin-callback:config-operation-apply` | `OnConfigOperationApply` | `ConfigOperationApplyInput` | `ConfigOperationApplyOutput` | Apply a config operation |
| `ze-plugin-callback:config-operation-rollback` | `OnConfigOperationRollback` | `ConfigOperationRollbackInput` | `ConfigOperationRollbackOutput` | Roll back config operations |
| `ze-plugin-callback:config-operation-commit` | `OnConfigOperationCommit` | `ConfigOperationCommitInput` | `ConfigOperationCommitOutput` | Commit reversible operation state; retain undo through whole-reload acceptance |
| `ze-plugin-callback:config-complete` | `OnConfigComplete` | `ConfigCompleteInput` | `ConfigCompleteOutput` | Acknowledge cleanup for the named original transaction after acceptance or completed abort |
| `ze-plugin-callback:source-policy` | `OnSourcePolicy` | `rpc.SourcePolicyInput` | `rpc.SourcePolicyOutput` | Settle prepare/apply/abort/finalize for a captured retained source |
| `ze-plugin-callback:validate-open` | `OnValidateOpen` | `ValidateOpenInput` | `ValidateOpenOutput` | Validate an OPEN message |
| `ze-plugin-callback:filter-update` | `OnFilter` (or `OnFilterUpdate`, its adapter) | `FilterUpdateBatchInput` (`routes`, each a `FilterUpdateInput`) | `FilterUpdateBatchOutput` (`verdicts`, each a `FilterUpdateOutput`) | Filter a batch of routes, one verdict per route in request order ("The filter-update batch"). The engine waits `rpc.FilterUpdateTimeout` (5 s) for the whole batch, then applies the filter's `on-error` policy to each of its routes, so a handler that waits MUST bound the wait well below it |
| `ze-plugin-callback:doctor-check` | `OnDoctorCheck` | `DoctorCheckInput` | `DoctorCheckOutput` | Run a doctor check |
| `ze-plugin-callback:resolver-cache-clear` | `OnResolverCacheClear` | `ResolverCacheClearInput` | `ResolverCacheClearOutput` | Clear a declared resolver cache of the plugin's current clients, answering the entries removed |
| `ze-plugin-callback:enrich-show` | `OnEnrichShow` | `EnrichShowInput` | `EnrichShowOutput` | Add data to show output |
| `ze-plugin-callback:post-startup` | `OnAllPluginsReady` | None | None | Signal that all plugins are ready |
| No wire method (DirectBridge only) | `OnStructuredEvent` | `[]any` of `*rpc.StructuredEvent` | None | Deliver structured events without JSON |
| `ze-plugin-callback:bye` | `OnBye` | `ByeInput` | None | Notify the plugin of its live removal or a subsystem stop. A daemon stop sends none: it closes the connection |
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigComplete, operation callback registration -->
<!-- source: pkg/plugin/rpc/types.go -- ConfigCompleteInput, ConfigCompleteOutput -->

An `OnConfigComplete` handler automatically declares `config-completion` in
Stage 1. Its context carries the absolute `deadline-ms` work bound. Complete
cleanup before returning: callback queue admission is not a receipt. Keep
operation journals through `OnConfigOperationCommit`; discard only the matching
transaction on accepted completion. Rejected completion follows original undo,
after section-owned configuration and filters have been restored. A completion
error after acceptance reports an active candidate, not a rollback.
Requests and receipts MUST carry an explicit JSON boolean `accepted`, including
`false` for rejection. Missing or `null` decisions never invoke SDK cleanup and
never acknowledge completion to the engine; they are malformed messages.
<!-- source: pkg/plugin/sdk/sdk.go -- Plugin.Run -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigComplete -->
<!-- source: internal/component/plugin/server/reload_tx.go -- completeBatches, finishWithLock -->

Plugins retaining source state MUST declare `source-policy-owner` in Stage 1,
even if they do not implement the lifecycle yet. That unsupported declaration
refuses policy changes before publication; pure observers are not owners.
In Go, a plugin with no handler sets `SourcePolicyOwner: true` in the
`sdk.Registration` it passes to `Run`; the engine then refuses the change and
tells the operator to take the peer down around it.
`OnSourcePolicy(func(context.Context, rpc.SourcePolicyInput) error)` automatically
declares both ownership and `source-policy` handler support. All four operations
use one registered callback on both transports, not the shutdown `OnDrain` path.

Requests carry a transaction, operation, captured source identity, target epoch
and absolute deadline. The engine sends the earlier of its caller's budget and
the request deadline, so the handler's context never outlives the caller. The
source identity comprises peer, peer incarnation,
session generation, original policy epoch, receive frontier and template
generation. Every field MUST be present and non-null; the last two may be zero.
Prepare/finalize require explicit target zero. Apply requires a fresh target
newer than the captured epoch. Abort carries target zero when the transaction was
prepared and never applied: release your prepare pins and change nothing else (no
restamp, no eligibility change, no downstream write). After apply, abort carries a
fresh target newer than the captured epoch, and you restamp restored state to it.
Refuse a zero abort after apply. The coordinator enforces monotonicity between calls.
The SDK rejects malformed, unknown and expired requests before calling the owner.

The engine strictly drains event application before prepare, but the handler
MUST additionally fence its own workers and downstream writes. It MUST honor the
context deadline and return only after the requested work settles. Keep prepare
pins through apply; abort restores them (a zero-target abort only releases them),
while finalize releases accepted pins.
The SDK returns an exact identity/target receipt only after the handler returns.
The engine checks the captured process and connection again after acknowledgement.
Neither this callback transport nor copied epoch fields alone invalidate routes.

The handler does not run on the callback loop. The Go SDK starts one
source-policy worker for the plugin's connection and stops it with the plugin,
on both transports, so a handler may wait for events delivered after its
request, or call the engine during prepare or apply, without blocking the loop
that delivers them. What that means for your handler:

- It runs concurrently with `OnEvent` and the other callbacks. Protect the
  state they share. Every event delivered before the request has already been
  applied when it starts.
- Calls never overlap and arrive in the order the engine sent them, so one
  source's prepare, apply, abort and finalize reach you in order.
- Its context ends at the request deadline or when the plugin stops. A request
  that expired while queued fails without calling you.
- At most 16 requests wait behind the running one. A request beyond that is
  answered at once with an error receipt saying the queue is full, and your
  handler never sees it.
- A panic fails the request, and nothing runs after it: every request already
  queued gets a failed receipt, then the plugin's connection ends with that
  error.
  A plugin written in another language owes the same: answer `source-policy`
  without stopping its event delivery.
<!-- source: pkg/plugin/sdk/sdk_source_policy.go -- SourcePolicyHandler, sourcePolicyWorker, sourcePolicyQueueDepth -->

Retained/replayed received routes MUST preserve their original
`StoredRoute.SourcePolicyEpoch` (`source-policy-epoch`), never stamp the current
epoch onto old work. Zero is unrecorded, independently of session generation.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnSourcePolicy -->
<!-- source: pkg/plugin/rpc/source_policy.go -- SourcePolicyInput.Validate, UnmarshalJSON -->
<!-- source: internal/component/plugin/server/source_policy.go -- CaptureSourcePolicyOwners, SourcePolicyOwner.Call -->


A peer's AS is not always known when an event is written. A dynamic peer's AS
comes from its OPEN, so the OPEN Ze sends, a NOTIFICATION Ze sends in
OpenSent and the FSM changes before OpenConfirm carry none. The JSON peer
object then omits `remote.as`, a text line says `remote as unknown`, and
`StructuredEvent.PeerAS` is the zero `rpc.RemoteAS`, which means "not yet
known" and never AS 0. Read it with `PeerAS.ASN()`, which answers
`(uint32, bool)`, and branch on the bool. An UPDATE always names the AS.
Rendered without a branch, it writes the number or `unknown` through `%v`, `%s`
and `log/slog`, and the number or `null` through `encoding/json`, where a field
tagged `omitzero` is omitted while the AS is unknown.
<!-- source: pkg/plugin/rpc/remote_as.go -- RemoteAS, ASN, MarshalJSON, IsZero, String, LogValue -->

A session-down event names the session it ended: `session-generation` in the
JSON and text forms, `StructuredEvent.SessionGeneration` in the structured
one, a nonzero number captured when that session closed. An up carries none.
A forwarding plugin that asks the engine to withdraw or replace a departed
peer's routes (`recovery-source` meta on its `update` command) MUST name that
session as `recovery-generation` meta, a decimal string: the engine refuses a
departure without it, and with it owes only routes that session or an older
one wrote, so a down handled after the peer reconnected never takes the newer
session's routes (`docs/architecture/plugin/rib-storage-design.md`,
"Source-DOWN replacement and sent ordering").
<!-- source: pkg/plugin/rpc/bridge.go -- StructuredEvent -->
<!-- source: internal/component/bgp/plugins/cmd/update/recovery.go -- recoveryFromMeta -->

### The filter-update batch

One `filter-update` exchange carries a batch of routes. The request is
`{"routes":[...]}`, each entry one route as `FilterUpdateInput` describes it
(`filter`, `direction`, `peer`, `peer-as`, `update`, `fields`, `raw`). The
answer is `{"verdicts":[...]}`, one `FilterUpdateOutput` per route, at the
position of its route.

```
#7 ze-plugin-callback:filter-update {"routes":[{"filter":"lp","direction":"import","peer":"192.0.2.1","peer-as":65001,"update":"","fields":{"prefix":["10.0.0.0/24"]}},{"filter":"lp","direction":"import","peer":"192.0.2.1","peer-as":65001,"update":"","fields":{"prefix":["10.0.1.0/24"]}}]}
#7 ok {"verdicts":[{"action":"reject"},{"action":"modify","sets":[{"field":"med","action":"set","value":50}]}]}
```

- **What a batch holds.** Ze sends the routes queued for the plugin when the
  exchange starts, at most `rpc.FilterUpdateBatchMax` (64). A lone route goes
  at once in a batch of one; routes that queue while an exchange is in flight
  go together in the next one. Routes from one source are queued in the order
  that source produced them, and a later batch holds only routes queued after
  the earlier one was sent, so per-source order holds within and across
  batches.
- **One timeout per batch.** `rpc.FilterUpdateTimeout` (5 s) bounds the whole
  exchange. When it expires, or the plugin answers an error, each route of the
  batch takes its filter's `on-error`, one outcome per route.
- **One verdict per route.** A reply with more or fewer verdicts than the
  request held routes is malformed: no verdict of it is applied, and each
  route of the batch takes its filter's `on-error`. The next batch is judged
  normally.
- **A route the filter cannot judge.** A verdict carrying `error` (a string)
  fails that route alone under its filter's `on-error`; the other routes of
  the batch keep their verdicts. The Go SDK answers a handler's returned error
  this way: `OnFilterUpdate` calls the handler once per route, in request
  order, so a Go filter is written for one route.
<!-- source: pkg/plugin/rpc/types.go -- FilterUpdateBatchMax, FilterUpdateBatchInput, FilterUpdateBatchOutput -->
<!-- source: internal/component/plugin/ipc/filter_batch.go -- SendFilterUpdate, filterBatcher -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnFilterUpdate -->

### The Route Fields of filter-update

`FilterUpdateInput.Fields` (`fields` on the wire) is one JSON object holding the
route fields the filter declared in `reads`, and no other. Each key is the
vocabulary field name, never the UPDATE event's `attr` key: `community`, not
`communities`. Each value takes the shape the event's `attr` map gives that
attribute (`docs/architecture/api/json-format.md`, "Attributes"), written by
the same formatter. The fields the `attr` table has no row for take these
shapes:

| Field | Shape | Example |
|-------|-------|---------|
| `prefix` | object: family to the list of announced NLRI, in the event's NLRI shapes (ADD-PATH adds `path-id`) | `{"ipv4/unicast":["10.0.0.0/24"]}` |
| `family` | list of the announced families | `["ipv4/unicast","ipv6/unicast"]` |
| `next-hop` | object: family to the next hop of its announced routes | `{"ipv4/unicast":"192.0.2.1"}` |
| `protocol` | `"ebgp"` or `"ibgp"`: the session the route came from (import) or goes to (export) | `"ebgp"` |
| `origin-as` | the origin AS number, or `"none"` for RFC 6811's NONE | `65002` |

For example, a filter that declared `community`, `as-path` and
`local-preference` reads
`"fields":{"community":["65001:100"],"as-path":[65001,65002],"local-preference":100}`.

- A declared field the route does not carry is left out of the object.
- A filter that declared no route field (a raw-only filter) is sent no
  `fields` member, and `raw` is sent only to a filter that declared `raw`.
- A field Ze cannot read, such as a malformed attribute, is never sent empty:
  the filter's `on-error` decides the route instead of the filter.
- Stage 1 refuses a declared BGP field this binary cannot write: the
  community fields need `bgp-filter-community`, `originator-id` and
  `cluster-list` need `bgp-rr`, and `aigp` needs `bgp-aigp`.
- In a chain that holds only plugin filters, the values are the route as the
  chain received it: a modify an earlier filter answered reaches this filter
  in the text `update` only, as it does for `raw`.
- In a chain that also holds an in-process filter, the values are the route
  as the earlier members left it: a local-preference, med or community an
  earlier member set, in process or in a plugin, is what this filter reads.
  `raw` is still the received body. Such a chain sends no text `update`, so a
  filter that declared no `reads` fails the route under its `on-error`.
<!-- source: internal/component/bgp/policyview/fields_json.go -- AppendFields, CheckFieldWriter -->
<!-- source: internal/component/bgp/policyview/register_writers.go -- the field writers -->
<!-- source: internal/component/bgp/format/announced_json.go -- AppendAnnouncedNLRIJSON -->
<!-- source: internal/component/bgp/reactor/filter_chain.go -- filterFields -->
<!-- source: internal/component/bgp/policychain/external.go -- judgeExternal -->

### The Sets of a filter-update answer

A filter in a chain that also holds an in-process filter answers its changes
in `FilterUpdateOutput.Sets` (`sets` on the wire), with `"action":"modify"`.
Each entry names one field the filter declared in `sets`, how it changes, and
the value in the field's shape:

| Field | Actions | Value | Example |
|-------|---------|-------|---------|
| `local-preference` | `set`, `increment`, `decrement` | a number from 0 to 4294967295 | `{"field":"local-preference","action":"set","value":200}` |
| `med` | `set`, `increment`, `decrement` | a number from 0 to 4294967295 | `{"field":"med","action":"increment","value":10}` |
| `community` | `add`, `remove` | one community string | `{"field":"community","action":"add","value":"65000:100"}` |
| `as-path` | `prepend` | a count of local AS copies, 1 to 32 | `{"field":"as-path","action":"prepend","value":2}` |
| `as-path` | `remove` | `"private"` removes the private-use AS numbers; `"private-peer-as"` replaces each with the peer AS | `{"field":"as-path","action":"remove","value":"private"}` |
| `prefix` | `remove` | one announced prefix, in the shape the `prefix` field is sent: the prefix string, or under ADD-PATH `{"prefix":...,"path-id":...}` | `{"field":"prefix","action":"remove","value":"10.0.9.0/24"}` |
| `family` | `remove` | one family name | `{"field":"family","action":"remove","value":"ipv6/unicast"}` |

- An increment or decrement starts from the route's value, or from
  `bgp defaults attribute` when the route carries none, and stays within 0 and
  4294967295. The changes fold in chain order with the in-process filters'
  changes and produce the same UPDATE bytes.
- The route fails under the filter's `on-error`, logged and counted, with none
  of its sets applied, when an entry names a field the filter did not declare,
  a field the chain's attachment does not let a filter set, or a field no BGP
  chain sets; when the action is not one the field takes; or when the value has
  the wrong shape. The other routes are judged on their own.
- The `as-path`, `prefix` and `family` entries are intents Ze resolves with
  state the plugin does not hold: a prepend uses the local AS and the
  session's AS number width, `"private-peer-as"` the peer AS, a prefix remove
  drops the entry from the body NLRI and from MP_REACH_NLRI (an MP_REACH_NLRI
  left empty leaves the UPDATE), and a family remove drops that family's
  MP_REACH_NLRI and MP_UNREACH_NLRI (the body NLRI and withdrawn routes for
  `ipv4/unicast`). Prepends add up across the chain to at most 32. A prefix
  the route does not announce, or one with host bits set, fails the route.
- The route fails the same way when the answer carries `sets` beside `accept`
  or `reject`, or a text `update`.
- A filter that declares `raw` among its reads may answer `"action":"modify"`
  with a whole-body `raw` rewrite and no `sets`. The rewrite is terminal: the
  later filters of the chain are not called, the chain's final still decides
  the route, and the rewrite is sent in place of the received body. The route
  fails under the filter's `on-error` when the filter did not declare `raw`,
  when the rewrite is shorter than the 4 octets of an UPDATE body's two length
  fields, and when an earlier filter of the chain already set a field: the
  rewrite was made from the received body, so it would discard that change.
- `teardown` on an import route closes the session, as in a chain of plugin
  filters only; on export it is ignored and the action stands.
<!-- source: internal/component/bgp/policychain/external.go -- externalAnswer, applyExternalSets, rawAnswer -->
<!-- source: internal/component/bgp/policychain/chain.go -- rawOutcome -->
<!-- source: internal/component/bgp/policychain/external_intents.go -- applyASPathIntent, applyPrefixIntent, applyFamilyIntent -->
<!-- source: pkg/plugin/rpc/types.go -- FilterSet, FilterSetAction -->

### The Filter Text an `OnFilterUpdate` Handler Reads

`FilterUpdateInput.Update` is the UPDATE as text: `<name> <value>` attribute
pairs, then one NLRI block for each section the UPDATE carries.

| Block | UPDATE section |
|-------|----------------|
| `nlri add <prefix>...` | The legacy NLRI field, always IPv4 unicast |
| `nlri del <prefix>...` | The legacy Withdrawn Routes, always IPv4 unicast |
| `mp <family> add <prefix>...` | An MP_REACH_NLRI attribute, any family, IPv4 unicast included |
| `mp <family> del <prefix>...` | An MP_UNREACH_NLRI attribute |

For example: `origin igp as-path [65001] nlri add 10.0.0.0/24 mp ipv6/unicast add 2001:db8::/32`.
Split the NLRI value at every `nlri` and `mp` token, and read the family only
from an `mp` block. A family whose NLRI is not a CIDR prefix renders a marker
block with no prefix (`mp l2vpn/evpn add`).

A `FilterModify` answer that changes NLRI returns the whole NLRI value, in the
same keywords, with prefixes removed and the rest in the order given. The
engine refuses, and drops the route, an answer that adds or reorders a prefix,
names a section the UPDATE does not carry, or changes a non-CIDR block. The
full contract is in `docs/architecture/api/process-protocol.md`, "NLRI Blocks
and the Modify Answer".
<!-- source: internal/component/bgp/reactor/filter_format.go -- AppendUpdateForFilter -->
<!-- source: internal/component/bgp/reactor/filter_nlri_delta.go -- sectionNLRIDelta -->

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
   |  REMOVAL: bye                                    |
   |<- #99 ze-plugin-callback:bye {"reason":"..."} ---|
   |-- #99 ok ---------------------------------------->|
   |  (plugin exits)                                  |
```

A daemon stop sends no `bye`: the engine closes the connection, and the
plugin's `Run` returns nil (`docs/architecture/api/process-protocol.md`,
"Shutdown").

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
with `{"status":"ok"}` or `{"status":"error","error":"..."}`. A plugin with no
`OnConfigVerify` handler accepts at config-verify. A plugin that registered neither
`OnConfigApply` nor `DeclareAppliesNothing` answers both with an error naming the
plugin and the roots, so the reload is refused rather than accepted and ignored
(handlers.md, "A plugin with no apply handler").
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigVerify, OnConfigApply, initCallbackDefaults, configVerifyCallback, refuseConfigApply -->

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
