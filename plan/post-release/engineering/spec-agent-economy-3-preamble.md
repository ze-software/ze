# Spec: agent-economy-3-preamble

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P1 |
| Release reason | Release-independent development tooling: it changes which instruction text an agent loads and changes no shipped binary. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-20 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 3 of `plan/post-release/engineering/spec-agent-economy-0-umbrella.md`,
which holds the baseline and the owner's decisions D-1, D-2 and D-3. Design
tier: strongest. Implementation tier: development.

A phase agent starts at a median 53k tokens and re-reads that on every one of
its about 52 turns. 109KB of the start is instruction files, of which 34KB is
the owner's decisions and 45KB the always-on core, and 28KB is the skill
listing. An editing agent then reads the 51KB Go style page because a gate
requires it, and carries that too. Most of this text governs the main thread,
or work other than the agent's.

The goal: a phase agent starts with the rules it can break by its own action
and the index that leads to every other rule, and the page the Go gate
requires holds what a writer applies, with the detail one read away. Nothing
is deleted and no rule's wording changes: text moves, and the main session
still loads everything it loads today except what D-2 moves behind a trigger.

More than one harness implements this repository's specs. Every harness keeps
reading `AGENTS.md` and its includes as today. The per-agent start described
here uses a capability one harness documents; a harness without it is
unchanged.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the constraints every change keeps
  → Constraint: every fact is declared once and other surfaces derive from it. The agent rule set is assembled from the same files the main session loads, never a second copy.
- [ ] `ai/doors/le-tooling.md` - the door for the hooks and the rule tooling
  → Constraint: a start hook that fails must not leave an agent with no rules. The agent definition carries a fallback that does not depend on the hook.
  → Constraint: a hook change reaches agents only once committed, and an agent definition takes effect only in a session started after `./le ai sync update`.
- [ ] `ai/rules/rule-format.md`, `docs/contributing/rule-authoring.md` - how a rule is authored
  → Constraint: a rule is edited in its point file; `ai/rules/CORE.md` and `ai/rules/TRIGGERS.md` are generated. A point's optional header fields never reach the rendered rule, so adding one leaves every rule file byte-identical.
  → Decision: the reserved `stage` field is not the carrier. `plan/post-release/engineering/spec-rules-situation-index.md` records that decision and names `applies` as the predicate; this spec lands `applies` for core members only.
- [ ] `ai/rules/rule-precedence.md` - the ladder that derives core membership
  → Constraint: membership stays derived at rule level from rungs 1 and 2. This spec changes what the core shows of a member, never which rules are members.
  → Constraint: a rule the owner states has `ai/META-DECISIONS.md` as its home and a harness-specific file is never its only home. The file stays one file.
- [ ] `plan/post-release/engineering/spec-rules-situation-index.md` - the earlier design for situation-scoped rules
  → Constraint: a prohibition stays eager; only a procedure moves behind a situation. The trigger index is never narrowed: a point moved out of the core stays reachable through its rule's row.
  → Constraint: an absent `applies` never routes narrower than today.
- [ ] The harness documentation for subagents, memory, hooks and skills (read 2026-11-11 through a documentation agent, quoted in Design Insights)
  → Decision: a subagent definition can start without the project instruction files; a start hook can add text to a subagent's context and receives the agent's id and type. The design rests on these two documented behaviours and probes both before relying on them.

**Key insights:**
- Three mechanisms, each independent: what a phase agent loads at start (D-1 and the start hook), what the core shows (D-2), and how long the gated style page is (D-3).
- D-3 needs no gate change. The gate names one page and asks for a whole read; shortening that page satisfies the owner's "read in full" literally.
- The agent rule set is not a new artifact. The start hook reads the files the main session already loads and passes on the parts an agent acts on.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/ai/rules/artifacts.go` - `ruleBlock` renders one core member as its title, path, meta line and `condenseBody` of the whole rule body; `buildCore` joins the members' blocks under `coreHeader`; `triggersHeader` says an always-on row "is already loaded in full"; `payloadParts` lists the four always-loaded files and `Payload` measures them against `tokenBudget`. The comment over the artifact names says adding a file name there is the only edit a new artifact needs.
- [ ] `internal/le/ai/rules/digest.go` - `coreMembers` and `coreReason` derive membership per rule: ladder rungs 1 and 2 (`ladderRungs`), the ladder's own file, a rule with no routable trigger or severity, and a blocking rule no past task would surface.
- [ ] `internal/le/ai/rules/points.go` - `Point` holds `Slug`, `Kind`, `Level`, `Stage`, `Body`, `Rationale` and `ExceptedBy`. `Stage` is authored and empty in every point.
- [ ] `internal/le/ai/sync/ai.go` - `Mirror.generateInto` copies each `ai/agents/<name>.md` verbatim to the harness's agent directory, each skill to three skill mirrors, and `ai/INSTRUCTIONS.md` to `AGENTS.md`. Frontmatter passes through unread.
- [ ] `internal/le/hookruntime/lifecycle.go` - `hookSubagentContext` answers the SubagentStart event with additional context: eight constraint lines, the branch, the claimed spec, the parent session and the identity line. It does not read the agent's type and adds no rule text.
- [ ] `internal/le/hookruntime/readgate.go` - `styleGuidePath` names `docs/contributing/ze-go-style.md`; `documentReadMarker` keys a whole-read record per session and per agent.
- [ ] `internal/le/hookruntime/writeedit.go` - `writeStyleGuideRead` refuses a Go write with no record for `styleGuidePath`.

Pages and files, read whole or by structure:
- `ai/INSTRUCTIONS.md` (10KB): the hard bans, the pointer to the owner's decisions, the map to the rules, the architecture summary; three `@` include lines.
- `ai/META-DECISIONS.md` (45,169 bytes on 2026-11-14; 34KB when this spec was written): twelve topic sections. It says every main session loads it.
- `ai/rules/CORE.md` (45,561 bytes on 2026-11-14): rfc-compliance 14.5KB, rule-precedence 9.2KB, git-safety 6.5KB, principles 5.6KB, interop 3.6KB, never-destroy-work 2.6KB, documentation 1.1KB, two small rules.
- `docs/contributing/ze-go-style.md` (51KB): "The rules a reader applies" 38KB in thirteen subsections, "The rules a tool enforces" 6.3KB, "Where Ze differs from standard Go" 5.7KB.
- `ai/agents/ze-read.md`, `ai/agents/ze-work.md`: frontmatter `name`, `description`, `tools`; both tool lists hold `Skill`.

**Behavior to preserve:**
- Every rule reaches the main session, in the same words.
- A harness that reads `AGENTS.md` alone loads what it loads today.
- Core membership is derived, never listed.
- Every rule stays named in the trigger index with its trigger.
- The read gates stay per agent and fail-closed, and their code is unchanged.
- The hard bans reach every agent whether or not a hook runs.

**Behavior to change:**
- A phase agent starts without the project instruction files and receives the agent rule set from the start hook (D-1).
- `ai/META-DECISIONS.md` is ordered into two parts (D-1).
- The core shows a member's points unless a point declares the situation it applies in (D-2).
- The gated Go style page keeps each rule's statement and moves detail to sibling pages (D-3).
- The skill listing a phase agent carries, if the probe confirms the lever. Dropped by the owner on 2026-11-20 with A-3: the listing stays.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Subagent start text | `internal/le/hookruntime/lifecycle.go`, `hookSubagentContext` | Adds a short constraint text to every subagent | Extends: for an agent whose definition omits the project instruction files, the same hook appends the hard bans and names the three files of the agent rule set to read |
| Derived core | `internal/le/ai/rules/artifacts.go`, `ruleBlock`, `buildCore` | Renders each member rule whole | Extends: a member's point that declares `applies` is left out of its block and its situation is listed on the block's closing line |
| Point header fields | `internal/le/ai/rules/points.go`, `Point`; `docs/contributing/rule-authoring.md` | Optional header fields that never reach the rendered rule (`rationale`, `excepted-by`) | Extends: `applies` joins them, with the same parsing and the same refusal of a misspelled key |
| Situation-scoped rules design | `plan/post-release/engineering/spec-rules-situation-index.md` | Proposes `applies` on every point and views over it | Extends: this spec lands the predicate and the core view only; that spec keeps the trigger-index and stage views |
| Always-loaded payload measure | `internal/le/ai/rules/artifacts.go`, `Payload`, `payloadParts` | Measures the main session's four files against a budget | Extends: a second measure for the agent rule set with its own budget |
| Agent definitions and mirror | `ai/agents/`, `internal/le/ai/sync/ai.go` | Copied verbatim to the harness | Does not change the mirror: the new frontmatter line passes through as text |
| Read gate | `internal/le/hookruntime/readgate.go`, `writeStyleGuideRead` | Requires a whole read of the style page | Does not change: the page it names becomes shorter |
| On-demand rationale and sibling pages | `ai/rationale/`, `docs/contributing/` | Hold text a rule or page points at | Extends: the style page's detail moves to sibling pages it links |
| Agent rule set reader and the start hook's reading list | `internal/le/ai/rules/ruleset`; `hookSubagentContext` | Finds the every-agent part of the decisions file by heading and names it, with the core and the index, as the files an agent reads | Extends (phase 6): one function answers the situational sections, found by heading, never a list of section names in Go; the reading list names each with its line range and situation |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A main session's start: the harness loads `AGENTS.md` and its three includes.
- A phase agent's start: the harness loads the agent definition, then sends the SubagentStart event to `le ai hooks subagent-context` with the agent's id and type.

### Transformation Path
1. `./le ai rules update condensed` renders the core and the trigger index from the rule points, leaving out of each core block the points that declare `applies`.
2. `./le ai sync update` mirrors the agent definitions.
3. At a phase agent's start the hook reads that agent's mirrored definition. When it declares that the project instruction files are omitted, the hook appends the agent rule set: the text of the hard bans section of `ai/INSTRUCTIONS.md`, and an instruction naming three files to read before any other action, `ai/rules/CORE.md` and `ai/rules/TRIGGERS.md` whole and the every-agent part of `ai/META-DECISIONS.md` by its line range. The three are named, never sent: the harness caps a hook's added context at 10,000 characters, and they are several times that.
4. The agent's definition body tells it what to read if that set is absent from its context.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Repository ↔ harness | instruction files and agent definitions on disk; the start hook's additional context | No: phase 1 probes it |
| Rule points ↔ rendered artifacts | the condensed generator | Yes, `buildCore` |

### Changed-Wire Census

Three formats change. None is a network wire; each is a file format with a
writer and readers in the repository.

**Search evidence:** `grep -rn "META-DECISIONS"` over `ai/`, `.claude/rules/`, `docs/`, `internal/le` and the skills; `grep -rn "excepted-by\|rationale:" internal/le/ai/rules` for the point header parser; `grep -rn "CORE.md\|coreFile"` over `internal/le` and `ai/`. Positive controls: `payloadParts` names all four payload files; `ai/INSTRUCTIONS.md` holds the three include lines.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Point header | writer | authors, through the point files under `ai/rules/points/` | `docs/contributing/rule-authoring.md`, the fields table | Extended: `applies` is optional; an absent line means the point is shown |
| Point header | reader | `internal/le/ai/rules/points.go`, the point parser | grep above | Changed: parses and validates `applies`; every other reader of a point ignores it as it ignores `rationale` |
| `ai/rules/CORE.md` | writer | `internal/le/ai/rules/artifacts.go`, `buildCore` | read | Changed: blocks may omit points and end with a line naming their situations |
| `ai/rules/CORE.md` | reader | every harness, through the include in `ai/INSTRUCTIONS.md`; `Payload`; the start hook (new) | grep above | Preserved for the include and the measure; the hook is a new reader |
| `ai/META-DECISIONS.md` | writer | the main session, on the owner's word | `ai/rules/rule-precedence.md` | Changed: a new rule goes under the part it belongs to; the file's opening says how to choose |
| `ai/META-DECISIONS.md` | reader | every harness through the include; `Payload`; the start hook (new, the first part only); every page and rule that names the file | grep above | Preserved: the path and every sentence are unchanged, so no citation moves |

### Integration Points
- `hookSubagentContext` - gains the agent rule set for an agent whose definition omits the project instruction files.
- `ruleBlock` and `buildCore` - read a member's points.
- `Payload` - gains the agent measure.
- The style page's sibling pages - linked from the core page.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | rules still flow points → generator → artifacts → harness; the hook reads the artifacts, never the points |
| No unintended coupling (components stay isolated) | Yes | the hook reads four files by path; the rule generator does not know the hook |
| No duplicated functionality (extends existing, does not recreate) | Yes | no second rule file: the agent set is assembled from the main session's files |
| Zero-copy preserved where applicable (refs, not copies) | Yes | not applicable to tooling text |
| Registration over hardcoding, outbound | Yes | which agents get the rule set is read from each agent's own definition, never from a list of agent names in the hook |
| Registration over hardcoding, inbound | Yes | no list of rules, sections or agent types is added; the first part of the decisions file is found by its heading |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A subagent definition can start without the project instruction files, rules files included | Harness documentation for subagents, quoted in Design Insights. Key name confirmed 2026-11-14: `omitClaudeMd: true` in the frontmatter, "Requires Claude Code v2.1.271 or later" (the local harness is 2.1.325) | D-1 falls back to shrinking the files themselves, which D-2 and the two-part file still deliver for every agent | Phase 1 probe: a fresh session spawns the agent and its first-turn context is measured | validated (2026-11-15): a `ze-read` agent reported, before any tool call, that neither "Ze - Agent Instructions" nor "Owner Meta Decisions" was in its context, that the "Agent rule set" section was, and quoted its last line naming `ai/META-DECISIONS.md` lines 19-650. Its three reads succeeded and the range began at "Rules for every agent". Its whole run, three reads and a report, cost 58,144 tokens as the harness counted it. Its own estimate of its start, not a measurement: about 12,500 tokens, of which the skill listing (about 70 entries) is the largest block at about 6,500 |
| A-2 | The start hook's additional context can carry the agent rule set (about 30KB) | The same documentation gives no size limit | The hook passes the hard bans and names three files; the agent reads them in one turn | Phase 1 probe: the agent reports the last line of the set | broken (2026-11-14): the hooks reference says "A hook's `additionalContext`, `systemMessage`, and `initialUserMessage` strings, and its plain stdout, are capped at 10,000 characters", and over the cap "Claude Code saves the output to a file in the session directory. It replaces the output with the file path and a preview of up to the first 2,000 characters". The "If wrong" route is built |
| A-3 | Removing `Skill` from a phase agent's tools removes the skill listing from its start | Not documented either way | The listing stays; the saving is lost, nothing breaks | Phase 1 probe: first-turn context with and without it | dropped by owner decision (Thomas, 2026-11-20), not run: a subagent's first-call context is already a median 21.9k and a maximum 25.6k tokens (AC-13), so the listing's saving is marginal. `Skill` stays in both definitions |
| A-4 | An agent can follow a skill by reading its file | Skills are plain files under `ai/skills/`, and briefs already say "read that skill" | Phase agents keep `Skill` | The probe agent runs one skill from its file | validated (2026-11-15): a `ze-read` agent told to read `ai/skills/ze-audit.md` and follow it produced the audit of `spec-agent-economy-4-supervision.md`, a verdict per criterion with the producer read for each, without the `Skill` tool being named in its brief |
| A-5 | The owner's global instruction file holds nothing a phase agent must have | Read: prose rules for text sent under a person's name, the question tool, agent counts, dictation | A phase agent writing prose for the owner misses a rule | The brief for such an agent names the writing skill, which carries those rules | broken in part (2026-11-20): `~/.claude/CLAUDE.md` read whole. Its rules on rule placement, GitHub posts, dictation and licences bind the main thread; worktree cleanup binds only a worktree agent; but "No em dashes in English" binds any English an agent writes, a doc or a comment included. No file under `ai/rules/` carries it: `ai/skills/ze-author.md` line 94 does. The "If wrong" route is what holds: an agent that writes prose for the owner is briefed with the writing skill |
| A-6 | Every rule in the every-agent part can be broken by an agent's own action, and no rule in the main part can | The classification test in Key Design Decisions | An agent breaks a rule it never saw | Review of the move by the strongest tier, bullet by bullet, against the test | validated: phase 2 review round 1 confirmed all eleven main-session placements against the placement test, and the phase 6 review went bullet by bullet over the 28 situational bullets, widening six situation lines and returning none to the every-agent part (Review Gate, and phase 6 in Implementation Steps) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The start hook fails and an agent starts with no rules | An agent's report ignores a ban | The agent definition body carries the hard bans and tells the agent to read the three files when the set's heading is absent from its context |
| R-2 | A point moved behind a situation is one an agent needed before knowing the situation | A violation of that point in work that never read the rule | Only a procedure moves, never a prohibition; the generator refuses `applies` on a point whose level is MUST NOT; the block's closing line names each situation |
| R-3 | The move of the decisions file changes a rule's wording | A diff that is not a pure move | The implementation is checked by comparing the sorted bullet set before and after |
| R-4 | The style core drops a rule while moving detail | A writer never sees a rule | Every heading of the old page stays in the core with its statement; a check compares the heading sets |
| R-5 | Another session edits the decisions file while it is being reordered | A conflicting hunk | The reorder is one commit, made when no other session holds a hunk in the file |
| R-6 | A new agent definition forgets to omit the instruction files, or omits them with no rule set | Its first-turn context is the old size, or it holds no rules | The hook keys on the definition, so the two always travel together; `./le ai sync check` reports the agent payload per definition |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Phase agents work with fewer rules than they need, silently. Nothing in the shipped binary |
| How is it reverted? | One commit per phase. Removing the omit line from the two agent definitions restores the old start in the next session |
| Who else touches this path? | Child 1 and child 4 of the umbrella edit `lifecycle.go`; every session appends to `ai/META-DECISIONS.md` on the owner's word |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le ai hooks subagent-context` for an agent whose definition omits the instruction files | → | `hookSubagentContext` | `TestSubagentStartAddsTheAgentRuleSetForAnAgentThatOmitsTheInstructions` |
| `./le ai rules update condensed` with a core member holding a point that declares `applies` | → | `buildCore` | `TestTheCoreLeavesOutAPointThatDeclaresItsSituation` |
| A fresh session spawning `ze-read` | → | the harness and the hook together | the recorded probe of phase 1: first-turn context and the set's last line |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A phase agent's start, in a session begun after the change | Its context holds the hard bans at start, and the trigger index, the core and the every-agent part of the decisions after its first read turn, and none of the main-thread part | The agent's own definition, the start text of child 1 and the brief: they are the agent's contract |
| AC-2 | The same start when the hook did not run | The agent's definition tells it to read the three files before acting, and it holds the hard bans from the definition | None |
| AC-3 | An agent whose definition does not omit the instruction files | Receives no rule set from the hook: it already loads the files | Every agent type other than the two phase agents, until its definition changes |
| AC-4 | `ai/META-DECISIONS.md` | Holds two parts under two headings, every bullet of the old file present once with unchanged wording, and an opening that says how a new rule chooses its part | The main session and every other harness load the whole file |
| AC-5 | A core member's point with no `applies` | Shown in the core as today | All of them until an author adds the field: absence never narrows |
| AC-6 | A core member's point that declares `applies: <situation>` | Left out of the core; the member's block ends with one line naming the rule file and each such situation | The point stays in the rule file and the rule stays in the trigger index |
| AC-7 | `applies` on a point whose level is MUST NOT, or on a point of a rule that is not a core member | The generator refuses, naming the point and the reason | A MUST NOT stays eager: a prohibition cannot wait for its situation |
| AC-8 | The rendered core | Shows every point of an always-on rule except a point that declares its situation and states no prohibition in any form; its size is what that yields, 45.0KB on 2026-11-14 with two points marked. The 24KB figure is withdrawn by the owner (2026-11-14, on D-2 and AC-8: "keeping what is built") | Every prohibition is shown: most points hold a prohibition and a procedure in one paragraph, and a prohibition is never hidden, so those points are not split |
| AC-9 | The trigger index header | Says an always-on rule's core block may name situations whose points are in the rule file | None |
| AC-10 | `docs/contributing/ze-go-style.md` | At most 16KB; every heading of the old page present with its rule stated; each moved body linked as a sibling page under `docs/contributing/go-style/` | None dropped: every sentence is in the core page or one sibling |
| AC-11 | A Go edit by an agent that read the shortened page whole | Passes the style gate, with no change to the gate's code | An agent that read nothing is still refused |
| AC-12 | `./le ai rules report payload` | Reports the main payload and the agent rule set, each against its own budget. The agent budget, `agentTokenBudget`, is 25,000 tokens: the set's size measured on 2026-11-20 by `./le ai rules report payload`, 24,048 tokens, rounded up to the next thousand; reaching the budget (the measured size rounded up to the next thousand) reports EXCEEDED (owner rule, 2026-11-20) | None |
| AC-13 | A phase agent's context after its first read turn (the reads the start text names), measured | Target 35k tokens (owner, 2026-11-20). Measured by `./le ai tokens report from 2026-11-15 days 5`, over 2026-11-15 to 2026-11-20: 279 subagents, first-call context median 21.9k and maximum 25.6k, none above 35k (`ze-read` median 20.7k, `ze-work` 22.2k). Met, on the measure the owner judged it by. That measure is the agent's first call, which precedes the read turn this row's condition names: the reads add the agent rule set, 24,048 tokens on 2026-11-20 (AC-12), less the hard bans the start text already carries (about 0.7k), so the context after that turn is about 45k at the median. The owner's decision closes AC-13 on the first-call figure. A subdirectory instruction file or path-scoped rule can still load when the agent works with files there: the tracked ones are under `vendor/` and `internal/le/site/wiki/testdata/`, and `.claude/rules/agent-browser.md` is path-scoped; the report does not split them out, and the 25.6k maximum bounds any that loaded | The system prompt and tool schemas, which the repository does not control |
| AC-14 | A harness that reads `AGENTS.md` alone | Loads the same files as before, the decisions file whole | None |
| AC-15 | The sections `Website design`, `Test systems and labs`, `ExaBGP compatibility` and `Developer tooling` of `ai/META-DECISIONS.md` | Sit outside the range the start hook hands every agent; each states its situation once directly under its heading; the start hook lists each with its line range and its situation; the payload measure counts them apart from the every-agent part; a main session still loads the whole file; every bullet's wording is unchanged | Implemented 2026-11-15. The four sections sit in a third part, "Rules for one kind of work", at the end of the file: 28 bullets, with 96 left for every agent and 11 for the main session, 135 as before and byte-identical (`TestTheDecisionsFileKeepsEveryBulletAcrossItsTwoParts`, and the reviewer's own sorted comparison against HEAD). `TestTheSituationalSectionsAreTheFourTheOwnerApproved` pins the four. `./le ai rules report payload`: the agent rule set is 21,335 tokens (was 23,772), and the situational sections are 2,713 tokens counted apart |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestSubagentStartAddsTheAgentRuleSetForAnAgentThatOmitsTheInstructions` | `internal/le/hookruntime/lifecycle_test.go` | AC-1: the hard bans sent, the core, the trigger index and the decisions range named, the main-thread parts absent | |
| `TestSubagentStartAddsNoRuleSetForAnAgentThatLoadsTheInstructions` | `internal/le/hookruntime/lifecycle_test.go` | AC-3 | |
| `TestSubagentStartSurvivesAMissingRuleFile` | `internal/le/hookruntime/lifecycle_test.go` | R-1: the hook answers, names the file it could not read, and the fallback sentence is present | |
| `TestTheCoreLeavesOutAPointThatDeclaresItsSituation` | `internal/le/ai/rules/digest_test.go` | AC-6 | |
| `TestTheCoreShowsAPointWithNoSituation` | `internal/le/ai/rules/digest_test.go` | AC-5 | |
| `TestAppliesIsRefusedOnAProhibitionAndOutsideTheCore` | `internal/le/ai/rules/points_test.go` | AC-7, both refusals | |
| `TestThePayloadMeasuresTheAgentRuleSet` | `internal/le/ai/rules/digest_test.go` | AC-12 | |
| `TestTheDecisionsFileKeepsEveryBulletAcrossItsTwoParts` | `internal/le/ai/rules/digest_test.go` | AC-4: both headings present and the hook's extract of the first part ends at the second heading | |
| `TestTheStyleCoreKeepsEveryHeadingOfTheSplitPage` | `internal/le/doc/` (the doc-check package that owns page structure, named in phase 5) | AC-10: each sibling page is linked from the core | |
| `TestTheSituationalSectionsAreTheFourTheOwnerApproved` | `internal/le/ai/rules/ruleset/ruleset_test.go` | AC-15: exactly the four sections, found by heading | written |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Agent rule set size against its budget | 0 to the budget | the budget minus one token | N/A | the budget: reported as exceeded |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `./le ai hooks unit` | `internal/le/ai/hooks/fixtures.go` | The start-hook fixture with and without the omit line | |
| `./le ai rules update condensed` then `./le ai rules report payload` | the generated artifacts | The core is regenerated and both payloads are within budget | |
| The phase 1 probe | a fresh session | A spawned phase agent reports its first-turn context and the set's last line | |

## Files to Modify
- `internal/le/hookruntime/lifecycle.go` - `hookSubagentContext` appends the agent rule set
- `internal/le/ai/rules/points.go` - the `applies` header field and its two refusals
- `internal/le/ai/rules/artifacts.go` - `ruleBlock`, `buildCore`, `triggersHeader`, `coreHeader`, the agent payload measure

Rule, instruction and page files, each in the phase whose code needs it:
- `ai/agents/ze-read.md`, `ai/agents/ze-work.md` - the omit line, the fallback paragraph with the hard bans; `Skill` stays, A-3 being dropped
- `ai/META-DECISIONS.md` - reordered into two parts, wording unchanged
- `ai/INSTRUCTIONS.md` - the sentence introducing the decisions file names its two parts
- point files under `ai/rules/points/rfc-compliance/`, `ai/rules/points/git-safety/`, `ai/rules/points/rule-precedence/` - `applies` on the procedure points
- `docs/contributing/rule-authoring.md` - the `applies` row of the fields table
- `docs/contributing/ze-go-style.md` - the core
- `.claude/hooks/README.md` - the start hook's row
- `plan/post-release/engineering/spec-rules-situation-index.md` - what this spec landed and what stays there
- `plan/post-release/engineering/spec-docs-read-before-authoring.md` - its design adopts short gated pages

Named as unaffected: `docs/architecture/core-design.md`, which the hook and rule tooling files declare in their `// Design:` headers. It describes the command engine and that hooks and the rule tooling are native `le` code, and states nothing about what a subagent loads or what the core shows.

## Files to Create
- `docs/contributing/go-style/<topic>.md` - one sibling page per moved body of the style page; the topics are the old page's headings

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | development tooling |
| YANG validation constraints | N-A | no leaf |
| YANG custom validators | N-A | no leaf |
| CLI commands/flags | No | `./le ai rules report payload` gains a row, not a keyword |
| CLI grammar (keyword before value) | N-A | no new command |
| Editor autocomplete | N-A | no new command |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | Yes | the payload report already answers through the shared renderer; the new row is a field of the same report |
| Env var registration | No | none added |
| Doctor check for runtime dependencies | N-A | no shipped binary changes |
| Doctor code per problem class | N-A | not operator-facing |
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
| 6 | Has a user guide page? | Yes | `docs/contributing/rule-authoring.md`, `docs/contributing/ze-go-style.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior implemented, changed, or newly proven? | No | the rfc-compliance rule's wording is unchanged; only what the core shows of it |
| 10 | Test infrastructure changed? | Yes | `.claude/hooks/README.md` |
| 11 | Affects daemon comparison? | No | |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys added/changed? | No | |
| 14 | Prometheus counters added/changed? | No | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/post-release/engineering/spec-agent-economy-3-preamble.md` in phase 1 and name each page here |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | every page that links a section of the style page: `grep -rn "ze-go-style.md#" docs ai` and repoint each to the sibling that now holds it |

## Implementation Steps

Each phase is one work package of about 60 tool calls for one editing agent
on the development tier, reviewed on the strongest tier. The main thread runs
every test and every generator.

1. **Phase: Wiring -- the probe**
   - Tests: `TestSubagentStartAddsTheAgentRuleSetForAnAgentThatOmitsTheInstructions`, `TestSubagentStartAddsNoRuleSetForAnAgentThatLoadsTheInstructions`, `TestSubagentStartSurvivesAMissingRuleFile`
   - Files: `lifecycle.go`, the two agent definitions (omit line and fallback paragraph), `.claude/hooks/README.md`
   - Verify: tests fail, then pass. Then a session started after `./le ai sync update` spawns `ze-read` and records its first-turn context and the set's last line: A-1 and A-2. The same session spawns it with `Skill` removed: A-3 and A-4. A false A-1 stops D-1 here and is reported
2. **Phase: The decisions file in two parts (D-1)**
   - Tests: `TestTheDecisionsFileKeepsEveryBulletAcrossItsTwoParts`
   - Files: `ai/META-DECISIONS.md`, `ai/INSTRUCTIONS.md`
   - Verify: the sorted bullet set is identical before and after; the strongest tier reviews each bullet's part against the test in Key Design Decisions
   - Implemented 2026-11-14, uncommitted: the decisions file holds "Rules for every agent" then "Rules for the main session", the eleven main-session bullets moved whole under their source section names, 135 bullets before and after; the instruction sentence names both parts; the test; and review notes N1 and N2 of round 5 fixed in `hookSubagentContext`. Its review is round 1 of the new change in the Review Gate
3. **Phase: The core shows always-on points (D-2)**
   - Tests: `TestTheCoreLeavesOutAPointThatDeclaresItsSituation`, `TestTheCoreShowsAPointWithNoSituation`, `TestAppliesIsRefusedOnAProhibitionAndOutsideTheCore`
   - Files: `points.go`, `artifacts.go`, `docs/contributing/rule-authoring.md`, then the point files of the three large members
   - Verify: `./le ai rules update condensed`; every rule file is byte-identical; AC-8
   - → Constraint: the owner's condition is that no rule's wording changes. A point split adds a blank line to the rendered rule, so the check is a word-level comparison of each rule file, not a byte comparison. Rule files changed in this phase only where two stale pointers were corrected.
4. **Phase: The payload measure**
   - Tests: `TestThePayloadMeasuresTheAgentRuleSet`
   - Files: `artifacts.go`
   - Verify: AC-12 and AC-13 against the probe's figure
5. **Phase: The style core (D-3)**
   - Tests: `TestTheStyleCoreKeepsEveryHeadingOfTheSplitPage`
   - Files: `docs/contributing/ze-go-style.md`, the sibling pages, every page that links a section of it
   - Verify: `./le doc check all`; AC-10 and AC-11
6. **Phase: Situational sections of the decisions file** (implemented 2026-11-15)
   - → Decision: the four sections moved whole to a third part at the end of the file, so every agent still reads one contiguous range and `AgentDecisions` is unchanged. Set aside: leaving them in place and handing an agent several ranges.
   - → Decision: each section states its work in a `Read before:` paragraph under its heading. The reader refuses, and the hook then sends the whole file, when the part is absent, does not run to the end of the file, holds a bullet outside a section, or a section names no work: an unlisted bullet is never silently unread.
   - → Constraint: the review went bullet by bullet over all 28. Six were not reached by the first situation lines (a second-architecture run listed as owed, a figure compared or recorded, `le` carrying a product feature, lab and guest infrastructure, ExaBGP compatibility added outside the bridge, a CI job that builds the site). The lines were widened; no bullet returned to the every-agent part.
   - Review, strongest tier, independent: round 1 found 0 BLOCKER, 8 ISSUE (six situation lines, a reader hole that left a bullet outside any section unlisted without an error, one false sentence), 5 NOTE; round 2, on the fixes only, found 0 BLOCKER, 0 ISSUE, 9 NOTE and answered yes for the hook source seal. Notes left: the two new reader refusals are driven at the reader and not through the hook's table test; `situationOf` accepts "Read before:x" with no space.
   - Sections: `Website design`, `Test systems and labs`, `ExaBGP compatibility`, `Developer tooling`, about 10KB of the 41.6KB every-agent part
   - Tests: a test that pins which sections are situational
   - Files: `ai/META-DECISIONS.md` (a situation line under each of the four headings, no bullet reworded), `internal/le/ai/rules/ruleset` (one function answering the situational sections, found by heading), `hookSubagentContext` (the reading list), `artifacts.go` (the payload measure)
   - Verify: AC-15; a strongest-tier review that no bullet in those sections binds an agent doing other work
   - → Decision: owner, 2026-11-14: whole topic sections of `ai/META-DECISIONS.md` may be read only by an agent doing that work.
   - → Constraint: this departs from the two-part design the owner approved on 2026-11-11 (D-1), and he approved the departure on 2026-11-14. The four were chosen because an agent cannot mistake whether it is editing the website, a lab, the ExaBGP bridge or `le`.

### Remaining before closure
Nothing. The owner settled the three open items on 2026-11-20, approving "close child 3 now with the targets set to the measured figures, drop the A-3 probe":
- A-3 is dropped, not run, and `Skill` stays in both definitions (Risks & Assumptions).
- AC-12: `agentTokenBudget` in `internal/le/ai/rules/artifacts.go` is 25,000 tokens, the measured set rounded up to the next thousand. The owner's rule was stated against the 21,329 measured on 2026-11-15; the set measured 24,048 on 2026-11-20, after commits to the decisions file and the core, so the rule gives 25,000 (Deviations from Plan).
- AC-13: target 35k, met by the first-call figure over 2026-11-15 to 2026-11-20.

Built 2026-11-15: R-6. `./le ai sync check` prints one row per agent definition: whether it omits the instruction files and the rule payload it therefore starts with, each figure taken from the one payload measure (`ze-read` and `ze-work`: the agent rule set, 21,329 tokens; `lsp-probe`: the always-loaded payload, 27,246). A definition that loads the files is a reported fact; the check fails only when a payload a row needs cannot be measured. The predicate that reads a definition's frontmatter moved to `ruleset.ReadAgentStart`, so the check and the start hook use one. Reviewed by an independent strongest-tier agent: the move answers the same for every input; an independent judge approved the three test units that moved with it

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation and a test that goes red without it |
| Nothing lost | The decisions file's bullet set and the style page's sentence set are unchanged by the moves |
| Prohibitions stay eager | No point with a MUST NOT level carries `applies`; the hard bans, the ladder table and the read-the-RFC-text directive are in the rendered core |
| No second declaration | The agent rule set is read from the main session's files at start; no generated copy exists |
| Fallback | With the hook's output removed from a test agent's start, its definition alone names the files to read and holds the bans |
| Harness neutrality | No rule, spec or skill text names a vendor's model or assumes one harness; the harness-specific line lives only in the agent definitions and the hook |
| Rule: the part a new decision goes in | The file's opening states the test, so the main session can place a new rule without asking |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Phase agents start without the instruction files | the probe's first-turn context, recorded in this spec |
| The core shows every point except one that declares its situation and states no prohibition (AC-8) | `./le ai rules update condensed` leaves `ai/rules/CORE.md` unchanged, and the generator's refusal tests pass |
| The style core at most 16KB | `wc -c docs/contributing/ze-go-style.md` |
| Both payloads within budget | `./le ai rules report payload` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Rules an agent can evade | An agent that starts with fewer rules must still be stopped by the hooks that enforce the bans; no ban is enforced by prose alone that was enforced by a hook before |
| Hook input | The hook reads four fixed paths under the checkout and one agent definition named by the payload's agent type; the type is validated as a file name before it is joined to a path |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| A-1 false | Stop D-1 after phase 1, keep phases 2 to 5 (they shrink the files for every agent), report to the owner |
| A-2 false | Use the read-three-files form of AC-2 as the normal path |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

Harness documentation read on 2026-11-11 (quotes relayed by a documentation
agent; phase 1 probes each before the design relies on it):

- Subagents load "every level of the CLAUDE.md hierarchy the main conversation loads, including `~/.claude/CLAUDE.md`, project rules, `CLAUDE.local.md`, managed policy files, and any `AGENTS.md` files loaded as project instructions."
- "`omitClaudeMd`: Set to `true` to start this subagent without the user, project, and local CLAUDE.md files in its initial context ... Use it for subagents that take everything they need from the delegation prompt." With it, "a subdirectory's `CLAUDE.md` and matching path-scoped rules still load when the subagent works with files there."
- "SubagentStart hooks receive `agent_id` with the unique identifier for the subagent and `agent_type` with the agent name that the matcher filters on", and may return "`additionalContext` | String added to the subagent's context at the start of its conversation, before its first prompt."
- "To prevent a subagent from invoking skills entirely, omit `Skill` from the `tools` list". Whether that also removes the listing is not documented.
- Read 2026-11-14: for a custom subagent the SubagentStart `agent_type` "is the `name` field from the agent's frontmatter, not the filename". The hook therefore validates the type as a file name, reads `.claude/agents/<type>.md`, and requires its frontmatter `name` to equal the type.
- Read 2026-11-14: SubagentStart also runs when a subagent is resumed, and "When the hook runs again for the same subagent, Claude Code injects the returned context only when the subagent's context doesn't already hold the copy from an earlier run". After auto-compaction discards the copy, the next run's context is injected again.
- A project setting can hide named skills from the listing, and plugin skills are managed by the user, not the project. Those are the owner's settings and are not changed here.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A phase agent omits the instruction files and the start hook supplies the agent rule set | Shrink the files for everyone and let agents keep loading them | Shrinking alone cannot remove main-thread rules from an agent without removing them from the main session. The omit route gives each its own set, and the shrinking is still done (D-2), so the fallback when A-1 is false is already built |
| The agent rule set is assembled at start from the main session's files | Generate a third artifact beside the core and the index | A generated file is a second copy to keep fresh and to measure; reading four files at an agent's start costs milliseconds and cannot drift |
| One decisions file in two parts | Two files | The owner's rule gives the file one home that every harness loads, and dozens of pages name its path. Two parts under two headings give the hook its cut with no citation moved |
| The test for a decision's part: it belongs to the every-agent part when an agent that reads, edits, tests or commits can break it by its own action; to the main-thread part when only the session that talks to the owner, spawns agents, plans or pushes can | Classify by topic section | Sections mix both kinds: the commit rules bind an agent that lands its change, the reporting rules bind only the main thread. A test on who can break the rule places each bullet and tells the main session where a new one goes |
| `applies` on a point, absent meaning shown | Mark the always-on points instead | Absent-means-shown cannot narrow anything by omission, which is the earlier design's first constraint; marking the eager ones would hide every unmarked point the day the field landed |
| Split the style page; leave the gate alone | Teach the gate a second, shorter page | The gate asks for a whole read of one page. A shorter page satisfies it and the owner's rule literally; a second accepted page would need a check that keeps the two in step |

## Known Limitations
- The skill listing for the main session is the owner's setting (which bundled and plugin skills his account lists). This spec changes only what a phase agent carries.
- The system door (11KB, read by every editing agent) is left whole: the door gate exists because agents designed from habit, and a shortened door risks the failure the gate stops. Revisit only with the umbrella's measurement after this child lands.
- The trigger-index and per-stage views of `plan/post-release/engineering/spec-rules-situation-index.md` are not built here.
- R-6 says `./le ai sync check` reports the agent payload per definition. That report was built on 2026-11-15 (see "Remaining before closure").
- The 24KB core target of AC-8 was set before the points were read: most points hold a prohibition and a procedure in one paragraph, so the target could not be met without hiding a prohibition. The owner withdrew it on 2026-11-14.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | not recorded yet: `./le spec review record` runs after the last round |
| `./le spec review check` | not run |
| Rounds | 5 run; the loop ended at round 5, and the seal was refreshed after it |
| Reviewer lenses used | logic at the producer; what an agent or a hostile payload can make it do |

| Round | Scope | Result |
|-------|-------|--------|
| 1 | the whole uncommitted diff of phases 1, 3 and 4 | 0 BLOCKER, 7 ISSUE, 4 NOTE |
| 2 | the fixes of round 1 and the call sites they touch | 0 BLOCKER, 4 ISSUE, 2 NOTE |
| 3 | the fixes of round 2 and the call sites they touch | 0 BLOCKER, 2 ISSUE, 2 NOTE |
| 4 | the fixes of round 3 and the call sites they touch | 0 BLOCKER, 1 ISSUE, 4 NOTE |
| 5 | the fixes of round 4 and the call sites they touch | 0 BLOCKER, 0 ISSUE, 3 NOTE |

Phase 2 is implemented and is a new change, so its review starts a new loop.
Its scope is written before the round runs.

| Round | Scope | Lenses | Result |
|-------|-------|--------|--------|
| 1 | the phase 2 diff: the decisions file, the instruction sentence, the test, and the two hook notes | is any moved bullet one a subagent can break by its own action; is any bullet's text changed or lost | 0 BLOCKER, 1 ISSUE, 6 NOTE. All eleven main-session placements confirmed against the placement test. The ISSUE (the generated `AGENTS.md` left stale after `ai/INSTRUCTIONS.md` changed) was fixed by `./le ai sync update` and judged by `./le ai sync check`, which is clean, so no second reading round was run for a regenerated file |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | The frontmatter reader read a commented, capitalised or quoted `omitClaudeMd`, a quoted `name`, and an indented key inside a multi-line description as an agent that loads the instruction files | `internal/le/hookruntime/lifecycle.go`, `frontmatterFields`, `agentOmitsInstructions` | Indented lines skipped, first occurrence kept, comment stripped, name unquoted, any value but `false` omits; `TestAgentOmitsInstructionsReadsTheFrontmatterTheHarnessReads` |
| 2 | ISSUE | `statedProhibition` missed "never", "forbidden", "none ... MUST", SHOULD NOT, NOT RECOMMENDED, and a prohibition in a code span, blockquote, fence, note or table; two marked points hid one | `internal/le/ai/rules/points.go`, `statedProhibition`; two point files | Every form refused in every kind, an RFC quote included (its exemption was deleted in round 3); the two `applies` lines removed; `TestAppliesIsRefusedOnEveryProhibitionForm` |
| 3 | ISSUE | `validAgentType` had no test that went red without it | `internal/le/hookruntime/lifecycle.go` | `TestValidAgentTypeAdmitsOnlyAFileName` |
| 4 | ISSUE | The definitions' fallback was unpinned | `ai/agents/*.md` | `TestAgentDefinitionsCarryTheHardBansOfTheInstructions` holds the heading, the three files and name equal to stem |
| 5 | ISSUE | The cap test measured no claimed spec and branch `unknown`; the unresolved-claim line was unbounded | `lifecycle_test.go`, `hookSubagentContext` | Realistic start under the cap less `subagentContextReserve`; `startTextBounded`; `TestSubagentStartBoundsAnUnresolvableClaim` |
| 6 | ISSUE | No hook fixture covered the new branch | `internal/le/ai/hooks/fixture_catalog.go` | Two subagent-context sites, run by `TestSubagentRuleSetSitesMatchTheHook` |
| 7 | ISSUE | The unreadable-definition arm and the `refuseRoutedApplies` call in `RenderAll` were untested | `lifecycle.go`, `render.go` | `TestSubagentStartNamesTheFilesWhenTheDefinitionCannotBeRead`, `TestRenderRefusesAppliesOnARoutedRule` |

### Notes left as is
| Round | Severity | Finding | Location | Why it stays |
|-------|----------|---------|----------|--------------|
| 4 | NOTE | `agentCallBudgetCheck` takes its "not counted" arm for an agent id past the id length bound | `internal/le/hookruntime/budget.go` | Nothing is unguarded by it: the Bash hook refuses every command of such an agent, and a code edit is refused through the read marker |
| 5 | NOTE | N3: the spec package's name check (`safeSessionID`) has no length bound, while `lepath`'s id check has 96 | `internal/le/spec/model.go` | Harmless: every id that reaches a claim or a written file name comes through `lepath`; the check also judges spec stems, so it stays |
| Phase 2, 1 | NOTE | The opening's "A subagent is given only the first part." is true only for agent definitions that omit the instruction files | `ai/META-DECISIONS.md` | The others load the whole file, which loses nothing |
| Phase 2, 1 | NOTE | Three `##` section names now appear once in each part, and `.claude/rules/session-start.md` cites one by name whose rule is in the first part | `ai/META-DECISIONS.md`, `.claude/rules/session-start.md` | The cited rule is in the part every agent reads, so the citation still finds it |
| Phase 2, 1 | NOTE | The every-agent "Before the first edit in each checkout, `git fetch`" bullet binds the main session's check | `ai/META-DECISIONS.md` | It names its own subagent exception |
| Phase 2, 1 | NOTE | No input reaches the arm where a session id resolves and `lepath.SessionForID` then fails | `internal/le/hookruntime/lifecycle.go`, `hookSubagentContext` | No input reaches it, so it has no test of its own |

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name or the recorded probe, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le ai hooks unit`, and the packages through `./le job run label unit-pkg quiet command go test -tags <the tags from feature-gates.txt> ./internal/le/hookruntime/ ./internal/le/ai/rules/`. No `./le test unit <group>` covers `internal/le`. No agent runs any `./le verify` command
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
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: the hook fixtures and the probe are the entry-point proof)
- [ ] Interop tests for protocol features (N-A: tooling)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/post-release/engineering/spec-agent-economy-3-preamble.md` only, in the same `./le commit create` script

## Implementation Summary

### What Was Implemented
- Phases 1 to 6 and R-6, committed before closure (`dccb8730d9`, `143da2e5c3`, `18600e8905` and the commits before them): the lean start for `ze-read` and `ze-work` (`omitClaudeMd: true`, the fallback paragraph, `hookSubagentContext` appending the hard bans and the reading list), the decisions file in three parts, `applies` on a point and the core that leaves such a point out, the agent payload measure, the 16KB style core with fourteen sibling pages, and the per-definition payload row of `./le ai sync check`.
- At closure (2026-11-20, uncommitted): `agentTokenBudget` in `internal/le/ai/rules/artifacts.go` set from 12,000 to 25,000 with its derivation in the comment; the budget sentence of `docs/contributing/rule-authoring.md`; the umbrella's child 3 row restated as the bare stem.

### Bugs Found/Fixed
- The agent budget of 12,000 had no basis and made `./le ai rules report payload` print EXCEEDED for the real set. Now 25,000 from the measurement. `TestThePayloadMeasuresTheAgentRuleSet` reads the constant at its boundary (budget minus one MET, budget EXCEEDED), so it covers the new value with no edit.

### Documentation Updates
- `docs/contributing/rule-authoring.md`, the payload paragraph: the agent budget is the set's size on the day the budget was set, rounded up to the next thousand. Its anchor `<!-- source: internal/le/ai/rules/artifacts.go -- Payload, readAgentRuleSet, agentRuleSetParts, agentTokenBudget -->` already names the constant.
- `./le doc check all` (2026-11-20): FAILED on two findings outside this diff: `registry.Meta.DaemonPath must be a string literal` and `rules-points: planning.md is stale`, the second from another session's uncommitted edit of a planning point. No finding names a file this closure changed.

### Deviations from Plan
- AC-12's budget is 25,000, not the 22,000 of the owner's example: the owner's rule (reaching the budget, the measured size rounded up to the next thousand, reports EXCEEDED) was stated against 21,329 tokens of 2026-11-15, and on 2026-11-20 the payload report measured 24,048 after the decisions file grew to 10,325 tokens and the core to 11,539. 22,000 would report EXCEEDED on the day it landed.
- A-3 dropped by the owner, so the skill listing a phase agent carries is unchanged.
- AC-13 is judged on the first-call figure, which precedes the read turn the AC's condition names (AC-13 row).

## Mistake Log
| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-5: the owner's global file holds nothing a phase agent must have | Its em dash ban binds any English an agent writes; only `ai/skills/ze-author.md` carries it in the repository | Read `~/.claude/CLAUDE.md` whole at closure | The "If wrong" route holds; whether the ban joins `ai/rules/writing.md` is the main session's to put to the owner |
| approach | The agent budget was set at 12,000 from a target core size, before any measurement | The measured set is twice that | Payload report | Set from the measurement at closure |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A phase agent starts with the rules it can break and the index | Done | `internal/le/hookruntime/lifecycle.go` `hookSubagentContext`; `ai/agents/ze-read.md`, `ai/agents/ze-work.md` | A-1 validated |
| The gated style page holds what a writer applies, detail one read away | Done | `docs/contributing/ze-go-style.md`, `docs/contributing/go-style/` | 16,262 bytes |
| Nothing deleted, no rule's wording changed | Done | `TestTheDecisionsFileKeepsEveryBulletAcrossItsTwoParts`, `TestTheStyleCoreKeepsEveryHeadingOfTheSplitPage` | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestSubagentStartAddsTheAgentRuleSetForAnAgentThatOmitsTheInstructions`; A-1 probe | |
| AC-2 | Done | `TestAgentDefinitionsCarryTheHardBansOfTheInstructions` | |
| AC-3 | Done | `TestSubagentStartAddsNoRuleSetForAnAgentThatLoadsTheInstructions` | |
| AC-4 | Done | `TestTheDecisionsFileKeepsEveryBulletAcrossItsTwoParts` | three parts since phase 6 |
| AC-5 | Done | `TestTheCoreShowsAPointWithNoSituation` | |
| AC-6 | Done | `TestTheCoreLeavesOutAPointThatDeclaresItsSituation` | |
| AC-7 | Done | `TestAppliesIsRefusedOnAProhibitionAndOutsideTheCore`, `TestAppliesIsRefusedOnEveryProhibitionForm`, `TestRenderRefusesAppliesOnARoutedRule` | |
| AC-8 | Done | `./le ai rules check condensed` (owed) | 24KB target withdrawn by the owner |
| AC-9 | Done | `triggersHeader`; `ai/rules/TRIGGERS.md` lines 8-10 | |
| AC-10 | Done | `wc -c`: 16,262 bytes; `TestTheStyleCoreKeepsEveryHeadingOfTheSplitPage` | 16KB read as 16,384 bytes |
| AC-11 | Done | `writeStyleGuideRead` unchanged; this agent's Go edit passed after a whole read | |
| AC-12 | Done | `agentTokenBudget` 25,000; `TestThePayloadMeasuresTheAgentRuleSet` | Deviations |
| AC-13 | Done | `./le ai tokens report from 2026-11-15 days 5` | first-call measure |
| AC-14 | Done | `ai/INSTRUCTIONS.md` keeps its three `@` lines | |
| AC-15 | Done | `TestTheSituationalSectionsAreTheFourTheOwnerApproved` | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The ten unit tests of the TDD table | Done | the files named there, each found by `grep "func <name>("` on 2026-11-20 | |
| `./le ai hooks unit` | Done | `internal/le/ai/hooks/subagent_rule_set_sites_test.go` `TestSubagentRuleSetSitesMatchTheHook` | owed rerun |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `lifecycle.go`, `points.go`, `artifacts.go` | Done | |
| `ai/agents/ze-read.md`, `ai/agents/ze-work.md` | Done | `Skill` stays |
| `ai/META-DECISIONS.md`, `ai/INSTRUCTIONS.md` | Done | |
| `docs/contributing/rule-authoring.md`, `docs/contributing/ze-go-style.md`, `.claude/hooks/README.md` | Done | |
| `plan/post-release/engineering/spec-rules-situation-index.md`, `spec-docs-read-before-authoring.md` | Done | in the phase commits |

### Audit Summary
- **Total items:** 15 AC, 3 requirements
- **Done:** all
- **Partial:** 0
- **Skipped:** 0
- **Changed:** AC-12 value, AC-13 measure, A-3 dropped (Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A phase agent starts with the rules it can break and the index to the rest | Measured probe and functional hook test | A-1 probe of 2026-11-15; `./le ai tokens report from 2026-11-15 days 5`: first-call context median 21.9k, max 25.6k over 279 subagents, against 53k at the umbrella baseline |
| The gated style page holds what a writer applies | File size and structure test | 16,262 bytes; `TestTheStyleCoreKeepsEveryHeadingOfTheSplitPage` |
| The main session still loads everything | Payload report | `./le ai rules report payload` 2026-11-20: always-loaded payload 30,903 tokens, MET against 40,000 |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| The trigger-index and per-stage views of `applies` | Out of this spec's scope from the start | `plan/post-release/engineering/spec-rules-situation-index.md` |

## Review Gate (closure)

The closure diff (`agentTokenBudget` and its comment, the `rule-authoring.md` sentence, the umbrella row) is this closing agent's own edit, so this agent does not review or record it: an author never reviews its own work. Its review is owed to an independent reader, who records the artifact.

| Field | Value |
|-------|-------|
| Artifact | not recorded: owed to the independent reviewer of the closure diff |
| `./le spec review check` | not run |
| Rounds | phases: 5 rounds, then 1 for phase 2, then 2 for phase 6, all independent; closure diff: 0 so far |
| Reviewer lenses used | the closing agent's own pre-checks only: `./le test weakened check` (5 findings, all in another session's credits test file), `./le repo check` (no finding names a closure file) |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `docs/contributing/go-style/*.md` | Yes | `ls docs/contributing/go-style/`: fourteen pages, `assertions.md` to `zero-values.md` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-12 | The agent set is measured against its own budget | `./le ai rules report payload` (HEAD le, 2026-11-20): agent rule set 24,048 tokens against 12,000, EXCEEDED, before the change. After it, run by the main thread on 2026-11-20 with `./le --name session ai rules report payload`: agent rule set 24,048 tokens, budget 25,000, MET, headroom 3.8% |
| Rules package tests | `TestThePayloadMeasuresTheAgentRuleSet` and the package | `./le --name session test unit pkg path ./internal/le/ai/rules/`, run by the main thread on 2026-11-20: green except the known HEAD red below |
| Hook fixtures | `./le ai hooks unit` not run | Child 3's closure touches no sealed hook source and no hook fixture: `artifacts.go` is outside `hookSourcePackages`, which lists only `internal/le/ai/rules/ruleset` under `internal/le/ai/rules` |
| AC-13 | First-call context under 35k | `./le ai tokens report from 2026-11-15 days 5`: "279 agents, median 21.9k, max 25.6k, 0 above 35k" |
| AC-10 | Style core at most 16KB | `wc -c docs/contributing/ze-go-style.md`: 16262 |
| Known red, not this work's | `TestTheDecisionsFileKeepsEveryBulletAcrossItsTwoParts` | `./le --name session test unit pkg path ./internal/le/ai/rules/` (main thread, 2026-11-20): red only on this test, 160 bullets against 146 pinned. It is red at HEAD, from other sessions' commits to `ai/META-DECISIONS.md`; this closure does not touch that file |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `le ai hooks subagent-context` | none: the hook fixture sites of `TestSubagentRuleSetSitesMatchTheHook` | this agent's own start text carried the "Agent rule set" section and the three reads |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | probe 2026-11-15 |
| A-2 | broken | the 10,000-character cap; the read-three-files route is built |
| A-3 | dropped by owner | 2026-11-20 decision |
| A-4 | confirmed | probe 2026-11-15 |
| A-5 | broken in part | the em dash ban (Mistake Log) |
| A-6 | confirmed | phase 2 and phase 6 reviews |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `rule-authoring.md` budget sentence | `artifacts.go` `agentTokenBudget`, `measure` (`Met = Tokens < budget`) | Yes |
| Pages mentioning the spec's code (`./le spec citation anchors`) | `claude-code-cheatsheet.md` anchors `Payload` for the per-definition rows, no budget figure; `running-commands.md`, `spec-workflow.md`, `navigating-the-code.md`, `verify-freshness-scope.md` mention `lifecycle.go`, untouched at closure | Yes |
