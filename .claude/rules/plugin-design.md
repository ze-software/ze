# Plugin Design Patterns

**BLOCKING:** All plugins MUST follow these patterns for consistency.

## Plugin Architecture

Ze uses a two-tier plugin system:

| Layer | Location | Purpose |
|-------|----------|---------|
| Public SDK | `pkg/plugin/sdk/` | High-level callback abstraction for external plugins |
| RPC Types | `pkg/plugin/rpc/` | Shared YANG RPC type definitions |
| Internal | `internal/plugin/<name>/` | Low-level protocol implementations |
| CLI Wrapper | `cmd/ze/bgp/plugin_<name>.go` | Standard CLI interface |

## 5-Stage Protocol (MANDATORY)

All plugins follow this startup sequence over **YANG RPC** (NUL-framed JSON over dual Unix socket pairs):

```
Stage 1: Declaration    → Plugin sends: ze-plugin-engine:declare-registration (Socket A)
Stage 2: Config         → Engine sends: ze-plugin-callback:configure (Socket B)
Stage 3: Capability     → Plugin sends: ze-plugin-engine:declare-capabilities (Socket A)
Stage 4: Registry       → Engine sends: ze-plugin-callback:share-registry (Socket B)
Stage 5: Ready          → Plugin sends: ze-plugin-engine:ready (Socket A), enter event loop
```

**Socket A** = plugin → engine RPCs (registration, routes, subscribe)
**Socket B** = engine → plugin callbacks (config, events, encode/decode, bye)

## Plugin Registration Pattern

**File:** `cmd/ze/bgp/plugin.go`

Plugins are registered in the dispatch switch:

```go
switch args[0] {
case "gr":       return cmdPluginGR(args[1:])
case "hostname": return cmdPluginHostname(args[1:])
case "flowspec": return cmdPluginFlowSpec(args[1:])
// Add new plugins here
}
```

## CLI Wrapper Pattern (MANDATORY)

**File:** `cmd/ze/bgp/plugin_<name>.go`

Every plugin wrapper MUST use `PluginConfig` and `RunPlugin()`:

```go
func cmdPluginXxx(args []string) int {
    cfg := PluginConfig{
        Name:         "xxx",                    // Plugin name
        Features:     "nlri yang",              // Space-separated: nlri, capa, yang
        SupportsNLRI: true,                     // Enable --nlri flag
        SupportsCapa: false,                    // Enable --capa flag
        GetYANG:      xxx.GetYANG,              // Returns YANG schema
        ConfigLogger: xxx.SetLogger,            // Configures logger
        RunCLIDecode: xxx.RunCLIDecode,         // CLI decode handler
        RunDecode:    xxx.RunDecode,            // Engine decode handler (optional)
        RunEngine:    xxx.RunXxxPlugin,         // Engine mode handler (net.Conn, net.Conn) int
    }
    return RunPlugin(cfg, args)
}
```

**RunEngine signature:** `func(engineConn, callbackConn net.Conn) int`

In engine mode, `RunPlugin()` reads `ZE_ENGINE_FD` and `ZE_CALLBACK_FD` environment variables to obtain the socket pair connections, then calls `RunEngine`.

## Standard Plugin Flags

`RunPlugin()` automatically provides these flags:

| Flag | Type | Description |
|------|------|-------------|
| `--log-level` | string | `disabled`, `debug`, `info`, `warn`, `err` |
| `--yang` | bool | Output YANG schema and exit |
| `--features` | bool | List supported decode features |
| `--decode` | bool | Engine decode protocol mode (if `RunDecode` provided) |
| `--nlri` | string | Decode NLRI hex (if `SupportsNLRI`) |
| `--capa` | string | Decode capability hex (if `SupportsCapa`) |
| `--text` | bool | Output human-readable instead of JSON |

## Internal Plugin Pattern

**File:** `internal/plugin/<name>/<name>.go`

### Logger Setup (MANDATORY)

```go
var logger = slogutil.DiscardLogger()

func SetLogger(l *slog.Logger) {
    if l != nil {
        logger = l
    }
}
```

### SDK Entry Point Pattern

All internal plugins use the SDK callback pattern:

```go
func RunXxxPlugin(engineConn, callbackConn net.Conn) int {
    p := sdk.NewWithConn("xxx", engineConn, callbackConn)
    defer func() { _ = p.Close() }()

    // Register callbacks for engine-initiated RPCs
    p.OnEvent(func(jsonStr string) error { ... })
    p.OnDecodeNLRI(func(family, hex string) (string, error) { ... })
    p.OnExecuteCommand(func(serial, cmd string, args []string, peer string) (status, data string, err error) { ... })

    // Optional: register startup subscriptions (atomically with ready)
    p.SetStartupSubscriptions([]string{"update", "state"}, nil, "")

    // Run the 5-stage protocol + event loop
    ctx := context.Background()
    err := p.Run(ctx, sdk.Registration{
        Families: []sdk.FamilyDecl{{Name: "l2vpn/evpn", Mode: "decode"}},
        Commands: []sdk.CommandDecl{{Name: "xxx status", Description: "..."}},
    })
    if err != nil {
        logger.Error("plugin failed", "error", err)
        return 1
    }
    return 0
}
```

### Available SDK Callbacks

| Callback | Purpose | Used by |
|----------|---------|---------|
| `OnEvent` | Receive BGP events (UPDATE, OPEN, state changes) | RIB, RR, GR, Hostname |
| `OnConfigure` | Receive config sections during Stage 2 | GR, Hostname |
| `OnDecodeNLRI` | Decode NLRI hex for a family | EVPN, VPN, BGP-LS, FlowSpec |
| `OnEncodeNLRI` | Encode NLRI from args for a family | FlowSpec |
| `OnDecodeCapability` | Decode capability hex by code | Hostname |
| `OnExecuteCommand` | Handle API commands routed to this plugin | RIB, RR |
| `OnShareRegistry` | Receive registry sharing info | (general) |
| `OnBye` | Notification of shutdown | (general) |
| `OnStarted` | Post-startup hook (safe to call engine) | (general) |

### SDK Engine Calls (plugin → engine via Socket A)

| Method | Purpose |
|--------|---------|
| `p.UpdateRoute(ctx, peer, command)` | Send route update/forward/withdraw |
| `p.SubscribeEvents(ctx, events, families, peer)` | Subscribe to event types |
| `p.UnsubscribeEvents(ctx)` | Clear all subscriptions |

## Plugin Invocation Modes

| Mode | Syntax | Implementation |
|------|--------|----------------|
| Fork (default) | `pluginname` | Subprocess via exec, sockets via FD inheritance |
| Internal | `ze.pluginname` | Goroutine + Unix socket pair |
| Direct | `ze-pluginname` | Sync in-process call |
| Path | `/path/to/binary` | Execute external binary, sockets via FD inheritance |

## Decode Protocol

Plugins supporting `--decode` mode respond to stdin commands:

```
decode capability <code> <hex>  →  decoded json {...}
decode nlri <family> <hex>      →  decoded json [...]
```

Response format:
- `decoded json <json>` - Success with JSON payload
- `decoded unknown` - Cannot decode

Note: This text-based decode protocol is separate from the YANG RPC protocol.
It is used only for the `--decode` CLI flag, not for engine-mode operation.

## In-Process Decoder Registration

**File:** `cmd/ze/bgp/decode.go`

Register decoders for fallback (tests, direct mode):

```go
var inProcessDecoders = map[string]func(input, output *bytes.Buffer) int{
    "flowspec": func(in, out *bytes.Buffer) int { return flowspec.RunFlowSpecDecode(in, out) },
    "evpn":     func(in, out *bytes.Buffer) int { return evpn.RunEVPNDecode(in, out) },
    // Add new decoders here
}
```

## Internal Plugin Runner Registration

**File:** `internal/plugin/inprocess.go`

Register internal plugin runners (engine-mode entry points):

```go
var internalPluginRunners = map[string]InternalPluginRunner{
    "rib":      rib.RunRIBPlugin,
    "gr":       gr.RunGRPlugin,
    "rr":       rr.RunRouteServer,
    "hostname": func(e, c net.Conn) int { ... },
    "flowspec": func(e, c net.Conn) int { ... },
    // Add new internal plugins here
}
```

Type: `InternalPluginRunner = func(engineConn, callbackConn net.Conn) int`

## Family-to-Plugin Mapping

**File:** `cmd/ze/bgp/decode.go`

Register which plugin handles which address families:

```go
var pluginFamilyMap = map[string]string{
    "ipv4/flow":     "flowspec",
    "l2vpn/evpn":    "evpn",
    "ipv4/vpn":      "vpn",
    "bgp-ls/bgp-ls": "bgpls",
    // Add new family mappings here
}
```

## Capability-to-Plugin Mapping

```go
var pluginCapabilityMap = map[uint8]string{
    73: "hostname",  // FQDN capability
    // Add new capability mappings here
}
```

## New Plugin Checklist

When adding a new plugin:

```
[ ] Create internal implementation: internal/plugin/<name>/<name>.go
[ ] Add package-level logger with SetLogger()
[ ] Implement SDK callback pattern (NewWithConn + callbacks + Run)
[ ] Create CLI wrapper: cmd/ze/bgp/plugin_<name>.go
[ ] Use PluginConfig + RunPlugin() pattern (RunEngine takes net.Conn pair)
[ ] Register in cmd/ze/bgp/plugin.go switch
[ ] Register in internalPluginRunners (inprocess.go) for internal plugins
[ ] Register in inProcessDecoders if decode support
[ ] Register in pluginFamilyMap if NLRI decode support
[ ] Register in pluginCapabilityMap if capability decode support
[ ] Add YANG schema if configuration support
[ ] Add functional tests in test/plugin/
```
