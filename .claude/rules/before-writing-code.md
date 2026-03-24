# Before Writing Code

**BLOCKING:** Complete before writing any code, tests, or documentation.
Rationale: `.claude/rationale/before-writing-code.md`

```
[ ] 1. Search for existing implementations (Grep/Glob) — extend if found
[ ] 2. Know source files — use digests if available, read + write digest if not
[ ] 3. Verify file paths exist (Glob/Grep)
[ ] 4. Buffer-first check (wire encoding) — see `rules/buffer-first.md`
[ ] 5. Lazy-first check — can the consumer use existing wire type methods directly? See `design-principles.md` "Lazy over eager"
[ ] 6. Bulk-edit check — modifying >2 files with the same pattern? Change ONE first, test it, confirm it works, THEN apply to the rest. Never assume a pattern works across files without validation.
```

Before any spec: READ source files, document current behavior, preserve by default.

**Red flags:** new file without checking for similar existing ones; function that might duplicate existing; can't name 3 related files.
