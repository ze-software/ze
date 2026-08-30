# Pre-Commit Verification

**When:** before running the verification gate, or after running a commit script that carried Go
**Severity:** blocking

## Directives

**The verification gate MUST run against a COMMIT in a throwaway worktree, `./le verify worktree`, and MUST NOT run against the working tree (owner directive, 2026-08-21).** An in-place run is void the moment the tree moves under it, and it never says so: earlier stages judged a tree that no longer exists. A red from such a run MUST NOT be diagnosed as a defect, and its green MUST NOT be cited as evidence.

**Nothing else in this repository COMPILES what git HOLDS: `go build`, every verify stage and every native test action read your WORKING TREE, uncommitted and untracked files included.** So a CONSUMER MUST NOT be committed while the file that DEFINES a symbol it newly uses stays uncommitted, and a commit script that carried Go MUST be followed by `./le repository tracked-build check`. Its red is cleared by committing the producer, never by reverting the consumer. It compiles no `_test.go`, so a test file committed without its fixture producer stays invisible to it. The design is `docs/architecture/testing/tracked-build-gate.md`.
