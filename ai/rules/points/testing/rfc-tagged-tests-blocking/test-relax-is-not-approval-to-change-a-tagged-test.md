---
kind: note
level:
stage:
---
`// test-relax:` does **not** authorize changing a tagged test: it is your own
justification, not the user's approval. Enforced by the `rfc-tagged-test` hook, which runs
before `test-weakening` precisely so the relax token cannot pre-empt it
(`ai/rules/repo-maintenance.md`). Once the USER approves, record what they approved:
`// rfc-test-change-approved: <date> <what and why>`.
