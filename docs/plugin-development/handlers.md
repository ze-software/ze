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
p.OnBye(func(reason string) { ... })

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

### OnBye

Called when the engine sends a shutdown notification. On the pipe transport,
the SDK responds OK before invoking this callback. On the bridge transport,
the callback runs and then the result is sent back through the channel.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnBye -->

```go
p.OnBye(func(reason string) {
    log.Printf("shutting down: %s", reason)
    // Clean up resources
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
the config sections it receives, and the config leaf that sets each code. It is
the only way a plugin declares capabilities, and it must be called before
`Run()`. A plugin whose set does not come from config registers an extractor
that ignores the sections.
<!-- source: pkg/plugin/sdk/capability_declaration.go -- DeclareCapabilitiesFrom, capabilityChanges -->

`Run` calls the extractor once, on the Stage 2 configuration, and sends its
capabilities at Stage 3. The engine puts that declaration in every OPEN until
the plugin process ends, so it is never rewritten: a runtime `configure`
redelivery reaches `OnConfigure` and never the extractor.

At config-verify the plugin's own `OnConfigVerify` handler answers first. When
it accepts, the SDK runs the extractor on the candidate sections and compares
the two declarations per peer selector (`Peers`) and code. A changed payload, a
code removed from a peer that stays configured, and a code added (a peer added
with a capability included) are refused, naming the peer, the leaf and the
plugin, followed by `sdk.RestartSteps()`. A peer the candidate no longer names
is accepted: it sends no OPEN again. A declaration with no `Peers` applies to
every peer and is compared as one selector, named `every peer`.

```go
p.DeclareCapabilitiesFrom(func(sections []sdk.ConfigSection) (sdk.CapabilityDeclaration, error) {
    return sdk.CapabilityDeclaration{
        Capabilities: []sdk.CapabilityDecl{
            {Code: 64, Encoding: "hex", Payload: "0078", Peers: []string{"peer1"}},
        },
        Peers: []string{"peer1"}, // every peer selector the config names
    }, nil
}, map[uint8]string{64: "graceful-restart"})
```

A plugin that reads the `bgp` root wraps its per-section extraction with
`configjson.BGPCapabilityExtractor`, which reports every peer selector of the
tree (`configjson.CapabilitySelectors`).

## Engine Methods (Plugin to Engine)

These methods are available at runtime (after startup) to call the engine:
<!-- source: pkg/plugin/sdk/sdk_engine.go -- all methods -->

### UpdateRoute

Injects a route update to matching peers.

```go
peers, routes, err := p.UpdateRoute(ctx, "192.168.1.1", "update text nhop set 10.0.0.1 nlri ipv4/unicast add 10.0.1.0/24")
```

### DispatchCommand

Dispatches a command through the engine's command dispatcher (inter-plugin communication).

```go
status, data, err := p.DispatchCommand(ctx, "rib adjacent inbound show")
```

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
