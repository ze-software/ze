---
kind: directive
level: MUST
stage:
---
**Every design decision MUST start from this reading, whatever the artifact.**

| What | Where | Prevents |
|------|-------|----------|
| Design principles | "Design Principles" below | "Good enough" backends, translation layers, implicit behavior, a missed abstraction (abstract at two use cases) |
| Plugin architecture | `ai/rules/plugins.md` | Wrong package, import violations, wrong communication mechanism |
| Registration pattern | `ai/patterns/registration.md` | A missing `init()`, registry, or blank import |
| Existing core packages | `ls internal/core/` | Missing an existing pattern such as `internal/core/family/` |
| Module tiers | `docs/architecture/module-tiers.md` | A package in the wrong tier, which fails `./le tier check` |
