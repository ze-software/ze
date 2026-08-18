---
kind: directive
level: MUST
stage:
---
- **A command's response payload MUST be structured data. It MUST NOT be text a renderer already formatted.** `| json`, `| yaml` and `| table` are three renderings of ONE payload, and a handler that answers with finished text has picked the reader's format for them. `ResponseData` (`internal/component/plugin/types.go`) is what keeps a bare string out of `Response.Data`. `Map`, `Slice[T]` and a struct embedding `DataMarker` satisfy it with structure. `RawJSON` satisfies it too and is the one implementor that can carry finished text past the compiler, so it MUST hold `json.Marshal` output over a value. A decoder or formatter that can emit either shape MUST be asked for the structured one, and the pipe layer renders it.
