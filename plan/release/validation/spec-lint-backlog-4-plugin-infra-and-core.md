# Spec: lint-backlog-4-plugin-infra-and-core

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | 14 findings of the full lint in the plugin infrastructure, the public SDK and statestore; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in the plugin
infrastructure and the public SDK (door `ai/doors/plugin-infra.md`) and in
`internal/core/statestore` (door `ai/doors/core.md`). By linter: errcheck 10,
unparam 1, modernize 1, gocritic 1, misspell 1. All but one are in test files.
Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/component/plugin/ipc/filter_batch_test.go` | 24 | unparam |
| `internal/component/plugin/ipc/tls_test.go` | 455 | errcheck |
| `internal/component/plugin/process/process_stop_test.go` | 87 | errcheck |
| `internal/component/plugin/process/recovery_delivery_integration_test.go` | 242 | errcheck |
| `internal/component/plugin/registry/filter_declaration.go` | 47 | modernize |
| `internal/component/plugin/server/local_bridge_test.go` | 121 | gocritic |
| `internal/component/plugin/server/startup_chain_refused_test.go` | 94 | misspell |
| `internal/component/plugin/sysproc_linux_test.go` | 54 | errcheck |
| `internal/core/statestore/statestore_reader_test.go` | 95 | errcheck |
| `pkg/plugin/sdk/sdk_callbacks_test.go` | 67 | errcheck |
| `pkg/plugin/sdk/sdk_callbacks_test.go` | 100 | errcheck |
| `pkg/plugin/sdk/sdk_callbacks_test.go` | 124 | errcheck |
| `pkg/plugin/sdk/sdk_callbacks_test.go` | 151 | errcheck |
| `pkg/plugin/sdk/sdk_callbacks_test.go` | 182 | errcheck |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/plugin-infra.md` - the SDK, the registry and the process protocol
  → Constraint: `pkg/plugin/sdk` is a published contract; only its tests change here, and no exported name moves
  → Constraint: a test that mutates the registry takes `registry.Snapshot()` and restores it
- [ ] `ai/doors/core.md` - statestore
  → Constraint: `statestore.Put` answers `false, nil` with no store; a test checking an error checks the acknowledging call
- [ ] `docs/contributing/ze-go-style.md` - "Every error is handled"
  → Constraint: an ignored error in a test is checked or carries `//nolint:errcheck // <reason>`

**Key insights:**
- The only non-test finding is a modernize rewrite in `registry/filter_declaration.go`; it must not change the Stage 1 declaration a plugin sends.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/registry/filter_declaration.go` - the modernize site
- [ ] `pkg/plugin/sdk/sdk_callbacks_test.go` - five ignored errors
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- The process protocol, the SDK's exported surface, every test's assertion.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| Style rule table | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |
| Scoped plugin tests | `./le job run label plugin command go test -race ./internal/component/plugin/... ./pkg/plugin/...` (plugin-infra door, Gates) | Runs the plugin packages under race | Applies: AC-2 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/component/plugin/... ./pkg/plugin/... ./internal/core/statestore/..."`

### Transformation Path
1. Read each finding; check the error, or annotate with the reason it cannot matter.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Test and one source edit | No |

### Integration Points
- None beyond the packages listed.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | The scoped run before the first edit | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Checking a previously ignored error makes a test fail on a teardown race | A flaky red in the changed test | Read why the call can fail; assert only what the test owns, annotate the rest with its reason |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the three trees | Zero findings |
| AC-2 | The scoped plugin and statestore tests under `-race` | Pass |
| AC-3 | Any `//nolint` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | the changed packages | AC-2 | |

## Files to Modify
- The files in the Task table.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG, CLI, env, doctor, metrics, BGP family surface | N-A | Lint fixes; no surface changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1-15, 17 | Feature, config, CLI, API, plugin, wire, SDK, RFC, test infrastructure, comparison, architecture, metadata, metrics, inventory, examples | No | Lint fixes change no documented behavior |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-4-plugin-infra-and-core.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. Fix the test findings, then the registry modernize.
3. Scoped lint zero; tests pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Teardown errors | Error checks distinguish failures the test owns from teardown races, without weakening its assertions (R-1). |
| Published contracts | The registry rewrite preserves the Stage 1 declaration and SDK exports. Registry tests restore their snapshots. |
| Plugin and store evidence | All three trees are lint-clean, race tests pass, and ignored errors carry specific reasons (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Plugin and statestore scopes cleared | Record zero scoped lint findings and race-test results for the three trees (AC-1–2). |
| Preserved SDK and declaration contracts | Inspect the registry rewrite and SDK test assertions, including each ignored-error reason (AC-3). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| TLS and process failures | Error checks preserve TLS, process-stop and callback assertions and distinguish expected teardown failures. |
| Shared state | Registry tests restore snapshots, and statestore tests check the acknowledging operation without masking its error. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the three trees>"` zero; `./le job run label plugin command go test -race ./internal/component/plugin/... ./pkg/plugin/... ./internal/core/statestore/...`. Full-run evidence belongs to the umbrella's CI run.
