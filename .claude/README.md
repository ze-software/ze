# .claude/ Directory Index

## Structure

```
.claude/
├── rules/              # Auto-loaded by file path (30 files)
│   ├── session-start.md      # TOP 6 RULES + session checklist (*)
│   ├── post-compaction.md    # Recovery after context compaction (*)
│   ├── before-writing-code.md # Pre-code checks (*)
│   ├── planning.md           # Pre-implementation planning (*)
│   ├── tdd.md                # TDD rules (**/*.go)
│   ├── go-standards.md       # Go coding standards (**/*.go)
│   ├── rfc-compliance.md     # RFC rules (internal/bgp/**/*.go)
│   ├── architecture-summary.md # Condensed system overview (*)
│   ├── naming.md             # Ze naming convention (*)
│   └── ...                   # See CLAUDE.md for full list + rationale
├── hooks/              # Automation scripts
│   ├── session-start.sh      # Git status, active specs (SessionStart)
│   ├── compaction-reminder.sh # Detect compaction (UserPromptSubmit)
│   ├── block-destructive-git.sh # Block dangerous git (PreToolUse:Bash)
│   ├── block-claude-plans.sh # Block wrong plan location (PreToolUse:Write)
│   ├── auto_linter.sh        # Lint on file write (PostToolUse)
│   └── validate-spec.sh      # Validate spec format (PostToolUse)
├── commands/           # Custom skills
│   ├── code-review.md  # /code-review for PR reviews
│   └── rfc-summarisation.md # /rfc-summarisation for RFC summaries
├── output-styles/      # Communication style
│   └── ze-style.md     # Terse emoji-prefixed
├── backups/            # Work preservation (git diff patches)
├── INDEX.md            # Doc navigation (RFC mappings, architecture docs)
└── settings.json       # Hooks, permissions, output style
```

## Quick Start

1. Rules auto-load based on file path
2. Hooks automate git check, linting, spec validation
3. Read `INDEX.md` to find architecture docs
4. Run `make lint && make unit-test && make functional-test` before claiming done

## Architecture Docs

Architecture documentation is in `docs/architecture/`:
- `docs/architecture/core-design.md` - **START HERE**
- `docs/architecture/wire/` - Wire formats
- `docs/architecture/behavior/` - FSM, signals
- `docs/architecture/api/` - API architecture
- `docs/architecture/config/` - Config syntax

## Key Workflows

### Planning
1. Write spec to `docs/plan/spec-<task>.md`
2. Follow template in `.claude/rules/planning.md`
3. Hook blocks writes to `.claude/plans/` (wrong location)

### Post-Compaction
1. Hook detects compaction, reminds to re-read spec
2. Spec has `## Post-Compaction Recovery` section listing what to read

---

**Updated:** 2026-01-22
