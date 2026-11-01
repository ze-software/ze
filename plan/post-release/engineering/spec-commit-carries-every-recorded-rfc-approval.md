# Spec: commit-carries-every-recorded-rfc-approval

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Development tooling: a missing `RFC-approved:` trailer loses the record of a judged test change, it does not change the shipped binary |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-01 |

Scope is tooling, so End-to-End User Stories, Interop Tests and RFC
Documentation do not apply.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`./le commit create` wrote ten of eleven recorded `RFC-approved:` trailers in
fd4c866c74. The commit changed eleven RFC-tagged `.ci` files, an independent
`ze-test-judge` approval was recorded for every one, and the commit carries no
line for `plugin.dynamic-peer-gets-group-role-capability`. Nothing refused the
commit. Goal: find why that recorded row was not matched to its unit, fix the
match, and make the gate refuse a commit that changes an RFC-tagged unit and
carries no `RFC-approved:` line (or owed admission) for it.

## Required Reading

### Architecture Docs
- [ ] `docs/contributing/committing.md` - the commit flow, its keywords and refusals
- [ ] `docs/contributing/testing.md` - the approval route for a weakening and an RFC-tagged test
- [ ] `docs/architecture/testing/test-health.md` - the design of the approval files
- [ ] `ai/doors/le-tooling.md` - le boundaries and gates

**Key insights:**
- To be written at design.

## Current Behavior (MANDATORY)

**Source files read:** (to be completed at research)
- [ ] `internal/le/commit/prepare.go` - assembles `RFCApprovals` and the trailers the commit carries (`mergeRFCApprovals`)
- [ ] `internal/le/commit/rfcchange.go` - `rfcChangeProblems` lists changed tag carriers and matches each unit to an approval row
- [ ] `internal/le/rfc/approve.go` - `ApprovalPath`, the `tmp/commit-rfc-approved-<session>.md` file
- [ ] `internal/le/test/weakened/approve.go` - the `tmp/commit-weakening-approved-<session>.md` file

**Behavior to preserve:**
- One `RFC-approved:` line per approved unit the commit uses; used rows are dropped once the commit lands

**Behavior to change:**
- A recorded approval for a changed `.ci` unit is carried as its trailer
- A changed RFC-tagged unit with no trailer and no owed admission refuses the commit

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| RFC change gate | `internal/le/commit/rfcchange.go` `rfcChangeProblems`, `RFCChange.unit` | Lists changed tag carriers, names each changed unit, matches it to a row in the RFC approval file, returns problems for an unapproved one | Extends: the unit naming or matching for a `.ci` carrier dropped one of eleven; fix it here, no second gate |
| Trailer writer | `internal/le/commit/prepare.go` `RFCApprovals`, `mergeRFCApprovals`, `admissionTrailer` | Writes the `RFC-approved:` lines (and owed-admission trailers) the commit message carries | Extends: every used row becomes a line; the gate checks the written set against the changed units |
| RFC approval file | `internal/le/rfc/approve.go` `ApprovalPath` (`tmp/commit-rfc-approved-<session>.md`), written by `./le rfc approve` | Holds the judge's recorded verdict per unit, keyed on the main thread's session | Does not change: it held the row; the reader is at fault |
| Weakening approval file | `internal/le/test/weakened/approve.go` (`tmp/commit-weakening-approved-<session>.md`), `testweakened.Check` | Holds weakening verdicts; an RFC row also admits a weakening, merged by `mergeRFCApprovals` | Does not change; read to confirm the merge did not drop the row |
| Population library | `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` | Owns the wider class of gates judging the wrong population | Does not apply directly: this spec owns one traced symptom; if the cause is the population choice, the fix moves there |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le commit create` over named paths, reading the session's approval files

### Transformation Path
1. `rfcChangeProblems` names the changed RFC-tagged units
2. Each unit is matched to a recorded approval row
3. `prepare.go` writes the matched rows as trailers

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Approval file to commit message | untracked `tmp/` file read from disk | No |

### Integration Points
- To be written at design.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The row for `plugin.dynamic-peer-gets-group-role-capability` was recorded before fd4c866c74 | spec-filter-wire-0-umbrella session state, phase 4e handoff | The defect is in recording, not in matching | Reproduce with a fixture approval file and that `.ci` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A refusal on a missing trailer blocks commits that today pass legitimately | A refusal in a commit with an owed admission | The owed-admission trailer satisfies the check |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Commits refused, or judged test changes landing without their record; nothing user-visible |
| How is it reverted? | Single commit revert |
| Who else touches this path? | spec-gates-share-one-semantic-population (same gate family) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le commit create` over a changed RFC-tagged `.ci` with a recorded approval | → | `rfcChangeProblems`, `mergeRFCApprovals` | `TestCommitCarriesEveryRecordedRFCApproval` (to be written) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A commit changing the eleven tagged `.ci` units of fd4c866c74, each with a recorded approval | The message carries eleven `RFC-approved:` lines, `plugin.dynamic-peer-gets-group-role-capability` among them |
| AC-2 | A commit changing an RFC-tagged unit with no recorded approval and no owed admission | `./le commit create` refuses and names the unit |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCommitCarriesEveryRecordedRFCApproval` | `internal/le/commit/rfcchange_test.go` | AC-1 | |
| `TestCommitRefusesTaggedChangeWithoutTrailer` | `internal/le/commit/rfcchange_test.go` | AC-2 | |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | - | Tooling; the unit tests drive `./le commit create`'s preparation over a fixture repository | |

## Files to Modify
- `internal/le/commit/rfcchange.go` - unit naming or matching for a `.ci` carrier
- `internal/le/commit/prepare.go` - the written trailer set checked against the changed units
- `docs/contributing/committing.md` - the refusal, if it is new

## Files to Create
- None beyond the tests above.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling |
| YANG validation constraints | N-A | tooling |
| YANG custom validators | N-A | tooling |
| CLI commands/flags | No | no new keyword |
| CLI grammar (keyword before value) | N-A | no new keyword |
| Editor autocomplete | N-A | tooling |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output command |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | none |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/committing.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | | to be derived at design |
| 17 | Existing docs show config/CLI/API examples for this area? | No | |

## Implementation Steps

1. **Phase: Wiring** -- reproduce fd4c866c74's population in a fixture, write the failing AC-1 test
2. **Phase: Fix** -- trace the dropped row to its producer, fix it, add the AC-2 refusal

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The trailer count equals the changed tagged units the approval file covers |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| AC-1, AC-2 tests | `./le job run label unit-pkg command go test ./internal/le/commit/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | An approval row naming a unit the commit does not change is not carried |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `plan/journal/check-cannot-see-the-change-it-looks-for.md` | 2026-11-01 | `./le commit create` RFC-approved trailer writer (`internal/le/commit/prepare.go`) and the commit gate |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- To be written at design.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, and `./le verify changed` over the closing commits
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove` the spec, in the same `./le commit create` script
