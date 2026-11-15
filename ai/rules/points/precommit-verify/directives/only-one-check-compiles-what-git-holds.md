---
kind: directive
level: MUST NOT
stage:
---
**Only `./le repo compiles check` compiles what git holds: `go build`, every other verify stage and every native test action read your working tree, uncommitted and untracked files included.** So a consumer MUST NOT be committed while the file that defines a symbol it newly uses stays uncommitted. That compile is a repo-wide compile, which is full testing: it runs in the release check GitHub CI runs on a pushed commit, and a session MUST NOT start it on its own initiative, after a commit that carried Go included. A commit that carried Go owes the scoped tests of the packages it reaches and nothing wider; the committed-tree compile is CI's. A red it reports is cleared by committing the producer, never by reverting the consumer. It compiles no `_test.go`, so a test file committed without its fixture producer stays invisible to it. The design is `docs/architecture/testing/tracked-build-gate.md`.
