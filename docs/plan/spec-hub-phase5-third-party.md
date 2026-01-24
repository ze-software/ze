# Spec: Hub Phase 5 - Third-Party Plugin Support

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/hub-architecture.md` - Hub design, third-party example
4. `internal/plugin/schema.go` - SchemaRegistry (Phase 1)
5. `internal/plugin/hub.go` - Hub orchestration (Phase 4)

## Task

Document and enable third-party plugin development. Create developer documentation, example plugins, and tooling to help external developers create plugins that extend ZeBGP's configuration schema.

### Goals

1. Plugin Developer Guide documentation
2. Example third-party plugin (complete working example)
3. Plugin SDK/library for common operations
4. Schema validation tooling (`ze plugin validate`)
5. Plugin testing utilities

### Non-Goals

- Plugin marketplace/repository
- Plugin signing/security (future)
- Automatic plugin discovery

### Dependencies

- Phase 1-4: All Hub infrastructure must be complete

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/hub-architecture.md` - [third-party example section]
- [ ] `docs/architecture/api/ipc_protocol.md` - [protocol specification]
- [ ] `docs/architecture/api/process-protocol.md` - [5-stage protocol]

### Source Files
- [ ] `cmd/ze-subsystem/main.go` - [reference implementation]
- [ ] `internal/plugin/schema.go` - [schema format]

**Key insights:**
- Third-party plugins follow same protocol as internal subsystems
- YANG schema enables type-safe configuration extension
- Plugins need clear error handling and logging patterns

## Design

### Plugin Developer Guide Structure

```
docs/plugin-development/
├── README.md           # Overview and quick start
├── protocol.md         # 5-stage protocol details
├── schema.md           # YANG schema authoring
├── handlers.md         # Verify/Apply handler patterns
├── commands.md         # Adding API commands
├── testing.md          # Testing your plugin
└── examples/           # Example plugins
    ├── go/             # Go plugin example
    ├── python/         # Python plugin example
    └── shell/          # Shell script plugin
```

### Plugin SDK (Go)

The Go SDK provides a high-level interface for writing plugins:

**Plugin lifecycle:**
1. `New(name)` - Create plugin with name
2. `SetSchema(yang, handlers...)` - Set YANG schema and handler paths
3. `OnVerify(prefix, handler)` - Register verify handler for path prefix
4. `OnApply(prefix, handler)` - Register apply handler for path prefix
5. `OnCommand(name, handler)` - Register command handler
6. `Run()` - Start the 5-stage protocol loop

**Handler contexts:**
| Context | Fields |
|---------|--------|
| VerifyContext | Action (create/modify/delete), Path, Old data, New data |
| ApplyContext | Action, Path, Old data, New data |
| CommandContext | Command name, Arguments |

### Example Plugin (Go)

A minimal third-party plugin:

1. **Declare YANG schema** with handlers for "acme-monitor" path
2. **Verify handler** checks endpoint starts with "https://"
3. **Apply handler** starts/updates monitoring based on config
4. **Command handler** returns monitor status

**YANG schema structure:**
```yang
module acme-monitor {
    namespace "urn:acme:monitor";
    prefix acme;

    container acme-monitor {
        leaf endpoint { type string; mandatory true; }
        leaf interval { type uint32 { range "10..3600"; } default 60; }
    }
}
```

### Plugin Validation Tool

```bash
# Validate plugin YANG schema
ze plugin validate schema ./my-plugin.yang

# Validate plugin binary follows protocol
ze plugin validate binary ./my-plugin

# Test plugin with sample config
ze plugin test ./my-plugin --config test.conf

# Generate plugin scaffold
ze plugin init my-plugin --lang go
ze plugin init my-plugin --lang python
```

### Python Plugin Example

Python plugins follow the same pattern using a Python SDK:

**Plugin pattern:**
1. Create plugin with name
2. Set YANG schema and handlers
3. Register verify/apply/command handlers with decorators
4. Run protocol loop

**Key differences from Go:**
- Decorators for handler registration (`@plugin.on_verify`)
- Exceptions for errors (`raise ValueError`)
- Same protocol, different language idiom

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPluginSDK_Protocol` | `pkg/plugin/plugin_test.go` | SDK follows 5-stage protocol | |
| `TestPluginSDK_SchemaDeclaration` | `pkg/plugin/plugin_test.go` | Schema declared correctly | |
| `TestPluginSDK_VerifyHandler` | `pkg/plugin/plugin_test.go` | Verify handlers called | |
| `TestPluginSDK_ApplyHandler` | `pkg/plugin/plugin_test.go` | Apply handlers called | |
| `TestPluginSDK_CommandHandler` | `pkg/plugin/plugin_test.go` | Commands registered and called | |
| `TestPluginValidate_Schema` | `cmd/ze/plugin/validate_test.go` | Schema validation works | |
| `TestPluginValidate_Binary` | `cmd/ze/plugin/validate_test.go` | Binary validation works | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Plugin name length | 1-64 | 64 chars | 0 | 65 chars |
| Handler prefix depth | 1-10 | 10 segments | 0 | 11 segments |
| Command name length | 1-128 | 128 chars | 0 | 129 chars |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `plugin-example-go` | `test/data/plugin/example-go.ci` | Go example plugin works | |
| `plugin-example-python` | `test/data/plugin/example-python.ci` | Python example plugin works | |
| `plugin-validate-schema` | `test/data/plugin/validate-schema.ci` | Schema validation CLI | |
| `plugin-third-party` | `test/data/plugin/third-party.ci` | Full third-party integration | |

## Files to Create

### Documentation
- `docs/plugin-development/README.md` - Overview
- `docs/plugin-development/protocol.md` - Protocol details
- `docs/plugin-development/schema.md` - YANG authoring
- `docs/plugin-development/handlers.md` - Handler patterns
- `docs/plugin-development/commands.md` - Command registration
- `docs/plugin-development/testing.md` - Testing guide

### SDK
- `pkg/plugin/plugin.go` - Go plugin SDK
- `pkg/plugin/plugin_test.go` - SDK tests
- `pkg/plugin/context.go` - Context types
- `pkg/plugin/protocol.go` - Protocol implementation

### Python SDK (optional, separate package)
- `pkg/python/ze_plugin/__init__.py` - Python SDK
- `pkg/python/ze_plugin/protocol.py` - Protocol implementation
- `pkg/python/setup.py` - Package setup

### Tools
- `cmd/ze/plugin/validate.go` - Validation commands
- `cmd/ze/plugin/init.go` - Scaffold generator
- `cmd/ze/plugin/test.go` - Plugin testing

### Examples
- `examples/plugin/go/main.go` - Go example
- `examples/plugin/go/go.mod` - Go module
- `examples/plugin/python/plugin.py` - Python example
- `examples/plugin/shell/plugin.sh` - Shell example

## Files to Modify

- `cmd/ze/main.go` - Add `ze plugin` subcommands
- `Makefile` - Add example build targets

## Implementation Steps

**Self-Critical Review:** After each step, review for issues and fix before proceeding.

1. **Write SDK tests** - Create plugin_test.go
2. **Run tests** - Verify FAIL (paste output)
3. **Implement Plugin SDK** - Create pkg/plugin/
4. **Run tests** - Verify PASS
5. **Create Go example** - Working example plugin
6. **Test Go example** - Manual integration test
7. **Write validation tests** - Create validate_test.go
8. **Implement validation CLI** - Create cmd/ze/plugin/
9. **Run tests** - Verify PASS (paste output)
10. **Write documentation** - All docs/plugin-development/ files
11. **Create Python example** - Optional but valuable
12. **Functional tests** - Create and run
13. **Verify all** - `make lint && make test && make functional` (paste output)

## Documentation Outline

### README.md
- What is a ZeBGP plugin?
- Quick start (5 minutes to hello world)
- Prerequisites
- Installation

### protocol.md
- 5-stage protocol overview
- Stage 1: Declaration messages
- Stage 2: Config (verify/apply)
- Stage 3-5: Capability, Registry, Ready
- Message format reference
- Error handling

### schema.md
- YANG basics for plugin authors
- Defining your schema
- Types and constraints
- leafref for references
- Best practices
- Testing your schema

### handlers.md
- Verify vs Apply
- Handler routing
- Context objects
- Error handling
- State management
- Example patterns

### commands.md
- Registering commands
- Command naming conventions
- Arguments and responses
- JSON vs text responses

### testing.md
- Unit testing handlers
- Integration testing with ZeBGP
- Using ze plugin validate
- CI/CD integration

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] Plugin Developer Guide complete
- [ ] Examples working
- [ ] Code comments added

### Completion
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-hub-phase5-third-party.md`
