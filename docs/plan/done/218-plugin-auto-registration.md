# Spec: plugin-auto-registration

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `.claude/rules/plugin-design.md` - current plugin patterns
4. `internal/plugin/inprocess.go` - current hardcoded maps
5. `internal/plugin/resolve.go` - current plugin resolution
6. `cmd/ze/bgp/plugin.go` - current CLI dispatch
7. `cmd/ze/bgp/plugin_common.go` - PluginConfig struct and RunPlugin()
8. `cmd/ze/bgp/decode.go` - current decode maps

## Task

Replace all hardcoded plugin dispatch and registration with a compile-time `init()` registry pattern.

**Goal:** Adding or removing a plugin should require ONLY:
1. Creating/removing the plugin's `internal/plugin/<name>/` directory
2. Adding/removing ONE blank import line in a single imports file

No switch cases, no maps, no help text, no CLI wrappers to manually maintain.

**Non-goals:**
- Runtime plugin loading (Go does not support dynamic loading at runtime without CGO)
- Changing the 5-stage RPC protocol or SDK interface
- Changing external plugin behavior (fork/exec)
- Changing the PluginConfig struct semantics

## Required Reading

### Architecture Docs
- [ ] `.claude/rules/plugin-design.md` - current plugin patterns (CRITICAL)
- [ ] `.claude/rules/cli-patterns.md` - CLI dispatch patterns
- [ ] `docs/architecture/core-design.md` - overall system design

## Current Behavior (MANDATORY)

**Source files read:**
- [x] `cmd/ze/bgp/plugin.go` - hardcoded switch/case dispatch (9 plugins + test) and help text
- [x] `cmd/ze/bgp/plugin_common.go` - PluginConfig struct and RunPlugin() shared implementation
- [x] `cmd/ze/bgp/plugin_gr.go` - GR CLI wrapper
- [x] `cmd/ze/bgp/plugin_hostname.go` - Hostname CLI wrapper
- [x] `cmd/ze/bgp/plugin_llnh.go` - LLNH CLI wrapper
- [x] `cmd/ze/bgp/plugin_flowspec.go` - FlowSpec CLI wrapper
- [x] `cmd/ze/bgp/plugin_evpn.go` - EVPN CLI wrapper
- [x] `cmd/ze/bgp/plugin_vpn.go` - VPN CLI wrapper
- [x] `cmd/ze/bgp/plugin_bgpls.go` - BGP-LS CLI wrapper
- [x] `cmd/ze/bgp/plugin_rib.go` - RIB CLI wrapper
- [x] `cmd/ze/bgp/plugin_rr.go` - RR CLI wrapper
- [x] `cmd/ze/bgp/plugin_test_cmd.go` - Test plugin command
- [x] `cmd/ze/bgp/decode.go` - pluginCapabilityMap, pluginFamilyMap, inProcessDecoders
- [x] `internal/plugin/inprocess.go` - internalPluginRunners, internalPluginYANG, internalPluginWantsConfig, familyToPlugin
- [x] `internal/plugin/resolve.go` - internalPluginInfo, PluginInfo, IsInternalPlugin, AvailableInternalPlugins

### Current Hardcoded Registration Points (12 locations)

**MUST UPDATE for every new plugin (5 locations):**

| Location | File | What |
|----------|------|------|
| CLI dispatch switch | `cmd/ze/bgp/plugin.go:15-43` | `case "name": return cmdPluginName(args)` |
| CLI help text | `cmd/ze/bgp/plugin.go:46-82` | Human-readable plugin list |
| CLI wrapper file | `cmd/ze/bgp/plugin_<name>.go` | Creates PluginConfig, calls RunPlugin() |
| Engine runner map | `internal/plugin/inprocess.go:33-67` | `internalPluginRunners` map |
| Package imports | `internal/plugin/inprocess.go:1-20` | Import plugin packages |

**SOMETIMES UPDATE (7 locations, depends on plugin capabilities):**

| Location | File | What | When |
|----------|------|------|------|
| YANG schema map | `internal/plugin/inprocess.go:71-76` | `internalPluginYANG` | Plugin provides YANG config |
| Config roots map | `internal/plugin/inprocess.go:81-85` | `internalPluginWantsConfig` | Plugin wants config sections |
| Family-to-plugin map | `internal/plugin/inprocess.go:141-155` | `familyToPlugin` | Plugin handles address families |
| CLI family map | `cmd/ze/bgp/decode.go:316-326` | `pluginFamilyMap` | Plugin handles families for CLI decode |
| CLI capability map | `cmd/ze/bgp/decode.go:308-311` | `pluginCapabilityMap` | Plugin decodes capabilities |
| In-process decoders | `cmd/ze/bgp/decode.go:669-674` | `inProcessDecoders` | Plugin has decode-only function |
| Plugin metadata | `internal/plugin/resolve.go:40-68` | `internalPluginInfo` | Plugin has description/RFCs |

### Current Plugin Inventory

| Plugin | Engine Runner | YANG | Config Roots | Families | Capabilities | CLI Decode | Description |
|--------|--------------|------|--------------|----------|--------------|------------|-------------|
| rib | RunRIBPlugin | yes (ze-rib) | - | - | - | - | Route Information Base |
| gr | RunGRPlugin | yes (ze-graceful-restart) | bgp | - | - | capa | Graceful Restart |
| rr | RunRouteServer | - | - | - | - | - | Route Reflector |
| hostname | RunHostnamePlugin | yes (ze-hostname) | bgp | - | 73 (FQDN) | capa | FQDN capability |
| llnh | RunLLNHPlugin | yes (ze-link-local-nexthop) | bgp | - | 77 | capa | Link-local next-hop |
| flowspec | RunFlowSpecPlugin | - | - | ipv4/flow, ipv6/flow, ipv4/flow-vpn, ipv6/flow-vpn | - | nlri | FlowSpec |
| evpn | RunEVPNPlugin | - | - | l2vpn/evpn | - | nlri | EVPN |
| vpn | RunVPNPlugin | - | - | ipv4/vpn, ipv6/vpn | - | nlri | VPN |
| bgpls | RunBGPLSPlugin | - | - | bgp-ls/bgp-ls, bgp-ls/bgp-ls-vpn | - | nlri | BGP-LS |

**Behavior to preserve:**
- `ze bgp plugin <name> [flags]` CLI interface and all existing flags
- `ze bgp plugin help` listing all available plugins
- `ze bgp decode` using plugin family/capability maps for auto-dispatch
- `internal/plugin` package's `IsInternalPlugin()`, `AvailableInternalPlugins()`, `GetInternalPluginRunner()`, `GetPluginForFamily()`, `GetRequiredPlugins()`, `CollectPluginYANG()`, `GetAllInternalPluginYANG()`, `GetInternalPluginWantsConfig()`, `InternalPluginInfo()` public API
- All existing functional tests pass unchanged
- `plugin test` subcommand continues to work

**Behavior to change:**
- Registration moves from hardcoded maps to `init()` self-registration
- CLI dispatch moves from switch/case to registry lookup
- Help text is auto-generated from registry
- Plugin wrapper files (`plugin_<name>.go`) move from `cmd/ze/bgp/` into each plugin's own package

## Data Flow (MANDATORY)

### Entry Point
- Plugin registration happens at **import time** via `init()` functions
- CLI dispatch happens at `cmd/ze/bgp/plugin.go:cmdPlugin()` via registry lookup
- Engine plugin startup happens at `internal/plugin/inprocess.go` via registry lookup

### Transformation Path
1. Go compiler imports plugin packages (via blank imports in a single file)
2. Each plugin's `init()` calls a registration function with its full descriptor
3. Registration function stores the descriptor in a package-level registry
4. At runtime, `cmdPlugin()` looks up the registry by name instead of switch/case
5. Engine uses `GetInternalPluginRunner()` which reads from the same registry
6. CLI decode uses `pluginFamilyMap`/`pluginCapabilityMap` which are populated from registry
7. Help text is generated dynamically from registry entries

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin package -> Registry | `init()` calls `Register()` | [ ] |
| CLI dispatch -> Registry | `cmdPlugin()` looks up handler by name | [ ] |
| Engine startup -> Registry | `GetInternalPluginRunner()` reads registry | [ ] |
| Decode CLI -> Registry | `pluginFamilyMap` populated from registry | [ ] |

### Integration Points
- `internal/plugin/registry.go` (new) - central registry, replaces all hardcoded maps
- `cmd/ze/bgp/plugin.go` - switch replaced with registry lookup
- `cmd/ze/bgp/decode.go` - maps populated from registry
- `internal/plugin/inprocess.go` - maps replaced with registry queries

### Architectural Verification
- [ ] No bypassed layers (plugins register through a single registry)
- [ ] No unintended coupling (each plugin only knows about the registry interface)
- [ ] No duplicated functionality (one registry replaces 12 locations)
- [ ] Zero-copy preserved where applicable (no change to wire handling)

## Design

### Two-Level Registration

There are two distinct contexts where plugin information is needed:

**Level 1: Internal plugin registry** (`internal/plugin/`)
- Engine runner functions
- YANG schemas
- Config roots
- Family-to-plugin mapping
- Plugin metadata (description, RFCs)

**Level 2: CLI plugin registry** (`cmd/ze/bgp/`)
- CLI command handlers (the `cmdPlugin<Name>` functions)
- CLI decode functions (in-process decoders)
- CLI family-to-plugin mapping (for `ze bgp decode`)
- CLI capability-to-plugin mapping (for `ze bgp decode`)
- Help text generation

Both levels must be unified into a single registration call per plugin.

### Registration Descriptor

Each plugin registers a single descriptor containing ALL its metadata. This replaces the current scattered maps. The descriptor is a data structure with these fields:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| Name | string | yes | Plugin name (e.g., "flowspec") |
| Description | string | yes | Human-readable description for help text |
| RFCs | list of string | no | Related RFC numbers |
| Features | string | no | Space-separated feature list (e.g., "nlri yang") |
| SupportsNLRI | bool | no | Plugin can decode NLRI |
| SupportsCapa | bool | no | Plugin can decode capabilities |
| Families | list of string | no | Address families handled (e.g., "ipv4/flow") |
| Capabilities | list of uint8 | no | Capability codes handled (e.g., 73, 77) |
| ConfigRoots | list of string | no | Config roots wanted (e.g., "bgp") |
| YANG | string | no | YANG schema content |
| RunEngine | function | yes | Engine mode handler `func(net.Conn, net.Conn) int` |
| ConfigureEngineLogger | function | no | Configure logger for in-process mode |
| CLIHandler | function | yes | CLI handler `func([]string) int` |
| InProcessDecoder | function | no | Decode function for CLI `func(*bytes.Buffer, *bytes.Buffer) int` |

### Where Registration Happens

Each plugin's `init()` function lives in its OWN package, NOT in a central file. This is the key design decision that enables "add/remove folder = add/remove plugin".

**File location:** `internal/plugin/<name>/register.go`

Each plugin creates a `register.go` file that calls the central registry's `Register()` function from its `init()`.

### Single Import File

A single file controls which plugins are compiled in:

**File location:** `internal/plugin/all/all.go`

This file contains ONLY blank imports:

```
// Package all imports all internal plugins, triggering their init() registration.
// To add or remove a plugin, add or remove its import line.
package all

import (
    _ "codeberg.org/thomas-mangin/ze/internal/plugin/bgpls"
    _ "codeberg.org/thomas-mangin/ze/internal/plugin/evpn"
    _ "codeberg.org/thomas-mangin/ze/internal/plugin/flowspec"
    _ "codeberg.org/thomas-mangin/ze/internal/plugin/gr"
    _ "codeberg.org/thomas-mangin/ze/internal/plugin/hostname"
    _ "codeberg.org/thomas-mangin/ze/internal/plugin/llnh"
    _ "codeberg.org/thomas-mangin/ze/internal/plugin/rib"
    _ "codeberg.org/thomas-mangin/ze/internal/plugin/rr"
    _ "codeberg.org/thomas-mangin/ze/internal/plugin/vpn"
)
```

The `cmd/ze/` binary imports `internal/plugin/all` to pull in all plugins.

### Import Cycle Prevention

**Critical design constraint:** `internal/plugin/<name>/` currently imports from `internal/plugin/bgp/` (message, capability, nlri packages). The registry must live in a package that plugin packages can import WITHOUT creating cycles.

**Solution:** The registry lives in a **new leaf package** with no dependencies on any plugin:

**Package:** `internal/plugin/registry`

This package:
- Defines the `Registration` struct (the descriptor)
- Provides `Register(reg Registration)` function
- Provides query functions: `Get(name)`, `All()`, `Families()`, `Capabilities()`, etc.
- Has ZERO imports from any `internal/plugin/<name>/` package
- Has ZERO imports from `cmd/` packages

**Dependency graph:**

```
internal/plugin/registry        (leaf: no plugin imports)
    ^           ^
    |           |
internal/plugin/<name>/     cmd/ze/bgp/
    ^                           ^
    |                           |
internal/plugin/all         (blank imports all plugins)
    ^
    |
cmd/ze/main.go              (imports internal/plugin/all)
```

### CLI Handler Registration

Currently each plugin has a `cmd/ze/bgp/plugin_<name>.go` file that creates a `PluginConfig` and calls `RunPlugin()`. This logic must move into each plugin's package.

**Challenge:** `RunPlugin()` lives in `cmd/ze/bgp/` package. Plugin packages cannot import `cmd/` packages.

**Solution:** Move `PluginConfig` struct and `RunPlugin()` into a shared package that both `cmd/ze/bgp/` and plugin packages can import.

**New package:** `pkg/plugin/cli` (or `internal/plugin/cli`)

This package contains:
- The `PluginConfig` struct (moved from `cmd/ze/bgp/plugin_common.go`)
- The `RunPlugin()` function (moved from `cmd/ze/bgp/plugin_common.go`)
- Helper functions: `connsFromEnv()`, `readHexFromStdin()`, etc.

Each plugin then registers its CLI handler as a function that calls `cli.RunPlugin()` with its `cli.PluginConfig`.

### Registration Descriptor (Full)

The complete registration descriptor merges engine-side and CLI-side information:

| Field | Type | Required | Currently In | Maps To |
|-------|------|----------|--------------|---------|
| Name | string | yes | everywhere | map key |
| Description | string | yes | `internalPluginInfo` | help text |
| RFCs | list of string | no | `internalPluginInfo` | help text |
| Families | list of string | no | `familyToPlugin` + `pluginFamilyMap` | both maps |
| CapabilityCodes | list of uint8 | no | `pluginCapabilityMap` | capability map |
| ConfigRoots | list of string | no | `internalPluginWantsConfig` | config delivery |
| YANG | string | no | `internalPluginYANG` | schema loading |
| RunEngine | func(net.Conn, net.Conn) int | yes | `internalPluginRunners` | engine startup |
| ConfigureEngineLogger | func(string) | no | `internalPluginRunners` wrappers | in-process logger |
| CLIHandler | func([]string) int | yes | `plugin_<name>.go` | CLI dispatch |
| InProcessDecoder | func(*bytes.Buffer, *bytes.Buffer) int | no | `inProcessDecoders` | CLI decode fallback |

### What Changes in Each Existing File

**Files to DELETE (replaced by registry):**
- `cmd/ze/bgp/plugin_gr.go` - logic moves to `internal/plugin/gr/register.go`
- `cmd/ze/bgp/plugin_hostname.go` - logic moves to `internal/plugin/hostname/register.go`
- `cmd/ze/bgp/plugin_llnh.go` - logic moves to `internal/plugin/llnh/register.go`
- `cmd/ze/bgp/plugin_flowspec.go` - logic moves to `internal/plugin/flowspec/register.go`
- `cmd/ze/bgp/plugin_evpn.go` - logic moves to `internal/plugin/evpn/register.go`
- `cmd/ze/bgp/plugin_vpn.go` - logic moves to `internal/plugin/vpn/register.go`
- `cmd/ze/bgp/plugin_bgpls.go` - logic moves to `internal/plugin/bgpls/register.go`
- `cmd/ze/bgp/plugin_rib.go` - logic moves to `internal/plugin/rib/register.go`
- `cmd/ze/bgp/plugin_rr.go` - logic moves to `internal/plugin/rr/register.go`

**Files to MODIFY:**
- `cmd/ze/bgp/plugin.go` - replace switch/case with registry lookup, auto-generate help
- `cmd/ze/bgp/plugin_common.go` - move PluginConfig + RunPlugin to `internal/plugin/cli/`
- `cmd/ze/bgp/decode.go` - replace hardcoded maps with registry queries
- `internal/plugin/inprocess.go` - replace all hardcoded maps with registry queries
- `internal/plugin/resolve.go` - replace `internalPluginInfo` with registry queries

**Files to CREATE:**
- `internal/plugin/registry/registry.go` - central registration and query API
- `internal/plugin/cli/cli.go` - PluginConfig + RunPlugin (moved from cmd/)
- `internal/plugin/all/all.go` - blank imports file
- `internal/plugin/<name>/register.go` - one per plugin (9 files)

### Example: What a Plugin's register.go Looks Like

For the `gr` plugin (`internal/plugin/gr/register.go`):

The `init()` function calls `registry.Register()` with a descriptor containing:
- Name: "gr"
- Description: "Graceful Restart state management"
- RFCs: ["4724"]
- SupportsCapa: true
- YANG: the embedded schema content
- ConfigRoots: ["bgp"]
- RunEngine: gr.RunGRPlugin
- ConfigureEngineLogger: a function that calls gr.SetLogger
- CLIHandler: a function that creates a cli.PluginConfig and calls cli.RunPlugin
- No InProcessDecoder (GR doesn't support decode-only mode)

For the `flowspec` plugin (`internal/plugin/flowspec/register.go`):

- Name: "flowspec"
- Description: "FlowSpec NLRI encoding/decoding"
- RFCs: ["8955", "8956"]
- SupportsNLRI: true
- Families: ["ipv4/flow", "ipv6/flow", "ipv4/flow-vpn", "ipv6/flow-vpn"]
- RunEngine: flowspec.RunFlowSpecPlugin
- ConfigureEngineLogger: calls flowspec.SetFlowSpecLogger
- CLIHandler: creates PluginConfig with ExtraFlags for --family, calls RunPlugin
- InProcessDecoder: flowspec.RunFlowSpecDecode

### Auto-Generated Help Text

`pluginUsage()` in `cmd/ze/bgp/plugin.go` currently has hardcoded text. It should be replaced with a function that iterates the registry and formats help dynamically:

1. Query all registered plugins sorted by name
2. For each plugin, format: `  <name>    <description> [RFC references]`
3. Pad names to align descriptions
4. Keep the static header/footer text (usage examples, config examples)

### Auto-Generated Decode Maps

`cmd/ze/bgp/decode.go` currently has three hardcoded maps. These should be replaced with functions that query the registry:

1. `pluginFamilyMap` -> `registry.FamilyMap()` returns `map[string]string`
2. `pluginCapabilityMap` -> `registry.CapabilityMap()` returns `map[uint8]string`
3. `inProcessDecoders` -> `registry.InProcessDecoders()` returns the decoder map

These can be computed once at init time (since the registry is populated during init) or lazily on first access.

### The `test` Subcommand

`plugin test` is special - it's a debugging tool, not a real plugin. It does NOT register in the engine runner map or any decode map. It should remain as a hardcoded case in the CLI dispatch, separate from the registry. Alternatively, it could register in the CLI-only part of the registry with no engine runner.

**Decision:** Keep `plugin test` as a hardcoded special case in `cmdPlugin()`. It's not a real plugin.

### Thread Safety

`init()` functions run sequentially in Go (per package import order), so no mutex is needed during registration. The registry is write-once (during init) and read-many (during runtime). No synchronization needed.

### Validation at Registration Time

`Register()` should validate:
- Name is non-empty
- Name is unique (panic on duplicate - programming error, not runtime error)
- RunEngine is non-nil
- CLIHandler is non-nil
- If Families is set, each family string contains "/" (format validation)
- If CapabilityCodes is set, codes are > 0

Validation failures should `panic()` since they indicate a programming error caught at startup, not a runtime condition.

## Documentation Updates Required

### `.claude/rules/plugin-design.md`

This file contains the authoritative plugin development guide. It must be updated to:

1. **Replace the "New Plugin Checklist"** with a new checklist that reflects the registry pattern
2. **Add a "Plugin Registration" section** describing the `register.go` pattern
3. **Update the "Plugin Registration Pattern"** section (currently describes the switch/case in plugin.go)
4. **Update "Internal Plugin Runner Registration"** section (currently describes inprocess.go map)
5. **Update "In-Process Decoder Registration"** section (currently describes decode.go map)
6. **Update "Family-to-Plugin Mapping"** section
7. **Update "Capability-to-Plugin Mapping"** section
8. **Add import cycle prevention guidance**

The new "New Plugin Checklist" should be:

```
[ ] Create internal implementation: internal/plugin/<name>/<name>.go
[ ] Add package-level logger with SetLogger()
[ ] Implement SDK callback pattern (NewWithConn + callbacks + Run)
[ ] Create register.go with init() that calls registry.Register()
[ ] Add blank import to internal/plugin/all/all.go
[ ] Add YANG schema in schema/ subdirectory if plugin has configuration
[ ] Add functional tests in test/plugin/
```

Compare to the current checklist which has 12 items, many of which are the scattered registration points this spec eliminates.

### `CLAUDE.md`

No changes needed to CLAUDE.md - it references plugin-design.md which will be updated.

### Memory file

After implementation, update `/Users/thomas/.claude/projects/-Users-thomas-Code-codeberg-org-thomas-mangin-ze-main/memory/MEMORY.md` to note the new plugin registration pattern.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegister` | `internal/plugin/registry/registry_test.go` | Registration stores and retrieves descriptor | |
| `TestRegisterDuplicate` | `internal/plugin/registry/registry_test.go` | Duplicate name panics | |
| `TestRegisterEmptyName` | `internal/plugin/registry/registry_test.go` | Empty name panics | |
| `TestRegisterNilRunner` | `internal/plugin/registry/registry_test.go` | Nil RunEngine panics | |
| `TestRegisterNilCLI` | `internal/plugin/registry/registry_test.go` | Nil CLIHandler panics | |
| `TestGet` | `internal/plugin/registry/registry_test.go` | Get returns registered plugin | |
| `TestGetUnknown` | `internal/plugin/registry/registry_test.go` | Get returns nil for unknown | |
| `TestAll` | `internal/plugin/registry/registry_test.go` | All returns sorted list | |
| `TestFamilyMap` | `internal/plugin/registry/registry_test.go` | FamilyMap returns correct mapping | |
| `TestCapabilityMap` | `internal/plugin/registry/registry_test.go` | CapabilityMap returns correct mapping | |
| `TestAllRegistered` | `internal/plugin/all/all_test.go` | All 9 plugins are registered after import | |
| `TestExistingAPIPreserved` | `internal/plugin/inprocess_test.go` | IsInternalPlugin, AvailableInternalPlugins, GetPluginForFamily still work | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Existing plugin tests | `test/plugin/*.ci` | All existing plugin tests pass unchanged | |
| Existing decode tests | `test/decode/*.ci` | All existing decode tests pass unchanged | |
| Existing parse tests | `test/parse/*.ci` | All existing parse tests pass unchanged | |
| `ze bgp plugin help` | manual verification | Help lists all 9 plugins with descriptions | |

### Future (if deferring any tests)
- Fuzz test for registration with malformed family strings - defer until needed

## Files to Modify

- `cmd/ze/bgp/plugin.go` - replace switch/case with registry lookup, auto-generate help
- `cmd/ze/bgp/decode.go` - replace hardcoded maps with registry queries
- `internal/plugin/inprocess.go` - replace hardcoded maps with registry delegation
- `internal/plugin/resolve.go` - replace internalPluginInfo with registry queries
- `.claude/rules/plugin-design.md` - update plugin development guide

## Files to Create

- `internal/plugin/registry/registry.go` - central registry package
- `internal/plugin/registry/registry_test.go` - registry unit tests
- `internal/plugin/cli/cli.go` - PluginConfig + RunPlugin (moved from cmd/ze/bgp/plugin_common.go)
- `internal/plugin/all/all.go` - blank imports for all plugins
- `internal/plugin/all/all_test.go` - verify all plugins registered
- `internal/plugin/gr/register.go` - GR plugin registration
- `internal/plugin/hostname/register.go` - Hostname plugin registration
- `internal/plugin/llnh/register.go` - LLNH plugin registration
- `internal/plugin/flowspec/register.go` - FlowSpec plugin registration
- `internal/plugin/evpn/register.go` - EVPN plugin registration
- `internal/plugin/vpn/register.go` - VPN plugin registration
- `internal/plugin/bgpls/register.go` - BGP-LS plugin registration
- `internal/plugin/rib/register.go` - RIB plugin registration
- `internal/plugin/rr/register.go` - RR plugin registration

## Files to Delete

- `cmd/ze/bgp/plugin_gr.go` - replaced by internal/plugin/gr/register.go
- `cmd/ze/bgp/plugin_hostname.go` - replaced by internal/plugin/hostname/register.go
- `cmd/ze/bgp/plugin_llnh.go` - replaced by internal/plugin/llnh/register.go
- `cmd/ze/bgp/plugin_flowspec.go` - replaced by internal/plugin/flowspec/register.go
- `cmd/ze/bgp/plugin_evpn.go` - replaced by internal/plugin/evpn/register.go
- `cmd/ze/bgp/plugin_vpn.go` - replaced by internal/plugin/vpn/register.go
- `cmd/ze/bgp/plugin_bgpls.go` - replaced by internal/plugin/bgpls/register.go
- `cmd/ze/bgp/plugin_rib.go` - replaced by internal/plugin/rib/register.go
- `cmd/ze/bgp/plugin_rr.go` - replaced by internal/plugin/rr/register.go
- `cmd/ze/bgp/plugin_common.go` - moved to internal/plugin/cli/cli.go

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Create registry package** - `internal/plugin/registry/registry.go` with Registration struct and Register/Get/All/FamilyMap/CapabilityMap functions
   -> **Review:** Is the API minimal? Does it avoid import cycles? Thread safety ok?

2. **Write registry tests** - `internal/plugin/registry/registry_test.go`
   -> **Review:** Do tests cover registration, lookup, duplicate detection, validation?

3. **Run tests** - Verify FAIL (paste output)
   -> **Review:** Do tests fail for the RIGHT reason?

4. **Implement registry** - Minimal code to pass tests
   -> **Review:** Is this the simplest solution?

5. **Create CLI package** - Move PluginConfig + RunPlugin to `internal/plugin/cli/cli.go`
   -> **Review:** Does this break any existing imports? Import cycle free?

6. **Create register.go for each plugin** - One per plugin (9 files), each with init() calling registry.Register()
   -> **Review:** Does each register.go contain ALL metadata currently spread across 12 locations?

7. **Create all.go** - `internal/plugin/all/all.go` with blank imports
   -> **Review:** Are all 9 plugins imported?

8. **Modify cmd/ze/bgp/plugin.go** - Replace switch/case with registry lookup
   -> **Review:** Does `plugin test` still work? Does `plugin help` auto-generate correctly?

9. **Modify cmd/ze/bgp/decode.go** - Replace hardcoded maps with registry queries
   -> **Review:** Do all decode paths still work?

10. **Modify internal/plugin/inprocess.go** - Delegate to registry
    -> **Review:** Do all public API functions still return the same values?

11. **Modify internal/plugin/resolve.go** - Delegate to registry
    -> **Review:** Does plugin resolution still work?

12. **Delete old files** - Remove plugin_<name>.go wrappers and plugin_common.go from cmd/ze/bgp/
    -> **Review:** No orphaned references?

13. **Update plugin-design.md** - New plugin checklist, registration pattern docs
    -> **Review:** Would a new developer following this guide create a correct plugin?

14. **Run all tests** - `make lint && make test && make functional`
    -> **Review:** Zero regressions?

15. **Final self-review** - Re-read all changes for bugs, edge cases, unused code

## Implementation Summary

### What Was Implemented
- Central registry package (`internal/plugin/registry/`) with `Registration` struct and full query API
- Shared CLI package (`internal/plugin/cli/`) with `PluginConfig`, `RunPlugin()`, and `BaseConfig()` helper
- Per-plugin `register.go` files (9 plugins) using `init()` + `registry.Register()` pattern
- Auto-import file (`internal/plugin/all/all.go`) generated by `scripts/gen-plugin-imports.go`
- CLI dispatch via `registry.Lookup()` replacing hardcoded switch/case
- Decode maps via `registry.FamilyMap()`, `registry.CapabilityMap()`, `registry.InProcessDecoders()`
- Engine-side maps delegated to registry via `inprocess.go` and `resolve.go`
- `plugin-design.md` rewritten with new registration pattern and checklist
- `Makefile` updated with `generate` target

### Bugs Found/Fixed
- Duplicate `_ "internal/plugin/all"` import in `cmd/ze/bgp/plugin.go` (redundant with transitive import via `encode.go` → `internal/plugin` → `all`) — removed
- Arrow inconsistency in `inprocess.go` doc comments (`->` vs `→`) — fixed to `→`

### Design Insights
- `cli.BaseConfig(&reg)` eliminates field duplication between `Registration` and `PluginConfig` — 5 common fields copied automatically, plugin-specific handlers set by caller
- Closures in `register.go` capture `&reg` (local variable); `Register(reg)` copies struct but CLIHandler closure still references local — both have identical data, neither mutated after registration
- `registry` is a true leaf package (zero project imports) preventing import cycles
- Go `init()` functions run sequentially — no mutex needed for write-once registry
- `Register()` returns `error` instead of panicking — plugins handle with `os.Exit(1)` in init

### Deviations from Plan
- `Register()` returns `error` instead of panicking on validation failures (more testable, less surprising)
- Added `scripts/gen-plugin-imports.go` generator with `//go:generate` directive (not in original spec)
- Added `Makefile` `generate` target (not in original spec)
- Added `BaseConfig()` helper function to `cli` package (post-implementation code review suggestion)
- Spec planned `TestGet` and `TestGetUnknown` but implementation uses `TestLookupUnknown` and `TestHas` (same coverage, better naming match to API)
- Spec planned `TestRegisterNilRunner` but implementation names it `TestRegisterNilRunEngine` (matches field name)

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Replace hardcoded switch/case dispatch | ✅ Done | `cmd/ze/bgp/plugin.go:27-33` | `registry.Lookup(args[0])` replaces switch |
| Replace hardcoded help text | ✅ Done | `cmd/ze/bgp/plugin.go:41` | `registry.WriteUsage(os.Stderr)` auto-generates |
| Replace hardcoded engine runner map | ✅ Done | `internal/plugin/inprocess.go:72-83` | `GetInternalPluginRunner()` reads registry |
| Replace hardcoded YANG map | ✅ Done | `internal/plugin/inprocess.go:28-34` | `GetInternalPluginYANG()` reads `reg.YANG` |
| Replace hardcoded config roots map | ✅ Done | `internal/plugin/inprocess.go:18-24` | `GetInternalPluginWantsConfig()` reads `reg.ConfigRoots` |
| Replace hardcoded family-to-plugin map | ✅ Done | `internal/plugin/inprocess.go:87-89` | `registry.PluginForFamily()` |
| Replace hardcoded CLI family map | ✅ Done | `cmd/ze/bgp/decode.go` | `registry.FamilyMap()` replaces `pluginFamilyMap` |
| Replace hardcoded CLI capability map | ✅ Done | `cmd/ze/bgp/decode.go` | `registry.CapabilityMap()` replaces `pluginCapabilityMap` |
| Replace hardcoded in-process decoders | ✅ Done | `cmd/ze/bgp/decode.go` | `registry.InProcessDecoders()` replaces `inProcessDecoders` |
| Replace hardcoded plugin metadata | ✅ Done | `internal/plugin/resolve.go` | `registry.Lookup()` replaces `internalPluginInfo` |
| Create central registry | ✅ Done | `internal/plugin/registry/registry.go` (243 lines) | Leaf package, zero project imports |
| Create per-plugin register.go | ✅ Done | `internal/plugin/{gr,hostname,llnh,flowspec,evpn,vpn,bgpls,rib,rr}/register.go` | 9 files |
| Create single imports file | ✅ Done | `internal/plugin/all/all.go` (20 lines) | Generated by `scripts/gen-plugin-imports.go` |
| Update plugin-design.md | ✅ Done | `.claude/rules/plugin-design.md` | Rewritten with new pattern |
| Adding a plugin = folder + one import line | ✅ Done | Pattern verified | register.go + blank import in all.go |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestRegister | ✅ Done | `registry/registry_test.go:33` | |
| TestRegisterDuplicate | ✅ Done | `registry/registry_test.go:57` | |
| TestRegisterEmptyName | ✅ Done | `registry/registry_test.go:74` | |
| TestRegisterNilRunner | ✅ Done | `registry/registry_test.go:88` | Named `TestRegisterNilRunEngine` |
| TestRegisterNilCLI | ✅ Done | `registry/registry_test.go:103` | Named `TestRegisterNilCLIHandler` |
| TestGet | ✅ Done | `registry/registry_test.go:148,160` | Split: `TestLookupUnknown` + `TestHas` |
| TestGetUnknown | ✅ Done | `registry/registry_test.go:148` | Named `TestLookupUnknown` |
| TestAll | ✅ Done | `registry/registry_test.go:179` | |
| TestFamilyMap | ✅ Done | `registry/registry_test.go:226` | |
| TestCapabilityMap | ✅ Done | `registry/registry_test.go:260` | |
| TestAllRegistered | ✅ Done | `all/all_test.go:15` | Named `TestAllPluginsRegistered` |
| TestExistingAPIPreserved | 🔄 Changed | `all/all_test.go:46-117` | Split into 5 focused tests instead of 1 monolith |
| Existing functional tests pass | ✅ Done | `make functional` | All 80 tests pass |

### Additional Tests (beyond spec)
| Test | Location | Notes |
|------|----------|-------|
| TestRegisterInvalidFamily | `registry/registry_test.go:118` | Family format validation |
| TestRegisterValidFamily | `registry/registry_test.go:133` | Valid family accepted |
| TestNames | `registry/registry_test.go:204` | Sorted name list |
| TestInProcessDecoders | `registry/registry_test.go:288` | Decoder map query |
| TestYANGSchemas | `registry/registry_test.go:315` | Schema collection |
| TestConfigRootsMap | `registry/registry_test.go:342` | Config roots query |
| TestPluginForFamily | `registry/registry_test.go:369` | Family lookup |
| TestRequiredPlugins | `registry/registry_test.go:390` | Dedup required plugins |
| TestWriteUsage | `registry/registry_test.go:409` | Help text generation |
| TestWriteUsageEmpty | `registry/registry_test.go:451` | Empty registry help |
| TestReset | `registry/registry_test.go:467` | Test isolation |
| TestAllPluginsHaveRunEngine | `all/all_test.go:46` | Integration check |
| TestAllPluginsHaveCLIHandler | `all/all_test.go:58` | Integration check |
| TestAllPluginsHaveDescription | `all/all_test.go:70` | Integration check |
| TestFamilyMappings | `all/all_test.go:82` | Integration check |
| TestCapabilityMappings | `all/all_test.go:108` | Integration check |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/registry/registry.go` | ✅ Created | 243 lines |
| `internal/plugin/registry/registry_test.go` | ✅ Created | 479 lines, 21 tests |
| `internal/plugin/cli/cli.go` | ✅ Created | 263 lines (moved from cmd/ + BaseConfig) |
| `internal/plugin/all/all.go` | ✅ Created | 20 lines (generated) |
| `internal/plugin/all/all_test.go` | ✅ Created | 117 lines, 6 integration tests |
| 9x `internal/plugin/<name>/register.go` | ✅ Created | gr, hostname, llnh, flowspec, evpn, vpn, bgpls, rib, rr |
| `cmd/ze/bgp/plugin.go` (modified) | ✅ Modified | Switch → registry.Lookup dispatch |
| `cmd/ze/bgp/decode.go` (modified) | ✅ Modified | 3 maps → registry queries |
| `internal/plugin/inprocess.go` (modified) | ✅ Modified | All maps → registry delegation |
| `internal/plugin/resolve.go` (modified) | ✅ Modified | Plugin info → registry delegation |
| `.claude/rules/plugin-design.md` (modified) | ✅ Modified | Full rewrite for registry pattern |
| 9x `cmd/ze/bgp/plugin_<name>.go` (deleted) | ✅ Deleted | gr, hostname, llnh, flowspec, evpn, vpn, bgpls, rib, rr |
| `cmd/ze/bgp/plugin_common.go` (deleted) | ✅ Deleted | Moved to internal/plugin/cli/cli.go |

### Additional Files (beyond spec)
| File | Status | Notes |
|------|----------|-------|
| `scripts/gen-plugin-imports.go` | ✅ Created | Auto-generates all.go |
| `internal/plugin/all/gen.go` | ✅ Created | go:generate directive |
| `Makefile` | ✅ Modified | Added `generate` target |

### Audit Summary
- **Total items:** 41 (15 requirements + 13 planned tests + 13 planned files)
- **Done:** 40
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 1 (TestExistingAPIPreserved split into 5 focused tests — better coverage)

## Checklist

### Design
- [x] No premature abstraction (replacing 12 scattered locations with 1 registry - clear win)
- [x] No speculative features (only what's needed to replace current hardcoding)
- [x] Single responsibility (registry stores, plugins register, CLI dispatches)
- [x] Explicit behavior (init() is Go's standard compile-time wiring mechanism)
- [x] Minimal coupling (plugins know only about registry interface)
- [x] Next-developer test (add plugin = create register.go + add import line)

### TDD
- [x] Tests written
- [x] Tests FAIL (verified during implementation)
- [x] Implementation complete
- [x] Tests PASS (27 registry + integration tests pass)
- [x] Feature code integrated into codebase
- [x] Functional tests verify end-user behavior (80 functional tests pass)

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation (during implementation)
- [x] Required docs read
- [x] plugin-design.md updated with new pattern
- [x] MEMORY.md updated with registry pattern note

### Completion
- [x] Architecture docs updated with learnings (plugin-design.md rewritten)
- [x] Implementation Audit completed
- [x] Spec updated with Implementation Summary
- [x] Spec moved to `docs/plan/done/218-plugin-auto-registration.md`
- [ ] All files committed together
