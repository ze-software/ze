# Spec: hook-git-verb-prefix-match

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Agent tooling: a false refusal costs every session a retry and pushes agents toward wrappers, but no shipped behaviour changes |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom (2026-11-11).** A test-judge agent's read-only
`git merge-base --is-ancestor <sha> <ref>` was refused by the Bash hook as
`git merge`, the branch-integration ban. It is the second occurrence: the
2026-09-02 row of `plan/journal/hook-existing-patterns-false-positive.md`
records the first.

**Cause, read at the producer.** `gitVerbRun`
(`internal/le/hookruntime/bash.go`) finds the verb with `strings.Index` and
tests only the byte before it (start of line, a separator, a pipe, an opening
subshell, a newline). It never tests the byte after it, so every pattern is a
prefix match: `bashBranchMove` carries `git merge`, which matches
`git merge-base`.

**Goal.** A git verb guard matches the verb as a whole word: `git merge-base`
passes, while `git merge`, `git merge --ff-only` and `git merge <branch>` stay
refused. Every other pattern the guards carry keeps its verdict.

The 2026-09-02 row stays listed in
`plan/post-release/engineering/spec-gates-share-one-semantic-population.md`;
the fix here closes both rows, and that spec's entry is dropped in the same
commit.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the doors this work crosses
  → Constraint: a hook checks what a command is, never how a message is worded (`ai/META-DECISIONS.md`); a refusal is never passed by a wrapper
- [ ] `plan/learned/HOOK-FRICTION.md` - known false positives of each hook, before one is changed

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/hookruntime/bash.go` - `gitVerbRun` (leading boundary only), `bashBranchMove` and the staging guard (its callers), `gitInvocation`

**Behavior to preserve:**
- [to research]

**Behavior to change:**
- A verb pattern matches only when the byte after it ends a word

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Git verb position test | `internal/le/hookruntime/bash.go` `gitVerbRun`, `insideQuotes` | Judges a git verb by the byte before it, with a quote exemption | Extends: [to research] |
| Git global option strip | `internal/le/hookruntime/bash.go` `gitInvocation`, `gitGlobalOptions` | Puts the verb back beside `git` past `-C`, `-c` and the other leading options | Does not apply: [to research] |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A Bash tool call, through the PreToolUse hook

### Transformation Path
1. [to research]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ hook | [to research] | No |

### Integration Points
- [to research]

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A Bash call `git merge-base --is-ancestor <sha> main` | → | `gitVerbRun`, `bashBranchMove` | [hook golden row, to design] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to design] | `internal/le/hookruntime/` | `git merge-base` passes; `git merge`, `git merge --ff-only` and `git merge <branch>` are refused | |

## Files to Modify
- `internal/le/hookruntime/bash.go` - [to design]

## Implementation Steps

1. **Phase: Red golden rows, both ways** -- [to design]
2. **Phase: Trailing word boundary** -- [to design]

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le ai hooks unit`, `./le test unit pkg path ./internal/le/hookruntime/`

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `hook-existing-patterns-false-positive` | 2026-11-11 | `gitVerbRun` prefix match: `git merge-base --is-ancestor` refused as `git merge` (second occurrence) |
| `hook-existing-patterns-false-positive` | 2026-11-12 | `gitVerbRun` prefix match: `git merge-base --is-ancestor` refused as `git merge` (third occurrence) |
