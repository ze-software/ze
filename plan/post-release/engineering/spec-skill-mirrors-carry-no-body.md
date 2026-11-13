# Spec: skill-mirrors-carry-no-body

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P3 |
| Release reason | Release-independent development tooling: it changes how each agent harness reaches a skill's text and changes no shipped binary. |
| RFC gaps | - |
| Depends | - |
| Phase | 1/5 |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-13 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Each agent harness reads a skill from its own directory (`.claude/skills/`,
`.codex/skills/`, `.agents/skills/`). `./le ai sync update` fills those
directories with full copies of `ai/skills/<name>.md`, all git-ignored. A copy
goes stale the moment its source changes, in this session or another, and
nothing shows it until the next session start prints "generated agent files
are stale". Until somebody runs the sync, an agent that loads the skill follows
the old text.

The goal: a harness's skill file carries no copy of the skill's body, so the
text an agent follows is always the one in `ai/skills/`, and a change to a
skill's body owes no sync.

The model is the stub skill Rex installs (`rex skill setup`): a few lines that
tell the agent where the real guide is. Rex points at a command because its
guide lives in the binary. Ze's guide lives in a file of the checkout, so the
stub points at that file.

Owner permissions for this spec (one-off, 2026-11-13): the WIP cap does not
apply to its claim, an agent on the development tier reviews the spec before
implementation starts, and implementation follows that review without a
further gate. The owner approved the stub design on 2026-11-13, with the
leftover cleanup included.

Out of scope: an operator-facing installer that points an agent at
`ze show skill` (the Rex `skill setup` analogue for the product). `AGENTS.md`
and the `.claude/agents/` definitions stay copies: a harness loads both as
they are, with no step at which an agent could follow a pointer.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every change respects
  → Constraint: every fact is declared once and every other surface derives from it; the stub is the derived surface and `ai/skills/<name>.md` the declaration.
  → Constraint: every message a user sees names the problem, why, and the exact fix; the refusal for a source with no usable frontmatter follows that shape.
- [ ] `ai/doors/le-tooling.md` - the door for `internal/le`
  → Constraint: a tool is one package under `internal/le/` whose verbs are an action table (`leaction.New`); no new verb is added here, so no new registration is owed.
  → Constraint: the main thread runs every gate and package test; an implementing agent lists `./le test unit pkg path ./internal/le/ai/sync/` as owed.
  → Constraint: the session-start hook calls `aisync.Mirror.Check` (`internal/le/hookruntime/lifecycle.go`, `reportAgentFiles`) and prints its warning when the check errors or answers a stale path. A change to `Check`'s meaning reaches every session at once, so `Check` keeps its meaning and `hookruntime` is not edited.
  → Decision: when the check refuses a source (AC-5), the hook prints "stale, run `./le ai sync update`". That is roundabout and not wrong: the command it names then prints the refusal with its fix.
- [ ] `ai/doors/system-plugins.md` - the door for `internal/plugins`, read because this spec names the product's skills plugin as a mechanism it does not touch
  → Constraint: a plugin owns its whole surface; the product's `show skill` guides stay in `internal/plugins/skills` and are not reached from le.
- [ ] `ai/rules/repo-maintenance.md`, "Canonical Sources and Sync Direction" - the rule the sync package names as its design
  → Constraint: `ai/skills/*.md` is the canonical source and `./le ai sync update` the only writer of the per-tool files; a generated file is never edited by hand. The row stays true and its wording changes from "mirrors" holding a copy to files holding a pointer.
- [ ] `ai/rules/user-facing-errors.md` - shape of a refusal
  → Constraint: the refusal names the file, what is missing, and the edit that fixes it, with the fix last.

### RFC Summaries (Scope: protocol)
Not applicable: Scope is tooling.

### Harness behaviour (outside readers of the generated file)
- [ ] Claude Code skills documentation (`code.claude.com/docs/en/skills.md`, `context-window.md`, `tools-reference.md`), read 2026-11-13
  → Constraint: the skill body is read from disk when the skill is invoked; only the description is loaded at session start. A stub's pointer is therefore followed against the current source on every load.
  → Constraint: after compaction an invoked skill's body is re-injected capped at 5,000 tokens. A stub is far below the cap and returns whole, so it must itself say to read the source again.
  → Constraint: command output is cut at about 30,000 characters by default, and six skills are larger. The pointer is a file read, never a command that prints the skill.
  → Decision: a symlinked `SKILL.md` file is undocumented and an open defect (`anthropics/claude-code#14836`) makes the skill listing miss symlinked entries. Symlinks are not used.
- [ ] OpenAI Codex source (`codex-rs/exec-server/src/local_file_system.rs`, tag `rust-v0.169.0`), read by a research agent 2026-11-13
  → Decision: the skill walk skips a symlinked file (the entry is dropped unless it is a followed directory). A symlinked `SKILL.md` would make every skill vanish in Codex. Symlinks are not used.
  → Constraint: `name` and `description` are required in the frontmatter.
- [ ] Pi coding agent source (`packages/coding-agent/src/core/skills.ts`) and OpenCode source (`packages/opencode/src/skill/index.ts`), read by a research agent 2026-11-13
  → Constraint: Pi does not load a skill with no `description`, and both list a skill to the model by its `name` and `description`. The stub carries the source's real frontmatter, never a placeholder.
  → Constraint: OpenCode caches the body at load, so the agent reaches the source only through the read the stub orders.

**Key insights:** (minimal context to resume after compaction)
- The generated skill file becomes a stub: the source's frontmatter, a generated-by marker, one instruction to read `ai/skills/<name>.md` in full. Identical bytes in all three harness directories.
- The `.agents` path rewrite is deleted, not kept: it rewrites `.claude/rules/...` to `.agents/rules/...`, a directory that does not exist.
- `Check` is not changed at all: it compares against a fresh generation over the union of paths, in the four generated trees, and any path the two sides disagree about is stale. With stubs it answers stale only when the set of skills or a skill's frontmatter changed, or a file is left over.
- Only `Sync` changes for leftovers. In the three skill trees it removes a `<name>/SKILL.md` that carries the marker and has no source. Every other file the generation does not produce, in any of the four trees, is left byte for byte and named in the sync's answer with the fix (delete it if it is a leftover). So the command the stale warning names now either clears the staleness or says what is left and why.
- The marker is one exact line, the first line after the frontmatter block. A file carries it only when that line is byte-equal to the generated one for its own skill name.
- The stub holds no argument placeholder, so the harness appends the arguments it was given; the stub says the placeholder in the source stands for them.
- No new command. `internal/le/hookruntime` is not edited: nothing in it needs to change.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/ai/sync/ai.go` (364L) - `Mirror.generateInto` copies each `ai/skills/<name>.md` verbatim to `.claude/skills/<name>/SKILL.md` and `.codex/skills/<name>/SKILL.md`, and with every `.claude/` replaced by `.agents/` to `.agents/skills/<name>/SKILL.md`. It also copies `ai/agents/*.md` and `ai/INSTRUCTIONS.md`. `Sync` writes into the checkout and removes a root `CLAUDE.md`; it deletes nothing else. `Check` generates into a scratch tree and compares the union of paths in each mirror tree, so a path only in the checkout is reported stale.
  → Constraint: one generator serves the sync and the check (`generateInto`), so the check can never judge against a second implementation. The stub is produced inside that one generator.
  → Constraint: a missing source is an error, never an empty run (`ErrNoSource`).
- [ ] `internal/le/ai/sync/actions.go` (118L) - the verbs `update`, `check`, `preview`; `check` answers exit 1 for a stale tree and for a tree it could not judge.
- [ ] `internal/le/ai/sync/report.go` (61L) - `Report` carries `Mode`, `Skills`, `Agents`, `Stale`; `Text` prints one `stale:` line per path and the line naming `./le ai sync update`.
- [ ] `internal/le/ai/sync/ai_test.go`, `actions_test.go` - the fixture's two skills carry a `name` and no `description`; `TestTheAgentsMirrorRepointsClaudePathsAndTheOthersDoNot` pins the path rewrite; `TestACheckNamesEveryKindOfDrift` pins that an extra file is drift.
- [ ] `internal/le/hookruntime/lifecycle.go`, `reportAgentFiles` - prints the session-start warning when `Check` errors or answers any stale path.
- [ ] `ai/skills/ze-check.md`, sections 4a and 4b - tell the agent to scan `.claude/skills/*/SKILL.md` for stale file references and broken `/ze-` references.
- [ ] `internal/le/rfc/check_ratchets.go` (line 516) - a refusal that points the reader at `.claude/skills/ze-rfc/SKILL.md`.
- [ ] `ai/rationale/documentation.md` (lines 38 and 74 to 79) - says the skills are "synced to `.claude/skills/`" and that the sync writes `CLAUDE.md`, which the sync removes.

- [ ] `internal/le/ai/sync/register.go` (41L) - the area's `Description` says "sync every tool's copy of the skills and instructions"; `ai/INDEX.md` carries a hand copy of it.
- [ ] `ai/agents/ze-read.md`, `ai/agents/ze-work.md` (lines 7 and 8 of each) - tell a phase agent to "Read that skill and follow it", naming no path.

**Measured on 2026-11-13:** 34 skills, 429,362 bytes of source, every source carrying exactly `name` and `description` in its frontmatter (7,412 bytes in all), every `name` equal to its file's stem. Six sources exceed 30,000 bytes; the largest is 44,270 bytes and 710 lines. The sources hold 19 occurrences of `.claude/` that the rewrite touches; 2 of them name a path under `.claude/skills/`. `.agents/` holds only `skills/`, so the other 17 are rewritten to paths that do not exist. Six sources use the harness's argument placeholder (`ze-rfc`, `ze-rfc-audit`, `ze-mutation-fix`, `ze-fix-alloc`, `ze-find-alloc`, `ze-hunt`).

**Behavior to preserve:** (unless the user explicitly said to change it)
- The three target directories and the `<name>/SKILL.md` layout each harness reads.
- `update`, `check`, `preview`: their names, exit codes, and the JSON keys `mode`, `skills`, `agents`, `stale`.
- `Check` writes nothing into the checkout.
- `AGENTS.md`, `.claude/agents/*.md` and the removal of a root `CLAUDE.md`, unchanged.
- The session-start warning and its text.

**Behavior to change:** (only what the user asked for)
- A generated skill file holds a stub in place of a copy.
- The `.agents` path rewrite is removed.
- A source whose frontmatter cannot make a stub is refused.
- The sync deletes a leftover stub, and names every other file it found in a generated tree and left in place.
- The subagent definitions and the le registration name `ai/skills/` as where a skill's text is.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| The agent-file generator | `internal/le/ai/sync/ai.go`, `Mirror.generateInto` | Writes each harness's copy of every skill, the subagent definitions and `AGENTS.md` | Extends: the skill branch writes a stub built from the source's frontmatter. The agent and instructions branches are untouched |
| The `.agents` path rewrite | `internal/le/ai/sync/ai.go`, `generateInto` (the `bytes.ReplaceAll` of `claudePrefix`) | Rewrites `.claude/` to `.agents/` in the `.agents` copy | Replaces: deleted. With no body in the generated file there is nothing to rewrite, and 17 of its 19 targets do not exist |
| The freshness check | `internal/le/ai/sync/ai.go`, `Mirror.Check`, `driftIn` | Compares the checkout against a fresh generation, over the union of paths, in the three skill trees and `.claude/agents` | Does not apply: unchanged. It compares against the same generator, which now writes stubs, so its verdict narrows with no edit. `Sync` reuses `driftIn`'s union walk to find the files it must remove or name |
| The sync's removal of a file no generation writes | `internal/le/ai/sync/ai.go`, `Mirror.Sync` (the removal of a root `CLAUDE.md`) | Removes the one file whose presence harms and that no source produces | Extends: the same step removes a leftover stub, and names what it left |
| The session-start warning | `internal/le/hookruntime/lifecycle.go`, `reportAgentFiles` | Warns when `Check` answers a stale path | Does not apply: it reads `Check`'s verdict, which keeps its meaning. The hook source is not edited |
| Canonical source and sync direction | `ai/rules/repo-maintenance.md`, "Canonical Sources and Sync Direction" | Declares `ai/skills/*.md` as the source of the per-tool files | Extends: the row says what the generated file now holds |
| The product's bundled skills | `internal/plugins/skills/main.go`, `showSkill` (`ze show skill`) | Serves operator guides embedded in the shipped binary | Does not apply: those are product guides for an operator's agent, read through the command registry. This spec concerns the development skills under `ai/skills/`, which `le` never embeds |
| `le help ai` | `docs/contributing/running-commands.md`, "The whole surface for an agent" | Describes le's command surface to an agent | Does not apply: no command is added, so its tables do not change |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `./le ai sync update`, `./le ai sync check` and the session-start hook, each reaching `aisync.Mirror`.
- Input: `ai/skills/<name>.md`, a markdown file opening with a frontmatter block.

### Transformation Path
1. `Mirror.sources` lists the skill names from `ai/skills/`.
2. For each skill, the generator reads the source, takes its frontmatter block, and refuses the run when the block is absent, has no `name`, has no `description`, or names a skill other than the file's stem.
3. The generator composes the stub: the frontmatter block byte for byte, then the marker as the first line after the block, naming the source, then the instruction. The same bytes go to all three harness directories.
4. `Sync` then walks the four generated trees for paths the generation did not produce. In the three skill trees, a `<name>/SKILL.md` whose first line after its frontmatter is byte-equal to the marker for `<name>` is a leftover stub: it is removed, with its directory when that leaves it empty. Every other such path, in any of the four trees, is left untouched and listed in the sync's answer.
5. `Check` is unchanged: it generates into a scratch tree and names every path the two sides disagree about.
6. A harness loads the stub when the skill is invoked and appends the arguments it was given; the agent reads `ai/skills/<name>.md` and follows it. A phase agent told its skill by name reads `ai/skills/<name>.md` directly, because its definition now names that path.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| le to the harness's skill directory | A git-ignored file per skill, written by `Sync` | No |
| Harness to agent | The harness puts the stub in context; the agent performs the read it orders | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `cmd/`, `ai/`, `docs/`, `plan/`, `.claude/rules/`, `.claude/hooks/`, `.claude/settings.json`, `.github/`, `.gitignore`, and every `*.sh`, `*.py`, `*.json`, `*.yml`, `*.yaml`, `*.toml` outside `vendor/`, `tmp/` and `bin/`. Terms: `.claude/skills`, `.agents/skills`, `.codex/skills`, `SKILL.md`, "skills mirror", "Read that skill". Positive control: the search finds the writer's own constants in `internal/le/ai/sync/ai.go` (lines 52 to 54 and 65). Findings beyond the writer: one Go message (`internal/le/rfc/check_ratchets.go`), the area's registered description (`internal/le/ai/sync/register.go`) and its hand copy in `ai/INDEX.md`, the two subagent definitions, two prose files under `ai/` (`ai/skills/ze-check.md`, `ai/rationale/documentation.md`), six specs under `plan/`, the two test files of the package, and no shell, Python, workflow or configuration reader in the repository. `.gitignore` (lines 121 to 123) ignores the three directories and reads nothing in them. `website/data/wiki.json` and two `vendor/` lock files match a term and read nothing of this format. The design review of 2026-11-13 repeated the search independently and added the `plan/`, registration and subagent-definition rows.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Generated skill file | writer | `internal/le/ai/sync/ai.go`, `Mirror.generateInto` | The only non-test Go site that composes a path under the three directories (`<harness dir>/<name>/SKILL.md`) | Change: writes a stub |
| Generated skill file | reader | `internal/le/ai/sync/ai.go`, `Mirror.Check`, `driftIn`, `sameFile` | Reads both trees' bytes to compare | Preserve: unchanged code, compared against the same generator |
| Generated skill file | reader | `internal/le/ai/sync/ai.go`, `Mirror.Sync` (new reader) | Reads the first line after the frontmatter of a path the generation did not produce | Change: a new read, to tell a leftover stub from any other file |
| Generated skill file | reader | `ai/agents/ze-read.md`, `ai/agents/ze-work.md` (a phase agent told to "read that skill") | Lines 7 and 8 of each name no path, so the agent reads whichever file it finds | Change: both name `ai/skills/<name>.md`, so a phase agent reads the source in one step |
| Generated skill file | reader | `internal/le/ai/sync/register.go`, `Description`, and its hand copy in `ai/INDEX.md` (a person or agent choosing a command) | Says the sync writes "every tool's copy of the skills" | Change: says what the sync writes now |
| Generated skill file | reader | Specs under `plan/` naming a generated skill file: `plan/release/validation/spec-design-needs-owner-approval.md`, `plan/release/fixes/spec-route-policy-1-gate-proof-and-forked-sysrib.md`, `plan/release/fixes/spec-rfc-proof-debt-outside-filter-wire.md`, `plan/post-release/engineering/spec-structure-checkin.md`, `plan/post-release/engineering/spec-prose-cites-symbols-not-facts.md` | Each cites a path under a generated directory as something to read or to edit | Preserve: not edited by this spec. A reading entry still leads to the text, through the stub. An entry naming a generated file as an edit target was wrong before this change (`ai/rules/repo-maintenance.md`: a generated file is never edited), and the write goes to the source either way. Several of these specs are claimed by other sessions |
| Generated skill file | reader | `internal/le/hookruntime/lifecycle.go`, `reportAgentFiles` | Calls `Mirror.Check` and reads `Stale` | Preserve: same call, same field, same meaning |
| Generated skill file | reader | Claude Code (outside the repository) | Reads frontmatter at session start and the body at invocation | Preserve the frontmatter; the body becomes the pointer the agent follows |
| Generated skill file | reader | Codex CLI, OpenCode, Pi (outside the repository) | Each reads `name` and `description`, then the body on use | Preserve the frontmatter; the body becomes the pointer. The `.agents` file no longer differs from the others |
| Generated skill file | reader | `ai/skills/ze-check.md`, sections 4a and 4b (an agent scanning the files' text) | Names `.claude/skills/*/SKILL.md` as the scan target | Change: the scan target becomes `ai/skills/*.md`, or it would pass over text that is no longer there |
| Generated skill file | reader | `internal/le/rfc/check_ratchets.go`, the enrolment refusal (a person or agent following the pointer) | Points at `.claude/skills/ze-rfc/SKILL.md` | Change: points at `ai/skills/ze-rfc.md`, the file that holds the text |
| Generated skill file | reader | `ai/rationale/documentation.md` (prose) | Describes the copies | Change: describes the stub, and stops saying the sync writes `CLAUDE.md` |

### Integration Points
- `aisync.Mirror.generateInto` - the stub is composed here, so `Sync` and `Check` share it.
- `aisync.Report` - gains the paths a sync removed and the paths it left, printed by `Text` and carried in the JSON. A check's answer is unchanged.
- `aisync.driftIn` - its union walk is what `Sync` reuses to find paths the generation did not produce, so the sync and the check agree on what is extra.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An agent that loads a stub performs the read it orders before acting | Rex's stub (`~/.claude/skills/rex/SKILL.md`, 13 lines) works this way in Claude Code, Codex, OpenCode and Pi on this machine (`rex skill status`, 2026-11-13) | Agents act on a skill's description alone; the change is reverted | A live load of one skill in a fresh session after the sync: the agent's first action is the read of `ai/skills/<name>.md` | unvalidated |
| A-2 | Every source's frontmatter `name` equals its file stem | The 34 sources each carry one `name` (counted 2026-11-13); equality not yet compared | The refusal for a mismatched name fires on real sources and they are corrected in the same work | The first `./le ai sync update` after the change answers no refusal | unvalidated |
| A-3 | A single read returns a whole skill | Claude Code's read returns up to 25,000 tokens per call; the largest source is 44,270 bytes, about 11,000 tokens | A large skill arrives cut; the stub's wording then orders the read in parts | Read of `ai/skills/ze-rfc.md` in one call returns all 710 lines | unvalidated |
| A-4 | An agent's working directory is the checkout root when it loads a skill | Sessions start in the checkout; subagents inherit it | A repository-relative path does not resolve; the stub names the path as relative to the repository root so the agent can still find it | The live load of A-1, repeated from a subagent | unvalidated |
| A-5 | No test outside `internal/le/ai/sync` asserts the content of a generated skill file | The census search found no other reader in Go | A second test goes red and is brought into this work | The package tests the change reaches (`./le repo changed scope print both`) pass | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | An agent skips the read and works from the description | Work that ignores a skill's gates in the first session after the change | The instruction is the stub's whole body and is imperative; A-1's live proof runs before closure. Fallback: revert the one commit |
| R-2 | After compaction the agent no longer holds the skill's text | A long-running agent stops following its skill's later steps | The stub is re-injected whole and orders the read again after compaction. Today's copies are cut at 5,000 tokens at that point, so this is no worse |
| R-3 | A source being edited by another session is read half-written | An agent follows a skill mid-edit | Same exposure as today, where a sync copies the working tree. Not addressed here |
| R-4 | A skill loaded through the harness costs one more tool call | Turn counts in the main thread rise slightly | Accepted for the main thread: one read per load, against a stale skill followed in silence. A phase agent pays nothing extra, because its definition names the source directly |
| R-5 | The sync deletes a file somebody wrote | A personal skill disappears from a harness directory | Only a `<name>/SKILL.md` in a skill tree is ever deleted, only when its first line after the frontmatter is byte-equal to the marker for `<name>`, and only when no source has that name. A file at a path a current source owns is overwritten, as today |
| R-6 | A harness that reads two of the directories lists every skill twice | OpenCode reads both `.claude/skills` and `.agents/skills` | Existing condition, unverified here and unchanged by this spec. The two files are now identical, which removes the difference between the duplicates |
| R-7 | A checkout other than this one holds a pre-change full copy whose source is gone; it carries no marker, so the sync does not remove it | Every session there keeps the stale warning | The warning names the sync, and the sync's answer now names that file and says to delete it. No such file exists in this checkout (the check answered "in sync" on 2026-11-13) |
| R-8 | A skill that takes arguments gets none | An agent runs an argument-taking skill on a literal placeholder | The stub holds no placeholder, so the harness appends the arguments, as it does today for a skill without one; the stub says what the placeholder in the source stands for. AC-11 loads an argument-taking skill |
| R-9 | The walk that finds extra paths makes the sync slow or the check slower | Session start takes longer | The check's code is unchanged. The sync walks four small trees once more, and is run by hand |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every agent in every harness loads its skills through the generated files. A wrong stub means agents work without their skill text: no shipped behaviour, but every development session |
| How is it reverted? | Revert the commit and run `./le ai sync update`, which rewrites the full copies. The generated files are git-ignored, so nothing else carries state |
| Who else touches this path? | Every session reads the generated files; any session may run the sync. `plan/post-release/engineering/spec-agent-economy-4-supervision.md` trims skill bodies in `ai/skills/` and does not touch the generator |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le ai sync update` (the `update` verb through `Answer`) | → | `Mirror.Sync`, `Mirror.generateInto` | `TestTheSyncVerbWritesAStubThatNamesItsSource` |
| `./le ai sync check` (the `check` verb through `Answer`) | → | `Mirror.Check` | `TestTheCheckVerbAnswersZeroAfterASkillBodyChanges` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A sync over a checkout with skills | Each of the three harness directories holds, for every skill, a file whose frontmatter block is the source's block byte for byte and whose body names `ai/skills/<name>.md` as the file to read in full | The frontmatter survives because the harness lists the skill by it. No line of the source's body survives, because any copied line can go stale |
| AC-2 | The same sync | The three files of one skill are byte-identical | None: no harness-specific text remains, because the stub names no harness path |
| AC-3 | A skill's body is edited after a sync | The check answers exit 0 and "generated agent files in sync" | Nothing is stale, because no generated file holds the body |
| AC-4 | A skill's `description` is edited after a sync | The check answers exit 1 and names that skill's three files; a sync then makes the check answer exit 0 | The frontmatter is the one copied part, so it is the one part that can still be stale |
| AC-5 | A source with no frontmatter block, no `name`, no `description`, or a `name` that is not the file's stem, sorted after a valid source | The sync and the check refuse, no file of any skill is written or changed (the valid one included), and the message names the file, what is wrong, and the line to add or edit | None: a stub without a description is a skill Pi does not load and no harness can describe. A `name` that is not the stem is refused because Claude Code names a skill by its directory and the other harnesses by `name`, so the same skill would carry two names |
| AC-6 | A skill's source is deleted after a sync | The check names its three leftover files stale; the sync removes them and their emptied directories and its answer names each removed path; the check then answers exit 0 | Files of other skills are untouched. A directory holding another file is kept, and that file is named as left (AC-7) |
| AC-7 | A file in one of the three skill trees that the generation does not produce and that is not a leftover stub: a `SKILL.md` whose first line after its frontmatter is not the marker for its directory's name (a pre-change full copy whose source is gone, a hand-written skill, a stub edited by hand), or any sibling file | The sync leaves it byte for byte and its answer names the path with the reason (not generated by le) and the fix (delete it if it is a leftover). The check names it stale, as today | The file survives because le cannot show it wrote it. It stays stale so that no file a harness may load goes unreported, and the command the warning names now says what to do about it |
| AC-8 | A file without the marker at a path a current source owns | The sync overwrites it with the stub | None at that path: the path belongs to the source, and every pre-change full copy of a living skill sits exactly there |
| AC-9 | The stub's instruction | It orders one complete read of `ai/skills/<name>.md` with no partial range, says to follow the file as the skill's instructions, says the argument placeholder in that file stands for the arguments given to the skill, says to read the file again after compaction, and says to stop and report when it cannot be read. The stub itself holds no argument placeholder | The five orders survive as the stub's whole body. No placeholder survives, because a harness that finds none appends the arguments to what it delivers, and one written into the stub's prose would be substituted there |
| AC-10 | `ai/skills/ze-check.md`, the enrolment refusal in `internal/le/rfc`, `ai/rationale/documentation.md`, the rule's source row, the area's registered description with its `ai/INDEX.md` row, and the two subagent definitions | Each names `ai/skills/` as where a skill's text is. `ze-check` scans `ai/skills/*.md` and resolves a skill reference to `ai/skills/<name>.md`. The subagent definitions tell a phase agent to read `ai/skills/<name>.md` in full. The rule's row and the description say the generated file holds the name, the description and a pointer | The generated directories are still named where the text is about what a harness loads |
| AC-11 | After the sync, a fresh main session invokes an argument-taking skill, and a `ze-read` subagent is told to run a phase through a named skill | In the main session the harness delivers the stub with the arguments appended and the agent's next action is a read of `ai/skills/<name>.md` with no offset and no limit. The subagent's first read of skill text is `ai/skills/<name>.md` | None: this is the goal the spec exists for. The proof is Claude Code only; the other three harnesses are named as unproven in Known Limitations |
| AC-12 | A subagent definition's source is deleted after a sync | The check names `.claude/agents/<name>.md` stale, as today. The sync leaves it and its answer names it with the reason and the fix | The file survives because a subagent definition carries no marker; it stays reported, and the sync now says what to do |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTheSyncVerbWritesAStubThatNamesItsSource` | `internal/le/ai/sync/actions_test.go` | The `update` verb leaves a stub naming the source path (wiring, AC-1) | |
| `TestTheCheckVerbAnswersZeroAfterASkillBodyChanges` | `internal/le/ai/sync/actions_test.go` | The `check` verb answers 0 after a body edit (wiring, AC-3) | |
| `TestAStubCarriesTheSourcesFrontmatterByteForByte` | `internal/le/ai/sync/ai_test.go` | The frontmatter block is the source's (AC-1) | |
| `TestAStubCarriesNoLineOfTheSourcesBody` | `internal/le/ai/sync/ai_test.go` | A marker line of the source's body is absent from all three files (AC-1) | |
| `TestTheThreeFilesOfASkillAreByteIdentical` | `internal/le/ai/sync/ai_test.go` | No harness-specific text (AC-2). Takes the place of `TestTheAgentsMirrorRepointsClaudePathsAndTheOthersDoNot`, whose behaviour is deleted | |
| `TestAChangedDescriptionIsStaleUntilSynced` | `internal/le/ai/sync/ai_test.go` | Frontmatter drift is named and a sync clears it (AC-4) | |
| `TestASourceThatCannotMakeAStubIsRefused` | `internal/le/ai/sync/ai_test.go` | One row per defect: no block, no name, no description, name not the stem. The bad source sorts after a valid one, and no file exists afterwards, so an implementation that writes as it goes is red. The message names the file and the fix (AC-5) | |
| `TestASyncRemovesTheStubOfADeletedSkill` | `internal/le/ai/sync/ai_test.go` | Leftover stubs and their emptied directories go, the answer names them, other skills stay (AC-6) | |
| `TestACheckIsCleanOnceTheLeftoverStubIsRemoved` | `internal/le/ai/sync/ai_test.go` | Stale before the sync, exit 0 after it (AC-6) | |
| `TestASyncNamesAFileItLeftInASkillTree` | `internal/le/ai/sync/ai_test.go` | One row per kind: an unmarked `SKILL.md` with no source, a stub whose marker line was edited, a sibling file. The answer names the path, the reason and the fix (AC-7) | |
| `TestAMarkerElsewhereInTheFileDoesNotMakeItALeftoverStub` | `internal/le/ai/sync/ai_test.go` | The marker text further down a hand-written file does not get it removed (AC-7) | |
| `TestASyncNamesALeftoverAgentDefinitionAndTheCheckStillCallsItStale` | `internal/le/ai/sync/ai_test.go` | `.claude/agents` is outside the removal (AC-12) | |
| `TestTheStubOrdersAFullReadOfItsSourceAndHoldsNoArgumentPlaceholder` | `internal/le/ai/sync/ai_test.go` | The five orders are present and the placeholder is absent (AC-9) | |
| `TestTheSyncReportNamesWhatItRemovedAndWhatItLeft` | `internal/le/ai/sync/ai_test.go` | The text and the JSON of a sync's answer carry both lists, absent when empty | |

Preservation tests, green before the change and kept green by it. They are written first and are not counted in "Tests FAIL":

| Test | File | Preserves |
|------|------|-----------|
| `TestASyncLeavesAFileItDidNotGenerateByteForByte` | `internal/le/ai/sync/ai_test.go` | Today's sync deletes nothing in the trees; the unmarked kinds of AC-7 must stay so |
| `TestASyncOverwritesAnUnmarkedFileAtASourcesPath` | `internal/le/ai/sync/ai_test.go` | Today's sync overwrites whatever sits at a source's path (AC-8) |
| `TestACheckNamesEveryKindOfDrift` (existing, unchanged) | `internal/le/ai/sync/ai_test.go` | An extra file is drift, in all four trees |

Existing tests the change reaches: `TestASyncWritesEveryMirrorOfEverySource` keeps its path list; the fixture's sources gain a `description`. `TestTheAgentsMirrorRepointsClaudePathsAndTheOthersDoNot` is removed with the behaviour it pins, and `TestTheThreeFilesOfASkillAreByteIdentical` takes its place. That removal is a test change that could lower what a test proves, so an independent agent running `ze-test-judge` judges it before it lands (`ai/META-DECISIONS.md`, "Test changes"). AC-10 is prose and one message string: its proof is the search in the Deliverables Checklist and, for the message, the existing test of the enrolment refusal if one asserts its text.

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | The feature takes no numeric input | N-A | N-A | N-A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | N-A | le's own verbs carry no `.ci` suite; the two wiring tests drive `update` and `check` through the action table `Answer`, which is the entry point `./le ai sync` reaches. The end-user proof is AC-11, a live skill load recorded at closure | |

## Files to Modify
- `internal/le/ai/sync/ai.go` - compose the stub in the generator, refuse a source that cannot make one, delete the path rewrite, and in `Sync` remove leftover stubs and collect the paths left in place; `Check` is not edited; package comment updated
- `internal/le/ai/sync/report.go` - the paths a sync removed and the paths it left, in the JSON and the text
- `internal/le/ai/sync/actions.go` - the `update` summary says what is written and removed
- `internal/le/ai/sync/register.go` - the area's `Description` says what the sync writes now
- `ai/agents/ze-read.md`, `ai/agents/ze-work.md` - "Read that skill" names `ai/skills/<name>.md`, read in full
- `internal/le/ai/sync/ai_test.go`, `internal/le/ai/sync/actions_test.go` - the tests above; fixture sources gain a `description`
- `internal/le/rfc/check_ratchets.go` - the enrolment refusal points at `ai/skills/ze-rfc.md`
- `ai/skills/ze-check.md` - section 4a scans `ai/skills/*.md`; section 4b scans them and resolves a `/ze-` reference to `ai/skills/<name>.md` existing, in place of a skill directory
- `ai/rules/points/repo-maintenance/canonical-sources-and-sync-direction/sync-generated-files-from-their-canonical-source.md` - the skills row says what the generated file holds; `ai/rules/repo-maintenance.md` is rendered from it by `./le ai rules update render`
- `ai/rationale/documentation.md` - the two passages on the generated copies
- `ai/INDEX.md` - the `./le ai sync` row, a hand copy of the registered `Description`, follows it

## Files to Create
- None. The stub is composed in `ai.go`; no template file is added, because a second file holding the stub's text would be a second declaration of it.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | le tooling; no config and no RPC |
| YANG validation constraints | N-A | No YANG leaf |
| YANG custom validators | N-A | No YANG leaf |
| CLI commands/flags | No | No verb or flag is added; `update`, `check` and `preview` keep their names |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | N-A | No YANG leaf |
| Functional test for new RPC/API | N-A | No RPC or API |
| Pipe completeness | N-A | The report already answers through the shared renderer; one field is added to it |
| Env var registration | N-A | No env var |
| Doctor check for runtime dependencies | N-A | `ze doctor` judges the shipped product; this is development tooling with no runtime dependency |
| Doctor code per problem class | N-A | The one new refusal is an le diagnostic at development time, outside the product's doctor surface |
| Prometheus counters/metrics | N-A | No runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not a BGP feature |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Development tooling; no `features/<id>.md` covers it |
| 2 | Config syntax changed? | No | No config |
| 3 | CLI command added/changed? | No | No `ze` command; le's verbs keep their names |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | None |
| 6 | Has a user guide page? | No | None for le's agent files |
| 7 | Wire format changed? | No | No protocol wire |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | No | Unit tests of one package only |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `ai/rationale/documentation.md` (the generated agent files); `ai/rules/points/repo-maintenance/canonical-sources-and-sync-direction/sync-generated-files-from-their-canonical-source.md` and its render `ai/rules/repo-maintenance.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `ai/rules/repo-maintenance.md` is declared by the `// Design:` header of the three `internal/le/ai/sync` files and is updated through its point. `./le spec citation anchors spec plan/post-release/engineering/spec-skill-mirrors-carry-no-body.md` is run at implementation and every page it lists is named in the implementation summary. `docs/architecture/core-design.md` is declared by `internal/le/rfc/check_ratchets.go` and by `internal/le/ai/sync/register.go`, and is unaffected: it describes how an le area registers and the rfc area as one command, and this work changes only a description string and the file a refusal points its reader at |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `ai/skills/ze-check.md` names the generated files as a scan target; `ai/INDEX.md` row for `./le ai sync` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the two verb-level tests, written to fail against today's copies
   - Tests: `TestTheSyncVerbWritesAStubThatNamesItsSource`, `TestTheCheckVerbAnswersZeroAfterASkillBodyChanges`
   - Files: `internal/le/ai/sync/actions_test.go`; the fixture in `ai_test.go` gains a `description` per skill
   - Verify: both fail, the first because the file holds the source's body, the second because a body edit is reported stale
2. **Phase: The stub** -- compose it in the generator, refuse a source that cannot make one, delete the path rewrite
   - Tests: the AC-1, AC-2, AC-4, AC-5 and AC-9 rows of the unit table; the repoint test is replaced once its judge approves
   - Files: `internal/le/ai/sync/ai.go`, `internal/le/ai/sync/ai_test.go`
   - Verify: tests fail, implement, tests pass; both wiring tests pass
3. **Phase: Leftovers** -- `Sync` removes a leftover stub and names what it left; the report carries both lists; `Check` is not edited
   - Tests: the two preservation tests first (green), then the AC-6, AC-7 and AC-12 rows and the report row (red, then green)
   - Files: `internal/le/ai/sync/ai.go`, `internal/le/ai/sync/report.go`, `internal/le/ai/sync/actions.go`, both test files
   - Verify: tests fail, implement, tests pass; `TestACheckNamesEveryKindOfDrift` passes untouched
4. **Phase: The readers** -- every place that names the generated files as where a skill's text is
   - Files: `ai/skills/ze-check.md`, `ai/agents/ze-read.md`, `ai/agents/ze-work.md`, `internal/le/ai/sync/register.go`, `internal/le/rfc/check_ratchets.go`, `ai/rationale/documentation.md`, the repo-maintenance rule point, `ai/INDEX.md`
   - Verify: the main thread runs `./le ai rules update render`, the package tests of `internal/le/rfc` that cover the refusal, and `./le doc check all`
5. **Phase: The live proof** -- the main thread runs `./le ai sync update` and `./le ai sync check` in the checkout, then loads skills
   - Verify: 34 stubs in each of three directories and the check clean. A fresh main session invokes `ze-rfc` with an argument, and a `ze-read` agent is told to run a phase through a named skill. Recorded as evidence: for the main session, the skill call, the stub it returned with the arguments appended, and the next call, a read of `ai/skills/ze-rfc.md` with no offset and no limit; for the subagent, its first read of skill text; and the output of `./le ai sync check` (AC-11, A-1, A-2, A-3, A-4)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | The live load of AC-11 was run in a fresh session and from a subagent |
| Correctness | The stub is composed in one place and the check compares against that same composition; a leftover is deleted only in a skill tree, only as `<name>/SKILL.md`, only when its first line after the frontmatter is byte-equal to the marker for `<name>`; `Check` and `driftIn`'s verdict are byte-for-byte what they were |
| Naming | The JSON keys for removed and left paths are kebab-case and absent when empty, like `stale` |
| Data flow | Nothing outside `internal/le/ai/sync` composes or parses a stub |
| Operator errors and doctor codes | The refusal for a source that cannot make a stub names the file, what is missing and the line to add, with the fix last |
| Rule: no-layering | The path rewrite, its constants and its test are gone; no option keeps full copies |
| Rule: stale-comments | The package comment of `ai.go` no longer says a skill is "copied verbatim" |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Stubs in all three directories | After `./le ai sync update`: every `SKILL.md` under the three directories is under 2,000 bytes and names its source |
| No path rewrite | A search of `internal/le/ai/sync/ai.go` for the two prefix constants answers nothing |
| A body edit owes no sync | Append a line to one skill source in a fixture: the check answers 0 (the wiring test) |
| Readers repointed | A search of `ai/skills`, `internal/le/rfc` and `ai/rationale` for a generated `SKILL.md` path answers nothing |
| Package tests green | `./le test unit pkg path ./internal/le/ai/sync/` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | A skill name comes from a file name the run listed in `ai/skills/`; the stub's path is built from it and never from frontmatter text, so a `name` field cannot steer the write or the pointer elsewhere. The refusal for a `name` that is not the stem enforces it |
| Deletion scope | The sync deletes only inside the three skill trees, only a `<name>/SKILL.md` whose first line after the frontmatter is the exact marker for `<name>`, only when no source has that name, and a directory only when that leaves it empty. Nothing under `.claude/agents` is ever deleted. A symlink in a tree is never followed out of it |
| Instruction injection | The stub's body is fixed text plus the source path. A source's `description` is copied into frontmatter as today, no more exposed than before |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| The live load of AC-11 shows an agent not reading the source | DESIGN: the stub's wording, then the choice of mechanism, with the evidence put to the owner |
| A test-judge refusal | The author fixes the code so the old test still holds, or brings the refusal to the main thread |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The Rex stub points at a command because Rex's guide is compiled into its binary. A command is the wrong pointer here: six skills exceed the harness's command-output limit and would arrive cut with no error. The right analogue of "ask the thing that owns the text" is a read of the file that owns it.
- The path rewrite for `.agents` looked like harness support and was mostly a defect: it invented 17 paths that do not exist. Removing the body removed the question.
- Design review, 2026-11-13 (independent agent, development tier, before any code). Verdict: not as written. What it changed: the stale-or-foreign split in the check was dropped, because it reached the subagent definitions and hid leftovers (`Check` is now untouched and the sync names what it leaves); the marker became one exact line in a fixed position; the stub was made to hold no argument placeholder, for the six skills that take arguments; the subagent definitions and the registered description joined the readers; two tests were relabelled as preservation tests; AC-5's fixture orders the bad source after a good one; AC-11 names its evidence and its limit to one harness; two measurements were corrected. One finding was not taken: the Architectural Verification table keeps its template answers, which the implementation fills with evidence at closure, as every spec does.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Stub with a read instruction | A symlink from each generated file to its source | Codex's skill walk skips a symlinked file, and Claude Code documents only symlinked directories, with an open listing defect. A symlink would also keep the description fresh, which the stub does not |
| Stub with a read instruction | Symlinked directories, with each source moved to `ai/skills/<name>/SKILL.md` | Works in all four harnesses on paper, but moves 34 files, changes every reference to them, and still meets Claude Code's listing defect. Offered to the owner and not chosen |
| A file read as the pointer | A command that prints the skill, as Rex does | Six skills exceed the 30,000 character output limit; it would also add a verb, a hook surface and launcher latency to every load |
| One stub for every harness | Claude Code's embedded-command placeholder in its own file | One harness only, with an undocumented output cap and undocumented behaviour in a subagent |
| Remove the copy | Keep copies and sync at session start | A source edited mid-session stays stale until the next session, and a hook that writes the tree is the defect recorded in `plan/journal/check-mode-mutates-the-tree.md` |
| A marker line in the body, exact and first after the frontmatter | A version field in frontmatter, as Rex's `rex-stub` | The check already compares bytes against a fresh generation, so no version is needed to find an outdated stub. The marker has one job: showing that le wrote a file it is about to delete. A substring match anywhere in the file would let a hand-written file quoting the marker be deleted |
| The check is not changed; the sync names what it leaves | The check sorts checkout-only paths into stale and foreign, and answers exit 0 for foreign | The first design, refused at the design review: the check walks `.claude/agents` too, where nothing carries a marker, so a deleted subagent's leftover would have stopped being reported. It also hid a pre-change leftover copy that every harness still loads. Leaving the check alone keeps every such file reported, and the sync's answer supplies the fix the warning could not |
| A phase agent reads the source directly | A phase agent loads the stub, then reads the source | The subagent definitions already tell the agent to read its skill; naming the source path there saves the hop and does not depend on the harness delivering a stub |
| Overwrite an unmarked file at a source's path | Refuse it as foreign, as Rex does | Every pre-change full copy is exactly such a file; refusing would make the first sync after this change fail everywhere |
| Refuse a source without `name` and `description` | Write a stub without them | Pi does not load a skill with no description and no harness could list it; a silent half-stub is the zero-as-answer defect |

## Known Limitations
- A skill that is added, removed, renamed or re-described still owes a sync. The frontmatter has to sit in the generated file because the harness reads it there.
- `AGENTS.md` and `.claude/agents/*.md` stay copies and can still be stale.
- The live proof (AC-11) runs in Claude Code. Codex, OpenCode and Pi follow the same stub pattern Rex installs for them, and are not proven here.
- A file in a generated tree that le cannot show it wrote stays reported as stale until it is deleted by hand; the sync names it and says so.
- Specs under `plan/` that cite a generated skill file are not edited (the census names them).
- Whether current Codex still reads a project's `.codex/skills` was not established; the directory keeps being written as today.
- OpenCode may list each skill twice because it reads two of the directories. Unverified, and unchanged by this spec.

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
- [ ] AC-1..AC-12 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/le/ai/sync/` and the package tests of `internal/le/rfc` that cover the enrolment refusal. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A: the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: le verbs carry no `.ci` suite; AC-11 is the end-to-end proof)
- [ ] Interop tests for protocol features (N-A: no protocol)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
