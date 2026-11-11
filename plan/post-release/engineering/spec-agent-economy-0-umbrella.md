# Spec: agent-economy-0-umbrella

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P1 |
| Release reason | Release-independent development tooling: it changes no shipped binary, and it is ordered first because every other spec is paid for in the tokens and wall time it removes. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Agents developing Ze burn tokens and wall time on the development system
itself rather than on the product. This umbrella holds the 2026-11-11
measurement, the owner's decisions, and the acceptance criteria for the five
child specs that remove the waste. It modifies no file itself.

### Owner permissions for this work (2026-11-11)

This permission is for this umbrella and its children only. It is not a
standing rule.

| Permission | What it allows |
|------------|----------------|
| WIP cap exception | 13 specs were in progress against a cap of 12. The umbrella's children may be claimed above the cap, each claim setting the cap one above the current count. No other spec uses this. |

That this main session supervises the umbrella while its agents claim and run
the child specs, each under its own agent claim, is no longer a permission of
this umbrella: a standing owner rule now allows it for every spec.

### Owner decisions (2026-11-11)

| # | Decision | Owning child |
|---|----------|--------------|
| D-1 | `ai/META-DECISIONS.md` splits into a part every agent loads and a part only the main thread loads | 3 |
| D-2 | The always-on core shows only each member rule's always-on points and names the file for the rest. The bans and the precedence ladder stay | 3 |
| D-3 | The Go style gate accepts a short core page, and "read in full" applies to that page. Long tables move to on-demand sections | 3 |
| D-4 | The `go build` guard allows a scoped compile of packages that produce no binary, and no longer exempts a whole-tree build | 1 |
| D-5 | Scope: one umbrella with the five children below. Child 5 runs last, after child 2's wait-time record shows whether it is worth its risk | all |

### Baseline (three weeks to 2026-11-11)

Measured from the session transcripts under the Claude project directory:
78 main sessions, 1,653 subagents, about 100,000 model turns, about $6,500
recorded cost. The two scripts that produced the figures were session scratch
and are not kept; the method is: per assistant message, sum the `usage` fields
once per message id; per tool call, pair the call with its result and take the
timestamp difference.

| Measure | Value |
|---------|-------|
| Input tokens re-read from cache | 16.3 billion (12.1 billion in subagents) |
| Output tokens | 65 million |
| Context at a subagent's first turn | median 53k tokens |
| Context at a main session's first turn | median 78k tokens |
| Subagent turns and tool calls | median 52 turns, 59 calls |
| Subagents reaching 100 calls | 344 of 1,656 |
| Main session context | average turn 353k; peak median 186k, one in ten above 744k |
| Tool calls | Bash 83,037; Edit 12,111; Read 10,735; Write 2,005 |
| Failed tool calls | Bash 3,310; Edit 1,171; Write 174 |
| Shell file reads (`grep`, `sed`, `cat`) | 27,118, of which 4,704 repeat the previous call's file |
| `./le help <area>` lookups | about 800 |
| `./le commit create` calls for scripts produced | 2,980 for 1,383 |
| Foreground test and build wall time | 102 hours in 9,742 runs, plus 1,375 background runs |

The fixed preamble is the subagent start (53k) times every subagent turn
(87,859) plus the main start times every main turn: about a third of all
input tokens. In a subagent, 109KB of it is instruction files and 28KB is the
skill listing.

Refusals by text, with counts:

| Refusal | Count |
|---------|-------|
| Expensive command piped through a filter | about 610 |
| Design doors not read (Edit, Write) | about 500 |
| Auto mode gave no verdict because a hook changed the call's input | about 490 |
| Shell write to `plan/` or `ai/rules/` | about 380 |
| Producing source not read before a spec edit | about 300 |
| `go build` without `-o bin/` | about 280 |
| Agent past its 100-call budget | about 170 |
| Test deletion asks for approval | about 160 |
| `/tmp` access | about 130 |
| Raw `go test` outside job admission | about 95 |
| Derived file could not be rebuilt | about 90 |

Test and build runs, foreground:

| Command | Runs | Hours | Median |
|---------|------|-------|--------|
| `./le job run` | 3,596 | 53.7 | 28s |
| `./le repo compiles` | 401 | 11.5 | 107s |
| `./le go ...` | 412 | 5.8 | 12s |
| `./le test bgp` | 667 | 5.3 | 14s |
| `./le rfc check` | 439 | 4.8 | 29s |
| raw `go vet`, `go test`, `go build` | 1,857 | 7.8 | under 10s, 707 failed |

Forced reads: the Go style guide (49KB) 858 times, the system door (11KB)
865 times, each then carried for the rest of the agent's turns.

## Children

| # | Spec | Owns | Depends |
|---|------|------|---------|
| 1 | spec-agent-economy-1-call-shape (closed) | The Bash hook's command rewrite, quote-blind guards, the budget's counting, refusal messages that teach a forbidden form, what a subagent is told at start | - |
| 2 | spec-agent-economy-2-scoped-test (closed) | A short action that tests, vets or compiles named packages; persisted queue wait; nearest-name answer for an unknown command; commit-create ergonomics | 1 (the hook messages name this action) |
| 3 | `plan/post-release/engineering/spec-agent-economy-3-preamble.md` | D-1, D-2, D-3; the skill listing served to phase agents; main-only `.claude/rules/` files | - |
| 4 | `plan/post-release/engineering/spec-agent-economy-4-supervision.md` | Context-size nudge at agent spawn; a warning on a second spec claim; the agent's call count reported to the main thread; skill bodies shed branch-only reference text; test-judge batching | 1 (same hook file) |
| 5 | `plan/post-release/engineering/spec-agent-economy-5-scoped-gates.md` | A cheaper post-commit compile gate and a changed-only RFC check, CI keeping the whole-corpus runs | 2 |
| 6 | `plan/post-release/engineering/spec-agent-economy-6-shell-parser.md` | The pre-tool Bash hook reads a command line with one bash parser (`mvdan.cc/sh/v3/syntax`) in place of the hand splitter and the byte allow-list; a reader line through loops, conditionals, groups and substitutions; a heavy job refused wherever the line runs it; the after-line walk's wrapper options, base-name match and `set`; the raw-job refusal's quiet form | 1 (the hook it replaces) |

### Model tier per child (owner, 2026-11-11)

Tiers, not model names: more than one harness implements these specs. The
development tier is the harness's high-capability model; no work here runs on
a cheaper tier. The strongest tier is used where an error would be silent or
would reach every session.

| Child | Implementation | Design and review | Why |
|-------|----------------|-------------------|-----|
| 1, phase 1 (identity check) | development | strongest | A wrong check blocks every subagent or lets one act under another's identity |
| 1, phases 2 to 4 | development | development | Well specified; the existing refusal tests catch a regression |
| 2 | development | development | Joins parts that exist; its failures are loud |
| 3 | development | strongest | Which rules an agent stops loading is judgment, and a dropped rule fails silently |
| 4 | development | development; strongest review for the skill-body trimming | A step lost from a skill shows only as worse work later |
| 5 | development | strongest | The only child that changes what a gate proves |
| Test-change judging | - | development | Independence of context matters more than tier |

Existing specs this work meets:

| Spec | Relation |
|------|----------|
| `plan/post-release/engineering/spec-rules-situation-index.md` (skeleton) | Proposes the per-point `applies` predicate. Child 3 lands its first slice (D-2) and names what stays there |
| `plan/post-release/engineering/spec-native-action-job-admission.md` (skeleton) | Every heavy action admits. Child 2 lands the wait record first, so its cost is measurable |
| `plan/post-release/engineering/spec-docs-read-before-authoring.md` (design) | Adds forced whole-page reads. Its design adopts child 3's core-page acceptance before it is implemented |
| `plan/post-release/engineering/spec-gates-report-cause-and-progress.md` (design) | Gate failure cause and progress. Does not cover hook refusals; no overlap |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
  → Constraint: a feature registers and is discovered; no central list learns a name. The hook checks stay registered in the one gate map.
- [ ] `ai/doors/le-tooling.md` - the door for `le`, the hooks and the test trees
  → Constraint: the hooks run on every Bash, Write and Edit call of every session, so a hook that refuses wrongly or panics blocks every session at once.
  → Constraint: the shared `le` is built from HEAD, so a hook change reaches agents only once it is committed; an uncommitted edit is judged with a named build.
- [ ] `ai/rules/context-economy.md` - the rules this work makes mechanical
  → Decision: batching, the 60-call package, the 100-call handoff and one read per file are already rules. The baseline shows prose alone does not hold them, so each child changes a mechanism, never only a sentence.
- [ ] `ai/rules/planning.md` - how phases and agents are run
  → Constraint: review runs in a context that did not produce the work; each child keeps its own Review Gate.

**Key insights:**
- Spend is turn count times context size. Children 1 and 4 cut turns; children 3 and 4 cut context; children 2 and 5 cut wall time.
- Child 1 lands first: once the command rewrite is gone, every later agent stops losing Bash calls.

## Current Behavior (MANDATORY)

**Source files read:** the producers each child changes are listed in that child. The umbrella's own claims rest on these:
- [ ] `internal/le/hookruntime/session.go` - `writeParentSessionPrefix` prepends the session-identity exports to every subagent Bash command and returns it as changed input
- [ ] `internal/le/hookruntime/budget.go` - `agentCallBudgetCheck` counts every hooked call of an editing agent, a refused one included
- [ ] `internal/le/hookruntime/runtime.go` - `Run` runs every check of an action, then the prefix rewrite when none blocked
- [ ] `internal/le/hookruntime/bash.go` - the Bash guards split a command on separators without regard for quotes
- [ ] `internal/le/hookruntime/lifecycle.go` - `hookSubagentContext` tells a subagent about 1.5KB of constraints and nothing of batching, the budget or what counts as a read

**Behavior to preserve:**
- Every guard keeps judging what a command, path or file is, and stays fail-closed.
- A subagent's `le` call never runs under the main thread's identity.
- The always-on rules a phase agent acts on stay loaded in that agent.

**Behavior to change:**
- Only what the children's acceptance criteria name.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Native hook checks | `internal/le/hookruntime/runtime.go`, `nativeHookActions` | One registry of every pre and post tool check | Extends: children 1 and 4 change checks in place and register nothing outside it |
| Agent call budget | `internal/le/hookruntime/budget.go`, `agentCallBudgetCheck` | Refuses an editing agent's 101st hooked call | Extends: child 1 changes what is counted, child 4 reports the count |
| Reached-package selector | `./le repo changed scope print both` | Names the packages and tags a change reaches | Extends: child 2's short action and child 5's scoped gates consume it |
| Job admission and quiet output | `./le job run ... quiet` | Admits a heavy job and prints one line on green | Extends: child 2 puts it behind a short action and records the wait |
| Derived always-on core | `./le ai rules update condensed` | Renders `ai/rules/CORE.md` and `ai/rules/TRIGGERS.md` from the precedence ladder | Extends: child 3 renders a member's always-on points instead of its whole body |
| Read gates | `ai/doors/README.md`, the door and style gates | Refuse an edit until the covering pages were read whole | Extends: child 3 points the style gate at a core page |

## Data Flow (MANDATORY)

### Entry Point
- A tool call of an agent, as the hook payload the harness sends to `le ai hooks <name>`.
- The instruction files the harness loads at an agent's start.

### Transformation Path
1. The harness assembles an agent's start from the instruction files, the skill listing and the SubagentStart hook's text.
2. Each tool call passes the pre-tool hook, runs, then passes the post-tool hook.
3. Each result returns to the model, which re-reads the whole context on the next turn.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Harness ↔ hook | JSON payload in, exit code and JSON response out | Yes, `internal/le/hookruntime/runtime.go` `Run` |

### Changed-Wire Census

Each child holds the census for the wire it changes. The umbrella changes none.

**Search evidence:** the umbrella edits no producer; child 1 records the census for the hook response and the session-identity variables.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| none | writer | none: the umbrella edits no file | children hold their own | preserved |
| none | reader | none: the umbrella edits no file | children hold their own | preserved |

### Integration Points
- Each child names its own.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | every change stays inside the hook registry, the rule generator or a registered `le` action |
| No unintended coupling | Yes | no product package is touched |
| No duplicated functionality | Yes | see Existing Mechanisms |
| Zero-copy preserved where applicable | Yes | not applicable to tooling |
| Registration over hardcoding, outbound | Yes | new actions register through `leroot` |
| Registration over hardcoding, inbound | Yes | no list learns a new name |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A subagent Bash call through `le ai hooks pretool-bash` | → | child 1's identity check | `TestSubagentReaderCommandPassesUnchanged` |
| per child | → | per child | each child's Wiring Test table |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| per child | per child | per child | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| per child | per child | per child | |

## Files to Modify
- None. The children carry every edit.

## Files to Create
- The five child specs named under Children.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- child 1, whose wiring test is the row above
2. **Phase: children 2, 3 and 4** -- in parallel, each under its own agent claim
3. **Phase: child 5** -- after child 2's wait record exists
4. **Phase: measure** -- repeat the baseline method and fill the umbrella's acceptance criteria

## Acceptance Criteria

Measured the same way as the baseline, over the two weeks after the last
child closes.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A phase agent's first turn | Its context is at most 35k tokens | The system prompt, the tool schemas and the rules a phase agent acts on stay: they are the agent's contract |
| AC-2 | Subagent Bash calls | Fewer than 1 in 200 fails with "auto mode gave no verdict" | A call the classifier refuses on its own judgment stays refused: that verdict is not ours to remove |
| AC-3 | Failed tool calls | At most 2% of Bash calls and 4% of Edit and Write calls fail | A refusal of a real violation stays: the guards keep judging what the command is |
| AC-4 | Editing agents | At most 8% reach the 100-call budget | A package that proves too large still hands off: the budget stays |
| AC-5 | Main sessions | No session peaks above 600k tokens | A session whose owner keeps it open after the nudge is his choice |
| AC-6 | One package's tests | One command of at most six words runs them with the right tags and answers in one line on green | The long `./le job run ... command` form stays for commands the short action does not cover |
| AC-7 | Queue wait | Each admitted run's wait is recorded beside its run time | Runs that never queue record zero: that is the answer, not an absence |

## Known Limitations

- Whether the harness can serve a subagent fewer instruction files than the
  main session is unverified. Child 3 probes it before it designs on it.
- The baseline scripts were not kept. Child 4 owns a kept, repeatable measure
  if the umbrella's acceptance criteria need one.

## Checklist

### Goal Gates (MUST pass)
- [ ] Every child spec closed, or its remaining work a spec of its own named here
- [ ] AC-1..AC-7 measured and recorded with the measurement window

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
