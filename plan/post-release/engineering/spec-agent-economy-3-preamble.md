# Spec: agent-economy-3-preamble

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | tooling |
| Priority | P1 |
| Release reason | Release-independent development tooling: it changes which instruction text an agent loads and changes no shipped binary. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-11 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 3 of `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`,
which holds the baseline and the owner's decisions D-1, D-2 and D-3.

A subagent starts at a median 53k tokens and re-reads that on every one of
its about 52 turns. 109KB of the start is instruction files and 28KB is the
skill listing. An editing agent then reads the 49KB Go style guide because a
gate requires it, and carries that too. Most of this text governs the main
thread or another kind of work than the agent's.

The goal: a phase agent loads the rules it acts on and nothing else, and the
gates that force a read are satisfied by a short page that holds the rules a
writer applies.

## Research Findings (2026-11-11, read at the producer by a research agent; re-read at design)

| Finding | Where |
|---------|-------|
| `ai/INSTRUCTIONS.md` is copied to `AGENTS.md`; skills and agent definitions are mirrored; a root `CLAUDE.md` is deleted because it would shadow `AGENTS.md`. `.claude/rules/*.md` are hand-kept | `internal/le/ai/sync/ai.go`, `Mirror.generateInto`, `Mirror.Sync` |
| The three `@` includes are static lines, and the same four files are measured against a 40,000-token budget that passes at about 23.7k | `ai/INSTRUCTIONS.md`; `internal/le/ai/rules/artifacts.go`, `payloadParts`; `digest.go`, `tokenBudget` |
| Core membership is derived per rule: rungs 1 and 2 of the ladder, the ladder itself, and rules no task would surface | `internal/le/ai/rules/digest.go`, `coreMembers`, `coreReason` |
| A core block is the whole rule: the condenser keeps every bold directive, and every directive is bold | `internal/le/ai/rules/artifacts.go`, `ruleBlock`, `condenseBody` |
| Core section sizes: rfc-compliance 14.5KB, rule-precedence 9.2KB, git-safety 6.5KB, principles 5.6KB, interop 3.6KB, never-destroy 2.6KB, documentation 1.1KB | `ai/rules/CORE.md` |
| The subagent start hook injects about 1.5KB and no rule text; nothing gives a subagent a smaller instruction set | `internal/le/hookruntime/lifecycle.go`, `hookSubagentContext` |
| A phase agent acts on about 11 to 12KB of the 34KB of `ai/META-DECISIONS.md`; the rest governs the main thread, the website, labs, releases or reports (the researcher's judgment by section) | `ai/META-DECISIONS.md` |
| Four `.claude/rules/` files (7KB) are main-thread procedures | `.claude/rules/session-start.md`, `post-compaction.md`, `planning.md`, `memory.md` |
| Agent definitions use `name`, `description`, `tools`. Both phase agents list `Skill`. The repo's skill descriptions are 6.5KB of the 28KB listing; the rest is harness and plugin skills | `ai/agents/ze-read.md`, `ai/agents/ze-work.md` |
| The style gate refuses a Go edit with no marker for the style page; the door gates work the same way. A marker is written only for a whole read, is keyed per session and per agent, and holds no content hash | `internal/le/hookruntime/writeedit.go`, `writeStyleGuideRead`, `writeDesignDoorsRead`; `readgate.go`, `documentReadMarker`; `lifecycle.go`, `hookSourceRead` |
| The style page is 51KB: reader rules 38KB, tool-enforced rules 6.3KB, differences from standard Go 5.7KB; 15.9KB is table rows | `docs/contributing/ze-go-style.md` |
| `spec-rules-situation-index` proposes a per-point `applies` predicate, and the point field `stage` is reserved | `plan/post-release/engineering/spec-rules-situation-index.md`; `docs/contributing/rule-authoring.md` |

Unverified, to probe before designing on it:

| Question | Probe |
|----------|-------|
| Does removing `Skill` from an agent's `tools:` remove the skill listing from its start? | Edit a definition, sync, spawn the agent in a new session, measure its first-turn context |
| Can the harness serve a subagent fewer instruction files than the main session? | The harness documentation, then the same measure |
| Can a `.claude/rules/` file be scoped to the main thread? | The same |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/le-tooling.md` - the door for the hooks and the rule tooling
  → Constraint: a gate that refuses wrongly blocks every session; the read gates stay fail-closed and per agent.
- [ ] `ai/rules/rule-format.md` and `docs/contributing/rule-authoring.md` - how a rule is authored
  → Constraint: a rule is edited in its point file; `ai/rules/CORE.md` and `ai/rules/TRIGGERS.md` are generated, never hand-edited.
- [ ] `ai/rules/rule-precedence.md` - the ladder that derives core membership
  → Constraint: membership stays derived at rule level. Rungs 1 and 2 are stop rungs, so the bans, the ladder table and the read-the-RFC-text directive stay in the core. A Claude-only file is never a rule's only home.

**Key insights:**
- The derivation says which rules are core, and says nothing of how much of each the core shows. D-2 changes the showing, not the membership.
- `ai/META-DECISIONS.md` says of itself that every main session loads it. D-1 keeps that true and stops subagents carrying the main-only part.

## Current Behavior (MANDATORY)

**Source files read:** the main thread read one producer; the rest of the Research Findings table is owed a read at design.
- [ ] `internal/le/hookruntime/lifecycle.go` - `hookSubagentContext` injects eight constraint lines, the branch, the claimed spec and the parent session; it reads no agent type and carries no rule text

**Behavior to preserve:**
- Every rule a phase agent acts on still reaches it before it acts.
- The main session still loads every owner decision.
- A harness that reads `AGENTS.md` alone still finds every rule file named there.
- The read gates stay per agent: a parent's read never counts for a child.

**Behavior to change:**
- D-1, D-2 and D-3, and the skill listing if the probe confirms the lever.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Derived core and triggers | `internal/le/ai/rules/artifacts.go`, `ruleBlock` | Renders each member rule whole | Extends: renders a member's always-on points and names the file and trigger for the rest (D-2), as the first slice of `spec-rules-situation-index` |
| Instruction mirror | `internal/le/ai/sync/ai.go` | Copies instructions, skills and agents to each harness | Extends: carries the second decisions file if D-1 needs the mirror to know it |
| Session-start report | `internal/le/hookruntime/lifecycle.go`, `sessionStartSteps` | Prints the session's status block to the main thread | Candidate carrier for main-only text (D-1), to be decided at design |
| Read gates and markers | `internal/le/hookruntime/readgate.go` | Refuse an edit until the gated page was read whole | Extends: the style gate's page becomes the core page (D-3) |
| Agent definitions | `ai/agents/` | Fix each phase agent's tool list | Extends: drops `Skill` if the probe shows it removes the listing |

## Data Flow (MANDATORY)

### Entry Point
- The harness assembles an agent's start from `AGENTS.md` and its includes, `.claude/rules/`, the skill listing and the start hook's text.

### Transformation Path
1. `./le ai rules update condensed` renders the core and the trigger index from the rule points.
2. `./le ai sync update` mirrors instructions, skills and agent definitions.
3. The harness loads the result at each agent's start.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Repository ↔ harness | instruction files and agent definitions on disk | No |

### Integration Points
- The rule renderer, the instruction mirror, the read gates and their markers, the agent definitions.

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le ai rules update condensed` | → | the core renderer | `TestTheCoreShowsOnlyAMembersAlwaysOnPoints` |
| a Go edit after reading the style core | → | the style gate | `TestStyleGateAcceptsTheCorePage` |
| a spawned phase agent | → | its start | a recorded first-turn context of at most 35k tokens |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A phase agent's start | Holds the every-agent decisions and not the main-thread ones | The main session holds both: it supervises |
| AC-2 | The rendered core | Shows each member's always-on points and one line naming its file and trigger | The bans, the ladder table and the read-the-RFC-text directive stay shown in full |
| AC-3 | A rule point with no always-on marker | Is shown: an absent marker never narrows | None |
| AC-4 | A Go edit by an agent that read the style core whole | Passes the style gate | An agent that read neither page is still refused |
| AC-5 | The style core | Holds every rule a writer applies; the long tables and the tool-enforced list are on-demand sections it names | None dropped: every rule is in one of the two |
| AC-6 | A phase agent's first turn | At most 35k tokens | The system prompt, tool schemas and the rules it acts on |
| AC-7 | A harness reading `AGENTS.md` alone | Finds both decisions files named there | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTheCoreShowsOnlyAMembersAlwaysOnPoints` | `internal/le/ai/rules/` | AC-2, AC-3 | |
| `TestStyleGateAcceptsTheCorePage` | `internal/le/hookruntime/` | AC-4 | |
| others | named at design | AC-1, AC-5, AC-7 | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le ai hooks unit` | `internal/le/ai/hooks/` | The gate fixtures with the core page | |

## Files to Modify
- Named at design, after the probes and the producer reads. The areas are the rule renderer and the point files of three rules, the decisions file and the instructions that include it, the agent definitions, the style page and its gate, and the main-only rules files.

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the three probes, recorded; the failing wiring tests
2. **Phase: the skill listing and the main-only rules files** -- if the probes confirm the levers
3. **Phase: the decisions split (D-1)**
4. **Phase: the core shows always-on points (D-2)**
5. **Phase: the style core and its gate (D-3)**

## Open Questions for Design
- How the main thread loads the main-only decisions: a second include, the session-start report, or a main-scoped rules file. The probes decide.
- Which points of rfc-compliance, git-safety and rule-precedence are always-on. The owner's rule: the bans and the ladder stay.
- Where the cut in the style page falls, section by section.
- `plan/post-release/engineering/spec-docs-read-before-authoring.md` adopts the core-page acceptance before it is implemented.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] The targeted tests pass, run by the main thread: the packages `internal/le/ai/rules`, `internal/le/ai/sync` and `internal/le/hookruntime`, and `./le ai hooks unit`. No `./le test unit <group>` covers `internal/le`. No agent runs any `./le verify` command

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
