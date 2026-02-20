---
paths:
  - "**/*.md"
  - "docs/**/*"
  - ".claude/**/*"
---

# Documentation Standards

Rationale: `.claude/rationale/documentation.md`

## File Naming

| Type | Convention | Example |
|------|------------|---------|
| Docs | lowercase-hyphens | `pool-architecture.md` |
| Go files | snake_case | `pool_test.go` |
| Packages | lowercase | `package pool` |
| Special | UPPERCASE | `README.md`, `INDEX.md` |

## Placement

| Content | Location |
|---------|----------|
| Claude workflow rules | `.claude/rules/` |
| Rule rationale/examples | `.claude/rationale/` |
| Architecture/design | `docs/architecture/` |
| Wire format | `docs/architecture/wire/` |
| API docs | `docs/architecture/api/` |
| Config docs | `docs/architecture/config/` |
| RFC summaries | `rfc/short/` |
| Active specs | `docs/plan/` |
| Completed specs | `docs/plan/done/` |

## Single Source of Truth

Never duplicate content. Pick one canonical location, others reference by path.

| Content | Canonical Location |
|---------|-------------------|
| Make targets | `Makefile` + `rules/testing.md` |
| Architecture doc paths | `.claude/INDEX.md` |
| Rule content | `.claude/rules/<name>.md` |
| RFC keyword mapping | `.claude/INDEX.md` |

## File Size

Reference docs < 15 KB, plans < 10 KB, READMEs < 3 KB. Compress, don't split.
