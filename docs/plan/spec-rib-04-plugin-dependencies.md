# Spec: rib-04 — Plugin Dependency Declarations

**Status:** Ready for implementation.

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `internal/plugin/registry/registry.go` — Registration struct, Register()
3. `pkg/plugin/rpc/types.go` — DeclareRegistrationInput
4. `internal/config/loader.go` — LoadReactor*, plugin loading paths
5. `internal/plugins/bgp-rr/server.go` — replayDisabled (to remove)

## Task

Two-layer plugin dependency declaration system:

**1. Go registry `Dependencies` field** — enables auto-loading of required plugins before startup. When bgp-rr is loaded, ze automatically loads bgp-adj-rib-in.

**2. Protocol `dependencies` field in stage 1** `declare-registration` — enables runtime validation that all declared dependencies are present (all plugins, including future external ones).

**Root cause:** bgp-rr depends on bgp-adj-rib-in for route replay (`DispatchCommand("adj-rib-in replay ...")`). When bgp-adj-rib-in is not loaded, replay silently fails — `replayDisabled` is permanently set, and late-connecting peers miss all prior routes.

**Depends on:** None (prerequisite for spec-rib-05)
**Part of series:** rib-01 → rib-02 → rib-03 → rib-04 (this) → rib-05

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — plugin architecture, 5-stage protocol
  → Constraint: Dependencies validated at stage 1 (declare-registration)
  → Decision: Auto-loaded deps get Internal: true
- [ ] `.claude/rules/plugin-design.md` — Registration fields, 5-stage protocol
  → Constraint: Registration struct field additions follow existing pattern

**Key insights:**
- (to be completed during research phase)

## Current Behavior (MANDATORY)

**Source files read:** (must complete before implementation)
- [ ] `internal/plugin/registry/registry.go` — Registration struct (line 32-73), Register() (line 97-123), error sentinels (line 75-86)
- [ ] `pkg/plugin/rpc/types.go` — DeclareRegistrationInput (line 19-27)
- [ ] `internal/plugin/subsystem.go` — Stage 1 handler (line 114-148)
- [ ] `internal/config/loader.go` — LoadReactor() (line 88), LoadReactorWithPlugins() (line 104), LoadReactorFileWithPlugins() (line 169), mergeCliPlugins() (line 247)
- [ ] `internal/plugins/bgp-rr/register.go` — Registration struct literal (line 12-33)
- [ ] `internal/plugins/bgp-rr/server.go` — replayDisabled atomic (line 141-143), skip block (line 749-756), permanent-disable (line 772-775), replayForPeer() (line 744)

**Behavior to preserve:**
- All existing plugin registration and loading
- 5-stage startup protocol
- bgp-rr replay mechanism (replayForPeer)
- 5-retry loop for transient startup races in replay

**Behavior to change:**
- Add Dependencies field to Registration struct
- Add dependencies field to protocol stage 1
- Auto-load dependency plugins in all 3 loading paths
- Remove replayDisabled silent degradation (fail loudly instead)

## Data Flow (MANDATORY)

### Entry Point
- Plugin Registration via `init()` in `register.go` (Go registry)
- Config loading via `LoadReactor*()` functions (auto-expansion)
- Plugin startup via stage 1 `declare-registration` (runtime validation)

### Transformation Path
1. Plugin calls `registry.Register()` with Dependencies field in `init()`
2. Config loader calls `expandDependencies()` on plugin list — adds missing deps
3. Engine starts each plugin, receives stage 1 declare-registration
4. Engine validates each declared dependency is in running plugin set
5. If dependency missing at stage 1, engine rejects plugin startup

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin init() → Registry | `Register()` stores Dependencies on Registration | [ ] |
| Config → Loader | `expandDependencies()` consults registry, adds missing plugins | [ ] |
| Plugin → Engine (stage 1) | `DeclareRegistrationInput.Dependencies` JSON field | [ ] |
| Engine stage 1 validation | Check each dep in running plugin set | [ ] |

### Integration Points
- `registry.Register()` — add Dependencies validation (no empty, no self-dep)
- `registry.ResolveDependencies()` — iterative expansion with cycle detection
- `expandDependencies()` — new function in loader, wired into 3 loading paths
- `subsystem.go` stage 1 — dependency presence check
- `bgp-rr/register.go` — first consumer of Dependencies field

### Architectural Verification
- [ ] No bypassed layers
- [ ] No unintended coupling
- [ ] No duplicated functionality
- [ ] Zero-copy preserved where applicable

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| Two-layer deps (registry + protocol) | Registry for pre-startup auto-loading (Go only). Protocol for runtime validation (all plugins, including future external). |
| Iterative loop-until-stable expansion | Handles transitive deps naturally. Simpler than topological sort. |
| Auto-added deps are Internal: true | Go plugins registered via init(), always in-process. |
| `expandDependencies()` in all 3 loader paths | Production, test, and config-file loading all benefit. |
| Fail loudly on missing dep at stage 1 | Silent degradation (replayDisabled) caused the original bug. |
| Remove replayDisabled | Dead code after auto-loading guarantees adj-rib-in is present. |

## Changes

### 1. Registration struct — `internal/plugin/registry/registry.go`

**Add field** (after line 43, in Optional metadata section):

| Field | Type | Purpose |
|-------|------|---------|
| `Dependencies` | `[]string` | Plugin names that must also be loaded |

**Add error sentinels** (after line 85):

| Sentinel | When |
|----------|------|
| `ErrCircularDependency` | A → B → A (detected at resolution time) |
| `ErrMissingDependency` | Known plugin declares dep on unknown name |

**Add validation in `Register()`** (after family validation, line 118): reject empty dependency names and self-dependencies.

**Add `ResolveDependencies(requested []string) ([]string, error)`**: iterative loop-until-stable expansion. For each plugin in the list, look up Dependencies from registry and add any missing ones. Repeat until no new deps are added. Track visited set for cycle detection. Plugins not in registry (external) are skipped — their deps come from protocol layer instead. Returns error on circular deps or when a registered plugin declares a dep on an unregistered name.

### 2. Protocol handshake — `pkg/plugin/rpc/types.go`

**Add field** to `DeclareRegistrationInput` (line 19):

| Field | JSON | Purpose |
|-------|------|---------|
| `Dependencies` | `"dependencies"` | Plugin names this plugin requires |

### 3. Engine-side validation — `internal/plugin/subsystem.go`

At stage 1 handler (line 124, after parsing `regInput`): extract `regInput.Dependencies`. Validate each declared dependency is in the running plugin set. If a dependency is missing, send error response and return error (plugin cannot start without its deps).

### 4. SDK declaration — `pkg/plugin/sdk/sdk.go`

Update SDK `Registration` (alias of `DeclareRegistrationInput`) — no code change needed since it's a type alias. But update the SDK `run()` method to populate `Dependencies` from the plugin's config if provided.

### 5. bgp-rr registration — `internal/plugins/bgp-rr/register.go`

Add `Dependencies: []string{"bgp-adj-rib-in"}` in the struct literal.

### 6. Dependency expansion in loader — `internal/config/loader.go`

**New function** `expandDependencies(plugins []reactor.PluginConfig) ([]reactor.PluginConfig, error)`:
1. Collect names from current plugin list into a set
2. Call `registry.ResolveDependencies(names)`
3. For each new name not already in list, append `reactor.PluginConfig` with `Internal: true`, `Encoder: "json"`
4. Log each auto-added plugin via slog.Info

Auto-added dependencies are always `Internal: true` — they're Go plugins registered via `init()`, always in-process.

**Wire into all 3 loading paths** (after plugin list finalized, before CreateReactorFromTree):
- `LoadReactor()` — after line 96
- `LoadReactorWithPlugins()` — after line 124
- `LoadReactorFileWithPlugins()` — after line 222

Works for config-file plugins too — `expandDependencies` takes the plugin config slice regardless of whether plugins came from CLI args or config `plugin { external ... }` blocks.

### 7. Remove `replayDisabled` from bgp-rr — `internal/plugins/bgp-rr/server.go`

With auto-dependency loading, `adj-rib-in` is guaranteed present when `bgp-rr` runs. Remove `replayDisabled` field (line 141-143), skip block (line 749-756), permanent-disable logic (line 772-775). Keep 5-retry loop for transient startup races. After 5 retries fail, log as error (not silently degrade).

### 8. Documentation — `.claude/rules/plugin-design.md`

Add `Dependencies` row to the Registration Fields table.

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `LoadReactorWithPlugins(cfg, "-", ["ze.bgp-rr"])` | → | `expandDependencies()` auto-adds bgp-adj-rib-in | `TestExpandDependencies_Integration` |
| Plugin sends declare-registration with dependencies | → | `subsystem.go` stage 1 rejects missing dep | `TestStage1_DependencyValidation` |
| bgp-rr peer connect triggers replay | → | `replayForPeer()` dispatches to adj-rib-in | `TestHandleStateUpReplay` (existing) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `LoadReactorWithPlugins` with `["ze.bgp-rr"]` | Plugin list contains both bgp-rr and bgp-adj-rib-in |
| AC-2 | Plugin declares dep on missing plugin at stage 1 | Engine rejects with error |
| AC-3 | Plugin declares dep on present plugin at stage 1 | Startup proceeds normally |
| AC-4 | A depends on B depends on C, only A requested | All three in expanded list |
| AC-5 | Circular dependency A→B→A | `ResolveDependencies` returns `ErrCircularDependency` |
| AC-6 | `replayDisabled` field | Does not exist in bgp-rr server struct |

## 🧪 TDD Test Plan

### Unit Tests — Registry
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRegisterWithDependencies` | `internal/plugin/registry/registry_test.go` | Field stored, retrievable via Lookup | |
| `TestRegisterSelfDependency` | `internal/plugin/registry/registry_test.go` | Self-ref rejected | |
| `TestRegisterEmptyDependencyName` | `internal/plugin/registry/registry_test.go` | Empty string rejected | |
| `TestResolveDependencies_NoDeps` | `internal/plugin/registry/registry_test.go` | Returns same list unchanged | |
| `TestResolveDependencies_DirectDep` | `internal/plugin/registry/registry_test.go` | A depends on B: both in result | |
| `TestResolveDependencies_TransitiveDep` | `internal/plugin/registry/registry_test.go` | A→B→C: all three in result | |
| `TestResolveDependencies_AlreadyPresent` | `internal/plugin/registry/registry_test.go` | Both requested, no duplicate | |
| `TestResolveDependencies_CircularDep` | `internal/plugin/registry/registry_test.go` | A→B→A: returns ErrCircularDependency | |
| `TestResolveDependencies_MissingDep` | `internal/plugin/registry/registry_test.go` | A depends on unknown: returns ErrMissingDependency | |
| `TestResolveDependencies_Diamond` | `internal/plugin/registry/registry_test.go` | A→C, B→C: C appears once | |

### Unit Tests — Loader
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestExpandDependencies` | `internal/config/loader_test.go` | Dependency auto-added as Internal=true, Encoder="json" | |
| `TestExpandDependencies_NoDuplicate` | `internal/config/loader_test.go` | Already-present dep not duplicated | |
| `TestExpandDependencies_Integration` | `internal/config/loader_test.go` | LoadReactorWithPlugins with `["ze.bgp-rr"]` produces list with both | |

### Unit Tests — Protocol
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestStage1_DependencyValidation` | `internal/plugin/subsystem_test.go` | Plugin declaring dep on missing plugin gets error | |
| `TestStage1_DependencySatisfied` | `internal/plugin/subsystem_test.go` | Plugin declaring dep on loaded plugin passes | |

### Unit Tests — Integration
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBgpRRDependsOnAdjRibIn` | `internal/plugin/all/all_test.go` | bgp-rr has Dependencies containing "bgp-adj-rib-in" | |

### Boundary Tests (MANDATORY for numeric inputs)

No new numeric fields.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|

## Files to Modify

- `internal/plugin/registry/registry.go` — Add Dependencies field, Register() validation, ResolveDependencies()
- `internal/plugin/registry/registry_test.go` — 10 new tests
- `pkg/plugin/rpc/types.go` — Add Dependencies field to DeclareRegistrationInput
- `internal/plugin/subsystem.go` — Stage 1 dependency validation
- `internal/plugin/subsystem_test.go` — 2 new tests
- `internal/plugins/bgp-rr/register.go` — Add Dependencies declaration
- `internal/plugins/bgp-rr/server.go` — Remove replayDisabled field + all references
- `internal/plugins/bgp-rr/server_test.go` — Update tests for replayDisabled removal
- `internal/config/loader.go` — Add expandDependencies(), wire into 3 loading paths
- `internal/config/loader_test.go` — 3 new tests (including integration)
- `internal/plugin/all/all_test.go` — 1 new test
- `.claude/rules/plugin-design.md` — Add Dependencies to Registration Fields table

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | No | |
| CLI commands/flags | No | |
| Plugin SDK docs | [x] | `.claude/rules/plugin-design.md` — add Dependencies field |
| Functional test | [ ] | |

## Implementation Steps

1. **Write registry unit tests** — TestRegisterWithDependencies, TestResolveDependencies_* → Review: edge cases?
2. **Run tests** → Verify FAIL (new tests reference non-existent field/function)
3. **Implement registry changes** — Dependencies field, Register() validation, ResolveDependencies()
4. **Run tests** → Verify PASS
5. **Write loader unit tests** — TestExpandDependencies, TestExpandDependencies_NoDuplicate
6. **Run tests** → Verify FAIL
7. **Implement expandDependencies()** — new function + wire into 3 loading paths
8. **Run tests** → Verify PASS
9. **Write protocol unit tests** — TestStage1_DependencyValidation, TestStage1_DependencySatisfied
10. **Run tests** → Verify FAIL
11. **Implement stage 1 validation** — subsystem.go dependency check
12. **Run tests** → Verify PASS
13. **Add bgp-rr dependency declaration** — register.go
14. **Write integration test** — TestBgpRRDependsOnAdjRibIn
15. **Remove replayDisabled** — server.go field + all references
16. **Add protocol field** — DeclareRegistrationInput.Dependencies
17. **Update plugin-design.md** — Dependencies row in Registration Fields
18. **Verify all** → `make ze-verify`

### Failure Routing

| Failure | Route To |
|---------|----------|
| ResolveDependencies fails on existing plugins | Check registry state in test setup |
| Stage 1 validation rejects valid plugin | Check running plugin set construction |
| expandDependencies adds wrong deps | Check registry Lookup for name matching |
| replayDisabled removal breaks tests | Find tests that assert on replayDisabled |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Topological sort for dep resolution | Over-engineering — iterative loop-until-stable is simpler | Iterative expansion |
| Add adj-rib-in to ze-chaos config only | Fix should be in ze itself, not ze-chaos-specific | Plugin dependency declarations |

## Design Insights

- Two-layer system: Go registry for pre-startup auto-loading + protocol for runtime validation
- Iterative loop-until-stable handles transitive deps without topological sort
- Auto-added dependencies always Internal: true (Go plugins via init())
- replayDisabled was a silent degradation bug — fail loudly is better
- adj-rib-in stores routes by SOURCE peer, so replay works for both initial connection AND reconnection

## Implementation Summary

### What Was Implemented
- (pending)

### Bugs Found/Fixed
- (pending)

### Documentation Updates
- (pending)

### Deviations from Plan
- (pending)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make ze-unit-test` passes
- [ ] `make ze-functional-test` passes
- [ ] Feature code integrated (`internal/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
- [ ] `make ze-lint` passes
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING — before ANY commit)
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-rib-04-plugin-dependencies.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec. One commit = code + tests + spec.
