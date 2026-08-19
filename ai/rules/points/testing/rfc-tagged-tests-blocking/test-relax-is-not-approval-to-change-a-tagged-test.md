---
kind: note
level:
stage:
---
A row in `test/weakened.md` does **not** authorize changing a tagged test: it is your own
justification, not the user's approval. Enforced by the `rfc-tagged-test` hook, which runs
before `test-weakening` precisely so the weakening record cannot pre-empt it
(`ai/rules/repo-maintenance.md`). Once the USER approves, write what they approved as one
row in `test/rfc-changed.md`. The row names the test the change touches. Commit that file
with the change. The hook reads the file from disk, so the row comes first and the edit
comes second.

**A justification explains one diff, so it belongs with the commit and not in the file
forever.** That is the principle behind `test/weakened.md` being REPLACED per commit, and
its own prose gives the reason: a permanent comment "explains a change the reader of the
test file can no longer see", and storing it permanently "is what built the pile" -- 601
justifications across 413 files before the `test-relax:` mechanism was retired for it.
Before writing any justification an instruction hands you, check whether this repository
already has a canonical home for that class of record. A gate's message states a
constraint to satisfy; it does not decide where the record belongs.

**`rfc-test-change-approved:` was the last mechanism on the wrong side of this principle,
and it is RETIRED** (owner ruling, 2026-08-19). It was a comment, so it stayed in the test
file after the diff it explained was gone: 255 of them across 120 test files. Beside them
sit 27 `test-relax:` that survived the reform meant to replace them, and 6
`test-asserts-nothing:`.

Never write a new one. No gate reads one. `_rfc_tagged_change_err`
(`.claude/hooks/pretool-writeedit.py`) stopped accepting a marker in the edit's own text.
`rfc_changed_problems` (`scripts/dev/commit_helper.py`) stopped accepting one in a commit.
Both read `test/rfc-changed.md` instead. The 255 already in the tree leave in their own
sweep. Until it lands, a marker records an approval given before the ledger existed, and
it authorizes nothing you are editing today. Recorded in
`plan/journal/guard-message-teaches-the-violation.md`.
