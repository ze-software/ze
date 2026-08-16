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
