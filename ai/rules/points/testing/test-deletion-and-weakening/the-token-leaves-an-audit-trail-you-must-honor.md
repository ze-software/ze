---
kind: directive
level: MUST
stage:
---
**A row in `test/weakened.md` MUST state a real reason; writing one without a
reason is a violation.** The row unblocks the edit and leaves the audit trail,
and the commit MUST carry the file: `internal/le/commit` refuses a commit that
weakens a test without it, so no row is left behind in the working tree. Why the
file is replaced per commit rather than accumulated is
`docs/architecture/testing/test-health.md`.
