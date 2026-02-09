# Plugin Design Patterns

**BLOCKING:** All plugins MUST follow these patterns for consistency.

## Plugin Architecture

Ze uses a two-tier plugin system with compile-time registration:

| Layer | Location | Purpose |
|-------|----------|---------|
| Registry | `internal/plugin/registry/` | Central plugin registry (leaf package, no plugin deps) |
| Public SDK | `pkg/plugin/sdk/` | High-level callback abstraction for external plugins |
| RPC Types | `pkg/plugin/rpc/` | Shared YANG RPC type definitions |
| Internal | `internal/plugin/<name>/` | Low-level protocol implementations + `register.go` |
| All imports | `internal/plugin/all/` | Blank imports triggering all plugin `init()` |
| CLI shared | `internal/plugin/cli/` | `PluginConfig` + `RunPlugin()` shared framework |

## Registration Architecture

Plugins self-register at compile time via Go's `init()` mechanism:

```
internal/plugin/registry/       ← Leaf package (no plugin imports)
    registry.go                 ← Register(), Lookup(), Names(), FamilyMap(), etc.

internal/plugin/<name>/         ← Each plugin package
    <name>.go                   ← Plugin logic
    register.go                 ← init() { registry.Register(...) }

internal/plugin/all/            ← Auto-generated aggregation file
    all.go                      ← Generated blank imports (make generate)
    gen.go                      ← go:generate directive

scripts/gen-plugin-imports.go   ← Generator: discovers register.go, writes all.go

cmd/ze/bgp/plugin.go           ← Imports all, dispatches via registry.Lookup()
internal/plugin/inprocess.go   ← Delegates to registry for runners/YANG/config
cmd/ze/bgp/decode.go           ← Maps populated from registry at init time
```

**Key design:** The registry package is a leaf with zero plugin dependencies. Plugin packages import the registry (not vice versa). The `all` package uses blank imports to trigger `init()` registration.

## 5-Stage Protocol (MANDATORY)

All plugins follow this startup sequence over **YANG RPC** (NUL-framed JSON over dual Unix socket pairs):

```
Stage 1: Declaration    -> Plugin sends: ze-plugin-engine:declare-registration (Socket A)
Stage 2: Config         -> Engine sends: ze-plugin-callback:configure (Socket B)
Stage 3: Capability     -> Plugin sends: ze-plugin-engine:declare-capabilities (Socket A)
Stage 4: Registry       -> Engine sends: ze-plugin-callback:share-registry (Socket B)
Stage 5: Ready          -> Plugin sends: ze-plugin-engine:ready (Socket A), enter event loop
```

**Socket A** = plugin -> engine RPCs (registration, routes, subscribe)
**Socket B** = engine -> plugin callbacks (config, events, encode/decode, bye)

## Plugin Registration via init() (MANDATORY)

**File:** `internal/plugin/<name>/register.go`

Every internal plugin MUST have a `register.go` file that calls `registry.Register()` in `init()`. The Registration struct provides ALL metadata needed for the plugin to be integrated throughout the system.

### Registration Fields

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `Name` | string | Yes | Plugin name (e.g., "rib", "gr", "flowspec") |
| `Description` | string | Yes | Human-readable description for help text |
| `RunEngine` | func(net.Conn, net.Conn) int | Yes | Engine-mode entry point (Socket A, Socket B) |
| `CLIHandler` | func([]string) int | Yes | CLI dispatch handler (calls RunPlugin) |
| `RFCs` | []string | No | Related RFCs (e.g., ["RFC 4724"]) |
| `Families` | []string | No | Address families handled (e.g., ["ipv4/flow", "ipv6/flow"]) |
| `CapabilityCodes` | []uint8 | No | Capability codes decoded by this plugin |
| `ConfigRoots` | []string | No | Config roots this plugin wants |
| `YANG` | string | No | YANG schema content |
| `ConfigureEngineLogger` | func(string) | No | Logger setup for engine mode |
| `InProcessDecoder` | func(*bytes.Buffer, *bytes.Buffer) int | No | In-process decode function for CLI |
| `Features` | string | No | Space-separated feature flags ("nlri yang capa") |
| `SupportsNLRI` | bool | No | Enable --nlri CLI flag |
| `SupportsCapa` | bool | No | Enable --capa CLI flag |

### Registration Validation

`Register()` validates:
- Name must be non-empty
- RunEngine must be non-nil
- CLIHandler must be non-nil
- Family strings must contain "/" (format "afi/safi")
- Duplicate names are rejected

Registration errors cause `os.Exit(1)` in `init()` - a broken plugin registration is a fatal startup error.

## CLI Shared Framework

**File:** `internal/plugin/cli/cli.go`

The `PluginConfig` struct and `RunPlugin()` function live in a shared package importable by plugin `register.go` files. Each plugin's `CLIHandler` creates a `PluginConfig` and calls `RunPlugin()`.

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

### SDK Engine Calls (plugin -> engine via Socket A)

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
decode capability <code> <hex>  ->  decoded json {...}
decode nlri <family> <hex>      ->  decoded json [...]
```

Response format:
- `decoded json <json>` - Success with JSON payload
- `decoded unknown` - Cannot decode

Note: This text-based decode protocol is separate from the YANG RPC protocol.
It is used only for the `--decode` CLI flag, not for engine-mode operation.

## Auto-Populated Maps

These maps are populated automatically from the registry at package init time.
No manual registration needed - plugins declare their capabilities in `register.go`.

| Map | Location | Source |
|-----|----------|--------|
| CLI dispatch | `cmd/ze/bgp/plugin.go` | `registry.Lookup()` |
| Plugin runners | `internal/plugin/inprocess.go` | `registry.Lookup().RunEngine` |
| YANG schemas | `internal/plugin/inprocess.go` | `registry.YANGSchemas()` |
| Config roots | `internal/plugin/inprocess.go` | `registry.ConfigRootsMap()` |
| Family->plugin | `cmd/ze/bgp/decode.go` | `registry.FamilyMap()` |
| Capability->plugin | `cmd/ze/bgp/decode.go` | `registry.CapabilityMap()` |
| In-process decoders | `cmd/ze/bgp/decode.go` | `registry.InProcessDecoders()` |

## New Plugin Checklist

When adding a new plugin, you need exactly TWO files:

```
[ ] Create internal implementation: internal/plugin/<name>/<name>.go
[ ] Add package-level logger with SetLogger()
[ ] Implement SDK callback pattern (NewWithConn + callbacks + Run)
[ ] Create register.go: internal/plugin/<name>/register.go
    - init() calls registry.Register() with full Registration struct
    - CLIHandler creates PluginConfig and calls cli.RunPlugin()
    - Set Families, CapabilityCodes, InProcessDecoder as applicable
[ ] Run make generate (auto-discovers register.go, regenerates all.go)
[ ] Update TestAllPluginsRegistered expected count in all_test.go
[ ] Add YANG schema if configuration support (internal/plugin/<name>/schema/)
[ ] Add functional tests in test/plugin/
```

**What you do NOT need to do (handled automatically):**
- No changes to `cmd/ze/bgp/plugin.go` (dispatch)
- No changes to `cmd/ze/bgp/decode.go` (family/capability/decoder maps)
- No changes to `internal/plugin/inprocess.go` (runners/YANG/config)
- No individual `cmd/ze/bgp/plugin_<name>.go` wrapper files
- No manual editing of `internal/plugin/all/all.go` (generated by `make generate`)

## Integration Tests

**File:** `internal/plugin/all/all_test.go`

Verifies that all plugins register correctly:

| Test | Validates |
|------|-----------|
| `TestAllPluginsRegistered` | All expected plugins are registered |
| `TestAllPluginsHaveRunEngine` | No nil RunEngine handlers |
| `TestAllPluginsHaveCLIHandler` | No nil CLIHandler handlers |
| `TestAllPluginsHaveDescription` | Help text has descriptions for all plugins |
| `TestFamilyMappings` | Expected families map to correct plugins |
| `TestCapabilityMappings` | Expected capabilities map to correct plugins |
