---
kind: note
level:
stage:
---
A row in `test/weakened.md` does **not** authorize changing a tagged test: it is your own
justification, not the user's approval. Enforced by the `rfc-tagged-test` hook, which runs
before `test-weakening` precisely so the weakening record cannot pre-empt it
(`ai/rules/repo-maintenance.md`). Once the USER approves, record what they approved:
`// rfc-test-change-approved: <date> <what and why>`.

**A justification explains one diff, so it belongs with the commit and not in the file
forever.** That is the principle behind `test/weakened.md` being REPLACED per commit, and
its own prose gives the reason: a permanent comment "explains a change the reader of the
test file can no longer see", and storing it permanently "is what built the pile" -- 601
justifications across 413 files before the `test-relax:` mechanism was retired for it.
Before writing any justification an instruction hands you, check whether this repository
already has a canonical home for that class of record. A gate's message states a
constraint to satisfy; it does not decide where the record belongs.

**`rfc-test-change-approved:` is the one mechanism that has not moved yet, and it is
KNOWN to be on the wrong side of this principle** (owner ruling, 2026-08-19). The tree
holds 255 of them across 120 test files, plus 27 `test-relax:` that survived the reform
meant to replace them, and 6 `test-asserts-nothing:`. Keep writing the marker while the
hook demands it: a rule that forbids it today would refuse every author for obeying
another gate, and `scripts/dev/audit-test-relaxation.py` calls
`grep -rn 'rfc-test-change-approved'` its only backstop against a forged token. The
repair lands as one set -- a per-commit ledger, a `commit_helper.py` gate reading it, the
hook message rewritten to point at it, then the sweep -- and the ORDER is load-bearing:
sweeping first leaves the tree contradictory. Recorded in
`plan/journal/guard-message-teaches-the-violation.md`.
