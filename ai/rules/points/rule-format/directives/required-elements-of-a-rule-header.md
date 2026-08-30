---
kind: directive
level: MUST
stage:
---
**A rule header MUST carry these elements, in this exact order:**

| Element | Requirement |
|---------|-------------|
| `# Title` | First non-blank line, a single H1. It MUST NOT contain "BLOCKING": `**Severity:**` carries that, and no tool can read a title marker |
| `**When:** <trigger>` | Required. One line. The SITUATION that makes this rule apply, phrased so an agent can match it against the task at hand |
| `**Severity:** blocking\|advisory` | Required. `blocking` means a gate or hook enforces it, or violating it breaks correctness; `advisory` means a strong convention. It MUST agree with the prose: a rule whose body says BLOCKING MUST NOT declare `advisory` |
| `**Related:** slug, slug` | Optional. Comma-separated rule slugs (filename without `.md`), no paths |
