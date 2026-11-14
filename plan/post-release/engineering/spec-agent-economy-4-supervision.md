# Spec: agent-economy-4-supervision

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P1 |
| Release reason | Release-independent development tooling: it changes how the main thread is told about context size and agent cost and changes no shipped binary. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

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
closed, one sat `ready` and unclaimed, two stayed skeletons, the session that
ran them ended, and nothing reported it. Every safeguard against abandoned
work reads one spec whose Status is `in-progress` under a live claim. An
umbrella never reaches that state, and the one detector that blocks a stop
exempts it by name, so a set of specs has no owner once its session ends.

The second goal: the state of a spec set is derived from its children and
from the claims held on them, never declared in the umbrella. A set that has
started and not finished is reported as live, a live set nobody holds is
reported as stalled with the child to claim next, and a session that worked
a set does not stop inside it without a committed handover.

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

Unverified, to probe before designing on it:

| Question | Probe |
|----------|-------|
| Does a non-blocking message from a PostToolUse or PreToolUse hook reach the model? | Emit additional context from a named build and read the next turn |
| Does the Agent tool's PostToolUse payload, or a SubagentStop payload, carry the agent id? | Record one payload into scratch |
| Is the transcript flushed before a pre-tool hook runs? | Compare the transcript tail with the call in hand |

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
- The close-only claim above the cap keeps working.
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
| Stop hook | `internal/le/hookruntime/lifecycle.go`, `hookStop` | Refuses a stop on an implemented spec that is not closed | Extends: the same refusal covers a session that stops inside a live set with no committed handover |
| Stale-skeleton flag and status inventory | `internal/le/spec/status/specstatus.go`, `skeletonStale`; `report.go` | Lists every spec by bucket and Status and flags old skeleton ideas | Extends: the inventory gains the set view. The stale flag is not the carrier: it measures age, and a stalled set is stalled on its first day |
| WIP list | `internal/le/spec/session.go`, `wip` | Lists `in-progress` specs, stalest first, when a claim meets the cap | Extends: shows live sets beside them |
| Committed handover | `plan/handover/` | Holds a handover once a session writes one | Extends: becomes what a session owes when it stops inside a live set |
| Change-set trailer | `./le commit create`, the `Spec-work: <stem>` line | Ties each commit to the spec it was made under | Extends: with the spec's removal commit it is the record of which children closed, in place of the hand-written Children table |

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

### Integration Points
- The agent-spawn hook, the transcript usage parser, the agent call counter, the spec claim, the rationale pages.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| PostToolUse of an Agent call with a transcript past the threshold | → | the context-size report | `TestAgentSpawnReportsContextPastTheHandoffThreshold` |
| `./le spec claim` of a second spec in one main session | → | `Claim` | `TestClaimWarnsOnASecondSpecInOneMainSession` |
| `./le spec status` over a set with a closed child, an open child and no claim | → | the derived set state | `TestStatusReportsAStartedSetNobodyHoldsAsStalled` |
| The Stop event of a session that claimed a child of a live set, with no handover committed | → | `hookStop` | `TestStopInsideALiveSetIsRefusedWithoutACommittedHandover` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The main thread spawns an agent while its last recorded context is above 600k | The hook tells it the size and to write the state file and hand off | Below the threshold it reports nothing: a message on every spawn is the noise this removes |
| AC-2 | An editing agent finishes | The main thread is told that agent's call count | A read-only agent has no counter and reports none |
| AC-3 | An editing agent reaches 60 and 80 counted calls | It is told its count, its target of 60 and its budget of 100 | The call itself passes |
| AC-4 | A main session claims a second spec after releasing a first | The claim answers with a warning naming the first spec and the fresh-session rule | Agent claims and the close-only claim are not warned |
| AC-5 | `ze-implement`, `ze-review`, `ze-close`, `ze-rfc` | Each body holds every step it held, and its branch-only reference text sits in a page the step names | None lost: every moved sentence is one read away |
| AC-6 | A phase that owes several test judgments | One judge agent takes them in one brief | A judgment that must not see another's context stays alone |
| AC-7 | `./le ai tokens` over a window of transcripts | Reports the umbrella's baseline figures | None |
| AC-8 | A spec set with at least one child closed or claimed and at least one child still open | The status view reports the set as live and names its closed children, its open children and the next one to run, whatever Status the umbrella declares | A set none of whose children was ever claimed or closed stays an idea and is reported as today |
| AC-9 | A live set on which no session and no agent holds a claim | The status view and the session-start summary report it as stalled, with the next child and the command that claims it | A set whose every open child waits on a named spec outside the set reports that spec instead: there is nothing to pick up |
| AC-10 | A session releases its claim on a child of a set whose other children are still open, or closes that child | The release answers with the remaining children, the next one, and the handover owed before the session stops | Releasing the last open child reports the umbrella's own criteria as what remains |
| AC-11 | A session that claimed an umbrella or a child of a live set stops while children remain open and no tracked handover for the set was committed after its last commit for the set | The stop is refused, naming the set, the next child and the handover to write | A session that claimed nothing in the set stops freely, and so does one whose set has no open child |
| AC-12 | An umbrella all of whose children are closed | The closure detector reports it as owing its own acceptance criteria and its closure, and the stop hook treats it as an implemented spec that is not closed | A child's closure record alone still never makes the umbrella a closure candidate |
| AC-13 | A claim that meets the WIP cap, and the WIP report | The list shows live sets beside the `in-progress` specs, stalled ones first | A set shown there holds no slot unless the owner decides it does (Open Questions) |
| AC-14 | The planning rule and the session-start rule | The planning rule states what a session owes when it stops inside a live set: the committed handover naming the next child. The session-start rule points at it and no longer reads as an order to stop with nothing left behind | One spec per session stays the rule: the next child still runs in a fresh session |
| AC-15 | The implement skill's review loop | It states the round limit the planning rule states: five rounds are the session's, the sixth is the owner's | The sentence that each round covers less than the one before stays |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAgentSpawnReportsContextPastTheHandoffThreshold` | `internal/le/hookruntime/` | AC-1 | |
| `TestClaimWarnsOnASecondSpecInOneMainSession` | `internal/le/spec/` | AC-4 | |
| `TestStatusReportsAStartedSetNobodyHoldsAsStalled` | `internal/le/spec/status/` | AC-8, AC-9 | |
| `TestStopInsideALiveSetIsRefusedWithoutACommittedHandover` | `internal/le/hookruntime/` | AC-11 | |
| others | named at design | AC-2, AC-3, AC-7, AC-10, AC-12, AC-13 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le ai hooks unit` | `internal/le/ai/hooks/` | The hook fixtures with the new report | |

## Files to Modify
- Named at design, after the probes and the producer reads. The areas are the agent-spawn hook and the budget, the spec claim, the token report, four skill bodies and the rationale pages they will point at, and the test-judge skill. For the spec set: the status inventory and the closure detector, the claim and the release, the stop hook and the session-start summary, the planning rule's point file and `.claude/rules/session-start.md`, and `docs/contributing/spec-workflow.md`, "Spec sets" and "What each closure gate reads".

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the three probes, recorded; the failing wiring tests
2. **Phase: the context-size and call-count reports**
3. **Phase: the claim warning**
4. **Phase: the spec set** -- the derived state first, then each reader of it: status view, session-start summary, release, WIP list, closure detector, stop hook; then the two rules and the workflow page
5. **Phase: skill bodies and test-judge batching**
6. **Phase: the kept measure**

## Open Questions for Design
- The three probes above decide whether the reports are additional context or a refusal at the next spawn.
- Whether AC-7's measure is a new action or an extension of the existing token report.
- How a closed child is known. The child's file is gone, and the umbrella's Children table is a hand-written copy, so it cannot be the source. The candidates are the commit that removed the spec together with the `Spec-work:` trailer, or the closure record the detector already reads. The answer must come from what git holds.
- How the handover for a set is recognised. A hook judges what a file is, never its wording, so the candidate is a tracked file under `plan/handover/` whose name carries the set's prefix, committed after the session's last commit for the set.
- For the owner: does a live set hold a WIP slot? If it does, the cap counts work that is started and unfinished, and a set held open for a two-week measurement blocks a slot for that time. If it does not, the cap keeps counting claims only and the set is visible without costing one.
- For the owner: may a stop inside a live set be acknowledged without a handover, as the closure refusal can be today, and by whom?

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
- [ ] The targeted tests pass, run by the main thread: the packages `internal/le/hookruntime`, `internal/le/spec`, `internal/le/spec/status` and `internal/le/ai/tokens`, and `./le ai hooks unit`. No `./le test unit <group>` covers `internal/le`. No agent runs any `./le verify` command

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
