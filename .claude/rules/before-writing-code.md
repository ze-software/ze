# Before Writing Code

**BLOCKING:** Complete before writing any code, tests, or documentation.
Rationale: `.claude/rationale/before-writing-code.md`

```
[ ] 1. Search for existing implementations (Grep/Glob) — extend if found
[ ] 2. Know source files — use digests if available, read + write digest if not
[ ] 3. Verify file paths exist (Glob/Grep)
[ ] 4. Buffer-first check (wire encoding) — WriteTo(buf, off), not Pack()/make([]byte)
```

Before any spec: READ source files, document current behavior, preserve by default.

**Red flags:** new file without checking for similar existing ones; function that might duplicate existing; can't name 3 related files.
