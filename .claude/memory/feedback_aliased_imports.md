---
name: Aliased imports for duplicate package names
description: goimports silently removes imports when two packages share the same name; use aliased imports to prevent this
type: feedback
---

When two Go packages in the module share the same name (e.g., `cmd/ze/iface/` and `internal/component/iface/`), goimports cannot resolve which to use and silently removes the import.

**Why:** goimports uses package name matching. With ambiguous names, it drops the import rather than guessing, causing undefined symbol errors on the next lint pass.

**How to apply:** Always use an aliased import when importing a package whose name is shared by another package in the module. Example: `ifacepkg "codeberg.org/thomas-mangin/ze/internal/component/iface"`. Also, add import + usage in the same Edit call to prevent goimports from removing an "unused" import between edits.
