# Before Writing Code

Rationale: `.claude/rationale/before-writing-code.md`

**BLOCKING:** Complete before writing any code, tests, or documentation.

```
[ ] 1. Search for existing implementations (Grep/Glob) — use/extend if found
[ ] 2. Know source files to modify — use digests if available, read + write digest if not
[ ] 3. Verify file paths — Glob/Grep to confirm target exists
[ ] 4. Buffer-first check (wire encoding) — WriteTo(buf, off), not Pack()/make([]byte)
```

## Before Any Spec

1. READ actual source files to modify (not docs)
2. Document current behavior
3. Preserve behavior by default

## Red Flags

- Creating new file without checking for similar existing files
- Writing function that might duplicate existing functionality
- Can't name 3 existing files your code relates to
