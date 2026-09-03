# Pattern: Plugin

Structural template for creating a Ze plugin.
Rules: `ai/rules/plugins.md`. Architecture: `docs/architecture/core-design.md`.

## Also Read

| Rule | When it applies |
|------|----------------|
| `ai/rules/goroutine-lifecycle.md` | OnStarted goroutines, worker patterns, cleanup |
| `ai/rules/plugins.md` (Cross-Boundary Value Types) | Any data crossing plugin boundaries |
| `ai/rules/plugins.md` (DirectBridge) | Sync request/response to/from engine |
| `ai/rules/plugins.md` (EventBus Typed Payloads) | Async broadcast events |
| `ai/rules/plugins.md` | Calling another `internal/component/*` package's function directly instead of through DirectBridge/DispatchCommand |
| `ai/rules/go-standards.md` | Plugin name, YANG prefix, log subsystem |
| Full navigation: `ai/INDEX.md` | |

## File Structure

```
internal/component/bgp/plugins/<name>/
  register.go         # REQUIRED: init() -> registry.Register()
  <name>.go           # REQUIRED: Package doc, logger, RunXxxPlugin()
  <name>_test.go      # Tests
  yang/               # If plugin has YANG config or command schema
    ze-<name>-conf.yang # hand-written; the only hand-written file here
    embed.go          # GENERATED: //go:embed ze-<name>-conf.yang
    register.go       # GENERATED: init() -> yang.RegisterModule()
```

After creating, run `./le repository generate` to update `internal/component/plugin/all/all.go`.

## register.go Template

```go
package bgp_<name>

import (
    "fmt"
    "os"

    "github.com/ze-software/ze/internal/core/slogutil"
    "github.com/ze-software/ze/internal/component/plugin/cli"
    "github.com/ze-software/ze/internal/component/plugin/registry"
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
        // YANG:            zeyang.Ze<Name>YANG,
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

    "github.com/ze-software/ze/internal/core/slogutil"
    sdk "github.com/ze-software/ze/pkg/plugin/sdk"
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

Register in `registry.Registration`. The field is typed, so it takes the
registry directly and there is no assertion to write:
```go
ConfigureMetrics: func(reg metrics.Registry) { SetMetricsRegistry(reg) },
```
<!-- source: internal/component/plugin/registry/registry.go -- Registration.ConfigureMetrics -->
<!-- source: internal/component/bgp/plugins/role/register.go -- ConfigureMetrics call site -->

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

## Core-to-Plugin Communication

For choosing between EventBus and DirectBridge, see
`ai/rules/plugins.md` "DirectBridge: Choosing the Right Communication Pattern".
Short version: EventBus for async broadcast, DirectBridge for sync request/response.

## Optional Capabilities

### Route Filters

A BGP route filter takes one of two routes, and the choice is decided by WHEN
the filter's name exists.

**A filter named in CONFIG declares its filter TYPE on the registration.** The
operator writes `bgp { policy { <type> NAME { ... } } }`, so the instance names
are unknown at compile time and none of them can be registered at Stage 1.
`FilterTypes` names the YANG list instead, `BuildFilterRegistry`
(`internal/component/bgp/config/filter_registry.go`) discovers each instance
from the `ze:filter` marker on that list, and a chain ref canonicalizes to
`<plugin>:NAME`. The plugin answers each dispatch in `OnFilterUpdate`. This is
the route `filter_aspath`, `filter_prefix` and `filter_path_asn` take:

```go
FilterTypes: []string{"as-path-list"},   // the YANG list name under bgp/policy
```

**A filter type that discharges a config obligation declares which one.** A
config rule can require a peer to name a filter that performs a given check, and
the rule must not spell a filter type: it asks the registry which types carry
the obligation (`registry.FilterTypesDischarging`). Declaring it keeps the type
name inside the plugin, and lets a second implementation qualify later:

```go
FilterObligations: []string{filterapi.ObligationTransitLeak},
```

The obligation names are constants in the domain that defines them, and an
obligation declared by a plugin owning no filter type is a registration error.
The one obligation today is the transit-leak check RFC 7454 Section 9
recommends, required of a peer that declares an RFC 9234 role
(`internal/component/bgp/config/peers.go`, `validateLeakFilterObligations`).

```go
p.OnFilterUpdate(func(in *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
    return handleFilterUpdate(in), nil
})
```

**A filter whose name is fixed at COMPILE time registers with `filterapi`.** It
lives in the BGP-owned seam package `internal/component/bgp/filterapi`, not in
the generic plugin registry, and it goes in the same init() as
`registry.Register()`:

```go
filterapi.Register(filterapi.Filter{
    Name:     "bgp-myfilter", // plugin name; breaks ordering ties
    Stage:    filterapi.FilterStagePolicy,
    Priority: 0,
    Ingress: func(info filterapi.PeerFilterInfo, raw []byte, m map[string]any) (bool, []byte) {
        // Return (accept, possibly-modified-bytes)
        return true, nil
    },
    Egress: func(src, dst filterapi.PeerFilterInfo, raw []byte, m map[string]any, acc *filterapi.ModAccumulator) bool {
        // Return accept/reject
        return true
    },
})
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
[ ] Run ./le repository generate (updates all.go)
[ ] Regenerate the snapshots TestRegisteredPluginNames reads, in testdata/:
    go test -tags '<ze_core + features>' ./internal/component/plugin/all/ -update
    (the package path comes BEFORE -update; a package named after a flag the go
     command does not know is read as an argument for the test binary instead)
[ ] If the plugin owns a filter type: add its row to the TestFilterTypeMappings
    expected map, which is exhaustive in both directions
[ ] If YANG config or commands: create yang/ subdir with the .yang file, then generate embed.go + register.go
[ ] If capabilities: set CapabilityCodes, Features: "capa"
[ ] If NLRI codec: set Families, InProcessNLRIDecoder/Encoder, Features: "nlri"
[ ] If the plugin produces custom event types: set EventTypes (e.g. ["update-rpki"])
[ ] If the plugin adds a runtime dependency: set DoctorChecks (see ai/rules/repo-maintenance.md)
[ ] If route metadata: register keys in docs/architecture/meta/README.md, and create docs/architecture/meta/<name>.md from the template there
[ ] Functional tests in test/plugin/
[ ] No imports of sibling plugins (use DispatchCommand for inter-plugin communication)
```

`./le repository generate` populates the rest: CLI dispatch, plugin runners,
YANG embed and register glue, config roots, family and capability maps, and
decoder maps.

## Sub-Dispatcher Registration

A command group with sub-actions registers each handler rather than switching on
`args[0]`. The dispatcher then owns help, the unknown-command error, and the
"did you mean" suggestion.

```go
var fooDispatcher = newFooDispatcher()

func newFooDispatcher() *subdispatch.Dispatcher {
    d := subdispatch.New("foo", "Foo operations")
    d.Register("bar", runBar, subdispatch.SubMeta{Desc: "Do bar"})
    d.Register("baz", runBaz, subdispatch.SubMeta{Desc: "Do baz"})
    return d
}

func runFoo(args []string) int {
    return fooDispatcher.Dispatch(args)
}
```
