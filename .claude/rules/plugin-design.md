# Plugin Design Patterns

**BLOCKING:** All plugins MUST follow these patterns.
Rationale: `.claude/rationale/plugin-design.md`

## Architecture

| Layer | Location | Purpose |
|-------|----------|---------|
| Registry | `internal/plugin/registry/` | Central registry (leaf package, no plugin deps) |
| Public SDK | `pkg/plugin/sdk/` | Callback abstraction for external plugins |
| RPC Types | `pkg/plugin/rpc/` | Shared YANG RPC type definitions |
| Internal | `internal/plugins/<name>/` | Plugin implementations + `register.go` |
| All imports | `internal/plugin/all/` | Blank imports triggering all `init()` |
| CLI shared | `internal/plugin/cli/` | `PluginConfig` + `RunPlugin()` |

## Import Rules (BLOCKING)

Infrastructure code MUST NOT import plugin implementations directly. Use registry lookups.

- `internal/plugin/`, `internal/plugins/bgp/`, `internal/config/`, `cmd/ze/` → must use registry
- NLRI decoding: `registry.NLRIDecoder(family)` → `func(hex) (json, error)`
- NLRI encoding: `registry.NLRIEncoder(family)` → `func(args) (hex, error)`
- Plugin `register.go` and `all/all.go` blank imports: allowed
- Schema imports (`<plugin>/schema/`): allowed (data, not logic)
- Test imports: tolerated

## 5-Stage Protocol

```
Stage 1: Declaration    → Plugin sends: ze-plugin-engine:declare-registration (Socket A)
Stage 2: Config         → Engine sends: ze-plugin-callback:configure (Socket B)
Stage 3: Capability     → Plugin sends: ze-plugin-engine:declare-capabilities (Socket A)
Stage 4: Registry       → Engine sends: ze-plugin-callback:share-registry (Socket B)
Stage 5: Ready          → Plugin sends: ze-plugin-engine:ready (Socket A), enter event loop
```

Socket A = plugin→engine. Socket B = engine→plugin.

## Registration Fields

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `Name` | string | Yes | Plugin name |
| `Description` | string | Yes | Human-readable description |
| `RunEngine` | func(Conn, Conn) int | Yes | Engine-mode entry point |
| `CLIHandler` | func([]string) int | Yes | CLI dispatch handler |
| `Families` | []string | No | Address families (format "afi/safi") |
| `CapabilityCodes` | []uint8 | No | Capability codes decoded |
| `ConfigRoots` | []string | No | Config roots this plugin wants |
| `YANG` | string | No | YANG schema content |
| `InProcessNLRIDecoder` | func(family, hex) (string, error) | No | NLRI decode |
| `InProcessNLRIEncoder` | func(family, args) (string, error) | No | NLRI encode |
| `Features` | string | No | Space-separated flags ("nlri yang capa") |

## New Plugin Checklist

```
[ ] Create internal/plugins/<name>/<name>.go (package-level logger with SetLogger)
[ ] Create internal/plugins/<name>/register.go (init() → registry.Register())
[ ] Run make generate (regenerates all.go)
[ ] Update TestAllPluginsRegistered expected count
[ ] Add YANG schema if configuration support (schema/ subdir)
[ ] Add functional tests in test/plugin/
```

Auto-populated (no manual changes needed): CLI dispatch, plugin runners, YANG schemas, config roots, family/capability maps, decoder maps.

## Plugin Invocation Modes

| Mode | Syntax | Implementation |
|------|--------|----------------|
| Fork (default) | `pluginname` | Subprocess via exec |
| Internal | `ze.pluginname` | Goroutine + Unix socket pair |
| Direct | `ze-pluginname` | Sync in-process call |
| Path | `/path/to/binary` | External binary |
