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

## Proximity Principle (BLOCKING)

**Related code belongs together.** The "delete the folder" test is a mechanical check for proximity.

| Rule | Meaning |
|------|---------|
| All code for a concern in its folder | Commands, handlers, registration, logic — not scattered across packages |
| No external references to internals | Infrastructure, reactor, other units never import a specific plugin/command module |
| Blank import is the only coupling | A single `_ "package"` triggers init(); removing it cleanly disables the unit |
| Engine core works without any command module | Reactor, FSM, wire layer must function without CLI command handlers |

## YANG Is Required (BLOCKING)

**All RPCs need YANG registration for the CLI.** Any command handler without a YANG schema is a structural issue to fix, not a different category. There is no "command module" — everything with RPCs is a plugin and lives under `plugins/<name>/`.

| Registration | YANG | Location |
|-------------|------|----------|
| `registry.Register()` (SDK) | Required | `plugins/<name>/` |
| `pluginserver.RegisterRPCs()` (engine-side) | Required | `plugins/<name>/` |

**Anti-pattern:** Placing command handlers in reactor/ (couples engine core to command surface), in a separate handler/ package (middleman), or in a `command/` folder (formalizes missing YANG as acceptable). Commands belong in `plugins/` with YANG schemas.

## Import Rules (BLOCKING)

Infrastructure MUST NOT import plugin implementations directly — use registry lookups.

- `internal/plugin/`, `internal/plugins/bgp/`, `internal/component/config/`, `cmd/ze/` → registry
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

After Stage 5: SDK wraps Socket A in `MuxConn` for concurrent RPCs. Engine dispatches Socket A requests in goroutines. Wire format: `#<id> <verb> [<json>]\n` (see `docs/architecture/api/wire-format.md`).

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
| `Dependencies` | []string | No | Plugin names that must also be loaded (auto-expanded) |
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
| Fork (default) | `pluginname` | Subprocess via exec, TLS connect-back |
| Internal | `ze.pluginname` | Goroutine + net.Pipe + DirectBridge (hot path bypasses pipes) |
| Direct | `ze-pluginname` | Sync in-process call |
| Path | `/path/to/binary` | External binary, TLS connect-back |

## Transport

| Plugin type | Transport | Auth | Config |
|-------------|-----------|------|--------|
| Internal (goroutine) | `net.Pipe()` then DirectBridge | N/A | implicit |
| External (local) | TLS over TCP (single connection) | Token via `ZE_PLUGIN_HUB_TOKEN` env | `plugin { hub { listen ...; secret ...; } }` |
| External (remote) | TLS over TCP (single connection) | Token via out-of-band config | `plugin { hub { listen ...; secret ...; } }` |

External plugins connect back to the engine's TLS listener. Auth is stage 0: `#0 auth {"token":"...","name":"..."}`. After auth, the standard 5-stage handshake proceeds over the same single MuxConn connection.
