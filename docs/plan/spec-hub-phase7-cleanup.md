# Spec: hub-phase7-cleanup

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/plan/hub-separation-phases.md` - phase overview
4. Previous phase specs (1-6) in `docs/plan/done/`

## Task

Final cleanup after hub separation:
1. Remove old reactor code (now under `internal/plugin/bgp/reactor/`)
2. Update any remaining imports
3. Verify all tests pass
4. Update documentation to reflect new structure

**Scope:** Cleanup, import updates, documentation.

**Depends on:** Phases 1-6 complete

## Required Reading

### Source Files
- [ ] `internal/reactor/` - should be empty or deleted
- [ ] `internal/bgp/` - should be empty or deleted
- [ ] `internal/rib/` - should be empty or deleted (peer RIB moved)
- [ ] `go.mod` - verify module structure

**Key insights:**
- Phase 4 moved code but may have left stubs
- Old import paths may exist in comments or docs
- Architecture docs need updates for new paths

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A - cleanup phase | | | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A - no new numeric inputs | | | | |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `hub-full-startup` | `test/data/hub/full-startup.ci` | Complete system starts correctly | |
| `hub-peer-session` | `test/data/hub/peer-session.ci` | BGP session establishes via hub | |

## Files to Delete

- `internal/reactor/` - if any stubs remain
- `internal/bgp/` - if any stubs remain
- `internal/rib/` - if any stubs remain (peer RIB)

## Files to Modify

- `docs/architecture/core-design.md` - Update package paths
- `docs/architecture/system-architecture.md` - Verify accuracy
- Any files with old import paths in comments

## Implementation Steps

1. **Search for old imports** - Find any remaining references
   ```
   grep -r "internal/reactor" --include="*.go"
   grep -r "internal/bgp" --include="*.go"
   grep -r "internal/rib" --include="*.go" | grep -v "internal/plugin"
   ```

   → **Review:** Any legitimate references remaining?

2. **Delete empty directories** - Remove old package locations

3. **Update documentation** - Fix package paths in docs
   - `docs/architecture/core-design.md`
   - Any RFC summaries referencing old paths

4. **Run full test suite** - Verify everything works
   ```bash
   make lint && make test && make functional
   ```

   → **Review:** All 80+ functional tests pass?

5. **Final verification** - Check package structure
   ```
   internal/
   ├── hub/           # New hub code
   ├── plugin/
   │   ├── bgp/       # BGP engine (moved from internal/bgp, reactor, rib)
   │   │   ├── message/
   │   │   ├── attribute/
   │   │   ├── nlri/
   │   │   ├── capability/
   │   │   ├── fsm/
   │   │   ├── reactor/
   │   │   ├── rib/
   │   │   └── filter/
   │   ├── rib/       # Adj-RIB plugin (separate process)
   │   └── gr/        # GR plugin
   ```

## Design Decisions

### What to preserve vs delete

| Item | Action | Reason |
|------|--------|--------|
| Old package directories | Delete | Code moved in Phase 4 |
| Import path comments | Update | Keep docs accurate |
| Test files | Keep | Tests moved with code |

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
