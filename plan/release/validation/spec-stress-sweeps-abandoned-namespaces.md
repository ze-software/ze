# Spec: stress-sweeps-abandoned-namespaces

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Each stress harness killed outright leaves two network namespaces on the host; they accumulate across runs and no later run removes them |
| RFC gaps | - |
| Depends | - (spec-stress-children-die-with-harness, closed: every stress child now dies with the harness, so a dead run's namespaces hold no process) |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom (read at the producer, 2026-11-11).** The stress harness names its
namespaces after its own suffix (`stressBirdRunner.initializeNames` in
`internal/le/test/integration/stressbird.go`: `ze-stress-ze-<suffix>` and
`ze-stress-bb-<suffix>`, the suffix being the harness PID unless
`ZE_STRESS_SUFFIX` sets it). Its opening cleanup (`stressBirdRunner.cleanup`
with `reportErrors` false) deletes only those two names. A harness killed
outright runs no cleanup, so its two namespaces stay under `/run/netns`, and no
later run removes them because no later run carries that suffix.

**Goal.** A stress run removes the namespaces an earlier run left behind when
that run is dead, and never touches the namespaces of a run that is still
alive.

**Open for design.** How a dead run is told from a live one when the suffix is
a PID that may have been reused, or a `ZE_STRESS_SUFFIX` value that is not a
PID at all; and how the recorder fixtures in `stressbird_test.go`, whose call
order every `TestStressBird*` test asserts, learn the sweep.

## Required Reading

### Architecture Docs
- [ ] `docs/functional-tests.md` - the stress harness section and its cleanup paragraph
  → Constraint: [to research]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/test/integration/stressbird.go` - `initializeNames`, `cleanup`, `stressNamespacePath`

**Behavior to preserve:**
- [to research]

**Behavior to change:**
- A dead run's namespaces are removed by the next run

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| [to research] | `internal/le/test/integration/stressbird.go` `stressBirdRunner.cleanup` | [to research] | [to research] |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [to research]

### Transformation Path
1. [to research]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ `/run/netns` | [to research] | No |

### Integration Points
- [to research]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le test integration stress-bird` after a killed run | → | `stressBirdRunner.cleanup` | [to design] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to design] | `internal/le/test/integration/` | a dead run's namespaces are removed; a live run's are kept | |

## Files to Modify
- `internal/le/test/integration/stressbird.go` - [to design]

## Implementation Steps

1. **Phase: Wiring and red test** -- [to design]
2. **Phase: Sweep** -- [to design]

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le test unit pkg path ./internal/le/test/integration/`
