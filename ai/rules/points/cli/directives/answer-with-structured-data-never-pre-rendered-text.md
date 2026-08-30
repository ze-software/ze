---
kind: directive
level: MUST
stage:
---
- **A command's response payload MUST be structured data satisfying `ResponseData` (`internal/component/plugin/types.go`), and MUST NOT be text a renderer already formatted.** `| json`, `| yaml` and `| table` are three renderings of ONE payload, so a handler that answers with finished text has picked the reader's format for them, and a row's state belongs in a field rather than a character glued to an identifier. Every command that produces output MUST route it through `ApplyPipes` and answer every GLOBAL operator on every surface the catalog (`internal/component/command/pipe_catalog.go`) makes it available on, deriving availability from that catalog rather than a hand-copied list; an operator the answer shape cannot support MUST be refused BY NAME, and the `always`, `with-rows`, `when-streaming` and `local-only` qualifiers MUST NOT be flattened into unconditional support.
