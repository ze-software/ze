# Before Writing Code Rationale

Why: `.claude/rules/before-writing-code.md`

## Historical Lesson
Invented a new JSON format instead of reading `decode.go` and preserving the existing one. This is why "read source files, document current behavior, preserve by default" exists.

## Document New Understanding Table

After work, if you learned something new about the codebase:

| What you learned | Where to document |
|------------------|-------------------|
| Wire format behavior | `docs/architecture/wire/` |
| API behavior | `docs/architecture/api/` |
| FSM/session behavior | `docs/architecture/behavior/` |
| Test patterns | `docs/functional-tests.md` |
| RFC interpretation | `rfc/short/` |

## Why Each Checklist Step

1. **Search for existing implementations** -- If found: STOP. Use it, extend it, or document why new code is needed. Prevents duplication.
2. **Know the source files** -- Use file digests from per-spec session state when available. Only re-read full file when digest lacks detail for the specific edit.
3. **Verify file paths** -- Never guess file locations from context. Use Glob/Grep to confirm.
4. **Buffer-first encoding check** -- Check if `WriteTo` exists on the type first. New wire types: implement `wire.BufWriter` from the start.
