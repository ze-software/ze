# Spec: lint-backlog-5-hub-and-policy

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | 18 findings of the full lint in `cmd/ze` and the policy component; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in `cmd/ze` and
`cmd/ze/hub` (door `ai/doors/hub-engine.md`), all in test files, and in
`internal/component/policy` (covered by `ai/doors/system.md` alone), all
goconst. By linter: goconst 10, gocritic 3, errcheck 2, exhaustive 1,
modernize 1, unused 1. Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `cmd/ze/hub/build_tag_ssh_absent_test.go` | 252 | errcheck |
| `cmd/ze/hub/build_tag_ssh_absent_test.go` | 255 | errcheck |
| `cmd/ze/hub/local_bridge_match_test.go` | 184 | gocritic |
| `cmd/ze/hub/local_bridge_match_test.go` | 195 | exhaustive |
| `cmd/ze/hub/surface_parity_test.go` | 224 | modernize |
| `cmd/ze/hub/surface_parity_test.go` | 278 | gocritic |
| `cmd/ze/hub/surface_parity_test.go` | 337 | gocritic |
| `cmd/ze/schema_helpers_test.go` | 25 | unused |
| `internal/component/policy/chain.go` | 127 | goconst |
| `internal/component/policy/chain.go` | 129 | goconst |
| `internal/component/policy/export_chain.go` | 20 | goconst |
| `internal/component/policy/export_chain.go` | 41 | goconst |
| `internal/component/policy/export_chain.go` | 43 | goconst |
| `internal/component/policy/reference_source.go` | 146 | goconst |
| `internal/component/policy/reference_source.go` | 182 | goconst |
| `internal/component/policy/rib_compile.go` | 190 | goconst |
| `internal/component/policy/rib_compile.go` | 192 | goconst |
| `internal/component/policy/route_chain_path.go` | 42 | goconst |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/hub-engine.md` - `cmd/ze` and the hub
  → Constraint: an untagged test does not see BGP; the hub tests run with the tags their package needs
  → Constraint: `TestEveryCommandOnEverySurface`-style parity tests must keep proving every surface; a gocritic rewrite keeps every case
- [ ] `ai/doors/system.md` - the policy component sits under no subsystem door
  → Constraint: a goconst constant names one concept once; a config path or chain keyword the YANG already declares is read from its declaration where one exists
- [ ] `docs/contributing/ze-go-style.md` - "The rules a tool enforces"

**Key insights:**
- The policy goconst strings are config words (chain and source names); check whether a declared constant already holds each before adding one.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/policy/export_chain.go` - three repeated strings
- [ ] `cmd/ze/hub/surface_parity_test.go` - the parity test's gocritic and modernize sites
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- The policy chain's config words and every hub test's assertions.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| Style rule table | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |
| Enumeration gate | `./le arch enumeration check` | Refuses a literal that restates what a registry holds | Applies: a new constant must not restate a registry |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope ./cmd/ze/... ./internal/component/policy/...`

### Transformation Path
1. Read each finding; fix, or annotate with the reason the linter is wrong there.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Source edits inside the packages | No |

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
| R-1 | Deleting the `unused` helper in `cmd/ze/schema_helpers_test.go` removes something a tagged build uses | A flavor's test build fails | Check every build flavor before deleting (`./le repo compiles check`), or tag the file |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over `cmd/ze/...` and `internal/component/policy/...` | Zero findings |
| AC-2 | The package tests with their tags | Pass |
| AC-3 | Any `//nolint` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | `cmd/ze/hub`, `cmd/ze`, `internal/component/policy` | AC-2 | |

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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-5-hub-and-policy.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. Policy goconst; hub test findings.
3. Scoped lint zero; tests pass.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope ./cmd/ze/... ./internal/component/policy/...` zero; `./le job run label hub command go test -tags <tags> ./cmd/ze/hub/ ./cmd/ze/ ./internal/component/policy/`; `./le verify changed` over the commits
