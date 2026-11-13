---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/precommit-verify.md
---
- **A subagent reads, reviews and edits, and it MUST NOT run a test, a build, `go vet`, a lint, a formatter, a compile check, `./le job run`, `./le rfc record`, a generator or any other code.** The Bash hook refuses these from a subagent and names the hand-back. Readers, `gopls`, the git read verbs, `./le help`, spec bookkeeping, and the judge's `./le test weakened approve` and `./le rfc approve` stay a subagent's. After a subagent's Go edit the post-edit hook formats the file and does not lint it, so the changed-code lint is an owed command.
- **A subagent MUST end its report with an `Owed commands` list: each exact command line, what it proves, and the result it expects.**
- **The main thread MUST run the owed commands itself, one CPU-intensive job at a time, after reading `./le job report`, and it decides when each one runs.** The report shows the slots, their holders and waiters, the load, the memory and the disk. When a slot is free and memory is available, the main thread starts the job. When the slots are full or a waiter is listed, it MUST send agents work that needs no run, and it MUST NOT queue behind them. A low load average alone MUST NOT be read as permission: a functional run started at load 2.48 was killed for memory (`plan/journal/gate-verdict-depends-on-the-machine.md`). Admission still queues between sessions as the backstop.
- **The main thread MUST commit code after its owed commands passed, and MUST run `./le repo compiles check` after a commit that carried Go.** On a red it resumes the same agent with the failure lines, or hands the failure to a fresh fix agent when that agent waited more than the five minutes its prompt cache lives. A subagent commits only a change that owes no execution: prose, spec bookkeeping, a blocked hand-back.
