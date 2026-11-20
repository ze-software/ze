# Navigating the Code

How to answer a question about this repository without reading a whole file.
Two capabilities cover almost every question: a symbol server for what the code
IS, and the generated indexes for what it is FOR. Neither replaces the other.

The rule that says WHEN you owe these routes is `ai/rules/context-economy.md`.
This page says how each one works.

## Resolving a Go symbol

There are two routes to one server, and they give the same answers.

1. The agent LSP tool, when the harness serves it. Load it with
   `ToolSearch query="select:LSP"`.
2. The `gopls` command line, from any context that has a shell. `./le setup install`
   installs `gopls` and puts it on PATH (`requiredTools`,
   `internal/le/setup/tools.go`).

An empty `ToolSearch` result means this context has no LSP tool. It does not
mean the capability is absent: run `gopls` instead.

### Which operation answers which question

| Question | LSP tool operation | `gopls` command | What comes back |
|----------|--------------------|-----------------|-----------------|
| What is in this file? | `documentSymbol` | `gopls symbols <file>` | every symbol with its line range: the map you would otherwise read the whole file to build |
| What does this one symbol declare or say? | `goToDefinition`, then `hover` | `gopls definition <file>:<line>:<col>` | the declaration and its doc comment, without the file around it |
| Who calls this? | `findReferences` | `gopls references <file>:<line>:<col>` | every call site as file plus line. `grep` on a common name returns the comments and the string literals too |
| Who calls this, and from inside WHICH function? | `callHierarchy` | `gopls call_hierarchy <file>:<line>:<col>` | each caller's range AND the enclosing function that `references` leaves you to work out |
| Where does a name I can spell actually live? | `workspaceSymbol` | `gopls workspace_symbol <name>` | the file holding it, without guessing a directory |
| Does this file compile, and with what errors? | (diagnostics) | `gopls check <file>` | the type errors for that file. Silence and exit 0 mean clean |

### The two-step recipe

`gopls symbols` prints one line per symbol as `Name Kind <line>:<col>-<line>:<col>`.
That `<line>:<col>` is exactly what `definition`, `references` and
`call_hierarchy` take. Find the symbol in step one, ask about it in step two.
Never guess a position.

```
$ gopls symbols internal/component/bgp/config/resolve.go
ResolveBGPTree Function 43:6-43:20
$ gopls definition internal/component/bgp/config/resolve.go:43:6
.../resolve.go:43:6-43:20: defined here as func ResolveBGPTree(tree *config.Tree) (map[string]any, error)
ResolveBGPTree resolves peer-group inheritance and returns the bgp block as map[string]any.
$ gopls references internal/component/bgp/config/resolve.go:43:6
.../loader_create.go:274:28-42
.../peers.go:53:18-32
```

Positions are 1-based, and the column is the start of the identifier rather
than the start of the line. A `func` declaration puts its name at column 6.

### What it costs

A `symbols` map of a large file is an order of magnitude smaller than the file.
Measure it for any file rather than trusting a frozen number, because both
sides grow:

```
gopls symbols <file> | wc -c ; wc -c < <file>
```

Every invocation starts a fresh server and loads the workspace, so budget
seconds rather than milliseconds. A 60-second timeout is generous. Several
independent `gopls` questions belong in one message, as with any independent
calls.

### Why `gopls mcp` is not registered

Headless `gopls mcp` watches every directory under the workspace root and holds
one open file descriptor per file, because fsnotify uses kqueue on macOS. It
honors no directory filter: `skipDir` in
`golang.org/x/tools/gopls/internal/filewatcher/fsnotify_watcher.go` skips only
names starting with `.` or `_`, and `testdata`. Use the LSP tool or the command
line above.

## Which index answers which question

The symbol routes answer what code IS. These answer what it is FOR. Grep an
index; do not read one. `ai/CODE-TO-DOCS.md` and `ai/DOCS-TO-CODE.md` are each
several hundred kilobytes, so reading either whole costs more than the source
it was meant to save.

| Question | Where the answer is | What comes back |
|----------|---------------------|-----------------|
| What is this file for, and which doc governs it? | the file's own `// Design:` header | the design doc, plus the sibling files that own each detail |
| Which docs cover this code? | `grep` the path in `ai/CODE-TO-DOCS.md` | every doc citing it. Both render shapes carry the full path from the checkout root, so `grep internal/component/bgp/reactor/peer.go` answers that file alone |
| Which `.go` files implement this design doc? | `grep` the doc path in `ai/DOCS-TO-CODE.md` | every file whose `// Design:` header cites that doc, one line each |
| What does this package do? | `grep` the package path in `ai/PACKAGE-MAP.md` | one line, taken from the package doc comment |
| How does this subsystem flow, entry to exit? | `ai/digests/<subsystem>.md` | the flow with `file:line`, the load-bearing files, and the invariants |

<!-- source: internal/le/doc/index/codetodocs_report.go -- renderCodeIndex -->

The three indexes are DERIVED, and git tracks none of them. A `Write` or an
`Edit` to a file that feeds one REMOVES it. A Bash command that names its path
REBUILDS it before that command runs. So a grep reads the tree as it now stands,
never the tree as it stood at the last commit.

A rebuild that fails blocks the command, because a grep of a file nobody built
answers "no match" for a tree the author cannot see.
<!-- source: internal/le/hookruntime/postwrite.go -- postInvalidateDerived -->
<!-- source: internal/le/hookruntime/bash.go -- preMaterializeDerived -->

The rebuild and the removal hold one lock per checkout,
`tmp/derived-artifacts.lock`. So another session's edit waits for a rebuild in
flight rather than removing files the rebuild is writing. A reader that waited
for a peer's rebuild of the same index finds it whole and does not build it
again. A lock that cannot be taken blocks the command too, and the refusal
names the fix. Each hook waits for the lock for half its own hook timeout, so
a lock another session still holds at that bound refuses the command with
"Run the command again", and the edit reports the index stale and not removed.
<!-- source: internal/le/derived/lock.go -- LockPath, Artifact.Ensure, Artifact.Remove -->
<!-- source: internal/le/hookruntime/bash.go -- materializeRefusal -->

**Which searches rebuild, and which do not.** The read hook rebuilds when the
command text holds the index's own path, or a directory that contains it with
its trailing slash. `grep -n X ai/PACKAGE-MAP.md` and `grep -rn X ai/` both
rebuild. A search that names neither does not: `rg ResolveBGPTree`,
`grep -rn foo .`, `grep -rn X ai` without the slash, and any `./le` action that
reads an index from inside its own process. After an edit removed the index, one
of those reads a tree without it and answers no match.
<!-- source: internal/le/hookruntime/bash.go -- commandNamesArtifact -->

Name the path when the answer matters. `grep -n X ai/PACKAGE-MAP.md` is the form
that always reads a current index.

**Which writes invalidate, and which do not.** The removal is keyed to the
`Write` and `Edit` TOOLS. A write that reaches the file some other way moves an
input with no hook in the path: `sed -i`, a shell heredoc, `git rebase`, `git
stash pop`, `git checkout`, and `./le repo update`. Each of those leaves
the index PRESENT and stale, and neither the read hook nor a session start
rebuilds one that is present, so it stays stale until the next `Write` or `Edit`
to one of its inputs. Run the writer yourself after a bulk rewrite:
`./le repo package-map update`, `./le doc index update`,
`./le doc index update`.
<!-- source: internal/le/hookruntime/runtime.go -- nativeHookActions -->
<!-- source: internal/le/hookruntime/bash.go -- preMaterializeDerived -->

**What a session start prints, and what it cuts.** `./le ai hooks
session-start` prints the source-reading notice and the rules pointer first,
before any report that reads the tree. `.claude/settings.json` gives the hook
5 s, and the harness drops everything a hook printed when it kills it. So the
reports that follow run under the hook's own 3.5 s budget. A report still
running at that point is cut, and one stderr line names it with every report
after it. The notice is already out, and nothing after the cut prints.
<!-- source: internal/le/hookruntime/lifecycle.go -- hookSessionStart, runSessionStartSteps -->

Session startup does not render derived artifacts, even when absent. Name an
artifact in the command that needs it, so its on-demand hook rebuilds it before
the read. Required startup context is not held behind generator work.
<!-- source: internal/le/hookruntime/lifecycle.go -- sessionStartSteps, hookSessionStart -->

The hook source seal is in `internal/le/ai/hooks/fixtures.go`.
`hookSourcePackages` declares bounded, direct package scopes for the runtime,
the fixture producers and their named policy authorities. Discovery includes
every non-test `.go` file directly in each package, across all platforms. This
is a conservative source-change boundary, not a recursive import closure or a
claim about the current host's build-selected files. A new helper in an existing
scope is discovered automatically; crossing into a new authority package needs
an explicit disposition of that boundary.

The digest reads only the supplied checkout root. It length-frames scope and
checkout-relative file identities, canonical regular-file modes and content,
in sorted order. Missing, empty or unreadable package scopes and unreadable,
nonregular or symlinked sources refuse the read. Symlinked scopes, their
ancestors and an ambiguous root refuse too. A removed member, truncated file
or executable-mode change can instead produce a different digest; comparison
with the unchanged proof must then refuse it. This fingerprint does not parse
Go or require every individual source body to be nonempty.

`TestHookSourcesCoverNativeProducers` compares the discovered identities with
package membership selected independently from producer locations and fixture
mappings. The source-authority regressions compute an unchanged positive
baseline, mutate an isolated root and exercise the actual comparison consumer,
without treating a stale stored seal as proof of discrimination. These checks
establish bounded source coverage and digest sensitivity, not whether the hook
behavior is right; the producer-bound category probes answer that separate
question. Reading a mutable root is not an atomic snapshot of concurrent writes.
<!-- source: internal/le/ai/hooks/fixtures.go -- hookSourcePackages, hookSourceFiles, hookSourceContentDigest, checkHookSourcesAgainst -->
<!-- source: internal/le/ai/hooks/hook_population_test.go -- TestHookSourcesCoverNativeProducers, TestEveryWatchedHookSourceDiscriminates -->
<!-- source: internal/le/ai/hooks/hook_source_authority_regression_test.go -- hookAuthorityPackages, TestHookAuthorityContentInvalidatesComputedProof, TestHookAuthorityPopulationInvalidatesComputedProof, TestHookAuthorityUnavailableFailsClosed, TestHookAuthorityOrderAndExclusions, TestHookAuthorityContentFraming -->
<!-- source: internal/le/ai/hooks/fixture_grounding_test.go -- TestEveryProbeSeparatesItsAllowFromItsRefusal -->


Only Bash is intercepted. The `Read` and the `Grep` TOOL reach no hook. One of
them opens an index the last edit removed, and reports a missing file.

Ask the question from Bash, or write the index yourself. The writers are
`./le repo package-map update` for `ai/PACKAGE-MAP.md`, and
`./le doc index update` and `./le doc index update` for the other
two.
<!-- source: internal/le/hookruntime/bash.go -- preMaterializeDerived -->

Every non-test `.go` file carries its own answer in a `// Design: <doc> -- topic`
header. The header is the file's leading run of comment and blank lines, so a
`// Design:` line counts wherever it sits in that run and not after the
`package` clause. `DesignHeader` (`internal/le/doc/consistency/decisions.go`)
is the one reader of it: `ai/DOCS-TO-CODE.md`, `./le doc consistency`, the spec
citation audit and the edit gate all read a header through it.
<!-- source: internal/le/doc/consistency/decisions.go -- DesignHeader, designTargets -->
<!-- source: internal/le/doc/index/docstocode.go -- designRefs -->

A digest orients; it never proves. `ai/digests/*.md` are hand-maintained
(`ai/digests/README.md`), so open the files a digest names before you state what
code does (`ai/rules/evidence.md`).

## Measuring what a session spends

`./le ai tokens` reads this machine's Claude Code transcript store and
prints per-session and per-agent-type context costs. The store grows with every
session, so re-run it for current ratios instead of copying an absolute figure
into a rule or a document. `./le ai tokens report session <id-prefix>` prints the
per-agent-type table for one session, which is the only comparison that holds:
across sessions the always-on preamble changes size and swamps the difference.
`./le ai tokens report project <checkout>` reads another checkout's sessions: it takes
the checkout's path, and le derives the store's project slug from it, because
every slug begins with a dash and a value never does.

Token counts there are characters divided by 3.6, the approximation
`internal/le/ai/tokens/tokeneconomy.go` uses.

### Taking the report over a window

`from <YYYY-MM-DD>` opens the window at that UTC date, and `days <n>` gives
it a length: `./le ai tokens report from 2026-11-20 days 14` reads the
fortnight from 20 November, and a date alone reads from that date to now.
`days 7` alone reads the last seven whole UTC days, from midnight seven days
ago to midnight today, today excluded: two runs on the same day read the same
window, which a span counted back from the moment of the run would not. With
neither keyword, the report covers the whole store, as it always did.

A call is placed by the earliest timestamp its records carry. A main session
can run for days, so each of its calls and tool calls is kept on its own time.
A subagent is kept or dropped whole, by the time of its first call, because
its figures belong to the whole agent: an agent that starts inside the window
brings every call it made, those after the window's end included, into every
rate and total of the report. A bounded window leaves out a main-thread call
that carries no timestamp, and a subagent whose first call carries none, and
the window line counts each apart. A window that holds no call says so
instead of printing zeros.
<!-- source: internal/le/ai/tokens/criteria.go -- NewWindow, applyWindow -->

### The agent-economy figures

The last block of the report holds the figures the agent-economy work is
measured by, each with one definition. A figure whose source holds nothing in
the window prints `not measured` and the reason, never a zero.

The agent-economy umbrella measured its baseline from the same transcripts:
per assistant message, the `usage` fields summed once per message id, and per
tool call, the call paired with its result and the two timestamps subtracted.
The report does the same. Where the umbrella names a figure without saying
exactly how it was counted, the row says what the report chose, so a
comparison with the baseline knows what it compares.

| Figure | Definition |
|--------|------------|
| First-call context of a subagent | The context of each subagent's first API call, the spawn prompt included: count, median, maximum, and how many are above 35k, the umbrella's AC-1 bound for a phase agent. Then the same per recorded agent type, because AC-1 binds phase agents and an Explore or review agent is another population. A fork is left out, because its first call carries its parent's context. The umbrella's 53k median does not say whether it counted forks, so a comparison with it sets a figure without forks against one that may hold them. An agent with no recorded type is in the overall figure and counted apart |
| First-call context of a main session | The context of each main session's first API call, for the sessions whose first main-thread call falls in the window: count, median and maximum |
| Subagent turns and tool calls | Over every subagent in the window, forks included: the median of its API calls and of its tool calls of any tool, and how many made at least 100 tool calls of any tool. The baseline's "344 of 1,656" counts over all subagents, so this is its figure; it is a different figure from the budget line below |
| Subagent Bash calls with no auto-mode verdict | Of the subagent Bash calls that have a recorded result, those whose result record carries the denial kind `automode-unavailable`, which the harness writes when its auto-mode classifier gave no verdict. The calls with no recorded result are counted apart |
| Failed Bash, Edit and Write calls | Of the calls of Bash, of Edit, of Write, and of Edit and Write together (how AC-3 states its bound), in every thread, those with a recorded result whose result carries `is_error`, whether the tool ran and failed or never ran. A call with no recorded result is counted apart: it is neither a success nor a failure |
| `ze-work` agents at the 100-call budget | Of the `ze-work` agents that are not forks, those with at least 100 calls of Bash, Write, Edit, MultiEdit or NotebookEdit, not counting a call whose result carries the denial kind `permission-rule`. The hook binds by agent type alone, so it also counts a fork of type `ze-work`, which this figure leaves out. The harness writes `permission-rule` for every hook refusal, and the hook's counter counted some of them: it skips a call an earlier check of its own action refused, but it counts the budget's own refusal past 100 (`bumpAgentCalls` runs before the comparison) and a call another pre-tool hook refused. The record cannot tell these apart, so the figure skips them all. Skipping the budget's own refusals moves no agent, because the budget refuses only after the counter passed 100; skipping another hook's refusal can count an agent short of a budget the hook did reach. Every other denial (`automode-unavailable`, `automode-blocked`, `user-rejected`, `cancelled`, `interrupted`) is given after the hooks ran, so the counter counted it. A subagent with no recorded agent type is not judged and is counted apart |
| Main-session context peak | Per session, the largest context any of its main-thread calls in the window carried: the number of sessions, the median, the 90th percentile and the largest peak. The percentile interpolates between the two closest ranks, the rule that makes the median the midpoint of the two middles; the umbrella's "one in ten above" does not say which rule it used |
| Main-thread context per call | The mean context of the main-thread calls in the window, subagent calls excluded |
| Cache-read input in subagents | The cache-read input tokens of every subagent call in the window, and their share of the cache-read input of every call. The total is the `cache-read` line of the totals block |
| Bash commands read | The Bash calls of every thread whose command parses as bash, of all of them. A call with no command, one longer than 64 KiB, or one that does not parse is in none of the four figures below. The bound comes before the parse because the parser has no nesting limit and a stack it exhausts ends the process, which no recover catches. Of the command text the report keeps one word in memory, the file a shell read names, to compare with the thread's next call; it never prints or serializes that word or any other part of the command. A command is named by its first word, so a wrapper (`timeout`, `env`, `xargs`, `bash -c`) hides the job or the read it runs, and `go -C dir test` reads as no job. `time` is a bash keyword, not a wrapper: the command it times is read |
| Shell file reads | The Bash calls whose command runs `grep`, `sed` or `cat` with a file: the last operand, after a pattern or script for `grep` and `sed`. An operand is a word that is neither an option nor an option's value: the value of `grep -A`, `-B`, `-C`, `-m`, `-e`, `-f` and of `sed -e`, `-f` (and their long forms) is skipped, and after `-e` or `-f` one operand is the file. A directory operand (`grep -r x internal/le`) counts as a file, and `sed -i` counts as a read. A repeat is one whose file is the file of the thread's previous tool call, a Read, Edit or Write naming it or a shell read of it. The umbrella does not say which commands it counted, how it found the file, or what "the previous call" was, so these are the report's choices. A file is compared as typed |
| `./le help` lookups | The Bash calls with a command that runs `le help`, under any path to `le`. The umbrella's "about 800" does not say how it counted |
| `./le commit create` calls | The Bash calls with a command that runs `le commit create`, and of them those answered with no error, which are the ones that produced a script: a refused create exits non-zero and writes none. The umbrella's "scripts produced" may have been counted another way, such as distinct script paths, which the report cannot see without keeping result text |
| Foreground test and build runs | The Bash calls with a command that runs `le job run`, `le repo compiles`, `le go`, `le test`, `le rfc check`, `go vet`, `go test` or `go build`, the families the umbrella's table of runs names. A foreground run's wall time is its result record's timestamp less its tool_use record's timestamp, the baseline's method; a run missing either is untimed. A run started in the background is counted apart and not timed, because its result arrives before it ends. A call whose result carries a denial kind never ran, so it is counted apart as denied rather than as a run timed at its refusal. The umbrella states two figures that disagree: its headline is 102 hours in 9,742 runs, while the rows of its table of runs sum to 7,372 runs and 88.9 hours. This figure counts the families of the table, so it compares with the table's sum, not with the headline. The table lists `./le test bgp` where the report counts every `le test` |
| Queue wait of admitted runs | From the checkout's job record, `tmp/.ze-verify-duration.txt` (see [running commands](running-commands.md), "How long a verify takes"): the runs in the window, how many carry their wait and how many were written before the wait was recorded, the seconds they ran, and the total, median and largest wait over the rows that carry one, which is not measured when no row in the window carries one. A row that does not parse is counted as unreadable |
<!-- source: internal/le/ai/tokens/criteria.go -- FirstTurn, NoVerdict, Failures, BudgetWall, MainPeak, QueueWait, readQueueWait -->
<!-- source: internal/le/ai/tokens/baseline.go -- MainFirstTurn, AgentRuns, SubagentCacheRead, ShellFigures, JobTime -->
<!-- source: internal/le/ai/tokens/shell.go -- readShell -->
<!-- source: internal/le/ai/tokens/tokeneconomy.go -- scanUserRecord, recordTime -->

The budget's agent type, its tool list and its 100 are copies of the call
budget's own declarations in `internal/le/hookruntime/budget.go`, which are
unexported there; a change to the budget is a change to these copies, and
`TestTheTokenReportCopiesTheCallBudget` in that package compares them. The job
record is the checkout's: `project <checkout>` reads that checkout's record
with its sessions, and a project directory that is not a git checkout keeps
no record, which the queue wait line says. A run that attached to another session's identical run, or
ran inside a parent's slot, held no slot and wrote no row, so it is in no
queue figure.
