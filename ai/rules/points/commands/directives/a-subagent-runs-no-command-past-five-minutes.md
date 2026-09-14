---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/context-economy.md
---
- **A command that can take longer than five minutes MUST NOT run from a subagent that has done other work: `./le verify lint run`, `./le verify worktree`, `./le test-unit all`, a `./le functional` suite, or a `./le job run` that admits one of them.** A subagent's prompt cache lives five minutes and the main thread's one hour, so a call that outlasts the window ends the cache and the next call rewrites the whole context, at the write price, for one result. The price is the context at that moment: nothing in a fresh agent spawned for the gate alone, everything in an implementation agent at 500k. The 600-second Bash timeout is not a budget: a run that reaches it returns nothing and loses the cache anyway.
- **An editing agent proves its own package with the scoped run, and the gates it owes go in its handoff under "Verified green" as OWED: the main thread MUST run them once after the phase agents return, itself or through a fresh `/ze-verify` agent, and dispatch the findings to a fix agent.** The scoped run is the package-level `go test -race` recipe under `./le job run` in `docs/contributing/running-commands.md`, and the post-write hook has already linted each package the agent wrote (`postFormatGo`, `internal/le/hookruntime/postwrite.go`).
