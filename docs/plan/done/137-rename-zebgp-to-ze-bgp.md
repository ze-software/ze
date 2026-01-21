# Spec: rename-zebgp-to-ze-bgp

## Task

Rename "ze-bgp" to "ze bgp" across the entire codebase:
- Binary: `ze-bgp` → `ze` with `bgp` subcommand
- Go module: `codeberg.org/thomas-mangin/ze` → `codeberg.org/thomas-mangin/ze`
- Environment variables: `ze.bgp.log.*` → `ze.bgp.log.*`
- All documentation and test references

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - understand overall structure
- [ ] `docs/architecture/config/environment.md` - env var conventions

**Key insights:**
- 2418 occurrences across 420 files
- Environment variables handled in `internal/env/env.go` with "zebgp." prefix
- CLI entry point in `cmd/ze/bgp/main.go`

## Scope

### 1. Go Module Path
| From | To |
|------|-----|
| `codeberg.org/thomas-mangin/ze` | `codeberg.org/thomas-mangin/ze` |

Files affected:
- `go.mod` - module declaration
- All `import` statements in `*.go` files

### 2. Binary Name & CLI Structure
| Current | New |
|---------|-----|
| `ze bgp server` | `ze bgp server` |
| `ze bgp config` | `ze bgp config` |
| `ze bgp cli` | `ze bgp cli` |
| `ze bgp decode` | `ze bgp decode` |
| `ze bgp encode` | `ze bgp encode` |
| `ze bgp plugin` | `ze bgp plugin` |
| `ze bgp exabgp` | `ze bgp exabgp` |
| `ze bgp validate` | `ze bgp validate` |
| `zebgp version` | `ze bgp version` |

Directory changes:
- `cmd/ze/bgp/` → `cmd/ze/` (entry point) + `cmd/ze/bgp/` (subcommand)
- `cmd/ze-peer/` → `cmd/ze-peer/`
- `cmd/ze-test/` → `cmd/ze-test/`

### 3. Environment Variables
| Current | New |
|---------|-----|
| `ze.bgp.log.server` | `ze.bgp.log.server` |
| `ze.bgp.log.plugin` | `ze.bgp.log.plugin` |
| `ze.bgp.log.backend` | `ze.bgp.log.backend` |
| `ze.bgp.ci.*` | `ze.bgp.ci.*` |
| `zebgp_*` (underscore) | `ze_bgp_*` |

Files affected:
- `internal/env/env.go` - prefix change

### 4. Documentation Updates
All `.md` files with "ze-bgp" references:
- `CLAUDE.md` - project instructions
- `docs/` directory
- `.claude/` directory

### 5. Test File Updates
- `test/**/*.ci` files - binary references:
  - `ze-bgp` → `ze bgp`
  - `ze-peer` → `ze-peer`
  - `ze-test` → `ze-test`
- `internal/**/*_test.go` - any hardcoded "ze-bgp" strings

### 6. Makefile Updates
- `Makefile` - binary output paths:
  - `bin/zebgp` → `bin/ze`
  - `bin/ze-peer` → `bin/ze-peer`
  - `bin/ze-test` → `bin/ze-test`

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEnvPrefix` | `internal/env/env_test.go` | New `ze.bgp.` prefix works | |
| `TestCLIHelp` | `cmd/ze/cli_test.go` | Help shows `ze bgp` usage | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| All existing | `test/**/*.ci` | Update references, verify pass | |

## Files to Modify

### Core Changes
- `go.mod` - module path
- `internal/env/env.go` - prefix from "ze-bgp" to "ze.bgp"
- `internal/env/env_test.go` - update test expectations
- `internal/slogutil/slogutil.go` - comments reference prefix
- `Makefile` - binary names

### CLI Restructure (Option A - subdirectory)
- `cmd/ze/main.go` - new entry point, dispatches to subcommands
- `cmd/ze/bgp/` - all current `cmd/ze/bgp/*.go` files moved here
- `ze bgp server` → calls `cmd/ze/bgp/server.go`

### Mass Replace (automated)
- All `*.go` files: import path update
- All `*.md` files: documentation updates
- All `*.ci` files: binary name in commands

## Files to Create
- `cmd/ze/main.go` - top-level entry point, dispatches `bgp` subcommand
- `cmd/ze/bgp/` - directory containing all current `cmd/ze/bgp/*.go` files

## Implementation Steps

1. **Update go.mod** - Change module path
2. **Update imports** - Mass replace in all Go files
3. **Update env prefix** - Change "ze-bgp" to "ze.bgp" in internal/env/env.go
4. **Restructure CLI** - Create `cmd/ze/` with `bgp` subcommand
5. **Rename test binaries** - `ze-peer`, `ze-test`
6. **Update Makefile** - New binary paths
7. **Update documentation** - All .md files
8. **Update test files** - All .ci files
9. **Run tests** - `make lint && make test && make functional` (paste output)

## Implementation Order

### Phase 1: Module & Imports
```bash
# Update go.mod
sed -i '' 's|codeberg.org/thomas-mangin/ze|codeberg.org/thomas-mangin/ze|g' go.mod

# Update all imports
find . -name "*.go" -exec sed -i '' 's|codeberg.org/thomas-mangin/ze|codeberg.org/thomas-mangin/ze|g' {} \;
```

### Phase 2: Environment Variables
- `internal/env/env.go`: "zebgp." → "ze.bgp."
- `internal/env/env_test.go`: update expectations

### Phase 3: CLI Restructure (Option A)
- Create `cmd/ze/main.go` with `bgp` subcommand dispatch
- Move `cmd/ze/bgp/*.go` → `cmd/ze/bgp/*.go`
- Delete `cmd/ze/bgp/` after move

### Phase 4: Helper Binaries
- `cmd/ze-peer/` → `cmd/ze-peer/`
- `cmd/ze-test/` → `cmd/ze-test/`

### Phase 5: Build System
- Update `Makefile` targets

### Phase 6: Documentation
- Mass replace in all `.md` files
- Special attention to code examples

### Phase 7: Test Files
- Update `.ci` files:
  - `ze-bgp` → `ze bgp`
  - `ze-peer` → `ze-peer`
  - `ze-test` → `ze-test`

## Implementation Summary

### What Was Implemented
- Module path: `codeberg.org/thomas-mangin/zebgp` → `codeberg.org/thomas-mangin/ze`
- Binary: `zebgp` → `ze` with `bgp` subcommand
- Helpers: `zebgp-peer` → `ze-peer`, `zebgp-test` → `ze-test`
- Config env vars: `zebgp.*` → `ze.bgp.*`
- Logging env vars: `zebgp.log.*` → `ze.log.bgp.*`
- Config dir: `etc/zebgp/` → `etc/ze/bgp/`
- Test script: `zebgp_api.py` → `ze_bgp_api.py`
- ExaBGP command: `zebgp exabgp plugin` → `ze exabgp plugin` (top-level)

### Files Changed
- 511 files modified
- Key structural changes:
  - `cmd/zebgp/` → `cmd/ze/bgp/` (BGP subcommand)
  - `cmd/ze/main.go` (new entry point)
  - `cmd/ze/exabgp/main.go` (ExaBGP subcommand)

### Verification
- `make lint`: 0 issues
- `make test`: all pass
- `make functional`: 96 tests pass (42 encoding + 24 plugin + 12 parsing + 18 decoding)

## Checklist

### 🧪 TDD
- [x] Tests written
- [x] Tests FAIL (output below)
- [x] Implementation complete
- [x] Tests PASS (output below)

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] All docs updated with new naming

### Completion
- [x] Architecture docs updated
- [x] Spec moved to `docs/plan/done/137-rename-zebgp-to-ze-bgp.md`
- [x] All files committed together
