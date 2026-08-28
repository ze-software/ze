# Spec: prove a refactor commit moved code without changing it

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

A commit labelled `refactor:` claims behavior did not change. Nothing checks the
claim, and a large file split is exactly where a behavior change hides: the diff
is too large to read, every hunk looks like a move, and a reviewer who
spot-checks sees moves everywhere.

There is a cheap mechanical proof. Split both trees into one file per function
body, then diff the sets. A pure move leaves every body byte-identical and
leaves the non-function diff holding only imports, package clauses and headers.
Anything else is a behavior change wearing a refactor label, and the tool names
the function it is hiding in.

Goal: a tool taking two commits and a path, reporting the function bodies that
differ, the functions added, and the functions removed.

Decide whether it runs as a gate on a `refactor:` subject or stays a hand-run
check. A gate needs an escape for a refactor that deliberately rewrites a body,
and an escape nobody can audit is worse than no gate at all.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/git-safety.md` - commit granularity and message conventions
  → Decision: <to be filled>
  → Constraint: <to be filled>

**Key insights:** (minimal context to resume after compaction)
- <to be filled>

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/` - the existing pattern for a repository-wide dev script

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

Tooling only, no daemon code. The driving surface is a new dev script beside
`internal/le/weakened/audit.go`, with a Go-hosted test proving that a
changed body is reported and that a pure move is not.

## Files to Modify

- `internal/le/` - <reference pattern only; the new tool lands beside it>

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
