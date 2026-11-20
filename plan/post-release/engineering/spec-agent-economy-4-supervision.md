# Spec: agent-economy-4-supervision

| Field | Value |
|-------|-------|
| Status | done |
| Scope | tooling |
| Priority | P1 |
| Release reason | Release-independent development tooling: it changes how the main thread is told about context size and agent cost and changes no shipped binary. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 4 of `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`,
which holds the baseline.

Main sessions are 12% of turns and 26% of tokens: their average turn carries
353k of context, and one session in ten peaks above 744k. The rules that
bound this (hand off past 600k, one spec per session, cut a work package at
about 60 calls) are prose with nothing behind them. 344 of 1,656 editing
agents ran into their 100-call wall, and the main thread never learns how
many calls an agent used. Skill bodies of 29 to 44KB load whole into the agent
that runs them, branch-only reference text included.

The goal: the main thread is told when its context passes the handoff
threshold and how many calls each agent used, an agent is told its target
before it hits the wall, a second spec claim in one session is flagged, and a
skill body holds the steps an agent runs with the reference text one read
away.

### A spec set nobody is working (added 2026-11-14)

The umbrella this spec belongs to stalled on 2026-11-11: three children
closed, one sat `ready` and unclaimed, two stayed skeletons, and the session
that ran them ended. It did leave a committed handover naming the next child
(`plan/handover/2026-11-11-agent-economy.md`). Nothing showed that handover
to a later session, and on 2026-11-14 a cleanup of closed handovers removed
it (935398f8df): three closed children made the set look finished. Every safeguard against abandoned
work reads one spec whose Status is `in-progress` under a live claim. An
umbrella never reaches that state, and the one detector that blocks a stop
exempts it by name, so a set of specs has no owner once its session ends.

The second goal: the state of a spec set is derived from its children and
from the claims held on them, never declared in the umbrella. A set that has
started and not finished is reported as live, a live set nobody holds is
reported as stalled with the child to claim next, and a session does not
stop while it leaves a child of the set half done.

Owner decision (2026-11-14): two limits, counting different things. The open
limit counts every spec started and not closed, a started set included,
whether or not anyone works on it: the reason to close before opening. The
active limit counts the specs worked on right now, each under a live claim:
the bound on what runs at once (`ai/META-DECISIONS.md`, "How agents work").
Today one cap reads `in-progress` Status alone, so it is neither: it misses a
started set, and it counts a spec nobody holds the same as one being worked.

## Research Findings (2026-11-11, read at the producer by a research agent; re-read at design)

| Finding | Where |
|---------|-------|
| The 600k handoff exists only as rule text. No hook reads context size | `ai/rules/points/planning/directives/the-main-thread-supervises-and-verifies-what-comes-back.md` |
| The hook payload carries the transcript path and no usage field | `internal/le/hookruntime/runtime.go`, `Payload` |
| Usage is already parsed from a transcript, over the whole file | `internal/le/ai/tokens/tokeneconomy.go`, `usageInt`, `ScanTranscript` |
| The per-turn hooks print nothing on a normal prompt | `internal/le/hookruntime/lifecycle.go`, `hookCompactionReminder`, `hookStop`, `hookEndSummary` |
| The state file's mechanical content is a snapshot; handoffs are written by hand | `internal/le/session/summary.go`, `snapshotText`; `ai/skills/ze-implement.md`, "Phase handoff" |
| A claim overwrites the session marker and checks only the cap, for a ready spec. Release keeps no history, so a second claim passes silently | `internal/le/spec/session.go`, `specOwner.Claim`, `Release` |
| The agent is told 100, not 60. Nothing warns before the wall | `ai/agents/ze-work.md`; `internal/le/hookruntime/budget.go` |
| The agent's counter file has no reader, and no SubagentStop hook is wired | `internal/le/hookruntime/budget.go`, `bumpAgentCalls`; `.claude/settings.json` |
| A continuation is a fresh agent that pays the start, the skill body and the spec again | `ai/rules/context-economy.md` |
| Skill sizes and what each could shed without losing a step: ze-implement 29.6KB (10 to 12KB), ze-review 41.5KB (12 to 15KB), ze-close 28.7KB (6 to 8KB), ze-rfc 43.7KB (18 to 20KB) | `ai/skills/` |
| `ai/rationale/` already holds on-demand text that four skills point at; skill supporting files are not mirrored | `ai/rationale/`; `internal/le/ai/sync/ai.go` |
| ze-test-judge ran 232 times; its body is 3.6KB and its report is one row per unit, so its cost is the agent start and one judge can take several units | `ai/skills/ze-test-judge.md` |
| `spec-tooling-one-off-defects` AC-22 owns the state renderer's heading | `plan/post-release/engineering/spec-tooling-one-off-defects.md` |

Spec-set findings (2026-11-14, read at the producer by the main thread):

| Finding | Where |
|---------|-------|
| A claim moves only a `ready` spec to `in-progress`. On any other Status it writes the session marker and leaves the spec as it was, so an umbrella claimed as a skeleton stays one | `internal/le/spec/session.go`, `specOwner.Claim` |
| The stop hook refuses a stop only when the closure detector reports the claimed spec blocked. Otherwise it warns, and only for an `in-progress` spec | `internal/le/hookruntime/lifecycle.go`, `hookStop` |
| The closure detector answers false for any spec that is not `in-progress`, and false for every umbrella | `internal/le/spec/status/closure.go`, `closureCompletedNotClosed` |
| An umbrella is recognised by `-umbrella-` in its stem, or by another spec carrying its stem and a number. The exemption exists because an umbrella matches its children's closure records | `internal/le/spec/status/closure.go`, `inspectClosureSpec`; `docs/contributing/spec-workflow.md`, "Spec sets" |
| The stale flag covers a skeleton idea past a time limit and nothing else | `internal/le/spec/status/specstatus.go`, `skeletonStale` |
| The WIP list and the cap read `in-progress` specs only | `internal/le/spec/session.go`, `wip` |
| The status vocabulary has no event for "children are running", so an umbrella is never owed a transition | `docs/contributing/spec-workflow.md`, the event table |
| The rules tell a session that closed a child to stop: one spec per session, and stop when the task is done. Only the owner-is-away rule makes a session run a whole set | `.claude/rules/session-start.md`, "Session Focus"; `ai/META-DECISIONS.md`, "How agents work" |
| The per-spec state file is the handoff and lives in untracked session scratch. A committed handover is owed only once one is written | `ai/META-DECISIONS.md`, "A handover lives in the repository" |
| A closed child's file is deleted, so the umbrella's Children table is the only place that says which children closed, and it is written by hand | `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`, "Children" |

Design research (2026-11-14, by a research agent; each row re-read at the producer by the main thread):

| Finding | Where |
|---------|-------|
| A claim is one flat file per session or per agent holding the spec's file name. Nothing lists the claims of all sessions: the cap check reads spec Status only | `internal/le/le/path/session.go`, `SpecClaimMarker`; `internal/le/spec/session.go`, `wip` |
| Whether a session is alive is judged in one place, the session reaper: a pid pin checked against the process list, the session id in a process's arguments, or a recent transcript. An agent's marker has no signal of its own and outlives the agent | `internal/le/session/reap.go`, `judge` |
| Claim markers leak. The reaper removes a marker only when its session still has a dated directory among the dead, so a marker whose directory is gone stays: 64 markers against 46 session directories on 2026-11-14 | `internal/le/session/reap.go`, `flatMarkers` |
| Which spec a commit closes is judged for the commit in hand only. No record of removed specs is kept, and `plan/learned/` holds nothing for the three closed children, so the removal commits in git history are the only source, and reading them is a walk of the history | `internal/le/commit/review.go`, `closedSpecStem`, `specWorkPopulation` |
| Two functions group specs into sets and they disagree. The inventory and the roadmap use a file-name pattern that takes any name with a number in it for a set and cannot read a prefix that holds digits. The closure detector uses the umbrella marker or a sibling stem | `internal/le/spec/status/specstatus.go`, `detectSet`; `internal/le/spec/status/closure.go`, `inspectClosureSpec` |
| The session-start summary reads no git on purpose, to stay fast | `internal/le/spec/status/specstatus.go`, `StatusPhrases`; `internal/le/hookruntime/lifecycle.go`, `reportSpecs` |
| A release answers that it released and nothing more | `internal/le/spec/actions.go`, `answerRelease` |
| The closure refusal's acknowledgement file is written by nobody in the tooling. The refusal itself prints the one-line command that creates it, so any agent can clear its own refusal | `internal/le/spec/status/closure.go`, `CheckClosure`; `closure_report.go` |
| The claim refusal at the cap prints the setting that raises the cap by one, although the owner's rule says the cap is never raised | `internal/le/spec/session_report.go`, `ClaimReport.Text` |
| An earlier record decided "No new rule" for an umbrella left open after its last child, and proposed treating an umbrella with closed children as a closure signal | `plan/learned/HOOK-FRICTION.md`, F21 |

Harness questions, answered from the harness documentation on 2026-11-14 except the last:

| Question | Answer |
|----------|--------|
| Does a non-blocking message from a PostToolUse or PreToolUse hook reach the model? | Yes: "Claude Code wraps the string in a system reminder", read on the next model request. Each string is capped at 10,000 characters, which a one-line report never nears |
| Does the Agent tool's PostToolUse payload carry the agent id? | Yes, in the tool response. Its token figures cover the final request only |
| Is the transcript flushed before a pre-tool hook runs? | No: it "is written asynchronously and may lag". The context size read is one turn old, which a 600k threshold tolerates |
| Does a SubagentStop payload carry the agent id? | Not found in the documentation. Record one payload into scratch at phase 1 |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/le-tooling.md` - the door for the hooks
  → Constraint: a hook runs for every session, so the context-size read touches only the transcript's tail, never the whole file, and never refuses on a read error.
- [ ] `ai/rules/context-economy.md` and `ai/rules/planning.md` - the rules this makes mechanical
  → Decision: the thresholds are the rules' own: 600k for the main thread, about 60 calls for a package, 100 for the budget.
- [ ] `ai/META-DECISIONS.md` - "A hook checks what a command, path or file is, never how a message is worded"
  → Constraint: a usage number read from the transcript is a fact about a file, within that rule.

**Key insights:**
- The nudge belongs at agent spawn, not at prompt submit: a session whose owner is away receives few prompts and spawns many agents.
- Every step of a skill stays in its body. Only rationale, examples and branch-only tables move.

## Current Behavior (MANDATORY)

**Source files read:** the main thread read two producers; the rest of the Research Findings table is owed a read at design.
- [ ] `internal/le/hookruntime/budget.go` - `bumpAgentCalls` writes the agent's counter file and nothing reads it back for the main thread; the refusal past 100 is the agent's only notice
- [ ] `internal/le/hookruntime/runtime.go` - `Payload` carries the transcript path, the agent id and type, and no usage or context-size field
- [ ] `internal/le/spec/session.go` - `Claim` transitions a `ready` spec only and writes the marker for any other Status; `Release` removes the marker and answers nothing; `wip` lists `in-progress` specs
- [ ] `internal/le/hookruntime/lifecycle.go` - `hookStop` blocks on the closure detector's verdict for the claimed spec and warns for an `in-progress` one
- [ ] `internal/le/spec/status/closure.go` - `closureCompletedNotClosed` returns false for a spec that is not `in-progress` and for every umbrella

**Behavior to preserve:**
- An umbrella is never raised to a closure candidate from a child's closure record: that false positive stays removed.
- A set none of whose children was ever claimed or closed is still an idea, reported as today.
- Closure agents claim under their own agent id, and up to three closures run at once: the second-claim warning keys on the main session alone.
- A claim taken only to close a spec needs no route above a limit: the open limit judges only a claim that starts a spec, and a spec whose work is committed is already started, so claiming it adds nothing to the open count. The active limit binds it like any other claim.
- No hook judges wording.

**Behavior to change:**
- Only what the acceptance criteria name.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Transcript usage parser | `internal/le/ai/tokens/tokeneconomy.go` | Sums usage over a transcript | Extends: a tail-only read for the hook, and the kept measure the umbrella's criteria need |
| Agent-spawn marker hook | `.claude/settings.json`, PostToolUse on Agent | Writes a marker when an agent is spawned | Extends: the same hook reports context size and the finished agent's call count |
| Agent call counter | `internal/le/hookruntime/budget.go` | Counts an editing agent's calls | Extends: gains a reader, and the agent hears its target |
| Spec claim | `internal/le/spec/session.go`, `Claim` | Records the session's spec | Extends: remembers earlier claims of the session and warns on a second |
| On-demand rationale pages | `ai/rationale/` | Hold text a skill points at | Extends: receives the reference text four skills shed |
| Spec set naming | `docs/contributing/spec-workflow.md`, "Spec sets"; `internal/le/spec/status/closure.go`, `inspectClosureSpec` | A shared prefix and number make a set, and `-umbrella-` marks its parent | Extends: the same naming is what the derived set state is computed over. No second way to declare a set |
| Closure detector | `internal/le/spec/status/closure.go`, `closureCompletedNotClosed` | Reports an `in-progress` spec whose work is recorded as done, and exempts umbrellas | Extends: the exemption keeps its reason and gains a positive check for an umbrella whose children are all closed |
| Stop hook | `internal/le/hookruntime/lifecycle.go`, `hookStop` | Refuses a stop on an implemented spec that is not closed, and only warns for an `in-progress` claim | Extends: the warning becomes a refusal when the session would leave a claimed spec `in-progress` with nothing committed that says where it stands |
| Stale-skeleton flag and status inventory | `internal/le/spec/status/specstatus.go`, `skeletonStale`; `report.go` | Lists every spec by bucket and Status and flags old skeleton ideas | Extends: the inventory gains the set view. The stale flag is not the carrier: it measures age, and a stalled set is stalled on its first day |
| WIP cap and list | `internal/le/spec/session.go`, `wip`, `Claim` | One cap over `in-progress` specs, checked when a `ready` spec is claimed, with the stalest listed on refusal | Replaces: two limits. The open limit counts started and unclosed specs and live sets; the active limit counts live claims. The one cap is deleted, not kept beside them |
| Spec claims | `internal/le/spec/session.go`, the session and agent markers | Record which session or agent holds which spec | Extends: the live claims are what the active limit counts |
| Committed handover | `plan/handover/` | Holds a handover once a session writes one | Does not apply to a set left between children: the derived state already names the next child, so a note would be a second copy of it. It stays the home for work left half done |
| Spec removal in git history | `internal/le/commit/review.go`, `closedSpecStem` | Judges which spec the commit in hand closes | Extends: the same judgment over past commits is the record of which children closed, kept in a cache keyed by HEAD so no reader walks history twice. The `Spec-work:` trailer is not the source: a commit made without a claim carries none |
| Set detection in the inventory | `internal/le/spec/status/specstatus.go`, `detectSet` | Names a spec's set from its file name | Replaces: one grouping function answers the inventory, the roadmap and the closure detector. `detectSet` and the sibling-stem test in `inspectClosureSpec` are deleted |
| Session liveness | `internal/le/session/reap.go`, `judge` | Decides which sessions are dead, for the reaper | Extends: the same judgment says whether a claim is live. No second liveness test is written |
| Claim marker reaping | `internal/le/session/reap.go`, `flatMarkers` | Removes the markers of a dead session that still has a directory | Extends: a marker whose session has no directory is reaped too, and an agent's marker goes when the agent finishes |

## Data Flow (MANDATORY)

### Entry Point
- The PostToolUse payload of an Agent call in the main thread.
- `./le spec claim <spec>` and `./le spec release`.
- `./le spec status`, the session-start summary, and the Stop event of a session.

### Transformation Path
1. The hook reads the transcript's tail for the last recorded context size and the finished agent's counter.
2. It returns both to the main thread as additional context, with the handoff instruction when the size passes the threshold.
3. For a spec set, the status reader groups the specs that share a prefix, reads which children still exist and which closed, and reads the claims held on them.
4. From those it answers one state per set: idea, live, stalled or children closed. The status view, the session-start summary, the claim, the release and the stop hook each read that one answer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ hook | payload in, additional context out | No |

### Changed-Wire Census

Four file or environment formats change or gain a reader. None is a network
wire.

**Search evidence:** `grep -rn "ZE_SPEC_WIP_CAP\|wipCapEntry" internal/le` finds the setting's reader in `internal/le/spec/session.go` and its printed form in `session_report.go`; the research agent's grep over `ai/`, `.claude/rules/`, `docs/contributing/` and `plan/` finds the text that names it. `grep -rn "closure-ack" internal/le` finds one reader and no writer. Positive control: `SpecClaimMarker` is the one producer of a claim marker's path.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Claim marker file | writer | `internal/le/spec/session.go`, `Claim` | read | Preserved: one line holding the spec's file name |
| Claim marker file | reader | `specClaim` in the hooks, `currentSpec`, the reaper's `flatMarkers`; new: the claim enumerator | read | Extended: a new reader lists every marker; the format is unchanged |
| The WIP cap setting `ZE_SPEC_WIP_CAP` | writer | an operator or an agent setting it for one claim, as `ai/skills/ze-implement.md`, `ze-close.md`, `ze-status.md`, `.claude/rules/session-start.md`, `docs/contributing/spec-workflow.md` and `ai/META-DECISIONS.md` describe | research grep | Replaced: two settings, one per limit. Every page, skill, rule and spec that names the old one is converted in this work |
| The WIP cap setting `ZE_SPEC_WIP_CAP` | reader | `internal/le/spec/session.go`, `wipCapEntry`, `cap`; printed by `ClaimReport.Text` | read | Replaced with the two readers; the old name is deleted, not kept as an alias |
| Removed-spec cache (new) | writer | the grouping function in `internal/le/spec/status` | none exists today | New: a file under the session scratch root keyed by HEAD |
| Removed-spec cache (new) | reader | the grouping function, for the status view, the session-start summary, the claim, the release and the stop hook | none exists today | New |
| Hook additional context on an Agent call | writer | the after-hook on the Agent tool in `internal/le/hookruntime` | `.claude/settings.json` | Extended: gains the context size and the agent's call count |
| Hook additional context on an Agent call | reader | the harness, which shows it to the main thread | harness documentation | Preserved: the envelope is unchanged and each string stays far under the 10,000-character cap |

### Integration Points
- The agent-spawn hook, the transcript usage parser, the agent call counter, the spec claim, the rationale pages.
- The status inventory, the roadmap's set column, the closure detector, the session reaper, the session-start summary, the release answer, the stop hook and `./le commit create`'s removal check.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| PostToolUse of an Agent call with a transcript past the threshold | → | the context-size report | `TestAgentSpawnReportsContextPastTheHandoffThreshold` |
| `./le spec claim` of a second spec in one main session | → | `Claim` | `TestClaimWarnsOnASecondSpecInOneMainSession` |
| `./le spec status` over a set with a closed child, an open child and no claim | → | the derived set state | `TestStatusReportsAStartedSetNobodyHoldsAsStalled` |
| The Stop event of a session whose claimed spec is `in-progress` with nothing committed that says where it stands | → | `hookStop` | `TestStopLeavingAClaimedSpecHalfDoneIsRefused` |
| `./le spec claim` of a `ready` spec with the open limit reached and the active limit not | → | `Claim` | `TestClaimIsRefusedAtTheOpenLimitAndNamesWhatToClose` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The main thread spawns an agent while its last recorded context is above 600k | The hook tells it the size and to write the state file and hand off | Below the threshold it reports nothing: a message on every spawn is the noise this removes |
| AC-2 | An editing agent finishes | The main thread is told that agent's call count | A read-only agent has no counter and reports none |
| AC-3 | An editing agent reaches 60 and 80 counted calls | It is told its count, its target of 60 and its budget of 100 | The call itself passes |
| AC-4 | A main session claims a second spec after releasing a first | The claim answers with a warning naming the first spec and the fresh-session rule | Agent claims are not warned |
| AC-5 | `ze-implement`, `ze-review`, `ze-close`, `ze-rfc` | Each body holds every step it held, and its branch-only reference text sits in a page the step names | None lost: every moved sentence is one read away |
| AC-6 | A phase that owes several test judgments | One judge agent takes them in one brief | A judgment that must not see another's context stays alone |
| AC-7 | `./le ai tokens` over a window of transcripts | Reports the umbrella's baseline figures | None |
| AC-8 | A spec set with at least one child closed or claimed and at least one child still open | The status view reports the set as live and names its closed children, its open children and the next one to run, whatever Status the umbrella declares | A set none of whose children was ever claimed or closed stays an idea and is reported as today |
| AC-9 | A live set on which no live session and no running agent holds a claim | The status view and the session-start summary report it as stalled, with the next child, the command that claims it, and the set's handover file when one is tracked. The session-start summary reads the cache of removed specs and runs no git walk | A set whose every open child waits on a named spec outside the set reports that spec instead: there is nothing to pick up |
| AC-10 | A session releases its claim on a child of a set whose other children are still open, or closes that child | The release answers with the remaining children and the next one, which is what the next session will be shown | Releasing the last open child reports the umbrella's own criteria as what remains |
| AC-11 | A session stops while the spec it claimed is `in-progress` and nothing committed says where the work stands: no handover under `plan/handover/` and no blocked hand-back in the spec | The stop is refused, naming the spec and the two ways out: close it, or commit where it stands and release the claim | A session that closed its child and left the set between children stops freely: the derived state is its handover. The owner ending the session himself is never a hook's business |
| AC-12 | An umbrella all of whose children are closed | The closure detector reports it as owing its own acceptance criteria and its closure, and the stop hook treats it as an implemented spec that is not closed. `plan/learned/HOOK-FRICTION.md` F21 is retired with this: its "No new rule" is reversed and its proposed fix is what lands | A child's closure record alone still never makes the umbrella a closure candidate |
| AC-13 | A claim of a `ready` spec when the open limit is reached | The claim is refused and lists what is open and unheld, stalled sets first, as the things to close | A claim made only to close a spec whose work is committed still passes, as today |
| AC-14 | The planning rule and the session-start rule | The planning rule states that a session never leaves a claimed spec half done with nothing committed, and that a set left between children needs no note. The session-start rule points at it | One spec per session stays the rule: the next child still runs in a fresh session |
| AC-16 | A claim when the active limit is reached and the open limit is not | The claim is refused and lists the live claims and who holds each | A claim that resumes a spec already open adds nothing to the open count: only the active limit judges it |
| AC-17 | The WIP report | It shows the two counts apart, each against its limit: open, and being worked on now | None |
| AC-18 | A commit that removes a tracked handover whose name carries the prefix of a live set, or the stem of an open spec | The commit is refused, naming the set and its open children | A handover of a set whose children are all closed, and one of a closed spec, are removed freely |
| AC-19 | A claim marker whose session has no directory left, and the marker of an agent that has finished | The reaper removes the first; the Agent call's after-hook removes the second | A marker of a live session or a running agent stays |
| AC-20 | The inventory, the roadmap and the closure detector | Each takes a spec's set from one grouping function, which reads a prefix holding digits and does not take a lone numbered name for a set | A set of one file with no umbrella and no sibling is not a set |
| AC-21 | The claim refusal at a limit | It names what to close or who holds the live claims, and prints no setting that raises a limit | A claim of a spec that is already started is never refused by the open limit, so the close-only route needs no setting |
| AC-15 | The implement skill's review loop | It states the round limit the planning rule states: five rounds are the session's, the sixth is the owner's | The sentence that each round covers less than the one before stays |

### Scope cut (owner decision, Thomas, 2026-11-20)

Child 4 is cut to packages 4 and 8 and a lowered AC-1 threshold, then closed.
These criteria are dropped by that decision and are not owed by this spec:
AC-2, AC-3, AC-4, AC-10, AC-11, AC-12, AC-13, AC-14, AC-16, AC-17, AC-21, and
the agent half of AC-19 (the reaper half landed with package 3).

| Dropped | Reason, as measured or ruled |
|---------|------------------------------|
| AC-2, AC-3, the agent half of AC-19 | Agents do not need the supervision they add: the median first-call context of an agent is 21.9k, and 6.9% of ze-work agents reach the 100-call budget. The existing budget and its refusal at 100 remain |
| AC-11, AC-14 | The harness fires its stop event at the end of every turn, so AC-11's refusal would block a session that pauses to ask the owner a question. AC-14's rule text only pointed at that refusal |
| AC-13, AC-16, AC-17, AC-21 | The two limits already stand as written rules in `ai/META-DECISIONS.md`, "How agents work", and the existing cap remains as the tooling's check |
| AC-10, AC-12 | Cut with packages 6 and 7, which carried them, under the same decision |
| AC-4 | Package 6, the second-claim warning, cut with the low-value group on 2026-11-20 by Thomas's approval. The one-spec-per-session rule stays as prose |

What stays owed: AC-1 with its lowered threshold, AC-8 and AC-9 (package 4),
AC-18 (package 8). AC-5, AC-6, AC-15 (package 10), AC-7 (package 11), AC-19's
reaper half (package 3) and AC-20 (package 1) are built.

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAgentSpawnReportsContextPastTheHandoffThreshold` | `internal/le/hookruntime/` | AC-1 | |
| `TestClaimWarnsOnASecondSpecInOneMainSession` | `internal/le/spec/` | AC-4 | |
| `TestStatusReportsAStartedSetNobodyHoldsAsStalled`, with the survivors and doubts in `setstate_test.go` | `internal/le/spec/status/` | AC-8, AC-9 | Written, not run |
| `TestStopLeavingAClaimedSpecHalfDoneIsRefused` | `internal/le/hookruntime/` | AC-11 | |
| `TestClaimIsRefusedAtTheOpenLimitAndNamesWhatToClose` | `internal/le/spec/` | AC-13 | |
| `TestTheMainThreadIsToldAnAgentsCallCount` | `internal/le/hookruntime/` | AC-2 | |
| `TestAnAgentIsWarnedAtSixtyAndEightyCallsAndTheCallPasses` | `internal/le/hookruntime/` | AC-3 | |
| `TestTheTokenReportCarriesEveryUmbrellaFigure` | `internal/le/ai/tokens/` | AC-7, over a fixture transcript window | |
| `TestReleaseNamesTheChildrenThatRemain` | `internal/le/spec/` | AC-10 | |
| `TestAnUmbrellaWhoseChildrenAreAllClosedOwesItsClosure`, `TestAChildsClosureRecordAloneNeverMakesTheUmbrellaACandidate` | `internal/le/spec/status/` | AC-12 | |
| `TestClaimIsRefusedAtTheActiveLimitAndNamesTheHolders`, `TestTheActiveLimitSaysWhenLivenessCannotBeJudged` | `internal/le/spec/` | AC-16 | |
| `TestTheWIPReportShowsBothCounts` | `internal/le/spec/` | AC-17 | |
| `TestAHandoverOfALiveSetCannotBeRemoved`, `TestAHandoverOfAClosedSetIsRemoved`, `TestAHandoverNamedForAnOpenChildOfAnIdeaSetIsKept` | `internal/le/commit/` | AC-18 | Written, not run |
| `TestSessionStartReportsAStalledSetWhenTheClaimsAnswerInTime`, `TestSessionStartNamesAStartedSetWhenTheClaimsMissTheirDeadline` | `internal/le/hookruntime/` | AC-9 at session start | Written, not run |
| `TestAMarkerWithNoSessionDirectoryIsReaped`, `TestALiveSessionsMarkerIsKept` | `internal/le/session/` | AC-19 | |
| `TestOneGroupingFunctionAnswersEveryReader` (a prefix holding a digit, a lone numbered name that is no set, an umbrella with no number) | `internal/le/spec/status/` | AC-20 | |
| `TestAClaimOfAStartedSpecIsNeverRefusedByTheOpenLimit`, `TestTheLimitRefusalPrintsNoSetting` | `internal/le/spec/` | AC-13, AC-21 | |
| A check that every sentence moved out of a skill body is named by a step of that skill | found or added by package 10 | AC-5, AC-6, AC-14, AC-15 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le ai hooks unit` | `internal/le/ai/hooks/` | The hook fixtures with the new report | |

## Files to Modify
- Named at design, after the probes and the producer reads. The areas are the agent-spawn hook and the budget, the spec claim, the token report, four skill bodies and the rationale pages they will point at, and the test-judge skill. For the spec set: the status inventory and the closure detector, the claim and the release, the stop hook and the session-start summary, the planning rule's point file and `.claude/rules/session-start.md`, and `docs/contributing/spec-workflow.md`, "Spec sets" and "What each closure gate reads".

## Implementation Steps

The audit of 2026-11-15 found nothing built (6 criteria partly present, 15
missing) and split the work into the packages below. Each is one editing
agent on the development tier, compiles on its own and lands as its own
commit. Every package but 10 and 11 touches a package under the hook source
seal and owes an independent review before the seal is refreshed.

| # | Package | Criteria | Where |
|---|---------|----------|-------|
| 1 | The one grouping function; `detectSet`, its pattern and the sibling-stem test deleted | AC-20 | `internal/le/spec/status`; `docs/contributing/spec-workflow.md`, "Spec sets" |
| 2 | The removed-spec record and its cache | AC-8 (source) | the removal judgement moved out of `internal/le/commit` to a package both it and `spec/status` import; the cache in `spec/status` |
| 3 | Claim enumeration, liveness, markers with no directory | AC-19 (reaper half) | `internal/le/le/path`, `internal/le/session` |
| 4 | The derived set state and its two views | AC-8, AC-9 | `spec/status`, the session-start summary |
| 5 | The two limits, and every user of the old cap converted | AC-13, AC-16, AC-17, AC-21 | `internal/le/spec`, the three hookruntime tests that set the old cap, every rule, skill, page and spec that names it |
| 6 | The release answer and the second-claim warning | AC-10, AC-4 | `internal/le/spec` |
| 7 | Umbrella closure, the stop refusal, the two rules; F21 retired | AC-12, AC-11, AC-14 | the closure detector, the stop hook, the planning point file, `.claude/rules/session-start.md` |
| 8 | The handover removal refusal | AC-18 | `internal/le/commit` |
| 9 | The agent after-hook and the budget notices | AC-1, AC-2, AC-3, AC-19 (agent half) | the lifecycle hook, the budget, `ai/agents/ze-work.md` |
| 10 | Skill bodies | AC-5, AC-6, AC-15 | four skill bodies, `ai/rationale/`, the test-judge skill |
| 11 | The kept measure | AC-7 | `internal/le/ai/tokens` |

Packages 1 to 8 run in that order. Packages 9, 10 and 11 are independent of
the chain. Package 7's stop refusal waits on the owner's answer recorded
under "Open with the owner".

Where the packages stand (2026-11-15):

| # | State |
|---|-------|
| 1 | Landed, 81ace1d673. Two review rounds |
| 2 | Implemented and reviewed three rounds. → Decision: set membership comes from the spec files alone; the record of closed specs only adds closed children to a set the files form, so every reader agrees on the sets whether or not it read the record. → Constraint: a number-less parent whose numbered children are all closed is therefore not recognisable as a set; only a `-0-umbrella` file is a set on its own. Package 7's umbrella closure reaches the `-0-umbrella` shape only. → Constraint: nothing calls `LoadClosedSpecs` or `ReadClosedSpecs` yet; package 4 wires them, and its session-start line prints the command that computes the record beside "not computed" |
| 3 | Implemented; the reaper half of AC-19. → Decision: a session is called dead only on proof and is kept, with the verdict cannot tell, when its directory tree or one of its own flat markers was written within one day, the rule the store trim already applied. → Decision: with no Claude process visible the reaper refuses to judge and removes nothing, as the trim does; the three reaper tests whose inputs changed were approved by an independent judge. → Constraint: `ListClaims` and `Judge` walk the tree of every unproven idle directory, so package 4 must measure them before calling them inside the session-start time budget. → Constraint: `spec-le-session-clean-preserves-active-harnesses.md` owns the same surface; this package meets its AC-2, AC-3 and AC-4 and only partly its AC-1 (a quiet live peer of another harness is kept only while it wrote within a day) |
| 4 | Implemented 2026-11-20, tests written first, not yet run: `internal/le/spec/status/setstate.go` (`DeriveSets`, `SessionStartSets`), the action `./le spec status set list` in `answer.go`, the tests in `setstate_test.go`, `docs/contributing/spec-workflow.md` "The state of a set". → Decision: the state has a fifth value, cannot tell, with its reason: claims not read, a holder the reaper cannot judge (every agent's marker under a live parent), an unreadable claim marker, or the record not computed while nothing else shows the set started. Each would otherwise be a silent stalled or idea. → Decision: the set view is its own action, because the inventory's payload is its rows and `| json` and `| count` read them. → Decision: a spec named in a child's Depends cell blocks it only while the population still holds it, and the set's own umbrella never does. → Decision (fix round 2026-11-20): the session-start summary is wired as its own step, `reportSpecSets` in `internal/le/hookruntime/lifecycle.go`, reading the spec files, the git index and the record cache, and running no git. → Decision (final fix round 2026-11-20, after an independent review found the claims always passed as unread, so session start could never report a stalled set): the step reads the claims through `ListClaims` under its own deadline, `sessionStartClaimsBudget` (1 s of the 3.5 s budget), in one goroutine that sends one answer into a one-slot channel and ends with the hook. The main thread measured `./le --name session spec current` at 9.9 s and `spec status set list` at 8.0 s on a loaded host, which is le's startup, so `ListClaims`' in-process cost stays host-dependent and is bounded by the deadline rather than measured. Within it, a stalled set prints with its next child, its claim command and its handover; past it, or on a read error, the claims pass as not read with "claims not read within the session-start budget" (or the error) and `./le spec status set list`, never a silent stalled or idea. Tested by `TestSessionStartReportsAStalledSetWhenTheClaimsAnswerInTime` and `TestSessionStartNamesAStartedSetWhenTheClaimsMissTheirDeadline` (`lifecycle_specsets_test.go`), with `TestTheSessionStartSummaryReadsTheRecordAndRunsNoGit` kept. → Decision: a cannot tell carries what settles it (`SetView.Fix`), the command for its own doubt |
| 5 to 7 | Dropped by the owner decision of 2026-11-20 ("Scope cut" above) |
| 8 | Implemented 2026-11-20, revised after an independent review: `internal/le/commit/handover.go`, called from `Create` after the dependency gate. → Decision: the gate refuses live, stalled and cannot tell as `DeriveSets` answers them; idea and children closed pass. A record of closed specs that cannot be extended is not computed (`ClosedSpecsNotComputed`), never the file's older record. → Decision, the stem rule: a set is tied to a handover by its `SetView.Handovers` (the prefix as contiguous tokens of the handover's name, one-token prefixes included, since a prefix names a family); any open spec, set member or not, is tied when the name holds its whole stem as contiguous tokens and the stem has two tokens or more (`HandoverNamesSpec`), so `vrf-leak` is no handover of spec `vrf`. → Decision (final fix round 2026-11-20, after an independent review): a set member is no longer judged by its set's state alone, since AC-18 protects "the stem of an open spec": a handover named for an open child of an idea set (`...-agent-economy-2-second.md`) is refused. The one spec the stem rule skips is a number-less umbrella, whose stem is the set prefix, so an idea set's own prefix-only handover still passes; tested by `TestAHandoverNamedForAnOpenChildOfAnIdeaSetIsKept`. Stated in `docs/contributing/committing.md`. The claims and record readers are injected in the tests |
| 9 | AC-1 implemented and reviewed: past 600,000 tokens of context, an agent spawn tells the main thread its size and the fresh-session rule. AC-3 (the notices at 60 and 80 calls) is written as a patch and NOT applied: it needs two hunks in `internal/le/hookruntime/runtime.go`, the permission layer refused that edit to the agent, and the main thread does not apply a refused edit on an agent's request. The patch and its two documentation hunks are in the session scratch as `ac3-budget-notice.patch` and `ac3-docs.patch`; the owner decides. AC-2 and the agent half of AC-19 need the SubagentStop event: record one payload first |
| 10 | Landed, 3909e97f96. Two review rounds. The honest saving is 0.8 to 1.9KB a skill: nearly all their text is instruction, and instructions stayed |
| 11 | Implemented and reviewed three rounds as an extension of `./le ai tokens report`: a window (`from`, `days`) and every figure of the umbrella's criteria and baseline table. → Constraint: queue wait per agent or session is not recordable from existing data; the job record has no session or agent column |

## Open Questions for Design
- Whether AC-7's measure is a new action or an extension of the existing token report.
- ~~The SubagentStop payload, to record in package 9.~~ Answered by the scope cut of 2026-11-20: AC-2 and the agent half of AC-19, the only criteria that needed it, are dropped, so no probe is owed.

### Open with the owner

~~The harness fires its stop event at the end of every turn, not only when a
session ends. AC-11 as written therefore refuses a session that pauses to ask
the owner a question in the middle of a spec, until it commits a handover.
The stop hook cannot tell a pause from an abandonment. Package 7 builds
everything else and leaves the stop at today's warning until he chooses.~~
Answered 2026-11-20: AC-11 is dropped for this reason ("Scope cut"), and the
stop stays at today's warning.

~~Package 9's AC-3 patch (the notices at 60 and 80 calls), left for the owner
to decide.~~ Answered 2026-11-20: AC-3 is dropped, and the patch is not
applied.

Settled by the owner on 2026-11-14, and no longer open:

| Value | Decision | Why |
|-------|----------|-----|
| The open limit's starting value | The open count measured on the day the limit lands, lowered as specs close and never raised | Today's cap of 12 would refuse every new claim on the first day: 10 specs are `in-progress` and each started set adds one, so the open count is between about 12 and 26 |
| The active limit | 6 distinct specs under a live claim | Four main sessions held claims in one day, and up to three closures may run beside them |

The landing commit records the measured count as the limit's first value, so the number has one home and its date.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One grouping function in the status package answers every reader. Per set it gives the fields below | Keep the two detectors and add a third for the set state | Two already disagree; a third would be the third copy of one fact |
| Closed children come from the spec removals in git history, held in a cache under the session scratch root keyed by HEAD and extended from the cached commit to HEAD | The umbrella's Children table; the `Spec-work:` trailer; `plan/learned/` | The table is written by hand, a commit without a claim has no trailer, and no learned file exists for the three closed children. A walk on every call costs seconds, and the session-start summary must not run one |
| A claim is live when the reaper's judgment says its session is, and for an agent when the agent has not finished | A new liveness test by marker age | Age cannot tell a long task from a dead session, and the reaper already holds the judgment |
| The stop refusal of AC-11 recognises "where it stands" by a tracked handover whose name carries the spec's stem or its set's prefix, or by the spec's Status moved to `blocked` | Read the handover's text | A hook judges what a file is, never its wording. The set prefix is accepted because the agent-economy handover was named for the set |
| No acknowledgement file for AC-11 | Copy the closure refusal's file | Nothing in the tooling writes that file and the refusal prints the command that creates it, so it is a self-bypass. Whether the closure refusal keeps its own is not changed here |
| A claim that starts no spec is outside the open limit, so the close-only route has no setting. The decisions file's sentence that names `ZE_SPEC_WIP_CAP` is rewritten in package 5 to say so | A second setting that lifts the open limit for one claim | Today's claim already judges only a `ready` spec, so the setting never mattered for a spec whose work is committed; under two limits there is nothing left for it to lift |
| The judgement of which commit closed a spec moves out of `internal/le/commit` into a package below both it and `spec/status` | Call it from `spec/status` | `commit` imports `spec/status`, so the call would be an import cycle |
| When the reaper cannot judge liveness (no projects directory), the active limit says it cannot tell and does not refuse | Count every marker as live; count none | A refusal on a count nobody could take is a wrong answer, and so is a silent zero |
| The removed-spec cache is one file shared by every session under `tmp/session/`. The session-start summary reads it and runs no git: when the cache was computed at another commit than HEAD, the summary prints the state it holds and names that commit. Any `./le spec status` call extends it | A cache per session; a walk at session start | A new session's own scratch is empty, and the session-start hook has a time budget a history walk does not fit |
| The umbrella's owed closure reaches a session three ways: the release of the last child names it, the session-start summary and the status view report it, and the stop hook judges it only when the umbrella itself is claimed | The stop hook judging the set of the last released claim | A released claim leaves nothing for the stop hook to read |
| An agent's claim marker is removed when its agent stops, from the SubagentStop event | Remove it in the Agent call's after-hook | A background agent's Agent call returns at launch (observed 2026-11-15: the call answered while the agent ran), so the after-hook would remove a running agent's marker |
| The transcript tail reader of AC-1 lives in a package under the hook source seal | Extend `internal/le/ai/tokens` and import it from the hook | That package is outside the seal, and what decides a hook's output is sealed |
| The one cap is deleted and every user of it is converted in this work: the claim, the report, the tests, and each rule, skill, page and spec that names `ZE_SPEC_WIP_CAP` or "the WIP cap" | Keep the cap and add a second limit beside it | The owner's two limits count different things, and a third number that counts neither would stay in the tree |

The set, as the grouping function answers it:

| Field | Meaning |
|-------|---------|
| Prefix | The set's name, from the shared file-name prefix |
| Umbrella | The umbrella's path, or none |
| Open children | Each with its number, Status and Depends |
| Closed children | Stems, from the cache of removed specs |
| Held | For each open child and the umbrella: the holder, and whether that claim is live |
| State | idea, live, stalled or children closed |
| Next | The lowest-numbered open child whose Depends are met, or the spec outside the set that blocks it |

Measured 2026-11-15: 10 specs `in-progress` against the cap of 12; 68 claim
markers under `tmp/session/` against 53 session directories.

Owner decisions (2026-11-14), no longer open:

| Decision | What it settles |
|----------|-----------------|
| The open limit takes over today's cap, and starts at the measured open count | The research showed today's value of 12 refuses every new claim on the first day, so the owner set the starting value to the count measured when the limit lands |
| The design as written in this spec | Shown to the owner on 2026-11-14 with its alternatives; he settled its two open values the same day |
| Each started, unclosed child counts as itself in the open limit, and a live set with no such child counts once | The agent-economy set, between children, costs one |
| No acknowledgement route past AC-11 | The refusal binds an agent ending its own turn, and its two ways out are cheap |

## Checklist

### Goal Gates (MUST pass)
- [x] Every criterion the scope cut of 2026-11-20 keeps is demonstrated: AC-1, AC-5, AC-6, AC-7, AC-8, AC-9, AC-15, AC-18, AC-19 (reaper half), AC-20
- [x] The targeted tests pass, run by the main thread: `./le test unit pkg path ./internal/le/commit/`, `./le test unit pkg path ./internal/le/spec/status/` and `./le test unit pkg path ./internal/le/hookruntime/` (one foreign red, Pre-Commit Verification). `internal/le/spec`, `internal/le/ai/tokens` and `./le ai hooks unit` after the seal refresh are owed in this closure's run list. No agent runs any `./le verify` command

### TDD
- [x] Tests written
- [x] Tests FAIL: each fix-round test was written against the code it fixes (AC-9 at session start, AC-18 member stem); the red is the pre-fix behavior each test's PREVENTS line names
- [x] Tests PASS: main-thread runs, Pre-Commit Verification

### Closure
- [x] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it

## Implementation Summary

### What Was Implemented
- Package 4, AC-8 and AC-9: `DeriveSets` and `SessionStartSets` in `internal/le/spec/status/setstate.go`, five states (idea, live, stalled, children closed, cannot tell with its reason and fix), the action `./le spec status set list` (`answer.go`), and the session-start step `reportSpecSets` with `sessionStartClaims` in `internal/le/hookruntime/lifecycle.go`, which reads `ListClaims` under `sessionStartClaimsBudget` (1 s).
- Package 8, AC-18: `handoverRefusal` and `handoverProblem` in `internal/le/commit/handover.go`, called from `Create` (`prepare.go`) after the dependency gate.
- AC-1's lowered threshold: `handoffContextTokens` 400,000 in `internal/le/hookruntime/spawncontext.go` (owner, 2026-11-20), with `.claude/hooks/README.md`, `ai/rationale/context-economy.md` and the planning point file.
- The hook source seal refreshed in `internal/le/ai/hooks/fixtures.go` (`hookSourcesDigest`) over this child's sealed files and f65f6cac5e, which two independent reviews had cleared.
- Packages 1, 2, 3, 10 and 11 landed earlier ("Where the packages stand").

### Bugs Found/Fixed
- Session start passed the claims as unread, so it could never report a stalled set: `TestSessionStartReportsAStalledSetWhenTheClaimsAnswerInTime`.
- A handover named for an open child of an idea set was removed freely: `TestAHandoverNamedForAnOpenChildOfAnIdeaSetIsKept`.
- A data race: the claims goroutine read the package variable a test restores; the reader is now copied before the `go` statement (`sessionStartClaims`), covered by the race-enabled run of `TestSessionStartNamesAStartedSetWhenTheClaimsMissTheirDeadline`.

### Documentation Updates
- `docs/contributing/spec-workflow.md`, "The state of a set" and its readers table (source anchor `setstate.go -- DeriveSets, setState, setNext, SessionStartSets`): the session-start claims deadline.
- `docs/contributing/committing.md`, the handover gate (anchors `handover.go -- handoverRefusal, handoverProblem`, `setstate.go -- HandoverNamesSpec, IsHandover, setHandovers`): the stem rule over every open spec.
- `.claude/hooks/README.md` and `ai/rationale/context-economy.md`: the 400k threshold.
- `./le doc check all`: owed in this closure's run list.

### Deviations from Plan
- The scope cut of 2026-11-20 (owner) dropped AC-2, AC-3, AC-4, AC-10 to AC-14, AC-16, AC-17, AC-21 and the agent half of AC-19.
- AC-1's threshold is 400k, not the 600k its row states: the owner lowered it on 2026-11-20.
- `ListClaims`' cost is bounded by a deadline rather than measured, since it is host-dependent (row 4).

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Session start passed the claims as unread until `ListClaims` was measured | A measurement on one host bounds nothing on another; a deadline bounds the wait on every host | Independent review, BLOCKER | Fixed in `sessionStartClaims` |
| approach | Set members were judged by their set's state alone | AC-18 protects "the stem of an open spec", member or not | Independent review, ISSUE | Fixed in `handoverProblem` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| The main thread is told when its context passes the handoff threshold | Done | `internal/le/hookruntime/spawncontext.go` | 400k by owner decision |
| A set's state is derived, a live set nobody holds is reported as stalled with the child to claim next | Done | `internal/le/spec/status/setstate.go`, `internal/le/hookruntime/lifecycle.go` `reportSpecSets` | |
| Agent call count, target notices, second-claim warning, stop refusal, two limits | Changed | "Scope cut" | Dropped by the owner, 2026-11-20 |
| Skill bodies hold the steps, reference text one read away | Done | package 10, 3909e97f96 | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAgentSpawnReportsContextPastTheHandoffThreshold` (`spawncontext_test.go`) | Threshold 400k |
| AC-5, AC-6, AC-15 | Done | package 10, 3909e97f96 | |
| AC-7 | Done | `internal/le/ai/tokens/criteria_test.go` | package 11 |
| AC-8 | Done | `TestStatusReportsAStartedSetNobodyHoldsAsStalled` and the live, idea and children-closed cases in `setstate_test.go` | |
| AC-9 | Done | `TestTheSessionStartSummaryReadsTheRecordAndRunsNoGit`, `TestSessionStartReportsAStalledSetWhenTheClaimsAnswerInTime`, `TestSessionStartNamesAStartedSetWhenTheClaimsMissTheirDeadline` | |
| AC-18 | Done | `TestAHandoverOfALiveSetCannotBeRemoved`, `TestAHandoverOfAClosedSetIsRemoved`, `TestAHandoverNamedForAnOpenChildOfAnIdeaSetIsKept`, `TestCreateRefusesTheRemovalOfAHandoverOfALiveSet` | |
| AC-19 (reaper half) | Done | `TestAMarkerWithNoSessionDirectoryIsReaped`, `TestALiveSessionsMarkerIsKept` | package 3 |
| AC-20 | Done | `TestOneGroupingFunctionAnswersEveryReader` | package 1, 81ace1d673 |
| AC-2, AC-3, AC-4, AC-10 to AC-14, AC-16, AC-17, AC-21, AC-19 (agent half) | Changed | "Scope cut" | Dropped by the owner |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| AC-8, AC-9 status tests | Done | `internal/le/spec/status/setstate_test.go` | green, race on |
| AC-9 session-start tests | Done | `internal/le/hookruntime/lifecycle_specsets_test.go` | green |
| AC-18 tests | Done | `internal/le/commit/handover_test.go` | green, race on |
| Tests of dropped criteria | Changed | - | Dropped with their criteria |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/le/spec/status/setstate.go`, `answer.go`, `closed.go` | Done | |
| `internal/le/commit/handover.go`, `prepare.go` | Done | |
| `internal/le/hookruntime/lifecycle.go`, `spawncontext.go` | Done | |
| `docs/contributing/spec-workflow.md`, `docs/contributing/committing.md` | Done | |

### Audit Summary
- **Total items:** 21 criteria
- **Done:** 10 (AC-1, 5, 6, 7, 8, 9, 15, 18, 19 reaper half, 20)
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 11 dropped by the owner's scope cut of 2026-11-20 (Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| The main thread is told when its context passes the handoff threshold | unit test over the real hook entry | `TestAgentSpawnReportsContextPastTheHandoffThreshold`, green |
| A live set nobody holds is reported as stalled with the child to claim next | real tree, through the real entry point | `./le --name session spec status set list` on this checkout reported `agent-economy` live, with its closed children, its open children, the next child and its handover (main thread, 2026-11-20); `TestSessionStartReportsAStalledSetWhenTheClaimsAnswerInTime` drives `hookSessionStart` to the stalled line |
| The handover of open work cannot be removed | the real commit entry point | `TestCreateRefusesTheRemovalOfAHandoverOfALiveSet` drives `Create` |
| Skill bodies hold the steps, reference one read away | landed | package 10, 3909e97f96 |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| AC-2, AC-3, AC-4, AC-10, AC-11, AC-12, AC-13, AC-14, AC-16, AC-17, AC-21 | Dropped by the owner, 2026-11-20 ("Scope cut") | None: the owner dropped them |
| AC-19, agent half: an agent's claim marker removed when the agent finishes | Dropped by the owner, 2026-11-20. The real tree shows the cost: 8 agent claim markers on child 3, left by a live session, each read "cannot tell" by the reaper, so none is removed and none makes a set read stalled | None: the owner dropped it; the main thread puts the 8 markers to him |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/agent-economy-4-supervision-f3b4fff4-8d92-47f3-93bd-c9b996d828f0.md` (verdict clean, 16 files, 2026-11-20T18:39:38Z) |
| `./le spec review check` | owed with commit A |
| Rounds | 4 |
| Reviewer lenses used | logic and wiring, concurrency, the AC text against the code, Go style |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | Session start passed the claims as unread, so it never reported a stalled set (AC-9) | `lifecycle.go` `reportSpecSets` | `sessionStartClaims` under `sessionStartClaimsBudget` |
| 2 | ISSUE | A set member was judged by its set's state alone, so a handover named for an open child of an idea set was removed (AC-18) | `handover.go` `handoverProblem` | the stem rule over every open spec, number-less umbrella skipped |
| 3 | ISSUE | The claims goroutine read a package variable a test restores (data race) | `lifecycle.go` `readSessionStartClaims` | the reader copied before the `go` statement |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/le/spec/status/setstate.go`, `setstate_test.go` | Yes | `git status`: untracked, present |
| `internal/le/commit/handover.go`, `handover_test.go` | Yes | `git status`: untracked, present |
| `internal/le/hookruntime/lifecycle_specsets_test.go` | Yes | `git status`: untracked, present |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-8, AC-9 | The set state and its two views | `internal/le/spec/status` unit tests green, race on (main thread, 2026-11-20) |
| AC-9 | Session start reads the claims under its deadline | `internal/le/hookruntime` green except `TestAgentDefinitionsCarryTheHardBansOfTheInstructions`, red at HEAD since e8206daeda changed `ai/INSTRUCTIONS.md` without re-copying the agent definitions: another session's, not this work's |
| AC-18 | The handover gate | `internal/le/commit` unit tests green, race on |
| AC-1 | The 400k threshold | `grep -n "handoffContextTokens = 400_000" internal/le/hookruntime/spawncontext.go` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `./le spec status set list` | none: le tooling, no `.ci` suite covers it | Run on the real tree by the main thread: agent-economy live, closed and open children, next child, handover |
| The session-start hook | none | `TestSessionStartReportsAStalledSetWhenTheClaimsAnswerInTime` calls `hookSessionStart` |
| `./le commit create` | none | `TestCreateRefusesTheRemovalOfAHandoverOfALiveSet` calls `Create` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| none | - | The spec declares no A-N row; the one open measure (`ListClaims` at session start) is bounded by `sessionStartClaimsBudget` instead |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `spec-workflow.md` readers table | `sessionStartClaims`, `sessionStartClaimsBudget` in `lifecycle.go` | Yes |
| `committing.md` handover gate | `handoverProblem` in `handover.go` | Yes |
| Lint | `internal/le/spec/status` clean; `internal/le/commit` and `internal/le/hookruntime` refused to plan while another session edited L2TP files they depend on; an earlier `internal/le/commit` run found only the two `rangeValCopy` findings, since fixed, and a pre-existing `unparam` in `changechecks.go` | Owed again |
| `./le ai hooks unit` | red before the seal refresh only on the stale digest | Owed again |
