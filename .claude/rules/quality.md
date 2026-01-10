# Quality Standards

## Core Principle

**Do the work properly. No shortcuts.**

## Linting

### Never Disable Linters to Hide Problems

When facing lint issues:
1. **FIX the issue** - this is the only acceptable action
2. **DO NOT** disable linters, checks, or rules to make issues disappear
3. **DO NOT** add exclusion patterns to avoid fixing code

Disabling a linter is hiding problems, not fixing them.

### Acceptable Exclusions

Only these exclusions are permitted:
- `fieldalignment` (govet) - confirm with users if it should be silenced
- Test file exclusions for `dupl`, `goconst`, `prealloc`, `gosec` - tests have different requirements

Everything else: fix the actual code.

### When Facing Many Issues

If there are many lint issues:
1. Create a todo list to track them
2. Fix them systematically, file by file
3. Do not take shortcuts regardless of volume

## Rationale

- Each lint check exists for a reason
- "Style" issues affect readability and maintainability
- Performance warnings are real performance issues
- The linter config reflects project standards - respect it

## What This Means

- If `hugeParam` warns about passing large structs by value: pass by pointer
- If `rangeValCopy` warns about copying in range: use index or pointer
- If `shadow` warns about variable shadowing: rename the variable
- If `emptyStringTest` suggests `s == ""`: use that form
- If `appendCombine` suggests combining appends: combine them

No exceptions. Do the work.
