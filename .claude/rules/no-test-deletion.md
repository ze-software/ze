---
paths:
  - "**/*.go"
  - "test/**"
---

# Test Deletion

Rationale: `.claude/rationale/no-test-deletion.md`

ASK user before deleting any test code (`*_test.go`, `.ci`, `Test*`, `t.Run`, assertions, table entries).
Exception: user already explicitly requested the deletion.

**Legitimate:** testing removed functionality, duplicating another test, fundamentally wrong, replacing with better coverage.
**Not legitimate:** failing and hard to fix, slow, "annoying", don't understand what it checks.
