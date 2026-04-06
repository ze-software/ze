# Plugin Handlers

Handlers process engine-initiated callbacks during startup and at runtime.
They are registered on the `sdk.Plugin` before calling `Run()`.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- all On* methods -->

## Registration

Register handlers before calling `Run()`:

```go
p := sdk.NewWithConn("my-plugin", conn)

// Startup handlers
p.OnConfigure(func(sections []sdk.ConfigSection) error { ... })
p.OnShareRegistry(func(commands []sdk.RegistryCommand) { ... })

// Runtime handlers
p.OnEvent(func(event string) error { ... })
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) { ... })
p.OnConfigVerify(func(sections []sdk.ConfigSection) error { ... })
p.OnConfigApply(func(diffs []sdk.ConfigDiffSection) error { ... })
p.OnValidateOpen(func(input *sdk.ValidateOpenInput) *sdk.ValidateOpenOutput { ... })
p.OnBye(func(reason string) { ... })

// Post-startup callback (safe to make engine calls here)
p.OnStarted(func(ctx context.Context) error { ... })

// NLRI handlers
p.OnEncodeNLRI(func(family string, args []string) (string, error) { ... })
p.OnDecodeNLRI(func(family string, hex string) (string, error) { ... })
p.OnDecodeCapability(func(code uint8, hex string) (string, error) { ... })

p.Run(ctx, sdk.Registration{...})
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
p.OnExecuteCommand(func(serial, command string, args []string, peer string) (status, data string, err error) {
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
| `data` | `string` | JSON-encoded response data |
| `err` | `error` | Non-nil triggers error response |

### OnConfigVerify

Called during config reload to validate a candidate config. The plugin receives
the full candidate config sections and returns nil to accept or an error to reject.
If no handler is registered, config-verify returns OK (no-op).
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnConfigVerify -->
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
accept or an error to reject. If no handler is registered, config-apply returns
OK (no-op).
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

Plugins that handle address families register encode/decode handlers:
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnEncodeNLRI, OnDecodeNLRI, OnDecodeCapability -->

```go
// Encode NLRI from text arguments to hex
p.OnEncodeNLRI(func(family string, args []string) (string, error) {
    // args: component keywords (e.g., ["destination", "10.0.0.0/24"])
    hex, err := encodeFlowSpec(family, args)
    return hex, err
})

// Decode NLRI from hex to JSON
p.OnDecodeNLRI(func(family string, hex string) (string, error) {
    json, err := decodeFlowSpec(family, hex)
    return json, err
})

// Decode capability from hex to JSON
p.OnDecodeCapability(func(code uint8, hex string) (string, error) {
    if code == 73 { // FQDN
        return decodeFQDN(hex)
    }
    return "", fmt.Errorf("unknown capability code: %d", code)
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

### SetCapabilities

Sets the capabilities to declare during Stage 3. Must be called before `Run()`.
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- SetCapabilities -->

```go
p.SetCapabilities([]sdk.CapabilityDecl{
    {Code: 64, Encoding: "hex", Payload: "0078", Peers: []string{"192.168.1.1"}},
})
```

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
hex, err := p.EncodeNLRI(ctx, "ipv4/flow", []string{"destination", "10.0.0.0/24"})
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
| No handler registered | `{"status":"ok"}` (graceful no-op) |

**Unknown methods** are rejected with an error response:
<!-- source: pkg/plugin/sdk/sdk_dispatch.go -- eventLoop, bridgeEventLoop -->

```
#5 error {"code":"error","message":"unknown method: ze-plugin-callback:foo"}
```
