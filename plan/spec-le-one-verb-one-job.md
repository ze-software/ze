# Spec: le-one-verb-one-job

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | plan/spec-le-every-area-dispatches-through-one-table.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`le` has 184 distinct verb words over 200 declared actions. Only seven words
repeat at all (`check`, `selftest`, `update`, `report`, `write`, `run`, `list`).
A reader who learns one area therefore has almost no basis to predict the next,
which is habit 1 of `ai/rules/writing.md` (synonym rotation) on a command
surface rather than in prose.

Three separable defects sit under that number.

**One job, two verbs.** Twelve actions regenerate a committed artifact from its
canonical source, split across two words with nothing to predict which:

| Verb | Areas |
|------|-------|
| `write` | `feature-tags`, `iana-asn`, `plugin imports`, `web-assets`, `yang glue` |
| `update` | `arch-map`, `discovery-index`, `docs-to-code`, `site facts`, `site wiki`, `test-health`, `wiki-catalog` |

The owner ruled on 2026-09-12 that `update` wins and the five `write` areas are
renamed. Ze is pre-release, so `ai/rules/cli.md` requires an outright
replacement: no alias, no deprecation.

**Exit code 2 means four different things.** `leaction` chose 2 for a usage
error specifically so a caller could tell it from a gate that ran and failed,
and its own comment says so. Outside that core the number was reused:

| Producer | What 2 means there |
|----------|--------------------|
| `leaction.refuseVerb`, `refuseValue` | usage error |
| `internal/le/commit/actions.go` | usage error AND execution failure, and it invents 3 for `review-check` not clean |
| `internal/le/rfc/check.go` | "could not run" AND "ran, found real violations"; never uses 1 |
| `internal/le/cligrammar/actions.go` | "the tree the gate could not read", an environment failure |
| `leroot.RefuseArgument` | answers **1** for the same mistake `leaction.refuseValue` answers 2 for |

`internal/le/spec/status/answer.go` uses 1 for the bare form's errors and 2 for
the `closure` sub-verbs' errors, inside one area.

**Help does not fit a terminal.** `internal/core/helpfmt/helpfmt.go` pads the
name column and appends the description with no width limit, and 82 of 200 `Why`
strings exceed 120 characters. The worst is 502 (`scratch cache-clean`), then 501
(`rfc extraction-classify`), 386 (`qemu all-tests`), 376 (`rfc
extraction-create`) and 376 (`rfc reseal`). Each prints as one physical line, so
the terminal soft-wraps it into the description column's left gutter and the
two-column layout collapses.

## Work this spec inherits

`plan/spec-le-publishes-its-command-surface.md` names exit-code discipline, verb
vocabulary and help wrapping as out of its scope. This is that spec.

The rename depends on spec 2: the seven hand-rolled areas must declare their
tables before a verb rename can be verified against a manifest rather than
against a grep.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/cli.md` - governs verb choice and exit codes
  → Constraint: [fill during research: whether the ze action-verb set governs le or only the operator CLI]
- [ ] `ai/rules/writing.md` - habit 1, synonym rotation
  → Constraint: [fill during research]
- [ ] `docs/architecture/cli/root-namespace-grammar.md` - [why relevant]
  → Constraint: [fill during research]

**Key insights:** (minimal context to resume after compaction)
- [fill during research]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/core/helpfmt/helpfmt.go` - pads the name column, appends the description, never wraps
- [ ] `internal/le/leaction/leaction.go` - `refuseVerb` and `refuseValue` answer 2 and say why
- [ ] `internal/le/leroot/leroot.go` - `RefuseArgument` answers 1 for the same mistake

**Behavior to preserve:**
- [fill during research]

**Behavior to change:**
- [fill during research]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `argv` reaching `leroot.Dispatch`; the exit code leaving the process.

### Transformation Path
1. [fill during research]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Area ↔ caller | the process exit status, read by scripts, hooks and CI | No |

### Integration Points
- [fill during research]

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The five `write` areas can be renamed outright with no alias | `ai/rules/cli.md`: Ze is unreleased, so a second spelling is renamed rather than aliased; owner ruling 2026-09-12 | A migration period would be needed | owner ruling already given | CONFIRMED |
| A-2 | Every caller of a renamed verb is findable, so no caller is left naming a verb the registry no longer holds | the manifest spec 1 publishes, plus the citation resolver | A hook, a CI workflow or a generated file silently names the old verb | the citation check over the corpus, run before and after | unvalidated |
| A-3 | No caller branches on the specific value 2 in a way that a split would break | [fill during research] | Splitting the code changes a caller's behavior silently | grep every reader of an `le` exit status | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The rename breaks a citation among the 2,358 in the instruction corpus | an agent running a verb that exits 1 | Land the citation resolver first so the rename is verifiable, not hoped |
| R-2 | Splitting exit 2 changes a gate's verdict in CI | a workflow passing where it used to fail, which is the silent direction | Enumerate every reader of an le exit status before changing any producer |
| R-3 | Wrapping help changes output every golden test asserts | a wave of golden-file failures | Wrap at render time only, and update the goldens in the same change |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A renamed verb breaks every citation of it, including ones inside hooks and CI. A wrongly split exit code turns a red gate green, which is the dangerous direction |
| How is it reverted? | A single commit revert per defect; the three are separable and should land separately |
| Who else touches this path? | Specs 1 and 2 of this series land first. `plan/spec-le-command-namespaces.md` owns le's root naming grammar |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le web-assets update` | → | the renamed action answering where `write` did | `TestTheRegenerationVerbIsUpdateEverywhere` |
| Every cited `./le <area> <verb>` in the corpus | → | the manifest resolver | `TestNoCitationNamesAVerbTheRegistryDoesNotHold` |
| A `Why` string past the terminal width | → | `helpfmt` wrapping at render time | `TestALongWhyWrapsIntoTheDescriptionColumn` |
| A usage error against an execution failure | → | the two distinct exit codes | `TestAUsageErrorAndAnExecutionFailureAnswerDifferentCodes` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Any action that regenerates a committed artifact from its source | The verb is `update`, and no area registers `write` for that job |
| AC-2 | `./le feature-tags write` and the other four old spellings | Refused as an unknown action, with no alias accepted |
| AC-3 | Every `./le` invocation cited in the instruction corpus and in Go string literals | Resolves against the manifest, verb included |
| AC-4 | A usage error in any area | Answers one code, distinct from the code a gate answers when it ran and found a defect |
| AC-5 | An environment failure in any area | Answers a code distinct from both of the above |
| AC-6 | A `Why` string longer than the terminal width | Wraps under the description column, and the name column stays aligned |
| AC-7 | `leroot.RefuseArgument` and `leaction.refuseValue` | Answer the same code for the same mistake |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTheRegenerationVerbIsUpdateEverywhere` | `internal/le/leroot/manifest_test.go` | AC-1, AC-2 | |
| `TestAUsageErrorAndAnExecutionFailureAnswerDifferentCodes` | `internal/le/leaction/leaction_test.go` | AC-4, AC-5, AC-7 | |
| `TestALongWhyWrapsIntoTheDescriptionColumn` | `internal/core/helpfmt/helpfmt_test.go` | AC-6 | |
| `TestNoCitationNamesAVerbTheRegistryDoesNotHold` | `internal/le/workflowcheck/workflowcheck_test.go` | AC-3 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| terminal width for wrapping | 20-500 columns | [fill during design] | [fill during design] | [fill during design] |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | `le` registers no daemon command; the dispatcher tests drive the real entry point from argv | N-A | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | Scope is tooling | N-A | N-A | |

## Files to Modify
- `internal/le/featuretags/`, `internal/le/ianaasn/`, `internal/le/plugin/imports/`, `internal/le/webassets/`, `internal/le/yang/glue/` - rename `write` to `update`
- `internal/core/helpfmt/helpfmt.go` - wrap the description column at render time
- `internal/le/leroot/leroot.go` - `RefuseArgument` answers the code `leaction` answers
- `internal/le/commit/actions.go`, `internal/le/rfc/check.go`, `internal/le/cligrammar/actions.go`, `internal/le/spec/status/answer.go` - one meaning per code
- `ai/`, `docs/`, `.claude/`, `.github/workflows/` - every citation of a renamed verb

## Files to Create
- [fill during research]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI commands/flags | Yes | five verbs renamed |
| CLI grammar (keyword before value) | Yes | `./le cli-grammar` is the gate |
| Pipe completeness | Yes | unchanged: every area already answers structured data |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md`, `ai/INDEX.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/spec-le-one-verb-one-job.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the citation resolver covers verbs, and it is red on the five old spellings
   - Tests: `TestNoCitationNamesAVerbTheRegistryDoesNotHold`
   - Files: `internal/le/workflowcheck/`
   - Verify: the test fails naming the citations a rename will break
2. **Phase: one verb per job** -- rename `write` to `update` in five areas and repoint every citation
3. **Phase: one meaning per exit code** -- enumerate every reader of an le exit status, then split
4. **Phase: help that fits** -- wrap the description column and update the goldens

## Known Limitations
- [fill during design]

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
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
