# Pattern: Plugin

Structural template for creating a Ze plugin.
Rules: `rules/plugin-design.md`. Architecture: `docs/architecture/core-design.md`.

## File Structure

```
internal/component/bgp/plugins/<name>/
  register.go         # REQUIRED: init() -> registry.Register()
  <name>.go           # REQUIRED: Package doc, logger, RunXxxPlugin()
  <name>_test.go      # Tests
  schema/             # If plugin has YANG config
    register.go       # init() -> yang.RegisterModule()
    embed.go          # //go:embed ze-<name>-conf.yang
    ze-<name>-conf.yang
```

After creating, run `make generate` to update `internal/component/plugin/all/all.go`.

## register.go Template

```go
package bgp_<name>

import (
    "fmt"
    "os"

    "codeberg.org/thomas-mangin/ze/internal/core/slogutil"
    "codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
    "codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
    reg := registry.Registration{
        Name:        "bgp-<name>",
        Description: "Human-readable description",
        RunEngine:   Run<Name>Plugin,
        // CLIHandler assigned below (closure captures reg)

        // Common optional fields:
        // RFCs:            []string{"4271"},
        // Features:        "yang",           // space-separated: "nlri", "yang", "capa"
        // ConfigRoots:     []string{"bgp"},
        // YANG:            schema.Ze<Name>YANG,
        // CapabilityCodes: []uint8{64},
        // Dependencies:    []string{"bgp-rib"},
        // Families:        []string{"ipv4/unicast"},
        // EventTypes:      []string{"update-<name>"},
    }

    reg.CLIHandler = func(args []string) int {
        cfg := cli.BaseConfig(&reg)
        cfg.ConfigLogger = func(level string) {
            SetLogger(slogutil.PluginLogger(reg.Name, level))
        }
        return cli.RunPlugin(cfg, args)
    }

    if err := registry.Register(reg); err != nil {
        fmt.Fprintf(os.Stderr, "%s: registration failed: %v\n", reg.Name, err)
        os.Exit(1)
    }
}
```

## Main Plugin File Template

```go
// Design: docs/architecture/... -- plugin topic
//
// Package bgp_<name> implements ...
package bgp_<name>

import (
    "context"
    "log/slog"
    "net"
    "sync/atomic"

    "codeberg.org/thomas-mangin/ze/internal/core/slogutil"
    sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
    d := slogutil.DiscardLogger()
    loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// SetLogger sets the package-level logger.
func SetLogger(l *slog.Logger) {
    if l != nil {
        loggerPtr.Store(l)
    }
}

// Run<Name>Plugin is the in-process entry point.
func Run<Name>Plugin(conn net.Conn) int {
    p := sdk.NewWithConn("bgp-<name>", conn)
    defer func() { _ = p.Close() }()

    p.OnConfigure(func(sections []sdk.ConfigSection) error {
        // Stage 2: receive config
        return nil
    })

    p.OnEvent(func(eventText string) error {
        // Stage 5+: receive events from engine
        return nil
    })

    ctx := context.Background()
    if err := p.Run(ctx, sdk.Registration{
        WantsConfig: []string{"bgp"},
    }); err != nil {
        logger().Error("plugin failed", "error", err)
        return 1
    }
    return 0
}
```

## Logger Pattern (MANDATORY)

All plugins use `atomic.Pointer[slog.Logger]` -- NOT a plain variable.
Tests run multiple in-process plugin instances concurrently; plain variables cause data races.

```go
var loggerPtr atomic.Pointer[slog.Logger]
func init() { loggerPtr.Store(slogutil.DiscardLogger()) }
func logger() *slog.Logger { return loggerPtr.Load() }
func SetLogger(l *slog.Logger) { if l != nil { loggerPtr.Store(l) } }
```

## Metrics Pattern (When Needed)

Same atomic pattern for metrics:

```go
var metricsPtr atomic.Pointer[<Name>Metrics]
func SetMetricsRegistry(reg metrics.Registry) {
    m := &<Name>Metrics{ /* gauges/counters */ }
    metricsPtr.Store(m)
}
```

Register in `registry.Registration`:
```go
ConfigureMetrics: func(reg any) { SetMetricsRegistry(reg.(metrics.Registry)) },
```

## 5-Stage Protocol

| Stage | Direction | What happens |
|-------|-----------|-------------|
| 1 | Plugin -> Engine | Declare registration (families, commands, capabilities) |
| 2 | Engine -> Plugin | Send config sections (`OnConfigure` callback) |
| 3 | Plugin -> Engine | Declare per-peer capabilities |
| 4 | Engine -> Plugin | Send command registry |
| 5 | Plugin -> Engine | Ready; enter event loop (`OnEvent` callback) |

## Event Delivery

| Plugin type | Callback | Data format |
|-------------|----------|-------------|
| Internal (goroutine) | `OnStructuredEvent` | `*rpc.StructuredEvent` (lazy attrs, zero-copy) |
| External (subprocess) | `OnEvent` | JSON text string |

## Sending Commands

Plugins send commands to the engine as text -- never import sibling plugins:

```go
status, data, err := p.DispatchCommand(ctx, "rib routes 10.0.0.0/24")
```

The engine routes by prefix to the owning plugin's CLIHandler.

## Optional Capabilities

### Route Filters

```go
IngressFilter: func(info registry.PeerFilterInfo, raw []byte, m map[string]any) (bool, []byte) {
    // Return (accept, possibly-modified-bytes)
},
EgressFilter: func(info registry.PeerFilterInfo, raw []byte, m map[string]any, acc *registry.ModAccumulator) bool {
    // Return accept/reject
},
FilterStage:    registry.FilterStagePolicy,
FilterPriority: 0,
```

### NLRI Codec

```go
Families: []string{"ipv4/my-safi"},
Features: "nlri",
InProcessNLRIDecoder: func(family, hex string) (string, error) { ... },
InProcessNLRIEncoder: func(family string, args []string) (string, error) { ... },
```

### In-Process Decoder (for `ze bgp decode`)

```go
InProcessDecoder: func(input, output *bytes.Buffer) int {
    return RunDecodeMode(input, output)
},
```

## Features String

Space-separated flags: `"nlri yang capa"`.

| Flag | Meaning |
|------|---------|
| `nlri` | Plugin provides NLRI encode/decode |
| `yang` | Plugin has YANG schema |
| `capa` | Plugin handles capabilities |

## Plugin vs Component vs Subsystem

| | Plugin | Component | Subsystem |
|-|--------|-----------|-----------|
| Registration | `registry.Register()` in `init()` | Wired in startup code | Hard-coded in reactor |
| Coupling | Loose (discovered via registry) | Tight (direct imports) | Tightest (global state) |
| Removal test | Remove blank import in all.go | Cascade of import errors | Immediate panic |
| Config | Via SDK `OnConfigure` callback | Embedded in core YANG | Passed as args |

## Reference Implementations

| Variant | File | Notes |
|---------|------|-------|
| Full-featured (RIB, filters, metrics) | `plugins/rib/register.go` | Reference for complex plugins |
| Capability + decode | `plugins/gr/register.go` | GR plugin with CapabilityCodes |
| Attribute modifier | `plugins/role/register.go` | OTC attribute handling |
| Minimal with filters | `plugins/filter_community/register.go` | Simple filter plugin |
| NLRI codec | `plugins/nlri_ipv4/register.go` | NLRI decode/encode |

## Checklist

```
[ ] Create plugins/<name>/register.go with init() -> registry.Register()
[ ] Create plugins/<name>/<name>.go with atomic logger + Run<Name>Plugin()
[ ] Run make generate (updates all.go)
[ ] Update TestAllPluginsRegistered expected count
[ ] If YANG config: create schema/ subdir with register.go + embed.go + .yang file
[ ] If capabilities: set CapabilityCodes, Features: "capa"
[ ] If NLRI codec: set Families, InProcessNLRIDecoder/Encoder, Features: "nlri"
[ ] If route metadata: register keys in docs/architecture/meta/README.md
[ ] Functional tests in test/plugin/
[ ] No imports of sibling plugins (use DispatchCommand for inter-plugin communication)
```
