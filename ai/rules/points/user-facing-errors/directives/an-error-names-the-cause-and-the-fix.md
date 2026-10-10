---
kind: directive
level: MUST
stage:
rationale: ai/rationale/user-facing-errors.md
---
- **Every error a user or an operator can see MUST say what went wrong in their terms, why it matters, and the exact next step or command that fixes it.** This binds every user interface Ze has, not the CLI alone: the CLI, the web UI, `ze doctor`, the logs, the `./le` tooling, and the API and gNMI error answers. The message MUST name what the reader typed, configured or installed, and MUST NOT name only the library or the call that failed. A bare exit code, an errno, a Go error chain, or a string internal to a tool (`exited 127`, `exec: "go": executable file not found in $PATH`) MUST NOT be the whole message: it MAY follow the explanation as evidence, never replace it.
