# Spec: le-command-namespaces

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-08-28 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`le` registers 86 root commands (`le help`, counted from the five group
sections). Twenty-one of them hyphenate an object to one of its members:
`verify-lint`, `spec-session`, `doc-check`, and so on.

`docs/architecture/cli/command-namespacing.md`, section "Token naming: hyphen
for one name, space for a namespace", says a hyphen joins words that name one
indivisible thing. When the left part is an object with members, the object
becomes its own token. `docs/architecture/cli/root-namespace-grammar.md`
records that ze's ROOT namespace drifted the same way, that
`grammar.CheckRootNamespace` was the fix, and that the load-bearing deliverable
was the gate rather than the renames. That gate reads ze's YANG command tree.
It has never seen le.

The goal is three things. Split the nine object families into namespaces. Move
each command's package to the directory its new name predicts. Add the feeder
that keeps le's root surface from drifting back.

The design is decided. This spec records it and works out its mechanical
consequences.

  → Constraint: a namespace member stays a SEPARATELY REGISTERED command, at
    its own full path, with its own group and its own directory. The family
    does not become one root that sub-dispatches its members internally.
    `TestGateStagesAreNotWorkflowOrReport` forces this: `verify` is a workflow
    command and `verify-lint` is a gate, so a merged root could declare only
    one group and the test would fail with no way to state the truth. Five of
    the nine families mix groups the same way. The merge is the obvious
    simplification and it is the one this spec exists to refuse; Key Design
    Decisions carries the full reasoning and the count that follows from it.

### The nine families

| Today | Becomes |
|-------|---------|
| verify-deps, verify-lint, verify-lock, verify-status, verify-summary | `le verify deps`, `lint`, `lock`, `status`, `summary`. `le verify` keeps worktree, current, list |
| spec-citation, spec-session, spec-status | `le spec citation`, `session`, `status` |
| yang-glue, yang-migration, yang-leaf-mentions | `le yang glue`, `migration`, `leaf-mentions` |
| config-claims, config-coercion | `le config claims`, `coercion` |
| plugin-boundary, plugin-imports | `le plugin boundary`, `imports` |
| command-list, command-ownership | `le command list`, `ownership` |
| doc-check, doc-wiring | `le doc check`, `wiring` |
| site-facts | `le site facts`. `le site` keeps its six verbs |
| repository-tracked-build | `le repository tracked-build`. `le repository` keeps its four verbs |

`leaf-mentions` and `tracked-build` stay hyphenated. Each names one thing.

Twenty-seven roots keep their hyphen because the left part names no object.
This is the doc's own "a shared prefix is not a namespace" trap: dash-stdio,
arch-map, build-artifacts, ci-dispatch, cli-grammar, discovery-index,
feature-tags, fs-persistence, go-extract, gokrazy-gosum, hook-check,
htmx-upgrade, iana-asn, iface-resolution, perf-bench, platform-vet,
port-defaults, protocol-skeleton, source-rewrite, staticcheck-feature-matrix,
stress-repro, terminal-demo, token-economy, vendor-web, web-assets,
wiki-catalog, working-tree.

The six `test-*` commands are the deliberate exception. The reasoning is in Key
Design Decisions.

### Open question for the owner

`docvalid` and `docs-to-code` belong to the `doc` object, and neither name
survives the move. `le doc valid` and `le doc to-code` both read badly, so both
need a rename, which is a separate decision. This spec leaves them as roots and
does not answer it. Neither detector in the new gate flags them, so leaving
them costs no false red: `docvalid` carries no hyphen, and `docs-to-code` has
the left segment `docs`, which is neither a root nor shared with another root.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/cli/command-namespacing.md` - the shape rule this spec applies
  → Constraint: a hyphen joins words naming one indivisible thing. An object with members takes its own token.
  → Constraint: a shared prefix is not a namespace. `flow-export` and `flow-recent` share a word by coincidence and stay compound.
- [ ] `docs/architecture/cli/root-namespace-grammar.md` - the same rule over a root surface, and the precedent to follow
  → Decision: object roots with a closed-keyword sub-dispatch, as `ze traffic`, `ze isis` and `ze ospf` do. Bare `ze isis` enumerates its members.
  → Decision: the gate is the load-bearing deliverable, not the renames.
  → Constraint: a renamed token is not a renamed identity. `traffic-control` is also a config identity and that spelling stayed.
- [ ] `ai/rules/cli.md` - R9 and the sibling-collision check the feeder extends
  → Constraint: keyword before value. A namespace member is a keyword, so it never introduces a free-form value.
- [ ] `docs/architecture/core-design.md` - the design document `leroot`, `leaction` and every command package declares. It carries le's composition, the directory rule and the group ladder, so the rule this spec changes is stated there
  → Constraint: the package sits at the path its command name predicts. This spec extends that rule to a nested path rather than replacing it.
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - the design document `verifyengine` and the verify command declare. It carries the fixed stage population and the freshness certificate
  → Constraint: the stage population is declared, not discovered. Renaming a stage's command changes a declared row, so the population file moves with the family.

### RFC Summaries (Scope: protocol)

Not applicable. No wire protocol changes.

**Key insights:**
- The existing feeder builds its namespace set from ze's YANG verbs and containers. le's namespaces are in neither set, so feeding le's roots to the existing check would find nothing. le needs its own namespace set.
- Under the split, `le verify` becomes a word-prefix of five other commands. Longest-prefix resolution already exists in the registry, so the sub-dispatch is the registry itself rather than new machinery.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/leroot/dispatch.go` - `Dispatch` builds a two-word lookup `{"le", args[0]}` and refuses any result carrying trailing words. `Usage` and `usageSections` render the commands grouped by group.
- [ ] `internal/le/leroot/leroot.go` - `Register`, `CommandPath`, `RegisterShape`, `Owns`, `Owned`, `Run`. `CommandPath` joins the prefix `le ` to the name, so a name holding a space already yields a three-word path.
- [ ] `internal/le/leroot/group.go` - `GroupOf` and `setGroup` key a group by command NAME, and the five groups are declared at each registration site.
- [ ] `internal/component/command/registry/registry.go` - `LookupLocalData` matches the longest registered prefix and returns the words that follow it. `HasLocal` matches an exact path. `ListLocal` sorts by full path.
- [ ] `internal/component/command/answer_shape.go` - `ShapeForCommand` resolves by the longest registered command path that is a prefix of the asked path.
- [ ] `internal/le/group_test.go` - `TestEveryCommandIsFoundAtThePathItsNamePredicts` maps a name to a directory by removing hyphens, and scans one directory level back the other way.
- [ ] `internal/le/completeness_test.go` - `areaFloor = 80` and `actionWordFloor = 200`, read from `leroot.Commands()` and `Meta.ResolveSubs()`.
- [ ] `internal/le/completeness_record_test.go` - `portedProducer` holds Target, Area, Verb, Note. Thirty rows name an area this spec renames.
- [ ] `internal/le/verify/engine/stages.go` - 43 stages, each an Identity of one Command plus Args. `stage` joins the name with a slash.
- [ ] `internal/le/verify/engine/run.go` - `stageLogPath` replaces every slash in a stage name with a hyphen before it writes the log file name.
- [ ] `internal/le/verify/dispatch/dispatch.go` - `lookupTool` repeats the two-word lookup and the empty-trailing refusal. `dispatch` calls `leroot.Owns(identity.Command)` and `leroot.Run(identity.Command, ...)`.
- [ ] `internal/le/leaction/leaction.go` - `Area.Answer` dispatches args[0] against a closed verb table and refuses an unknown verb with code 2.
- [ ] `internal/component/command/grammar/checker.go` - `CheckRootNamespace` walks each hyphen boundary of a root and flags a left segment present in the namespace set.
- [ ] `internal/le/cligrammar/cligrammar.go` - builds that namespace set from ze's YANG verbs and containers, applies a roots floor, and honors `rootNamespaceExempt`.
- [ ] `internal/le/wikicatalog/catalog.go` - skips every registry path with the `le ` prefix, so the generated catalog is out of scope.

**Behavior to preserve:**
- Every stage log file name. `stageLogPath` flattens slashes to hyphens, so `verify/deps/alloc` and today's `verify-deps/alloc` both produce `verify-deps-alloc`. No verification artifact path may move.
- Every exit code. `leaction` answers 2 for an unknown verb and `leroot.RefuseArgument` answers 1 for an unwanted value.
- `le verify list mode full`, which `.github/workflows/verify.yml` reads.
- The nineteen `le hook-check <action>` entries in `.claude/settings.json`. None of them names a renamed command.
- The five help group titles and their order.

**Behavior to change:**
- Twenty-one command names gain a space in place of a hyphen.
- `Dispatch` resolves a command of more than one word, and answers a bare namespace token by listing its members.
- Twenty-one packages move under a parent directory, and `internal/le/verify/engine` becomes `internal/le/verify/engine`.

## Data Flow (MANDATORY)

### Entry Point
- A developer types `./le verify lint run`, or a stage runner asks for the same command in process.
- Format at entry: `os.Args[1:]`, a slice of words, reaching `leroot.Dispatch` through `internal/le/register.go` `run`.

### Transformation Path
1. `Dispatch` prepends `le` to the words and asks `registry.LookupLocalData` for the longest registered prefix, bounded at three words.
2. The registry answers the handler for `le verify lint` and the trailing words `[run]`.
3. `Dispatch` rebuilds the matched NAME from the consumed words and calls `leroot.Run(name, handler, trailing, ...)`.
4. `Run` splits the pipe chain, resolves the chain against the same registry, calls the tool, and renders the payload.
5. The tool's `leaction.Area` dispatches `run` against its own closed verb table.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| argv to registry | words joined by a single space into a registered path | No |
| registry to tool | trailing words become the tool's own argv | No |
| verify engine to registry | `verifydispatch.lookupTool` resolves `Identity.Command` as a path | No |
| name to directory | `group_test` predicts a nested path from a spaced name | No |

### Integration Points
- `registry.LookupLocalData` already returns trailing words. Nothing in it changes.
- `command.ShapeForCommand` already resolves by longest prefix. Nothing in it changes, and that is the hazard R-6 records.
- `leroot.CommandPath` already accepts a spaced name. Nothing in it changes.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | |
| Zero-copy preserved where applicable | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|-----------|-------|----------|--------------|--------|
| A-1 | A command name holding a space needs no change in `Register`, `CommandPath`, `RegisterShape` or `Owns` | `CommandPath` concatenates the prefix and the name, and `HasLocal` compares whole paths | every registration site needs a second field for the namespace | `TestNamespacedCommandRegistersAtItsFullPath` | unvalidated |
| A-2 | `leroot.Commands()` still answers one entry per registered command, so the count stays 86 and `areaFloor = 80` holds | `Commands` filters `ListLocal` by the `le ` prefix and strips it | the floor must fall to 65 and the record's Area column changes meaning | `TestLiveRegistryHoldsEveryLeCommand` | unvalidated |
| A-3 | Stage log file names do not move | `verifyengine.stageLogPath` replaces every slash with a hyphen before building the name | every verification artifact path changes and the failure index breaks | `TestStageLogNamesAreUnchangedByNamespacing` | unvalidated |
| A-4 | `.claude/settings.json` needs no edit | all 19 hook entries call `le hook-check <action>`, which is not renamed | every session hook breaks at once | grep of the file | unvalidated |
| A-5 | The generated wiki catalog needs no edit | `wikicatalog/catalog.go` skips every path with the `le ` prefix, and a namespaced path still carries it | the catalog gains 21 rows and its check goes red | `./le wiki-catalog check` |  unvalidated |
| A-6 | Two words is the deepest le command | every family in the table is one object and one member | the three-word bound in `Dispatch` refuses a real command | `TestNoRegisteredLeCommandExceedsTwoWords` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A bare token sweep rewrites a name that is not an le command. Twenty-five occurrences share a spelling: `ze-bgp:command-list`, `ze-plugin:command-list` and `ze-system:command-list` are wire methods, and `doctor-config-claims-unavailable` is a diagnostic code | a YANG `ze:command` line or `internal/core/diagnostic/codes.go` appears in the diff | the sweep matches the whole quoted token only, and the diff of every file outside `internal/le` is read by hand. Same failure as `plan/journal/bulk-rename-corruption.md`, 2026-08-28 |
| R-2 | A textual sweep turns an argv word into one word holding a space. 151 exact string literals across 43 Go files are argv words or name constants | `no such action in verify: deps` from a fixture | each literal is split into two argv words, never rewritten in place |
| R-3 | `changedStages` switches on three literal stage names and its default branch appends. A stale literal does not error, it silently stops excluding the stage | the changed mode gets slower and runs the allocation pass | `TestChangedModeExcludesTheFullOnlyStages` asserts the three are absent by identity, not by string |
| R-4 | A commit whose file set is chosen by "files this change touched" splits a Go package across the boundary, and HEAD stops compiling. `plan/journal/concurrent-session-corruption.md` carries the 2026-08-28 row where exactly that left five of six build flavors red | `./le repository tracked-build check` red on a commit that names none of the broken paths | every commit's file set is closed over the packages it touches, and the COMMIT is compiled through `./le verify worktree` before the next one starts |
| R-5 | Longest-prefix resolution over the whole argv can capture a free-form value. `le job run label x command le verify lint` offers nine words to the matcher | a command runs with the wrong arguments and no error | the lookup is bounded at three words, which is one more than the deepest registered path |
| R-6 | `ShapeForCommand` resolves by longest prefix, so a member that declares no shape inherits its namespace root's shape instead of defaulting to undeclared | a pipe operator is refused on a member that should support it | `TestEveryLeCommandDeclaresItsOwnAnswerShape` |
| R-7 | 304 files carry the header `Code generated by ./le yang glue write` and 40 carry the `plugin-imports` form. Sweeping them by text leaves the generator disagreeing with its output | `./le yang glue check` red on files nobody edited | the generator's string changes and the artifacts are rewritten by their own write action |
| R-8 | A future action word on a namespace root would be shadowed by a member of the same name | none, until the action silently stops running | `TestNoMemberShadowsItsNamespaceRootVerb` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every development and verification entry point in the repository. No shipped binary behavior changes: `le` is a development tool, and `cmd/ze`'s product commands are untouched |
| How is it reverted? | Per family. Each family is one commit carrying its package move, its registrations, its call sites and its regenerated artifacts, so a single revert restores one family |
| Who else touches this path? | Every concurrent session in this checkout. `le` is how they build, test, verify and commit, so a half-landed family is felt immediately by people who did not ask for it |

Measured citation counts, by tree, for the 21 renamed names.

| Surface | Files | What holds them |
|---------|-------|-----------------|
| `ai/` | 115 | rules, skills, indexes. 54 of them are rule points under `ai/rules/points/`, which render into `ai/rules/*.md`, `INDEX.md`, `TRIGGERS.md` and `CORE.md` |
| `docs/` | 28 | contributing guides and architecture pages |
| `.claude/` | 22 | generated copies of `ai/skills/`, gitignored, rewritten by `./le ai skills-sync` |
| `.github/` | 1 | one line, `workflows/govulncheck.yml:31`, which runs `./le verify deps vulnerability` |
| `plan/journal`, `plan/learned`, templates | 27 | durable rows that outlive every spec |
| Go and test trees | 491 | 345 generated, 146 hand-edited |

Generated artifacts that are rewritten rather than edited: `ai/PACKAGE-MAP.md`
(22 rows), `ai/DOCS-TO-CODE.md` (105 rows), `ai/CODE-TO-DOCS.md` (23 rows), and
344 `Code generated by` headers.

Fixed populations that name a renamed command: 17 of the 43 rows in
`internal/le/verify/engine/stages.go`, and 30 of the 225 rows in
`internal/le/completeness_record_test.go`.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le verify lint run` from argv | → | `leroot.Dispatch` longest-prefix resolution, then `verifylint` `Answer` | `TestDispatchResolvesATwoWordCommand` |
| `./le spec` with no member | → | `leroot.Dispatch` namespace-listing branch | `TestBareNamespaceTokenListsItsMembers` |
| a verify stage Identity naming `verify deps` | → | `verifydispatch.lookupTool` and `leroot.Run` | `TestVerifyDispatchResolvesANamespacedStage` |
| `le help` | → | `leroot.Usage` and `usageSections` | `TestEveryToolDeclaresARenderedGroup` |
| a command name | → | the directory that registers it | `TestEveryCommandIsFoundAtThePathItsNamePredicts` |
| `./le cli-grammar` | → | the le root-namespace feeder | `TestLeRootNamespaceFeederFlagsAHyphenatedObjectRoot` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le verify deps vulnerability`, `verify lint run`, `verify lock`, `verify status check`, `verify summary` | each runs what its hyphenated name ran, and `le verify worktree`, `current` and `list` still run their own verbs |
| AC-2 | `./le spec citation`, `spec session current`, `spec status` | each runs what its hyphenated name ran |
| AC-3 | `./le yang glue check`, `yang migration`, `yang leaf-mentions` | each runs what its hyphenated name ran, and `leaf-mentions` keeps its hyphen |
| AC-4 | `./le config claims`, `config coercion check` | each runs what its hyphenated name ran |
| AC-5 | `./le plugin boundary check`, `plugin imports check` | each runs what its hyphenated name ran |
| AC-6 | `./le command list`, `command ownership` | each runs what its hyphenated name ran |
| AC-7 | `./le doc check verify`, `doc wiring` | each runs what its hyphenated name ran |
| AC-8 | `./le site facts check` | runs what `site-facts check` ran, and `le site`'s six verbs still run |
| AC-9 | `./le repository tracked-build check` | runs what `repository-tracked-build check` ran, and `le repository`'s four verbs still run |
| AC-10 | argv of more than two words after `le` | `Dispatch` resolves the longest registered prefix, bounded at three words including `le`, and passes only the trailing words to the tool |
| AC-11 | `./le spec` with no member word | lists the three members with their descriptions and exits 0. An unknown first word still prints `unknown command` and exits 1 |
| AC-12 | any registered command name | its package is at the directory the name predicts, one directory per word with hyphens removed, and every directory holding a `register.go` under `internal/le` is predicted by some name. Both directions hold to a depth of two words |
| AC-13 | a hyphenated le root whose left segment is a registered root, or is shared by two or more roots | `./le cli-grammar` reports it as an R9 finding, unless the segment is recorded as an exception with a reason. `test` is the only exception |
| AC-14 | a full verification run | every stage writes the same log file name it wrote before the rename, and the failure index names the same stages |
| AC-15 | any registered le command | it declares its own answer shape, so no member inherits its namespace root's shape through prefix resolution |
| AC-16 | any registered le command that is a word-prefix of another | the following word is not one of its own action verbs |
| AC-17 | `internal/le/verify/engine` | is at `internal/le/verify/engine`, and `internal/le/gaterun` has not moved |

## End-to-End User Stories

Scope is a development tool, so this section is the developer's own path.

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | runs `./le verify worktree` over a commit | verify area, `verifyengine` stage population, `verifydispatch` per-stage resolution, each namespaced stage command | `TestVerifyDispatchResolvesANamespacedStage` |
| 2 | types `./le spec` to find the spec commands | `Dispatch` namespace listing | `TestBareNamespaceTokenListsItsMembers` |
| 3 | reads `le help` and opens the package behind a command | help entry name, then the directory the name predicts | `TestEveryCommandIsFoundAtThePathItsNamePredicts` |
| 4 | adds a hyphenated root that hides a namespace | `./le cli-grammar` le feeder | `TestLeRootNamespaceFeederFlagsAHyphenatedObjectRoot` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDispatchResolvesATwoWordCommand` | `internal/le/leroot/dispatch_test.go` | AC-10. A two-word registration wins over its one-word prefix, and only the trailing words reach the tool | |
| `TestDispatchBoundsTheLookupAtThreeWords` | `internal/le/leroot/dispatch_test.go` | R-5. A free-form value beyond the third word is never consumed as a command word | |
| `TestDispatchKeepsThePipeWordOutOfTheLookup` | `internal/le/leroot/dispatch_test.go` | `le verify deps \| json` resolves the member and leaves the chain to `Run` | |
| `TestBareNamespaceTokenListsItsMembers` | `internal/le/leroot/dispatch_test.go` | AC-11, both branches: a namespace token lists and exits 0, an unknown token refuses and exits 1 | |
| `TestNamespacedCommandRegistersAtItsFullPath` | `internal/le/leroot/leroot_test.go` | A-1. `Register`, `CommandPath`, `RegisterShape` and `Owns` accept a spaced name | |
| `TestRegisterRefusesAMalformedName` | `internal/le/leroot/leroot_test.go` | a leading, trailing or doubled space panics at init rather than registering an unreachable path | |
| `TestEveryLeCommandDeclaresItsOwnAnswerShape` | `internal/le/group_test.go` | AC-15, R-6 | |
| `TestNoMemberShadowsItsNamespaceRootVerb` | `internal/le/group_test.go` | AC-16, R-8. Read from `Meta.ResolveSubs()` of the prefix command | |
| `TestNoRegisteredLeCommandExceedsTwoWords` | `internal/le/group_test.go` | A-6, and the bound AC-10 relies on | |
| `TestEveryCommandIsFoundAtThePathItsNamePredicts` | `internal/le/group_test.go` | AC-12. Existing test, rewritten for a nested path in both directions | |
| `TestLiveRegistryHoldsEveryLeCommand` | `internal/le/completeness_test.go` | A-2. The floor still describes the population it guards | |
| `TestVerifyDispatchResolvesANamespacedStage` | `internal/le/verify/dispatch/dispatch_test.go` | the stage runner resolves a spaced Command and refuses an unregistered one | |
| `TestStageLogNamesAreUnchangedByNamespacing` | `internal/le/verify/engine/run_test.go` | AC-14, A-3. Asserts the exact file names for the five namespaced stage identities | |
| `TestChangedModeExcludesTheFullOnlyStages` | `internal/le/verify/engine/stages_test.go` | R-3. Absence asserted by identity, so a stale literal fails | |
| `TestLeRootNamespaceFeederFlagsAHyphenatedObjectRoot` | `internal/le/cligrammar/cligrammar_test.go` | AC-13, red then green: a fixture root `verify-lint` is flagged, and the post-rename population is clean | |
| `TestLeRootNamespaceExceptionNeedsAReason` | `internal/le/cligrammar/cligrammar_test.go` | AC-13. An exception with no reason is refused, so the list cannot become an escape hatch |  |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| words offered to `LookupLocalData`, including `le` | 1 to 3 | 3 | N/A | a fourth word is never joined into a candidate path |
| command name words | 1 to 2 | 2 | an empty name panics | a third word fails `TestNoRegisteredLeCommandExceedsTwoWords` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-namespace-dispatch` | `test/ui/le-namespace-dispatch.ci` | a developer runs a namespaced command, a bare namespace token, and an unknown token, and sees the right answer and exit code for each | |
| `le-spec-status-answers` | `test/ui/le-spec-status-answers.ci` | existing test, its invocation becomes two argv words | |
| `verify-scope-freshness-scoped` | `test/runner/verify-scope-freshness-scoped.ci` | existing test, its `verify-status` invocations become two argv words | |

### Interop Tests (Scope: protocol)

Not applicable. No wire protocol changes.

## Files to Modify

Counts with their exceptions, rather than a row for each of 491 files.

- `internal/le/leroot/dispatch.go` - `Dispatch` resolves a bounded longest prefix, rebuilds the matched name, and gains the namespace-listing branch. `Usage`'s doc comment says eighty-six commands and must be corrected.
- `internal/le/leroot/leroot.go` - `Register` validates the name's words. `CommandPath`, `RegisterShape`, `Owns` and `Run` need no body change, and their doc comments state that a name can carry a namespace word.
- `internal/le/verify/dispatch/dispatch.go` - `lookupTool` resolves a spaced `Identity.Command` as a full path instead of a one-word one.
- `internal/le/verify/engine/stages.go` - 17 of 43 stage rows name a renamed command. `changedStages` matches its three exclusions by identity.
- `internal/le/group_test.go` - the directory rule becomes a nested path in both directions, bounded at two levels.
- `internal/le/completeness_record_test.go` - 30 rows gain a space in their Area column.
- `internal/le/completeness_test.go` - the message wording, which says "areas" of a population that now holds members. The floors are unchanged, see Key Design Decisions.
- `internal/le/register.go` - 21 blank imports change path, plus `verifyengine`.
- `internal/le/cligrammar/cligrammar.go` - the new le feeder, its two detectors, its exception list and its own roots floor.
- 21 `register.go` files - one command name each.
- 146 hand-edited Go files, of which 43 hold the 151 exact-string argv literals and name constants.
- `ai/rules/points/` - 54 point files, then `./le rules render-update`, `index-update` and `condensed-update`.
- `ai/skills/` and `ai/INDEX.md` - then `./le ai skills-sync` for the 22 gitignored copies.
- `docs/` - 28 files, including `docs/architecture/cli/root-namespace-grammar.md`, which records that le was never covered and must now record the le feeder.
- `plan/journal/` and `plan/learned/` - 27 durable files.
- `.github/workflows/govulncheck.yml` - one line.

## Files to Create
- `internal/le/verify/engine/` - `internal/le/verify/engine` moved, losing the word invented because `verify` was taken.
- `internal/le/verify/dispatch/` - `internal/le/verify/dispatch` moved, for the same reason.
- 21 moved packages, one per renamed command, at the path its new name predicts.
- `test/ui/le-namespace-dispatch.ci` - the functional test above.

`internal/le/gaterun` does NOT move. See Key Design Decisions.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le registers local data handlers, not YANG commands |
| YANG validation constraints | N-A | no YANG leaf changes |
| YANG custom validators | N-A | no YANG leaf changes |
| CLI commands/flags | Yes | 21 `internal/le/*/register.go` files, and `internal/le/leroot/dispatch.go` |
| CLI grammar (keyword before value) | Yes | the namespace word is a keyword, and the new feeder in `internal/le/cligrammar/cligrammar.go` enforces it |
| Editor autocomplete | N-A | le has no editor surface |
| Functional test for new RPC/API | Yes | `test/ui/le-namespace-dispatch.ci` |
| Pipe completeness | Yes | `leroot.Run` already routes every answer through `ProcessPipesDefaultFormatLocal`, and `TestDispatchKeepsThePipeWordOutOfTheLookup` proves the chain survives the longer lookup |
| Env var registration | N-A | no new env var |
| Doctor check for runtime dependencies | N-A | no new file path, socket, service, port or binary |
| Prometheus counters/metrics | N-A | no daemon state |
| BGP family surface | N-A | no BGP change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | `le` is a development tool and no shipped feature changes |
| 2 | Config syntax changed? | No | no config surface touched |
| 3 | CLI command added/changed? | Yes | `ai/rules/points/` for the command names agents type, then the rendered rules. `docs/guide/command-reference.md` documents ze, not le, so it is unaffected |
| 4 | API/RPC added/changed? | No | no wire method changes. The three `command-list` wire methods keep their spelling, see R-1 |
| 5 | Plugin added/changed? | No | no plugin registration changes |
| 6 | Has a user guide page? | Yes | `docs/contributing/testing.md`, `docs/contributing/documentation-testing.md` and the 26 other `docs/` files |
| 7 | Wire format changed? | No | no wire bytes touched |
| 8 | Plugin SDK/protocol changed? | No | `pkg/plugin` and `pkg/ze` untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no protocol behavior |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, for the renamed suite entry points |
| 11 | Affects daemon comparison? | No | no shipped capability changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/cli/root-namespace-grammar.md` gains the le feeder and loses the statement that le is uncovered |
| 13 | Route metadata keys added/changed? | No | no route metadata |
| 14 | Prometheus counters added/changed? | No | no counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md` names the commands an agent must find |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED, not answered from memory: run `./le spec citation anchors spec plan/spec-le-command-namespaces.md` and name every document it lists. The 22 rows of `ai/PACKAGE-MAP.md`, 105 of `ai/DOCS-TO-CODE.md` and 23 of `ai/CODE-TO-DOCS.md` are regenerated, not hand-edited |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every `./le <renamed>` example in `docs/` and `ai/` is an example a reader copies, so each is verified against the built binary |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- teach dispatch to resolve more than one word, while every command still has one
   - Tests: `TestDispatchResolvesATwoWordCommand`, `TestDispatchBoundsTheLookupAtThreeWords`, `TestDispatchKeepsThePipeWordOutOfTheLookup`, `TestBareNamespaceTokenListsItsMembers`, `TestNamespacedCommandRegistersAtItsFullPath`, `TestRegisterRefusesAMalformedName`, `TestVerifyDispatchResolvesANamespacedStage`
   - Files: `internal/le/leroot/dispatch.go`, `internal/le/leroot/leroot.go`, `internal/le/verify/dispatch/dispatch.go`
   - Verify: the tests fail against a registry holding only one-word names, then pass with a test-only two-word registration. Every existing command still resolves, so this phase is one commit that changes no command name
2. **Phase: the directory rule** -- rewrite the structural test before any package moves
   - Tests: `TestEveryCommandIsFoundAtThePathItsNamePredicts`, `TestNoRegisteredLeCommandExceedsTwoWords`, `TestEveryLeCommandDeclaresItsOwnAnswerShape`, `TestNoMemberShadowsItsNamespaceRootVerb`
   - Files: `internal/le/group_test.go`
   - Verify: green against today's flat tree, because a one-word name predicts a one-level path under the new rule as well
3. **Phase: one family** -- verify first, because it is the largest and every other family repeats its shape
   - Tests: AC-1, plus `TestStageLogNamesAreUnchangedByNamespacing` and `TestChangedModeExcludesTheFullOnlyStages`
   - Files: five `register.go` files, five package moves, `verifyengine` to `verify/engine`, `verifydispatch` to `verify/dispatch`, `internal/le/register.go`, `stages.go`, the record rows, and every citation of the five names
   - Verify: `./le verify worktree` over the resulting commit, then read the log
4. **Phase: the other eight families** -- spec, yang, config, plugin, command, doc, site, repository
   - Tests: AC-2 through AC-9
   - Files: as phase 3, one family at a time
   - Verify: each family is its own commit, closed over its packages, and `./le verify worktree` runs over the commits on a cadence rather than before each one
5. **Phase: the gate** -- the le root-namespace feeder, last, because it is red until the families have landed
   - Tests: `TestLeRootNamespaceFeederFlagsAHyphenatedObjectRoot`, `TestLeRootNamespaceExceptionNeedsAReason`
   - Files: `internal/le/cligrammar/cligrammar.go`, `docs/architecture/cli/root-namespace-grammar.md`
   - Verify: reintroduce a hyphenated root, watch the gate flag it, restore. This is the wiring proof the precedent doc asks for

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each of the 21 names resolves at its new path, and none resolves at its old one |
| Feature completeness | `le verify`, `le site` and `le repository` still answer their own verbs, and the six namespaces with no root of their own list their members |
| Correctness | The lookup bound is three words, and a value beyond it is never read as a command word |
| Correctness | The three `command-list` wire methods and `doctor-config-claims-unavailable` are unchanged. Grep them by their full spelling in the final diff |
| Naming | `leaf-mentions` and `tracked-build` keep their hyphen. `test-*` keeps its hyphen and is the gate's only exception |
| Data flow | The stage population names commands, and `verifydispatch` resolves them. Neither hardcodes a namespace word |
| Rule: `ai/rules/no-layering.md` | The hyphenated name is deleted, never kept beside the new one as an alias |
| Rule: `ai/rules/git-safety.md` | Each commit's file set is closed over its Go packages, and the commit is compiled through `./le verify worktree` |
| Rule: `ai/rules/stale-comments.md` | `Usage`'s "Eighty-six commands" and `completeness_test.go`'s "areas" wording |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| 21 renamed commands | `./le help` lists each with a space and none with the old hyphen |
| 23 moved packages | `find internal/le -maxdepth 2 -name register.go` matches what `TestEveryCommandIsFoundAtThePathItsNamePredicts` predicts |
| `internal/le/gaterun` unmoved | `ls internal/le/gaterun` |
| the le feeder | `./le cli-grammar` prints a nonzero le roots-checked count |
| no old spelling left | `grep -rIn` for each of the 21 names over `ai docs plan/journal plan/learned .github internal cmd test` returns only the 25 non-le identities of R-1 |
| the workflow line | `grep -n "le verify deps" .github/workflows/govulncheck.yml` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The lookup joins argv words into a registry key. A word holding a space or a slash must not be able to reach a path it was not typed for, so the bound and the exact-match registry are the control |
| Authorization that could fail open | `verifydispatch` refuses an unowned or unregistered command with code 2. The namespaced resolution must keep that refusal rather than falling back to the namespace root's handler |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, back to RESEARCH |
| Lint failure | Fix inline. If architectural, back to DESIGN |
| Functional test fails | Check the AC: wrong AC to DESIGN, correct AC to IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| `repository-tracked-build` red after a commit | The file set was not closed over its packages. Land the missing files in the next commit immediately, per R-4 |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The registry is already the closed-keyword sub-dispatcher the precedent asks for. `LookupLocalData` matches the longest registered prefix and hands back the rest, so a namespace needs no new type, no parent handler and no member table. `Dispatch` throws that answer away today by offering it one word.
- The two detectors a le feeder needs are exact over today's tree. A left segment that is itself a registered root catches repository, site and verify. A left segment shared by two or more roots catches test, verify, yang, spec, plugin, doc, config and command. Their union is the nine families plus `test`, so a one-entry exception list is enough and nothing else is flagged.
- `stageLogPath` flattens slashes to hyphens, which means the whole verification artifact surface survives the rename untouched. That was luck rather than design, and the test that pins it turns it into design.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The `test-*` family is not split | absorb all 13 commands of the Suites group and the two `test-*` gates under `le test` | The six names are five different KINDS of thing sharing a word, which is the doc's own shared-prefix trap. A `le test` namespace would also promise to hold the real suites, and it would not: functional, integration, deployment, qemu, fuzz, mutation and stress-repro are not under it. The alternative also adds a word to the most-typed commands in the repository. `le functional` is cited 130 times across `ai/` and `docs/` and `le qemu` 134 times |
| A namespace member is registered at its own full path, and keeps its own group, its own Meta and its own directory | merge the family into one root that sub-dispatches its members internally | Forced by an existing test. `TestGateStagesAreNotWorkflowOrReport` requires every verify stage's command to be a gate, a generator or a suite. `verify` is a workflow command and `verify-lint` is a gate, so a merged root could only declare one group and the test would fail with no way to state the truth. Four of the nine families mix groups the same way: verify, spec, yang, plugin and command |
| `areaFloor = 80` and `actionWordFloor = 200` are unchanged | lower the floor to 65 | A consequence of the decision above. `leroot.Commands()` returns one entry per registered command, so the population stays 86 and the floor still guards it. Under the rejected merge the count would fall to 65 single-word roots and the floor would have to fall below it, which would weaken the guard for every future command. What does change is the WORDING of the failure message and the Area column of 30 record rows, neither of which is coverage |
| A bare namespace token is answered by `Dispatch`, not by a registered enumerator | register `le spec`, `le yang`, `le config`, `le plugin`, `le command` and `le doc` as commands whose answer lists their members | An enumerator would need a group, and a group is a claim about what the command is for. `le spec` holds a gate, a workflow command and a report, so no group is true. It would also need a Subs line, and `TestReportAreasWriteNothing` would then judge a member's write marker as the enumerator's own. Listing from the registry in `Dispatch` needs no registration, no group and no Subs |
| The lookup is bounded at three words including `le` | scan every argv word | Two words is the deepest command this design creates, so a third candidate word can only come from a value. `le job run label x command <argv...>` offers nine words to an unbounded matcher. The style guide asks for a limit on everything, and this is where it goes |
| `internal/le/verify/engine` becomes `internal/le/verify/engine` | leave it, or rename it without moving it | The word `engine` was invented because `verify` was taken as a directory name. The namespace frees it. Its ten importer packages do not argue against the move: the package IS verify's stage population, whoever reads it |
| `internal/le/verify/dispatch` becomes `internal/le/verify/dispatch` | leave it beside `gaterun` as a shared library | Same invented word, same reason. Its two importers are `verify` and `commit`, and `commit` imports it to run the verify gate, so both are the verify workflow. Read the other way, an importer outside the family means it is shared and should stay put. The first reading governs, because the criterion is what the package IS |
| `internal/le/gaterun` does NOT move | move it under `verify` with the engine | It names a general job runner rather than anything of verify's. Twelve of its fifteen importer packages are outside the verify family: job, qemu, functional, deployment, integration, terminaldemo, buildartifacts, changed, perfbench, platformvet, testchaos and testunit |
| `docvalid` and `docs-to-code` are left alone | rename them in this spec | Both belong to the `doc` object and neither name survives the move. That is a naming decision for the owner, and the gate does not force it: neither detector flags either name |

## Known Limitations

- The le feeder detects a hyphenated root whose left segment is a registered root, or is shared by two or more roots. A single root whose left segment names an object with no other member is invisible to both detectors. `docs-to-code` is the live example, which is why the open question above needs a person rather than a check.
- `internal/le/workflowcheck` keeps its place. It registers no command, so the directory rule does not reach it, and no namespace claims it.
- The local-handler surface is still uncovered by ze's own root feeder, exactly as `docs/architecture/cli/root-namespace-grammar.md` records under "Coverage boundary". This spec adds a le feeder and does not close that hole.

## RFC Documentation (Scope: protocol)

Not applicable. No protocol code changes.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-17 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Every commit's file set is closed over the Go packages it touches, and each commit compiles on its own
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
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
