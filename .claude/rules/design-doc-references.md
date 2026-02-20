# Design Document References

**BLOCKING:** All source files MUST reference the design document(s) that govern them.

## Why This Rule Exists

Without traceability, developers cannot find the reasoning behind code decisions. When splitting, refactoring, or debugging, the first question is "what design governs this?" — the answer should be in the file itself, not discovered through archaeology.

## The Rule

Every `.go` source file (excluding generated files and test files) MUST include a `// Design:` comment referencing the architecture or design document(s) that describe its purpose, data flow, or behavioral contract.

## Format

Place the reference in the **package doc comment block** at the top of the file, after the `// Package` line:

```
// Package reactor implements the BGP event loop and peer management.
//
// Design: docs/architecture/core-design.md
// Design: docs/architecture/wire/messages.md
package reactor
```

For files that are not the primary package file (no `// Package` line), place it as the first comment:

```
// Design: docs/architecture/core-design.md — peer FSM state transitions
//
// This file implements the per-peer finite state machine.
package reactor
```

## What Counts as a Design Document

| Document Type | Location | When to Reference |
|---------------|----------|-------------------|
| Architecture docs | `docs/architecture/` | Primary design source |
| Wire format docs | `docs/architecture/wire/` | Wire encoding/decoding files |
| API docs | `docs/architecture/api/` | API command handling files |
| Config docs | `docs/architecture/config/` | Config parsing files |
| RFC summaries | `rfc/short/` | Protocol implementation files |
| Completed specs | `docs/plan/done/` | Files created by a specific spec |

## Specificity

Be as specific as useful:

| Level | Example | When |
|-------|---------|------|
| Document only | `// Design: docs/architecture/core-design.md` | File relates to entire doc |
| With section | `// Design: docs/architecture/core-design.md Section 4` | File relates to specific section |
| With topic | `// Design: docs/architecture/core-design.md — pool architecture` | Section numbers may shift |

Topic annotations (after `—`) are preferred over section numbers because they survive doc restructuring.

## When to Add References

| Situation | Action |
|-----------|--------|
| Creating a new file | Add `// Design:` before writing code |
| Splitting a file | Each new file inherits relevant references from the original |
| Touching a file that lacks references | Add references for the parts you understand |
| No design doc exists | Note `// Design: (none — predates documentation)` to flag the gap |

## Exemptions

| File Type | Why Exempt |
|-----------|------------|
| `*_test.go` | Tests reference specs via test names, not doc comments |
| `*_gen.go` / generated files | Generated code is not maintained by hand |
| `register.go` (plugin init) | Trivial — just calls `registry.Register()` |
| `embed.go` (schema embeds) | Trivial — just `//go:embed` directives |
| `doc.go` | Already IS documentation |

## Validation

When reviewing code, check:
- New files have `// Design:` comments
- Split files carried forward relevant references
- Referenced documents actually exist (stale paths = broken traceability)
