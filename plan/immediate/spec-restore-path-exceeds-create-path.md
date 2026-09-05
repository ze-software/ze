# Spec: a restore path that does more than the create path

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-08-18 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A shape worth hunting in Ze: state restored after a restart is programmed more
completely than state created fresh. The restore path accumulates every binding
the object needs, because restoring is written once against a whole object. The
create path grows one field at a time, and a later binding is added to restore
alone. The result is a component correct after a restart and wrong on first use,
which is the reverse of the failure anyone thinks to test.

The class is invisible to a test that restarts before asserting, and invisible
to a test that never restarts. Catching it needs a test asserting the SAME
programmed state by both routes.

Goal: find whether Ze carries the shape. Candidates are every component with a
restore, reconcile, or resync entry point that programs a backend:
`internal/component/iface`, `internal/component/ike/engine`,
`internal/component/l2tp`, and the firewall and FIB backends. For each,
enumerate what restore programs and what create programs, and report the
difference.

A second half is worth deciding at design time. Where the two paths diverge
legitimately, error handling must not diverge with them: a restore that returns
an error while fresh creation only logs a warning leaves an object up and
unconfigured, which is the same defect wearing a different hat.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/plugins.md` - how a plugin programs its backend
  → Decision: <to be filled>
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/iface/config_apply.go` - carries a reconcile entry point beside the apply path

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
| Test | File | Validates |
|------|------|-----------|
| <to be filled> | `test/plugin/restore-matches-create.ci` | the same programmed state results whether an object is created fresh or restored |  <!-- doc-links: ignore (fixture this spec will create; the spec is `skeleton` and the work is not implemented) -->

## Files to Modify

- `internal/component/iface/config_apply.go` - <what changes>

## Implementation Steps

1. <to be filled>

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] `./le verify worktree` green

### Integration Checklist
- [ ] <to be filled>

### Documentation Update Checklist
- [ ] <to be filled>
