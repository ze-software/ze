# Spec: le-own-binary

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release-independent le tooling: le rebuilds whenever any of ze changes, so most commits cost every session a candidate build |
| Depends | spec-le-built-from-head |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-25 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Owner decision, 2026-10-25, while approving `spec-le-built-from-head`:

| Id | Decision | Thomas's words |
|----|----------|----------------|
| O-1 | Splitting le from ze is its own spec, written now and run after `spec-le-built-from-head` closes | "Spec the split, finish this first" |

le is `cmd/ze` built with the `ze_le` tag, so it links 836 first-party packages,
the whole of ze included. Measured 2026-10-24/25: of 54 commits in 24 hours that
changed an input of the le build, 41 touched `internal/le` or `cmd/ze`. The
other 13 touched neither and each still started a candidate build of le from
HEAD (`le`, `le_inputs`, `start_candidate`), shared by every session.

Goal: le is built from its own entry point and links only the packages its
commands need, so a commit that changes ze and nothing le links starts no le
build, and le's input key (`le_inputs` in the `le` script) names only le's own
inputs.

The design, the acceptance criteria and the test plan are written when the spec
leaves skeleton, through `/ze-spec`.

## Required Reading

### Related specs
- [ ] `spec-le-built-from-head` (closed; `docs/contributing/running-commands.md` carries its design) - the dispatcher, the builds in `bin/builds/`, the input key this spec narrows; once it closes, its content lives in `docs/contributing/running-commands.md`, "Launcher builds do not overwrite running binaries"

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - le's run order and personality dispatch (`binaryName`, `LookupRoot`)
- [ ] `docs/architecture/system-architecture.md` - the programs and their build personalities
- [ ] `ai/doors/le-tooling.md` - le's boundaries

## Current Behavior (MANDATORY)

**Source files read:** (read when the spec leaves skeleton)
- [ ] `le` - `le_inputs`, and `build_le`, which builds `./cmd/ze` with `-tags ze_le,<feature tags>`
- [ ] `cmd/ze/main.go` - the personality dispatch from the binary's file name

**Behavior to preserve:**
- The dispatcher, the builds in `bin/builds/` and their names
- `ze le`, le under ze's name

**Behavior to change:**
- le's entry point and what it links; le's input key

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le <command>`

### Transformation Path
1. Written when the spec leaves skeleton

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Script ↔ build | the dispatcher builds le's own entry point | No |

### Integration Points
- The `le` script, `build_le` and `le_inputs`

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | - | written when the spec leaves skeleton |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | le's commands can be linked without the ze packages they do not call | to be measured with `go list -deps` | the split narrows less than hoped | `go list -deps` of the new entry point | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An le command imports a ze package through a registry, so the narrowed build loses it silently | a command missing from `le help` | a test that compares the command trees |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every `./le` call in every session |
| How is it reverted? | Revert the commit; the dispatcher builds the old entry point |
| Who else touches this path? | Every session |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le help` after a commit that changes only ze code le does not link | → | le's narrowed input key | named when the spec leaves skeleton |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A commit that changes only ze code le does not link | No candidate build of le starts |
| AC-2 | `./le help` from the new build | Lists every command the current build lists |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| named when the spec leaves skeleton | - | AC-1, AC-2 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| named when the spec leaves skeleton | - | - | |

## Files to Modify
- `le` - `build_le` and `le_inputs`
- `cmd/ze` - the le personality moves out

## Files to Create
- le's own entry point, named when the spec leaves skeleton

## Implementation Steps

1. **Phase: Wiring** - written when the spec leaves skeleton

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| le from its own entry point | `go list -deps` of it shows no ze package le does not call |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Build inputs | the input key still covers every file the build reads |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 12 | Internal architecture changed? | Yes | `docs/architecture/system-architecture.md`, `ai/doors/le-tooling.md` |

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] The targeted tests for what changed pass: `./le job run ... command go test -race` over `cmd/ze` and the new entry point, and `./le verify changed`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
