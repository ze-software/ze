# Documentation

**BLOCKING:** Every feature change MUST update the specific documentation it affects.
Rationale: Code without matching docs is incomplete. "Update the docs" is not actionable.

## Principle

Name the file, name the section, describe the change. Never say "update documentation" generically.

## Documentation Categories

| # | Category | Location | When to update |
|---|----------|----------|----------------|
| 1 | Feature list | `docs/features.md` | New user-facing feature |
| 2 | User guide | `docs/guide/<topic>.md` | Feature with usage instructions |
| 3 | Config syntax | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` | Config format changes |
| 4 | CLI reference | `docs/guide/command-reference.md` | New/changed CLI commands |
| 5 | API/RPC docs | `docs/architecture/api/commands.md`, `api/architecture.md` | New/changed RPCs or event types |
| 6 | Plugin guide | `docs/guide/plugins.md`, `docs/plugin-development/` | Plugin SDK or lifecycle changes |
| 7 | Wire format | `docs/architecture/wire/*.md` | Encoding/decoding changes |
| 8 | Plugin SDK rules | `.claude/rules/plugin-design.md` | Registration fields, protocol changes |
| 9 | RFC compliance | `rfc/short/rfcNNNN.md` | New RFC implementation |
| 10 | Test infrastructure | `docs/functional-tests.md`, `docs/architecture/testing/` | New test tools or patterns |
| 11 | Comparison | `docs/comparison.md` | Feature parity with other daemons |
| 12 | Architecture | `docs/architecture/core-design.md` or subsystem doc | Structural design changes |

## In Specs

Every spec MUST have a **Documentation Update Checklist** (see `plan/TEMPLATE.md`).
Each row answered Yes/No. Each Yes names the file and what to add.

## In /implement

Stage 12 (Documentation review) is BLOCKING. Doc updates are part of the commit, not follow-up work.

## NOT Documentation

- Code comments (`// Design:`, `// Related:`) -- covered by `design-doc-references.md` and `related-refs.md`
- Learned summaries (`plan/learned/`) -- covered by `spec-preservation.md`
- Memory entries -- covered by `memory.md`
