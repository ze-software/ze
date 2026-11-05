---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/context-economy.md
---
- **A command that can take longer than five minutes MUST NOT run from a subagent that has done other work: `./le go lint run`, `./le test unit all`, a `./le test functional` suite, or a `./le job run` that admits one of them.** A subagent's prompt cache lives five minutes and the main thread's one hour, so a call that outlasts the window ends the cache and the next call rewrites the whole context, at the write price, for one result. The price is the context at that moment: nothing in a fresh agent spawned for the gate alone, everything in an implementation agent at 500k. The 600-second Bash timeout is not a budget: a run that reaches it returns nothing and loses the cache anyway.
- **An editing agent hands its scoped commands to the main thread as OWED. All testing and code execution MUST run in the main thread, serialized to avoid overloading the shared machine; subagents MUST NOT run tests, builds, lint, formatters or other code.** The main thread runs the package-level `go test -race` recipe under `./le job run` in `docs/contributing/running-commands.md`, reads the actual results and dispatches findings to a fix agent.
