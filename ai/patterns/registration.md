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

`./le repository generate` auto-generates `internal/component/plugin/all/all.go` by scanning
the filesystem for `register.go` files. Adding/removing a plugin = add/remove files
+ run `./le repository generate`.

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
| `InProcessNLRIDecoder` | NLRI hex -> JSON |
| `InProcessNLRIEncoder` | NLRI args -> hex |
| `InProcessDecoder` | Full message decode (for `ze bgp decode`) |
| `ConfigureEngineLogger` | Callback to set plugin logger |
| `ConfigureMetrics` | Callback to set metrics registry |

**Location:** `internal/component/plugin/registry/registry.go`
**Registration:** `registry.Register(Registration{...})` in plugin `init()`
**Query:** `registry.Lookup(name)`, `registry.All()`, `registry.FamilyMap()`,
`registry.CapabilityMap()` (route filter chains live in
`internal/component/bgp/filterapi`, not here)
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

### CLI Command Registry (in-process command providers)

Root commands (`ze bgp`, `ze interface`, ...) and offline local handlers
(`show bgp decode`, `show config history`, ...). The **owner package** owns a
`register.go` whose `init()` registers itself; the registry **dispatches**
owner-backed roots, so the owner lives under `internal/` and `cmd/ze` never
imports it directly. `cmd/ze` keeps only no-owner / process-global commands.

**Location:** `internal/component/command/registry/registry.go` (stdlib-only leaf).
**Registration:**
- `registry.MustRegisterRootHandler(name, handler, Meta)` -- **owner-backed** `ze <name>`: handler + metadata, dispatched by the registry. `handler` is `func(*RuntimeContext, []string) int`. Rejects empty name / nil handler / duplicate owner.
- `registry.RegisterRoot(name, Meta)` -- **no-owner / process-global** metadata only; `cmd/ze/main.go` dispatches it (start, version, help, ...).
- `registry.MustRegisterLocal(path, handler)` / `MustRegisterLocalMeta(...)` -- path-keyed offline shortcuts.
- `registry.SetRuntimeStorage(fn)` (main.go) + `registry.RuntimeStorage()` -- blob store for storage-backed local shortcuts.

**Query:**
- `registry.LookupRoot(name)` -- owner root dispatch (used by `main.go` before the static switch).
- `registry.LookupLocal(words)` -- longest-prefix handler lookup (used by `RunCommand` and `main.go`).
- `registry.ListRoot()` / `ListLocal()` / `ListRootBySection()` -- used by `help ai`.

**Leaf guarantee:** the registry imports only the standard library, so any owner
(`internal/component/*`, `internal/plugins/*`, `internal/core/*`) imports it from
`init()` with no cycle. Dispatch-time deps (storage, plugin list, process flags)
flow through `RuntimeContext` (heavy types as function values), never imported
into the registry.

**Linking:** owner `init()` runs because the package is blank-imported. Until the
generated command-provider aggregator lands (Phase 7 of command-surface-ownership),
the blank imports are hand-listed in `cmd/ze/main.go`.

**Pattern guidance:** `ai/patterns/cli-command.md` -- "Command
Registration (BLOCKING)" section.

### Subcommand Dispatch (`subdispatch.Dispatcher`)

Root commands that have their own subcommands (install, uninstall, analyze, perf)
use `internal/core/subdispatch.Dispatcher` for map-based dispatch and derived usage.

**Location:** `internal/core/subdispatch/subdispatch.go`

**Pattern:** the feature package creates a `Dispatcher`, exports `Register`/`Dispatch`,
and populates subcommands. The root handler delegates to `Dispatch()`.

```go
// internal/<feature>/dispatch.go
package feature

import "github.com/ze-software/ze/internal/core/subdispatch"

var dispatcher = subdispatch.New("<feature>", "<one-line summary>")

func Register(name string, handler func([]string) int, meta subdispatch.SubMeta) {
    dispatcher.Register(name, handler, meta)
}

func Dispatch(args []string) int { return dispatcher.Dispatch(args) }
```

```go
// internal/<feature>/register.go
package feature

import "github.com/ze-software/ze/internal/component/command/registry"

func init() {
    populateSubcommands()
    registry.MustRegisterRootHandler("<feature>", func(_ *registry.RuntimeContext, args []string) int {
        return Dispatch(args)
    }, registry.Meta{
        Description: "<summary>",
        Mode:        "offline",
        Section:     registry.SectionTest,
        SubsFunc:    func() string { return dispatcher.Subcommands() },
    })
}
```

**Existing examples:** `cmd/ze/install/`, `cmd/ze/uninstall/`

**Usage text** is derived by `Dispatcher.usage()` from registered subcommands using
`internal/core/helpfmt`. No static help strings.

### Binary Personality Registration

Binary personalities (ze-test, ze-chaos, ze-perf, ze-analyze) are separate binaries
built from `cmd/ze/` with build tags. Each personality's domain code lives in
`internal/`, self-registers via `init()`, and the cmd/ze file is a build-tagged
blank import.

**Pattern:**
```go
// cmd/ze/ze_<personality>_register.go
//go:build ze_<personality>

package main

import _ "github.com/ze-software/ze/internal/<feature>"
```

The blank import triggers the feature package's `init()`, which registers the root
handler with the CLI command registry. `dispatchMain()` in `dispatch.go` uses
`defaultDispatch()` which looks up the root handler in the registry.

**Do not:**
- Put domain logic in `cmd/ze/` files (no switch/case dispatch, no handlers, no types)
- Set `binarySetup` when the feature self-registers via init() (unnecessary)
- Use build tags on `internal/` packages (tags gate binary composition via blank imports, not library availability)

**Existing gap:** ze-test uses a local map in `cmd/ze/ze_test_register.go` instead
of `subdispatch.Dispatcher`. Mock server domain code is in `cmd/ze/ze_test_*.go`
instead of `internal/test/mock/`. `spec-cmd-reorg` addresses this.

### Doctor Check Registry

Offline readiness checks for `ze doctor`. The runner owns execution phases so
missing-config and parse-failure behavior stay explicit. The component,
backend, command, or plugin that owns the runtime dependency owns the check
registration, check function, and unit test.

**Current implementation:** `internal/core/diagnostic/doctor_registry.go`
**Registration:** owner package `register.go` or `doctor_check.go`, using the doctor check registry. If the current registry location is not importable from the owner, move or expose a leaf registry API before adding the check.
**Runner boundary:** `internal/component/doctor` queries registered checks by phase and keeps only runner/output tests plus checks with no narrower owner.
**Metadata:** name, phase, order, component, dependencies, platforms, diagnostic codes, check function
**Validation:** rejects duplicate check names, unknown phases, missing metadata, invalid lower-kebab identifiers, and duplicate codes within one check
**Code metadata:** registered `doctor-*` codes must resolve through `diagnostic.Lookup`

### Attribute Name Registry

Maps BGP attribute codes to human-readable names.

**Location:** `internal/core/bgp/attribute/attribute.go`
**Registration:** `attribute.RegisterName(code, name)` in plugin init()
**Query:** `AttributeCode.String()` for display
**Count:** 20+ pre-registered (ORIGIN, AS_PATH, NEXT_HOP, etc.) + plugin additions

### Attribute JSON Formatter Registry

Plugins register how their attribute types render to JSON. Replaces the
hardcoded switch that was in `format/text_json.go`. The format package
calls `GetJSONFormatter(code)`, writes the key, calls `AppendValue(buf, attr)`,
writes flags. Returns nil = fall through to hex.

**Location:** `internal/core/bgp/attribute/json.go`
**Registration:** `attribute.RegisterJSONFormatter(code, key, fn)` in owner's `register.go`
**Query:** `attribute.GetJSONFormatter(code)` returns `*JSONFormatter` or nil
**Formatter:** named function in owner's `json.go` (never inline closure)
**Consumer:** `format/text_json.go:appendAttributeJSON` -- mark/truncate pattern
**Registered:**
- `attribute/register.go`: Origin, NextHop, ASPath, MED, LocalPref (core, no plugin)
- `plugins/filter_community/register.go`: Community, LargeCommunity, ExtCommunity, IPv6ExtCommunity
- `plugins/aigp/register.go`: AIGP

**Ownership rule:** if a plugin registers `AttrModHandler` for a code, it also
owns that code's JSON formatter. Core attributes (RFC 4271 mandatory, no
dedicated plugin) register from `attribute/register.go`.

**Banned:** switch cases in `appendAttributeJSON`, inline closures in registration,
`AppendValue(nil, attr)` (allocates on hot path -- pass buf directly).

### Attribute Modification Handler Registry

Plugins that modify attributes during egress (forward path).

**Location:** `internal/component/bgp/filterapi/filterapi.go` (BGP-owned seam package)
**Registration:** `filterapi.RegisterAttrModHandler(code, handler)` in plugin init()
**Query:** `filterapi.AttrModHandlerFor(code)`, `filterapi.AttrModHandlers()`
**Registered:** role (OTC code 35), filter-community (codes 8, 16, 32)

### Filter Chain (via filterapi)

Route filters ordered by stage + priority. Registered with
`filterapi.Register(filterapi.Filter{...})` in the plugin's init(),
separate from the generic `registry.Registration` (the generic registry
carries no protocol filter knowledge).

| Stage | Value | Purpose | Example |
|-------|-------|---------|---------|
| Protocol | 0 | RFC-mandated checks | Loop detection (RFC 4271/4456) |
| Policy | 100 | Operator-configured | Community filtering |
| Annotation | 200 | Protocol stamps | OTC stamping (RFC 9234) |

**Query:** `filterapi.IngressFilters()`, `filterapi.EgressFilters()` return ordered slices

### Reactor Capability Flag (via filterapi)

A boolean reactor capability that a plugin owns and activates at init(), so
that removing the plugin package makes the corresponding reactor path inert
("delete the folder, the feature vanishes"). Same init()-time model as the
filter seams above; the reactor reads it once at construction (never a
per-message lookup) and caches it on the `Reactor`.

**Location:** `internal/component/bgp/filterapi/filterapi.go` (BGP-owned seam package)
**Registration:** `filterapi.EnableRSForwarding()` in plugin init()
**Query:** `filterapi.RSForwardingEnabled()` (read once in `reactor.New`, cached in `rsForwardingEnabled`; the per-UPDATE fast-path gate checks the cached bool)
**Registered:** bgp-rs (route-server fast-path forwarding)

### Metrics (no central registry)

No central metric registry. Each component creates metrics via a `metrics.Registry` interface
(Counter, Gauge, Histogram factories). The interface is injected via `ConfigureMetrics` callback.

**Location:** `internal/core/metrics/`
**Convention:** metric names follow `ze_<subsystem>_<metric>`
**Consumers:** `/metrics` HTTP endpoint, `show metrics` CLI

### Show Enricher Registry

Plugins register enrichers for show commands. At show time, the handler calls
`show.Enrich(command, base)` to merge plugin-contributed data into its output.
Enrichers mutate the base `map[string]any` in place.

<!-- source: internal/core/show/show.go -- Register, Enrich, EnrichBrief -->

**Location:** `internal/core/show/show.go` (leaf package, stdlib-only imports)
**Registration:** `show.MustRegister(command, key, show.Enricher{Detail, Brief})` in plugin `init()`
**Consumer:** handler calls `show.Enrich(command, base)` or `show.EnrichBrief(command, base)`
**Key:** `command` is the CLI path with selectors stripped (e.g., `"show subscriber detail"`);
`key` is unique per command (e.g., `"cos"`). Duplicate keys are rejected.
**Ordering:** enrichers called in alphabetical key order per command.
**Panic safety:** `Enrich()` recovers from panicking enrichers and logs a warning.
**Test:** `show.ResetForTest()` clears all registrations.

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
   - Attr mod handlers call filterapi.RegisterAttrModHandler()
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
See `ai/patterns/plugin.md`. Touch: plugin registry, YANG module registry (if schema),
attribute name registry (if new attr), attr mod handler (if modifying attrs).
Run `./le repository generate`.

### New config option
See `ai/patterns/config-option.md`. Touch: YANG module (leaf definition),
env var registry (if under environment/), validator registry (if custom validation).

### New CLI command (online)
See `ai/patterns/cli-command.md`. Touch: RPC command registry, YANG module (command tree),
optionally CLI local command registry (if also offline).

### New env var
Touch: env var registry (`env.MustRegister()`), YANG module (leaf under environment/).
See `ai/rules/config.md`: every YANG environment leaf = matching env var.

### New YANG module
Create `schema/register.go` + `schema/embed.go` with `//go:embed`. Run `./le repository generate`.

### New attribute code
Touch: attribute name registry, optionally attr mod handler registry.
In plugin `register.go`: `attribute.RegisterName(code, "NAME")`.

### New filter
Register with `filterapi.Register(filterapi.Filter{Name, Stage, Priority, Ingress, Egress})`
(`internal/component/bgp/filterapi`) in the plugin's init(), alongside `registry.Register()`.
See `ai/patterns/plugin.md`.

### New reactor capability flag (plugin-owned)
When a reactor code path should exist only while a plugin does, add a bool +
`EnableX()`/`XEnabled()` to `internal/component/bgp/filterapi` (include it in
Snapshot/Restore/ResetForTest), call `EnableX()` in the plugin's init(), read it
once in `reactor.New` into a cached field, and gate the path on that field.
Never spell the plugin name in reactor/central code. Example: `EnableRSForwarding`
(route-server fast path).

### New show enricher (in-process)
Register with `show.MustRegister(command, key, show.Enricher{Detail: fn, Brief: fn})`
(`internal/core/show`) in the plugin's init(). Add `show.Enrich()` or
`show.EnrichBrief()` calls in the target handler if not already present.

### New show enricher (external plugin)
Declare `EnricherDecl{Command, Key}` in `DeclareRegistrationInput.Enrichers` at
Stage 1. Register `OnEnrichShow(fn)` callback in the SDK. The server registers a
proxy enricher via `show.Register()` for each declaration; at show time the proxy
serializes the base map, calls `ze-plugin-callback:enrich-show` with a 2s timeout,
and merges the response into the base map. Cleanup: proxy enrichers are removed via
`show.Unregister()` when the plugin process exits (`RegisterProcessCleanup` hook).

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
| Plugins never import siblings | `ai/rules/plugins.md` import rules + code review |
| No duplicate show enricher keys | `show.Register()` returns error; `show.MustRegister()` panics |
| All blank imports auto-generated | `./le repository generate` + `internal/le/pluginimports/pluginimports.go` |
| YANG is source of truth for CLI tree | WireMethod -> YANG path mapping in dispatcher |
