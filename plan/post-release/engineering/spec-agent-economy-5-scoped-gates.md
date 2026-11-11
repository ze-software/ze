# Spec: agent-economy-5-scoped-gates

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release-independent development tooling: it changes how two local gates choose what they judge and changes no shipped binary. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 5 of `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`,
which holds the baseline.

Two gates judge the whole tree on every run although each run follows one
small change. `./le repo compiles check` is required after every commit that
carries Go: it exports HEAD and builds five flavors of the whole tree, 401
runs and 11.5 hours in three weeks at a median 107 seconds. `./le rfc check`
judges every row of the corpus and vets every package holding an RFC tag: 439
runs and 4.8 hours at a median 29 seconds.

This is the only child that changes what a gate proves, so it runs last, and
only after the wait record of child 2 shows the time is run time. CI keeps
both whole-tree runs unchanged.

The goal: the local run of each gate judges what the change in hand can
reach, in a fraction of the time, and proves for that change what the
whole-tree run proves.

## Research Findings (2026-11-11, read at the producer by a research agent; re-read at design)

| Finding | Where |
|---------|-------|
| The compile gate exports HEAD with `git archive` into session scratch (about 180MB), then builds five flavors in sequence, each a whole-tree build under its tags plus two package listings | `internal/le/repo/compiles/repositorytrackedbuild.go`, `Run`; `matrix.go`, `buildMatrix` |
| It never reads the working tree, takes no admission slot, and reports per-flavor seconds but not export time. The page says about 45 seconds warm; the median measured is 107 | the same; `docs/contributing/testing.md` |
| A rule requires it after every commit that carried Go | `ai/rules/precommit-verify.md`, points `only-one-check-compiles-what-git-holds` and `land-first-never-a-private-export`; `ai/agents/ze-work.md` |
| What it exists to catch: a committed consumer whose producer is uncommitted, which the working tree hides | `docs/contributing/testing.md`, "The one stage that does not read your working tree" |
| The RFC check takes no keyword and judges the whole corpus, including an unadmitted vet under full tags of every package holding an RFC tag | `internal/le/rfc/check.go`, `check`; `check_compile.go`, `checkTagPackagesCompile`, `goTagPackages` |
| The rule says the check judges every row of the corpus on every run | `ai/rules/rfc-compliance.md` |
| `./le test weakened check` is already scoped to a diff; its 14-second mean is unexplained | `internal/le/test/weakened/audit.go`, `Audit` |
| The reached-package selector exists | `internal/le/repo/changed/selector.go` |
| `spec-native-action-job-admission` (skeleton) would admit these actions, which may add wait | `plan/post-release/engineering/spec-native-action-job-admission.md` |

Unverified, to measure before designing on it:

| Question | Measure |
|----------|---------|
| How the compile gate's 107 seconds split between the export and the five flavors | One run with the structured report, by the main thread |
| What share of the RFC check's 29 seconds is the vet | The same |
| Whether queueing adds to either | The wait record of child 2 |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/le-tooling.md` - the door for `le` and its gates
  → Constraint: a gate's verdict must not depend on the machine or on another session's uncommitted work.
- [ ] `ai/rules/precommit-verify.md` - what a commit owes
  → Constraint: exactly one check compiles what git holds, and it must keep catching a committed consumer of an uncommitted producer.
- [ ] `ai/rules/rfc-compliance.md` - what the RFC check judges
  → Constraint: every row in the corpus is judged somewhere on every pushed commit. A scoped local mode restates that rule; it does not remove it.

**Key insights:**
- A committed consumer can break only on a producer whose file differs between HEAD and the working tree. The packages that import such a producer are the population the compile gate needs locally.
- The whole-corpus runs stay in CI, which verifies every pushed commit.

## Current Behavior (MANDATORY)

**Source files read:** the main thread read one producer; the rest of the Research Findings table is owed a read at design.
- [ ] `internal/le/repo/compiles/matrix.go` - `buildMatrix` holds five flavors (distro, appliance, setup, host, installer), each built over the whole module; its comment prices the set at about 45 seconds warm and a row at about 3 seconds, and each flavor names anchor files so a wrong tag set cannot pass by compiling nothing

**Behavior to preserve:**
- CI runs both gates over the whole tree.
- The compile gate still goes red for a committed consumer of an uncommitted producer.
- The RFC check still refuses, for the change in hand, everything it refuses today.

**Behavior to change:**
- The local default of each gate judges what the change reaches.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Tracked-build gate | `internal/le/repo/compiles` | Builds every flavor of what git holds | Extends: a scoped population for the local run, the whole matrix kept for CI |
| RFC check | `internal/le/rfc/check.go` | Judges the whole corpus | Extends: a `changed` keyword restricting rows and vetted packages to what the change reaches |
| Reached-package selector | `internal/le/repo/changed/selector.go` | Packages and tags a change reaches | Extends: both scoped modes consume it |
| Job admission | `internal/le/job` | Admits heavy work | Does not apply until the wait record shows what admission would cost |

## Data Flow (MANDATORY)

### Entry Point
- `./le repo compiles check` after a commit, and `./le rfc check`.

### Transformation Path
1. The gate asks the selector for the packages the change reaches.
2. It builds or judges that population.
3. CI runs the whole population on the pushed commit.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Local gate ↔ CI | the same action with and without the scope keyword | No |

### Integration Points
- The tracked-build gate and its flavor matrix, the RFC check and its package vet, the reached-package selector.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le repo compiles check` with a committed consumer and an uncommitted producer | → | the scoped population | `TestScopedCompileStaysRedForACommittedConsumerOfAnUncommittedProducer` |
| `./le rfc check changed` | → | the scoped row and package set | `TestScopedRFCCheckJudgesTheRowsTheChangeReaches` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A commit whose consumers all have committed producers | The local compile gate answers green without building the whole tree in five flavors | CI builds the whole matrix |
| AC-2 | A committed consumer of an uncommitted producer | The local compile gate goes red and names both | None |
| AC-3 | `./le rfc check changed` after a change touching one RFC's rows or tagged tests | Judges those rows and vets those packages | Rows the change cannot reach are judged by CI |
| AC-4 | The same change under the whole-corpus run | The scoped run refuses everything the whole run refuses for that change | A refusal owned by another session's uncommitted work is not this change's |
| AC-5 | The gates' structured reports | Each names its phases' seconds, the export included | None |
| AC-6 | The pages and rules that describe the two gates | State the scoped local mode and the whole-tree CI run | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestScopedCompileStaysRedForACommittedConsumerOfAnUncommittedProducer` | `internal/le/repo/compiles/` | AC-2 | |
| `TestScopedRFCCheckJudgesTheRowsTheChangeReaches` | `internal/le/rfc/` | AC-3, AC-4 | |
| others | named at design | AC-1, AC-5 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the compile gate's selftest | `internal/le/repo/compiles/` | The gate still catches its founding defect | |

## Files to Modify
- Named at design, after the measures and the producer reads. The areas are the tracked-build gate, the RFC check, the two rules that describe them and their pages.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the measures, recorded; the failing wiring tests
2. **Phase: the compile gate's local population**
3. **Phase: the RFC check's changed mode**
4. **Phase: rules and pages**

## Open Questions for Design
- For the compile gate: run it asynchronously from the commit script, or scope its population. The measures above decide.
- Whether an RFC-tagged test change makes the scoped RFC mode a change the test judge must see.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] The targeted tests pass, run by the main thread: the packages `internal/le/repo/compiles` and `internal/le/rfc`. No `./le test unit <group>` covers `internal/le`. No agent runs any `./le verify` command

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
