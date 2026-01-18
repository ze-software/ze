# Spec: graceful-restart-plugin

## Task
Create a standalone GR plugin separate from RIB that:
- Receives per-peer GR config (restart-time) during Stage 2
- Registers GR capabilities per-peer during Stage 3
- Is independent of RIB plugin

**Status:** ✅ COMPLETE - Phase 1 (plugin code) and Phase 2 (config flow) implemented

## Required Reading

### Architecture Docs
- [x] `docs/architecture/core-design.md` - Plugin architecture overview
- [x] `docs/architecture/api/capability-contract.md` - Capability registration protocol

### RFC Summaries
- [x] `docs/rfc/rfc4724.md` - Graceful Restart mechanism

**Key insights:**
- Plugins declare config patterns in Stage 1, receive config in Stage 2, register capabilities in Stage 3
- GR capability is code 64, wire format: [R:1][Reserved:3][RestartTime:12] = 2 bytes

## Design

### Phase 1: Plugin Code (DONE)

GR plugin implementation exists but cannot receive config until Phase 2 is complete.

### Phase 2: Plugin-Driven Config Parsing (PENDING)

**Requirement:** ZeBGP engine must have NO hardcoded knowledge of GR. The plugin defines the config schema.

#### Plugin Syntax Change

**Step 0:** Change plugin syntax from multiple top-level blocks to single container with `external` keyword:

**Old syntax (deprecated):**
```
plugin gr {
    run "zebgp plugin gr";
}
plugin rib {
    run "zebgp plugin rib";
}
```

**New syntax:**
```
plugin {
    external gr {
        run "zebgp plugin gr";
        encoder json;
    }
    external rib {
        run "zebgp plugin rib";
    }
}
```

This mirrors `template { group <name> {} }` syntax and enables:
- Cleaner two-phase parsing (parse entire `plugin {}` block first)
- Future plugin types (`builtin`, `wasm`) alongside `external`

#### Config File Ordering

```
# Plugin block MUST come first in config
plugin {
    external gr {
        run "zebgp plugin gr";
        encoder json;
    }
}

# Peers come after plugin block
peer 127.0.0.1 {
    capability {
        graceful-restart {      # ← Plugin defines this schema
            restart-time 120;
        }
    }
}
```

#### Startup Sequence

```
CONFIG FILE                     ENGINE                          PLUGINS
───────────                     ──────                          ───────

plugin {                 →      1. Parse plugin {} block ONLY
  external gr { ... }           2. Start plugin processes  →    Started
  external rib { ... }          3. Wait for schema hooks   ←    declare conf schema capability
}                                                                 graceful-restart { restart-time <\d+>; }
                                                           ←    declare done
                                ─── CONFIG PARSING BARRIER ───
peer 127.0.0.1 {         →      4. Parse rest of config
  capability {                     (using plugin-extended schema)
    graceful-restart {
      restart-time 120;         5. Deliver matching config →    config peer 127.0.0.1
    }                                                             graceful-restart restart-time 120
  }
}
                                6. Continue normal stages  →    capability hex 64 0078 peer 127.0.0.1
                                                           ←    capability done
                                                           ←    ready
```

#### Engine Changes Required

| Component | Change |
|-----------|--------|
| `pkg/config/loader.go` | Two-phase parsing: plugins first, then rest |
| `pkg/config/schema.go` | Dynamic schema extension from plugin declarations |
| `pkg/plugin/server.go` | New "schema declaration" stage before config parsing |
| `pkg/plugin/registration.go` | Parse `declare conf schema ...` commands |

#### Plugin Schema Declaration

```
# GR plugin declares its config schema in Stage 1:
declare conf schema capability graceful-restart { restart-time <restart-time:\d+>; }
declare done
```

This tells the engine:
- Under `capability { }` block, allow `graceful-restart { restart-time <int>; }`
- Capture `restart-time` value for delivery to this plugin

#### Config Delivery Format

```
# Engine delivers to plugin:
config peer 192.168.1.1 graceful-restart restart-time 120
config peer 10.0.0.1 graceful-restart restart-time 90
config done
```

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              CONFIG FILE                                     │
│                                                                             │
│   plugin gr { run "zebgp plugin gr"; }     ◄── Parsed first                 │
│                                                                             │
│   peer 127.0.0.1 {                                                          │
│       capability {                                                          │
│           graceful-restart { restart-time 120; }  ◄── Schema from plugin    │
│       }                                                                     │
│   }                                                                         │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
┌───────────────────────────────┐   ┌───────────────────────────────────────┐
│         GR PLUGIN             │   │              ENGINE                    │
│                               │   │                                       │
│ 1. declare conf schema ...    │──▶│ Extends config parser                 │
│                               │   │                                       │
│ 2. receives:                  │◀──│ Delivers matching config              │
│    config peer X gr rt 120    │   │                                       │
│                               │   │                                       │
│ 3. sends:                     │──▶│ Injects into OPEN                     │
│    capability hex 64 0078     │   │                                       │
│    peer X                     │   │                                       │
└───────────────────────────────┘   └───────────────────────────────────────┘
                                                    │
                                                    ▼
                                    ┌───────────────────────────────────────┐
                                    │            OPEN MESSAGE                │
                                    │  Capability 64: [0x00][0x78]          │
                                    │  (restart-time=120)                   │
                                    └───────────────────────────────────────┘
```

### Key Design Principles

1. **No GR code in engine**: ZeBGP has zero knowledge of RFC 4724
2. **Plugin defines schema**: `graceful-restart { restart-time X; }` syntax comes from plugin
3. **Generic delivery**: Engine delivers raw config values, plugin interprets them
4. **Polyglot plugins**: Any language can implement GR plugin (Go, Python, Rust, etc.)

### Phase 2 Implementation Details

#### Step 1: Plugin-Only Schema (`pkg/config/loader.go`)

Create a minimal schema that only parses `plugin` blocks:

```go
// PluginOnlySchema returns a schema for parsing just plugin blocks.
func PluginOnlySchema() *Schema {
    schema := NewSchema()
    schema.Define("plugin", List(TypeString,
        Field("run", MultiLeaf(TypeString)),
        Field("encoder", Leaf(TypeString)),
        Field("respawn", Leaf(TypeBool)),
        Field("timeout", Leaf(TypeString)),
    ))
    return schema
}

// LoadPluginsOnly parses only plugin blocks from config.
func LoadPluginsOnly(input string) ([]PluginConfig, error) {
    p := NewParser(PluginOnlySchema())
    tree, err := p.Parse(input)
    if err != nil {
        return nil, err
    }
    return extractPluginConfigs(tree), nil
}
```

#### Step 2: Dynamic Schema Extension (`pkg/config/schema.go`)

Add method to extend capability schema at runtime:

```go
// SchemaExtension represents a plugin-declared schema addition.
type SchemaExtension struct {
    Path   string     // "capability.graceful-restart"
    Fields []FieldDef // Fields for the new block
}

// ExtendCapability adds a capability sub-block to the schema.
func (s *Schema) ExtendCapability(name string, fields ...FieldDef) error {
    capNode, err := s.Lookup("peer.capability")
    if err != nil {
        return fmt.Errorf("capability node not found: %w", err)
    }
    if container, ok := capNode.(*ContainerNode); ok {
        container.children[name] = Flex(fields...)
        container.order = append(container.order, name)
    }
    return nil
}
```

#### Step 3: Schema Declaration Parsing (`pkg/plugin/registration.go`)

Parse `declare conf schema` commands from plugins:

```go
// SchemaDeclaration represents a plugin's config schema extension.
type SchemaDeclaration struct {
    Path     string            // "capability.graceful-restart"
    Pattern  string            // Pattern with captures
    Fields   map[string]string // field name -> type (uint16, string, etc.)
}

// parseConfSchema handles "declare conf schema <path> { <pattern> }".
func (reg *PluginRegistration) parseConfSchema(args []string, line string) error {
    // Parse: declare conf schema capability graceful-restart { restart-time <restart-time:\d+>; }
    // Extract path (capability.graceful-restart) and field definitions
    // ...
    reg.SchemaDeclarations = append(reg.SchemaDeclarations, decl)
    return nil
}
```

#### Step 4: Two-Phase Parsing Flow (`pkg/config/loader.go`)

```go
// LoadReactorWithPluginSchema implements two-phase parsing:
// 1. Parse plugin blocks only
// 2. Start plugins, collect schema declarations
// 3. Parse full config with extended schema
func LoadReactorWithPluginSchema(input string, schemaExtensions []SchemaExtension) (*BGPConfig, *reactor.Reactor, error) {
    // Build schema with plugin extensions
    schema := BGPSchema()
    for _, ext := range schemaExtensions {
        if err := schema.ExtendCapability(ext.Name, ext.Fields...); err != nil {
            return nil, nil, fmt.Errorf("extend schema: %w", err)
        }
    }

    // Parse with extended schema
    p := NewParser(schema)
    tree, err := p.Parse(input)
    // ... rest of parsing
}
```

#### Step 5: Coordinator Integration (`pkg/plugin/server.go`)

Add schema collection stage before config delivery:

```go
// Stage 0.5: Collect schema declarations from all plugins
// This happens BEFORE config parsing continues
func (s *Server) collectSchemaDeclarations() []SchemaExtension {
    var extensions []SchemaExtension
    for _, proc := range s.procManager.processes {
        reg := proc.Registration()
        for _, decl := range reg.SchemaDeclarations {
            extensions = append(extensions, SchemaExtension{
                Path:   decl.Path,
                Fields: declToFields(decl.Fields),
            })
        }
    }
    return extensions
}
```

#### Step 6: Remove Hardcoded GR (`pkg/config/bgp.go`)

Remove `graceful-restart` from the hardcoded schema:

```go
// capability block - plugins add their own capabilities
Field("capability", Container(
    Field("asn4", LeafWithDefault(TypeBool, configTrue)),
    Field("route-refresh", Flex()),
    // graceful-restart REMOVED - added by GR plugin
    Field("add-path", Flex(...)),
    // ...
)),
```

#### Step 7: Update GR Plugin (`pkg/plugin/gr/gr.go`)

Update declaration to use schema syntax:

```go
func (g *GRPlugin) doStartupProtocol() {
    // Stage 1: Declare schema (NEW)
    g.send("declare conf schema capability graceful-restart { restart-time <restart-time:\\d+>; }")
    g.send("declare done")

    // Stage 2: Parse config (unchanged)
    g.parseConfig()
    // ...
}
```

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestGRPlugin_ParseConfig` | `pkg/plugin/gr/gr_test.go` | Config line parsing | ✅ |
| `TestGRPlugin_CapabilityWireFormat` | `pkg/plugin/gr/gr_test.go` | RFC 4724 wire encoding | ✅ |
| `TestPluginOnlySchema` | `pkg/config/loader_test.go` | Only parses plugin blocks | |
| `TestSchemaExtendCapability` | `pkg/config/schema_test.go` | Dynamic schema extension | |
| `TestParseSchemaDeclaration` | `pkg/plugin/registration_test.go` | Parse `declare conf schema` | |
| `TestTwoPhaseConfigParsing` | `pkg/config/loader_test.go` | Full two-phase flow | |
| `TestGRSchemaDeclaration` | `pkg/plugin/gr/gr_test.go` | GR plugin declares schema | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| restart-time | 0-4095 | 4095 | N/A | 4096 (masked to 12 bits) |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `graceful-restart` | `test/data/plugin/graceful-restart.ci` | GR capability in OPEN | |

## Files to Create
| File | Purpose |
|------|---------|
| `pkg/plugin/gr/gr.go` | GR plugin implementation |
| `cmd/zebgp/plugin_gr.go` | CLI command `zebgp plugin gr` |

## Files to Modify
| File | Change |
|------|--------|
| `pkg/plugin/rib/rib.go` | Remove GR config/capability code |
| `cmd/zebgp/plugin.go` | Add `gr` case to dispatch + update usage |
| `test/data/plugin/graceful-restart.conf` | Use GR plugin instead of RIB |

## Implementation Steps

### 1. GR Plugin (`pkg/plugin/gr/gr.go`)

```go
type GRPlugin struct {
    input    *bufio.Scanner
    output   io.Writer
    grConfig map[string]uint16 // peerAddr → restart-time
    mu       sync.Mutex
    serial   int
}

func (g *GRPlugin) Run() int {
    g.doStartupProtocol()
    g.eventLoop()  // Minimal - just handle shutdown
    return 0
}

func (g *GRPlugin) doStartupProtocol() {
    // Stage 1: Declaration
    g.send("declare conf peer * capability rfc4724:restart-time <restart-time:\\d+>")
    g.send("declare done")

    // Stage 2: Parse config
    g.parseConfig()

    // Stage 3: Register capabilities per-peer
    g.registerCapabilities()

    // Stage 4: Wait for registry
    g.waitForLine("registry done")

    // Stage 5: Ready
    g.send("ready")
}

func (g *GRPlugin) parseConfig() {
    for g.input.Scan() {
        line := g.input.Text()
        if line == "config done" {
            return
        }
        // Parse: "config peer 192.168.1.1 rfc4724:restart-time 120"
        g.parseConfigLine(line)
    }
}

func (g *GRPlugin) registerCapabilities() {
    const grCapCode = 64
    for peerAddr, restartTime := range g.grConfig {
        // RFC 4724: [R bit:1][Reserved:3][Restart Time:12]
        capValue := fmt.Sprintf("%04x", restartTime&0x0FFF)
        g.send("capability hex %d %s peer %s", grCapCode, capValue, peerAddr)
        slog.Debug("gr: registered capability", "peer", peerAddr, "restart-time", restartTime)
    }
    g.send("capability done")
}
```

### 2. CLI Command (`cmd/zebgp/plugin_gr.go`)

```go
package main

import (
    "os"
    "codeberg.org/thomas-mangin/zebgp/pkg/plugin/gr"
)

func cmdPluginGR(_ []string) int {
    plugin := gr.NewGRPlugin(os.Stdin, os.Stdout)
    return plugin.Run()
}
```

### 3. Update plugin.go dispatch

```go
case "gr":
    return cmdPluginGR(args[1:])
```

And add to usage:
```
  gr           Run as Graceful Restart capability plugin
```

### 4. Remove GR from RIB Plugin

In `pkg/plugin/rib/rib.go`:
- Remove `grConfig map[string]uint16` field
- Remove `declare conf peer * capability rfc4724:restart-time ...` line
- Remove `registerCapabilities()` function
- Keep `waitForLine("config done")` (no config patterns to parse)
- Send `capability done` immediately in Stage 3

### 5. Update Test Config

`test/data/plugin/graceful-restart.conf`:
```
plugin gr {
    run "zebgp plugin gr";
    encoder json;
}

peer 127.0.0.1 {
    router-id 1.2.3.4;
    local-address 127.0.0.1;
    local-as 1;
    peer-as 1;
    group-updates disable;

    family {
        ipv4/unicast;
    }
    capability {
        graceful-restart {
            restart-time 120;
        }
    }

    static {
        route 192.168.1.0/24 next-hop 10.0.0.1;
    }

    process gr {
    }
}
```

## Verification

1. **Unit test**: `go test ./pkg/plugin/gr/...`
2. **Lint**: `make lint`
3. **Functional test**: `go run ./test/cmd/functional plugin 6`
4. **Manual test**:
   ```bash
   SLOG_LEVEL=DEBUG zebgp server test/data/plugin/graceful-restart.conf
   ```
   Check that GR capability appears in OPEN message.

## Key Design Decisions

1. **Separate plugin**: GR is capability injection, not route storage
2. **Stateless after startup**: No runtime state needed beyond config
3. **Minimal event loop**: Just handle shutdown signal
4. **RFC 4724 wire format**: `[R:1][Reserved:3][RestartTime:12]` = 2 bytes

## Implementation Summary

### Phase 1: What Was Implemented
- `pkg/plugin/gr/gr.go` - GR plugin with 5-stage startup protocol
- `pkg/plugin/gr/gr_test.go` - Unit tests for config parsing, wire format, startup
- `cmd/zebgp/plugin_gr.go` - CLI command wrapper
- `cmd/zebgp/plugin.go` - Added `gr` case to dispatch + usage string
- `pkg/plugin/rib/rib.go` - Removed GR config/capability code (grConfig field, registerCapabilities)
- `test/data/plugin/graceful-restart.conf` - Updated to use GR plugin

### Phase 2: Pending
- Engine changes for plugin-driven config parsing (see Phase 2 design above)
- Plugin cannot receive config until Phase 2 is complete

### Bugs Found/Fixed
- **Missing capability creation**: Config `graceful-restart { restart-time X; }` was parsed but no `GracefulRestart` capability was created, so `ConfigValues()` was never called and plugins received no config.
- **Fix approach**: Phase 2 removes all GR knowledge from engine; plugin defines schema.

### Design Insights
- **Implicit process binding already works**: `deliverConfig()` delivers config to ALL peers with matching capability, regardless of `process` bindings. Process bindings are only needed for runtime events.
- **Plugin-driven config is required**: Engine must not have hardcoded knowledge of capabilities like GR. Plugins must define their own config schema.
- **Two-phase config parsing**: Parse `plugin` blocks first, start plugins, let them extend schema, then parse rest of config.

### Deviations from Plan
- Removed `process gr {}` from test config - not needed for capability-only plugins
- Phase 2 (engine config hooks) discovered as necessary - plugin code exists but cannot work without it

## Checklist

### Phase 1: Plugin Code
- [x] Tests written
- [x] Tests FAIL (verified before implementation)
- [x] Implementation complete
- [x] Tests PASS
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [ ] `make functional` passes (blocked on Phase 2)

### Phase 2: Engine Config Hooks
- [ ] Two-phase config parsing (plugins first)
- [ ] `declare conf schema` command parsing
- [ ] Dynamic schema extension
- [ ] Generic config delivery to plugins
- [ ] Functional test passes end-to-end

### Documentation
- [x] Required docs read
- [x] RFC summaries read
- [x] RFC references added to code
- [x] Architecture docs updated with plugin-driven config design

### Completion
- [x] Spec updated with Implementation Summary
- [ ] Phase 2 implementation complete
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
