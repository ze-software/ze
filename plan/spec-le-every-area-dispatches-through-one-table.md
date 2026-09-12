# Spec: le-every-area-dispatches-through-one-table

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Depends | plan/spec-le-publishes-its-command-surface.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-09-12 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`leaction.Area` is the declared dispatcher for an `le` area: it owns the verb
table, the bare listing, the trailing help word, the closed keyword grammar and
the refusal messages. Sixty-three areas use it. Seven do not, and each of those
seven open-codes a private copy of the same four jobs.

`plan/journal/helper-bypassed-by-an-open-coded-copy.md` holds 33 rows of exactly
this shape. These seven are the same class on le's own command surface.

| Area | Producer | Arms | What the copy costs |
|------|----------|------|---------------------|
| `commit` | `internal/le/commit/actions.go` | 9 | Its own `commandVerbs` table, its own `parseKeywords`, its own `commandError`. `commit create` declares 21 keywords in a private map inside `parseCreate`, reachable from no help surface. `--help` after any verb exits 2 with `unknown keyword "--help"`. This is the command `CLAUDE.md` makes mandatory for every commit |
| `spec session` | `internal/le/spec/session/actions.go` | 18 | Three levels of nested dispatch and a third error grammar, an EBNF brace dump matching neither `leaction`'s nor `commit`'s. A bare invocation prints zero bytes and exits 0 |
| `stress-repro` | `internal/le/stressrepro/actions.go` | 11 keyword arms, 1 verb | No help check anywhere, so `run suite --help` sets `suite=--help` and starts the burn. Spec 1 guards it at the dispatcher; the grammar stays unpublished until this spec |
| `weekly` | `internal/le/weekly/answer.go` | 1 implicit, unnamed | No verb dispatch at all: every word is a keyword arm. Exit 1 for every failure, usage and execution alike |
| `go-extract` | `internal/le/goextract/answer.go` | 1 implicit, unnamed | Same shape as `weekly` |
| `spec status` | `internal/le/spec/status/answer.go` | 3 | The bare form uses exit 1 for its errors while both `closure` sub-verbs use exit 2 for theirs, in one area |
| ~~`verify status`~~ | `internal/le/verify/status/answer.go` | 4 | DONE 2026-09-12, in `plan/spec-le-publishes-its-command-surface.md`. Its `check` verb accumulates repeated `path <value>` pairs, so migrating it was what gave `leaction.Parameter.Repeat` and `Arguments.Values` a production caller and closed that spec's unwired-symbol finding. Six areas remain |

Spec 1 publishes the surface and stops a help word from running work. It cannot
publish what these seven declare, because they declare no table. This spec makes
the table universal, which makes the manifest complete, the help guard able to
answer rather than merely refuse, and `leroot.RegisterActions` mandatory rather
than optional.

The conversion inventory is already measured and carries no blocker:

- **No positional values anywhere.** Every value in all seven areas already
  follows an explicit keyword. `goextract` and `weekly` state that choice in
  their own doc comments.
- **Every payload is already structured data.** No area answers pre-rendered
  text, so `leroot.Run` needs no change.
- **Two areas need their sub-verbs flattened.** `spec session` nests three deep
  (`state {current|latest}`, `review {hash|record|check}`, `model current`) and
  `spec status` nests one (`closure {list|check}`). `commit` already flattens by
  hand (`debt-list`, `debt-status`, `debt-clear`, `debt-discharge`), which is the
  pattern to follow.
- **One genuine conflict.** `spec status` answers DATA for a bare invocation,
  where `leaction.Area.Answer` answers the action listing. That bare form needs a
  named verb.

## Work this spec inherits

Spec 1's Known Limitations name two items that land here:

- The seven areas are guarded against a trailing help word but their grammar is
  not published, because they have no action table.
- `leroot.RegisterActions` is optional while an area declares no table. The test
  `TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList`
  (`internal/le/actions_test.go`) holds `areasWithoutAnActionTable`, and it
  ratchets both ways: an unlisted area with no table fails, and a listed area
  that starts publishing fails until its row is deleted.

**The list is 26, not 7.** Measured 2026-09-12: 89 registered areas, 63 wired.
The seven above are the multi-verb hand-rolled dispatchers. Nineteen more
declare no table for a different reason, and this spec owes a decision about
each group:

| Reason | Areas |
|--------|-------|
| Refuses every argument by hand, one implicit action | `cli-grammar`, `command ownership`, `config claims`, `consistency`, `digest`, `gokrazy-gosum`, `iface-resolution`, `tracked` |
| Declares its own `ActionList` type, as `stress-repro` does | `doc check`, `docvalid`, `test-helper`, `verify summary` |
| Builds a `leaction.List` inline from an unexported `actions()` | `job`, `verify lock` |
| Single-verb report with no grammar | `command list`, `inventory`, `spec citation`, `token-economy`, `working-tree` |

A single-verb area still has a verb, and several of them do take keywords, so
"it needs no table" is a claim to test against each body rather than assume. The
four that declare their own `ActionList` are the clearest case: that type is a
second declaration of the same fact `leaction` already holds.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - [why relevant]
  → Constraint: [fill during research]
- [ ] `ai/rules/cli.md` - [why relevant]
  → Constraint: [fill during research]

**Key insights:** (minimal context to resume after compaction)
- [fill during research]

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/actions.go` - [fill during research]

**Behavior to preserve:**
- [fill during research]

**Behavior to change:**
- [fill during research]

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- [fill during research: argv reaching each of the seven areas' `Answer`]

### Transformation Path
1. [fill during research]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Dispatcher ↔ area | `leroot.Answer` plus the actions provider spec 1 added | No |

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
| A-1 | No area of the seven takes a positional value | conversion inventory, 2026-09-12, read across all seven parsers | The flat keyword model cannot describe them | re-read each parser during research | unvalidated |
| A-2 | Every payload of the seven is already structured data | conversion inventory, 2026-09-12 | `leroot.Run` would need a change | compile and round-trip each payload | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `commit` is the command every session must use, so a regression here stops every peer session committing | a commit script failing to generate | Migrate `commit` last, behind the six smaller areas, and keep its exit codes byte-identical |
| R-2 | Flattening `spec session`'s sub-verbs renames commands that `CLAUDE.md`, the rules corpus and the skills all cite | a citation naming a verb the registry no longer holds | The manifest from spec 1 makes the citation check buildable; run it over the corpus before and after |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Development commands for every session sharing the checkout. `commit` and `spec session` are on the path every session takes |
| How is it reverted? | A single commit revert per area, if each area lands as its own commit |
| Who else touches this path? | `plan/spec-le-publishes-its-command-surface.md` lands first and this spec depends on it |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le commit create --help` | → | `leaction.Area.actionUsage` rendering commit's 21 keywords | `TestCommitCreatePublishesItsKeywordGrammar` |
| `./le stress-repro run --help` | → | the dispatcher rendering the run grammar from the registered table | `TestStressReproPublishesItsRunGrammar` |
| `./le \| json` | → | `leroot.Manifest` carrying all 89 areas with no exemption | `TestManifestCoversEveryAreaWithNoMigrationList` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le commit create --help` | Renders all 21 keywords, marking `subject` required and `body`, `file`, `file-list`, `remove`, `remove-list` repeatable, and exits 0 |
| AC-2 | Each of the seven areas, bare | Answers its action listing in the same shape every other area answers |
| AC-3 | `./le \| json` | Names every registered area's actions, with no area exempt |
| AC-4 | `TestEveryRegisteredAreaProvidesActionsOrIsOnTheMigrationList` | The migration list is empty and the exemption branch is deleted |
| AC-5 | Every verb of the seven | Keeps the exit code it returns today, because exit-code discipline is a later spec |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCommitCreatePublishesItsKeywordGrammar` | `internal/le/commit/actions_test.go` | AC-1 | |
| `TestManifestCoversEveryAreaWithNoMigrationList` | `internal/le/leroot/manifest_test.go` | AC-3, AC-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | this spec adds no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | `le` registers no daemon command; the dispatcher tests drive the real entry point from argv | N-A | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | Scope is tooling | N-A | N-A | |

## Files to Modify
- `internal/le/commit/actions.go` - declare the table, delete `commandVerbs`, `parseKeywords` and `commandError`
- `internal/le/spec/session/actions.go` - flatten the nested sub-verbs onto one table
- `internal/le/stressrepro/actions.go` - declare `run` and its ten keywords
- `internal/le/weekly/answer.go` - name the implicit action
- `internal/le/goextract/answer.go` - name the implicit action
- `internal/le/spec/status/answer.go` - name the bare form's verb, flatten `closure`
- `internal/le/verify/status/answer.go` - declare the four verbs
- `internal/le/leroot/leroot.go` - make the actions provider mandatory

## Files to Create
- [fill during research]

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| CLI grammar (keyword before value) | Yes | `./le cli-grammar` is the gate |
| Pipe completeness | Yes | unchanged: every area already answers structured data |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md`, `docs/contributing/committing.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: `./le spec citation anchors spec plan/spec-le-every-area-dispatches-through-one-table.md` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the smallest area declares its table and the manifest sees it
   - Tests: `TestManifestCoversEveryAreaWithNoMigrationList`, failing on six remaining exemptions
   - Files: `internal/le/verify/status/answer.go`
   - Verify: the migration list shrinks by one
2. **Phase: the five simple areas** -- `verify status`, `spec status`, `go-extract`, `weekly`, `stress-repro`
3. **Phase: `spec session`** -- flatten three levels onto one table
4. **Phase: `commit`** -- last, because every session depends on it
5. **Phase: the exemption is deleted** -- the actions provider becomes mandatory

## Known Limitations
- Verb vocabulary, exit-code discipline and help-text wrapping are not this
  spec's subject. They are `plan/spec-le-one-verb-one-job.md`.

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
- [ ] AC-1..AC-5 all demonstrated
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
