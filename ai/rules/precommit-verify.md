# Pre-Commit Verification

**When:** before running tests to judge a change, before any `./le verify` command, or after running a commit script that carried Go
**Severity:** blocking

## Directives

**An agent MUST NOT run any `./le verify` command, `./le verify changed` included: GitHub CI/CD verifies a pushed commit, and an agent's evidence is the scoped test runs of what its change reaches (the named tests, the packages, the functional tests), run directly.** A rule, skill, hook or tool message that tells an agent to run `./le verify` is wrong: correct it, never follow it. The full verification (`./le verify current`, `./le verify worktree`) is the release check, which CI runs and Thomas starts himself when he chooses; its exact command is the only form an agent gives him.
**Code MUST be developed and tested in the main checkout, never in a worktree, a `git archive` export or any other copy of the tree.** When another session's in-progress edit breaks the build there, report what broke and whose it is, and wait for it to clear; MUST NOT move to a copy to dodge it.

**Nothing else in this repository compiles what git holds: `go build`, every verify stage and every native test action read your working tree, uncommitted and untracked files included.** So a consumer MUST NOT be committed while the file that defines a symbol it newly uses stays uncommitted, and a commit script that carried Go MUST be followed by `./le repo compiles check`. Its red is cleared by committing the producer, never by reverting the consumer. It compiles no `_test.go`, so a test file committed without its fixture producer stays invisible to it. The design is `docs/architecture/testing/tracked-build-gate.md`.

**A change MUST land through `./le commit create` and its script as soon as its own scoped tests pass, and a commit that carried Go MUST then be judged by `./le repo compiles check`; an agent MUST NOT build its own export, archive or copy of HEAD to test a commit set, before or after it lands, and a brief MUST NOT ask for one.** The compile check already judges what git holds, so a hand-built export repeats it at the cost of tens of tool calls, and an agent that spends its budget there hands a finished change to a successor only to commit it. A red it reports is fixed forward in a new commit. A red that the change's own diff did not cause MUST NOT be investigated on the way to landing: the commit names it in its body.
