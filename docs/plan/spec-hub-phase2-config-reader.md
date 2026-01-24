# Spec: Hub Phase 2 - Config Reader Process

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/hub-architecture.md` - Hub design, Config Reader role
4. `docs/architecture/config/yang-config-design.md` - YANG config design
5. `internal/plugin/subsystem.go` - SubsystemHandler pattern
6. `internal/config/tokenizer.go` - existing config tokenizer

## Task

Create the Config Reader as a separate process that receives YANG schemas from the Hub, parses the config file, and reports parsed config blocks back to the Hub.

### Goals

1. Create `ze-config-reader` binary (specialized process, not a regular plugin)
2. Receive schemas + config path from Hub (text protocol)
3. Parse config file using existing tokenizer
4. Send verify/apply requests to Hub for each config block
5. Handle reload requests from Hub

### Non-Goals

- YANG validation (Phase 3)
- Hub-side verify/apply routing (Phase 4) - Config Reader sends verify/apply, Hub routes them
- Semantic validation - just parse and structure, plugins validate semantics

### Dependencies

- Phase 1: Schema Infrastructure (schemas must be collected first)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/hub-architecture.md` - [Config Reader responsibilities, message formats]
- [ ] `docs/architecture/config/syntax.md` - [config syntax]
- [ ] `docs/architecture/api/ipc_protocol.md` - [text protocol format]

### Source Files
- [ ] `internal/config/tokenizer.go` - [existing tokenizer]
- [ ] `internal/config/parser.go` - [existing parser]
- [ ] `cmd/ze-subsystem/main.go` - [subsystem pattern to follow]
- [ ] `internal/plugin/subsystem.go` - [how Hub spawns processes]

**Key insights:**
- Config Reader is spawned after Stage 1 completes (after all plugins declare schemas)
- Config Reader does NOT participate in Stage 1 - it receives schemas, doesn't declare them
- Uses text protocol like regular plugins (`#serial command`, `@serial response`)
- Must handle hierarchical config blocks and map to handler paths

## Design

### Config Reader Binary

The Config Reader binary (`ze-config-reader`) is a specialized process (NOT a regular plugin):

1. **No Stage 1** - Config Reader is spawned AFTER Stage 1 completes
2. Receive schemas + config path from Hub (YANG content inline)
3. Parse config file using existing tokenizer
4. Validate against combined YANG schema (Phase 3)
5. Send verify/apply requests for each config block
6. Handle reload requests from Hub

**Why no Stage 1?**
- Config Reader needs schemas from all plugins before it can work
- Hub spawns it after collecting all schema declarations
- Hub handles `config reload` and `config validate` commands directly, delegates to Config Reader

### Message Protocol

Config Reader uses text protocol but with a simplified lifecycle (no 5-stage).

**Hub initializes Config Reader (after spawning)**
```
config schema ze-bgp handlers bgp,bgp.peer yang <<EOF
module ze-bgp {
  namespace "urn:ze:bgp";
  ...
}
EOF
config schema ze-rib handlers rib yang <<EOF
module ze-rib {
  ...
}
EOF
config path /etc/ze/config.conf
config done
```

**Key points:**
- Hub sends YANG content inline (collected from plugins in Stage 1)
- Config Reader receives complete schemas
- Heredoc format for multi-line YANG

**Runtime: Config Reader sends verify requests**
```
#1 config verify handler "bgp.peer" action create path "bgp.peer[address=192.0.2.1]" data '{"address":"192.0.2.1","peer-as":65002}'
```

**Hub responds**
```
@1 done
```
or
```
@1 error peer-as cannot equal local-as
```

**Config Reader signals completion**
```
#2 config complete
```

**Hub sends reload request**
```
#abc config reload
```

**Config Reader responds**
```
@abc done
```

**Key consistency points:**
- All messages use text protocol: `#serial command args` and `@serial status [data]`
- Status values: `done`, `error` (standard values)
- Config Reader uses same protocol as regular plugins

### Handler Path Mapping

The Config Reader maps config blocks to handler paths:

| Config Block | Handler Path |
|--------------|--------------|
| `bgp { ... }` | `bgp` |
| `bgp { peer 192.0.2.1 { ... } }` | `bgp.peer[address=192.0.2.1]` |
| `bgp { peer-group upstream { ... } }` | `bgp.peer-group[name=upstream]` |
| `rib { ... }` | `rib` |

The mapping uses schema handler declarations to know which blocks exist.

### Config Reader State

Config Reader maintains:

| Field | Description |
|-------|-------------|
| schemas | List of schema info (module, namespace, handlers, yang) |
| configPath | Path to config file |
| current | Current parsed config (for diff on reload) |

**SchemaInfo fields:**
- Module name
- Namespace URI
- Handler paths list
- YANG content (received inline from Hub)

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestConfigReader_ParseInit` | `cmd/ze-config-reader/main_test.go` | Parse init message | |
| `TestConfigReader_ParseConfig` | `cmd/ze-config-reader/main_test.go` | Parse config file to blocks | |
| `TestConfigReader_MapToHandlers` | `cmd/ze-config-reader/main_test.go` | Map blocks to handler paths | |
| `TestConfigReader_UnknownBlock` | `cmd/ze-config-reader/main_test.go` | Error on unknown top-level block | |
| `TestConfigReader_Reload` | `cmd/ze-config-reader/main_test.go` | Reload produces diff | |
| `TestHub_SpawnConfigReader` | `internal/plugin/hub_test.go` | Hub spawns and initializes Config Reader | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Config file size | 1-10MB | 10MB | 0 (empty) | >10MB |
| Block nesting depth | 1-20 | 20 levels | 0 | 21 levels |
| Handler path length | 1-512 | 512 chars | 0 | 513 chars |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `config-reader-basic` | `test/data/plugin/config-reader-basic.ci` | Basic config parsing | |
| `config-reader-nested` | `test/data/plugin/config-reader-nested.ci` | Nested blocks mapped correctly | |
| `config-reader-error` | `test/data/plugin/config-reader-error.ci` | Unknown block error | |
| `config-reader-reload` | `test/data/plugin/config-reader-reload.ci` | Config reload diff | |

## Files to Create

- `cmd/ze-config-reader/main.go` - Config Reader binary
- `cmd/ze-config-reader/main_test.go` - Unit tests
- `cmd/ze-config-reader/parser.go` - Config to handler path mapping
- `internal/plugin/hub.go` - Hub orchestration (if not exists)
- `test/data/plugin/config-reader-*.ci` - Functional tests

## Files to Modify

- `internal/plugin/subsystem.go` - Add Config Reader spawning logic
- `Makefile` - Add ze-config-reader build target

## Implementation Steps

**Self-Critical Review:** After each step, review for issues and fix before proceeding.

1. **Write unit tests** - Create main_test.go with Config Reader tests
2. **Run tests** - Verify FAIL (paste output)
3. **Create binary scaffold** - Basic main.go with message reading
4. **Implement init handling** - Parse schemas and config path
5. **Run tests** - Verify partial PASS
6. **Implement config parsing** - Use existing tokenizer, map to handlers
7. **Run tests** - Verify PASS (paste output)
8. **Add reload support** - Diff current vs new config
9. **Integrate with Hub** - Add spawning logic to subsystem.go
10. **Functional tests** - Create and run
11. **Verify all** - `make lint && make test && make functional` (paste output)

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
- [ ] Code comments added

### Completion
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-hub-phase2-config-reader.md`
