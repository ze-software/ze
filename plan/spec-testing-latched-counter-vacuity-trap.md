# Spec: a latched counter passes a test it cannot fail

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

`ai/rules/interop-and-goal-validation.md` lists four vacuity traps. A fifth is
missing, and neither the rule nor mutation-verify catches it.

A test reading a CUMULATIVE or LATCHED counter across an event asserts something
that was already true before the event: flows-ever-verified,
sessions-ever-established, a high-water mark, any "has happened at least once"
flag. It passes when the behavior under test is entirely broken, because the
counter still holds what an earlier phase put there. Mutation testing does not
catch it either: disabling the producer leaves the latched value true, so the
test stays green under mutation and therefore reads as discriminating.

The tell is mechanical. Reset the counter immediately before the event, and the
assertion must still pass. If it cannot, the test was reading history rather
than behavior.

Goal: add the fifth row carrying that tell, and decide whether the shape can be
found mechanically. Survey `test/` for assertions on values that only ever
increase, and report which are already vacuous, so the spec repairs tests rather
than only documenting the hazard.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/interop-and-goal-validation.md` - holds the vacuity-trap table
  → Decision: <to be filled>
  → Constraint: <to be filled>
- [ ] `ai/rules/testing.md` - mutation-verify and the discrimination requirement
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `scripts/dev/audit-test-relaxation.py` - audits a diff for deleted or weakened tests, and is the closest existing mechanical check to the survey this spec needs

`ai/rules/interop-and-goal-validation.md` holds the four-row trap table and has
no latched-counter row.

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

Not applicable: this spec changes a rule and surveys existing tests. It adds no
daemon behavior of its own. Any test it repairs is repaired in its own suite.

## Files to Modify

- `ai/rules/interop-and-goal-validation.md` - <what changes>

## Implementation Steps

1. <to be filled>

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before implementation
- [ ] Tests PASS after implementation
- [ ] `make ze-precommit-verify` green

### Integration Checklist
- [ ] <to be filled>

### Documentation Update Checklist
- [ ] <to be filled>
