# Spec: lint-backlog-8-api-and-commands

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | 6 findings of the full lint in the web UI, MCP and the command model; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in the web UI and
MCP (door `ai/doors/api-surfaces.md`) and in the command model (door
`ai/doors/commands-cli.md`). By linter: goconst 3, exhaustive 3. Line numbers
are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/component/command/surface.go` | 48 | goconst |
| `internal/component/command/usage.go` | 68 | goconst |
| `internal/component/mcp/tools.go` | 1025 | goconst |
| `internal/component/web/editor.go` | 586 | exhaustive |
| `internal/component/web/fragment.go` | 446 | exhaustive |
| `internal/component/web/workbench_enrich.go` | 144 | exhaustive |

### Refreshed findings

The parent reported a fourth current issue in the ongoing scoped baseline:
modernize requests `maps.Copy` in `internal/component/command/node.go`,
`copyNodeForOverlay` (reported at line 346). The run has not finished, so this
is an added finding, not a complete refreshed inventory or a passing result.

| File | Producer | Linter |
|------|----------|--------|
| `internal/component/command/node.go` | `copyNodeForOverlay` | modernize |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/api-surfaces.md` - thin adapters over the dispatcher
  → Constraint: an adapter never copies a fact the YANG model or the registry holds; a goconst in `mcp/tools.go` that names a command word reads it from the registry where it exists
- [ ] `ai/doors/commands-cli.md` - the command model is infrastructure
  → Constraint: the command model holds no owner's word; `surface.go` and `usage.go` constants stay generic
  → Constraint: `command.AnswerSurface` is the one surface predicate, judged by `TestEveryCommandOnEverySurface`
- [ ] `docs/contributing/ze-go-style.md` - the `exhaustive` C/O/P classification

**Key insights:**
- The three web switches are over editor and fragment kinds; classify each before listing members.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/command/surface.go` - repeated enum diagnostic strings in the `String` methods
- [ ] `internal/component/command/node.go` - `copyNodeForOverlay`, which copies the children map
- [ ] `internal/component/web/editor.go` - the exhaustive site
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Which surfaces offer each command; every web page's rendering.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| Enum diagnostics | `internal/component/command/surface.go` `Surface.String`, `RegistrationKind.String` | Names enum values in diagnostic output | Reuses a constant for repeated diagnostic text, preserving the text |
| Surface parity | `cmd/ze/hub/surface_parity_test.go` `TestEveryCommandOnEverySurface` | Compares each surface producer with command eligibility | Applies as regression evidence, not as the location of the goconst fix |
| Overlay copy | `internal/component/command/node.go` `copyNodeForOverlay` | Copies the children map while sharing its child pointers | Modernizes the copy while preserving map ownership and child sharing |
| Style rule table and C/O/P | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/component/command/... ./internal/component/mcp/... ./internal/component/web/..."`

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
| R-1 | A constant shared between surface and usage code couples two concerns | A reviewer sees one constant naming two concepts | One constant per concept, in the file that owns it |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the packages | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the three packages | Zero findings |
| AC-2 | The package tests with their tags, `TestEveryCommandOnEverySurface` included | Pass |
| AC-3 | Any `//nolint` or `//exhaustive:ignore` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | `internal/component/command`, `mcp`, `web` | AC-2 | |
| `TestEveryCommandOnEverySurface` | `cmd/ze/hub/surface_parity_test.go` (requires `ze_ssh && ze_web`) | AC-2 | |

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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-8-api-and-commands.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. Fix the historical and refreshed findings still present in the scoped run.
3. Scoped lint zero; tests pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Constant ownership | Each constant names one concept. Generic command code gains no owner-specific words or registry copies (R-1). |
| Rendering and surfaces | C/O/P fixes preserve web rendering and unknown outcomes. `AnswerSurface` retains every command's offered surfaces. |
| API evidence | Three scopes are lint-clean, tagged tests include `TestEveryCommandOnEverySurface`, and annotations state reasons (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Three API scopes cleared | Record zero scoped lint findings and tagged package-test results, including the refreshed node-copy finding (AC-1–2). |
| Surface parity retained | Run the separate hub parity test in the TDD checklist and inspect annotation reasons (AC-2–3). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Command exposure | Diagnostic-string extraction leaves command eligibility unchanged, as checked by the hub parity test. |
| Web input and overlay ownership | Switch fixes preserve safe unknown handling. Node-copy modernization keeps a separate children map and existing child sharing. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the three packages>"` zero; `./le job run label api command go test -race -tags <tags> ./internal/component/command/... ./internal/component/mcp/... ./internal/component/web/...`. Full-run evidence belongs to the umbrella's CI run.
- [ ] Select `<tags>` as `ze_core` plus all unique first-column `ze_` tags in `feature-gates.txt`, per `docs/contributing/running-commands.md`, "A bare go test is not ./le test unit". This includes `ze_ssh` and `ze_web`.
- [ ] Separately run `./le job run label api-parity command go test -race -tags "<tags>" ./cmd/ze/hub/ -run '^TestEveryCommandOnEverySurface$' -count=1` and record its result.
