---
kind: table
level:
stage:
---
| What | Where | Prevents |
|------|-------|----------|
| Design principles | "Design Principles" below | "Good enough" backends, translation layers, implicit behavior, missed abstractions (abstract at 2+ use cases) |
| Plugin architecture | `ai/rules/plugins.md` | Wrong package, import violations, wrong comm mechanism |
| Registration pattern | `ai/patterns/registration.md` | Missing init + registry + blank import pattern |
| Existing core packages | `ls internal/core/` | Missing patterns like `internal/core/family/` |
