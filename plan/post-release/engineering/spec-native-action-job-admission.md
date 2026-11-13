# Spec: native-action-job-admission

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Priority | P3 |
| Release reason | Development tooling: job admission for `./le` actions on the shared machine. No shipped behavior changes, so it stays post-release engineering. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

Dependent: `plan/post-release/engineering/spec-test-verdicts-independent-of-load.md`
adds memory and disk to admission. This spec leaves it one attachment point:
the per-action declaration (see "What this leaves for the resource-class spec").

## Task

The owner's goal, in his words: "for the main agent to be able to monitor the
machine load and decide how to schedule tests and work". `./le job report`
shows the job slots, their holders and waiters, and the machine. It is only
useful when the heavy work on the machine holds a slot. Today most heavy
`./le` actions take none: the unit groups, the integration labs, the
functional gating run, the RFC record walk, every build, every perf and chaos
run. On being shown this the owner said: "fix the defects !".

Every heavy action already declares itself (`leaction.Action.Heavy`,
`leroot.RegisterHeavy`), so the dispatcher can tell a heavy line from a light
one. This spec uses that declaration to admit.

Goals:

1. Every `./le` line that declares itself heavy holds exactly one job slot
   while its body runs, unless its own declaration says it admits itself or
   takes no slot, with the reason.
2. `./le job report` shows each such holder under a label a reader
   recognizes, with its session and agent.
3. A tree of `le` processes holds one slot, and never waits for a slot its
   own root holds.
4. A new heavy action is admitted the day it declares itself heavy, with no
   edit outside its own package, and a test fails when one is not.
5. The sentence "admission still queues whatever is started" in
   `docs/contributing/running-commands.md` becomes true for every heavy
   `./le` line.

### The owner's decisions, 2026-11-13

| # | Question | Decision | Set aside | Accepted cost |
|---|----------|----------|-----------|---------------|
| 1 | Where admission happens | "Central, by declaration": one admission point in the dispatcher takes a slot for every action that declares itself heavy, before its body runs, and releases it with the action's exit code. An opt-out is declared on the action's own row. The actions that already admit themselves keep their label, shared verdicts and progress log by declaring so | Each of about thirty areas calling admission itself: the admitted set becomes a hand-kept list again (`plan/journal/refactor-removes-feature.md`, 2026-09-05), and no behavioral test can prove a new heavy action is admitted | More queueing between sessions. A newly admitted action holds a slot without a progress log, so a waiter sees who holds it and not what stage it is at |
| 2 | What a long run weighs | "One slot, whole run": a Docker interop lab or a QEMU guest holds one slot from start to end, as `./le test qemu run` already does | A lab taking every slot | Weighing by memory and disk stays with `spec-test-verdicts-independent-of-load.md` |
| 3 | `./le exec <program>` | "By the program it carries": `exec` takes a slot only when the program behind it is a heavy tool, by the definition the hook already applies to raw commands: the Go toolchain's executing verbs, the linters, the formatters. `./le exec go test` and `./le exec golangci-lint run` take a slot, or run inside their parent's. `./le exec gopls` and `./le exec go version` take none. The hook and `exec` share one definition | Shims stating the raw-admission escape; a private shim marker; shims that stop using `exec` | A program outside the definition, run through `exec`, takes no slot: the same gap as the hook's list |
| 4 | The raw-admission escape | Unchanged. `ZE_ADMIT_RAW="<reason>"` stays outside admission and keeps one reader, the hook (`ai/rules/points/commands/directives/heavy-jobs-are-admitted-by-native-actions-never-typed-raw.md`: "A one-off that MUST NOT queue states its reason in the command"). `exec` does not read it. `./le job report` keeps showing such work only as machine load | - | - |

### What this replaces in the skeleton of 2026-09-05

| Skeleton | Now | Why |
|----------|-----|-----|
| Key decision "Admit inside each action rather than around the dispatcher", because "a dispatcher cannot tell a heavy action from `./le spec current`" | Replaced by decision 1 | `HeavyCommand` answers that question from each area's own declaration, so the reason is gone |
| Phase "the rest: one action at a time, each with its own `jobArgv`" | Replaced: one admission point, one derived label | The per-action list is the hand-kept population the journal row records shrinking with nothing red |
| AC-3, a check over actions that reach a heavy tool with no ticket | Replaced by a behavioral census over the declarations (AC-15) | The forward check is possible only through a central seam. The converse check is a limitation, stated below |
| AC-4, two sessions asking for the same heavy action attach | Kept only for the actions that admit themselves | A generic attach would let a second identical build take the first one's verdict without the action having agreed to share |
| A-2, admitting `verify current` | Removed | `runFast` and `admitWorktree` admit today |
| Nested rule, exit-code contract, `./le job run` as the generic route, the Owned journal rows table | Kept | Still true at the producers |

### Where the code shaped the approved design

Three points where the code did not allow the first wording. The owner
ruled on each on 2026-11-13.

| # | First wording | What the code does | Ruling |
|---|---------------|--------------------|--------|
| C-1 | The dispatcher releases the slot with the action's exit code | `le exec` never returns: its `run` calls `replace`, which replaces the `le` process with the program, so no code after the body runs and nothing can release | Accepted: `exec` admits itself. When the carried program is a heavy tool and no parent holds a slot, it runs the program as a child and releases with the child's code. Otherwise it replaces the process as today |
| C-2 | `./le exec` takes a slot like any heavy action | Every tool shim `./le setup` writes starts its tool through `le exec` (`shimScript`): `gopls`, `golangci-lint`, `staticcheck`. An editor's `gopls` server would hold a slot for as long as the editor runs | Decision 3: by the program it carries. A shim's `gopls` takes no slot. The shims do not change |
| C-3 | The dispatcher admits before the body | `verify evidence release-candidate` is `WholeTree`, heavy and does not admit itself, and the whole-tree gate runs inside the body | Accepted: the dispatcher asks the whole-tree gate before admitting, and the body asks again |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/le-tooling.md` - who depends on `le` from outside this session
  → Constraint: a before-dispatch hook "never prints, never waits, returns nothing", so admission cannot be one. It sits in the dispatch path itself.
  → Constraint: every hook call of every session runs `le ai hooks <name>` through the same dispatcher, so the admission point must cost a light line nothing beyond an in-memory lookup.
- [ ] `docs/architecture/core-design.md`, "Command engine and composition roots" - how an area registers
  → Constraint: an area registers from `init()` in its own package. The dispatcher names no area, so each handling is read from a declaration.
- [ ] `docs/contributing/running-commands.md`, "One verify at a time" - the admitted population the page states
  → Decision: the page's list of admitting actions is replaced by the criterion, and its last sentence, which names this spec as owed work, goes.
- [ ] `docs/contributing/running-commands.md`, "Waiting for another session's job" - what the report shows and leaves out
  → Constraint: the report's closing notice and the paragraph under it say a Docker interop lab takes no slot. Decision 2 makes that false, so both change with the code (`slotsNotice`).
- [ ] `docs/contributing/running-commands.md`, "The long form, for any other command" - `./le job run`
  → Constraint: the child's environment names the job's entry and keeps the parent setting (`commandEnvironment`, `childEnviron`), which is what makes `job run command ./le <heavy>` one slot.
- [ ] `docs/contributing/running-commands.md`, "Why one owner runs the suites" - attribution
  → Constraint: admission does not replace the one-owner rule. A subagent still starts no heavy line; the hook refuses it before the binary runs.
- [ ] `ai/rules/principles.md` - registration over enumeration, no silent zero
  → Constraint: the default for a heavy line is admission. A line nobody declared anything about is admitted, never skipped.
- [ ] `plan/journal/refactor-removes-feature.md`, 2026-09-05 - how the admitted population was lost
  → Decision: the admitted set is derived from the heavy declarations and never listed.

**Key insights:** (minimal context to resume after compaction)
- The mechanism exists and is tested (`internal/le/job`). The heavy declarations exist. What is missing is the one call that joins them.
- `internal/le/job` imports `leroot` (its registration) and `leaction` (its action table), so neither can import `job`. The join is an injected seam that `job` fills.
- A holder that names itself in its environment makes every `le` below it `KindInside` (`insideParent`). That is the whole nesting rule.
- `le exec` replaces its process, and every tool shim starts through it. Both shape decision 3.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/job/job.go` - `admit` validates the label, answers `KindInside` when `insideParent` finds a readable parent entry, fingerprints the inputs with `InputHash` for every asker, then queues. `Release` writes the duration row and the receipt, and removes the entry and the log. `defaultSlots` is the core count divided by `gotoolchain.CoresPerJob`; nothing weighs a job.
- [ ] `internal/le/job/registry.go` - `scanAndClaim` counts live and unassessable entries; `shares` lets an asker attach only when the asker's `MayAttach` is true and label, key and tree match, and a tree of `Unknown` matches nothing; `take` always creates a log file and writes a `LOG` field; `reportBusy` prints log silence for a holder whose entry names a log.
- [ ] `internal/le/job/answer.go` - `job run` is a row that declares `Heavy` and a `KindArgv` parameter, and calls `Admission.Run`.
- [ ] `internal/le/job/register.go` - imports `leroot` to register the area.
- [ ] `internal/le/job/overview.go` - `slotsNotice` says Docker labs appear only in the machine readings. A holder row carries label, PID, liveness, age, session, agent, command and entry, and reads no log.
- [ ] `internal/le/le/root/heavy.go` - `HeavyCommand` answers the heavy verdict and the command name from one resolution (`resolveLine`), reading `RegisterAdmitted`, `RegisterHeavy`, `RegisterExpensive` and the rows' `Heavy`.
- [ ] `internal/le/le/root/leroot.go` - `runWithStartup` splits the pipe chain, validates it, then runs the handler, or for a `PrepareArgs` row runs the preparation, the startup and the prepared answer. It renders after the body returns.
- [ ] `internal/le/le/root/dispatch.go` - `dispatchLine` answers a retired spelling, a namespace and a trailing help word before any handler, and asks for a missing value at a terminal before it calls `runWithStartup`.
- [ ] `internal/le/le/root/beforedispatch.go` - a before-dispatch hook returns nothing and must not wait.
- [ ] `internal/le/le/action/leaction.go` - `Area.admit` runs the whole-tree gate inside the area, after the line parsed and before the body. `List.Accepts` parses a line without running it.
- [ ] `internal/le/heavy_census_test.go` - `TestCensusActionsDeclareHeavy` pins the heavy verdict of every area's rows.
- [ ] `internal/le/test/harnesstool/harnesstool.go` - `admitRun`: no checkout runs unadmitted; `KindInside` runs in process; a claim re-executes the runner as a child so the log grows.
- [ ] `internal/le/exec/exec.go`, `internal/le/exec/register.go` - `run` replaces the process; the area registers as forwarding, heavy and carrier.
- [ ] `internal/le/setup/shims.go` - `shimScript` writes a shell script that replaces itself with `<le> exec <real tool>`.
- [ ] `internal/le/hookruntime/bash.go` - `bashRawHeavy` refuses a heavy job typed raw. It judges typed text, so a heavy tool an `le` action starts inside its own process passes it.
- [ ] `internal/le/hookruntime/subagent_heavy.go` - `heavyTool` is the definition decision 3 names. It reads a command's words down to the program (`programTokens`), then judges the program by name: `go` with one of the verbs test, build, vet, run, generate, install, fmt, fix; `golangci-lint` unless `lintRunsNoAnalysis`; `staticcheck`, `govulncheck`, `gofmt`, `goimports`, `gofumpt`. An `le` line is judged by `leroot.HeavyCommand`, so `./le exec go version` is handed back today because `exec` registers its whole area heavy.
- [ ] `internal/le/hookruntime/rawheavy.go` - `lintRunsNoAnalysis` is the one list of the linter's subcommands that run no analysis. `rawHeavyWords`, the main thread's raw-job guard, refuses a narrower set (`go test`, `go vet`, the linter) and advises the admitted form; it reads `ZE_ADMIT_RAW` through `admitRawAssignment`.

**What admits today, verified at each call site:**

| Action | Site | Label and policy |
|--------|------|------------------|
| `job run` | `answerRun`, `internal/le/job/answer.go` | the typed label; attach on |
| `test unit pkg` | `admittedQuietRun`, `internal/le/test/unit/pkg.go` | `RunQuiet`; attach on |
| `go lint run` | `internal/le/go/lint/actions.go`, `AdmitContext` in process | its own label and argv; writes the slot's log when the ticket names one |
| `verify current`, `worktree`, `changed` | `runFast`, `internal/le/verify/current.go`; `admitWorktree`, `internal/le/verify/lifecycle.go`; `nameJobParent`, `internal/le/verify/engine/run.go` | names itself parent of its stages |
| suite runners | `RunnerAnswer` and `admitRun`, `internal/le/test/harnesstool/harnesstool.go` | marks `RegisterAdmitted`; re-executes itself |
| `test stress-repro` | `admitRun` and `runClaimed`, `internal/le/test/stressrepro/run.go` | attach off; names itself parent |
| `test qemu run` | `admitQEMURun`, `internal/le/test/qemu/run_admit.go` | attach off; re-executes itself |
| the qemu build helpers | three `Run` calls in the package `internal/le/test/qemu` | `qemu-build-le`, `qemu-build-ze`, `mobike-build-<name>` |
| `test attribution report` | `admittedRunner`, `internal/le/test/attribution/actions.go` | one slot per re-run, attach off |

**What takes no slot today:** `test unit <group>` and `test integration`
(`commandRunner`, `internal/le/test/integration/actions.go`, calls
`gaterun.Run`, and its native runners call the lab directly);
`test functional gating` (`commandLine` and `Execute`,
`internal/le/test/functional/run.go`, start each suite as a child `le` under
`timeout`; the parent holds nothing); `rfc record` and `test redwalk`;
`feature run`; `repo check generated`; `exec`; and the rest of the heavy
census in `internal/le/heavy_census_test.go`.

**Behavior to preserve:**
- Every action's exit code, unchanged. The commit gate and `job run` read it.
- The self-admitting actions' labels, attach policy and logs.
- The nested-job rule: a job whose named parent entry exists runs inside it.
- A listing, a help page, a namespace and a light action start nothing and wait for nothing.
- The whole-tree gate refuses before any slot is asked for.
- `ZE_ADMIT_RAW` as the hook reads it today.
- With no checkout, a run is unadmitted.

**Behavior to change:**
- Every heavy line not declared otherwise takes one slot before its body runs.
- `exec` takes a slot when typed as an `le` line.
- The report's notice and the page stop saying a Docker lab takes no slot.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Heavy declaration on a row | `internal/le/le/action/leaction.go` `Action.Heavy` | Marks a verb the main thread schedules; the hook refuses it from a subagent | Extends: the same declaration now also selects admission. No second declaration of "heavy" |
| Heavy declaration by name | `internal/le/le/root/heavy.go` `RegisterHeavy` | An area with no rows, an all-heavy area, or one selection (`repo check generated`) | Extends: read by the slot reading as it is read by `HeavyCommand` |
| One resolution of a typed line | `internal/le/le/root/heavy.go` `resolveLine`, `HeavyCommand`, `HeavyLine` | Answers verdict and command name the way dispatch resolves the line | Extends: the slot reading is a third answer from the same resolution, and the label is derived from the name `HeavyCommand` answers |
| Carrier marks | `internal/le/le/root/heavy.go` `RegisterCarrier`, `CarrierLine`; `leaction.KindArgv` | `exec` and `job run` carry a program the line names. `exec` also registers its whole area heavy | Extends: a carrier area may register a judge of the program it carries, and its line is then heavy only when the judge names that program. `exec` registers one and drops its area-wide heavy registration. Its carrier mark stays for the pipe guard |
| Self-admitting area mark | `internal/le/le/root/leroot.go` `RegisterAdmitted`, `Admits` | An area whose answer takes its own slot: the suite runners | Extends: it becomes the area-wide form of "admits itself", and a row form is added beside it |
| No-slot mark | `internal/le/le/root/leroot.go` `RegisterExpensive`, `Expensive` | `ai hooks unit`: judged by its whole output, "takes no slot" | Replaces: the row declares `Heavy` and the no-slot reason. One declaration of "no slot", which carries its reason |
| Whole-tree gate | `internal/le/le/action/leaction.go` `Action.WholeTree`, `Area.admit`, `SetWholeTreeAdmission` | Refuses a release check with exit 2 unless a person at a terminal started it, before the body | Extends: the dispatcher asks the same gate before it asks for a slot. Its injected-function shape is the precedent for the admission seam |
| Dispatch path | `internal/le/le/root/leroot.go` `runWithStartup`; `internal/le/le/root/dispatch.go` `dispatchLine` | Resolves, validates the pipe chain, runs the body, renders | Extends: one call before the body. No second dispatcher |
| Parse without running | `internal/le/le/action/leaction.go` `List.Accepts` | Tells whether a verb's parser takes the words | Extends: a line its own parser refuses is never queued |
| Admission library | `internal/le/job/job.go` `Admission`, `admit`, `Ticket.Release`, `Run`, `RunClaimed` | Claims, attaches or runs inside; releases with a code | Extends: a third policy beside attach on and attach off (unshared, no log) and one entry point that owns claim, parent naming, body and release |
| Nesting | `internal/le/job/job.go` `insideParent`, `childEnviron`; `internal/le/verify/engine/run.go` `nameJobParent`; `internal/le/test/stressrepro/run.go` `runClaimed` | A named, existing parent entry makes a nested admit `KindInside` | Extends: the central holder names itself the way `nameJobParent` does. The two name-and-restore sites become one function in `job` |
| Owner fields and waiter record | `internal/le/job/registry.go` `caller`, `writeOwner`; `queue` and its waiter record | Entries and waiters name session and agent | Extends: unchanged, so central holders and waiters appear in the report with no new code |
| Report | `internal/le/job/overview.go` `Overview`, `holderRow`, `slotsNotice` | Slots, holders, waiters, machine | Extends: no new field. The notice text changes |
| Registry pruning | `job.Prune`; `./le scratch clean` | Removes dead entries, logs and receipts older than its floor | Does not apply: central holders write no receipt and no log, so they add nothing to prune beyond an entry a killed process leaves |
| Census test | `internal/le/heavy_census_test.go` `TestCensusActionsDeclareHeavy` | Pins which rows are heavy | Extends: a sibling census pins which heavy rows admit themselves or take no slot, and drives the dispatcher with a fake admitter |
| Raw-admission escape | `internal/le/hookruntime/bash.go` `bashRawHeavy`; `internal/le/hookruntime/rawheavy.go` `admitRawAssignment` | The hook lets a main-thread raw command through when it states a reason | Does not apply: decision 4. It keeps one reader |
| Heavy-tool definition | `internal/le/hookruntime/subagent_heavy.go` `heavyTool`; `internal/le/hookruntime/rawheavy.go` `lintRunsNoAnalysis` | Judges a program's words by name: the Go executing verbs, the linters, the formatters | Extends by moving: the judgment over program words moves to a leaf package the hook and `exec` both import. The hook keeps reading a shell line down to those words. No second copy |
| Runner without a checkout | `internal/le/test/harnesstool/harnesstool.go` `admitRun`, `lepath.ErrNoCheckout` | Runs unadmitted | Extends: the central point follows the same rule |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A typed line, `./le <area> <words>`, reaching `leroot.DispatchWithStartup`. Format: argv words.

### Transformation Path
1. `dispatchLine` refuses a retired spelling, answers a namespace, answers a trailing help word, and at a terminal asks for a missing required value. None of these reaches the admission point.
2. `runWithStartup` splits and validates the pipe chain. A refused chain answers 2 and reaches no admission.
3. The slot reading answers one of four for the area and its words: light, admitted centrally under a command name, admits itself, or no slot with its reason. It reads the same resolution `HeavyCommand` reads.
4. For "admitted centrally" only: the line is parsed without running (`List.Accepts`) when the area has a grammar. A line the parser refuses skips admission and goes to the handler, which prints the refusal as today.
5. The whole-tree gate is asked for the row. A refusal answers 2 here, before any slot.
6. The injected admitter is called with the command name, the typed line and the body. It answers the body's code, or a refusal code when no slot could be asked for.
7. Inside the admitter (`internal/le/job`): no checkout runs the body unadmitted; a named parent runs the body inside it; otherwise it waits for a slot, claims, names its own entry as parent in the process environment, runs the body, restores the environment and releases with the body's code.
8. `runWithStartup` renders the payload and answers the code, as today.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Dispatcher to admission library | One injected function, registered by `internal/le/job` from `init()` | No |
| Holder to registry | Entry file under `tmp/.ze-jobs/`, written under the registry lock (`take`) | No |
| Holder to its children | `ze.run.job` in the process environment, inherited by every process the body starts | No |
| Holder to other sessions | The entry's fields, read by `scanAndClaim` and `Overview` | No |

### Changed-Wire Census

Two wires change. The registry entry gains a holder that writes an empty
`LOG` and a `TREE` of `unknown`. The parent variable gains one writer.

**Search evidence:** scope `internal/le`, `cmd`, the `le` launcher script,
and every `*.sh`, `*.py`, `*.yml`, `*.yaml` and `*.ci` outside `tmp/`,
`vendor/` and `bin/`. Terms: `field(body, "LOG")`, `"TREE"`, `"STATE"`,
`"KEY"`, `held.log`, `held.tree`, `LABEL=`, `STATE=running`, `.ze-jobs`,
`ZE_RUN_JOB`, `ParentKey`, `ParentVariable`, `nameJobParent`. Positive
control: the search finds `take` writing `STATE=running` and `readEntry`
reading `LOG`. Outside Go the only hits are the two `.ci` files, which name
the variable in comments; the launcher script names neither.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Registry entry | writer | `internal/le/job/registry.go` `take` | writes every field through `writeField`; always creates the log and writes `LOG` | Change: an unshared claim writes an empty `LOG`, creates no log file, and writes `TREE=unknown` |
| Registry entry | writer | package `internal/le/test/qemu`, `guestJobParentEntry` | writes `LABEL=qemu-held-by-host` only | Preserve: read only by `insideParent`, which needs `LABEL` |
| Registry entry | reader | `internal/le/job/registry.go` `readEntry`, `registryLog` | an empty or foreign `LOG` reads as "names no log" | Preserve: already the handled state |
| Registry entry | reader | `internal/le/job/registry.go` `shares` | a tree of `Unknown`, or a mismatch, shares nothing | Preserve: this is what makes an unshared entry unattachable |
| Registry entry | reader | `internal/le/job/registry.go` `reportBusy` | prints the log clause only when the entry names a log | Preserve: a waiter behind a central holder sees label, PID, age and entry |
| Registry entry | reader | `internal/le/job/overview.go` `holderRow` | reads label, PID, started, owner, command | Preserve |
| Registry entry | reader | `internal/le/job/job.go` `insideParent` | reads `LABEL` | Preserve |
| Registry entry | reader | `internal/le/job/job.go` `Ticket.Release` | removes the entry and the path the ticket names as its log | Change: releases a ticket with no log without touching a path |
| Registry entry | reader | `internal/le/go/lint/actions.go`, the admitting action | opens the log only when the ticket names one | Preserve |
| Registry entry | reader | the pruner, the attach follower and the holders listing in package `internal/le/job` | not read in this session | A-6: read each before the entry change lands |
| Registry entry | reader | an `le` built before this change, in another session | same readers as above, older code | Preserve: R-8 |
| Parent variable | writer | `childEnviron`; `nameJobParent`; the stress-repro `runClaimed`; three guest sites in package `internal/le/test/qemu` | each names an entry path under `ze.run.job` or `ZE_RUN_JOB` | Change: one function in `job` names and restores; `nameJobParent` and the stress-repro site call it. The qemu sites keep their guest spelling |
| Parent variable | reader | `internal/le/job/job.go` `insideParent` | reads `env.Get(ParentKey)` and the file it names | Preserve |

### Integration Points
- `leroot.runWithStartup` - calls the slot reading and the admitter before the body.
- `leaction.Action` - two new row declarations.
- `leroot.RegisterAdmitted` - kept as the area-wide "admits itself".
- `job.Admission` - the unshared policy and the entry point the seam is filled with.
- `job.Overview` - the notice text.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## The design

### The declarations

Proposed names, in words. The editing agent may choose better ones; the
meaning is fixed.

| Declaration | Form | Meaning | Refused at startup when |
|-------------|------|---------|-------------------------|
| "Admits itself" | A boolean on the action row | The body takes its own slot through `internal/le/job`, under its own label, attach policy and log. The central point does nothing | The row is light, or also declares "no slot" |
| "Admits itself", area-wide | `leroot.RegisterAdmitted(name)`, which exists | Every line of the area that runs something admits itself. For an area with no rows, and for an area whose rows all do | - |
| "No slot" | A text on the action row: the reason | Heavy for a subagent (the hook still hands the line back) and takes no slot. The reason is shown on the action's help page and in the manifest | The row is light, or the row also admits itself |
| "No slot", by name | A registration beside `RegisterHeavy`, taking a name and a reason. The name is an area, or an area, a verb and a selection, as `RegisterHeavy` reads it | The same, for a pass-through area and for one selection of a row | The reason is empty, or the name is not heavy |
| Nothing | - | A heavy line is admitted centrally | - |

The default is the safe side. A heavy action whose author declares nothing
takes a slot.

### The slot reading

One function in `leroot`, beside `HeavyCommand` and reading the same
`resolveLine` result. It answers light, central with the command name, admits
itself, or no slot with the reason.

A sweep line names several verbs. It is central when any heavy verb it names
is central, so a self-admitting verb named first cannot carry a central verb
past admission. The self-admitting verb then runs inside the line's slot.

### The seam

`leroot` holds one injected admitter. `internal/le/job` registers it from
`init()`. A second registration panics at init. A test replaces it through a
restore-returning setter, as `SetWholeTreeAdmission` does.

| Input | Meaning |
|-------|---------|
| Command name | What `HeavyCommand` answers: `test functional gating`, `chaos run`, `repo check generated` |
| Typed line | `le` and the words as typed, for the entry's command field, so the report shows which suite or scope |
| Body | Runs the action and answers its exit code |

It answers the exit code to return. When no admitter is registered and the
line is central, the dispatcher refuses with exit 2 and says the binary was
built without its job package. It never runs the body unadmitted.

### Where the admission point sits, and in what order

In `runWithStartup`, immediately before the body: before the handler on the
ordinary path, and between the preparation and the startup on the
`PrepareArgs` path.

| Order | Step | Why here |
|-------|------|----------|
| 1 | Help, namespace, retired spelling, prompt for a missing value | `dispatchLine` answers these first. A prompt at a terminal never holds a slot |
| 2 | Pipe chain validation | A refused chain answers 2 without queueing |
| 3 | Slot reading | Light, self-admitting and no-slot lines go straight to the body |
| 4 | Parse without running | A line its parser refuses goes to the handler unadmitted, which refuses it. A refusal the body itself makes after parsing (a missing required keyword) is made inside the slot and releases at once |
| 5 | Whole-tree gate | "before any admission, slot, worktree or container" (`Action.WholeTree`). `leaction` exposes the gate `Area.admit` uses, for a line and a table. The body asks it again, and in one process the answer is the same |
| 6 | Admit, body, release | - |
| 7 | Render | The slot is free before the payload is rendered |

Of the three `WholeTree` rows today, `verify worktree` and `verify current`
admit themselves, so only `verify evidence release-candidate` reaches step 5.

### What the central admitter does

| Situation | Behavior |
|-----------|----------|
| No checkout (`lepath.ErrNoCheckout`) | Runs the body unadmitted and prints nothing, as a suite runner does: no registry exists to share |
| The environment's admission policy is out of range | Refuses with exit 2 and the existing `Validate` message. The body does not run |
| A parent entry is named and exists | `KindInside`: runs the body, holds nothing. Covers `job run command ./le <heavy>`, a stage of a verify, a suite under gating |
| A parent entry is named and gone | Admits anew, as `insideParent` already answers |
| A slot is free | Claims |
| Every slot is held | Writes its waiter record, prints the waiting banner on stderr at the existing interval, polls. SIGINT or SIGTERM during the wait ends it: the record is withdrawn, the body does not run, and the code is 130 |
| Claimed | Names its entry as the parent in the process environment, runs the body, restores the previous value, releases with the body's code |
| The body panics | The release and the restore run first, with the cannot-start code, and the panic continues |
| The body ends the process without returning | The entry stays. Its PID is gone, so every scan and the report ignore it at once, and `./le scratch clean` removes the file |

The signal root exists only for the wait. It is removed before the body
starts, so each body keeps its own signal handling.

### The policy of a central claim

| Property | Value | Why |
|----------|-------|-----|
| Attach, as asker | Off | The action never agreed that another run's verdict is its own. A second identical build must build |
| Attach, as holder | Impossible | The entry's `TREE` is `unknown`, which `shares` matches against nothing |
| Fingerprint | Skipped | See below |
| Log | None: an empty `LOG` field and no log file | The owner accepted no progress log. An empty log file would make every waiter print "has not grown for N s" and, past the quiet window, "ask its owner" about a healthy holder |
| Receipt | None | Only a follower reads a receipt, and nothing follows an unshared holder |
| Duration row | Written | "previous run took" stays true per label |
| Label | The command name with each space as a hyphen: `test-functional-gating`, `chaos-run`. Command names hold letters, digits, hyphens and spaces, so the result is a valid label | The report shows what holds the slot |
| Command field | The typed line | The report's second line shows the keywords |
| Owner fields | Session and agent, as today (`caller`) | Unchanged |

**The fingerprint can be skipped, decided from the code.** `admit` calls
`InputHash` for every asker, three git calls. The tree has three uses:
`shares` compares it for an asker that may attach; `take` writes it so a
later asker can attach to this holder; the ticket and the report carry it.
An unshared claim needs none: it never attaches, and nothing may attach to
it. Skipping it must write the `Unknown` sentinel, never an empty or stale
value, because `shares` treats `Unknown` as matching nothing. `MayAttach`
alone cannot carry this: a caller that sets it to false today still
fingerprints so that others can share its run. So the unshared policy is a
separate, named setting of `Admission`.

**What a waiter sees.** The banner: the holder's label, PID, elapsed time
and entry path. `./le job report`: label, PID, liveness, age, session, agent,
the typed line, the entry. Neither shows a stage.

### Nesting: one slot per tree, no deadlock, no self-attach

| Rule | Where it comes from |
|------|---------------------|
| A central holder names its own entry under `ze.run.job` for the length of its body | `nameJobParent` does this for verify; `env.Set` writes the process environment and removes every other spelling |
| Every `le` the body starts, at any depth, finds that entry and is `KindInside` | `insideParent` |
| A self-admitting action inside a central holder runs in process and holds nothing | `admitRun` on `KindInside`; `Admission.Run` on `KindInside` |
| A central line inside a self-admitting holder holds nothing | `Run` and `RunClaimed` hand the entry to the child (`childEnviron`); verify and stress-repro name themselves |
| No path attaches to its own parent | The parent is found through the environment before any scan, and an unshared entry matches no asker |

A deadlock needs a child that asks for a slot while its parent holds the last
one. That can only happen when the child does not see the parent: a body that
builds a child's environment from nothing. R-1 and AC-5 cover it.

### Each class of heavy action

| Class or action | Handling | Why |
|-----------------|----------|-----|
| `test unit <group>`, `test integration <lab>`, `test deployment`, `test fuzz run`, `test netlab`, `test scale`, `test fail-syscall`, `test local-data-coverage`, `test bgp`, `test ospf` | Central | Take no slot today. A Docker lab holds one slot for its whole run (decision 2) |
| `test functional gating` | Central | The parent holds one slot and names itself. Each child suite is a self-admitting runner that finds the parent and runs inside |
| `rfc record`, `rfc check`, `rfc discriminate`, `test redwalk check` | Central | Their children are `le` processes that find the parent |
| `verify deps`, `verify evidence release-candidate` | Central | Same. The whole-tree gate of `verify evidence` is asked first |
| `feature run`, `build ...`, `perf ...`, `chaos run`, `chaos selftest`, `repo compiles`, `repo bootstraps`, `repo codeql check`, `go staticcheck check`, `go vet-platforms`, `go module`, `go stdpatch`, `site build`, `site config-tree`, `site talk`, `site terminal-demo build`, `render` and `pty`, `doc yang-contract doc-drift`, `weekly report`, `setup install`, `tools`, `proto`, `claude-server`, `web`, `appliance-modules`, `setup docker host build` and `start`, `setup docker kernel install` and `apparmor` | Central | Declared heavy, admitted nowhere today. `site talk` refreshes one talk and returns; `terminal-demo pty` runs in the renderer container, where no checkout means no admission |
| `test qemu` verbs other than `run` | Central | One slot for the whole guest run. Their in-process builds (`qemu-build-le`, `qemu-build-ze`, `mobike-build-<name>`) find the parent and run inside |
| `repo check generated` | Central | The line is heavy by its registered selection, so the dispatcher admits it. Its in-process call of `rfc.Answer` then runs inside the slot. This closes the bypass |
| `job run` | Admits itself | It is the admission |
| `test unit pkg` | Admits itself | Quiet run with its own log and report |
| `go lint run` | Admits itself | Its own label, shared verdicts and log |
| `verify current`, `worktree`, `changed` | Admits itself | Shared verdicts, a log followers replay, and the parent naming of its stages |
| Suite runners; `test stress-repro` | Admits itself, area-wide (`RegisterAdmitted`, already declared) | The log must grow while the slot is held |
| `test qemu run` | Admits itself | Holds `qemu-run` and hands the guest a stand-in parent |
| `test attribution report` | Admits itself | One slot per re-run today. Unchanged; whether it should hold one slot for its whole report is not part of this spec |
| `exec` of a heavy tool: `exec go test`, `exec golangci-lint run`, a `golangci-lint` or `staticcheck` shim | Admits itself | Decision 3 and C-1. Label `exec-` and the tool's name. Inside a parent's slot (`./le go lint run` starting the linter) it holds nothing |
| `exec` of an `le` line: `exec ./le test unit all` | Not admitted by `exec` | The carried `le` admits its own line. The hook still hands it back from a subagent |
| `exec` of any other program: `exec gopls`, `exec go version`, the `gopls` shim | Light: no slot, no registry access, the process is replaced | Decision 3. Byte for byte today's behavior |
| `ai hooks unit` | No slot: "an in-process selftest of the hooks, single process, seconds" | Declared today as taking no slot (`RegisterExpensive`) |
| `test integration clean` | No slot: "removes containers and adds no load; queueing it protects nothing" | Heavy only because it touches the shared Docker daemon |
| `setup docker host stop` | No slot: "stops the Docker host and adds no load" | Same |

### `exec`, by the program it carries

**One definition, in a leaf package.** The judgment over program words
moves out of `heavyTool` into a new package, `internal/le/heavytool`, as one
function (proposed name `Name`): given a program's words from the program
on, it answers the name of the heavy tool they start, or nothing. It holds
the Go verbs, the linter with `lintRunsNoAnalysis`, and the formatter and
checker names. It imports the standard library only.

| Why a new leaf package | Evidence |
|------------------------|----------|
| `exec` should not import `hookruntime` | `hookruntime` imports about thirty `le` packages (`verify`, `go/lint`, `rfc`, `commit`, `session`). `internal/le/setup` imports `exec`, so any path from those to `setup` would be a cycle, and a launcher that only replaces its process would link the whole hook |
| `leroot` is not its home | `leroot` is the registration adapter and names no program. Both sides could import it, but tool names there are a central list in the dispatcher |
| Both can import a leaf | `exec` imports `crashlog`, `textbuf`, `stdpatch/gen`, `lepath`, `scratch` and `leroot`; `hookruntime` imports `leroot`. A package that imports neither closes no cycle |

**What is shared and what stays in the hook.**

| Part | Where | Why |
|------|-------|-----|
| Is this program, with these words, a heavy tool | `internal/le/heavytool` | The one definition. `heavyTool` and `exec` both call it |
| Reading a shell line down to program words: the parse tree, segments, assignments, launchers (`programTokens`, `everyCommand`, `commandSegments`) | `hookruntime` | `exec` receives an argv. It has no shell line to read |
| Is this `le` line heavy (`leWords`, `leroot.HeavyCommand`) | `hookruntime`, unchanged | It reads the registry, which the leaf package must not import |
| The main thread's raw-job guard (`rawHeavyWords`) and its advice | `hookruntime` | It refuses a narrower set on purpose, because it advises a short form. It calls the shared `lintRunsNoAnalysis` from the leaf package |

**How the hook judges `./le exec <program>`.** `leroot` gains one
registration beside `RegisterCarrier`: a carrier area registers a judge of
the program words it carries. `HeavyCommand` answers heavy for such an
area's line only when the judge names the program. `exec` registers a judge
that asks the leaf package, and for a carried `le` program asks `HeavyLine`
of the carried words. `exec` no longer registers its whole area heavy. The
hook needs no branch for `exec` and names no area: `heavyTool` still asks
`HeavyCommand`, and the census and the slot reading read the same answer.

| A subagent types | Before | After |
|------------------|--------|-------|
| `./le exec gopls symbols x.go` | handed back | runs |
| `./le exec go version` | handed back | runs |
| `./le exec go test ./x/` | handed back | handed back |
| `./le exec golangci-lint run` | handed back | handed back |
| `./le exec golangci-lint config verify` | handed back | runs |
| `./le exec ./le test unit all` | handed back | handed back |

The slot reading answers "admits itself" for a heavy `exec` line. The
area-wide `RegisterAdmitted` is not used for it, because that mark makes
every line of the area heavy (`heavyByName`); the judge registration carries
the self-admission mark for the lines it names heavy.

**What `exec` does.**

| Case | Behavior |
|------|----------|
| The program is not a heavy tool | Replaces the process, as today. No registry access, no new code on the path but the one in-memory judgment |
| Heavy tool, no checkout | Replaces the process, as today |
| Heavy tool, a parent entry is named and exists | Replaces the process, as today. The tree holds the parent's slot |
| Heavy tool, no parent | Waits for a slot as a central line does (waiter record, banner on stderr, 130 on a signal), claims with the unshared policy, runs the program as a child, releases with its code, and exits with it |

**What changes in the child case, and only there.**

| Aspect | Process replaced (today, and every other case) | Child (heavy tool, slot claimed) |
|--------|------------------------------------------------|----------------------------------|
| Process | The program has `le`'s PID | `le` stays as the parent; the program is its child |
| stdin, stdout, stderr | The program's own | The same file descriptors, inherited. No pipe, no tee, no log, so a terminal stays a terminal and color stays |
| Signals | Delivered to the program | SIGINT and SIGTERM sent to `le` are forwarded to the child (`forwardSignals`). At a terminal the foreground group gets Ctrl-C too, so the child can receive it twice, as under `./le job run` |
| Exit | The program's status, a signal death included | The program's exit code. A death by signal is answered as the code `gaterun.ExitCode` gives it, not as a signal death of `le` |
| Environment | `programEnvironment` | The same, plus the claimed entry named as the job parent, so an `le` the tool starts runs inside |

### What this leaves for the resource-class spec

`spec-test-verdicts-independent-of-load.md` plans "the action declares its
resource class (peak memory, disk written)". The row declarations and the
by-name registration this spec adds are where that class attaches, and the
admitter's inputs are where it is passed. This spec adds no class and weighs
nothing: one slot per admitted line.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each child `le` of `test functional gating`, `rfc record`, `test redwalk`, `verify deps` and `verify evidence` inherits the parent's environment | `Execute` starts `le` under `timeout` with an environment built by `set.Environment`; the RFC observer hands its child an `environ` built by its caller. Neither builder was read | The child takes a second slot, and deadlocks at the last one | AC-5's test, with one slot, per tree | unvalidated |
| A-2 | No centrally admitted body ends the process without returning | `os.Exit` appears in `internal/le` only in the stdpatch goroot program, the write-edit hook and two interop lab programs; `exec` is handled separately | A slot's entry is left for `scratch clean`, and no duration row is written | grep for `os.Exit`, `syscall.Exec` and `crashlog.Exec` under each central area in phase 1 | unvalidated |
| A-3 | No heavy row uses `PrepareArgs` | grep: `commit`, `test weakened`, `spec`, `rfc approve`, `deps` | The census test would run a real prepared body when the wiring breaks | the census test asserts it and fails by name | unvalidated |
| A-4 | `setup docker host start` returns once the profile runs | `runStart` returns the outcome of `start`, which takes a lock and builds; its end was not read | It would hold a slot for the VM's lifetime, and needs "no slot" | read `start` to its end in phase 1 | unvalidated |
| A-5 | `test integration clean` is a row the no-slot declaration can reach | The area builds its rows from a table of declared actions and registers the whole area heavy; the `clean` entry was not read | The no-slot declaration needs a field on that table's entries | read the declared-actions table in phase 1 | unvalidated |
| A-6 | The pruner, the attach follower and the holders listing read an entry with an empty `LOG` and `TREE=unknown` without error | `readEntry` and `registryLog` already return an empty log for an absent field | A prune or a report fails on a central entry | read the three files; a unit test per reader | unvalidated |
| A-7 | No test outside `internal/le/job` dispatches a central line through `leroot.Run` or `Dispatch` and expects no admission | 18 test files call them; their lines were not read | Unit tests would claim real slots in the checkout | phase 1 lists each call and installs the fake where the line is central | unvalidated |
| A-8 | Nothing but the hook's two guards reads `leroot.Expensive` | `RegisterExpensive`'s comment | Replacing it breaks a reader | grep in phase 1 | unvalidated |
| A-9 | `terminal-demo pty` runs only where no checkout exists, or under `terminal-demo render` | Its `Why`: "the renderer container runs it" | Typed on the host it takes a slot, which is correct for a recorder | read the terminal-demo entrypoint in phase 1 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Deadlock at the last slot: a child that does not see its parent asks for a slot the parent holds | A waiter whose session and command are a child of the only holder | AC-5 per tree under one slot. The holder names itself in the process environment, so only a body that drops the environment can break it |
| R-2 | Self-attach: a child follows its own parent's log | Two processes of one tree, neither progressing | Central entries are unshared in both directions. Self-admitting actions are unchanged |
| R-3 | A long lab or guest starves other sessions | Waiters behind one holder for an hour in the report | Accepted by decision 2. The report names the holder and its session. Weighing is the resource-class spec |
| R-4 | A queue forms behind an action that waits for a person | A holder with an age of hours and an idle machine | Prompts for a missing value run before admission. Bodies that prompt mid-run are found in phase 1 and declared "no slot" with the reason, or prompt first |
| R-5 | Double release | A duration row written twice | The admitter owns the one release; `Ticket.Release` is a no-op on a second call |
| R-6 | A panic in the body leaks a slot | A holder row whose PID is gone | The release runs before the panic continues. A process that dies anyway is ignored by every scan |
| R-7 | A signal during the wait leaves a waiter record | A waiter row whose process is gone | `queue` withdraws the record on every way out. The report ignores a dead waiter |
| R-8 | An `le` built before this change runs beside one built after | An old session's heavy line not in the report | The old binary takes no slot, as today, and reads the new entries with the readers the census lists as preserved. It ends when that session's next build lands |
| R-9 | A defect at the admission point stalls every session's `./le`, hooks included | Every hook call slow or refused | The light path is an in-memory reading with no disk access, pinned by a test that fails when a light line reaches the admitter or the registry |
| R-10 | A raw `staticcheck` or `golangci-lint run` typed at a terminal now queues through its shim | A person's lint waiting behind a test run | Intended by decision 3. The banner names the holder. `./le go lint run` holds the slot itself, so its linter child runs inside |
| R-13 | The shared definition and the hook drift apart again | A program the hook hands back that `exec` runs unadmitted | AC-21: one package holds the names, and a test fails when another file under `internal/le` lists them |
| R-14 | A caller relies on `exec` having the program's PID, or on a signal death, in the child case | A wrapper that signals the PID and sees `le` | Only heavy tools with no parent take the child path. `le` forwards SIGINT and SIGTERM. The table in the design states each difference |
| R-11 | A light line calls a heavy action's body in process | Heavy work with no holder | Known Limitations: the declaration census owns it |
| R-12 | Tests that dispatch a central line claim real slots | A unit test waiting on the machine's queue | A-7; the fake admitter |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every session's `./le` goes through this point, each hook call included. A defect that waits or refuses on a light line stalls every session at once. A defect that admits nothing leaves today's behavior |
| How is it reverted? | One commit: remove the admitter's registration and the dispatcher call. With no admitter a central line refuses, so the two go together |
| Who else touches this path? | Every area's rows gain no edit except the ones declaring "admits itself" or "no slot". `spec-test-verdicts-independent-of-load.md` edits the admission library after this |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A central heavy line typed at `le`, through `leroot.Run` | → | the admission call in `runWithStartup` | `TestCentralLineAsksAdmitterBeforeBody` |
| Every registered heavy line | → | the slot reading and the dispatcher | `TestCensusHeavyActionsAreAdmitted` |
| The built `le`, a heavy line typed while the only slot is held | → | the admitter `job` registers | `test/ui/le-job-admits-heavy-action.ci` |
| `le exec <heavy tool>` typed while the only slot is held | → | `exec`'s own admission, through the shared definition | `test/ui/le-job-admits-heavy-action.ci`, exec step |
| A subagent's `./le exec <program>` through the pretool hook | → | the carrier judge behind `HeavyCommand` | `TestSubagentExecIsJudgedByItsProgram` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A heavy line of each class marked Central in the table, with a free slot | Its body runs holding exactly one registry entry, whose label is the command name with hyphens and whose command field is the typed line | A line inside a parent's slot holds none: the tree already holds one |
| AC-2 | `./le job report` while such a line runs | A holder row shows that label, the session and agent that typed it, its age and the typed line | An unreadable identity shows as unknown, as today: admission protects the machine and the owner fields only describe |
| AC-3 | A self-admitting action (each row of that class) with a free slot | It holds exactly one entry, under the label it uses today, with its log and its attach policy unchanged | Its nested builds and stages run inside, as today |
| AC-4 | A self-admitting action started inside a central holder, and a central line started inside a self-admitting holder | The inner one runs and the registry holds one entry for the tree | None: two entries for one tree is the defect |
| AC-5 | `test functional gating` with a child suite, and `rfc record` with a functional child, each with one slot in total | The run completes, and the registry never holds a waiter that belongs to the tree | A waiter from another session: that is the queue working |
| AC-6 | `le exec <heavy tool>` (`go test`, `golangci-lint run`) with a free slot and no parent | The tool runs as a child on the caller's own stdin, stdout and stderr, the registry holds one entry labeled `exec-<tool>`, and `le` answers the tool's exit code | Inside a parent's slot, and with no checkout, `exec` replaces the process as today: nothing is left to hold |
| AC-7 | `le exec gopls`, `le exec go version`, `le exec golangci-lint config verify`, and a `gopls` shim, each with every slot held | The program replaces the `le` process at once, and the registry directory is not read or written | None is admitted: decision 3. A program outside the definition takes no slot, the accepted gap |
| AC-20 | The hook judging a subagent's `./le exec <program>` line | It hands the line back exactly when the shared definition names the carried program, or the carried program is a heavy `le` line; `./le exec gopls symbols x.go` runs | The main thread's lines are not judged by this check, as today |
| AC-21 | The heavy-tool names and verbs | They exist in one package. `heavyTool`, `rawHeavyWords` and `exec` call it, and no other file under `internal/le` lists them | The hook's shell-line reading and its `le`-line branch stay in `hookruntime` |
| AC-8 | A listing, a help word, a namespace, a light action, each with every slot held | It answers at once, and the registry directory is not read or written | None |
| AC-9 | Each action declared "no slot" | It runs at once with every slot held, and its help page and the manifest state the reason | None: a no-slot declaration with no reason stops `le` at startup |
| AC-10 | A central line while every slot is held | It waits, its waiter record appears in `./le job report` with its session, and it starts when a slot frees | A waiter ended by SIGINT or SIGTERM: the record goes, the body never runs, the code is 130 |
| AC-11 | A central action whose body answers a nonzero code | `le` exits with that code and the slot is free | A panic in the body: the slot is released and the panic is reported as before |
| AC-12 | `./le job run label x command ./le <central heavy line>` | The registry holds one entry, labeled `x` | None |
| AC-13 | A central line run where no checkout exists | The body runs and answers its code | None is admitted there: no registry exists to share |
| AC-14 | A central line of a `WholeTree` row, typed with no terminal, with every slot held | It is refused with exit 2 at once, with the whole-tree message | None: a refusal after a wait is the defect |
| AC-15 | `TestCensusHeavyActionsAreAdmitted` over the registered tables | Each heavy line is admitted centrally, or is listed in the test as self-admitting, or as no-slot with its reason. The test fails naming the line when the dispatcher runs a central line's body without asking the admitter, and when a line's handling differs from the pinned one | The test's pinned lists: a new self-admitting or no-slot action is a decision someone writes down |
| AC-16 | A raw command under `ZE_ADMIT_RAW="<reason>"` | The hook lets it through as today, it holds no slot, and the report shows it only in the machine readings | That is the escape's purpose |
| AC-17 | `docs/contributing/running-commands.md`, "Waiting for another session's job" | It states: "The report states readings and no verdict, and admission still queues whatever is started." and the paragraph above it no longer lists a Docker interop lab among work that takes no slot | Raw-escape commands and other checkouts stay listed: they take no slot here |
| AC-18 | A central line a second session types while an identical one runs | Both run their own body, one after the other when one slot exists | Self-admitting actions still attach: they declared that sharing |
| AC-19 | Each row in "Owned journal rows" | It is re-judged at the producer. The open one (2026-10-24) is closed with the cause found, or shown fixed by the removal of the holder-breaking code it names | The two rows already marked fixed stay as they are |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSlotReadingOfEachDeclaration` | `internal/le/le/root/slot_test.go` | light, central, admits itself, no slot with reason, for a fixture table (AC-1, AC-9) | |
| `TestSlotReadingOfASweepIsCentralWhenAnyVerbIs` | `internal/le/le/root/slot_test.go` | the sweep rule | |
| `TestCentralLineAsksAdmitterBeforeBody` | `internal/le/le/root/slot_test.go` | order: admitter, then body; the code is the body's (AC-11) | |
| `TestLightLineNeverReachesAdmitter` | `internal/le/le/root/slot_test.go` | AC-8, R-9 | |
| `TestRefusedLineIsNotQueued` | `internal/le/le/root/slot_test.go` | a parse refusal and a pipe refusal skip the admitter | |
| `TestWholeTreeRefusesBeforeAdmitter` | `internal/le/le/root/slot_test.go` | AC-14 | |
| `TestCentralLineRefusesWithNoAdmitter` | `internal/le/le/root/slot_test.go` | exit 2, body not run | |
| `TestStartupRefusesAnEmptyNoSlotReason` | `internal/le/le/root/slot_test.go` | AC-9 | |
| `TestCensusHeavyActionsAreAdmitted` | `internal/le/admission_census_test.go` | AC-15, with a fake admitter that records and refuses, and a recording handler | |
| `TestUnsharedClaimWritesNoLogAndUnknownTree` | `internal/le/job/central_test.go` | the entry's fields | |
| `TestUnsharedClaimSkipsTheFingerprint` | `internal/le/job/central_test.go` | no git call on the unshared path | |
| `TestNothingAttachesToAnUnsharedHolder` | `internal/le/job/central_test.go` | AC-18 | |
| `TestCentralHolderNamesItselfForItsBody` | `internal/le/job/central_test.go` | a nested admit in the body is `KindInside`; the value is restored after (AC-4) | |
| `TestCentralReleaseOnPanic` | `internal/le/job/central_test.go` | R-6 | |
| `TestCentralWaitEndsOnSignal` | `internal/le/job/central_test.go` | AC-10's survivor | |
| `TestCentralRunsUnadmittedWithNoCheckout` | `internal/le/job/central_test.go` | AC-13 | |
| `TestCentralLabelIsTheCommandName` | `internal/le/job/central_test.go` | AC-1 | |
| `TestBusyBannerOmitsLogForALoglessHolder` | `internal/le/job/central_test.go` | what a waiter sees | |
| `TestReportShowsACentralHolder` | `internal/le/job/overview_test.go` | AC-2 | |
| `TestNameOfEachHeavyTool` | `internal/le/heavytool/heavytool_test.go` | each verb, linter subcommand and formatter, heavy and light (AC-21) | |
| `TestExecOfAHeavyToolClaimsAndAnswersItsCode` | `internal/le/exec/exec_test.go` | AC-6 | |
| `TestExecOfAHeavyToolReplacesTheProcessInsideAParent` | `internal/le/exec/exec_test.go` | AC-6's survivor | |
| `TestExecOfALightProgramTouchesNoRegistry` | `internal/le/exec/exec_test.go` | AC-7 | |
| `TestExecChildInheritsTheCallersStreams` | `internal/le/exec/exec_test.go` | the child case keeps stdin and the terminal | |
| `TestCarrierLineIsHeavyByItsProgram` | `internal/le/le/root/slot_test.go` | the judge registration, with a fixture carrier (AC-20) | |
| `TestSubagentExecIsJudgedByItsProgram` | `internal/le/hookruntime/subagent_heavy_test.go` | AC-20, the six lines of the table | |

**Existing expectations that change.** Each one lowers what a test asserts
for some line, so each needs the verdict of an independent `ze-test-judge`
agent before it is edited (`ai/META-DECISIONS.md`, "Test changes"). The
source that makes the old expectation wrong is decision 3.

| Test | Today | After |
|------|-------|-------|
| `TestCensusActionsDeclareHeavy`, `internal/le/heavy_census_test.go`, line `exec go version` | heavy | light; `exec go test ./x` is added as heavy |
| The same test, pass-through row `exec` judged with `exec probe word` | heavy | light; the row moves to a by-program check |
| `internal/le/hookruntime/subagent_heavy_test.go`, row `./le exec go version` naming `./le exec` | handed back | runs; a row for `./le exec go test` keeps the hand-back |
| The recorded verdict table and fixture catalog of `internal/le/ai/hooks`, any row with an `le exec` line | as recorded | re-recorded for the lines whose verdict changes. Phase 3 lists the rows before the judge is asked |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | - | - | - | - |

This work adds no numeric input. `ZE_RUN_SLOTS` keeps its existing bounds and tests.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-job-admits-heavy-action` | `test/ui/le-job-admits-heavy-action.ci`, fixture `ui/le-job-admits-heavy-action` | In a scratch registry named by `ZE_REPO_ROOT` with one slot, a `job run` holder is running. A heavy `le` line typed at the built binary waits and is listed as a waiter with its label. A light line answers at once. `le exec` of a fixture program named as a heavy tool (a stub called `staticcheck`) waits too, and `le exec` of a stub under any other name runs at once. When the holder ends, each runs, answers its own code, and a duration row carries its label | |

The models are `test/ui/le-job-admits.ci` and `test/ui/le-job-report.ci`:
a scratch tree, `ZE_RUN_JOB` and the surrounding identity removed from the
driven processes, no wait on a duration. The fixture uses a heavy action
whose body ends at once in a scratch tree, and names it in its comment.

### Interop Tests (Scope: protocol)
N/A - Scope is tooling; no wire-visible behavior changes.

## Files to Modify
- `internal/le/le/action/leaction.go` - the two row declarations, their publication on `Row`, the exported whole-tree gate
- `internal/le/le/root/heavy.go` - the slot reading; the no-slot registration by name
- `internal/le/le/root/leroot.go` - the admitter seam; the call in `runWithStartup`; `RegisterExpensive` removed; startup checks of the declarations
- `internal/le/job/job.go` - the unshared policy; the central entry point; one name-and-restore function
- `internal/le/job/registry.go` - `take` and `Release` for a claim with no log
- `internal/le/job/register.go` - registers the admitter
- `internal/le/job/overview.go` - `slotsNotice`
- `internal/le/job/answer.go`, `internal/le/go/lint/actions.go`, `internal/le/verify/actions.go`, `internal/le/test/unit/pkg.go` (its row), `internal/le/test/qemu/actions.go`, `internal/le/test/attribution/actions.go` - declare "admits itself"
- `internal/le/verify/engine/run.go`, `internal/le/test/stressrepro/run.go` - call the one name-and-restore function
- `internal/le/ai/hooks/actions.go`, `internal/le/ai/hooks/register.go` - `Heavy` and the no-slot reason on the row
- `internal/le/test/integration/actions.go`, `internal/le/setup/docker/host/actions.go` - the no-slot reasons
- `internal/le/exec/exec.go`, `internal/le/exec/register.go` - the by-program judge, self-admission in the child case; the area-wide heavy registration goes
- `internal/le/hookruntime/subagent_heavy.go` - `heavyTool` calls the shared definition and keeps no list of its own
- `internal/le/hookruntime/rawheavy.go` - `lintRunsNoAnalysis` moves to the shared package; `rawHeavyWords` calls it there
- `internal/le/heavy_census_test.go` - `ai hooks unit` is heavy by its row; the `exec` rows, after the judge's verdict
- `internal/le/hookruntime/subagent_heavy_test.go` - the `exec` rows, after the judge's verdict
- `docs/contributing/running-commands.md` - "One verify at a time", "Waiting for another session's job", "The long form, for any other command"
- `docs/architecture/core-design.md` - the admission paragraph the `// Design:` headers of the job and dispatcher files name
- `docs/guide/developer-setup.md` - the shims, named by the shim file's `// Design:` header
- `ai/rules/commands.md` and its points - what a typed heavy `le` line does

## Files to Create
- `internal/le/le/root/slot_test.go`
- `internal/le/admission_census_test.go`
- `internal/le/job/central_test.go`
- `internal/le/heavytool/heavytool.go` - the shared heavy-tool definition
- `internal/le/heavytool/heavytool_test.go`
- `test/ui/le-job-admits-heavy-action.ci`
- the fixture `ui/le-job-admits-heavy-action` under `internal/test/fixture/`, beside the one for `ui/le-job-report`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | `le` is development tooling and declares no YANG |
| YANG validation constraints | N-A | same |
| YANG custom validators | N-A | same |
| CLI commands/flags | No | no new command, verb or keyword; existing lines gain admission |
| CLI grammar (keyword before value) | N-A | no grammar changes |
| Editor autocomplete | N-A | no new word |
| Functional test for new RPC/API | Yes | `test/ui/le-job-admits-heavy-action.ci` |
| Pipe completeness | N-A | no answer shape changes |
| Env var registration | No | no new variable; `ze.run.job` is registered, and `ZE_ADMIT_RAW` gains no reader |
| Doctor check for runtime dependencies | N-A | no new file path, binary or service; `ze doctor` does not cover `le` |
| Doctor code per problem class | N-A | the refusals are `le` diagnostics on stderr, not product problems |
| Prometheus counters/metrics | N-A | tooling |
| BGP family surface (new SAFI / capability / attribute) | N-A | tooling |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | tooling; no `features/<id>.md` |
| 2 | Config syntax changed? | No | - |
| 3 | CLI command added/changed? | No | no product command |
| 4 | API/RPC added/changed? | No | - |
| 5 | Plugin added/changed? | No | - |
| 6 | Has a user guide page? | Yes | `docs/guide/developer-setup.md`: a shim's tool takes a job slot when it is a heavy tool (`golangci-lint run`, `staticcheck`) and none otherwise (`gopls`). The shims themselves do not change |
| 7 | Wire format changed? | No | the registry entry is described in `running-commands.md`, row 10 |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented, changed, or newly proven? | No | - |
| 10 | Test infrastructure changed? | Yes | `docs/contributing/running-commands.md`: "One verify at a time" states the criterion (heavy by declaration, central by default, the two opt-outs) in place of the list and drops the sentence naming this spec; "Waiting for another session's job" per AC-17; "The long form" says a heavy `le` line needs no `job run` |
| 11 | Affects daemon comparison? | No | - |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`: the admission seam between the dispatcher and `internal/le/job` |
| 13 | Route metadata keys added/changed? | No | - |
| 14 | Prometheus counters added/changed? | No | - |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | - |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived at implementation with `./le spec citation anchors spec plan/post-release/engineering/spec-native-action-job-admission.md`. Known now: `running-commands.md` anchors the job, registry, overview and answer files of `internal/le/job` and the suite-runner file. `docs/architecture/testing/verify-freshness-scope.md`, declared by the verify files this spec edits, is unaffected: the edit adds the "admits itself" declaration to three verify rows and moves the parent naming into `job`, and changes nothing the page describes about what a verification judges or when its record is fresh. The phase that edits those files reads the page and confirms it |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | the sample report in "Waiting for another session's job": its last line changes with `slotsNotice` |

## Implementation Steps

Each phase is cut for one editing agent in about sixty tool calls. Each
lands as its own commit once its owed commands pass.

1. **Phase: Wiring (MANDATORY FIRST)** -- the declarations, the slot reading, the seam with no production filler, the dispatcher call in its order
   - Tests: every `slot_test.go` test, `TestCensusHeavyActionsAreAdmitted`
   - Files: the action library, the heavy reading, the dispatcher, the rows that declare "admits itself" or "no slot", `ai hooks`, the heavy census test
   - Also: validate A-2, A-3, A-4, A-5, A-7, A-8, A-9 by reading; install the fake admitter in each test A-7 finds
   - Verify: with no filler registered, the census test drives a fake. The built `le` is not changed in behavior yet: the phase ends with the seam unfilled and the dispatcher call guarded by "an admitter is registered", so the commit is safe alone. Phase 2 removes that guard
2. **Phase: the central admitter** -- the unshared policy, no log, no fingerprint, parent naming, release, the wait's signal root, no checkout; `job` registers into the seam; the guard goes
   - Tests: `central_test.go`, `TestReportShowsACentralHolder`
   - Files: the admission library, its registration and report, the verify engine's and stress-repro's parent naming
   - Also: validate A-6
   - Verify: a central line through the built `le` holds one entry
3. **Phase: `exec` by its program** -- the leaf package, the hook reading it, the carrier judge registration, `exec`'s child case
   - Tests: `TestNameOfEachHeavyTool`, the four `exec_test.go` tests, `TestCarrierLineIsHeavyByItsProgram`, `TestSubagentExecIsJudgedByItsProgram`
   - Files: `internal/le/heavytool`, the two hook files, the heavy reading, `internal/le/exec`, `docs/guide/developer-setup.md`
   - First: list every existing expectation that changes (the table in the test plan, and the recorded verdict rows) and hand the list to the main thread for an independent judge. Edit those tests only after the verdict
   - The move of the definition into the leaf package changes no verdict and can land as its own commit before the rest
4. **Phase: the functional proof** -- the `.ci`, its fixture, and the tree tests of AC-5 (A-1)
   - Tests: `le-job-admits-heavy-action`
   - Files: `test/ui/`, `internal/test/fixture/`
5. **Phase: pages, rule and owned rows** -- `running-commands.md`, `core-design.md`, `ai/rules/commands.md` points, AC-19
   - Files: the pages; `plan/journal/lifetime-ends-before-its-last-reader.md`

| Phases | Can run in parallel? |
|--------|----------------------|
| 1, then 2 | No: 2 fills the seam 1 defines and edits the dispatcher after it |
| 3 and 4 | Yes, after 2: disjoint packages. 4's exec step waits for 3. The first commit of 3, the move into the leaf package, depends on nothing and can run beside 1 |
| 5 | Yes, beside 3 and 4, after 2: it edits pages and a journal file only. The page text for `exec` waits for 3 |

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Default is admission | A heavy row with no declaration reads as central. No code path reads "unknown" as "no slot" |
| No central list | `leroot`, `leaction` and `job` name no area, verb or program. Each handling is read from a declaration made in the action's package |
| Order | Help, pipe refusal, parse refusal and the whole-tree gate each answer before the registry directory is touched |
| Light path | A light line performs no file, lock or process operation because of this change |
| One release | Exactly one release per claim on return, on panic and on a canceled wait, and none for `KindInside` |
| Unshared in both directions | The entry's tree is the `Unknown` sentinel, never empty |
| Parent naming | Set through `env.Set` after the claim, restored before the release, every other spelling removed |
| Messages | Each refusal says what is wrong, why, and the command that fixes it (`ai/rules/user-facing-errors.md`): no admitter, policy out of range, registry unreadable |
| Rule: `ai/rules/no-layering.md` | `RegisterExpensive` and the two hand-written name-and-restore sites are deleted, not kept beside their replacement |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The slot reading and the seam | `./le test unit pkg path ./internal/le/le/root/` |
| The census | `./le test unit pkg path ./internal/le/ run TestCensus` |
| The central admitter | `./le test unit pkg path ./internal/le/job/` |
| `exec` by its program, and the shared definition | `./le test unit pkg path ./internal/le/exec/`, `./internal/le/heavytool/` and `./internal/le/hookruntime/`; `./le ai hooks unit` for the recorded verdicts |
| The built binary admits | `./le test ui` with the new `.ci` |
| `RegisterExpensive` is gone | `grep -rn RegisterExpensive internal/` finds nothing |
| The page sentence | `grep -n "admission still queues whatever is" docs/contributing/running-commands.md` and no "Docker interop lab" in the paragraph above it |
| The notice | `./le job report` ends with a line that does not name Docker labs |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Label injection | The label is built from a registered command name, and for `exec` from a program base name with every byte outside the label alphabet replaced; `validLabel` still judges it |
| Entry field injection | The typed line goes into the command field through `writeField`, which makes line breaks inert, so no argv can forge a `SESSION` line |
| Parent forgery | A process that sets `ze.run.job` to an existing entry runs unadmitted. This is so today, and the variable is the nesting contract. No new trust is added: the central point reads it through the same `insideParent` |
| `exec` judged by its program | The judgment reads the program's base name, so a heavy tool copied or linked under another name takes no slot and is not handed back. That is the hook's existing gap, accepted in decision 3, and no wider than today for a raw command |
| Resource exhaustion | A central claim adds one small entry and no log or receipt. A killed holder leaves one file for `scratch clean` |
| Fail closed | An unreadable registry, an unreadable parent entry and an out-of-range policy each refuse. None runs the body unadmitted. Only "no checkout" does, by the existing rule |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| A tree deadlocks under one slot | The child lost the environment: fix the body that builds it. Never raise the slot count in the test |
| An assumption breaks | Record it in the table, add the Mistake Log row at closure, and return to DESIGN when the classes table changes |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The declaration that tells the hook a line is heavy is the same fact that tells the dispatcher to admit it. One declaration, two readers.
- Admission by default is what makes the population a criterion. An opt-out has to be written, with a reason, in the action's own package.
- A holder with an empty log file is worse than a holder with no log: the waiting banner reports its silence as a symptom.
- `le exec` is not an action with a body that returns. Any design that wraps bodies has to treat it apart.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| lifetime-ends-before-its-last-reader.md | 2026-10-24 | verify slot entry in the job registry |
| artifact-written-with-no-remover.md | 2026-11-13 | result receipts in the job registry are never removed |
| gate-verdict-depends-on-the-machine.md | 2026-11-13 | the job package's staged-executable test copies a system shell macOS kills |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One admission point in the dispatcher, by declaration | Each area calls admission (the skeleton) | The owner's decision 1. The per-area set is a hand-kept list |
| The point sits in `runWithStartup` | Inside `leaction.Area` | `Area` does not see a pass-through area or an area that dispatches by hand, so it would need a second site |
| An injected admitter that takes the body | The dispatcher holds a ticket and releases it | `job` imports `leroot`, so the dispatcher cannot name a ticket. Giving the body to the admitter puts claim, naming, release and panic handling in one function |
| The whole-tree gate is asked by the dispatcher too | Require every `WholeTree` row to admit itself | `verify evidence release-candidate` is `WholeTree` and does not admit itself |
| Unshared claim: no attach, no fingerprint, no log, no receipt | Leave attach on; write an empty log | A shared verdict the action did not agree to; a silence diagnostic about a healthy holder |
| Replace `RegisterExpensive` with a row declaration that carries the reason | Keep it beside the new one | Two declarations of "no slot", one without a reason |
| `exec` admits itself and runs a child when it claims | Admit centrally and let the dead entry free the slot | The second relies on the dead-holder path as the normal path and leaves a file per call |
| `exec` is heavy by the program it carries | Every `exec` line takes a slot; shims state the raw escape; a shim marker; shims without `exec` | The owner's decision 3. An editor's `gopls` must not hold a slot, and `ZE_ADMIT_RAW` keeps one reader |
| The heavy-tool definition moves to a leaf package | `exec` imports `hookruntime`; the list lives in `leroot` | The hook package links about thirty areas; the dispatcher names no program |
| A carrier registers a judge of its program | A branch for `exec` in the hook | The hook names no area: it asks `HeavyCommand`, and the census and the slot reading get the same answer |
| One slot per line, whole run | A lab takes every slot | The owner's decision 2 |

## Known Limitations
- A light line that calls a heavy action's body in process, or starts a heavy tool itself, takes no slot. Nothing walks process-start sites to find one. The heavy declaration census (`TestCensusActionsDeclareHeavy`) is the existing guard: it pins which rows are heavy. A gate over process-start sites is not part of the approved design; the report to the owner names it.
- A program outside the heavy-tool definition, run through `exec`, takes no slot. The owner accepted this.
- A central holder shows no stage. The owner accepted this.
- `test attribution report` keeps one slot per re-run.
- A job is one slot whatever it uses. Memory and disk are `plan/post-release/engineering/spec-test-verdicts-independent-of-load.md`.

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
- [ ] AC-1..AC-21 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests and the `ui` functional suite. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Each existing expectation phase 3 changes carries an independent judge's verdict
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A: the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
