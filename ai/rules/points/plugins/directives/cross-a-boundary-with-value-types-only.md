---
kind: directive
level: MUST
stage:
---
- **A payload that crosses a plugin or component boundary MUST be a self-contained value type.** It carries no pointer field into data another plugin or component owns, and a shared core package is no exception. The surface-by-surface list is `docs/architecture/plugin/plugin-system.md`, "Cross-boundary value types".
