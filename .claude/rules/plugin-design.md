# Plugin Design

**BLOCKING:** All plugins MUST follow these patterns.
Rationale: `.claude/rationale/plugin-design.md`

## Architecture

| Layer | Location | Purpose |
|-------|----------|---------|
| Registry | `internal/plugin/registry/` | Central registry (leaf package, no plugin deps) |
| Public SDK | `pkg/plugin/sdk/` | Callback abstraction for external plugins |
| RPC Types | `pkg/plugin/rpc/` | Shared YANG RPC types + `MuxConn` for concurrent RPCs |
| Internal | `internal/plugins/<name>/` | Plugin implementations + `register.go` |
| All imports | `internal/plugin/all/` | Blank imports triggering all `init()` |
| CLI shared | `internal/plugin/cli/` | `PluginConfig` + `RunPlugin()` |

## Import Rules (BLOCKING)

Infrastructure MUST NOT import plugin implementations directly — use registry lookups.

- `internal/plugin/`, `internal/plugins/bgp/`, `internal/config/`, `cmd/ze/` → registry
- NLRI decoding: `registry.NLRIDecoder(family)` → `func(hex) (json, error)`
- NLRI encoding: `registry.NLRIEncoder(family)` → `func(args) (hex, error)`
- Plugin `register.go` and `all/all.go` blank imports: allowed
- Schema imports (`<plugin>/schema/`): allowed (data, not logic)
- Test imports: tolerated

## 5-Stage Protocol

| Stage | Direction | RPC |
|-------|-----------|-----|
| 1. Declaration | Plugin→Engine (A) | `ze-plugin-engine:declare-registration` |
| 2. Config | Engine→Plugin (B) | `ze-plugin-callback:configure` |
| 3. Capability | Plugin→Engine (A) | `ze-plugin-engine:declare-capabilities` |
| 4. Registry | Engine→Plugin (B) | `ze-plugin-callback:share-registry` |
| 5. Ready | Plugin→Engine (A) | `ze-plugin-engine:ready`, enter event loop |

After Stage 5: SDK wraps Socket A in `MuxConn` for concurrent RPCs. Engine dispatches Socket A requests in goroutines.

## Registration Fields

| Field | Type | Required | Purpose |
|-------|------|----------|---------|
| `Name` | string | Yes | Plugin name |
| `Description` | string | Yes | Human-readable description |
| `RunEngine` | func(Conn, Conn) int | Yes | Engine-mode entry point |
| `CLIHandler` | func([]string) int | Yes | CLI dispatch handler |
| `Families` | []string | No | Address families ("afi/safi") |
| `CapabilityCodes` | []uint8 | No | Capability codes decoded |
| `ConfigRoots` | []string | No | Config roots plugin wants |
| `YANG` | string | No | YANG schema content |
| `InProcessNLRIDecoder` | func | No | NLRI decode |
| `InProcessNLRIEncoder` | func | No | NLRI encode |
| `Features` | string | No | Space-separated flags ("nlri yang capa") |

## New Plugin Checklist

```
[ ] Create internal/plugins/<name>/<name>.go (package-level logger with SetLogger)
[ ] Create internal/plugins/<name>/register.go (init() → registry.Register())
[ ] Run make generate (regenerates all.go)
[ ] Update TestAllPluginsRegistered expected count
[ ] Add YANG schema if config support (schema/ subdir)
[ ] Add functional tests in test/plugin/
```

Auto-populated: CLI dispatch, plugin runners, YANG schemas, config roots, family/capability maps, decoder maps.

## Invocation Modes

| Mode | Syntax | Implementation |
|------|--------|----------------|
| Fork (default) | `pluginname` | Subprocess via exec |
| Internal | `ze.pluginname` | Goroutine + socket pair + DirectBridge (hot path bypasses sockets) |
| Direct | `ze-pluginname` | Sync in-process call |
| Path | `/path/to/binary` | External binary |
