# Spec: hook-git-verb-prefix-match

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Agent tooling: a false refusal costs every session a retry and pushes agents toward wrappers, but no shipped behaviour changes |
| RFC gaps | - |
| Depends | - |
| Phase | 3/3 |
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
records the first, and a 2026-11-12 row records a third.

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
the fix here closes all three rows.

**Correction (2026-11-12, implementation).** This section first said that
spec's owned-rows entry would be dropped in the same commit. It is not: the
spec edit gate refuses any edit of that spec until it carries its own
`## Existing Mechanisms` section, which is that spec's design work (R-4). The
entry stays there, naming a row that is now `fixed:`, until that spec is next
edited.

**WIP cap (2026-11-12).** Thomas ordered this spec implemented at once, with
19 specs in progress against a cap of 12. The claim was taken one above the
count for this spec alone; it is a permission for this work, not a change of
the cap.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the doors this work crosses
  → Constraint: a hook checks what a command is, never how a message is worded (`ai/META-DECISIONS.md`); a refusal is never passed by a wrapper
  → Constraint: a hook that refuses wrongly blocks every concurrent session at once (`le-tooling.md`, "Who depends on it from outside"), so the change is judged by `./le ai hooks unit` and the package tests before it lands
  → Constraint: the `le-tooling.md` Traps list states this defect in the present tense ("A hook pattern matched a prefix"); the line is reworded in the same change, or the door is wrong
- [ ] `plan/learned/HOOK-FRICTION.md` - known false positives of each hook, before one is changed
  → Decision: it carries no entry for `gitVerbRun`, `gitInvocation` or `git merge`; the only records of this defect are the three journal rows, so nothing recorded there constrains the fix
- [ ] `docs/contributing/running-commands.md`, "The Bash guard matches your command text" - the page that documents `gitVerbRun`
  → Constraint: the page says the guard reads a command POSITION and names the quote exemption and the option strip; it is silent on where the verb ends. The fix adds that sentence to this section in the same change
- [ ] `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` - the spec that lists the 2026-09-02 row
  → Decision: its AC-8 ("`git merge-base` and other verbs that share a prefix with a banned verb: the hook allows them; the check matches whole argv words") plans to replace the text match with parsed argv words. That conversion stays there. This spec fixes the verdict inside the existing text match, so AC-8's observable half is already true when that spec runs, and its argv conversion must keep it true
- [ ] `ai/rules/principles.md`, "A zero that another branch relies on is a guard" - why an accidental refusal is named before it is removed
  → Constraint: the prefix match refuses commands nobody listed. Each one this fix stops refusing is named in the Acceptance Criteria as a permitted survivor or kept refused on purpose; none disappears unnamed

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/hookruntime/bash.go` - `gitVerbRun` (leading boundary only), `bashBranchMove` and `bashDestructiveGit` (its two callers), `gitInvocation`, `insideQuotes`, `shellInvoker`
  → Constraint: `gitVerbRun` has exactly two callers, and both pass a literal pattern from a list in the caller. No other check uses it
  → Constraint: the patterns are of two shapes. Thirteen end in the verb (`git commit`, `git push`, `git reset`, `git restore`, `git revert`, `git stash`, `git clean`, `git add`, `git rm`, `git mv`, `git merge`, `git rebase`, `git switch`). Thirteen continue past the verb with an argument (`git checkout --`, `git checkout -f`, `git checkout HEAD`, `git push --force`, `git push -f`, `git checkout -b`, `git checkout -B`, and the six `git branch` flags)
  → Constraint: in an argument pattern the verb is already a whole word, because the pattern's own space ends it. Its tail is a prefix on purpose: `git checkout --` refuses every long option (`--ours`, `--theirs`, `--force`, `--orphan`), `git checkout HEAD` refuses `HEAD~1` and `HEAD^`, `git branch -d` refuses the combined flag `-dr`. A trailing test on these would clear refusals of destructive forms
- [ ] `internal/le/hookruntime/runtime.go` - the `pretool-bash` check list
  → Constraint: `bashDestructiveGit` and `bashBranchMove` run on every Bash call of every session, main thread and subagent alike
- [ ] `internal/le/hookruntime/readers.go` - `gitReadVerbs`
  → Decision: `merge-base` is already declared a reader verb there, so once the branch guard stops refusing it a subagent runs it without its identity line, like `git log`. No reader change is owed
- [ ] `internal/le/hookruntime/bash_git_test.go` - the guard's tests
  → Constraint: each test drives the real `pretool-bash` action through `runHook` and asserts the exit code and a phrase of the refusal. New tests take the same route
- [ ] `internal/le/hookruntime/verdicts_test.go`, `testdata/guard-verdicts-head.jsonl` - the recorded verdict table
  → Constraint: `TestGuardVerdictsOfHeadStand` fails when a check that refused a recorded line stops refusing it. No recorded line flips under this design: the only recorded `git merge` line is `git merge main`, and no recorded line refused by either git guard has a name byte directly after a verb-ending pattern (searched 2026-11-12)
  → Constraint: a line added to the hook golden catalog joins the corpus, so its row is appended to the table by an update run of that test. An update only appends
- [ ] `internal/le/ai/hooks/parity_catalog.go`, `parity.go` - the hook golden rows `./le ai hooks unit` runs
  → Constraint: the `BASH_GOLDEN` table declares its population (`expected`), and a fixture count that differs is a failure. A new row moves the count with it

**Behavior to preserve:**
- A verb is judged by command position: at the start, or after a separator, a pipe, an opening subshell or a newline. A verb inside quotes is prose, except where another shell is handed the string to run
- Git's options before the verb are stripped first, so `git -C <dir> merge` is refused as `git merge` is
- Every argument pattern keeps its prefix match
- Every line recorded as refused in `guard-verdicts-head.jsonl` stays refused by the same check
- The refusal texts of both guards

**Behavior to change:**
- A pattern that ends in the verb matches only when the byte after it cannot continue a git command name

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Git verb position test | `internal/le/hookruntime/bash.go` `gitVerbRun`, `insideQuotes` | Judges a git verb by the byte before it, with a quote exemption | Extends: the same function gains the matching test on the byte after a verb-ending pattern. No second matcher is written |
| Git global option strip | `internal/le/hookruntime/bash.go` `gitInvocation`, `gitGlobalOptions` | Puts the verb back beside `git` past `-C`, `-c` and the other leading options | Does not apply: it runs before the match and leaves the verb's own text intact, so `git -c x merge-base` reaches the matcher as `git merge-base` and takes the new test unchanged |
| Parsed shell command | `internal/le/hookruntime/shellparse.go` `commandParse`, `parsedcommands.go` `everyCommand` | Parses the command line into commands and words for the build, pipe, deletion and reader checks | Does not apply here: moving the git guards onto parsed words is AC-8 of `spec-gates-share-one-semantic-population.md`, which redefines what quoting, `bash -c` and an unparsable line mean for these guards. That is a replacement of the matcher, owned there |
| Git reader verbs | `internal/le/hookruntime/readers.go` `gitReadVerbs` | Lists the git commands a reader line may run, `merge-base` among them | Does not apply: it already admits the command this spec frees |
| Recorded verdict table | `internal/le/hookruntime/verdicts_test.go` `TestGuardVerdictsOfHeadStand` | Refuses a change that clears a refusal recorded at HEAD | Extends: the freed line is appended as a row with no refusing check, so a later change that refuses it again is visible as a diff |
| Hook golden rows | `internal/le/ai/hooks/parity_catalog.go` `BASH_GOLDEN` | Declares command lines and the exit code the hook owes each, run by `./le ai hooks unit` | Extends: one row for the freed command, and the declared population moves with it |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A Bash tool call. The harness runs the PreToolUse hook `le ai hooks pretool-bash` with the call as JSON on standard input

### Transformation Path
1. The `pretool-bash` action reads the `command` string from the payload
2. Each check in the action's list runs in order; `bashDestructiveGit` and `bashBranchMove` are two of them (`runtime.go`)
3. Each of the two strips git's leading options from the command (`gitInvocation`)
4. Each asks `gitVerbRun` once per pattern whether the command runs it. This is the step that changes: a pattern ending in the verb now also needs the verb to end there
5. The first pattern that matches produces the verdict: exit code 2 and the refusal text. No match lets the remaining checks run

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ hook | JSON payload on standard input; exit code and refusal text back. Neither changes | No |

### Changed-Wire Census

No wire changes. The hook payload, the exit codes and both refusal texts are
untouched: the change is confined to which command strings one predicate
answers true for.

**Search evidence:** `gitVerbRun` searched across `internal/le/hookruntime/`
and the tree's Markdown on 2026-11-12. Callers: `bashDestructiveGit` and
`bashBranchMove` only. Control: the search finds the known caller
`bashBranchMove` and the known page `docs/contributing/running-commands.md`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Hook verdict (exit code and text) | writer | `internal/le/hookruntime/bash.go` `bashBranchMove`, `bashDestructiveGit` | both build the verdict from a literal; this spec edits neither literal | Preserved: `TestEveryBranchMovingVerbIsRefused` and `TestEveryForbiddenGitVerbIsRefused` assert the texts |
| Hook verdict (exit code and text) | reader | the harness, outside the repository | reads exit code 2 as a refusal and shows the text | Preserved: same code, same text |

### Integration Points
- `gitVerbRun` - the only function whose answer changes
- `TestGuardVerdictsOfHeadStand` - gains appended rows through its update run
- `BASH_GOLDEN` - gains one row

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
| A-1 | A git command name is made of ASCII letters, digits and hyphens, so those three classes are exactly the bytes that continue one | The command list of git 2.43 on this host (`git --exec-path`): every name sharing a prefix with a banned verb continues with a hyphen or a letter (`merge-base`, `mergetool`, `commit-tree`) | A banned verb followed by some other byte would be read as a longer command and pass | The boundary test rows of the TDD plan: the verb followed by each shell byte that can end a word is still refused | confirmed 2026-11-12: `TestTheVerbEndsAtEveryByteThatCannotContinueAName` green over eleven forms |
| A-2 | The merge strategy backends and index writers that share the `git merge` prefix were never refused on purpose | Neither guard's list names them, and no test or recorded verdict row holds one; the guards also do not refuse `git cherry-pick`, `git read-tree` or `git update-index`, which write the same index | If Thomas wants them refused, AC-4 is where they stay refused and the rest of the design stands | Thomas's answer at the design gate | confirmed 2026-11-12: Thomas ruled that a read-only command is allowed. The eight that perform a merge are not read-only, so they stay refused by name (AC-4) |
| A-3 | No recorded verdict flips | Search of `guard-verdicts-head.jsonl` on 2026-11-12 for a refused line with a name byte after a verb-ending pattern: none | `TestGuardVerdictsOfHeadStand` goes red and names the line | That test, run after the change | confirmed 2026-11-12: `TestGuardVerdictsOfHeadStand` green after the change; its update run added two rows and changed none |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The trailing test is applied to an argument pattern, and `git checkout --ours`, `git checkout HEAD~1` or `git branch -dr` stops being refused | `TestAnArgumentPatternKeepsItsPrefix` red | The test exists before the change and names each form |
| R-2 | A git alias whose name extends a banned verb (`merge-ff`) runs that verb | None from the hook: an alias under any name already bypasses a guard that reads the command text | Unchanged exposure, not introduced here. Recorded so the argv conversion in `spec-gates-share-one-semantic-population.md` does not assume the text match covered it |
| R-3 | A banned verb directly followed by a quote or an expansion (`git merge"" main`) is read as a longer command | The boundary rows | The test is negative: only a letter, a digit or a hyphen continues a name, so every other byte ends the verb and the command is refused |
| R-4 | Dropping the row from the other spec's owned-rows table needs that spec's doors read first, and it names many subsystems | The spec edit hook refuses the edit | The implementing session reads the doors the refusal names before that edit; the edit is one table row. Happened 2026-11-12, and reading was not enough: the gate also refused because that spec (Status design) has no `## Existing Mechanisms` section. Writing that section is that spec's design, so the edit was not made and the entry stays |
| R-5 | Another session edits `bash.go` at the same time | `git status` shows the file dirty before the first edit | Commit this spec's hunks alone (`hunks` in `./le commit create`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Too loose: a session can run a banned git verb in the shared checkout. Too strict: every session's Bash calls with a git command are refused. Nothing in the shipped binary |
| How is it reverted? | One commit reverted by the owner; `le` rebuilds from HEAD on the next call |
| Who else touches this path? | `spec-gates-share-one-semantic-population.md` (AC-8) will replace this matcher with parsed argv words |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A Bash call `git merge-base --is-ancestor <sha> main` through the `pretool-bash` action | → | `bashBranchMove`, `gitVerbRun` | `TestALongerGitCommandIsNotTheBannedVerb` (`bash_git_test.go`) |
| The same line through `./le ai hooks unit` | → | the `pretool-bash` action | `BASH_GOLDEN` row `git merge-base --is-ancestor abc main`, exit 0 |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `git merge-base --is-ancestor <sha> main`, bare, behind `-C <dir>`, and after `&&` | The hook exits 0 and the command runs | `git merge-base` in every position: it reads history and writes nothing |
| AC-2 | `git merge`, `git merge --ff-only`, `git merge other`, and `git merge` directly followed by `;`, `&`, a pipe, `)`, a newline, a redirect or a quote | The hook exits 2 with the branch-move refusal | None: every byte that is not a letter, a digit or a hyphen ends the verb, so no spelling of the banned verb passes |
| AC-3 | `git checkout --ours f`, `git checkout HEAD~1 f`, `git checkout -fq`, `git branch -dr origin/x` | The hook exits 2, as today | None: an argument pattern keeps its prefix match, because its tail covers a family of destructive forms |
| AC-4 | `git merge-recursive`, `git merge-octopus`, `git merge-resolve`, `git merge-ours`, `git merge-subtree`, `git merge-index`, `git merge-one-file`, `git mergetool` | The hook exits 2 with the branch-move refusal, each named as its own pattern | `git merge-tree`, `git merge-file`, `git commit-tree` and `git commit-graph` pass: none writes the shared index or moves a branch. The eight named here perform a merge into the index and working tree, which is what the ban on `git merge` exists to stop |
| AC-5 | Every line recorded in `guard-verdicts-head.jsonl` | Each is refused by every check that refused it when recorded | The appended rows for the freed lines, which record no refusing check |
| AC-6 | The page section "The Bash guard matches your command text" and the Traps line of `ai/doors/le-tooling.md` | The page states where a verb ends and why an argument pattern stays a prefix; the door no longer states the defect as current | None: both describe the changed function |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestALongerGitCommandIsNotTheBannedVerb` | `internal/le/hookruntime/bash_git_test.go` | AC-1, and the passing half of AC-4: each line exits 0 through the real action | |
| `TestTheVerbEndsAtEveryByteThatCannotContinueAName` | `internal/le/hookruntime/bash_git_test.go` | AC-2: the banned verb followed by each word-ending byte exits 2 | |
| `TestAnArgumentPatternKeepsItsPrefix` | `internal/le/hookruntime/bash_git_test.go` | AC-3: each destructive long or combined form exits 2 | |
| `TestAMergeBackendIsRefusedByName` | `internal/le/hookruntime/bash_git_test.go` | AC-4: each of the eight exits 2 with the branch-move text | |
| `TestGuardVerdictsOfHeadStand` (existing) | `internal/le/hookruntime/verdicts_test.go` | AC-5 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| none | N-A: the change takes no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `BASH_GOLDEN` row `git merge-base --is-ancestor abc main` | `internal/le/ai/hooks/parity_catalog.go` | An agent asks whether one commit is an ancestor of another and gets an answer instead of a refusal, under `./le ai hooks unit` | |

## Files to Modify
- `internal/le/hookruntime/bash.go` - `gitVerbRun` gains the trailing test for a verb-ending pattern, and its comment says so; `bashBranchMove` gains the eight named patterns of AC-4
- `internal/le/hookruntime/bash_git_test.go` - the four new tests
- `internal/le/hookruntime/testdata/guard-verdicts-head.jsonl` - rows appended by the update run, never edited by hand
- `internal/le/ai/hooks/parity_catalog.go` - one `BASH_GOLDEN` row and its declared population
- `docs/contributing/running-commands.md` - "The Bash guard matches your command text"
- `ai/doors/le-tooling.md` - the Traps line
- `plan/journal/hook-existing-patterns-false-positive.md` - the three rows marked fixed
- `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` - not modified: the gate refuses the edit (R-4)
- `internal/le/ai/hooks/parity.go`, `parity_test.go`, `fixtures.go` - found in implementation: the golden population count, its exit-code populations and two stored digests (`parityCatalogDigest`, `hookSourcesDigest`) mirror the catalog and the hook sources, and move with them

## Files to Create
- none

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | A development hook; no config or RPC |
| YANG validation constraints | N-A | No leaf |
| YANG custom validators | N-A | No leaf |
| CLI commands/flags | N-A | No command or flag is added |
| CLI grammar (keyword before value) | N-A | No command |
| Editor autocomplete | N-A | No leaf |
| Functional test for new RPC/API | N-A | No RPC; the hook golden row is the entry-point test |
| Pipe completeness | N-A | No output |
| Env var registration | N-A | No variable |
| Doctor check for runtime dependencies | N-A | No runtime dependency, and `le` is not the product |
| Doctor code per problem class | N-A | The refusal is a development hook's, read by an agent; both texts are unchanged |
| Prometheus counters/metrics | N-A | No observable state |
| BGP family surface | N-A | Not protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Development tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | No | |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | No | The hook is not test infrastructure |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | `bash.go` declares `docs/architecture/core-design.md`; that page describes how le runs native gates, not what the git guard matches, so it is unaffected |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/contributing/running-commands.md` (edited), `ai/doors/le-tooling.md` (edited). `.claude/hooks/README.md` anchors `bash.go` and describes `bashBranchMove` as refusing every verb that integrates a branch, which stays true. The implementing phase runs `./le spec citation anchors spec` on this spec and names any further page |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/running-commands.md` shows `git -C /other/tree commit`; it stays correct |

## Implementation Steps

1. **Phase: Red tests through the real action** -- write the four tests and the golden row before any change to the matcher
   - Tests: `TestALongerGitCommandIsNotTheBannedVerb`, `TestTheVerbEndsAtEveryByteThatCannotContinueAName`, `TestAnArgumentPatternKeepsItsPrefix`, `TestAMergeBackendIsRefusedByName`, the `BASH_GOLDEN` row
   - Files: `bash_git_test.go`, `parity_catalog.go`
   - Verify: the first test and the golden row are red because `git merge-base` exits 2. The other three are green before the change, and each is shown red once by breaking the matcher the way its risk row describes (R-1, R-3) or by dropping one named pattern, then restored
2. **Phase: Trailing word boundary** -- the matcher, the eight named patterns, the comment
   - Tests: all five of the plan
   - Files: `bash.go`
   - Verify: `./le test unit pkg path ./internal/le/hookruntime/` and `./le ai hooks unit` green; the verdict table's update run appends the new rows and changes no recorded one
3. **Phase: Pages and records** -- the page section, the door line, the three journal rows, the other spec's table
   - Files: `running-commands.md`, `le-tooling.md`, the journal shard, the other spec
   - Verify: `./le doc check all`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC has a test that fails when its behaviour is broken |
| Correctness | The trailing test is reached only for a pattern that ends in the verb; no argument pattern takes it |
| Correctness | The eight named patterns are matched as whole words too, so `git merge-indexer` style names do not ride on them |
| Data flow | Both guards still strip git's leading options before matching |
| Rule: stale comments | The comment above `gitVerbRun` describes both boundaries |
| Rule: verdict table | No recorded row was edited; the diff of the table is additions only |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `git merge-base` passes the hook | `./le ai hooks unit`, the new golden row |
| The banned verb and the argument forms stay refused | `./le test unit pkg path ./internal/le/hookruntime/ run 'TestTheVerbEnds|TestAnArgumentPattern|TestAMergeBackend|TestEveryBranchMoving|TestEveryForbiddenGit'` |
| No recorded refusal cleared | `./le test unit pkg path ./internal/le/hookruntime/ run TestGuardVerdictsOfHeadStand` |
| Three journal rows closed | `./le spec journal report` no longer counts them as open |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Guard bypass | A spelling of a banned verb that the trailing test reads as a longer command: quotes, expansions, redirects and separators glued to the verb |
| Fail-open | A pattern whose shape is misclassified as verb-ending or argument, in either direction |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A recorded verdict flips | A-3 is broken: stop, name the line, back to DESIGN |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
- The defect is narrower than "every pattern is a prefix match". Only a pattern that ends in the verb is unintentionally a prefix; a pattern that continues past the verb already ends the verb with its own space, and its prefix tail is doing deliberate work
- The prefix match was also an unnamed guard: it refused eight git commands that merge into the shared index and that no list names. A plain word boundary would have dropped that refusal with no line deleted and no test red

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `hook-existing-patterns-false-positive` | 2026-11-11 | `gitVerbRun` prefix match: `git merge-base --is-ancestor` refused as `git merge` (second occurrence) |
| `hook-existing-patterns-false-positive` | 2026-11-12 | `gitVerbRun` prefix match: `git merge-base --is-ancestor` refused as `git merge` (third occurrence) |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Add the trailing test to the existing text matcher | Move both git guards onto the parsed command words of `commandParse` | The parsed route is the right end state and is already AC-8 of `spec-gates-share-one-semantic-population.md`. It also changes what quoting, `bash -c`, `eval` and an unparsable line mean for two guards that protect the shared index, which is a replacement with its own verdict table work. Three sessions have been refused; the narrow fix lands now and that spec's conversion inherits a correct verdict |
| The trailing test applies only to a pattern that ends in the verb | Apply it to every pattern | Applied to all, `git checkout --ours`, `git checkout HEAD~1` and `git branch -dr` would pass. The Goal says every other pattern keeps its verdict |
| A name continues with a letter, a digit or a hyphen; every other byte ends the verb | The verb ends only at whitespace, a separator or the end of the line | The negative form refuses more: a quote, an expansion or a redirect glued to the verb still ends it (R-3) |
| Keep the eight merge backends refused, each by name | Let the word boundary free them | Decided by Thomas's rule that a read-only command is allowed (A-2): the line between passing and refused is whether the command only reads. They merge into the index and working tree, which is what the `git merge` ban protects. The cost is eight literal patterns in a list. Freed instead, they would join `git cherry-pick` and `git read-tree`, which the guards have never refused |

## Known Limitations
- The git guards still read the command text, so a git alias and a script file reach a banned verb as before. The parsed-words conversion is `plan/post-release/engineering/spec-gates-share-one-semantic-population.md` (AC-8)

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
- [ ] AC-1..AC-6 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Targeted runs: `./le ai hooks unit`, `./le test unit pkg path ./internal/le/hookruntime/`

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + the journal rows
- [ ] **Commit B:** the spec removed
