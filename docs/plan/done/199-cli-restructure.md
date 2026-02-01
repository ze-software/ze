# Spec: cli-restructure

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze/main.go` - main CLI entry point
4. `cmd/ze/bgp/main.go` - bgp subcommand routing
5. `cmd/ze/config/main.go` - current config subcommand

## Task

Restructure Ze CLI to reflect that Ze engine owns config parsing, validation, and schema management. Move commands from `ze bgp` to root level where Ze (not the BGP plugin) is the authority.

**Rationale:** Ze engine has the YANG schema, performs validation, and passes JSON to plugins via API. Commands that operate on Ze's config system should be at root level, not under `bgp`.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - Ze engine vs plugin separation
- [ ] `docs/architecture/config/syntax.md` - Config parsing ownership

**Key insights:**
- Ze engine owns YANG schema and config parsing
- Plugins receive pre-parsed, pre-validated JSON via API
- Config validation is centralized in Ze, not per-plugin

## Current Behavior (MANDATORY)

**Source files read:**

- [ ] `cmd/ze/main.go` - Routes: `bgp`, `cli`, `config`, `exabgp`, `version`, `help`, config file detection
- [ ] `cmd/ze/bgp/main.go` - Routes: `server` (deprecated), `validate`, `decode`, `encode`, `config`, `plugin`, `config-dump`, `schema`, `version`, `help`
- [ ] `cmd/ze/config/main.go` - Routes: `edit` only (incomplete)
- [ ] `cmd/ze/config/edit.go` - Uses `internal/config/editor`, identical to bgp version
- [ ] `cmd/ze/bgp/config.go` - Routes: `edit`, `check`, `migrate`, `fmt`
- [ ] `cmd/ze/bgp/config_edit.go` - Uses `internal/config/editor`, identical to root version
- [ ] `cmd/ze/bgp/validate.go` - Uses `config.YANGSchema()`, `config.NewParser()`, `config.TreeToConfig()`
- [ ] `cmd/ze/bgp/schema.go` - Uses `plugin.SchemaRegistry`, shows ze-bgp, ze-plugin, ze-types modules
- [ ] `cmd/ze/bgp/configdump.go` - Uses `config.YANGSchema()`, `config.NewParser()`, outputs parsed config

**Behavior to preserve:**
- All command functionality (validate, config edit/check/migrate/fmt, schema list/show/handlers/protocol)
- Exit codes (0=success, 1=error, 2=file not found for validate)
- Output formats (text and JSON where supported)
- Flag names and behavior (-v, -q, --json, -o, --in-place, -w)

**Behavior to change:** (user explicitly requested)
- `ze bgp validate` → `ze validate`
- `ze bgp config` → `ze config` (merge with existing, remove duplicate)
- `ze bgp schema` → `ze schema`
- `ze bgp config-dump` → `ze config dump` (becomes subcommand of config)
- `ze bgp version` → removed (use `ze version`)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A | N/A | No new logic - only command routing changes | N/A |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | No numeric inputs added | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `validate-valid` | `test/cli/validate-valid.ci` | `ze validate config.conf` exits 0 for valid config | |
| `validate-invalid` | `test/cli/validate-invalid.ci` | `ze validate bad.conf` exits 1 with error message | |
| `config-edit-help` | `test/cli/config-edit-help.ci` | `ze config edit --help` shows usage | |
| `config-check` | `test/cli/config-check.ci` | `ze config check config.conf` works | |
| `config-dump` | `test/cli/config-dump.ci` | `ze config dump config.conf` shows parsed config | |
| `schema-list` | `test/cli/schema-list.ci` | `ze schema list` shows registered schemas | |
| `old-path-error` | `test/cli/old-path-error.ci` | `ze bgp validate` shows deprecation message | |

### Future (if deferring any tests)
- None - all tests should be implemented with the change

## Files to Modify

- `cmd/ze/main.go` - Add routes for `validate`, `schema`; update `config` routing
- `cmd/ze/bgp/main.go` - Remove `validate`, `config`, `config-dump`, `schema`, `version`; add deprecation warnings
- `cmd/ze/config/main.go` - Add routes for `check`, `migrate`, `fmt`, `dump`

## Files to Create

- `cmd/ze/validate/main.go` - Moved from `cmd/ze/bgp/validate.go`
- `cmd/ze/schema/main.go` - Moved from `cmd/ze/bgp/schema.go`
- `cmd/ze/config/check.go` - Moved from `cmd/ze/bgp/config_check.go`
- `cmd/ze/config/migrate.go` - Moved from `cmd/ze/bgp/config_migrate.go`
- `cmd/ze/config/fmt.go` - Moved from `cmd/ze/bgp/config_fmt.go`
- `cmd/ze/config/dump.go` - Moved from `cmd/ze/bgp/configdump.go`
- `test/cli/*.ci` - Functional tests for new command paths

## Files to Delete

- `cmd/ze/bgp/validate.go` - Moved to `cmd/ze/validate/`
- `cmd/ze/bgp/schema.go` - Moved to `cmd/ze/schema/`
- `cmd/ze/bgp/config.go` - Merged into `cmd/ze/config/`
- `cmd/ze/bgp/config_edit.go` - Duplicate of `cmd/ze/config/edit.go`
- `cmd/ze/bgp/config_check.go` - Moved to `cmd/ze/config/`
- `cmd/ze/bgp/config_migrate.go` - Moved to `cmd/ze/config/`
- `cmd/ze/bgp/config_fmt.go` - Moved to `cmd/ze/config/`
- `cmd/ze/bgp/configdump.go` - Moved to `cmd/ze/config/dump.go`

## Implementation Steps

Each step ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Create functional tests for new paths** - Write .ci tests that will fail until implementation
   → **Review:** Do tests cover all moved commands? Both success and error cases?

2. **Run tests** - Verify FAIL (paste output)
   → **Review:** Do tests fail for the RIGHT reason (command not found)?

3. **Move validate command** - Create `cmd/ze/validate/main.go`, update `cmd/ze/main.go`
   → **Review:** Package name correct? Imports updated? Help text updated?

4. **Move schema command** - Create `cmd/ze/schema/main.go`, update `cmd/ze/main.go`
   → **Review:** Package name correct? Imports updated? Help text updated?

5. **Consolidate config command** - Move subcommands from bgp to config, add dump
   → **Review:** All subcommands present? Duplicate edit.go removed?

6. **Add deprecation warnings** - Update `cmd/ze/bgp/main.go` to warn on old paths
   → **Review:** Warnings helpful? Point to new command path?

7. **Delete old files** - Remove moved/duplicate files from `cmd/ze/bgp/`
   → **Review:** No orphaned imports? No broken references?

8. **Run tests** - Verify PASS (paste output)
   → **Review:** All tests pass? Both old deprecation and new paths work?

9. **Update help text** - Ensure all usage strings reflect new structure
   → **Review:** Examples show new paths? Consistent formatting?

10. **Final verification** - `make lint && make test && make functional`
    → **Review:** Zero issues? All tests pass?

## Design Decisions

- **Separate packages per command** (validate, schema, config): Follows existing pattern (`cmd/ze/bgp/`, `cmd/ze/cli/`, `cmd/ze/exabgp/`)
- **Deprecation warnings instead of immediate removal**: Allows scripts to adapt, provides migration path
- **config dump as subcommand**: Aligns with other config subcommands (edit, check, migrate, fmt)

## Checklist

### Design (see `rules/design-principles.md`)
- [ ] No premature abstraction (3+ concrete use cases exist?) - N/A, restructuring
- [ ] No speculative features (is this needed NOW?) - Yes, reflects architecture
- [ ] Single responsibility (each component does ONE thing?) - Yes
- [ ] Explicit behavior (no hidden magic or conventions?) - Yes
- [ ] Minimal coupling (components isolated, dependencies minimal?) - Yes
- [ ] Next-developer test (would they understand this quickly?) - Yes, clearer structure

### TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs (N/A - no numeric inputs)
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation (during implementation)
- [ ] Required docs read
- [ ] Help text updated for all moved commands
- [ ] Usage examples updated

### Completion (after tests pass)
- [ ] Architecture docs updated with learnings
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together

## New CLI Structure (after implementation)

```
ze
├── version                    # "ze 0.1.0"
├── validate                   # NEW: moved from ze bgp validate
├── config                     # EXPANDED: merged with ze bgp config
│   ├── edit
│   ├── check
│   ├── migrate
│   ├── fmt
│   └── dump                   # NEW: was ze bgp config-dump
├── schema                     # NEW: moved from ze bgp schema
│   ├── list
│   ├── show
│   ├── handlers
│   └── protocol
├── cli
├── bgp                        # REDUCED: protocol-specific only
│   ├── decode
│   ├── encode
│   └── plugin
├── exabgp
└── <config>                   # Auto-detect and start daemon
```
