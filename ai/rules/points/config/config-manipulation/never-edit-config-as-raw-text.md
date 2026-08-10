---
kind: directive
level: MUST NOT
stage:
---
**Config MUST NOT be edited by any of these methods:**
- Raw text surgery (regex, string replace, brace counting, line insertion)
- Custom merge functions that parse config syntax outside the config system
- Any manipulation that assumes config structure from text patterns
