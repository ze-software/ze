# Spec: arch-1 — Boundary Interfaces

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `docs/plan/spec-arch-0-system-boundaries.md` — umbrella spec with all interface definitions
3. `pkg/ze/` — the files created by this spec

## Task

Create the `pkg/ze/` package with all five boundary interfaces and their associated types, as defined in the umbrella spec. This is **new files only** — no existing code changes, no behavior changes.

## Required Reading

### Architecture Docs
- [ ] `docs/plan/spec-arch-0-system-boundaries.md` — umbrella spec with interface tables
  → Decision: interfaces live in `pkg/ze/`
  → Decision: Bus uses hierarchical topics with prefix matching
  → Decision: Event payload is always `[]byte`
  → Decision: ConfigProvider includes Load/Validate/Save

### Source Files (current boundaries to understand)
- [ ] `internal/plugin/types.go` — existing ReactorLifecycle interface (17 methods)
  → Constraint: Subsystem interface is the replacement — simpler (4 methods)
- [ ] `pkg/plugin/plugin.go` — existing SDK Plugin type
  → Constraint: SDK will eventually consume these interfaces; don't conflict with existing types

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugin/types.go` — ReactorLifecycle, BGPHooks, PeerInfo, RPCRegistration
- [ ] `pkg/plugin/plugin.go` — Plugin SDK type with callbacks

**Behavior to preserve:**
- All existing code continues to compile and work unchanged — this phase adds new files only
- No existing imports change

**Behavior to change:**
- None — pure addition of new interface definitions

## Data Flow (MANDATORY)

### Entry Point
- No data flows through these interfaces yet — they are definitions only
- Implementations will be created in Phases 2-5

### Transformation Path
1. Phase 1 creates interface definitions in `pkg/ze/`
2. Phase 2+ creates implementations that satisfy these interfaces
3. Existing code is gradually refactored to use these interfaces

### Boundaries Crossed

| Boundary | Mechanism | Content |
|----------|-----------|---------|
| `pkg/ze/` ↔ implementations | Go interface satisfaction | Method signatures only (this phase) |

### Integration Points
- `pkg/ze/Bus` will be implemented by extracted `SubscriptionManager` + delivery logic (Phase 2)
- `pkg/ze/PluginManager` will be implemented by extracted `ProcessManager` + startup coordinator (Phase 3)
- `pkg/ze/ConfigProvider` will wrap `internal/config/` (Phase 4)
- `pkg/ze/Engine` will compose all three (Phase 5)
- `pkg/ze/Subsystem` will be implemented by BGP reactor (Phase 5)

### Architectural Verification
- [ ] No bypassed layers — interfaces only, no implementations
- [ ] No unintended coupling — `pkg/ze/` has zero imports from internal packages
- [ ] No duplicated functionality — these are new abstractions, not copies
- [ ] Zero-copy preserved — Event.Payload is `[]byte`, no forced copying

## Wiring Test (MANDATORY — NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Import `pkg/ze` | → | Package compiles, interfaces available | `TestInterfacesCompile` |
| Mock Bus implementation | → | Satisfies `ze.Bus` interface | `TestMockBusSatisfiesInterface` |
| Mock Subsystem implementation | → | Satisfies `ze.Subsystem` interface | `TestMockSubsystemSatisfiesInterface` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `pkg/ze/` package imported | Compiles with zero internal imports |
| AC-2 | Type implementing all Bus methods | Satisfies `ze.Bus` interface |
| AC-3 | Type implementing all Subsystem methods | Satisfies `ze.Subsystem` interface |
| AC-4 | Type implementing all ConfigProvider methods | Satisfies `ze.ConfigProvider` interface |
| AC-5 | Type implementing all PluginManager methods | Satisfies `ze.PluginManager` interface |
| AC-6 | Type implementing all Engine methods | Satisfies `ze.Engine` interface |
| AC-7 | `go vet ./pkg/ze/...` | No issues |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestInterfacesCompile` | `pkg/ze/ze_test.go` | Package compiles, basic type assertions | |
| `TestMockBusSatisfiesInterface` | `pkg/ze/ze_test.go` | Mock Bus satisfies ze.Bus | |
| `TestMockSubsystemSatisfiesInterface` | `pkg/ze/ze_test.go` | Mock Subsystem satisfies ze.Subsystem | |
| `TestMockConfigProviderSatisfiesInterface` | `pkg/ze/ze_test.go` | Mock ConfigProvider satisfies ze.ConfigProvider | |
| `TestMockPluginManagerSatisfiesInterface` | `pkg/ze/ze_test.go` | Mock PluginManager satisfies ze.PluginManager | |
| `TestMockEngineSatisfiesInterface` | `pkg/ze/ze_test.go` | Mock Engine satisfies ze.Engine | |
| `TestMockConsumerSatisfiesInterface` | `pkg/ze/ze_test.go` | Mock Consumer satisfies ze.Consumer | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N/A | — | Phase 1 has no end-user-visible behavior | — |

## Files to Modify

- No existing files modified in this phase

## Files to Create

- `pkg/ze/bus.go` — Bus, Consumer, Event, Topic, Subscription interfaces and types
- `pkg/ze/config.go` — ConfigProvider interface and ConfigChange type
- `pkg/ze/engine.go` — Engine interface
- `pkg/ze/plugin.go` — PluginManager, PluginProcess, Capability interfaces and types
- `pkg/ze/subsystem.go` — Subsystem interface
- `pkg/ze/ze_test.go` — Interface satisfaction tests with mock types

## Implementation Steps

1. **Write tests** — mock types that must satisfy each interface → Review: all methods covered?
2. **Run tests** → Verify FAIL (interfaces don't exist yet)
3. **Implement** — create interface files → Minimal definitions matching umbrella spec
4. **Run tests** → Verify PASS
5. **Verify all** → `make test-all`
6. **Complete spec** → Fill audit tables

### Failure Routing

| Failure | Route To |
|---------|----------|
| Interface method conflicts with existing types | Adjust method name/signature |
| Import cycle | Ensure pkg/ze/ has zero internal imports |
| Lint failure | Fix inline |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete — every row has a concrete test name, none deferred
- [ ] `make test-all` passes (lint + all ze tests)
- [ ] Feature code integrated (`pkg/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` — no failures)

### Quality Gates (SHOULD pass — defer with user approval)
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
- [ ] Critical Review passes — all 6 checks in `rules/quality.md` documented pass in spec
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Spec moved to `docs/plan/done/NNN-<name>.md`
- [ ] **Spec included in commit** — NEVER commit implementation without the completed spec
