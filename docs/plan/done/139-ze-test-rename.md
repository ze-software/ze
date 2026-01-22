# Spec: ze-test-rename

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `cmd/ze-test/main.go` - entry point
4. `cmd/ze-test/bgp.go` - BGP test commands

## Task

Rename ze-test commands for consistency:
- `ze-test run` → `ze-test bgp`
- `encoding` → `encode`
- `decoding` → `decode`
- `parsing` → `parse`

## Required Reading

### Architecture Docs
- [x] `docs/functional-tests.md` - test system documentation

**Key insights:**
- Commands used throughout docs and Makefile
- Permission rules in `.claude/settings.local.json` reference old names

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N/A | N/A | Rename only, no logic changes | N/A |

### Functional Tests
| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| Existing tests | `test/data/*/` | All 80+ tests must pass after rename | ✅ |

## Files to Modify
- `cmd/ze-test/main.go` - change `run` to `bgp` in switch
- `cmd/ze-test/run.go` → `cmd/ze-test/bgp.go` - rename file and update content
- `Makefile` - update functional test targets
- `internal/test/runner/report.go` - update debug output commands
- `.claude/rules/testing.md` - update documentation
- `.claude/settings.local.json` - update permission rule
- `docs/functional-tests.md` - update all examples
- `docs/debugging-tools.md` - update all examples

## Files to Create
None

## Implementation Steps
1. [x] Rename `cmd/ze-test/run.go` to `cmd/ze-test/bgp.go`
2. [x] Update `main.go` switch case from `run` to `bgp`
3. [x] Update all command names in `bgp.go`
4. [x] Update Makefile targets
5. [x] Update `report.go` debug commands
6. [x] Update `.claude/rules/testing.md`
7. [x] Update `.claude/settings.local.json` permission
8. [x] Update `docs/functional-tests.md`
9. [x] Update `docs/debugging-tools.md`
10. [x] Run `make test && make lint && make functional`

## Implementation Summary

### What Was Implemented
- Renamed `ze-test run` → `ze-test bgp`
- Renamed subcommands: `encoding` → `encode`, `decoding` → `decode`, `parsing` → `parse`
- Updated Makefile targets: `functional-encode`, `functional-plugin`, `functional-decode`, `functional-parse`
- Updated all documentation examples

### Files Changed
| File | Change |
|------|--------|
| `cmd/ze-test/main.go` | `run` → `bgp` in switch |
| `cmd/ze-test/run.go` → `bgp.go` | Renamed + updated all command names |
| `Makefile` | Updated 4 target names |
| `internal/test/runner/report.go` | Updated debug command output |
| `.claude/rules/testing.md` | Updated docs |
| `.claude/settings.local.json` | Updated permission |
| `docs/functional-tests.md` | Updated ~10 examples |
| `docs/debugging-tools.md` | Updated ~6 examples |

### Deviations from Plan
None

## Checklist

### 🧪 TDD
- [x] Tests written (existing tests cover functionality)
- [x] Tests FAIL - N/A (rename only)
- [x] Implementation complete
- [x] Tests PASS

### Verification
- [x] `make lint` passes
- [x] `make test` passes
- [x] `make functional` passes

### Documentation
- [x] Required docs read
- [x] All examples updated

### Completion
- [x] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] All files committed together
