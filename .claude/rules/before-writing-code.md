# Before Writing Code

**BLOCKING:** Complete these checks BEFORE writing any code, tests, or documentation.

## Pre-Code Checklist

```
[ ] 1. Search for existing implementations
      - Use Grep/Glob to find similar patterns, tests, and functionality
      - If found: STOP. Use it, extend it, or document why new code is needed.

[ ] 2. Know the source files you will modify
      - If file digest exists in session-state.md: use that (don't re-read full file)
      - If no digest: read the file, then write a digest to session-state.md
      - Only re-read full file when digest lacks detail needed for the specific edit

[ ] 3. Verify file paths
      - Use Glob/Grep to confirm the target file exists and is correct
      - Never guess file locations from context

[ ] 4. Buffer-first encoding check (if writing wire encoding)
      - Use WriteTo(buf, off), NOT Pack()/make([]byte)
      - Check if WriteTo exists on the type first
      - New wire types: implement wire.BufWriter from the start
```

## Before Writing ANY Spec

Before writing or editing ANY spec file (`docs/plan/spec-*.md`):

1. **READ the source files that will be modified** - Not docs, the ACTUAL CODE
2. **Document current behavior** - What does the code do NOW?
3. **Preserve behavior by default** - Unless user explicitly says to change it

**Historical lesson:** Invented a new JSON format instead of reading `decode.go` and preserving the existing one.

## Red Flags

Stop and investigate if:
- Creating a new file without checking for similar existing files
- Writing a function that might duplicate existing functionality
- You can't name 3 existing files your code relates to

## Document New Understanding

After work, if you learned something new about the codebase:

| What you learned | Where to document |
|------------------|-------------------|
| Wire format behavior | `docs/architecture/wire/` |
| API behavior | `docs/architecture/api/` |
| FSM/session behavior | `docs/architecture/behavior/` |
| Test patterns | `docs/functional-tests.md` |
| RFC interpretation | `rfc/short/` |
