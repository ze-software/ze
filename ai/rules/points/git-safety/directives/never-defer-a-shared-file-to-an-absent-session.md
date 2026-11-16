---
kind: directive
level: MUST NOT
stage:
rationale: ai/rationale/git-safety.md
---
**Concurrent edits MUST NOT make a file unavailable for your commit. When the affected code compiles, commit the whole changed files even when they contain other sessions' changes.** Do not wait for a clean file or sole authorship. Acknowledge the concurrent changes in the commit body, naming their owners when known.

**The committed snapshot MUST include the dependencies it needs.** Judge it against HEAD plus the selected files. A compiling working tree does not prove a selected snapshot compiles when an unselected file supplies a new function or type. Carry the required files in the same coherent chunk or coordinate their publication, rather than treating another session's authorship as a blocker.

**Hunk selection MUST NOT be used merely to isolate authorship.** It remains available when a genuinely separate change cannot land in the same coherent chunk. Use `hunks <path> <patch-file>` without removing anyone's edits from the working-tree file. The content gates and review pin judge HEAD plus that patch, while the working-tree file keeps every session's work (`docs/contributing/committing.md`, "Committing some hunks of a shared file").

**You MUST NOT leave completed work uncommitted on the assumption that another session will land the shared file.** Commit the compiling chunk through the native named-file route. Shared files are normal in this checkout, and another session's edits alone require neither permission nor a handoff.
