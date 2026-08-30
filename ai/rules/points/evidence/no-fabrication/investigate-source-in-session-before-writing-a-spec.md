---
kind: directive
level: MUST
stage:
---
**You MUST read the implementation source this session before writing a spec or a design.** `writeDesignEvidence` (`internal/le/hookruntime/writeedit.go`) refuses a Write or Edit to a spec or design file under `plan/`, or to anything under `.claude/plan/`, when the session recorded no source read, and it is fail-closed on a session id it cannot resolve.
**Only a Read of a `.go`, `.sh`, `.yang`, `.mk` or `Makefile` path records the marker** (`hookSourceRead`, `internal/le/hookruntime/lifecycle.go`). A loaded LSP tool satisfies the gate on its own.
**The gate is a BACKSTOP and you MUST NOT treat passing it as evidence.** It cannot tell whether the code you read is the code your claim depends on, it accepts ANY recorded read rather than the spec's own subject, and a `Bash` investigation with `grep` or `sed` is invisible to it. The obligation above is what you owe; the gate only catches never having looked.
