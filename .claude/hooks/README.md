# Claude Hooks

All registered hooks execute in the compiled root `le` command. The canonical
configuration is `.claude/settings.json`; each entry invokes:

```text
$CLAUDE_PROJECT_DIR/le ai hooks <hook-name>
```

The runtime lives in `internal/le/hookruntime`. `internal/le/ai/hooks` owns the
registered command surface, the 208 typed dispatcher fixtures, and the 607 typed
behavior fixtures. Hook payloads are read from standard input as JSON. Hook
protocol output is written directly by the Go runtime, and exit codes retain the
Claude severity contract: 0 permits, 1 warns, and 2 blocks.

<!-- source: internal/le/hookruntime/runtime.go -- nativeHookActions, Run, runLifecycleHook -->
`nativeHookActions` is the runtime authority for the four check groups. A check
this page names but that registry does not hold is a defect in this page.

## Native runtime families

| Hook family | Native owner |
|---|---|
| Bash command guards | `internal/le/hookruntime/bash.go` |
| Write/Edit guards | `internal/le/hookruntime/writeedit.go` |
| Post-write formatting and advisories | `internal/le/hookruntime/postwrite.go` |
| Session identity and parent propagation | `internal/le/hookruntime/session.go` |
| Session, marker, compaction, stop, and validation hooks | `internal/le/hookruntime/lifecycle.go` |
| JSON dispatch and shared scratch identity | `internal/le/hookruntime/runtime.go` |

Session identity resolution and dated session paths are canonical in
`internal/le/le/path/session.go`. Test weakening is judged by
`internal/le/test/weakened`, journal rows by `internal/le/spec/journal`, and spec session
state by `internal/le/spec`. The hook runtime calls those packages
in-process rather than launching a second implementation.

## Event wiring

<!-- source: .claude/settings.json -- hooks -->

| Event | Matcher | Action |
|---|---|---|
| SessionStart | every | `session-start` |
| UserPromptSubmit | every | `compaction-reminder` |
| PreToolUse | `Bash` | `pretool-bash` |
| PreToolUse | `Write\|Edit\|MultiEdit\|NotebookEdit` | `pretool-writeedit` |
| PostToolUse | `LSP` | `mark-lsp-invoked` |
| PostToolUse | `Read` | `mark-source-read` |
| PostToolUse | `Task\|Agent` | `mark-agent-spawned` |
| PostToolUse | `Write\|Edit` | `validate-spec`, `posttool-writeedit` |
| PreCompact | every | `pre-compact-save` |
| Stop | every | `block-premature-stop`, `rule-coverage-report`, `session-end-summary`, `session-end-deferrals` |
| SubagentStart | every | `subagent-context` |

A dispatcher returns the worst verdict of its checks, and `pretool-bash` also
writes the parent session prefix when nothing blocked.

Each section below carries one table under a heading naming its Go source. The
`Enforces` column names the rule stems that check's `// ze point:` bindings
name. `hookTableProblems` (`internal/le/ai/rules/hooktable.go`) compares the two
columns against the Go registry, so a new check owes a row here and a deleted
check's row cannot survive it.

## PreToolUse: Bash (`internal/le/hookruntime/bash.go`)

<!-- source: internal/le/hookruntime/bash.go -- bashWorktreeCopy, bashDestructiveGit, bashBranchMove, bashRootBuild, bashLossyPipe, bashRawHeavy, bashPollLoop, bashSystemTmp, bashScratch, bashTestDeletion, bashGovernedWrite, preMaterializeDerived, bashStyleGuideRead -->

| Check | Enforces | What it refuses |
|---|---|---|
| `bashWorktreeCopy` | `ai/INSTRUCTIONS.md` prohibition, no rule point | A file copy out of a worktree into the main tree. A worktree agent lands its work by committing, and a direct copy overwrites whatever another session left uncommitted at the destination. |
| `bashDestructiveGit` | `git-safety.md` | Every git verb that discards work or publishes it. Committing and pushing are allowed through the prepared script alone, because sessions share one index. |
| `bashBranchMove` | `git-safety.md` | Every git verb that creates, switches, renames, deletes or integrates a branch. Which branch a session works on is the user's choice, and a session that moves it lands the work where the user is not looking. |
| `bashRootBuild` | build hygiene, no rule point | A Go compile that names no output path. Without `-o` the binary lands in the working directory under the package name. |
| `bashLossyPipe` | `commands.md` | Piping an expensive command through a filter that drops output. The part a filter removes is usually the part that says why the run failed. |
| `bashRawHeavy` | `commands.md` | A heavy job typed raw, outside job admission. One machine carries several sessions, and an unadmitted job oversubscribes it. |
| `bashPollLoop` | `commands.md` | An unbounded wait loop. A loop with no timeout holds the session captive. |
| `bashSystemTmp` | `testing.md` | The system temporary directory. Session scratch belongs under the per-session directory, which is reaped with the session. |
| `bashScratch` | `commands.md` | Ad-hoc scratch written at the `tmp/` root. Sessions share that tree, so an unqualified name collides. |
| `bashTestDeletion` | `testing.md` | Deleting a test without approval. A deleted test is indistinguishable from a test that never existed. It fails closed and judges the whole line: it asks when a word, quoted or not, is `rm`, ends in `/rm`, or is `-delete`, AND any word is a test path. A test path is a `*_test.go`, `.ci` or `.et` file, a path with a `test` or `testdata` element, or, inside `internal/` or `test/`, a path with any element whose name contains `test` (`internal/component/cli/testing`, `internal/foo/*test*`); `test/draft/` is exempt, and outside those two trees a name that only contains "test", such as `docs/architecture/testing/`, is not one. Words break at whitespace and at `; & | ( ) { } ! < >`, so a redirect glued to a path leaves the path whole. A line with an unclosed quote that runs `rm` is asked about. A line that names `rm` without running it, such as `grep rm x_test.go`, is asked about too. Known gaps: a bare word at the checkout root with no `/`, such as `rm -r test` or `rm -r test*`, because on a whole line `test` is `go test` or the test builtin far more often than the directory; a command inside a `bash -c` string or inside `$(...)` in double quotes; `find ... -delete` with no test word on the line; `rm -rf` of a directory that holds tests but has no element naming `test`; `git clean`; `mv`; and truncation or a redirect that overwrites a test. |
| `bashGovernedWrite` | `commands.md` | A shell write into `plan/` or `ai/rules/`. Those trees are guarded by the Write/Edit hook, and a shell write runs none of its checks. |
| `preMaterializeDerived` | `principles.md` | A command naming a derived artifact the tree does not hold. It rebuilds the artifact first, and refuses the command when the rebuild fails, because a grep of an absent file answers "no match" for a tree nobody rendered. |
| `bashStyleGuideRead` | `go-standards.md` | Never refuses. A command that prints the whole of `docs/contributing/ze-go-style.md` with `cat`, `bat` (no line range), `nl`, `less` or `more`, and has no pipe, writes the marker `writeStyleGuideRead` reads. `head`, `tail` and `sed` print a range and record nothing. It runs before the command, so a command that then fails still leaves the marker. |

## PreToolUse: Write/Edit (`internal/le/hookruntime/writeedit.go`)

<!-- source: internal/le/hookruntime/writeedit.go -- writeLineCitation, writeGenerated, writeRenderedRule, writePointOverwrite, writePointLanguage, writeDesignEvidence, writeSpecStatus, writeStyleGuideRead, writeGoPatterns, writeFilePatterns, writeWeakening, writeCISleep, writeYangDescription -->

| Check | Enforces | What it refuses |
|---|---|---|
| `writeLineCitation` | `evidence.md`, `writing.md` | A line-number citation in repository prose. |
| `writeGenerated` | `repo-maintenance.md` | An edit to a generated file. The verdict names its source. |
| `writeRenderedRule` | `repo-maintenance.md` | An edit to a rendered rule, which `ai/rules/points/` owns. |
| `writePointOverwrite` | `never-destroy-work.md` | A Write that replaces an existing rule point. |
| `writePointLanguage` | `rule-format.md` | A rule directive that states no RFC 2119 level. |
| `writeDesignEvidence` | `evidence.md` | A spec or design written before its source was read. |
| `writeSpecStatus` | `planning.md` | A source edit while the claimed spec is not `in-progress`. |
| `writeStyleGuideRead` | `go-standards.md` | A Go Write, Edit or MultiEdit from a session, or a subagent, with no record of reading `docs/contributing/ze-go-style.md`. A subagent's payload carries the parent's `session_id` and its own `agent_id`, so each subagent needs its own read: the marker is `tmp/session/.style-guide-read-<session>`, with `-agent-<agent_id>` appended for a subagent. An id that cannot name a marker is refused. Go written through a Bash heredoc never reaches this check. |
| `writeGoPatterns` | `architecture.md`, `cli.md`, `go-standards.md`, `performance.md`, `quality.md`, `plugins.md`, `goroutine-lifecycle.md` | The Go patterns the rules ban, file by file: handlers, panic, legacy logging, allocating formatting, nolint, init registration, switch dispatch, anonymous goroutines, and fake buffer handles. |
| `writeFilePatterns` | `architecture.md`, `commands.md`, `config.md`, `quality.md`, `testing.md` | The file-level patterns the rules ban, by path: path shape, package name, scratch location, lint exclusion, config version, and CI observers. |
| `writeWeakening` | `testing.md` | A proposed edit that lowers what a test proves. |
| `writeCISleep` | `testing.md` | A pause in a `.ci` test that names no justification marker. |
| `writeYangDescription` | `writing.md` | A proposed YANG `ze:help` past 96 characters or 25 words. Also one carrying a semicolon or no full stop, and a `description` that repeats the `ze:help` beside it. It judges the statements that render on a one-line row, and a `leaf` or `leaf-list` only in a config module. It reports the summaries whose owner it did not find, it warns rather than blocks, and `./le doc yang-contract help-shape` is the gate. |

`writeGoPatterns` is the edit-time allocation-pattern check. It blocks
`fmt.Sprintf`, `fmt.Fprintf`, `fmt.Printf`, and `strconv.FormatInt` or
`strconv.FormatUint` in production Go. The broader allocation audit stays with
its native verification action.

## PreToolUse: Bash and Write/Edit, the editing agent's budget (`internal/le/hookruntime/budget.go`)

<!-- source: internal/le/hookruntime/budget.go -- bashCallBudget, writeCallBudget, agentCallBudgetCheck -->

| Check | Enforces | What it does |
|---|---|---|
| `bashCallBudget` | `context-economy.md` | Counts a `ze-work` agent's Bash calls in `tmp/session/<parent>/agents/<agent_id>.calls` and refuses the call past 100, except one that names the per-spec state file or asks for its path. The refusal tells the agent to append its handoff and report that the package needs a continuation. |
| `writeCallBudget` | `context-economy.md` | The same counter over Write, Edit, MultiEdit and NotebookEdit. |

The budget binds the editing agent only: `agent_type` in the hook payload names
it, a read-only agent cut mid-review would lose coverage, and the main thread
carries no `agent_id`. Read and ToolSearch pass unhooked, so the count is of
the calls the hook sees, under a tenth short of the agent's total.

## PostToolUse: Write/Edit (`internal/le/hookruntime/postwrite.go`)

<!-- source: internal/le/hookruntime/postwrite.go -- postFormatGo, postFileSize, postDeferral, postJournal, postRFCHeader, postTestDocs, postFuzz, postVague, postBoundary, postInvalidateDerived -->

| Check | Enforces | What it does |
|---|---|---|
| `postFormatGo` | `quality.md` | Formats the edited Go file, then refuses it while the linter still reports an issue in its package. |
| `postFileSize` | Go style guidance, no rule point | Reports a Go file past the 1,000-line advisory. |
| `postDeferral` | heuristic advisory, no rule point | Reports deferral language in a document that is not a deferral. |
| `postJournal` | journal format, no rule point | Reports a journal file that `./le commit create` would refuse. |
| `postRFCHeader` | Go style guidance, no rule point | Reports a file citing RFCs with no `// RFC:` header. |
| `postTestDocs` | Go style guidance, no rule point | Reports a test file with no `VALIDATES:` or `PREVENTS:` comment. |
| `postFuzz` | advisory, no rule point | Reports a wire parser whose package carries no fuzz target. |
| `postVague` | Go style guidance, no rule point | Reports a vague variable name in the edited Go file. |
| `postBoundary` | advisory, no rule point | Reports numeric validation whose sibling test names no boundary. |
| `postInvalidateDerived` | `principles.md` | Removes every derived artifact the written file feeds, so a later reader rebuilds it rather than reading a map of the tree as it stood before the edit. |

## Lifecycle actions (`internal/le/hookruntime/lifecycle.go`)

<!-- source: internal/le/hookruntime/lifecycle.go -- runLifecycleHook -->
These return hook protocol output directly rather than a check verdict, so
`nativeHookActions` does not hold them. `runLifecycleHook` dispatches each one.

| Action | Event | What it does |
|---|---|---|
| `session-start` | SessionStart | Validates the raw session ID, publishes the accepted ID, and prints status. It deletes nothing; `./le session reap` owns proof-based cleanup. |
| `compaction-reminder` | UserPromptSubmit | Detects compaction and reminds the session to read the post-compaction rule. |
| `pre-compact-save` | PreCompact | Saves session state before compaction. |
| `block-premature-stop` | Stop | Refuses the stop while the claimed spec is implemented but not closed. Warns in one line when the claim is `in-progress` or no subagent was spawned. It never reads the last message. |
| `rule-coverage-report` | Stop | Reports which rule points this session's transcript exercised. |
| `session-end-summary` | Stop | Calls `./le session end-summary`. It preserves handoffs and never releases a spec claim. |
| `session-end-deferrals` | Stop | Prints the open deferral count. Advisory. |
| `subagent-context` | SubagentStart | Validates the parent session ID and emits the parent context. |
| `mark-lsp-invoked` | PostToolUse `LSP` | Writes the session-scoped LSP marker. |
| `mark-source-read` | PostToolUse `Read` | A Read of `docs/contributing/ze-go-style.md` that returns every line (no offset past line 1, and a limit, or the tool's 2000-line cap, that reaches the last line) writes the style-guide marker `writeStyleGuideRead` reads. A Read of a `.go`, `.sh`, `.yang`, `.mk` or `Makefile` path writes a source-read evidence marker for its kind. Neither inspects what the Read returned. |
| `mark-agent-spawned` | PostToolUse `Task\|Agent` | Writes the session-scoped agent marker. |
| `validate-spec` | PostToolUse `Write\|Edit` | `hookValidateSpec` validates the spec's Wiring Test table. |

`writeDesignEvidence` in `writeedit.go` consumes the LSP and source-read markers
before a design or spec write, and `hookStop` reads the agent marker. Reads never
block.

`UserPromptSubmit` stdout reaches the model and its stderr does not.

## Commit-time gates (`internal/le/commit`)

<!-- source: internal/le/commit/prepare.go -- Create -->
These are not Claude hooks. `Create` runs them when `./le commit create`
generates the commit script, which is the only sanctioned commit path. The
helper knows the exact add and remove set, so the gates inspect that instead of
the shared staging area. A refused gate writes no script.

| Gate | Producer | Override keyword |
|---|---|---|
| test-weakening | `testweakened.ProspectiveCommit`, `testweakened.CheckCommit` | none |
| rfc-approval | `rfcChangeProblems` (`rfcchange.go`), reading `tmp/commit-rfc-approved-<session>.md` and writing the `RFC-approved:` trailer | `rfc-change-ok` |
| test-coverage | `testCoverageProblems` (`prepare.go`) | `no-test` |
| verify-status | `verificationState` (`verification.go`) | `unverified`, `missing-full-verify-ok` |
| structural-gate | `structuralGateReds` (`verification.go`) | `structural-red-ok`, `broken-head-fix` |
| independent review | `closureStem`, `CheckReview` (`review.go`) | `review-override` |

<!-- source: internal/le/commit/debt.go -- debtGates, recordDebt, openDebt -->
An override does not discharge the obligation. `recordDebt` writes one row per
overridden gate under `plan/verification-debt/`, and `openDebt` refuses a push
while any row is open. `debtGates` names the six keys above, plus `stale-index-ok`, whose gate was deleted on 2026-09-11 and whose declaration survives so the ledger rows already naming it stay clearable.

The review gate runs only for a closure commit, which `closureStem` identifies.
`ROUND_CAP` and `cmd_record` in `internal/le/spec/review.go` price the
review round count at `./le spec review record`, not here.

## Changed-file gates (`./le doc wiring`)

<!-- source: internal/le/doc/wiring/docwiring.go -- checker.run -->
These are native `./le` actions rather than Claude hooks, and `./le verify
worktree` runs them.

| Check | Name | What it refuses |
|---|---|---|
| `checkSleepRatchet` | `ci-sleep-ratchet` | How MANY pauses the `.ci` corpus holds, against a committed delta baseline. |
| `checkSleepJustification` | `ci-sleep-justification` | How many of those pauses are UNEXPLAINED: each needs a comment above or trailing it. |
| `checkLoadExcuses` | `known-failure-load-excuse` | A changed `plan/known-failures/` shard that blames host load. |
| `checkLogSubsystemKeys` | `ci-log-subsystem-key` | A `ze.log.<subsystem>` key whose subsystem carries a hyphen that no Go literal declares. `getLogEnv` splits on `.` only, so `ze.log.bgp.adj-rib-in` sets nothing and the level stays at the WARN default. |
| `checkDesignRefs` | `design-refs` | A `// Design:` reference naming no document. Unconditional in a repository checkout, because closing a spec orphans references in any source file. |
| `checkDocDrift` | `doc-drift` | A commit that changed a symbol a `<!-- source: -->` anchor claims while that page stood still. It maps each diff hunk to its declaration, so an edit elsewhere in the file reports nothing. |

`checkWiring` runs beside them and reports each exported symbol the change adds
that no non-test file under `internal/` or `cmd/` names.

## Tree checks (`./le repo`)

<!-- source: internal/le/repository/repository.go -- Run -->
`./le repo tree-check` declares an empty changed set and runs the three
tree-wide checks. `./le repo check` runs all five over the current tree.

| Check | Scope | What it reports |
|---|---|---|
| `checkSourceAnchorLineNumbers` | tree-wide | A `<!-- source: -->` anchor carrying a line number, because line numbers rot. |
| `checkSourceAnchorStalePaths` | tree-wide | A repo-relative anchor path that no file or directory answers. |
| `checkSpecACCompleteness` | tree-wide | An acceptance-criterion row of an `in-progress` spec whose `Demonstrated By` cell is empty. |
| `checkCrossPackageWiring` | changed files | An exported symbol with no cross-package non-test caller. Not in the gate. |
| `checkCLIHandlerCoverage` | changed files | A newly registered command that no `.ci` test names. Not in the gate. |

The two changed-file checks take `git diff HEAD` plus untracked files as their
subject. Several sessions share this checkout, so that list carries other
sessions' half-written work, and both checks demand a completeness a file in the
middle of an edit cannot show. They stay out of the gate.

## Prose gate (ASD-STE100)

<!-- source: internal/le/doc/ste/actions.go -- Actions -->
`./le doc ste check` compares each changed file with its own text at HEAD and prints
only the habits that grew, so a document nobody touched can never fail.
`./le doc ste review` reports the whole tree and `./le doc ste review-changed` reports
the working tree's changes. Surfaces are Markdown under `docs/`, `ai/`, `plan/`
and the root, prose comments in `.go`, and `description` strings in `.yang`.
STE is a guideline, so no gate refuses on it.

## Hook tests

| Runner | Covers |
|---|---|
| `internal/le/ai/hooks/parity.go` | Golden exit-code regression over the 208 dispatcher fixtures, in four tables, one per registered check group. |
| `./le ai hooks unit` | The 607 typed behavior fixtures: spec validation, source-read evidence, commit-time gates, delegation, and the registered write and Bash checks. |

Run the focused hook proof with:

```text
./le ai hooks unit
```

RFC ownership and weakening probes use isolated Git repositories with the
original test committed at HEAD before proposing a whole-file Write. `git init`
alone is insufficient: unreadable committed text must still fail closed.
A refusal probe must report an unauthorized evidence change, not fixture setup
or committed-text read failure. The draft-incubator control commits both
original paths so its outside-draft refusal tests ownership.
<!-- source: internal/le/ai/hooks/fixtures.go -- committedProbeTree, draftIncubatorTree, probeVerdict -->
