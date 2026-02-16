---
paths:
  - "**/*.md"
  - "docs/**/*"
  - ".claude/**/*"
---

# Documentation Standards

## File Naming

| Type | Convention | Example |
|------|------------|---------|
| Docs | lowercase-hyphens | `pool-architecture.md` |
| Go files | snake_case | `pool_test.go` |
| Packages | lowercase | `package pool` |
| Special files | UPPERCASE | `README.md`, `INDEX.md` |

### Go Files

```
foo.go           # Implementation
foo_test.go      # Tests
platform_linux.go # Build-tagged
foo_gen.go       # Generated
```

## Documentation Placement

### Decision Tree

1. **Claude workflow rules?** → `.claude/rules/`
2. **Claude commands/hooks?** → `.claude/commands/`, `.claude/hooks/`
3. **Architecture/design docs?** → `docs/architecture/`
4. **RFC summaries?** → `rfc/short/`
5. **Wire format reference?** → `docs/architecture/wire/`
6. **Implementation plan?** → `docs/plan/`
7. **Completed plan?** → `docs/plan/done/`

### Directory Structure

```
docs/
├── rfc/                    # RFC summaries
├── architecture/
│   ├── wire/               # Wire format docs
│   ├── behavior/           # FSM, signals
│   ├── config/             # Config syntax
│   ├── api/                # API docs
│   └── edge-cases/         # AS4, ADD-PATH, etc.
├── exabgp/                 # ExaBGP comparison
└── *.md                    # General docs

.claude/
├── rules/                  # Claude workflow rules
├── commands/               # Slash commands
├── hooks/                  # Session hooks
├── output-styles/          # Communication style
└── INDEX.md                # Navigation

docs/plan/
├── spec-*.md               # Active specs
└── done/                   # Completed specs
```

### Quick Reference

| Doc Type | Location |
|----------|----------|
| Claude rules | `.claude/rules/` |
| Wire formats | `docs/architecture/wire/` |
| RFC summaries | `rfc/short/` |
| Design docs | `docs/architecture/` |
| API docs | `docs/architecture/api/` |
| Config docs | `docs/architecture/config/` |
| Edge cases | `docs/architecture/edge-cases/` |
| Implementation plans | `docs/plan/` |
| Completed plans | `docs/plan/done/` |

## File Size Policy

- Reference docs: < 15 KB
- Plans: < 10 KB
- READMEs: < 3 KB

**If exceeding: compress, don't split**

## Single Source of Truth

Never duplicate information across documentation files. When multiple files need the same content:

1. **Pick one canonical location** — the file closest to the topic owns it
2. **Point, don't copy** — other files reference it with a path, not a summary
3. **Index files are pointers** — `CLAUDE.md`, `AGENT.md`, `INDEX.md` link to rules, they don't restate them

| Content | Canonical Location | Others Should |
|---------|-------------------|---------------|
| Make targets | `Makefile` + `.claude/rules/testing.md` | Reference, not list |
| Architecture doc paths | `.claude/INDEX.md` | Point to INDEX |
| Rule content | `.claude/rules/<name>.md` | Point to rule file |
| CLI patterns | `.claude/rules/cli-patterns.md` | Point to rule file |
| RFC keyword mapping | `.claude/rules/planning.md` | Point to planning |

**Why:** Duplicated content drifts. AGENT.md had UPPERCASE doc paths and TOP 5 rules while CLAUDE.md had moved to lowercase and TOP 6. The fix cost a full session.

## Forbidden

- Docs in `docs/architecture/` (moved to `docs/`)
- UPPERCASE for regular docs (except README, INDEX)
- snake_case for markdown files
