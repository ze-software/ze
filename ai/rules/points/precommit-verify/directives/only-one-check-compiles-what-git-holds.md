---
kind: directive
level: MUST NOT
stage:
---
**Nothing else in this repository compiles what git holds: `go build`, every verify stage and every native test action read your working tree, uncommitted and untracked files included.** So a consumer MUST NOT be committed while the file that defines a symbol it newly uses stays uncommitted, and a commit script that carried Go MUST be followed by `./le repo compiles check`. Its red is cleared by committing the producer, never by reverting the consumer. It compiles no `_test.go`, so a test file committed without its fixture producer stays invisible to it. The design is `docs/architecture/testing/tracked-build-gate.md`.
