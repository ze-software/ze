---
kind: note
level:
stage:
---
`test/draft/` is gitignored and skipped by every repo-wide gate, so a draft
cannot redden anything for anyone. Changing an existing test is the same move:
copy it into the incubator, work there, `mv` it back. Full workflow: the
`/ze-test` skill, `test/draft/README.md`, `docs/functional-tests.md`.
