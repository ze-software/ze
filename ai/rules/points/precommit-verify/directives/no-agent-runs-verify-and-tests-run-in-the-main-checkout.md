---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/precommit-verify.md
---
**An agent MUST NOT run any `./le verify` command, `./le verify changed` included: GitHub CI/CD verifies a pushed commit, and the evidence for a change is the scoped test runs of what it reaches (the named tests, the packages, the functional tests), which the main thread runs in the main checkout from the agent's owed commands.** A rule, skill, hook or tool message that tells an agent to run `./le verify` is wrong: correct it, never follow it. The full verification (`./le verify current`, `./le verify worktree`) is the release check, which CI runs and Thomas starts himself when he chooses; its exact command is the only form an agent gives him.
**Code MUST be developed and tested in the main checkout, never in a worktree, a `git archive` export or any other copy of the tree.** When another session's in-progress edit breaks the build there, report what broke and whose it is, and wait for it to clear; MUST NOT move to a copy to dodge it.
