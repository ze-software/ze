---
kind: directive
level: MUST
stage:
---
- **A command MUST write errors to stderr and MUST return an exit code; `os.Exit()` MUST NOT be called in a handler.** `-` means stdin or stdout, and a user-supplied path MUST be read or written through `internal/core/cliio` rather than a raw `os` call, which `./le dash-stdio check` enforces. Every error MUST say what failed, why, and what to do next, naming what the operator configured rather than the library, and a check that cannot run MUST return an error rather than a zero result. Every validation error surfaced to an agent MUST carry a stable diagnostic code, registered with its explanation. Every user-facing command MUST have tab-completion; `Hidden: true` on a `CommandDecl` is the exception, for an internal or diagnostic command that still works when typed in full.
