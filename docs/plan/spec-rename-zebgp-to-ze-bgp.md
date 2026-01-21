# Spec: rename-zebgp-to-ze-bgp

## Task

Rename "zebgp" to "ze bgp" across the entire codebase:
- Binary: `zebgp` → `ze` with `bgp` subcommand
- Go module: `codeberg.org/thomas-mangin/zebgp` → `codeberg.org/thomas-mangin/ze`
- Environment variables: `zebgp.log.*` → `ze.bgp.log.*`
- All documentation and test references

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - understand overall structure
- [ ] `docs/architecture/config/environment.md` - env var conventions

**Key insights:**
- 2418 occurrences across 420 files
- Environment variables handled in `internal/env/env.go` with "zebgp." prefix
- CLI entry point in `cmd/zebgp/main.go`

## Scope

### 1. Go Module Path
| From | To |
|------|-----|
| `codeberg.org/thomas-mangin/zebgp` | `codeberg.org/thomas-mangin/ze` |

Files affected:
- `go.mod` - module declaration
- All `import` statements in `*.go` files

### 2. Binary Name & CLI Structure
| Current | New |
|---------|-----|
| `zebgp server` | `ze bgp server` |
| `zebgp config` | `ze bgp config` |
| `zebgp cli` | `ze bgp cli` |
| `zebgp decode` | `ze bgp decode` |
| `zebgp encode` | `ze bgp encode` |
| `zebgp plugin` | `ze bgp plugin` |
| `zebgp exabgp` | `ze bgp exabgp` |
| `zebgp validate` | `ze bgp validate` |
| `zebgp version` | `ze bgp version` |

Directory changes:
- `cmd/zebgp/` → `cmd/ze/` (new entry point with `bgp` subcommand)
- `cmd/zebgp-peer/` → `cmd/ze-bgp-peer/` (or keep as helper tool)
- `cmd/zebgp-test/` → `cmd/ze-bgp-test/` (or keep as helper tool)

### 3. Environment Variables
| Current | New |
|---------|-----|
| `zebgp.log.server` | `ze.bgp.log.server` |
| `zebgp.log.plugin` | `ze.bgp.log.plugin` |
| `zebgp.log.backend` | `ze.bgp.log.backend` |
| `zebgp.ci.*` | `ze.bgp.ci.*` |
| `zebgp_*` (underscore) | `ze_bgp_*` |

Files affected:
- `internal/env/env.go` - prefix change

### 4. Documentation Updates
All `.md` files with "zebgp" references:
- `CLAUDE.md` - project instructions
- `docs/` directory
- `.claude/` directory

### 5. Test File Updates
- `test/**/*.ci` files - references to `zebgp` binary
- `internal/**/*_test.go` - any hardcoded "zebgp" strings

### 6. Makefile Updates
- `Makefile` - binary output paths

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
- `internal/env/env.go` - prefix from "zebgp" to "ze.bgp"
- `internal/env/env_test.go` - update test expectations
- `internal/slogutil/slogutil.go` - comments reference prefix
- `Makefile` - binary names

### CLI Restructure
- `cmd/zebgp/main.go` → `cmd/ze/main.go` - new entry point with `bgp` subcommand
- Move all `cmd/zebgp/*.go` → `cmd/ze/bgp/*.go` or similar structure

### Mass Replace (automated)
- All `*.go` files: import path update
- All `*.md` files: documentation updates
- All `*.ci` files: binary name in commands

## Files to Create
- `cmd/ze/main.go` - new top-level entry point
- `cmd/ze/bgp.go` - bgp subcommand that delegates to current zebgp logic

## Implementation Steps

1. **Update go.mod** - Change module path
2. **Update imports** - Mass replace in all Go files
3. **Update env prefix** - Change "zebgp" to "ze.bgp" in internal/env/env.go
4. **Restructure CLI** - Create `cmd/ze/` with `bgp` subcommand
5. **Rename test binaries** - `zebgp-peer`, `zebgp-test`
6. **Update Makefile** - New binary paths
7. **Update documentation** - All .md files
8. **Update test files** - All .ci files
9. **Run tests** - `make lint && make test && make functional` (paste output)

## Implementation Order

### Phase 1: Module & Imports
```bash
# Update go.mod
sed -i '' 's|codeberg.org/thomas-mangin/zebgp|codeberg.org/thomas-mangin/ze|g' go.mod

# Update all imports
find . -name "*.go" -exec sed -i '' 's|codeberg.org/thomas-mangin/zebgp|codeberg.org/thomas-mangin/ze|g' {} \;
```

### Phase 2: Environment Variables
- `internal/env/env.go`: "zebgp." → "ze.bgp."
- `internal/env/env_test.go`: update expectations

### Phase 3: CLI Restructure
- Create `cmd/ze/main.go` with `bgp` subcommand dispatch
- Move existing `cmd/zebgp/` code under `cmd/ze/bgp/` or inline

### Phase 4: Helper Binaries
- `cmd/zebgp-peer/` → `cmd/ze-bgp-peer/`
- `cmd/zebgp-test/` → `cmd/ze-bgp-test/`

### Phase 5: Build System
- Update `Makefile` targets

### Phase 6: Documentation
- Mass replace in all `.md` files
- Special attention to code examples

### Phase 7: Test Files
- Update `.ci` files with new binary names

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] All docs updated with new naming

### Completion
- [ ] Architecture docs updated
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
