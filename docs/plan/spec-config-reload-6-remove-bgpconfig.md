# Spec: config-reload-6-remove-bgpconfig

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/config/bgp.go` — BGPConfig struct, TreeToConfig(), PeerConfig
4. `internal/config/loader.go` — CreateReactorWithDir(), configToPeer()

**Parent spec:** `spec-reload-lifecycle-tests.md` (umbrella)
**Depends on:** `spec-config-reload-3-sighup.md` (reactor VerifyConfig/ApplyConfigDiff must exist)

## Task

Remove the `BGPConfig` typed intermediate struct. The config system should flow: config file → parse → generic `map[string]any` tree → YANG validate → deliver to consumers as `map[string]any`. Each consumer (reactor, CLI, plugins) parses what it needs directly from the generic tree.

Currently the flow is: config file → parse → Tree → `TreeToConfig()` → `BGPConfig` → `CreateReactorWithDir()` → reactor. The `BGPConfig` struct is a BGP-specific typed layer that contradicts the generic YANG-driven config architecture.

What to eliminate:
- `BGPConfig` struct and all sub-types (`PeerConfig`, `PluginConfig`, `CapabilityConfig`, etc.)
- `TreeToConfig()` function (the bridge from generic tree to typed struct)
- `configToPeer()` function (converts `PeerConfig` to reactor `PeerSettings`)
- `CreateReactorWithDir()` dependency on `BGPConfig`

What replaces it:
- Reactor parses peer settings directly from `map[string]any` subtree
- CLI commands walk the `map[string]any` tree for display/validation
- Config loader returns only the generic tree (already stored as `ConfigTree`)

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — reactor config, peer settings
- [ ] `docs/architecture/config/yang-config-design.md` — generic YANG-driven config model

### Source Files (MUST read)
- [ ] `internal/config/bgp.go` — BGPConfig struct (line 116), TreeToConfig() (line 330), PeerConfig (line 128), all sub-types
- [ ] `internal/config/bgp_util.go` — utility functions for BGPConfig types
- [ ] `internal/config/loader.go` — CreateReactorWithDir() (line 376), configToPeer() (line 434), createReloadFunc()
- [ ] `internal/plugin/bgp/reactor/reactor.go` — PeerSettings struct, how reactor receives config
- [ ] `cmd/ze/config/main.go` — printConfig(), cmdDump() — CLI consumers of BGPConfig
- [ ] `cmd/ze/validate/main.go` — semanticValidation() — CLI validation consumer

**Key insights:**
- `BGPConfig` has 100 occurrences across 13 files (4 non-test files are the real consumers)
- `configToPeer()` is 400+ lines converting PeerConfig → PeerSettings — this logic moves into reactor
- `ConfigTree map[string]any` already exists inside BGPConfig — it's the generic tree we want to keep
- `PeerSettings` (reactor's own type) remains — the reactor still needs typed peer data internally
- CLI `printConfig()` can walk the tree directly or use the YANG schema for labels
- `semanticValidation()` moves to reactor's VerifyConfig() (already planned in sub-spec 3)

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/config/bgp.go` — defines BGPConfig with 8 fields, PeerConfig with 25+ fields, 10+ sub-types
- [ ] `internal/config/loader.go` — TreeToConfig() creates BGPConfig, CreateReactorWithDir() consumes it, configToPeer() converts PeerConfig → PeerSettings
- [ ] `cmd/ze/config/main.go` — printConfig() iterates BGPConfig fields for human-readable display
- [ ] `cmd/ze/validate/main.go` — semanticValidation() checks BGPConfig fields for consistency

**Behavior to preserve:**
- Reactor must receive equivalent peer settings (same PeerSettings struct)
- CLI `ze config check` must produce same validation output
- CLI `ze config dump` must produce equivalent display
- Config reload via SIGHUP/coordinator must work identically
- All existing functional tests must pass

**Behavior to change:**
- Config loader returns `map[string]any` tree, not `BGPConfig`
- Reactor parses peer settings from `map[string]any` directly (internal parsing)
- CLI walks `map[string]any` tree for display and validation
- `TreeToConfig()` function eliminated
- `BGPConfig` struct and all sub-types eliminated
- `configToPeer()` logic absorbed into reactor's tree-to-peer parsing

## Data Flow (MANDATORY)

### Entry Point
- Config file parsed into `map[string]any` tree (unchanged)
- Tree delivered to reactor, CLI, plugins (unchanged format)

### Transformation Path
1. Config file → Tree parser → `map[string]any` (existing, unchanged)
2. YANG validation on the tree (existing, unchanged)
3. **Before:** `TreeToConfig()` → `BGPConfig` → `CreateReactorWithDir()` → `configToPeer()` → `PeerSettings`
4. **After:** `map[string]any` tree → reactor startup → reactor internal `parsePeerFromTree()` → `PeerSettings`
5. Plugins receive `map[string]any` subtrees via coordinator (unchanged from sub-specs 1-3)
6. CLI walks `map[string]any` tree directly for display/validation

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config → Reactor | `map[string]any` tree passed at startup | [ ] |
| Config → CLI | `map[string]any` tree for display/validation | [ ] |
| Config → Plugins | JSON sections via coordinator RPC | [ ] |

### Integration Points
- `reactor.Config` — currently stores typed fields (RouterID, LocalAS); will store tree reference instead
- `PeerSettings` — reactor's internal typed struct remains (not exposed to config package)
- `LoadReactorWithConfig()` — entry point changes from returning BGPConfig to returning tree
- `configToPeer()` logic — absorbed into reactor package as internal parsing

### Architectural Verification
- [ ] No bypassed layers (tree → reactor → PeerSettings, no typed intermediate)
- [ ] No unintended coupling (config package no longer knows about BGP peer semantics)
- [ ] No duplicated functionality (parsing moves to reactor, not duplicated)
- [ ] Zero-copy preserved where applicable (N/A)

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestReactorParsePeerFromTree` | `internal/plugin/bgp/reactor/config_test.go` | Reactor parses valid peer from map[string]any | |
| `TestReactorParsePeerFromTreeInvalid` | `internal/plugin/bgp/reactor/config_test.go` | Invalid tree data → clear error | |
| `TestReactorParsePeerCapabilities` | `internal/plugin/bgp/reactor/config_test.go` | Capability parsing from tree (ASN4, GR, AddPath, etc.) | |
| `TestReactorParsePeerStaticRoutes` | `internal/plugin/bgp/reactor/config_test.go` | Static route parsing from tree | |
| `TestReactorParsePeerFamilies` | `internal/plugin/bgp/reactor/config_test.go` | Family config parsing from tree | |
| `TestReactorStartupFromTree` | `internal/plugin/bgp/reactor/config_test.go` | Full reactor startup from map[string]any tree | |
| `TestConfigLoaderReturnsTree` | `internal/config/loader_test.go` | LoadReactor returns map[string]any, not BGPConfig | |
| `TestCLIDumpFromTree` | `cmd/ze/config/main_test.go` | CLI dump displays config from tree correctly | |
| `TestCLIValidateFromTree` | `cmd/ze/validate/main_test.go` | CLI validate works with tree input | |

### Boundary Tests (MANDATORY for numeric inputs)
N/A — peer settings boundary values already tested in reactor. Parsing from tree uses same ranges.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| All existing parse tests | `test/parse/*.ci` | Config parsing still works identically | |
| All existing plugin tests | `test/plugin/*.ci` | Plugin behavior unchanged | |
| All existing reload tests | `test/reload/*.ci` | Reload pipeline works with direct tree parsing | |

## Files to Modify
- `internal/config/loader.go` — remove BGPConfig dependency, return tree directly
- `internal/plugin/bgp/reactor/reactor.go` — add tree-to-peer parsing (absorb configToPeer logic)
- `cmd/ze/config/main.go` — printConfig/cmdDump walk tree instead of BGPConfig
- `cmd/ze/validate/main.go` — semanticValidation works on tree (or delegates to reactor verify)

## Files to Create
- `internal/plugin/bgp/reactor/config.go` — reactor-internal config parsing from map[string]any
- `internal/plugin/bgp/reactor/config_test.go` — tests for tree-to-peer parsing

## Files to Delete
- `internal/config/bgp.go` — BGPConfig struct and all sub-types (after all consumers migrated)
- `internal/config/bgp_util.go` — utility functions for BGPConfig types
- `internal/config/bgp_test.go` — tests for BGPConfig creation

## Implementation Steps

### Step 1: Create reactor config parser with tests
Write `parsePeerFromTree(tree map[string]any) (*PeerSettings, error)` in a new `config.go` file inside the reactor package. This absorbs the logic from `configToPeer()`. Write tests that parse peer settings from map[string]any trees matching the YANG schema structure.

### Step 2: Write reactor startup-from-tree tests
Test that a reactor can start from a `map[string]any` tree without going through BGPConfig. The tree provides router-id, local-as, listen, peers, plugins.

### Step 3: Implement reactor startup from tree
Add a new startup path that accepts `map[string]any` tree and internally parses to PeerSettings. Initially keep both paths (BGPConfig and tree) to allow incremental migration.

### Step 4: Migrate config loader
Change `LoadReactorWithConfig()` to return the tree and use the reactor's tree-based startup path. Remove `TreeToConfig()` call.

### Step 5: Migrate CLI commands
Change `printConfig()` to walk the `map[string]any` tree. Change `semanticValidation()` to either walk the tree directly or delegate to reactor's VerifyConfig().

### Step 6: Delete BGPConfig
Remove `internal/config/bgp.go`, `bgp_util.go`, `bgp_test.go`. Remove all BGPConfig references. Fix compilation errors.

### Step 7: Clean up configToPeer
Remove `configToPeer()` from loader.go (logic now in reactor's config.go). Remove `CreateReactorWithDir()` BGPConfig parameter.

### Step 8: Verify
Run `make lint && make test && make functional` — all tests pass.

## Implementation Audit

<!-- BLOCKING: Complete BEFORE moving spec to done. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Reactor parses peers from map[string]any | | | |
| Config loader returns tree, not BGPConfig | | | |
| CLI dump works from tree | | | |
| CLI validate works from tree | | | |
| BGPConfig struct deleted | | | |
| TreeToConfig() deleted | | | |
| configToPeer() absorbed into reactor | | | |
| bgp.go, bgp_util.go, bgp_test.go deleted | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| TestReactorParsePeerFromTree | | | |
| TestReactorParsePeerFromTreeInvalid | | | |
| TestReactorParsePeerCapabilities | | | |
| TestReactorParsePeerStaticRoutes | | | |
| TestReactorParsePeerFamilies | | | |
| TestReactorStartupFromTree | | | |
| TestConfigLoaderReturnsTree | | | |
| TestCLIDumpFromTree | | | |
| TestCLIValidateFromTree | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/config/loader.go` | | |
| `internal/plugin/bgp/reactor/reactor.go` | | |
| `internal/plugin/bgp/reactor/config.go` | | |
| `internal/plugin/bgp/reactor/config_test.go` | | |
| `cmd/ze/config/main.go` | | |
| `cmd/ze/validate/main.go` | | |
| `internal/config/bgp.go` | | Delete |
| `internal/config/bgp_util.go` | | Delete |
| `internal/config/bgp_test.go` | | Delete |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Design
- [x] No premature abstraction (reactor parses its own config, no generic framework)
- [x] No speculative features (only eliminates BGPConfig, no new capabilities)
- [x] Single responsibility (config package provides tree, reactor interprets it)
- [x] Explicit behavior (parsing errors are clear, same validation as before)
- [x] Minimal coupling (config package no longer knows about BGP peer semantics)
- [x] Next-developer test (reactor owns its config parsing, natural ownership)

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
