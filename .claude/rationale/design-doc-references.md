# Design Document References Rationale

Why: `.claude/rules/design-doc-references.md`

## Why This Rule Exists
Without traceability, developers cannot find the reasoning behind code decisions. When splitting, refactoring, or debugging, the first question is "what design governs this?" -- the answer should be in the file itself, not discovered through archaeology.

## What Counts as a Design Document

| Document Type | Location | When to Reference |
|---------------|----------|-------------------|
| Architecture docs | `docs/architecture/` | Primary design source |
| Wire format docs | `docs/architecture/wire/` | Wire encoding/decoding files |
| API docs | `docs/architecture/api/` | API command handling files |
| Config docs | `docs/architecture/config/` | Config parsing files |
| RFC summaries | `rfc/short/` | Protocol implementation files |
| Completed specs | `docs/plan/done/` | Files created by a specific spec |

## Format Examples

Primary package file (with `// Package` line):
```
// Package reactor implements the BGP event loop and peer management.
//
// Design: docs/architecture/core-design.md
// Design: docs/architecture/wire/messages.md
package reactor
```

Non-primary file (no `// Package` line):
```
// Design: docs/architecture/core-design.md -- peer FSM state transitions
//
// This file implements the per-peer finite state machine.
package reactor
```

## Specificity Levels

| Level | Example | When |
|-------|---------|------|
| Document only | `// Design: docs/architecture/core-design.md` | File relates to entire doc |
| With section | `// Design: docs/architecture/core-design.md Section 4` | File relates to specific section |
| With topic | `// Design: docs/architecture/core-design.md -- pool architecture` | Preferred: survives doc restructuring |

## Validation Checks
- New files have `// Design:` comments
- Split files carried forward relevant references
- Referenced documents actually exist (stale paths = broken traceability)
