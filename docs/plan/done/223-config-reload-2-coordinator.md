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
| reloadConfig() orchestration | ✅ Done | `reload.go:48` | Verify→apply with diff computation |
| ReloadFromDisk() entry point | ✅ Done | `reload.go:26` | Calls configLoader then reloadConfig |
| No-change detection (empty diff) | ✅ Done | `reload.go:64` | Returns nil when diff is empty |
| Verify-all-before-apply semantics | ✅ Done | `reload.go:115-133` | Collects all verify errors before checking |
| Per-root filtering by WantsConfigRoots | ✅ Done | `reload.go:86-99` | rootHasChanges filters per-root |
| Concurrent reload rejection | ✅ Done | `reload.go:50-52` | TryLock on reloadMu |
| SetConfigTree on reactor | ✅ Done | `types.go:453`, `reactor.go:339` | Interface + implementation |
| ConfigDiff → ConfigDiffSection conversion | ✅ Done | `reload.go:281-329` | buildDiffSections groups by root |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestReloadConfigNoChange | ✅ Done | `reload_test.go:169` | |
| TestReloadConfigVerifyFails | ✅ Done | `reload_test.go:199` | |
| TestReloadConfigVerifyThenApply | ✅ Done | `reload_test.go:235` | |
| TestReloadConfigPerRootFiltering | ✅ Done | `reload_test.go:268` | |
| TestReloadConfigMultiplePlugins | ✅ Done | `reload_test.go:305` | |
| TestReloadConfigConcurrentRejected | ✅ Done | `reload_test.go:343` | |
| TestReloadFromDiskParseError | ✅ Done | `reload_test.go:369` | |

### Extra Tests (beyond spec)
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestReloadFromDiskNoLoader | ✅ Done | `reload_test.go:396` | Missing loader → error |
| TestDiffMapsLocal | ✅ Done | `reload_test.go:411` | Local diff logic correctness |
| TestRootHasChanges | ✅ Done | `reload_test.go:446` | Root path matching |
| TestReloadConfigRootRemoved | ✅ Done | `reload_test.go:466` | Root removal → plugin still notified |
| TestReloadConfigWildcardRoot | ✅ Done | `reload_test.go:506` | Wildcard root → verify + apply |
| TestDiffPairJSONKeys | ✅ Done | `reload_test.go:543` | JSON keys are kebab-case |
| TestBuildDiffSections | ✅ Done | `reload_test.go:561` | Per-root grouping |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/server.go` | ✅ Modified | configLoader/reloadMu fields + deliverConfigRPC marshal error logging |
| `internal/plugin/types.go` | ✅ Modified | Added SetConfigTree to ReactorInterface |
| `internal/plugin/reload.go` | ✅ Created | Full coordinator with logging and edge-case handling |
| `internal/plugin/reload_test.go` | ✅ Created | 14 tests with mock plugin responders |

### Additional Files Modified
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/bgp/reactor/reactor.go` | ✅ Modified | SetConfigTree implementation |
| `internal/plugin/handler_test.go` | ✅ Modified | SetConfigTree stub on mockReactor |
| `internal/plugin/refresh_test.go` | ✅ Modified | SetConfigTree stub on mockReactorRefresh |
| `internal/plugin/update_text_test.go` | ✅ Modified | SetConfigTree stub on mockReactorBatch |

### Audit Summary
- **Total items:** 22
- **Done:** 22
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 0

## Checklist

### Design
- [x] No premature abstraction (coordinator is minimal, no framework)
- [x] No speculative features (only verify→apply, no rollback in v1)
- [x] Single responsibility (coordinator orchestrates, plugins decide)
- [x] Explicit behavior (verify must all pass, apply is sequential)
- [x] Minimal coupling (generic map[string]any, no typed config)
- [x] Next-developer test (follows deliverConfigRPC pattern)

### TDD
- [x] Tests written (14 tests in reload_test.go)
- [x] Tests FAIL (verified RED before implementation)
- [x] Implementation complete (reload.go)
- [x] Tests PASS (all 14 pass)
- [x] Feature code integrated into codebase (server.go, types.go, reactor.go)
- [x] Functional tests verify end-user behavior (N/A — coordinator is internal, functional tests in sub-spec 5)

### Verification
- [x] `make lint` passes (0 issues)
- [x] `make test` passes (all unit tests pass)
- [x] `make functional` passes (93/93 editor, 38/42 BGP — 4 pre-existing FlowSpec/BGP-LS timeouts unrelated to reload)

## Implementation Summary

### What Was Implemented
- `reload.go`: Full config reload coordinator with verify→apply protocol
  - `ConfigLoader` type and `SetConfigLoader()` for disk-based reload
  - `ReloadFromDisk()` entry point (parses config, calls reloadConfig)
  - `ReloadConfig()` public API / `reloadConfig()` internal implementation
  - Local `diffMaps()` (equivalent to `config.DiffMaps()`, avoids import cycle)
  - `rootHasChanges()` for per-root diff filtering
  - `buildDiffSections()` for ConfigDiff → per-root ConfigDiffSection conversion
  - `topLevelRoot()` helper for key grouping
- `types.go`: Added `SetConfigTree(map[string]any)` to `ReactorInterface`
- `reactor.go`: Implemented `SetConfigTree` on `reactorAPIAdapter` (mutex-protected)
- `server.go`: Added `configLoader ConfigLoader` and `reloadMu sync.Mutex` fields
- Mock reactor stubs added to 3 existing test files

### Bugs Found/Fixed
- **Root removal silent skip:** When a config root was entirely removed from the new config, `extractConfigSubtree` returned nil and the plugin was silently skipped. Fixed by sending `"{}"` section for removed roots. Regression test: `TestReloadConfigRootRemoved`.
- **Wildcard apply filter:** Plugins with `WantsConfigRoots: ["*"]` passed verify but never received apply, because `slices.Contains(["*"], "bgp")` is false. Fixed by checking `wantsAll` flag. Regression test: `TestReloadConfigWildcardRoot`.
- **PascalCase JSON keys:** `diffPair` struct had no JSON tags, producing `"Old"/"New"` instead of kebab-case `"old"/"new"`. Fixed with `json:"old"` / `json:"new"` tags. Regression test: `TestDiffPairJSONKeys`.
- **Silent marshal errors:** `json.Marshal` failures in verify section building, `buildDiffSections`, and `deliverConfigRPC` were silently dropped. Fixed with `logger().Error` calls including root/plugin context.
- **No reload logging:** `reloadConfig()` had no logging. Added Info/Debug/Warn at: start, no-change, diff stats, no-affected, verify phase, verify failed, apply phase, completed.

### Design Insights
- Local `diffMaps` duplication was necessary to avoid import cycle (internal/plugin cannot import internal/config, confirmed via `go list -json`). The implementation is identical to `config.DiffMaps`.
- The coordinator uses `TryLock()` (Go 1.18+) for non-blocking concurrent reload rejection, which is cleaner than a channel-based semaphore.
- Edge cases (root removal, wildcard) required explicit handling because the happy path (concrete roots with values present) masks absence/wildcard semantics.

### Documentation Updates
- None — no architectural changes

### Deviations from Plan
- Added `ReloadConfig()` as a public method (not in original spec) — needed for editor integration (sub-spec 4)
- Added 7 extra tests beyond the spec's TDD plan for internal helpers and critical review bug fixes
- Added logging throughout `reloadConfig()` and error logging for marshal failures (not in original spec)
