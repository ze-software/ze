# Before Writing Code

**BLOCKING:** Complete these checks BEFORE writing any code, tests, or documentation.

## Core Principle

**You are NOT ALLOWED to write any code until you understand the existing code structure.**

## Pre-Code Checklist

```
[ ] 1. Search for existing implementations
      - Use Grep/Glob to find similar patterns, tests, and functionality
      - If found: STOP. Use it, extend it, or document why new code is needed.

[ ] 2. Read the relevant source files
      - Understand current implementation and patterns
      - Identify extension points

[ ] 3. Check architecture docs
      - Read docs matching task keywords (see planning.md keyword table)

[ ] 4. Understand data flow
      - Entry points, transformations, exit points

[ ] 5. Verify file paths
      - Use Glob/Grep to confirm the target file exists and is correct
      - Never guess file locations from context
      - If user references a file type (e.g., 'article guidelines'), search for it — don't assume it's CLAUDE.md or MEMORY.md

[ ] 6. Buffer-first encoding check (see `rules/buffer-first.md`)
      - If writing wire encoding: use WriteTo(buf, off), NOT Pack()/make([]byte)
      - If calling an encoding function: check if WriteTo exists on the type first
      - If adding a new wire type: implement wire.BufWriter from the start
```

## Verification

Before writing code, you MUST be able to answer:

1. **What existing code relates to this task?** (file paths and function names)
2. **What patterns does the codebase use?** (naming, error handling, testing)
3. **How will your changes integrate?** (callers, callees, shared data structures)

## Before Writing ANY Spec

Before writing or editing ANY spec file (`docs/plan/spec-*.md`):

1. **READ the source files that will be modified** - Not docs, the ACTUAL CODE
2. **Document current behavior** - What does the code do NOW?
3. **Preserve behavior by default** - Unless user explicitly says to change it

**Historical lesson:** Invented a new JSON format instead of reading `decode.go` and preserving the existing one. This wasted money and broke tests.

**Verification question before spec writing:**
> "Have I read the actual source files listed in 'Files to Modify'? Can I describe what they currently do?"

If NO → READ THE CODE FIRST. Do not proceed.

## Red Flags

Stop and investigate if:
- Creating a new file without checking for similar existing files
- Writing a function that might duplicate existing functionality
- You can't name 3 existing files your code relates to
- Creating a new test file when a test case could be added to an existing file

## Document New Understanding

After work, if you learned something new about the codebase:

| What you learned | Where to document |
|------------------|-------------------|
| Wire format behavior | `docs/architecture/wire/` |
| API behavior | `docs/architecture/api/` |
| FSM/session behavior | `docs/architecture/behavior/` |
| Test patterns | `docs/functional-tests.md` |
| RFC interpretation | `rfc/short/` |
