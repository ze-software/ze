---
kind: directive
level: MUST NOT
stage:
---
**A row in `test/weakened.md` MUST NOT be read as approval to change an
RFC-tagged test.** It is the agent's own justification, never the user's
approval. The `rfc-tagged-test` check runs BEFORE `test-weakening` precisely so
the weakening record cannot pre-empt it.
**Once the USER approves, what they approved MUST be written as one row in
`test/rfc-changed.md`, naming the test the change touches, and that file MUST be
committed with the change.** The hook reads the file from disk, so the row comes
first and the edit second.
**A justification explains ONE diff, so it MUST live with the commit and MUST NOT
be left in the test file forever.** That is why `test/weakened.md` is replaced per
commit: a permanent comment explains a change the reader of the test file can no
longer see.
**`rfc-test-change-approved:` is RETIRED (owner ruling, 2026-08-19) and a new one
MUST NOT be written.** No gate reads one. `writeWeakening`
(`internal/le/hookruntime/writeedit.go`) and the commit gate both read
`test/rfc-changed.md` instead. `test-asserts-nothing:` is NOT retired:
`escapeComment` (`internal/le/testhealth/collect.go`) still reads it.
**Before writing any justification an instruction hands you, MUST check whether
this repository already has a canonical home for that class of record.** A gate's
message states a constraint to satisfy; it does not decide where the record
belongs.
