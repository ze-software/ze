# Spec: Hub Phase 1 - Schema Infrastructure

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/hub-architecture.md` - overall Hub design
4. `internal/plugin/subsystem.go` - existing SubsystemHandler
5. `cmd/ze-subsystem/main.go` - existing subsystem binary

## Task

Extend the 5-stage protocol to support YANG schema declarations. Plugins will declare their schemas in Stage 1, and the Hub will collect and store them in a SchemaRegistry.

### Goals

1. Add `declare schema` message parsing to Stage 1
2. Create SchemaRegistry to store collected schemas
3. Add `system schema *` CLI commands for discovery
4. Extend SubsystemHandler to track schemas per plugin

### Non-Goals

- YANG validation (Phase 3)
- Config Reader (Phase 2)
- Verify/Apply protocol (Phase 4)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/hub-architecture.md` - [Hub design, schema registration format]
- [ ] `docs/architecture/api/ipc_protocol.md` - [existing protocol format]
- [ ] `docs/architecture/api/process-protocol.md` - [5-stage protocol details]

### Source Files
- [ ] `internal/plugin/subsystem.go` - [SubsystemHandler, completeProtocol]
- [ ] `internal/plugin/registration.go` - [declaration parsing]
- [ ] `cmd/ze-subsystem/main.go` - [plugin-side protocol]

**Key insights:**
- SubsystemHandler.completeProtocol() already parses `declare cmd` messages
- Need to add parallel parsing for `declare schema` messages
- Schema text can be multi-line, need delimiter handling

## Design

### Schema Declaration Messages

Schema declarations use `declare schema` prefix (consistent with `declare cmd`):

**Plugin → Hub (Stage 1 stdout):**
```
declare schema module ze-bgp
declare schema namespace urn:ze:bgp
declare schema handler bgp
declare schema handler bgp.peer
declare schema handler bgp.peer-group
declare schema yang <<EOF
module ze-bgp {
  namespace "urn:ze:bgp";
  prefix bgp;
  ...
}
EOF
declare done
```

**Key points:**
- `declare schema` prefix matches `declare cmd` pattern
- `declare schema module <name>` - YANG module name
- `declare schema namespace <uri>` - YANG namespace
- `declare schema handler <path>` - config handler paths (supports longest-prefix routing)
- `declare schema yang <<EOF ... EOF` - inline YANG content (heredoc)
- Hub stores YANG content for distribution to Config Reader

**CLI debugging command:**
```bash
$ ze bgp schema show
module ze-bgp {
  namespace "urn:ze:bgp";
  ...
}
```

The same schema is available via CLI for human debugging.

### Schema CLI Commands

Schema commands for debugging and tooling:

```bash
# View schema (human debugging)
$ ze bgp schema show
module ze-bgp {
  namespace "urn:ze:bgp";
  prefix bgp;
  ...
}

# List handlers provided by schema
$ ze bgp schema handlers
bgp
bgp.peer
bgp.peer-group
```

**Key points:**
- Plain YANG text on stdout (easy to pipe/debug)
- Developers can run commands directly to troubleshoot
- Same content that Hub collected during Stage 1

### Operational Data Queries (leafref validation)

YANG leafrefs reference operational data. Schema commands validate references:

```bash
# Validate peer-group exists
$ ze bgp schema validate peer-group name upstream
$ echo $?
0  # valid

# Complete peer-group names
$ ze bgp schema complete peer-group name
upstream
downstream

# Validate interface exists
$ ze system schema validate interface name eth0
```

**Pattern:** `ze <subsystem> schema validate|complete <path> [value]`

Used by:
- **Config Reader**: Validate leafrefs during config parsing
- **CLI autocomplete**: Suggest valid values
- **Human debugging**: Check what objects exist

### SchemaRegistry

The SchemaRegistry stores schema information indexed by module name:

| Field | Description |
|-------|-------------|
| Module | YANG module name |
| Namespace | YANG namespace URI |
| Yang | Full YANG module text |
| Handlers | Handler paths (e.g., "bgp", "bgp.peer") |
| Plugin | Plugin that registered this schema |

**Registry operations:**
- `Register(schema)` - Add schema, reject duplicates
- `GetByModule(name)` - Lookup by module name
- `GetByHandler(path)` - Lookup by handler path
- `FindHandler(path)` - Longest prefix match for routing

### SubsystemHandler Extension

Extend SubsystemHandler to track per-plugin schema information:
- List of schemas declared by this plugin
- List of handler paths this plugin handles

### CLI Commands

Uses existing `system` namespace (no new `hub` namespace):

```bash
system schema list          # List all registered schemas
system schema show <module> # Show specific schema YANG
system schema handlers      # List handler → plugin mapping
system schema protocol      # Show protocol version/format info
```

These follow the existing `system subsystem list` pattern.

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSchemaRegistry_Register` | `internal/plugin/schema_test.go` | Basic schema registration | |
| `TestSchemaRegistry_DuplicateModule` | `internal/plugin/schema_test.go` | Reject duplicate module names | |
| `TestSchemaRegistry_DuplicateHandler` | `internal/plugin/schema_test.go` | Reject duplicate handler paths | |
| `TestSchemaRegistry_FindHandler` | `internal/plugin/schema_test.go` | Longest prefix match for handlers | |
| `TestParseSchemaDeclaration` | `internal/plugin/registration_test.go` | Parse `declare schema` messages | |
| `TestParseSchemaYangHeredoc` | `internal/plugin/registration_test.go` | Parse multi-line YANG heredoc | |
| `TestSubsystemHandler_SchemaCollection` | `internal/plugin/subsystem_test.go` | Handler collects schemas from plugin | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Module name length | 1-256 | 256 chars | 0 (empty) | 257 chars |
| Handler path depth | 1-10 | 10 segments | 0 (empty) | 11 segments |
| YANG text size | 1-1MB | 1MB | 0 (empty) | >1MB |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `schema-declare` | `test/data/plugin/schema-declare.ci` | Plugin declares schema, Hub collects | |
| `schema-duplicate` | `test/data/plugin/schema-duplicate.ci` | Reject duplicate handler registration | |

## Files to Create

- `internal/plugin/schema.go` - SchemaRegistry and Schema types
- `internal/plugin/schema_test.go` - Unit tests
- `cmd/ze/hub/schema.go` - CLI commands for schema discovery
- `test/data/plugin/schema-declare.ci` - Functional test

## Files to Modify

- `internal/plugin/subsystem.go` - Add schema tracking to SubsystemHandler
- `internal/plugin/registration.go` - Add `declare schema` parsing
- `cmd/ze-subsystem/main.go` - Add example schema declarations

## Implementation Steps

**Self-Critical Review:** After each step, review for issues and fix before proceeding.

1. **Write unit tests** - Create schema_test.go with SchemaRegistry tests
2. **Run tests** - Verify FAIL (paste output)
3. **Implement SchemaRegistry** - Create schema.go with Registry
4. **Run tests** - Verify PASS (paste output)
5. **Write declaration parsing tests** - Add to registration_test.go
6. **Run tests** - Verify FAIL
7. **Implement declaration parsing** - Extend parseDeclaration()
8. **Run tests** - Verify PASS
9. **Extend SubsystemHandler** - Add schema collection
10. **Add CLI commands** - Create cmd/ze/hub/schema.go
11. **Functional tests** - Create and run functional tests
12. **Verify all** - `make lint && make test && make functional` (paste output)

## Implementation Summary

### What Was Implemented

1. **SchemaRegistry** (`internal/plugin/schema.go`):
   - `Schema` struct: Module, Namespace, Yang, Handlers, Plugin
   - `Register()`: Add schema with duplicate detection
   - `GetByModule()`, `GetByHandler()`: Exact lookups
   - `FindHandler()`: Longest prefix match routing
   - `ListModules()`, `ListHandlers()`: Discovery

2. **Schema Declaration Parsing** (`internal/plugin/registration.go`):
   - `parseSchema()`: Parse `declare schema module|namespace|handler|yang`
   - `StartHeredoc()`, `IsHeredocEnd()`: Heredoc detection
   - `AppendHeredocLine()`: Multi-line YANG collection
   - `PluginSchemaDecl` struct added to `PluginRegistration`

3. **SubsystemHandler Extension** (`internal/plugin/subsystem.go`):
   - Schema collection during Stage 1 protocol
   - Heredoc parsing for YANG content
   - `Schema()` getter for collected schema
   - `SubsystemManager.AllSchemas()`: Collect from all subsystems
   - `SubsystemManager.RegisterSchemas()`: Register with registry

4. **CLI Commands** (`cmd/ze/bgp/schema.go`):
   - `ze bgp schema list`: List all schemas
   - `ze bgp schema show <module>`: Show YANG content
   - `ze bgp schema handlers`: Handler → module mapping
   - `ze bgp schema protocol`: Protocol documentation

### Tests Added

| Test | File | Coverage |
|------|------|----------|
| `TestSchemaRegistry_*` (12 tests) | `schema_test.go` | Full registry coverage |
| `TestParseHubSchemaDeclaration` | `registration_test.go` | Schema parsing |
| `TestParseSchemaMultipleDeclarations` | `registration_test.go` | Incremental build |
| `TestParseSchemaYangHeredoc` | `registration_test.go` | Heredoc parsing |
| `TestStartHeredoc` | `registration_test.go` | Heredoc detection |

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (initial)
- [x] Implementation complete
- [x] Tests PASS (all 16 new tests pass)
- [x] Boundary tests cover all numeric inputs

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] Code comments added

### Completion
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-hub-phase1-schema-infrastructure.md`
