# Spec: hook-daemon-proof-website-tree

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P3 |
| Release reason | Agent tooling: a false refusal of a spec edit costs a session a redesign or a wrapper temptation, but no shipped behaviour changes |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Symptom (2026-11-12).** A tooling spec
(`plan/post-release/engineering/spec-activity-codebase-growth.md`) named a new
script for the public website under `website/assets/js/` in its Files to
Create. The spec write was refused with "Daemon-path changes must reference a
functional test file under test/ (.ci, .et or .wb)". Nothing in that spec
reaches the daemon.

**Cause, read at the producer.** `daemonProofProblem`
(`internal/le/hookruntime/specproof.go`) treats every named path as daemon
source unless `daemonProofExempt` exempts it or `sourceKind` says it is not
source. `sourceKind` counts `.js`, because the daemon's own web UI ships
scripts. `daemonProofExempt` exempts `_test.go`, `internal/le/`, `test/`, the
non-production prefixes and any `testdata` segment. It never exempts the
`website/` tree, which is the public site and is linked into no binary.

**Goal.** A spec that changes only the public website tree and tooling owes no
functional test under `test/`, while a spec that changes a script of the
daemon's web UI (`internal/component/web`) still does.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the doors this work crosses
  → Constraint: a hook checks what a path is, never how a message is worded (`ai/META-DECISIONS.md`); a refusal is never passed by a wrapper
- [ ] `plan/learned/HOOK-FRICTION.md` - known false positives of each hook, before one is changed

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/hookruntime/specproof.go` - `daemonProofProblem`, `daemonProofExempt`, `functionalTestFormat`

**Behavior to preserve:**
- [to research]

**Behavior to change:**
- A path under the public website tree is not daemon source for this gate

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Daemon-proof exemption list | `internal/le/hookruntime/specproof.go` `daemonProofExempt` | Exempts test files, `internal/le/`, `test/`, the non-production prefixes and `testdata` | Extends: [to research] |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A Write or Edit of a spec file, through the PreToolUse hook

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
| A spec write naming only tooling and a public-website script | → | `daemonProofProblem`, `daemonProofExempt` | [hook golden row, to design] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [to design] | `internal/le/hookruntime/` | A public-website script owes no functional test; a script of the daemon's web UI still does | |

## Files to Modify
- `internal/le/hookruntime/specproof.go` - [to design]

## Implementation Steps

1. **Phase: Red golden rows, both ways** -- [to design]
2. **Phase: The website tree exemption** -- [to design]

## Checklist

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le ai hooks unit`, `./le test unit pkg path ./internal/le/hookruntime/`

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `hook-existing-patterns-false-positive` | 2026-11-12 | `daemonProofExempt`: a public-website script named by a tooling spec judged daemon source |
