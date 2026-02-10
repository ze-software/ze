# Spec: config-reload-2-coordinator

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugin/server.go` — deliverConfigRPC(), extractConfigSubtree(), process management
4. `internal/config/diff.go` — DiffMaps() implementation

**Parent spec:** `spec-reload-lifecycle-tests.md` (umbrella)
**Depends on:** `spec-config-reload-1-rpc.md` (RPC types must exist)

## Task

Build the reload coordinator that orchestrates verify→apply across all config-interested plugins. The coordinator:
1. Computes diff between running and new config using `DiffMaps()`
2. Sends `config-verify` RPC to each plugin with matching `WantsConfigRoots`
3. If ALL verify pass → sends `config-apply` RPC with diff sections
4. If ANY verify fails → aborts, returns error, running config unchanged

The coordinator is generic — it works with `map[string]any` trees and never touches typed config structs.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — plugin protocol, Server role
- [ ] `docs/architecture/config/yang-config-design.md` — handler interface, commit flow

### Source Files (MUST read)
- [ ] `internal/plugin/server.go` — Server struct, deliverConfigRPC(), extractConfigSubtree(), procManager
- [ ] `internal/plugin/types.go` — ReactorInterface (GetConfigTree, other methods)
- [ ] `internal/config/diff.go` — DiffMaps(), ConfigDiff, DiffPair
- [ ] `pkg/plugin/rpc/types.go` — ConfigSection, ConfigVerifyInput/Output, ConfigApplyInput/Output (from sub-spec 1)

**Key insights:**
- `extractConfigSubtree(configTree, root)` already extracts per-root sections — reuse for verify
- `procManager` tracks running plugins and their connections
- `WantsConfigRoots` is set during Stage 1 registration
- Sequential verify is simpler and sufficient (concurrent adds timeout complexity)
- Coordinator needs mutex to prevent concurrent reload requests

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/server.go` — Server delivers config at Stage 2 via deliverConfigRPC(), no reload path
- [ ] `internal/config/diff.go` — DiffMaps() produces ConfigDiff{Added, Removed, Changed} with dotted keys
- [ ] `internal/plugin/types.go` — ReactorInterface includes GetConfigTree() returning map[string]any

**Behavior to preserve:**
- Stage 2 config delivery unchanged
- DiffMaps() API unchanged
- Server startup/shutdown unchanged
- All existing plugin lifecycle unchanged

**Behavior to change:**
- Server gains `reloadConfig()` method for orchestrating verify→apply
- Server gains `ReloadFromDisk()` entry point for SIGHUP (wired in sub-spec 3)

## Data Flow (MANDATORY)

### Entry Point
- `server.reloadConfig(ctx, newTree map[string]any)` — called by SIGHUP handler or editor
- `server.ReloadFromDisk(ctx)` — parses config file, calls reloadConfig

### Transformation Path
1. Get running tree: `s.reactor.GetConfigTree()`
2. Compute diff: `config.DiffMaps(running, newTree)`
3. If diff has no Added, Removed, or Changed → return nil (no-op)
4. For each running plugin with `WantsConfigRoots`:
   - Extract per-root section from newTree using `extractConfigSubtree()`
   - Filter to only roots that have changes in the diff
5. **Verify phase:** For each affected plugin, send `SendConfigVerify()` with sections
6. Collect verify results — if ANY error, return aggregated error
7. **Apply phase:** Build `ConfigDiffSection` per root from the ConfigDiff
8. For each affected plugin, send `SendConfigApply()` with diff sections
9. Update running config: `s.reactor.SetConfigTree(newTree)`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Coordinator → Plugin | SendConfigVerify/SendConfigApply via Socket B | [ ] |
| Coordinator → Reactor | Direct method call (SetConfigTree) | [ ] |
| Coordinator → DiffMaps | Function call (internal/config package) | [ ] |

### Integration Points
- `extractConfigSubtree()` in server.go — reuse for building verify sections
- `procManager.Running()` or equivalent — get list of running plugins
- `proc.Registration().WantsConfigRoots` — check plugin's declared interests
- `proc.ConnB()` — get Socket B for sending RPCs
- `config.DiffMaps()` — compute changes

### Architectural Verification
- [ ] No bypassed layers (coordinator is the single path for config changes)
- [ ] No unintended coupling (works with map[string]any, no typed config)
- [ ] No duplicated functionality (reuses extractConfigSubtree)
- [ ] Zero-copy preserved where applicable (N/A — config is JSON)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReloadConfigNoChange` | `internal/plugin/reload_test.go` | Empty diff → no verify/apply sent | |
| `TestReloadConfigVerifyFails` | `internal/plugin/reload_test.go` | Verify error → no apply sent, error returned | |
| `TestReloadConfigVerifyThenApply` | `internal/plugin/reload_test.go` | All verify OK → apply sent to all, running updated | |
| `TestReloadConfigPerRootFiltering` | `internal/plugin/reload_test.go` | Only plugins with matching WantsConfigRoots get RPCs | |
| `TestReloadConfigMultiplePlugins` | `internal/plugin/reload_test.go` | Multiple plugins, one rejects → all abort | |
| `TestReloadConfigConcurrentRejected` | `internal/plugin/reload_test.go` | Second reload while first in progress → error | |
| `TestReloadFromDiskParseError` | `internal/plugin/reload_test.go` | Config parse failure → error, running unchanged | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no new numeric inputs.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | N/A | Coordinator is internal — functional tests in sub-spec 5 | |

## Files to Modify
- `internal/plugin/server.go` — add ReloadFromDisk(), reloadConfig() methods (or import from reload.go)
- `internal/plugin/types.go` — add SetConfigTree() to ReactorInterface if not present

## Files to Create
- `internal/plugin/reload.go` — coordinator logic
- `internal/plugin/reload_test.go` — coordinator tests with mock plugins

## Implementation Steps

### Step 1: Write coordinator tests with mocks
Create mock plugins (mock ConnB that records RPCs) and mock reactor (mock GetConfigTree/SetConfigTree). Test verify-fail, verify-then-apply, no-change, per-root filtering.

### Step 2: Implement coordinator
Add `reloadConfig()` and `ReloadFromDisk()` to Server. Use mutex for concurrency protection. Reuse `extractConfigSubtree()` for building per-root sections.

### Step 3: Add SetConfigTree to reactor interface
If not already present, add `SetConfigTree(map[string]any)` to ReactorInterface and implement on reactorAPIAdapter.

### Step 4: Build ConfigDiffSection from ConfigDiff
Helper function to convert `config.ConfigDiff` (flat dotted keys) into per-root `[]rpc.ConfigDiffSection` by grouping keys by their top-level root.

### Step 5: Verify
Run `make lint && make test` — all tests pass.

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| reloadConfig() orchestration | | | |
| ReloadFromDisk() entry point | | | |
| No-change detection (empty diff) | | | |
| Verify-all-before-apply semantics | | | |
| Per-root filtering by WantsConfigRoots | | | |
| Concurrent reload rejection | | | |
| SetConfigTree on reactor | | | |
| ConfigDiff → ConfigDiffSection conversion | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestReloadConfigNoChange | | | |
| TestReloadConfigVerifyFails | | | |
| TestReloadConfigVerifyThenApply | | | |
| TestReloadConfigPerRootFiltering | | | |
| TestReloadConfigMultiplePlugins | | | |
| TestReloadConfigConcurrentRejected | | | |
| TestReloadFromDiskParseError | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/server.go` | | |
| `internal/plugin/types.go` | | |
| `internal/plugin/reload.go` | | |
| `internal/plugin/reload_test.go` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Design
- [x] No premature abstraction (coordinator is minimal, no framework)
- [x] No speculative features (only verify→apply, no rollback in v1)
- [x] Single responsibility (coordinator orchestrates, plugins decide)
- [x] Explicit behavior (verify must all pass, apply is sequential)
- [x] Minimal coupling (generic map[string]any, no typed config)
- [x] Next-developer test (follows deliverConfigRPC pattern)

### TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Feature code integrated into codebase
- [ ] Functional tests verify end-user behavior

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes
