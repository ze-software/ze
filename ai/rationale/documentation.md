# Documentation Rationale

Why: `ai/rules/writing.md`, `ai/rules/documentation.md`

## Why the page comes first, and why its edit lands with the code

`ai/rules/documentation.md` states two obligations. Read the page before you
investigate. Update the page in the work that changed the behavior. Both exist
because the repository was paying twice.

A survey of the development skills on 2026-08-30 found the shape of the problem.
Of eleven skills that read or change code, only `/ze-close` and `/ze-doc-update`
carried any documentation instruction at all. `/ze-implement` carried none, and
deferred the whole **Documentation Update Checklist** to closure. No skill named
`ai/CODE-TO-DOCS.md`, `ai/DOCS-TO-CODE.md`, or `ai/INDEX.md`, so the two
generated indexes that map a file to its pages were reachable and unused.

That produces two costs, and the rule removes each one.

- An agent spawned to learn how a surface works rediscovers what a page already
  states. The session pays for the second answer, and every later session pays
  again, because nothing was written down that was not written down before.
- A page updated at closure is written from memory, about code that moved
  several times since. It records the intent rather than the shipped behavior.
  Between the code edit and the closure edit, the page is wrong, and a reader
  who trusts it is misled with no signal.

The ordering also protects `ai/rules/evidence.md`. A page is a map, never
evidence. Reading it first tells you where the producing function is. Reading
the producing function is still what settles what the code does.

## Historical Lesson: Content Drift
AGENT.md had UPPERCASE doc paths and TOP 5 rules while CLAUDE.md had moved to lowercase and TOP 6. The fix cost a full session. This is why "single source of truth" exists.

## Placement Decision Tree
0. A behavior rule every agent must follow? -> `ai/rules/points/<rule>/`, then render
1. Claude-only workflow rules? -> `.claude/rules/`
2. Agent skills/hooks? -> `ai/skills/` (synced to `.claude/skills/`), `.claude/hooks/`
3. Architecture/design docs? -> `docs/architecture/`
4. RFC summaries? -> `rfc/short/`
5. Wire format reference? -> `docs/architecture/wire/`
6. Implementation plan? -> `plan/`
7. Problem found while working? -> a row in `plan/journal/<class>.md`

## Directory Structure

```
docs/
  rfc/                     # RFC summaries
  architecture/
    wire/                  # Wire format docs
    behavior/              # FSM, signals
    config/                # Config syntax
    api/                   # API docs
    edge-cases/            # AS4, ADD-PATH, etc.
  exabgp/                  # ExaBGP comparison

ai/
  rules/                   # Shared agent rules, rendered from ai/rules/points/
  rules/points/            # The canonical rule source: one point per file
  skills/                  # Skill sources
  agents/                  # Subagent definitions
  patterns/                # Structural templates
  rationale/               # Why a rule exists
  digests/                 # Flow digests, entry to exit with file:line

plan/
  spec-*.md                # Active specs
  TEMPLATE.md              # Spec template

plan/journal/
  <class>.md               # One file per problem class, one row per occurrence
```

`ai/` is where an author edits. The tooling installs each agent's own copy
elsewhere, and those copies are generated and gitignored. `./le ai skills-sync`
writes `.claude/skills/`, `.claude/agents/`, `CLAUDE.md` and `AGENTS.md` from
`ai/`, and `./le rules render-update` writes `ai/rules/*.md` from
`ai/rules/points/`. Two directories stay hand-written and Claude-only, because
no other agent reads them: `.claude/rules/` and `.claude/hooks/`.

## Go File Naming Pattern
- `foo.go` -- Implementation
- `foo_test.go` -- Tests
- `platform_linux.go` -- Build-tagged
- `foo_gen.go` -- Generated

## Single Source of Truth

| Content | Canonical Location | Others Should |
|---------|-------------------|---------------|
| Native actions | `./le` + `ai/rules/testing.md` | Reference, not list |
| Architecture doc paths | `ai/INDEX.md` | Point to INDEX |
| Rule content | `ai/rules/points/<rule>/` | Point to `ai/rules/<rule>.md`, which is rendered from it |
| CLI patterns | `ai/rules/cli.md` | Point to rule file |

## Forbidden
- UPPERCASE for regular docs (except README, INDEX)
- snake_case for markdown files
