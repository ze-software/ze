# Pattern: Registration Architecture

Ze has a modular core where everything plugs in via registration. No component
hard-codes knowledge of another. Discovery happens through registries, not imports.

## The Model

```
Blank import (all.go)
  -> package init()
    -> Registry.Register(...)
      -> Runtime query: Registry.Lookup(), Registry.All(), etc.
```

Every module (plugin, schema, env var, command, validator) follows this model.
The core never imports a specific plugin. Plugins never import each other.
Communication is through text commands (`DispatchCommand`) and registries.

`make generate` auto-generates `internal/component/plugin/all/all.go` by scanning
the filesystem for `register.go` files. Adding/removing a plugin = add/remove files
+ run `make generate`.

## All Registration Mechanisms

### Plugin Registry (central)

The primary registry. A plugin registration carries everything the core needs.

| Field | Purpose |
|-------|---------|
| `Name` | Plugin identity (kebab-case) |
| `Description` | Human-readable |
| `RunEngine` | Engine-mode entry point `func(net.Conn) int` |
| `CLIHandler` | CLI dispatch `func([]string) int` |
| `Families` | Address families `[]string{"ipv4/unicast"}` |
| `CapabilityCodes` | BGP capability codes `[]uint8{64}` |
| `ConfigRoots` | Config sections plugin needs `[]string{"bgp"}` |
| `Dependencies` | Other plugins that must load first |
| `YANG` | YANG schema string |
| `Features` | Space-separated flags: `"nlri yang capa"` |
| `EventTypes` | Events this plugin produces `[]string{"update-rpki"}` |
| `SendTypes` | Send operations this plugin enables |
| `IngressFilter` | Route ingress filter function |
| `EgressFilter` | Route egress filter function |
| `FilterStage` | Coarse ordering (Protocol=0, Policy=100, Annotation=200) |
| `FilterPriority` | Fine ordering within stage |
| `InProcessNLRIDecoder` | NLRI hex -> JSON |
| `InProcessNLRIEncoder` | NLRI args -> hex |
| `InProcessDecoder` | Full message decode (for `ze bgp decode`) |
| `ConfigureEngineLogger` | Callback to set plugin logger |
| `ConfigureMetrics` | Callback to set metrics registry |

**Location:** `internal/component/plugin/registry/registry.go`
**Registration:** `registry.Register(Registration{...})` in plugin `init()`
**Query:** `registry.Lookup(name)`, `registry.All()`, `registry.FamilyMap()`,
`registry.CapabilityMap()`, `registry.IngressFilters()`, `registry.EgressFilters()`
**Validation:** rejects empty name, duplicates, invalid family format, circular deps
**Count:** ~40 plugins

### YANG Module Registry

Every config schema, API definition, and CLI tree registers its YANG module.

**Location:** `internal/component/config/yang/register.go`
**Registration:** `yang.RegisterModule(name, content)` in `schema/register.go` init()
**Query:** `yang.Modules()`, `yang.Loader.Resolve()`
**Triggered by:** blank imports of schema packages
**Count:** 38+ modules

Two-phase loading:
1. `LoadEmbedded()` -- core types (`ze-extensions.yang`, `ze-types.yang`)
2. `LoadRegistered()` -- everything else from init() registrations

### Environment Variable Registry

Every `ze.*` variable must be declared before use.

**Location:** `internal/core/env/registry.go`
**Registration:** `env.MustRegister(EnvEntry{Key, Type, Default, Description, Private, Secret})`
**Query:** `env.Get(key)`, `env.GetInt()`, `env.GetBool()`, `env.GetDuration()`
**Validation:** `env.Get()` on unregistered key = abort (programming error)
**Special:** `Secret: true` clears from OS env after first read. Prefix wildcards supported.
**Count:** 20+ entries across cmd/, internal/component/

### RPC Command Registry (online commands)

Online commands register their handlers for the daemon dispatcher.

**Location:** `internal/component/plugin/server/rpc_register.go`
**Registration:** `pluginserver.RegisterRPCs(RPCRegistration{WireMethod, Handler, RequiresSelector})`
**Query:** `AllBuiltinRPCs()`, `LoadBuiltins()` maps WireMethod -> CLI path via YANG
**Triggered by:** init() in `internal/component/cmd/<verb>/<verb>.go` and handler packages
**Count:** 18+ handler packages

### YANG Validator Registry

Custom validators for YANG leaves that need runtime validation beyond enum/range/pattern.

**Location:** `internal/component/config/yang/validator_registry.go`
**Registration:** `ValidatorRegistry.Register(name, CustomValidator{ValidateFn, CompleteFn})`
**Query:** `GetValidateExtension()` reads `ze:validate` from YANG
**Validation:** `CheckAllValidatorsRegistered()` panics if any `ze:validate` name has no handler
**YANG ref:** `ze:validate "name"` on leaf. Pipe-separated for multiple: `"a|b"`.

### CLI Command Registry

Root subcommand metadata (`ze bgp`, `ze ping`, ...) and offline local
command handlers (`show bgp decode`, `ping`, ...). Every subcommand
package under `cmd/ze/` owns a `register.go` whose `init()` registers
itself; `cmd/ze/main.go` imports the packages for side-effects and the
registry is populated before dispatch.

**Location:** `cmd/ze/internal/cmdregistry/registry.go`
**Registration:**
- `cmdregistry.RegisterRoot(name, Meta)` for `ze <name>` metadata
- `cmdregistry.MustRegisterLocal(path, handler)` for path-keyed handlers
- `cmdregistry.MustRegisterLocalMeta(path, handler, meta)` when the handler also wants display metadata

**Query:**
- `cmdregistry.LookupLocal(words)` -- longest-prefix handler lookup (used by `RunCommand` and `main.go`'s dispatch fallback)
- `cmdregistry.ListRoot()` / `ListLocal()` -- used by `help --ai`

**Cycle avoidance:** `cmdutil` imports `cli` for tree walking. A
subcommand package that imports `cmdutil` from its `register.go` would
cycle through `cli -> cmdutil -> cli`. `cmdregistry` is a leaf package
(stdlib-only), so every `register.go` can import it safely.
`cmdutil.RegisterLocalCommand` remains as a thin passthrough to
`cmdregistry.RegisterLocal` for backward compatibility.

**Pattern guidance:** `ai/patterns/cli-command.md` -- "Command
Registration (BLOCKING)" section.

### Attribute Name Registry

Maps BGP attribute codes to human-readable names.

**Location:** `internal/component/bgp/attribute/attribute.go`
**Registration:** `attribute.RegisterName(code, name)` in plugin init()
**Query:** `AttributeCode.String()` for display
**Count:** 20+ pre-registered (ORIGIN, AS_PATH, NEXT_HOP, etc.) + plugin additions

### Attribute Modification Handler Registry

Plugins that modify attributes during egress (forward path).

**Location:** `internal/component/plugin/registry/registry.go`
**Registration:** `registry.RegisterAttrModHandler(code, handler)` in plugin init()
**Query:** `AttrModHandlerFor(code)`, `AttrModHandlers()`
**Registered:** role (OTC code 35), filter-community (codes 8, 16, 32)

### Filter Chain (via Plugin Registry)

Route filters ordered by stage + priority. Part of plugin Registration, not separate.

| Stage | Value | Purpose | Example |
|-------|-------|---------|---------|
| Protocol | 0 | RFC-mandated checks | Loop detection (RFC 4271/4456) |
| Policy | 100 | Operator-configured | Community filtering |
| Annotation | 200 | Protocol stamps | OTC stamping (RFC 9234) |

**Query:** `registry.IngressFilters()`, `registry.EgressFilters()` return ordered slices

### Metrics (no central registry)

No central metric registry. Each component creates metrics via a `metrics.Registry` interface
(Counter, Gauge, Histogram factories). The interface is injected via `ConfigureMetrics` callback.

**Location:** `internal/core/metrics/`
**Convention:** metric names follow `ze_<subsystem>_<metric>`
**Consumers:** `/metrics` HTTP endpoint, `show metrics` CLI

### Web Routes (no registry)

HTTP handlers registered directly on `http.ServeMux` during hub startup. No discovery mechanism.

**Location:** `cmd/ze/hub/main.go`, `internal/component/web/server.go`
**Pattern:** `srv.Handle("GET /show/...", authWrap(handler))`

### Route Metadata (convention, no registry)

Dynamic `map[string]any` passed through filter chain. No formal registry.
Plugins define keys by convention (prefix with plugin name).

**Location:** filter function signatures
**Convention:** `"src-role"` (role plugin), etc.
**Documented:** `docs/architecture/meta/README.md`

## How Registration Flows at Startup

```
1. main() imports internal/component/plugin/all
2. all.go blank-imports every plugin + schema package
3. Each package's init() runs:
   - Plugins call registry.Register()
   - Schemas call yang.RegisterModule()
   - Env vars call env.MustRegister()
   - RPC handlers call pluginserver.RegisterRPCs()
   - Attr names call attribute.RegisterName()
   - Attr mod handlers call registry.RegisterAttrModHandler()
4. main() continues:
   - yang.LoadEmbedded() + yang.LoadRegistered()
   - CheckAllValidatorsRegistered()
   - LoadBuiltins() maps WireMethod -> CLI path
   - registerLocalCommands()
   - Start HTTP server (web routes)
   - Start BGP subsystem (filter chain built from registry)
```

All registration is complete before any concurrent access. Registries are read-only after init.

## Adding Something New

### New plugin
See `patterns/plugin.md`. Touch: plugin registry, YANG module registry (if schema),
attribute name registry (if new attr), attr mod handler (if modifying attrs).
Run `make generate`.

### New config option
See `patterns/config-option.md`. Touch: YANG module (leaf definition),
env var registry (if under environment/), validator registry (if custom validation).

### New CLI command (online)
See `patterns/cli-command.md`. Touch: RPC command registry, YANG module (command tree),
optionally CLI local command registry (if also offline).

### New env var
Touch: env var registry (`env.MustRegister()`), YANG module (leaf under environment/).
See `rules/config-design.md`: every YANG environment leaf = matching env var.

### New YANG module
Create `schema/register.go` + `schema/embed.go` with `//go:embed`. Run `make generate`.

### New attribute code
Touch: attribute name registry, optionally attr mod handler registry.
In plugin `register.go`: `attribute.RegisterName(code, "NAME")`.

### New filter
Part of plugin registration. Set `IngressFilter`/`EgressFilter` + `FilterStage` + `FilterPriority`.
See `patterns/plugin.md`.

### New event/send type
Part of plugin registration. Set `EventTypes`/`SendTypes` fields.
Consumers use `registry.PluginForEventType()` / `registry.PluginForSendType()`.

## Invariants

| Rule | Enforced by |
|------|-------------|
| No unregistered env var access | `env.Get()` aborts on unknown key |
| No YANG validator without handler | `CheckAllValidatorsRegistered()` panics at startup |
| No duplicate plugin names | `registry.Register()` returns error |
| No circular plugin deps | Dependency resolver rejects cycles |
| No missing plugin deps | Resolver checks all declared deps exist |
| Plugins never import siblings | `rules/plugin-design.md` import rules + code review |
| All blank imports auto-generated | `make generate` + `scripts/codegen/plugin_imports.go` |
| YANG is source of truth for CLI tree | WireMethod -> YANG path mapping in dispatcher |
