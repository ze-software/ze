# Plugin Design Patterns

**BLOCKING:** All plugins MUST follow these patterns for consistency.

## Plugin Architecture

Ze uses a two-tier plugin system:

| Layer | Location | Purpose |
|-------|----------|---------|
| Public SDK | `pkg/plugin/` | High-level handler abstraction for external plugins |
| Internal | `internal/plugin/<name>/` | Low-level protocol implementations |
| CLI Wrapper | `cmd/ze/bgp/plugin_<name>.go` | Standard CLI interface |

## 5-Stage Protocol (MANDATORY)

All plugins follow this startup sequence:

```
Stage 1: Declaration    → Send: declare encoding, schema, handlers, commands
Stage 2: Config         → Receive config lines until "config done"
Stage 3: Capability     → Send: "capability done"
Stage 4: Registry       → Wait for "registry done"
Stage 5: Ready          → Send: "ready", enter command loop
```

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
        RunEngine:    xxx.RunEngine,            // Engine mode handler
    }
    return RunPlugin(cfg, args)
}
```

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

### Main Struct Pattern

```go
type XxxPlugin struct {
    in      io.Reader
    out     io.Writer
    mu      sync.Mutex
    config  map[string]*PeerConfig  // Per-peer configuration
}
```

### Run() Method Pattern

```go
func (p *XxxPlugin) Run() int {
    if err := p.doStartupProtocol(); err != nil {
        return 1
    }
    return p.eventLoop()
}
```

### Thread-Safe Output

```go
func (p *XxxPlugin) send(format string, args ...any) {
    p.mu.Lock()
    defer p.mu.Unlock()
    fmt.Fprintf(p.out, format+"\n", args...)
}
```

## Plugin Invocation Modes

| Mode | Syntax | Implementation |
|------|--------|----------------|
| Fork (default) | `pluginname` | Subprocess via exec |
| Internal | `ze.pluginname` | Goroutine + pipe |
| Direct | `ze-pluginname` | Sync in-process call |
| Path | `/path/to/binary` | Execute external binary |

## Decode Protocol

Plugins supporting `--decode` mode respond to stdin commands:

```
decode capability <code> <hex>  →  decoded json {...}
decode nlri <family> <hex>      →  decoded json [...]
```

Response format:
- `decoded json <json>` - Success with JSON payload
- `decoded unknown` - Cannot decode

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
[ ] Implement 5-stage startup protocol
[ ] Create CLI wrapper: cmd/ze/bgp/plugin_<name>.go
[ ] Use PluginConfig + RunPlugin() pattern
[ ] Register in cmd/ze/bgp/plugin.go switch
[ ] Register in inProcessDecoders if decode support
[ ] Register in pluginFamilyMap if NLRI decode support
[ ] Register in pluginCapabilityMap if capability decode support
[ ] Add YANG schema if configuration support
[ ] Add functional tests in test/plugin/
```
