---
paths:
  - "docs/plan/spec-*.md"
---

# No Code in Specs

Rationale: `.claude/rationale/spec-no-code.md`

**BLOCKING:** Specs MUST NOT contain code snippets (any language).

| Instead of | Use |
|------------|-----|
| Go struct | Table: Field / Type / Description |
| Function implementation | Prose: numbered steps describing behavior |
| Code example | Text: input/output format |
| State machine code | State transition table |

Validated by `validate-spec.sh` hook.
