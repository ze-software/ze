# Spec: hub-phase2-config

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/hub-separation-phases.md` - phase overview
4. `docs/plan/done/NNN-hub-phase1-foundation.md` - Phase 1 (must be done)

## Task

Implement 3-section config parsing in the hub:
1. `env { }` - global settings (handled by hub)
2. `plugin { }` - process declarations (what to fork)
3. Remaining blocks - plugin configs (stored for later routing)

**Scope:** Parse config, handle env, know what to fork. No YANG validation yet, no JSON delivery yet.

**Depends on:** Phase 1 complete

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/system-architecture.md` - config file structure
- [ ] `docs/architecture/config/yang-config-design.md` - config design

### Source Files
- [ ] `internal/config/tokenizer.go` - existing tokenizer
- [ ] `yang/ze-plugin.yang` - plugin declaration syntax
- [ ] Phase 1 files - `internal/hub/hub.go`

**Key insights:**
- Existing tokenizer can parse the config syntax
- `ze-plugin.yang` defines `plugin { external NAME { run "..."; } }`
- Hub stores config as map-of-maps
- Env block sets up environment before forking

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseEnvBlock` | `internal/hub/config_test.go` | Parse env { } correctly | |
| `TestParsePluginBlock` | `internal/hub/config_test.go` | Parse plugin { external ... } | |
| `TestParseConfigBlocks` | `internal/hub/config_test.go` | Parse remaining blocks as map | |
| `TestEnvApplied` | `internal/hub/config_test.go` | Env settings affect hub | |
| `TestPluginListExtracted` | `internal/hub/config_test.go` | Get list of processes to fork | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - config values validated by YANG in Phase 3 | | | | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| Deferred to Phase 5 | | | |

## Files to Modify

- `internal/hub/hub.go` - Add LoadConfig method

## Files to Create

- `internal/hub/config.go` - Config parsing, env handling
- `internal/hub/config_test.go` - Unit tests

## Implementation Steps

1. **Write unit tests** - Test config parsing

   → **Review:** Cover malformed configs? Missing sections?

2. **Run tests** - Verify FAIL (paste output)

3. **Create config.go** - Parse 3-section config

   **Config structure:**
   | Field | Type | Description |
   |-------|------|-------------|
   | Env | map | Environment settings from `env { }` |
   | Plugins | list | Plugin declarations from `plugin { external ... }` |
   | Blocks | map | Remaining config blocks (stored for routing) |

   **Behavior:**
   1. Parse config file using existing tokenizer
   2. Extract env block, apply settings
   3. Extract plugin block, build list of processes to fork
   4. Store remaining blocks as nested map

   → **Review:** Reuses existing tokenizer?

4. **Handle env block** - Apply settings
   - `api-socket` → set socket path
   - `log-level` → configure logging
   - `working-dir` → set cwd

   → **Review:** What env vars are needed?

5. **Run tests** - Verify PASS (paste output)

6. **Verify** - `make lint && make test && make functional`

## Design Decisions

### Config storage format

Hub stores config as nested maps (`map[string]any`), not as Go structs. This allows:
- Schema-agnostic storage
- Easy JSON conversion
- Plugins define their own structure via YANG

### Env block handling

| Setting | Effect |
|---------|--------|
| `api-socket` | Unix socket path for CLI |
| `log-level` | Hub logging verbosity |
| `working-dir` | Working directory for children |

## Implementation Summary

<!-- Fill after implementation -->

### What Was Implemented
- [List actual changes]

### Bugs Found/Fixed
- [Any bugs discovered]

### Deviations from Plan
- [Any differences and why]

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

### Documentation (during implementation)
- [ ] Required docs read

### Completion (after tests pass)
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
