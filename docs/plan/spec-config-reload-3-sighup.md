# Spec: config-reload-3-sighup

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/plugin/bgp/reactor/reactor.go` — Reload(), peerSettingsEqual(), signal wiring
4. `internal/plugin/bgp/reactor/signal.go` — SignalHandler, OnReload callback

**Parent spec:** `spec-reload-lifecycle-tests.md` (umbrella)
**Depends on:** `spec-config-reload-2-coordinator.md` (coordinator must exist)

## Task

Wire SIGHUP signal to the reload coordinator instead of reactor.Reload() directly. Refactor reactor.Reload() into separate VerifyConfig() and ApplyConfigDiff() methods that the coordinator can call independently.

Two changes:
1. **Reactor refactor:** Split `Reload()` into `VerifyConfig(bgpTree)` and `ApplyConfigDiff(bgpTree)` — verify checks peer settings are valid without modifying state, apply computes peer diff and executes add/remove
2. **Signal wiring:** SIGHUP callback calls `server.ReloadFromDisk()` instead of `adapter.Reload()`

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — reactor role, peer management
- [ ] `docs/architecture/config/yang-config-design.md` — handler verify/apply pattern

### Source Files (MUST read)
- [ ] `internal/plugin/bgp/reactor/reactor.go` — Reload() at line 973, peerSettingsEqual() at line 1062, AddPeer(), RemovePeer()
- [ ] `internal/plugin/bgp/reactor/signal.go` — SignalHandler struct, OnReload() registration
- [ ] `internal/plugin/bgp/reactor/reload_test.go` — existing 8 reload tests
- [ ] `internal/plugin/server.go` — Server struct (coordinator lives here after sub-spec 2)

**Key insights:**
- Reload() does: parse config → build new peer map → diff → remove old → add new
- VerifyConfig needs to: parse peer settings from map[string]any, validate (addresses, ASNs, no duplicates)
- ApplyConfigDiff needs to: same peer diff logic as Reload() but receives map[string]any not file path
- SIGHUP handler at reactor.go line 4173 creates reactorAPIAdapter and calls Reload()
- Existing reload tests use `simpleReloadFunc` — these must keep working
- Keep backward-compatible `Reload()` wrapper that calls verify then apply

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/bgp/reactor/reactor.go` — Reload() reads config file via reloadFunc, diffs peers by address, stops/adds
- [ ] `internal/plugin/bgp/reactor/signal.go` — SIGHUP → OnReload callback → adapter.Reload()
- [ ] `internal/plugin/bgp/reactor/reload_test.go` — 8 tests: add/remove/change/error/noop scenarios

**Behavior to preserve:**
- Existing 8 reload tests must pass unchanged
- `simpleReloadFunc` test helper must still work
- `peerSettingsEqual()` comparison logic unchanged
- Backward-compatible `Reload()` wrapper for any other callers

**Behavior to change:**
- SIGHUP handler calls coordinator instead of reactor.Reload()
- New `VerifyConfig()` method validates peer settings without modifying state
- New `ApplyConfigDiff()` method receives config tree, computes peer diff, executes changes
- Reactor implements a `ConfigHandler` interface (or equivalent) for coordinator to call

## Data Flow (MANDATORY)

### Entry Point
- **Before:** SIGHUP → SignalHandler → adapter.Reload() → reloadFunc(configPath) → diff peers
- **After:** SIGHUP → SignalHandler → server.ReloadFromDisk() → coordinator → reactor.VerifyConfig() + reactor.ApplyConfigDiff()

### Transformation Path
1. SIGHUP signal received by SignalHandler
2. OnReload callback fires → calls `server.ReloadFromDisk()`
3. Coordinator parses config, computes diff, sends verify RPCs to plugins
4. Coordinator calls `reactor.VerifyConfig(bgpTree)`:
   - Extracts peer settings from `map[string]any` tree
   - Validates: addresses parseable, ASNs non-zero, no duplicate addresses
   - Returns error if invalid, nil if OK
5. If all verify pass, coordinator sends apply RPCs
6. Coordinator calls `reactor.ApplyConfigDiff(bgpTree)`:
   - Extracts peer settings from tree (same as verify)
   - Builds new peer map by address
   - Diffs against current peers: categorize as added/removed/changed
   - Stops removed/changed peers
   - Adds new/changed peers
7. Coordinator updates running config reference

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Signal → Coordinator | OnReload callback → server.ReloadFromDisk() | [ ] |
| Coordinator → Reactor | Direct function call (VerifyConfig, ApplyConfigDiff) | [ ] |
| Reactor → Peers | peer.Stop(), r.AddPeer() for changes | [ ] |

### Integration Points
- `SignalHandler.OnReload()` — change callback target
- `reactorAPIAdapter` — add VerifyConfig/ApplyConfigDiff methods
- `ReactorInterface` — add VerifyConfig/ApplyConfigDiff to interface
- `Server` — coordinator methods from sub-spec 2

### Architectural Verification
- [ ] No bypassed layers (SIGHUP goes through coordinator, not direct reload)
- [ ] No unintended coupling (reactor receives map[string]any, converts internally)
- [ ] No duplicated functionality (verify/apply reuse existing peer diff logic)
- [ ] Zero-copy preserved where applicable (N/A)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReactorVerifyConfigValid` | `internal/plugin/bgp/reactor/reload_test.go` | Valid peer settings pass verification | |
| `TestReactorVerifyConfigInvalidAddress` | `internal/plugin/bgp/reactor/reload_test.go` | Invalid address → error | |
| `TestReactorVerifyConfigDuplicateAddress` | `internal/plugin/bgp/reactor/reload_test.go` | Duplicate peer address → error | |
| `TestReactorVerifyConfigNoMutation` | `internal/plugin/bgp/reactor/reload_test.go` | Verify does not modify reactor state | |
| `TestReactorApplyConfigDiffAddPeer` | `internal/plugin/bgp/reactor/reload_test.go` | New peer in tree → added to reactor | |
| `TestReactorApplyConfigDiffRemovePeer` | `internal/plugin/bgp/reactor/reload_test.go` | Peer missing from tree → removed | |
| `TestReactorApplyConfigDiffChangedPeer` | `internal/plugin/bgp/reactor/reload_test.go` | Changed settings → peer restarted | |
| `TestReactorReloadBackwardCompat` | `internal/plugin/bgp/reactor/reload_test.go` | Existing Reload() still works via wrapper | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — peer settings validation reuses existing parsing; no new numeric ranges.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `reload-add-peer` | `test/reload/add-peer.ci` | Config adds peer, SIGHUP, new BGP session established | |
| `reload-remove-peer` | `test/reload/remove-peer.ci` | Config removes peer, SIGHUP, session drops with NOTIFICATION | |
| `reload-no-change` | `test/reload/no-change.ci` | SIGHUP with same config, no sessions disrupted | |

## Files to Modify
- `internal/plugin/bgp/reactor/reactor.go` — add VerifyConfig(), ApplyConfigDiff(), keep Reload() as wrapper
- `internal/plugin/types.go` — add VerifyConfig/ApplyConfigDiff to ReactorInterface
- `internal/plugin/server.go` — wire coordinator to call reactor verify/apply (if not in reload.go)

## Files to Create
- `test/reload/add-peer.ci` — functional test: add peer via SIGHUP
- `test/reload/remove-peer.ci` — functional test: remove peer via SIGHUP
- `test/reload/no-change.ci` — functional test: no-op SIGHUP

## Implementation Steps

### Step 1: Write reactor verify/apply tests
Test VerifyConfig with valid/invalid/duplicate peers. Test ApplyConfigDiff with add/remove/change scenarios. Test that verify does NOT mutate state.

### Step 2: Implement VerifyConfig
Extract peer settings parsing from map[string]any tree. Validate addresses, ASNs, uniqueness. Return error on invalid config.

### Step 3: Implement ApplyConfigDiff
Reuse existing peer diff logic from Reload(). Accept map[string]any tree instead of file path. Compute peer changes, execute add/remove.

### Step 4: Keep Reload() as backward-compatible wrapper
Reload() calls reloadFunc to get peer settings, then delegates to VerifyConfig + ApplyConfigDiff.

### Step 5: Wire SIGHUP to coordinator
Change OnReload callback from adapter.Reload() to server.ReloadFromDisk(). The server holds the coordinator from sub-spec 2.

### Step 6: Update ReactorInterface
Add VerifyConfig and ApplyConfigDiff methods to the interface. Update mock implementations in tests.

### Step 7: Write functional tests
Create .ci files for add-peer, remove-peer, no-change scenarios.

### Step 8: Verify
Run `make lint && make test && make functional` — all tests pass.

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| VerifyConfig() method | | | |
| ApplyConfigDiff() method | | | |
| Reload() backward-compatible wrapper | | | |
| SIGHUP → coordinator wiring | | | |
| ReactorInterface updated | | | |
| Functional test: add-peer | | | |
| Functional test: remove-peer | | | |
| Functional test: no-change | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestReactorVerifyConfigValid | | | |
| TestReactorVerifyConfigInvalidAddress | | | |
| TestReactorVerifyConfigDuplicateAddress | | | |
| TestReactorVerifyConfigNoMutation | | | |
| TestReactorApplyConfigDiffAddPeer | | | |
| TestReactorApplyConfigDiffRemovePeer | | | |
| TestReactorApplyConfigDiffChangedPeer | | | |
| TestReactorReloadBackwardCompat | | | |
| reload-add-peer.ci | | | |
| reload-remove-peer.ci | | | |
| reload-no-change.ci | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugin/bgp/reactor/reactor.go` | | |
| `internal/plugin/types.go` | | |
| `internal/plugin/server.go` | | |
| `test/reload/add-peer.ci` | | |
| `test/reload/remove-peer.ci` | | |
| `test/reload/no-change.ci` | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Design
- [x] No premature abstraction (verify/apply split follows VyOS handler pattern)
- [x] No speculative features (no rollback in v1, just verify→apply)
- [x] Single responsibility (verify validates, apply executes, coordinator orchestrates)
- [x] Explicit behavior (verify returns error, apply returns error, both are clear)
- [x] Minimal coupling (reactor receives map[string]any, converts internally)
- [x] Next-developer test (split follows existing Reload() logic)

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
