---
kind: directive
level:
stage:
---
1. **Symptom** -- the exact failure, verbatim (error text, rejected input, failing assertion).
2. **Root cause** -- traced to the exact function where behavior diverges from intent, named as the file plus the symbol. Read the path; do not guess. If you cannot name it, you have not diagnosed it yet.
3. **Owning layer** -- which layer/component owns the correct fix.
4. **Two candidate fixes, labeled** -- at least one `[workaround]` and one `[source]`. Name what each changes and what each leaves broken for the next caller.
5. **Why not the workaround** -- one sentence on why the local edit is wrong.
