# Ze Plugin Development Guide

This guide explains how to create plugins for ze. Plugins extend ze's functionality by handling BGP events, providing commands, managing address families, and injecting custom capabilities.

## Quick Start (5 minutes)

### 1. Create a Plugin

```go
package main

import (
	"context"
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

func main() {
	// Connect to the engine via TLS using environment variables.
	// Reads ZE_PLUGIN_HUB_HOST (default 127.0.0.1), ZE_PLUGIN_HUB_PORT
	// (default 12700), and ZE_PLUGIN_HUB_TOKEN (required).
	p, err := sdk.NewFromEnv("my-plugin")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer p.Close()

	// Register event handler (receives BGP events as JSON strings).
	p.OnEvent(func(event string) error {
		fmt.Println("event:", event)
		return nil
	})

	// Register config handler (receives config sections during startup).
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			fmt.Printf("config root=%s data=%s\n", s.Root, s.Data)
		}
		return nil
	})

	// Subscribe to updates at startup (race-free -- registered atomically
	// with the "ready" signal before routes start flowing).
	p.SetStartupSubscriptions([]string{"update"}, nil, "")

	// Run the 5-stage startup protocol and enter the event loop.
	// Returns nil on clean shutdown (bye received).
	if err := p.Run(context.Background(), sdk.Registration{
		Families: []sdk.FamilyDecl{{Name: "ipv4/unicast", Mode: "both"}},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
```
<!-- source: pkg/plugin/sdk/sdk.go -- NewFromEnv, Run, Plugin -->
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnEvent, OnConfigure, SetStartupSubscriptions -->
<!-- source: pkg/plugin/sdk/sdk_types.go -- Registration, FamilyDecl, ConfigSection -->

### 2. Build and Test

```bash
go build -o my-plugin
ze plugin validate binary ./my-plugin
```

### 3. Configure Ze

```
plugin {
    hub {
        server local {
            host 127.0.0.1;
            port 12700;
            secret "shared-token";
        }
    }
}

process my-plugin {
    run "./my-plugin";
}
```

The engine sets these environment variables before launching the plugin process:

| Variable | Purpose |
|----------|---------|
| `ZE_PLUGIN_HUB_HOST` | TLS host to connect to (default 127.0.0.1) |
| `ZE_PLUGIN_HUB_PORT` | TLS port to connect to (default 12700) |
| `ZE_PLUGIN_HUB_TOKEN` | Per-plugin auth token (unique per plugin, cleared from env after read) |
| `ZE_PLUGIN_CERT_FP` | SHA-256 fingerprint of the engine's TLS certificate (for cert pinning) |
| `ZE_PLUGIN_NAME` | Plugin name as configured in ze |

Each plugin receives its own unique auth token. The token is bound to the plugin name: a plugin cannot use its token to authenticate as a different plugin. The token is automatically cleared from the OS environment after the SDK reads it, so it is not visible in `/proc/<pid>/environ`.

When `ZE_PLUGIN_CERT_FP` is set, the SDK verifies the engine's TLS certificate fingerprint during the handshake, preventing man-in-the-middle attacks.
<!-- source: pkg/plugin/sdk/sdk.go -- NewFromTLSEnv, env var registrations -->
<!-- source: internal/component/plugin/process/process.go -- startExternal env var setup -->

## What Can Plugins Do?

- **Handle BGP events** -- Receive UPDATE, OPEN, peer state changes as JSON
- **Provide commands** -- Expose custom commands via the API
- **Manage address families** -- Encode/decode NLRI for custom address families
- **Extend configuration** -- Add config sections with YANG schemas
- **Validate config changes** -- Accept or reject config during reload
- **Apply config diffs** -- React to config changes at runtime
- **Inject capabilities** -- Add BGP capabilities to OPEN messages
- **Validate OPEN messages** -- Accept or reject peer sessions
- **Emit events** -- Push events for other plugins to consume
- **Dispatch commands** -- Invoke commands on other plugins through the engine
<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- OnEvent, OnConfigure, OnExecuteCommand, OnEncodeNLRI, OnDecodeNLRI, OnConfigVerify, OnConfigApply, OnValidateOpen -->
<!-- source: pkg/plugin/sdk/sdk_engine.go -- EmitEvent, DispatchCommand -->

## Architecture

```
Ze Engine                          Plugin
    |                                |
    |--- Stage 1: declare-registration -->|  Plugin declares families, commands, schema
    |<-- ok -----------------------------|
    |                                |
    |--- Stage 2: configure ------------>|  Engine sends config sections
    |<-- ok -----------------------------|
    |                                |
    |--- Stage 3: declare-capabilities ->|  Plugin declares BGP capabilities
    |<-- ok -----------------------------|
    |                                |
    |--- Stage 4: share-registry ------->|  Engine shares command registry
    |<-- ok -----------------------------|
    |                                |
    |--- Stage 5: ready ---------------->|  Plugin enters event loop
    |<-- ok -----------------------------|
    |                                |
    |--- deliver-batch --------------->|  Runtime: BGP events
    |<-- ok ----------------------------|
    |                                |
    |--- execute-command ------------->|  Runtime: command execution
    |<-- ok {"status":"done"} ---------|
    |                                |
    |<-- update-route ------------------|  Runtime: plugin sends routes
    |--- ok {"peers-affected":2} ----->|
    |                                |
    |--- bye -------------------------->|  Shutdown
    |<-- ok ----------------------------|
```
<!-- source: pkg/plugin/sdk/sdk.go -- Run (stages 1-5) -->
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput, ConfigureInput, DeclareCapabilitiesInput, ShareRegistryInput, ReadyInput, ExecuteCommandInput, UpdateRouteInput, ByeInput -->

Plugins communicate via a single bidirectional TLS connection using the `#<id> <verb> [<json>]` wire format. MuxConn multiplexes concurrent RPCs by distinguishing responses (`ok`/`error`) from requests (method name as verb).
<!-- source: pkg/plugin/rpc/message.go -- ParseLine, FormatRequest, FormatResult, FormatError -->

For internal plugins (running as goroutines inside the engine), the connection is a `net.Pipe`, and after startup a DirectBridge bypasses the pipe for event delivery.
<!-- source: pkg/plugin/sdk/sdk.go -- NewWithConn, bridge discovery -->

## Documentation

| Guide | Description |
|-------|-------------|
| [protocol.md](protocol.md) | 5-stage protocol details |
| [schema.md](schema.md) | YANG schema authoring |
| [handlers.md](handlers.md) | Config verify/apply handlers |
| [commands.md](commands.md) | Adding commands |
| [testing.md](testing.md) | Testing your plugin |

## Example Plugins

- [Go Example](../../examples/plugin/go/) -- Complete Go plugin

## SDK Reference

The Go SDK (`pkg/plugin/sdk`) provides:
<!-- source: pkg/plugin/sdk/sdk.go -- Plugin struct -->
<!-- source: pkg/plugin/sdk/sdk_types.go -- all type aliases -->

### Core Types

| Type | Purpose |
|------|---------|
| `sdk.Plugin` | Main plugin struct, created via `NewFromEnv` or `NewWithConn` |
| `sdk.Registration` | Stage 1 declaration (= `rpc.DeclareRegistrationInput`) |
| `sdk.FamilyDecl` | Address family declaration (`Name` + `Mode`) |
| `sdk.CommandDecl` | Command declaration (`Name` + `Description` + `Args`) |
| `sdk.SchemaDecl` | YANG schema declaration (`Module` + `YANGText` + `Handlers`) |
| `sdk.ConfigSection` | Config section (`Root` + `Data`) |
| `sdk.ConfigDiffSection` | Config diff (`Root` + `Added` + `Removed` + `Changed`) |
| `sdk.CapabilityDecl` | BGP capability for OPEN injection |
| `sdk.RegistryCommand` | Command in the shared registry |
<!-- source: pkg/plugin/sdk/sdk_types.go -- Registration, FamilyDecl, CommandDecl, SchemaDecl, ConfigSection, ConfigDiffSection, CapabilityDecl, RegistryCommand -->
<!-- source: pkg/plugin/rpc/types.go -- DeclareRegistrationInput, FamilyDecl, CommandDecl, SchemaDecl, ConfigSection, CapabilityDecl, RegistryCommand -->

### Constructors

| Function | Purpose |
|----------|---------|
| `sdk.NewFromEnv(name)` | Connect via TLS using `ZE_PLUGIN_HUB_*` env vars. Returns `(*Plugin, error)` |
| `sdk.NewWithConn(name, conn)` | Create from existing `net.Conn` (internal plugins, testing) |

<!-- source: pkg/plugin/sdk/sdk.go -- NewFromEnv, NewWithConn -->

### Callback Registration (call before `Run`)

| Method | Signature | When Called |
|--------|-----------|------------|
| `OnConfigure` | `func([]ConfigSection) error` | Stage 2: engine delivers config |
| `OnShareRegistry` | `func([]RegistryCommand)` | Stage 4: engine shares command registry |
| `OnEvent` | `func(string) error` | Runtime: BGP event delivery |
| `OnExecuteCommand` | `func(serial, command string, args []string, peer string) (status, data string, err error)` | Runtime: command execution |
| `OnEncodeNLRI` | `func(family string, args []string) (string, error)` | Runtime: NLRI encoding request |
| `OnDecodeNLRI` | `func(family string, hex string) (string, error)` | Runtime: NLRI decoding request |
| `OnDecodeCapability` | `func(code uint8, hex string) (string, error)` | Runtime: capability decoding request |
| `OnConfigVerify` | `func([]ConfigSection) error` | Reload: validate candidate config |
| `OnConfigApply` | `func([]ConfigDiffSection) error` | Reload: apply config diff |
| `OnValidateOpen` | `func(*ValidateOpenInput) *ValidateOpenOutput` | Runtime: validate OPEN messages |
| `OnBye` | `func(string)` | Shutdown: reason string |
| `OnStarted` | `func(ctx context.Context) error` | Post-startup: safe to make engine calls |

<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- all On* methods -->

### Configuration Methods (call before `Run`)

| Method | Purpose |
|--------|---------|
| `SetStartupSubscriptions(events, peers []string, format string)` | Race-free event subscription (included in Stage 5 ready RPC) |
| `SetEncoding(enc string)` | Set event encoding preference (`"json"` or `"text"`) |
| `SetCapabilities(caps []CapabilityDecl)` | Set BGP capabilities to declare in Stage 3 |

<!-- source: pkg/plugin/sdk/sdk_callbacks.go -- SetStartupSubscriptions, SetEncoding, SetCapabilities -->

### Engine Calls (call after startup, typically in `OnStarted` or `OnEvent`)

| Method | Purpose |
|--------|---------|
| `UpdateRoute(ctx, peerSelector, command)` | Inject route updates to matching peers |
| `DispatchCommand(ctx, command)` | Route a command through the engine's dispatcher |
| `EmitEvent(ctx, namespace, eventType, direction, peerAddress, event)` | Push events to subscribers |
| `SubscribeEvents(ctx, events, peers, format)` | Subscribe to event delivery |
| `UnsubscribeEvents(ctx)` | Stop event delivery |
| `DecodeNLRI(ctx, family, hex)` | Decode NLRI via engine registry |
| `EncodeNLRI(ctx, family, args)` | Encode NLRI via engine registry |
| `DecodeMPReach(ctx, hex, addPath)` | Decode MP_REACH_NLRI attribute |
| `DecodeMPUnreach(ctx, hex, addPath)` | Decode MP_UNREACH_NLRI attribute |
| `DecodeUpdate(ctx, hex, addPath)` | Decode full UPDATE message body |

<!-- source: pkg/plugin/sdk/sdk_engine.go -- all exported methods -->

### Lifecycle

| Method | Purpose |
|--------|---------|
| `Run(ctx, Registration)` | Execute 5-stage startup + event loop. Returns `nil` on clean shutdown |
| `Close()` | Close connections and listeners. Safe to call multiple times |
| `Listeners()` | Access listen sockets received via fd passing during startup |

<!-- source: pkg/plugin/sdk/sdk.go -- Run, Close, Listeners -->

### Complete Example

```go
p, err := sdk.NewFromEnv("my-plugin")
if err != nil {
    log.Fatal(err)
}
defer p.Close()

// Register callbacks.
p.OnConfigure(func(sections []sdk.ConfigSection) error {
    // Process config during startup.
    return nil
})

p.OnEvent(func(event string) error {
    // Handle BGP events (UPDATE, peer state, etc.).
    return nil
})

p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, string, error) {
    // Handle command execution. Return (status, data, error).
    return "done", `{"status":"running"}`, nil
})

// Subscribe to updates at startup.
p.SetStartupSubscriptions([]string{"update"}, nil, "")

// Run the plugin.
err = p.Run(context.Background(), sdk.Registration{
    Families: []sdk.FamilyDecl{{Name: "ipv4/unicast", Mode: "both"}},
    Commands: []sdk.CommandDecl{{Name: "my-plugin status", Description: "Show status"}},
    Schema:   &sdk.SchemaDecl{Module: "my-plugin", YANGText: myYANG, Handlers: []string{"my-prefix"}},
})
```
<!-- source: pkg/plugin/sdk/sdk.go -- Run -->
<!-- source: pkg/plugin/sdk/sdk_types.go -- Registration, FamilyDecl, CommandDecl, SchemaDecl -->
