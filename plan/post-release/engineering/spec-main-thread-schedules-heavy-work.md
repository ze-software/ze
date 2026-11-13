# Spec: main-thread-schedules-heavy-work

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Development tooling: it changes which agent may start a test, build or lint and what the main thread can see of the machine. It changes nothing in the shipped binary |
| RFC gaps | - |
| Depends | - |
| Phase | 7/7, closure |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Owner permission for this spec only (Thomas, 2026-11-13): it may be claimed
above the spec WIP cap. This is not a change to the cap.

## Task

The owner's rule says all testing and code execution runs in the main thread,
serialized, and that subagents edit and then hand their commands to the main
thread (`ai/META-DECISIONS.md`, "Delegate to subagents"). The goal behind it,
in Thomas's words: "the goal is for the main agent to be able to monitor the
machine load and decide how to schedule tests and work".

Today neither half holds. No hook refuses a test, build or lint a subagent
starts, the editing agent's definition and several skills tell agents to run
their package tests and commit, and the main thread has no command that shows
who holds the job slots or how loaded the machine is.

Goals:

| # | Goal |
|---|------|
| G-1 | A subagent that types a test, a build, a lint, a formatter or a heavy `./le` action is refused by the Bash hook and told to hand the command to the main thread |
| G-2 | The main thread reads one command and sees the job slots, who holds and who waits for them, and the machine's load, memory and disk |
| G-3 | Every agent definition, skill, rule and page says the same thing: agents edit and hand back owed commands, the main thread runs them and commits the code |

Thomas's decisions, all on 2026-11-13:

| # | Decision |
|---|----------|
| D-1 | The Bash hook refuses heavy work from a subagent by a deny-list: the Go toolchain's executing verbs, the linters and the formatters by program name, the raw-admission escape, and every `./le` action that declares itself heavy. He chose the deny-list over an allow-list, knowing it fails open for a script file, `make`, Docker, or an action nobody declared |
| D-2 | The main thread commits, after it has run the agent's owed commands. On a red it resumes the same agent with the result |
| D-3 | A new read-only `./le job report` shows the slots, the live holders and waiters, the load average against the core count, free memory and free disk. Admission keeps queueing as the backstop between sessions |
| D-4 | `/ze-verify` moves to the main thread. `/ze-implement`, `/ze-close`, `/ze-debug` and `/ze-review` keep their agents, which hand back owed commands for the main thread to run |
| D-5 | After a subagent's Go edit the post-edit hook formats the file (gofmt, goimports) and no longer runs the changed-code lint. The main thread runs that lint as an owed command |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every change respects
  → Constraint: a feature registers itself and is discovered; no central list learns its name. The hook's literal list of heavy areas is such a list and is replaced by declarations.
- [ ] `ai/doors/le-tooling.md` - the hooks are le, and every session runs them
  → Constraint: a hook that refuses wrongly blocks every session at once. Each new refusal carries a pass case beside it.
  → Constraint: a new action in an existing area sits in that area's package and action table; `TestEveryCommandIsFoundAtThePathItsNamePredicts` judges it.
- [ ] `ai/doors/core.md` - `internal/core` is the leaf tier
  → Constraint: `internal/core/hostload` imports neither `internal/component` nor `internal/le`. The memory reading is written there against the standard library and `golang.org/x/sys`, and the product's `readMeminfo` is not imported.
- [ ] `.claude/hooks/README.md` - what a new check owes
  → Constraint: a new check owes a `// ze point:` binding, a README row with source anchors, fixture sites, and a re-seal of the fixture and source digests after discrimination checks and independent review.
  → Constraint: the hook judges the command text and never the files a command runs. A script file is not opened.
- [ ] `docs/contributing/running-commands.md` - job admission, session identity, verb vocabulary
  → Constraint: "Waiters never signal holders, delete their files, or prune their result receipts." The report is read-only on the same terms.
  → Constraint: `report` is the verb whose kind "reads and prints, exit 0 whenever it answered".
- [ ] `docs/contributing/rule-authoring.md` - points, manifests and generated rule files
  → Constraint: a point file is never renamed, because its path is the id a check binds to. A new directive is a new point file added to the manifest. `./le ai rules update render` then `./le ai rules update condensed` regenerate the rule files.
- [ ] `ai/rules/user-facing-errors.md` - refusal shape
  → Constraint: the problem first, then why, then the next step on a line of its own, with nothing after it.
- [ ] `plan/journal/guard-message-teaches-the-violation.md`, `plan/journal/guard-blocks-its-own-authors-repair.md`
  → Constraint: a refusal never names a route the caller is also refused, and never asks the caller for something it cannot do. The subagent refusal names the hand-back only.
- [ ] `plan/journal/gate-verdict-depends-on-the-machine.md` (2026-09-06 row)
  → Constraint: a functional run started at load average 2.48 was killed for memory. Load average alone is never presented as a go signal: the report shows available memory beside it and the rule text says so.
- [ ] `ai/rationale/precommit-verify.md`, `ai/rationale/context-economy.md`
  → Decision: the "land first" and "five-minute cache" arguments gave agents the package test and the commit to save agent spend. The owner's later rule (commit ebe1768139, 2026-11-04) overrides them. Both pages are rewritten to say what replaced them and what it costs.

**Key insights:**
- The owner's rule reached only `ai/META-DECISIONS.md` and one rule point. The agent definition (f3bbf09f11, 5c81db8d03), the land-first point and four skills still say the opposite.
- The hook refuses only raw `go test`, `go vet` and `golangci-lint` today, for every caller alike, and advises `./le job run`. Nothing reads the caller's identity for a heavy verdict.
- The hook's wider classifier names `go lint`, `verify` and four `test` areas by literal. Only three sites register (two admitted, one expensive).
- A waiting job leaves no trace, and a registry entry records no session or agent.
- `internal/core/hostload` already reads load on Linux and darwin but answers zero when it fails.
- `Run` runs every check and prints every refusal, so a subagent's heavy line would carry the hand-back and the admitted-form advice together unless the other guards stay silent for that caller.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/hookruntime/runtime.go` - `Payload` carries `AgentID` and `AgentType`; `nativeHookActions` lists the checks; `Run` runs every check and joins every refusal; the call budget is last
- [ ] `internal/le/hookruntime/bash.go` - `expensiveWords`, `heavyArea`, `expensiveAction` class a line as expensive for the lossy-pipe guard; `heavyArea` names `go lint`, `verify`, and `test functional`, `integration`, `qemu`, `unit` by literal; `bashRawHeavy` refuses raw heavy tools for every caller and advises `./le job run` and `ZE_ADMIT_RAW`
- [ ] `internal/le/hookruntime/rawheavy.go` - `rawHeavyWords` refuses `go test`, `go vet`, `golangci-lint` except its cheap subcommands, reads through the launcher set, and lifts the refusal for a `ZE_ADMIT_RAW` reason
- [ ] `internal/le/hookruntime/session.go` - `bashAgentIdentity` and `commandIdentityPrefix` each decide the caller is a subagent from `AgentID` or the `CLAUDE_CODE_FORK_SUBAGENT` environment variable
- [ ] `internal/le/hookruntime/budget.go` - the call budget keys on `AgentID` and `AgentType`
- [ ] `internal/le/hookruntime/lifecycle.go` - `subagentCallRules` already tells a subagent to hand tests, builds, lint and formatters to the main thread
- [ ] `internal/le/hookruntime/postwrite.go` - `postFormatGo` runs gofmt, goimports and then a changed-code `golangci-lint` after every Go edit, for every caller, outside admission; `lintNotRun` advises `./le go lint run scope`
- [ ] `internal/le/hookruntime/postbash.go` - `postBashGofmt` advises `gofmt -w <path>` to every caller
- [ ] `internal/le/ai/hooks/fixture_catalog.go` - the behavioral fixture sites, with four subagent sites for the identity check
- [ ] `internal/le/le/root/leroot.go` - `RegisterAdmitted` (area, takes a job slot), `RegisterExpensive` (area plus verb, judged by its whole output), `RegisterForwarding` (an area with no action rows); the hook is the only reader of the first two
- [ ] `internal/le/le/action/leaction.go` - `Action` carries `Writes`, `Does`, `WholeTree` and no field saying it starts a tool; `WholeTree` is the precedent for a per-action declaration made where the verb registers
- [ ] `internal/le/job/answer.go`, `internal/le/job/register.go` - the area declares one action, `run`, and one answer shape
- [ ] `internal/le/job/registry.go` - `take` writes the entry (label, pid, process group, tree, key, start time, log, state, command); `scanAndClaim` counts live entries against the slots; `alive` forks one `ps` per entry
- [ ] `internal/le/job/job.go` - `queue` waits by rescanning and writes nothing; slots default to the core count divided by the cores per job
- [ ] `internal/le/job/holders.go` - `Holders` lists entries without judging liveness
- [ ] `internal/core/hostload/` - `Snapshot` answers the one-minute load, the core count and two process counts, and zero on failure
- [ ] `internal/core/diskspace/diskspace.go` - `Free` and `Stat` answer free space or an error
- [ ] `internal/le/go/toolchain/gotoolchain.go` - `totalMemoryGiB` parses `/proc/meminfo` and execs `sysctl` itself, and answers zero on failure

**Behavior to preserve:**
- The main thread's refusals and advice are unchanged: raw heavy tools are refused with the admitted form, `ZE_ADMIT_RAW` still lifts that refusal, and an admitted `./le` action passes.
- A reader line that names a heavy command inside a pattern passes for every caller.
- A subagent still runs readers, `gopls`, git read verbs, spec bookkeeping, scratch commands, help pages, `./le test weakened approve`, `./le rfc approve`, and `./le commit create` with its script for a change that owes no execution.
- Admission's queueing, attach and release are unchanged, and a waiter record never occupies a slot.
- `hostload.Snapshot` keeps its answer for the functional runner.
- The post-edit hook formats every Go edit for every caller.
- `./le job run` keeps its grammar, its exit code and its report.
- The lint memory limit `totalMemoryGiB` feeds keeps its floor on a machine whose memory cannot be read.

**Behavior to change:**
- A subagent's heavy command is refused (D-1).
- A subagent's Go edit is no longer linted by the post-edit hook (D-5).
- The hook's literal list of heavy areas is deleted; each area declares its own heavy actions.
- Admission records a waiter, and stamps each entry and waiter with its session and agent.
- `./le job` gains the `report` action (D-3).
- Agent definitions, skills, rule points, rationale and pages are corrected (D-2, D-4).

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Caller identity in the hook payload | `internal/le/hookruntime/runtime.go` `Payload.AgentID`; `session.go` `bashAgentIdentity` | Tells a subagent from the main thread, derived separately in two functions | Extends: one predicate answers "this call comes from a subagent" and the two existing sites and the new check read it |
| Raw heavy refusal | `internal/le/hookruntime/rawheavy.go` `rawHeavyWords`; `bash.go` `bashRawHeavy` | Refuses raw `go test`, `go vet`, `golangci-lint` for every caller and advises the admitted form | Extends: its launcher reading and tree judgment are reused by the new check. Its advice is not shown to a subagent, which is refused the admitted form too |
| Expensive classification | `internal/le/hookruntime/bash.go` `expensiveWords`, `heavyArea`, `expensiveAction` | Classes a line expensive for the lossy-pipe guard, partly from a literal list of areas | Replaces the literal list in `heavyArea` with the declarations below. The lossy-pipe guard reads the same declarations |
| Admitted and expensive registrations | `internal/le/le/root/leroot.go` `RegisterAdmitted`, `RegisterExpensive` | An area says its run takes a job slot; an action says it is judged by its whole output | Extends: both keep their meaning and both count as heavy. A third declaration covers an action that starts a tool without either |
| Per-action declaration on the action table | `internal/le/le/action/leaction.go` `Action.WholeTree` | An action declares a property where its verb registers, so no list of such verbs exists | Extends: the heavy declaration is a field beside it, read through `leroot.ActionsOf` |
| Reading a typed line as an action | `internal/le/le/root/dispatch.go` `resolve` (unexported), `citation.go` `JudgeCitation`, `leaction.List.AtVerb`, `TrailingWordAsksForHelp` | Dispatch, help and the citation judge read which area and action a line names | Extends: an exported reading answers the area, the action and whether the line asks for help, so the hook judges a line the way dispatch runs it |
| Reader-line exemption | `internal/le/hookruntime/readerline.go` `ctxReaderLine` | A line made only of readers passes the heavy guards | Extends: the new check opens with it |
| Subagent start text | `internal/le/hookruntime/lifecycle.go` `subagentCallRules` | Tells a subagent to hand tests, builds, lint and formatters to the main thread | Extends: it gains the owed-commands format, the commit rule and the lint that is now owed |
| Post-edit formatter and lint | `internal/le/hookruntime/postwrite.go` `postFormatGo` | Formats and lints after every Go edit | Extends: the lint step is skipped when the caller is a subagent |
| Job registry | `internal/le/job/registry.go` `take`, `readEntry`, `scanAndClaim`; `holders.go` `Holders` | Records each admitted job and counts slots | Extends: entries gain session and agent fields, a waiter writes a record of its own, and the report reads both |
| Process-table liveness for holders | `internal/le/scratch/cacheclean.go` (the `Holders` caller) | Matches holder PIDs against one process-table scan | Extends: the report judges liveness the same way, one scan for all entries |
| Host load reader | `internal/core/hostload` `Snapshot` | One-minute load, core count, counts of test processes, zero on failure | Extends: a reading that reports failure, the three load figures and available memory. `Snapshot` keeps its contract |
| Free disk reader | `internal/core/diskspace` `Free`, `Stat` | Free space or an error | Used as is |
| Total memory reader | `internal/le/go/toolchain/gotoolchain.go` `totalMemoryGiB` | Total memory in whole GiB, zero on failure, with its own parser | Replaced by the hostload memory reading, so one reader holds the fact |
| Available memory, product side | `internal/component/host/memory_linux.go` `readMeminfo` | Product inventory, Linux only | Does not apply: core never imports a component (`ai/doors/core.md`), and tooling reads through `internal/core` |
| Rule point for the owner rule | `ai/rules/points/commands/directives/a-subagent-runs-no-command-past-five-minutes.md` | Holds the five-minute rule and, in a second sentence, the main-thread rule | Extends: the main-thread rule becomes a point of its own that the new check binds to, and the five-minute point loses the sentence that contradicts it |
| Job admission across sessions | `docs/contributing/running-commands.md`, "Waiting for another session's job" | One job per slot, peers queue | Does not change it: it stays the backstop between sessions (D-3) |
| Admission by class and resource | `plan/post-release/engineering/spec-test-verdicts-independent-of-load.md` AC-5 | Planned: admission checks memory and disk | Does not apply: that spec makes admission act on the readings. This spec shows them to the main thread and shares the reader it will need |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A Bash tool call: the harness sends the hook a payload with the command text and, for a subagent, an agent id.
- A Write or Edit tool call on a Go file: the post-edit hook receives the same payload shape.
- `./le job run` and every admitted action: the job registry.
- `./le job report`: the main thread's question.

### Transformation Path
1. The hook decodes the payload and asks the caller predicate whether the call comes from a subagent.
2. For a subagent, the new check reads the command the way the raw-heavy guard does (reader line first, then text and tree judgments through launchers), and classes it heavy when its program is a Go executing verb, a linter or a formatter, or when it is an `./le` line whose resolved action or area declared itself heavy.
3. A heavy line is refused with the hand-back message. The raw-heavy, lossy-pipe and build guards add no advice for that caller.
4. Admission writes a waiter record while a job waits and removes it when the job is admitted or gives up. Each entry and waiter record carries the session and agent that asked.
5. The report reads the registry once, scans the process table once, drops dead entries from the counts, reads the machine through hostload and diskspace, and answers one document with rows.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness to hook | The hook payload, `agent_id` on pre-tool calls | Yes for pre-tool Bash and Write (existing tests use it). No for post-tool Write and Edit: assumption A-2 |
| Hook to le registry | `leroot` reads of declarations, in process | Yes: `bash.go` reads `leroot.Admits` and `leroot.Expensive` today |
| Job process to registry files | Entry and waiter files under the jobs directory | Yes for entries |
| le to operating system | `/proc`, `sysctl`, `statfs`, the process table | Partly: load and disk exist, memory is new |

### Changed-Wire Census

**Search evidence:** `git grep` over the tree outside `vendor/` and `plan/` for `ze-jobs`, `JobsDir`, `STARTED=`, `PGID=`, `LABEL=`, `STATE=running`, in `.go`, `.sh` and `.py`. Positive control: the search finds the known writer `take` (`registry.go`) and the known reader path through `readEntry`. It found one more writer nobody listed, `guestJobParentEntry` in `internal/le/test/qemu/guestle.go`, and no shell or Python reader. The pages that tell a person to read the files are `docs/contributing/running-commands.md` and `ai/rationale/git-safety.md`.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Job registry entry file | writer | `internal/le/job/registry.go` `take` | Writes the nine fields under the registry lock | Change: adds a session field and an agent field after the existing ones |
| Job registry entry file | writer | `internal/le/test/qemu/guestle.go` `guestJobParentEntry` | A fixed entry the guest holds so nested stages read a parent | Preserve: it carries only a label, and the reader treats absent session and agent fields as unknown |
| Job registry entry file | reader | `internal/le/job/registry.go` `readEntry` (used by `scanAndClaim`, `shares`, `Holders`) | Parses fields by name | Change: reads the two new fields; an entry without them is valid and reports the owner as unknown |
| Job registry entry file | reader | `internal/le/scratch/cacheclean.go`, through `job.Holders` | Reads PID and label only | Preserve: `Holder` keeps those fields |
| Job registry entry file | reader | A person, told by `docs/contributing/running-commands.md` to read the files | Prose | Change: the page names `./le job report` instead |
| Waiter record (new) | writer | `internal/le/job/job.go` `queue` | New | New file suffix in the jobs directory, never matched by the entry glob |
| Waiter record (new) | reader | The report action in `internal/le/job` | New | New |
| Hook payload | writer | The harness | Unchanged | Preserve: no field added or renamed |
| Hook payload | reader | `internal/le/hookruntime/runtime.go` `Payload` | Unchanged decoding; one more consumer of `AgentID` | Preserve |

### Integration Points
- `nativeHookActions["pretool-bash"]` - the new check is listed before `bashLossyPipe` and `bashRawHeavy`, and the call budget stays last.
- `leaction.Action` - the heavy declaration.
- `leroot` - the area-level declaration for an area with no action rows, the predicate that answers a line, and the exported line reading.
- `job` area action table - the `report` action and its answer shape.
- `subagentCallRules` - the start text.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The hook reads an le line through `leroot.resolveLine` (`internal/le/le/root/heavy.go`), which calls the functions dispatch runs (`withoutLauncherWords`, `splitChain`, `retiredSpelling`, `resolve`, `nestedNamespace`, `asksForUsage`), so the hook and the binary resolve one line one way. The report reads the registry through `readEntry` and `readWaiter`, the readers admission uses, and the machine through `internal/core/hostload` and `internal/core/diskspace` |
| No unintended coupling (components stay isolated) | Yes | `internal/core/hostload` imports the standard library, `golang.org/x/sys/unix` and `internal/core/textbuf` only (`memory_darwin.go`, `hostload.go`). `internal/le/hookruntime` imports `leroot` and no area package: `heavy_declared_test.go` links the areas it judges by blank import, in the test binary alone |
| No duplicated functionality (extends existing, does not recreate) | Yes | `totalMemoryGiB` (`internal/le/go/toolchain/gotoolchain.go`) reads `hostload.MemoryTotal` and its own parser, `sysctl` call and `atoi` are deleted. `lintRunsNoAnalysis` is the one list of the linter's cheap subcommands, read by `rawHeavyWords` and `heavyTool`. `fromSubagent` is the one caller predicate. Closure review found one duplicate and folded it: `Snapshot` counted Go test processes through `sh -c "ps \| grep -c"` beside `GoTestProcesses`; `goTestProcessCount` (`hostload.go`) now reads `GoTestProcesses` |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Development tooling with no wire path. Strings are built through `textbuf.Buffer` |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | `job report` is a row of the `job` area's action table (`area`, `internal/le/job/answer.go`) with its shape registered in `register.go`. Each heavy action declares `Heavy: true` on its own row or calls `leroot.RegisterHeavy` from its own `register.go`; `exec` calls `leroot.RegisterCarrier`. `leaction.Action.Heavy` is a field every area may set, beside `WholeTree`, and names no area |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | Yes | Searched at closure: `git grep -n "heavyArea\|expensiveAction" -- internal/le` returns the guarded set `heavyAreas` in `heavy.go` and dated history comments in `internal/le/ai/hooks/fixtures.go`, and no function: `heavyArea`, `expensiveAction` and `expensiveNameWordsMax` are deleted from `internal/le/hookruntime/bash.go`. `expensiveWords` (`bash.go`) and `heavyTool` (`subagent_heavy.go`) ask `leroot.HeavyLine`, `CarrierLine` and `HeavyCommand`, which derive from the action tables (`ActionsOf`) and the `RegisterAdmitted`, `RegisterHeavy`, `RegisterExpensive` and `RegisterCarrier` sets. What the hook files still name are external programs, which cannot register: `go` and its verbs, `golangci-lint`, `staticcheck`, `govulncheck`, `gofmt`, `goimports`, `gofumpt`. `verifyArea` (`lossypipe.go`) remains, read only to choose the advice of a refusal already made (`verifyCommand`). Help, completion and `le help ai` derive `job report` from the action table. `heavyCensus` (`internal/le/heavy_census_test.go`) is a test's expectation, not a reader the product consults |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A forked subagent is detected by the `CLAUDE_CODE_FORK_SUBAGENT` environment variable of the hook process | `session.go` `bashAgentIdentity` relies on it; `spec-design-needs-owner-approval.md` lists the same assumption as unvalidated | A fork is judged as the main thread and can run heavy work | A probe: a fork runs a refused command and the hook's verdict is read back | broken as written. The fact: a fork's hook payload carries an agent id, and the live probe judged a fork as a subagent with the variable unset. The variable is a harness setting (the harness documents it only as what turns fork mode off when set to 0), so it marks no hook process, and a session that set it would have its main thread read as a subagent. `fromSubagent` (`internal/le/hookruntime/session.go`) still reads it beside the agent id; the journal row is in `plan/journal/gate-verdict-depends-on-the-machine.md` (2026-11-13), owned by `spec-hook-le-data-arguments` |
| A-2 | The post-tool payload of a subagent's Write or Edit carries its agent id | `Payload` decodes it for every hook kind; no post-write test or code reads it | The hook cannot tell whose edit it is, and D-5 cannot be built as decided: the choice goes back to Thomas | A probe: a subagent edits a scratch Go file and the recorded payload is read | confirmed. The harness documents that PostToolUse carries `agent_id` and `agent_type` for a subagent's tool call, and the live probe showed it: a `ze-work` subagent's unformatted Go Write came back formatted with no lint message, although the file held a word the linter flags (`postFormatGo`, `internal/le/hookruntime/postwrite.go`) |
| A-3 | An agent of another kind (a workflow or teammate agent) delivers an agent id like a subagent | Not documented in the repository | Such an agent is judged as the main thread | The same probe, for each agent kind this harness offers | partly confirmed. A teammate carries an agent id, per the harness documentation. A workflow agent is not stated there and was not probed. If it carries none it is judged as the main thread, so its heavy line passes the hand-back and meets the raw-job guard's advice instead: the fail-open the deny-list accepts (D-1), listed under Known Limitations |
| A-4 | No shell or Python program reads the registry entry format | The census search above | A reader breaks on the new fields | The census search, rerun at closure | confirmed at closure: `git grep -lE "ze-jobs\|JobsDir\|STARTED=\|SESSION=\|AGENT="` over `*.sh` and `*.py` outside `vendor/` and `plan/` returns no file. The Go hits are `internal/le/job`, its tests, and two compiled fixtures. The QEMU guest's `printf` writes the label line only (`guestJobParentEntry`, `internal/le/test/qemu/guestle.go`) |
| A-5 | Available memory on darwin can be read from the system's page statistics without a new module | `go.mod` vendors `golang.org/x/sys`; hostload already execs `sysctl` on darwin | The darwin reading answers unavailable, which the report states | The darwin unit test on the owner's Mac | confirmed. `readMemoryAvailable` (`internal/core/hostload/memory_darwin.go`) runs `vm_stat`, and `parseVMStatAvailable` (`parse.go`) answers the "Pages free", "Pages speculative" and "Pages inactive" counts multiplied by the page size the header states. It is an upper estimate: a dirty inactive page is compressed or written out before reuse, and purgeable pages are left out. Total memory is `unix.SysctlUint64("hw.memsize")`. The report on the owner's Mac read 18.6 GiB of 64.0 GiB |
| A-6 | Every census action marked "verdict owed" can be judged by reading its code | The census agent did not read them whole | An undeclared heavy action stays runnable by a subagent, which D-1 accepts | The implementer records a verdict per action in this spec | confirmed. No census cell says "verdict owed": each names its verdict and the producing function, and `TestCensusActionsDeclareHeavy` (`internal/le/heavy_census_test.go`) reads every census area's registered table |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The refusal over-matches and blocks a subagent's legitimate command: a listing, a help page, a reader that quotes a heavy command, a commit body that names `go test` | A subagent reports a refusal for a command that runs nothing | The check reads the resolved action and the help question, opens with the reader-line exemption, and uses the tree judgment for command words. Each shape is a pass row in the tests and fixtures |
| R-2 | An agent waits more than five minutes for its owed commands and is resumed on an expired prompt cache, rewriting its whole context | Agent spend per phase rises | The rule text tells the main thread to resume the same agent when its tests came back fast and to hand a red to a fresh fix agent with the failure when the wait was long. Recorded in `ai/rationale/context-economy.md` |
| R-3 | Every test summary lands in the main thread, the largest context | Main-thread context grows faster per spec | Quiet runs answer a verdict, a log path and failure lines only. The existing handoff rule at 600k stays |
| R-4 | A subagent runs heavy work through a route the deny-list does not see: a script file, a string handed to a shell, `make`, Docker, an undeclared action | The report shows load with no holder | Accepted by D-1. Recorded under Known Limitations. The report's process counts and load make it visible |
| R-5 | The report reads a failed load reading as an idle machine | A zero load on a busy machine | A reading that failed is reported as unavailable with its reason, never as zero |
| R-6 | A waiter record outlives its waiter and shows a queue that is not there | The report lists a waiter whose process is gone | The report judges each waiter's liveness against the process table and omits the dead |
| R-7 | The new waiter write races the registry lock or changes who is admitted | A contention test changes its verdict | The waiter record is never read by `scanAndClaim`, whose glob matches entries only. The existing contention tests run unchanged |
| R-8 | Another spec edits the same files: `agent-economy-3` (`ai/agents/ze-work.md`, `lifecycle.go`), `agent-economy-4` (skill bodies), `hook-le-data-arguments` (`bash.go`, `rawheavy.go`), `design-needs-owner-approval` (`runtime.go`, `bash.go`), `test-verdicts-independent-of-load` (`internal/le/job`) | A commit conflict | All are skeleton or design. This spec lands first; each is named under Blast Radius |
| R-9 | The hook fixture and source digests are re-sealed without the discrimination checks | A fixture passes that could not go red | `.claude/hooks/README.md`, "Hook tests", order is followed: discrimination, independent review, then the digests |
| R-10 | An area declares an action heavy that an agent needs for its own bookkeeping | A subagent cannot do what its skill tells it | The census marks the light actions of each mixed area. The pass rows name `./le test weakened approve`, `./le rfc approve`, `./le spec` commands and `./le commit create` |
| R-11 | Other sessions run an le built before this spec while this one runs a later build | A waiter of an older le is invisible; an entry has no owner | Both read as unknown or absent, never as an error. The report says what it could not see |
| R-12 | An older guard that returned nil for a handed-back line would change which guard the recorded verdict table says refused it | `TestGuardVerdictsOfHeadStand` (`internal/le/hookruntime/verdicts_test.go`) loses a subagent row's refusal | Found in implementation: the first build of AC-9 did exactly that and lost 158 rows. The three guards keep their verdict and drop only their text (`withoutHandedBackAdvice`, `internal/le/hookruntime/subagent_heavy.go`) |
| R-13 | A guard's own text reaches a subagent on a line the heavy-work check did not hand back, and names the admitted form or the escape | A subagent types `./le job run ...` after reading a refusal | Each guard builds a second text for that caller (`unhanded`), which names only the command with no filter or the hand-back. `TestSubagentIsNeverAdvisedTheAdmittedForm` types the advised command back as the same caller |
| R-14 | The quote-blind text judgment reads a heavy tool's name after a separator inside a quoted argument (a commit body holding `; gofmt x`) and hands the line back | A subagent's prose commit is refused as a formatter run | Kept on purpose so the check covers every line the raw-job guard refuses for every caller. The class is `plan/journal/hook-existing-patterns-false-positive.md`, owned by `spec-hook-le-data-arguments`. The agent rewords the text |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing in the shipped binary. A wrong refusal blocks every subagent in every session until fixed; a wrong pass leaves today's behavior |
| How is it reverted? | A commit revert per phase. The registry fields and the waiter record are ignored by an older le |
| Who else touches this path? | `spec-agent-economy-3-preamble`, `spec-agent-economy-4-supervision`, `spec-agent-economy-5-scoped-gates` (its `./le repo compiles check` citation moves to the main thread), `spec-hook-le-data-arguments`, `spec-native-action-job-admission` (its AC-1 and AC-3 use the same heavy criterion and can derive from this spec's declaration), `spec-test-verdicts-independent-of-load`, `plan/release/validation/spec-design-needs-owner-approval` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A subagent's Bash payload carrying a heavy command, through the `pretool-bash` action | → | the new subagent heavy check in `internal/le/hookruntime` | `TestSubagentHeavyCommandIsHandedBack` |
| A subagent's Bash payload carrying `./le test unit pkg path <dir>` | → | the area's heavy declaration, read through `leroot` | `TestSubagentHeavyActionIsReadFromItsDeclaration` |
| A subagent's Edit payload on a Go file, through the `posttool-writeedit` action | → | `postFormatGo` | `TestSubagentEditFormatsAndSkipsLint` |
| `le job report` typed at the launcher | → | the report action in `internal/le/job` | `test/ui/le-job-report.ci` |
| A job that waits for a slot | → | the waiter record written by `queue` | `TestWaitingJobIsListedByTheReport` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A subagent types `go test`, `go build`, `go vet`, `go run`, `go generate` or `go install`, alone, behind a launcher, or inside a loop | The hook refuses with exit 2. The message says a subagent does not run this, that the main thread schedules it on the shared machine, and ends with the instruction to list the exact command under owed commands in its report, with the command on the last line | `go version`, `go env`, `go list`, `go doc` and `go help` pass: they read and start no build of the tree |
| AC-2 | A subagent types a linter (`golangci-lint`, `staticcheck`, `govulncheck`) or a formatter (`gofmt`, `goimports`, `gofumpt`) | Refused as AC-1 | The linter's cheap subcommands the raw-heavy guard already exempts (`config`, `version`, `help`, `linters`, `cache`, `completion`) pass, because they run no analysis |
| AC-3 | A subagent prefixes a heavy command with a `ZE_ADMIT_RAW` reason | Refused as AC-1, with a message that names the hand-back as the only next step | The main thread's use of the escape is unchanged, because the escape exists for the caller who schedules |
| AC-4 | A subagent types an `./le` action that declared itself heavy, `./le job run`, or `./le exec` | Refused as AC-1 | The same area's light actions pass, read per action: `./le verify status check`, `./le test functional list`, `./le rfc approve`, `./le test weakened approve`, `./le repo check`, and `./le job report` |
| AC-5 | A subagent types a bare area name that prints a listing, a line ending in a help word, or `./le help <area>` | The command passes and prints its listing or help | All of them pass: a listing starts no tool. An area with no action rows that declared itself heavy is refused on every line but a help request, because its handler runs on its bare name |
| AC-6 | A subagent types a reader line that names a heavy command in a pattern or quoted text, or `./le commit create` with a body naming `go test` | The command passes | Every reader line passes for every caller, as today |
| AC-7 | The main thread types any command of AC-1 to AC-4 | The verdict and message are byte-identical to today's: raw heavy tools are refused with the admitted form, admitted actions pass | All of today's main-thread behavior survives, because the main thread is the scheduler |
| AC-8 | A forked subagent types a heavy command | Refused as AC-1 | The fork's readers and light actions pass as a subagent's do, because a fork is a subagent |
| AC-9 | A subagent is refused a heavy command | The hook prints exactly one refusal for that line, the hand-back, and its text holds the command to list and neither the `./le job run` form, a `Use:` line nor build advice | Refusals of other kinds (a destructive git verb, a system temp path) still print beside it, because they are other defects of the same line |
| AC-10 | An `./le` area or action that starts a build, test, lint, formatter over more than one file, Docker, QEMU, a fuzzer, a benchmark or another heavy action | It declares itself heavy where its verb registers, and the hook's literal list of heavy areas is gone. The census table in this spec has a verdict for every row | An action that reads files, queries git or writes a ledger declares nothing and stays light |
| AC-11 | The lossy-pipe guard judges a line for any caller | It classes as expensive every line it classes as expensive today, plus every action that newly declared itself heavy | `./le verify reds` and `./le verify list` stop being classed expensive, because they only read the certificate |
| AC-12 | A subagent edits a Go file with Write or Edit | The file is formatted by gofmt and goimports, and the hook answers without starting a linter process | The main thread's edit is still formatted and linted as today |
| AC-13 | A job is admitted while `CLAUDE_CODE_SESSION_ID` and `ZE_AGENT_ID` name its caller | Its registry entry records that session and that agent | An entry written by an older le, or by the QEMU guest's fixed parent entry, has neither and is read as owner unknown |
| AC-14 | A job asks for a slot and all are taken | It writes a waiter record carrying its label, process, start of the wait, session, agent and command, and removes it when it is admitted, attaches, or is cancelled | The record is outside the slot count, so admission's decisions are the ones it makes today |
| AC-15 | The main thread runs `./le job report` | It prints, and renders through `json`, `yaml` and `table`: the slot total, occupied and free counts; one row per live holder with label, process, age, session, agent and command; one row per live waiter with label, process, waited time, session and agent; the core count; the one, five and fifteen minute load averages; available and total memory; free disk on the volumes holding the checkout's `tmp` and the Go build cache; the machine-wide counts of running test processes; and a line stating that the slots count only admitted jobs | Entries and waiter records whose process is gone are omitted from rows and counts, because they hold nothing |
| AC-16 | A machine reading fails or the platform cannot provide it | The report shows that reading as unavailable with the reason, and every other reading is still shown. Exit code 0 | A zero is shown only when the reading succeeded and was zero |
| AC-17 | The report runs while other sessions hold and wait for slots | It answers the document of AC-15, and afterwards every registry file is byte-identical and every listed process is still running | Every file and process survives untouched: the report is read-only, on the terms waiters already keep |
| AC-18 | A subagent starts | Its start text tells it to hand tests, builds, lint and formatters to the main thread as owed commands in its report, that the changed-code lint is one of them, and that it commits only a change that owes no execution | The identity line, the budget and the other start rules are unchanged |
| AC-19 | An agent or a main thread reads `ai/agents/ze-work.md`, `ai/agents/ze-read.md`, the skills, the rule points, the rationale pages and the contributing pages named in Files to Modify | Each states that subagents edit and hand back owed commands, that the main thread runs them one CPU-intensive job at a time after reading `./le job report`, and that the main thread commits code. `/ze-verify` declares that it runs in the main thread | A subagent's commit of prose, spec bookkeeping or a blocked hand-back stays, because such a change owes no execution |
| AC-20 | The main thread decides when to start a heavy job | The rule text gives it the procedure: read the report; start when a slot is free and memory is available; when slots are full or a waiter is listed, send agents work that needs no run instead of queueing; never read a low load average alone as permission | Admission still queues a job the main thread starts anyway, because it is the backstop between sessions |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSubagentHeavyCommandIsHandedBack` | `internal/le/hookruntime/subagent_heavy_test.go` | AC-1, AC-2: each tool form, launcher and loop is refused for a subagent | |
| `TestSubagentHeavyRefusalEndsWithTheCommandToHandBack` | same | AC-1, AC-9: message shape, last line, and absence of the admitted form and the escape | |
| `TestSubagentAdmitRawReasonDoesNotLiftTheRefusal` | same | AC-3 | |
| `TestSubagentHeavyActionIsReadFromItsDeclaration` | same | AC-4: declared actions, `job run`, `exec` | |
| `TestSubagentLightCommandsPass` | same | AC-4 survivors, AC-5, AC-6 | |
| `TestForkHeavyCommandIsHandedBack` | same | AC-8 | |
| `TestMainThreadHeavyVerdictsAreUnchanged` | same | AC-7, over the recorded verdict table's main-thread rows | |
| `TestSubagentHeavyLineCarriesOneRefusal` | same | AC-9 | |
| `TestCallerPredicateAgreesWithIdentityCheck` | `internal/le/hookruntime/session_identity_test.go` | The one predicate answers what `bashAgentIdentity` decided before | |
| `TestHeavyLineReadsActionAreaAndHelp` | `internal/le/le/root/heavy_test.go` | The line reading: action row, area-level declaration, listing, help word, admitted and expensive registrations | |
| `TestCensusActionsDeclareHeavy` | `internal/le/heavy_census_test.go` | AC-10: every census row answers heavy, and each named light action of a mixed area answers light | |
| `TestLossyPipeReadsTheDeclarations` | `internal/le/hookruntime/bash_test.go` | AC-11 | |
| `TestSubagentEditFormatsAndSkipsLint` | `internal/le/hookruntime/postwrite_test.go` | AC-12 | |
| `TestMainThreadEditStillLints` | same | AC-12 survivor | |
| `TestSubagentUnformattedFileAdviceNamesNoFormatterRun` | `internal/le/hookruntime/postbash_test.go` | The post-Bash advice for a subagent names the hand-back | |
| `TestEntryRecordsSessionAndAgent` | `internal/le/job/registry_owner_test.go` | AC-13 | |
| `TestEntryWithoutOwnerFieldsReadsAsUnknown` | same | AC-13 survivor | |
| `TestWaitingJobWritesAndRemovesItsRecord` | `internal/le/job/waiter_test.go` | AC-14 | |
| `TestWaiterRecordNeverOccupiesASlot` | same | AC-14 survivor | |
| `TestWaitingJobIsListedByTheReport` | `internal/le/job/overview_test.go` | AC-15 waiters | |
| `TestReportListsLiveHoldersAndOmitsDeadEntries` | same | AC-15 | |
| `TestReportShowsAnUnavailableReadingAsUnavailable` | same | AC-16 | |
| `TestReportWritesNothing` | same | AC-17 | |
| `TestJobAreaDeclaresRunAndReport` | `internal/le/job/answer_test.go` | Replaces the test that asserts the area has one action | |
| `TestReadingReportsFailure` | `internal/core/hostload/hostload_test.go` | AC-16 at the reader | |
| `TestAvailableMemoryIsParsed` | `internal/core/hostload/memory_linux_test.go`, `memory_darwin_test.go` | Memory reading from recorded samples of each platform | |
| `TestSubagentStartTextNamesOwedCommands` | `internal/le/hookruntime/lifecycle_test.go` | AC-18 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| None | The report takes no numeric input | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-job-report` | `test/ui/le-job-report.ci`, fixture `internal/test/fixture/ui_fixture_le_job_report.go` | In a scratch checkout root, one job holds the only slot and a second waits. `le job report` lists the holder and the waiter with their sessions, shows one slot occupied and none free, and exits 0 | |
| Hook fixture sites for the subagent heavy check | `internal/le/ai/hooks/fixture_catalog.go`, `fixtures.go` | `./le ai hooks unit` judges a subagent heavy refusal and a subagent light pass through the real dispatcher | |

## Files to Modify
- `internal/le/le/action/leaction.go` - the heavy declaration on `Action`
- `internal/le/le/root/leroot.go`, `dispatch.go` - the area-level declaration, the heavy predicate over a typed line, the exported line reading
- `internal/le/hookruntime/runtime.go` - register the check
- `internal/le/hookruntime/bash.go`, `rawheavy.go` - the literal list removed, the shared reading, no admitted-form advice for a subagent
- `internal/le/hookruntime/session.go` - the caller predicate
- `internal/le/hookruntime/postwrite.go`, `postbash.go` - D-5 and the advice
- `internal/le/hookruntime/lifecycle.go` - the start text
- `internal/le/ai/hooks/fixture_catalog.go` - sites, a subagent probe, counts and digests, with the sibling fixture and parity files of that package
- Every area in the census table - its declaration, in its action table or `register.go`
- `internal/le/job/answer.go`, `register.go`, `registry.go`, `job.go` - the action, its shape, the owner fields, the waiter record
- `internal/core/hostload/` - the reading that reports failure, load figures, memory
- `internal/le/go/toolchain/gotoolchain.go` - `totalMemoryGiB` reads through hostload
- `.claude/hooks/README.md` - the check's row, the identity section, the fixture counts
- `docs/contributing/running-commands.md` - "Waiting for another session's job", "Why one owner runs the suites", "Session identity, and what each agent owns", "Testing one package while you develop it", "The Bash guard matches your command text"
- `docs/contributing/testing.md`, `docs/contributing/rfc-conformance-gates.md`, `docs/contributing/claude-code-cheatsheet.md`, `docs/contributing/spec-workflow.md` - who runs what
- `docs/architecture/testing/runner-architecture.md` - "Contended run verdicts": the two kinds of reader `internal/core/hostload` holds. It is the design page the new hostload files name in their `// Design:` header
- `ai/doors/le-tooling.md`, `ai/doors/core.md` - the boundary and gate rows name the caller
- `ai/agents/ze-work.md`, `ai/agents/ze-read.md` - no package test, no landing of code, the owed-commands report
- `ai/skills/ze-verify.md` - main thread
- `ai/skills/ze-implement.md`, `ze-close.md`, `ze-debug.md`, `ze-review.md`, `ze-review-deep.md`, `ze-hunt.md`, `ze-check.md`, `ze-commit-check.md`, `ze-mutation-fix.md`, `ze-test.md`, `ze-rfc.md`, `ze-rfc-audit.md`, `ze-fix-alloc.md`, `ze-extract.md`, `ze-find-alloc.md`, `ze-test-judge.md` - executing steps are the main thread's
- `ai/rules/points/commands/` - the five-minute point loses its contradicting sentence; the heavy-jobs point limits the escape to the main thread; manifest
- `ai/rules/points/precommit-verify/directives/land-first-never-a-private-export.md`, `no-agent-runs-verify-and-tests-run-in-the-main-checkout.md`
- `ai/rules/points/completion/directives/verification-debt-is-not-defect-debt.md`
- `ai/rules/points/planning/directives/the-main-thread-supervises-and-verifies-what-comes-back.md`, `planning/work-phases/rfc-proof-records-ship-with-the-code.md`, `planning/spec-metadata-blocking/a-closure-ends-closed-or-handed-back.md`
- `ai/rules/points/quality/linting/run-the-focused-check-and-paste-its-output.md`, `ai/rules/points/pre-release/directives/fix-known-reds-by-targeted-reruns-and-verify-once.md`, `ai/rules/points/interop-and-goal-validation/directives/revert-the-change-and-confirm-the-test-goes-red.md`, `ai/rules/points/rfc-compliance/directives/prove-a-new-tag-discriminates-its-claim.md` - name the main thread as the runner
- The rendered rule files under `ai/rules/` - regenerated, never edited
- `ai/INSTRUCTIONS.md`, `ai/META-DECISIONS.md` ("An agent's evidence is the scoped test runs ... run directly": name the main thread as the runner)
- `ai/rationale/precommit-verify.md`, `ai/rationale/context-economy.md`
- `ai/INDEX.md` - the `./le job` row
- `plan/templates/TEMPLATE-CLOSURE.md` - the row that tells a closure agent to run `go test -run`

## Files to Create
- `internal/le/hookruntime/subagent_heavy.go`, `subagent_heavy_test.go` - the check
- `internal/le/le/root/heavy.go`, `heavy_test.go` - the declaration registry and predicate
- `internal/le/heavy_census_test.go` - the census proof
- `internal/le/job/overview.go`, `overview_test.go`, `waiter.go`, `waiter_test.go`, `registry_owner_test.go` - the report and the waiter record
- `internal/core/hostload/memory_linux.go`, `memory_darwin.go`, `memory_other.go` and their tests
- `test/ui/le-job-report.ci`, `internal/test/fixture/ui_fixture_le_job_report.go`, `internal/test/fixture/register_ui_le_job_report.go`
- `ai/rules/points/commands/directives/heavy-work-starts-in-the-main-thread.md` - the point the check binds to, holding the owner rule and the scheduling procedure of AC-20

### Census of heavy actions (AC-10)

Each row declares itself heavy. "Verdict owed" rows are read by the implementer, who replaces the cell with heavy or light and the reason.

| Area | Heavy actions | Light actions of the same area |
|------|---------------|--------------------------------|
| `verify` | `current`, `worktree`, `changed` | `reds`, `list`, and the bare name, which prints the listing: the table holds no default action. `verify status` and `verify summary` are areas of their own and declare nothing |
| `verify deps` | `evidence-vet`, `vulnerability`, `unit-cached`, `unit-changed`, `unit-race-changed`, `alloc` | - |
| `verify evidence` | `release-candidate` | - |
| `go lint` | `run` | - |
| `go staticcheck` | `check` | `rows` |
| `go vet-platforms` | `darwin`, `freebsd`, `openbsd` | - |
| `go module` | `move`, `rename` | - |
| `go stdpatch` | `goroot` | - |
| `test unit` | every group and `pkg` | bare listing |
| `test functional` | `gating` and every suite name | `list`, `preview`, bare listing |
| the 27 suite areas and `test stress-repro` | already registered as admitted | - |
| `test integration` | the area: every action, `clean` included, because each drives the Docker daemon every session shares (`leroot.RegisterHeavy(Area)`, `internal/le/test/integration/register.go`) | bare listing |
| `test qemu` | every action | bare listing |
| `test deployment` | every action | bare listing |
| `test fuzz` | `run` | `list` |
| `test attribution` | `report` | - |
| `test redwalk` | `check` | - |
| `test netlab` | `run`, and `check` and `update`: each starts `netlab create` for the rendered lab and `ze config validate` for every golden (`runNetlabCreate`, `internal/le/test/netlab/netlab.go`), a golden test run | `package`, which copies files |
| `rfc` | `check`, `record`, `discriminate` | `approve`, `tagged-scope`, `reseal`, `audit-stamp`, `evidence-split`, `move`, `tree`, `next`, `update`, `quote-backfill`, `extraction`, `skeletons`, and `selftest`: it judges in-process fixtures and starts only `git` in a throwaway repository (`selftestGit`, `internal/le/rfc/selftest_core.go`) |
| `repo compiles` | `check`, and `selftest`: each case calls `Build`, which starts `go build` (`selftestCases`, `internal/le/repo/compiles/selftest.go`) | `matrix` |
| `repo bootstraps` | `check` | - |
| `repo` | `check generated`: it runs `rfc check`, a heavy action, among the generated-artifact checks (`generationChecks`, `internal/le/repo/generate.go`). The selection is a value of the `check` action, so the area registers it (`leroot.RegisterHeavy`) | `check`, `check tree`, `update` |
| `build` | `host-driver run`, `installer`, `preview run`, `gokrazy` | `gosum check`, `preview check` |
| `chaos selftest` | `lint`, `unit`, `cli-unit`, `all` | - |
| `chaos run` | the area | - |
| `perf` | `run`, `filter`, `send` | `suggest`, `record`, `compare`, `track` |
| `doc yang-contract` | `doc-drift` | - |
| `feature` | `run` | `report` |
| `site` | `build`, `config-tree`, `talk` | `check`, `facts`, `history`, `wiki`, `release`, `bundle`, `activity` |
| `site terminal-demo` | `build`, `render`, `pty` | - |
| `setup` | `install`, `tools`, `proto`, `claude-server`, `web`, `appliance-modules`, `docker host build`, `docker host start`, `docker host stop`, `docker kernel install`, `docker kernel apparmor` | `check` |
| `weekly` | `report` | `publish`, the default action |
| `job` | `run` | `report` |
| `exec` | the area | - |
| `ai hooks` | `unit`, already registered as expensive | - |
| `repo codeql` | `check`: it starts the CodeQL executable against a database (`runExtractionCodeQL`, `internal/le/repo/codeql/extraction_check.go`) | `packages`, `prepare`: each starts `go list` and `git` only |
| `test fail-syscall` | the area: it executes the command typed after `--`, whatever it is (`Run`, `internal/test/failsyscall/failsyscall_linux.go`), as `exec` does | - |
| `test scale` | the area: `l2tp` starts a mock RADIUS server and loads a running LNS with 10 tunnels of 200 sessions by default (`CmdL2tpScale`, `internal/test/cli/cmd_l2tp_scale.go`) | - |
| `test local-data-coverage` | the area: it starts the `ze` binary once for every local data command (`run`, `internal/test/localdatacoverage/localdatacoverage.go`) | - |
| `doc wiring` | - | `check` (the default action) and the templ orphan report: linked Go checkers that read files and query `git` (`goActions`, `internal/le/doc/wiring/delegate.go`) |
| `test interop-bgp` | - | the area: it runs one compiled personality a scenario starts, and starts nothing itself (`Helper`, `internal/le/interoplab/bgp/helper.go`) |
| `mrt` | - | the area: the analyzer reads MRT files in process and starts no program (`Answer`, `internal/le/mrt/mrt.go`) |

`repo changed` and `scratch clean`, `scratch trim` stay light by decision: the first lists packages in seconds and tells an agent what it owes, the second frees disk and runs no work.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `le` is development tooling and has no YANG surface |
| YANG validation constraints | N-A | Same |
| YANG custom validators | N-A | Same |
| CLI commands/flags | Yes | `internal/le/job/answer.go`: the `report` action in the area's table |
| CLI grammar (keyword before value) | Yes | The action takes no parameter; `./le cli grammar` judges the area |
| Editor autocomplete | N-A | No YANG leaf. `le` completion derives from the action table |
| Functional test for new RPC/API | Yes | `test/ui/le-job-report.ci` |
| Pipe completeness | Yes | The answer is a struct rendered by the shared renderer, shape registered in `internal/le/job/register.go` |
| Env var registration | No | The report reads `ze.run.slots` through the existing entry and adds none |
| Doctor check for runtime dependencies | N-A | `ze doctor` reports the product. The report reads `/proc`, `sysctl` and the process table on the development machine and states an unavailable reading itself |
| Doctor code per problem class | N-A | The refusals are hook verdicts to an agent, not operator errors of the product |
| Prometheus counters/metrics | N-A | No product state |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | `features/` lists product features; `le` tooling has no entry there |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | Yes | `docs/contributing/running-commands.md` holds le's commands; `docs/guide/command-reference.md` does not list `le job` |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | No | Contributor pages only |
| 7 | Wire format changed? | No | The registry entry is a local file, documented in `docs/contributing/running-commands.md` |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/testing.md`: who runs a test. `docs/functional-tests.md` is unaffected: no suite, runner or format changes |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, "Command engine and composition roots": the per-action heavy declaration, if that section describes `WholeTree`; otherwise named as unaffected with the reason |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `ai/INDEX.md` row for `./le job`; `.claude/hooks/README.md` check table |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/post-release/engineering/spec-main-thread-schedules-heavy-work.md`; the pages in Files to Modify are the known ones |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/running-commands.md`, `ai/doors/le-tooling.md` and `ai/doors/core.md` show the package-test and `./le job run` examples with no caller named |

Discovery, per `ai/rules/repo-maintenance.md`:

| Question | Answer |
|----------|--------|
| Where an agent looks first | `ai/INDEX.md`: the `./le job` row and the row "Who executes this phase (main thread vs subagent)" |
| What rule prevents regression | The new point `commands/directives/heavy-work-starts-in-the-main-thread`, bound to the check |
| What registry prevents drift | The heavy declaration on the action table and `leroot`; `TestCensusActionsDeclareHeavy` pins the known set |
| What verification proves it | `./le ai hooks unit`, the hookruntime and job package tests, `./le test functional ui` for the `.ci` |

## Implementation Steps

Every phase's editing agent hands back its owed commands. The main thread runs them, one at a time, and commits. This spec is implemented under its own rule.

1. **Phase: Wiring (MANDATORY FIRST)** -- the declaration, the predicate, the caller predicate, the check registered and refusing nothing, the `report` action registered and answering an empty document
   - Tests: `TestSubagentHeavyCommandIsHandedBack`, `TestSubagentHeavyActionIsReadFromItsDeclaration`, `TestWaitingJobIsListedByTheReport`, `test/ui/le-job-report.ci`, `TestJobAreaDeclaresRunAndReport`
   - Files: `leaction.go`, `leroot` heavy files, `hookruntime/runtime.go`, `session.go`, `subagent_heavy.go`, `job/answer.go`, `job/register.go`
   - Verify: the entry points exist; the wiring tests fail because the check refuses nothing and the report is empty
2. **Phase: Declarations** -- every census area declares; the literal list in `heavyArea` is deleted; the lossy-pipe guard reads the declarations; each "verdict owed" cell is filled in this spec
   - Tests: `TestCensusActionsDeclareHeavy`, `TestHeavyLineReadsActionAreaAndHelp`, `TestLossyPipeReadsTheDeclarations`
   - Files: the census areas, `hookruntime/bash.go`
   - Verify: tests fail, then pass; the recorded verdict table's main-thread rows stand
3. **Phase: The refusal** -- the check's body, the tool names, the escape, one refusal per line, the advice of the other guards, the post-Bash advice
   - Tests: the remaining tests of `subagent_heavy_test.go`, `TestCallerPredicateAgreesWithIdentityCheck`, `TestSubagentUnformattedFileAdviceNamesNoFormatterRun`
   - Files: `hookruntime/subagent_heavy.go`, `rawheavy.go`, `postbash.go`, the verdict table and its test, the fixture catalog and digests, `.claude/hooks/README.md`, the new rule point and manifest
   - Verify: tests fail, then pass; `./le ai hooks unit`; `./le ai rules report gate-map`
4. **Phase: Post-edit** -- probe A-2 first; then the lint skip and the start text
   - Tests: `TestSubagentEditFormatsAndSkipsLint`, `TestMainThreadEditStillLints`, `TestSubagentStartTextNamesOwedCommands`
   - Files: `hookruntime/postwrite.go`, `lifecycle.go`
   - Verify: the probe's payload is recorded in this spec; tests fail, then pass. If the payload carries no agent id, stop and take D-5 back to Thomas
5. **Phase: Registry** -- owner fields and the waiter record
   - Tests: `TestEntryRecordsSessionAndAgent`, `TestEntryWithoutOwnerFieldsReadsAsUnknown`, `TestWaitingJobWritesAndRemovesItsRecord`, `TestWaiterRecordNeverOccupiesASlot`
   - Files: `job/registry.go`, `job.go`, `waiter.go`
   - Verify: tests fail, then pass; the package's contention and ownership tests pass unchanged
6. **Phase: Machine readings and the report** -- hostload, memory, the report's content, the `.ci`
   - Tests: the hostload tests, the `overview_test.go` tests, `test/ui/le-job-report.ci`
   - Files: `internal/core/hostload/`, `gotoolchain.go`, `job/overview.go`, the fixture, the docs for the command
   - Verify: tests fail, then pass; `./le test functional ui` for the one test; the report read once on this machine and pasted
7. **Phase: Text** -- agent definitions, skills, rule points, rationale, pages, doors, template; then the generators
   - Tests: `./le ai rules check metadata`, `./le doc check all`, `./le ai sync check`
   - Files: the prose files in Files to Modify
   - Verify: a grep for the sentences the inventory found returns none; the generated files are regenerated by the main thread

Phases 2, 5 and 7 touch disjoint files and run in parallel once phase 1 has landed. Phase 3 follows 2, phase 6 follows 5, phase 4 is independent of both.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | The main-thread rows of the recorded verdict table are unchanged. No subagent pass row in that table for an approve or bookkeeping command turned into a refusal |
| Fail-closed where decided, fail-open where decided | A caller with an agent id is never judged as the main thread. A script file passes, as D-1 accepts, and no code pretends otherwise |
| Messages | The hand-back refusal names no route a subagent is refused, asks for nothing a subagent cannot do, and ends with the command |
| No central list | `heavyArea` holds no area name. The only names left in the hook are external programs, which cannot register |
| Honest readings | No reading in the report is a zero that stands for a failure. The sentence on what slots do not count is present |
| Read-only | The report opens no file for writing and sends no signal |
| Text agrees | No canonical source still tells a subagent to run a test, a build, a lint or a compile check, or to land code |
| Rule: `ai/rules/no-layering.md` | The literal list is deleted, not kept beside the declarations. `totalMemoryGiB` holds no second parser |
| Rule: `ai/rules/stale-comments.md` | The comments on `bashRawHeavy`, `heavyArea`, `RegisterAdmitted`, `subagentCallRules` and `Holders` describe the new behavior |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The check is registered and bound | `./le ai rules report gate-map` names it with its point |
| A subagent's heavy command is refused | The hookruntime package test, and one live probe: a `ze-read` agent types `go test ./internal/le/job/` and reports the hook's message |
| The report answers | `./le job report` and `./le job report \| json` on this machine |
| The waiter is visible | The `.ci` test |
| Text agrees | `grep` over `ai/agents`, `ai/skills`, `ai/rules/points`, `ai/rationale`, `docs/contributing` for "Land your change yourself", "Prove your package", "spawn an agent to run this skill" in `ze-verify` returns none |
| Generated files are current | `./le ai sync check`, `./le ai rules check metadata` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Identity cannot be dropped | The verdict keys on the payload's agent id and the fork variable, never on text the agent typed |
| Registry fields are data | A session or agent id read from an entry is validated as a file-name-safe id before it is printed, and a malformed one is shown as unknown |
| Waiter record path | The record is created inside the jobs directory with exclusive creation, and the report refuses a path outside it, as the entry's log reader does |
| Report output | A command line read from an entry is printed as data and never executed |
| Resource use | The report forks one process-table scan, whatever the number of registry files |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| Probe A-2 shows no agent id on a post-tool payload | Stop phase 4 and ask Thomas which way D-5 is built |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The owner's rule existed for nine days in two files while six others said the opposite. A rule that changes who does a thing has to be carried to every text that assigns that thing, in the same commit, and to a hook, or it holds only where it is read.
- "What the hook classes as heavy" was assumed broad when decision D-1 was first worded and turned out to be three programs. The assumption was flagged as unverified and research overturned it before the spec was written.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Deny-list in the hook (D-1) | Allow-list: a subagent runs only readers and `./le` actions that declare themselves safe | Thomas chose it. The allow-list fails closed and covers scripts, at the price of friction whenever an agent needs something undeclared |
| A per-action heavy declaration beside `WholeTree`, plus an area-level one for areas with no action rows | Derive heaviness from the verb kind; keep the literal list in the hook and extend it | The verb kind is wrong both ways (`rfc check` and `repo compiles check` are gates that compile; `spec claim` and `commit create` are `run` verbs that start nothing). A list in the hook is the central enumeration the system door bans |
| Admitted and expensive registrations count as heavy | One registration replacing all three | Each keeps a meaning another reader uses: admitted means a slot is taken, expensive means the whole output is the verdict. Heavy is the union plus the actions that are neither |
| `./le exec` and `./le job run` declare themselves heavy | Read through them like a launcher and judge the program behind | Reading through needs the hook to know two le areas by name. A subagent that needs `gopls` types it directly |
| A subagent still commits a change that owes no execution | The hook refuses `./le commit create` from every subagent | D-2's reason is that a commit follows the tests. Prose, spec bookkeeping and a blocked hand-back owe none, and the closure rule requires the hand-back commit |
| Admission writes a waiter record | Report waiters as unknown | D-3 asks for waiters, and a queue the main thread cannot see is the thing it must schedule around |
| The report judges liveness with one process-table scan | Call `alive` per entry as admission does | Admission forks one `ps` per entry; the registry holds thousands of files |
| Memory and load are read in `internal/core/hostload` | A reader inside `internal/le/job`; reuse the product's `readMeminfo` | hostload is the existing tooling reader on both platforms, and `spec-test-verdicts-independent-of-load` needs the same readings for admission |
| The report shows readings and decides nothing | The report prints a go or wait verdict | Thresholds belong to admission by class (`spec-test-verdicts-independent-of-load`). The goal is that the main agent decides |
| A carrier is heavy for the subagent check and not expensive for the piped-command guard (`leroot.RegisterCarrier`, `CarrierLine`; `exec` registers, `job run` is one by its argv parameter) | One predicate for both guards | The two guards ask different questions. The subagent check asks whether the line starts heavy work, and `job run` and `exec` do. The pipe guard asks whether a filter truncates the verdict the command prints, and it judges commands, never arguments: the command a carrier runs is its argument, and refusing `./le job run ... \| tail` would advise wrapping the admitted form in a second one |
| The three older guards keep their verdict on a handed-back line and drop only their text (`withoutHandedBackAdvice`) | The guards return nil for a line the heavy-work check refuses | The recorded verdict table pins which guard refused each line, and a nil verdict removed 158 subagent rows. An empty text still counts in the exit code and prints nothing (`shownVerdicts`), so the agent reads one refusal |
| On a line a subagent is refused without a hand-back, the guard's text names only a route that caller has | Show the main thread's advice | The admitted form and the stated-reason escape are refused to a subagent on every line, so advice naming them teaches the violation (`plan/journal/guard-message-teaches-the-violation.md`) |
| `test integration` is heavy as an area, `clean` included | `clean` stays light because it starts no test | Every action drives the Docker daemon every session shares, and a subagent removes nothing on it (`ai/META-DECISIONS.md`, "How agents work") |
| The report takes no registry lock | Read under the lock admission takes | Taking the lock creates the lock file, a write AC-17 forbids, and holds up every admission for a process-table scan. The cost is stated in the header of `internal/le/job/overview.go`: an entry caught between its creation and its fields reads as unreadable and occupied for that instant |
| `Reading` renders as a value or as `unavailable: <reason>`, with unexported fields and two constructors | A float with a sentinel, or a pointer | A failed reading then cannot be written as a number by any renderer, and the zero `Reading` is unavailable too (`MarshalJSON`, `internal/le/job/overview.go`) |
| The refusal's heading names the command le resolved (`leroot.HeavyCommand`) | Print the first words typed after `./le` | Counting words named a keyword as the action (`./le job run label`). The name comes from the resolution that gave the verdict |
| `go fmt` and `go fix` are refused beside the six executing verbs (added at closure review) | Leave them to the deny-list's fail-open | Each is the toolchain's own formatter or rewriter over whole packages, the work D-1 names by "formatters", and a subagent refused `gofmt` would reach for `go fmt` |

## Known Limitations
- A subagent can still start heavy work through a script file, a string handed to a shell, `make`, Docker, QEMU, a compiled binary, or an `./le` action nobody declared. Thomas accepted this with the deny-list (D-1). The report's load, memory and process counts are where such work shows.
- The hook reads a program behind the launchers its launcher table names (`launcherValueOptions`, `internal/le/hookruntime/rawheavy.go`: `env`, `timeout`, `time`, `nice`, `sudo`, a shell). A heavy tool behind another launcher (`nohup`, `xargs`, `command`) is read as that launcher and passes, for the raw-job guard and for the subagent check alike. It is the same deny-list limit.
- A workflow agent whose hook payload carries no agent id is judged as the main thread (A-3).
- The subagent check keeps the quote-blind text judgment, so a heavy tool's name after `;`, `&&` or a bar inside a quoted argument hands the line back (R-14); `plan/post-release/engineering/spec-hook-le-data-arguments.md` owns reading a quoted argument as data.
- The slots count only admitted jobs. `./le test integration` and a command run under the raw-admission escape take none; `plan/post-release/engineering/spec-native-action-job-admission.md` owns admitting them.
- The report shows this checkout's registry. Another checkout's jobs appear only in the machine readings.
- The report shows readings and applies no threshold; `plan/post-release/engineering/spec-test-verdicts-independent-of-load.md` owns admission by memory and disk.

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
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
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
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-main-thread-schedules-heavy-work.md` only, in the same `./le commit create` script (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- The Bash hook check `bashSubagentHeavy` (`internal/le/hookruntime/subagent_heavy.go`), listed in `nativeHookActions["pretool-bash"]` before the build, pipe and raw-job guards. It refuses a subagent's test, build, vet, lint, formatter and heavy `./le` line with one hand-back whose last line is the command as typed.
- The heavy declaration: `leaction.Action.Heavy` on an action row, `leroot.RegisterHeavy` for an area or one selection of an action, `leroot.RegisterCarrier` for an area that runs a program its line names, and the readers `HeavyLine`, `HeavyCommand` and `CarrierLine` (`internal/le/le/root/heavy.go`). About forty areas declare. The hook's literal list (`heavyArea`, `expensiveAction`) is deleted.
- One caller predicate, `fromSubagent` (`internal/le/hookruntime/session.go`), read by the identity check, the advice prefix, the heavy-work check, the post-edit hook and the post-Bash advice.
- The post-edit hook formats a subagent's Go edit and starts no linter (`postFormatGo`), and the start text names the owed commands, the owed lint and the commit rule (`subagentCallRules`).
- The job registry records who asked (`SESSION`, `AGENT`, written by `take` and `writeWaiter` through `writeField`), a waiting job writes and withdraws a waiter record (`internal/le/job/waiter.go`), and `./le job report` answers the slots, holders, waiters and machine readings (`Admission.Overview`, `internal/le/job/overview.go`).
- Machine readings that report failure (`internal/core/hostload/readings.go`, `parse.go`, `memory_*.go`), and `totalMemoryGiB` reads through them.
- The text: two agent definitions, sixteen skills, the rule points and their rendered rules, two rationale pages, the contributing pages, two doors, the templates, `ai/INSTRUCTIONS.md`, `ai/META-DECISIONS.md`, `ai/INDEX.md` and `.claude/hooks/README.md`.

### Bugs Found/Fixed
- A caller-controlled argv word holding a line break followed by `SESSION=x` wrote a field line a reader took as the owner. `writeField` (`internal/le/job/registry.go`) keeps every text value on one line. Covered by `TestCommandTextCannotForgeTheOwner` and `TestWaiterCommandTextCannotForgeTheOwner`.
- The first build of AC-9 made three guards return nil for a handed-back line, which removed their refusals from the recorded verdict table. `withoutHandedBackAdvice` keeps the verdict and drops the text. Covered by `TestGuardVerdictsOfHeadStand` and `TestSubagentHeavyLineCarriesOneRefusal`.
- A guard's own text advised a subagent the admitted form on a line the check did not hand back. Each guard now builds a text for that caller. Covered by `TestSubagentIsNeverAdvisedTheAdmittedForm`.
- The refusal heading counted words after `./le` and named a keyword as the action. `leroot.HeavyCommand` names the resolved command. Covered by `TestSubagentHeavyRefusalNamesTheResolvedAction` and `TestHeavyCommandNamesWhatTheLineResolvedTo`.
- Found by the closure review: `go fmt` and `go fix` passed for a subagent while `gofmt` was refused. `heavyTool` now names both. Covered by two rows of `heavyToolLines` in `TestSubagentHeavyCommandIsHandedBack`.
- Found by the closure review: `Snapshot` and the report counted Go test processes through two implementations. `goTestProcessCount` (`internal/core/hostload/hostload.go`) reads `GoTestProcesses`.

### Documentation Updates
- `docs/contributing/running-commands.md`: "Waiting for another session's job", "Why one owner runs the suites", the identity and package-test sections, and at closure the passage on which le lines the pipe guard refuses, with anchors on `expensiveWords` and `HeavyLine, CarrierLine`.
- `docs/architecture/testing/runner-architecture.md`, "Contended run verdicts": the two kinds of hostload reader, anchored on `readings.go` and `Snapshot`.
- `.claude/hooks/README.md`: the `bashSubagentHeavy` section and the rows of the guards it changed; at closure the verb list gained `fmt` and `fix`.
- `ai/INDEX.md`: the `./le job` row.
- `./le doc check all` at closure: every red line is under `../gh-pages` (the published command catalog, another session's). It reports `PUBLISHED: 0 disagreement(s) with .claude/hooks/README.md` and names no page this spec edited.

### Deviations from Plan
- `test integration` is heavy as an area, `clean` included. The plan kept `clean` light.
- `job run` and `exec` are heavy and are not expensive for the pipe guard (the carrier rule). The plan had one predicate for both guards.
- The three older guards refuse a handed-back line without text. The plan had them add no refusal.
- `leroot.RegisterHeavy` takes a second form, an area, a verb and a selection (`repo check generated`).
- `TestAvailableMemoryIsParsed` sits in `hostload_test.go`, because the parsers are untagged and one file tests both platforms' text.
- The report answer shape is `ShapeDoc`: the document holds two row sets.
- A-1 was wrong as written: a fork carries an agent id (see Assumptions).
- `go fmt` and `go fix` join the refused verbs of AC-1.
- `ai/skills/ze-test-judge.md` and the point `a-closure-ends-closed-or-handed-back.md` needed no edit: neither tells a subagent to run a test.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The first wording of D-1 assumed the hook already classed a broad set of commands as heavy | It refused three programs (`rawHeavyWords`) and classed a few areas by literal for the pipe guard only | Research before the spec was written | D-1 was reworded as a deny-list with declarations. Recorded under Design Insights |
| assumption | A-1: a fork is told by `CLAUDE_CODE_FORK_SUBAGENT` in the hook process | A fork's payload carries an agent id, and the variable is a harness setting | The live fork probe and the harness documentation | Journal row in `plan/journal/gate-verdict-depends-on-the-machine.md`, owned by `spec-hook-le-data-arguments` |
| approach | AC-9 was first built by making three guards return nil for a handed-back line | The recorded verdict table pins which guard refused each line, and lost 158 subagent rows | The main thread's run of the hookruntime package | `withoutHandedBackAdvice` keeps the verdict and drops the text |
| approach | The main thread started a lint while its own agent was editing the linted package, twice | A lint judges the tree at the instant it runs, so the run judged a half-written file | The main thread's own account of its runs | The rule point's procedure: an owed command runs after the agent that owes it has handed back |
| approach | A test was added to a file after the judge's verdict on that file | The approval is pinned to the content the judge read, so the later edit staled it | The gate refused the stale approval | An approval is asked for once the file is final |
| approach | Implementation left `running-commands.md` naming `heavyArea`, a function it deleted, and a `Related:` header naming `expensiveAction` | A page and a header that name a deleted symbol are stale the moment the symbol goes | The closure review's grep for the deleted names | Both repaired at closure. A deleted symbol is grepped across `docs/` and headers in the commit that deletes it |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| G-1: a subagent's heavy command is refused and handed back | Done | `internal/le/hookruntime/subagent_heavy.go` `bashSubagentHeavy`, `heavyTool` | Deny-list, by D-1 |
| G-2: one command shows slots, holders, waiters and the machine | Done | `internal/le/job/overview.go` `Admission.Overview`; `answer.go` `answerReport` | |
| G-3: every text says the same thing | Done | commit 3726eca6b2 and the closure repairs | |
| D-1 to D-5 | Done | as the ACs below | D-4: `ai/skills/ze-verify.md` declares the main thread |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `heavyTool` reads `go` with `test`, `build`, `vet`, `run`, `generate`, `install`, `fmt`, `fix` behind `programTokens`; `handBackOf` walks every parsed command. `TestSubagentHeavyCommandIsHandedBack`, `TestSubagentHeavyRefusalEndsWithTheCommandToHandBack` (five lines, the command last) | Live probe: `go test` refused |
| AC-2 | Done | `heavyTool`, `lintRunsNoAnalysis`. Same tests; `TestSubagentLightCommandsPass` rows `golangci-lint version` and `linters` | Live probe: `gofmt` refused |
| AC-3 | Done | `programTokens` reads through the assignment, so `heavyTool` judges the program. `TestSubagentAdmitRawReasonDoesNotLiftTheRefusal`, with the main-thread half passing | Live probe |
| AC-4 | Done | `leroot.HeavyCommand`, `heavyInTable`. `TestSubagentHeavyActionIsReadFromItsDeclaration`, `TestSubagentHeavyRefusalNamesTheResolvedAction` (`job run`, `exec`, `test unit pkg`), `TestCensusActionsDeclareHeavy` light rows | Live probe: `./le test unit pkg`, `./le job run` refused; `./le job report` passed |
| AC-5 | Done | `resolveLine` (help, namespace), `heavyInTable` (listing, a pass-through area's trailing help). `TestHeavyLineReadsActionAreaAndHelp`, census rows `test unit`, `help test unit`, `exec --help` | Live probe: bare `./le test functional`, `./le help job` |
| AC-6 | Done | `bashSubagentHeavy` opens with `ctxReaderLine`. `TestSubagentLightCommandsPass` grep rows and the `commit create` row | A quoted separator before a tool name is still read (R-14) |
| AC-7 | Done | `bashSubagentHeavy` answers nil and `withoutHandedBackAdvice` answers the guard's own verdict when `fromSubagent` is false. `TestMainThreadHeavyVerdictsAreUnchanged`, `TestGuardVerdictsOfHeadStand`, the main-thread control of `TestSubagentHeavySitesAreJudgedByTheHook` | The pipe guard's set changed as AC-11 states |
| AC-8 | Done | `fromSubagent`. `TestForkHeavyCommandIsHandedBack`, `TestCallerPredicateAgreesWithIdentityCheck` | Live probe: a fork was judged by its agent id |
| AC-9 | Done | `withoutHandedBackAdvice`, `shownVerdicts`. `TestSubagentHeavyLineCarriesOneRefusal` (one `BLOCKED`, five lines, a git refusal still beside it), `TestSubagentIsNeverAdvisedTheAdmittedForm` | |
| AC-10 | Done | `Action.Heavy`, `RegisterHeavy`, the declarations. `TestCensusActionsDeclareHeavy` reads each registered table. `heavyArea` is gone | |
| AC-11 | Done | `expensiveWords` asks `HeavyLine` and `CarrierLine`. `TestLossyPipeReadsTheDeclarations` (`verify list` pipeable, `rfc check` refused, `job run ... \| tail` passes); census row `verify reds` light | |
| AC-12 | Done | `postFormatGo` returns after the two formatters when `fromSubagent`. `TestSubagentEditFormatsAndSkipsLint` (a stand-in linter leaves a marker if started), `TestMainThreadEditStillLints` | Live probe |
| AC-13 | Done | `Admission.caller`, `take`, `writeOwner`, `ownerOf`. `TestEntryRecordsSessionAndAgent` (through `Admit`), `TestEntryWithoutOwnerFieldsReadsAsUnknown`, `TestEntryOfAMainThreadRecordsNoAgent` | |
| AC-14 | Done | `queue`, `announceWait`, `withdrawWait`, `writeWaiter`. `TestWaitingJobWritesAndRemovesItsRecord` (two processes), `TestCanceledWaitRemovesItsRecord`, `TestWaiterRecordNeverOccupiesASlot` | |
| AC-15 | Done | `overview`, `Overview.Text`, `RegisterShape("job report", ShapeDoc)`. `TestWaitingJobIsListedByTheReport`, `TestReportListsLiveHoldersAndOmitsDeadEntries`, `TestReportReadingRendersInEveryFormat`, `test/ui/le-job-report.ci` | Run at closure, pasted under Goal Validation |
| AC-16 | Done | `Reading`, `readMachine`, the hostload readers. `TestReportShowsAnUnavailableReadingAsUnavailable` (a failed load beside a measured zero), `TestReadingReportsFailure` | `answerReport` exits 0 whenever it answered |
| AC-17 | Done | `overview` takes no lock and writes nothing. `TestReportWritesNothing` (bytes and modification times, no lock file, no directory created), the fixture's `judgeReadOnly` | |
| AC-18 | Done | `subagentCallRules`. `TestSubagentStartTextNamesOwedCommands` | This closing agent's own start text carried the three lines |
| AC-19 | Done | The files of commit 3726eca6b2. The Deliverables greps return none; `./le ai sync check` answers in sync | |
| AC-20 | Done | The point `ai/rules/points/commands/directives/heavy-work-starts-in-the-main-thread.md`, third bullet, rendered into `ai/rules/commands.md` | Prose; `./le ai rules report gate-map` binds the point to the check |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The eight tests of `subagent_heavy_test.go` | Done | `internal/le/hookruntime/subagent_heavy_test.go` | Plus `TestSubagentHeavyRefusalNamesTheResolvedAction` and `TestSubagentIsNeverAdvisedTheAdmittedForm` |
| `TestCallerPredicateAgreesWithIdentityCheck` | Done | `internal/le/hookruntime/session_identity_test.go` | |
| `TestHeavyLineReadsActionAreaAndHelp` | Done | `internal/le/le/root/heavy_test.go` | Plus the carrier, command-name and read-only tests |
| `TestCensusActionsDeclareHeavy` | Done | `internal/le/heavy_census_test.go` | |
| `TestLossyPipeReadsTheDeclarations` | Done | `internal/le/hookruntime/bash_test.go` | |
| `TestSubagentEditFormatsAndSkipsLint`, `TestMainThreadEditStillLints` | Done | `internal/le/hookruntime/postwrite_test.go` | |
| `TestSubagentUnformattedFileAdviceNamesNoFormatterRun` | Done | `internal/le/hookruntime/postbash_test.go` | |
| `TestEntryRecordsSessionAndAgent`, `TestEntryWithoutOwnerFieldsReadsAsUnknown` | Done | `internal/le/job/registry_owner_test.go` | |
| `TestWaitingJobWritesAndRemovesItsRecord`, `TestWaiterRecordNeverOccupiesASlot` | Done | `internal/le/job/waiter_test.go` | |
| `TestWaitingJobIsListedByTheReport`, `TestReportListsLiveHoldersAndOmitsDeadEntries`, `TestReportShowsAnUnavailableReadingAsUnavailable`, `TestReportWritesNothing` | Done | `internal/le/job/overview_test.go` | |
| `TestJobAreaDeclaresRunAndReport` | Done | `internal/le/job/answer_test.go` | Replaced the one-action test under a judge's approval |
| `TestReadingReportsFailure` | Done | `internal/core/hostload/hostload_test.go` | |
| `TestAvailableMemoryIsParsed` | Changed | `internal/core/hostload/hostload_test.go` | One file for both platforms' text |
| `TestSubagentStartTextNamesOwedCommands` | Done | `internal/le/hookruntime/lifecycle_test.go` | |
| `le-job-report` | Done | `test/ui/le-job-report.ci`, `internal/test/fixture/ui_fixture_le_job_report.go` | Held 80 stress iterations before promotion |
| Hook fixture sites | Done | `internal/le/ai/hooks/fixture_catalog.go`, `subagent_heavy_sites_test.go` | Five sites, each run through `hookruntime.Run` |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file under Files to Create | Done | Listed under Pre-Commit Verification |
| `internal/le/hookruntime/` and `internal/le/le/` files | Done | Also `lossypipe.go`, which holds the pipe refusal's texts |
| The census areas | Done | Each area's own `actions.go` or `register.go` |
| `internal/le/job/`, `internal/core/hostload/`, `gotoolchain.go` | Done | |
| The prose files | Done | Two needed no edit (Deviations) |
| `internal/le/ai/hooks/fixtures.go` digests | Done at the seal step | Sealed after this review, as `.claude/hooks/README.md`, "Hook tests", orders |

### Audit Summary
- **Total items:** 3 goals, 5 decisions, 20 acceptance criteria, 27 planned tests, the file groups above
- **Done:** all
- **Partial:** none
- **Skipped:** none
- **Changed:** the nine entries under Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| G-1: a subagent's heavy command is refused and handed back | Live probe on the committed hook, beyond the unit assertions | A `ze-read` subagent was refused `go test`, a `ZE_ADMIT_RAW`-prefixed `go test`, `./le test unit pkg`, `./le job run` and `gofmt`, each with the hand-back, and passed `./le job report`, `./le spec current`, `go version`, a reader naming `go test`, bare `./le test functional` and `./le help job`. A fork was judged the same by its agent id. This closure ran under the same hook: its readers and light gates passed |
| G-2: one command shows the slots, who holds and waits, and the machine | The command, run at closure on the owner's Mac | `./le job report` printed: `job slots: 4 total, 0 occupied, 4 free`; `holders: none`; `waiters: none`; `cores: 16`; `load 1m: 7.27 (0.45 per core)`; `load 5m: 15.42`; `load 15m: 19.20`; `memory available: 18.6 GiB of 64.0 GiB total`; `disk free, tmp and Go build cache (one volume): 45.7 GiB`; `le test processes: 0`; `go test processes: 0`; and the notice on what the slots do not count. `./le job report \| json` rendered each reading as `{"value": n}`. Holders and waiters with their sessions: `test/ui/le-job-report.ci` |
| G-3: every text says the same thing | Greps and the generated-copy check, run at closure | `grep -rn "Land your change yourself\|Prove your package"` over `ai/agents`, `ai/skills`, `ai/rules/points`, `ai/rationale`, `docs/contributing`, `ai/INSTRUCTIONS.md`, `ai/doors`, `plan/templates` and `.claude/hooks/README.md` returns nothing. `ai/skills/ze-verify.md` holds no "spawn an agent to run this skill". `./le ai sync check` answers `generated agent files in sync`. `./le ai rules report gate-map` lists `commands/directives/heavy-work-starts-in-the-main-thread <- bashSubagentHeavy`, `DANGLING: 0`, `REGRESSED: 0` |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| A probe of a workflow agent's hook payload (A-3), and keying the caller on the agent id alone (A-1) | The harness offered no workflow agent to probe, and changing the predicate rewrites the fork tests | `plan/post-release/engineering/spec-hook-le-data-arguments.md` (journal row `gate-verdict-depends-on-the-machine`, 2026-11-13) |
| Reading a quoted argument as data in the text judgment the subagent check reuses (R-14) | The same split serves the build and pipe guards, and one change fixes all three | `plan/post-release/engineering/spec-hook-le-data-arguments.md` |
| Admitting `./le test integration` and raw-escape commands to the slots, and pruning result receipts | Outside this spec from its design: the report shows what admission does today | `plan/post-release/engineering/spec-native-action-job-admission.md` |
| Admission acting on memory and disk | The report shows the readings and decides nothing, by design | `plan/post-release/engineering/spec-test-verdicts-independent-of-load.md` |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/main-thread-schedules-heavy-work-ed46d90a-75f4-4e5a-81eb-76bf12dc5a18.md` (96 files, verdict clean), recorded after the seal over the final content of `internal/le/ai/hooks/fixtures.go` |
| `./le spec review check` | `review_gate: OK (96 code files, clean, hashes match ...)` |
| Rounds | 3 |
| Reviewer lenses used | hook correctness and escape shapes; fail-open and fail-closed; messages; no central list; text agreement and the skill walk; stale comments and anchored pages; security; the style pass |

Round 1 scope, written before it ran: the whole diff of the commits that carry this spec's `Spec-work:` line (4f23dad6cf, a7e6646804, 3726eca6b2, 6ed5abeb73, 6ec0050f8d, 76ce134b55, ee00f04a8d), every lens above, each reading the producer.

Round 2 scope, written before it ran: the fixes of round 1 and what they touch: `heavyTool` and its two test rows, `goTestProcessCount` and `Snapshot`, the two comments in `waiter.go`, the passage in `running-commands.md`, the README verb list, the `Related:` header of `expensive_test.go`.

Pre-checks: `./le test weakened check base origin/main` answered clean over 19 changed test files. `./le repo check` named 7 issues, none in a file of this change set (`page.go`, `rfc/next.go`, `rfc/profilecoverage.go`, `site/buildproducer.go`, `test/deployment/vppevidencempls.go`).

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `go fmt` and `go fix` pass for a subagent while `gofmt` is refused: the toolchain's own formatter over whole packages escapes G-1 | `heavyTool`, `internal/le/hookruntime/subagent_heavy.go` | Both verbs added, two rows in `heavyToolLines`, the README verb list |
| 2 | ISSUE | A page names a deleted function and states verdicts the change reversed (`verify reds` and `verify list` are pipeable now) | `docs/contributing/running-commands.md`, the passage after "It is a query, never a certificate" | Passage rewritten from `expensiveWords` and `HeavyLine`, with anchors |
| 3 | ISSUE | Two implementations of one count: `processCount` through a shell pipeline beside `GoTestProcesses` (style guide, "State that goes stale") | `internal/core/hostload/hostload.go` | `goTestProcessCount` reads `GoTestProcesses`; `processCount` deleted |
| 4 | ISSUE | A paired call states its obligation on neither side (style guide, "Comments say why and state the contract") | `announceWait`, `withdrawWait`, `internal/le/job/waiter.go` | MUST on both |
| 5 | NOTE | A file header names the deleted `expensiveAction` | `internal/le/ai/hooks/expensive_test.go` | Header names `expensiveWords` |
| 6 | NOTE | A launcher outside the hook's table (`nohup`, `xargs`) hides a heavy tool, for the raw-job guard and this check alike | `launcherValueOptions`, `internal/le/hookruntime/rawheavy.go` | Recorded under Known Limitations: the deny-list's accepted limit |
| 7 | NOTE | The quote-blind text judgment hands back a line whose quoted argument holds a separator and a tool name | `handBackOf` | R-14, owned by `spec-hook-le-data-arguments` |
| 8 | NOTE | A recorded command line is printed with its control characters other than line breaks | `truncate(held.command)`, `internal/le/job/overview.go` | Not fixed: the registry is written by this user's own sessions. Recorded here |

Judged: the fold of finding 3 left `parseDigits` with no caller but `TestParseDigitsStopsAtNonDigit`. An independent `ze-test-judge` approved deleting both: the new path never turns text into a number, and each old input's concern is proved by `TestReadingReportsFailure` (`processLines`) and `TestGoTestProcessesAreCountedByName`. Both are deleted, and `TestSnapshot` gained the assertion the judge recommended: `GoTestProcs` is at least 1, because the test binary is itself a `.test` process, which proves the wiring the fold changed.

Round 3 scope, written before it ran: the three digest constants pasted into `internal/le/ai/hooks/fixtures.go` with their comments, the judged deletion, and the added assertion. Result: 0 BLOCKER, 0 ISSUE. Each constant holds the bytes `TestEveryStoredDigestMatchesTheTreeItRatchets` printed; the site refusal of `TestFixtureCatalogExactPopulationAndContent` reads the same `fixtureSiteDigest` constant (`fixtures.go`, the check that prints "site content changed"), so no second literal exists. `internal/core/hostload`, `internal/le/job` and `internal/le/ai/hooks` are outside `hookSourcePackages`, so those edits do not move the source digest. The digest hashes the working tree, where `internal/le/rfc`, a watched package, carries another session's uncommitted edits. The first literal sealed those edits, so it was replaced: `hookSourcesDigest` is the digest of the content the committed tree holds, computed by an independent read-only agent with the code's own framing and the edited files taken from their `HEAD` blobs, after it reproduced the working-tree value over the same file set. It was computed again against `f43c8ce222` after that session committed into the package. The seal test is therefore red in a working tree that still holds those edits and green on a clean checkout of the commit. That replacement is the one edit after round 3, a constant and its comment, re-recorded in the artifact above.

Round 2 result: 0 BLOCKER, 0 ISSUE. `heavyTool` reads the verb at `program[1]`, where Go takes it; the two new rows run through `TestSubagentHeavyCommandIsHandedBack` and through the main-thread loop of `TestMainThreadHeavyVerdictsAreUnchanged`; no recorded verdict row and no fixture site names either verb. `GoTestProcesses` and the pipeline it replaces count the same thing, a `ps` name holding `.test`, and a failed sample is still zero for `Contended`, named as a guard.

The style pass ran over every changed Go file. No `panic` is reachable from input: `liveness.word` is called only after the gone verdict returned, and `Unavailable` is called with a fixed or non-empty reason. Every loop is bounded by a listing, a table or a path's depth, and the queue's endless loop says so. The report takes a validated `*Admission`.

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/hookruntime/subagent_heavy.go`, `subagent_heavy_test.go` | Yes | `ls` at closure, 12K and 26K |
| `internal/le/le/root/heavy.go`, `heavy_test.go` | Yes | `ls`, 14K and 17K |
| `internal/le/heavy_census_test.go` | Yes | `ls`, 8.5K |
| `internal/le/job/overview.go`, `overview_test.go`, `waiter.go`, `waiter_test.go`, `registry_owner_test.go` | Yes | `ls`, 25K, 24K, 5.7K, 11K, 7.5K |
| `internal/core/hostload/memory_linux.go`, `memory_darwin.go`, `memory_other.go` | Yes | `ls`, 812, 1.2K, 627 bytes. Their tests are in `hostload_test.go` |
| `test/ui/le-job-report.ci`, `internal/test/fixture/ui_fixture_le_job_report.go`, `register_ui_le_job_report.go` | Yes | `ls`, 2.2K, 17K, 430 bytes |
| `ai/rules/points/commands/directives/heavy-work-starts-in-the-main-thread.md` | Yes | `ls`, 1.9K |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 to AC-9 | The hook's verdicts and texts | The producers and the test bodies read at closure (Implementation Audit). The package run is the main thread's: `./le test unit pkg path ./internal/le/hookruntime/`, owed again after the closure fix |
| AC-10 | Declarations, no list | `git grep -n "Heavy: *true\|RegisterHeavy(\|RegisterCarrier("` over `internal/le` lists each declaration in its own area's `actions.go` or `register.go`; `git grep "heavyArea\|expensiveAction" -- internal/le` finds no function |
| AC-11 | The pipe guard reads the declarations | `expensiveWords` read; `TestLossyPipeReadsTheDeclarations` rows read |
| AC-12 | Format, no lint | `postFormatGo` read: the return sits above `exec.LookPath("golangci-lint")`. This closure's own Go edits came back with no lint verdict |
| AC-13, AC-14 | Owner fields, waiter record | `take`, `writeWaiter`, `readEntry`, `readWaiter` read; the test bodies read |
| AC-15 to AC-17 | The report | `./le job report` and `./le job report \| json` run at closure; `./le help job` lists `report` |
| AC-18 | Start text | This agent's start text held the three lines |
| AC-19, AC-20 | Text | The greps of Goal Validation; the point file read |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `le job report` typed at the launcher | `test/ui/le-job-report.ci` | Read: it runs `le test fixture ui/le-job-report`, whose driver starts two `le job run` processes in a scratch checkout, polls `le job report \| json` until the waiter is listed, judges the rows and the read-only property, and expects `OK` |
| A subagent's Bash payload through `pretool-bash` | none: a hook has no `.ci`; the fixture sites are its end-to-end form | `TestSubagentHeavySitesAreJudgedByTheHook` runs each site through `hookruntime.Run`; the live probes ran the installed hook |
| A subagent's Edit payload through `posttool-writeedit` | none | `Probe("postFormatGo", ...)` in `TestSubagentEditFormatsAndSkipsLint`; the live probe |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | broken | A fork carries an agent id; the variable is a harness setting. Mistake Log, Deviations, journal row |
| A-2 | confirmed | Harness documentation and the live edit probe |
| A-3 | confirmed for a teammate, not established for a workflow agent | Harness documentation. The consequence and its owner are under Known Limitations and Work Not Done |
| A-4 | confirmed | The census search at closure: no `.sh` or `.py` reader |
| A-5 | confirmed | `readMemoryAvailable`, `parseVMStatAvailable`; 18.6 GiB of 64.0 GiB read on the Mac |
| A-6 | confirmed | Every census cell holds a verdict and its producer; the producers were read at closure |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| 3, CLI command: `docs/contributing/running-commands.md`, "Waiting for another session's job" | `answerReport`, `Overview.Text` | Yes |
| 10, test infrastructure: `docs/contributing/testing.md` names the main thread as the runner | commit 3726eca6b2 | Yes |
| 12, architecture: `docs/architecture/core-design.md` does not describe `WholeTree` (`grep -n WholeTree docs/architecture/core-design.md` returns nothing), so it is unaffected. The declaration is documented in `running-commands.md` and `.claude/hooks/README.md` | `leaction.Action.Heavy` | Yes |
| 15, inventory: `ai/INDEX.md` `./le job` row; `.claude/hooks/README.md` check table | `./le doc check all`: 0 disagreement with the README | Yes |
| 16, anchored pages: the seven pages `./le spec citation anchors` names anchor on `Environment`, `RunContext`, `New`, `Answer`, `Run`, `Action.WholeTree` and session-start symbols, none of which changed meaning | `grep -n "source: internal/le/..."` over those pages | Yes |
| 17, examples: the doors name the main thread above their gate tables | `ai/doors/le-tooling.md`, `ai/doors/core.md`, "Gates" | Yes |
| 1, 2, 4 to 9, 11, 13, 14: not applicable | Development tooling: no feature, config, RPC, plugin, wire, RFC, comparison, metadata or metric surface changed | Yes |
| Doctor check | None owed: the report reads the development machine and states an unavailable reading itself | Yes |

### Security Review
| Check | Result |
|-------|--------|
| Identity cannot be dropped | `fromSubagent` reads `ctx.payload.AgentID` and the hook process's environment. An assignment typed on the line does not reach the hook's environment |
| Registry fields are data | `ownerOf` passes each value through `lepath.ValidSessionID` and `ValidAgentID`; a hostile value reads as unknown (`TestEntryWithoutOwnerFieldsReadsAsUnknown`). `writeField` keeps a value on one line |
| Waiter record path | `os.CreateTemp` in the jobs directory creates it exclusively; the report reads a `.wait` name only when the listing says it is a regular file |
| Report output | The command is truncated to 200 bytes and printed; nothing executes it |
| Resource use | One `procscan.Scan`, two `ps` reads and one `vm_stat` or `/proc` read, whatever the registry holds. The closure fold removed a `sh -c` pipeline |
