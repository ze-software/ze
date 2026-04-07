---
paths:
  - "**/*.go"
  - "docs/**"
---

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
| 5 | API/RPC docs | `docs/architecture/api/commands.md`, `docs/architecture/api/architecture.md` | New/changed RPCs or event types |
| 6 | Plugin guide | `docs/guide/plugins.md`, `docs/plugin-development/` | Plugin SDK or lifecycle changes |
| 7 | Wire format | `docs/architecture/wire/*.md` | Encoding/decoding changes |
| 8 | Plugin SDK rules | `.claude/rules/plugin-design.md` | Registration fields, protocol changes |
| 9 | RFC compliance | `rfc/short/rfcNNNN.md` | New RFC implementation |
| 10 | Test infrastructure | `docs/functional-tests.md`, `docs/architecture/testing/` | New test tools or patterns |
| 11 | Comparison | `docs/comparison.md` | Feature parity with other daemons |
| 12 | Architecture | `docs/architecture/core-design.md` or subsystem doc | Structural design changes |
| 13 | Route metadata | `docs/architecture/meta/README.md` + `docs/architecture/meta/<plugin>.md` | Plugin sets or reads route metadata keys |

## In Specs

Every spec MUST have a **Documentation Update Checklist** (see `plan/TEMPLATE.md`).
Each row answered Yes/No. Each Yes names the file and what to add.

## In /ze-implement

Stage 12 (Documentation review) is BLOCKING. Doc updates are part of the commit, not follow-up work.

## Source Anchors (BLOCKING)

**Every factual claim** in `docs/` must be verified against actual code before writing.
Never describe what you *think* the code does. Read the source first.

Add HTML comment anchors tying claims to code locations:

```
<!-- source: internal/component/bgp/reactor/forward_pool.go — ForwardPool -->
```

These are invisible in rendered markdown but let future sessions verify accuracy.

| Rule | Detail |
|------|--------|
| When to add | Every paragraph with a factual claim (syntax, field names, behavior, data structures) |
| Format | `<!-- source: <relative-path> — <symbol-or-topic> -->` |
| Placement | After the paragraph or table row containing the claim. **NEVER inside fenced code blocks** (between ` ``` ` delimiters) -- place after the closing fence. Inside code blocks, HTML comments render as visible text. |
| When editing docs | Verify existing anchors still match reality. Fix stale ones |
| When changing code | Check if any doc has an anchor pointing to the changed file. Update if claim is now wrong |
| Granularity | One anchor per factual paragraph or table. Not every sentence, not every file |

**Before writing any documentation:** read the actual source file. After writing: add the anchor.
**Before editing existing documentation:** grep for `<!-- source:` anchors, verify each one.

## Validation

Run `make ze-doc-test` after editing any file under `docs/`, after adding or removing a plugin, or after touching a YANG `ze:command` declaration. The umbrella target runs `check-doc-drift` (validates doc counts/lists vs live registry) and `validate-commands` (validates YANG `ze:command` <-> RPC handler contract). Both fail the make target on drift; both report all issues found.

Not part of `ze-verify` today because of a pre-existing drift backlog. Run on demand. See `docs/contributing/documentation-testing.md` for the full workflow and how to interpret output.

## NOT Documentation

- Code comments (`// Design:`, `// Related:`) -- covered by `design-doc-references.md` and `related-refs.md`
- Learned summaries (`plan/learned/`) -- covered by `spec-preservation.md`
- Memory entries -- covered by `memory.md`
