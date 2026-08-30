---
kind: directive
level: MUST NOT
stage:
---
**Nothing else in this repository COMPILES what git HOLDS: `go build`, every verify stage and every native test action read your WORKING TREE, uncommitted and untracked files included.** So a CONSUMER MUST NOT be committed while the file that DEFINES a symbol it newly uses stays uncommitted, and a commit script that carried Go MUST be followed by `./le repository tracked-build check`. Its red is cleared by committing the producer, never by reverting the consumer. It compiles no `_test.go`, so a test file committed without its fixture producer stays invisible to it. The design is `docs/architecture/testing/tracked-build-gate.md`.
