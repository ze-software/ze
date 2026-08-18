---
kind: directive
level: MUST
stage:
---
- **Every CLI command MUST place a closed keyword before any user-supplied value.** This eliminates ambiguity where a free-form value could collide with a keyword.
- **All CLI commands MUST follow the patterns in "CLI Patterns" below.** Structural template: `ai/patterns/cli-command.md`. Rationale: `ai/rationale/cli-patterns.md`.
- **Every command that produces output MUST support all pipe operators.**
- **A command's response payload MUST be structured data. It MUST NOT be text a renderer already formatted.** `| json`, `| yaml` and `| table` are three renderings of ONE payload. A handler that answers with finished text has picked the reader's format for them, and it leaves the other two renderings nothing to work with. `ResponseData` (`internal/component/plugin/types.go`) is what enforces this: only `Map`, `Slice[T]`, and a struct embedding `DataMarker` satisfy the interface, which is how a bare string stays out of `Response.Data`. A decoder or formatter that can emit either shape MUST be asked for the structured one, and the pipe layer renders it.
- **A row's state MUST be a field or a column; it MUST NOT be a character glued to a value.** No `*`, `>`, `+` or leading dot on an identifier. A sigil corrupts the value for `| grep` and has nowhere to live in `| json`, so the text and JSON forms stop agreeing on what the value is. `*` is also already an input token here, the selector wildcard. See "A value carries no marker" below.
- **Every error, log line, and failure output you write MUST let a human or an agent see what failed, why, and what to do next, without opening the source.** The error is the corrective signal: if it does not point at the fix, the reader cannot act and an agent cannot self-correct.
- **All JSON output MUST follow the conventions in "JSON Format" below.** Rationale: `ai/rationale/json-format.md`.
- **All agent-facing CLI output MUST follow the rules in "Agent Tooling Contract" below.**
