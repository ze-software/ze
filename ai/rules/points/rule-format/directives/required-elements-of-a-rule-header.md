---
kind: table
level: MUST NOT
stage:
---
| Element | Requirement |
|---------|-------------|
| `# Title` | First non-blank line, a single H1. It MUST NOT contain "BLOCKING": `**Severity:**` carries that, and no tool can read a title marker. |
| `**When:** <trigger>` | Required. One line. The SITUATION that makes this rule apply, phrased so an agent can match it against the task at hand. See "The trigger is a routing key". |
| `**Severity:** blocking\|advisory` | Required. `blocking` = a gate/hook enforces it or violating it breaks correctness; `advisory` = strong convention. It MUST agree with the prose: a rule whose body says BLOCKING may not declare `advisory`. |
| `**Related:** slug, slug` | Optional. Comma-separated rule slugs (filename without `.md`), no paths. |
