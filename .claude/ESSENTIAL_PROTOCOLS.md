# Essential Protocols Reference

**Note:** Core rules are now in `.claude/rules/` and auto-loaded by file path.
This file is kept as extended reference only.

## Quick Links

| Topic | Rule File |
|-------|-----------|
| TDD workflow | `rules/tdd.md` |
| Go standards | `rules/go-standards.md` |
| RFC compliance | `rules/rfc-compliance.md` |
| Git safety | `rules/git-safety.md` |

## Session Start

Automated via `hooks/session-start.sh`:
- Checks git status
- Reports test status from `plan/CLAUDE_CONTINUATION.md`
- Prompts if modified files exist

## File Locations

| Type | Location |
|------|----------|
| Session state | `plan/CLAUDE_CONTINUATION.md` |
| Plans, specs | `plan/` |
| Architecture docs | `.claude/zebgp/` |
| Backups | `.claude/backups/` |

## Verification Commands

```bash
make test       # Unit tests
make lint       # golangci-lint
make functional # Functional tests (37)
```

## Self-Review (After Task Completion)

1. Review changes for issues
2. Fix critical/medium issues immediately
3. Report minor items to user
4. Run `make test && make lint`

## Error Recovery

If something goes wrong:
1. Save: `git diff > .claude/backups/work-$(date +%Y%m%d-%H%M%S).patch`
2. Ask user for direction
3. Never discard work without permission

## Refactoring Protocol

- One function/type at a time
- Run verification after each step
- Paste exact output (no summaries)
- Stop if tests fail

---

**Updated:** 2025-12-31
