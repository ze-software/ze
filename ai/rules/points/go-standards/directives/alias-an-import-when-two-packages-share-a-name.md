---
kind: directive
level: MUST
stage:
---
- When two packages in the module share the same name (e.g., `internal/component/iface/cli/` and `internal/component/iface/`), goimports cannot resolve which to use and silently removes the import. You MUST use an aliased import in this case: `ifacepkg "github.com/ze-software/ze/internal/component/iface"`.
- You MUST add import + usage in the same Edit call to prevent goimports from removing an "unused" import between edits.
