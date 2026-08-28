# .claude/ Directory Index

## Structure

```
.claude/
├── rules/              # Auto-loaded by file path (30 files)
│   ├── session-start.md      # TOP 6 RULES + session checklist (*)
│   ├── post-compaction.md    # Recovery after context compaction (*)
│   ├── architecture.md # Pre-code checks (*)
│   ├── planning.md           # Pre-implementation planning (*)
│   ├── testing.md                # TDD rules (**/*.go)
│   ├── go-standards.md       # Go coding standards (**/*.go)
│   ├── rfc-compliance.md     # RFC rules (internal/bgp/**/*.go)
│   ├── architecture.md # Condensed system overview (*)
│   ├── go-standards.md             # Ze naming convention (*)
│   └── ...                   # See CLAUDE.md for full list + rationale
├── hooks/              # README.md only; every hook now runs in `./le hook-check <verb>`
│   └── README.md       # Native hook dispatcher: see .claude/settings.json for the verb list
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
4. Run `./le verify-lint run && ./le test-unit && ./le functional` before claiming done

## Architecture Docs

Architecture documentation is in `docs/architecture/`:
- `docs/architecture/core-design.md` - **START HERE**
- `docs/architecture/wire/` - Wire formats
- `docs/architecture/behavior/` - FSM, signals
- `docs/architecture/api/` - API architecture
- `docs/architecture/config/` - Config syntax

## Key Workflows

### Planning
1. Write spec to `plan/spec-<task>.md`
2. Follow template in `.claude/rules/planning.md`
3. Hook blocks writes to `.claude/plans/` (wrong location) <!-- doc-links: ignore (banned location, deliberately nonexistent) -->

### Post-Compaction
1. Hook detects compaction, reminds to re-read spec
2. Spec has `## Post-Compaction Recovery` section listing what to read

---

**Updated:** 2026-01-22
