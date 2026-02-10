# Spec: reload-lifecycle-tests

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (umbrella — references 5 sub-specs)
2. `.claude/rules/planning.md`
3. `docs/architecture/config/yang-config-design.md` — VyOS-style handler design
4. `docs/plan/done/155-hub-phase4-verify-apply.md` — verify/apply protocol
5. `docs/plan/done/185-config-json-delivery.md` — config JSON delivery + reload

## Task

Build the VyOS-style config reload pipeline: config change → YANG validate → plugin verify → save → plugin apply(diff). The pipeline is **generic** — it works with `map[string]any` config trees driven by YANG schemas, not BGP-specific types. Each plugin (including the BGP reactor) interprets its own config section independently.

This is implemented as 5 ordered sub-specs:

| Order | Spec | Scope |
|-------|------|-------|
| 1 | `spec-config-reload-1-rpc.md` | RPC types for config-verify and config-apply |
| 2 | `spec-config-reload-2-coordinator.md` | Reload coordinator orchestrating verify→apply |
| 3 | `spec-config-reload-3-sighup.md` | SIGHUP → coordinator + reactor refactor |
| 4 | `spec-config-reload-4-editor.md` | Editor commit → save + reload |
| 5 | `spec-config-reload-5-e2e.md` | End-to-end functional tests |
| 6 | `spec-config-reload-6-remove-bgpconfig.md` | Remove BGPConfig typed intermediate, reactor parses from tree |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — system components, plugin protocol
- [ ] `docs/architecture/config/yang-config-design.md` — VyOS handler interface, Diff struct, commit flow
- [ ] `docs/architecture/config/vyos-research.md` — VyOS architecture analysis

### Existing Specs (completed)
- [ ] `docs/plan/done/155-hub-phase4-verify-apply.md` — two-phase verify/apply protocol
- [ ] `docs/plan/done/185-config-json-delivery.md` — config JSON delivery, WantsConfigRoots, reload format
- [ ] `docs/plan/spec-signal-command.md` — PID file, ze signal, VyOS-style reload with per-peer diff

**Key insights:**
- The pipeline is generic: coordinator works with `map[string]any`, never touches `BGPConfig`
- Plugin SDK (`pkg/plugin/plugin.go`) has verify/apply handlers that are never invoked by the engine
- `DiffMaps()` in `internal/config/diff.go` is ready to use for computing config deltas
- Plugins declare `WantsConfigRoots` — coordinator only sends verify/apply to relevant plugins
- Reactor is called directly by coordinator (not via RPC) since it IS the engine

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/rpc/types.go` — RPC types for 5-stage protocol, no config-verify/apply types
- [ ] `internal/plugin/rpc_plugin.go` — Send methods for all stages + runtime RPCs, no config reload
- [ ] `pkg/plugin/sdk/sdk.go` — SDK dispatch for deliver-event, encode/decode, execute-command, bye
- [ ] `internal/plugin/server.go` — deliverConfigRPC() at Stage 2, no reload notification
- [ ] `internal/plugin/bgp/reactor/reactor.go` — Reload() reads config directly, bypasses plugins
- [ ] `internal/config/diff.go` — DiffMaps() computes Added/Removed/Changed on map[string]any
- [ ] `internal/config/editor/model_commands.go` — cmdCommit() validates YANG + saves, no plugin notify

**Behavior to preserve:**
- Existing 5-stage plugin protocol must work unchanged
- Existing config delivery at Stage 2 must work unchanged
- Existing `.ci` tests must not be affected
- `DiffMaps()` API must not change

**Behavior to change:**
- SIGHUP triggers coordinator instead of reactor.Reload() directly
- Editor commit triggers reload after save
- Plugins receive config-verify and config-apply RPCs on reload
- Reactor participates in verify/apply via direct function calls

## Data Flow (MANDATORY)

### Entry Point
- **SIGHUP:** OS signal received by SignalHandler, triggers ReloadFromDisk()
- **Editor commit:** cmdCommit() in model_commands.go, triggers coordinator after save

### Transformation Path
1. Parse new config file → `map[string]any` tree (generic, YANG-driven)
2. YANG validate the new tree
3. `DiffMaps(running, new)` → `ConfigDiff{Added, Removed, Changed}`
4. If diff empty → no-op return
5. Build per-root sections from diff for each plugin's `WantsConfigRoots`
6. **Verify phase:** Send `config-verify` RPC to each affected plugin + call reactor verify
7. If ANY verify fails → abort, return error, keep running config
8. **Apply phase:** Send `config-apply` RPC with diff to each plugin + call reactor apply
9. Reactor apply: convert `map[string]any` → peer settings, diff peers, add/remove
10. Update running config reference

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine ↔ Plugin | `config-verify` / `config-apply` RPCs via Socket B | [ ] |
| Coordinator ↔ Reactor | Direct function call (VerifyConfig, ApplyConfigDiff) | [ ] |
| Signal ↔ Coordinator | OnReload callback → ReloadFromDisk() | [ ] |
| Editor ↔ Engine | Save to disk + reload command | [ ] |

### Integration Points
- `deliverConfigRPC()` in server.go — pattern for sending per-root config to plugins
- `extractConfigSubtree()` in server.go — extracts per-root sections from tree
- `DiffMaps()` in diff.go — computes config deltas
- `GetConfigTree()` on reactor — returns running config as `map[string]any`
- `WantsConfigRoots` in DeclareRegistrationInput — plugins declare interest

### Architectural Verification
- [ ] No bypassed layers (all config changes go through coordinator)
- [ ] No unintended coupling (coordinator uses generic trees, not BGPConfig)
- [ ] No duplicated functionality (extends existing RPC protocol)
- [ ] Zero-copy preserved where applicable (N/A — config is JSON, not wire bytes)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| See sub-specs 1-5 | Various | Each sub-spec has its own TDD plan | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — no new numeric inputs in the pipeline itself.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `reload-add-peer` | `test/reload/add-peer.ci` | Add peer via config rewrite + SIGHUP | |
| `reload-remove-peer` | `test/reload/remove-peer.ci` | Remove peer, verify session drops | |
| `reload-no-change` | `test/reload/no-change.ci` | SIGHUP with unchanged config, no disruption | |
| `reload-verify-reject` | `test/reload/verify-reject.ci` | Plugin rejects config, reload aborts | |

## Files to Modify
- `pkg/plugin/rpc/types.go` — add config-verify/apply RPC types
- `internal/plugin/rpc_plugin.go` — add SendConfigVerify/SendConfigApply
- `pkg/plugin/sdk/sdk.go` — add config-verify/apply dispatch + OnConfig handlers
- `internal/plugin/server.go` — wire coordinator
- `internal/plugin/bgp/reactor/reactor.go` — split Reload() into VerifyConfig/ApplyConfigDiff
- `internal/config/editor/model_commands.go` — commit triggers reload

## Files to Create
- `internal/plugin/reload.go` — reload coordinator
- `internal/plugin/reload_test.go` — coordinator tests
- `test/reload/add-peer.ci` — functional test
- `test/reload/remove-peer.ci` — functional test
- `test/reload/no-change.ci` — functional test
- `test/reload/verify-reject.ci` — functional test

## Implementation Steps

### Step 1: Implement sub-spec 1 (RPC Protocol)
Implement `spec-config-reload-1-rpc.md` — RPC types, Send methods, SDK handlers.

### Step 2: Implement sub-spec 2 (Coordinator)
Implement `spec-config-reload-2-coordinator.md` — reload coordinator.

### Step 3: Implement sub-spec 3 (SIGHUP)
Implement `spec-config-reload-3-sighup.md` — SIGHUP integration + reactor refactor.

### Step 4: Implement sub-spec 4 (Editor)
Implement `spec-config-reload-4-editor.md` — editor commit integration.

### Step 5: Implement sub-spec 5 (E2E Tests)
Implement `spec-config-reload-5-e2e.md` — end-to-end functional tests.

### Step 6: Implement sub-spec 6 (Remove BGPConfig)
Implement `spec-config-reload-6-remove-bgpconfig.md` — eliminate BGPConfig typed intermediate, reactor parses from generic tree.

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| config-verify RPC type | | | Sub-spec 1 |
| config-apply RPC type | | | Sub-spec 1 |
| SDK dispatch for config-verify/apply | | | Sub-spec 1 |
| Reload coordinator | | | Sub-spec 2 |
| SIGHUP → coordinator | | | Sub-spec 3 |
| Reactor VerifyConfig/ApplyConfigDiff | | | Sub-spec 3 |
| Editor commit triggers reload | | | Sub-spec 4 |
| Functional tests | | | Sub-spec 5 |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| See individual sub-spec audits | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| See individual sub-spec audits | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Design
- [x] No premature abstraction (generic pipeline, not BGP-specific)
- [x] No speculative features (only what's needed for reload)
- [x] Single responsibility (coordinator orchestrates, plugins verify/apply independently)
- [x] Explicit behavior (verify must pass before apply, clear error messages)
- [x] Minimal coupling (coordinator uses map[string]any, plugins interpret own sections)
- [x] Next-developer test (follows existing RPC/SDK patterns exactly)

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
