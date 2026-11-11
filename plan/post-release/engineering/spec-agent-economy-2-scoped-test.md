# Spec: agent-economy-2-scoped-test

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P1 |
| Release reason | Release-independent development tooling: it adds a short `le` action for scoped tests and changes no shipped binary. |
| RFC gaps | - |
| Depends | - |
| Phase | 7 of 7 done; closure next |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 2 of `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`,
which holds the baseline.

Testing one package is the most frequent heavy action an agent takes, and it
has no short form. The agent assembles the build-tag list by hand and wraps
the command in job admission. Over three weeks that produced 1,857 raw
`go vet`, `go test` and `go build` attempts of which 707 failed, about 350
`./le help test` lookups, and false reds from missing tags. Queue wait is
measured and never saved, so run time cannot be told from waiting. A mistyped
`le` command answers with the whole usage, and `./le commit create` refuses a
body for one over-long word without naming it.

The goal: one short command tests, vets or compiles named packages under the
right tags, admitted, answering in one line on green; every admitted run
records its wait; a mistyped command answers with the nearest real one; a
commit body with one over-long word is accepted.

## Research Findings (2026-11-11, each producer re-read at design)

| Finding | Where | Design-time verdict |
|---------|-------|---------------------|
| `./le job run` passes its argv unchanged: no tags, no race flag, no package pattern, no timeout. The agent supplies the tags from `feature-gates.txt` | `internal/le/job/answer.go`, `answerRun`; `internal/le/job/job.go`, `Admission.Run`; `docs/contributing/running-commands.md`, "Testing one package while you develop it" | Confirmed. The page adds that the job boundary picks `CGO_ENABLED` from the child's race flags; that code (`process.go`) was not read here |
| The hook's suggested replacement for a raw `go test` adds no tags and no `quiet` | `internal/le/hookruntime/bash.go`, `admittedCommand`, called from `rawHeavy` | Confirmed. `rawHeavy` matches `go test` and `golangci-lint` only, so a raw `go vet` is refused by nothing |
| `./le test unit` has the groups bgp, core, plugins, config, cli, installer and all. None covers `internal/le`, and there is no per-package action | `internal/le/test/unit/groups.go`, `groups`; `actions.go`, `table` | Confirmed, with two facts the skeleton lacked. The groups run through `gaterun.Run`, which announces and streams the child with no job admission. The area dispatches every line through `Area.Sweep`, which refuses an action that declares parameters |
| Tags, timeout, race and the environment are already computed | `internal/le/go/toolchain/gotoolchain.go`, `Toolchain.GoTest`, `GoVet`, `GoList`, `Environment`, `TestOptions`, `EnvOptions` | Confirmed. `GoVet` carries the tags and neither a race flag nor a timeout |
| Admission with attach-to-identical-job exists | `internal/le/job/job.go`, `defaultSlots`, `Admission.queue` | Confirmed. The slot count is derived (cores divided by cores per job), four on the 32-core box by the function's own comment |
| Quiet output exists: one line on green, up to 20 failure lines on red | `internal/le/job/quiet.go`, `runQuiet`, `quietSummary`, `quietKeyLines`; `report.go`, `Report.Text` | Corrected. The green line holds the exit code and the log path only. The failure lines are the ones `runlog.Key` matches (`--- FAIL:`, `FAIL`, `panic:`, `fatal error:`), so a red names the failed test and omits the assertion text printed beneath it. `runQuiet` is unexported and passes no directory and no environment |
| Scope keywords `scope <packages>` and `changed` exist on lint | `internal/le/go/lint/actions.go`, `actions`, `changedScope` | Confirmed. `scope` is one string value holding space-separated patterns, so two patterns need shell quoting |
| The reached-package selector exists | `internal/le/repo/changed/scope.go`, `Packages`, `Scope.Resolve`; `selector.go`, `Scope.resolveSelector` | Corrected: `Scope.Resolve` is in `scope.go`. The entry for a consumer outside the package is `Packages(root)`, which answers a `ScopeReport` and an exit code. The answer can be the whole tree with exit 0: `widen` in `scope.go` and `failOpen` in `selector.go` answer the single package `./...` with `Widened` set and a `Reason`. Lint's `changedScope` passes that answer through |
| Wait is measured and dropped: the duration log holds label, seconds and time; seconds start at the claim | `internal/le/job/job.go`, `Ticket.Waited`, `Ticket.Release`; `internal/le/job/registry.go`, `appendDuration` | Corrected in two places. `appendDuration` is in `registry.go`. The wait is not wholly dropped: `Admission.RunContext` copies it into `Report.WaitedSeconds`, which the JSON payload carries; it is absent from the text and from the file. Only a claimed ticket writes a row |
| Run time since 2026-10-21: `unit-pkg` 2,164 runs and 26.7h; `lint` 488 and 12.2h; `test-bgp` 983 and 7.6h | `tmp/.ze-verify-duration.txt` | Not re-derived. Checked: the file holds 28,911 rows, every one of three columns |
| An unknown command prints `unknown command` and the full usage. A retired name already answers in one line | `internal/le/le/root/dispatch.go`, `dispatchLine`, `refuseRetired` | Confirmed, and a second site does the same: `helpAsked` for `le help <unknown>`. An unknown action of an area (`Area.refuseVerb`) and an unknown member of a namespace (`answerNamespace`) already answer in one or two lines |
| The commit body is wrapped at 72; "unwrappable" fires for one word longer than the line and does not name it | `internal/le/commit/input.go`, `wrapBody`, `messageWidth` | Confirmed. The measure is the line's indent plus the word |
| `docs/contributing/testing.md` shows a one-package test with no tags, and a command with a literal `bin/le` | that page: first-time setup, ladder step 1, the unit-test examples, the single-test examples, the cheat sheet | Confirmed. The same tagless form is in `docs/guide/developer-setup.md`, the note `internal/le/setup/setup.go` prints, and the rule point agents read |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the system door
  → Constraint: a feature registers and is discovered; no central list learns its name.
- [ ] `ai/doors/le-tooling.md` - the door for `le`
  → Constraint: a tool's package sits at the path its command name predicts, and `le help`, completion and `le help ai` list every new word.
  → Constraint: an action answers through the shared renderer and stays callable as Go.
  → Constraint: every hook runs on every session's call, so a hook change that refuses wrongly blocks all of them.
- [ ] `docs/contributing/running-commands.md` - launcher, tags, admission
  → Constraint: a session runs one CPU-intensive job at a time, and heavy work is admitted.
  → Decision: the failure lines of a run log are one declaration in `internal/le/runlog`.
- [ ] `docs/architecture/cli/command-namespacing.md` - a space is a directory level
  → Decision: an action of the registered command `test unit` adds no command name, so no directory.
- [ ] `ai/rules/cli.md` - keyword before value, exit codes, a dash-leading token is never a value
  → Constraint: the action's words are declared in its action table, and a usage refusal answers 2.

**Key insights:**
- Every part of the short action exists. The work is one action in the `test unit` table that joins the toolchain's argv builders, job admission's quiet run and the selector.
- The unit area sweeps; an action with parameters must reach `Area.Answer` instead.
- The selector can answer the whole tree with exit 0, so `changed` must read `Widened` and refuse.
- A list of values is a repeated keyword (`Parameter.Repeat`, `Arguments.Values`), and `keywordValue` already refuses a dash-leading value.
- The wait already rides the ticket to `Release`; the record needs one more column and the text one more clause.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/test/unit/groups.go` - the `groups` table, `Group.options`, `Group.Argv`, `Group.EnvOptions`; a group naming `Tags` (the installer) takes the bare core set plus its own tags
- [ ] `internal/le/test/unit/actions.go` - `table` builds one action per group plus `all`; `answer` resolves the root and the toolchain, then calls `Area.Sweep` for every line; `runner` calls the `actionRunner` seam, which is `gaterun.Run` in production
- [ ] `internal/le/test/unit/register.go` - registers `test unit`, its actions and the document shape; the description names the groups and `all`
- [ ] `internal/le/gaterun/gaterun.go` - `Run` announces and streams a child, with no admission
- [ ] `internal/le/go/toolchain/gotoolchain.go` - `GoTest`, `GoVet`, `GoList`, `TestOptions`, `EnvOptions`, `TestTags`
- [ ] `internal/le/job/job.go` - `DurationFile`, `defaultSlots`, `Ticket` with `Waited`, `Ticket.Release`, `Admission.queue`, `Admission.RunContext`
- [ ] `internal/le/job/registry.go` - `appendDuration` writes label, seconds and UTC time, tab-separated; `reportPrevious` reads the first two columns of the last row of a label
- [ ] `internal/le/job/quiet.go` - `runQuiet`, `quietLog`, `quietSummary`
- [ ] `internal/le/job/report.go` - `Report`, whose `Text` renders a quiet run as `job <label>: exit <code>, log <path>` and the key lines
- [ ] `internal/le/job/answer.go` - the `job run` grammar: `label`, `quiet`, `command`
- [ ] `internal/le/runlog/runlog.go` - `Key`, `FailedTests`, `Head` and the `failureLine` pattern
- [ ] `internal/le/go/lint/actions.go` - `scope` and `changed`, `changedScope`, and the in-process admission that releases its own ticket
- [ ] `internal/le/repo/changed/scope.go` - `Packages`, `Scope.Resolve`, `ScopeReport` with `Widened`, `Reason` and `Tags`; `widen` answers `./...` for an unreadable published scope
- [ ] `internal/le/repo/changed/selector.go` - `Scope.resolveSelector`: exit 0 for every answer, the fail-open ones included, 2 for a caller error or a missing manifest; `failOpen` and `everyPackage`; `reachedTags` adds the tags a changed file negates
- [ ] `internal/le/le/action/leaction.go` - `Action`, `Area.Answer`, `Area.Sweep`, `Area.TakesArguments`, `Area.refuseVerb`; `Area.AnswerOrSweep` sends a one-word line and a line opening with an argument-taking action to `Answer`, every other line to `Sweep`
- [ ] `internal/le/le/action/parameter.go` - `Parameter` with `Repeat`, `Arguments.Values`, `parseArguments`, `checkPositional` (one positional, first, never repeated, never a string), `checkKeyword`; `keywordValue` refuses a value that begins with a dash
- [ ] `internal/le/test/unit/testunit_test.go` - `TestAllRunsTheCheckoutThenTheTagGuardedGroups` pins that `all` on its own line answers an outer sweep whose one row holds the inner sweep
- [ ] `internal/le/job/process.go` - `commandEnvironment`: for a direct `go test` child it takes the environment it is given, removes the outer checkout identity, sets the test temp directory, and appends `CGO_ENABLED=1` when the argv carries the race flag; any other child keeps its environment
- [ ] `internal/le/go/toolchain/gotoolchain_cachereuse_test.go` - `TestASecondCheckoutPathReusesTheFirstOnesTestResult` runs the real `go test` over a module written under a temporary directory, with a `Toolchain` built as a value, `GOWORK` off and a private `GOENV`, and skips without `go`
- [ ] `internal/le/le/action/kind.go` - the kinds; `KindList` is comma-separated and refuses an empty item
- [ ] `internal/le/le/root/dispatch.go` - `resolve`, `members`, `dispatchLine`, `helpAsked`, `answerNamespace`, `refuseRetired`, `CommandWordsMax`
- [ ] `internal/le/group_test.go` - `TestEveryCommandIsFoundAtThePathItsNamePredicts` reads registered command names and directories holding a `register.go`
- [ ] `internal/le/commit/input.go` - `Message`, `wrapBody`, `messageWidth`
- [ ] `internal/le/hookruntime/bash.go` - `commandSegments`, `rawHeavy`, `admittedCommand`, `bashRawHeavy`
- [ ] `internal/le/ai/hooks/parity_catalog.go` - the golden rows for raw `go test`, `ZE_ADMIT_RAW` and the admitted form
- [ ] `internal/le/ai/hooks/fixtures.go` - the source seal re-derived when `bash.go` changes
- [ ] `internal/le/ai/hooks/parity.go` - `sharedScratchName` lists the duration file's name
- [ ] `internal/le/verify/current_test.go` - asserts the duration record opens with the label and a tab
- [ ] `internal/le/setup/setup.go` - the closing note prints a tagless package test
- [ ] `docs/contributing/testing.md` - the examples AC-10 names
- [ ] `docs/contributing/running-commands.md` - "A bare `go test` is not `./le test unit`", "How long a verify takes", "Testing one package while you develop it", "`quiet`, and the pattern it replaces"
- [ ] `docs/guide/developer-setup.md` - "After Setup" shows the tagless package test
- [ ] `ai/rationale/git-safety.md` - names the duration file and a writer (`_release` in `answer.go`) that does not exist
- [ ] `ai/rules/points/testing/native-test-actions/run-the-focused-test-through-a-native-action.md` - the rule point that names the long form
- [ ] `ai/doors/le-tooling.md` - the Boundaries bullet and the Gates row that name the long form

**Behavior to preserve:**
- The long `./le job run label <label> [quiet] command <argv>` form keeps working, with its grammar and its exit codes.
- Admission, the slot count, the attach rule and the registry files are unchanged.
- `./le test unit <group>...` and `./le test unit all` keep their sweep, their payload shape, their argv and their environment.
- A bare `./le test unit` lists the actions and runs nothing.
- The first three columns of a duration row keep their meaning and order.
- An unknown command exits 2, writes nothing to standard output and runs nothing. The words `unknown command: <word>` stay in the answer.
- A retired name, an unknown action and an unknown namespace member answer as today.
- A commit subject over 72 characters is refused, and a line that starts with a declared trailer key is refused.
- `ZE_ADMIT_RAW` with a reason passes the raw-command guard.

**Behavior to change:**
- Only what the acceptance criteria name.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Unit groups | `internal/le/test/unit/groups.go`, `Group`; `actions.go`, `table` | Run a fixed package pattern with the toolchain's tags and the race detector, unadmitted, as a sweep | Extends: `pkg` is one more action in the same table, built from the same toolchain. The group table also tells the action which patterns need a tag the feature set lacks |
| Toolchain argv builders | `internal/le/go/toolchain/gotoolchain.go`, `GoTest`, `GoVet`, `Environment` | Build the `go test` and `go vet` argv with tags and timeout, and the child environment | Extends by use: the action calls them with the options a host group uses. No new builder |
| Action table and its parser | `internal/le/le/action`, `Action`, `Parameter`, `Area.Answer` | Declare an action's words once; parse, help, completion and `le help ai` derive from it | Extends by use: the action's four words are keyword parameters, and the packages are the repeated keyword `path`, as `verify status check path` and `commit create file` already are. The parser is unchanged |
| The split between an answered line and a sweep | `internal/le/le/action/leaction.go`, `Area.AnswerOrSweep`, `Area.TakesArguments` | Sends a one-word line or a line that opens with an argument-taking action to `Answer`, the rest to `Sweep`; `chaos/selftest`, `test/functional`, `test/integration` and `verify/deps` dispatch through it | Does not apply as it stands, and its predicate is used. It answers a one-word line through `Answer`, so `all` alone and a single group would answer their own payload, where `TestAllRunsTheCheckoutThenTheTagGuardedGroups` pins the outer sweep. The unit area keeps `Sweep` for every line of groups and asks the same `TakesArguments` before it, so no existing payload changes. Adopting the whole function later changes that test's expectation and goes through `/ze-test-judge` |
| Job admission | `internal/le/job/job.go`, `Admission.Run` | Queue, claim, attach, run, release | Extends by use: the action admits under `unit-pkg`, or `vet-pkg` for a vet |
| Quiet mode | `internal/le/job/quiet.go`, `runQuiet`; `report.go`, `Report` | Send the child's output to a session log and answer the verdict, the log and the key lines | Extends: the quiet run becomes callable from another `le` package with a directory and an environment, and its text gains the wait |
| Failure-line reader | `internal/le/runlog/runlog.go`, `Key`, `FailedTests` | One declaration of which log lines say what broke | Extends: two readers beside them, one for the lines under each failed test and one counting packages that passed. `Key` is unchanged, so the verification failure index is too |
| Reached-package selector | `internal/le/repo/changed/scope.go`, `Packages` | Packages and tags a change reaches | Extends by use: the `changed` word consumes it, and reads `Widened` and `Reason` |
| Lint's `scope` and `changed` | `internal/le/go/lint/actions.go`, `changedScope` | Name packages as one string value, or take the change set; a widened answer is passed through, so the lint runs on the whole tree | Extends in part: `changed` is reused with its meaning, its conflict refusal and its empty answer. Its handling of a widened answer is not copied: a whole-tree test is not an agent's to start, so the action refuses it. `scope` does not apply: a string value needs shell quoting for two packages |
| One-line refusal of a retired name | `internal/le/le/root/dispatch.go`, `refuseRetired` | Answers a retired command in one line naming the successor, exit 2 | Extends: an unknown command answers in the same form, naming the nearest registered names |
| Namespace and command registry | `internal/le/le/root/dispatch.go`, `Commands`, `members`, `Hidden`, `NamespaceSummary` | Hold every command name and namespace | Extends by use: the nearest-name candidates are read from it, so no list of names is written |
| Commit body wrapper | `internal/le/commit/input.go`, `wrapBody` | Wraps at 72 without breaking words, refuses a word longer than the line | Extends: the refusal goes, and the over-long word takes a line of its own |
| Raw-command refusal | `internal/le/hookruntime/bash.go`, `rawHeavy`, `admittedCommand` | Refuses a raw `go test` and names the long admitted form | Extends: `rawHeavy` names the short action where it can express the caller's line, and matches `go vet` |
| Duration record | `internal/le/job/registry.go`, `appendDuration`, `reportPrevious` | One row per released claim | Extends: one more column at the end |

## Data Flow (MANDATORY)

### Entry Point
- `./le test unit pkg path <package> [path <package>...] [run <regex>] [vet]` or `./le test unit pkg changed [vet]`, typed by an agent or the owner.
- `./le <mistyped words>`, a `./le commit create` body, and a raw `go test` or `go vet` in a Bash call.

### Transformation Path
1. The dispatcher resolves `test unit` and hands the remaining words to the area. A line whose first word is an action that takes arguments goes to `Area.Answer`; every other line sweeps as today.
2. The action table parses the words, refusing a `path` value that begins with a dash. The action body resolves the packages: every `path` value, or the selector's answer for `changed`.
3. The body refuses a line naming both `path` and `changed`, a line naming neither, and `run` beside `vet`. It refuses the whole tree and its near forms: a `path` of `./...`, a `path` equal to a group's whole pattern, a `path` holding a group that declares its own tags, a `path` holding two or more groups, and a `changed` answer that is widened or holds `./...`. It then sets aside every package under a group that declares its own tags: a named one is refused, and one that `changed` reached is left out and named in the answer.
4. The toolchain builds the argv (`go test` with timeout, tags and race, or `go vet` with tags) and the environment a host group uses.
5. The quiet-run seam admits the job under its label, sends the child's output to the session log, and answers the job report with the wait. In production the seam is the quiet run of `internal/le/job`.
6. The action reads the log through `runlog` for the packages that ran a test and passed, the packages that ran none, and the lines under each failed test, and answers one report whose text is the failure lines, then the summary line.
7. On release, the ticket writes its row with the wait as the fourth column.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Session ↔ shared machine | job admission, the registry under `tmp/.ze-jobs`, the duration file | Yes: `admittedQuietRun` (`pkg.go`) admits through `job.NewIn` and `RunQuiet`; the main thread's real `./le test unit pkg changed vet` was admitted and wrote the four-column row `vet-pkg 220 2026-11-11T10:57:47Z 0` |
| `le` ↔ the Go toolchain | argv and environment, no shell | Yes: `pkgLine.command` takes the argv from `Toolchain.GoTest` and `GoVet` and `answerPkg` the environment from `Toolchain.Environment`; `TestPkgActionReportsARedFixturePackage` runs a real `go test` child through that path, and the real `changed vet` run vetted 516 packages, exit 0 |
| Hook ↔ every session's Bash call | `rawHeavy`'s verdict and message | Yes: `TestRawGoTestRefusalNamesTheShortAction` and `TestRefusalAdviceIsAcceptedByTheHook` drive the hook's `Run`, and the golden rows `./le ai hooks unit` judges hold a raw and a quoted `go vet`. One false positive found in use, a `go vet` quoted in a grep inside a `for` loop, is a journal row owned by `spec-tooling-one-off-defects` |

### Changed-Wire Census

**Search evidence:** Scope: the whole checkout, dot directories included, without `.git`, `vendor`, `tmp`, `bin`. Terms: `ze-verify-duration` over every text file of any language (`grep -rIl`); `DurationFile`, `appendDuration`, `reportPrevious` over Go under `internal`, `cmd`, `pkg`; `verify-duration` again over `.claude`, `.github`, `scripts`, `test`, `website`. Positive control: the first term found the declaration (`internal/le/job/job.go`) and four non-Go files, so the search reaches both. Findings: one writer, one parsing reader, one test reader, two name-only carriers, three prose mentions, and no shell, Python or workflow consumer. The live file on this machine holds 28,911 three-column rows.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Duration row: label, run seconds, UTC time, tab-separated, one per line | writer | `internal/le/job/registry.go`, `appendDuration`, called by `Ticket.Release` in `job.go` | Read: builds the three columns and appends with `O_APPEND` | Change: a fourth column, the whole seconds admission waited, after the time |
| same | reader | `internal/le/job/registry.go`, `reportPrevious` | Read: splits on tab, needs two or more columns, reads the second | Preserve: a fourth column is ignored. A test pins both row widths |
| same | reader (test) | `internal/le/verify/current_test.go`, the duration assertion | Read: checks the file opens with the label and a tab | Preserve: unchanged by a trailing column |
| same | reader (name only) | `internal/le/ai/hooks/parity.go`, `sharedScratchName`; `parity_catalog.go`, the redirect row | Read: both hold the file's name to let a redirect into shared scratch pass; neither opens the file | Preserve: no format is read |
| same | reader (a person) | `docs/contributing/running-commands.md`, "How long a verify takes"; `plan/post-release/engineering/spec-verify-scope-0-umbrella.md`, a `tail` in a deliverable row | Search hit; prose and a hand-run `tail` | Change the page: it gains the column list and the meaning of the fourth. The other spec's `tail` still reads a row |
| same | reader (prose) | `ai/rationale/git-safety.md` | Search hit: names the file and a writer, `_release` in `answer.go`, that does not exist | Change: name `Ticket.Release` and `appendDuration` |
| same | reader (existing rows) | the file itself on each machine | 28,911 rows of three columns here | Preserve: old rows stay, and a reader treats a missing fourth column as "wait not recorded", never as zero |
| same | reader (planned) | child 5 of the umbrella | The umbrella's decision D-5 reads the wait record | The column list on the page is its contract |

### Integration Points
- `Toolchain.GoTest`, `Toolchain.GoVet`, `Toolchain.Environment` - called by the action body.
- The quiet run in `internal/le/job` - called by the action body with the checkout root and the toolchain environment.
- `runlog` - two new readers the action calls on the quiet log.
- `repochanged.Packages` - called for `changed`.
- `Area.Answer` - reached from the unit area's `answer` for a line that opens with an argument-taking action.
- `dispatchLine` and `helpAsked` - both answer an unknown command through one function.
- `rawHeavy` - builds the refusal's replacement.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The action parses through its table and runs through admission; it starts no child of its own |
| No unintended coupling (components stay isolated) | Yes | `test/unit` imports `job`, `runlog` and `repo/changed`, as `go/lint` already does; none imports `test/unit` |
| No duplicated functionality (extends existing, does not recreate) | Yes | No second argv builder, admission path, log reader pattern or selector |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Tooling, no wire encoding |
| Registration over hardcoding, outbound | Yes | One action row in the area's own table; help, completion and `le help ai` derive from it |
| Registration over hardcoding, inbound | Yes | Searched for lists that would have to learn `pkg`: the dispatcher (`resolve`, `members`), the command-path test, the group table and the hook. None names an action. The nearest-name candidates come from `Commands` and the namespace registry. The tag refusal reads the group table. The one literal is the hook's replacement text, which names a command as its message |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | After child 1 lands, `rawHeavy` and `admittedCommand` still exist under those names, and `admittedCommand` writes the `quiet` form | Child 1's Existing Mechanisms rows for the two functions | Phase 6 edits whichever function then builds the raw-test replacement | `gopls symbols internal/le/hookruntime/bash.go` at the start of phase 6 | broken in its last clause: both names exist, `rawHeavy` now in `hookruntime/rawheavy.go` and `admittedCommand` still in `bash.go`, but `packageRefusal` (`rawheavy.go`) calls `admittedCommand` with quiet false, so the raw-job long form carries no `quiet`; the piped refusal does. The fallback was taken: phase 6 edited the function that builds the replacement |
| A-2 | A `go test` child run through admission with the toolchain's environment keeps the race detector's cgo setting | Read: `commandEnvironment` in `internal/le/job/process.go` keeps the environment it is given and appends `CGO_ENABLED=1` when the argv carries the race flag. A `go vet` child keeps the environment unchanged. Its call site in the stream was not read | The action passes no environment and lets the boundary choose | `TestPkgActionChildEnvironmentEnablesRace` | confirmed: `commandEnvironment` (`job/process.go`) appends `CGO_ENABLED=1` for a `go test` child whose argv or effective `GOFLAGS` carry the race flag and `CGO_ENABLED=0` otherwise, and returns a non-test child's environment unchanged; the test pins it |
| A-3 | The selector's `Packages` are patterns `go test` accepts | `go/lint/actions.go`, `changedScope` joins them into a scope | The action maps them to import paths with `GoList` first | `TestPkgActionChangedRunsTheReachedPackages`, and one real `changed` run at closure | confirmed: the main thread's real `./le test unit pkg changed vet` vetted 516 packages reached from 11 changed files, exit 0, and wrote the duration row `vet-pkg 220 2026-11-11T10:57:47Z 0` with its fourth column |
| A-4 | A keyword spelled `run` or `vet` passes the table's start-up checks and the grammar gate | `checkKeyword` refuses only spacing, a dash and a duplicate | Respell the keyword (`match` for the regex) and update AC-3 | The package's tests start `le`'s tables; `./le cli grammar`, run by the main thread | confirmed by the closure reviewer: `pkgParameters` (`pkg.go`) declares `run` and `vet` as keywords, the table starts in the package tests, and the grammar gate accepts them |
| A-5 | Without `-v`, `go test` prints one `ok` line per passing package and the assertion lines indented under each `--- FAIL:` line | Go's documented output | The readers are written against the format the fixture produces | `TestPkgActionReportsARedFixturePackage` | confirmed: that test runs a real `go test` over a passing and a failing fixture package and asserts one package ok and one failed, the `--- FAIL: TestTriple` line, the assertion text indented beneath it, then the summary |
| A-6 | Nothing outside the repository parses the duration file | The census; the umbrella says its measuring scripts were session scratch and are not kept | A consumer that splits on a fixed column count breaks | The census, repeated at closure | confirmed: `grep -rIl ze-verify-duration` over the checkout without `.git`, `vendor`, `tmp`, `bin` and `cache`, repeated on 2026-11-11, finds `job/job.go`, `ai/hooks/parity.go`, `ai/hooks/parity_catalog.go`, the two pages, the verify-scope umbrella and this spec: no consumer the census lacked |
| A-7 | A test can run the real `go test` over a fixture module, with no `feature-gates.txt` in the fixture root | Read: `TestASecondCheckoutPathReusesTheFirstOnesTestResult` does it today, with a `Toolchain` built as a value (so `New`, which reads the manifest and the pin, is not called), `GOWORK` off, a private `GOENV` and a skip without `go`. The unit area's `answer` takes the toolchain loader as a seam. `job.NewIn(root)` takes the registry root | The functional test uses a helper process in place of `go test` and the real run is demonstrated at closure | `TestPkgActionReportsARedFixturePackage` | confirmed: the test keeps the production quiet-run seam, so `admittedQuietRun` admits through `job.NewIn` at the fixture's own root, with `GOWORK` off, a private `GOENV` and a toolchain built as a value |
| A-8 | The spec's `Depends` row does not stop a claim | A search of `internal/le/spec` finds the field read by the status, roadmap and journal packages only, and by nothing under `claim` | Phases 1 to 5 could not start: the row becomes `-` and the dependency stays stated in phase 6 | The claim of this spec at the start of phase 1 | confirmed by the closure reviewer: the spec was claimed and phases 1 to 7 landed, the last as 6cc68aaba9 |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `changed` starts a whole-tree race run because the selector failed open | A `changed` run whose argv holds `./...` | AC-5 refuses a widened answer and any answer holding `./...`, before admission |
| R-2 | The hook change refuses a raw `go vet` in a place that is not a heavy run, blocking every session | A refusal on a `grep` or a quoted string | It lands after child 1's quote-aware segmenter, and its golden rows include a quoted `go vet` that passes |
| R-3 | A package behind a personality tag is tested without that tag and passes with part of its tests compiled out | A green on a package under the installer group | AC-11 sets it aside by reading the group table. Tests behind a tag no table declares (`integration`) stay invisible, which Known Limitations states |
| R-4 | A vacuous green: a pattern that matches nothing, or a `run` that matches no test | Exit 0 with no package that ran a test | AC-1 counts only packages that ran a test, and answers 1 when none did |
| R-5 | The nearest name misleads: a typo answers a command the caller did not mean | A wrong suggestion in a transcript | The answer never runs anything, lists up to three, and a word with no near name gets none |
| R-6 | An over-long word accepted in a body is a wrapped sentence the author meant to break | A commit with a 200-character line | Only a single word with no space is exempt: a URL, a path, a hash. A line of several words still wraps |
| R-7 | Phase 6 starts before child 1 has rewritten the refusal messages, and conflicts in `bash.go` or in the seal | Child 1's phase 3 or phase 4 commit not in `git log` | Phase 6 starts only after child 1's phase 3 (messages in `bash.go`) and phase 4 (the seal in `fixtures.go`) are committed |
| R-8 | A mixed three- and four-column file is misread by a later consumer as wait zero | A zero-wait median | The page states that a missing fourth column means not recorded |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A wrong `rawHeavy` refuses or passes Bash calls in every session. A wrong unknown-command answer misleads a typo. A wrong `Release` loses duration rows. A wrong wrapper refuses or mangles commit bodies. The action itself is new and breaks only its own callers |
| How is it reverted? | Each phase is one commit and reverts alone. Four-column rows already written stay readable by the three-column reader |
| Who else touches this path? | Child 1 (`hookruntime/bash.go`, lands first), child 4 (same hook file, later), child 5 (reads the wait record), `spec-native-action-job-admission.md` (admits every heavy action; the wait record lands here first) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le test unit pkg help`, through `leroot.Dispatch` | → | the action's row in the unit table | `TestTestUnitPkgIsReachableFromDispatch` |
| `le test unit pkg path <package>`, through `leroot.Dispatch`, with the quiet-run seam replaced | → | the action body: the seam receives the label, the toolchain argv, the checkout root and the environment | `TestDispatchReachesThePkgActionBody` |
| `le test unit pkg path <package>`, through the area's `answer` | → | the action body, the toolchain argv, the quiet-run seam | `TestPkgActionBuildsTheToolchainArgv` |
| a claimed ticket's `Release` | → | `appendDuration` | `TestDurationRecordCarriesTheWait` |
| `le <mistyped>`, through `leroot.Dispatch` | → | the nearest-name answer | `TestUnknownCommandNamesTheNearestRegisteredOne` |
| `Message` with an over-long body word | → | `wrapBody` | `TestMessagePutsAnOverLongWordOnItsOwnLine` |
| a raw `go test` Bash payload, through the hook's `Run` | → | `rawHeavy` | `TestRawGoTestRefusalNamesTheShortAction` |

## Acceptance Criteria

AC-1 changed its spelling: the skeleton wrote `<path>...`, and the action table holds one positional value that cannot repeat (`checkPositional`), so each package follows the repeated keyword `path`. AC-5 gained the whole-tree refusal: the selector answers `./...` with exit 0 when it fails open. AC-11 and AC-12 are new: the source shows a partly empty green (a tagged group's tests compiled out) and lines with no single meaning.

The quiet-run seam is one replaceable function of the unit package: it takes the label, the argv, the directory and the environment, and answers the job report and the exit code. Production points it at the quiet run of `internal/le/job`. A test replaces it through a function that returns the restore, as `SetTestTempDirForTest` does in the toolchain. It is not the `actionRunner` seam of the groups, which answers a `gaterun.ActionReport`.

How a package is matched against a group: the `path` value, or the selector's package, is reduced to a directory relative to the checkout (the module prefix of an import path, a leading `./`, a trailing `/` and a trailing `/...` removed). It is under a group when that directory equals the directory of the group's pattern or lies below it. It equals the group's whole pattern when the directories are equal and the value ends in `/...`. It holds a group when the value ends in `/...` and the directory of the group's pattern lies below the value's directory, so `./internal/...` holds every group and `./internal/component/...` holds three. A value that does not end in `/...` names one directory and holds no group.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `./le test unit pkg path <package> [path <package>...]` on green packages | Runs `go test` on them with the toolchain's timeout, feature tags and race detector, admitted under `unit-pkg`, and prints one line: `test unit pkg: exit 0, <n> packages ok, waited <s>s, log <path>`. A package counts as ok only when it ran a test: an `ok` line marked `[no tests to run]` and a `?` line marked `[no test files]` are counted apart and shown as `<m> ran no test`. When the child exits 0 and no package ran a test, with or without `run`, the action answers 1 and the line reads `test unit pkg: exit 1, no test ran (go test exited 0), log <path>`, naming the `run` value when one was given | The log holds the full output. The wait clause is absent at zero. Every other exit code is the child's. A run where some packages ran tests and others none stays green, with both counts shown |
| AC-2 | The same on a red package | Prints each failed test's `--- FAIL:` line with the lines indented beneath it, each with its log line number, and build or panic lines when no test line exists, then the summary line with the counts of packages ok and failed | Up to the quiet mode's cap of 20 lines; the log holds the rest and the summary names it |
| AC-3 | `run <regex>` beside the `path` words | The child's argv carries `-run <regex>`, so only matching tests run | A `run` that matches no test in any package is the red of AC-1, never a green |
| AC-4 | `vet` beside the `path` words or after `changed` | Runs `go vet` on the same packages under the same tags, admitted under `vet-pkg`, which type-checks them and their test files. Green prints `test unit pkg: exit 0, vet, waited <s>s, log <path>` | No package count: a green vet prints nothing per package |
| AC-5 | `changed` in place of `path` | Runs the packages `repochanged.Packages` answers for this checkout. A widened answer, or any answer holding `./...`, is refused with exit 2 before admission: the message gives the selector's reason, says a whole-tree run is the owner's (`./le test unit all`), and shows the `path` form to name packages | A change reaching no Go package prints that nothing is reached and exits 0. A selector that answers a non-zero code is refused with exit 2, naming `./le repo changed scope print packages`. The selector's `Tags` are not used: the run carries the full feature set |
| AC-6 | A claimed job's release, and any quiet run | The duration row holds the waited seconds as a fourth column, and a quiet summary shows `waited <s>s` when above zero | Older three-column rows stay readable. An attached run and a run inside a parent's slot write no row, as today, because they held no slot; their summary still shows the wait |
| AC-7 | A raw `go test` or raw `go vet` typed by an agent | Refused, and the message's replacement is the short action with each of the caller's own packages after `path`, and the caller's `-run` value as `run <regex>` | `ZE_ADMIT_RAW` with a reason passes. A `go test` line carrying a flag the short action does not supply (`-v`, `-count`, `-bench`, `-c`, `-cover`, `-json`, `-args` and any other) keeps the long admitted form from child 1, with one line naming the short action, because dropping the flag would change what the run does |
| AC-8 | An unknown `le` command, run or asked about with `le help` | Exit 2 and two lines on standard error: `error: unknown command: <word>. Nearest: le <name>` and `List every command: le help`. Nothing runs | A word near several names lists up to three. A word near none gets the first line without a nearest clause |
| AC-9 | A commit body holding one word longer than the line | Accepted: the word sits on a line of its own, with its indent, and the lines around it wrap as before | A subject over the limit and a hand-written trailer line stay refused |
| AC-10 | `docs/contributing/testing.md`, `docs/contributing/running-commands.md`, `docs/guide/developer-setup.md`, the rule point, the door and the setup note | Each one-package example is the short action. An example that keeps the long form carries the tags. Every example spells the launcher `./le` | The long form stays documented as the generic route for any other command |
| AC-11 | A `path` under a group that declares its own tags (the installer), a `path` that holds such a group, or a `changed` answer reaching such a package | A named `path`, under the group or holding it, is refused with exit 2 before admission: the message names the group and the command `./le test unit <group>`. A package `changed` reached is left out of the run, and the answer carries one line naming it and that command | Under the feature tags the package's tagged tests compile out while its untagged ones run, so a green would cover part of the package and read as all of it. Not covered, and stated in Known Limitations: tests behind a tag no table declares (`integration`), and files guarded by the absence of a feature tag |
| AC-12 | A `pkg` line with neither `path` nor `changed`; with both; with `run` and `vet`; a `path` value that begins with a dash; a `path` of `./...`; a `path` equal to a group's whole pattern; a `path` that holds two or more groups | Refused with exit 2 before admission, naming the word at fault and the form that works. The dash is refused by the parser (`keywordValue`). `./...` names `./le test unit all` as the owner's run. A group's pattern names `./le test unit <group>`. A value holding two or more groups is refused as a near-whole-tree run, naming the groups it spans and the `path` form for the packages meant | A sub-package of a group that declares no tags runs: it is the scoped test this action exists for. A value ending in `/...` that lies inside one group, or holds no group directory, runs. A value holding exactly one group that declares no tags runs: it spans less than two groups and hides no test |
| AC-13 | `./le commit create ... body-file <path>`, the file written with the Write tool. Added in phase 6: an inline body that names an environment-changing word or a heavy command is refused by the Bash hook | The file's text is the body, wrapped and gated exactly as `body` chunks holding the same text: each line wrapped at 72 with its indent, a blank line a paragraph break, a line starting with a declared trailer key refused. The keyword is one row of the create table, so `le help`, completion and the grammar derive it, and `docs/contributing/committing.md` lists it. It also needs one `createPolicy` entry (`internal/le/commit/actions.go`), which every live create keyword carries: it gives the body-file the same attestation as `body` | `body` is unchanged. `body` beside `body-file` is refused with exit 2, naming both and the fix. A missing file and a file holding no text are refused with exit 2, naming the file and the fix |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTestUnitPkgIsReachableFromDispatch` | `internal/le/unit_pkg_wiring_test.go` | Wiring: the help word after `pkg` answers its page with the four words, exit 0 | |
| `TestDispatchReachesThePkgActionBody` | `internal/le/unit_pkg_wiring_test.go` | Wiring: the real dispatcher reaches the body, observed at the replaced quiet-run seam | |
| `TestPkgActionBuildsTheToolchainArgv` | `internal/le/test/unit/pkg_test.go` | AC-1: argv, label and environment handed to the quiet-run seam, two `path` values in order | |
| `TestPkgActionGreenAnswersOneLine` | `internal/le/test/unit/pkg_test.go` | AC-1: the summary text for a green log | |
| `TestPkgActionCountsOnlyPackagesThatRanATest` | `internal/le/test/unit/pkg_test.go` | AC-1: `[no tests to run]` and `[no test files]` are counted apart | |
| `TestPkgActionAnswersRedWhenNoTestRan` | `internal/le/test/unit/pkg_test.go` | AC-1 and AC-3: exit 1 when the child exits 0 and no package ran a test, with and without `run` | |
| `TestPkgActionChangedRefusesAWidenedAnswer` | `internal/le/test/unit/pkg_test.go` | AC-5: exit 2, the selector's reason, nothing admitted | |
| `TestPkgActionRefusesTheWholeTree` | `internal/le/test/unit/pkg_test.go` | AC-12: `path ./...` and a `path` equal to a group's pattern | |
| `TestPkgActionRefusesAPathSpanningGroups` | `internal/le/test/unit/pkg_test.go` | AC-12: `path ./internal/...` and an import-path `.../internal/component/...` refused with 2, nothing admitted, naming each group spanned and the `path` form (closure review N6) | |
| `TestPkgActionRefusesADashLeadingPath` | `internal/le/test/unit/pkg_test.go` | AC-12: the parser's refusal, nothing admitted | |
| `TestPkgActionChangedLeavesOutAPackageBehindAGroupTag` | `internal/le/test/unit/pkg_test.go` | AC-11: the rest runs and the answer names the package and the group command | |
| `TestPackageIsMatchedAgainstAGroupPattern` | `internal/le/test/unit/pkg_test.go` | The matching rule, one row each: sub-package, import-path spelling, trailing slash, the whole pattern, a `/...` value holding a group with its own tags (refused, AC-11), a `/...` value holding two or more groups (refused, AC-12), a `/...` value inside one group and one holding no group (both run) | |
| `TestPkgActionRedPrintsFailuresThenTheSummary` | `internal/le/test/unit/pkg_test.go` | AC-2: order and counts | |
| `TestPkgActionRunKeywordSelectsTests` | `internal/le/test/unit/pkg_test.go` | AC-3 | |
| `TestPkgActionVetBuildsTheVetArgv` | `internal/le/test/unit/pkg_test.go` | AC-4: argv, label, summary | |
| `TestPkgActionChangedRunsTheReachedPackages` | `internal/le/test/unit/pkg_test.go` | AC-5 through a selector seam | |
| `TestPkgActionChangedReachingNothingExitsZero` | `internal/le/test/unit/pkg_test.go` | AC-5 survivor | |
| `TestPkgActionChangedRefusesWhenTheSelectorCannotAnswer` | `internal/le/test/unit/pkg_test.go` | AC-5 refusal | |
| `TestPkgActionRefusesAPackageBehindAGroupTag` | `internal/le/test/unit/pkg_test.go` | AC-11 for a named `path`, read from the group table | |
| `TestPkgActionRefusesALineWithNoSingleMeaning` | `internal/le/test/unit/pkg_test.go` | AC-12: neither word, both words, `run` with `vet` | |
| `TestPkgActionChildEnvironmentEnablesRace` | `internal/le/test/unit/pkg_test.go` | A-2 | |
| `TestUnitGroupsStillSweepBesideThePkgAction` | `internal/le/test/unit/pkg_test.go` | Preserve: a group line returns a sweep | |
| `TestFailuresKeepsTheLinesUnderEachFailedTest` | `internal/le/runlog/failures_test.go` | AC-2: the block reader and its cap | |
| `TestPackageResultsTellAPassFromNoTestRun` | `internal/le/runlog/failures_test.go` | AC-1: the two counts | |
| `TestQuietRunTakesADirectoryAndAnEnvironment` | `internal/le/job/quiet_test.go` | The exported quiet run | |
| `TestDurationRecordCarriesTheWait` | `internal/le/job/duration_test.go` | AC-6: four columns | |
| `TestPreviousRunReadsThreeAndFourColumnRows` | `internal/le/job/duration_test.go` | AC-6 survivor | |
| `TestQuietSummaryShowsTheWaitWhenAboveZero` | `internal/le/job/quiet_test.go` | AC-6: text at zero and above | |
| `TestUnknownCommandNamesTheNearestRegisteredOne` | `internal/le/le/root/nearest_test.go` | AC-8, one row per rank of the measure | |
| `TestUnknownCommandListsAtMostThreeCandidates` | `internal/le/le/root/nearest_test.go` | AC-8 survivor | |
| `TestUnknownCommandWithNoNearNamePointsAtHelp` | `internal/le/le/root/nearest_test.go` | AC-8 survivor | |
| `TestHelpOfAnUnknownCommandAnswersTheSameLines` | `internal/le/le/root/nearest_test.go` | AC-8: the `helpAsked` site | |
| `TestUnknownCommandRunsNothing` | `internal/le/le/root/nearest_test.go` | AC-8: no handler called, exit 2 | |
| `TestMessagePutsAnOverLongWordOnItsOwnLine` | `internal/le/commit/input_test.go` | AC-9 | |
| `TestMessageOverLongWordAddsNoEmptyLine` | `internal/le/commit/input_test.go` | AC-9: first word of a line, indented or not | |
| `TestMessageStillRefusesAnOverLongSubject` | `internal/le/commit/input_test.go` | AC-9 survivor | |
| `TestRawGoTestRefusalNamesTheShortAction` | `internal/le/hookruntime/bash_test.go` | AC-7 through the hook's `Run` | |
| `TestRawGoVetIsRefusedNamingTheShortAction` | `internal/le/hookruntime/bash_test.go` | AC-7 | |
| `TestRawGoTestWithAnotherFlagKeepsTheLongForm` | `internal/le/hookruntime/bash_test.go` | AC-7 survivor, one row per flag class; the long form keeps an assignment (env), a launcher and a directory change (env -C, re-review ISSUE), review I1 | |
| `TestRawGoTestAdviceKeepsAQuotedFilter` | `internal/le/hookruntime/rawheavy_test.go` | AC-7, review I1: the raw-job block keeps a quoted -run value and its package, piped or not, and an assignment through env | |
| `TestRawGoVetShortLineSaysVet` | `internal/le/hookruntime/rawheavy_test.go` | AC-7, review N6: the vet short line says vet | |
| `TestRawGoTestMoveTheFormCannotCarry` | `internal/le/hookruntime/rawheavy_test.go` | AC-7, re-review ISSUE: a move `env -C` cannot express keeps the long form with no move and a sentence naming it | |
| `TestRefusalAdviceIsAcceptedByTheHook` | `internal/le/hookruntime/rawheavy_test.go` | AC-7 and AC-12, re-review ISSUE and N7: every advised command passes the whole pretool-bash hook, main thread and subagent, a subagent's behind its identity line | |
| `TestRawJobAdviceUsesTheActionsOwnWords` | `internal/le/test/unit/hook_advice_test.go` | re-review N6: the hook's copied labels, command and keywords equal the action's constants | |
| `TestRawGoTestAdvisesTheShortFormOnlyWhereTheActionRuns` | `internal/le/test/unit/hook_advice_test.go` | AC-7 with AC-11 and AC-12, review N7: the short form only where the action runs the package | |
| `TestLossyPipeRefusalNamesTheQuietAdmittedForm` | `internal/le/hookruntime/bash_test.go` | AC-12, review I1 and I2: quoted rows, and the filter's own pipeline advised | |
| `TestScratchAdviceCarriesTheSubagentIdentityLine` | `internal/le/hookruntime/refusal_messages_test.go` | review N3: the scratch advice behind the agent's identity line | |
| `TestIdentityRefusalEndsWithTheFixingLine` | `internal/le/hookruntime/refusal_messages_test.go` | review N4: the exact line last; leading whitespace before the agent's own line passes | |
| `TestTestDeletionFixNamesThePathAndTheRoute` | `internal/le/hookruntime/bash_test.go` | review N1: the real path in the remove line | |
| `TestTestDeletionRefusalNamesTheNextAction` | `internal/le/hookruntime/bash_test.go` | review N1: the path for rm; leave-the-changes and no remove for a discard | |
| `TestBuildTreeRefusalNamesWhatItJudged` | `internal/le/hookruntime/bash_test.go` | review N5: a text-judged subtree is not called the whole tree | |
| subagent-context rows `payload-without-agent-id`, `payload-with-an-unusable-agent-id` | `internal/le/hookruntime/lifecycle_test.go` | review N2 and I3: a fork's whole line; an unusable agent id refused for every command, readers included | |
| `TestCreateBodyFileIsWrappedAndGatedLikeBody` | `internal/le/commit/body_file_test.go` | AC-13: same message as the `body` chunks, trailer line refused | |
| `TestCreateRefusesBodyBesideBodyFile` | `internal/le/commit/body_file_test.go` | AC-13 survivor: both keywords refused, exit 2 through `prepareCreate` | |
| `TestCreateBodyFileRefusesAFileWithNoText` | `internal/le/commit/body_file_test.go` | AC-13 survivor: a blank and a missing file | |
| `TestDocsShowTheShortPackageAction` | `internal/le/test/unit/pkg_test.go` | AC-10: the six surfaces hold the short action, no `go test` or `go vet` of a package without `-tags`, and no `bin/le` run as a command | |
| `TestSurfaceLinesAreJudgedByEveryRule` | `internal/le/test/unit/pkg_test.go` | AC-10, closure review I1: one seeded line per rule of `surfaceFaults` goes red (a tagless `go test` in a span, a fenced `go vet`, a prompt and an assignment, a placeholder package, a Go string; `bin/le` in a table-cell span, opening a cell, behind `./`, in a span opened on the line above, in a Go string), and the lines the surfaces hold that only name a flag or the installed copy stay green | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Failure lines printed | 0 to 20 | 20 | N-A | the 21st stays in the log |
| Nearest candidates listed | 0 to 3 | 3 | N-A | a fourth is counted, not listed |
| Edit distance accepted | 1 to 2, and under half the typed length | 2 | N-A | 3 gives no nearest name |
| Body word width | 1 to 72 with its indent | 72 wraps as before | N-A | 73 takes its own line |
| Waited seconds shown | 1 and above | 1 | 0 prints no clause | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestPkgActionReportsARedFixturePackage` | `internal/le/test/unit/pkg_test.go` | Shaped on `TestASecondCheckoutPathReusesTheFirstOnesTestResult`: a module written under a temporary directory, a toolchain built as a value, `GOWORK` off, a private `GOENV`, a skip without `go`. The real action body runs a real `go test` child, through a registry under the test's own root, over one passing and one failing fixture package: the answer holds the `--- FAIL:` line, the assertion text beneath it, one package ok and one failed | |
| `./le ai hooks unit` | `internal/le/ai/hooks/parity_catalog.go` | The golden rows: a raw `go vet` is refused, a quoted `go vet` in a `grep` passes, `ZE_ADMIT_RAW` passes | |
| Demonstration at closure | the main checkout | `./le test unit pkg path ./internal/le/job/` and `./le test unit pkg changed vet`, output pasted in the closure section | |

## Files to Modify
- `internal/le/test/unit/actions.go` - the `pkg` action row; `answer` hands a line that opens with an argument-taking action to `Area.Answer`
- `internal/le/test/unit/groups.go` - one function answering the group whose pattern holds a package and whose tags are its own
- `internal/le/test/unit/register.go` - the description names the package action
- `internal/le/job/quiet.go` - the quiet run exported, with a directory and an environment
- `internal/le/job/report.go` - the wait in the quiet text
- `internal/le/job/job.go` - `Release` passes the ticket's wait
- `internal/le/job/registry.go` - `appendDuration` writes the fourth column
- `internal/le/job/answer.go` - calls the exported quiet run
- `internal/le/runlog/runlog.go` - the failure-block reader, and the count of packages that passed apart from those that ran no test
- `internal/le/le/root/dispatch.go` - `dispatchLine` and `helpAsked` answer through the nearest-name function
- `internal/le/commit/input.go` - `wrapBody` places an over-long word on its own line
- `internal/le/hookruntime/bash.go` - `rawHeavy`, after child 1
- `internal/le/hookruntime/rawheavy.go` - `rawHeavy` and the raw-job refusal's advice, moved out of `bash.go` in phase 6
- `internal/le/hookruntime/lossypipe.go` - `lossyPipeRefusal` advises the caller's own command in the quiet admitted form behind the identity line, and the short action for a `go test`
- `internal/le/commit/actions.go` - the `body-file` row of the create table and its `createPolicy` entry (AC-13)
- `docs/contributing/committing.md` - lists `body-file` (AC-13)
- `internal/le/ai/hooks/parity_catalog.go` - golden rows for `go vet`
- `internal/le/ai/hooks/fixtures.go` - the source seal, re-derived
- `internal/le/setup/setup.go` - the closing note
- `docs/contributing/testing.md` - first-time setup, the ladder, the unit examples, the single-test examples, the cheat sheet
- `docs/contributing/running-commands.md` - "A bare `go test`", "How long a verify takes", "Testing one package", the quiet example
- `docs/guide/developer-setup.md` - "After Setup"
- `ai/rationale/git-safety.md` - the writer's name
- `ai/rules/points/testing/native-test-actions/run-the-focused-test-through-a-native-action.md` - names the short action; the condensed rule files follow from `./le ai rules update condensed`
- `ai/doors/le-tooling.md` - the Boundaries bullet and the Gates row

## Files to Create
- `internal/le/test/unit/pkg.go` - the action body and its report
- `internal/le/test/unit/pkg_test.go` - its tests
- `internal/le/unit_pkg_wiring_test.go` - the dispatch wiring test
- `internal/le/le/root/nearest.go` - the nearest-name measure
- `internal/le/le/root/nearest_test.go` - its tests
- `internal/le/runlog/failures_test.go` - the two readers' tests
- `internal/le/job/duration_test.go` - the record tests

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `le` is development tooling; no RPC and no config |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | Yes | `internal/le/test/unit/actions.go`, one action; no flag |
| CLI grammar (keyword before value) | Yes | Closed keywords only: `path` (repeated), `run`, `vet`, `changed`; no positional value. `./le cli grammar` run by the main thread |
| Editor autocomplete | Yes | Derived from the action table; nothing to write |
| Functional test for new RPC/API | N-A | no RPC; the action's functional test is in the TDD plan |
| Pipe completeness | Yes | The report is a structured payload with kebab-case keys answered through the shared renderer; the area's registered document shape is unchanged |
| Env var registration | No | no new variable |
| Doctor check for runtime dependencies | N-A | `ze doctor` covers the product; the action needs only the Go toolchain `le` already activates |
| Doctor code per problem class | N-A | no product problem class |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface | N-A | tooling |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | `features/` lists product features; this is developer tooling |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md`, `docs/contributing/testing.md`; `docs/guide/command-reference.md` covers `ze`, not `le` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md` |
| 7 | Wire format changed? | No | the duration file is a local record, described on the running-commands page |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/testing.md`; `docs/functional-tests.md` covers `.ci` suites and is unaffected |
| 11 | Affects daemon comparison? | No | none |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` describes the unit area as one command with an action table, which stays true; its one `job run` example shows how `--help` reaches the child and is unaffected |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registered command is added; an action joins an existing one |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived with `./le spec citation anchors spec plan/post-release/engineering/spec-agent-economy-2-scoped-test.md` on 2026-11-11. Declared: `docs/features/ai-first.md`, the design page of `internal/le/commit/input.go`. It is unaffected: a search of the tree for the refusal's word `unwrappable` finds it in `input.go` alone, so the page does not state the body-word refusal; phase 5 reads the page's commit text once more before it lands. `running-commands.md` anchors `job.go`, `registry.go` and `gotoolchain.go`, and is in Files to Modify. The command also lists six pages that mention a changed file (advisory); each is read in the phase that edits that file |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | The six surfaces of AC-10. Left as they are, with the reason: `docs/architecture/site-facts.md` (a golden update needing `-args`), `internal/le/test/cost/golog.go` (its reader needs `-v`), `internal/le/test/qemu/alltests.go` (an `integration` tag the feature set lacks) |

## Implementation Steps

Each phase is one editing agent of about 60 tool calls and ends in one commit. The main thread runs every test.

Only phase 6 depended on child 1 (`spec-agent-economy-1-call-shape`), whose phases 3 and 4 are committed and which has closed, so the `Depends` row is `-`. Phases 1 to 5 edit no Go file child 1 edits. Phases 3 and 7 edit `docs/contributing/running-commands.md`, which child 1's phase 1 also edits, in other sections.

1. **Phase: Wiring (MANDATORY FIRST)** -- the action exists and is reachable
   - Tests: `TestTestUnitPkgIsReachableFromDispatch`, `TestDispatchReachesThePkgActionBody`, `TestPkgActionBuildsTheToolchainArgv`, `TestPkgActionRunKeywordSelectsTests`, `TestPkgActionVetBuildsTheVetArgv`, `TestUnitGroupsStillSweepBesideThePkgAction`, `TestQuietRunTakesADirectoryAndAnEnvironment`, `TestPkgActionChildEnvironmentEnablesRace`
   - Files: `test/unit/actions.go`, `pkg.go`, `pkg_test.go`, `register.go`, `job/quiet.go`, `job/answer.go`, `unit_pkg_wiring_test.go`
   - Verify: the wiring tests fail before the row exists and pass after; the main thread runs the `test/unit`, `job` and `internal/le` package tests and `./le cli grammar`
2. **Phase: what the action answers** -- the summary, the red path, `changed`, the refusals
   - Tests: the remaining `TestPkgAction...` tests, the two `runlog` tests, `TestPkgActionReportsARedFixturePackage`
   - Files: `pkg.go`, `groups.go`, `runlog/runlog.go`, `failures_test.go`
   - Verify: tests fail → implement → pass; one real run of the action on a green package, by the main thread
3. **Phase: the wait record** -- persist and print the wait
   - Tests: `TestDurationRecordCarriesTheWait`, `TestPreviousRunReadsThreeAndFourColumnRows`, `TestQuietSummaryShowsTheWaitWhenAboveZero`
   - Files: `job/job.go`, `registry.go`, `report.go`, `duration_test.go`, and in the same commit the two page sections that describe the record and the quiet line, and `ai/rationale/git-safety.md`
   - Verify: the `job` and `verify` package tests named by the census
4. **Phase: the unknown-command answer**
   - Tests: the five `nearest_test.go` tests
   - Files: `le/root/nearest.go`, `dispatch.go`
   - Verify: the `le/root` and `internal/le` package tests. An existing assertion that the full usage follows an unknown command, if one is found, changes only through `/ze-test-judge`
5. **Phase: the commit body**
   - Tests: the three `input_test.go` tests
   - Files: `commit/input.go`
   - Verify: the `commit` package tests
6. **Phase: the refusal messages** -- starts only once child 1's phase 3 (the refusal messages in `bash.go`) and phase 4 (the seal in `fixtures.go`) are committed
   - Tests: the three `bash_test.go` tests, the golden rows, and for AC-13 the three `body_file_test.go` tests
   - Files: `hookruntime/bash.go`, `hookruntime/rawheavy.go` (the raw-job refusal's advice, moved out of `bash.go` so that file does not grow past 1000 lines), `hookruntime/lossypipe.go`, `ai/hooks/parity_catalog.go`, `parity.go`, `fixtures.go`; for AC-13 `commit/actions.go`, `commit/input.go`, `docs/contributing/committing.md`
   - Child 1's phase 3 already met part of the piped-command side: `lossyPipeRefusal` names `./le test unit pkg path <package>` for a piped go test and a verify. AC-7 itself (the raw, unpiped refusal) was unmet and is this phase's
   - Verify: the `hookruntime` package tests and `./le ai hooks unit`
7. **Phase: pages and the rule point**
   - Tests: `TestDocsShowTheShortPackageAction`
   - Files: `docs/contributing/testing.md`, `running-commands.md`, `developer-setup.md`, the rule point, `ai/doors/le-tooling.md`, `setup/setup.go`; then `./le ai rules update condensed` and `./le ai sync update`
   - Verify: `./le doc check all`, by the main thread

### The nearest-name measure

The candidates are every command name `Commands` answers that is not hidden, and every namespace token. The typed words are the line's own words before any pipe, at most `CommandWordsMax`. The first rank that yields a candidate decides, and candidates within a rank are listed in name order.

| Rank | A candidate is near when | Example |
|------|--------------------------|---------|
| 1 | Its words equal the typed words once order, spaces and hyphens are ignored | `unit test` and `test-unit` answer `test unit` |
| 2 | Its last words equal the typed leading words | `lint` answers `go lint` |
| 3 | The typed first word is within edit distance 2 of one of its words, counting a swap of two neighbours as one, and the distance is under half the typed word's length | `comit` answers the `commit` commands |

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line and a test that goes red without it |
| Correctness | The action's exit code is the child's, with one exception: 1 when the child exits 0 and no package ran a test. No path reaches a whole-tree run: `./...` typed, a group's pattern, a widened `changed`. A missing fourth column is never read as zero |
| Naming | The report's JSON keys are kebab-case. The labels are `unit-pkg` and `vet-pkg`. The keywords match the table: `path`, `run`, `vet`, `changed` |
| Data flow | The action starts no child itself: every run goes through admission. The unit area's group lines still sweep |
| No second copy | No tag list, no failure pattern and no command-name list is written outside the toolchain, `runlog` and the registry |
| Operator errors | Each refusal says what is wrong, why, and the exact command, with exit 2 (`ai/rules/user-facing-errors.md`) |
| Stale text | No comment or page still says `job run` is the only route to a package test, or that the summary answers three things. `.claude/hooks/README.md` is searched for the old replacement text |
| Rule: `ai/rules/rule-format.md` | The rule point keeps its front matter and one directive per bullet, and the condensed files are regenerated, never edited |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The action is listed | `./le help test unit` shows `pkg` with its four words |
| The action is in the agent reference | `./le help ai` output holds `test unit pkg` |
| A green run answers one line | `./le test unit pkg path ./internal/le/runlog/` prints one line and exits 0 |
| The whole tree is refused | `./le test unit pkg path ./...` exits 2 and names `./le test unit all` |
| The wait is recorded | `tail -1 tmp/.ze-verify-duration.txt` shows four columns after that run |
| An unknown command answers in two lines | `./le lint` prints two lines naming `le go lint`, exit 2 |
| An over-long word is accepted | `./le commit create` with a body holding an 80-character URL writes a script |
| The hook names the short action | `./le ai hooks unit` passes with the new rows |
| No tagless example | `grep -n "command go test \./" docs/contributing/testing.md docs/guide/developer-setup.md` finds nothing |
| No literal launcher path | `grep -n "command bin/le" docs/contributing/testing.md` finds nothing |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Argument injection | Packages and the regex reach `go` as argv words, never through a shell. The parser refuses a dash-leading value of `path` and of `run` (`keywordValue`), so no word can become a `go` flag such as `-exec` |
| Whole-tree run | No input reaches `./...`: typed, widened by the selector, or a group's pattern |
| Path handling | The log path comes from the quiet run, built from a validated label and a hash; the action builds no path from its input |
| Resource exhaustion | Every run is admitted; the failure lines are capped; the readers hold one line at a time |
| Autocorrection | The unknown-command answer never runs the nearest command |
| Commit text | The wrapper still refuses a line that starts with a declared trailer key, checked on the lines the commit will carry |
| Hook fail-open | A `go test` or `go vet` the mapping cannot express is still refused, with the long form |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A-1 false | Phase 6 edits the function that builds the raw-test replacement after child 1, named in Design Insights |
| A-2 or A-3 false | Take the fallback in the assumption's row and record it under Design Insights |
| A-4 false | Respell the keyword, update AC-3 and the pages, record it |
| An existing test asserts the old unknown-command or quiet text | `/ze-test-judge` before the expectation changes |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The unit groups are not admitted: they stream through `gaterun.Run`. Admitting them is `spec-native-action-job-admission.md`'s work, not this spec's.
- The wait was already in the job's JSON payload; only the text and the file lacked it.
- Phase 7: the functional-runner examples on `docs/contributing/testing.md` wrapped `bin/le test <suite>` in `./le job run`. Every `le test` runner already admits itself (`RunnerAnswer`, `internal/le/test/harnesstool/harnesstool.go`), so the examples became the bare `./le test bgp plugin 42` and its siblings rather than `./le` inside `job run`.
  → Decision: the AC-10 check `TestDocsShowTheShortPackageAction` judges three things on each surface: it holds `./le test unit pkg path `, every line holding `command go test` or `command go vet` holds `-tags`, and no line holds `command bin/le` or opens with `bin/le `. Prose that names `bin/le` as the installed copy stays.
  → Decision: a one-package example the short action cannot express keeps the long form and gains `-race -tags "$tags"`: the userns `-v -count=1` evidence run, the `-v` cost-report run, and an `integration`-tagged run on `running-commands.md`.
- Phase 7 found `docs/contributing/running-commands.md` naming the quiet log `job-<label>.log`, where `quietLog` (`internal/le/job/quiet.go`) has written `job-<label>-<first eight hex of the job key>.log` since 2026-09-03, and saying that two runs of one label overwrite it, where only two runs of the same command do. The page was repaired, with the attribution and cost-report examples that passed that path.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The short form is an action of the registered command `test unit` | A new registered command (`le go test`, in a new package), or a `pkg` form of `le job run` | The area already owns the toolchain, the group table and the word an agent looks for under `le help test`. An action adds no command name, so the command-path test, the namespace registry and the dispatcher are untouched. `job run` runs any program and must stay ignorant of Go |
| The action calls admission's quiet run as Go | Re-running `le job run label unit-pkg quiet command ...` as a child process | One process, a structured report, and no second trip through a grammar. `go/lint` already admits in process |
| Each package follows the repeated keyword `path` | One comma-separated positional value; a repeating positional (a parser change in `le/action`); lint's `scope` string | `Parameter.Repeat` with `Arguments.Values` is the list pattern `verify status check path` and `commit create file` already use. `ai/rules/cli.md` puts a keyword before every value. `keywordValue` refuses a dash-leading value, so the action body needs no check of its own. A comma list is a spelling no other `le` action has, a parser change reaches every action for one caller, and a string needs shell quoting |
| `changed` refuses a widened answer | Passing it through, as lint's `changedScope` does | The selector fails open to `./...` with exit 0, and a whole-tree race run is not an agent's to start. The refusal carries the selector's reason and the `path` form |
| A run in which no package ran a test answers 1 | Returning the child's 0 with a note | A `run` that matches nothing, or a pattern holding no test, is a question that got no answer; exit 0 would be read as a pass by any caller that reads the code |
| The unit area asks `TakesArguments` and keeps `Sweep` | Dispatching through `Area.AnswerOrSweep` | The whole function answers a one-word line through `Answer`, which changes the payload `all` and a single group return and the test that pins it |
| `vet` is a keyword of the same action | A sibling action | Packages, tags, admission and the summary are the same; only the argv builder differs. A sibling would declare the package grammar twice |
| The race detector is always on | A word to turn it off | A package tested without the race detector has not been tested the way the gate tests it. The long form still serves a run without it |
| A package behind a group's own tags is refused when named, and left out and named when `changed` reaches it | Applying that group's tags to the package | The installer group also pins a platform and falls back to vet off Linux; reproducing that for one package is a second copy of the group. The refusal names the group, which does it correctly |
| The red path prints the lines under each failed test | The existing key lines only | The key lines name the test and not why it failed, so the agent's next call is a read of the log. `Key` stays as it is because the verification index shares it |
| The wait is a fourth column at the end of the row | A second file, or a new column order | Every existing reader takes the first two or three columns. A second file would split one fact |
| An attached run writes no row | One row per admitted run | A row means a slot was held. A follower's wait is the holder's run time, and counting it would double the label's cost |
| The nearest name is ranked: reordered words, then trailing words, then spelling | Edit distance alone | A command typed by its last word (`lint` for `go lint`) or with its words swapped is at a large edit distance from the real name, so distance alone finds neither |
| The hook maps only a line the action expresses exactly | Dropping `-v` and `-count` from the suggestion | A suggestion that silently changes what the run does is a wrong answer; the long form with `quiet` is always correct |
| A vet runs under its own label | The `unit-pkg` label | The previous-run banner and the record are per label, and a vet's seconds would distort a test's |

## Known Limitations
- An unknown action inside an area and an unknown member of a namespace keep their present answers, which are already one or two lines. A nearest-name answer there is not part of this spec.
- A package test without the race detector, with `-v`, with a benchmark or with a coverage profile stays on the long `job run` form.
- `le commit message` takes no `body-file`: only `commit create` reads one (AC-13), so a message prepared alone still takes its body inline.
- The unit groups stay unadmitted; `plan/post-release/engineering/spec-native-action-job-admission.md` owns that.
- `rawHeavy` (`hookruntime/rawheavy.go`) compiles its launcher-option regexp on each call rather than once at package level (closure review N5). The cost is one compile each time it checks a word that follows a launcher such as `timeout` and does not start with a dash, and hoisting it changes a hook source, which re-derives the hook seal for every session, so it stays as it is.
- The action runs under the full feature tag set. Tests behind a tag no table declares, such as `integration`, compile out and the summary cannot say so; they stay on the long `job run` form with their tag. A file guarded by the absence of a feature tag compiles only in a build that drops the tag; the action ignores the selector's `Tags`, and the compile-out builds stay with `./le repo compiles check` and CI.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Every user story has a working path and a passing test (N-A: Scope is tooling)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests pass, run by the main thread in the main checkout: the packages under `internal/le/test/unit`, `internal/le/job`, `internal/le/runlog`, `internal/le/le/root`, `internal/le/commit`, `internal/le/hookruntime`, `internal/le/verify` and `internal/le`, with `./le ai hooks unit`, `./le cli grammar` and `./le doc check all`. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior: the fixture-package test and `./le ai hooks unit`
- [ ] Interop tests for protocol features (N-A: tooling, no protocol peer)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
