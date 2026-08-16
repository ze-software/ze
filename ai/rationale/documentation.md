# Documentation Rationale

Why: `ai/rules/writing.md`

## Historical Lesson: Content Drift
AGENT.md had UPPERCASE doc paths and TOP 5 rules while CLAUDE.md had moved to lowercase and TOP 6. The fix cost a full session. This is why "single source of truth" exists.

## Placement Decision Tree
1. Claude workflow rules? -> `.claude/rules/`
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

.claude/
  rules/                   # Auto-loaded Claude-specific rules
  skills/                  # Generated from ai/skills/ (make ze-ai-skills-sync)
  hooks/                   # Session hooks

plan/
  spec-*.md                # Active specs
  TEMPLATE.md              # Spec template

plan/journal/
  <class>.md               # One file per problem class, one row per occurrence
```

## Go File Naming Pattern
- `foo.go` -- Implementation
- `foo_test.go` -- Tests
- `platform_linux.go` -- Build-tagged
- `foo_gen.go` -- Generated

## Single Source of Truth

| Content | Canonical Location | Others Should |
|---------|-------------------|---------------|
| Make targets | `Makefile` + `ai/rules/testing.md` | Reference, not list |
| Architecture doc paths | `ai/INDEX.md` | Point to INDEX |
| Rule content | `.claude/rules/<name>.md` | Point to rule file |
| CLI patterns | `ai/rules/cli.md` | Point to rule file |

## Forbidden
- UPPERCASE for regular docs (except README, INDEX)
- snake_case for markdown files
