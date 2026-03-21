# Documentation Rationale

Why: `.claude/rules/documentation.md`

## Historical Lesson: Content Drift
AGENT.md had UPPERCASE doc paths and TOP 5 rules while CLAUDE.md had moved to lowercase and TOP 6. The fix cost a full session. This is why "single source of truth" exists.

## Placement Decision Tree
1. Claude workflow rules? -> `.claude/rules/`
2. Claude commands/hooks? -> `.claude/commands/`, `.claude/hooks/`
3. Architecture/design docs? -> `docs/architecture/`
4. RFC summaries? -> `rfc/short/`
5. Wire format reference? -> `docs/architecture/wire/`
6. Implementation plan? -> `plan/`
7. Learned summary? -> `plan/learned/`

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
  rules/                   # Auto-loaded rules (action directives)
  rationale/               # On-demand rationale (why/examples)
  commands/                # Slash commands
  hooks/                   # Session hooks

plan/
  spec-*.md                # Active specs
  TEMPLATE.md              # Spec template

plan/learned/
  NNN-*.md                 # Learned summaries (completed spec knowledge)
```

## Go File Naming Pattern
- `foo.go` -- Implementation
- `foo_test.go` -- Tests
- `platform_linux.go` -- Build-tagged
- `foo_gen.go` -- Generated

## Single Source of Truth

| Content | Canonical Location | Others Should |
|---------|-------------------|---------------|
| Make targets | `Makefile` + `.claude/rules/testing.md` | Reference, not list |
| Architecture doc paths | `.claude/INDEX.md` | Point to INDEX |
| Rule content | `.claude/rules/<name>.md` | Point to rule file |
| CLI patterns | `.claude/rules/cli-patterns.md` | Point to rule file |

## Forbidden
- UPPERCASE for regular docs (except README, INDEX)
- snake_case for markdown files
