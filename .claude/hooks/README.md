# Claude Hooks

Every hook that checks or blocks executes in the compiled root `le` command. The
canonical configuration is `.claude/settings.json`; each such entry invokes:

```text
$CLAUDE_PROJECT_DIR/le ai hooks <hook-name>
```

Every hook, the display-only one included, is that command: no hook is a shell
script.

The runtime lives in `internal/le/hookruntime`. `internal/le/ai/hooks` owns the
registered command surface, the 219 typed dispatcher fixtures, and the 551 typed
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
| PreToolUse | `Bash` | `pretool-bash`, `bash-timestamp` |
| PreToolUse | `Write\|Edit\|MultiEdit\|NotebookEdit` | `pretool-writeedit` |
| PostToolUse | `LSP` | `mark-lsp-invoked` |
| PostToolUse | `Read` | `mark-source-read` |
| PostToolUse | `Task\|Agent` | `mark-agent-spawned` |
| PostToolUse | `Write\|Edit\|MultiEdit` | `validate-spec` |
| PostToolUse | `Write\|Edit` | `posttool-writeedit` |
| PostToolUse | `Bash` | `posttool-bash` |
| PreCompact | every | `pre-compact-save` |
| Stop | every | `block-premature-stop`, `rule-coverage-report`, `session-end-summary`, `session-end-deferrals` |
| SubagentStart | every | `subagent-context` |

A dispatcher returns the worst verdict of its checks. No dispatcher changes the
tool input: `pretool-bash` checks a subagent's identity line
(`bashAgentIdentity`, below) and never adds it.

Each section below carries one table under a heading naming its Go source. The
`Enforces` column names the rule stems that check's `// ze point:` bindings
name. `hookTableProblems` (`internal/le/ai/rules/hooktable.go`) compares the two
columns against the Go registry, so a new check owes a row here and a deleted
check's row cannot survive it.

## PreToolUse: Bash (`internal/le/hookruntime/bash.go`)

<!-- source: internal/le/hookruntime/bash.go -- bashWorktreeCopy, bashDestructiveGit, bashBranchMove, bashRootBuild, bashLossyPipe, bashRawHeavy, bashPollLoop, bashSystemTmp, bashScratch, bashTestDeletion, bashGovernedWrite, preMaterializeDerived, bashDocumentRead -->
<!-- source: internal/le/hookruntime/bash.go -- textLossyPipe, expensiveWords -->
<!-- source: internal/le/hookruntime/lossypipe.go -- treeLossyPipe -->
<!-- source: internal/le/le/root/heavy.go -- HeavyLine, CarrierLine, RegisterHeavy, RegisterCarrier -->
<!-- source: internal/le/hookruntime/rawheavy.go -- textRawHeavy, treeRawHeavy, rawHeavyWords -->
<!-- source: internal/le/hookruntime/parsedcommands.go -- everyCommand, pipelineMembership -->
<!-- source: internal/le/hookruntime/readerdata.go -- withoutReaderData, readerLineButWrites -->
<!-- source: internal/le/le/root/leroot.go -- RegisterAdmitted, RegisterExpensive, Expensive -->
<!-- source: internal/le/hookruntime/subagent_heavy.go -- withoutHandedBackAdvice, handsBackHeavy -->

| Check | Enforces | What it refuses |
|---|---|---|
| `bashWorktreeCopy` | `ai/INSTRUCTIONS.md` prohibition, no rule point | A file copy out of a worktree into the main tree. A worktree agent lands its work by committing, and a direct copy overwrites whatever another session left uncommitted at the destination. |
| `bashDestructiveGit` | `git-safety.md` | Every git verb that discards work or publishes it. Committing and pushing are allowed through the prepared script alone, because sessions share one index. |
| `bashBranchMove` | `git-safety.md` | Every git verb that creates, switches, renames, deletes or integrates a branch. Which branch a session works on is the user's choice, and a session that moves it lands the work where the user is not looking. |
| `bashRootBuild` | build hygiene, no rule point | A Go compile that writes a binary outside `bin/`, and a whole-tree compile. The go command writes a binary only for one main package (`go help build`), so a build of one non-main package or of several packages passes, by path, `/...` pattern or module import path; `-o bin/...` and a session's `bin/` pass. Refused: one main package with no `-o` or with `-o` outside `bin/`; a build that names no package, or whose packages hold no Go file under its tags (read from `-tags` and from `GOFLAGS`, in the hook's environment or assigned on the line); a package the hook cannot resolve to a directory of the checkout, a word holding an unexpanded variable included; a package outside the checkout (`//...`, `/proc/self/cwd/...`), in a module nested below its root, or a `/...` pattern under its `tmp/`; a build after a `cd` earlier on the line, unless its `-o` resolves to the checkout's `bin/` from the directory the `cd` moved to and the `-C` directory (a `cd` is followed only when it starts the line or follows `;` or a newline and is ended by `;`, `&&` or a newline; a `cd` in a pipeline, behind `&`, after `&&` or `||`, before `\|\|`, to a variable, `cd -`, `cd -P`, or on a line holding a group or subshell leaves the directory unknown, and then only an absolute `-o` under the checkout's `bin/` passes); an `-o` that leaves `bin/` once the working directory, the `-C` directory and the path are resolved as the kernel resolves them, every symbolic link followed, as typed and cleaned, or that holds a link the hook cannot resolve or a character the shell expands; and a whole-tree build whatever the output: `./...`, `...`, `all`, or any `/...` pattern whose cleaned directory is the module root, by import path (`github.com/ze-software/ze/...`) or by path (`.//...`, `./cmd/../...`, an absolute path). A `/...` pattern is walked as the go command walks it, also skipping the checkout's `tmp/` and every directory holding its own `go.mod`; the walk stops at the second package and is refused past 2048 directories. The module path is read once per call. The advice names the caller's own package in the `go build -o bin/<name> <package>` form, and the whole-tree refusal names `./le repo changed scope print both`. A build is judged by its words only on a reader+build line: every other command is a reader its judge admits (see below), the build sets no variable but `GOFLAGS`, passes no `-toolexec`, `-overlay` or `-modfile`, and no command on the line redirects into a file (only `/dev/null` and another descriptor), since a reader could write the package the hook reads. Every other line holding `go build` is judged by its text: refused when it names `-toolexec`, `-overlay` or `-modfile` (joined by `=` or spaced, with one dash or two, or inside a `GOFLAGS` assignment), whatever its `-o`, because each makes the go command run or read what the line names, with a refusal that names the flag and gives the `go build -o bin/<name> ./cmd/<name>` form; refused unless it names `-o bin/` or a session `bin/`; and refused as a whole-tree build when a `...` pattern follows the `go build`. A subagent's build is refused the same way and with the same code, and the guard prints nothing for it (`withoutHandedBackAdvice`): a subagent runs no build wherever its binary goes, so the one text it reads is the hand-back of `bashSubagentHeavy` (below). A subagent's line that check does not hand back, whose text names `go build` in another program's argument, is refused with a text that names no output path: reword the line, or list it under `Owed commands`. |
| `bashLossyPipe` | `commands.md` | Piping an expensive command through a filter that drops output. The part a filter removes is usually the part that says why the run failed. A reader line passes; every other line is judged by its text, as before the guards read commands, and, when it parses, also by its tree: every pipeline at any depth (a loop or conditional body, a group, a subshell stage, a substitution) where an expensive command runs in an earlier stage than a lossy filter (`treeLossyPipe`, `lossypipe.go`), read from command words only, so an expensive command that is an argument of another command passes. An le line, typed as `le`, `./le` or `ze le`, is an expensive command when `leroot.HeavyLine` reads it as heavy and `leroot.CarrierLine` does not read it as a carrier (`expensiveWords`). The hook holds no list of areas: `HeavyLine` resolves the line as dispatch does and answers from what each area declared where its verb registers, which is an admitted area (`leroot.RegisterAdmitted`), a heavy area or a heavy selection of one action (`leroot.RegisterHeavy`), an expensive area or action (`leroot.RegisterExpensive`, such as `ai hooks unit`), or an action row that declares `Heavy`. A help question, a namespace, an unknown word and the bare name of an area that lists its actions start nothing and pass. A carrier (`exec`, which registered with `leroot.RegisterCarrier`, and an action whose row takes a command, such as `job run`) runs a program the line names as an argument, so it is not the expensive command of its pipeline, and the admitted form the refusal advises stays typeable. The text judgment reads the program behind assignments and launchers (`bash`, `sh`, `perl`, `ruby`, `sudo`, `time`, `nice`, `env`, `timeout`), skipping each launcher's options and numeric arguments and, for an option that takes a value (`timeout -s KILL`, `sudo -u u`, `time -o f`, `bash -o pipefail`), its value too, from the one table both launcher loops read (`launcherValueOptions`, `rawheavy.go`). The refusal shows the caller's own command, its assignments, launchers and redirections removed, in the quiet admitted form `./le job run label <label> quiet command <argv>`, and names `./le test unit pkg path <package>` for a `go test`; a verification command gets that scoped run instead. On a line that parses each advised word is one shell word: a value the hook reads exactly is quoted where it needs it, and a word the shell expands is written by its own text as typed (`./$p`, never `'./$p'`), so the advice pasted where the command stood runs what it ran (`parsedCommand.written`, `parsedcommands.go`). It never advises a redirect or a `tee`, which `commands.md` forbids. A subagent's piped heavy line is refused with the same code and no text (`withoutHandedBackAdvice`): the admitted form is one a subagent is refused too, so it reads the hand-back of `bashSubagentHeavy` (below) alone. A subagent's piped command that check does not hand back (a `golangci-lint` subcommand that runs no analysis) is refused with a text of its own that never names the admitted form: the command with no filter behind the caller's identity line, or the main thread's run when the output is too long (`lossyPipeSubagentAdvice`); `withoutHandedBackAdvice` gives a subagent no guard's main-thread text on any line. |
| `bashRawHeavy` | `commands.md` | A heavy job typed raw, outside job admission. One machine carries several sessions, and an unadmitted job oversubscribes it. A reader line passes; every other line is judged by its text, as before the guards read commands, reading the program behind assignments and launchers as `bashLossyPipe` does, a launcher option's value skipped with it, and, when it parses, also every simple command of its tree at any depth by the same word judgment (`treeRawHeavy`, `rawheavy.go`), its assignments read from the command's own assignment nodes, so `for p in a b; do go test ./$p; done` and `( go test ./x )` are refused; it reads command words only, so `./le job run label unit-pkg quiet command go test ./x` passes, and a real `ZE_ADMIT_RAW="<reason>"` assignment in front still passes the job for this guard, which the main thread alone can use: `bashSubagentHeavy` (below) refuses a subagent's job whatever is typed in front of it. The refusal advises `./le test unit pkg path <package>` for a `go test` or `go vet` that short action expresses exactly, and otherwise the caller's own command in the quiet admitted form `./le job run label <label> quiet command <argv>`, which its generic-form line names too. Each advised word is written back as `bashLossyPipe` writes it, a word the shell expands by its own text as typed, and such a word keeps the long form: the short action would judge the typed text, not the path the shell passes. A subagent's raw job is refused with the same code and no text (`withoutHandedBackAdvice`): both the admitted form and the stated reason are refused from a subagent, so it reads the hand-back of `bashSubagentHeavy` (below) alone. |
| `bashPollLoop` | `commands.md` | An unbounded wait loop. A loop with no timeout holds the session captive. |
| `bashSystemTmp` | `testing.md` | The system temporary directory. Session scratch belongs under the per-session directory, which is reaped with the session. On a reader line it judges words, quotes removed: a word that is `/tmp` or starts with `/tmp/`, or such a value after `=`, `:` or `,`, or a `/tmp` under a parent the shell expands (`"$D"/tmp/x`, `"${v}/tmp"`, `~/tmp`), which can expand to `/tmp`; a grep pattern holding a `/tmp` path and a `/tmp` under a literal parent (`foo/tmp/x`) pass, and a redirect to `/tmp` does not. Every other line is judged by its text, as before the guards read commands: `/tmp` after a word boundary, a quote, `=`, `$`, `(`, a backtick, `:`, `,`, a `}` (`${v}/tmp`, which is `/tmp` when `v` is empty), or a redirection operator `>` or `<` (`2>/tmp/err`), which the text judgment before the guards read commands missed, so it refuses a superset of what it refused. |
| `bashScratch` | `commands.md` | Ad-hoc scratch written at the `tmp/` root. Sessions share that tree, so an unqualified name collides. It refuses only a write, a `>` or `>>` redirection or a `tee` into a root name, so a reader line, which writes no file, passes (`grep -rn '> tmp/out.log' ai/rules`). On a line that parses and runs only readers, a here-document body that holds no substitution is data the reader prints, so its text is not scanned (`withoutReaderData`, `readerdata.go`) while the line's own redirection targets are; a body handed to `bash`, `sh` or an interpreter, or on a line that runs any command outside the reader set, keeps its text scanned. |
| `bashTestDeletion` | `testing.md` | Deleting a test without approval. A deleted test is indistinguishable from a test that never existed. The refusal asks no question: it says the agent cannot approve the deletion, names `./le commit create ... remove <path>` for a tracked test the agreed work removes, and otherwise tells the agent to report the path and the reason to the main thread. One line is read closer (`plainRmArguments`, `testdeletion.go`): a line in the modelled syntax with no substitution whose commands are each a plain `rm` with no expanded word or a reader that writes no file, is not `cd` and names no delete word; there only the `rm` arguments are judged, so `rm -f <scratch> && grep x internal/a_test.go` passes. Every other line is judged whole and fails closed: it refuses when a word, quoted or not, is `rm`, ends in `/rm`, or is `-delete`, AND any word is a test path. A test path is a `*_test.go`, `.ci` or `.et` file, a path with a `test` or `testdata` element, or, inside `internal/` or `test/`, a path with any element whose name contains `test` (`internal/component/cli/testing`, `internal/foo/*test*`); `test/draft/` is exempt, and outside those two trees a name that only contains "test", such as `docs/architecture/testing/`, is not one. Words break at whitespace and at `; & | ( ) { } ! < >`, so a redirect glued to a path leaves the path whole. A line with an unclosed quote that runs `rm` is refused. A line that names `rm` without running it, such as `grep rm x_test.go`, is refused too. Known gaps: a bare word at the checkout root with no `/`, such as `rm -r test` or `rm -r test*`, because on a whole line `test` is `go test` or the test builtin far more often than the directory; a command inside a `bash -c` string or inside `$(...)` in double quotes; `find ... -delete` with no test word on the line; `rm -rf` of a directory that holds tests but has no element naming `test`; `git clean`; `mv`; and truncation or a redirect that overwrites a test. |
| `bashGovernedWrite` | `commands.md` | A shell write into `plan/` or `ai/rules/`. Those trees are guarded by the Write/Edit hook, and a shell write runs none of its checks. |
| `preMaterializeDerived` | `principles.md` | A command naming a derived artifact the tree does not hold. It rebuilds the artifact first, and refuses the command when the rebuild fails, because a grep of an absent file answers "no match" for a tree nobody rendered. |
| `bashDocumentRead` | `go-standards.md`, `architecture.md` | Never refuses. A command that prints the whole of a gated document (`docs/contributing/ze-go-style.md`, or a page under `ai/doors/`) with `cat`, `bat` (no line range), `nl`, `less` or `more`, and has no pipe, redirection or substitution, writes that document's read marker, which `writeStyleGuideRead`, `writeDesignDoorsRead` and `writeSpecDoorsRead` read. `head`, `tail` and `sed` print a range and record nothing, and an unexpanded glob names no document. It runs before the command, so a command that then fails still leaves the marker. |

The text judgment is each guard's default verdict, unchanged from before the
build, piped-command, raw-job, `/tmp` and deletion guards judged commands;
the reader line (`judgeReaderLine`) and the build guard's reader+build line
may only lift it. A reader line is one that parses as bash, read once per
call with the `mvdan.cc/sh` parser, whose every node is one the node table in
`docs/contributing/running-commands.md` admits under the identity check
(pipelines and lists, groups, subshells, `!`, `time`, `if`, `while`,
`until`, `for name in`, `case`, `[[` file and string tests, `$( )` and `<( )`
of readers, the plain parameter expansions, here-documents and redirections
onto `/dev/null` or a descriptor, wherever they are written), whose every
simple command is in the reader set `bashAgentIdentity` uses with the
arguments its judge admits and no assignment in front, and that redirects
into no file (`> f`, `>> f`, `>| f`, `&> f`, `2> f`), since a reader could
otherwise write the git or shell configuration a later reader runs. A line
that does not parse, or holds a carriage return or a NUL byte, is never one.
The agent's own exact identity line in front of a reader line leaves it one.
On a reader line the `/tmp` guard judges every word of the tree (loop lists,
case words, assignment values, redirection targets), a here-document body and
a here-string being data, and the deletion guard reads a plain `rm` line from
the tree and every word of any other line, here-document bodies included.
Each judge is an allow-list of the options and argument shapes
it names (`readers.go`), so an abbreviated long option a program would
expand (`git grep --open=bash`, `sort --co=sh`) is refused; `printf`, `test`
and `[` refuse `-v` (and `test` `-R`), which evaluate an array subscript;
`sed` passes only print, delete, quit and substitution scripts with no `e`
or `w` flag; `gopls` passes only its read subcommands; and every judge
refuses a word the shell expands (`$`, an unquoted glob, a leading `~`),
since `$_` can hold an option. A reader with no judge (`cat`, `grep`,
`head` and the other commands whose options run, write and evaluate
nothing) takes expanded arguments, because no value turns one into a run
or a write; only its command word must be text. The same
judgments decide which subagent lines run without the identity line. On a
reader line nothing runs its arguments or its input as a program, so a name
or a separator inside quotes is text: `grep -n -E 'rebuild|go build' le` is
a grep. Every other line, one holding a shell (fed by `-c`, stdin, a
here-string or an rc file), a launcher (`watch`, `su`, `script`, `flock`,
`time`, `xargs`), an interpreter, `awk`, `eval` or any command the set does
not hold or a judge refuses, is judged by its text, so a command such a line
can run from a quoted string is refused there as before. The text match is
not a proof: a reader+build line whose build holds a word the shell expands
(`-tags $_`, `GOFLAGS=$X`) is judged by its text too, and the text judgment
reads only the `-toolexec`, `-overlay` and `-modfile` flags, the `...`
pattern and the `-o` path, so a flag that `$X` expands to is not seen. What
it never saw, such as the command string after a shell's `-c`
(`bash -c 'go test ./x'`, which the text judgment reads as one word) or a raw
`go test` inside a double-quoted `$(...)`, it does not see there either.

<!-- source: internal/le/hookruntime/readerline.go -- judgeReaderLine, ctxReaderLine, readerWalk, persists -->
<!-- source: internal/le/hookruntime/shellparse.go -- parseCommandLine, readWord -->
<!-- source: internal/le/hookruntime/readers.go -- readerCommands, readerCommand -->
<!-- source: internal/le/hookruntime/gobuild.go -- goBuildLine, judgeGoBuild, physicalPath -->
<!-- source: internal/le/hookruntime/gobuild.go -- goBuildText, goBuildToolchainFlag -->
<!-- source: internal/le/hookruntime/rawheavy.go -- launcherValueOptions, launcherArguments -->
<!-- source: internal/le/hookruntime/systemtmp.go -- namesSystemTmp -->

## PreToolUse: Write/Edit (`internal/le/hookruntime/writeedit.go`)

<!-- source: internal/le/hookruntime/writeedit.go -- writeLineCitation, writeGenerated, writeRenderedRule, writePointOverwrite, writePointLanguage, writeDesignEvidence, writeSpecStatus, writeStyleGuideRead, writeDesignDoorsRead, writeGoPatterns, writeFilePatterns, writeWeakening, writeCISleep, writeYangDescription -->

| Check | Enforces | What it refuses |
|---|---|---|
| `writeLineCitation` | `evidence.md`, `writing.md` | A line-number citation in repository prose. |
| `writeGenerated` | `repo-maintenance.md` | An edit to a generated file. The verdict names its source. |
| `writeRenderedRule` | `repo-maintenance.md` | An edit to a rendered rule, which `ai/rules/points/` owns. |
| `writePointOverwrite` | `never-destroy-work.md` | A Write that replaces an existing rule point. |
| `writePointLanguage` | `rule-format.md` | A rule directive that states no RFC 2119 level. |
| `writeDesignEvidence` | `evidence.md` | A spec or design with no current read of its existing producer subjects. Records bind checkout path, content digest and harness context; a generic marker, unrelated source or changed content is not subject evidence. |
| `writeSpecStatus` | `planning.md` | A source edit while the claimed spec is not `in-progress` or `verification`, or its metadata cannot be read and parsed unambiguously. It shares the inventory's metadata-table grammar with direct admission and validation rather than granting from a literal Status substring. |
| `writeStyleGuideRead` | `go-standards.md` | A Go Write, Edit or MultiEdit from a session, or a subagent, with no record of reading `docs/contributing/ze-go-style.md`. A subagent's payload carries the parent's `session_id` and its own `agent_id`, so each subagent needs its own read: the marker is `tmp/session/.doc-read-<escaped document path>-<session>`, with `+<agent_id>` appended for a subagent (`documentReadMarker`, `internal/le/hookruntime/readgate.go`). An id that cannot name a marker is refused. Go written through a Bash heredoc never reaches this check. |
| `writeDesignDoorsRead` | `architecture.md` | A Go (tests included), YANG or templ Write, Edit or MultiEdit from a session, or a subagent, with no record of a whole read of `ai/doors/system.md` and of every door covering the file. The covering doors are those whose `<!-- door-base: ... -->` header names the longest whole-component prefix of the file's path, all of them on a tie; the headers are parsed at call time, so a new door joins by its header. The refusal lists each unread door. Records and fail-closed identity are shared with `writeStyleGuideRead`; a missing `ai/doors/` or a header that does not close is refused, never read as "no door". |
| `writeGoPatterns` | `architecture.md`, `cli.md`, `go-standards.md`, `performance.md`, `quality.md`, `plugins.md`, `goroutine-lifecycle.md` | The Go patterns the rules ban, file by file: handlers, panic, legacy logging, allocating formatting, nolint, init registration, switch dispatch, anonymous goroutines, and fake buffer handles. |
| `writeFilePatterns` | `architecture.md`, `commands.md`, `config.md`, `quality.md`, `testing.md` | The file-level patterns the rules ban, by path: path shape, package name, scratch location, lint exclusion, config version, and CI observers. |
| `writeWeakening` | `testing.md` | A proposed edit that lowers what a test proves, unless a `ze-test-judge` approval for that test is recorded. |
| `writeCISleep` | `testing.md` | A pause in a `.ci` test that names no justification marker. |
| `writeYangDescription` | `writing.md` | A proposed YANG `ze:help` past 96 characters or 25 words. Also one carrying a semicolon or no full stop, and a `description` that repeats the `ze:help` beside it. It judges the statements that render on a one-line row, and a `leaf` or `leaf-list` only in a config module. It reports the summaries whose owner it did not find, it warns rather than blocks, and `./le doc yang-contract help-shape` is the gate. |

`writeGoPatterns` is the edit-time allocation-pattern check. It blocks
`fmt.Sprintf`, `fmt.Fprintf`, `fmt.Printf`, and `strconv.FormatInt` or
`strconv.FormatUint` in production Go. The broader allocation audit stays with
its native verification action.
`writeGoPatterns` reconstructs the whole proposed Go file for Write, Edit and
MultiEdit before judging panic calls individually. A replacement inside a
quoted string or comment is not a call. Each actual call needs its own
permitted invariant prefix and must parse; an allowed call never exempts
another. This syntax check does not prove that a peer cannot reach a permitted
panic.

<!-- source: internal/le/hookruntime/panic.go -- forbiddenPanic -->


### Spec door gate (`internal/le/hookruntime/specdoors.go`)

| Check | Enforces | What it refuses |
|---|---|---|
| `writeSpecDoorsRead` | `architecture.md` | A Write, Edit or MultiEdit of a spec (`plan/**/spec-*.md`, `specpath.IsSpec`) from a session, or a subagent, with no record of a whole read of `ai/doors/system.md` and of every door covering a repository path the spec names. The paths are every slash-holding run of path characters in the spec on disk and in the proposed text (`specRepoPaths`), mapped through the same `door-base` coverage as `writeDesignDoorsRead`; a brand-new spec naming no path owes `system.md` alone. The refusal lists each unread door, and the fail-closed cases are shared through `unreadDoors`. |

### Spec Existing Mechanisms gate (`internal/le/hookruntime/specmechanisms.go`)

| Check | Enforces | What it refuses |
|---|---|---|
| `writeSpecMechanisms` | `planning.md`, `testing.md` | A Write, Edit or MultiEdit of a spec (`specpath.IsSpec`) whose proposed design lacks an Existing Mechanisms data row, or whose non-skeleton runtime change names no functional proof under `test/`. It replays the edit over the file on disk; an Edit whose `old_string` is absent passes to the tool's own failure. `specmechanisms.Problem` is shared with the commit check. `daemonProofProblem` is shared with post-write validation: changed-source exemptions and functional-test permission use canonical identities, including planned paths, and reject unknown/outside resolution. New source roots owe proof; explicit nonproduction roots come from the architecture gate. |

The shared `specmechanisms.Problem` reads Status with `MetadataStatus`; compact
and spaced metadata rows have the same mechanisms obligation at the write hook
and commit consumer. Its existing gated-status population and section/data-row
judgment are unchanged.

### Direct-edit WIP gate (`internal/le/hookruntime/specwip.go`)

| Check | Enforces | What it refuses |
|---|---|---|
| `writeSpecWIP` | `planning.md` | A Write, Edit or MultiEdit that adds a new `in-progress` spec, whether claimed or not. It names the exact `./le spec claim <spec-file>` command: only that path holds the claim lock through the cap check and publication. Parsed, unambiguous metadata is required before granting an already-active or non-transition exemption; missing headers, duplicate Status rows and unknown values do not grant permission. Valid existing-active and non-transition edits remain allowed. |

<!-- source: internal/le/hookruntime/specwip.go -- writeSpecWIP -->
<!-- source: internal/le/spec/session.go -- CheckWIPTransition -->


## PreToolUse: Bash, a subagent's identity (`internal/le/hookruntime/session.go`)

<!-- source: internal/le/hookruntime/session.go -- bashAgentIdentity, identityLine, commandOutsideReaders -->
<!-- source: internal/le/hookruntime/readers.go -- readerCommands, readerCommand -->
<!-- source: internal/le/hookruntime/session.go -- textChangesEnvironment, changesEnvironment -->
<!-- source: internal/le/hookruntime/afterline.go -- takesAnotherIdentity, treeTakesAnotherIdentity, identityWrappers, commandWordAt, toolChangesEnvironment, startupVariable -->
<!-- source: internal/le/hookruntime/readerline.go -- judgeReaderLine, afterIdentityLine -->
<!-- source: internal/le/hookruntime/shellparse.go -- parseCommandLine -->

| Check | Enforces | What it refuses |
|---|---|---|
| `bashAgentIdentity` | `planning.md` | A subagent's Bash command that could start `le` without the agent's own identity. A reader line passes as typed: `commandOutsideReaders` asks `judgeReaderLine` whether the line parses as bash, every node of its tree is one the reader-line walk admits (the node table in `docs/contributing/running-commands.md`), every simple command is in the reader set (`readerCommands`) with the arguments its judge admits and no assignment in front, and no redirection writes a file but `/dev/null` or another descriptor. A command that starts with the agent's exact identity line, `export ZE_AGENT_ID=<agent>; export CLAUDE_CODE_SESSION_ID=<parent>; `, passes when what follows it takes no other identity (`takesAnotherIdentity`). On a line that parses, the after-line walk judges what follows from the tree, statement by statement at every depth. An admitted reader is skipped, and the identity variables it names count as read, so `grep ZE_AGENT_ID f` and `echo $ZE_AGENT_ID` pass; any other command that names either variable is refused. Refused by its form: a declaration (`export`, `declare`, `typeset`, `readonly`, `local`), a function definition, an arithmetic command or expansion, `let`, a coprocess, an assigning default (`${n:=x}`), a `{name}>` redirection, an assignment to an array element or to `BASH_ENV`, `ENV`, `SHELLOPTS`, `BASHOPTS`, `PROMPT_COMMAND` or a `BASH_FUNC_*` name, in front of a command or bare, a command word that holds `=`, and a command word the shell expands. Past the wrappers (`identityWrappers`: `builtin`, `command`, `flock`, `ltrace`, `nice`, `nohup`, `setsid`, `stdbuf`, `strace`, `time`, `timeout`, `xargs`), each with its value-taking options, counts and operands, the command word is matched by its base name, so `/usr/bin/env -i ./le x` and `timeout -s KILL 60 env -i ./le x` are read as `env`. A count or duration holds a digit, so `time . ./env.sh` is read as `.`. A wrapper option that sets the command's environment or user (`strace -E`, `strace -u`, `ltrace -u`) is refused. The command word is refused when it is `unset`, `env`, `exec`, `sudo`, `eval`, `source`, `.`, a declaration, `read`, `mapfile`, `readarray`, `getopts`, `trap`, `enable`, `let`, or `printf -v`; `set` passes only with `-e`, `-u`, `-x` and `-o pipefail`, `errexit`, `nounset` or `xtrace`, in their `-` and `+` forms. A substitution inside a statement that is not a reader is judged as commands of its own. A remainder that does not parse is judged by its text (`textChangesEnvironment`, the same wrapper table): every `set` and every mention of either variable is refused. `FOO=bar ./le x` and `cd x && ./le x` pass. The check catches an agent that forgets or mistypes its identity. It is not a sandbox: it judges the command, not the files the command runs, so a script the agent writes can set either variable and run `./le` (`bash ./s.sh`). The heavy-work check (`bashSubagentHeavy`, below) has the same limit, which the owner accepted when he chose a deny-list for it: a script file, a string handed to a shell, `make`, Docker, or an `./le` action nobody declared heavy still runs from a subagent, and `./le job report` is where such load shows. Refused: any other command, including a line that does not parse as bash or holds a carriage return or a NUL byte (`parseCommandLine`, `internal/le/hookruntime/shellparse.go`: one parse per hook call, and a parser panic reads as a line that does not parse), an identity line naming another agent or session, an identity line the parse does not read as the agent's own exports (`afterIdentityLine`), and every call of an agent whose id or session id cannot name a file. The refusal names the first construct or command outside the reader line and prints the line to type. The check never changes the command, and the main thread is never judged. |

The check replaced a rewrite: the hook used to prepend the identity line and
return the command as changed input, and auto mode then voided the call
(`plan/learned/HOOK-FRICTION.md`, "The Bash hook rewrote every subagent
command, and auto mode voided the call").


## PreToolUse: Bash, a subagent's heavy work (`internal/le/hookruntime/subagent_heavy.go`)

<!-- source: internal/le/hookruntime/subagent_heavy.go -- bashSubagentHeavy, handBackOf, heavyTool, leWords, withoutHandedBackAdvice -->
<!-- source: internal/le/heavytool/heavytool.go -- Name, LintRunsNoAnalysis -->
<!-- source: internal/le/hookruntime/session.go -- fromSubagent -->
<!-- source: internal/le/le/root/heavy.go -- HeavyLine, RegisterHeavy -->
<!-- source: internal/le/le/action/leaction.go -- Action -->

| Check | Enforces | What it refuses |
|---|---|---|
| `bashSubagentHeavy` | `commands.md` | A test, a build, a lint, a formatter or a heavy `./le` action a subagent types. One machine carries every session, and the main thread is the one caller that schedules heavy work on it, one job at a time. The caller is a subagent exactly when the payload carries an `agent_id`, a fork's included (`fromSubagent`, the predicate `bashAgentIdentity` reads too); the hook does not read `CLAUDE_CODE_FORK_SUBAGENT`, a harness setting the main thread's hook process holds too, and the main thread is never judged. A reader line passes, so a `grep` whose pattern names `go test` is a grep. On every other line the check reads each command of the parsed line at any depth, and then the quote-blind text split, so it covers every line `bashRawHeavy` refuses. A command is judged by its program, behind assignments and launchers: `go` with `test`, `build`, `vet`, `run`, `generate`, `install`, `fmt` or `fix`; `golangci-lint`, unless the subcommand runs no analysis; `staticcheck`, `govulncheck`, `gofmt`, `goimports` and `gofumpt` (that list is `heavytool.Name`, in `internal/le/heavytool`, the one definition `bashRawHeavy` reads its cheap linter subcommands from); and `le`, `./le` or `ze le` when `leroot.HeavyLine` reads the line as heavy, which is what each area declared (an admitted, heavy or expensive registration, or an action row with `Heavy`), carriers such as `job run` and `exec` included. A `ZE_ADMIT_RAW="<reason>"` assignment in front lifts nothing. The refusal names what the line starts, tells the agent to put the command under `Owed commands` in its report, and ends with the line as the agent typed it after its identity line, loops, directory changes and filters kept. It names no admitted form, and for such a line `bashRootBuild`, `bashLossyPipe` and `bashRawHeavy` keep their refusal and print nothing. A line the walk fails to read is refused. The check is a deny-list, so what it does not name still runs: the limits are listed under `bashAgentIdentity` above. |


## PreToolUse: Bash and Write/Edit, the editing agent's budget (`internal/le/hookruntime/budget.go`)

<!-- source: internal/le/hookruntime/budget.go -- bashCallBudget, writeCallBudget, agentCallBudgetCheck -->

| Check | Enforces | What it does |
|---|---|---|
| `bashCallBudget` | `context-economy.md` | Counts a `ze-work` agent's Bash calls in `tmp/session/<parent>/agents/<agent_id>.calls` and refuses the call past 100, except one that names the per-spec state file or asks for its path. A call an earlier check refused is not counted: the refusal already costs the agent a turn. `Run` passes that verdict on, so the budget stays the last check of its action. The refusal tells the agent to append its handoff and report that the package needs a continuation. |
| `writeCallBudget` | `context-economy.md` | The same counter over Write, Edit, MultiEdit and NotebookEdit. |

The budget binds the editing agent only: `agent_type` in the hook payload names
it, a read-only agent cut mid-review would lose coverage, and the main thread
carries no `agent_id`. Read and ToolSearch pass unhooked, so the count is of
the calls the hook sees, under a tenth short of the agent's total.

## PostToolUse: Write/Edit (`internal/le/hookruntime/postwrite.go`)

<!-- source: internal/le/hookruntime/postwrite.go -- postFormatGo, postFileSize, postDeferral, postJournal, postRFCHeader, postTestDocs, postFuzz, postVague, postBoundary, postInvalidateDerived -->

| Check | Enforces | What it does |
|---|---|---|
| `postFormatGo` | `quality.md` | Formats the edited Go file, then refuses it while the linter still reports an issue in its package. A lint run that does not complete (past its 60s budget, a golangci-lint failure, no binary) is reported as not run, never read as clean. A subagent's edit is formatted and not linted: the hook starts no linter for it and answers nothing, and the agent lists `./le go lint run scope "<pkg>"` under `Owed commands`. |
| `postFileSize` | Go style guidance, no rule point | Reports a Go file past the 1,000-line advisory. |
| `postDeferral` | heuristic advisory, no rule point | Reports deferral language in a document that is not a deferral. |
| `postJournal` | journal format, no rule point | Reports a journal file that `./le commit create` would refuse. |
| `postRFCHeader` | Go style guidance, no rule point | Reports a file citing RFCs with no `// RFC:` header. |
| `postTestDocs` | Go style guidance, no rule point | Reports a test file with no `VALIDATES:` or `PREVENTS:` comment. |
| `postFuzz` | advisory, no rule point | Reports a wire parser whose package carries no fuzz target. |
| `postVague` | Go style guidance, no rule point | Reports a vague variable name in the edited Go file. |
| `postBoundary` | advisory, no rule point | Reports numeric validation whose sibling test names no boundary. |
| `postInvalidateDerived` | `principles.md` | Removes every derived artifact the written file feeds, so a later reader rebuilds it rather than reading a map of the tree as it stood before the edit. |

## PostToolUse: Bash (`internal/le/hookruntime/postbash.go`)

<!-- source: internal/le/hookruntime/postbash.go -- postBashGofmt -->

| Check | Enforces | What it does |
|---|---|---|
| `postBashGofmt` | `quality.md` | Asks gofmt about every uncommitted Go file changed in the last two minutes, so an edit made from the shell (sed, a heredoc, a scripted rename) is checked as a Write or an Edit is. It blocks only for an unformatted file the command itself names, and lists each one; the files the command does not name are counted and reported, because the window cannot tell this session's write from another session's. The repair line names `gofmt -w <path>` to the main thread. To a subagent, whose formatter `bashSubagentHeavy` refuses, it names an Edit of the file, which the post-edit hook formats, or `gofmt -w <path>` under `Owed commands`. |

## Lifecycle actions (`internal/le/hookruntime/lifecycle.go`)

<!-- source: internal/le/hookruntime/lifecycle.go -- runLifecycleHook -->
These return hook protocol output directly rather than a check verdict, so
`nativeHookActions` does not hold them. `runLifecycleHook` dispatches each one.

| Action | Event | What it does |
|---|---|---|
| `session-start` | SessionStart | Validates the raw session ID, publishes the accepted ID, and prints required context before bounded status reports. It neither deletes files nor renders derived artifacts; `./le session clean` owns proof-based cleanup, and named artifact reads own derivation. |
| `compaction-reminder` | UserPromptSubmit | Detects compaction and reminds the session to read the post-compaction rule. |
| `pre-compact-save` | PreCompact | Saves session state before compaction. |
| `block-premature-stop` | Stop | Refuses the stop while the claimed spec is implemented but not closed. Warns in one line when the claim is `in-progress` or no subagent was spawned. It never reads the last message. |
| `rule-coverage-report` | Stop | Reports which rule points this session's transcript exercised. |
| `session-end-summary` | Stop | Calls `./le session end-summary`. Whole-checkout paths are labeled as shared changes with session attribution unavailable. It preserves handoffs and never releases a spec claim. |
| `session-end-deferrals` | Stop | Prints the open deferral count. Advisory. |
| `subagent-context` | SubagentStart | Validates the parent session ID and emits the parent context. Resolves the claimed filename to its current category path; an unresolved or ambiguous claim emits an explicit warning and repair instruction instead of an invented path. It ends with seven call rules (`subagentCallRules`): whole reads of the gated documents, Edit or Write under `plan/` and `ai/rules/`, independent calls in one message, tests, builds, lint and formatters handed to the main thread through an `Owed commands` list, the changed-code lint owed after a Go edit, a commit only for a change that owes no execution, and the call budget. |
| `mark-lsp-invoked` | PostToolUse `LSP` | Native LSP responses supply formatted navigation results, not observed source bytes and bounds, so they grant no source freshness. Use Read for source evidence; LSP remains available for navigation. |
| `mark-source-read` | PostToolUse `Read` | A whole Read of a gated document records the document-read marker. For source, the native structured result must name the same canonical checkout file and carry its observed text, returned range and total line count. The existing context marker stores only that range's digest and tool identity; current matching returned lines establish no unreturned whole-file version. Missing or unsupported results, changed observed text, unreadable paths and outside-checkout symlinks grant no source evidence. |
| `mark-agent-spawned` | PostToolUse `Task\|Agent` | Writes the session-scoped agent marker. |
| `validate-spec` | PostToolUse `Write\|Edit` | `hookValidateSpec` validates the spec structure and Wiring Test table. A non-skeleton spec changing daemon-runtime paths must name a native functional proof; a helper-only unit test does not satisfy that demand. |
| `bash-timestamp` | PreToolUse `Bash` | `hookBashTimestamp` (`timestamp.go`) prints a `▶ HH:MM:SS` system message before a Bash call, at most once a minute per session (`CLAUDE_BASH_STAMP_QUIET` sets the gap in seconds). It never judges a command and never blocks. |

`writeDesignEvidence` consumes observed source records before a design or spec
write. Existing producer paths come from Current Behavior, Files to Modify and
Files to Create in both the selected file and the proposed edit, not from
test-plan citations. Missing or unknown existing producers refuse; only genuinely
new Files to Create paths, and Current Behavior or Files to Modify paths whose
last commit in git history deleted them (`deletedInHistory`), may be absent. A
path git never held still refuses. A name whose first segment is no top-level
directory (`reactor/x.go`) is matched as a path suffix over tracked files
(`trackedSuffixMatches`): one match must be read, none is skipped, several
refuse. Reads and subjects share normalized,
symlink-resolved checkout identities. Matching returned ranges prove only their
observed text, not unseen source or understanding. A subjectless draft still
owes a usable current source read. `hookStop` reads the agent marker. Read hooks
never block.

Exemptions follow canonical subjects, not alias names: test-named or
unsupported-extension links to production still owe their target's current
read. Genuine canonical test/non-source subjects stay exempt; lookup or
confinement failures establish no exemption.

Strict source-identity resolution is separate from shared planned-write checkout
membership: it does not exempt a planned write through a checkout alias from
the door, mechanisms, WIP or weakening gate.

<!-- source: internal/le/hookruntime/designevidence.go -- recordSourceRead, designSubjects, deletedInHistory, rootAnchored, trackedSuffixMatches, currentSourceReads, designEvidenceProblem -->

<!-- source: internal/le/hookruntime/specproof.go -- daemonProofProblem -->
<!-- source: internal/le/session/summary.go -- snapshotText, snapshotGrammar -->


### Historical categories and current consumers

The old category names are not authority to restore a retired prose-sensitive
gate. These are the current dispositions; a status check is not evidence that
tests ran, and an advisory is not a refusal.

| Historical intent | Current rule and consumer | Required boundary and permitted behavior |
|---|---|---|
| Commit deferral (`commit-gate`) | `ai/rules/completion.md`; fixture producer `postDeferral` | Deferral wording in a document can produce an advisory, never a commit refusal. Separately, `hookStop` blocks Stop for a claimed, implemented-but-unclosed spec. An unclaimed session is permitted to stop regardless of its last message; no phrase classifier decides whether a commit is owed. |
| Test-first (`test-first`) | `ai/rules/testing.md`; fixture producer `testCoverageProblemsSelected`, the test-coverage refusal of `./le commit create`, and the independent test judgment workflow | A commit carrying Go that owes a test, with no `_test.go`, `.ci` or `.et` beside it, is refused unless `no-test` names a truthful reason; generated, vendored, `cmd/`, `register.go`, `embed.go` and `doc.go` files owe none. No edit-time hook judges it: `writeSpecStatus` judges the claimed spec's status, not test-first, and neither check certifies an observed red. |
| Delegation reminder (`delegation`) | `ai/rules/planning.md`; fixture producer `hookStop` and the `mark-agent-spawned` action | A claimed session with no spawned-agent marker gets a one-line warning, not a block. A recorded spawn removes that reason; with no claim, Stop is permitted. The main thread still owes the rule's phase delegation. |
| Subagent context (`subagent-context`) | `ai/rules/planning.md`; fixture producer `hookSubagentContext` | SubagentStart emits the parent's identity and context, resolves its claim to the current category, and names an unresolved claim explicitly. The hook does not judge the prose of a Task brief; the author still owes the canonical briefing rule. |

<!-- source: internal/le/hookruntime/lifecycle.go -- hookStop, hookSubagentContext, runLifecycleHook -->
<!-- source: internal/le/hookruntime/postwrite.go -- postDeferral -->
<!-- source: internal/le/hookruntime/writeedit.go -- writeSpecStatus -->
<!-- source: internal/le/commit/prepare.go -- testCoverageProblemsSelected, CoverageProblems -->
<!-- source: internal/le/ai/hooks/fixtures.go -- fixtureCategories, categoryProbes -->
<!-- source: internal/le/ai/hooks/fixture_commit_gate.go -- commitGates, commitGateVerdict -->
<!-- source: internal/le/hookruntime/one_off_proof_test.go -- TestHistoricalHookIntentHasCurrentConsumer -->

`UserPromptSubmit` stdout reaches the model and its stderr does not.

## Commit-time gates (`internal/le/commit`)

<!-- source: internal/le/commit/prepare.go -- Create -->
These are not Claude hooks. `Create` runs them when `./le commit create`
generates the commit script, which is the only sanctioned commit path. The
helper knows the exact add and remove set, so the gates inspect that instead of
the shared staging area. A refused gate writes no script.

| Gate | Producer | Override keyword |
|---|---|---|
| test-weakening | `testweakened.ProspectiveCommit`, `testweakened.Check`, reading `tmp/commit-weakening-approved-<session>.md` and writing the `Weakening-approved:` trailer | none: only a `ze-test-judge` approval recorded with `./le test weakened approve` admits a weakening |
| rfc-approval | `rfcChangeProblems` (`rfcchange.go`), reading `tmp/commit-rfc-approved-<session>.md` and writing the `RFC-approved:` trailer | `rfc-change-ok` |
| test-coverage | `testCoverageProblems` (`prepare.go`) | `no-test` |
| verify-status | `verificationState` (`verification.go`) | none: it never refuses a commit, because a missing verification is owed before a push |
| independent review | `closureStem`, `CheckReview` (`review.go`) | `review-override` |
| existing-mechanisms | `specMechanismsRefusal` (`specmechanisms.go`): an edited carried spec whose metadata is missing, invalid or ambiguous, or whose valid Status claims a design with no `## Existing Mechanisms` row; unchanged HEAD text and unchanged rename/move content are not judged | none: fix the metadata identified by the canonical error or add the required section row |

<!-- source: internal/le/commit/specmechanisms.go -- specMechanismsRefusal -->
<!-- source: internal/le/spec/status/specstatus.go -- MetadataStatus -->

<!-- source: internal/le/commit/prepare.go -- admissionTrailer -->
<!-- source: internal/le/commit/owed.go -- deriveDebt -->
An override does not discharge the obligation. `admissionTrailer` writes the
reason into the commit as a trailer (`Review-owed:`, `RFC-approval-owed:`),
and nothing else records it. No gate refuses a commit for a red: a red
prevents a release, never a commit (`ai/META-DECISIONS.md`, "Ze is
pre-release"). `deriveDebt` reads both trailers back on every ask and `./le commit push`
refuses while one is open on a commit it would publish
(`docs/contributing/committing.md`, "Pushing").

The review gate runs only for a closure commit, which `closureStem` identifies.
`ROUND_CAP` and `cmd_record` in `internal/le/spec/review.go` price the
review round count at `./le spec review record`, not here.

## Changed-file gates (`./le doc wiring`)

<!-- source: internal/le/doc/wiring/docwiring.go -- checker.run -->
These are native `./le` actions rather than Claude hooks, and Thomas's release check,
`./le verify worktree`, runs them.

| Check | Name | What it refuses |
|---|---|---|
| `checkSleepRatchet` | `ci-sleep-ratchet` | How MANY pauses the `.ci` corpus holds, against a committed delta baseline. |
| `checkSleepJustification` | `ci-sleep-justification` | How many of those pauses are UNEXPLAINED: each needs a comment above or trailing it. |
| `checkLoadExcuses` | `known-failure-load-excuse` | A changed `plan/known-failures/` shard that blames host load. |
| `checkLogSubsystemKeys` | `ci-log-subsystem-key` | A `ze.log.<subsystem>` key whose subsystem carries a hyphen and names no subsystem a Go literal declares, in any spelling. `env.Get` reads a dot, an underscore and a hyphen as one separator, so `ze.log.bgp.adj-rib-in` sets the `bgp.adj.rib.in` logger, while a key that matches no declared name (or leading run of one) sets nothing and the level stays at the WARN default. |
| `checkDesignRefs` | `design-refs` | A `// Design:` reference naming no document. Unconditional in a repository checkout, because closing a spec orphans references in any source file. |
| `checkDocDrift` | `doc-drift` | A commit that changed a symbol a `<!-- source: -->` anchor claims while that page stood still. It maps each diff hunk to its declaration, so an edit elsewhere in the file reports nothing. |

`checkWiring` runs beside them and reports each exported symbol the change adds
that no non-test file under `internal/` or `cmd/` names.

## Tree checks (`./le repo`)

<!-- source: internal/le/repository/repository.go -- Run -->
`./le repo check tree` declares an empty changed set and runs the three
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
| `internal/le/ai/hooks/parity.go` | Golden exit-code regression over the 219 dispatcher fixtures, in four tables, one per registered check group. The bash table is judged by the hook's own `pretool-bash` action over a main-thread payload. |
| `./le ai hooks unit` | The 551 typed behavior fixtures: spec validation, source-read evidence, commit-time gates, delegation, and the registered write and Bash checks. |

Run the focused hook proof with:

```text
./le ai hooks unit
```

The behavioral category probes call their named producers; registered-name and
allow/refusal separation tests detect a renamed producer or a non-discriminating
probe. The source watch manifest also covers the runtime's non-test helper
files and the session-summary implementation reached by the lifecycle hook.
`TestHookSourcesCoverNativeProducers` discovers that population from source
locations and package files, without a fixed file count.
<!-- source: internal/le/ai/hooks/fixture_grounding_test.go -- TestEveryProbeNamesARegisteredCheck, TestEveryProbeSeparatesItsAllowFromItsRefusal -->
<!-- source: internal/le/ai/hooks/hook_population_test.go -- TestHookSourcesCoverNativeProducers -->

A fixture site is a name with the exit and messages it expects. Two families
of sites are also run through the whole `pretool-bash` action with a
subagent's payload (`agent_id` set), which neither the parity judge nor a
category probe sends: the reader rows, and the heavy-work sites of
`bashSubagentHeavy`. Each heavy-work site is held to its exit and its
messages, and the same command typed by the main thread is its control, so a
site goes red when the check stops refusing a subagent or starts judging the
main thread. The post-write fixtures carry no caller: the subagent branch of
`postFormatGo` is proven in `internal/le/hookruntime/postwrite_test.go`, not
here.
<!-- source: internal/le/ai/hooks/subagent_heavy_sites_test.go -- TestSubagentHeavySitesAreJudgedByTheHook -->
<!-- source: internal/le/ai/hooks/reader_row_sites_test.go -- TestLooserBashRowsPassAsASubagentWithoutTheIdentityLine -->
<!-- source: internal/le/hookruntime/postwrite_test.go -- TestSubagentEditFormatsAndSkipsLint -->

A source seal is not a behavioral proof. Before replacing `hookSourcesDigest`,
run the producer-bound category separation and every watched-file
discrimination check, then get an independent review of the changed behavior
and source population. Only then get the expected digest from
`TestEveryStoredDigestMatchesTheTreeItRatchets` over the reviewed source.
Its printed literal is a candidate, not permission to refresh a seal.
`TestEveryWatchedHookSourceDiscriminates` uses an intact computed baseline,
so an already stale expected digest cannot make its mutation checks pass.
<!-- source: internal/le/ai/hooks/fixtures.go -- hookSourceContentDigest, checkHookSources -->
<!-- source: internal/le/ai/hooks/hook_population_test.go -- TestEveryWatchedHookSourceDiscriminates -->
<!-- source: internal/le/ai/hooks/parity_test.go -- TestEveryStoredDigestMatchesTheTreeItRatchets -->

RFC ownership and weakening probes use isolated Git repositories with the
original test committed at HEAD before proposing a whole-file Write. `git init`
alone is insufficient: unreadable committed text must still fail closed.
A refusal probe must report an unauthorized evidence change, not fixture setup
or committed-text read failure. The draft-incubator control commits both
original paths so its outside-draft refusal tests ownership.
<!-- source: internal/le/ai/hooks/fixtures.go -- committedProbeTree, draftIncubatorTree, probeVerdict -->
