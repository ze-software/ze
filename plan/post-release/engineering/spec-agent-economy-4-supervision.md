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

**Behavior to preserve:**
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

## Data Flow (MANDATORY)

### Entry Point
- The PostToolUse payload of an Agent call in the main thread.
- `./le spec claim <spec>`.

### Transformation Path
1. The hook reads the transcript's tail for the last recorded context size and the finished agent's counter.
2. It returns both to the main thread as additional context, with the handoff instruction when the size passes the threshold.

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

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAgentSpawnReportsContextPastTheHandoffThreshold` | `internal/le/hookruntime/` | AC-1 | |
| `TestClaimWarnsOnASecondSpecInOneMainSession` | `internal/le/spec/` | AC-4 | |
| others | named at design | AC-2, AC-3, AC-7 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le ai hooks unit` | `internal/le/ai/hooks/` | The hook fixtures with the new report | |

## Files to Modify
- Named at design, after the probes and the producer reads. The areas are the agent-spawn hook and the budget, the spec claim, the token report, four skill bodies and the rationale pages they will point at, and the test-judge skill.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the three probes, recorded; the failing wiring tests
2. **Phase: the context-size and call-count reports**
3. **Phase: the claim warning**
4. **Phase: skill bodies and test-judge batching**
5. **Phase: the kept measure**

## Open Questions for Design
- The three probes above decide whether the reports are additional context or a refusal at the next spawn.
- Whether AC-7's measure is a new action or an extension of the existing token report.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] The targeted tests pass, run by the main thread: the packages `internal/le/hookruntime`, `internal/le/spec` and `internal/le/ai/tokens`, and `./le ai hooks unit`. No `./le test unit <group>` covers `internal/le`. No agent runs any `./le verify` command

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
