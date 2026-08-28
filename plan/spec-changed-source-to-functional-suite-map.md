# Spec: nothing binds a changed directory to the suites that cover it

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`ai/rules/testing.md` states that the affected population is not the edited
population. For GO PACKAGES that is already handled: `changed-pkgs.sh`
(`internal/le/`) unions uncommitted changes, commits made since
the last green verify, and the reverse dependencies that import them.

The gap is functional suites. That script emits Go package directories, and
nothing maps a changed directory to the `.ci`, `.et` or `.wb` suites written to
prove that component works. So an edit under a component with a dedicated suite
runs `./le changed scope` and the package unit tests, goes green, and never
runs the suite that exists for it.

Goal: a declarative map from source path glob to the functional suites covering
it, read by CI and by a local target, so the binding is data rather than memory.

The map is the easy half. Keeping it honest is the hard half: decide what
happens when a component has no entry, because a map that silently covers
nothing repeats the problem it exists to fix.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/testing.md` - states the affected-population rule
  → Decision: <to be filled>
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/` - unions working tree, since-green commits, and reverse deps, and emits Go package directories only

**Behavior to preserve:**
- <to be filled>

**Behavior to change:**
- <to be filled>

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- <to be filled>

### Transformation Path
1. <to be filled>

### Boundaries Crossed
| Boundary | From | To |
|----------|------|-----|
| <to be filled> | <to be filled> | <to be filled> |

### Integration Points
- <to be filled>

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| <to be filled> | → | <to be filled> | <to be filled> |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| <to be filled> | <to be filled> | <to be filled> |

### Functional Tests

Tooling only, no daemon code. The driving surface is
`internal/le/` and its Go-hosted test, which must prove that an
edit under a mapped directory selects its suite and that an unmapped directory
is reported rather than passed over.

## Files to Modify

- `internal/le/` - <what changes>

## Implementation Steps

1. <to be filled>

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] `./le verify current mode full` green

### Integration Checklist
- [ ] <to be filled>

### Documentation Update Checklist
- [ ] <to be filled>
