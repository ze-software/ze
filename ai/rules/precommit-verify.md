# Pre-Commit Verification

**When:** before running the verification gate, or after running a commit script that carried Go
**Severity:** blocking

## Directives

**The verification gate MUST run against a commit in a throwaway worktree and MUST NOT run against the working tree: an agent's run is `./le verify changed`, and the full verification, `./le verify worktree`, is Thomas's release check, whose exact command an agent gives him.** An in-place run is void the moment the tree moves under it, and it never says so: earlier stages judged a tree that no longer exists. A red from such a run MUST NOT be diagnosed as a defect, and its green MUST NOT be cited as evidence.

**Nothing else in this repository compiles what git holds: `go build`, every verify stage and every native test action read your working tree, uncommitted and untracked files included.** So a consumer MUST NOT be committed while the file that defines a symbol it newly uses stays uncommitted, and a commit script that carried Go MUST be followed by `./le repo compiles check`. Its red is cleared by committing the producer, never by reverting the consumer. It compiles no `_test.go`, so a test file committed without its fixture producer stays invisible to it. The design is `docs/architecture/testing/tracked-build-gate.md`.

**A change MUST land through `./le commit create` and its script as soon as its own package tests pass, and the commit MUST then be judged by `./le repo compiles check` and `./le verify changed`; an agent MUST NOT build its own export, archive or copy of HEAD to test a commit set before it lands, and a brief MUST NOT ask for one.** Those two checks already compile what git holds in a throwaway worktree, so a hand-built export repeats them at the cost of tens of tool calls, and an agent that spends its budget there hands a finished change to a successor only to commit it. A red they report is fixed forward in a new commit. A red that the change's own diff did not cause MUST NOT be investigated on the way to landing: the commit names it in its body. `./le verify changed` runs in the main thread, whose prompt cache outlives it.
