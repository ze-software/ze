# Spec: hook-git-guard-misses-index-writers

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Agent tooling: the guard that keeps sessions off the shared index and the branch has holes, but no shipped behaviour changes |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Found (2026-11-12)** by the closure review of
`spec-hook-git-verb-prefix-match`, and present before it.

**Symptom.** The Bash hook's two git guards refuse the verbs their lists name.
Several git commands do the harm those lists exist to stop and are named by
neither, so the hook lets them run:

| Command | What it does in the shared checkout |
|---------|-------------------------------------|
| `git cherry-pick`, `git am` | add commits to the branch, outside `./le commit` |
| `git read-tree`, `git update-index`, `git apply --cached` | write the shared index |

**Cause, read at the producer.** `bashDestructiveGit` and `bashBranchMove`
(`internal/le/hookruntime/bash.go`) each hold a literal list of patterns, and
`gitVerbRun` answers only for a pattern it is given.

**Goal.** A git command that writes the shared index, or adds commits to or
moves the branch, is refused by the hook with the route that works, whether or
not a list names it today. Read-only git commands keep passing.

**To decide in design.** Whether the guards gain these names, or whether the
decision moves to the parsed command words that AC-8 of
`plan/post-release/engineering/spec-gates-share-one-semantic-population.md`
plans; and the full set of git commands that write the index or the branch,
derived from git's own command list rather than from memory.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the doors this work crosses
- [ ] `docs/contributing/running-commands.md`, "The Bash guard matches your command text" - what the guards do today
- [ ] `ai/rules/git-safety.md` - the bans the guards enforce

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/hookruntime/bash.go` - `bashDestructiveGit`, `bashBranchMove`, `gitVerbRun`, `gitInvocation`

**Behavior to preserve:**
- [to research]

**Behavior to change:**
- [to research]

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Git verb guards | `internal/le/hookruntime/bash.go` `bashDestructiveGit`, `bashBranchMove`, `gitVerbRun` | Refuse the git verbs two literal lists name, each matched as a whole word | [to research] |

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
| A Bash call `git cherry-pick <sha>` | → | the git guards | [to design] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to design] | `internal/le/hookruntime/` | each index-writing or branch-moving command is refused; read-only commands pass | |

## Files to Modify
- `internal/le/hookruntime/bash.go` - [to design]

## Implementation Steps

1. **Phase: Red tests through the real action** -- [to design]
2. **Phase: The guard** -- [to design]

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le ai hooks unit`, `./le test unit pkg path ./internal/le/hookruntime/`

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `gate-excludes-part-of-its-population` | 2026-11-12 | `bashDestructiveGit`, `bashBranchMove`: `git cherry-pick`, `git am`, `git read-tree`, `git update-index`, `git apply --cached` are not refused |
