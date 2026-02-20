# Test Deletion Guidelines

Rationale: `.claude/rationale/no-test-deletion.md`

ASK for user approval before deleting any test code (`*_test.go`, `.ci`, `Test*`, `t.Run`, assertions, table entries).
Exception: user already explicitly requested the deletion.

**Legitimate:** Testing removed functionality, duplicating another test, fundamentally wrong test, replacing with better coverage.
**Not legitimate:** Failing and hard to fix, slow, "annoying", don't understand what it checks.
