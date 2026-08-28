---
kind: table
level:
stage:
---
| What changed | Also check |
|---|---|
| New exported symbol | Wiring: who calls it? (`ai/rules/completion.md`) |
| Modified function signature | All callers (LSP findReferences or grep) |
| New goroutine | `ai/rules/goroutine-lifecycle.md`, cleanup on shutdown |
| New `make([]byte, N)` on wire path | Pool-backed alternative (`ai/rules/performance.md`) |
| New `fmt.Sprintf` | Append-based alternative (`ai/rules/performance.md`) |
| Guard/fallback added | Sibling call-site audit ("Sibling Call-Site Audit" above) |
| Error return ignored | `./le verify-lint run` reports the errcheck finding |
