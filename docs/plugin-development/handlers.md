# Plugin Handlers

Handlers process engine-initiated callbacks during startup and at runtime.
They are registered on the `sdk.Plugin` before calling `Run()`.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- all On* methods -->

## Registration

Register handlers before calling `Run()`:

```go
p := sdk.NewWithConn("my-plugin", conn)

// Startup handlers (named handler types defined in sdk package)
p.OnConfigure(func(sections []sdk.ConfigSection) error { ... })
p.OnShareRegistry(func(commands []sdk.RegistryCommand) { ... })

// Runtime handlers
p.OnEvent(func(event string) error { ... })
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) { ... })
p.OnConfigVerify(func(sections []sdk.ConfigSection) error { ... })
p.OnConfigApply(func(diffs []sdk.ConfigDiffSection) error { ... })
p.DeclareAppliesNothing("why a reload has nothing for this plugin to apply")
p.OnValidateOpen(func(input *sdk.ValidateOpenInput) *sdk.ValidateOpenOutput { ... })
p.OnBye(func(reason string) error { ... })

// Post-startup callback (safe to make engine calls here)
p.OnStarted(func(ctx context.Context) error { ... })

// NLRI handlers
p.OnEncodeNLRI(func(family string, args []string) (string, error) { ... })
p.OnDecodeNLRI(func(family string, hex string) (any, error) { ... })
p.OnDecodeCapability(func(code uint8, hex string) (any, error) { ... })

p.Run(ctx, &sdk.Registration{...})
```

## Handler Reference

### OnConfigure

Called during Stage 2 when the engine delivers config sections.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigure -->

```go
p.OnConfigure(func(sections []sdk.ConfigSection) error {
    for _, s := range sections {
        // s.Root: config root name (e.g., "bgp")
        // s.Data: JSON-encoded config data
        var cfg MyConfig
        if err := json.Unmarshal([]byte(s.Data), &cfg); err != nil {
            return fmt.Errorf("parse config: %w", err)
        }
        // Store config for later use
    }
    return nil
})
```

**ConfigSection fields:**
<!-- source: pkg/plugin/rpc/types.go -- ConfigSection -->

| Field | Type | Description |
|-------|------|-------------|
| `Root` | `string` | Config root name (e.g., `"bgp"`) |
| `Data` | `string` | JSON-encoded config data |

Return `nil` to accept the config, or an error to reject (aborts startup).

### OnShareRegistry

Called during Stage 4 when the engine shares the command registry.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnShareRegistry -->

```go
p.OnShareRegistry(func(commands []sdk.RegistryCommand) {
    for _, cmd := range commands {
        // cmd.Name: command name (e.g., "peer")
        // cmd.Plugin: plugin that registered it
        // cmd.Encoding: encoding format
    }
})
```

**RegistryCommand fields:**
<!-- source: pkg/plugin/rpc/types.go -- RegistryCommand -->

| Field | Type | Description |
|-------|------|-------------|
| `Name` | `string` | Command name |
| `Plugin` | `string` | Plugin that registered it |
| `Encoding` | `string` | Encoding format |

This handler does not return a value. The registry is informational.

### OnEvent

Called at runtime when the engine delivers a BGP event.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnEvent -->

```go
p.OnEvent(func(event string) error {
    // event is a JSON string containing the BGP event
    var evt map[string]any
    if err := json.Unmarshal([]byte(event), &evt); err != nil {
        return fmt.Errorf("parse event: %w", err)
    }

    eventType, _ := evt["type"].(string)
    switch eventType {
    case "state":
        // Peer state change (up/down)
    case "update":
        // BGP UPDATE received
    case "sent":
        // Route sent to peer
    }
    return nil
})
```

Return `nil` to acknowledge, or an error to signal failure.

### OnExecuteCommand

Called at runtime when the engine routes a command to the plugin.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnExecuteCommand -->
<!-- source: pkg/plugin/rpc/types.go -- ExecuteCommandInput, ExecuteCommandOutput -->

```go
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (status string, data any, err error) {
    // serial: request correlation ID
    // command: the command name (e.g., "rib adjacent status")
    // args: additional arguments
    // peer: peer address (if command is peer-scoped)

    switch command {
    case "rib adjacent status":
        result, _ := json.Marshal(map[string]any{
            "running": true,
            "peers":   2,
        })
        return "done", string(result), nil

    default:
        return "", "", fmt.Errorf("unknown command: %s", command)
    }
})
```

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `serial` | `string` | Request correlation ID |
| `command` | `string` | Full command name |
| `args` | `[]string` | Additional arguments |
| `peer` | `string` | Peer address (may be empty) |

**Return values:**

| Return | Type | Description |
|--------|------|-------------|
| `status` | `string` | `"done"` or `"error"` |
| `data` | `any` | The value the command produced, marshaled once by the SDK, or an `sdk.Records` walk |
| `err` | `error` | Non-nil triggers error response |

A command that walks a large collection returns an `sdk.Records` for `data`. The
SDK then writes one line for each row rather than one document, so neither the
plugin nor the engine holds the whole answer. See
[commands.md](commands.md), "Success with Rows".
<!-- source: pkg/plugin/records.go -- Records, Records.WriteAnswer -->

### OnConfigVerify

Called during config reload to validate a candidate config. The plugin receives
the full candidate config sections and returns nil to accept or an error to reject.
If no handler is registered, config-verify accepts. Either way, a plugin that
registered neither `OnConfigApply` nor `DeclareAppliesNothing` is refused before
the handler runs: see "A plugin with no apply handler" below.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigVerify, configVerifyCallback -->
<!-- source: pkg/plugin/rpc/types.go -- ConfigVerifyInput -->

```go
p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
    for _, s := range sections {
        var cfg MyConfig
        if err := json.Unmarshal([]byte(s.Data), &cfg); err != nil {
            return fmt.Errorf("invalid config: %w", err)
        }

        // Validate semantic constraints
        if cfg.HoldTime > 0 && cfg.HoldTime < 3 {
            return fmt.Errorf("hold-time must be 0 or >= 3, got %d", cfg.HoldTime)
        }
    }
    return nil // Accept
})
```

Return `nil` to accept the candidate config, or an error to reject the reload.

### OnConfigApply

Called during config reload to apply config changes. The plugin receives diff
sections describing what changed (added, removed, changed) and returns nil to
accept or an error to reject. A plugin that declares `WantsConfig` must register
it or call `DeclareAppliesNothing`; the handler must not be nil, and the call
panics on a nil one.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigApply -->
<!-- source: pkg/plugin/rpc/types.go -- ConfigApplyInput, ConfigDiffSection -->

```go
p.OnConfigApply(func(diffs []sdk.ConfigDiffSection) error {
    for _, d := range diffs {
        // d.Root: config root name
        // d.Added: JSON-encoded added config (may be empty)
        // d.Removed: JSON-encoded removed config (may be empty)
        // d.Changed: JSON-encoded changed config (may be empty)

        if d.Added != "" {
            // Apply additions
        }
        if d.Removed != "" {
            // Apply removals
        }
        if d.Changed != "" {
            // Apply modifications
        }
    }
    return nil
})
```

**ConfigDiffSection fields:**
<!-- source: pkg/plugin/rpc/types.go -- ConfigDiffSection -->

| Field | Type | Description |
|-------|------|-------------|
| `Root` | `string` | Config root name (e.g., `"bgp"`) |
| `Added` | `string` | JSON-encoded added config |
| `Removed` | `string` | JSON-encoded removed config |
| `Changed` | `string` | JSON-encoded changed config |

### DeclareAppliesNothing

A plugin that declares `WantsConfig` but holds nothing a reload can apply says
so in place of an apply handler. Use it when the plugin reads no leaf of its
roots, or reads only capability leaves, whose change `DeclareCapabilitiesFrom`
already refuses at config-verify. Its `config-apply` is accepted, the rollback
gate opens as for any accepted apply, and a debug line names the plugin, the
changed roots and the reason. The reason must not be empty: the call panics on
an empty one. A plugin that reads a leaf it could apply registers
`OnConfigApply` instead.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- DeclareAppliesNothing -->

```go
p.DeclareCapabilitiesFrom(hostnameCapabilities, hostnameCapabilityLeaves)
p.DeclareAppliesNothing("reads only the hostname capability leaves, fixed for the process life")
```

### A plugin with no apply handler

A plugin that registers neither `OnConfigApply` nor `DeclareAppliesNothing`
cannot apply a reload, so the SDK refuses one rather than accept a change the
plugin would never act on. Its `config-verify` answers an error before any
`OnConfigVerify` handler runs, and so does its `config-apply` if one arrives;
the refused apply opens no rollback gate. The reload is refused and the running
config stays in force. The text names the plugin, the roots that changed, and
the restart that delivers the change at startup instead:

```
plugin <name> cannot apply a change to <roots> while ze runs: it registered no config apply handler and does not declare that it applies nothing, so it would accept the change and keep running the old config
```
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- configVerifyCallback, refuseConfigApply, refuseWithoutApply -->

Two consequences follow for a third-party plugin that registers only
`OnConfigure`:

- A plugin that declares a shared root, such as `WantsConfig: []string{"bgp"}`,
  takes part in every reload that changes anything under that root, so it
  refuses every edit there, including one to a leaf it never reads. Register
  `OnConfigApply` (or `sdk.NewConfigTx(...).Register(p)`) when it reads a leaf
  a reload can change, or `DeclareAppliesNothing` when it reads none.
- Removing the plugin's root is refused too. The engine still sends the plugin
  a `config-verify` for a root the candidate no longer holds, with the section
  data `{}`, and the SDK refuses it like any other change.
<!-- source: internal/component/plugin/server/reload_compensation.go -- reloadConfigSections -->

### OnValidateOpen

Called when the engine receives an OPEN message and wants the plugin to validate
the session. When this handler is registered, `WantsValidateOpen` is automatically
set to `true` in the Stage 1 registration.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnValidateOpen -->
<!-- source: pkg/plugin/rpc/types.go -- ValidateOpenInput, ValidateOpenOutput -->

```go
p.OnValidateOpen(func(input *sdk.ValidateOpenInput) *sdk.ValidateOpenOutput {
    // input.Peer: peer address
    // input.Local: local OPEN message (ASN, RouterID, HoldTime, Capabilities)
    // input.Remote: remote OPEN message

    // Check remote capabilities
    for _, cap := range input.Remote.Capabilities {
        if cap.Code == 64 { // Graceful Restart
            // Validate GR parameters
        }
    }

    // Accept or reject
    return &sdk.ValidateOpenOutput{Accept: true}

    // To reject:
    // return &sdk.ValidateOpenOutput{
    //     Accept:        false,
    //     NotifyCode:    2, // OPEN Message Error
    //     NotifySubcode: 6, // Unacceptable Hold Time
    //     Reason:        "hold time too low",
    // }
})
```

**ValidateOpenInput fields:**
<!-- source: pkg/plugin/rpc/types.go -- ValidateOpenInput -->

| Field | Type | Description |
|-------|------|-------------|
| `Peer` | `string` | Peer address |
| `Local` | `ValidateOpenMessage` | Local OPEN message |
| `Remote` | `ValidateOpenMessage` | Remote OPEN message |

**ValidateOpenMessage fields:**
<!-- source: pkg/plugin/rpc/types.go -- ValidateOpenMessage -->

| Field | Type | Description |
|-------|------|-------------|
| `ASN` | `uint32` | AS number |
| `RouterID` | `string` | Router ID (IP string) |
| `HoldTime` | `uint16` | Hold time in seconds |
| `Capabilities` | `[]ValidateOpenCapability` | Capabilities (code + hex value) |

**ValidateOpenOutput fields:**
<!-- source: pkg/plugin/rpc/types.go -- ValidateOpenOutput -->

| Field | Type | Description |
|-------|------|-------------|
| `Accept` | `bool` | `true` to accept, `false` to reject |
| `NotifyCode` | `uint8` | NOTIFICATION code (on reject) |
| `NotifySubcode` | `uint8` | NOTIFICATION subcode (on reject) |
| `Reason` | `string` | Human-readable reason (on reject) |

If no handler is registered, OPEN validation returns `{accept: true}` (no-op).

### Route filters

A filter has one evaluator for both transports. Register it with
`p.OnFilter(eval)`, where `eval` takes one `*sdk.FilterRoute` and answers the
verdict and the sets (`*sdk.FilterUpdateOutput`), or an error that fails that
route under the filter's `on-error`.

| The plugin runs | The SDK installs `eval` as | The route carries |
|-----------------|----------------------------|-------------------|
| Internal (a goroutine in the daemon) | The typed `DirectBridge` filter slot, at Stage 5 | The engine's lazy view: `Fields()` encodes the declared fields only when called, `Raw()` the raw body, `Engine()` the engine's typed view for a filter compiled with it |
| Forked (any process) | The `filter-update` batch handler | The decoded input: `Fields()` and `Raw()` answer what the message carried, `Engine()` is nil |

A filter that needs to know which session it judges declares the session
facts in its `reads` (`peer-name`, `peer-group`, `peer-address`, `local-as`,
`peer-as`, `route-server-as`, `ibgp`, `rs-client`, `llgr-advertised`, and on
export the `source-peer-*` facts), and reads them from `Fields()` like a route
field, on both transports (protocol.md, "The Route Fields of filter-update").
It never keeps its own map of peers for this.

The slot is callable only from Stage 5 until the bridge closes. A stopped,
crashed or respawning plugin is unavailable: the engine applies the filter's
`on-error` and never answers from the old plugin's state.

**The fast-path contract.** Internal, `eval` runs on the engine's goroutines,
concurrently. It MUST be non-blocking and bounded, MUST be safe for
concurrent calls, MUST NOT keep the route, or any slice it answered, past its
return, and MUST read its state from a snapshot its own goroutines swap
atomically. Data it has not resolved yet takes the filter's `on-error`; once
resolved, the plugin asks Ze to re-judge the routes, which at HEAD is
`request peer <peer> refresh <family>` (RFC 2918).

The SDK reuses one borrowed route wrapper within a batch, only after each
evaluator call returns. Decoded input bytes and collected verdicts remain in
separate storage; concurrent batch invocations do not share the wrapper.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- answerFilterBatch -->

**The conformance harness.** Every fast-path filter's test MUST pass
`sdk.CheckFilterConformance(t, name, eval, routes)` under `-race`. It refuses
a call past `sdk.FilterCallDeadline`, a repeat call that answers differently
(state written without a snapshot), a view read after its call returned or a
kept slice whose reuse changes an answer, a fast-path verdict that differs
from the forked transport's, and concurrent calls that answer otherwise,
each report naming the filter and the clause.

**Building the answer.** `eval` answers one `*sdk.FilterUpdateOutput` per
route, in one of three shapes. The wire form of each change, and every
reason a route fails, is `protocol.md`, "The Sets of a filter-update answer".

| Answer | `Action` | Carries | The filter declares |
|--------|----------|---------|---------------------|
| Keep or drop the route as received | `sdk.FilterAccept`, `sdk.FilterReject` | nothing else: `Sets` beside them fails the route | its `Reads` |
| Change named fields | `sdk.FilterModify` | `Sets`, one `rpc.FilterSet` per change | each field it changes in `Sets` |
| Replace the whole UPDATE body | `sdk.FilterModify` | `Raw`, the new body, and no `Sets` | `sdk.FilterFieldRaw` in `Reads` |

A field set holds the field name, an `rpc.FilterSetAction`, and the value as
JSON in the field's shape:

```go
out := &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: []rpc.FilterSet{
	{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage(`200`)},
	{Field: "community", Action: rpc.FilterSetActionAdd, Value: json.RawMessage(`"65000:100"`)},
}}
```

The `as-path`, `prefix` and `family` fields take intents rather than values,
because Ze resolves them with state the plugin does not hold:
`rpc.FilterSetActionPrepend` with a count, `rpc.FilterSetActionRemove` with
`"private"` or `"private-peer-as"` for the AS path, with one announced prefix
as the `prefix` field reads it, or with one family name. A prepend never names
an AS number, and a prefix remove never names an offset: Ze finds each entry
in the body NLRI and in MP_REACH_NLRI. Every set of a route fails with the
route when one of them is refused, so an answer never applies half its
changes.

A `Raw` rewrite is terminal. The later filters of the chain are not called,
and the rewrite is refused when an earlier filter already set a field, since
it was built from the received body. The engine keeps the rewrite past the
call, so on the fast path it copies the bytes you answered (`ownRawRewrite`):
building the rewrite in a scratch buffer you reuse on the next call is safe.
Every other slice of the answer is the engine's from your return, as a
forked plugin's decoded answer is, and you never write into `route.Raw()`.
<!-- source: pkg/plugin/rpc/types.go -- FilterUpdateOutput, FilterSet, FilterSetAction -->
<!-- source: pkg/plugin/rpc/filter_slot.go -- FilterSlotHandler, ownRawRewrite -->

`OnFilter` is the only registration of a filter. An evaluator that reads the
route as one `*sdk.FilterUpdateInput` (the text form in its `Update`
field, for example) calls `route.Input()`, which encodes a fast-path route's
fields for that call. It is still an evaluator under the contract above:
its calls run concurrently, so state it keeps is guarded or snapshotted, and
it does not block.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnFilter -->
<!-- source: pkg/plugin/rpc/filter_slot.go -- Input -->
<!-- source: pkg/plugin/sdk/filter_conformance.go -- CheckFilterConformance -->

A filter can also place itself. `Registration.FilterInsertions` lists the
peers' filter lists your filter joins without the operator naming it: each
`rpc.FilterInsertion` gives the reference, `import` or `export`, a priority,
and the config path under `bgp peer` whose presence (or one of whose values)
means a peer needs it. Ze inserts it ahead of the operator's members on
exactly those peers, group inheritance included. The path must be a node the
daemon compiled. Your plugin's own YANG is not compiled yet
(`schema.md`, "What works today"), so a third-party plugin conditions on a
path one of ze's modules defines; a plugin compiled into ze conditions only on
config its own module adds. In place of the path, an insertion can carry a
`session` condition: session facts of the peer that must all hold, such as
`ibgp` `false` and `rs-client` `false` for every bilateral eBGP session; a
dynamic peer is judged when its OPEN arrives. Declare the insertions in the `Registration` you
pass to `p.Run`: that Stage 1 declaration is all ze knows of a third-party
plugin, and ze places the members before any peer starts. Name your filter by
the process name ze runs you under (`<name>:<filter>`, the name your
`plugin external <name>` entry gives), because that is the reference you
serve. An insertion marked `required` carries a `reason`: the operator cannot
name, move or deactivate that member, and a config that does is refused at
verify with your reason, naming the peer, the chain and the reference (this is
how `bgp-role` keeps the RFC 9234 Section 5 procedures out of operator reach).
The fields and refusals are in
`docs/architecture/api/process-protocol.md`, "Filter Insertion", and
`internal/test/plugins/fakefilter` (`FilterInsertions`) is a worked example:
run under another name, it inserts `<name>:judge` where a peer sets
`fakefilter { insert-copy <name>; }`, and `<name>:session-judge`, which reads
the `peer-name` session fact, on the session condition `peer-name`
`<name>-peer`.
<!-- source: pkg/plugin/rpc/types.go -- FilterInsertion, FilterInsertionSchemaPath -->
<!-- source: internal/component/bgp/config/filter_insertion.go -- requiredInsertionRefusal -->
<!-- source: internal/test/plugins/fakefilter/fakefilter.go -- FilterInsertions, sessionInsertion -->
<!-- source: internal/component/config/schema_origin.go -- CheckFilterInsertionPaths -->

### OnBye

Called when the engine sends `bye`: when it removes the plugin while the
daemon keeps running (reason `removed`), and when it stops a subsystem process.
A daemon stop sends no `bye`. It closes the connection, `Run` returns nil, and
cleanup the plugin owes on daemon stop runs after `Run` returns
(`docs/architecture/api/process-protocol.md`, "Shutdown"). On the pipe transport,
the SDK responds OK before invoking this callback. On the bridge transport,
the callback runs and then the result is sent back through the channel.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnBye -->

```go
p.OnBye(func(reason string) error {
    // Withdraw what this plugin installed. An error keeps the plugin
    // running, and the engine keeps its dependencies for a retry.
    return cleanUp(reason)
})
```

### OnStarted

Called after the 5-stage startup completes but before the event loop begins.
This is the safe place to make engine calls (e.g., `SubscribeEvents`).
Do NOT make engine calls inside `OnShareRegistry` or `OnConfigure` -- those
run while the engine is waiting for the response, causing a deadlock.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnStarted -->

```go
p.OnStarted(func(ctx context.Context) error {
    // Safe to call engine methods here
    return p.SubscribeEvents(ctx, []string{"update", "state"}, []string{"*"}, "json")
})
```

### NLRI Handlers

Plugins that handle address families register encode/decode handlers.

The decode handler takes a third argument, `addPath`. It states whether each
NLRI in `hex` carries a 4-octet Path Identifier ahead of it. RFC 7911 Section 3
adds that field once the two speakers negotiate ADD-PATH for the family, and the
octets do not announce it. A handler that ignores the flag reads the Path
Identifier as the first bytes of the NLRI.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnEncodeNLRI, OnDecodeNLRI, OnDecodeCapability, DecodeNLRIHandler -->

```go
// Encode NLRI from text arguments to hex
p.OnEncodeNLRI(func(family string, args []string) (string, error) {
    // args: component keywords (e.g., ["destination-ipv4", "10.0.0.0/24"])
    hex, err := encodeFlowSpec(family, args)
    return hex, err
})

// Decode NLRI from hex to data structure (SDK marshals once)
p.OnDecodeNLRI(func(family string, hex string, addPath bool) (any, error) {
    result, err := decodeFlowSpec(family, hex, addPath)
    return result, err
})

// Decode capability from hex to data structure (SDK marshals once)
p.OnDecodeCapability(func(code uint8, hex string) (any, error) {
    if code == 73 { // FQDN
        return decodeFQDN(hex)
    }
    return nil, fmt.Errorf("unknown capability code: %d", code)
})
```

## Pre-Run Configuration

These methods configure the plugin before calling `Run()`:

### SetStartupSubscriptions

Sets event subscriptions included in the Stage 5 "ready" RPC, registered
atomically before `SignalAPIReady`. This avoids the race between the engine
sending routes and the plugin subscribing.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- SetStartupSubscriptions -->

```go
p.SetStartupSubscriptions(
    []string{"update", "state"},    // event types
    []string{"*"},                   // peer filter
    "json",                          // format
)
```

### SetEncoding

Sets the event encoding preference (`"json"` or `"text"`). Text encoding uses
space-delimited output parseable by `strings.Fields` instead of nested JSON.
Must be called after `SetStartupSubscriptions` and before `Run()`.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- SetEncoding -->

```go
p.SetEncoding("text")
```

### DeclareCapabilitiesFrom

Registers the extractor that computes the plugin's capability declaration from
the config sections it receives. It is the only way a plugin declares
capabilities, and it must be called before `Run()`. A plugin whose set does not
come from config registers an extractor that ignores the sections.
<!-- source: pkg/plugin/sdk/capability_declaration.go -- DeclareCapabilitiesFrom, composeCapabilityVerify, CandidateCapabilities -->

`Run` calls the extractor on the Stage 2 configuration and sends its
capabilities at Stage 3. A runtime `configure` redelivery reaches
`OnConfigure` and never the extractor.

At config-verify the plugin's own `OnConfigVerify` handler answers first. When
it accepts, the SDK runs the extractor on the candidate sections and the answer
carries the whole candidate declaration (`capabilities` in the config-verify
answer). The engine installs it when the reload applies, so a peer the reload
adds sends its OPEN with its capabilities, and a running session whose
capability changed restarts when its peer advertised that code. A reload that
does not commit puts the previous declaration back. `sdk.CandidateCapabilities`
returns the declaration the answer carries, for a plugin's own tests.

```go
p.DeclareCapabilitiesFrom(func(sections []sdk.ConfigSection) (sdk.CapabilityDeclaration, error) {
    return sdk.CapabilityDeclaration{
        Capabilities: []sdk.CapabilityDecl{
            {Code: 64, Encoding: "hex", Payload: "0078", Peers: []string{"peer1"}},
        },
    }, nil
})
```

A plugin that reads the `bgp` root wraps its per-section extraction with
`configjson.BGPCapabilityExtractor`, which runs it on each `bgp` section and
refuses a section whose data is not JSON.

## Engine Methods (Plugin to Engine)

These methods are available at runtime (after startup) to call the engine:
<!-- source: pkg/plugin/sdk/sdk_engine.go -- all methods -->

### UpdateRoute

Injects a route update to matching peers.

```go
peers, routes, err := p.UpdateRoute(ctx, "192.168.1.1", "update text nhop set 10.0.0.1 nlri add 10.0.1.0/24")
```

### DispatchCommand

Dispatches a command through the engine's command dispatcher (inter-plugin communication).

```go
status, data, err := p.DispatchCommand(ctx, "rib adjacent inbound show")
```

A command that completed and stated a warning answers status `warning`. The
warning text, one line per entry, comes with `DispatchCommandResult`:

```go
result, err := p.DispatchCommandResult(ctx, "show config dump")
for _, line := range result.Warnings {
    slog.Warn("dispatched command warned", "line", line)
}
```

Every plugin receives those lines, whatever protocol version it declared: the
engine writes them in the `wrn` terminator to each plugin
(`docs/architecture/api/ipc_protocol.md`, "Answer Protocol").
<!-- source: pkg/plugin/sdk/sdk_engine.go -- DispatchCommandResult, DispatchResult -->
<!-- source: internal/component/plugin/dispatch.go -- WriteAnswer, answerWarnings -->

### EmitEvent

Pushes an event into the engine's delivery pipeline for plugin-to-plugin communication.

```go
delivered, err := p.EmitEvent(ctx, "bgp", "rpki", "received", "192.168.1.1", eventJSON)
```

### SubscribeEvents / UnsubscribeEvents

Subscribe or unsubscribe from event delivery.

```go
err := p.SubscribeEvents(ctx, []string{"update"}, []string{"*"}, "json")
err = p.UnsubscribeEvents(ctx)
```

### DecodeNLRI / EncodeNLRI

Request NLRI encoding/decoding from the engine via the plugin registry.

```go
hex, err := p.EncodeNLRI(ctx, "ipv4/flow", []string{"destination-ipv4", "10.0.0.0/24"})
json, err := p.DecodeNLRI(ctx, "ipv4/flow", "0701180A0000")
```

### DecodeMPReach / DecodeMPUnreach

Request MP_REACH_NLRI or MP_UNREACH_NLRI decoding from the engine.

```go
out, err := p.DecodeMPReach(ctx, hexAttrValue, false)
// out.Family, out.NextHop, out.NLRI

out, err := p.DecodeMPUnreach(ctx, hexAttrValue, false)
// out.Family, out.NLRI
```

### DecodeUpdate

Request full UPDATE message decoding from the engine.

```go
json, err := p.DecodeUpdate(ctx, hexUpdateBody, false)
```

## Error Handling

Return clear, actionable errors:

```go
// Good: tells user what's wrong and how to fix
return fmt.Errorf("hold-time must be at least %d seconds, got %d",
    minHoldTime, cfg.HoldTime)

// Bad: cryptic
return fmt.Errorf("validation failed")
```

**Config verify/apply responses** use structured status:
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigVerify, OnConfigApply, marshalStatusOK, marshalStatusError -->

| Situation | Response |
|-----------|----------|
| Handler returns nil | `{"status":"ok"}` |
| Handler returns error | `{"status":"error","error":"reason"}` |
| Neither `OnConfigApply` nor `DeclareAppliesNothing` registered | `{"status":"error","error":"plugin <name> cannot apply a change to <roots> while ze runs: ..."}` at config-verify and at config-apply |

**Unknown methods** are rejected with an error response:
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- eventLoop, bridgeEventLoop -->

```
#5 error {"code":"error","message":"unknown method: ze-plugin-callback:foo"}
```
