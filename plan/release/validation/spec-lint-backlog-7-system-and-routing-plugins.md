# Spec: lint-backlog-7-system-and-routing-plugins

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | 19 findings of the full lint in system plugins and the OSPF and IS-IS plugins; the full lint holds the release (`plan/release/validation/spec-lint-backlog-0-umbrella.md`) |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-02 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Admission uses the one-off WIP exception in `plan/release/validation/spec-lint-backlog-0-umbrella.md`, "Owner's one-off WIP exception".

Child of `plan/release/validation/spec-lint-backlog-0-umbrella.md`. Clear the
findings CI's full `./le go lint run` reported on 2026-11-02 in the system
plugins `debug`, `help`, `redistribute` and `skills` (door
`ai/doors/system-plugins.md`) and in the OSPF and IS-IS plugins (door
`ai/doors/routing-protocols.md`). By linter: unused 6 (all
`redistribute/metrics_test.go`), goconst 3, gocritic 3, modernize 2,
goimports 2, misspell 1, godot 1, unparam 1. Line numbers are the CI run's.

| File | Line | Linter |
|------|------|--------|
| `internal/plugins/debug/debug.go` | 84 | modernize |
| `internal/plugins/debug/debug.go` | 211 | goconst |
| `internal/plugins/help/help_ai_test.go` | 6 | goimports |
| `internal/plugins/help/help_command_plugin_modeled_test.go` | 28 | goimports |
| `internal/plugins/isis/redistribute/consumer.go` | 215 | gocritic |
| `internal/plugins/isis/redistribute/source.go` | 55 | goconst |
| `internal/plugins/isis/register.go` | 59 | misspell |
| `internal/plugins/isis/server.go` | 545 | godot |
| `internal/plugins/ospf/redistribute/consumer.go` | 225 | gocritic |
| `internal/plugins/ospf/redistribute/source.go` | 48 | goconst |
| `internal/plugins/ospf/ri_show_test.go` | 90 | unparam |
| `internal/plugins/redistribute/metrics_test.go` | 87 | unused |
| `internal/plugins/redistribute/metrics_test.go` | 89 | unused |
| `internal/plugins/redistribute/metrics_test.go` | 90 | unused |
| `internal/plugins/redistribute/metrics_test.go` | 97 | unused |
| `internal/plugins/redistribute/metrics_test.go` | 99 | unused |
| `internal/plugins/redistribute/metrics_test.go` | 100 | unused |
| `internal/plugins/redistribute/reload_test.go` | 109 | gocritic |
| `internal/plugins/skills/main.go` | 217 | modernize |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system-plugins.md` - edge plugins, ownership
  → Constraint: a plugin owns its whole surface; a goconst constant stays in the plugin's package
- [ ] `ai/doors/routing-protocols.md` - OSPF and IS-IS
  → Constraint: OSPFv2 and OSPFv3 share code; the two redistribute consumers (`ospf`, `isis`) carry the same gocritic and goconst findings, so fix them the same way in both
  → Constraint: an RFC quote in a comment stays byte-exact (the `isis/register.go` misspell and the `isis/server.go` godot may sit in one)
- [ ] `docs/contributing/ze-go-style.md` - "The rules a tool enforces"

**Key insights:**
- The six `unused` findings are one test helper block in `redistribute/metrics_test.go`: either a test lost its caller or the helper is dead; find which before deleting.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/redistribute/metrics_test.go` - the unused block
- [ ] `internal/plugins/ospf/redistribute/consumer.go` - the gocritic site mirrored in IS-IS
- [ ] `.golangci.yml` - the linter settings

**Behavior to preserve:**
- Redistribution behavior in both protocols; every test's assertions.

**Behavior to change:**
- Each finding above, fixed at its source.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Lint runner | `./le go lint run scope <packages>` | Lints a package scope | Applies: the evidence for this child |
| Style rule table | `docs/contributing/ze-go-style.md` | Names the fix each linter expects | Applies |
| Weakening judge | `ze-test-judge` | Approves a test change that could prove less | Applies if the unused block turns out to be a lost assertion that is restored or removed |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le go lint run scope "./internal/plugins/debug/... ./internal/plugins/help/... ./internal/plugins/redistribute/... ./internal/plugins/skills/... ./internal/plugins/ospf/... ./internal/plugins/isis/..."`

### Transformation Path
1. Read each finding; fix, or annotate with the reason the linter is wrong there.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| None | Source edits inside the plugins | No |

### Integration Points
- None beyond the packages listed.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The CI list is current for these packages | CI run of 2026-11-02 | Lines drifted | The scoped run before the first edit | unvalidated |
| A-2 | The unused helpers in `redistribute/metrics_test.go` have no live caller in any build flavor | golangci-lint runs every flavor | A tagged test uses them | Grep and `./le repo compiles check` before deleting | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The unused block was meant to be asserted and a test silently stopped checking metrics | The helpers read a metric no test now checks | Restore the assertion it served (a judge reviews), rather than delete |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le go lint run scope` over the plugins | → | golangci-lint | zero findings in the scoped run |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The scoped lint over the six plugin trees | Zero findings |
| AC-2 | The package tests with their tags (`ze_ospf`, `ze_isis` where gated) under `-race` | Pass |
| AC-3 | Any `//nolint` added | Names its linter and its reason |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| The existing package tests | the changed plugins | AC-2 | |

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
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors spec plan/release/validation/spec-lint-backlog-7-system-and-routing-plugins.md` |

## Implementation Steps

1. Scoped lint; record the current list.
2. The OSPF and IS-IS redistribute pair together; the rest per plugin.
3. Scoped lint zero; tests pass.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Metrics assertions | Establish whether unused helpers lost their callers. Restore lost assertions with judge review rather than remove coverage (A-2, R-1). |
| Paired consumers | OSPF and IS-IS receive equivalent redistribution fixes, constants remain plugin-owned, and quoted RFC text stays byte-exact. |
| Routing evidence | Six trees are lint-clean, tagged race tests pass, and annotations state reasons (AC-1–3). |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Six plugin scopes cleared | Record zero scoped lint findings and tagged race-test results (AC-1–2). |
| Preserved metrics coverage | Resolve unused-helper callers and cite judge evidence for assertion changes, with reasoned annotations (A-2, R-1, AC-3). |

### Security Review Checklist

| Concern | What to verify |
|---------|----------------|
| Redistribution decisions | Paired OSPF and IS-IS edits preserve route selection and source values. |
| Lost failure evidence | Unused-helper removal does not remove metrics assertions, and comment fixes preserve byte-exact RFC quotes. |

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] `./le go lint run scope "<the six trees>"` zero; `./le job run label plugins command go test -race -tags <tags> <changed packages>`. Full-run evidence belongs to the umbrella's CI run.
