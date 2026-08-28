---
kind: directive
level: MUST
stage:
---
**Reads never block.** `hookSourceRead` and the LSP lifecycle action in `internal/le/hookruntime/lifecycle.go` write non-blocking, session-scoped evidence markers. `writeDesignEvidence` in `internal/le/hookruntime/writeedit.go` consumes those markers before a design or spec write. A Read MUST return implementation content to count; an empty response, failed read, or a window under the native depth threshold records nothing.
