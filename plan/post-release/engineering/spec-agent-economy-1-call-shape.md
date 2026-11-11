# Spec: agent-economy-1-call-shape

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P1 |
| Release reason | Release-independent development tooling: it changes how the Bash hook judges an agent's command and changes no shipped binary. |
| RFC gaps | - |
| Depends | - |
| Phase | 1/4 |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 1 of `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`,
which holds the baseline and the owner's decisions.

A refused or voided tool call costs an agent a whole turn, and a turn re-reads
the agent's whole context. Over three weeks 3,310 Bash calls failed. Five
causes sit in the Bash hook itself and none of them is a violation by the
agent:

1. **The hook rewrites every subagent command.** It prepends the
   session-identity exports and returns the command as changed input. Auto
   mode then holds no verdict for the changed command and voids the call
   (about 490 recorded; research agents on 2026-11-11 lost between one call in
   four and one in three). A rewritten call also voids the other calls in the
   same message, so batching is punished.
2. **Guards match text, not commands.** Four guards split a command on `|`,
   `;` and `&&` without regard for quotes, or match a name anywhere in the
   text. A `grep` whose pattern holds the words "go build" is refused as a
   build and as a piped expensive command.
3. **Two refusals teach the wrong thing.** The piped-command refusal advises a
   redirect that `ai/rules/commands.md` forbids, and the test-deletion refusal
   ends in a question an agent cannot answer.
4. **The call budget counts refused calls.** A refusal costs the agent a turn
   and a unit of its budget.
5. **A subagent is not told the rules its calls are judged by.** The start
   text says nothing of batching, of the budget, of what counts as reading a
   door, or of where tests run.

The goal: a subagent's command reaches the shell exactly as typed, a guard
fires only on a command that does what the guard names, every refusal carries
the command that fixes it, and a refusal costs no budget.

Owner decision D-4 (umbrella): the `go build` guard allows a scoped compile of
packages that produce no binary, and no longer exempts a whole-tree build.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
  → Constraint: checks register in one place and are discovered; no second list of checks appears.
- [ ] `ai/doors/le-tooling.md` - the door for the hooks
  → Constraint: the hooks run on every Bash call of every session. A hook that refuses wrongly or panics blocks every session at once, so each changed guard stays fail-closed on input it cannot parse and `Run` keeps failing open on a panic.
  → Constraint: the shared `le` is built from HEAD. An uncommitted hook edit is judged with a named build (`./le --name session ...`), and reaches other agents only when committed.
  → Constraint: a hook pattern that matched a prefix refused `git merge-base` as `git merge` (trap, 2026-09-02). The fix here is word-level judgment, the same remedy.
- [ ] `ai/rules/commands.md` - how a heavy command is run
  → Constraint: a heavy command takes `quiet` and is never wrapped in a redirect to a scratch path. The piped-command refusal must name that form.
- [ ] `ai/rules/context-economy.md` - the budget and batching rules
  → Decision: the budget stays at 100 hooked calls for a `ze-work` agent. Only what is counted changes.
- [ ] `ai/rules/user-facing-errors.md` - what a message owes
  → Constraint: every refusal says what is wrong, why, and the exact command that fixes it. A question is not a fix.
- [ ] `docs/contributing/running-commands.md`, "Session identity, and what each agent owns"
  → Constraint: `le` keys an agent's commit namespace, debt file and spec claim on `ZE_AGENT_ID`, and the session directory and scratch on `CLAUDE_CODE_SESSION_ID`. A subagent's `le` call without them acts as the main thread.

**Key insights:**
- Only programs that read the two identity variables need them. A reader command (`grep`, `sed`, `cat`, a `git` read verb) reads neither, so it needs no prefix and no rewrite.
- A command that could reach `le` must carry the identity, and the safe way to get it there without changing input is for the agent to type the line the hook gives it. A missing line is refused with the exact line, never repaired in place.
- `lineWords` already splits a shell line into words with quotes honoured and reports a line it cannot parse. The guards extend it; they do not get a second tokenizer.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/hookruntime/session.go` - `writeParentSessionPrefix` builds `export ZE_AGENT_ID=<agent>; export CLAUDE_CODE_SESSION_ID=<parent>; ` and returns the subagent's command with it prepended as `updatedInput`. It applies to every Bash call of a payload carrying an agent id, and to a fork subagent with the session id alone. It refuses with exit 2 when the agent id or session id cannot name a file. It does not look at the command.
- [ ] `internal/le/hookruntime/runtime.go` - `Run` runs every check of the action, prints the verdicts, and for `pretool-bash` calls `writeParentSessionPrefix` when no check blocked. Every check runs whether or not an earlier one refused. A panic fails open.
- [ ] `internal/le/hookruntime/budget.go` - `agentCallBudgetCheck` binds `ze-work` only, increments the agent's counter file on every hooked call before any verdict is known, and refuses past 100 unless the call names the state file.
- [ ] `internal/le/hookruntime/bash.go` - `bashLossyPipe` splits statements and pipes with regular expressions that ignore quotes; `commandSegments` (used by `bashRawHeavy`) does the same; `bashRootBuild` matches `go build` anywhere in the text and exempts `go build ./...`; `bashSystemTmp` matches `/tmp` after a quote character, so a quoted variable followed by `/tmp/` is refused; `bashTestDeletion` asks when a delete word and a test path both appear anywhere in the line, and its message ends "Allow this test deletion?". `lineWords` is the quote-aware word splitter and `shellWords` splits one segment.
- [ ] `internal/le/hookruntime/lifecycle.go` - `hookSubagentContext` emits the start text: eight constraint lines, the branch, the claimed spec, the parent session id and scratch, and the sentence "The Bash PreToolUse hook adds this prefix to the command."
- [ ] `internal/le/ai/hooks/fixtures.go` - the fixture population and the source seal `hookSourcesDigest` over the hook files; a hook source edit re-derives the seal.

**Behavior to preserve:**
- A subagent's `le` call, and any command that could start one, never runs without the agent's own identity.
- An agent id or session id that cannot name a file still refuses the call.
- The main thread's commands are never changed and never need the line.
- Every guard still refuses the command it names: an expensive command piped through a filter, a raw `go test`, a binary built into the working directory, a write under `/tmp`, a deletion of a test.
- A line a guard cannot parse is judged as today, by its raw text.
- The budget stays 100, binds `ze-work` only, and lets the handoff call through.
- `Run` fails open on a panic and on malformed input.

**Behavior to change:**
- The hook stops returning changed input.
- The four text-matching guards judge commands and their words.
- The `go build` guard follows D-4.
- The piped-command and test-deletion refusals carry the fixing command.
- A refused call does not count against the budget.
- The start text carries the identity line and the rules a subagent's calls are judged by.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Identity prefix | `internal/le/hookruntime/session.go`, `writeParentSessionPrefix` | Rewrites every subagent command to carry the identity | Replaces: the same function becomes a check that passes a reader command, passes a command already carrying the exact line, and refuses any other with the line to type. The rewrite is deleted, not kept beside the check |
| Quote-aware word splitter | `internal/le/hookruntime/bash.go`, `lineWords` | Splits a line into words, honours quotes, reports an unparsable line | Extends: one segmenter built on it yields each command of a line with its words; the four guards read that |
| Per-segment word splitter | `internal/le/hookruntime/bash.go`, `shellWords`, `commandExpensive`, `rawHeavy` | Judge one segment's leading command past launchers and assignments | Extends: they receive segments from the new segmenter instead of regular-expression splits |
| Admitted replacement text | `internal/le/hookruntime/bash.go`, `admittedCommand` | Builds the `./le job run label <label> command <argv>` form for the raw-test refusal | Extends: the piped-command refusal uses it with `quiet`, so one function writes the advised form |
| Check registry | `internal/le/hookruntime/runtime.go`, `nativeHookActions` | The authority for every native check | Extends: the identity check and the budget stay members; nothing is added outside it |
| Start text | `internal/le/hookruntime/lifecycle.go`, `hookSubagentContext` | Injects the subagent's constraints | Extends: the same text carries the identity line and five rule lines |
| Tracked-file removal | `./le commit create ... remove <path>` | Deletes a tracked file when its commit lands | Does not apply to the guard's logic; the test-deletion refusal names it as the route for a tracked test |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The harness sends the PreToolUse payload for a Bash call to `le ai hooks pretool-bash`: tool input, session id, and for a subagent its agent id and type.
- The harness sends the SubagentStart payload to `le ai hooks subagent-context`.

### Transformation Path
1. `Run` decodes the payload and builds the context.
2. Each registered check judges the command. The Bash guards obtain the line's commands and words from one segmenter.
3. The identity check judges whether the command needs the identity line and whether it carries the exact one.
4. When no check refused, the budget counts the call.
5. The hook answers with an exit code and its messages. It writes no changed input.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ hook | JSON payload in; exit code, stderr text and an optional JSON response out | Yes, `Run` |
| Agent ↔ `le` | Two environment variables on the command the agent types | Yes, `internal/le/le/path` readers |

### Changed-Wire Census

Two wires change hands.

**Search evidence:** `grep -rn "ZE_AGENT_ID\|CLAUDE_CODE_SESSION_ID"` over every `.go` file outside `vendor/` and tests, over the `le` launcher script, and over `.sh`, `.py` and `.md` files outside `plan/` and `tmp/`; and `grep -rn "updatedInput"` over `internal/le`. Positive control: `internal/le/hookruntime/session.go` appears in both, as the one writer.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Hook response `updatedInput` | writer | `internal/le/hookruntime/session.go`, `writeParentSessionPrefix` | the only `updatedInput` writer under `internal/le` | Removed: the hook writes no changed input |
| Hook response `updatedInput` | reader | the Claude Code harness | outside the repository | Preserved: an absent `updatedInput` is the harness's normal case |
| Identity variables on a command | writer | `internal/le/hookruntime/session.go` (the prefix), and `internal/le/hookruntime/lifecycle.go` `hookSubagentContext` (names the session id in text) | grep above | Changed: the agent types the line the start text and the refusal give it; the variable names and values are unchanged |
| Identity variables on a command | reader | `internal/le/le/path` (`known_session.go`, `commitsession.go`, `session.go`), `internal/le/le/root/launcher.go`, `internal/le/le/human/command.go`, `internal/le/ai/rules/session_coverage.go`, `internal/le/job/quiet.go`, `internal/le/test/functional/binaries.go`, `internal/le/spec/model.go`, `internal/test/sessionpath/sessionpath.go`, `internal/test/fixture`, and the `le` launcher script (`--name session`) | grep above | Preserved: every reader sees the same names and values |

### Integration Points
- `nativeHookActions` - the identity check and the budget remain registered checks of `pretool-bash`.
- `lepath.AgentFingerprint` - still validates that the identity can name files.
- `leroot.Admits` - still decides which `le test` areas are heavy.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the command is judged by registered checks only; nothing rewrites it on the way to the shell |
| No unintended coupling (components stay isolated) | Yes | every edit is inside `internal/le/hookruntime` and its fixtures |
| No duplicated functionality (extends existing, does not recreate) | Yes | one segmenter on `lineWords`; the regular-expression splitters are removed |
| Zero-copy preserved where applicable (refs, not copies) | Yes | not applicable: a hook judges one short command line |
| Registration over hardcoding, outbound | Yes | no new command, view or handler; checks stay in `nativeHookActions` |
| Registration over hardcoding, inbound | Yes | the reader-command set is one declared table in the hook package, and a command absent from it needs the identity line, so an unknown command is refused with its fix and never passed unjudged |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The SubagentStart payload carries the agent's id | `Payload.AgentID` exists and the PreToolUse payload fills it; not observed for SubagentStart | The start text cannot print the full line; the agent learns it from its first refusal, one wasted call per agent | Phase 1 records one real SubagentStart payload into scratch and reads the field | unvalidated |
| A-2 | Auto mode voids a call only because the hook returns changed input | The voided calls' message names the changed input | Voids continue after the rewrite is gone | After the phase 1 commit, a research agent's Bash calls are counted for the "no verdict" text | unvalidated |
| A-3 | Only `le`, test binaries and scripts that start them read the two variables | Census above | A reader command needs the identity after all | The census, re-run at implementation | unvalidated |
| A-4 | An agent reliably types a line it is given verbatim | Agents retype the hook's output today to pass auto mode | Repeated refusals | The same count as A-2, for the identity refusal | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The identity check refuses commands that need no identity, blocking every subagent | Subagents report refusals on reader commands right after the commit | The reader set is declared in one table and extended; `./le rollback` moves the build aside |
| R-2 | A command outside the reader set reaches `le` without identity because the check misjudged it as a reader | A subagent's commit lands in the main thread's namespace | The check passes without the line only when every command of the line is in the reader set; anything else, an unparsable line included, needs the line |
| R-3 | Word-level judgment misses a real violation the text match caught | A piped `go test` passes | The existing refusal tests stay; an unparsable line keeps the text match |
| R-4 | Narrowing the test-deletion guard lets a test be deleted | A test file disappears without a commit `remove` | Only a plain `rm` with literal arguments is narrowed; substitution, a variable, `find`, `xargs`, a loop or `eval` keep the line-wide judgment |
| R-5 | A fork subagent has no agent id | Its commands are refused | It keeps today's rule: session id alone, same check |
| R-6 | Changing an existing refusal test's expected text lowers what it proves | The judge refuses | The change goes to `/ze-test-judge` with the rule the old advice broke |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every subagent in every session: either its commands are refused, or its `le` calls share the main thread's commit namespace and spec claim |
| How is it reverted? | `./le rollback` for the build; a single commit revert per phase, prepared as a script for the owner |
| Who else touches this path? | `plan/post-release/engineering/spec-tooling-one-off-defects.md` (in progress) edits hook messages; child 4 of the umbrella edits `lifecycle.go` after this child |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le ai hooks pretool-bash` with a subagent payload and a reader command | → | the identity check, through `Run` | `TestSubagentReaderCommandPassesUnchanged` |
| `le ai hooks pretool-bash` with a subagent payload and an `le` command lacking the line | → | the identity check, through `Run` | `TestSubagentCommandOutsideTheReaderSetNeedsTheIdentityLine` |
| `le ai hooks subagent-context` | → | `hookSubagentContext` | `TestSubagentStartCarriesTheIdentityLine` |
| `le ai hooks pretool-bash` with a refused command from a budgeted agent | → | the budget, through `Run` | `TestRefusedCallDoesNotConsumeTheBudget` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A subagent's Bash command whose every command is in the reader set | The hook exits 0 and writes nothing to standard output, so the command runs exactly as typed | None changed: the hook returns changed input for no command at all |
| AC-2 | A subagent's Bash command holding any command outside the reader set, not starting with the exact identity line | Refused with exit 2; the message shows the full identity line for this agent, ready to paste in front of the command | A reader-only command passes without it (AC-1): it reads neither variable |
| AC-3 | The same command starting with the exact identity line | Passes unchanged | None |
| AC-4 | A subagent's command starting with an identity line naming another agent or session | Refused, naming the line this agent owns | None: an agent must not act under another identity |
| AC-5 | A main-thread Bash command | Judged as today and never asked for the line | All of them: the main thread is the identity `le` resolves by default |
| AC-6 | A SubagentStart payload | The start text holds the identity line (in full when the payload carries the agent id, else the session part and the sentence that the first refusal prints the rest), and states: read a door or a spec's source with the Read tool, whole; edit `plan/` and `ai/rules/` with Edit or Write only; put independent calls in one message; hand tests and builds to the main thread; the budget is 100 hooked calls and refused calls are not counted | The eight existing constraint lines stay |
| AC-7 | `grep -rn "go build" plan/ \| head -5`, and a `grep` whose pattern holds `go test ./x \| tail`, `/tmp/x` or `golangci-lint` | Passes the build, piped-command, raw-test and `/tmp` guards | A real `go test ./x \| tail -5`, a real raw `go test`, a real write under `/tmp` stay refused: the command does what the guard names |
| AC-8 | A line with an unclosed quote that holds a guarded name | Judged by its raw text as today | All of today's refusals on such a line: a guard that cannot see where a word ends cannot clear it |
| AC-9 | `go build ./internal/x/` or several packages, none producing a binary | Passes | `go build -o bin/<name> ./cmd/<name>` and the session `bin/` form pass as today |
| AC-10 | `go build` of one main package with no `-o`, or with no package | Refused; the message carries the caller's own package in the `-o bin/` form | None |
| AC-11 | `go build ./...` | Refused; the message names `./le repo changed scope print both` as the way to find the packages to build | None: a whole-tree build is what the no-whole-tree rule forbids |
| AC-12 | An expensive command piped through a filter | Refused; the message shows the caller's own command in the `./le job run label <label> quiet command <argv>` form and no longer advises a redirect or `tee` | The refusal itself stays |
| AC-13 | A budgeted agent's call that another check refuses | The agent's counter is unchanged | A call that passes every other check is counted, a handoff call past the budget still passes |
| AC-14 | `rm -f <scratch file> && grep x internal/a_test.go`, where the `rm` is a plain command with literal arguments | Passes: only the `rm`'s own arguments are judged | A line using substitution, a variable, `find`, `xargs`, a loop or `eval` with a delete keeps the line-wide judgment: the hook cannot see what such a line deletes |
| AC-15 | A deletion of a test path | Refused; the message says an agent cannot approve it, names `./le commit create ... remove <path>` for a tracked test, and tells the agent to report the path and the reason to the main thread otherwise. It asks no question | None |
| AC-16 | `cat "$CLAUDE_JOB_DIR"/tmp/x`, or any word whose path merely contains `/tmp/` after another directory | Passes the `/tmp` guard | A word that is `/tmp` or starts with `/tmp/`, after quotes are removed, stays refused |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSubagentReaderCommandPassesUnchanged` | `internal/le/hookruntime/session_identity_test.go` | AC-1 through `Run`: exit 0, empty standard output | |
| `TestSubagentCommandOutsideTheReaderSetNeedsTheIdentityLine` | `internal/le/hookruntime/session_identity_test.go` | AC-2: exit 2 and the exact line in the message | |
| `TestSubagentCommandCarryingItsIdentityLinePasses` | `internal/le/hookruntime/session_identity_test.go` | AC-3 | |
| `TestSubagentIdentityLineOfAnotherAgentIsRefused` | `internal/le/hookruntime/session_identity_test.go` | AC-4 | |
| `TestMainThreadCommandNeedsNoIdentityLine` | `internal/le/hookruntime/session_identity_test.go` | AC-5 | |
| `TestSubagentUnparsableCommandNeedsTheIdentityLine` | `internal/le/hookruntime/session_identity_test.go` | R-2: an unparsable line is never a reader line | |
| `TestSubagentStartCarriesTheIdentityLine` | `internal/le/hookruntime/lifecycle_test.go` | AC-6, both payload shapes | |
| `TestGuardsIgnoreCommandNamesInsideQuotes` | `internal/le/hookruntime/bash_test.go` | AC-7, one row per guard | |
| `TestGuardsJudgeAnUnparsableLineByItsText` | `internal/le/hookruntime/bash_test.go` | AC-8 | |
| `TestRootBuildAllowsPackagesThatProduceNoBinary` | `internal/le/hookruntime/bash_test.go` | AC-9 | |
| `TestRootBuildRefusalNamesTheCallersPackage` | `internal/le/hookruntime/bash_test.go` | AC-10 | |
| `TestRootBuildRefusesTheWholeTree` | `internal/le/hookruntime/bash_test.go` | AC-11 | |
| `TestLossyPipeRefusalNamesTheQuietAdmittedForm` | `internal/le/hookruntime/bash_test.go` | AC-12; replaces the expectation of `TestLossyPipeRefusalNamesTheScopedRun`, through `/ze-test-judge` | |
| `TestRefusedCallDoesNotConsumeTheBudget` | `internal/le/hookruntime/budget_test.go` | AC-13 through `Run` | |
| `TestTestDeletionJudgesOnlyTheArgumentsOfAPlainRm` | `internal/le/hookruntime/bash_test.go` | AC-14, with the line-wide survivors as rows | |
| `TestTestDeletionRefusalNamesTheNextAction` | `internal/le/hookruntime/bash_test.go` | AC-15 | |
| `TestSystemTmpIgnoresATmpDirectoryUnderAnotherPath` | `internal/le/hookruntime/bash_test.go` | AC-16 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Counted calls of a budgeted agent | 0-100 | 100 | N/A | 101 (refused unless a handoff call), already covered by `TestAgentCallBudgetRefusesTheCallPastTheBudget` |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le ai hooks unit` | `internal/le/ai/hooks/fixtures.go` | Every golden row and behavioral fixture of the hooks, with rows added for AC-1, AC-2, AC-7 and AC-12 and the source seal re-derived | |
| Live probe | a research agent spawned after the phase 1 commit | Its Bash calls run with no "no verdict" failure, and its first `le` call carries the line (A-2, A-4) | |

## Files to Modify
- `internal/le/hookruntime/session.go` - the prefix rewrite becomes the identity check
- `internal/le/hookruntime/runtime.go` - `Run` no longer calls a rewrite; the budget counts only an unrefused call
- `internal/le/hookruntime/budget.go` - counting moves behind the other verdicts
- `internal/le/hookruntime/bash.go` - one segmenter on `lineWords`; `bashLossyPipe`, `bashRawHeavy`, `bashRootBuild`, `bashSystemTmp`, `bashTestDeletion` and their messages
- `internal/le/hookruntime/lifecycle.go` - the start text
- `internal/le/ai/hooks/fixtures.go` - new rows, the counts and the source seal

Pages and rule points that change with the code, in the same phase:
- `.claude/hooks/README.md` - the rows for the prefix, the budget and each changed guard
- `docs/contributing/running-commands.md` - "Session identity, and what each agent owns": the agent types the line
- `ai/rules/points/planning/directives/brief-every-subagent-with-its-spec-phase-and-rules.md` - "the `ZE_AGENT_ID` its hook exports" becomes the line the agent types
- `ai/rules/points/context-economy/directives/an-editing-agent-hands-off-at-one-hundred-tool-calls.md` - refused calls are not counted
- `ai/agents/ze-work.md` - the budget sentence
- `plan/learned/HOOK-FRICTION.md` - the auto-mode void and its cause

Named as unaffected: `docs/architecture/core-design.md`, which the hook files
declare in their `// Design:` headers. It describes the command engine and
that hook policy is native `le` code, and says nothing of the prefix, the
guards' matching or the budget's counting, so no sentence in it becomes wrong.

## Files to Create
- `internal/le/hookruntime/session_identity_test.go` - the identity check's tests
- `internal/le/hookruntime/shellline.go` - the one line splitter (`lineCommands`, and `lineWords` on it), created in phase 1

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | development tooling, no config |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | No | no `le` command is added or changed |
| CLI grammar (keyword before value) | N-A | no command |
| Editor autocomplete | N-A | no command |
| Functional test for new RPC/API | N-A | no RPC; the hook fixtures are the entry-point proof |
| Pipe completeness | N-A | a hook writes the hook protocol, not command output |
| Env var registration | No | the two variables exist and are not `ze.*` keys |
| Doctor check for runtime dependencies | N-A | no shipped binary changes |
| Doctor code per problem class | N-A | hook refusals reach an agent, not an operator of `ze` |
| Prometheus counters/metrics | N-A | no shipped binary changes |
| BGP family surface | N-A | not protocol work |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | development tooling |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/contributing/running-commands.md` |
| 7 | Wire format changed? | No | the hook response loses a field the harness treats as optional |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | |
| 10 | Test infrastructure changed? | Yes | `.claude/hooks/README.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | the hook registry is unchanged |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `./le spec citation anchors` run in phase 1 named three pages. `docs/contributing/navigating-the-code.md` anchors `nativeHookActions`, `preMaterializeDerived`, `commandNamesArtifact` and the session-start steps; `docs/contributing/spec-workflow.md` anchors `hookStop`, `hookValidateSpec`, `validateSpecText` and `validateSpecRFCGaps`; `docs/architecture/testing/verify-freshness-scope.md` anchors `reportDerivedDebt` and `linkedWorktree`. Phase 1 changes none of those symbols and no sentence beside them, so all three are unaffected by phase 1. Phases 2 and 3 re-check `navigating-the-code.md` for `bash.go`. The pages phase 1 edited: `docs/contributing/running-commands.md` and `.claude/hooks/README.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/running-commands.md`, `docs/contributing/testing.md` where they show a piped or redirected test command |

## Implementation Steps

Each phase is one work package of about 60 tool calls for one `ze-work`
agent, committed when it passes. The main thread runs every test.

1. **Phase: Wiring -- the identity check**
   - Tests: the four Wiring Test rows except the budget row, plus `TestSubagentCommandCarryingItsIdentityLinePasses`, `TestSubagentIdentityLineOfAnotherAgentIsRefused`, `TestMainThreadCommandNeedsNoIdentityLine`, `TestSubagentUnparsableCommandNeedsTheIdentityLine`
   - Files: `session.go`, `runtime.go`, `lifecycle.go` (the identity line only), `session_identity_test.go`, the running-commands page, the planning point, HOOK-FRICTION
   - Verify: tests fail, then pass; validate A-1 from a recorded payload; after the commit, the live probe validates A-2 and A-4
2. **Phase: Guards judge commands**
   - Tests: `TestGuardsIgnoreCommandNamesInsideQuotes`, `TestGuardsJudgeAnUnparsableLineByItsText`, the three `TestRootBuild*`, `TestSystemTmpIgnoresATmpDirectoryUnderAnotherPath`
   - Files: `bash.go`, `.claude/hooks/README.md`
   - Verify: every existing refusal test in `bash_test.go` still passes unchanged
3. **Phase: Messages, test deletion and the budget**
   - Tests: `TestLossyPipeRefusalNamesTheQuietAdmittedForm`, `TestTestDeletion*`, `TestRefusedCallDoesNotConsumeTheBudget`
   - Files: `bash.go`, `budget.go`, `runtime.go`, `lifecycle.go` (the five rule lines), the context-economy point, `ai/agents/ze-work.md`, then `./le ai sync update` and `./le ai rules update condensed`
   - Verify: the changed expectation of the existing piped-command test is approved by `/ze-test-judge` before the edit
4. **Phase: Fixtures and seal**
   - Tests: `./le ai hooks unit`
   - Files: `internal/le/ai/hooks/fixtures.go`
   - Verify: the seal is re-derived through `TestEveryStoredDigestMatchesTheTreeItRatchets`, with a dated comment naming this spec

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and a test that goes red without it |
| Correctness | No path remains on which a subagent command outside the reader set runs without its identity; an unparsable line is never treated as a reader line |
| Fail-closed | Each changed guard, given a line `lineWords` cannot parse, refuses what it refused before |
| No layering | The regular-expression splitters and the rewrite are deleted, not kept as a fallback |
| Messages | Every changed refusal names what is wrong, why, and the exact command; none asks a question; none advises a form another rule forbids |
| Rule: a hook judges what a command is | No check reads the wording of a message or a prompt |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No changed input is written | `grep -rn updatedInput internal/le/hookruntime` finds no writer |
| One segmenter | `grep -n "regexp.MustCompile(\`&&" internal/le/hookruntime/bash.go` finds no command splitter outside the poll-loop check |
| Start text carries the line and the rules | `TestSubagentStartCarriesTheIdentityLine` |
| Hook fixtures green | `./le ai hooks unit` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Identity | An agent cannot pass a command under another agent's or the main thread's identity; the line is compared to the payload's ids, never trusted from the command |
| Injection | The refusal message prints the caller's command; it is text to an agent, never executed by the hook |
| Fail-open | The only fail-open paths are the existing ones in `Run` (panic, malformed payload) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A-1 false | Use the fallback text of AC-6 and record it under Design Insights |
| A-2 false | Stop after phase 1 and report: the void has another cause, and the umbrella's AC-2 needs a new design |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

Phase 1 (2026-11-11):

- The identity check needs each command's first word, and `lineWords` yields
  one flat word list with the command boundaries dropped. So the one segmenter
  this spec plans for phase 2 was built in phase 1: `lineCommands`, in the new
  file `internal/le/hookruntime/shellline.go`, and `lineWords` is now its
  flattened output with the same words as before. The move to a new file keeps
  `bash.go` under the 1000-line threshold. Phase 2 points the four guards at
  `lineCommands`; it does not build a segmenter.
- The identity check is a registered check, `bashAgentIdentity`, in
  `nativeHookActions`, bound to the planning point that briefs a subagent. It
  runs with every other check, so an agent sees the identity refusal and any
  guard's refusal in one turn. The rewrite ran only when no check refused.
- A double-quoted `$(` and a backtick hide a command inside one word, so the
  segmenter reports a command substitution and the check asks for the line.
  A single-quoted one is text and passes.
- After its own line a command is still refused when it sets or unsets either
  identity variable (an assignment in front of a command, or an argument of
  `export`, `unset`, `env`, `declare`, `typeset`, `readonly`, `local`).
- Two reader-set members stay on trust and the table's comment says so: a GNU
  `sed` script can run a command through its `e` command, and `gopls` starts
  the go tool. `find`, `rg`, `sort` and `git` carry an argument judge.
- A-1 is not validated by phase 1: no hook records a SubagentStart payload,
  and the agent cannot make the harness send one. The start text handles both
  shapes, so the first agent spawned after the commit shows which shape the
  harness sends: the full line, or the session part with the fallback sentence.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The agent types its identity line; the hook checks it | Keep the rewrite but only for commands that reach `le` | A rewrite of any call voids that call and its siblings in the same message, and `le` calls are the ones an agent can least afford to lose. A typed line passes auto mode like any command |
| A command needs the line unless every command in it is a declared reader | Require the line only when the text names `le` | Naming `le` is a text match of the kind this spec removes, and a script or a test binary reaches the identity without naming `le`. Requiring the line by default is wrong only in the safe direction: a refusal with its fix |
| A refused call is not counted | Count turns instead of calls | The payload has no message id, so counting turns needs the transcript at pre-tool time, which is unverified. Not counting refusals removes the double cost now; child 4 reports the count to the main thread |
| Narrow the test-deletion guard only for a plain `rm` with literal arguments | Pair every delete with its arguments | A shell has more ways to reach a delete than a hook can parse; the narrow case is the one a hook can see whole |

## Known Limitations
- The budget still counts parallel calls separately. Rewarding batching in the budget needs a turn count, which child 4 of the umbrella examines with the transcript.
- A raw `go vet` without tags is not refused here. `plan/post-release/engineering/spec-agent-economy-2-scoped-test.md` owns the short action that replaces it and the message that names it.

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
- [ ] AC-1..AC-16 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le ai hooks unit`, and the two packages through `./le job run label unit-pkg quiet command go test -tags <the tags from feature-gates.txt> ./internal/le/hookruntime/ ./internal/le/ai/hooks/`. No `./le test unit <group>` covers `internal/le` (the groups are bgp, core, plugins, config, cli and installer), so the package form is the scoped run. No agent runs any `./le verify` command
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: the hook fixtures are the entry-point proof)
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-agent-economy-1-call-shape.md` only, in the same `./le commit create` script
